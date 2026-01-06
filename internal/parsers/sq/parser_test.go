package sq

import (
	"math"
	"testing"

	"acars_parser/internal/acars"
)

func TestParser(t *testing.T) {
	testCases := []struct {
		name string
		text string
		want struct {
			iata, icao string
			lat, lon   float64
			freq       float64
		}
	}{
		{
			name: "KORD Chicago",
			text: "02XAORDKORD04158N08754WV136975/ARINC",
			want: struct{ iata, icao string; lat, lon, freq float64 }{"ORD", "KORD", 41.9667, -87.9, 136.975},
		},
		{
			name: "YSSY Sydney",
			text: "02XSSYDYSSY03357S15111EV136975/",
			want: struct{ iata, icao string; lat, lon, freq float64 }{"SYD", "YSSY", -33.95, 151.1833, 136.975},
		},
		{
			name: "LBSF Sofia",
			text: "02XASOFLBSF14242N02324EB136975/ARINC",
			want: struct{ iata, icao string; lat, lon, freq float64 }{"SOF", "LBSF", 42.7, 23.4, 136.975},
		},
		{
			name: "PAFA Fairbanks",
			text: "02XAFAIPAFA16449N14752WV136975/ARINC",
			want: struct{ iata, icao string; lat, lon, freq float64 }{"FAI", "PAFA", 64.8167, -147.8667, 136.975},
		},
		{
			name: "KHNL Honolulu",
			text: "02XAHNLKHNL22119N15755WV136975/ARINC",
			want: struct{ iata, icao string; lat, lon, freq float64 }{"HNL", "KHNL", 21.3167, -157.9167, 136.975},
		},
	}

	p := &Parser{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &acars.Message{
				Text: tc.text,
				ID:   1,
			}

			result := p.Parse(msg)
			if result == nil {
				t.Fatalf("expected result, got nil")
			}

			sq, ok := result.(*Result)
			if !ok {
				t.Fatalf("expected *Result, got %T", result)
			}

			if sq.IATACode != tc.want.iata {
				t.Errorf("IATA: got %q, want %q", sq.IATACode, tc.want.iata)
			}

			if sq.ICAOCode != tc.want.icao {
				t.Errorf("ICAO: got %q, want %q", sq.ICAOCode, tc.want.icao)
			}

			if math.Abs(sq.Latitude-tc.want.lat) > 0.1 {
				t.Errorf("Latitude: got %.4f, want %.4f", sq.Latitude, tc.want.lat)
			}

			if math.Abs(sq.Longitude-tc.want.lon) > 0.1 {
				t.Errorf("Longitude: got %.4f, want %.4f", sq.Longitude, tc.want.lon)
			}

			if math.Abs(sq.FreqMHz-tc.want.freq) > 0.001 {
				t.Errorf("Frequency: got %.3f, want %.3f", sq.FreqMHz, tc.want.freq)
			}
		})
	}
}

func TestQuickCheck(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"02XAORDKORD04158N08754WV136975/ARINC", true},
		{"02XSSYDYSSY03357S15111EV136975/", true},
		{"00XS", false}, // This is a short squitter with no position
		{"FPN/FNQFA123", false},
		{"", false},
	}

	for _, tt := range tests {
		got := p.QuickCheck(tt.text)
		if got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestRejectsInvalid(t *testing.T) {
	p := &Parser{}

	invalid := []string{
		"00XS",         // Short squitter
		"FPN/FNQFA123", // Not SQ format
		"02XAORD",      // Too short
		"",             // Empty
		"GARBAGE TEXT", // Random text
	}

	for _, text := range invalid {
		msg := &acars.Message{Text: text, ID: 1}
		if result := p.Parse(msg); result != nil {
			t.Errorf("expected nil for %q, got %+v", text, result)
		}
	}
}
