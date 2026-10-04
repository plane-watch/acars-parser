// Package envelope parses the ARINC 622 envelope of binary messages on
// labels AA (CPDLC) and A6 (ADS-C): the ground station, the IMI and the
// registration, when the envelope's CRC is valid.
package envelope

import (
	"fmt"
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/parsers/arinc"
	"acars_parser/internal/registry"
)

// Result represents extracted envelope data.
type Result struct {
	MsgID        int64  `json:"message_id"`
	Timestamp    string `json:"timestamp"`
	Tail         string `json:"tail,omitempty"`
	Station      string `json:"station,omitempty"`
	MessageType  string `json:"message_type,omitempty"` // AT1, CR1, ADS
	PayloadBytes int    `json:"payload_bytes,omitempty"`
}

func (r *Result) Type() string     { return "envelope" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser extracts tail numbers from envelope headers.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "envelope" }
func (p *Parser) Labels() []string { return []string{"AA", "A6"} }
func (p *Parser) Priority() int    { return 100 } // Run early.

// imiRe matches an IMI followed by the start of the registration field,
// which is a dot unless the registration has seven characters.
var imiRe = regexp.MustCompile(`\.(?:AT1|CR1|CC1|DR1|ADS)[A-Z0-9.-]`)

func (p *Parser) QuickCheck(text string) bool {
	// Must start with envelope header.
	return strings.HasPrefix(text, "/") && imiRe.MatchString(text)
}

// Parse reads the envelope with arinc.Parse, so a message is parsed only
// when the whole envelope is well formed and its CRC is valid. The payload
// is not decoded: on label A6 it is an ADS-C contract request from the
// ground, which holds no position (the aircraft's reports are on B6,
// decoded by the adsc parser).
func (p *Parser) Parse(msg *acars.Message) registry.Result {
	env, err := arinc.Parse(strings.TrimSpace(msg.Text))
	if err != nil {
		return nil
	}
	return &Result{
		MsgID:        int64(msg.ID),
		Timestamp:    msg.Timestamp,
		Tail:         env.Registration,
		Station:      env.GroundStation,
		MessageType:  env.IMI,
		PayloadBytes: len(env.Payload),
	}
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{ParserName: p.Name()}
	passed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{Passed: passed}
	if !passed {
		trace.QuickCheck.Reason = "No envelope header (/<station>.AT1, CR1, CC1, DR1 or ADS) found"
		return trace
	}
	env, err := arinc.Parse(strings.TrimSpace(msg.Text))
	var value string
	if err != nil {
		value = "error: " + err.Error()
	} else {
		value = fmt.Sprintf("station %s, IMI %s, registration %s, %d payload bytes",
			env.GroundStation, env.IMI, env.Registration, len(env.Payload))
	}
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "envelope",
		Pattern: "ARINC 622 envelope with CRC verification (arinc.Parse)",
		Matched: err == nil,
		Value:   value,
	})
	trace.Matched = err == nil
	return trace
}
