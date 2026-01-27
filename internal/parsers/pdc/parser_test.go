package pdc

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser(t *testing.T) {
	testCases := []struct {
		name string
		text string
		want struct {
			flightNum    string
			origin       string
			destination  string
			runway       string
			sid          string
			squawk       string
			depFreq      string
			altitude     string
			aircraftType string
			atis         string
		}
	}{
		{
			name: "Jetstar YBBN to YMML PDC",
			text: `.MELOJJQ 301036
AGM
AN VH-OFW/MA 511A
-  /
PDC 301035
JST577 A21N YBBN 1120
CLEARED TO YMML VIA
SANEG TWO DEP
ROUTE:SANEG Q35 OSOTI Q35 PKS Q35 DORSU H119 ARBEY DCT
CLIMB VIA SID TO: 6000
DEP FREQ: 118.450
SQUAWK 1007
XXX EXPECT RUNWAY 01R XXX`,
			want: struct {
				flightNum    string
				origin       string
				destination  string
				runway       string
				sid          string
				squawk       string
				depFreq      string
				altitude     string
				aircraftType string
				atis         string
			}{
				flightNum:    "JST577",
				origin:       "YBBN",
				destination:  "YMML",
				runway:       "01R",
				sid:          "SANEG2",
				squawk:       "1007",
				depFreq:      "118.450",
				altitude:     "6000",
				aircraftType: "A21N",
				atis:         "",
			},
		},
		{
			name: "Edelweiss LSZH to EFIV PDC",
			text: `/GVACLXA.DC1/CLD 1042 251230 LSZH PDC 108
EDW308L CLRD TO EFIV OFF 16 VIA DEGES3S
ALT 5000 FT
SQUAWK 3016 ATIS Y
AIRBORNE FREQ 125.955 TSAT 1055
AT TOBT +OR- 5MIN AND BEFORE STARTING
DE-ICING, REPORT READY ON FREQ 121.9306DAA`,
			want: struct {
				flightNum    string
				origin       string
				destination  string
				runway       string
				sid          string
				squawk       string
				depFreq      string
				altitude     string
				aircraftType string
				atis         string
			}{
				flightNum:   "EDW308L",
				origin:      "LSZH",
				destination: "EFIV",
				runway:      "16",
				sid:         "DEGES3S",
				squawk:      "3016",
				depFreq:     "125.955",
				altitude:    "5000",
				atis:        "Y",
			},
		},
		{
			name: "Delta KPHL to KDTW PDC with DEPARTING format",
			text: `42 PDC 1540 PHL DTW
***DATE/TIME OF PDC RECEIPT: 30DEC 1032Z

**** PREDEPARTURE  CLEARANCE ****

DAL1540 DEPARTING KPHL  TRANSPONDER 2747
SKED DEP TIME 1100   EQUIP  B712/L
FILED FLT LEVEL 280`,
			want: struct {
				flightNum    string
				origin       string
				destination  string
				runway       string
				sid          string
				squawk       string
				depFreq      string
				altitude     string
				aircraftType string
				atis         string
			}{
				flightNum:    "DAL1540",
				origin:       "KPHL",
				destination:  "", // Only IATA DTW in header, no ICAO destination
				squawk:       "2747",
				aircraftType: "B712",
			},
		},
		{
			name: "US Regional PDC with route containing destination ICAO",
			text: `PDC
001
PDT5898 1772 KPHL
E145/L P1834
145 310
-DITCH T416 JIMEE-
KPHL DITCH LUIGI HNNAH CYUL`,
			want: struct {
				flightNum    string
				origin       string
				destination  string
				runway       string
				sid          string
				squawk       string
				depFreq      string
				altitude     string
				aircraftType string
				atis         string
			}{
				flightNum:    "PDT5898",
				origin:       "KPHL",
				destination:  "CYUL",
				squawk:       "1772",
				aircraftType: "E145",
				altitude:     "310",
			},
		},
		{
			name: "Canadian Jazz Aviation PDC",
			text: `.ANPOCQK 170457
AGM
AN CGGMZ
-  FLIGHT JZA810/17 CYVR
KSEA
PDC
JZA810 0031 CYVR
M/DH8D/R P0505
150
YVR MARNR MARNR8
EDCT
USE SID GRG7
DEPARTURE RUNWAY 26L
DESTINATION KSEA`,
			want: struct {
				flightNum    string
				origin       string
				destination  string
				runway       string
				sid          string
				squawk       string
				depFreq      string
				altitude     string
				aircraftType string
				atis         string
			}{
				flightNum:    "JZA810",
				origin:       "CYVR",
				destination:  "KSEA",
				runway:       "26L",
				sid:          "GRG7",
				squawk:       "0031",
				altitude:     "150",
				aircraftType: "DH8D",
			},
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

			pdc, ok := result.(*Result)
			if !ok {
				t.Fatalf("expected *Result, got %T", result)
			}

			if pdc.FlightNumber != tc.want.flightNum {
				t.Errorf("FlightNumber: got %q, want %q", pdc.FlightNumber, tc.want.flightNum)
			}

			if pdc.Origin != tc.want.origin {
				t.Errorf("Origin: got %q, want %q", pdc.Origin, tc.want.origin)
			}

			if pdc.Destination != tc.want.destination {
				t.Errorf("Destination: got %q, want %q", pdc.Destination, tc.want.destination)
			}

			if pdc.Runway != tc.want.runway {
				t.Errorf("Runway: got %q, want %q", pdc.Runway, tc.want.runway)
			}

			if pdc.SID != tc.want.sid {
				t.Errorf("SID: got %q, want %q", pdc.SID, tc.want.sid)
			}

			if pdc.Squawk != tc.want.squawk {
				t.Errorf("Squawk: got %q, want %q", pdc.Squawk, tc.want.squawk)
			}

			if pdc.DepartureFreq != tc.want.depFreq {
				t.Errorf("DepartureFreq: got %q, want %q", pdc.DepartureFreq, tc.want.depFreq)
			}

			if pdc.InitialAltitude != tc.want.altitude {
				t.Errorf("InitialAltitude: got %q, want %q", pdc.InitialAltitude, tc.want.altitude)
			}

			if pdc.AircraftType != tc.want.aircraftType {
				t.Errorf("AircraftType: got %q, want %q", pdc.AircraftType, tc.want.aircraftType)
			}

			if pdc.ATIS != tc.want.atis {
				t.Errorf("ATIS: got %q, want %q", pdc.ATIS, tc.want.atis)
			}
		})
	}
}

