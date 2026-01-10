package label39

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser(t *testing.T) {
	testCases := []struct {
		name string
		text string
		want struct {
			header      string
			flightLevel int
			originICAO  string
			destICAO    string
			eta         string
			fuel        int
			lat         float64
			lon         float64
		}
	}{
		{
			name: "CDC01 AFL1334 header",
			text: "CDC01AFL1334 / 181749UUEEULAA 174954 FOB 89 LATN 55.992 LONE 37.424",
			want: struct {
				header      string
				flightLevel int
				originICAO  string
				destICAO    string
				eta         string
				fuel        int
				lat         float64
				lon         float64
			}{
				header:      "CDC01",
				flightLevel: 1334,
				originICAO:  "UUEE",
				destICAO:    "ULAA",
				eta:         "17:49:54",
				fuel:        89,
				lat:         55.992,
				lon:         37.424,
			},
		},
		{
			name: "ADC01 AFL1334 header",
			text: "ADC01AFL1334 / 181750UUEEULAA 175006 FOB 89 LATN 55.992 LONE 37.424",
			want: struct {
				header      string
				flightLevel int
				originICAO  string
				destICAO    string
				eta         string
				fuel        int
				lat         float64
				lon         float64
			}{
				header:      "ADC01",
				flightLevel: 1334,
				originICAO:  "UUEE",
				destICAO:    "ULAA",
				eta:         "17:50:06",
				fuel:        89,
				lat:         55.992,
				lon:         37.424,
			},
		},
		{
			name: "ODO01 AFL1383 header",
			text: "ODO01AFL1383 /18190051USHHUUEE 005106 FOB 89 LATN 61.040 LONE 69.109",
			want: struct {
				header      string
				flightLevel int
				originICAO  string
				destICAO    string
				eta         string
				fuel        int
				lat         float64
				lon         float64
			}{
				header:      "ODO01",
				flightLevel: 1383,
				originICAO:  "USHH",
				destICAO:    "UUEE",
				eta:         "00:51:06",
				fuel:        89,
				lat:         61.040,
				lon:         69.109,
			},
		},
		{
			name: "PBS01 AFL1531 header",
			text: "PBS01AFL1531 /19190127UNTTUUEE 012700 FOB 184 LATN 56.392 LONE 85.228",
			want: struct {
				header      string
				flightLevel int
				originICAO  string
				destICAO    string
				eta         string
				fuel        int
				lat         float64
				lon         float64
			}{
				header:      "PBS01",
				flightLevel: 1531,
				originICAO:  "UNTT",
				destICAO:    "UUEE",
				eta:         "01:27:00",
				fuel:        184,
				lat:         56.392,
				lon:         85.228,
			},
		},
		{
			name: "LDC01 AFL1334 header",
			text: "LDC01AFL1334 / 181749UUEEULAA 174924 FOB 89 LATN 55.992 LONE 37.424",
			want: struct {
				header      string
				flightLevel int
				originICAO  string
				destICAO    string
				eta         string
				fuel        int
				lat         float64
				lon         float64
			}{
				header:      "LDC01",
				flightLevel: 1334,
				originICAO:  "UUEE",
				destICAO:    "ULAA",
				eta:         "17:49:24",
				fuel:        89,
				lat:         55.992,
				lon:         37.424,
			},
		},
	}

	parser := &Parser{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &acars.Message{
				ID:        12345,
				Timestamp: "2024-01-18T17:49:54Z",
				Text:      tc.text,
				Label:     "39",
			}

			if !parser.QuickCheck(msg.Text) {
				t.Errorf("QuickCheck failed for valid message")
			}

			rawResult := parser.Parse(msg)
			if rawResult == nil {
				t.Fatalf("Parse returned nil")
			}

			result, ok := rawResult.(*Result)
			if !ok {
				t.Fatalf("Parse result is not *Result type")
			}

			if result.Header != tc.want.header {
				t.Errorf("Header = %q, want %q", result.Header, tc.want.header)
			}
			if result.OriginICAO != tc.want.originICAO {
				t.Errorf("OriginICAO = %q, want %q", result.OriginICAO, tc.want.originICAO)
			}
			if result.DestICAO != tc.want.destICAO {
				t.Errorf("DestICAO = %q, want %q", result.DestICAO, tc.want.destICAO)
			}
			if result.ETA != tc.want.eta {
				t.Errorf("ETA = %q, want %q", result.ETA, tc.want.eta)
			}
			if result.FuelOnBoard != tc.want.fuel {
				t.Errorf("FuelOnBoard = %d, want %d", result.FuelOnBoard, tc.want.fuel)
			}
			if result.Latitude != tc.want.lat {
				t.Errorf("Latitude = %f, want %f", result.Latitude, tc.want.lat)
			}
			if result.Longitude != tc.want.lon {
				t.Errorf("Longitude = %f, want %f", result.Longitude, tc.want.lon)
			}
		})
	}
}
