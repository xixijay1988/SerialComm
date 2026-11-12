package main

// crc16Modbus computes CRC-16/MODBUS: poly 0xA001, init 0xFFFF.
// Modbus transmits the CRC low byte first, so it returns [lo, hi].
func crc16Modbus(data []byte) []byte {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return []byte{byte(crc), byte(crc >> 8)}
}

// crc16CCITT computes CRC-16/CCITT-FALSE: poly 0x1021, init 0xFFFF,
// appended big-endian.
func crc16CCITT(data []byte) []byte {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return []byte{byte(crc >> 8), byte(crc)}
}

// sum8 returns the one's complement of the byte sum.
func sum8(data []byte) []byte {
	var s uint8
	for _, b := range data {
		s += b
	}
	return []byte{^s + 1}
}

// xor8 returns the XOR of all bytes.
func xor8(data []byte) []byte {
	var x uint8
	for _, b := range data {
		x ^= b
	}
	return []byte{x}
}

var checksums = []string{"None", "CRC16-Modbus", "CRC16-CCITT", "SUM8", "XOR8"}

func appendChecksum(data []byte, kind string) []byte {
	switch kind {
	case "CRC16-Modbus":
		return append(data, crc16Modbus(data)...)
	case "CRC16-CCITT":
		return append(data, crc16CCITT(data)...)
	case "SUM8":
		return append(data, sum8(data)...)
	case "XOR8":
		return append(data, xor8(data)...)
	}
	return data
}
