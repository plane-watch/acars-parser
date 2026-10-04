package cmcreport

import (
	"testing"

	"acars_parser/internal/acars"
)

// The header lines are real Boeing CMC reports (label H1) from the January
// 2026 corpus, with the tail as transmitted in the ACARS header; the HS-TWC
// report is from live traffic (October 2026).
func TestParse(t *testing.T) {
	tests := []struct {
		name string
		tail string
		text string
		want Result
	}{
		{
			name: "route report without an airline code",
			tail: "N37319",
			text: "RTE 1 06JAN26 0034 N37319 UAL2443 KORD/KJAX BCG59-U000-08E7 BCG3D-0MFC-0012 L 0008 06JAN26\r\nNCMM",
			want: Result{ReportType: "RTE", Sequence: 1, ReportDate: "06JAN26", ReportTime: "0034",
				Registration: "N37319", Flight: "UAL2443", Origin: "KORD", Destination: "KJAX"},
		},
		{
			name: "airline code separated by a space",
			tail: "HS-TWC",
			text: "RTE 1 04OCT26 0930 TG HS-TWC THA482 YPPH/VTBS BCG4F-45LD-0077 C L 0915 04OCT26\nFDE 34512742 A 0915 04OCT26",
			want: Result{ReportType: "RTE", Sequence: 1, ReportDate: "04OCT26", ReportTime: "0930", Airline: "TG",
				Registration: "HS-TWC", Flight: "THA482", Origin: "YPPH", Destination: "VTBS"},
		},
		{
			name: "airline code glued to an N-number",
			tail: "N703GT",
			text: "RTE 1 18JAN26 1327 5YN703GT GTI592 PANC/KMIA 311B-BCG-00W-19 L 1316 18JAN26",
			want: Result{ReportType: "RTE", Sequence: 1, ReportDate: "18JAN26", ReportTime: "1327", Airline: "5Y",
				Registration: "N703GT", Flight: "GTI592", Origin: "PANC", Destination: "KMIA"},
		},
		{
			name: "airline code glued to a dashed registration",
			tail: "B-17807",
			text: "CFG 41 19JAN26 0203 BRB-17807 EVA026 RCTP/KSEA BCG4F-45LD-0077 C L",
			want: Result{ReportType: "CFG", Sequence: 41, ReportDate: "19JAN26", ReportTime: "0203", Airline: "BR",
				Registration: "B-17807", Flight: "EVA026", Origin: "RCTP", Destination: "KSEA"},
		},
		{
			name: "header tail without the dash",
			tail: "HP9907",
			text: "RTE 1 19JAN26 2130 HP-9907 CMP229 KORD/MPTO BCG59-U000-08E7 BCG3A-0MFC-0015 L 2102 19JAN26",
			want: Result{ReportType: "RTE", Sequence: 1, ReportDate: "19JAN26", ReportTime: "2130",
				Registration: "HP-9907", Flight: "CMP229", Origin: "KORD", Destination: "MPTO"},
		},
		{
			name: "flight report",
			tail: ".PH-BKA",
			text: "PLF 1 18JAN26 0726 KL PH-BKA KLM602 KLAX/EHAM BCG4F-45LD-0077 C L 2137 17JAN26",
			want: Result{ReportType: "PLF", Sequence: 1, ReportDate: "18JAN26", ReportTime: "0726", Airline: "KL",
				Registration: "PH-BKA", Flight: "KLM602", Origin: "KLAX", Destination: "EHAM"},
		},
	}

	p := &Parser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Parse(&acars.Message{ID: 1, Label: "H1", Tail: tt.tail, Text: tt.text})
			if got == nil {
				t.Fatal("Parse returned nil")
			}
			r := *got.(*Result)
			r.MsgID, r.Timestamp = 0, ""
			if r != tt.want {
				t.Errorf("Parse() = %+v\nwant      %+v", r, tt.want)
			}
		})
	}
}

// TestParseDoesNotGuessTheRegistration checks that when the registration
// field does not end with the transmitted tail, no registration or airline
// is reported, but the rest of the header is.
func TestParseDoesNotGuessTheRegistration(t *testing.T) {
	for _, tail := range []string{"", "VH-XYZ"} {
		got := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "H1", Tail: tail,
			Text: "CFG 41 19JAN26 0203 BRB-17807 EVA026 RCTP/KSEA BCG4F-45LD-0077 C L"})
		if got == nil {
			t.Fatalf("tail %q: Parse returned nil", tail)
		}
		r := got.(*Result)
		if r.Registration != "" || r.Airline != "" {
			t.Errorf("tail %q: registration %q, airline %q, want both empty", tail, r.Registration, r.Airline)
		}
		if r.Flight != "EVA026" || r.Origin != "RCTP" || r.Destination != "KSEA" {
			t.Errorf("tail %q: flight and route = %q %q-%q", tail, r.Flight, r.Origin, r.Destination)
		}
	}
}

func TestParseRejectsOtherText(t *testing.T) {
	for _, text := range []string{"", "RTE", "POSN39006W075260,OYVAY", "RTE 1 BAD HEADER"} {
		if got := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "H1", Text: text}); got != nil {
			t.Errorf("Parse(%q) = %+v, want nil", text, got)
		}
	}
}
