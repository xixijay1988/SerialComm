package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// newTestApp returns an app with the same defaults main() uses.
func newTestApp() *app {
	return &app{
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
}

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

func TestChecksums(t *testing.T) {
	// CRC16-Modbus of "123456789" is 0x4B37, sent low byte first.
	if got := crc16Modbus([]byte("123456789")); got[0] != 0x37 || got[1] != 0x4B {
		t.Errorf("crc16Modbus = % X", got)
	}
	// CRC16-CCITT-FALSE of "123456789" is 0x29B1.
	if got := crc16CCITT([]byte("123456789")); got[0] != 0x29 || got[1] != 0xB1 {
		t.Errorf("crc16CCITT = % X", got)
	}
	if got := sum8([]byte{0x10, 0x20, 0x30}); got[0] != 0xA0 {
		t.Errorf("sum8 = % X", got)
	}
	if got := xor8([]byte{0x10, 0x21, 0x31}); got[0] != 0x00 {
		t.Errorf("xor8 = % X", got)
	}
	if b := appendChecksum([]byte{1, 2}, "XOR8"); len(b) != 3 {
		t.Errorf("appendChecksum = %v", b)
	}
}

func TestPayload(t *testing.T) {
	b, err := payload(sendSnap{text: "hello", suffix: "CRLF (\\r\\n)"})
	if err != nil || string(b) != "hello\r\n" {
		t.Errorf("payload text = %v %v", b, err)
	}
	b, err = payload(sendSnap{text: "41 42", hex: true})
	if err != nil || string(b) != "AB" {
		t.Errorf("payload hex = %v %v", b, err)
	}
	b, err = payload(sendSnap{text: "AA", hex: true, checksum: "SUM8"})
	if err != nil || len(b) != 2 || b[0] != 0xAA || b[1] != 0x56 {
		t.Errorf("payload checksum = %v %v", b, err)
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
	a := newTestApp()
	a.rxRecords = []rxRecord{{id: 1, dir: dirRX, data: []byte("hello")}}
	tt := ui.NewTester(a.view, 1180, 760)
	for _, want := range []string{"SerialComm", "PRO", "串口监视器",
		"已断开", "hello", "多条发送", "快捷命令组", "连接设备"} {
		if !tt.HasText(want) {
			t.Errorf("expected %q, texts %q", want, tt.Texts())
		}
	}
}
