package labelb3

import (
	"testing"

	"acars_parser/internal/acars"
)

// Real B3 gate information messages. When "-TYP/" is the last field, the
// message's 4-character checksum follows the type directly.
func TestParseAircraftType(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"four-character type followed by the checksum",
			"/CYVR.DC1/RCD 024\r\n804-CYVR-GATE Z-KSEA\r\nATIS Z\r\n-TYP/DH8DFB0F", "DH8D"},
		{"type followed by another field",
			"/LEVC.DC1/RCD 024\r\nLH22K-LEVC-GATE 2-EDDM\r\nATIS O\r\n-TYP/A320\r\n-RMK/6820", "A320"},
		{"three-character IATA type followed by the checksum",
			"/LSZH.DC1/RCD 000\r\nAEE5ZH-LSZH-GATE A04-LGAV\r\nATIS Y\r\n-TYP/32NC79F", "32N"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "B3", Text: tt.text})
			if got == nil {
				t.Fatal("Parse returned nil")
			}
			if r := got.(*Result); r.AircraftType != tt.want {
				t.Errorf("AircraftType = %q, want %q", r.AircraftType, tt.want)
			}
		})
	}
}