func TestRepublicAirways(t *testing.T) {
	text := `QUHDQDDRP~1PDC SEQ 001
RPA4783
DEP/KDCA
SKD/1229Z
FL360
-REBLL5 OTTTO Q80 DEWAK GROAT
PASLY5 KBNA-
KDCA REBLL5 OTTTO Q80./.KBNA


CLEARED REBLL5 DEPARTURE OTTTO TRSN

CLIMB VIA SID
EXP 360 10 MIN AFT DP,DPFRQ SEE SID



SQUAWK/7050
END`

	compiler := NewCompiler()
	if err := compiler.Compile(); err != nil {
		t.Fatalf("Compile error: %v", err)
	}

	result := compiler.Parse(text)
	if result == nil {
		t.Fatal("Expected match, got nil")
	}

	t.Logf("Format: %s", result.FormatName)
	t.Logf("Flight: %s", result.FlightNumber)
	t.Logf("Origin: %s", result.Origin)
	t.Logf("Squawk: %s", result.Squawk)

	if result.FlightNumber != "RPA4783" {
		t.Errorf("FlightNumber: got %q, want %q", result.FlightNumber, "RPA4783")
	}
	if result.Origin != "KDCA" {
		t.Errorf("Origin: got %q, want %q", result.Origin, "KDCA")
	}
}

func TestQuickCheck(t *testing.T) {
	p := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"PDC 301035", true},
		{"LSZH PDC 108", true},
		{"CLEARED TO YMML VIA", false}, // No PDC - might be oceanic clearance
		{"CLRD TO YSSY", false},         // No PDC - might be oceanic clearance
		{"CLRNCE 983", false},           // Oceanic clearance number, not PDC
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

	msg := &acars.Message{Text: "", ID: 1}
	if result := p.Parse(msg); result != nil {
		t.Errorf("expected nil for empty text, got %+v", result)
	}
}