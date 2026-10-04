package cpdlc

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/parsers/arinc"
	"acars_parser/internal/registry"
)

// IMI markers for quick check.
const (
	IMI_AT1 = ".AT1." // CPDLC message.
	IMI_CR1 = ".CR1." // Connection request.
	IMI_CC1 = ".CC1." // Connection confirm.
	IMI_DR1 = ".DR1." // Disconnect request.
)

// Result represents a decoded CPDLC message for the ACARS parser framework.
type Result struct {
	MsgID         int64            `json:"message_id"`
	Timestamp     string           `json:"timestamp"`
	MessageType   string           `json:"message_type"` // "cpdlc", "connect_request", "connect_confirm", "disconnect".
	Direction     string           `json:"direction"`    // "uplink" or "downlink".
	GroundStation string           `json:"ground_station,omitempty"`
	Registration  string           `json:"registration,omitempty"`
	Header        *MessageHeader   `json:"header,omitempty"`
	Elements      []MessageElement `json:"elements,omitempty"`
	FormattedText string           `json:"formatted_text,omitempty"` // Human-readable message.
	RawHex        string           `json:"raw_hex,omitempty"`
	Error         string           `json:"error,omitempty"`
}

func (r *Result) Type() string     { return "cpdlc" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses CPDLC messages (Labels AA, BA).
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "cpdlc" }
func (p *Parser) Labels() []string { return []string{"AA", "BA", "H1"} }
func (p *Parser) Priority() int    { return 50 } // Higher priority than generic parsers.

// imiRe matches a CPDLC IMI followed by the start of the registration
// field, which is a dot unless the registration has seven characters
// (".AT1.N514DN", ".AT1B-18772").
var imiRe = regexp.MustCompile(`\.(?:AT1|CR1|CC1|DR1)[A-Z0-9.-]`)

// QuickCheck checks if the message contains CPDLC markers.
func (p *Parser) QuickCheck(text string) bool {
	return imiRe.MatchString(text)
}

// Parse parses a CPDLC message.
func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text, label := envelopeText(msg)

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
	}

	// Determine direction using available indicators (in order of reliability):
	// 1. LinkDirection - explicit direction from feed (most reliable).
	// 2. BlockID - ACARS block ID: '0'-'9' = downlink, letters = uplink.
	// 3. Label - fallback: AA = downlink, BA = uplink (least reliable for CPDLC).
	result.Direction = determineDirection(msg, label)

	// Parse through ARINC layer (validates CRC, extracts payload).
	arincResult, err := arinc.Parse(text)
	if err != nil {
		// Categorise the error type.
		if errors.Is(err, arinc.ErrCRCFailed) {
			result.Error = "crc_failed"
		} else if errors.Is(err, arinc.ErrTooShort) {
			result.Error = "message_too_short"
		} else if errors.Is(err, arinc.ErrUnknownFormat) {
			return nil // Not an ARINC message, let other parsers handle it.
		} else {
			result.Error = "parse_failed: " + err.Error()
		}
		return result
	}

	result.GroundStation = arincResult.GroundStation
	result.Registration = arincResult.Registration
	result.RawHex = arincResult.RawHex

	// Determine message type from IMI.
	switch arincResult.IMI {
	case arinc.IMIAT1:
		result.MessageType = "cpdlc"
	case arinc.IMICR1:
		result.MessageType = "connect_request"
	case arinc.IMICC1:
		result.MessageType = "connect_confirm"
	case arinc.IMIDR1:
		result.MessageType = "disconnect"
	default:
		result.MessageType = "unknown"
	}

	// For connection messages, we don't have CPDLC payload to decode.
	if result.MessageType != "cpdlc" {
		return result
	}

	// Decode the CPDLC payload (CRC already stripped by ARINC layer).
	if len(arincResult.Payload) == 0 {
		result.Error = "decode_failed: no payload data"
		return result
	}

	direction := DirectionUnknown
	switch result.Direction {
	case "uplink":
		direction = DirectionUplink
	case "downlink":
		direction = DirectionDownlink
	}

	cpdlcMsg, err := DecodeWithUPER(arincResult.Payload, direction)
	switch {
	case errors.Is(err, ErrAmbiguousDirection):
		result.Error = "direction_unknown"
		return result
	case errors.Is(err, ErrNoValidElements):
		result.Error = "no_valid_elements"
		return result
	case err != nil:
		result.Error = "decode_failed: " + err.Error()
		return result
	}

	// The decoder corrects the direction when only the other message set
	// gives valid elements; report the direction it decoded with, which the
	// element labels belong to.
	switch cpdlcMsg.Direction {
	case DirectionUplink:
		result.Direction = "uplink"
	case DirectionDownlink:
		result.Direction = "downlink"
	}
	result.Header = &cpdlcMsg.Header
	result.Elements = cpdlcMsg.Elements

	// Format the human-readable text.
	result.FormattedText = formatMessage(cpdlcMsg)

	return result
}

