package parsers

import (
	"flag"
	"fmt"
	"sort"
	"strings"
	"testing"

	"acars_parser/internal/baseline"
	"acars_parser/internal/registry"
)

// updateBaseline re-records the expectations from the current parsers. Use it
// only after reviewing the differences the test reports:
//
//	go test ./internal/parsers -run TestBaseline -update-baseline
var updateBaseline = flag.Bool("update-baseline", false, "re-record the parser regression baseline")

// baselineDir holds the fixtures written by `acars_parser baseline`.
const baselineDir = "testdata/baseline"

// maxReportedCases limits how many failing cases are printed in full.
const maxReportedCases = 25

// TestBaseline is the parser regression gate. It re-parses every message in the
// recorded sample and fails on any added, removed or changed result.
func TestBaseline(t *testing.T) {
	cases, err := baseline.Load(baselineDir)
	if err != nil {
		t.Fatalf("load baseline: %v", err)
	}
	if len(cases) == 0 {
		t.Fatalf("no baseline cases in %s; record them with `acars_parser baseline`", baselineDir)
	}

	reg := registry.Default()
	reg.Sort()

	var failures []string
	changedByParser := make(map[string]int)
	for i := range cases {
		got, err := baseline.Observe(reg.Dispatch(cases[i].Message()))
		if err != nil {
			t.Fatalf("message %d: %v", cases[i].ID, err)
		}

		if *updateBaseline {
			cases[i].Expected = got
			continue
		}

		diffs := baseline.Compare(cases[i].Expected, got)
		if len(diffs) == 0 {
			continue
		}
		for _, d := range diffs {
			changedByParser[parserOfDiff(d)]++
		}
		failures = append(failures, fmt.Sprintf("message %d (stratum %s):\n    %s",
			cases[i].ID, cases[i].Stratum, strings.Join(diffs, "\n    ")))
	}

	if *updateBaseline {
		if err := baseline.Write(baselineDir, cases); err != nil {
			t.Fatalf("write baseline: %v", err)
		}
		t.Logf("re-recorded %d baseline cases", len(cases))
		return
	}

	if len(failures) == 0 {
		return
	}
	shown := failures
	if len(shown) > maxReportedCases {
		shown = shown[:maxReportedCases]
	}
	t.Errorf("%d of %d baseline messages changed. Differences by parser: %s\n\n%s",
		len(failures), len(cases), summariseCounts(changedByParser), strings.Join(shown, "\n"))
	if len(failures) > maxReportedCases {
		t.Errorf("... and %d more changed messages", len(failures)-maxReportedCases)
	}
	t.Error("review the differences; if every one is intended, re-record with: " +
		"go test ./internal/parsers -run TestBaseline -update-baseline")
}

// parserOfDiff extracts the parser name from a baseline.Compare difference,
// which always begins `parser "<name>"`.
func parserOfDiff(d string) string {
	rest := strings.TrimPrefix(d, `parser "`)
	if i := strings.IndexByte(rest, '"'); i >= 0 {
		return rest[:i]
	}
	return d
}

// summariseCounts formats counts as "a=3, b=1", highest first.
func summariseCounts(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = fmt.Sprintf("%s=%d", n, counts[n])
	}
	return strings.Join(parts, ", ")
}
