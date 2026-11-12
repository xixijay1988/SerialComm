package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	serial "go.bug.st/serial"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	maxRXRecords   = 2000
	maxRecordBytes = 512
)

var baudRates = []string{
	"300", "600", "1200", "2400", "4800", "9600", "14400", "19200",
	"38400", "57600", "115200", "128000", "230400", "250000", "256000",
	"460800", "500000", "576000", "921600", "1000000", "1500000", "2000000",
}

var (
	suffixes  = []string{"None", "CR (\\r)", "LF (\\n)", "CRLF (\\r\\n)"}
	intervals = []string{"50 ms", "100 ms", "200 ms", "500 ms", "1000 ms", "2000 ms", "5000 ms", "10000 ms"}
)

// record direction tags
const (
	dirRX  = iota // received data
	dirTX         // sent data echo
	dirSYS        // system messages (open/close/error)
)

// rxRecord is one line in the console.
type rxRecord struct {
	id   int
	ts   time.Time
	dir  int
	data []byte
}

// sendSnap is a mutex-protected copy of the send configuration the
// periodic goroutine uses, so it never reads view fields directly.
type sendSnap struct {
	text     string
	hex      bool
	suffix   string
	checksum string
	periodic bool
	interval time.Duration
}

// session is one serial connection with its own console and composer
// state. Fields are touched on the main thread only, except pending/snap
// which are mutex protected.
type session struct {
	a    *app
	id   int
	name string

	// port configuration
	portSel  string
	baud     string
	dataBits string
	parity   string
	stopBits string
	dtr      bool
	rts      bool

	// open port
	port    serial.Port
	portMu  sync.Mutex // guards port + opened
	opened  bool
	devName string // device path of the open port
	cts     bool
	dsr     bool

	// console
	rxList     ui.ListState
	rxRecords  []rxRecord
	filtered   []rxRecord
	nextRecID  int
	dispMode   int // 0 text, 1 hex, 2 mixed
	showRX     bool
	showTX     bool
	showTS     bool
	paused     bool
	autofollow bool
	query      string
	rxCount    int
	txCount    int

	rxMu      sync.Mutex // guards pending + scheduled
	pending   []byte
	scheduled bool

	// send / composer
	sendText   string
	sendHex    bool
	suffix     string
	checksum   string
	periodic   bool
	interval   string
	composeTab int

	snapMu       sync.Mutex // guards snap + lastPeriodic
	snap         sendSnap
	lastPeriodic time.Time

	status     string
	statusKind int // 0 muted, 1 success, 2 danger
}

// app is the window state: the shared port list, the session set, and
// app-level UI preferences.
type app struct {
	win atomic.Pointer[mygo.Window]

	ports      []portInfo
	refreshing atomic.Bool

	sessMu   sync.RWMutex // guards sessions
	sessions []*session
	active   int
	nextSID  int
	showSide bool

	// right panel / commands
	showPanel  bool
	rightTab   int
	cmdGroup   string
	customCmds map[string][]quickCmd
	addOpen    bool
	addName    string
	addValue   string

	themeMode int // 0 system, 1 light, 2 dark
	dark      bool

	searchH   ui.Handle
	wantFocus bool
}

