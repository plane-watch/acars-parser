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
		wantAltM   int
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
			wantAltM:   6622,
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
			wantAltM:   11040,
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
			wantAltM:   11579,
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
			wantAltM:   11270,
		},
		{
			name:       "POS02 with positive temperature",
			text:       "POS02AFL320 /12121212EGGWLFPG FUEL 89 TEMP 15 WDIR18045 WSPD 12 LATN 51.234 LONE 00.567 ETA1345 ROMEO ALT 32000",
			wantFormat: "POS02",
			wantFL:     320,
			wantOrigin: "EGGW",
			wantDest:   "LFPG",
			wantFuel:   89,
			wantTemp:   15,
			wantWDir:   180,
			wantWSpd:   12,
			wantLat:    51.234,
			wantLon:    0.567,
			wantETA:    "13:45",
			wantWpt:    "ROMEO",
			wantAltM:   9753,
		},
		{
			name:       "POS01 with southern latitude and western longitude",
			text:       "POS01AFL410 /09090909SBGRSCEL FUEL 120 TEMP-48 WDIR27090 WSPD 65 LATS 23.456 LONW 046.789 ETA1030 ALFA ALT 41000",
			wantFormat: "POS01",
			wantFL:     410,
			wantOrigin: "SBGR",
			wantDest:   "SCEL",
			wantFuel:   120,
			wantTemp:   -48,
			wantWDir:   270,
			wantWSpd:   65,
			wantLat:    -23.456,
			wantLon:    -46.789,
			wantETA:    "10:30",
			wantWpt:    "ALFA",
			wantAltM:   12496,
		},
		{
			name:       "POS03 with minimal data",
			text:       "POS03AFL250 /06060606KSFOKLAX",
			wantFormat: "POS03",
			wantFL:     250,
			wantOrigin: "KSFO",
			wantDest:   "KLAX",
		},
		{
			name:       "POS01 with zero fuel",
			text:       "POS01BA1234 /14141414EGLLLFPO FUEL 0 TEMP-10 WDIR09015 WSPD 20 LATN 50.123 LONE 001.234 ETA1530 ALT 28000",
			wantFormat: "POS01",
			wantFlight: "BA1234",
			wantFL:     280,
			wantOrigin: "EGLL",
			wantDest:   "LFPO",
			wantFuel:   0,
			wantTemp:   -10,
			wantWDir:   90,
			wantWSpd:   20,
			wantLat:    50.123,
			wantLon:    1.234,
			wantETA:    "15:30",
			wantAltM:   8534,
		},
		{
			name:       "POS01 Real world example USSS-ULLI",
			text:       "POS01AFL2827 /18190121USSSULLI FUEL 122 TEMP- 13 WDIR26384 WSPD 20 LATN 56.832 LONE 60.195 ETA0359 TUR ALT 11741",
			wantFormat: "POS01",
			wantFL:     117,
			wantOrigin: "USSS",
			wantDest:   "ULLI",
			wantFuel:   122,
			wantTemp:   -13,
			wantWDir:   263,
			wantWSpd:   20,
			wantLat:    56.832,
			wantLon:    60.195,
			wantETA:    "03:59",
			wantWpt:    "",
			wantAltM:   3578,
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

			if r.AltitudeM != tt.wantAltM {
				t.Errorf("AltitudeM = %d, want %d", r.AltitudeM, tt.wantAltM)
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
		{"POS03BA1234", true},
		{"POS10AFL100", true},
		{"Random message", false},
		{"POSITION REPORT", false},
		{"ETA01AFL1866", false},
		{"", false},
	}

	for _, tt := range tests {
		got := p.QuickCheck(tt.text)
		if got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}
