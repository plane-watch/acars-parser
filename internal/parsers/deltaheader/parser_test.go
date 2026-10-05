package deltaheader

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name, label, text string
		want              *Result
	}{
		{
			name:  "flight number line",
			label: "24",
			text:  "041124 KATL KIAD7\r\n/FN 1324",
			want:  &Result{Origin: "KATL", Destination: "KIAD", FlightNumberDigits: "1324"},
		},
		{
			name:  "flight number line with more text",
			label: "32",
			text:  "041132 KATL KIAD8\n/FN 1324\n         01R                   ",
			want:  &Result{Origin: "KATL", Destination: "KIAD", FlightNumberDigits: "1324"},
		},
		{
			name:  "no flight number line",
			label: "11",
			text:  "041311 KATL KIAD6\n132413010089111540560      Y            1245",
			want:  &Result{Origin: "KATL", Destination: "KIAD"},
		},
		{
			name:  "non-US airport",
			label: "24",
			text:  "311824 KJFK MMMX7\n/FN 0625",
			want:  &Result{Origin: "KJFK", Destination: "MMMX", FlightNumberDigits: "0625"},
		},
		{name: "label not echoed", label: "10", text: "041124 KATL KIAD7\n/FN 1324"},
		{name: "invalid airport", label: "24", text: "041124 KATL QQQQ7\n/FN 1324"},
		{name: "no digit after the route", label: "24", text: "041124 KATL KIAD\n/FN 1324"},
		{name: "text after the header line", label: "24", text: "041124 KATL KIAD7 X\n/FN 1324"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&Parser{}).Parse(&acars.Message{ID: 7, Label: tt.label, Text: tt.text})
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
			tt.want.MsgID = 7
			if *r != *tt.want {
				t.Errorf("got %+v, want %+v", *r, *tt.want)
			}
		})
	}
}
