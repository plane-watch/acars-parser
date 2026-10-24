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
			name: "C2 callsign with two suffix letters",
			tail: "N34562",
			text: "A321,000460,1,1,TB000000/REP032,00,00,4/C1N34562,JAN,03,055354,KIAH,KORR/C2SWR4WF,4300,09/C3801971",
			want: Result{AircraftSeries: "A321", Report: "032", Registration: "N34562", ReportDate: "JAN03",
				ReportTime: "055354", Origin: "KIAH", Flight: "SWR4WF"},
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
		{
			name: "C1 CSV line under a short header line",
			tail: "VH-EBP",
			text: "R01/A33001,1,1\r\nC1,.VH-EBP,25DEC31,17.03.50,WADD,YSSY,QFA044    ,5000,374\r\nC2,002,06.0,000000,D1333GE05CQFA11,000,052,052\r\n",
			want: Result{AircraftSeries: "A330", Report: "01", Registration: "VH-EBP", ReportDate: "DEC31",
				ReportTime: "170350", Origin: "WADD", Destination: "YSSY", Flight: "QFA044"},
		},
		{
			name: "WX block under a short header line, after a number line",
			tail: "N815NW",
			text: "281095500101010066028\r\nR81/A33081,1,1\r\n/WX02EN31KMSPPHNL\r\nN43582W09558218422749M3903161020XXXX250030ERV\r\n",
			want: Result{AircraftSeries: "A330", Report: "81", Origin: "KMSP", Destination: "PHNL",
				Latitude: 43.582, Longitude: -95.582},
		},
		{
			name: "76401 record on its own",
			tail: "N474WN",
			text: "76401\r\n02E04KSJCKBUR\r\nN37393W12196023300194P031300007G000025002PJ2R",
			want: Result{Report: "76401", Origin: "KSJC", Destination: "KBUR", Latitude: 37.393, Longitude: -121.96},
		},
		{
			name: "76401 record in a report with the long header",
			tail: "OE-IVA",
			text: "A320,047784,1,1,TB000000/REP301,00,00,1/76401\r\n02E02LCLKLSZH\r\nN42167E02128503383802M061039050GXXXX241030/9W\r\n",
			want: Result{AircraftSeries: "A320", Report: "301", Origin: "LCLK", Destination: "LSZH", Latitude: 42.167, Longitude: 21.285},
		},
		{
			name: "76401 record under a short header line",
			tail: "HB-JHK",
			text: "R01/A33001,1,1\r\n76401\r\n02E02VABBLSZH\r\nN47566E01233803453600M056276012GXXXX2400B8IH-\r\n",
			want: Result{AircraftSeries: "A330", Report: "01", Origin: "VABB", Destination: "LSZH", Latitude: 47.566, Longitude: 12.338},
		},
		{
			name: "C1 CSV line after a line of digits",
			tail: "EI-EDY",
			text: "072006100101011179212\r\nR07/A33007,1,1\r\nC1,.EI-EDY,26OCT07,18.50.39,EIDW,KORD,EIN123    ,4820,019\r\nC2,002,06.0",
			want: Result{AircraftSeries: "A330", Report: "07", Registration: "EI-EDY", ReportDate: "OCT07",
				ReportTime: "185039", Origin: "EIDW", Destination: "KORD", Flight: "EIN123"},
		},
		{
			name: "A380 report with fields separated by semicolons",
			tail: "G-XLEL",
			text: "A380000411,1,1,TB000000;REP210,80;H0121080605100087.G-XLEL10051026045954081;H02KMIA EGLLBAW6R     S0586S0785-GART-0009;H03LG LEVER DOWN",
			want: Result{AircraftSeries: "A380", Report: "210", Registration: "G-XLEL", ReportDate: "OCT05",
				ReportTime: "045954", Origin: "KMIA", Destination: "EGLL", Flight: "BAW6R"},
		},
		{
			name: "CC block after a ground address",
			tail: "G-NEOS",
			text: "/QUKAXBA PARJBXH.A321,021822,1,1,TB000000/REP001,85,02,1/CCG-NEOS,DEC31,174446,EGLL,LSZH,0720/C0TFC050BA030101/C106",
			want: Result{AircraftSeries: "A321", Report: "001", Registration: "G-NEOS", ReportDate: "DEC31",
				ReportTime: "174446", Origin: "EGLL", Destination: "LSZH", FlightNumberDigits: "0720"},
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

// A report naming another registration than the transmitted tail is not
// parsed, so that its flight and route are not credited to the
// transmitting aircraft. Without a transmitted tail, it is parsed without a
// registration.
func TestParseRejectsAnotherRegistration(t *testing.T) {
	text := "A321,014057,1,1,TB000000/REP001,00,00,1/CCVH-VWT,JAN20,040543,YSSY,YBBN,0816/C0TIA05JST4R0000"
	if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "VH-VWU", Text: text}); r != nil {
		t.Errorf("tail VH-VWU: got %+v, want nil", r)
	}
	got, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Text: text}).(*Result)
	if !ok {
		t.Fatal("no tail: Parse returned no result")
	}
	if got.Registration != "" || got.Origin != "YSSY" {
		t.Errorf("no tail: got %+v, want the route without a registration", *got)
	}
}

