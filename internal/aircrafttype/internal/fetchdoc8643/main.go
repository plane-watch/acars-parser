// Command fetchdoc8643 downloads the ICAO aircraft type designators (ICAO Doc
// 8643) from ICAO's public service and writes them as the CSV file that the
// aircrafttype package embeds. Run it through go generate:
//
//	go generate ./internal/aircrafttype
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// sourceURL is ICAO's public Doc 8643 service. It returns every entry as JSON
// in response to an empty POST.
const sourceURL = "https://doc8643.icao.int/external/aircrafttypes"

const (
	// maxResponseBytes bounds the response; the full list is about 1.6 MB.
	maxResponseBytes = 64 << 20
	// minDesignators is the fewest distinct designators accepted; the full
	// list has about 2,600.
	minDesignators = 2000
	// maxDropFraction is the largest acceptable fall in the number of
	// distinct designators compared with the existing file.
	maxDropFraction = 0.10
)

// designatorRe matches an ICAO aircraft type designator.
var designatorRe = regexp.MustCompile(`^[A-Z0-9]{2,4}$`)

type entry struct {
	Designator       string `json:"Designator"`
	ManufacturerCode string `json:"ManufacturerCode"`
	ModelFullName    string `json:"ModelFullName"`
	Description      string `json:"Description"`
	WTC              string `json:"WTC"`
}

func main() {
	out := flag.String("out", "doc8643.csv", "Output CSV file")
	flag.Parse()

	if err := run(*out); err != nil {
		fmt.Fprintf(os.Stderr, "fetchdoc8643: %v\n", err)
		os.Exit(1)
	}
}

func run(out string) error {
	client := &http.Client{Timeout: 2 * time.Minute}
	req, err := http.NewRequest(http.MethodPost, sourceURL, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", sourceURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: %s", sourceURL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(body) > maxResponseBytes {
		return fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	}

	var entries []entry
	if err := json.Unmarshal(body, &entries); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if err := validate(entries, countExisting(out)); err != nil {
		return fmt.Errorf("refusing to replace %s: %w", out, err)
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		for _, pair := range [][2]string{
			{a.Designator, b.Designator}, {a.ManufacturerCode, b.ManufacturerCode},
			{a.ModelFullName, b.ModelFullName}, {a.Description, b.Description}, {a.WTC, b.WTC},
		} {
			if pair[0] != pair[1] {
				return pair[0] < pair[1]
			}
		}
		return false
	})

	// Write to a temporary file beside the output and rename it into place,
	// so that a failure never leaves a truncated list.
	f, err := os.CreateTemp(filepath.Dir(out), ".doc8643-*.csv")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmp) // A no-op once the file has been renamed.
	}()

	// Comment lines record the provenance; the aircrafttype package skips them.
	if _, err := fmt.Fprintf(f,
		"# ICAO Doc 8643 aircraft type designators.\n"+
			"# Source: %s\n"+
			"# Fetched: %s (%d entries). Regenerate with: go generate ./internal/aircrafttype\n",
		sourceURL, time.Now().UTC().Format("2006-01-02"), len(entries)); err != nil {
		return err
	}

	w := csv.NewWriter(f)
	if err := w.Write([]string{"designator", "manufacturer", "model", "description", "wtc"}); err != nil {
		return err
	}
	for _, e := range entries {
		if err := w.Write([]string{e.Designator, e.ManufacturerCode, e.ModelFullName, e.Description, e.WTC}); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, out)
}

// validate checks a fetched list before it replaces the existing file: every
// entry has a valid designator, there are enough distinct designators, and
// there are not markedly fewer than in the existing file (existing is 0 when
// there is none).
func validate(entries []entry, existing int) error {
	distinct := make(map[string]bool)
	for i, e := range entries {
		if !designatorRe.MatchString(e.Designator) {
			return fmt.Errorf("entry %d has an invalid designator %q", i, e.Designator)
		}
		distinct[e.Designator] = true
	}
	if len(distinct) < minDesignators {
		return fmt.Errorf("only %d distinct designators, want at least %d", len(distinct), minDesignators)
	}
	if existing > 0 && float64(len(distinct)) < float64(existing)*(1-maxDropFraction) {
		return fmt.Errorf("%d distinct designators is a drop of more than %.0f%% from the existing %d",
			len(distinct), maxDropFraction*100, existing)
	}
	return nil
}

// countExisting returns the number of distinct designators in the existing
// CSV file, or 0 if it cannot be read.
func countExisting(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	r.Comment = '#'
	records, err := r.ReadAll()
	if err != nil || len(records) < 2 {
		return 0
	}
	distinct := make(map[string]bool)
	for _, rec := range records[1:] {
		distinct[rec[0]] = true
	}
	return len(distinct)
}
