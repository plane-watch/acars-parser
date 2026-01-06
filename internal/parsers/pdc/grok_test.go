package pdc

import (
	"testing"
)

func TestCompiler(t *testing.T) {
	c := NewCompiler()
	if err := c.Compile(); err != nil {
		t.Fatalf("failed to compile patterns: %v", err)
	}

	tests := []struct {
		name         string
		text         string
		wantFormat   string
		wantFlight   string
		wantOrigin   string
		wantDest     string
		wantAircraft string
		wantRunway   string
		wantSID      string
		wantSquawk   string
		wantRoute    string
	}{
		{
			name: "Australian Jetstar YSSY to YMML",
			text: `PDC 291826
JST501 A320 YSSY 1900
CLEARED TO YMML VIA
16L ABBEY3 DEP: XXX
ROUTE:DCT WOL H65 LEECE Q29 BOOIN DCT
CLIMB VIA SID TO: 5000
DEP FREQ: 129.700
SQUAWK 3670`,
			wantFormat:   "australian",
			wantFlight:   "JST501",
			wantOrigin:   "YSSY",
			wantDest:     "YMML",
			wantAircraft: "A320",
			wantRunway:   "16L",
			wantSID:      "ABBEY3",
			wantSquawk:   "3670",
			wantRoute:    "DCT WOL H65 LEECE Q29 BOOIN DCT",
		},
		{
			name: "Australian Jetstar truncated SID",
			text: `PDC 291826
JST400 A320 YSSY 1910
CLEARED TO YBCG VIA
16LKEVIN7 DEP OLSEM:TRAN
ROUTE:DCT OLSEM Y193 BANDA Y43 BERNI DCT
CLIMB VIA SID TO: 5000
DEP FREQ: 123.000
SQUAWK 1234`,
			wantFormat:  "australian",
			wantFlight:  "JST400",
			wantOrigin:  "YSSY",
			wantDest:    "YBCG",
			wantAircraft: "A320",
			wantRunway:  "16L",
			wantSID:     "KEVIN7",
			wantSquawk:  "1234",
		},
		{
			name: "DC1 Geneva (Swiss) format",
			text: `/GVACLXA.DC1/CLD 1851 251229 LSZH PDC 274
EIN34Y CLRD TO EIDW OFF 28 VIA VEBIT4W
ALT 5000 FT
SQUAWK 3041 ATIS R
AIRBORNE FREQ 125.955`,
			wantFormat: "dc1_clearance",
			wantFlight: "EIN34Y",
			wantOrigin: "LSZH",
			wantDest:   "EIDW",
			wantRunway: "28",
			wantSID:    "VEBIT4W",
			wantSquawk: "3041",
		},
		{
			name: "DC1 Helsinki format",
			text: `/HELCLXA.DC1/CLD 1849 251229 EFHK PDC 729
FIN609 CLRD TO EFIV OFF 04R VIA TEVRU5C
SQUAWK 1216 NEXT FREQ 121.800
QNH 992
TSAT 1910
CLIMB TO 4000 FT`,
			wantFormat: "dc1_clearance",
			wantFlight: "FIN609",
			wantOrigin: "EFHK",
			wantDest:   "EFIV",
			wantRunway: "04R",
			wantSID:    "TEVRU5C",
			wantSquawk: "1216",
		},
		{
			name: "US Delta format",
			text: `42 PDC 1260 MSP HDN
***DATE/TIME OF PDC RECEIPT: 29DEC 1827Z

**** PREDEPARTURE  CLEARANCE ****

DAL1260 DEPARTING KMSP  TRANSPONDER 2463
SKED DEP TIME 1857   EQUIP  A319/L
FILED FLT LEVEL 360`,
			wantFormat:  "us_delta",
			wantFlight:  "DAL1260",
			wantOrigin:  "KMSP",
			wantSquawk:  "2463",
			wantAircraft: "A319",
		},
		{
			name: "DC1 Warsaw format",
			text: `/WAWDLYA.DC1/CLD 1840 251229 EPWA
PDC 002
LOT3859 CLRD TO EPWR
OFF 29 VIA SOXER7G INITIAL
CLIMB ALTITUDE 6000 FEET
SQUAWK 1000 ATIS T`,
			wantFormat: "dc1_clearance",
			wantFlight: "LOT3859",
			wantOrigin: "EPWA",
			wantDest:   "EPWR",
			wantRunway: "29",
			wantSID:    "SOXER7G",
			wantSquawk: "1000",
		},
		{
			name: "Australian Virgin PDC UPLINK",
			text: `PDC UPLINK
VOZ1528 B738 YSSY 2010
CLEARED TO YMHB VIA
34R MARUB6 DEP WOL:TRAN
ROUTE:DCT WOL H20 MOTRA W407 IPLET DCT
CLIMB VIA SID TO: 5000
DEP FREQ: 129.700
SQUAWK 4045`,
			wantFormat:   "australian",
			wantFlight:   "VOZ1528",
			wantOrigin:   "YSSY",
			wantDest:     "YMHB",
			wantAircraft: "B738",
			wantRunway:   "34R",
			wantSID:      "MARUB6",
			wantSquawk:   "4045",
			wantRoute:    "DCT WOL H20 MOTRA W407 IPLET DCT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Parse(tt.text)
			if result == nil {
				t.Fatalf("expected result, got nil")
			}

			if result.FormatName != tt.wantFormat {
				t.Errorf("Format: got %q, want %q", result.FormatName, tt.wantFormat)
			}
			if result.FlightNumber != tt.wantFlight {
				t.Errorf("Flight: got %q, want %q", result.FlightNumber, tt.wantFlight)
			}
			if tt.wantOrigin != "" && result.Origin != tt.wantOrigin {
				t.Errorf("Origin: got %q, want %q", result.Origin, tt.wantOrigin)
			}
			if tt.wantDest != "" && result.Destination != tt.wantDest {
				t.Errorf("Destination: got %q, want %q", result.Destination, tt.wantDest)
			}
			if tt.wantAircraft != "" && result.Aircraft != tt.wantAircraft {
				t.Errorf("Aircraft: got %q, want %q", result.Aircraft, tt.wantAircraft)
			}
			if tt.wantRunway != "" && result.Runway != tt.wantRunway {
				t.Errorf("Runway: got %q, want %q", result.Runway, tt.wantRunway)
			}
			if tt.wantSID != "" && result.SID != tt.wantSID {
				t.Errorf("SID: got %q, want %q", result.SID, tt.wantSID)
			}
			if tt.wantSquawk != "" && result.Squawk != tt.wantSquawk {
				t.Errorf("Squawk: got %q, want %q", result.Squawk, tt.wantSquawk)
			}
			if tt.wantRoute != "" && result.Route != tt.wantRoute {
				t.Errorf("Route: got %q, want %q", result.Route, tt.wantRoute)
			}
		})
	}
}

func TestRejectsNonPDC(t *testing.T) {
	c := NewCompiler()
	if err := c.Compile(); err != nil {
		t.Fatalf("failed to compile patterns: %v", err)
	}

	// Messages that should NOT match.
	nonPDC := []string{
		// "NO DEPARTURE CLEARANCE" message.
		`.ANPOCQK 291847
AGM
AN CGPJZ
-  NO DEPARTURE CLEARANCE
MESSAGE ON FILE CONTACT
CLEARANCE DELIVERY VIA
VOICE REQUEST FULL ROUTE`,
		// Empty string.
		"",
		// Random ACARS message.
		"FPN/FNQFA123 ROUTE DATA",
	}

	for _, text := range nonPDC {
		result := c.Parse(text)
		if result != nil {
			t.Errorf("expected nil for non-PDC message, got format %q", result.FormatName)
		}
	}
}