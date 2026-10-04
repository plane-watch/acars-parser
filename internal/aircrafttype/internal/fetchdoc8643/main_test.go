package main

import (
	"fmt"
	"strings"
	"testing"
)

func entries(n int) []entry {
	out := make([]entry, n)
	for i := range out {
		out[i] = entry{Designator: fmt.Sprintf("Q%03X", i), ModelFullName: "model"}
	}
	return out
}

func TestValidateAcceptsAPlausibleList(t *testing.T) {
	if err := validate(entries(2600), 2614); err != nil {
		t.Errorf("validate() = %v, want nil", err)
	}
	if err := validate(entries(2600), 0); err != nil {
		t.Errorf("validate() with no existing file = %v, want nil", err)
	}
}

func TestValidateRejectsUnusableLists(t *testing.T) {
	tests := []struct {
		name     string
		entries  []entry
		existing int
		want     string
	}{
		{"too few distinct designators", entries(1500), 0, "distinct designators"},
		{"a large drop from the existing list", entries(2100), 2614, "drop"},
		{"an invalid designator", append(entries(2600), entry{Designator: "b-7!"}), 0, "invalid designator"},
		{"empty objects", append(entries(2600), entry{}), 0, "invalid designator"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.entries, tt.existing)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("validate() = %v, want an error containing %q", err, tt.want)
			}
		})
	}
}
