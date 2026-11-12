package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// newTestApp returns an app with the same defaults main() uses.
func newTestApp() *app {
	a := &app{
		showSide:   true,
		showPanel:  true,
		dark:       true,
		cmdGroup:   cmdGroupNames[0],
		customCmds: map[string][]quickCmd{},
	}
	a.newSession()
	return a
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
	s := a.activeSession()
	s.rxRecords = []rxRecord{{id: 1, dir: dirRX, data: []byte("hello")}}
	tt := ui.NewTester(a.view, 1280, 800)
	for _, want := range []string{"SerialComm", "PRO", "工作区", "活动会话",
		"会话 1", "当前设备", "串口监视器", "已断开", "hello",
		"多条发送", "快捷命令组", "连接设备", "◐ 跟随系统"} {
		if !tt.HasText(want) {
			t.Errorf("expected %q, texts %q", want, tt.Texts())
		}
	}
}

func TestSessions(t *testing.T) {
	a := newTestApp()
	s2 := a.newSession()
	if len(a.sessions) != 2 || a.activeSession() != s2 {
		t.Fatalf("newSession: sessions=%d active=%d", len(a.sessions), a.active)
	}
	a.active = 0
	a.closeSession(0)
	if len(a.sessions) != 1 || a.activeSession() != s2 {
		t.Fatalf("closeSession: sessions=%d active=%d", len(a.sessions), a.active)
	}
	// Cannot close the last session.
	a.closeSession(0)
	if len(a.sessions) != 1 {
		t.Fatal("last session was closed")
	}
}

func TestAppendRecordMergesLineAcrossReads(t *testing.T) {
	a := newTestApp()
	s := a.activeSession()
	// The device's \r\n line ending lands split across two reads:
	// "abc\r" then "\ndef\r\n". The merged record must not leave an
	// empty row behind.
	s.appendRecord(dirRX, []byte("abc\r"))
	s.appendRecord(dirRX, []byte("\ndef\r\n"))
	if len(s.rxRecords) != 2 {
		t.Fatalf("records = %d, want 2", len(s.rxRecords))
	}
	if got := string(s.rxRecords[0].data); got != "abc\r\n" {
		t.Fatalf("record 0 = %q", got)
	}
	if got := string(s.rxRecords[1].data); got != "def\r\n" {
		t.Fatalf("record 1 = %q", got)
	}
	// A following chunk without newline keeps appending to the open line.
	s.appendRecord(dirRX, []byte("ghi"))
	s.appendRecord(dirRX, []byte("jkl\r\n"))
	if len(s.rxRecords) != 3 || string(s.rxRecords[2].data) != "ghijkl\r\n" {
		t.Fatalf("records = %#v", s.rxRecords)
	}
	// A terminator after a completed line is a real empty line: it
	// keeps its own record so intentional blank lines are not lost.
	s.appendRecord(dirRX, []byte("uvw\r\n"))
	s.appendRecord(dirRX, []byte("\n"))
	s.appendRecord(dirRX, []byte("xyz\r\n"))
	if len(s.rxRecords) != 6 {
		t.Fatalf("records = %d, want 6", len(s.rxRecords))
	}
	if got := string(s.rxRecords[4].data); got != "\n" {
		t.Fatalf("record 4 = %q, want empty line", got)
	}
	if got := string(s.rxRecords[5].data); got != "xyz\r\n" {
		t.Fatalf("record 5 = %q", got)
	}
	// TX records never merge: two sends stay two rows.
	s.appendRecord(dirTX, []byte("AT"))
	s.appendRecord(dirTX, []byte("OK"))
	if len(s.rxRecords) != 8 {
		t.Fatalf("records = %d, want 8", len(s.rxRecords))
	}
}

func TestParseInterval(t *testing.T) {
	cases := map[string]int64{
		"500 ms": 500,
		"1000":   1000,
		"250 毫秒": 250,
		"":       0, // empty must not panic
		"abc ms": 0,
	}
	for in, want := range cases {
		if got := parseInterval(in).Milliseconds(); got != want {
			t.Errorf("parseInterval(%q) = %d ms, want %d", in, got, want)
		}
	}
}

func TestPrintable(t *testing.T) {
	if got := printable([]byte("hello\t世界")); got != "hello\t世界" {
		t.Errorf("printable utf8 = %q", got)
	}
	// Invalid UTF-8 and control bytes become '.', \r\n survive.
	if got := printable([]byte{0x00, 0xFF, 0xFE, 'a', '\r', '\n'}); got != "...a\r\n" {
		t.Errorf("printable binary = %q", got)
	}
}

func TestFilterRecords(t *testing.T) {
	a := newTestApp()
	s := a.activeSession()
	s.rxRecords = []rxRecord{
		{id: 1, dir: dirRX, data: []byte("hello\n")},
		{id: 2, dir: dirTX, data: []byte("AT")},
		{id: 3, dir: dirSYS, data: []byte("opened")},
		{id: 4, dir: dirRX, data: []byte("world\n")},
	}
	s.showTX = false
	s.filterRecords()
	if len(s.filtered) != 3 {
		t.Fatalf("filtered = %d, want 3 (TX hidden)", len(s.filtered))
	}
	s.query = "HELLO"
	s.filterRecords()
	if len(s.filtered) != 1 || s.filtered[0].id != 1 {
		t.Fatalf("search filtered = %#v", s.filtered)
	}
	// Search matches HEX text too in HEX mode.
	s.dispMode = 1
	s.query = "68 65" // "he"
	s.filterRecords()
	if len(s.filtered) != 1 || s.filtered[0].id != 1 {
		t.Fatalf("hex search filtered = %#v", s.filtered)
	}
}

