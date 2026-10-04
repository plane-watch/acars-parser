package adscrequest

import (
	"reflect"
	"testing"

	"acars_parser/internal/acars"
)

func ip(v int) *int         { return &v }
func fp(v float64) *float64 { return &v }

// The messages are real ADS-C uplinks (label A6) from the January 2026
// corpus; the expected values are libacars' decodes of them.
func TestParse(t *testing.T) {
	tests := []struct {
		name, text, station, reg string
		want                     []Request
	}{
		{
			name: "event contract, altitude range", text: "/MELCAYA.ADS.OH-LTS08091322BF21667CDF",
			station: "MELCAYA", reg: "OH-LTS",
			want: []Request{{Kind: "event", Contract: ip(9), AltitudeFloorFt: ip(34200), AltitudeCeilingFt: ip(35580)}},
		},
		{
			name: "event contract, waypoint changes", text: "/UPGCAYA.ADS.ZK-NZG081513226021FC141BBC",
			station: "UPGCAYA", reg: "ZK-NZG",
			want: []Request{{Kind: "event", Contract: ip(21), AltitudeFloorFt: ip(34800), AltitudeCeilingFt: ip(35200), WaypointChanges: true}},
		},
		{
			name: "cancel contract", text: "/BOMCAYA.ADS.A7-BFD02CA7A0F",
			station: "BOMCAYA", reg: "A7-BFD",
			want: []Request{{Kind: "cancel", Contract: ip(202)}},
		},
		{
			name: "periodic contract with groups and intent", text: "/TNRCAYA.ADS..B-LRV07070BC80E01100115021E7638",
			station: "TNRCAYA", reg: "B-LRV",
			want: []Request{{Kind: "periodic", Contract: ip(7), IntervalSecs: ip(576),
				Groups:         map[string]int{"earth_reference": 1, "meteo": 1},
				AircraftIntent: &Intent{Modulus: 2, ProjectionMinutes: 30}}},
		},
		{
			name: "periodic then event contract in one message", text: "/NANCDYA.ADS.N2996107000BCC0C000D010E0110010F011500010801140A28224A",
			station: "NANCDYA", reg: "N29961",
			want: []Request{
				{Kind: "periodic", Contract: ip(0), IntervalSecs: ip(832),
					Groups:         map[string]int{"flight_id": 0, "predicted_route": 1, "earth_reference": 1, "meteo": 1, "air_reference": 1},
					AircraftIntent: &Intent{Modulus: 0, ProjectionMinutes: 1}},
				{Kind: "event", Contract: ip(1), WaypointChanges: true, LateralDeviationNM: fp(5)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ok := (&Parser{}).Parse(&acars.Message{Label: "A6", Text: tt.text}).(*Result)
			if !ok {
				t.Fatal("Parse returned no result")
			}
			if r.GroundStation != tt.station || r.Registration != tt.reg || !reflect.DeepEqual(r.Requests, tt.want) {
				t.Errorf("got %s %s %+v\nwant %s %s %+v", r.GroundStation, r.Registration, r.Requests, tt.station, tt.reg, tt.want)
			}
		})
	}
}

// TestParseRelayedH1 checks the relayed form on label H1, and that an H1
// message without its original label, whose direction is unknown, is not
// read as a request (it could be a downlink report).
func TestParseRelayedH1(t *testing.T) {
	r, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Text: "- #MD/A6 BOMCAYA.ADS.A7-BFD02CA7A0F"}).(*Result)
	if !ok || len(r.Requests) != 1 || r.Requests[0].Kind != "cancel" {
		t.Errorf("relayed: %+v", r)
	}
	if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Text: "BOMCAYA.ADS.A7-BFD02CA7A0F"}); r != nil {
		t.Errorf("bare H1: %+v, want nil", r)
	}
	if r := (&Parser{}).Parse(&acars.Message{Label: "A6", LinkDirection: "downlink", Text: "/BOMCAYA.ADS.A7-BFD02CA7A0F"}); r != nil {
		t.Errorf("a downlink: %+v, want nil", r)
	}
}

func TestParseRejectsBadMessages(t *testing.T) {
	for _, text := range []string{
		"/BOMCAYA.ADS.A7-BFD02CA7A0E", // CRC changed.
		"/BOMCAYA.ADS.A7-BFD",         // No payload.
		"/MELCAYA.AT1.N514DN220012E8294A952882D8",
	} {
		if r := (&Parser{}).Parse(&acars.Message{Label: "A6", Text: text}); r != nil {
			t.Errorf("Parse(%q) = %+v, want nil", text, r)
		}
	}
}
