package acars

import (
	"encoding/json"
	"testing"
)

func TestFlexInt64_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    FlexInt64
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

	t.Run("tail from airframe", func(t *testing.T) {
		w := &NATSWrapper{
			Message: &NATSInner{
				ID:    456,
				Label: "80",
				Text:  "Position report",
			},
			Airframe: &Airframe{
				Tail: "N12345",
				ICAO: "A12345",
			},
		}

		msg := w.ToMessage()
		if msg == nil {
			t.Fatal("expected message, got nil")
		}
		if msg.Tail != "N12345" {
			t.Errorf("Tail = %s, want N12345 (from airframe)", msg.Tail)
		}
		if msg.Airframe == nil {
			t.Error("Airframe should be populated")
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
