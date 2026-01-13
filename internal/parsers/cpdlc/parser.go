package cpdlc

import (
	"encoding/hex"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// IMI (Interchange Message Identifier) markers for CPDLC messages.
const (
	IMI_AT1 = ".AT1." // CPDLC message (ACARS text).
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

	// Determine message type and extract payload.
	var imi string
	var payloadStart int

	if idx := strings.Index(text, IMI_AT1); idx >= 0 {
		result.MessageType = "cpdlc"
		imi = IMI_AT1
		payloadStart = idx + len(IMI_AT1)
	} else if idx := strings.Index(text, IMI_CR1); idx >= 0 {
		result.MessageType = "connect_request"
		imi = IMI_CR1
		payloadStart = idx + len(IMI_CR1)
	} else if idx := strings.Index(text, IMI_CC1); idx >= 0 {
		result.MessageType = "connect_confirm"
		imi = IMI_CC1
		payloadStart = idx + len(IMI_CC1)
	} else if idx := strings.Index(text, IMI_DR1); idx >= 0 {
		result.MessageType = "disconnect"
		imi = IMI_DR1
		payloadStart = idx + len(IMI_DR1)
	} else {
		return nil // No CPDLC markers found.
	}

	// Extract ground station from before the IMI marker.
	prefix := text[:payloadStart-len(imi)]
	if idx := strings.LastIndex(prefix, "/"); idx >= 0 {
		result.GroundStation = prefix[idx+1:]
	}

	// Extract payload (everything after IMI).
	payload := text[payloadStart:]

	// The payload format is: REGISTRATION + HEX_DATA
	// Registration is typically 6-7 characters followed by hex data.
	// Try to find where the hex data starts.
	reg, hexData := splitRegistrationAndData(payload)
	result.Registration = reg
	result.RawHex = hexData

	// Determine direction based on message label.
	// For CPDLC in our pipeline (dumpvdl2/acarsdec):
	//   AA = uplink   (ground -> aircraft)
	//   BA = downlink (aircraft -> ground)
	if msg.Label == "AA" {
		result.Direction = "uplink"
	} else {
		result.Direction = "downlink"
	}

	// For connection messages, we don't have CPDLC payload to decode.
	if result.MessageType != "cpdlc" {
		return result
	}

	// Decode the hex payload.
	if hexData == "" {
		result.Error = "no payload data"
		return result
	}

	data, err := hex.DecodeString(hexData)
	if err != nil {
		result.Error = "invalid hex data: " + err.Error()
		return result
	}

	// AT1 payloads in this project include a trailing 2-byte checksum/FCS which is
	// *not* part of the ASN.1 PER CPDLC message. Keep raw_hex as-is for debugging,
	// but trim those 2 bytes before decoding.
	if len(data) > 2 {
		data = data[:len(data)-2]
	}

	// Decode the CPDLC message.
	direction := DirectionDownlink
	if result.Direction == "uplink" {
		direction = DirectionUplink
	}

	decoder := NewDecoder(data, direction)
	cpdlcMsg, err := decoder.Decode()
	if err != nil {
		result.Error = "decode error: " + err.Error()
		return result
	}

	result.Header = &cpdlcMsg.Header
	result.Elements = cpdlcMsg.Elements

	// Format the human-readable text.
	result.FormattedText = formatMessage(cpdlcMsg)

	return result
}

// splitRegistrationAndData extracts the aircraft registration and hex data from the payload.
// Registration formats vary: N12345, VH-ABC, F-GSQC, TC-LLH, etc.
// Registrations are typically 5-7 characters including hyphens.
func splitRegistrationAndData(payload string) (string, string) {
	// Remove any trailing newlines or whitespace.
	payload = strings.TrimSpace(payload)

	// Handle empty or too short payloads.
	if len(payload) < 8 {
		return payload, ""
	}

	// Registration typically contains at least one non-hex char (e.g., hyphen, G, J, K, etc.)
	// or specific patterns like "N" followed by digits.
	// Strategy: find the first position where the remaining string is valid hex
	// and has a reasonable length (at least 6 chars for a minimal CPDLC message).

	// Try positions 5, 6, 7 (most common registration lengths).
	for _, regLen := range []int{6, 7, 5} {
		if regLen >= len(payload) {
			continue
		}
		candidate := payload[regLen:]
		if isValidHex(candidate) && len(candidate) >= 6 {
			return payload[:regLen], candidate
		}
	}

	// Fallback: scan for the first position where the rest is all hex.
	// Start from minimum registration length of 5.
	for i := 5; i < len(payload)-5; i++ {
		candidate := payload[i:]
		if isValidHex(candidate) {
			return payload[:i], candidate
		}
	}

	return payload, ""
}

// isValidHex checks if a string contains only valid hex characters.
func isValidHex(s string) bool {
	if len(s) == 0 || len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		isDigit := c >= '0' && c <= '9'
		isUpperHex := c >= 'A' && c <= 'F'
		isLowerHex := c >= 'a' && c <= 'f'
		if !isDigit && !isUpperHex && !isLowerHex {
			return false
		}
	}
	return true
}

// formatMessage creates a human-readable summary of the CPDLC message.
func formatMessage(msg *Message) string {
	if msg == nil || len(msg.Elements) == 0 {
		return ""
	}

	// If a free-text element is present, prefer that as the main summary
	// (this matches what libacars/acarslib typically surfaces for many uplinks/requests).
	for _, elem := range msg.Elements {
		if strings.EqualFold(strings.TrimSpace(elem.Label), "[freetext]") && strings.TrimSpace(elem.Text) != "" {
			return strings.TrimSpace(elem.Text)
		}
	}

	parts := make([]string, 0, len(msg.Elements))
	for _, elem := range msg.Elements {
		if strings.TrimSpace(elem.Text) != "" {
			parts = append(parts, strings.TrimSpace(elem.Text))
		} else if strings.TrimSpace(elem.Label) != "" {
			parts = append(parts, strings.TrimSpace(elem.Label))
		}
	}
	return strings.Join(parts, "; ")
}
