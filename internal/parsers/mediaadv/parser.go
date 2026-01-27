// Package mediaadv parses Media Advisory messages (Label SA).
// These messages report the status of data links (VHF, SATCOM, HF, VDL2, etc).
// Based on libacars media-adv implementation.
package mediaadv

import (
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// LinkType represents a data link type.
type LinkType struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// Result represents a Media Advisory message.
type Result struct {
	MsgID          int64      `json:"message_id"`
	Timestamp      string     `json:"timestamp"`
	Version        int        `json:"version"`
	CurrentLink    LinkType   `json:"current_link"`
	Established    bool       `json:"established"` // true = link established, false = link lost
	LinkTime       string     `json:"link_time"`   // HH:MM:SS
	AvailableLinks []LinkType `json:"available_links"`
	Text           string     `json:"text,omitempty"`
}

func (r *Result) Type() string     { return "media_advisory" }
func (r *Result) MessageID() int64 { return r.MsgID }

// linkDescriptions maps link codes to their descriptions.
var linkDescriptions = map[byte]string{
	'V': "VHF ACARS",
	'S': "Default SATCOM",
	'H': "HF",
	'G': "Global Star Satcom",
	'C': "ICO Satcom",
	'2': "VDL2",
	'X': "Inmarsat Aero H/H+/I/L",
	'I': "Iridium Satcom",
}

func getLinkType(code byte) LinkType {
	desc, ok := linkDescriptions[code]
	if !ok {
		desc = "Unknown"
	}
	return LinkType{
		Code:        string(code),
		Description: desc,
	}
}

func isValidLink(code byte) bool {
	_, ok := linkDescriptions[code]
	return ok
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// Parser parses Media Advisory messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "mediaadv" }
func (p *Parser) Labels() []string { return []string{"SA"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	// Media Advisory format: 0[E/L][link]HHMMSS[links]/[text]
	// Minimum length: 10 chars (0EV123456V).
	if len(text) < 10 {
		return false
	}
	// Version must be '0'.
	if text[0] != '0' {
		return false
	}
	// State must be 'E' (established) or 'L' (lost).
	if text[1] != 'E' && text[1] != 'L' {
		return false
	}
	// Third char must be valid link type.
	return isValidLink(text[2])
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	text := strings.TrimSpace(msg.Text)
	if len(text) < 10 {
		return nil
	}

	// Parse version (must be 0).
	version := int(text[0] - '0')
	if version != 0 {
		return nil
	}

	// Parse state: E = established, L = lost.
	state := text[1]
	if state != 'E' && state != 'L' {
		return nil
	}

	// Parse current link type.
	currentLink := text[2]
	if !isValidLink(currentLink) {
		return nil
	}

	// Parse timestamp (HHMMSS) at positions 3-8.
	if len(text) < 9 {
		return nil
	}
	for i := 3; i < 9; i++ {
		if !isDigit(text[i]) {
			return nil
		}
	}

	hour := (text[3]-'0')*10 + (text[4] - '0')
	minute := (text[5]-'0')*10 + (text[6] - '0')
	second := (text[7]-'0')*10 + (text[8] - '0')

	if hour > 23 || minute > 59 || second > 59 {
		return nil
	}

	result := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Version:     version,
		CurrentLink: getLinkType(currentLink),
		Established: state == 'E',
		LinkTime:    formatTime(hour, minute, second),
	}

	// Parse available links (after timestamp until '/' or end).
	text = text[9:]
	for len(text) > 0 && text[0] != '/' {
		if isValidLink(text[0]) {
			result.AvailableLinks = append(result.AvailableLinks, getLinkType(text[0]))
		} else {
			// Invalid character in available links.
			return nil
		}
		text = text[1:]
	}

	// Parse optional text after '/'.
	if len(text) > 1 && text[0] == '/' {
		result.Text = text[1:]
	}

	return result
}

func formatTime(h, m, s byte) string {
	return string([]byte{
		'0' + h/10, '0' + h%10, ':',
		'0' + m/10, '0' + m%10, ':',
		'0' + s/10, '0' + s%10,
	})
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	text := strings.TrimSpace(msg.Text)

	quickCheckPassed := p.QuickCheck(text)
	trace.QuickCheck = &registry.QuickCheck{
		Passed: quickCheckPassed,
	}

	if !quickCheckPassed {
		if len(text) < 10 {
			trace.QuickCheck.Reason = "Message too short (need at least 10 chars)"
		} else if text[0] != '0' {
			trace.QuickCheck.Reason = "Version not '0'"
		} else if text[1] != 'E' && text[1] != 'L' {
			trace.QuickCheck.Reason = "State not 'E' (established) or 'L' (lost)"
		} else if !isValidLink(text[2]) {
			trace.QuickCheck.Reason = "Invalid link type code"
		}
		return trace
	}

	// Add extractors for each parsing step.
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "version",
		Pattern: "position 0: '0'",
		Matched: text[0] == '0',
		Value:   string(text[0]),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "state",
		Pattern: "position 1: 'E' or 'L'",
		Matched: text[1] == 'E' || text[1] == 'L',
		Value:   string(text[1]),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "current_link",
		Pattern: "position 2: valid link code",
		Matched: isValidLink(text[2]),
		Value:   string(text[2]) + " (" + getLinkType(text[2]).Description + ")",
	})

	// Check timestamp.
	validTimestamp := len(text) >= 9
	for i := 3; i < 9 && validTimestamp; i++ {
		if !isDigit(text[i]) {
			validTimestamp = false
		}
	}
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "timestamp",
		Pattern: "positions 3-8: HHMMSS",
		Matched: validTimestamp,
		Value:   text[3:9],
	})

	trace.Matched = quickCheckPassed && validTimestamp

	return trace
}
