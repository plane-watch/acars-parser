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
			// The registration field is seven characters, padded on the
			// left with dots: a seven-character registration has no dot
			// after the IMI. Real message from the January 2026 corpus.
			name:           "Seven-character registration",
			text:           "/YEGE2YA.AT1B-1877224C8C0DE2B1624D9F3AA4F9C17A760F1D0",
			wantGS:         "YEGE2YA",
			wantIMI:        "AT1",
			wantReg:        "B-18772",
			wantPayloadLen: 15,
		},
		{
			// Two padding dots before a five-character registration.
			name:           "Five-character registration",
			text:           "/YQME2YA.AT1..N17RX22CE87E840CCD8",
			wantGS:         "YQME2YA",
			wantIMI:        "AT1",
			wantReg:        "N17RX",
			wantPayloadLen: 5,
		},
		{
			// A seven-character registration ending in a hex digit ("1"):
			// the field's length, not the hex, ends it.
			name:           "Registration ending in a hex digit",
			text:           "/SGNGWXA.ADSB-1673107010BCD0D010E0130D6",
			wantGS:         "SGNGWXA",
			wantIMI:        "ADS",
			wantReg:        "B-16731",
			wantPayloadLen: 8,
		},
		{
			// Lower-case payload hex; the CRC is valid.
			name:           "Lower-case hex",
			text:           "/NYCODYA.AT1.N784AV22c823e840fbce",
			wantGS:         "NYCODYA",
			wantIMI:        "AT1",
			wantReg:        "N784AV",
			wantPayloadLen: 5,
		},
		{
			// A dot inside the registration field is not padding.
			name:    "Dot inside the registration",
			text:    "/ABCD.AT1.N5.4DN004BCE",
			wantErr: ErrUnknownFormat,
		},
		{
			// Not an ARINC 622 IMI, although the CRC is valid.
			name:    "Unknown IMI",
			text:    "/ABCD.XY9.N514DN00A453",
			wantErr: ErrUnknownFormat,
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

// The inputs are real label H1 messages from the January 2026 corpus,
// truncated after the registration.
func TestUnwrap(t *testing.T) {
	tests := []struct {
		text, want, wantLabel string
	}{
		// Relayed with the original label.
		{"- #MD/AA PIKCPYA.AT1.N657UA21B75B", "/PIKCPYA.AT1.N657UA21B75B", "AA"},
		{"- #MD/A6 YQXE2YA.ADS.N830MH07010B", "/YQXE2YA.ADS.N830MH07010B", "A6"},
		// Without the leading "/".
		{"USADCXA.AT1.N200WN679F2093004DAA", "/USADCXA.AT1.N200WN679F2093004DAA", ""},
		// Already in the form Parse reads.
		{"/SOUCAYA.AT1.HL8251ABCD", "/SOUCAYA.AT1.HL8251ABCD", ""},
		// A seven-character registration follows the IMI with no dot.
		{"- #MD/AA YEGE2YA.AT1B-1877224C8C0DE2", "/YEGE2YA.AT1B-1877224C8C0DE2", "AA"},
		{"YEGE2YA.AT1B-1877224C8C0DE2", "/YEGE2YA.AT1B-1877224C8C0DE2", ""},
		{"/YEGE2YA.AT1B-1877224C8C0DE2", "/YEGE2YA.AT1B-1877224C8C0DE2", ""},
		// Character-oriented applications, such as AFN, follow the IMI with
		// "/" rather than ".".
		{"- #MD/A0 OAKODYA.AFN/FMHTZP16,.JA822J", "/OAKODYA.AFN/FMHTZP16,.JA822J", "A0"},
		{"OAKODYA.AFN/FMHTZP16,.JA822J", "/OAKODYA.AFN/FMHTZP16,.JA822J", ""},
		{"/OAKODYA.AFN/FMHTZP16,.JA822J", "/OAKODYA.AFN/FMHTZP16,.JA822J", ""},
	}
	for _, tt := range tests {
		got, label, ok := Unwrap(tt.text)
		if !ok || got != tt.want || label != tt.wantLabel {
			t.Errorf("Unwrap(%q) = %q, %q, %v; want %q, %q, true", tt.text, got, label, ok, tt.want, tt.wantLabel)
		}
	}
	for _, text := range []string{"- #MDREQPOS037B", "REQPOS", "- #MD/AA free text"} {
		if got, _, ok := Unwrap(text); ok {
			t.Errorf("Unwrap(%q) = %q, true; want false", text, got)
		}
	}
}
