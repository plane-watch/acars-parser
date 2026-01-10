package label27

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestLabel27Parser(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantFormat string
		wantFlight string
		wantFL     int
		wantOrigin string
		wantDest   string
		wantFuel   int
		wantTemp   int
		wantWDir   int
		wantWSpd   int
		wantLat    float64
		wantLon    float64
		wantETA    string
		wantWpt    string
		wantAlt    int
	}{
		{
			name:       "POS01 with AFL flight level format",
			text:       "POS01AFL1866 /16180720UUEEUDYZ FUEL 140 TEMP- 32 WDIR26631 WSPD 36 LATN 55.164 LONE 38.545 ETA1013 TUR ALT 21728",
			wantFormat: "POS01",
			wantFL:     217,
			wantOrigin: "UUEE",
			wantDest:   "UDYZ",
			wantFuel:   140,
			wantTemp:   -32,
			wantWDir:   266,
			wantWSpd:   36,
			wantLat:    55.164,
			wantLon:    38.545,
			wantETA:    "10:13",
			wantWpt:    "",
			wantAlt:    21728,
		},
		{
			name:       "POS01 with flight number format (SU0245)",
			text:       "POS01SU0245 /18181727FSIAUUEE FUEL 14 TEMP-55 WDIR25381 WSPD53 LATN 54.567 LONE 38.387 ETA1813 TUR ALT 36221",
			wantFormat: "POS01",
			wantFlight: "SU0245",
			wantFL:     362,
			wantOrigin: "FSIA",
			wantDest:   "UUEE",
			wantFuel:   14,
			wantTemp:   -55,
			wantWDir:   253,
			wantWSpd:   53,
			wantLat:    54.567,
			wantLon:    38.387,
			wantETA:    "18:13",
			wantWpt:    "",
			wantAlt:    36221,
		},
		{
			name:       "POS01 with no spaces in LAT/LON format",
			text:       "POS01AFL637 /17171847VTSPUNNT FUEL 145 TEMP- 55 WDIR34204 WSPD 27 LATN51.595 LONE089.709 ETA1957 TUR ALT 37992",
			wantFormat: "POS01",
			wantFL:     379,
			wantOrigin: "VTSP",
			wantDest:   "UNNT",
			wantFuel:   145,
			wantTemp:   -55,
			wantWDir:   342,
			wantWSpd:   27,
			wantLat:    51.595,
			wantLon:    89.709,
			wantETA:    "19:57",
			wantWpt:    "",
			wantAlt:    37992,
		},
		{
			name:       "POS01 with 3-letter airline code (SDM6599)",
			text:       "POS01SDM6599 /18181749ULLIUWKD FUEL 66 TEMP- 56 WDIR27582 WSPD 46 LATN 57.034 LONE 43.416 ETA1832 TUR ALT 36977",
			wantFormat: "POS01",
			wantFlight: "SDM6599",
			wantFL:     369,
			wantOrigin: "ULLI",
			wantDest:   "UWKD",
			wantFuel:   66,
			wantTemp:   -56,
			wantWDir:   275,
			wantWSpd:   46,
			wantLat:    57.034,
			wantLon:    43.416,
			wantETA:    "18:32",
			wantWpt:    "",
			wantAlt:    36977,
		},
	}

	p := &Parser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &acars.Message{
				Label: "27",
				Text:  tt.text,
			}

			result := p.Parse(msg)
			if result == nil {
				t.Fatalf("Parse returned nil")
			}

			r, ok := result.(*Result)
			if !ok {
				t.Fatalf("Result type is not *label27.Result")
			}

			if r.Format != tt.wantFormat {
				t.Errorf("Format = %q, want %q", r.Format, tt.wantFormat)
			}

			if tt.wantFlight != "" && r.FlightNum != tt.wantFlight {
				t.Errorf("FlightNum = %q, want %q", r.FlightNum, tt.wantFlight)
			}

			if tt.wantFL != 0 && r.FlightLevel != tt.wantFL {
				t.Errorf("FlightLevel = %d, want %d", r.FlightLevel, tt.wantFL)
			}

			if r.OriginICAO != tt.wantOrigin {
				t.Errorf("OriginICAO = %q, want %q", r.OriginICAO, tt.wantOrigin)
			}

			if r.DestICAO != tt.wantDest {
				t.Errorf("DestICAO = %q, want %q", r.DestICAO, tt.wantDest)
			}

			if r.FuelOnBoard != tt.wantFuel {
				t.Errorf("FuelOnBoard = %d, want %d", r.FuelOnBoard, tt.wantFuel)
			}

			if r.Temperature != tt.wantTemp {
				t.Errorf("Temperature = %d, want %d", r.Temperature, tt.wantTemp)
			}

			if r.WindDir != tt.wantWDir {
				t.Errorf("WindDir = %d, want %d", r.WindDir, tt.wantWDir)
			}

			if r.WindSpeed != tt.wantWSpd {
				t.Errorf("WindSpeed = %d, want %d", r.WindSpeed, tt.wantWSpd)
			}

			if r.Latitude != tt.wantLat {
				t.Errorf("Latitude = %f, want %f", r.Latitude, tt.wantLat)
			}

			if r.Longitude != tt.wantLon {
				t.Errorf("Longitude = %f, want %f", r.Longitude, tt.wantLon)
			}

			if r.ETA != tt.wantETA {
				t.Errorf("ETA = %q, want %q", r.ETA, tt.wantETA)
			}

			if r.Waypoint != tt.wantWpt {
				t.Errorf("Waypoint = %q, want %q", r.Waypoint, tt.wantWpt)
			}

			if r.Altitude != tt.wantAlt {
				t.Errorf("Altitude = %d, want %d", r.Altitude, tt.wantAlt)
			}
		})
	}
}

func TestLabel27QuickCheck(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"POS01AFL1866 /16180720UUEEUDYZ", true},
		{"POS02AFL2000", true},
		{"Random message", false},
		{"POSITION REPORT", false},
	}

	for _, tt := range tests {
		got := p.QuickCheck(tt.text)
		if got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}
