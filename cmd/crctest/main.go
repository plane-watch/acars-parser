// Package main tests various CRC-16 algorithms against known ACARS messages.
package main

import (
	"fmt"
	"strings"
)

// Test messages with their expected checksums (last 4 hex chars).
var testCases = []struct {
	full     string // Full message including checksum
	checksum string // Expected checksum (last 4 hex chars)
}{
	// Messages with /WD section - checksum after ,,,,
	{"FPN/ID23565S,WIDE12,ZPZWTCP12004/MR2,3/RP:DA:KMCF:AA:KTIK:F:CUSEK.T349.KNRAD..N25400W080030..N26140W080140..N25450W080230..FEMID.Q102.CIGAR.Q102.BACCA.Q102.BLVNS.Q105.HRV.J58.AEX..WOLUR:V:KNRAD,351,AT2200,,:V:N26140W080140,277,AT2200,,:V:N25450W080230,272,AT3600,,:V:CIGAR,269,AT3600,,:V:BACCA,271,AT3600,,:V:HRV,282,AT3400,,49BE/WD,,,,75A7", "75A7"},
	{"FPN/ID23565S,WIDE12,ZPZWTCP12004/MR2,5/RP:DA:KMCF:AA:KTIK:F:CUSEK.T349.KNRAD..N25400W080030..N26140W080140..N25450W080230..FEMID.Q102.CIGAR.Q102.BACCA.Q102.BLVNS.Q105.HRV.J58.AEX..WOLUR:V:KNRAD,351,AT2200,,:V:N26140W080140,277,AT2200,,:V:N25450W080230,272,AT3600,,:V:CIGAR,269,AT3600,,:V:BACCA,271,AT3600,,:V:HRV,282,AT3400,,49BE/WD,,,,A7A7", "A7A7"},
	// Different /WD messages.
	{"FPN/ID38883S,ROMA94,8VH072E14004/MR1,2/RP:DA:KWRI:AA:KSKA:R:06O:F:MXE..PENSY.J110.LARRI.Q430.BEETS.J110.GRAHM..MOAWK..MUSIT..NCOLY..BYPOR..KP18E..KP18Y..KU18S..KU15M..MLP:A:HILIE3.MLP(23O):V:PENSY,246,AT4000,,315D/WD,,,,B27B", "B27B"},
	{"FPN/ID00339S,RCH12,8VH067E12004/MR1,2/RP:DA:KWRI:AA:KSKA:F:FJC..SFK..DMACK..RUBKI..JUVAG..DLH..N47000W094000..N47300W100000..N48000W106000..CHOTE..MLP:V:DMACK,302,AT3000,,:V:N47300W100000,246,AT4000,,5FD6/WD,,,,0AE8", "0AE8"},
}

