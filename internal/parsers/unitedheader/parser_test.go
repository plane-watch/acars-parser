package unitedheader

import (
	"testing"

	"acars_parser/internal/acars"
)

// The messages are real United downlinks from the archive, cut after the
// header line.
func TestParse(t *testing.T) {
	tests := []struct {
		name, label, text string
		want              *Result
	}{
		{
			name:  "off event",
			label: "14",
			text:  "/14 OFF EVENT      / KFSD KDEN 19 153705/TIME 1537\r\n/AU 19709317/AON 18405316/AIN 18408316",
			want:  &Result{Code: "14", Title: "OFF EVENT", Origin: "KFSD", Destination: "KDEN", DayOfMonth: 19, ReportTime: "153705"},
		},
		{
			name:  "system configuration report",
			label: "33",
			text:  "/33 SYS CONF RPT   / KORD KBOS 31 180701\r\nAC TYPE B737\r\nACARS 2.5",
			want:  &Result{Code: "33", Title: "SYS CONF RPT", Origin: "KORD", Destination: "KBOS", DayOfMonth: 31, ReportTime: "180701"},
		},
		{
			name:  "title filling the field",
			label: "27",
			text:  "/27 APU SHUTDOWN AL/ CYVR KDEN 19 154820 104B0215482004010",
			want:  &Result{Code: "27", Title: "APU SHUTDOWN AL", Origin: "CYVR", Destination: "KDEN", DayOfMonth: 19, ReportTime: "154820"},
		},
		{name: "code is not the label", label: "15", text: "/14 OFF EVENT      / KFSD KDEN 19 153705/TIME 1537"},
		{name: "invalid airport", label: "14", text: "/14 OFF EVENT      / KFSD QQQQ 19 153705/TIME 1537"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&Parser{}).Parse(&acars.Message{ID: 5, Label: tt.label, Text: tt.text})
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			r, ok := got.(*Result)
			if !ok {
				t.Fatalf("got %T, want *Result", got)
			}
			tt.want.MsgID = 5
			if *r != *tt.want {
				t.Errorf("got  %+v\nwant %+v", *r, *tt.want)
			}
		})
	}
}
