package cpdlc

import (
	"strings"
	"testing"

	"acars_parser/internal/acars"
)

func TestQuickCheck(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		name      string
		text      string
		wantMatch bool
	}{
		{
			name:      "CPDLC message",
			text:      "/PIKCPYA.AT1.F-GSQC214823E24092E7",
			wantMatch: true,
		},
		{
			name:      "Connect request",
			text:      "/NYCODYA.CR1.N784AV12345678",
			wantMatch: true,
		},
		{
			name:      "Connect confirm",
			text:      "/YQXD2YA.CC1.TC-LLH12345678",
			wantMatch: true,
		},
		{
			name:      "Disconnect",
			text:      "/KZDCAYA.DR1.N12345",
			wantMatch: true,
		},
		{
			name:      "Non-CPDLC",
			text:      "Some random ACARS text",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parser.QuickCheck(tt.text); got != tt.wantMatch {
				t.Errorf("QuickCheck() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestParse(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		name         string
		label        string
		text         string
		wantType     string
		wantDir      string
		wantElements int
		wantError    bool
		wantErrType  string // Expected error type: "crc_failed", "decode_failed", etc.
	}{
		{
			// Full valid message with CRC (A7F0) from libacars example.
			name:         "Downlink dM48 Position Report (valid with CRC)",
			label:        "AA",
			text:         "/SOUCAYA.AT1.HL8251243F880C3D903BB412903604FE326C2479F4A64F7F62528B1A9CF8382738186AC28B16668E013DF464D8A7F0",
			wantType:     "cpdlc",
			wantDir:      "downlink",
			wantElements: 1, // dM48 = POSITION REPORT.
		},
		{
			// Message with valid CRC but malformed CPDLC payload.
			name:        "Valid CRC but decode fails",
			label:       "AA",
			text:        "/ANCATYA.AT1.N514DN220012E8294A952882D8",
			wantType:    "cpdlc",
			wantDir:     "downlink",
			wantError:   true,
			wantErrType: "decode_failed",
		},
		{
			// Message too short - missing CRC.
			name:        "Too short message",
			label:       "AA",
			text:        "/TESTAYA.AT1.N12345AB",
			wantError:   true,
			wantErrType: "message_too_short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &acars.Message{
				ID:        1,
				Label:     tt.label,
				Text:      tt.text,
				Timestamp: "2024-01-01T00:00:00Z",
			}

			result := parser.Parse(msg)
			if result == nil {
				if tt.wantError {
					return // Expected to fail, nil is acceptable for unknown format.
				}
				t.Fatal("Parse() returned nil")
			}

			r := result.(*Result)

			// Check error expectations.
			if tt.wantError {
				if r.Error == "" {
					t.Error("Expected error but got none")
				}
				if tt.wantErrType != "" && !strings.HasPrefix(r.Error, tt.wantErrType) {
					t.Errorf("Error = %q, want prefix %q", r.Error, tt.wantErrType)
				}
				return // Don't check other fields for error cases.
			}

			if r.Error != "" {
				t.Errorf("Unexpected error: %s", r.Error)
			}
			if r.MessageType != tt.wantType {
				t.Errorf("MessageType = %v, want %v", r.MessageType, tt.wantType)
			}
			if r.Direction != tt.wantDir {
				t.Errorf("Direction = %v, want %v", r.Direction, tt.wantDir)
			}
			if tt.wantElements > 0 && len(r.Elements) != tt.wantElements {
				t.Errorf("Elements count = %d, want %d", len(r.Elements), tt.wantElements)
			}
		})
	}
}

func TestBitReader(t *testing.T) {
	// Test basic bit reading.
	data := []byte{0xAB, 0xCD} // 1010 1011 1100 1101
	br := NewBitReader(data)

	// Read 4 bits - should be 1010 = 10.
	v, err := br.ReadBits(4)
	if err != nil {
		t.Fatalf("ReadBits(4) error: %v", err)
	}
	if v != 10 {
		t.Errorf("ReadBits(4) = %d, want 10", v)
	}

	// Read 4 more bits - should be 1011 = 11.
	v, err = br.ReadBits(4)
	if err != nil {
		t.Fatalf("ReadBits(4) error: %v", err)
	}
	if v != 11 {
		t.Errorf("ReadBits(4) = %d, want 11", v)
	}

	// Read 8 more bits - should be 1100 1101 = 205.
	v, err = br.ReadBits(8)
	if err != nil {
		t.Fatalf("ReadBits(8) error: %v", err)
	}
	if v != 0xCD {
		t.Errorf("ReadBits(8) = %d, want 205", v)
	}

	// Should have 0 bits remaining.
	if br.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", br.Remaining())
	}
}

func TestConstrainedInt(t *testing.T) {
	// Test constrained integer reading.
	// Range 0-7 needs 3 bits.
	data := []byte{0b10100000} // 101 = 5 in first 3 bits.
	br := NewBitReader(data)

	v, err := br.ReadConstrainedInt(0, 7)
	if err != nil {
		t.Fatalf("ReadConstrainedInt error: %v", err)
	}
	if v != 5 {
		t.Errorf("ReadConstrainedInt(0,7) = %d, want 5", v)
	}
}

// Note: TestIsValidHex and TestSplitRegistrationAndData moved to internal/parsers/arinc package.

func TestDecodeElementID(t *testing.T) {
	// Test that specific hex data decodes to the expected element ID.
	// Using valid libacars sample: dM48 Position Report with MsgID=8.
	parser := &Parser{}

	// Build ACARS message with valid CPDLC hex from libacars sample.
	// The hex includes the 2-byte CRC (A7F0) at the end.
	msg := &acars.Message{
		ID:        1,
		Label:     "AA",
		Text:      "/SOUCAYA.AT1.HL8251243F880C3D903BB412903604FE326C2479F4A64F7F62528B1A9CF8382738186AC28B16668E013DF464D8A7F0",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("Parse() returned nil")
	}

	r := result.(*Result)
	if r.MessageType != "cpdlc" {
		t.Fatalf("MessageType = %v, want cpdlc", r.MessageType)
	}

	if len(r.Elements) == 0 {
		t.Fatal("No elements decoded")
	}

	elem := r.Elements[0]
	// The expected element ID is 48 (dM48 = POSITION REPORT [positionreport]).
	if elem.ID != 48 {
		t.Errorf("Element ID = %d, want 48", elem.ID)
	}

	// Verify the label matches.
	if elem.Label != "POSITION REPORT [positionreport]" {
		t.Errorf("Element Label = %q, want 'POSITION REPORT [positionreport]'", elem.Label)
	}

	// Verify the CPDLC header MsgID is correct.
	// Note: r.MsgID is the ACARS message ID; CPDLC message ID is in the header.
	if r.Header == nil {
		t.Fatal("Header is nil")
	}
	if r.Header.MsgID != 8 {
		t.Errorf("Header.MsgID = %d, want 8", r.Header.MsgID)
	}

	t.Logf("Decoded element: ID=%d, Label=%s, Text=%s", elem.ID, elem.Label, elem.Text)
}