// CRC-16-ARINC from libacars - poly 0x1021, MSB-first (non-reflected).
var crc16ArincTable = [256]uint16{
	0x0000, 0x1021, 0x2042, 0x3063, 0x4084, 0x50A5, 0x60C6, 0x70E7,
	0x8108, 0x9129, 0xA14A, 0xB16B, 0xC18C, 0xD1AD, 0xE1CE, 0xF1EF,
	0x1231, 0x0210, 0x3273, 0x2252, 0x52B5, 0x4294, 0x72F7, 0x62D6,
	0x9339, 0x8318, 0xB37B, 0xA35A, 0xD3BD, 0xC39C, 0xF3FF, 0xE3DE,
	0x2462, 0x3443, 0x0420, 0x1401, 0x64E6, 0x74C7, 0x44A4, 0x5485,
	0xA56A, 0xB54B, 0x8528, 0x9509, 0xE5EE, 0xF5CF, 0xC5AC, 0xD58D,
	0x3653, 0x2672, 0x1611, 0x0630, 0x76D7, 0x66F6, 0x5695, 0x46B4,
	0xB75B, 0xA77A, 0x9719, 0x8738, 0xF7DF, 0xE7FE, 0xD79D, 0xC7BC,
	0x48C4, 0x58E5, 0x6886, 0x78A7, 0x0840, 0x1861, 0x2802, 0x3823,
	0xC9CC, 0xD9ED, 0xE98E, 0xF9AF, 0x8948, 0x9969, 0xA90A, 0xB92B,
	0x5AF5, 0x4AD4, 0x7AB7, 0x6A96, 0x1A71, 0x0A50, 0x3A33, 0x2A12,
	0xDBFD, 0xCBDC, 0xFBBF, 0xEB9E, 0x9B79, 0x8B58, 0xBB3B, 0xAB1A,
	0x6CA6, 0x7C87, 0x4CE4, 0x5CC5, 0x2C22, 0x3C03, 0x0C60, 0x1C41,
	0xEDAE, 0xFD8F, 0xCDEC, 0xDDCD, 0xAD2A, 0xBD0B, 0x8D68, 0x9D49,
	0x7E97, 0x6EB6, 0x5ED5, 0x4EF4, 0x3E13, 0x2E32, 0x1E51, 0x0E70,
	0xFF9F, 0xEFBE, 0xDFDD, 0xCFFC, 0xBF1B, 0xAF3A, 0x9F59, 0x8F78,
	0x9188, 0x81A9, 0xB1CA, 0xA1EB, 0xD10C, 0xC12D, 0xF14E, 0xE16F,
	0x1080, 0x00A1, 0x30C2, 0x20E3, 0x5004, 0x4025, 0x7046, 0x6067,
	0x83B9, 0x9398, 0xA3FB, 0xB3DA, 0xC33D, 0xD31C, 0xE37F, 0xF35E,
	0x02B1, 0x1290, 0x22F3, 0x32D2, 0x4235, 0x5214, 0x6277, 0x7256,
	0xB5EA, 0xA5CB, 0x95A8, 0x8589, 0xF56E, 0xE54F, 0xD52C, 0xC50D,
	0x34E2, 0x24C3, 0x14A0, 0x0481, 0x7466, 0x6447, 0x5424, 0x4405,
	0xA7DB, 0xB7FA, 0x8799, 0x97B8, 0xE75F, 0xF77E, 0xC71D, 0xD73C,
	0x26D3, 0x36F2, 0x0691, 0x16B0, 0x6657, 0x7676, 0x4615, 0x5634,
	0xD94C, 0xC96D, 0xF90E, 0xE92F, 0x99C8, 0x89E9, 0xB98A, 0xA9AB,
	0x5844, 0x4865, 0x7806, 0x6827, 0x18C0, 0x08E1, 0x3882, 0x28A3,
	0xCB7D, 0xDB5C, 0xEB3F, 0xFB1E, 0x8BF9, 0x9BD8, 0xABBB, 0xBB9A,
	0x4A75, 0x5A54, 0x6A37, 0x7A16, 0x0AF1, 0x1AD0, 0x2AB3, 0x3A92,
	0xFD2E, 0xED0F, 0xDD6C, 0xCD4D, 0xBDAA, 0xAD8B, 0x9DE8, 0x8DC9,
	0x7C26, 0x6C07, 0x5C64, 0x4C45, 0x3CA2, 0x2C83, 0x1CE0, 0x0CC1,
	0xEF1F, 0xFF3E, 0xCF5D, 0xDF7C, 0xAF9B, 0xBFBA, 0x8FD9, 0x9FF8,
	0x6E17, 0x7E36, 0x4E55, 0x5E74, 0x2E93, 0x3EB2, 0x0ED1, 0x1EF0,
}

func crc16Arinc(data []byte, init uint16) uint16 {
	crc := init
	for _, b := range data {
		crc = (crc << 8) ^ crc16ArincTable[((crc>>8)^uint16(b))&0xff]
	}
	return crc
}

