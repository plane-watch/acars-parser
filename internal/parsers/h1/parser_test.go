package h1

import (
	"encoding/json"
	"testing"

	"acars_parser/internal/acars"
)

func TestParseWaypointCoords(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLat float64
		wantLon float64
	}{
		{
			name:    "VELOX coordinates",
			input:   "N33490E034050",
			wantLat: 33.816666666666666, // 33° 49.0' N
			wantLon: 34.083333333333336, // 034° 05.0' E
		},
		{
			name:    "MUVIN coordinates",
			input:   "N31490E035327",
			wantLat: 31.816666666666666, // 31° 49.0' N
			wantLon: 35.545,             // 035° 32.7' E
		},
		{
			name:    "Western hemisphere",
			input:   "N37312W102468",
			wantLat: 37.52,              // 37° 31.2' N
			wantLon: -102.78,            // 102° 46.8' W
		},
		{
			name:    "Southern hemisphere",
			input:   "S33520E151180",
			wantLat: -33.866666666666667, // 33° 52.0' S
			wantLon: 151.3,               // 151° 18.0' E
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLat, gotLon := parseWaypointCoords(tt.input)

			// Allow small floating point tolerance.
			if abs(gotLat-tt.wantLat) > 0.01 {
				t.Errorf("parseWaypointCoords(%q) lat = %v, want %v", tt.input, gotLat, tt.wantLat)
			}
			if abs(gotLon-tt.wantLon) > 0.01 {
				t.Errorf("parseWaypointCoords(%q) lon = %v, want %v", tt.input, gotLon, tt.wantLon)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestFPNParseWithCoordinates(t *testing.T) {
	testText := `FPN/FNRJA111/RP:DA:OJAI:AA:EGLL:F:MUVIN,N31490E035327.L53..TAPUZ,N32020E034314.W13..VELOX,N33490E034050.N71..DESPO,N34269E034229`

	msg := &acars.Message{
		ID:    1,
		Label: "H1",
		Text:  testText,
	}

	parser := &FPNParser{}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("Failed to parse FPN message")
	}

	fpn, ok := result.(*FPNResult)
	if !ok {
		t.Fatal("Result is not FPNResult")
	}

	// Check that we got waypoints with coordinates.
	if len(fpn.Waypoints) == 0 {
		t.Fatal("No waypoints parsed")
	}

	// Find VELOX and check its coordinates.
	var velox *RouteWaypoint
	for i := range fpn.Waypoints {
		if fpn.Waypoints[i].Name == "VELOX" {
			velox = &fpn.Waypoints[i]
			break
		}
	}

	if velox == nil {
		t.Fatal("VELOX waypoint not found")
	}

	// VELOX should be at N33° 49.0' E034° 05.0'
	expectedLat := 33.816666666666666
	expectedLon := 34.083333333333336

	if abs(velox.Latitude-expectedLat) > 0.01 {
		t.Errorf("VELOX latitude = %v, want %v", velox.Latitude, expectedLat)
	}
	if abs(velox.Longitude-expectedLon) > 0.01 {
		t.Errorf("VELOX longitude = %v, want %v", velox.Longitude, expectedLon)
	}

	// Print full result for visual inspection.
	jsonBytes, _ := json.MarshalIndent(fpn, "", "  ")
	t.Logf("Parsed result:\n%s", string(jsonBytes))
}

func TestDetectTruncation(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		waypoints []RouteWaypoint
		route     string
		want      bool
	}{
		{
			name:      "normal complete message",
			text:      "FPN/SN123:DA:KSFO:AA:KLAX:F:WAYP1..WAYP2",
			waypoints: []RouteWaypoint{{Name: "WAYP1"}, {Name: "WAYP2"}},
			route:     "WAYP1..WAYP2",
			want:      false,
		},
		{
			name:      "multi-part without terminator",
			text:      "FPN/SN123#M1:DA:KSFO:AA:KLAX",
			waypoints: nil,
			route:     "",
			want:      true,
		},
		{
			name:      "multi-part with terminator",
			text:      "FPN/SN123#M1:DA:KSFO#MD:AA:KLAX",
			waypoints: nil,
			route:     "",
			want:      false,
		},
		{
			name: "all waypoints have coords",
			text: "FPN:DA:KSFO:AA:KLAX:F:WPT1,N33490E034050..WPT2,N34000E035000",
			waypoints: []RouteWaypoint{
				{Name: "WPT1", Latitude: 33.8, Longitude: 34.0},
				{Name: "WPT2", Latitude: 34.0, Longitude: 35.0},
			},
			route: "WPT1,N33490E034050..WPT2,N34000E035000",
			want:  false,
		},
		{
			name:      "ends with colon",
			text:      "FPN:DA:KSFO:AA:KLAX:",
			waypoints: nil,
			route:     "",
			want:      true,
		},
		{
			name:      "ends with comma",
			text:      "FPN:DA:KSFO:AA:KLAX:F:WAYP1,",
			waypoints: nil,
			route:     "WAYP1,",
			want:      true,
		},
		{
			name:      "ends with double period",
			text:      "FPN:DA:KSFO:AA:KLAX:F:WAYP1..",
			waypoints: nil,
			route:     "WAYP1..",
			want:      true,
		},
		{
			name:      "incomplete coordinate after comma",
			text:      "FPN:DA:KSFO:AA:KLAX:F:WAYP1,N334",
			waypoints: nil,
			route:     "WAYP1,N334",
			want:      true,
		},
		{
			name:      "complete coordinate after comma",
			text:      "FPN:DA:KSFO:AA:KLAX:F:WAYP1,N33490E034050",
			waypoints: nil,
			route:     "WAYP1,N33490E034050",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectTruncation(tt.text, tt.waypoints, tt.route)
			if got != tt.want {
				t.Errorf("detectTruncation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestH1PosParse_WindSpeedThreeDigits(t *testing.T) {
	// Example where wind is encoded as DDDSSS (3-digit speed with leading zero): 255044 => 255° / 44 kts.
	msg := &acars.Message{
		ID:        1,
		Label:     "H1",
		Text:      "POSN43451E017323,VRANA,032901,370,PETAK,034717,PINDO,M46,255044,141C21C",
		Timestamp: "2025-10-06T04:45:10.137514Z",
		Tail:      "TEST",
	}

	parser := &H1PosParser{}
	res := parser.Parse(msg)
	if res == nil {
		t.Fatalf("expected parse result, got nil")
	}

	pos, ok := res.(*H1PosResult)
	if !ok {
		t.Fatalf("expected *H1PosResult, got %T", res)
	}

	if pos.FlightLevel != 370 {
		t.Fatalf("flight level = %d, want 370", pos.FlightLevel)
	}
	if pos.Temperature != -46 {
		t.Fatalf("temperature = %d, want -46", pos.Temperature)
	}
	if pos.WindDir != 255 {
		t.Fatalf("wind_dir = %d, want 255", pos.WindDir)
	}
	if pos.WindSpeed != 44 {
		t.Fatalf("wind_speed = %d, want 44", pos.WindSpeed)
	}
}
