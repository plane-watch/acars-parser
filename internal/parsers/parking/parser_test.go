package parking

import (
	"testing"

	"acars_parser/internal/acars"
)

func TestParser_Parse(t *testing.T) {
	parser := &Parser{}

	text := "QUORYOAAF~1PKG INFO MSG\n\tESCALE  CDG\n\tPARKING PREVIS. A L ARRIVEE : K64\n\tTAPIS A BAGAGES :  330E69"

	msg := &acars.Message{ID: 12345, Label: "1E", Text: text}
	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	pr, ok := result.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", result)
	}

	if pr.Airport != "CDG" {
		t.Errorf("Airport = %q, want %q", pr.Airport, "CDG")
	}
	if pr.ParkingStand != "K64" {
		t.Errorf("ParkingStand = %q, want %q", pr.ParkingStand, "K64")
	}
	if pr.BaggageCarousel != "330E69" {
		t.Errorf("BaggageCarousel = %q, want %q", pr.BaggageCarousel, "330E69")
	}
}

func TestParser_QuickCheck(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		text string
		want bool
	}{
		{"PKG INFO MSG", true},
		{"QUORYOAAF~1PKG INFO MSG", true},
		{"Some other message", false},
	}

	for _, tt := range tests {
		if got := parser.QuickCheck(tt.text); got != tt.want {
			t.Errorf("QuickCheck(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}