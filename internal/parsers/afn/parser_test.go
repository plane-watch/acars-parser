package afn

import (
	"reflect"
	"testing"

	"acars_parser/internal/acars"
)

// The messages are real ATS facilities notification (AFN) messages from the
// January 2026 corpus, with valid CRCs.
func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		label string
		text  string
		want  Result
	}{
		{
			name:  "acknowledgement with the aircraft address",
			label: "A0",
			text:  "/OAKODYA.AFN/FMHTZP16,.JA822J,86D5BE,031321/FAK0,KZAK/FARADS,0/FARATC,0A0F8",
			want: Result{GroundStation: "OAKODYA", Callsign: "TZP16", Registration: "JA822J", AircraftAddress: "86D5BE",
				Time: "031321", Acknowledgement: &Acknowledgement{Code: "0", Facility: "KZAK"},
				Applications: []Application{{Name: "ADS", Code: "0"}, {Name: "ATC", Code: "0"}}},
		},
		{
			name:  "contact advisory",
			label: "A0",
			text:  "/ANCATYA.AFN/FMHCSN437,.B-20EN,,141411/FCAANCXFXA,039CD",
			want: Result{GroundStation: "ANCATYA", Callsign: "CSN437", Registration: "B-20EN", Time: "141411",
				ContactAdvisory: &ContactAdvisory{Station: "ANCXFXA", Code: "0"}},
		},
		{
			name:  "short header, application with an address",
			label: "A0",
			text:  "/CMBCBYA.AFN/FMHSVA817,HZ-AK18/FAK0,VCCF/FARADS,0/FARATC,0,CMBCBYADBC6",
			want: Result{GroundStation: "CMBCBYA", Callsign: "SVA817", Registration: "HZ-AK18",
				Acknowledgement: &Acknowledgement{Code: "0", Facility: "VCCF"},
				Applications:    []Application{{Name: "ADS", Code: "0"}, {Name: "ATC", Code: "0", Station: "CMBCBYA"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := (&Parser{}).Parse(&acars.Message{Label: tt.label, Text: tt.text}).(*Result)
			if !ok {
				t.Fatal("Parse returned no result")
			}
			got.MsgID, got.Timestamp = 0, ""
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("Parse\n got %+v\nwant %+v", *got, tt.want)
			}
		})
	}
}

// TestParseH1 checks the forms in which label H1 carries AFN messages.
func TestParseH1(t *testing.T) {
	for _, text := range []string{
		"- #MD/A0 OAKODYA.AFN/FMHTZP16,.JA822J,86D5BE,031321/FAK0,KZAK/FARADS,0/FARATC,0A0F8",
		"OAKODYA.AFN/FMHTZP16,.JA822J,86D5BE,031321/FAK0,KZAK/FARADS,0/FARATC,0A0F8",
	} {
		got, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Text: text}).(*Result)
		if !ok || got.AircraftAddress != "86D5BE" || got.GroundStation != "OAKODYA" {
			t.Errorf("Parse(%q) = %+v", text, got)
		}
	}
}

// TestParseRejectsBadCRC checks that a message whose CRC does not match is
// not parsed: a corrupted character would otherwise give a wrong address.
func TestParseRejectsBadCRC(t *testing.T) {
	for _, text := range []string{
		"/OAKODYA.AFN/FMHTZP16,.JA822J,86D5BF,031321/FAK0,KZAK/FARADS,0/FARATC,0A0F8",
		"/OAKODYA.AFN/FMHTZP16,.JA822J,86D5BE,031321/FAK0,KZAK/FARADS,0/FARATC,0A0F9",
		"/OAKODYA.AFN/FMHTZP16",
		"/OAKODYA.AT1.N123AB0029007963",
	} {
		if r := (&Parser{}).Parse(&acars.Message{Label: "A0", Text: text}); r != nil {
			t.Errorf("Parse(%q) = %+v, want nil", text, r)
		}
	}
}