// A C2 block that starts like a callsign but is not one names a flight
// that cannot be compared with the transmitted one, so the report is not
// parsed. Case does not matter, as for the rest of the report.
func TestParseRejectsUnreadableCallsign(t *testing.T) {
	for _, c2 := range []string{"UAL1A2", "ual1a2"} {
		text := "A321,000460,1,1,TB000000/REP032,00,00,4/C1N34562,JAN,03,055354,KIAH,KORR/C2" + c2 + ",4300,09/C3801971"
		if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "N34562", Text: text}); r != nil {
			t.Errorf("C2 %s: got %+v, want nil", c2, r)
		}
	}
	text := "A321,000460,1,1,TB000000/REP032,00,00,4/C1N34562,JAN,03,055354,KIAH,KORR/C2UAL4wf,4300,09/C3801971"
	if got, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "N34562", Text: text}).(*Result); !ok || got.Flight != "UAL4WF" {
		t.Errorf("C2 UAL4wf: got %+v, want flight UAL4WF", got)
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
		// An A380 report without a route.
		"A380000792,1,1,TB000000;REP212,80;H0121280601112870.9V-SKT10051026045827110;H02-------------------S0586S0785ACMSCDAM09;H03PARK BRAKE",
		// Not an ACMS header.
		"A321 IS AN AIRCRAFT",
		"RTE 1 06JAN26 0034 N37319 UAL2443 KORD/KJAX",
	} {
		if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "N589DT", Text: text}); r != nil {
			t.Errorf("Parse(%q) = %+v, want nil", text, r)
		}
	}
}

// The registry calls QuickCheck on the text as received, so it must look
// past a ground address too.
func TestQuickCheckAfterGroundAddress(t *testing.T) {
	text := "/QUKAXBA PARJBXH.A321,021822,1,1,TB000000/REP001,85,02,1/CCG-NEOS,DEC31,174446,EGLL,LSZH,0720/C0TFC050BA030101"
	if !(&Parser{}).QuickCheck(text) {
		t.Error("QuickCheck = false, want true")
	}
}

// An A380 report is parsed only when its H01 block names the transmitted
// tail as the whole registration.
func TestParseA380OtherAircraft(t *testing.T) {
	for _, tt := range []struct{ tail, text string }{
		{"G-XLEI", "A380000411,1,1,TB000000;REP210,80;H0121080605100087.G-XLEL10051026045954081;H02KMIA EGLLBAW6R     S0586"},
		{"N123", "A380000411,1,1,TB000000;REP210,80;H0121080605100087.N1234510051020045954081;H02KMIA EGLLBAW6R     S0586"},
	} {
		if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: tt.tail, Text: tt.text}); r != nil {
			t.Errorf("tail %s: got %+v, want nil", tt.tail, r)
		}
	}
}

// The CC block's registration field holds six characters, so a longer
// registration is sent without its first characters ("Z-NS34" for
// HZ-NS34); it is the transmitted tail, which is reported.
func TestParseTruncatedRegistration(t *testing.T) {
	for _, tt := range []struct{ tail, text string }{
		{"HZ-NS34", "A320,000053,1,1,TB000000/REP001,65,02,1/CCZ-NS34,OCT02,052340,OEJN,OERK,0201/C0TWP03005030001"},
		{"B-18103", "A321,081733,1,1,TB000000/REP003,64,01,4/CC-18103,OCT05,232510,RCTP,VHHH,0581/C0TWP03005030001"},
	} {
		got, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: tt.tail, Text: tt.text}).(*Result)
		if !ok {
			t.Fatalf("tail %s: Parse returned no result", tt.tail)
		}
		if got.Registration != tt.tail {
			t.Errorf("tail %s: Registration = %q, want the transmitted tail", tt.tail, got.Registration)
		}
	}
	// A transmitted tail spelt without its dash matches too.
	dashless := "A321,081733,1,1,TB000000/REP003,64,01,4/CC-18103,OCT05,232510,RCTP,VHHH,0581/C0TWP03005030001"
	if got, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: ".B18103", Text: dashless}).(*Result); !ok || got.Registration != "B18103" {
		t.Errorf("tail .B18103: got %+v, want registration B18103", got)
	}
	// A truncated field is shared by registrations with the same ending
	// (RA-12345, RF-12345), so it is accepted only with a flight, which
	// the extractor checks against the transmitted one.
	noFlight := "A320,000053,1,1,TB000000/REP001,65,02,1/CC-12345,OCT02,052340,UUEE,UUDD,0000/C0TWP03005030001"
	if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "RA-12345", Text: noFlight}); r != nil {
		t.Errorf("truncated field without a flight: got %+v, want nil", r)
	}
	// A registration field of Xs is unset and names no aircraft.
	unset := "A320,000053,1,1,TB000000/REP001,65,02,1/CCXXXXXX,OCT02,052340,EFHK,ESSA,0801/C0TWP03005030001"
	if got, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "OH-LZF", Text: unset}).(*Result); !ok || got.Registration != "" {
		t.Errorf("unset registration: got %+v, want a result without a registration", got)
	}
	// A different registration of six characters is another aircraft.
	text := "A321,000919,1,1,TB000000/REP020,64,01,4/CCN4062J,OCT24,045900,KJFK,KBOS,1620/C0TWP03005030001"
	if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "N462J", Text: text}); r != nil {
		t.Errorf("tail N462J, field N4062J: got %+v, want nil", r)
	}
}
