package acmsreport

import (
	"testing"

	"acars_parser/internal/acars"
)

// The reports are real Airbus ACMS reports (label H1) from the January 2026
// corpus, shortened after the C1 block, with the tail as transmitted; the
// 9H-WDJ report is from live traffic (October 2026).
func TestParse(t *testing.T) {
	tests := []struct {
		name string
		tail string
		text string
		want Result
	}{
		{
			name: "report on the day it was sent",
			tail: "VH-VWT",
			text: "A321,014057,1,1,TB000000/REP001,00,00,1/CCVH-VWT,JAN20,040543,YSSY,YBBN,0816/C0TIA05JST4R0000/C106,61902,5000,52,0010,0,0100,52,X",
			want: Result{AircraftSeries: "A321", Report: "001", Registration: "VH-VWT", ReportDate: "JAN20",
				ReportTime: "040543", Origin: "YSSY", Destination: "YBBN", FlightNumberDigits: "0816"},
		},
		{
			name: "A319 report",
			tail: "OE-LQC",
			text: "A319,015848,1,1,TB000000/REP002,81,01,1/CCOE-LQC,DEC31,174739,LIRF,EBBR,0085/C0TCF091U3070000/C106,06102,5000,42,0010,0,0100,42,X",
			want: Result{AircraftSeries: "A319", Report: "002", Registration: "OE-LQC", ReportDate: "DEC31",
				ReportTime: "174739", Origin: "LIRF", Destination: "EBBR", FlightNumberDigits: "0085"},
		},
		{
			name: "registration with a leading dot",
			tail: "N303NY",
			text: "A321,001648,1,1,TB000000/REP035,90,01,4/CC.N303NY,JAN12,061413,KJFK,KLAX,0117/C08LM3068ACZ4FH8M3SH0200ACZ44H8M3XH0200",
			want: Result{AircraftSeries: "A321", Report: "035", Registration: "N303NY", ReportDate: "JAN12",
				ReportTime: "061413", Origin: "KJFK", Destination: "KLAX", FlightNumberDigits: "0117"},
		},
		{
			name: "live report",
			tail: "9H-WDJ",
			text: "A321,026596,1,1,TB000000/REP004,00,00,1/CC9H-WDJ,OCT04,084040,EGGW,LBSF,0219/C0TWP02TWZZ20529/C105,51426,4000,52,0010,0,0100,52,X",
			want: Result{AircraftSeries: "A321", Report: "004", Registration: "9H-WDJ", ReportDate: "OCT04",
				ReportTime: "084040", Origin: "EGGW", Destination: "LBSF", FlightNumberDigits: "0219"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: tt.tail, Text: tt.text}).(*Result)
			if !ok {
				t.Fatalf("Parse returned no result")
			}
			got.MsgID, got.Timestamp = 0, ""
			if *got != tt.want {
				t.Errorf("Parse\n got %+v\nwant %+v", *got, tt.want)
			}
		})
	}
}

// The registration is reported only when it is the transmitted tail.
func TestParseDoesNotReportAnotherRegistration(t *testing.T) {
	for _, tail := range []string{"VH-VWU", ""} {
		msg := &acars.Message{Label: "H1", Tail: tail,
			Text: "A321,014057,1,1,TB000000/REP001,00,00,1/CCVH-VWT,JAN20,040543,YSSY,YBBN,0816/C0TIA05JST4R0000"}
		got, ok := (&Parser{}).Parse(msg).(*Result)
		if !ok {
			t.Fatalf("tail %q: Parse returned no result", tail)
		}
		if got.Registration != "" {
			t.Errorf("tail %q: Registration = %q, want none", tail, got.Registration)
		}
	}
}

func TestParseRejectsOtherText(t *testing.T) {
	for _, text := range []string{
		// No CC block.
		"A321,002244,1,1,TB000000/REP239,00,00,4/239N537DT0473011726093629777N44886W",
		// A CC block with blanked airports.
		"A321,020701,1,1,TB000000/REP019,84,01,4/CCN589DT,JAN19,212615,'''','''',0820/C0TWP03005030001",
		// Not an ACMS header.
		"A321 IS AN AIRCRAFT",
		"RTE 1 06JAN26 0034 N37319 UAL2443 KORD/KJAX",
	} {
		if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "N589DT", Text: text}); r != nil {
			t.Errorf("Parse(%q) = %+v, want nil", text, r)
		}
	}
}
