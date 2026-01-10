package cpdlc

import (
	"encoding/hex"
	"math"
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
	}{
		{
			name:         "Downlink DEVIATING message",
			label:        "AA",
			text:         "/BOMCAYA.AT1.A4O-SI005080204A",
			wantType:     "cpdlc",
			wantDir:      "downlink",
			wantElements: 1, // dM80 = DEVIATING [distanceoffset][direction] OF ROUTE.
		},
		{
			name:     "Connect request",
			label:    "AA",
			text:     "/NYCODYA.CR1.N784AVABCD1234",
			wantType: "connect_request",
			wantDir:  "downlink",
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
				t.Fatal("Parse() returned nil")
			}

			r := result.(*Result)
			if r.MessageType != tt.wantType {
				t.Errorf("MessageType = %v, want %v", r.MessageType, tt.wantType)
			}
			if r.Direction != tt.wantDir {
				t.Errorf("Direction = %v, want %v", r.Direction, tt.wantDir)
			}
			if tt.wantElements > 0 && len(r.Elements) != tt.wantElements {
				t.Errorf("Elements count = %d, want %d", len(r.Elements), tt.wantElements)
			}
			if tt.wantError && r.Error == "" {
				t.Error("Expected error but got none")
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

func TestIsValidHex(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ABCD1234", true},
		{"abcd1234", true},
		{"0123456789ABCDEF", true},
		{"ABC", false},  // Odd length.
		{"GHIJ", false}, // Invalid chars.
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isValidHex(tt.input); got != tt.want {
				t.Errorf("isValidHex(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitRegistrationAndData(t *testing.T) {
	tests := []struct {
		input   string
		wantReg string
		wantHex string
	}{
		{"F-GSQC214823E24092E7", "F-GSQC", "214823E24092E7"},
		{"N784AV22C823E840FBCE", "N784AV", "22C823E840FBCE"},
		// Full message from database (hex must be even length).
		{"TC-LLH2148242A526A48934D049A6820CE4106AD49F360D48B1104D8B4E9C18F150549E821CF9D1A4D29A821D089321A0873E754830EA20AF26A48411E0CE8916920893E6C5A7524C39201", "TC-LLH", "2148242A526A48934D049A6820CE4106AD49F360D48B1104D8B4E9C18F150549E821CF9D1A4D29A821D089321A0873E754830EA20AF26A48411E0CE8916920893E6C5A7524C39201"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotReg, gotHex := splitRegistrationAndData(tt.input)
			if gotReg != tt.wantReg {
				t.Errorf("reg = %q, want %q", gotReg, tt.wantReg)
			}
			if gotHex != tt.wantHex {
				t.Errorf("hex = %q, want %q", gotHex, tt.wantHex)
			}
		})
	}
}

func TestDecodeElementID(t *testing.T) {
	// Test that specific hex data decodes to the expected element ID.
	// Hex: 005080204A should decode to dM80 (DEVIATING).
	parser := &Parser{}

	msg := &acars.Message{
		ID:        1,
		Label:     "AA",
		Text:      "/BOMCAYA.AT1.A4O-SI005080204A",
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
	// The expected element ID is 80 (dM80 = DEVIATING [distanceoffset][direction] OF ROUTE).
	if elem.ID != 80 {
		t.Errorf("Element ID = %d, want 80", elem.ID)
	}

	// Verify the label matches.
	if elem.Label != "DEVIATING [distanceoffset] [direction] OF ROUTE" {
		t.Errorf("Element Label = %q, want 'DEVIATING [distanceoffset] [direction] OF ROUTE'", elem.Label)
	}

	t.Logf("Decoded element: ID=%d, Label=%s, Text=%s", elem.ID, elem.Label, elem.Text)
}

func TestDecodePositionReportDM48(t *testing.T) {
	// Raw hex payload (incl. FCS at the end in the ACARS message; our parser trims 2 bytes already).
	// This is a downlink (label BA) CPDLC dM48 POSITION REPORT and should decode to a populated PositionReport.
	rawHex := "20B2C90C3D903BAE2D1141ECCB325824E8B4A249686255AD06655B3041390B6B09360D693499564B009A26"

	b, err := hex.DecodeString(rawHex)
	if err != nil {
		t.Fatalf("hex decode: %v", err)
	}
	if len(b) < 3 {
		t.Fatalf("payload too short")
	}
	// Trim FCS (2 bytes) like the parser does.
	b = b[:len(b)-2]

	d := NewDecoder(b, DirectionDownlink)
	msg, err := d.Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if msg == nil {
		t.Fatalf("nil msg")
	}
	if msg.Header.MsgID != 1 {
		t.Fatalf("unexpected header: %+v", msg.Header)
	}
	if msg.Header.Timestamp == nil || msg.Header.Timestamp.Hours != 12 || msg.Header.Timestamp.Minutes != 44 {
		t.Fatalf("unexpected header timestamp: %+v", msg.Header.Timestamp)
	}
	if len(msg.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(msg.Elements))
	}
	el := msg.Elements[0]
	if el.ID != 48 {
		t.Fatalf("expected element 48, got %d", el.ID)
	}
	pr, ok := el.Data.(*PositionReport)
	if !ok || pr == nil {
		t.Fatalf("expected PositionReport data, got %T", el.Data)
	}

	// Spot-check key fields against libacars reference decode.
	if pr.PosCurrent == nil || pr.PosCurrent.Latitude == nil || pr.PosCurrent.Longitude == nil {
		t.Fatalf("missing pos_current lat/lon: %+v", pr.PosCurrent)
	}
	if math.Abs(*pr.PosCurrent.Latitude-46.3) > 0.01 || math.Abs(*pr.PosCurrent.Longitude-20.205) > 0.01 {
		t.Fatalf("unexpected pos_current lat/lon: %v,%v", *pr.PosCurrent.Latitude, *pr.PosCurrent.Longitude)
	}
	if pr.TimeAtPosCurrent == nil || pr.TimeAtPosCurrent.Hours != 12 || pr.TimeAtPosCurrent.Minutes != 44 {
		t.Fatalf("unexpected time_at_pos_current: %+v", pr.TimeAtPosCurrent)
	}
	if pr.Alt == nil || pr.Alt.Type != "flight_level" || pr.Alt.Value != 330 {
		t.Fatalf("unexpected alt: %+v", pr.Alt)
	}
	if pr.NextFix == nil || pr.NextFix.Name != "NERDI" {
		t.Fatalf("unexpected next_fix: %+v", pr.NextFix)
	}
	if pr.NextNextFix == nil || pr.NextNextFix.Name != "UVALU" {
		t.Fatalf("unexpected next_next_fix: %+v", pr.NextNextFix)
	}
	if pr.Temp == nil || pr.Temp.Type != "C" || math.Abs(pr.Temp.Value-(-48.0)) > 0.001 {
		t.Fatalf("unexpected temp: %+v", pr.Temp)
	}
	if pr.Winds == nil || pr.Winds.Direction != 314 || pr.Winds.Speed == nil || pr.Winds.Speed.Type != "kts" || pr.Winds.Speed.Value != 22 {
		t.Fatalf("unexpected winds: %+v", pr.Winds)
	}
	if pr.Speed == nil || pr.Speed.Type != "mach" || pr.Speed.Value != 83 {
		t.Fatalf("unexpected speed: %+v", pr.Speed)
	}
	if pr.ReportedWptPos == nil || pr.ReportedWptPos.Name != "MAVIR" {
		t.Fatalf("unexpected reported_wpt_pos: %+v", pr.ReportedWptPos)
	}
	if pr.ReportedWptTime == nil || pr.ReportedWptTime.Hours != 12 || pr.ReportedWptTime.Minutes != 42 {
		t.Fatalf("unexpected reported_wpt_time: %+v", pr.ReportedWptTime)
	}
	if pr.ReportedWptAlt == nil || pr.ReportedWptAlt.Type != "flight_level" || pr.ReportedWptAlt.Value != 330 {
		t.Fatalf("unexpected reported_wpt_alt: %+v", pr.ReportedWptAlt)
	}
}
