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
		{
			// Some reports leave a waypoint empty (5,903 with no current
			// waypoint in the archive).
			name: "empty current waypoint",
			text: "POSN53234W003058,,093649,410,MCT,094042,XAPOS,M60,34640,164/TS093649,1004264982",
			want: H1PosResult{Latitude: 53.39, Longitude: -3.096667, ReportTime: "093649", FlightLevel: 410,
				NextWaypoint: "MCT", ThirdWaypoint: "XAPOS", ETA: "094042",
				Temperature: -60, WindDir: 346, WindSpeed: 40},
		},
		{
			name: "empty third waypoint",
			text: "POSN42353W071502,BASEZ,034000,154,JFUND,034006,,M1,27437,2019674",
			want: H1PosResult{Latitude: 42.588333, Longitude: -71.836667, ReportTime: "034000", FlightLevel: 154,
				CurrentWaypoint: "BASEZ", NextWaypoint: "JFUND", ETA: "034006",
				Temperature: -1, WindDir: 274, WindSpeed: 37},
		},
		{
			// Waypoints named by a place, bearing and distance.
			name: "place-bearing-distance waypoints",
			text: "POSN25113E056046,NOLSU196-0022,034033,113,REXEV196-0027,034109,IMPIV196-0033,P9,10317,88/TS034033,0210267AD1",
			want: H1PosResult{Latitude: 25.188333, Longitude: 56.076667, ReportTime: "034033", FlightLevel: 113,
				CurrentWaypoint: "NOLSU196-0022", NextWaypoint: "REXEV196-0027", ThirdWaypoint: "IMPIV196-0033", ETA: "034109",
				Temperature: 9, WindDir: 103, WindSpeed: 17},
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

func TestH1PosParserRejectsNumericWaypoints(t *testing.T) {
	text := "POSN39006W075260,12345,014938,340,SMELI,015155,TRPOD,M52,290061,204D6A3"
	if got := (&H1PosParser{}).Parse(&acars.Message{ID: 1, Label: "H1", Text: text}); got != nil {
		t.Errorf("Parse() = %+v, want nil for an all-digit waypoint", got)
	}
}

// TestH1PosParserRouteLayout checks the longer layout that adds a distance
// before the ETA and ends with speeds, fuel and the route. Its first
// waypoint may be a runway or an altitude point ("1000").
func TestH1PosParserRouteLayout(t *testing.T) {
	tests := []struct {
		name, text                     string
		origin, dest, curr, next, wpt3 string
		fl                             int
	}{
		{
			name:   "named waypoints",
			text:   "POSN33005W096222,WIGIS,004904,143,JAYXX,27,005245,TRYTN,M4,281040,380K,305K,1429,162,KDAL,KBWI,,69,030134,1051,73/PR1429,238,390,162,,44,80,271060,M58,180,P0,P0/RI:DA:KDAL:AA:KBWI..JAYXX:D:EMMTT4.LOOS",
			origin: "KDAL", dest: "KBWI", curr: "WIGIS", next: "JAYXX", wpt3: "TRYTN", fl: 143,
		},
		{
			name:   "altitude point",
			text:   "POSN33125W096475,1000,035309,170,AKUNA,15,035525,TIKYS,M2,261028,354K,268K,1224,156,KDAL,KSTL,,112,050034,474,73/PR1224",
			origin: "KDAL", dest: "KSTL", curr: "1000", next: "AKUNA", wpt3: "TIKYS", fl: 170,
		},
		{
			name:   "runway",
			text:   "POSN30174W098116,RW36R,045504,187,PAYDA,1,045509,MUCKY,M12,285050,367K,275K,1406,286,KAUS,KLAX,06R,144,074940,1086,73/PR1406",
			origin: "KAUS", dest: "KLAX", curr: "RW36R", next: "PAYDA", wpt3: "MUCKY", fl: 187,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ok := (&H1PosParser{}).Parse(&acars.Message{ID: 1, Label: "H1", Text: tt.text}).(*H1PosResult)
			if !ok {
				t.Fatal("Parse returned no result")
			}
			if r.Origin != tt.origin || r.Destination != tt.dest || r.CurrentWaypoint != tt.curr ||
				r.NextWaypoint != tt.next || r.ThirdWaypoint != tt.wpt3 || r.FlightLevel != tt.fl {
				t.Errorf("got %+v", *r)
			}
		})
	}
}

// TestH1PosParserEmptyNextWaypoint checks a report whose next waypoint, ETA
// and third waypoint are empty.
func TestH1PosParserEmptyNextWaypoint(t *testing.T) {
	r, ok := (&H1PosParser{}).Parse(&acars.Message{ID: 1, Label: "H1",
		Text: "POSN38378W076050,PRNCZ,054324,334,,,,M53,28086,1223753"}).(*H1PosResult)
	if !ok {
		t.Fatal("Parse returned no result")
	}
	if r.CurrentWaypoint != "PRNCZ" || r.NextWaypoint != "" || r.ETA != "" || r.FlightLevel != 334 || r.Temperature != -53 {
		t.Errorf("got %+v", *r)
	}
}
