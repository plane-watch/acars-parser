package registry

import (
	"testing"

	"acars_parser/internal/acars"
)

// fakeResult is a minimal Result for registry tests.
type fakeResult struct{ typ string }

func (r *fakeResult) Type() string     { return r.typ }
func (r *fakeResult) MessageID() int64 { return 0 }

// fakeParser is a configurable Parser for registry tests.
type fakeParser struct {
	name     string
	labels   []string
	priority int
	matches  bool
}

func (p *fakeParser) Name() string             { return p.name }
func (p *fakeParser) Labels() []string         { return p.labels }
func (p *fakeParser) QuickCheck(_ string) bool { return p.matches }
func (p *fakeParser) Priority() int            { return p.priority }
func (p *fakeParser) Parse(_ *acars.Message) Result {
	if !p.matches {
		return nil
	}
	return &fakeResult{typ: p.name + "_type"}
}

// matchNames returns the parser names of the matches, in order.
func matchNames(matches []Match) []string {
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.Parser
	}
	return names
}

func equalStrings(a, b []string) bool {
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

func TestDispatchReportsParserNames(t *testing.T) {
	r := New()
	r.Register(&fakeParser{name: "label_parser", labels: []string{"H1"}, priority: 10, matches: true})
	r.Register(&fakeParser{name: "global_parser", priority: 10, matches: true})
	r.Register(&fakeParser{name: "non_matching", labels: []string{"H1"}, priority: 20, matches: false})
	r.Sort()

	matches := r.Dispatch(&acars.Message{Label: "H1", Text: "text"})

	want := []string{"label_parser", "global_parser"}
	if got := matchNames(matches); !equalStrings(got, want) {
		t.Fatalf("parser names = %v, want %v", got, want)
	}
	if got := matches[0].Result.Type(); got != "label_parser_type" {
		t.Errorf("matches[0].Result.Type() = %q, want %q", got, "label_parser_type")
	}
}

func TestSortOrdersEqualPrioritiesByName(t *testing.T) {
	// Registered in reverse alphabetical order with equal priorities: the order
	// must not depend on registration (import) order.
	r := New()
	r.Register(&fakeParser{name: "zeta", labels: []string{"RA"}, priority: 50, matches: true})
	r.Register(&fakeParser{name: "alpha", labels: []string{"RA"}, priority: 50, matches: true})
	r.Register(&fakeParser{name: "first", labels: []string{"RA"}, priority: 10, matches: true})
	r.Register(&fakeParser{name: "global_z", priority: 500, matches: true})
	r.Register(&fakeParser{name: "global_a", priority: 500, matches: true})
	r.Sort()

	matches := r.Dispatch(&acars.Message{Label: "RA", Text: "text"})

	want := []string{"first", "alpha", "zeta", "global_a", "global_z"}
	if got := matchNames(matches); !equalStrings(got, want) {
		t.Errorf("parser order = %v, want %v", got, want)
	}
}

func TestResultsReturnsResultsInMatchOrder(t *testing.T) {
	matches := []Match{
		{Parser: "a", Result: &fakeResult{typ: "a_type"}},
		{Parser: "b", Result: &fakeResult{typ: "b_type"}},
	}

	results := Results(matches)

	if len(results) != 2 || results[0].Type() != "a_type" || results[1].Type() != "b_type" {
		t.Errorf("Results() = %v, want [a_type b_type]", results)
	}
}

func TestRegisterRejectsDuplicateNames(t *testing.T) {
	r := New()
	r.Register(&fakeParser{name: "same", labels: []string{"H1"}, priority: 10})

	defer func() {
		if recover() == nil {
			t.Error("Register did not panic on a duplicate parser name")
		}
	}()
	r.Register(&fakeParser{name: "same", labels: []string{"RA"}, priority: 20})
}
