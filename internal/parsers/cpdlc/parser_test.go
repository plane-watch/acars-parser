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
			// Message with valid CRC - decodes successfully.
			// Direction is determined by semantic validation of decoded elements.
			name:     "Valid CRC decodes successfully",
			label:    "AA",
			text:     "/ANCATYA.AT1.N514DN220012E8294A952882D8",
			wantType: "cpdlc",
			// The payload is uM160 NEXT DATA AUTHORITY RJJJ, which only the
			// uplink message set decodes; the label alone does not decide
			// the direction.
			wantDir:      "uplink",
			wantElements: 1,
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

// TestParseRelayedH1 checks the forms in which label H1 carries CPDLC, using
// real messages from the January 2026 corpus.
func TestParseRelayedH1(t *testing.T) {
	tests := []struct {
		name, text, link, wantStation, wantReg, wantDir, wantText string
	}{
		{
			// Relayed with its original label, AA. Only the uplink message
			// set gives a valid element, so the decoder corrects the
			// direction, and the result reports the corrected one.
			name: "relayed with the original label", text: "- #MD/AA YQME2YA.AT1..N17RX22CE87E840CCD8",
			wantStation: "YQME2YA", wantReg: ".N17RX", wantDir: "uplink", wantText: "END SERVICE",
		},
		{
			// Without the leading "/", from live traffic (October 2026). The
			// payload is dM0 WILCO as a downlink and uM0 UNABLE as an
			// uplink; the link layer gave the direction.
			name: "bare, with the link direction", text: "USADCXA.AT1.N7857B618691D300B734", link: "downlink",
			wantStation: "USADCXA", wantReg: "N7857B", wantDir: "downlink", wantText: "WILCO",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "H1", Text: tt.text, LinkDirection: tt.link}).(*Result)
			if !ok {
				t.Fatal("Parse returned no result")
			}
			if r.Error != "" || r.GroundStation != tt.wantStation || r.Registration != tt.wantReg ||
				r.Direction != tt.wantDir || r.FormattedText != tt.wantText {
				t.Errorf("got error %q, station %q, reg %q, direction %q, text %q", r.Error, r.GroundStation, r.Registration, r.Direction, r.FormattedText)
			}
		})
	}
}

// TestParseDirectionFallback checks the direction of a message whose
// payload decodes validly both ways (dM0 WILCO, uM0 UNABLE) when the link
// layer and block ID do not give it. Label BA is a downlink and AA an
// uplink; label H1 carries both, so the message is not decoded.
func TestParseDirectionFallback(t *testing.T) {
	tests := []struct {
		label, text, wantDir, wantText, wantError string
	}{
		{"BA", "/USADCXA.AT1.N7857B618691D300B734", "downlink", "WILCO", ""},
		{"AA", "/USADCXA.AT1.N7857B618691D300B734", "uplink", "UNABLE", ""},
		{"H1", "USADCXA.AT1.N7857B618691D300B734", "", "", "direction_unknown"},
	}
	for _, tt := range tests {
		r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: tt.label, Text: tt.text}).(*Result)
		if !ok {
			t.Fatalf("label %s: Parse returned no result", tt.label)
		}
		if r.Direction != tt.wantDir || r.FormattedText != tt.wantText || r.Error != tt.wantError || (tt.wantError != "" && len(r.Elements) != 0) {
			t.Errorf("label %s: direction %q, text %q, error %q, %d elements; want %q, %q, %q",
				tt.label, r.Direction, r.FormattedText, r.Error, len(r.Elements), tt.wantDir, tt.wantText, tt.wantError)
		}
	}
}
