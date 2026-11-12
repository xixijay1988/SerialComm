package main

import "testing"

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
	// Two's complement: data+sum8 must add to zero.
	if got := sum8([]byte{0x10, 0x20, 0x30}); 0x10+0x20+0x30+int(got[0]) != 0x100 {
		t.Errorf("sum8 does not complement: % X", got)
	}
	if got := xor8([]byte{0x10, 0x21, 0x31}); got[0] != 0x00 {
		t.Errorf("xor8 = % X", got)
	}
	if b := appendChecksum([]byte{1, 2}, "XOR8"); len(b) != 3 {
		t.Errorf("appendChecksum = %v", b)
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

func TestPayload(t *testing.T) {
	b, err := payload(sendSnap{text: "hello", suffix: "CRLF (\\r\\n)"})
	if err != nil || string(b) != "hello\r\n" {
		t.Errorf("payload text = %v %v", b, err)
	}
	b, err = payload(sendSnap{text: "hi", suffix: "CR (\\r)"})
	if err != nil || string(b) != "hi\r" {
		t.Errorf("payload CR = %v %v", b, err)
	}
	b, err = payload(sendSnap{text: "hi", suffix: "LF (\\n)"})
	if err != nil || string(b) != "hi\n" {
		t.Errorf("payload LF = %v %v", b, err)
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

func TestPayloadSuffixBeforeChecksum(t *testing.T) {
	// A real Modbus write: 01 06 00 01 00 01 + CRC-16/MODBUS (0xCA19,
	// low byte first).
	b, err := payload(sendSnap{text: "01 06 00 01 00 01", hex: true,
		checksum: "CRC16-Modbus"})
	if err != nil {
		t.Fatal(err)
	}
	if got := b[6:]; got[0] != 0x19 || got[1] != 0xCA {
		t.Fatalf("payload crc = % X", got)
	}
	// A frame plus its CRC (low first) re-checksums to zero.
	if got := crc16Modbus(b); got[0] != 0 || got[1] != 0 {
		t.Fatalf("frame+crc recheck = % X, want 0", got)
	}
	// Suffix is inside the checksummed range.
	b, err = payload(sendSnap{text: "AA", hex: true,
		suffix: "CRLF (\\r\\n)", checksum: "XOR8"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 4 || b[3] != xor8([]byte{0xAA, '\r', '\n'})[0] {
		t.Fatalf("payload = % X", b)
	}
	// Bad hex must error, not send.
	if _, err := payload(sendSnap{text: "ZZ", hex: true}); err == nil {
		t.Error("bad hex should error")
	}
}
