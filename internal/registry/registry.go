// Package registry provides a message parser registry for dispatching
// ACARS messages to appropriate parsers.
package registry

import (
	"fmt"
	"sort"
	"sync"

	"acars_parser/internal/acars"
)

// Result is the common interface for all parse results.
type Result interface {
	Type() string     // e.g., "pdc", "route", "adsc"
	MessageID() int64 // The original message ID
}

// Match is one parser's result for a message, recorded with the name of the
// parser that produced it. Several parsers can match the same message.
type Match struct {
	Parser string // The producing parser's Name().
	Result Result
}

// Results returns the results of the matches, in the same order.
func Results(matches []Match) []Result {
	results := make([]Result, len(matches))
	for i, m := range matches {
		results[i] = m.Result
	}
	return results
}

// Parser is implemented by each message parser.
type Parser interface {
	// Name returns the parser's unique identifier.
	Name() string

	// Labels returns which ACARS labels this parser handles.
	// Empty slice means "all labels" (content-based parser like PDC).
	Labels() []string

	// QuickCheck performs a fast string check before expensive regex.
	// Returns true if the message MIGHT be parseable (false = definitely skip).
	// This should use strings.Contains/HasPrefix, NOT regex.
	QuickCheck(text string) bool

	// Priority determines order when multiple parsers match the same label.
	// Lower number = checked first. Cheaper checks should have lower priority.
	Priority() int

	// Parse attempts to parse the message, returns nil if not applicable.
	Parse(msg *acars.Message) Result
}

// Registry holds all registered parsers organised for efficient dispatch.
type Registry struct {
	mu sync.RWMutex

	// byLabel maps labels to parser slices, sorted by Priority (ascending)
	byLabel map[string][]Parser

	// global holds parsers that check all messages (content-based)
	global []Parser

	// catchAll holds parsers that run only when nothing else matched
	catchAll []Parser

	// sorted tracks whether parsers have been sorted
	sorted bool

	// names holds every registered parser name, which must be unique.
	names map[string]bool
}

// New creates a new Registry instance.
func New() *Registry {
	return &Registry{
		byLabel: make(map[string][]Parser),
		names:   make(map[string]bool),
	}
}

// Global default registry.
var defaultRegistry = New()

// Default returns the global registry instance.
func Default() *Registry {
	return defaultRegistry
}

// Register adds a parser to the default registry.
// Called during init() in each parser package.
func Register(p Parser) {
	defaultRegistry.Register(p)
}

// RegisterCatchAll adds a catch-all parser that runs when nothing else matches.
func RegisterCatchAll(p Parser) {
	defaultRegistry.RegisterCatchAll(p)
}

// Register adds a parser to the registry. It panics if a parser with the same
// name is already registered: names identify results and break ordering ties,
// so a duplicate is a programming error that must surface at start-up.
func (r *Registry) Register(p Parser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimName(p.Name())

	labels := p.Labels()
	if len(labels) == 0 {
		// Content-based parser - checks all messages
		r.global = append(r.global, p)
	} else {
		for _, label := range labels {
			r.byLabel[label] = append(r.byLabel[label], p)
		}
	}
	r.sorted = false
}

// RegisterCatchAll adds a catch-all parser. Like Register, it panics on a
// duplicate parser name.
func (r *Registry) RegisterCatchAll(p Parser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimName(p.Name())
	r.catchAll = append(r.catchAll, p)
	r.sorted = false
}

// claimName records a parser name, panicking if it is already taken.
// The caller must hold r.mu.
func (r *Registry) claimName(name string) {
	if r.names[name] {
		panic(fmt.Sprintf("registry: parser %q registered twice", name))
	}
	r.names[name] = true
}

// sortParsers orders parsers by ascending priority, breaking ties by name so
// that the dispatch order never depends on registration (import) order.
func sortParsers(parsers []Parser) {
	sort.Slice(parsers, func(i, j int) bool {
		if parsers[i].Priority() != parsers[j].Priority() {
			return parsers[i].Priority() < parsers[j].Priority()
		}
		return parsers[i].Name() < parsers[j].Name()
	})
}

