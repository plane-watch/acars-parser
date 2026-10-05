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
		{
			// Reports 032 and 037 corrupt the destination ("KLAA" for KLAS),
			// so it is not reported.
			name: "C1 block of report 037",
			tail: "N74532",
			text: "A321,039021,1,1,TB000000/REP037,00,00,4/C1N74532,OCT02,042921,KDEN,KLAA,1460/C29999,W553,06,801518,801524",
			want: Result{AircraftSeries: "A321", Report: "037", Registration: "N74532", ReportDate: "OCT02",
				ReportTime: "042921", Origin: "KDEN", FlightNumberDigits: "1460"},
		},
		{
			name: "blanked time in report 037",
			tail: "N491UA",
			text: "A320,002230,1,1,TB000000/REP037,00,00,4/C1N491UA,OCT04,XXXXXX,KSFO,KAUS,0400/C29999,I23232,06,010419,011426",
			want: Result{AircraftSeries: "A320", Report: "037", Registration: "N491UA", ReportDate: "OCT04",
				Origin: "KSFO", FlightNumberDigits: "0400"},
		},
		{
			name: "no time",
			tail: "N402FR",
			text: "A320,035141,1,1,TB000000/REP050,00,00,4/CCN402FR,OCT02,KATL,KIAD,3692/C001,1665,4000/C1035807,040848/",
			want: Result{AircraftSeries: "A320", Report: "050", Registration: "N402FR", ReportDate: "OCT02",
				Origin: "KATL", Destination: "KIAD", FlightNumberDigits: "3692"},
		},
		{
			name: "split date and a C2 callsign in report 032",
			tail: "N34562",
			text: "A321,000460,1,1,TB000000/REP032,00,00,4/C1N34562,JAN,03,055354,KIAH,KORR/C2UAL787,4300,09/C3801971,802140,0,0062",
			want: Result{AircraftSeries: "A321", Report: "032", Registration: "N34562", ReportDate: "JAN03",
				ReportTime: "055354", Origin: "KIAH", Flight: "UAL787"},
		},
		{
			name: "flight number 0000 is no flight number",
			tail: "HB-JDF",
			text: "A320,090398,1,1,TB000000/REP073,00,00,1/C1HB-JDF,OCT04,093931,LQSA,LSZH,0000/C2SW060053740013",
			want: Result{AircraftSeries: "A320", Report: "073", Registration: "HB-JDF", ReportDate: "OCT04",
				ReportTime: "093931", Origin: "LQSA", Destination: "LSZH"},
		},
		{
			name: "REP239 fixed-width record",
			tail: "N366NB",
			text: "A319,060733,1,1,TB000000/REP239,00,00,4/239N366NB2975123125181051192N45602W122616  2  5  2341  1T 0512  72\r\n00 128 126 0000260500J8IH-KLAXKPDX",
			want: Result{AircraftSeries: "A319", Report: "239", Registration: "N366NB", ReportDate: "DEC31",
				ReportTime: "181051", Origin: "KLAX", Destination: "KPDX", FlightNumberDigits: "2975"},
		},
		{
			name: "REP239 record with text after the route",
			tail: "N595DT",
			text: "A321,037018,1,1,TB000000/REP239,00,00,4/239N595DT1909100426083958789N42691W 85 77350-20-48257 99AH0503 11000 268 554 0000430510B8/9WKSLCKJFK   35000 2232-------.----   224 461",
			want: Result{AircraftSeries: "A321", Report: "239", Registration: "N595DT", ReportDate: "OCT04",
				ReportTime: "083958", Origin: "KSLC", Destination: "KJFK", FlightNumberDigits: "1909"},
		},
		{
			name: "report 281 route and position",
			tail: "N368NW",
			text: "A320,115883,1,1,TB000000/REP281,00,00,4//WX02EN04KBDLKDTW\r\nN42191W07288409361047P0222180140XXXX21003020)\r\n",
			want: Result{AircraftSeries: "A320", Report: "281", Origin: "KBDL", Destination: "KDTW",
				Latitude: 42.191, Longitude: -72.884},
		},
		{
			name: "report 281 without a position",
			tail: "N368NW",
			text: "A320,115883,1,1,TB000000/REP281,00,00,4//WX02EN04KBDLKDTW\r\n 42191 07288409361047P0222180140XXXX21003020)",
			want: Result{AircraftSeries: "A320", Report: "281", Origin: "KBDL", Destination: "KDTW"},
		},
		{
			name: "report 291 route",
			tail: "N909AM",
			text: "A321,147316,1,1,TB000000/REP291,00,00,4/\r\nTRP KPHL KPBI  8 8\r\n/A1 175021, 32.3356,- 80.3872,339,167.0,437,0.001984,",
			want: Result{AircraftSeries: "A321", Report: "291", Origin: "KPHL", Destination: "KPBI"},
		},
		{
			// The two-digit report number is kept as sent, so the 032 and
			// 037 rule does not apply: this family's destinations were real.
			name: "short header with a C1 block",
			tail: "N746UW",
			text: "A37/A31937,1,1/C1N746UW,OCT04,094839,KBTV,KDCA,2680/C206,25938,5000,XX,0010,0,0100,XX,X/C30017,26751",
			want: Result{AircraftSeries: "A319", Report: "37", Registration: "N746UW", ReportDate: "OCT04",
				ReportTime: "094839", Origin: "KBTV", Destination: "KDCA", FlightNumberDigits: "2680"},
		},
		{
			name: "short header with a C1TRP block",
			tail: "N193UW",
			text: "A38/A32138,1,1/C1TRP,180234,KDFW,KRIC,08,8,94238/C2348139,-0904766,330,01653,464,0252,1/C3349627",
			want: Result{AircraftSeries: "A321", Report: "38", ReportTime: "180234", Origin: "KDFW", Destination: "KRIC"},
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
		// Placeholder airports.
		"A321,000123,1,1,TB000000/REP035,00,00,4/C1N589DT,OCT04,102635,NODT,NODT,0000/C201,68035",
		// A REP239 record whose registration is not the transmitted tail.
		"A321,037018,1,1,TB000000/REP239,00,00,4/239N595DT1909100426083958789N42691W 85 77350-20-48257 99AH0503 11000 268 554 0000430510B8/9WKSLCKJFK",
		// A REP239 record without airports in the route field.
		"A321,037018,1,1,TB000000/REP239,00,00,4/239N589DT1909100426083958789N42691W 85 77350-20-48257 99AH0503 11000 268 554 0000430510B8/9W''",
		// Not an ACMS header.
		"A321 IS AN AIRCRAFT",
		"RTE 1 06JAN26 0034 N37319 UAL2443 KORD/KJAX",
	} {
		if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "N589DT", Text: text}); r != nil {
			t.Errorf("Parse(%q) = %+v, want nil", text, r)
		}
	}
}
