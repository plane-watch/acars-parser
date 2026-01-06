package crc

import "testing"

// Test cases from real FPN messages with known valid checksums.
// The message includes the checksum at the end (last 4 hex chars).
var testCases = []struct {
	name         string
	fullMessage  string // Full message including checksum hex at end
	checksumHex  string // The checksum (last 4 chars)
	shouldVerify bool
}{
	{
		name:         "FPN message with 75A7 checksum",
		fullMessage:  "FPN/ID23565S,WIDE12,ZPZWTCP12004/MR2,3/RP:DA:KMCF:AA:KTIK:F:CUSEK.T349.KNRAD..N25400W080030..N26140W080140..N25450W080230..FEMID.Q102.CIGAR.Q102.BACCA.Q102.BLVNS.Q105.HRV.J58.AEX..WOLUR:V:KNRAD,351,AT2200,,:V:N26140W080140,277,AT2200,,:V:N25450W080230,272,AT3600,,:V:CIGAR,269,AT3600,,:V:BACCA,271,AT3600,,:V:HRV,282,AT3400,,49BE/WD,,,,75A7",
		checksumHex:  "75A7",
		shouldVerify: true,
	},
	{
		name:         "FPN message with A7A7 checksum",
		fullMessage:  "FPN/ID23565S,WIDE12,ZPZWTCP12004/MR2,5/RP:DA:KMCF:AA:KTIK:F:CUSEK.T349.KNRAD..N25400W080030..N26140W080140..N25450W080230..FEMID.Q102.CIGAR.Q102.BACCA.Q102.BLVNS.Q105.HRV.J58.AEX..WOLUR:V:KNRAD,351,AT2200,,:V:N26140W080140,277,AT2200,,:V:N25450W080230,272,AT3600,,:V:CIGAR,269,AT3600,,:V:BACCA,271,AT3600,,:V:HRV,282,AT3400,,49BE/WD,,,,A7A7",
		checksumHex:  "A7A7",
		shouldVerify: true,
	},
	{
		name:         "FPN message with B27B checksum",
		fullMessage:  "FPN/ID38883S,ROMA94,8VH072E14004/MR1,2/RP:DA:KWRI:AA:KSKA:R:06O:F:MXE..PENSY.J110.LARRI.Q430.BEETS.J110.GRAHM..MOAWK..MUSIT..NCOLY..BYPOR..KP18E..KP18Y..KU18S..KU15M..MLP:A:HILIE3.MLP(23O):V:PENSY,246,AT4000,,315D/WD,,,,B27B",
		checksumHex:  "B27B",
		shouldVerify: true,
	},
	{
		name:         "FPN message with 0AE8 checksum",
		fullMessage:  "FPN/ID00339S,RCH12,8VH067E12004/MR1,2/RP:DA:KWRI:AA:KSKA:F:FJC..SFK..DMACK..RUBKI..JUVAG..DLH..N47000W094000..N47300W100000..N48000W106000..CHOTE..MLP:V:DMACK,302,AT3000,,:V:N47300W100000,246,AT4000,,5FD6/WD,,,,0AE8",
		checksumHex:  "0AE8",
		shouldVerify: true,
	},
	{
		name:         "Truncated message (invalid checksum)",
		fullMessage:  "FPN/ID23565S,WIDE12,ZPZWTCP12004/MR2,3/RP:DA:KMCF:AA:KTIK:F:CUSEK.T349.KNRAD/WD,,,,FFFF",
		checksumHex:  "FFFF",
		shouldVerify: false,
	},
}

func TestCRC16ArincVerification(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Strip the last 4 chars (checksum hex) from the message.
			msgWithoutCRC := tc.fullMessage[:len(tc.fullMessage)-4]

			// Decode the checksum hex to bytes.
			checksumBytes := []byte{
				HexToByte(tc.checksumHex[0], tc.checksumHex[1]),
				HexToByte(tc.checksumHex[2], tc.checksumHex[3]),
			}

			// Verify the CRC.
			result := Verify16Arinc([]byte(msgWithoutCRC), checksumBytes)
			if result != tc.shouldVerify {
				t.Errorf("Verify16Arinc() = %v, want %v", result, tc.shouldVerify)
			}
		})
	}
}

func TestCalculate16Arinc(t *testing.T) {
	// Test that Calculate produces the correct checksum for a known message.
	// Strip the checksum from the full message.
	tc := testCases[0]
	msgWithoutCRC := tc.fullMessage[:len(tc.fullMessage)-4]

	checksum := Calculate16Arinc([]byte(msgWithoutCRC))
	gotHex := []byte{
		hexDigit(checksum[0] >> 4),
		hexDigit(checksum[0] & 0x0F),
		hexDigit(checksum[1] >> 4),
		hexDigit(checksum[1] & 0x0F),
	}

	if string(gotHex) != tc.checksumHex {
		t.Errorf("Calculate16Arinc() = %s, want %s", string(gotHex), tc.checksumHex)
	}
}

func hexDigit(b byte) byte {
	if b < 10 {
		return '0' + b
	}
	return 'A' + b - 10
}

func TestIsHexDigit(t *testing.T) {
	tests := []struct {
		c    byte
		want bool
	}{
		{'0', true}, {'9', true},
		{'A', true}, {'F', true},
		{'a', true}, {'f', true},
		{'G', false}, {'g', false},
		{' ', false}, {'-', false},
	}
	for _, tt := range tests {
		if got := IsHexDigit(tt.c); got != tt.want {
			t.Errorf("IsHexDigit(%q) = %v, want %v", tt.c, got, tt.want)
		}
	}
}

func TestHexToByte(t *testing.T) {
	tests := []struct {
		high, low byte
		want      byte
	}{
		{'0', '0', 0x00},
		{'F', 'F', 0xFF},
		{'A', '5', 0xA5},
		{'7', 'E', 0x7E},
		{'a', 'b', 0xAB},
	}
	for _, tt := range tests {
		if got := HexToByte(tt.high, tt.low); got != tt.want {
			t.Errorf("HexToByte(%q, %q) = %02X, want %02X", tt.high, tt.low, got, tt.want)
		}
	}
}

func TestCRC16ArincRoundTrip(t *testing.T) {
	// Test that we can calculate a checksum and then verify it.
	message := "Test message for CRC verification"

	// Calculate the checksum.
	checksum := Calculate16Arinc([]byte(message))

	// Verify it.
	if !Verify16Arinc([]byte(message), checksum) {
		t.Error("Round-trip verification failed")
	}

	// Modify the checksum and verify it fails.
	badChecksum := []byte{checksum[0] ^ 0xFF, checksum[1]}
	if Verify16Arinc([]byte(message), badChecksum) {
		t.Error("Bad checksum should not verify")
	}
}