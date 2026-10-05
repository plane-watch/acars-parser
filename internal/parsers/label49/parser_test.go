package label49

import (
	"testing"

	"acars_parser/internal/acars"
)

// The messages are real label 49 downlinks from the archive, cut after the
// header line and the start of the next.
func TestParse(t *testing.T) {
	tests := []struct {
		name, text string
		want       *Result
	}{
		{
			name: "callsign padded on the left",
			text: "01ERDC    ETD103/311802OMAALEMD\r\n+38996265.0- 30.7",
			want: &Result{Report: "ERDC", Flight: "ETD103", DayTime: "311802", Origin: "OMAA", Destination: "LEMD"},
		},
		{
			name: "callsign padded on the right",
			text: "01DCAPUAL995    /311805EBBRKEWR\r\n+ 1563169.0-   .7",
			want: &Result{Report: "DCAP", Flight: "UAL995", DayTime: "311805", Origin: "EBBR", Destination: "KEWR"},
		},
		{
			name: "callsign without padding",
			text: "01ERDCQTR77Q/030655OTHHEGLL\r\n35992 277.6 -28.4",
			want: &Result{Report: "ERDC", Flight: "QTR77Q", DayTime: "030655", Origin: "OTHH", Destination: "EGLL"},
		},
		{name: "invalid airport", text: "01ERDC    ETD103/311802OMAAQQQQ\r\n+38996265.0- 30.7"},
		{name: "no callsign", text: "01ERDC          /311802OMAALEMD\r\n+38996265.0- 30.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&Parser{}).Parse(&acars.Message{ID: 4, Label: "49", Text: tt.text})
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
			tt.want.MsgID = 4
			if *r != *tt.want {
				t.Errorf("got  %+v\nwant %+v", *r, *tt.want)
			}
		})
	}
}
