// Package flightroutes looks up a flight's routes by its ICAO callsign
// ("QFA1") or its IATA flight number ("QF1"), merging the routes observed
// under either.
//
// Routes are stored under the identifier the messages gave, in either form.
// Their equivalence is learned from messages that named a flight both ways
// (the transmitted flight in one form and a result's flight in the other):
// per flight (a flight alias, needed for airlines whose callsigns are not
// their flight numbers: LO495 flies as LOT49H), and per airline (a code
// pair, QF and QFA, whose flight numbers are the callsigns' numbers).
// Equivalences are used only on enough evidence, so that an IATA code that
// two airlines use is not merged across them.
//
// The merged key is the ICAO callsign: ICAO airline codes are unique, while
// IATA codes are reused. A flight whose callsign is not known is returned
// under its own identifier, with its own routes only.
package flightroutes

import (
	"context"
	"sort"
	"time"

	"acars_parser/internal/flightid"
)

// Pair is an airline's IATA and ICAO codes as messages paired them, with
// the number of observations and how many had the same flight number and
// suffix in both forms.
type Pair struct {
	IATA, ICAO   string
	Observations int
	SameNumber   int
}

// Alias is a flight's IATA flight number and ICAO callsign as messages
// paired them, without leading zeros, with the number of observations.
type Alias struct {
	IATA, ICAO   string
	Observations int
}

// Route is a stored route of one identifier.
type Route struct {
	Flight              string
	Origin, Destination string
	Observations        int
	FirstSeen, LastSeen time.Time
}

