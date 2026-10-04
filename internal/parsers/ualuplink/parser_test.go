package ualuplink

import (
	"testing"

	"acars_parser/internal/acars"
)

// The messages are real United Airlines uplinks (label RA) from the January
// 2026 corpus, cut short where the rest does not matter.
func TestParse(t *testing.T) {
	tests := []struct {
		name string
		text string
		want Result
	}{
		{
			name: "IATA flight with a slash, CRLF line ends",
			text: "QUNDCULUA~1EPNF INFO\r\nUA1211/02 PHNL KDEN   \r\nSENT: 06:53:29Z \r\nNO DANGEROUS GOODS PLANNED  \r\n\r\n",
			want: Result{Title: "EPNF INFO", Flight: "UA1211", Day: 2, Origin: "PHNL", Destination: "KDEN"},
		},
		{
			name: "IATA flight with leading zeros, tab-indented lines",
			text: "QUNDCULUA~1HOWGOZIT ERROR\n\tUA0187/03 FAOR KEWR   \n\tSENT: 01:36:18Z \n\t3016-03 JNB\n\tERR: NO FLT PLAN",
			want: Result{Title: "HOWGOZIT ERROR", Flight: "UA0187", Day: 3, Origin: "FAOR", Destination: "KEWR"},
		},
		{
			name: "ICAO flight with a dash",
			text: "QUNDCULUA~1TURB SIGMET\n\tUAL252-04 PHNL KIAH\n\tYOUR FLIGHT IS WITHIN A\n\tTURBULENCE SIGMET AREA",
			want: Result{Title: "TURB SIGMET", Flight: "UAL252", Day: 4, Origin: "PHNL", Destination: "KIAH"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := (&Parser{}).Parse(&acars.Message{Label: "RA", Text: tt.text}).(*Result)
			if !ok {
				t.Fatal("Parse returned no result")
			}
			got.MsgID, got.Timestamp = 0, ""
			if *got != tt.want {
				t.Errorf("Parse\n got %+v\nwant %+v", *got, tt.want)
			}
		})
	}
}

// TestParseRejectsOtherText checks that messages without the flight header,
// or with an invalid one, are not parsed.
func TestParseRejectsOtherText(t *testing.T) {
	for _, text := range []string{
		// No flight header line.
		"QUNDCULUA~1RECEIVED 7582661\n\tRECEIVED 0 -\n\tLAVATORY A ACCESSORY COM",
		// The airports are not plausible ICAO codes.
		"QUNDCULUA~1EPNF INFO\r\nUA1211/02 XXXX KDEN\r\n",
		// The day of the month is out of range.
		"QUNDCULUA~1EPNF INFO\r\nUA1211/32 PHNL KDEN\r\n",
		// A header-like line in the body, not in the header position.
		"QUNDCULUA~1FREE TEXT\nDO NOT USE THE FOLLOWING ROUTE:\nUA1211/02 PHNL KDEN\nTHIS ROUTE HAS BEEN CANCELLED",
		// Another airline's uplink.
		"QUHDQOCLO~1RA101211540 TAKEOFF DATA\nUA1211/02 PHNL KDEN\n",
	} {
		if r := (&Parser{}).Parse(&acars.Message{Label: "RA", Text: text}); r != nil {
			t.Errorf("Parse(%q) = %+v, want nil", text, r)
		}
	}
}

// TestParseHeaderPositionsAndSpacing checks the header after a part marker,
// extra spaces and tabs, and CR-only line ends.
func TestParseHeaderPositionsAndSpacing(t *testing.T) {
	for _, text := range []string{
		"QUNDCULUA~1HOWGOZIT\r\n** PART 01 OF 01 **\r\nUA1211/02 PHNL KDEN\r\n",
		"QUNDCULUA~1EPNF INFO\n UA1211/02  PHNL\tKDEN \n",
		"QUNDCULUA~1EPNF INFO\rUA1211/02 PHNL KDEN\rSENT: 06:53:29Z",
	} {
		r, ok := (&Parser{}).Parse(&acars.Message{Label: "RA", Text: text}).(*Result)
		if !ok || r.Flight != "UA1211" || r.Origin != "PHNL" || r.Destination != "KDEN" {
			t.Errorf("Parse(%q) = %+v", text, r)
		}
	}
}
