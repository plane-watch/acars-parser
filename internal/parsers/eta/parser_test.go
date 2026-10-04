package eta

import (
	"testing"

	"acars_parser/internal/acars"
)

// The messages below are real label 5Z downlinks from the corpus. Expected
// values were read from the message text by hand.
func TestParse(t *testing.T) {
	tests := []struct {
		name string
		text string
		want Result
	}{
		{
			name: "ET expected time",
			text: "/ET EXP TIME       / KIAD EDDF 21 045413/EON 0532",
			want: Result{MessageType: "ET", Origin: "KIAD", Destination: "EDDF", DayOfMonth: 21, ReportTime: "045413", ETA: "0532"},
		},
		{
			name: "ET expected time with mode",
			text: "/ET EXP TIME       / KSNA KIAH 29 182901/EON 1908 AUTO",
			want: Result{MessageType: "ET", Origin: "KSNA", Destination: "KIAH", DayOfMonth: 29, ReportTime: "182901", ETA: "1908", Mode: "AUTO"},
		},
		{
			name: "IR in-range report",
			text: "/IR MKE0100001/UM   /WC   /IB   /ETA 0511",
			want: Result{MessageType: "IR", RawData: "MKE0100001", ETA: "0511"},
		},
		{
			name: "B6 landing data request",
			text: "/B6 LDG DATA REQ   / KIAD KBOS 31 182755 KBOS R22L/---- F30 G1460",
			want: Result{MessageType: "B6", Origin: "KIAD", Destination: "KBOS", DayOfMonth: 31, ReportTime: "182755", Runway: "22L"},
		},
		{
			name: "B6 landing data request with a padded runway",
			text: "/B6 LDG DATA REQ   / MMUN KEWR 10 025312 KEWR R4R /---- G1623",
			want: Result{MessageType: "B6", Origin: "MMUN", Destination: "KEWR", DayOfMonth: 10, ReportTime: "025312", Runway: "4R"},
		},
		{
			name: "B6 landing data request with a runway without a suffix",
			text: "/B6 LDG DATA REQ   / KIAD LSGG 21 045316 LSGG R22 /----",
			want: Result{MessageType: "B6", Origin: "KIAD", Destination: "LSGG", DayOfMonth: 21, ReportTime: "045316", Runway: "22"},
		},
		{
			name: "C3 gate request",
			text: "/C3 GATE REQ       / KEWR KATL 17 040207 1287 ---- ---- ---- ----",
			want: Result{MessageType: "C3", Origin: "KEWR", Destination: "KATL", DayOfMonth: 17, ReportTime: "040207"},
		},
	}

	p := &Parser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Parse(&acars.Message{ID: 1, Label: "5Z", Text: tt.text})
			if got == nil {
				t.Fatal("Parse returned nil")
			}
			r := got.(*Result)
			// Only the parsed fields are compared.
			r.MsgID, r.Timestamp, r.Tail = 0, "", ""
			if *r != tt.want {
				t.Errorf("Parse() = %+v\nwant      %+v", *r, tt.want)
			}
		})
	}
}

func TestParseRejectsUnrelatedText(t *testing.T) {
	p := &Parser{}
	for _, text := range []string{"", "/XX SOMETHING ELSE", "/B6 LDG DATA REQ"} {
		if got := p.Parse(&acars.Message{Label: "5Z", Text: text}); got != nil {
			t.Errorf("Parse(%q) = %+v, want nil", text, got)
		}
	}
}