// MergedRoute is a route of a flight, merged across its identifiers.
type MergedRoute struct {
	Origin       string    `json:"origin"`
	Destination  string    `json:"destination"`
	Observations int       `json:"observations"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	ObservedAs   []string  `json:"observed_as"` // The identifiers the route was stored under.
}

// Flight is the result of a lookup.
type Flight struct {
	// Callsign is the flight's ICAO callsign, or empty when it is not
	// known; the routes are then only those of the queried identifier.
	Callsign string `json:"callsign,omitempty"`
	// Identifiers are the identifiers whose routes are merged, sorted.
	Identifiers []string      `json:"identifiers"`
	Routes      []MergedRoute `json:"routes"`
}

// Store provides the learned equivalences and the stored routes.
type Store interface {
	AirlinePairs(ctx context.Context) ([]Pair, error)
	// FlightAliases returns the aliases whose IATA flight or ICAO callsign
	// is one of ids.
	FlightAliases(ctx context.Context, ids []string) ([]Alias, error)
	// Routes returns the routes stored under the given identifiers.
	Routes(ctx context.Context, flights []string) ([]Route, error)
}

// Evidence needed to use an equivalence.
const (
	// A code pair converts flight numbers when it was seen this often...
	minPairObservations = 20
	// ...with the same number in both forms this often...
	minPairSameNumber = 0.95
	// ...and accounts for this share of the IATA code's observations
	// (an IATA code used by two airlines does not).
	minPairShare = 0.9
	// A flight alias is used when seen this often, and as this share of
	// its IATA flight's aliases (some reports are about an adjacent leg).
	minAliasObservations = 2
	minAliasShare        = 0.6
)

// Lookup returns the routes of the flight with the given identifier, in
// either form.
func Lookup(ctx context.Context, s Store, query string) (Flight, error) {
	pairs, err := s.AirlinePairs(ctx)
	if err != nil {
		return Flight{}, err
	}
	codes := newCodes(pairs)

	id, ok := flightid.Parse(query, "")
	if !ok {
		return own(ctx, s, query)
	}
	aliases, err := s.FlightAliases(ctx, []string{id.String()})
	if err != nil {
		return Flight{}, err
	}
	callsign, known := canonical(id, aliases, codes)
	if !known {
		return own(ctx, s, id.String())
	}

	cs, _ := flightid.Parse(callsign, "")
	byCode := codeEquivalents(cs, codes)
	aliases, err = s.FlightAliases(ctx, append([]string{callsign}, byCode...))
	if err != nil {
		return Flight{}, err
	}
	ids := equivalents(cs, byCode, aliases)
	routes, err := s.Routes(ctx, ids)
	if err != nil {
		return Flight{}, err
	}
	return Flight{Callsign: callsign, Identifiers: ids, Routes: merge(routes)}, nil
}

// own returns an identifier's own routes, for a flight whose callsign is
// not known.
func own(ctx context.Context, s Store, ident string) (Flight, error) {
	routes, err := s.Routes(ctx, []string{ident})
	if err != nil {
		return Flight{}, err
	}
	return Flight{Identifiers: []string{ident}, Routes: merge(routes)}, nil
}

// codes holds the code pairs that convert flight numbers.
type codes struct {
	icaoFor map[string]string   // IATA code to ICAO code.
	iataFor map[string][]string // ICAO code to IATA codes.
}

func newCodes(pairs []Pair) *codes {
	c := &codes{icaoFor: map[string]string{}, iataFor: map[string][]string{}}
	iataTotal := map[string]int{}
	for _, p := range pairs {
		iataTotal[p.IATA] += p.Observations
	}
	for _, p := range pairs {
		if p.Observations < minPairObservations ||
			float64(p.SameNumber) < minPairSameNumber*float64(p.Observations) ||
			float64(p.Observations) < minPairShare*float64(iataTotal[p.IATA]) {
			continue
		}
		c.icaoFor[p.IATA] = p.ICAO
		c.iataFor[p.ICAO] = append(c.iataFor[p.ICAO], p.IATA)
	}
	return c
}

// trustedAlias returns the callsign an IATA flight is aliased to, or "".
func trustedAlias(iata string, aliases []Alias) string {
	total, best := 0, Alias{}
	for _, a := range aliases {
		if a.IATA != iata {
			continue
		}
		total += a.Observations
		if a.Observations > best.Observations {
			best = a
		}
	}
	if best.Observations < minAliasObservations || float64(best.Observations) < minAliasShare*float64(total) {
		return ""
	}
	return best.ICAO
}

// canonical returns a flight's ICAO callsign, and false if it is not known.
func canonical(id flightid.ID, aliases []Alias, c *codes) (string, bool) {
	if id.ICAO {
		return id.String(), true
	}
	if callsign := trustedAlias(id.String(), aliases); callsign != "" {
		return callsign, true
	}
	if icao, ok := c.icaoFor[id.Airline]; ok {
		return icao + id.Number + id.Suffix, true
	}
	return "", false
}

// codeEquivalents returns the IATA flights that a callsign's code pairs
// convert it to.
func codeEquivalents(cs flightid.ID, c *codes) []string {
	var out []string
	for _, iata := range c.iataFor[cs.Airline] {
		out = append(out, iata+cs.Number+cs.Suffix)
	}
	return out
}

// equivalents returns, sorted, the callsign and the IATA flights whose
// routes are its: those aliased to it, and those its code pairs convert it
// to, unless aliased to another callsign.
func equivalents(cs flightid.ID, byCode []string, aliases []Alias) []string {
	callsign := cs.String()
	set := map[string]bool{callsign: true}
	for _, a := range aliases {
		if a.ICAO == callsign && trustedAlias(a.IATA, aliases) == callsign {
			set[a.IATA] = true
		}
	}
	for _, iata := range byCode {
		if other := trustedAlias(iata, aliases); other == "" || other == callsign {
			set[iata] = true
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// merge merges routes by origin and destination, most observed first.
func merge(routes []Route) []MergedRoute {
	type key struct{ origin, dest string }
	byKey := map[key]*MergedRoute{}
	var order []key
	for _, r := range routes {
		k := key{r.Origin, r.Destination}
		m, ok := byKey[k]
		if !ok {
			m = &MergedRoute{Origin: r.Origin, Destination: r.Destination, FirstSeen: r.FirstSeen, LastSeen: r.LastSeen}
			byKey[k] = m
			order = append(order, k)
		}
		m.Observations += r.Observations
		if r.FirstSeen.Before(m.FirstSeen) {
			m.FirstSeen = r.FirstSeen
		}
		if r.LastSeen.After(m.LastSeen) {
			m.LastSeen = r.LastSeen
		}
		m.ObservedAs = append(m.ObservedAs, r.Flight)
	}
	out := make([]MergedRoute, 0, len(order))
	for _, k := range order {
		m := byKey[k]
		sort.Strings(m.ObservedAs)
		out = append(out, *m)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Observations > out[j].Observations })
	return out
}
