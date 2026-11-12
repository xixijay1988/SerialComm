package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	serial "go.bug.st/serial"
)

// portInfo is one enumerated serial port.
type portInfo struct {
	Name    string // e.g. /dev/cu.usbserial-FTXXXX
	Base    string // device name without /dev/cu. prefix
	Chip    string // chipset label: WCH CH34x, FTDI, CP210x, ...
	VID     string // USB vendor id, hex, when known
	PID     string // USB product id, hex, when known
	Product string // USB product string, when known
}

func (p portInfo) label() string {
	label := p.Base
	if p.Chip != "" {
		label += "  [" + p.Chip + "]"
	}
	if p.VID != "" {
		label += fmt.Sprintf(" %s:%s", p.VID, p.PID)
	}
	return label
}

// usbVendors maps well known USB vendor IDs of serial chips.
var usbVendors = map[string]string{
	"1A86": "WCH",         // CH340/341/342/343/344/347
	"0403": "FTDI",        // FT232/FT2232/FT4232...
	"10C4": "CP210x",      // Silicon Labs
	"067B": "PL2303",      // Prolific
	"2341": "Arduino",     // Arduino
	"303A": "Espressif",   // ESP32 built-in USB/JTAG
	"2E8A": "RP2xxx",      // Raspberry Pi Pico
	"0483": "STMicro",     // STM32 CDC
	"16C0": "VOTI/Teensy", // Van Ooijen / Teensy
	"1546": "u-blox",      // u-blox GNSS
	"0D28": "mbed",        // ARM mbed
}

// chipFromName guesses the driver/chip from the device base name.
func chipFromName(base string) string {
	b := strings.ToLower(base)
	switch {
	case strings.Contains(b, "wchusbserial"):
		return "WCH CH34x" // CH340/341/342/343/344/347 all use this driver name
	case strings.Contains(b, "usbserial"), strings.Contains(b, "usb_uart"):
		return "FTDI"
	case strings.Contains(b, "usbmodem"):
		return "CDC-ACM"
	case strings.Contains(b, "slab_uart"), strings.Contains(b, "slab_usbtouart"):
		return "CP210x"
	case strings.Contains(b, "pl2303"):
		return "PL2303"
	case strings.Contains(b, "bluetooth"), strings.Contains(b, "debug"):
		return "system"
	}
	return ""
}

var (
	reVID     = regexp.MustCompile(`"idVendor"\s*=\s*(\d+)`)
	rePID     = regexp.MustCompile(`"idProduct"\s*=\s*(\d+)`)
	reProduct = regexp.MustCompile(`"USB Product Name"\s*=\s*"([^"]*)"`)
)

// enrichIOReg reads the node's ancestor chain in ioreg to fill VID/PID and
// the product string. macOS only; silently leaves fields empty on failure.
// It forks ioreg and is SLOW (~hundreds of ms): never call it on the main
// thread.
func enrichIOReg(p *portInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ioreg", "-l", "-w0", "-n", p.Base).Output()
	if err != nil || len(out) == 0 {
		return
	}
	if m := reVID.FindSubmatch(out); len(m) == 2 {
		var v uint64
		fmt.Sscanf(string(m[1]), "%d", &v)
		p.VID = fmt.Sprintf("%04X", v)
	}
	if m := rePID.FindSubmatch(out); len(m) == 2 {
		var v uint64
		fmt.Sscanf(string(m[1]), "%d", &v)
		p.PID = fmt.Sprintf("%04X", v)
	}
	if m := reProduct.FindSubmatch(out); len(m) == 2 {
		p.Product = string(m[1])
	}
	if p.Chip == "" && p.VID != "" {
		if v, ok := usbVendors[p.VID]; ok {
			p.Chip = v
		}
	}
}

// portNames lists callout (/dev/cu.*) device paths. Cheap: a directory
// read, no subprocesses — safe to poll and to run on the main thread.
func portNames() []string {
	names, err := serial.GetPortsList()
	if err != nil {
		return nil
	}
	out := names[:0]
	for _, n := range names {
		// use the callout device, not the dial-in tty.*
		if strings.HasPrefix(n, "/dev/cu.") {
			out = append(out, n)
		}
	}
	return out
}

// listPortsQuick enumerates ports with name-based chip detection only —
// no ioreg, so it never blocks. Safe on the main thread.
func listPortsQuick() []portInfo {
	var ports []portInfo
	for _, n := range portNames() {
		p := portInfo{Name: n, Base: strings.TrimPrefix(n, "/dev/cu.")}
		p.Chip = chipFromName(p.Base)
		ports = append(ports, p)
	}
	return ports
}

// listPorts enumerates callout (/dev/cu.*) serial devices and enriches USB
// ones through ioreg. SLOW (a fork+exec per port): call it off the main
// thread only.
func listPorts() []portInfo {
	ports := listPortsQuick()
	for i := range ports {
		enrichIOReg(&ports[i])
	}
	return ports
}

// portMode parses the UI configuration into a serial.Mode.
func portMode(baud string, dataBits string, parity string, stopBits string) (*serial.Mode, error) {
	m := &serial.Mode{}
	if _, err := fmt.Sscanf(baud, "%d", &m.BaudRate); err != nil || m.BaudRate <= 0 {
		return nil, fmt.Errorf("invalid baud rate %q", baud)
	}
	switch dataBits {
	case "5":
		m.DataBits = 5
	case "6":
		m.DataBits = 6
	case "7":
		m.DataBits = 7
	default:
		m.DataBits = 8
	}
	switch parity {
	case "Odd":
		m.Parity = serial.OddParity
	case "Even":
		m.Parity = serial.EvenParity
	case "Mark":
		m.Parity = serial.MarkParity
	case "Space":
		m.Parity = serial.SpaceParity
	default:
		m.Parity = serial.NoParity
	}
	switch stopBits {
	case "1.5":
		m.StopBits = serial.OnePointFiveStopBits
	case "2":
		m.StopBits = serial.TwoStopBits
	default:
		m.StopBits = serial.OneStopBit
	}
	return m, nil
}
