package msgheader

import (
	"testing"

	"acars_parser/internal/acars"
)

// The messages are from live traffic (October and November 2026),
// shortened after the header.
func TestParse(t *testing.T) {
	tests := []struct {
		name, label, tail, text string
		flight                  string // The transmitted flight.
		want                    Result
	}{
		{
			name: "weather request", label: "5U", tail: "A6-ATC",
			text: "  01 WXRQ   0800/06 OMSJ/WMKK .A6-ATC\r\n/TYP 1/STA OOMS/STA OPKC/STA     ",
			want: Result{Sequence: "01", MessageType: "WXRQ", FlightNumberDigits: "0800", DayOfMonth: 6,
				Origin: "OMSJ", Destination: "WMKK", Registration: "A6-ATC"},
		},
		{
			// The flight field with a suffix gives its digits, when the
			// transmitted flight has that suffix.
			name: "flight number with a suffix", label: "5U", tail: "G-JZBF", flight: "LS038A",
			text: "  01 WXRQ   038A/26 EGGD/LMML .G-JZBF\r\n/TYP 1/STA LMML/STA LICB/STA LICC",
			want: Result{Sequence: "01", MessageType: "WXRQ", FlightNumberDigits: "038", DayOfMonth: 26,
				Origin: "EGGD", Destination: "LMML", Registration: "G-JZBF"},
		},
		{
			// Emirates gives the ICAO callsign.
			name: "position report with the callsign", label: "80", tail: "A6-JHB",
			text: "3N01 POSRPT UAE0069   /05 OMDB/EGKK .A6-JHB\r\nPOS N510404W0002938/ALT 035/MCH 000/FOB 0097",
			want: Result{Sequence: "3N01", MessageType: "POSRPT", Flight: "UAE0069", DayOfMonth: 5,
				Origin: "OMDB", Destination: "EGKK", Registration: "A6-JHB"},
		},
		{
			name: "callsign with a two-letter suffix", label: "80", tail: "A6-EVC",
			text: "3C01 WXLIDO UAE8UW    /25 KJFK/OMDB .A6-EVC\r\n/TYP 4/STA LBSF/STA LGAV/STA LBHP",
			want: Result{Sequence: "3C01", MessageType: "WXLIDO", Flight: "UAE8UW", DayOfMonth: 25,
				Origin: "KJFK", Destination: "OMDB", Registration: "A6-EVC"},
		},
		{
			// Fields after the registration are not parsed.
			name: "off report", label: "80", tail: "9V-SFP",
			text: "1103 OFFRP  7953  /07 KLAX/EHAM .9V-SFP 9999 07112026 0748\r\n/OUT 0723/OFF 0748/FOB 1251/ETA     ",
			want: Result{Sequence: "1103", MessageType: "OFFRP", FlightNumberDigits: "7953", DayOfMonth: 7,
				Origin: "KLAX", Destination: "EHAM", Registration: "9V-SFP"},
		},
		{
			name: "registration followed by a slash", label: "5U", tail: "N632AV",
			text: "  01 WXRQ   0585/25 KIAD/MSLP .N632AV/01F 02:03\r\n/TYP 1/STA MSLP/STA M",
			want: Result{Sequence: "01", MessageType: "WXRQ", FlightNumberDigits: "0585", DayOfMonth: 25,
				Origin: "KIAD", Destination: "MSLP", Registration: "N632AV"},
		},
		{
			name: "registration followed by a comma", label: "80", tail: "A6-APF", flight: "EY071M",
			text: "3J01 DSPTCH 071M/25 EGLL/OMAA .A6-APF,2135\r\nThanks, will give you an update.",
			want: Result{Sequence: "3J01", MessageType: "DSPTCH", FlightNumberDigits: "071", DayOfMonth: 25,
				Origin: "EGLL", Destination: "OMAA", Registration: "A6-APF"},
		},
		{
			// Without a transmitted tail, no registration is reported.
			name: "no transmitted tail", label: "10", tail: "",
			text: "10KF DEPMSG 8313/28 PHNL/YSSY .N429MC\r\n/POS N21055W158064/FOB 1257",
			want: Result{Sequence: "10KF", MessageType: "DEPMSG", FlightNumberDigits: "8313", DayOfMonth: 28,
				Origin: "PHNL", Destination: "YSSY"},
		},
	}
	for _, tt := range tests {
		got, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: tt.label, Tail: tt.tail, FlightNumber: tt.flight, Text: tt.text}).(*Result)
		if !ok {
			t.Errorf("%s: Parse returned no result", tt.name)
			continue
		}
		got.MsgID, got.Timestamp = 0, ""
		if *got != tt.want {
			t.Errorf("%s:\n got %+v\nwant %+v", tt.name, *got, tt.want)
		}
	}
}

// A header naming another aircraft, a flight field that is not a flight,
// an invalid airport or day, or other text is not parsed.
func TestParseRejects(t *testing.T) {
	tests := []struct{ name, tail, text string }{
		{"another aircraft", "A6-ATD", "  01 WXRQ   0800/06 OMSJ/WMKK .A6-ATC"},
		// The suffix is not compared by the extractor, so the parser
		// compares it: without the transmitted flight's, it is another
		// flight's (the transmitted flight below is LS038B).
		{"another suffix", "G-JZBF", "  01 WXRQ   038A/26 EGGD/LMML .G-JZBF"},
		{"not a flight", "A6-ATC", "  01 WXRQ   ABCDEF/06 OMSJ/WMKK .A6-ATC"},
		{"invalid airport", "A6-ATC", "  01 WXRQ   0800/06 QQQQ/WMKK .A6-ATC"},
		{"invalid day", "A6-ATC", "  01 WXRQ   0800/36 OMSJ/WMKK .A6-ATC"},
		{"other text", "A6-ATC", "POS N510404W0002938/ALT 035"},
	}
	for _, tt := range tests {
		if r := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "5U", Tail: tt.tail, FlightNumber: "LS038B", Text: tt.text}); r != nil {
			t.Errorf("%s: got %+v, want nil", tt.name, r)
		}
	}
}
