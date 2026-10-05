package h1

import (
	"testing"

	"acars_parser/internal/acars"
)

// The reports are real H1 PRG progress reports from the archive, except
// the last, which changes one to have a route that disagrees with DT.
func TestProgressParser(t *testing.T) {
	tests := []struct {
		name string
		text string
		want *ProgressResult
	}{
		{
			name: "destination, callsign and time stamp",
			text: "PRG/DTEHAM,18R,207,085235,041/FNDAL162/TS080747,090223AE30",
			want: &ProgressResult{Destination: "EHAM", Flight: "DAL162", ReportTime: "080747"},
		},
		{
			name: "callsign last, before the checksum",
			text: "PRG/TS093622,041026/DTEGCC,23R,218,110622,043/FNUAE34YB474",
			want: &ProgressResult{Destination: "EGCC", Flight: "UAE34Y", ReportTime: "093622"},
		},
		{
			name: "callsign first",
			text: "PRG/FNSCX3046/DTKLAL,10O,116,084840,295D18",
			want: &ProgressResult{Destination: "KLAL", Flight: "SCX3046"},
		},
		{
			name: "route from the RP section",
			text: "PRG/DTLEMG,12O,63,092914/PR1380,321,370,88,,0,0,,,70,,,P15,M6,40270,,1292,322/RP:DA:LSGG:AA:LEMG:A:BLN1A:AP:ILSZ12.LENHI(12O)35F0",
			want: &ProgressResult{Origin: "LSGG", Destination: "LEMG"},
		},
		{
			name: "Southwest LR layout",
			text: "PRG/LR,034035,SWA2568,KATL,KTPA,19L,25,1241,1141,100,1239,P25,124013,8,140K,D688,034128,2,29278C",
			want: &ProgressResult{Origin: "KATL", Destination: "KTPA", Flight: "SWA2568", ReportTime: "034035"},
		},
		{
			name: "RP arrival differs from DT",
			text: "PRG/DTLEMG,12O,63,092914/RP:DA:LSGG:AA:LEMD:A:BLN1A35F0",
			want: &ProgressResult{Destination: "LEMG"},
		},
		{
			// The last four characters are not hex, so there is no checksum.
			name: "last callsign without a checksum",
			text: "PRG/TS093622,041026/DTEGCC,23R,218,110622,043/FNUAL1234ZZ",
			want: &ProgressResult{Destination: "EGCC", Flight: "UAL1234ZZ", ReportTime: "093622"},
		},
		{
			// "UAL1234AB" and, without a checksum "34AB", "UAL12" are both
			// callsigns, so the flight is not reported.
			name: "ambiguous last callsign",
			text: "PRG/TS093622,041026/DTEGCC,23R,218,110622,043/FNUAL1234AB",
			want: &ProgressResult{Destination: "EGCC", ReportTime: "093622"},
		},
		{
			name: "seven digits before the checksum",
			text: "PRG/TS001457,110126/DTPHNL,26L,230,002215,046/FNUAL2195868",
			want: &ProgressResult{Destination: "PHNL", Flight: "UAL219", ReportTime: "001457"},
		},
		{name: "no destination", text: "PRG/HC,390,,TEESY,KLGA,317,314112"},
		{name: "not a progress report", text: "POSN39006W075260,OYVAY,014938,340"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&ProgressParser{}).Parse(&acars.Message{ID: 3, Label: "H1", Text: tt.text})
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			r, ok := got.(*ProgressResult)
			if !ok {
				t.Fatalf("got %T, want *ProgressResult", got)
			}
			tt.want.MsgID = 3
			if *r != *tt.want {
				t.Errorf("got  %+v\nwant %+v", *r, *tt.want)
			}
		})
	}
}
