package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func raw(t *testing.T, s string) json.RawMessage {
	t.Helper()
	return json.RawMessage(s)
}

func TestCompareIdenticalResultsHaveNoDiffs(t *testing.T) {
	want := []Expectation{{Parser: "pdc", Type: "pdc", Result: raw(t, `{"a":1,"b":"x"}`)}}
	// Same content with a different key order must compare equal.
	got := []Expectation{{Parser: "pdc", Type: "pdc", Result: raw(t, `{"b":"x","a":1}`)}}

	if diffs := Compare(want, got); len(diffs) != 0 {
		t.Errorf("Compare() = %v, want no diffs", diffs)
	}
}

func TestCompareReportsEveryKindOfChange(t *testing.T) {
	want := []Expectation{
		{Parser: "kept", Type: "kept", Result: raw(t, `{"alt":100,"nested":{"x":1}}`)},
		{Parser: "removed", Type: "removed", Result: raw(t, `{}`)},
	}
	got := []Expectation{
		{Parser: "kept", Type: "kept", Result: raw(t, `{"alt":200,"nested":{"x":1},"extra":true}`)},
		{Parser: "added", Type: "added", Result: raw(t, `{}`)},
	}

	diffs := Compare(want, got)

	wantDiffs := []string{
		`parser "added" added`,
		`parser "kept" field "alt" changed: 100 -> 200`,
		`parser "kept" field "extra" added: true`,
		`parser "removed" removed`,
	}
	if len(diffs) != len(wantDiffs) {
		t.Fatalf("Compare() = %v, want %v", diffs, wantDiffs)
	}
	for i := range diffs {
		if diffs[i] != wantDiffs[i] {
			t.Errorf("diff[%d] = %q, want %q", i, diffs[i], wantDiffs[i])
		}
	}
}

func TestCompareReportsNestedFieldRemoval(t *testing.T) {
	want := []Expectation{{Parser: "p", Type: "p", Result: raw(t, `{"route":{"origin":"YSSY","dest":"YMML"}}`)}}
	got := []Expectation{{Parser: "p", Type: "p", Result: raw(t, `{"route":{"origin":"YSSY"}}`)}}

	diffs := Compare(want, got)

	if len(diffs) != 1 || diffs[0] != `parser "p" field "route.dest" removed: "YMML"` {
		t.Errorf("Compare() = %v", diffs)
	}
}

func TestWriteThenLoadRoundTripsAndIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	cases := []Case{
		{ID: 2, Label: "H1", Text: "second", Stratum: "H1/pdc",
			Expected: []Expectation{{Parser: "pdc", Type: "pdc", Result: raw(t, `{"b":1,"a":2}`)}}},
		{ID: 1, Label: "H1", Text: "first", Stratum: "H1/unparsed"},
		{ID: 3, Label: "", Text: "no label", Stratum: "/unparsed"},
	}

	if err := Write(dir, cases); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Cases come back ordered by label file name ("label_H1" sorts before
	// "label__empty", as 'H' < '_'), then by ID within a file.
	var ids []int64
	for _, c := range loaded {
		ids = append(ids, c.ID)
	}
	if got, want := ids, []int64{1, 2, 3}; !equalIDs(got, want) {
		t.Errorf("loaded IDs = %v, want %v", got, want)
	}

	// Writing the loaded cases again produces byte-identical files.
	dir2 := t.TempDir()
	if err := Write(dir2, loaded); err != nil {
		t.Fatalf("second Write() error = %v", err)
	}
	for _, name := range []string{"label__empty.jsonl", "label_H1.jsonl"} {
		a := readFile(t, filepath.Join(dir, name))
		b := readFile(t, filepath.Join(dir2, name))
		if a != b {
			t.Errorf("%s differs between writes:\n%s\n---\n%s", name, a, b)
		}
	}
}

func TestLabelFileNameIsSafe(t *testing.T) {
	// Upper-case letters and digits are kept; every other byte is written as
	// %XX, so names stay distinct on case-insensitive file systems.
	tests := map[string]string{
		"H1": "label_H1.jsonl",
		"":   "label__empty.jsonl",
		"_d": "label_%5F%64.jsonl",
		"5Z": "label_5Z.jsonl",
		"1z": "label_1%7A.jsonl",
		"/x": "label_%2F%78.jsonl",
	}
	for label, want := range tests {
		if got := labelFileName(label); got != want {
			t.Errorf("labelFileName(%q) = %q, want %q", label, got, want)
		}
		if strings.ContainsAny(labelFileName(label), `/\`) {
			t.Errorf("labelFileName(%q) contains a path separator", label)
		}
	}
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestLabelsDifferingOnlyInCaseGetDistinctFiles guards against one label's
// cases overwriting another's on a case-insensitive file system (such as the
// macOS default), since the corpus has labels like "1Z" and "1z".
func TestLabelsDifferingOnlyInCaseGetDistinctFiles(t *testing.T) {
	pairs := [][2]string{{"1Z", "1z"}, {"4S", "4s"}, {"HQ", "Hq"}, {"_D", "_d"}}
	for _, p := range pairs {
		a, b := labelFileName(p[0]), labelFileName(p[1])
		if strings.EqualFold(a, b) {
			t.Errorf("labelFileName(%q) = %q and labelFileName(%q) = %q collide when case is ignored", p[0], a, p[1], b)
		}
	}

	dir := t.TempDir()
	cases := []Case{
		{ID: 1, Label: "1Z", Text: "upper", Stratum: "1Z/unparsed"},
		{ID: 2, Label: "1z", Text: "lower", Stratum: "1z/unparsed"},
	}
	if err := Write(dir, cases); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("loaded %d cases, want 2 (a label's cases were overwritten)", len(loaded))
	}
}