// CRC-16-CCITT (XModem) - polynomial 0x1021, init 0x0000.
func crc16CCITT(data []byte) uint16 {
	crc := uint16(0x0000)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// CRC-16-CCITT-FALSE - polynomial 0x1021, init 0xFFFF.
func crc16CCITTFalse(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// CRC-16-IBM (ANSI) - polynomial 0x8005, init 0x0000, reflected.
func crc16IBM(data []byte) uint16 {
	crc := uint16(0x0000)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// CRC-16-MODBUS - polynomial 0x8005, init 0xFFFF, reflected.
func crc16Modbus(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// Simple XOR checksum.
func xorChecksum(data []byte) uint16 {
	var crc uint16
	for i := 0; i < len(data)-1; i += 2 {
		crc ^= uint16(data[i])<<8 | uint16(data[i+1])
	}
	if len(data)%2 == 1 {
		crc ^= uint16(data[len(data)-1]) << 8
	}
	return crc
}

// Fletcher-16.
func fletcher16(data []byte) uint16 {
	var sum1, sum2 uint16
	for _, b := range data {
		sum1 = (sum1 + uint16(b)) % 255
		sum2 = (sum2 + sum1) % 255
	}
	return (sum2 << 8) | sum1
}

func main() {
	fmt.Println("Testing CRC algorithms against known ACARS messages")
	fmt.Println("====================================================")

	for _, tc := range testCases {
		// Strip the last 4 chars (checksum) to get the message body.
		msg := tc.full[:len(tc.full)-4]
		data := []byte(msg)

		fmt.Printf("\nMessage: ...%s\n", tc.full[max(0, len(tc.full)-40):])
		fmt.Printf("Expected checksum: %s\n", tc.checksum)
		fmt.Printf("Message body ends: ...%s\n", msg[max(0, len(msg)-30):])

		// Test various algorithms.
		fmt.Printf("  CRC-16-CCITT:       %04X\n", crc16CCITT(data))
		fmt.Printf("  CRC-16-CCITT-FALSE: %04X\n", crc16CCITTFalse(data))
		fmt.Printf("  CRC-16-IBM:         %04X\n", crc16IBM(data))
		fmt.Printf("  CRC-16-MODBUS:      %04X\n", crc16Modbus(data))
		fmt.Printf("  XOR-16:             %04X\n", xorChecksum(data))
		fmt.Printf("  Fletcher-16:        %04X\n", fletcher16(data))

		// Try from /RP onwards (skip header).
		if idx := strings.Index(msg, "/RP"); idx > 0 {
			portion := msg[idx:]
			fmt.Printf("  (from /RP onwards)\n")
			fmt.Printf("  CRC-16-CCITT:       %04X\n", crc16CCITT([]byte(portion)))
			fmt.Printf("  CRC-16-CCITT-FALSE: %04X\n", crc16CCITTFalse([]byte(portion)))
		}

		// Try just the main FPN body (after header, before /WD).
		if idx := strings.Index(msg, "/WD"); idx > 0 {
			beforeWD := msg[:idx]
			fmt.Printf("  (before /WD)\n")
			fmt.Printf("  CRC-16-CCITT:       %04X\n", crc16CCITT([]byte(beforeWD)))
			fmt.Printf("  CRC-16-CCITT-FALSE: %04X\n", crc16CCITTFalse([]byte(beforeWD)))
		}
	}

	// Also test the other checksum (49BE) that appears before /WD.
	fmt.Println("\n\nTesting the intermediate checksum (49BE)...")
	msg := "FPN/ID23565S,WIDE12,ZPZWTCP12004/MR2,3/RP:DA:KMCF:AA:KTIK:F:CUSEK.T349.KNRAD..N25400W080030..N26140W080140..N25450W080230..FEMID.Q102.CIGAR.Q102.BACCA.Q102.BLVNS.Q105.HRV.J58.AEX..WOLUR:V:KNRAD,351,AT2200,,:V:N26140W080140,277,AT2200,,:V:N25450W080230,272,AT3600,,:V:CIGAR,269,AT3600,,:V:BACCA,271,AT3600,,:V:HRV,282,AT3400,,"

	fmt.Printf("Testing portion ending with ',,' (before 49BE): ...%s\n", msg[len(msg)-40:])
	fmt.Printf("  CRC-16-CCITT:       %04X\n", crc16CCITT([]byte(msg)))
	fmt.Printf("  CRC-16-CCITT-FALSE: %04X\n", crc16CCITTFalse([]byte(msg)))
	fmt.Printf("  CRC-16-IBM:         %04X\n", crc16IBM([]byte(msg)))
	fmt.Printf("  CRC-16-MODBUS:      %04X\n", crc16Modbus([]byte(msg)))
	fmt.Printf("  CRC-16-ARINC(0):    %04X\n", crc16Arinc([]byte(msg), 0x0000))
	fmt.Printf("  CRC-16-ARINC(FFFF): %04X\n", crc16Arinc([]byte(msg), 0xFFFF))

	// Try verification mode - append checksum bytes and see if result is 0x1D0F.
	fmt.Println("\n\nTesting verification mode (message+checksum should give 0x1D0F)...")

	for i, tc := range testCases {
		msgWithoutCRC := tc.full[:len(tc.full)-4]
		checksumHex := tc.full[len(tc.full)-4:]

		// Decode checksum hex to bytes.
		checksumBytes := make([]byte, 2)
		_, _ = fmt.Sscanf(checksumHex, "%02X%02X", &checksumBytes[0], &checksumBytes[1])

		// Append checksum bytes to message.
		combined := append([]byte(msgWithoutCRC), checksumBytes...)

		result := crc16Arinc(combined, 0xFFFF)
		valid := result == 0x1D0F

		fmt.Printf("Message %d: checksum=%s -> CRC=%04X valid=%v\n", i+1, checksumHex, result, valid)

		// Now try to calculate what the checksum should be.
		msgCRC := crc16Arinc([]byte(msgWithoutCRC), 0xFFFF)
		fmt.Printf("  CRC of message only: %04X\n", msgCRC)

		// The "augmented" checksum bytes that make CRC(msg+bytes)=0x1D0F
		// For CRC-16, we need bytes such that CRC(msg || bytes) = 0x1D0F
		// This is typically: XOR the message CRC with a constant
		fmt.Printf("  Checksum bytes: %02X %02X\n", checksumBytes[0], checksumBytes[1])
	}

	// Test if we can calculate the checksum.
	fmt.Println("\n\nTrying to reverse-engineer checksum calculation...")
	msg1 := testCases[0].full[:len(testCases[0].full)-4]
	crc1 := crc16Arinc([]byte(msg1), 0xFFFF)
	fmt.Printf("Message 1 CRC: %04X, expected checksum: 75A7\n", crc1)

	// The final checksum might be CRC ^ some_constant or just the CRC itself.
	// Or it might need to be byte-swapped.
	fmt.Printf("  Swapped: %04X\n", ((crc1&0xFF)<<8)|((crc1>>8)&0xFF))
	fmt.Printf("  XOR FFFF: %04X\n", crc1^0xFFFF)
	fmt.Printf("  XOR 1D0F: %04X\n", crc1^0x1D0F)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
