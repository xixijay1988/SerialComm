package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// hexDump renders b as space separated hex bytes, perLine bytes per line.
func hexDump(b []byte, perLine int) string {
	if len(b) == 0 {
		return ""
	}
	var s strings.Builder
	for i, c := range b {
		if i > 0 {
			if perLine > 0 && i%perLine == 0 {
				s.WriteByte('\n')
			} else {
				s.WriteByte(' ')
			}
		}
		fmt.Fprintf(&s, "%02X", c)
	}
	return s.String()
}

// parseHex parses a hex string like "AA 55 0x03 FF" into bytes. Whitespace,
// commas and 0x prefixes are ignored; anything else or a nibble count that
// is not even is an error.
func parseHex(s string) ([]byte, error) {
	var digits []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '0' && i+1 < len(s) && (s[i+1] == 'x' || s[i+1] == 'X'):
			i++
		case isHex(c):
			digits = append(digits, c)
		case c == ' ' || c == ',' || c == '\t' || c == '\n' || c == '\r' || c == '-' || c == ';':
		default:
			return nil, fmt.Errorf("invalid character %q", string(c))
		}
	}
	if len(digits) == 0 {
		return nil, fmt.Errorf("empty hex string")
	}
	if len(digits)%2 != 0 {
		return nil, fmt.Errorf("odd number of hex digits")
	}
	out := make([]byte, len(digits)/2)
	for i := 0; i < len(digits); i += 2 {
		out[i/2] = hexVal(digits[i])<<4 | hexVal(digits[i+1])
	}
	return out, nil
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func hexVal(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

// printable turns raw bytes into displayable text: printable ASCII and
// \r \n \t pass through, valid UTF-8 sequences (e.g. Chinese text) are
// kept, other bytes become '.'.
func printable(b []byte) string {
	var s strings.Builder
	s.Grow(len(b))
	for i := 0; i < len(b); {
		c := b[i]
		if c >= 0x20 && c < 0x7F || c == '\r' || c == '\n' || c == '\t' {
			s.WriteByte(c)
			i++
			continue
		}
		if c >= 0x80 {
			if r, size := utf8.DecodeRune(b[i:]); r != utf8.RuneError {
				s.Write(b[i : i+size])
				i += size
				continue
			}
		}
		s.WriteByte('.')
		i++
	}
	return s.String()
}
