package label26

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser(t *testing.T) {
	testCases := []struct {
		name string
		text string
		want struct {
			format      string
			flightLevel int
			reportTime  string
			originICAO  string
			destICAO    string
			fuel        int
			temp        int
			windDir     int
			windSpeed   int
			lat         float64
			lon         float64
			eta         string
			waypoint    string
			altitude    int
		}
	}{
		{
			name: "ETA AFL1346 negative temperature high altitude",
			text: "ETA01AFL1346 /17181015UUEEULMK FUEL 74 TEMP- 58 WDIR34627 WSPD 24 LATN 64.087 LONE 35.290 ETA1105 TUR ALT 33973",
			want: struct {
				format      string
				flightLevel int
				reportTime  string
				originICAO  string
				destICAO    string
				fuel        int
				temp        int
				windDir     int
				windSpeed   int
				lat         float64
				lon         float64
				eta         string
				waypoint    string
				altitude    int
			}{
				format:      "ETA01",
				flightLevel: 339,
				reportTime:  "17:18:10",
				originICAO:  "UUEE",
				destICAO:    "ULMK",
				fuel:        74,
				temp:        -58,
				windDir:     346,
				windSpeed:   24,
				lat:         64.087,
				lon:         35.290,
				eta:         "11:05",
				waypoint:    "",
				altitude:    33973,
			},
		},
		{
			name: "ETA AFL1842 positive temperature low altitude",
			text: "ETA01AFL1842 /17181023UUEEUMMS FUEL 79 TEMP 1 WDIR19515 WSPD 11 LATN 54.100 LONE 27.891 ETA1032 TUR ALT 3834",
			want: struct {
				format      string
				flightLevel int
				reportTime  string
				originICAO  string
				destICAO    string
				fuel        int
				temp        int
				windDir     int
				windSpeed   int
				lat         float64
				lon         float64
				eta         string
				waypoint    string
				altitude    int
			}{
				format:      "ETA01",
				flightLevel: 38,
				reportTime:  "17:18:10",
				originICAO:  "UUEE",
				destICAO:    "UMMS",
				fuel:        79,
				temp:        1,
				windDir:     195,
				windSpeed:   11,
				lat:         54.100,
				lon:         27.891,
				eta:         "10:32",
				waypoint:    "",
				altitude:    3834,
			},
		},
		{
			name: "ALT AFL1842 with turbulence indicator",
			text: "ALT01AFL1842 /17181021UUEEUMMS FUEL 79 TEMP WDIR16168 WSPD 7 LATN 54.145 LONE 28.001 ETA1033 TUR ALT 4562",
			want: struct {
				format      string
				flightLevel int
				reportTime  string
				originICAO  string
				destICAO    string
				fuel        int
				temp        int
				windDir     int
				windSpeed   int
				lat         float64
				lon         float64
				eta         string
				waypoint    string
				altitude    int
			}{
				format:      "ALT01",
				flightLevel: 45,
				reportTime:  "17:18:10",
				originICAO:  "UUEE",
				destICAO:    "UMMS",
				fuel:        79,
				temp:        0,
				windDir:     161,
				windSpeed:   7,
				lat:         54.145,
				lon:         28.001,
				eta:         "10:33",
				waypoint:    "",
				altitude:    4562,
			},
		},
		{
			name: "ETA AFL010 low flight level",
			text: "ETA01AFL010    /18190728UUEEULLI    \r\nFUEL   92\r\nTEMP- 55\r\nWDIR 4156\r\nWSPD  28\r\nLATN 58.435\r\nLONE 32.931\r\nETA0758\r\nTUR \r\nALT  31253\r\n\r\n\r\n",
			want: struct {
				format      string
				flightLevel int
				reportTime  string
				originICAO  string
				destICAO    string
				fuel        int
				temp        int
				windDir     int
				windSpeed   int
				lat         float64
				lon         float64
				eta         string
				waypoint    string
				altitude    int
			}{
				format:      "ETA01",
				flightLevel: 312,
				reportTime:  "18:19:07",
				originICAO:  "UUEE",
				destICAO:    "ULLI",
				fuel:        92,
				temp:        -55,
				windDir:     55,
				windSpeed:   28,
				lat:         58.435,
				lon:         32.931,
				eta:         "07:58",
				waypoint:    "",
				altitude:    31253,
			},
		},
	}

	parser := &Parser{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &acars.Message{
				ID:        12345,
				Timestamp: "2024-01-17T18:10:15Z",
				Text:      tc.text,
			}

			if !parser.QuickCheck(msg.Text) {
				t.Errorf("QuickCheck failed for valid message")
			}

			rawResult := parser.Parse(msg)
			if rawResult == nil {
				t.Fatalf("Parse returned nil")
			}

			result, ok := rawResult.(*Result)
			if !ok {
				t.Fatalf("Parse result is not *Result type")
			}

			if result.Format != tc.want.format {
				t.Errorf("Format = %q, want %q", result.Format, tc.want.format)
			}
			if result.FlightLevel != tc.want.flightLevel {
				t.Errorf("FlightLevel = %d, want %d", result.FlightLevel, tc.want.flightLevel)
			}
			if result.ReportTime != tc.want.reportTime {
				t.Errorf("ReportTime = %q, want %q", result.ReportTime, tc.want.reportTime)
			}
			if result.OriginICAO != tc.want.originICAO {
				t.Errorf("OriginICAO = %q, want %q", result.OriginICAO, tc.want.originICAO)
			}
			if result.DestICAO != tc.want.destICAO {
				t.Errorf("DestICAO = %q, want %q", result.DestICAO, tc.want.destICAO)
			}
			if result.FuelOnBoard != tc.want.fuel {
				t.Errorf("FuelOnBoard = %d, want %d", result.FuelOnBoard, tc.want.fuel)
			}
			if result.Temperature != tc.want.temp {
				t.Errorf("Temperature = %d, want %d", result.Temperature, tc.want.temp)
			}
			if result.WindDir != tc.want.windDir {
				t.Errorf("WindDir = %d, want %d", result.WindDir, tc.want.windDir)
			}
			if result.WindSpeed != tc.want.windSpeed {
				t.Errorf("WindSpeed = %d, want %d", result.WindSpeed, tc.want.windSpeed)
			}
			if result.Latitude != tc.want.lat {
				t.Errorf("Latitude = %f, want %f", result.Latitude, tc.want.lat)
			}
			if result.Longitude != tc.want.lon {
				t.Errorf("Longitude = %f, want %f", result.Longitude, tc.want.lon)
			}
			if result.ETA != tc.want.eta {
				t.Errorf("ETA = %q, want %q", result.ETA, tc.want.eta)
			}
			if result.Waypoint != tc.want.waypoint {
				t.Errorf("Waypoint = %q, want %q", result.Waypoint, tc.want.waypoint)
			}
			if result.Altitude != tc.want.altitude {
				t.Errorf("Altitude = %d, want %d", result.Altitude, tc.want.altitude)
			}
		})
	}
}