func main() {
	a := &app{
		showSide:   true,
		showPanel:  true,
		dark:       true,
		cmdGroup:   cmdGroupNames[0],
		customCmds: map[string][]quickCmd{},
	}
	a.newSession()
	a.setPorts(listPortsQuick())
	go a.watchPorts()
	go a.periodicLoop()
	mygo.App.WhenReady(func() {
		a.win.Store(mygo.NewWindow(mygo.WindowOptions{
			Title:     "SerialComm",
			Width:     1280,
			Height:    800,
			MinWidth:  1060,
			MinHeight: 660,
			StateKey:  "main",
			Content:   ui.View(a.view),
		}))
		mygo.Theme.SetSource(mygo.ThemeSystem)
		a.dark = mygo.Theme.IsDark()
		mygo.Theme.OnUpdated(func() {
			if w := a.win.Load(); w != nil {
				w.Update(func() { a.dark = mygo.Theme.IsDark() })
			}
		})
		// Enrich the port list with USB details once the window exists.
		a.refreshPortsAsync()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// portLabels returns the select options for the current port list.
func (a *app) portLabels() []string {
	labels := make([]string, len(a.ports))
	for i, p := range a.ports {
		labels[i] = p.label()
	}
	if len(labels) == 0 {
		labels = []string{"无可用串口"}
	}
	return labels
}

// --- sessions ---------------------------------------------------------

func (a *app) newSession() *session {
	a.nextSID++
	s := &session{
		a:          a,
		id:         a.nextSID,
		baud:       "115200",
		dataBits:   "8",
		parity:     "None",
		stopBits:   "1",
		dtr:        true,
		rts:        true,
		suffix:     "None",
		checksum:   "None",
		interval:   "1000 ms",
		showRX:     true,
		showTX:     true,
		autofollow: true,
		status:     "请选择串口设备",
	}
	s.name = fmt.Sprintf("会话 %d", s.id)
	a.sessMu.Lock()
	a.sessions = append(a.sessions, s)
	a.sessMu.Unlock()
	a.active = len(a.sessions) - 1
	if a.portSelValid(s) == false && len(a.ports) > 0 {
		s.portSel = a.ports[0].label()
	}
	return s
}

// portSelValid reports whether the session's selection still exists.
func (a *app) portSelValid(s *session) bool {
	for _, p := range a.ports {
		if p.label() == s.portSel {
			return true
		}
	}
	return false
}

// closeSession removes the session, closing its port first.
func (a *app) closeSession(i int) {
	if len(a.sessions) <= 1 {
		return
	}
	s := a.sessions[i]
	s.closePort()
	a.sessMu.Lock()
	a.sessions = append(a.sessions[:i], a.sessions[i+1:]...)
	a.sessMu.Unlock()
	if a.active >= len(a.sessions) {
		a.active = len(a.sessions) - 1
	}
}

// eachSession iterates the sessions under the read lock, for background
// goroutines. The main thread may read a.sessions directly.
func (a *app) eachSession(fn func(*session)) {
	a.sessMu.RLock()
	defer a.sessMu.RUnlock()
	for _, s := range a.sessions {
		fn(s)
	}
}

// activeSession returns the session shown in the main area.
func (a *app) activeSession() *session {
	if len(a.sessions) == 0 {
		return nil
	}
	if a.active >= len(a.sessions) {
		a.active = len(a.sessions) - 1
	}
	return a.sessions[a.active]
}

// --- port management -------------------------------------------------

// setPorts replaces the port list and fixes session selections.
// Main thread only.
func (a *app) setPorts(ports []portInfo) {
	a.ports = ports
	for _, s := range a.sessions {
		if !a.portSelValid(s) {
			if len(a.ports) > 0 {
				s.portSel = a.ports[0].label()
			} else {
				s.portSel = ""
			}
		}
	}
}

// refreshPortsAsync runs the slow ioreg-enriched enumeration off the main
// thread, then applies it and detects unplug of open ports. Safe to call
// from anywhere; concurrent calls collapse into one.
func (a *app) refreshPortsAsync() {
	if !a.refreshing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer a.refreshing.Store(false)
		ports := listPorts()
		w := a.win.Load()
		if w == nil {
			return
		}
		w.Update(func() {
			a.setPorts(ports)
			for _, s := range a.sessions {
				if s.opened && !portNameListed(ports, s.devName) {
					s.closePort()
					s.setStatus("端口已被拔出", 2)
					s.sysLog("port removed: " + s.devName)
				}
			}
		})
	}()
}

func portNameListed(ports []portInfo, name string) bool {
	for _, p := range ports {
		if p.Name == name {
			return true
		}
	}
	return false
}

// watchPorts polls the device list cheaply and triggers a full refresh
// only when the set of devices changes. Runs for the life of the process.
func (a *app) watchPorts() {
	var last []string
	for range time.Tick(1500 * time.Millisecond) {
		names := portNames()
		if !slices.Equal(names, last) {
			last = names
			a.refreshPortsAsync()
		}
		a.eachSession(func(s *session) { s.pollModemBits() })
	}
}

// pollModemBits reads CTS/DSR while the port is open.
func (s *session) pollModemBits() {
	s.portMu.Lock()
	p := s.port
	s.portMu.Unlock()
	if p == nil {
		return
	}
	bits, err := p.GetModemStatusBits()
	if err != nil {
		return
	}
	if w := s.a.win.Load(); w != nil {
		w.Update(func() {
			s.cts = bits.CTS
			s.dsr = bits.DSR
		})
	}
}

// selectedPort returns the port the session's selector points at.
func (s *session) selectedPort() *portInfo {
	for i := range s.a.ports {
		if s.a.ports[i].label() == s.portSel {
			return &s.a.ports[i]
		}
	}
	return nil
}

// portInUse reports whether another session holds this device open.
func (a *app) portInUse(name string, except *session) *session {
	for _, s := range a.sessions {
		if s != except && s.opened && s.devName == name {
			return s
		}
	}
	return nil
}

func (s *session) togglePort() {
	a := s.a
	if s.opened {
		s.closePort()
		s.setStatus("已断开", 0)
		s.sysLog("串口已断开")
		return
	}
	p := s.selectedPort()
	if p == nil {
		s.setStatus("未选择串口", 2)
		return
	}
	if other := a.portInUse(p.Name, s); other != nil {
		s.setStatus(fmt.Sprintf("端口已被 %s 占用", other.name), 2)
		return
	}
	mode, err := portMode(s.baud, s.dataBits, s.parity, s.stopBits)
	if err != nil {
		s.setStatus(err.Error(), 2)
		return
	}
	port, err := serial.Open(p.Name, mode)
	if err != nil {
		s.setStatus("打开失败: "+err.Error(), 2)
		return
	}
	s.portMu.Lock()
	s.port = port
	s.opened = true
	s.devName = p.Name
	s.portMu.Unlock()
	_ = port.SetDTR(s.dtr)
	_ = port.SetRTS(s.rts)
	s.setStatus(fmt.Sprintf("已连接 %s @ %d bps", p.Base, mode.BaudRate), 1)
	s.sysLog(fmt.Sprintf("串口已打开 %s @ %d bps, %s/%s/%s",
		p.Name, mode.BaudRate, s.dataBits, s.parity, s.stopBits))
	go s.readLoop(port)
}

func (s *session) closePort() {
	s.portMu.Lock()
	defer s.portMu.Unlock()
	if s.port != nil {
		_ = s.port.Close()
		s.port = nil
	}
	s.opened = false
	s.cts, s.dsr = false, false
}

// applyMode re-configures the open port after a parameter change.
func (s *session) applyMode() {
	s.portMu.Lock()
	defer s.portMu.Unlock()
	if s.port == nil {
		return
	}
	mode, err := portMode(s.baud, s.dataBits, s.parity, s.stopBits)
	if err != nil {
		s.setStatus(err.Error(), 2)
		return
	}
	if err := s.port.SetMode(mode); err != nil {
		s.setStatus("参数设置失败: "+err.Error(), 2)
		return
	}
	s.setStatus(fmt.Sprintf("%d bps", mode.BaudRate), 1)
}

func (s *session) setModemBits() {
	s.portMu.Lock()
	defer s.portMu.Unlock()
	if s.port == nil {
		return
	}
	if err := s.port.SetDTR(s.dtr); err != nil {
		s.setStatus("DTR: "+err.Error(), 2)
	}
	if err := s.port.SetRTS(s.rts); err != nil {
		s.setStatus("RTS: "+err.Error(), 2)
	}
}

// --- receive ---------------------------------------------------------

func (s *session) readLoop(p serial.Port) {
	buf := make([]byte, 8192)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			s.rxMu.Lock()
			s.pending = append(s.pending, buf[:n]...)
			need := !s.scheduled
			s.scheduled = true
			s.rxMu.Unlock()
			if need {
				s.a.win.Load().Update(func() {
					s.rxMu.Lock()
					data := s.pending
					s.pending = nil
					s.scheduled = false
					s.rxMu.Unlock()
					s.rxCount += len(data)
					if !s.paused {
						s.appendRecord(dirRX, data)
					}
				})
			}
		}
		if err != nil {
			s.a.win.Load().Update(func() {
				s.portMu.Lock()
				stillOpen := s.opened && s.port == p
				s.portMu.Unlock()
				if stillOpen {
					s.closePort()
					s.setStatus("读取错误: "+err.Error(), 2)
					s.sysLog("read error: " + err.Error())
				}
			})
			return
		}
	}
}