// envelopeText returns the message's text in the ARINC envelope form that
// arinc.Parse reads, and the label that stands for the message in the
// direction fallback. Label H1 carries CPDLC relayed with its original label
// ("- #MD/AA ..."), which is returned in place of H1, or without the leading
// "/" (see arinc.Unwrap).
func envelopeText(msg *acars.Message) (text, label string) {
	text, label = msg.Text, msg.Label
	if inner, relayed, ok := arinc.Unwrap(text); ok {
		text = inner
		if relayed != "" {
			label = relayed
		}
	}
	return text, label
}

// determineDirection returns the direction of a message whose label (or,
// for a relayed message, original label) is label, or "" if it is not
// known. Priority: LinkDirection > BlockID > Label.
func determineDirection(msg *acars.Message, label string) string {
	// 1. Use explicit link_direction if available (most reliable).
	if msg.LinkDirection != "" {
		switch msg.LinkDirection {
		case "uplink":
			return "uplink"
		case "downlink":
			return "downlink"
		}
	}

	// 2. Use block_id if available.
	// Per ACARS spec: '0'-'9' = downlink (air to ground), 'A'-'X' = uplink (ground to air).
	if msg.BlockID != "" && len(msg.BlockID) > 0 {
		blockChar := msg.BlockID[0]
		if blockChar >= '0' && blockChar <= '9' {
			return "downlink"
		}
		if blockChar >= 'A' && blockChar <= 'Z' {
			return "uplink"
		}
	}

	// 3. Fall back to the label: AA carries uplinks and BA downlinks (as
	// A6 and B6 do for ADS-C). In ten minutes of live traffic (October
	// 2026), all 118 AA messages were uplinks and all 77 BA messages
	// downlinks. Label H1 carries both directions (100 downlinks and 12
	// uplinks in the same traffic), so it gives none.
	switch label {
	case "AA":
		return "uplink"
	case "BA":
		return "downlink"
	}
	return ""
}

// formatMessage creates a human-readable summary of the CPDLC message.
func formatMessage(msg *Message) string {
	if len(msg.Elements) == 0 {
		return ""
	}

	parts := make([]string, 0, len(msg.Elements))
	for _, elem := range msg.Elements {
		if elem.Text != "" {
			parts = append(parts, elem.Text)
		} else {
			parts = append(parts, elem.Label)
		}
	}

	return strings.Join(parts, "; ")
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	quickCheckPassed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{
		Passed: quickCheckPassed,
	}

	if !quickCheckPassed {
		trace.QuickCheck.Reason = "No CPDLC IMI marker (.AT1., .CR1., .CC1., .DR1.) found"
		return trace
	}

	text, _ := envelopeText(msg)

	// Identify which IMI marker is present.
	imiType := ""
	if strings.Contains(text, IMI_AT1) {
		imiType = "AT1 (CPDLC message)"
	} else if strings.Contains(text, IMI_CR1) {
		imiType = "CR1 (connection request)"
	} else if strings.Contains(text, IMI_CC1) {
		imiType = "CC1 (connection confirm)"
	} else if strings.Contains(text, IMI_DR1) {
		imiType = "DR1 (disconnect request)"
	}

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "imi_type",
		Pattern: ".AT1., .CR1., .CC1., or .DR1.",
		Matched: imiType != "",
		Value:   imiType,
	})

	// Try ARINC layer parsing.
	arincResult, err := arinc.Parse(text)
	arincOK := err == nil

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "arinc_parse",
		Pattern: "ARINC envelope parsing with CRC verification",
		Matched: arincOK,
		Value: func() string {
			if err != nil {
				return "error: " + err.Error()
			}
			return "OK"
		}(),
	})

	if arincOK {
		trace.Extractors = append(trace.Extractors, registry.Extractor{
			Name:    "ground_station",
			Pattern: "extracted from ARINC envelope",
			Matched: arincResult.GroundStation != "",
			Value:   arincResult.GroundStation,
		})

		trace.Extractors = append(trace.Extractors, registry.Extractor{
			Name:    "registration",
			Pattern: "extracted from ARINC envelope",
			Matched: arincResult.Registration != "",
			Value:   arincResult.Registration,
		})

		trace.Extractors = append(trace.Extractors, registry.Extractor{
			Name:    "payload_size",
			Pattern: "decoded binary payload",
			Matched: len(arincResult.Payload) > 0,
			Value:   fmt.Sprintf("%d bytes", len(arincResult.Payload)),
		})
	}

	trace.Matched = arincOK
	return trace
}
