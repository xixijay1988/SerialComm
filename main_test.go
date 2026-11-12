package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestParseHex(t *testing.T) {
	cases := []struct {
		in   string
		want []byte
		err  bool
	}{
		{"AA 55 03 FF", []byte{0xAA, 0x55, 0x03, 0xFF}, false},
		{"aa55", []byte{0xAA, 0x55}, false},
		{"0x01 0x02", []byte{0x01, 0x02}, false},
		{"AA,55;03-FF", []byte{0xAA, 0x55, 0x03, 0xFF}, false},
		{"", nil, true},
		{"A", nil, true},
		{"GG", nil, true},
		{"AA 5", nil, true},
	}
	for _, c := range cases {
		got, err := parseHex(c.in)
		if (err != nil) != c.err {
			t.Errorf("parseHex(%q) err=%v want err=%v", c.in, err, c.err)
			continue
		}
		if err == nil && string(got) != string(c.want) {
			t.Errorf("parseHex(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestHexDump(t *testing.T) {
	if got := hexDump([]byte{0xAA, 0xBB, 0x01}, 16); got != "AA BB 01" {
		t.Errorf("hexDump = %q", got)
	}
	if got := hexDump([]byte{1, 2, 3, 4, 5}, 2); got != "01 02\n03 04\n05" {
		t.Errorf("hexDump wrap = %q", got)
	}
}

func TestPayload(t *testing.T) {
	b, err := payload(sendSnap{text: "hello", suffix: "\\r\\n"})
	if err != nil || string(b) != "hello\r\n" {
		t.Errorf("payload text = %v %v", b, err)
	}
	b, err = payload(sendSnap{text: "41 42", hex: true})
	if err != nil || string(b) != "AB" {
		t.Errorf("payload hex = %v %v", b, err)
	}
	if _, err := payload(sendSnap{text: ""}); err == nil {
		t.Error("empty payload should error")
	}
}

func TestPortMode(t *testing.T) {
	m, err := portMode("115200", "8", "None", "1")
	if err != nil || m.BaudRate != 115200 || m.DataBits != 8 {
		t.Fatalf("portMode = %+v %v", m, err)
	}
	m, err = portMode("9600", "7", "Even", "2")
	if err != nil || m.BaudRate != 9600 || m.DataBits != 7 ||
		m.Parity != 2 /*EvenParity*/ || m.StopBits != 2 /*TwoStopBits*/ {
		t.Fatalf("portMode = %+v %v", m, err)
	}
	if _, err := portMode("abc", "8", "None", "1"); err == nil {
		t.Error("bad baud should error")
	}
}

func TestChipFromName(t *testing.T) {
	cases := map[string]string{
		"wchusbserial-1410": "WCH CH34x",
		"usbserial-FT1ABC":  "FTDI",
		"usbmodem1101":      "CDC-ACM",
		"SLAB_USBtoUART":    "CP210x",
		"PL2303G-USBtoUART": "PL2303",
	}
	for name, want := range cases {
		if got := chipFromName(name); got != want {
			t.Errorf("chipFromName(%q) = %q, want %q", name, got, want)
		}
	}
}

// The view runs without a window in tests.
func TestView(t *testing.T) {
	a := &app{
		baud: "115200", dataBits: "8", parity: "None",
		stopBits: "1", suffix: "None", periodMs: 1000,
	}
	a.rxRecords = []rxRecord{{id: 1, data: []byte("hello")}}
	tt := ui.NewTester(a.view, 920, 640)
	if !tt.HasText("Closed") {
		t.Errorf("expected Closed state, texts %q", tt.Texts())
	}
	if !tt.HasText("hello") {
		t.Errorf("expected rx data shown, texts %q", tt.Texts())
	}
}
