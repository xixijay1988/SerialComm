package main

import (
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

const maxRXRecords = 2000

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

// app is the state the window shows. All fields are touched on the main
// thread only, except pending/snap which are mutex protected.
type app struct {
	win atomic.Pointer[mygo.Window]

	// port configuration
	ports    []portInfo
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

	refreshing atomic.Bool

	// console
	rxList    ui.ListState
	rxRecords []rxRecord
	filtered  []rxRecord
	nextRecID int
	dispMode  int // 0 text, 1 hex, 2 mixed
	showRX    bool
	showTX    bool
	showTS    bool
	paused    bool
	autofollow bool
	query     string
	searchH   ui.Handle
	wantFocus bool
	rxCount   int
	txCount   int

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

	snapMu sync.Mutex // guards snap
	snap   sendSnap

	// right panel / commands
	showPanel  bool
	rightTab   int
	cmdGroup   string
	customCmds map[string][]quickCmd
	addOpen    bool
	addName    string
	addValue   string

	dark       bool
	status     string
	statusKind int // 0 muted, 1 success, 2 danger
}

func main() {
	a := &app{
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
		showPanel:  true,
		dark:       true,
		cmdGroup:   cmdGroupNames[0],
		customCmds: map[string][]quickCmd{},
		status:     "请选择串口设备",
	}
	a.setPorts(listPortsQuick())
	go a.watchPorts()
	go a.periodicLoop()
	mygo.App.WhenReady(func() {
		a.win.Store(mygo.NewWindow(mygo.WindowOptions{
			Title:     "SerialComm",
			Width:     1180,
			Height:    760,
			MinWidth:  900,
			MinHeight: 620,
			StateKey:  "main",
			Content:   ui.View(a.view),
		}))
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

// --- port management -------------------------------------------------

// setPorts replaces the port list and fixes the selection. Pure — runs on
// the main thread.
func (a *app) setPorts(ports []portInfo) {
	a.ports = ports
	found := false
	for _, p := range a.ports {
		if p.label() == a.portSel {
			found = true
		}
	}
	if !found && len(a.ports) > 0 {
		a.portSel = a.ports[0].label()
	}
	if len(a.ports) == 0 {
		a.portSel = ""
	}
}

// refreshPortsAsync runs the slow ioreg-enriched enumeration off the main
// thread, then applies it and detects unplug of the open port. Safe to
// call from anywhere; concurrent calls collapse into one.
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
			gone := a.opened && !portNameListed(ports, a.devName)
			a.setPorts(ports)
			if gone {
				a.closePort()
				a.setStatus("端口已被拔出", 2)
				a.sysLog("port removed: " + a.devName)
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
		a.pollModemBits()
	}
}

// pollModemBits reads CTS/DSR while the port is open.
func (a *app) pollModemBits() {
	a.portMu.Lock()
	p := a.port
	a.portMu.Unlock()
	if p == nil {
		return
	}
	bits, err := p.GetModemStatusBits()
	if err != nil {
		return
	}
	if w := a.win.Load(); w != nil {
		w.Update(func() {
			a.cts = bits.CTS
			a.dsr = bits.DSR
		})
	}
}

func (a *app) selectedPort() *portInfo {
	for i := range a.ports {
		if a.ports[i].label() == a.portSel {
			return &a.ports[i]
		}
	}
	return nil
}

func (a *app) togglePort() {
	if a.opened {
		a.closePort()
		a.setStatus("已断开", 0)
		a.sysLog("串口已断开")
		return
	}
	p := a.selectedPort()
	if p == nil {
		a.setStatus("未选择串口", 2)
		return
	}
	mode, err := portMode(a.baud, a.dataBits, a.parity, a.stopBits)
	if err != nil {
		a.setStatus(err.Error(), 2)
		return
	}
	port, err := serial.Open(p.Name, mode)
	if err != nil {
		a.setStatus("打开失败: "+err.Error(), 2)
		return
	}
	a.portMu.Lock()
	a.port = port
	a.opened = true
	a.devName = p.Name
	a.portMu.Unlock()
	_ = port.SetDTR(a.dtr)
	_ = port.SetRTS(a.rts)
	a.setStatus(fmt.Sprintf("已连接 %s @ %d bps", p.Base, mode.BaudRate), 1)
	a.sysLog(fmt.Sprintf("串口已打开 %s @ %d bps, %s/%s/%s",
		p.Name, mode.BaudRate, a.dataBits, a.parity, a.stopBits))
	go a.readLoop(port)
}

func (a *app) closePort() {
	a.portMu.Lock()
	defer a.portMu.Unlock()
	if a.port != nil {
		_ = a.port.Close()
		a.port = nil
	}
	a.opened = false
	a.cts, a.dsr = false, false
}

// applyMode re-configures the open port after a parameter change.
func (a *app) applyMode() {
	a.portMu.Lock()
	defer a.portMu.Unlock()
	if a.port == nil {
		return
	}
	mode, err := portMode(a.baud, a.dataBits, a.parity, a.stopBits)
	if err != nil {
		a.setStatus(err.Error(), 2)
		return
	}
	if err := a.port.SetMode(mode); err != nil {
		a.setStatus("参数设置失败: "+err.Error(), 2)
		return
	}
	a.setStatus(fmt.Sprintf("%d bps", mode.BaudRate), 1)
}

func (a *app) setModemBits() {
	a.portMu.Lock()
	defer a.portMu.Unlock()
	if a.port == nil {
		return
	}
	if err := a.port.SetDTR(a.dtr); err != nil {
		a.setStatus("DTR: "+err.Error(), 2)
	}
	if err := a.port.SetRTS(a.rts); err != nil {
		a.setStatus("RTS: "+err.Error(), 2)
	}
}

// --- receive ---------------------------------------------------------

func (a *app) readLoop(p serial.Port) {
	buf := make([]byte, 8192)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			a.rxMu.Lock()
			a.pending = append(a.pending, buf[:n]...)
			need := !a.scheduled
			a.scheduled = true
			a.rxMu.Unlock()
			if need {
				a.win.Load().Update(func() {
					a.rxMu.Lock()
					data := a.pending
					a.pending = nil
					a.scheduled = false
					a.rxMu.Unlock()
					a.rxCount += len(data)
					if !a.paused {
						a.appendRecord(dirRX, data)
					}
				})
			}
		}
		if err != nil {
			a.win.Load().Update(func() {
				a.portMu.Lock()
				stillOpen := a.opened && a.port == p
				a.portMu.Unlock()
				if stillOpen {
					a.closePort()
					a.setStatus("读取错误: "+err.Error(), 2)
					a.sysLog("read error: " + err.Error())
				}
			})
			return
		}
	}
}

// appendRecord adds a console line; main thread only.
func (a *app) appendRecord(dir int, data []byte) {
	a.nextRecID++
	a.rxRecords = append(a.rxRecords, rxRecord{
		id: a.nextRecID, ts: time.Now(), dir: dir, data: data,
	})
	if len(a.rxRecords) > maxRXRecords {
		a.rxRecords = a.rxRecords[len(a.rxRecords)-maxRXRecords:]
	}
}

func (a *app) sysLog(msg string) {
	a.appendRecord(dirSYS, []byte(msg))
}

// recordText renders one record under the current display mode.
func (a *app) recordText(r rxRecord) string {
	if r.dir == dirSYS {
		return printable(r.data)
	}
	switch a.dispMode {
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
func (a *app) filterRecords() {
	q := strings.ToLower(a.query)
	a.filtered = a.filtered[:0]
	for _, r := range a.rxRecords {
		switch r.dir {
		case dirRX:
			if !a.showRX {
				continue
			}
		case dirTX:
			if !a.showTX {
				continue
			}
		}
		if q != "" && !strings.Contains(strings.ToLower(a.recordText(r)), q) {
			continue
		}
		a.filtered = append(a.filtered, r)
	}
}

func (a *app) saveLog() {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.win.Load(),
		Title:       "保存日志",
		DefaultPath: "serialcomm.log",
	})
	if err != nil || path == "" {
		return
	}
	var s strings.Builder
	for _, r := range a.rxRecords {
		dir := "RX"
		if r.dir == dirTX {
			dir = "TX"
		} else if r.dir == dirSYS {
			dir = "SYS"
		}
		fmt.Fprintf(&s, "[%s] %s %s\n", r.ts.Format("15:04:05.000"), dir, a.recordText(r))
	}
	if err := os.WriteFile(path, []byte(s.String()), 0o644); err != nil {
		a.setStatus("保存失败: "+err.Error(), 2)
		return
	}
	a.setStatus("已保存: "+path, 1)
}

// --- send ------------------------------------------------------------

func (a *app) updateSnap() {
	a.snapMu.Lock()
	defer a.snapMu.Unlock()
	a.snap = sendSnap{
		text:     a.sendText,
		hex:      a.sendHex,
		suffix:   a.suffix,
		checksum: a.checksum,
		periodic: a.periodic,
		interval: parseInterval(a.interval),
	}
}

func parseInterval(s string) time.Duration {
	n, _ := strconv.Atoi(strings.Fields(s)[0])
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

func (a *app) sendNow() {
	a.updateSnap()
	data, err := payload(a.snap)
	if err != nil {
		a.setStatus("发送失败: "+err.Error(), 2)
		return
	}
	n, err := a.writePort(data)
	if err != nil {
		a.setStatus("发送失败: "+err.Error(), 2)
		return
	}
	a.txCount += n
	a.appendRecord(dirTX, data)
	a.setStatus(fmt.Sprintf("已发送 %d 字节", n), 1)
}

// sendCommand sends a quick-command string through the normal pipeline.
func (a *app) sendCommand(v string) {
	a.sendText = v
	a.sendNow()
}

func (a *app) writePort(data []byte) (int, error) {
	a.portMu.Lock()
	defer a.portMu.Unlock()
	if a.port == nil {
		return 0, fmt.Errorf("串口未打开")
	}
	return a.port.Write(data)
}

// periodicLoop resends the configured payload while periodic send is on.
func (a *app) periodicLoop() {
	for {
		a.snapMu.Lock()
		s := a.snap
		a.snapMu.Unlock()
		if !s.periodic || s.interval <= 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		time.Sleep(s.interval)
		data, err := payload(s)
		if err != nil {
			continue
		}
		n, err := a.writePort(data)
		if err == nil && n > 0 {
			if w := a.win.Load(); w != nil {
				w.Update(func() {
					a.txCount += n
					if a.showTX {
						a.appendRecord(dirTX, data)
					}
				})
			}
		}
	}
}

func (a *app) setStatus(s string, kind int) {
	a.status, a.statusKind = s, kind
}
