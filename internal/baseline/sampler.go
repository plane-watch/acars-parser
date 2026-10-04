package baseline

import (
	"container/heap"
	"sort"
	"strings"
)

// StratumOf returns the stratum a message belongs to: its label and the sorted
// result types the current parsers produce for it ("H1/unparsed" when none do).
// Stratifying by current output gives every registered parser its own share of
// the sample, including parsers that did not exist when the message was stored.
func StratumOf(label string, expected []Expectation) string {
	if len(expected) == 0 {
		return label + "/unparsed"
	}
	types := make([]string, len(expected))
	for i, e := range expected {
		types[i] = e.Type
	}
	sort.Strings(types)
	return label + "/" + strings.Join(types, "+")
}

// Sampler keeps, for each stratum, the cases with the lowest (hash, ID) pairs.
// The result depends only on the cases offered, not on the order they are
// offered in, and memory is bounded by the number of strata times the limit.
type Sampler struct {
	perStratum int
	strata     map[string]*caseHeap
}

// NewSampler returns a sampler that keeps up to perStratum cases per stratum.
func NewSampler(perStratum int) *Sampler {
	return &Sampler{perStratum: perStratum, strata: make(map[string]*caseHeap)}
}

// Offer considers a case for its stratum (c.Stratum), ranked by hash.
func (s *Sampler) Offer(c Case, hash uint64) {
	h, ok := s.strata[c.Stratum]
	if !ok {
		h = &caseHeap{}
		s.strata[c.Stratum] = h
	}
	entry := rankedCase{hash: hash, c: c}
	if h.Len() < s.perStratum {
		heap.Push(h, entry)
		return
	}
	// The heap's root is the highest-ranked kept case; replace it if the new
	// case ranks lower.
	if entry.less((*h)[0]) {
		(*h)[0] = entry
		heap.Fix(h, 0)
	}
}

// Strata returns the number of strata seen.
func (s *Sampler) Strata() int {
	return len(s.strata)
}

// Cases returns the kept cases, ordered by stratum and then by (hash, ID).
func (s *Sampler) Cases() []Case {
	names := make([]string, 0, len(s.strata))
	for name := range s.strata {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []Case
	for _, name := range names {
		entries := append([]rankedCase(nil), *s.strata[name]...)
		sort.Slice(entries, func(i, j int) bool { return entries[i].less(entries[j]) })
		for _, e := range entries {
			out = append(out, e.c)
		}
	}
	return out
}

type rankedCase struct {
	hash uint64
	c    Case
}

func (a rankedCase) less(b rankedCase) bool {
	if a.hash != b.hash {
		return a.hash < b.hash
	}
	return a.c.ID < b.c.ID
}

// caseHeap is a max-heap of ranked cases: the root ranks highest, so it is the
// first to be displaced by a lower-ranked case.
type caseHeap []rankedCase

func (h caseHeap) Len() int            { return len(h) }
func (h caseHeap) Less(i, j int) bool  { return h[j].less(h[i]) }
func (h caseHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *caseHeap) Push(x interface{}) { *h = append(*h, x.(rankedCase)) }
func (h *caseHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
