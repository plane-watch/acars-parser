package h1

import (
	"math"
	"testing"

	"acars_parser/internal/acars"
)

// The messages below are real H1 POSN reports from the corpus. Expected values
// were decoded from the message text by hand:
// POSN<lat><lon>,<current wpt>,<time>,<FL>,<next wpt>,<ETA>,<third wpt>,<temp>,<wind>,...
func TestH1PosParser(t *testing.T) {
	tests := []struct {
		name string
		text string
		want H1PosResult
	}{
		{
			// The wind field is six digits in most reports (83,572 in the
			// January corpus): DDDSSS, so "290061" is 290 degrees at 61 kt.
			name: "six-digit wind",
			text: "POSN39006W075260,OYVAY,014938,340,SMELI,015155,TRPOD,M52,290061,204D6A3",
			want: H1PosResult{Latitude: 39.01, Longitude: -75.433333, ReportTime: "014938", FlightLevel: 340,
				CurrentWaypoint: "OYVAY", NextWaypoint: "SMELI", ThirdWaypoint: "TRPOD", ETA: "015155",
				Temperature: -52, WindDir: 290, WindSpeed: 61},
		},
		{
			name: "six-digit wind over 100 kt",
			text: "POSN30374W081044,WOPNR,143646,360,PRMUS,144407,KENLL,M52,242105,12879C7",
			want: H1PosResult{Latitude: 30.623333, Longitude: -81.073333, ReportTime: "143646", FlightLevel: 360,
				CurrentWaypoint: "WOPNR", NextWaypoint: "PRMUS", ThirdWaypoint: "KENLL", ETA: "144407",
				Temperature: -52, WindDir: 242, WindSpeed: 105},
		},
		{
			// Five-digit winds (56,796 reports) are DDDSS: "04326" is 043 degrees at 26 kt.
			name: "five-digit wind and a lat/lon next waypoint",
			text: "POSN59058E017163,POGOK,161341,360,N58548E016310,161700,UPGAS,M60,04326,200662F",
			want: H1PosResult{Latitude: 59.096667, Longitude: 17.271667, ReportTime: "161341", FlightLevel: 360,
				CurrentWaypoint: "POGOK", NextWaypoint: "N58548E016310", ThirdWaypoint: "UPGAS", ETA: "161700",
				Temperature: -60, WindDir: 43, WindSpeed: 26},
		},
	}

	p := &H1PosParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Parse(&acars.Message{ID: 1, Label: "H1", Text: tt.text})
			if got == nil {
				t.Fatal("Parse returned nil")
			}
			r := got.(*H1PosResult)

			if math.Abs(r.Latitude-tt.want.Latitude) > 1e-5 || math.Abs(r.Longitude-tt.want.Longitude) > 1e-5 {
				t.Errorf("position = %f,%f, want %f,%f", r.Latitude, r.Longitude, tt.want.Latitude, tt.want.Longitude)
			}
			// Compare the remaining fields exactly.
			r.MsgID, r.Timestamp, r.Tail = 0, "", ""
			r.Latitude, r.Longitude = tt.want.Latitude, tt.want.Longitude
			if *r != tt.want {
				t.Errorf("Parse() = %+v\nwant      %+v", *r, tt.want)
			}
		})
	}
}
