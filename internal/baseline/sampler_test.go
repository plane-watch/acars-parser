package baseline

import (
	"testing"
)

func TestStratumOf(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		expected []Expectation
		want     string
	}{
		{"no results", "H1", nil, "H1/unparsed"},
		{"one result", "B6", []Expectation{{Parser: "adsc", Type: "adsc"}}, "B6/adsc"},
		{
			name:  "several results are sorted",
			label: "RA",
			expected: []Expectation{
				{Parser: "weather", Type: "weather"},
				{Parser: "takeoff_data", Type: "takeoff_data"},
			},
			want: "RA/takeoff_data+weather",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StratumOf(tt.label, tt.expected); got != tt.want {
				t.Errorf("StratumOf() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSamplerKeepsTheLowestHashesPerStratum(t *testing.T) {
	s := NewSampler(2)
	// Offered out of order; the two lowest (hash, ID) pairs per stratum win.
	s.Offer(Case{ID: 10, Stratum: "A"}, 50)
	s.Offer(Case{ID: 11, Stratum: "A"}, 10)
	s.Offer(Case{ID: 12, Stratum: "A"}, 30)
	s.Offer(Case{ID: 13, Stratum: "A"}, 30) // Same hash as 12: the lower ID wins.
	s.Offer(Case{ID: 20, Stratum: "B"}, 99)

	got := s.Cases()

	wantIDs := []int64{11, 12, 20}
	if len(got) != len(wantIDs) {
		t.Fatalf("Cases() returned %d cases, want %d: %v", len(got), len(wantIDs), got)
	}
	for i, c := range got {
		if c.ID != wantIDs[i] {
			t.Errorf("case %d ID = %d, want %d", i, c.ID, wantIDs[i])
		}
	}
	if s.Strata() != 2 {
		t.Errorf("Strata() = %d, want 2", s.Strata())
	}
}

func TestSamplerResultDoesNotDependOnOfferOrder(t *testing.T) {
	offers := []struct {
		c Case
		h uint64
	}{
		{Case{ID: 1, Stratum: "A"}, 7}, {Case{ID: 2, Stratum: "A"}, 3},
		{Case{ID: 3, Stratum: "A"}, 5}, {Case{ID: 4, Stratum: "B"}, 1},
		{Case{ID: 5, Stratum: "A"}, 3},
	}

	forward := NewSampler(2)
	for _, o := range offers {
		forward.Offer(o.c, o.h)
	}
	backward := NewSampler(2)
	for i := len(offers) - 1; i >= 0; i-- {
		backward.Offer(offers[i].c, offers[i].h)
	}

	a, b := forward.Cases(), backward.Cases()
	if len(a) != len(b) {
		t.Fatalf("different sample sizes: %d and %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Errorf("case %d: forward ID %d, backward ID %d", i, a[i].ID, b[i].ID)
		}
	}
}
