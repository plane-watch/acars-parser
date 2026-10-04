package aircrafttype

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// Regenerate doc8643.csv from ICAO's public service.
//go:generate go run ./internal/fetchdoc8643 -out doc8643.csv

// doc8643CSV is the ICAO Doc 8643 list of aircraft type designators, fetched
// from ICAO by internal/fetchdoc8643. Its comment header records the source
// and fetch date.
//
//go:embed doc8643.csv
var doc8643CSV string

// designators is the set of ICAO aircraft type designators that Normalise
// will return. Values shaped like designators are often something else in
// older parser output (flight levels such as "F240", checksum fragments such
// as "C8AE", callsign fragments such as "BAW8"), so only designators in the
// ICAO list are accepted.
//
// Membership reduces such false matches but cannot eliminate them: the ICAO
// list itself contains designators that look like words ("ATIS", "WIND"),
// flight levels ("F100") or hex (332 are four hex digits). The protection
// against junk is that parsers capture types only from labelled fields.
var designators = mustLoadDesignators(doc8643CSV)

// mustLoadDesignators reads the designator column of the embedded CSV. The
// file is part of the build, so a malformed file is a programming error.
func mustLoadDesignators(data string) map[string]bool {
	set, err := loadDesignators(data)
	if err != nil {
		panic(fmt.Sprintf("aircrafttype: embedded doc8643.csv: %v", err))
	}
	return set
}

func loadDesignators(data string) (map[string]bool, error) {
	r := csv.NewReader(strings.NewReader(data))
	r.Comment = '#'
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if len(header) == 0 || header[0] != "designator" {
		return nil, fmt.Errorf("unexpected header %v", header)
	}
	set := make(map[string]bool)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if d := strings.TrimSpace(rec[0]); d != "" {
			set[d] = true
		}
	}
	// The full list has about 2,600 designators; far fewer means the file
	// is incomplete.
	if len(set) < minDesignators {
		return nil, fmt.Errorf("only %d designators, want at least %d", len(set), minDesignators)
	}
	return set, nil
}

// minDesignators is the fewest distinct designators a complete list has.
const minDesignators = 2000
