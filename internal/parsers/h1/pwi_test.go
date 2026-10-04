package h1

import (
	"reflect"
	"testing"

	"acars_parser/internal/acars"
)

// The messages below are real H1 PWI (predicted wind information) uplinks
// from the corpus, decoded by hand. Climb (CB) and descent (DD) winds are
// dot-separated FFFDDDSSS groups: flight level, direction, speed. The last
// group in a section can be followed by other data (":,,,,", or the message's
// 4-character checksum), and long messages are wrapped with "\n\t".
func TestPWIParser(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		climb   []AltitudeWind
		descent []AltitudeWind
		route   []RouteWindLayer
	}{
		{
			name: "single route wind",
			text: "PWI/WD300,DUBED,226039C583",
			route: []RouteWindLayer{{FlightLevel: 300, Waypoints: []WaypointWind{
				{Waypoint: "DUBED", WindDir: 226, WindSpeed: 39},
			}}},
		},
		{
			name: "final groups followed by a section separator and by the checksum",
			text: "- #MDPWI/DD300148018.240142004.180020005.100328007:,,,,/CB170022006.100338003B5DE",
			descent: []AltitudeWind{
				{FlightLevel: 300, WindDir: 148, WindSpeed: 18},
				{FlightLevel: 240, WindDir: 142, WindSpeed: 4},
				{FlightLevel: 180, WindDir: 20, WindSpeed: 5},
				{FlightLevel: 100, WindDir: 328, WindSpeed: 7},
			},
			climb: []AltitudeWind{
				{FlightLevel: 170, WindDir: 22, WindSpeed: 6},
				{FlightLevel: 100, WindDir: 338, WindSpeed: 3},
			},
		},
		{
			// Multi-block messages carry a "- #MD" block marker wherever a block
			// ends, even inside a token: "VE- #MDLDT" is the waypoint VELDT.
			// Built from fragments of real messages 5038523 and 5713260.
			name: "block markers inside tokens",
			text: "- #MDPWI/WD380,GAPLI,275062,380M67- #MD.SIDDI,297063,380M68.VE- #MDLDT,303051,380M53C583",
			route: []RouteWindLayer{{FlightLevel: 380, Waypoints: []WaypointWind{
				{Waypoint: "GAPLI", WindDir: 275, WindSpeed: 62, Temperature: -67},
				{Waypoint: "SIDDI", WindDir: 297, WindSpeed: 63, Temperature: -68},
				{Waypoint: "VELDT", WindDir: 303, WindSpeed: 51, Temperature: -53},
			}}},
		},
		{
			// Block markers are "- #MD" or "- #M1" to "- #M3". From real
			// message 4937768: "330- #M1041" is the wind 330041.
			name: "numbered block marker inside a wind",
			text: "PWI/WD400,HVE,330041,400M56.GGAPP,330- #M1041,400M58.AALAN,330043,400M583B13",
			route: []RouteWindLayer{{FlightLevel: 400, Waypoints: []WaypointWind{
				{Waypoint: "HVE", WindDir: 330, WindSpeed: 41, Temperature: -56},
				{Waypoint: "GGAPP", WindDir: 330, WindSpeed: 41, Temperature: -58},
				{Waypoint: "AALAN", WindDir: 330, WindSpeed: 43, Temperature: -58},
			}}},
		},
		{
			// Some route winds are five digits, DDDSS (117,122 groups in the
			// January 2026 corpus): "24270" is 242 degrees at 70 kt.
			// From real message 6250236.
			name: "five-digit route winds",
			text: "- #MDPWI/WD410,PASAS,24270,410M62.STG,26749,410M68C583",
			route: []RouteWindLayer{{FlightLevel: 410, Waypoints: []WaypointWind{
				{Waypoint: "PASAS", WindDir: 242, WindSpeed: 70, Temperature: -62},
				{Waypoint: "STG", WindDir: 267, WindSpeed: 49, Temperature: -68},
			}}},
		},
		{
			// Route waypoints can be lat/lon points. From real message 10961877.
			name: "lat/lon route waypoints",
			text: "PWI/WD360,N53089E019480,259014,360M66.N53196E019325,247016,360M66.LARMA,225033,360M65C583",
			route: []RouteWindLayer{{FlightLevel: 360, Waypoints: []WaypointWind{
				{Waypoint: "N53089E019480", WindDir: 259, WindSpeed: 14, Temperature: -66},
				{Waypoint: "N53196E019325", WindDir: 247, WindSpeed: 16, Temperature: -66},
				{Waypoint: "LARMA", WindDir: 225, WindSpeed: 33, Temperature: -65},
			}}},
		},
		{
			name: "group wrapped across lines",
			text: "- #MDPWI/SN2601183180/WD280,/WD310,/WD340,/WD370,/DD350270024.310\n\t270031.200240023.1002400076E63",
			descent: []AltitudeWind{
				{FlightLevel: 350, WindDir: 270, WindSpeed: 24},
				{FlightLevel: 310, WindDir: 270, WindSpeed: 31},
				{FlightLevel: 200, WindDir: 240, WindSpeed: 23},
				{FlightLevel: 100, WindDir: 240, WindSpeed: 7},
			},
		},
	}

	p := &PWIParser{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Parse(&acars.Message{ID: 1, Label: "H1", Text: tt.text})
			if got == nil {
				t.Fatal("Parse returned nil")
			}
			r := got.(*PWIResult)
			if !reflect.DeepEqual(r.ClimbWinds, tt.climb) {
				t.Errorf("climb winds = %+v\nwant           %+v", r.ClimbWinds, tt.climb)
			}
			if !reflect.DeepEqual(r.DescentWinds, tt.descent) {
				t.Errorf("descent winds = %+v\nwant             %+v", r.DescentWinds, tt.descent)
			}
			if !reflect.DeepEqual(r.RouteWinds, tt.route) {
				t.Errorf("route winds = %+v\nwant           %+v", r.RouteWinds, tt.route)
			}
		})
	}
}
