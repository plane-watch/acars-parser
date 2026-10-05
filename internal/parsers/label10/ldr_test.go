package label10

import (
	"math"
	"testing"

	"acars_parser/internal/acars"
)

// The messages are real Southwest label 10 LDR reports (October 2026).
func TestLDRParser(t *testing.T) {
	tests := []struct {
		name, text string
		want       *LDRResult
	}{
		{
			name: "western longitude with three digits",
			text: "LDR01,189,D,SWA-2600-019,0,N 35.194,W119.735,34995,  9.4,KSFO,KLAX,KLAX,24R/,25L/,/,0,0,,,,,,,0,0,0,00,,105.5,08.4,113.9,,,",
			want: &LDRResult{Report: "LDR01", Latitude: 35.194, Longitude: -119.735, Altitude: 34995, Origin: "KSFO", Destination: "KLAX"},
		},
		{
			name: "padded longitude",
			text: "LDR01,189,D,SWA-2600-019,0,N 33.514,W 80.732,31995, 11.8,KRIC,KMCO,KMCO,17L/,18R/,/,0,0,,,,,,,0,0,0,00,,135.8,09.0,144.8,,,",
			want: &LDRResult{Report: "LDR01", Latitude: 33.514, Longitude: -80.732, Altitude: 31995, Origin: "KRIC", Destination: "KMCO"},
		},
		{name: "invalid airport", text: "LDR01,189,D,SWA-2600-019,0,N 33.514,W 80.732,31995, 11.8,KRIC,QQQQ,KMCO,17L/"},
		{name: "latitude out of range", text: "LDR01,189,D,SWA-2600-019,0,N 93.514,W 80.732,31995, 11.8,KRIC,KMCO,KMCO,17L/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&LDRParser{}).Parse(&acars.Message{ID: 2, Label: "10", Text: tt.text})
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			r, ok := got.(*LDRResult)
			if !ok {
				t.Fatalf("got %T, want *LDRResult", got)
			}
			if math.Abs(r.Latitude-tt.want.Latitude) > 1e-9 || math.Abs(r.Longitude-tt.want.Longitude) > 1e-9 {
				t.Errorf("position = %v, %v, want %v, %v", r.Latitude, r.Longitude, tt.want.Latitude, tt.want.Longitude)
			}
			r.Latitude, r.Longitude = tt.want.Latitude, tt.want.Longitude
			tt.want.MsgID = 2
			if *r != *tt.want {
				t.Errorf("got  %+v\nwant %+v", *r, *tt.want)
			}
		})
	}
}