// Sort sorts all parser slices by priority, then name. Call before dispatching.
func (r *Registry) Sort() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sorted {
		return
	}

	for label := range r.byLabel {
		sortParsers(r.byLabel[label])
	}
	sortParsers(r.global)
	sortParsers(r.catchAll)

	r.sorted = true
}

// Dispatch routes a message to the appropriate parsers and returns every match,
// each with the name of the parser that produced it. Multiple parsers can match
// the same message (e.g., PDC + route info). Label parsers come first, then
// global parsers, each in the order established by Sort(). If Sort() has not
// been called, parsers are in registration order.
func (r *Registry) Dispatch(msg *acars.Message) []Match {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []Match

	// 1. Try label-specific parsers first (most efficient path)
	if parsers, ok := r.byLabel[msg.Label]; ok {
		for _, p := range parsers {
			// Quick check before expensive parse
			if !p.QuickCheck(msg.Text) {
				continue
			}
			if result := p.Parse(msg); result != nil {
				results = append(results, Match{Parser: p.Name(), Result: result})
			}
		}
	}

	// 2. Try global (content-based) parsers
	for _, p := range r.global {
		if !p.QuickCheck(msg.Text) {
			continue
		}
		if result := p.Parse(msg); result != nil {
			results = append(results, Match{Parser: p.Name(), Result: result})
		}
	}

	// 3. If nothing matched, try catch-all parsers
	if len(results) == 0 && len(r.catchAll) > 0 {
		for _, p := range r.catchAll {
			if result := p.Parse(msg); result != nil {
				results = append(results, Match{Parser: p.Name(), Result: result})
			}
		}
	}

	return results
}

// DispatchFirst returns only the first successful parse result.
// Useful when you only need one result per message.
func (r *Registry) DispatchFirst(msg *acars.Message) Result {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try label-specific parsers
	if parsers, ok := r.byLabel[msg.Label]; ok {
		for _, p := range parsers {
			if !p.QuickCheck(msg.Text) {
				continue
			}
			if result := p.Parse(msg); result != nil {
				return result
			}
		}
	}

	// Try global parsers
	for _, p := range r.global {
		if !p.QuickCheck(msg.Text) {
			continue
		}
		if result := p.Parse(msg); result != nil {
			return result
		}
	}

	// Try catch-all
	for _, p := range r.catchAll {
		if result := p.Parse(msg); result != nil {
			return result
		}
	}

	return nil
}

// RegisteredLabels returns all labels that have parsers registered.
func (r *Registry) RegisteredLabels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	labels := make([]string, 0, len(r.byLabel))
	for label := range r.byLabel {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

// ParserCount returns the total number of unique registered parsers.
// Parsers registered for multiple labels are only counted once.
func (r *Registry) ParserCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use a map to deduplicate parsers (some may be registered for multiple labels).
	seen := make(map[string]bool)

	for _, p := range r.global {
		seen[p.Name()] = true
	}
	for _, parsers := range r.byLabel {
		for _, p := range parsers {
			seen[p.Name()] = true
		}
	}
	for _, p := range r.catchAll {
		seen[p.Name()] = true
	}

	return len(seen)
}

// AllParsers returns all registered parsers (global, label-specific, and catch-all).
// This is useful for debugging and listing available parsers.
func (r *Registry) AllParsers() []Parser {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use a map to deduplicate parsers (some may be registered for multiple labels).
	seen := make(map[string]bool)
	var result []Parser

	// Add global parsers.
	for _, p := range r.global {
		if !seen[p.Name()] {
			seen[p.Name()] = true
			result = append(result, p)
		}
	}

	// Add label-specific parsers.
	for _, parsers := range r.byLabel {
		for _, p := range parsers {
			if !seen[p.Name()] {
				seen[p.Name()] = true
				result = append(result, p)
			}
		}
	}

	// Add catch-all parsers.
	for _, p := range r.catchAll {
		if !seen[p.Name()] {
			seen[p.Name()] = true
			result = append(result, p)
		}
	}

	return result
}
