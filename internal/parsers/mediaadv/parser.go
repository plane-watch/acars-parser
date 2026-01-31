// Package mediaadv parses Media Advisory messages (Label SA).
// These messages report the status of data links (VHF, SATCOM, HF, VDL2, etc).
// Based on libacars media-adv implementation.
package mediaadv

import (
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
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

// Grok compiler singleton.
var (
	grokCompiler *patterns.Compiler
	grokOnce     sync.Once
	grokErr      error
)

// getCompiler returns the singleton grok compiler.
func getCompiler() (*patterns.Compiler, error) {
	grokOnce.Do(func() {
		grokCompiler = patterns.NewCompiler(Formats, nil)
		grokErr = grokCompiler.Compile()
	})
	return grokCompiler, grokErr
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

	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(text)
	if match == nil {
		return nil
	}

	// Validate timestamp components.
	timeStr := match.Captures["time"]
	if len(timeStr) != 6 {
		return nil
	}

	hour := (timeStr[0]-'0')*10 + (timeStr[1] - '0')
	minute := (timeStr[2]-'0')*10 + (timeStr[3] - '0')
	second := (timeStr[4]-'0')*10 + (timeStr[5] - '0')

	if hour > 23 || minute > 59 || second > 59 {
		return nil
	}

	// Parse state.
	state := match.Captures["state"]
	established := state == "E"

	// Parse current link.
	currentLinkCode := match.Captures["current_link"]
	if len(currentLinkCode) == 0 {
		return nil
	}

	result := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Version:     0,
		CurrentLink: getLinkType(currentLinkCode[0]),
		Established: established,
		LinkTime:    formatTime(hour, minute, second),
		Text:        match.Captures["text"],
	}

	// Parse available links.
	available := match.Captures["available"]
	for i := 0; i < len(available); i++ {
		if isValidLink(available[i]) {
			result.AvailableLinks = append(result.AvailableLinks, getLinkType(available[i]))
		}
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

	// Get the compiler trace.
	compiler, err := getCompiler()
	if err != nil {
		trace.QuickCheck.Reason = "Failed to get compiler: " + err.Error()
		return trace
	}

	// Get detailed trace from compiler.
	compilerTrace := compiler.ParseWithTrace(text)

	// Convert format traces to generic format traces.
	for _, ft := range compilerTrace.Formats {
		trace.Formats = append(trace.Formats, registry.FormatTrace{
			Name:     ft.Name,
			Matched:  ft.Matched,
			Pattern:  ft.Pattern,
			Captures: ft.Captures,
		})
	}

	trace.Matched = compilerTrace.Match != nil
	return trace
}