// appendRecord adds console lines; main thread only. Data is split at
// newlines so each record is one display line, and capped at
// maxRecordBytes so a single large read cannot create a giant row. A
// line split across reads continues the previous record instead of
// becoming an empty or partial row.
func (s *session) appendRecord(dir int, data []byte) {
	for len(data) > 0 {
		if n := len(s.rxRecords); n > 0 {
			last := &s.rxRecords[n-1]
			open := last.dir == dir && dir == dirRX && len(last.data) < maxRecordBytes &&
				(len(last.data) == 0 || last.data[len(last.data)-1] != '\n')
			if open {
				m := maxRecordBytes - len(last.data)
				if m > len(data) {
					m = len(data)
				}
				if i := bytes.IndexByte(data[:m], '\n'); i >= 0 {
					m = i + 1
				}
				last.data = append(last.data, data[:m]...)
				data = data[m:]
				continue
			}
		}
		n := len(data)
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			n = i + 1
		}
		if n > maxRecordBytes {
			n = maxRecordBytes
		}
		s.nextRecID++
		s.rxRecords = append(s.rxRecords, rxRecord{
			id: s.nextRecID, ts: time.Now(), dir: dir, data: data[:n],
		})
		data = data[n:]
	}
	if len(s.rxRecords) > maxRXRecords {
		s.rxRecords = s.rxRecords[len(s.rxRecords)-maxRXRecords:]
	}
}

