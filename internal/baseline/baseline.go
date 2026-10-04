// Package baseline records the parser output for a fixed sample of stored
// messages and compares later parser output against it. It is the regression
// gate for parser changes: any added, removed or changed result is reported,
// and nothing passes until the baseline is deliberately re-recorded.
//
// A baseline is a directory of JSON Lines files, one per ACARS label
// (label_<label>.jsonl), each line holding one Case, plus a manifest.json that
// records how the sample was drawn and how many cases each file holds. Load
// rejects a baseline whose files do not match its manifest, so a deleted or
// truncated file cannot make the gate pass with less coverage.
package baseline

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// ManifestFile is the name of the file that describes how a baseline was drawn.
const ManifestFile = "manifest.json"

// Expectation is one parser's result for a message, stored as canonical JSON.
type Expectation struct {
	Parser string          `json:"parser"`
	Type   string          `json:"type"`
	Result json.RawMessage `json:"result"`
}

// Case is one sampled message and the results the parsers produced for it when
// the baseline was recorded.
type Case struct {
	ID        int64  `json:"id"`
	Timestamp string `json:"timestamp,omitempty"`
	Label     string `json:"label"`
	Tail      string `json:"tail,omitempty"`
	Text      string `json:"text"`
	// Stratum is the "<label>/<stored parser type>" group the case was sampled from.
	Stratum  string        `json:"stratum"`
	Expected []Expectation `json:"expected"`
}

// Manifest records how a baseline sample was drawn, so that it can be
// reproduced, and what it contains, so that it can be validated.
type Manifest struct {
	Source     string `json:"source"`      // The table the sample was drawn from.
	Cutoff     string `json:"cutoff"`      // Only messages at or before this UTC time were sampled.
	PerStratum int    `json:"per_stratum"` // The maximum number of cases per stratum.
	Ordering   string `json:"ordering"`    // The deterministic ordering used within a stratum.
	Strata     int    `json:"strata"`      // The number of strata sampled.
	SampledBy  string `json:"sampled_by"`  // The build that drew the sample (it defines the strata).
	RecordedBy string `json:"recorded_by"` // The build that recorded the current expectations.

	// Set by Save.
	Cases int            `json:"cases"` // The total number of cases.
	Files map[string]int `json:"files"` // The number of cases in each label file.
}

// Message converts the case into the ACARS message that the parsers receive.
func (c Case) Message() *acars.Message {
	// The stored corpus's flight column came from Airframes' flight record,
	// not the transmission, so it is deliberately not part of a case.
	return &acars.Message{
		ID:        acars.FlexInt64(c.ID),
		Timestamp: c.Timestamp,
		Label:     c.Label,
		Text:      c.Text,
		Tail:      c.Tail,
	}
}

// Observe converts dispatch matches into expectations.
func Observe(matches []registry.Match) ([]Expectation, error) {
	out := make([]Expectation, 0, len(matches))
	for _, m := range matches {
		b, err := json.Marshal(m.Result)
		if err != nil {
			return nil, fmt.Errorf("marshal %s result: %w", m.Parser, err)
		}
		out = append(out, Expectation{Parser: m.Parser, Type: m.Result.Type(), Result: b})
	}
	return out, nil
}

// Compare returns a sorted, human-readable list of every difference between the
// expected and the observed results, keyed by parser name. An empty list means
// the results are identical. Object key order does not matter; every added,
// removed or changed parser or field is a difference.
func Compare(want, got []Expectation) []string {
	wantBy := byParser(want)
	gotBy := byParser(got)

	var diffs []string
	for name, w := range wantBy {
		g, ok := gotBy[name]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("parser %q removed", name))
			continue
		}
		if w.Type != g.Type {
			diffs = append(diffs, fmt.Sprintf("parser %q type changed: %s -> %s", name, w.Type, g.Type))
		}
		for _, d := range compareJSON(w.Result, g.Result) {
			diffs = append(diffs, fmt.Sprintf("parser %q %s", name, d))
		}
	}
	for name := range gotBy {
		if _, ok := wantBy[name]; !ok {
			diffs = append(diffs, fmt.Sprintf("parser %q added", name))
		}
	}
	sort.Strings(diffs)
	return diffs
}

func byParser(es []Expectation) map[string]Expectation {
	m := make(map[string]Expectation, len(es))
	for _, e := range es {
		m[e.Parser] = e
	}
	return m
}

