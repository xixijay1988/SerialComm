package main

import "testing"

func TestRecordTextStripsTerminator(t *testing.T) {
	a := newTestApp()
	s := a.activeSession()
	s.appendRecord(dirRX, []byte("line\r\n"))
	s.appendRecord(dirRX, []byte("\n"))
	s.appendRecord(dirRX, []byte("next\r\n"))
	if len(s.rxRecords) != 2 {
		t.Fatalf("records = %d, want 2", len(s.rxRecords))
	}
	if got := s.recordText(s.rxRecords[0]); got != "line" {
		t.Fatalf("row text = %q, want %q", got, "line")
	}
	if got := s.recordText(s.rxRecords[1]); got != "next" {
		t.Fatalf("row text = %q, want %q", got, "next")
	}
}
