// Package registry provides tracing interfaces for parser debugging.
package registry

import "acars_parser/internal/acars"

// TraceResult contains trace information from a parser's attempt to parse a message.
type TraceResult struct {
	ParserName string        // Name of the parser.
	QuickCheck *QuickCheck   // QuickCheck result (nil if not applicable).
	Formats    []FormatTrace // Format/pattern match attempts (for grok-style parsers).
	Extractors []Extractor   // Post-processing extractor results.
	Matched    bool          // Whether the parser matched the message.
}

// QuickCheck contains the result of a parser's quick check.
type QuickCheck struct {
	Passed bool   // Whether the quick check passed.
	Reason string // Optional reason for the result.
}

// FormatTrace contains debug information about a format/pattern match attempt.
type FormatTrace struct {
	Name     string            // Format or pattern name.
	Matched  bool              // Whether the pattern matched.
	Pattern  string            // The regex pattern used.
	Captures map[string]string // Captured groups (if matched).
}

// Extractor contains debug information about a field extractor.
type Extractor struct {
	Name    string // Extractor name (e.g., "squawk", "frequency").
	Pattern string // The regex pattern used.
	Matched bool   // Whether the extractor matched.
	Value   string // Extracted value (if matched).
}

// Traceable is implemented by parsers that support debug tracing.
// This allows the debug command to show detailed information about
// why a parser did or didn't match a message.
type Traceable interface {
	// ParseWithTrace attempts to parse the message and returns detailed trace information.
	// The trace includes information about which patterns were tried and their results.
	ParseWithTrace(msg *acars.Message) *TraceResult
}