package main

import (
	"testing"

	"acars_parser/internal/registry"
)

// typedResult is a minimal registry.Result for reparse tests.
type typedResult struct{ typ string }

func (r *typedResult) Type() string     { return r.typ }
func (r *typedResult) MessageID() int64 { return 0 }

func TestComparableMatch(t *testing.T) {
	matches := []registry.Match{
		{Parser: "weather", Result: &typedResult{typ: "weather"}},
		{Parser: "takeoff_data", Result: &typedResult{typ: "takeoff_data"}},
	}

	tests := []struct {
		name       string
		storedType string
		matches    []registry.Match
		wantParser string
		wantOK     bool
	}{
		{
			name:       "stored type is compared with the result of the same type, not the first",
			storedType: "takeoff_data",
			matches:    matches,
			wantParser: "takeoff_data",
			wantOK:     true,
		},
		{
			name:       "stored type no longer produced falls back to the first match",
			storedType: "pdc",
			matches:    matches,
			wantParser: "weather",
			wantOK:     true,
		},
		{
			name:       "unparsed row is compared with the first match",
			storedType: "unparsed",
			matches:    matches,
			wantParser: "weather",
			wantOK:     true,
		},
		{
			name:       "no matches",
			storedType: "weather",
			matches:    nil,
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := comparableMatch(tt.storedType, tt.matches)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got.Parser != tt.wantParser {
				t.Errorf("parser = %q, want %q", got.Parser, tt.wantParser)
			}
		})
	}
}
