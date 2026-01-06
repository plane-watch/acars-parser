package patterns

import (
	"math"
	"testing"
)

// almostEqual checks if two floats are equal within a tolerance.
func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestParseDMSCoord(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		degDigits int
		dir       string
		want      float64
		tolerance float64
	}{
		// DDMMD format (5 digits, tenths of minutes) - latitude.
		{
			name:      "DDMMD latitude north",
			input:     "34138", // 34 deg 13.8 min
			degDigits: 2,
			dir:       "N",
			want:      34.23, // 34 + 13.8/60
			tolerance: 0.001,
		},
		{
			name:      "DDMMD latitude south",
			input:     "34138",
			degDigits: 2,
			dir:       "S",
			want:      -34.23,
			tolerance: 0.001,
		},
		// DDDMMD format (6 digits, tenths of minutes) - longitude.
		{
			name:      "DDDMMD longitude east",
			input:     "151235", // 151 deg 23.5 min
			degDigits: 3,
			dir:       "E",
			want:      151.391667, // 151 + 23.5/60
			tolerance: 0.001,
		},
		{
			name:      "DDDMMD longitude west",
			input:     "151235",
			degDigits: 3,
			dir:       "W",
			want:      -151.391667,
			tolerance: 0.001,
		},
		// DDMMSS format (6 digits, seconds) - latitude.
		{
			name:      "DDMMSS latitude north",
			input:     "341348", // 34 deg 13 min 48 sec
			degDigits: 2,
			dir:       "N",
			want:      34.23, // 34 + 13/60 + 48/3600
			tolerance: 0.001,
		},
		// DDDMMSS format (7 digits, seconds) - longitude.
		{
			name:      "DDDMMSS longitude east",
			input:     "1512335", // 151 deg 23 min 35 sec
			degDigits: 3,
			dir:       "E",
			want:      151.393056, // 151 + 23/60 + 35/3600
			tolerance: 0.001,
		},
		// DDMM.M format (with decimal point) - latitude.
		{
			name:      "DDMM.M latitude north",
			input:     "3413.8", // 34 deg 13.8 min
			degDigits: 2,
			dir:       "N",
			want:      34.23,
			tolerance: 0.001,
		},
		{
			name:      "DDMM.MM latitude",
			input:     "3413.85",
			degDigits: 2,
			dir:       "N",
			want:      34.230833,
			tolerance: 0.001,
		},
		// DDDMM.M format (with decimal point) - longitude.
		{
			name:      "DDDMM.M longitude west",
			input:     "15123.5", // 151 deg 23.5 min
			degDigits: 3,
			dir:       "W",
			want:      -151.391667,
			tolerance: 0.001,
		},
		// Edge cases.
		{
			name:      "equator",
			input:     "00000",
			degDigits: 2,
			dir:       "N",
			want:      0,
			tolerance: 0.001,
		},
		{
			name:      "prime meridian",
			input:     "000000",
			degDigits: 3,
			dir:       "E",
			want:      0,
			tolerance: 0.001,
		},
		{
			name:      "empty string",
			input:     "",
			degDigits: 2,
			dir:       "N",
			want:      0,
			tolerance: 0.001,
		},
		// Note: DDMMTT (hundredths of minutes) is NOT supported by the shared function.
		// The fst parser uses DDMMTT for 6-digit latitudes, but this conflicts with
		// DDMMSS (seconds) used by label22 for the same format. Since both are valid
		// interpretations, fst keeps its own parseCoord function for this case.
		//
		// Real-world examples from parsers.
		{
			name:      "Sydney latitude",
			input:     "33520", // 33 deg 52.0 min S
			degDigits: 2,
			dir:       "S",
			want:      -33.866667,
			tolerance: 0.001,
		},
		{
			name:      "Sydney longitude",
			input:     "151180", // 151 deg 18.0 min E
			degDigits: 3,
			dir:       "E",
			want:      151.3,
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDMSCoord(tt.input, tt.degDigits, tt.dir)
			if !almostEqual(got, tt.want, tt.tolerance) {
				t.Errorf("ParseDMSCoord(%q, %d, %q) = %v, want %v (tolerance %v)",
					tt.input, tt.degDigits, tt.dir, got, tt.want, tt.tolerance)
			}
		})
	}
}

func TestParseLatitude(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		dir       string
		want      float64
		tolerance float64
	}{
		{"simple north", "34138", "N", 34.23, 0.001},
		{"simple south", "34138", "S", -34.23, 0.001},
		{"decimal north", "3413.8", "N", 34.23, 0.001},
		{"seconds format", "341348", "N", 34.23, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLatitude(tt.value, tt.dir)
			if !almostEqual(got, tt.want, tt.tolerance) {
				t.Errorf("ParseLatitude(%q, %q) = %v, want %v", tt.value, tt.dir, got, tt.want)
			}
		})
	}
}

func TestParseLongitude(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		dir       string
		want      float64
		tolerance float64
	}{
		{"simple east", "151235", "E", 151.391667, 0.001},
		{"simple west", "151235", "W", -151.391667, 0.001},
		{"decimal east", "15123.5", "E", 151.391667, 0.001},
		{"seconds format", "1512335", "E", 151.393056, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLongitude(tt.value, tt.dir)
			if !almostEqual(got, tt.want, tt.tolerance) {
				t.Errorf("ParseLongitude(%q, %q) = %v, want %v", tt.value, tt.dir, got, tt.want)
			}
		})
	}
}

func TestParseDecimalCoord(t *testing.T) {
	tests := []struct {
		name  string
		value string
		dir   string
		want  float64
	}{
		{"positive north", "34.23", "N", 34.23},
		{"positive south", "34.23", "S", -34.23},
		{"positive east", "151.39", "E", 151.39},
		{"positive west", "151.39", "W", -151.39},
		{"empty", "", "N", 0},
		{"invalid", "abc", "N", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDecimalCoord(tt.value, tt.dir)
			if !almostEqual(got, tt.want, 0.001) {
				t.Errorf("ParseDecimalCoord(%q, %q) = %v, want %v", tt.value, tt.dir, got, tt.want)
			}
		})
	}
}