// compareJSON flattens two JSON documents into JSON Pointer paths (RFC 6901)
// and reports the fields that were added, removed or changed. Arrays are
// compared as whole values.
func compareJSON(want, got json.RawMessage) []string {
	w := flatten(want)
	g := flatten(got)

	var diffs []string
	for path, wv := range w {
		gv, ok := g[path]
		switch {
		case !ok:
			diffs = append(diffs, fmt.Sprintf("field %q removed: %s", path, wv))
		case gv != wv:
			diffs = append(diffs, fmt.Sprintf("field %q changed: %s -> %s", path, wv, gv))
		}
	}
	for path, gv := range g {
		if _, ok := w[path]; !ok {
			diffs = append(diffs, fmt.Sprintf("field %q added: %s", path, gv))
		}
	}
	return diffs
}

// flatten decodes a JSON document and returns its leaf values as compact JSON,
// keyed by JSON Pointer path. Escaping "~" and "/" in keys keeps every path
// unambiguous, so {"a/b":1} and {"a":{"b":1}} never collide. A document that is
// not an object is returned under the empty path.
func flatten(doc json.RawMessage) map[string]string {
	out := make(map[string]string)
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber() // Keep numbers exactly as written.
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		out[""] = string(doc)
		return out
	}
	flattenValue("", v, out)
	return out
}

// pointerEscaper escapes a key as a JSON Pointer reference token.
var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

func flattenValue(prefix string, v interface{}, out map[string]string) {
	if obj, ok := v.(map[string]interface{}); ok && len(obj) > 0 {
		for k, child := range obj {
			flattenValue(prefix+"/"+pointerEscaper.Replace(k), child, out)
		}
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", v))
	}
	out[prefix] = string(b)
}

// labelFileName returns the file name for a label's cases. Upper-case letters
// and digits are kept, and every other byte is written as %XX. The name is
// therefore always a single, safe path element, and labels that differ only in
// case (the corpus has "1Z" and "1z") never share a file on a case-insensitive
// file system.
func labelFileName(label string) string {
	if label == "" {
		return "label__empty.jsonl"
	}
	var b strings.Builder
	for i := 0; i < len(label); i++ {
		c := label[i]
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return "label_" + b.String() + ".jsonl"
}

// Save replaces the baseline in dir with the given cases and manifest. The new
// baseline is written in full to a staging directory next to dir and then
// swapped in, so a failure part-way through leaves the previous baseline
// intact. The manifest's case and file counts are set from the cases. Output is
// deterministic for a given set of cases and manifest.
func Save(dir string, cases []Case, m Manifest) error {
	dir = filepath.Clean(dir)
	staging := dir + ".staging"
	previous := dir + ".previous"

	// Recover from an interrupted swap first: if the baseline is missing but
	// a moved-aside copy exists, restore it, so that it is never deleted
	// before a replacement is in place.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if _, err := os.Stat(previous); err == nil {
			if err := os.Rename(previous, dir); err != nil {
				return fmt.Errorf("restore the baseline from an interrupted save: %w", err)
			}
		}
	}

	if err := os.RemoveAll(staging); err != nil {
		return fmt.Errorf("clear %s: %w", staging, err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", staging, err)
	}

	files, err := writeCases(staging, cases)
	if err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	m.Cases = len(cases)
	m.Files = files
	if err := writeManifest(staging, m); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}

	// Swap the staging directory into place, keeping the previous baseline
	// until the new one is in position. A leftover moved-aside copy is stale
	// at this point, because the baseline itself exists.
	if err := os.RemoveAll(previous); err != nil {
		return fmt.Errorf("clear %s: %w", previous, err)
	}
	hadPrevious := false
	if _, err := os.Stat(dir); err == nil {
		if err := os.Rename(dir, previous); err != nil {
			return fmt.Errorf("move aside %s: %w", dir, err)
		}
		hadPrevious = true
	}
	if err := os.Rename(staging, dir); err != nil {
		if hadPrevious {
			if rollbackErr := os.Rename(previous, dir); rollbackErr != nil {
				return fmt.Errorf("install %s: %w; restoring the previous baseline also failed (it is in %s): %v",
					dir, err, previous, rollbackErr)
			}
		}
		return fmt.Errorf("install %s: %w", dir, err)
	}
	if hadPrevious {
		if err := os.RemoveAll(previous); err != nil {
			return fmt.Errorf("remove %s: %w", previous, err)
		}
	}
	return nil
}

