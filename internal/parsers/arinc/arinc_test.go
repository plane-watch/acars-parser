package arinc

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		wantGS         string
		wantIMI        string
		wantReg        string
		wantPayloadLen int // -1 means expect error.
		wantErr        error
	}{
		{
			// Full valid message from libacars example (includes CRC A7F0).
			name:           "Valid AT1 message with CRC",
			text:           "/SOUCAYA.AT1.HL8251243F880C3D903BB412903604FE326C2479F4A64F7F62528B1A9CF8382738186AC28B16668E013DF464D8A7F0",
			wantGS:         "SOUCAYA",
			wantIMI:        "AT1",
			wantReg:        "HL8251",
			wantPayloadLen: 42, // 44 bytes hex - 2 bytes CRC = 42 bytes payload.
		},
		{
			// This message has valid CRC but malformed CPDLC content.
			// CRC passes, but CPDLC decode will fail later.
			name:           "Valid CRC but malformed CPDLC content",
			text:           "/ANCATYA.AT1.N514DN220012E8294A952882D8",
			wantGS:         "ANCATYA",
			wantIMI:        "AT1",
			wantReg:        "N514DN",
			wantPayloadLen: 8, // 10 bytes - 2 bytes CRC = 8 bytes.
		},
		{
			// Truly truncated message - missing CRC bytes entirely.
			name:    "Too short - missing CRC",
			text:    "/TESTAYA.AT1.N12345AB",
			wantErr: ErrTooShort,
		},
		{
			name:    "Invalid format - no IMI",
			text:    "/TESTAYA.N12345ABCDEF",
			wantErr: ErrUnknownFormat,
		},
		{
			name:    "Invalid format - not ARINC",
			text:    "Some random ACARS text",
			wantErr: ErrUnknownFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.text)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.GroundStation != tt.wantGS {
				t.Errorf("GroundStation = %q, want %q", result.GroundStation, tt.wantGS)
			}
			if result.IMI != tt.wantIMI {
				t.Errorf("IMI = %q, want %q", result.IMI, tt.wantIMI)
			}
			if result.Registration != tt.wantReg {
				t.Errorf("Registration = %q, want %q", result.Registration, tt.wantReg)
			}
			if tt.wantPayloadLen >= 0 && len(result.Payload) != tt.wantPayloadLen {
				t.Errorf("Payload length = %d, want %d", len(result.Payload), tt.wantPayloadLen)
			}
		})
	}
}

func TestSplitRegistrationAndHex(t *testing.T) {
	tests := []struct {
		input   string
		wantReg string
		wantHex string
	}{
		{"HL8251243F880C3D903BB4", "HL8251", "243F880C3D903BB4"},
		{"N784AV22C823E840FBCE", "N784AV", "22C823E840FBCE"},
		{"F-GSQC214823E24092E7", "F-GSQC", "214823E24092E7"},
		{"A4O-SI005080204A", "A4O-SI", "005080204A"},
		// Edge case: short registration.
		{"N1ABCD", "N1", "ABCD"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotReg, gotHex := splitRegistrationAndHex(tt.input)
			if gotReg != tt.wantReg {
				t.Errorf("registration = %q, want %q", gotReg, tt.wantReg)
			}
			if gotHex != tt.wantHex {
				t.Errorf("hex = %q, want %q", gotHex, tt.wantHex)
			}
		})
	}
}

func TestValidateCRC(t *testing.T) {
	tests := []struct {
		name   string
		imi    string
		reg    string
		hexStr string
		want   bool
	}{
		{
			// Full hex including CRC (A7F0) from libacars example.
			name:   "Valid libacars sample",
			imi:    "AT1",
			reg:    "HL8251",
			hexStr: "243F880C3D903BB412903604FE326C2479F4A64F7F62528B1A9CF8382738186AC28B16668E013DF464D8A7F0",
			want:   true,
		},
		{
			// Real message from database - has valid CRC but malformed CPDLC.
			name:   "Valid CRC malformed CPDLC",
			imi:    "AT1",
			reg:    "N514DN",
			hexStr: "220012E8294A952882D8",
			want:   true,
		},
		{
			// Corrupted CRC - flip a bit in the CRC bytes.
			name:   "Invalid - corrupted CRC",
			imi:    "AT1",
			reg:    "HL8251",
			hexStr: "243F880C3D903BB412903604FE326C2479F4A64F7F62528B1A9CF8382738186AC28B16668E013DF464D8A7F1", // Changed last byte.
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hexData := make([]byte, len(tt.hexStr)/2)
			for i := 0; i < len(tt.hexStr); i += 2 {
				var b byte
				_, _ = parseHexByte(tt.hexStr[i:i+2], &b)
				hexData[i/2] = b
			}

			got := validateCRC(tt.imi, tt.reg, hexData)
			if got != tt.want {
				t.Errorf("validateCRC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func parseHexByte(s string, b *byte) (int, error) {
	var v byte
	for _, c := range s {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= byte(c - '0')
		case c >= 'A' && c <= 'F':
			v |= byte(c - 'A' + 10)
		case c >= 'a' && c <= 'f':
			v |= byte(c - 'a' + 10)
		}
	}
	*b = v
	return 2, nil
}

func TestIsCPDLC(t *testing.T) {
	if !IsCPDLC("AT1") {
		t.Error("AT1 should be CPDLC")
	}
	if !IsCPDLC("CR1") {
		t.Error("CR1 should be CPDLC")
	}
	if IsCPDLC("ADS") {
		t.Error("ADS should not be CPDLC")
	}
}
