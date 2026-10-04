package aircrafttype

import "testing"

// The raw values are aircraft types as transmitted and captured by the
// parsers in the January 2026 corpus.
func TestNormalise(t *testing.T) {
	tests := []struct {
		raw    string
		want   string
		wantOK bool
	}{
		// Already ICAO designators.
		{"B738", "B738", true},
		{"E75L", "E75L", true},
		{"CRJ9", "CRJ9", true},
		{"BCS3", "BCS3", true},
		{" a320 ", "A320", true},

		// Designator followed by a cabin or cargo configuration.
		{"A333-BCS3", "A333", true},
		{"A332-BCS1", "A332", true},

		// Boeing model names, with or without the B prefix and engine suffix.
		{"B737-800", "B738", true},
		{"B737-700", "B737", true},
		{"737-800 CFM56-7B26", "B738", true},
		{"737-700 CFM56-7B24", "B737", true},
		{"B7378MAX", "B38M", true},
		{"767-300 PW4060", "B763", true},
		{"777-200 PW4090-3", "B772", true},
		{"777-200 GE90-94B", "B772", true},
		{"787-8 GENX-1B70", "B788", true},
		{"787-9 GENX-1B76A", "B789", true},
		{"787-10 GENX-1B76", "B78X", true},

		// Airbus model names: the series gives the designator.
		{"A330-323", "A333", true},
		{"A330-223", "A332", true},
		{"A321-271", "A321", true},
		{"A320-232", "A320", true},
		{"A319-131", "A319", true},
		{"A321-271N", "A21N", true},
		{"A350-941", "A359", true},

		// IATA codes with a single ICAO designator.
		{"388", "A388", true},
		{"32N", "A20N", true},

		// Ambiguous: more than one designator is possible, so none is given.
		{"AT7", "", false},
		{"737", "", false},
		{"B777-DHK1", "", false},
		{"B777", "", false},
		{"A330", "", false},
		{"A350-BCS1", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := Normalise(tt.raw)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("Normalise(%q) = %q, %v, want %q, %v", tt.raw, got, ok, tt.want, tt.wantOK)
		}
	}
}
