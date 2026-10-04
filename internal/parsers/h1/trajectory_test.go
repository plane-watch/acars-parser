package h1

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestTrajectoryParser_Parse(t *testing.T) {
	parser := &TrajectoryParser{}

	tests := []struct {
		name           string
		tail           string
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
			tail: "N8747Q",
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
			tail: "N8951S",
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
			// The registration field holds "XXX", not a registration.
			name: "76502 format B737-800",
			tail: "N8315C",
			text: "++76502,XXX,B737-800,260111,WN0297,KMDW,KLAX,1175,SW2501\r\n3\r\n" +
				"N4148.2,W08828.3,110221,19322,-36.3,262,048,CL,00000,0,\r\n" +
				"N4148.1,W08830.9,110221,20125,-36.7,258,047,CL,00000,0,\r\n" +
				"N4148.1,W08833.5,110222,20945,-37.0,255,057,CL,00000,0,\r\n:\r\n",
			wantReg:        "",
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
				Tail:  tt.tail,
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
	if pos.WindDirection != 269 {
		t.Errorf("WindDirection = %d, want 269", pos.WindDirection)
	}
	if pos.WindSpeed != 117 {
		t.Errorf("WindSpeed = %d, want 117", pos.WindSpeed)
	}
	if pos.Time != "070342" {
		t.Errorf("Time = %q, want 070342 (DDHHMM)", pos.Time)
	}
}

// TestTrajectoryParser_RegistrationField checks that the header's
// registration field is reported only when it is the transmitted tail. It
// also holds fleet numbers ("201"), "XXX" and truncated registrations
// (" N8852" for N8852Q), which must not become a registration, for example
// when the message has no tail.
func TestTrajectoryParser_RegistrationField(t *testing.T) {
	const body = "\r\n1\r\nN3640.3,W09644.9,070342,33998,-48.3,269,117,ER,00000,0,\r\n:\r\n"
	tests := []struct{ field, tail, want string }{
		{"N8951S", "N8951S", "N8951S"},
		{"N8951S", "", ""},
		{"201", "N201LV", ""},
		{" N8852", "N8852Q", ""},
	}
	for _, tt := range tests {
		text := "++86501," + tt.field + ",B7378MAX,260107,WN2545,KMCO,KDEN,0059,SMX34-2502-F320" + body
		r, ok := (&TrajectoryParser{}).Parse(&acars.Message{ID: 1, Label: "H1", Tail: tt.tail, Text: text}).(*TrajectoryResult)
		if !ok {
			t.Fatalf("field %q: Parse returned no result", tt.field)
		}
		if r.Registration != tt.want {
			t.Errorf("field %q, tail %q: Registration = %q, want %q", tt.field, tt.tail, r.Registration, tt.want)
		}
	}
}

// TestTrajectoryParser_RejectsInvalidAirports checks that a header whose
// airports are not plausible ICAO codes is not parsed.
func TestTrajectoryParser_RejectsInvalidAirports(t *testing.T) {
	text := "++86501,N8951S,B7378MAX,260107,WN2545,KMCO,XXXX,0059,SMX34-2502-F320\r\n1\r\n" +
		"N3640.3,W09644.9,070342,33998,-48.3,269,117,ER,00000,0,\r\n:\r\n"
	if r := (&TrajectoryParser{}).Parse(&acars.Message{ID: 1, Label: "H1", Tail: "N8951S", Text: text}); r != nil {
		t.Errorf("Parse = %+v, want nil", r)
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

// TestTrajectoryParser_TraceMatchesParse checks that tracing reports a
// match only when Parse does: a header with an invalid airport matches the
// regexes but is not parsed.
func TestTrajectoryParser_TraceMatchesParse(t *testing.T) {
	text := "++86501,N8951S,B7378MAX,260107,WN2545,KMCO,XXXX,0059,SMX34-2502-F320\r\n1\r\n" +
		"N3640.3,W09644.9,070342,33998,-48.3,269,117,ER,00000,0,\r\n:\r\n"
	msg := &acars.Message{ID: 1, Label: "H1", Tail: "N8951S", Text: text}
	if trace := (&TrajectoryParser{}).ParseWithTrace(msg); trace.Matched {
		t.Error("ParseWithTrace reports a match that Parse rejects")
	}
}

// TestTrajectoryParser_UnitedABS checks United's ABS reports, which use the
// same samples under another header (real message, cut short). The header
// names no flight, and its type field ("B737", followed by part of the
// registration) does not identify a model, so no type is reported.
func TestTrajectoryParser_UnitedABS(t *testing.T) {
	text := "ABS026AA_N37510,B737N37-1260104,UA    ,KPHX,KIAH,0878,BCG2E-S200-0009\r\n" +
		"N3107.9,W10040.9,041759,29898,-37.5,256,066,DC,00000,0,\r\n" +
		"N3107.0,W10030.5,041800,28991,-35.5,258,062,DC,00000,0,\r\n"
	r, ok := (&TrajectoryParser{}).Parse(&acars.Message{ID: 1, Label: "H1", Tail: "N37510", Text: text}).(*TrajectoryResult)
	if !ok {
		t.Fatal("Parse returned no result")
	}
	if r.Report != "ABS026" || r.Registration != "N37510" || r.AircraftType != "" || r.FlightNumber != "" ||
		r.Date != "260104" || r.Origin != "KPHX" || r.Destination != "KIAH" || r.SystemID != "BCG2E-S200-0009" || len(r.Positions) != 2 {
		t.Errorf("got %+v", r)
	}
	if p := r.Positions[0]; p.Time != "041759" || p.WindDirection != 256 || p.WindSpeed != 66 || p.Phase != "DC" {
		t.Errorf("first sample %+v", p)
	}
}
