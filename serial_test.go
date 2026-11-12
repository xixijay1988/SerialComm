package main

import (
	"testing"

	serial "go.bug.st/serial"
)

func TestChipFromName(t *testing.T) {
	cases := map[string]string{
		"wchusbserial-1410":     "WCH CH34x",
		"wchusbserial-2130":     "WCH CH34x",
		"usbserial-FT1ABC":      "FTDI",
		"usb_uart-0001":         "FTDI",
		"usbmodem1101":          "CDC-ACM",
		"SLAB_USBtoUART":        "CP210x",
		"PL2303G-USBtoUART":     "PL2303",
		"Bluetooth-Incoming-Port": "system",
		"debug-console":           "system",
		"something-unknown":       "",
	}
	for name, want := range cases {
		if got := chipFromName(name); got != want {
			t.Errorf("chipFromName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestPortLabel(t *testing.T) {
	p := portInfo{Base: "usbserial-1", Chip: "FTDI", VID: "0403", PID: "6001"}
	if got := p.label(); got != "usbserial-1  [FTDI] 0403:6001" {
		t.Fatalf("label = %q", got)
	}
	p = portInfo{Base: "usbmodem1101"}
	if got := p.label(); got != "usbmodem1101" {
		t.Fatalf("bare label = %q", got)
	}
	p = portInfo{Base: "x", Chip: "CDC-ACM"}
	if got := p.label(); got != "x  [CDC-ACM]" {
		t.Fatalf("chip-only label = %q", got)
	}
}

// The ioreg regexes must extract decimal VID/PID and the product string
// from real-looking ioreg -l output.
func TestIORegRegexes(t *testing.T) {
	out := `+-o IOSerialBSDClient  <class ...>
  |     "idVendor" = 6790
  |     "idProduct" = 29987
  |     "USB Product Name" = "USB Single Serial"
`
	v := reVID.FindStringSubmatch(out)
	if len(v) != 2 || v[1] != "6790" {
		t.Fatalf("reVID = %v, want 6790 (0x1A86 WCH)", v)
	}
	if p := rePID.FindStringSubmatch(out); len(p) != 2 || p[1] != "29987" {
		t.Fatalf("rePID = %v, want 29987 (0x7523 CH340)", p)
	}
	if n := reProduct.FindStringSubmatch(out); len(n) != 2 || n[1] != "USB Single Serial" {
		t.Fatalf("reProduct = %v", n)
	}
	if v, ok := usbVendors["1A86"]; !ok || v != "WCH" {
		t.Fatalf("usbVendors[1A86] = %q %v", v, ok)
	}
}

func TestPortMode(t *testing.T) {
	m, err := portMode("115200", "8", "None", "1")
	if err != nil || m.BaudRate != 115200 || m.DataBits != 8 ||
		m.Parity != serial.NoParity || m.StopBits != serial.OneStopBit {
		t.Fatalf("portMode = %+v %v", m, err)
	}
	m, err = portMode("9600", "7", "Even", "2")
	if err != nil || m.BaudRate != 9600 || m.DataBits != 7 ||
		m.Parity != serial.EvenParity || m.StopBits != serial.TwoStopBits {
		t.Fatalf("portMode = %+v %v", m, err)
	}
	m, err = portMode("4800", "5", "Odd", "1.5")
	if err != nil || m.DataBits != 5 || m.Parity != serial.OddParity ||
		m.StopBits != serial.OnePointFiveStopBits {
		t.Fatalf("portMode = %+v %v", m, err)
	}
	// Unknown values fall back to the common defaults, not errors.
	m, err = portMode("115200", "9", "wat", "3")
	if err != nil || m.DataBits != 8 || m.Parity != serial.NoParity ||
		m.StopBits != serial.OneStopBit {
		t.Fatalf("fallback portMode = %+v %v", m, err)
	}
	for _, bad := range []string{"abc", "0", "-1"} {
		if _, err := portMode(bad, "8", "None", "1"); err == nil {
			t.Errorf("baud %q should error", bad)
		}
	}
}

func TestPortNameListed(t *testing.T) {
	ports := []portInfo{{Name: "/dev/cu.a"}, {Name: "/dev/cu.b"}}
	if !portNameListed(ports, "/dev/cu.b") {
		t.Error("expected /dev/cu.b listed")
	}
	if portNameListed(ports, "/dev/cu.c") {
		t.Error("unexpected /dev/cu.c listed")
	}
}
