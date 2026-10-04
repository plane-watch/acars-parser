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
		// Boeing names the 737 MAX models with a single-digit variant; the
		// LEAP-1B engine is fitted only to the MAX.
		{"737-8 LEAP-1B28", "B38M", true},
		{"737-9 LEAP-1B28", "B39M", true},
		{"737-8", "B38M", true},
		// A MAX variant with a 737 NG engine is contradictory.
		{"737-8 CFM56-7B26", "", false},
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

		// Embraer E2 model names: the suffix is part of the model, not a
		// configuration.
		{"E195-E2", "E295", true},
		{"E190-E2", "E290", true},

		// B737 is itself a designator (the 737-700).
		{"B737", "B737", true},

		// Values shaped like designators that are not on the list of ICAO
		// designators are not normalised: they include flight levels,
		// checksum fragments and words seen in older parser output.
		{"TEST", "", false},
		{"NONE", "", false},
		{"A220", "", false},
		{"F240", "", false},
		{"C8AE", "", false},
		{"BAW8", "", false},
		{"A330-BCS1", "", false},

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

// TestMappingTargetsAreListedDesignators keeps the mapping tables consistent
// with the designator list, so that a mapping cannot return an unlisted type.
func TestMappingTargetsAreListedDesignators(t *testing.T) {
	var targets []string
	for _, m := range []map[string]string{iataCodes, boeingVariants, boeingMax, airbusSeries, embraerE2} {
		for _, d := range m {
			targets = append(targets, d)
		}
	}
	for _, fam := range airbusFamily {
		targets = append(targets, fam[0], fam[1])
	}
	for _, d := range targets {
		if !designators[d] {
			t.Errorf("mapping target %q is not in the designator list", d)
		}
	}
}

// TestDesignatorsComeFromDoc8643 checks that the designator list is the
// embedded ICAO Doc 8643 data rather than a hand-written list.
func TestDesignatorsComeFromDoc8643(t *testing.T) {
	if len(designators) < 2000 {
		t.Fatalf("designators holds %d entries, want the full ICAO list (over 2,000)", len(designators))
	}
	tests := map[string]bool{
		"B461": true, "B463": true, "E75L": true, "GA6C": true, "B3XM": true, "P8": true,
		// Not ICAO designators (each was once in a hand-written list).
		"B731": false, "BA46": false, "E175": false, "GL6T": false,
	}
	for d, want := range tests {
		if designators[d] != want {
			t.Errorf("designators[%q] = %v, want %v", d, designators[d], want)
		}
	}
}

func TestLoadDesignatorsRejectsAnIncompleteFile(t *testing.T) {
	if _, err := loadDesignators("# comment\ndesignator,manufacturer,model,description,wtc\nB738,BOEING,737-800,L2J,M\n"); err == nil {
		t.Error("loadDesignators accepted a file with one designator")
	}
}