func TestRecordTextModes(t *testing.T) {
	a := newTestApp()
	s := a.activeSession()
	r := rxRecord{id: 1, dir: dirRX, data: []byte("AB\r\n")}
	s.dispMode = 1
	if got := s.recordText(r); got != "41 42 0D 0A" {
		t.Fatalf("hex mode = %q", got)
	}
	s.dispMode = 2
	got := s.recordText(r)
	if !strings.HasPrefix(got, "AB") || !strings.Contains(got, "41 42") {
		t.Fatalf("mixed mode = %q", got)
	}
	// SYS records ignore display mode.
	s.dispMode = 1
	r.dir = dirSYS
	if got := s.recordText(r); got != "AB" {
		t.Fatalf("sys text = %q", got)
	}
}

func TestPayloadSuffixBeforeChecksum(t *testing.T) {
	// Checksum covers data + line ending, per Modbus convention.
	b, err := payload(sendSnap{text: "01 06 00 01 00 01", hex: true,
		checksum: "CRC16-Modbus"})
	if err != nil {
		t.Fatal(err)
	}
	want := appendChecksum(appendChecksum(
		[]byte{0x01, 0x06, 0x00, 0x01, 0x00, 0x01}, "None"), "CRC16-Modbus")
	if string(b) != string(want) {
		t.Fatalf("payload = % X, want % X", b, want)
	}
	// Suffix applied before checksum.
	b, err = payload(sendSnap{text: "AA", hex: true,
		suffix: "CRLF (\\r\\n)", checksum: "XOR8"})
	if err != nil {
		t.Fatal(err)
	}
	want2 := xor8([]byte{0xAA, '\r', '\n'})
	if len(b) != 4 || b[3] != want2[0] {
		t.Fatalf("payload = % X", b)
	}
	// Bad hex must error, not send.
	if _, err := payload(sendSnap{text: "ZZ", hex: true}); err == nil {
		t.Error("bad hex should error")
	}
}

func TestAppendChecksumUnknown(t *testing.T) {
	in := []byte{1, 2, 3}
	if got := appendChecksum(in, "None"); string(got) != string(in) {
		t.Fatalf("None = %v", got)
	}
	if got := appendChecksum(in, "bogus"); string(got) != string(in) {
		t.Fatalf("unknown kind = %v", got)
	}
}

func TestAppendRecordCaps(t *testing.T) {
	a := newTestApp()
	s := a.activeSession()
	// A chunk larger than maxRecordBytes splits into two records, the
	// first capped and open, the second taking the rest with its newline.
	big := append(bytes.Repeat([]byte{'x'}, maxRecordBytes+10), '\n')
	s.appendRecord(dirRX, big)
	if len(s.rxRecords) != 2 {
		t.Fatalf("records = %d, want 2", len(s.rxRecords))
	}
	if len(s.rxRecords[0].data) != maxRecordBytes {
		t.Fatalf("record 0 len = %d, want %d", len(s.rxRecords[0].data), maxRecordBytes)
	}
	if got := string(s.rxRecords[1].data); got != "xxxxxxxxxx\n" {
		t.Fatalf("record 1 = %q", got)
	}
	// LF-only input splits per line.
	s.rxRecords = nil
	s.appendRecord(dirRX, []byte("a\nb\nc"))
	if len(s.rxRecords) != 3 {
		t.Fatalf("records = %d, want 3", len(s.rxRecords))
	}
	// Records beyond maxRXRecords are evicted from the front.
	s.rxRecords = nil
	base := s.nextRecID
	for i := 0; i < maxRXRecords+10; i++ {
		s.appendRecord(dirRX, []byte("x\n"))
	}
	if len(s.rxRecords) != maxRXRecords {
		t.Fatalf("records = %d, want capped at %d", len(s.rxRecords), maxRXRecords)
	}
	if s.rxRecords[0].id != base+11 {
		t.Fatalf("oldest kept id = %d, want %d", s.rxRecords[0].id, base+11)
	}
}

func TestPortInUse(t *testing.T) {
	a := newTestApp()
	s1 := a.activeSession()
	s1.devName = "/dev/cu.usbserial-1"
	s1.opened = true
	s2 := a.newSession()
	if got := a.portInUse("/dev/cu.usbserial-1", s2); got != s1 {
		t.Fatal("portInUse did not find s1")
	}
	// A session does not conflict with itself; closed ports don't count.
	if a.portInUse("/dev/cu.usbserial-1", s1) != nil {
		t.Fatal("self-conflict reported")
	}
	s1.opened = false
	if a.portInUse("/dev/cu.usbserial-1", s2) != nil {
		t.Fatal("closed port reported as in use")
	}
}

func TestGroupCommands(t *testing.T) {
	a := newTestApp()
	a.customCmds[a.cmdGroup] = []quickCmd{{"mine", "mycmd"}}
	got := a.groupCommands()
	if len(got) != len(cmdGroups[a.cmdGroup])+1 || got[len(got)-1].Value != "mycmd" {
		t.Fatalf("groupCommands = %#v", got)
	}
}

func TestSendNowNoPort(t *testing.T) {
	a := newTestApp()
	s := a.activeSession()
	s.sendText = "AT"
	s.sendNow()
	if s.txCount != 0 || len(s.rxRecords) != 0 {
		t.Fatalf("sendNow without port: tx=%d records=%d", s.txCount, len(s.rxRecords))
	}
	if s.status != "发送失败: 串口未打开" {
		t.Fatalf("status = %q", s.status)
	}
}