func (s *session) sysLog(msg string) {
	s.appendRecord(dirSYS, []byte(msg))
}

// recordText renders one record under the current display mode.
func (s *session) recordText(r rxRecord) string {
	if r.dir == dirSYS {
		return printable(r.data)
	}
	switch s.dispMode {
	case 1:
		return hexDump(r.data, 16)
	case 2:
		text := printable(r.data)
		h := hexDump(r.data, 0)
		if len(h) > 64 {
			h = h[:64] + " …"
		}
		return text + "   │  " + h
	default:
		return printable(r.data)
	}
}

// filterRecords applies the RX/TX toggles and the search query.
func (s *session) filterRecords() {
	q := strings.ToLower(s.query)
	s.filtered = s.filtered[:0]
	for _, r := range s.rxRecords {
		switch r.dir {
		case dirRX:
			if !s.showRX {
				continue
			}
		case dirTX:
			if !s.showTX {
				continue
			}
		}
		if q != "" && !strings.Contains(strings.ToLower(s.recordText(r)), q) {
			continue
		}
		s.filtered = append(s.filtered, r)
	}
}

// dirTag returns the label and color for a record direction.
func (a *app) dirTag(dir int) (string, ui.Color) {
	p := a.pal()
	switch dir {
	case dirTX:
		return "TX", p.tx
	case dirSYS:
		return "SYS", p.sys
	}
	return "RX", p.rx
}

