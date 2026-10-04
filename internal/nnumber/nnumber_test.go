package nnumber

import "testing"

func TestICAOAddress(t *testing.T) {
	tests := []struct {
		tail   string
		want   string
		wantOK bool
	}{
		// The first and last addresses of the US civil block.
		{"N1", "A00001", true},
		{"N99999", "ADF7C7", true},
		// Letter suffixes directly after the first digit.
		{"N1A", "A00002", true},
		{"N1AA", "A00003", true},
		// Tails as transmitted can carry a leading dot or be lower case.
		{".N1", "A00001", true},
		{"n1", "A00001", true},
		// Not N-numbers.
		{"VH-EBO", "", false},
		{"N", "", false},
		{"N0123", "", false},   // The first digit cannot be 0.
		{"N1I", "", false},     // I and O are never used.
		{"N123456", "", false}, // Too long.
		{"N1A2", "", false},    // A digit cannot follow a letter.
	}
	for _, tt := range tests {
		got, ok := ICAOAddress(tt.tail)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("ICAOAddress(%q) = %q, %v, want %q, %v", tt.tail, got, ok, tt.want, tt.wantOK)
		}
	}
}
