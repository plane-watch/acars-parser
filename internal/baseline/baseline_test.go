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
		`parser "kept" field "/alt" changed: 100 -> 200`,
		`parser "kept" field "/extra" added: true`,
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

	if len(diffs) != 1 || diffs[0] != `parser "p" field "/route/dest" removed: "YMML"` {
		t.Errorf("Compare() = %v", diffs)
	}
}

// TestCompareDistinguishesStructurallyDifferentDocuments guards against field
// paths colliding: a key containing the separator must not be confused with a
// nested object, and neither may an empty key.
func TestCompareDistinguishesStructurallyDifferentDocuments(t *testing.T) {
	pairs := [][2]string{
		{`{"a.b":1}`, `{"a":{"b":1}}`},
		{`{"a/b":1}`, `{"a":{"b":1}}`},
		{`{"":{"a":1}}`, `{"a":1}`},
		{`{"a":{}}`, `{"a":null}`},
		{`{"a":[1,2]}`, `{"a":[2,1]}`},
	}
	for _, p := range pairs {
		want := []Expectation{{Parser: "p", Type: "p", Result: raw(t, p[0])}}
		got := []Expectation{{Parser: "p", Type: "p", Result: raw(t, p[1])}}
		if diffs := Compare(want, got); len(diffs) == 0 {
			t.Errorf("Compare(%s, %s) reported no differences", p[0], p[1])
		}
	}
}

func testManifest() Manifest {
	return Manifest{Source: "test", Cutoff: "2026-01-01 00:00:00", PerStratum: 10, Ordering: "id"}
}

func TestSaveThenLoadRoundTripsAndIsDeterministic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "baseline")
	cases := []Case{
		{ID: 2, Label: "H1", Text: "second", Stratum: "H1/pdc",
			Expected: []Expectation{{Parser: "pdc", Type: "pdc", Result: raw(t, `{"b":1,"a":2}`)}}},
		{ID: 1, Label: "H1", Text: "first", Stratum: "H1/unparsed"},
		{ID: 3, Label: "", Text: "no label", Stratum: "/unparsed"},
	}

	if err := Save(dir, cases, testManifest()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, manifest, err := Load(dir)
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
	if manifest.Cases != 3 || manifest.Files["label_H1.jsonl"] != 2 || manifest.Files["label__empty.jsonl"] != 1 {
		t.Errorf("manifest counts = %d cases, files %v", manifest.Cases, manifest.Files)
	}

	// Saving the loaded cases again produces byte-identical files.
	dir2 := filepath.Join(t.TempDir(), "baseline")
	if err := Save(dir2, loaded, manifest); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	for _, name := range []string{"label__empty.jsonl", "label_H1.jsonl", ManifestFile} {
		a := readFile(t, filepath.Join(dir, name))
		b := readFile(t, filepath.Join(dir2, name))
		if a != b {
			t.Errorf("%s differs between saves:\n%s\n---\n%s", name, a, b)
		}
	}
}

func TestSaveReplacesThePreviousBaselineCompletely(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "baseline")
	first := []Case{{ID: 1, Label: "H1", Stratum: "H1/x"}, {ID: 2, Label: "RA", Stratum: "RA/x"}}
	if err := Save(dir, first, testManifest()); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	second := []Case{{ID: 3, Label: "H1", Stratum: "H1/x"}}
	if err := Save(dir, second, testManifest()); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}

	loaded, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != 3 {
		t.Errorf("loaded %v, want only case 3", loaded)
	}
	if _, err := os.Stat(filepath.Join(dir, "label_RA.jsonl")); !os.IsNotExist(err) {
		t.Errorf("stale label_RA.jsonl remains (stat error %v)", err)
	}
	// No staging or backup directories are left behind.
	entries, _ := os.ReadDir(filepath.Dir(dir))
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("parent directory holds %v, want only the baseline", names)
	}
}

func TestLoadRejectsABaselineThatDoesNotMatchItsManifest(t *testing.T) {
	setup := func(t *testing.T) string {
		dir := filepath.Join(t.TempDir(), "baseline")
		cases := []Case{
			{ID: 1, Label: "H1", Stratum: "H1/x"},
			{ID: 2, Label: "H1", Stratum: "H1/x"},
			{ID: 3, Label: "RA", Stratum: "RA/x"},
		}
		if err := Save(dir, cases, testManifest()); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		return dir
	}

	tests := []struct {
		name   string
		damage func(t *testing.T, dir string)
	}{
		{"label file deleted", func(t *testing.T, dir string) {
			mustRemove(t, filepath.Join(dir, "label_RA.jsonl"))
		}},
		{"cases removed from a file", func(t *testing.T, dir string) {
			lines := strings.SplitAfter(readFile(t, filepath.Join(dir, "label_H1.jsonl")), "\n")
			mustWrite(t, filepath.Join(dir, "label_H1.jsonl"), lines[0])
		}},
		{"unlisted label file added", func(t *testing.T, dir string) {
			mustWrite(t, filepath.Join(dir, "label_ZZ.jsonl"), `{"id":9,"label":"ZZ","text":"","stratum":"ZZ/x","expected":[]}`+"\n")
		}},
		{"manifest deleted", func(t *testing.T, dir string) {
			mustRemove(t, filepath.Join(dir, ManifestFile))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setup(t)
			tt.damage(t, dir)
			if _, _, err := Load(dir); err == nil {
				t.Error("Load() succeeded on a damaged baseline")
			}
		})
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

	dir := filepath.Join(t.TempDir(), "baseline")
	cases := []Case{
		{ID: 1, Label: "1Z", Text: "upper", Stratum: "1Z/unparsed"},
		{ID: 2, Label: "1z", Text: "lower", Stratum: "1z/unparsed"},
	}
	if err := Save(dir, cases, testManifest()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("loaded %d cases, want 2 (a label's cases were overwritten)", len(loaded))
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

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}