func (s *session) saveLog() {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      s.a.win.Load(),
		Title:       "保存日志",
		DefaultPath: "serialcomm.log",
	})
	if err != nil || path == "" {
		return
	}
	var b strings.Builder
	for _, r := range s.rxRecords {
		tag, _ := s.a.dirTag(r.dir)
		fmt.Fprintf(&b, "[%s] %s %s\n",
			r.ts.Format("15:04:05.000"), tag, s.recordText(r))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		s.setStatus("保存失败: "+err.Error(), 2)
		return
	}
	s.setStatus("已保存: "+path, 1)
}

// copyLog copies the rendered log to the clipboard.
func (s *session) copyLog(svc ui.Services) {
	var b strings.Builder
	for _, r := range s.rxRecords {
		tag, _ := s.a.dirTag(r.dir)
		fmt.Fprintf(&b, "[%s] %s %s\n",
			r.ts.Format("15:04:05.000"), tag, s.recordText(r))
	}
	svc.WriteClipboard(b.String())
	s.setStatus("日志已复制到剪贴板", 1)
}

// --- send ------------------------------------------------------------

func (s *session) updateSnap() {
	s.snapMu.Lock()
	defer s.snapMu.Unlock()
	s.snap = sendSnap{
		text:     s.sendText,
		hex:      s.sendHex,
		suffix:   s.suffix,
		checksum: s.checksum,
		periodic: s.periodic,
		interval: parseInterval(s.interval),
	}
}

func parseInterval(str string) time.Duration {
	n, _ := strconv.Atoi(strings.Fields(str)[0])
	return time.Duration(n) * time.Millisecond
}

// payload builds the bytes to send from a sendSnap: data, line ending,
// then the checksum over everything.
func payload(s sendSnap) ([]byte, error) {
	var data []byte
	if s.hex {
		b, err := parseHex(s.text)
		if err != nil {
			return nil, err
		}
		data = b
	} else {
		data = []byte(s.text)
	}
	switch s.suffix {
	case "CR (\\r)":
		data = append(data, '\r')
	case "LF (\\n)":
		data = append(data, '\n')
	case "CRLF (\\r\\n)":
		data = append(data, '\r', '\n')
	}
	data = appendChecksum(data, s.checksum)
	if len(data) == 0 {
		return nil, fmt.Errorf("请输入要发送的数据")
	}
	return data, nil
}

func (s *session) sendNow() {
	s.updateSnap()
	data, err := payload(s.snap)
	if err != nil {
		s.setStatus("发送失败: "+err.Error(), 2)
		return
	}
	n, err := s.writePort(data)
	if err != nil {
		s.setStatus("发送失败: "+err.Error(), 2)
		return
	}
	s.txCount += n
	s.appendRecord(dirTX, data)
	s.setStatus(fmt.Sprintf("已发送 %d 字节", n), 1)
}

// sendCommand sends a quick-command string through the normal pipeline.
func (s *session) sendCommand(v string) {
	s.sendText = v
	s.sendNow()
}

func (s *session) writePort(data []byte) (int, error) {
	s.portMu.Lock()
	defer s.portMu.Unlock()
	if s.port == nil {
		return 0, fmt.Errorf("串口未打开")
	}
	return s.port.Write(data)
}

// periodicLoop resends the configured payload while periodic send is on,
// independently for each session.
func (a *app) periodicLoop() {
	for {
		time.Sleep(50 * time.Millisecond)
		now := time.Now()
		a.eachSession(func(s *session) {
			s.snapMu.Lock()
			snap := s.snap
			due := snap.periodic && snap.interval > 0 &&
				now.Sub(s.lastPeriodic) >= snap.interval
			if due {
				s.lastPeriodic = now
			}
			s.snapMu.Unlock()
			if !due {
				return
			}
			data, err := payload(snap)
			if err != nil {
				return
			}
			n, err := s.writePort(data)
			if err == nil && n > 0 {
				if w := a.win.Load(); w != nil {
					w.Update(func() {
						s.txCount += n
						if s.showTX {
							s.appendRecord(dirTX, data)
						}
					})
				}
			}
		})
	}
}

func (s *session) setStatus(str string, kind int) {
	s.status, s.statusKind = str, kind
}
