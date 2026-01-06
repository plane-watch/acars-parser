package atis

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestATISParser(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantAirport string
		wantLetter  string
		wantQNH     string
		wantRunways []string
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
			wantAirport: "VHHH",
			wantLetter:  "G",
			wantQNH:     "1015",
			wantRunways: []string{"07C", "07R"},
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
			wantQNH:     "1029",
			wantRunways: []string{"34L", "33L", "33R"},
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
			if r.QNH != tt.wantQNH {
				t.Errorf("QNH = %q, want %q", r.QNH, tt.wantQNH)
			}
			if len(r.Runways) != len(tt.wantRunways) {
				t.Errorf("Runways = %v, want %v", r.Runways, tt.wantRunways)
			}
		})
	}
}
