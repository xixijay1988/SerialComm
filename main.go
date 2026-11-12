package main

import (
	"fmt"
	"log"
	"os"
	"slices"
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

var suffixes = []string{"None", "\\r", "\\n", "\\r\\n", "\\n\\r"}

// rxRecord is one batch of bytes received from the port.
type rxRecord struct {
	id   int
	ts   time.Time
	data []byte
}

// app is the state the window shows. All fields are touched on the main
// thread only, except pending/snapshot which are mutex protected.
type app struct {
	win atomic.Pointer[mygo.Window]

	// port configuration
	ports    []portInfo
	portSel  string // label of the selected port
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

	refreshing atomic.Bool

	// receive
	rxList    ui.ListState
	rxRecords []rxRecord
	nextRecID int
	dispMode  int // 0 text, 1 hex
	showTS    bool
	paused    bool
	rxCount   int
	txCount   int

	rxMu      sync.Mutex // guards pending + scheduled
	pending   []byte
	scheduled bool

	// send
	sendText string
	sendHex  bool
	suffix   string
	periodic bool
	periodMs float64

	snapMu sync.Mutex // guards snap
	snap   sendSnap

	status     string
	statusKind int // 0 muted, 1 success, 2 danger
}

// sendSnap is a mutex-protected copy of the send configuration the
// periodic goroutine uses, so it never reads view fields directly.
type sendSnap struct {
	text     string
	hex      bool
	suffix   string
	periodic bool
	interval time.Duration
}

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Gap(8).Padding(10).Children(func() {
		a.viewConfig(c)
		a.viewStatusLine(c, t)
		a.viewReceive(c)
		a.viewReceiveBar(c, t)
		a.viewSend(c)
	})
}

// viewConfig is the row of port parameters, as in SSCOM.
func (a *app) viewConfig(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		labels := make([]string, len(a.ports))
		for i, p := range a.ports {
			labels[i] = p.label()
		}
		ui.Select(c, &a.portSel, labels).
			Label("Port").MinWidth(280).Grow(1).Disabled(a.opened)
		ui.Button(c, "Refresh").OnClick(a.refreshPortsAsync)
		ui.Checkbox(c, &a.dtr, "DTR").Disabled(!a.opened).
			OnChange(func() { a.setModemBits() })
		ui.Checkbox(c, &a.rts, "RTS").Disabled(!a.opened).
			OnChange(func() { a.setModemBits() })
		ui.Spacer(c).Grow(1)
		open := ui.PrimaryButton(c, "Open")
		if a.opened {
			open = ui.PrimaryButton(c, "Close")
		}
		open.MinWidth(96).OnClick(a.togglePort)
	})
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Baud").TextColor(t.TextMuted)
		ui.Combobox(c, &a.baud, baudRates).
			Label("Baud rate").Width(100).
			OnChange(a.applyMode)
		ui.Text(c, "Data").TextColor(t.TextMuted)
		ui.Select(c, &a.dataBits, []string{"5", "6", "7", "8"}).
			Label("Data bits").Width(64).
			OnChange(a.applyMode)
		ui.Text(c, "Parity").TextColor(t.TextMuted)
		ui.Select(c, &a.parity, []string{"None", "Odd", "Even", "Mark", "Space"}).
			Label("Parity").Width(88).
			OnChange(a.applyMode)
		ui.Text(c, "Stop").TextColor(t.TextMuted)
		ui.Select(c, &a.stopBits, []string{"1", "1.5", "2"}).
			Label("Stop bits").Width(64).
			OnChange(a.applyMode)
	})
}

// viewStatusLine shows the port state, byte counters and a message.
func (a *app) viewStatusLine(c *ui.Context, t *ui.Theme) {
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		state, color := "Closed", t.TextMuted
		if a.opened {
			state, color = "Opened: "+a.devName, t.Success
		}
		ui.Text(c, state).TextColor(color)
		ui.Textf(c, "RX: %d   TX: %d", a.rxCount, a.txCount).
			TextColor(t.TextMuted)
		ui.Button(c, "Reset counters").OnClick(func() {
			a.rxCount, a.txCount = 0, 0
		})
		ui.Spacer(c).Grow(1)
		sc := t.TextMuted
		switch a.statusKind {
		case 1:
			sc = t.Success
		case 2:
			sc = t.Danger
		}
		ui.Text(c, a.status).TextColor(sc).SingleLine()
	})
}

// viewReceive is the scrolling receive area.
func (a *app) viewReceive(c *ui.Context) {
	a.rxList.FollowEnd = true
	a.rxList.Key = func(row int) any { return a.rxRecords[row].id }
	ui.List(c, &a.rxList, len(a.rxRecords), func(i int) {
		ui.Text(c, a.recordText(a.rxRecords[i])).
			Font("monospace").FontSize(12).Selectable()
	}).Fill().Border(1, c.Theme().Border).Radius(6)
}

// viewReceiveBar holds the display options under the receive area.
func (a *app) viewReceiveBar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Segmented(c, &a.dispMode, "Text", "HEX").Label("Display format")
		ui.Checkbox(c, &a.showTS, "Timestamp")
		ui.Checkbox(c, &a.paused, "Pause")
		ui.Spacer(c).Grow(1)
		ui.Button(c, "Clear").OnClick(func() {
			a.rxRecords = nil
		})
		ui.Button(c, "Save…").OnClick(a.saveLog)
	})
}

