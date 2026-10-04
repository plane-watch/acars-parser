package acars

import (
	"encoding/json"
	"testing"
)

func TestFlexInt64_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  FlexInt64
	}{
		{"integer", `123`, 123},
		{"string number", `"456"`, 456},
		{"empty string", `""`, 0},
		{"negative integer", `-100`, -100},
		{"negative string", `"-200"`, -200},
		{"large number", `9223372036854775807`, 9223372036854775807},
		{"zero", `0`, 0},
		{"string zero", `"0"`, 0},
		{"invalid string", `"not a number"`, 0},
		{"null", `null`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got FlexInt64
			err := json.Unmarshal([]byte(tt.input), &got)
			if err != nil {
				t.Fatalf("Unmarshal returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("FlexInt64 = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNATSWrapper_ToMessage(t *testing.T) {
	t.Run("nil message", func(t *testing.T) {
		w := &NATSWrapper{}
		msg := w.ToMessage()
		if msg != nil {
			t.Errorf("expected nil, got %+v", msg)
		}
	})

	t.Run("basic conversion", func(t *testing.T) {
		w := &NATSWrapper{
			Message: &NATSInner{
				ID:        123,
				Timestamp: "2024-01-15T12:00:00Z",
				Label:     "H1",
				Text:      "Test message",
				Tail:      "VH-ABC",
				Frequency: 131.55,
			},
		}

		msg := w.ToMessage()
		if msg == nil {
			t.Fatal("expected message, got nil")
		}
		if msg.ID != 123 {
			t.Errorf("ID = %d, want 123", msg.ID)
		}
		if msg.Label != "H1" {
			t.Errorf("Label = %s, want H1", msg.Label)
		}
		if msg.Tail != "VH-ABC" {
			t.Errorf("Tail = %s, want VH-ABC", msg.Tail)
		}
	})

	t.Run("does not take facts from Airframes metadata", func(t *testing.T) {
		// Airframes' airframe and flight records are its own enrichment, not
		// transmitted data, so they must not fill in the message's fields.
		w := &NATSWrapper{
			Message: &NATSInner{
				ID:    456,
				Label: "80",
				Text:  "Position report",
			},
			Airframe: &Airframe{Tail: "N12345", ICAO: "A12345"},
			Flight:   &Flight{Flight: "XYZ999"},
		}

		msg := w.ToMessage()
		if msg == nil {
			t.Fatal("expected message, got nil")
		}
		if msg.Tail != "" {
			t.Errorf("Tail = %q, want empty (not taken from airframe)", msg.Tail)
		}
		if msg.FlightNumber != "" {
			t.Errorf("FlightNumber = %q, want empty (not taken from the flight record)", msg.FlightNumber)
		}
	})

	t.Run("keeps the transmitted flight number and link-layer addresses", func(t *testing.T) {
		w := &NATSWrapper{
			Message: &NATSInner{
				ID:      7709358816,
				Label:   "H1",
				Text:    "text",
				Flight:  "TK6308  ",
				FromHex: "4BB154",
				ToHex:   "11904a",
			},
			Flight: &Flight{Flight: "THY6308"},
		}

		msg := w.ToMessage()
		if msg.FlightNumber != "TK6308" {
			t.Errorf("FlightNumber = %q, want %q (transmitted, trimmed)", msg.FlightNumber, "TK6308")
		}
		if msg.FromHex != "4BB154" || msg.ToHex != "11904A" {
			t.Errorf("FromHex, ToHex = %q, %q, want 4BB154, 11904A (upper-cased)", msg.FromHex, msg.ToHex)
		}
	})

	t.Run("preserves direction indicators", func(t *testing.T) {
		w := &NATSWrapper{
			Message: &NATSInner{
				ID:            789,
				Label:         "AA",
				Text:          "CPDLC message",
				BlockID:       "2",
				LinkDirection: "downlink",
			},
		}

		msg := w.ToMessage()
		if msg == nil {
			t.Fatal("expected message, got nil")
		}
		if msg.BlockID != "2" {
			t.Errorf("BlockID = %s, want 2", msg.BlockID)
		}
		if msg.LinkDirection != "downlink" {
			t.Errorf("LinkDirection = %s, want downlink", msg.LinkDirection)
		}
	})
}

func TestMessage_JSONRoundTrip(t *testing.T) {
	original := &Message{
		ID:        12345,
		Timestamp: "2024-01-15T10:30:00Z",
		Label:     "H1",
		Text:      "FPN/FN123:DA:YSSY:AA:KLAX",
		Tail:      "VH-OQA",
		Frequency: 131.55,
		Airframe: &Airframe{
			Tail: "VH-OQA",
			ICAO: "7C6B2D",
		},
		Flight: &Flight{
			Flight:             "QF1",
			DepartingAirport:   "YSSY",
			DestinationAirport: "KLAX",
		},
	}

	// Marshal to JSON.
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal back.
	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify key fields.
	if decoded.ID != original.ID {
		t.Errorf("ID = %d, want %d", decoded.ID, original.ID)
	}
	if decoded.Label != original.Label {
		t.Errorf("Label = %s, want %s", decoded.Label, original.Label)
	}
	if decoded.Text != original.Text {
		t.Errorf("Text = %s, want %s", decoded.Text, original.Text)
	}
	if decoded.Airframe == nil {
		t.Error("Airframe should not be nil")
	} else if decoded.Airframe.ICAO != original.Airframe.ICAO {
		t.Errorf("Airframe.ICAO = %s, want %s", decoded.Airframe.ICAO, original.Airframe.ICAO)
	}
	if decoded.Flight == nil {
		t.Error("Flight should not be nil")
	} else if decoded.Flight.Flight != original.Flight.Flight {
		t.Errorf("Flight.Flight = %s, want %s", decoded.Flight.Flight, original.Flight.Flight)
	}
}

func TestAircraftAddress(t *testing.T) {
	tests := []struct {
		name   string
		msg    Message
		want   string
		wantOK bool
	}{
		{"explicit downlink: the aircraft is the sender",
			Message{LinkDirection: "downlink", FromHex: "4BB154", ToHex: "11904A"}, "4BB154", true},
		{"explicit uplink: the aircraft is the recipient",
			Message{LinkDirection: "uplink", FromHex: "11904A", ToHex: "4BB154"}, "4BB154", true},
		{"digit block ID is a downlink",
			Message{BlockID: "3", FromHex: "A21127", ToHex: "11919A"}, "A21127", true},
		{"letter block ID is an uplink",
			Message{BlockID: "C", FromHex: "11919A", ToHex: "A21127"}, "A21127", true},
		{"direction unknown",
			Message{FromHex: "A21127", ToHex: "11919A"}, "", false},
		{"no address for the direction",
			Message{LinkDirection: "downlink", ToHex: "11919A"}, "", false},
		{"not a 24-bit hex address",
			Message{LinkDirection: "downlink", FromHex: "XYZ123"}, "", false},
		{"all zeros is not an aircraft address",
			Message{LinkDirection: "downlink", FromHex: "000000"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.msg.AircraftAddress()
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("AircraftAddress() = %q, %v, want %q, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
