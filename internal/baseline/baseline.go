// Package baseline records the parser output for a fixed sample of stored
// messages and compares later parser output against it. It is the regression
// gate for parser changes: any added, removed or changed result is reported,
// and nothing passes until the baseline is deliberately re-recorded.
//
// A baseline is a directory of JSON Lines files, one per ACARS label
// (label_<label>.jsonl), each line holding one Case, plus a manifest.json that
// records how the sample was drawn.
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
	Flight    string `json:"flight,omitempty"`
	Text      string `json:"text"`
	// Stratum is the "<label>/<stored parser type>" group the case was sampled from.
	Stratum  string        `json:"stratum"`
	Expected []Expectation `json:"expected"`
}

// Manifest records how a baseline sample was drawn, so that it can be reproduced.
type Manifest struct {
	Source        string `json:"source"`         // The table the sample was drawn from.
	Cutoff        string `json:"cutoff"`         // Only messages at or before this timestamp were sampled.
	PerStratum    int    `json:"per_stratum"`    // The maximum number of cases per stratum.
	Ordering      string `json:"ordering"`       // The deterministic ordering used within a stratum.
	Strata        int    `json:"strata"`         // The number of strata sampled.
	Cases         int    `json:"cases"`          // The number of cases written.
	ParserVersion string `json:"parser_version"` // The build that recorded the expectations.
}

// Message converts the case into the ACARS message that the parsers receive.
func (c Case) Message() *acars.Message {
	msg := &acars.Message{
		ID:        acars.FlexInt64(c.ID),
		Timestamp: c.Timestamp,
		Label:     c.Label,
		Text:      c.Text,
		Tail:      c.Tail,
	}
	if c.Flight != "" {
		msg.Flight = &acars.Flight{Flight: c.Flight}
	}
	return msg
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

// compareJSON flattens two JSON documents into dotted field paths and reports
// the fields that were added, removed or changed. Arrays are compared as whole
// values.
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
// keyed by dotted path. A document that is not an object is returned under the
// empty path.
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

func flattenValue(prefix string, v interface{}, out map[string]string) {
	if obj, ok := v.(map[string]interface{}); ok && len(obj) > 0 {
		for k, child := range obj {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			flattenValue(path, child, out)
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

// Write replaces the baseline cases in dir: existing label files are removed,
// and the cases are written one file per label, ordered by ID then stratum.
// Output is deterministic for a given set of cases.
func Write(dir string, cases []Case) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	old, err := filepath.Glob(filepath.Join(dir, "label_*.jsonl"))
	if err != nil {
		return err
	}
	for _, f := range old {
		if err := os.Remove(f); err != nil {
			return fmt.Errorf("remove stale %s: %w", f, err)
		}
	}

	byFile := make(map[string][]Case)
	for _, c := range cases {
		if c.Expected == nil {
			c.Expected = []Expectation{} // Written as [] rather than null.
		}
		name := labelFileName(c.Label)
		byFile[name] = append(byFile[name], c)
	}

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
				return fmt.Errorf("marshal case %d: %w", c.ID, err)
			}
			buf.Write(b)
			buf.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// Load reads every baseline case in dir, ordered by label file name and then
// by the order within each file.
func Load(dir string) ([]Case, error) {
	files, err := filepath.Glob(filepath.Join(dir, "label_*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	var cases []Case
	for _, f := range files {
		fileCases, err := loadFile(f)
		if err != nil {
			return nil, err
		}
		cases = append(cases, fileCases...)
	}
	return cases, nil
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

// WriteManifest writes the manifest describing how the baseline was drawn.
func WriteManifest(dir string, m Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ManifestFile), append(b, '\n'), 0o644)
}
