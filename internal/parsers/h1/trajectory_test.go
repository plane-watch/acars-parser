package h1

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestTrajectoryParser_Parse(t *testing.T) {
	parser := &TrajectoryParser{}

	tests := []struct {
		name           string
		text           string
		wantReg        string
		wantType       string
		wantFlight     string
		wantOrigin     string
		wantDest       string
		wantPositions  int
		wantFirstPhase string
	}{
		{
			name: "WN0057 KDAL-KHOU climb",
			text: "++86501,N8747Q,B7378MAX,260107,WN0057,KDAL,KHOU,0208,SMX34-2502-F320\r\n6\r\n" +
				"N3248.3,W09658.6,070355,10498, 05.3,271,029,CL,00000,0,\r\n" +
				"N3246.5,W09658.4,070355,10963, 04.5,270,027,CL,00000,0,\r\n" +
				"N3244.7,W09658.2,070355,11917, 02.8,271,022,CL,00000,0,\r\n" +
				"N3242.8,W09657.9,070356,12815, 00.8,289,023,CL,00000,0,\r\n" +
				"N3240.8,W09657.7,070356,13624,-00.3,297,028,CL,00000,0,\r\n" +
				"N3238.8,W09657.3,070356,14714,-02.5,284,030,CL,00000,0,\r\n:\r\n",
			wantReg:        "N8747Q",
			wantType:       "B7378MAX",
			wantFlight:     "WN0057",
			wantOrigin:     "KDAL",
			wantDest:       "KHOU",
			wantPositions:  6,
			wantFirstPhase: "CL",
		},
		{
			name: "WN2545 KMCO-KDEN enroute",
			text: "++86501,N8951S,B7378MAX,260107,WN2545,KMCO,KDEN,0059,SMX34-2502-F320\r\n6\r\n" +
				"N3640.3,W09644.9,070342,33998,-48.3,269,117,ER,00000,0,\r\n" +
				"N3643.4,W09706.1,070345,33999,-48.8,270,114,ER,00000,0,\r\n" +
				"N3649.7,W09726.5,070348,34002,-48.5,268,115,ER,00000,0,\r\n" +
				"N3656.7,W09746.9,070351,34001,-49.0,269,116,ER,00000,0,\r\n" +
				"N3703.6,W09807.3,070354,34001,-49.0,269,115,ER,00000,0,\r\n" +
				"N3710.4,W09827.7,070357,34001,-49.0,268,116,ER,00000,0,\r\n:\r\n",
			wantReg:        "N8951S",
			wantType:       "B7378MAX",
			wantFlight:     "WN2545",
			wantOrigin:     "KMCO",
			wantDest:       "KDEN",
			wantPositions:  6,
			wantFirstPhase: "ER",
		},
		{
			name: "76502 format B737-800",
			text: "++76502,XXX,B737-800,260111,WN0297,KMDW,KLAX,1175,SW2501\r\n3\r\n" +
				"N4148.2,W08828.3,110221,19322,-36.3,262,048,CL,00000,0,\r\n" +
				"N4148.1,W08830.9,110221,20125,-36.7,258,047,CL,00000,0,\r\n" +
				"N4148.1,W08833.5,110222,20945,-37.0,255,057,CL,00000,0,\r\n:\r\n",
			wantReg:        "XXX",
			wantType:       "B737-800",
			wantFlight:     "WN0297",
			wantOrigin:     "KMDW",
			wantDest:       "KLAX",
			wantPositions:  3,
			wantFirstPhase: "CL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &acars.Message{
				ID:    12345,
				Label: "H1",
				Text:  tt.text,
			}

			result := parser.Parse(msg)
			if result == nil {
				t.Fatal("expected result, got nil")
			}

			tr, ok := result.(*TrajectoryResult)
			if !ok {
				t.Fatalf("expected *TrajectoryResult, got %T", result)
			}

			if tr.Registration != tt.wantReg {
				t.Errorf("Registration = %q, want %q", tr.Registration, tt.wantReg)
			}
			if tr.AircraftType != tt.wantType {
				t.Errorf("AircraftType = %q, want %q", tr.AircraftType, tt.wantType)
			}
			if tr.FlightNumber != tt.wantFlight {
				t.Errorf("FlightNumber = %q, want %q", tr.FlightNumber, tt.wantFlight)
			}
			if tr.Origin != tt.wantOrigin {
				t.Errorf("Origin = %q, want %q", tr.Origin, tt.wantOrigin)
			}
			if tr.Destination != tt.wantDest {
				t.Errorf("Destination = %q, want %q", tr.Destination, tt.wantDest)
			}
			if len(tr.Positions) != tt.wantPositions {
				t.Errorf("Positions count = %d, want %d", len(tr.Positions), tt.wantPositions)
			}
			if len(tr.Positions) > 0 && tr.Positions[0].Phase != tt.wantFirstPhase {
				t.Errorf("First position phase = %q, want %q", tr.Positions[0].Phase, tt.wantFirstPhase)
			}
		})
	}
}

func TestTrajectoryParser_LatLonParsing(t *testing.T) {
	parser := &TrajectoryParser{}

	// Test message with known coordinates.
	text := "++86501,N8951S,B7378MAX,260107,WN2545,KMCO,KDEN,0059,SMX34-2502-F320\r\n6\r\n" +
		"N3640.3,W09644.9,070342,33998,-48.3,269,117,ER,00000,0,\r\n:\r\n"

	msg := &acars.Message{ID: 1, Label: "H1", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	tr := result.(*TrajectoryResult)
	if len(tr.Positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(tr.Positions))
	}

	pos := tr.Positions[0]

	// N3640.3 = 36 degrees + 40.3 minutes = 36 + 40.3/60 = 36.6717
	wantLat := 36.0 + 40.3/60.0
	if diff := pos.Latitude - wantLat; diff > 0.001 || diff < -0.001 {
		t.Errorf("Latitude = %f, want ~%f", pos.Latitude, wantLat)
	}

	// W09644.9 = -(96 degrees + 44.9 minutes) = -(96 + 44.9/60) = -96.7483
	wantLon := -(96.0 + 44.9/60.0)
	if diff := pos.Longitude - wantLon; diff > 0.001 || diff < -0.001 {
		t.Errorf("Longitude = %f, want ~%f", pos.Longitude, wantLon)
	}

	if pos.Altitude != 33998 {
		t.Errorf("Altitude = %d, want 33998", pos.Altitude)
	}
	if pos.Temperature != -48.3 {
		t.Errorf("Temperature = %f, want -48.3", pos.Temperature)
	}
	if pos.Heading != 269 {
		t.Errorf("Heading = %d, want 269", pos.Heading)
	}
	if pos.Speed != 117 {
		t.Errorf("Speed = %d, want 117", pos.Speed)
	}
}

func TestTrajectoryParser_QuickCheck(t *testing.T) {
	parser := &TrajectoryParser{}

	tests := []struct {
		text string
		want bool
	}{
		{"++86501,N8747Q,B7378MAX", true},
		{"++76502,N8747Q,B7378MAX", true},
		{"++12345,N8747Q,B7378MAX", false},
		{"Some other message", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text[:20], got, tt.want)
		}
	}
}