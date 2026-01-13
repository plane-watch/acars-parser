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
			altitudeM   int
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
				altitudeM   int
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
				altitudeM:   10355,
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
				altitudeM   int
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
				altitudeM:   1168,
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
				altitudeM   int
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
				altitudeM:   1390,
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
				altitudeM   int
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
				altitudeM:   9525,
			},
		},
		{
			name: "ETA with flight number instead of AFL",
			text: "ETA01SU0245 /19120815UUEEUDYZ FUEL 120 TEMP- 45 WDIR28030 WSPD 35 LATN 52.123 LONE 40.456 ETA1230 ALT 35000",
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
				altitudeM   int
			}{
				format:      "ETA01",
				flightLevel: 350,
				reportTime:  "19:12:08",
				originICAO:  "UUEE",
				destICAO:    "UDYZ",
				fuel:        120,
				temp:        -45,
				windDir:     280,
				windSpeed:   35,
				lat:         52.123,
				lon:         40.456,
				eta:         "12:30",
				waypoint:    "",
				altitudeM:   10668,
			},
		},
		{
			name: "Southern and Western coordinates",
			text: "ETA02AFL2500 /14253042KJORLPLA FUEL 65 TEMP 12 WDIR09025 WSPD 18 LATS 33.500 LONW 118.250 ETA1545 ALT 25000",
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
				altitudeM   int
			}{
				format:      "ETA02",
				flightLevel: 250,
				reportTime:  "14:25:30",
				originICAO:  "KJOR",
				destICAO:    "LPLA",
				fuel:        65,
				temp:        12,
				windDir:     90,
				windSpeed:   18,
				lat:         -33.500,
				lon:         -118.250,
				eta:         "15:45",
				waypoint:    "",
				altitudeM:   7620,
			},
		},
		{
			name: "Missing temperature field",
			text: "ALT01AFL3800 /20141500EGLLLFPG FUEL 88 WDIR22045 WSPD 42 LATN 48.750 LONE 2.350 ETA1620 ALT 38000",
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
				altitudeM   int
			}{
				format:      "ALT01",
				flightLevel: 380,
				reportTime:  "20:14:15",
				originICAO:  "EGLL",
				destICAO:    "LFPG",
				fuel:        88,
				temp:        0,
				windDir:     220,
				windSpeed:   42,
				lat:         48.750,
				lon:         2.350,
				eta:         "16:20",
				waypoint:    "",
				altitudeM:   11582,
			},
		},
		{
			name: "Zero altitude",
			text: "ETA01AFL0050 /08301245KATLJFK FUEL 15 TEMP 22 WDIR18008 WSPD 5 LATN 35.000 LONW 85.000 ETA0910 ALT 5000",
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
				altitudeM   int
			}{
				format:      "ETA01",
				flightLevel: 50,
				reportTime:  "08:30:12",
				originICAO:  "KATL",
				destICAO:    "KJFK",
				fuel:        15,
				temp:        22,
				windDir:     180,
				windSpeed:   5,
				lat:         35.000,
				lon:         -85.000,
				eta:         "09:10",
				waypoint:    "",
				altitudeM:   1524,
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
			if result.AltitudeM != tc.want.altitudeM {
				t.Errorf("AltitudeM = %d, want %d", result.AltitudeM, tc.want.altitudeM)
			}
		})
	}
}
