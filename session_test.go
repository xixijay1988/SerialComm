package main

import (
	"bytes"
	"math/rand"
	"strings"
	"testing"
	"time"

	serial "go.bug.st/serial"
)

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

// TestAppendRecordNoByteLoss is a property test: however the input
// stream is chopped into reads, no byte may ever be dropped, reordered
// or duplicated, and a record left without a line ending (mid-line)
// is only allowed to be the last one or a record full to the cap.
func TestAppendRecordNoByteLoss(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 200; trial++ {
		var stream []byte
		for i := 0; i < rng.Intn(3000); i++ {
			switch r := rng.Intn(100); {
			case r < 8:
				stream = append(stream, '\n')
			case r < 13:
				stream = append(stream, '\r')
			default:
				stream = append(stream, byte('a'+rng.Intn(26)))
			}
		}
		a := newTestApp()
		s := a.activeSession()
		var in []byte
		for len(stream) > 0 {
			n := 1 + rng.Intn(64)
			if n > len(stream) {
				n = len(stream)
			}
			in = append(in, stream[:n]...)
			s.appendRecord(dirRX, stream[:n])
			stream = stream[n:]
		}
		var out []byte
		for i, r := range s.rxRecords {
			if r.dir != dirRX {
				t.Fatalf("trial %d record %d dir=%d", trial, i, r.dir)
			}
			last := i == len(s.rxRecords)-1
			open := len(r.data) == 0 || r.data[len(r.data)-1] != '\n'
			if open && !last && len(r.data) != maxRecordBytes {
				t.Fatalf("trial %d record %d open mid-sequence: %q",
					trial, i, r.data)
			}
			out = append(out, r.data...)
		}
		if !bytes.Equal(in, out) {
			t.Fatalf("trial %d byte loss: in=%dB out=%dB", trial, len(in), len(out))
		}
	}
}

func TestRecordText(t *testing.T) {
	a := newTestApp()
	s := a.activeSession()
	s.appendRecord(dirRX, []byte("line\r\n"))
	s.appendRecord(dirRX, []byte("\n"))
	s.appendRecord(dirRX, []byte("next\r\n"))
	if len(s.rxRecords) != 3 {
		t.Fatalf("records = %d, want 3", len(s.rxRecords))
	}
	if got := s.recordText(s.rxRecords[0]); got != "line" {
		t.Fatalf("row text = %q, want %q", got, "line")
	}
	if got := s.recordText(s.rxRecords[1]); got != "" {
		t.Fatalf("blank row text = %q, want empty", got)
	}
	if got := s.recordText(s.rxRecords[2]); got != "next" {
		t.Fatalf("row text = %q, want %q", got, "next")
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
	a.customCmds[a.cmdGroup] = []quickCmd{{Title: "mine", Value: "mycmd"}}
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

// fakePort is a serial.Port that captures writes for assertions.
type fakePort struct {
	written [][]byte
}

func (f *fakePort) SetMode(*serial.Mode) error { return nil }
func (f *fakePort) Read([]byte) (int, error)   { return 0, nil }
func (f *fakePort) Drain() error               { return nil }
func (f *fakePort) ResetInputBuffer() error    { return nil }
func (f *fakePort) ResetOutputBuffer() error   { return nil }
func (f *fakePort) SetDTR(bool) error          { return nil }
func (f *fakePort) SetRTS(bool) error          { return nil }
func (f *fakePort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}
func (f *fakePort) SetReadTimeout(time.Duration) error { return nil }
func (f *fakePort) Close() error                       { return nil }
func (f *fakePort) Break(time.Duration) error          { return nil }
func (f *fakePort) Write(p []byte) (int, error) {
	f.written = append(f.written, append([]byte{}, p...))
	return len(p), nil
}

func TestSendCommandOwnConfig(t *testing.T) {
	a := newTestApp()
	s := a.activeSession()
	fp := &fakePort{}
	s.port = fp
	s.opened = true

	// A Modbus-style command sends hex + CRC regardless of the send
	// area's text-mode settings.
	s.sendHex = false
	s.suffix = "CRLF (\\r\\n)"
	s.sendCommand(quickCmd{Title: "read", Value: "01 03 00 00 00 02",
		Hex: true, Checksum: "CRC16-Modbus"})
	if len(fp.written) != 1 {
		t.Fatalf("writes = %d, want 1", len(fp.written))
	}
	got := fp.written[0]
	want := appendChecksum([]byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x02},
		"CRC16-Modbus")
	if !bytes.Equal(got, want) {
		t.Fatalf("sent % X, want % X", got, want)
	}
	if s.txCount != len(want) {
		t.Fatalf("txCount = %d", s.txCount)
	}
	// The TX echo shows the bytes sent.
	if len(s.rxRecords) != 1 || s.rxRecords[0].dir != dirTX ||
		!bytes.Equal(s.rxRecords[0].data, want) {
		t.Fatalf("records = %#v", s.rxRecords)
	}

	// A text command with its own CRLF suffix.
	s.sendCommand(quickCmd{Title: "at", Value: "AT", Suffix: "CRLF (\\r\\n)"})
	if len(fp.written) != 2 || string(fp.written[1]) != "AT\r\n" {
		t.Fatalf("second write = % X", fp.written)
	}

	// A command whose hex content is invalid fails before writing.
	s.sendCommand(quickCmd{Title: "bad", Value: "ZZ", Hex: true})
	if len(fp.written) != 2 {
		t.Fatalf("bad hex wrote %d frames", len(fp.written))
	}
}

func TestQuickCmdBadges(t *testing.T) {
	if got := (quickCmd{Value: "AT", Suffix: "CRLF (\\r\\n)"}).badges(); got != "CRLF" {
		t.Fatalf("badges = %q", got)
	}
	if got := (quickCmd{Hex: true, Checksum: "CRC16-Modbus"}).badges(); got != "HEX · CRC16-Modbus" {
		t.Fatalf("badges = %q", got)
	}
	if got := (quickCmd{}).badges(); got != "" {
		t.Fatalf("empty badges = %q", got)
	}
}