// viewSend is the send input and its options.
func (a *app) viewSend(c *ui.Context) {
	ui.TextArea(c, &a.sendText).
		Placeholder("Data to send…").
		Label("Data to send").
		MinHeight(64).MaxHeight(110).FillWidth().
		OnChange(a.updateSnap)
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Checkbox(c, &a.sendHex, "HEX").Label("Send as hex").
			OnChange(a.updateSnap)
		ui.Select(c, &a.suffix, suffixes).
			Label("Append").Width(96).
			OnChange(a.updateSnap)
		ui.Checkbox(c, &a.periodic, "Periodic").Label("Periodic send").
			OnChange(a.updateSnap)
		ui.NumberInput(c, &a.periodMs, 20, 3600000, 50).
			Label("Interval ms").Width(100).
			OnChange(a.updateSnap)
		ui.Text(c, "ms").TextColor(c.Theme().TextMuted)
		ui.Spacer(c).Grow(1)
		ui.PrimaryButton(c, "Send").MinWidth(96).
			Disabled(!a.opened).
			OnClick(a.sendNow)
	})
}

func main() {
	a := &app{
		baud:     "115200",
		dataBits: "8",
		parity:   "None",
		stopBits: "1",
		dtr:      true,
		rts:      true,
		suffix:   "None",
		periodMs: 1000,
		status:   "Select a port",
	}
	a.setPorts(listPortsQuick())
	go a.watchPorts()
	go a.periodicLoop()
	mygo.App.WhenReady(func() {
		a.win.Store(mygo.NewWindow(mygo.WindowOptions{
			Title:     "SerialComm",
			Width:     920,
			Height:    640,
			MinWidth:  720,
			MinHeight: 480,
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
				a.setStatus("Port removed", 2)
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
		if slices.Equal(names, last) {
			continue
		}
		last = names
		a.refreshPortsAsync()
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
		a.setStatus("Closed", 0)
		return
	}
	p := a.selectedPort()
	if p == nil {
		a.setStatus("No port selected", 2)
		return
	}
	mode, err := portMode(a.baud, a.dataBits, a.parity, a.stopBits)
	if err != nil {
		a.setStatus(err.Error(), 2)
		return
	}
	port, err := serial.Open(p.Name, mode)
	if err != nil {
		a.setStatus("Open failed: "+err.Error(), 2)
		return
	}
	a.portMu.Lock()
	a.port = port
	a.opened = true
	a.devName = p.Name
	a.portMu.Unlock()
	_ = port.SetDTR(a.dtr)
	_ = port.SetRTS(a.rts)
	a.setStatus(fmt.Sprintf("%s @ %d baud", p.Name, mode.BaudRate), 1)
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
		a.setStatus("Set mode: "+err.Error(), 2)
		return
	}
	a.setStatus(fmt.Sprintf("%d baud", mode.BaudRate), 1)
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
						a.nextRecID++
						a.rxRecords = append(a.rxRecords, rxRecord{
							id: a.nextRecID, ts: time.Now(), data: data,
						})
						if len(a.rxRecords) > maxRXRecords {
							a.rxRecords = a.rxRecords[len(a.rxRecords)-maxRXRecords:]
						}
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
					a.setStatus("Read error: "+err.Error(), 2)
				}
			})
			return
		}
	}
}

func (a *app) recordText(r rxRecord) string {
	var s strings.Builder
	if a.showTS {
		s.WriteString("[" + r.ts.Format("15:04:05.000") + "] ")
	}
	if a.dispMode == 1 {
		s.WriteString(hexDump(r.data, 16))
	} else {
		s.WriteString(printable(r.data))
	}
	return s.String()
}

func (a *app) saveLog() {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      a.win.Load(),
		Title:       "Save received data",
		DefaultPath: "serial-log.txt",
	})
	if err != nil || path == "" {
		return
	}
	var s strings.Builder
	for _, r := range a.rxRecords {
		s.WriteString(a.recordText(r))
		s.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(s.String()), 0o644); err != nil {
		a.setStatus("Save failed: "+err.Error(), 2)
		return
	}
	a.setStatus("Saved: "+path, 1)
}

// --- send ------------------------------------------------------------

func (a *app) updateSnap() {
	a.snapMu.Lock()
	defer a.snapMu.Unlock()
	a.snap = sendSnap{
		text:     a.sendText,
		hex:      a.sendHex,
		suffix:   a.suffix,
		periodic: a.periodic,
		interval: time.Duration(a.periodMs) * time.Millisecond,
	}
}

// payload builds the bytes to send from a sendSnap.
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
	case "\\r":
		data = append(data, '\r')
	case "\\n":
		data = append(data, '\n')
	case "\\r\\n":
		data = append(data, '\r', '\n')
	case "\\n\\r":
		data = append(data, '\n', '\r')
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("nothing to send")
	}
	return data, nil
}

func (a *app) sendNow() {
	a.updateSnap()
	data, err := payload(a.snap)
	if err != nil {
		a.setStatus("Send: "+err.Error(), 2)
		return
	}
	a.portMu.Lock()
	defer a.portMu.Unlock()
	if a.port == nil {
		a.setStatus("Port not open", 2)
		return
	}
	n, err := a.port.Write(data)
	if err != nil {
		a.setStatus("Send: "+err.Error(), 2)
		return
	}
	a.txCount += n
	a.setStatus(fmt.Sprintf("Sent %d bytes", n), 1)
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
		a.portMu.Lock()
		if a.port != nil {
			if n, err := a.port.Write(data); err == nil {
				w := a.win.Load()
				if w != nil {
					w.Update(func() { a.txCount += n })
				}
			}
		}
		a.portMu.Unlock()
	}
}

func (a *app) setStatus(s string, kind int) {
	a.status, a.statusKind = s, kind
}
