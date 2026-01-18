package fst

import (
	"acars_parser/internal/acars"
	"strings"
	"testing"
)

func TestFSTBasicParse(t *testing.T) {
	parser := &Parser{}
	msg := &acars.Message{Text: "FST01EGLLOMAAN418071E0214075390 245 145M 57C 3828713613851713682504540050"}
	res := parser.Parse(msg)
	result, ok := res.(*Result)
	if !ok {
		t.Fatalf("Expected *Result, got %T", res)
	}
	if result.Origin != "EGLL" {
		t.Errorf("Expected origin EGLL, got %s", result.Origin)
	}
	if result.Destination != "OMAA" {
		t.Errorf("Expected destination OMAA, got %s", result.Destination)
	}
	// Dodajte ovde dodatne provere za polja koja su važna za vaš slučaj
}

func TestFSTCoordinates_418071_0214075(t *testing.T) {
	parser := &Parser{}
	msg := &acars.Message{Text: "FST01EGLLOMAAN418071E0214075390 245 145M 57C 3828713613851713682504540050"}
	res := parser.Parse(msg)
	result, ok := res.(*Result)
	if !ok {
		t.Fatalf("Expected *Result, got %T", res)
	}

	t.Logf("Raw input: %s", msg.Text)
	t.Logf("Parsed coordinates: Lat=%.6f, Lon=%.6f", result.Latitude, result.Longitude)

	expectedLat := 41.8071
	expectedLon := 21.4075

	if result.Latitude < expectedLat-0.0001 || result.Latitude > expectedLat+0.0001 {
		t.Errorf("Expected latitude ≈ %.4f, got %.6f", expectedLat, result.Latitude)
	}
	if result.Longitude < expectedLon-0.0001 || result.Longitude > expectedLon+0.0001 {
		t.Errorf("Expected longitude ≈ %.4f, got %.6f", expectedLon, result.Longitude)
	}
}

func TestFSTCoordinates_463712_0208236(t *testing.T) {
	parser := &Parser{}
	msg := &acars.Message{Text: "FST01EGLLZSPDN463712E0208236350 784 181 M59C 2103411211946911600000511449"}
	res := parser.Parse(msg)
	result, ok := res.(*Result)
	if !ok {
		t.Fatalf("Expected *Result, got %T", res)
	}

	t.Logf("Raw input: %s", msg.Text)
	t.Logf("Parsed coordinates: Lat=%.6f, Lon=%.6f", result.Latitude, result.Longitude)

	expectedLat := 46.3712
	expectedLon := 20.8236

	if result.Latitude < expectedLat-0.0001 || result.Latitude > expectedLat+0.0001 {
		t.Errorf("Expected latitude ≈ %.4f, got %.6f", expectedLat, result.Latitude)
	}
	if result.Longitude < expectedLon-0.0001 || result.Longitude > expectedLon+0.0001 {
		t.Errorf("Expected longitude ≈ %.4f, got %.6f", expectedLon, result.Longitude)
	}

	if result.Origin != "EGLL" {
		t.Errorf("Expected origin EGLL, got %s", result.Origin)
	}
	if result.Destination != "ZSPD" {
		t.Errorf("Expected destination ZSPD, got %s", result.Destination)
	}
	if result.FlightLevel != 350 {
		t.Errorf("Expected flight level 350, got %d", result.FlightLevel)
	}
}

func TestFSTTemperature(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"M59C", "FST01EGLLZSPDN463712E0208236350 784 181 M59C 2103411211946911600000511449", -59},
		{"M59C_2", "FST01EGLLZSPDN470036E0191236350 797 169 M59C 2900911011847611600000511439", -59},
		{"M62C", "FST01EGLLZSPDN475956E0174268350 809 157 M62C 3403510811746511600000511429", -62},
		{"M56C", "FST01EGKKOPISN458392E0229894370 465 180 M56C 1634711011749411600017151211", -56},
		{"M55C", "FST01EGKKOPISN464425E0212284370 475 169 M55C 2934510811649411600017151201", -55},
		{"M65C", "FST01VOBLEGLLN477510E0170613380 157 650 M65C 931028729145511600012581112", -65},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &Parser{}
			msg := &acars.Message{Text: tt.input}
			res := parser.Parse(msg)
			result, ok := res.(*Result)
			if !ok {
				t.Fatalf("Expected *Result, got %T", res)
			}
			if result.Temperature != tt.expected {
				t.Errorf("Expected temperature %d, got %d", tt.expected, result.Temperature)
			}
			t.Logf("Temperature: %d°C", result.Temperature)
		})
	}
}

func TestFSTAllFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Example1", "FST01EGLLOMAAN418071E0214075390 245 145M 57C 3828713613851713682504540050"},
		{"Example2", "FST01EGLLZSPDN463712E0208236350 784 181 M59C 2103411211946911600000511449"},
		{"Example3", "FST01EGLLZSPDN470036E0191236350 797 169 M59C 2900911011847611600000511439"},
		{"Example4", "FST01EGLLZSPDN475956E0174268350 809 157 M62C 3403510811746511600000511429"},
		{"Example5", "FST01EGKKOPISN458392E0229894370 465 180 M56C 1634711011749411600017151211"},
		{"Example6", "FST01EGKKOPISN464425E0212284370 475 169 M55C 2934510811649411600017151201"},
		{"Example7", "FST01VOBLEGLLN477510E0170613380 157 650 M65C 931028729145511600012581112"},
		{"Example8_India_UK", "FST01VOBLEGLLN458091E0237397380 197 610 M56C 1834029630046111600012571032"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &Parser{}
			msg := &acars.Message{Text: tt.input}
			res := parser.Parse(msg)
			result, ok := res.(*Result)
			if !ok {
				t.Fatalf("Expected *Result, got %T", res)
			}
			t.Logf("Input: %s", tt.input)
			t.Logf("Origin=%s Dest=%s", result.Origin, result.Destination)
			t.Logf("FL=%d Heading=%d Unknown1=%d Track=%d GS=%d WindSpeed=%d WindDir=%d Temp=%d°C",
				result.FlightLevel, result.Heading, result.Unknown1, result.Track, result.GroundSpeed,
				result.WindSpeed, result.WindDirection, result.Temperature)

			// Prikaži sve raw fieldove
			rest := tt.input[len("FST01"):]
			rest = rest[12:] // preskoci origin+dest+coords
			fields := strings.Fields(rest)
			t.Logf("Raw fields: %v", fields)
		})
	}
}
