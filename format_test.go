package main

import (
	"fmt"
	"strings"
	"testing"
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
		{"0XAB 0xcd", []byte{0xAB, 0xCD}, false},
		{"AA,55;03-FF", []byte{0xAA, 0x55, 0x03, 0xFF}, false},
		{"\tAA\n55\r ", []byte{0xAA, 0x55}, false},
		{"DEADBEEF", []byte{0xDE, 0xAD, 0xBE, 0xEF}, false},
		{"", nil, true},
		{"-;, \t", nil, true}, // separators only is still empty
		{"A", nil, true},
		{"0x", nil, true},
		{"GG", nil, true},
		{"AA 5", nil, true},
		{"你好", nil, true},
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

// FuzzParseHex feeds arbitrary input; success must round-trip: encoding
// the result as hex and parsing again yields the same bytes.
func FuzzParseHex(f *testing.F) {
	for _, s := range []string{"AA 55", "0xDE,0xAD", "aa-bb;cc", "A", "", "zz"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got, err := parseHex(in)
		if err != nil {
			return // rejected input is fine, crashing is not
		}
		var enc strings.Builder
		for i, b := range got {
			if i > 0 {
				enc.WriteByte(' ')
			}
			fmt.Fprintf(&enc, "%02X", b)
		}
		again, err := parseHex(enc.String())
		if err != nil || string(again) != string(got) {
			t.Fatalf("parseHex(%q) = %v; reparse %q = %v, %v",
				in, got, enc.String(), again, err)
		}
	})
}

func TestHexDump(t *testing.T) {
	if got := hexDump(nil, 16); got != "" {
		t.Errorf("hexDump(nil) = %q", got)
	}
	if got := hexDump([]byte{0xAA, 0xBB, 0x01}, 16); got != "AA BB 01" {
		t.Errorf("hexDump = %q", got)
	}
	if got := hexDump([]byte{1, 2, 3, 4, 5}, 2); got != "01 02\n03 04\n05" {
		t.Errorf("hexDump wrap = %q", got)
	}
	// perLine of 0 means never wrap.
	if got := hexDump([]byte{1, 2, 3}, 0); got != "01 02 03" {
		t.Errorf("hexDump nowrap = %q", got)
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
	// A truncated UTF-8 sequence at the end degrades byte by byte.
	if got := printable([]byte{'a', 0xE4, 0xB8}); got != "a.." {
		t.Errorf("printable truncated utf8 = %q", got)
	}
}
