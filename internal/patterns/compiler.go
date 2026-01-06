// Package patterns provides shared regex patterns and helper functions for ACARS parsing.
// This file contains the grok-style pattern compiler.

package patterns

import (
	"regexp"
	"strings"
)

// Format represents a message format with named capture groups.
type Format struct {
	Name     string         // Format name for identification
	Pattern  string         // Pattern with {PLACEHOLDER} syntax
	Compiled *regexp.Regexp // Compiled regex (populated by Compile)
	Fields   []string       // Field names in capture order (for documentation)
}

// Compiler manages pattern compilation and parsing for a set of formats.
type Compiler struct {
	basePatterns map[string]string
	formats      []Format
}

// NewCompiler creates a new pattern compiler with the given formats.
// It merges the provided base patterns with the global BasePatterns,
// allowing local patterns to override global ones.
func NewCompiler(formats []Format, localPatterns map[string]string) *Compiler {
	c := &Compiler{
		basePatterns: make(map[string]string),
		formats:      make([]Format, len(formats)),
	}

	// Copy global base patterns.
	for k, v := range BasePatterns {
		c.basePatterns[k] = v
	}

	// Overlay local patterns (can override global ones).
	for k, v := range localPatterns {
		c.basePatterns[k] = v
	}

	// Copy formats.
	copy(c.formats, formats)

	return c
}

// Compile expands all {PLACEHOLDER} references and compiles regexes.
func (c *Compiler) Compile() error {
	for i := range c.formats {
		expanded := c.expand(c.formats[i].Pattern)
		re, err := regexp.Compile(expanded)
		if err != nil {
			return err
		}
		c.formats[i].Compiled = re
	}
	return nil
}

// expand replaces {PLACEHOLDER} with actual regex patterns.
func (c *Compiler) expand(pattern string) string {
	result := pattern
	for name, regex := range c.basePatterns {
		placeholder := "{" + name + "}"
		result = strings.ReplaceAll(result, placeholder, regex)
	}
	return result
}

// Match represents a successful pattern match with extracted fields.
type Match struct {
	FormatName string            // Name of the matched format
	Captures   map[string]string // Named capture group values
}

// Parse attempts to parse text using all compiled formats.
// Returns the first successful match, or nil if no format matches.
func (c *Compiler) Parse(text string) *Match {
	upperText := strings.ToUpper(text)

	for _, format := range c.formats {
		if format.Compiled == nil {
			continue
		}

		match := format.Compiled.FindStringSubmatch(upperText)
		if match == nil {
			continue
		}

		result := &Match{
			FormatName: format.Name,
			Captures:   make(map[string]string),
		}

		// Extract named groups.
		for i, name := range format.Compiled.SubexpNames() {
			if i == 0 || name == "" {
				continue
			}
			result.Captures[name] = match[i]
		}

		return result
	}

	return nil
}

// ParseAll attempts to parse text using all compiled formats.
// Returns all successful matches (useful when formats extract different fields).
func (c *Compiler) ParseAll(text string) []*Match {
	upperText := strings.ToUpper(text)
	var results []*Match

	for _, format := range c.formats {
		if format.Compiled == nil {
			continue
		}

		match := format.Compiled.FindStringSubmatch(upperText)
		if match == nil {
			continue
		}

		result := &Match{
			FormatName: format.Name,
			Captures:   make(map[string]string),
		}

		// Extract named groups.
		for i, name := range format.Compiled.SubexpNames() {
			if i == 0 || name == "" {
				continue
			}
			result.Captures[name] = match[i]
		}

		results = append(results, result)
	}

	return results
}

// FindAllMatches finds all occurrences of a pattern in text.
// Useful for patterns that can match multiple times (e.g., oceanic fixes, waypoints).
func (c *Compiler) FindAllMatches(text string, formatName string) []map[string]string {
	upperText := strings.ToUpper(text)
	var results []map[string]string

	for _, format := range c.formats {
		if format.Name != formatName || format.Compiled == nil {
			continue
		}

		matches := format.Compiled.FindAllStringSubmatch(upperText, -1)
		for _, match := range matches {
			captures := make(map[string]string)
			for i, name := range format.Compiled.SubexpNames() {
				if i == 0 || name == "" {
					continue
				}
				captures[name] = match[i]
			}
			results = append(results, captures)
		}
		break
	}

	return results
}

// GetCapture is a helper to safely get a capture value with a default.
func (m *Match) GetCapture(name string, defaultVal string) string {
	if m == nil {
		return defaultVal
	}
	if val, ok := m.Captures[name]; ok && val != "" {
		return val
	}
	return defaultVal
}

// FormatTrace contains debug information about a format match attempt.
type FormatTrace struct {
	Name     string            // Format name
	Matched  bool              // Whether the pattern matched
	Pattern  string            // The expanded regex pattern
	Captures map[string]string // Captured groups (if matched)
}

// ParseTrace contains complete trace information for a parse attempt.
type ParseTrace struct {
	Formats []FormatTrace // All format match attempts
	Match   *Match        // The first successful match (if any)
}

// ParseWithTrace attempts to parse text and returns detailed trace information.
// This is useful for debugging why patterns don't match.
func (c *Compiler) ParseWithTrace(text string) *ParseTrace {
	upperText := strings.ToUpper(text)
	trace := &ParseTrace{
		Formats: make([]FormatTrace, 0, len(c.formats)),
	}

	for _, format := range c.formats {
		ft := FormatTrace{
			Name:    format.Name,
			Pattern: c.expand(format.Pattern),
		}

		if format.Compiled == nil {
			trace.Formats = append(trace.Formats, ft)
			continue
		}

		match := format.Compiled.FindStringSubmatch(upperText)
		if match == nil {
			ft.Matched = false
			trace.Formats = append(trace.Formats, ft)
			continue
		}

		// Pattern matched.
		ft.Matched = true
		ft.Captures = make(map[string]string)

		for i, name := range format.Compiled.SubexpNames() {
			if i == 0 || name == "" {
				continue
			}
			ft.Captures[name] = match[i]
		}

		trace.Formats = append(trace.Formats, ft)

		// Set the first match result.
		if trace.Match == nil {
			trace.Match = &Match{
				FormatName: format.Name,
				Captures:   ft.Captures,
			}
		}
	}

	return trace
}
