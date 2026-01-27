package cpdlc

import (
	"errors"
	"fmt"
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
func (p *Parser) Labels() []string { return []string{"AA", "BA"} }
func (p *Parser) Priority() int    { return 50 } // Higher priority than generic parsers.

// QuickCheck checks if the message contains CPDLC markers.
func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, IMI_AT1) ||
		strings.Contains(text, IMI_CR1) ||
		strings.Contains(text, IMI_CC1) ||
		strings.Contains(text, IMI_DR1)
}

// Parse parses a CPDLC message.
func (p *Parser) Parse(msg *acars.Message) registry.Result {
	text := msg.Text

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
	}

	// Determine direction based on message label.
	// AA = downlink (aircraft to ground).
	// BA = uplink (ground to aircraft).
	if msg.Label == "AA" {
		result.Direction = "downlink"
	} else {
		result.Direction = "uplink"
	}

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

	direction := DirectionDownlink
	if result.Direction == "uplink" {
		direction = DirectionUplink
	}

	decoder := NewDecoder(arincResult.Payload, direction)
	cpdlcMsg, err := decoder.Decode()
	if err != nil {
		result.Error = "decode_failed: " + err.Error()
		return result
	}

	result.Header = &cpdlcMsg.Header
	result.Elements = cpdlcMsg.Elements

	// Format the human-readable text.
	result.FormattedText = formatMessage(cpdlcMsg)

	return result
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

	text := msg.Text

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