// writeCases writes the cases one file per label, ordered by ID then stratum,
// and returns the number of cases in each file.
func writeCases(dir string, cases []Case) (map[string]int, error) {
	byFile := make(map[string][]Case)
	for _, c := range cases {
		if c.Expected == nil {
			c.Expected = []Expectation{} // Written as [] rather than null.
		}
		name := labelFileName(c.Label)
		byFile[name] = append(byFile[name], c)
	}

	counts := make(map[string]int, len(byFile))
	for name, fileCases := range byFile {
		sort.Slice(fileCases, func(i, j int) bool {
			if fileCases[i].ID != fileCases[j].ID {
				return fileCases[i].ID < fileCases[j].ID
			}
			return fileCases[i].Stratum < fileCases[j].Stratum
		})

		var buf bytes.Buffer
		for _, c := range fileCases {
			b, err := json.Marshal(c)
			if err != nil {
				return nil, fmt.Errorf("marshal case %d: %w", c.ID, err)
			}
			buf.Write(b)
			buf.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
		counts[name] = len(fileCases)
	}
	return counts, nil
}

func writeManifest(dir string, m Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ManifestFile), append(b, '\n'), 0o644)
}

// Load reads and validates the baseline in dir. Cases are ordered by label file
// name and then by their order within each file. It fails if the manifest is
// missing, or if the label files present, or the number of cases in any of
// them, differ from the manifest.
func Load(dir string) ([]Case, Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		return nil, m, fmt.Errorf("read manifest: %w", err)
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, m, fmt.Errorf("parse manifest: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(dir, "label_*.jsonl"))
	if err != nil {
		return nil, m, err
	}
	sort.Strings(files)

	var cases []Case
	seen := make(map[string]bool, len(files))
	ids := make(map[int64]string)
	for _, f := range files {
		name := filepath.Base(f)
		want, listed := m.Files[name]
		if !listed {
			return nil, m, fmt.Errorf("%s is not listed in the manifest", name)
		}
		fileCases, err := loadFile(f)
		if err != nil {
			return nil, m, err
		}
		if len(fileCases) != want {
			return nil, m, fmt.Errorf("%s holds %d cases, the manifest records %d", name, len(fileCases), want)
		}
		for _, c := range fileCases {
			if err := validateCase(c, name); err != nil {
				return nil, m, err
			}
			if other, dup := ids[c.ID]; dup {
				return nil, m, fmt.Errorf("message %d appears in both %s and %s", c.ID, other, name)
			}
			ids[c.ID] = name
		}
		seen[name] = true
		cases = append(cases, fileCases...)
	}
	for name := range m.Files {
		if !seen[name] {
			return nil, m, fmt.Errorf("%s is listed in the manifest but missing", name)
		}
	}
	if len(cases) != m.Cases {
		return nil, m, fmt.Errorf("baseline holds %d cases, the manifest records %d", len(cases), m.Cases)
	}
	return cases, m, nil
}

// validateCase checks that a loaded case is well formed and belongs in the
// file it was read from, so that rows replaced with null or copied between
// files cannot keep the counts while losing coverage.
func validateCase(c Case, file string) error {
	if c.ID == 0 {
		return fmt.Errorf("%s: a case has no message ID (a null or empty row?)", file)
	}
	if labelFileName(c.Label) != file {
		return fmt.Errorf("%s: message %d has label %q, which belongs in %s", file, c.ID, c.Label, labelFileName(c.Label))
	}
	if c.Stratum == "" {
		return fmt.Errorf("%s: message %d has no stratum", file, c.ID)
	}
	parsers := make(map[string]bool, len(c.Expected))
	for _, e := range c.Expected {
		if e.Parser == "" {
			return fmt.Errorf("%s: message %d has an expectation with no parser", file, c.ID)
		}
		if parsers[e.Parser] {
			return fmt.Errorf("%s: message %d has two expectations from parser %q", file, c.ID, e.Parser)
		}
		parsers[e.Parser] = true
	}
	return nil
}

func loadFile(path string) ([]Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var cases []Case
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // Some messages and results are large.
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var c Case
		if err := json.Unmarshal([]byte(text), &c); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		cases = append(cases, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return cases, nil
}
