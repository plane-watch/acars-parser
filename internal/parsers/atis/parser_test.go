package atis

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestATISParser(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		wantAirport    string
		wantLetter     string
		wantType       string
		wantQNH        string
		wantTemp       string
		wantWind       string
		wantRunways    []string
		wantApproaches []string
	}{
		{
			name: "Hong Kong arrival",
			text: `/HKGATYA.TI2/VHHH ARR ATIS G
	1806Z
	ARRIVALS, RWY 07C.
	EXP ILS  APCH, RWY 07C.
	RWY 07R IS CLSD FOR
	MAINT. WIND 100/09KT
	VIS 10KM CLD FEW 2000FT
	T18 DP14 QNH 1015HPA=
	ACKNOWLEDGE INFO G ON
	FIRST CTC WITH APP.FE6F`,
			wantAirport:    "VHHH",
			wantLetter:     "G",
			wantType:       "ARR",
			wantQNH:        "1015",
			wantTemp:       "18",
			wantRunways:    []string{"07C", "07R"},
			wantApproaches: []string{"ILS  APCH"},
		},
		{
			name: "Incheon arrival",
			text: `/ICNDLXA.TI2/RKSI ARR ATIS W
	1800Z
	EXP ILS APCH RWY 34L
	WIND 360/15KT
	CAVOK
	T MS 8
	DP MS 17
	QNH 1029
	RWY 33L UNUSABLE DUE TO WORK IN PROGRESS
	RWY 33R UNUSABLE DUE TO WORK IN PROGRESS
	CAUTION BIRD ACTIVITY`,
			wantAirport: "RKSI",
			wantLetter:  "W",
			wantType:    "ARR",
			wantQNH:     "1029",
			wantTemp:    "-8",
			wantRunways: []string{"34L", "33L", "33R"},
		},
		{
			name: "Perth ENR ATIS with WND/TMP format",
			text: `/ATISAXS.TI2/YPPH ENR ATIS I
1428Z
ATIS YPPH I   311428
   APCH: EXP RNP APCH
   RWY: 06
+ WND: 100/25-35, MAX XW 25 KTS
   WX: CAVOK
+ TMP: 24
   QNH: 1016
   SIGWX: SIGMET NOVEMBER 0 1 VALID,
SEV TURB FCST BLW 3000 FT
END OF ATIS I`,
			wantAirport:    "YPPH",
			wantLetter:     "I",
			wantType:       "ENR",
			wantQNH:        "1016",
			wantTemp:       "24",
			wantWind:       "100/25-35, MAX XW 25 KTS",
			wantRunways:    []string{"06"},
			wantApproaches: []string{"RNP APCH"},
		},
		{
			name: "Perth DEP ATIS with RNP approach",
			text: `/ATSEKXA.TI2/YPPH DEP ATIS H
1312Z ATIS YPPH H   311312
+ APCH: EXP RNP APCH
+ RWY: 06
+ WND: 110/15-25, MAX XW 25 KTS
   WX: CAVOK
+ TMP: 26
   QNH: 1016
   SIGWX: SIGMET NOVEMBER 0 1 VALID,
SEV TURB FCST BLW 3000 FT`,
			wantAirport:    "YPPH",
			wantLetter:     "H",
			wantType:       "DEP",
			wantQNH:        "1016",
			wantTemp:       "26",
			wantWind:       "110/15-25, MAX XW 25 KTS",
			wantRunways:    []string{"06"},
			wantApproaches: []string{"RNP APCH"},
		},
	}

	p := &Parser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &acars.Message{
				Label: "A9",
				Text:  tt.text,
			}

			if !p.QuickCheck(tt.text) {
				t.Errorf("QuickCheck failed")
				return
			}

			result := p.Parse(msg)
			if result == nil {
				t.Errorf("Parse returned nil")
				return
			}

			r, ok := result.(*Result)
			if !ok {
				t.Errorf("Result is not *Result type")
				return
			}

			if r.Airport != tt.wantAirport {
				t.Errorf("Airport = %q, want %q", r.Airport, tt.wantAirport)
			}
			if r.ATISLetter != tt.wantLetter {
				t.Errorf("ATISLetter = %q, want %q", r.ATISLetter, tt.wantLetter)
			}
			if tt.wantType != "" && r.ATISType != tt.wantType {
				t.Errorf("ATISType = %q, want %q", r.ATISType, tt.wantType)
			}
			if r.QNH != tt.wantQNH {
				t.Errorf("QNH = %q, want %q", r.QNH, tt.wantQNH)
			}
			if tt.wantTemp != "" && r.Temperature != tt.wantTemp {
				t.Errorf("Temperature = %q, want %q", r.Temperature, tt.wantTemp)
			}
			if tt.wantWind != "" && r.Wind != tt.wantWind {
				t.Errorf("Wind = %q, want %q", r.Wind, tt.wantWind)
			}
			if len(r.Runways) != len(tt.wantRunways) {
				t.Errorf("Runways = %v, want %v", r.Runways, tt.wantRunways)
			}
			if len(tt.wantApproaches) > 0 {
				if len(r.Approaches) != len(tt.wantApproaches) {
					t.Errorf("Approaches = %v, want %v", r.Approaches, tt.wantApproaches)
				}
			}
		})
	}
}
