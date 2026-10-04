// Package envelope parses aircraft registration from ACARS envelope headers.
// Handles AA (AT1/CR1) and A6 (ADS) labels which contain binary payloads
// but have structured headers with tail numbers embedded.
package envelope

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/crc"
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

// tailPatterns for different registration formats.
// Order matters - more specific patterns first.
var tailPatterns = []*regexp.Regexp{
	// European format with hyphen: F-GSQC, D-AIMH, G-XLEI, CS-TUI, PH-AOE, etc.
	// 1-2 letter prefix, hyphen, 3-4 letters (no digits after hyphen).
	regexp.MustCompile(`^([A-Z]{1,2}-[A-Z]{3,4})`),

	// Australian: VH-ZNM (VH prefix is common).
	regexp.MustCompile(`^(VH-[A-Z]{3})`),

	// Chinese/HK B-numbers: B-LQC (letters), B-1341 (digits), B-227M (mixed).
	regexp.MustCompile(`^(B-[A-Z0-9]{3,4})`),

	// Turkish: TC-LLH.
	regexp.MustCompile(`^(TC-[A-Z]{3})`),

	// US N-numbers: N784AV, N4649K, N879FD.
	regexp.MustCompile(`^(N[0-9]{1,5}[A-Z]{0,2})`),

	// Japanese: JA792A, JA884A.
	regexp.MustCompile(`^(JA[0-9]{3,4}[A-Z]?)`),

	// Korean: HL8382, HL8250.
	regexp.MustCompile(`^(HL[0-9]{4})`),

	// Singapore: 9V-OJA.
	regexp.MustCompile(`^(9V-[A-Z]{3})`),

	// Malaysia: 9M-MRO.
	regexp.MustCompile(`^(9M-[A-Z]{3})`),

	// Qatar: A7-ANR.
	regexp.MustCompile(`^(A7-[A-Z]{3})`),

	// UAE: A6-BLP.
	regexp.MustCompile(`^(A6-[A-Z]{3})`),

	// Oman: A4O-SK.
	regexp.MustCompile(`^(A4O-[A-Z]{2,3})`),

	// Thailand: HS-THU.
	regexp.MustCompile(`^(HS-[A-Z]{3})`),

	// Vietnamese: VN-A897.
	regexp.MustCompile(`^(VN-[A-Z][0-9]{3})`),

	// Spanish: EC-MNS, EC-NGT.
	regexp.MustCompile(`^(EC-[A-Z]{3})`),

	// Swiss: HB-IHF.
	regexp.MustCompile(`^(HB-[A-Z]{3})`),

	// Finnish: OH-LTS.
	regexp.MustCompile(`^(OH-[A-Z]{3})`),

	// Norwegian: LN-FNE.
	regexp.MustCompile(`^(LN-[A-Z]{3})`),

	// Swedish: SE-RSG.
	regexp.MustCompile(`^(SE-[A-Z]{3})`),
}

func (p *Parser) QuickCheck(text string) bool {
	// Must start with envelope header.
	return strings.HasPrefix(text, "/") && (strings.Contains(text, ".AT1.") ||
		strings.Contains(text, ".CR1.") ||
		strings.Contains(text, ".ADS"))
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
	}

	text := strings.TrimSpace(msg.Text)

	// Parse the envelope message to extract station, type, tail, and the raw text prefix for CRC.
	station, msgType, tail, textPrefix, hexPayload := parseEnvelopeWithPrefix(text)

	result.Station = station
	result.MessageType = msgType
	result.Tail = tail

	// Fallback: use tail from ACARS envelope if available.
	if result.Tail == "" && msg.Tail != "" {
		result.Tail = msg.Tail
	}

	// Verify CRC if we have the required components.
	if textPrefix != "" && hexPayload != "" {
		data, err := hex.DecodeString(hexPayload)
		if err != nil || len(data) < 3 {
			return nil // Invalid hex payload.
		}

		// Verify CRC using the raw 10-char text prefix.
		if !crc.VerifyArincBinaryRaw(textPrefix, data) {
			return nil // CRC mismatch - reject message.
		}

		// Strip the CRC from the payload. The payload is not decoded: on
		// label A6 it is an ADS-C contract request from the ground, which
		// holds no position (the aircraft's reports are on B6, decoded by
		// the adsc parser).
		data = data[:len(data)-2]
		result.PayloadBytes = len(data)
	}

	// Only return if we extracted something useful.
	if result.Tail == "" && result.Station == "" {
		return nil
	}

	return result
}

// parseEnvelopeWithPrefix parses an envelope message and extracts:
// - station: The ground station address.
// - msgType: The message type (AT1, CR1, ADS).
// - tail: The clean aircraft registration.
// - textPrefix: The raw 10-char text prefix for CRC verification (IMI + separator + registration).
// - hexPayload: The hex-encoded binary payload.
func parseEnvelopeWithPrefix(text string) (station, msgType, tail, textPrefix, hexPayload string) {
	if !strings.HasPrefix(text, "/") {
		return
	}

	// Remove leading slash.
	text = text[1:]

	// Find the IMI marker (.AT1, .CR1, .ADS).
	var imiIdx int
	for _, marker := range []string{".AT1", ".CR1", ".ADS"} {
		if idx := strings.Index(text, marker); idx >= 0 {
			imiIdx = idx
			msgType = marker[1:] // Strip the leading dot.
			break
		}
	}

	if msgType == "" || imiIdx < 4 {
		return
	}

	// Extract station (everything before the IMI marker).
	station = text[:imiIdx]

	// The text prefix for CRC starts after the dot before IMI.
	// Format: IMI (3 chars) + separator/registration (7 chars) = 10 chars total.
	prefixStart := imiIdx + 1 // Skip the dot before IMI.
	if len(text) < prefixStart+10 {
		return
	}

	textPrefix = text[prefixStart : prefixStart+10]
	remaining := text[prefixStart+10:]

	// The remaining text should be hex data.
	if len(remaining) >= 4 && len(remaining)%2 == 0 {
		if _, err := hex.DecodeString(remaining); err == nil {
			hexPayload = remaining
		}
	}

	// Extract clean tail from the text prefix (chars 4-10, after IMI and dot).
	// The format is "IMI.REG" where REG may include leading dots for short registrations.
	regPart := textPrefix[3:]                // Skip IMI (3 chars).
	regPart = strings.TrimLeft(regPart, ".") // Strip leading dots.

	// Try to extract tail - include some of the hex to help pattern matching.
	if len(remaining) >= 6 {
		tail = extractTail(regPart + remaining[:6])
	}
	if tail == "" {
		tail = extractTail(regPart)
	}

	return
}

// extractTail cleans and validates a tail number candidate.
func extractTail(candidate string) string {
	candidate = strings.ToUpper(candidate)

	for _, re := range tailPatterns {
		if m := re.FindStringSubmatch(candidate); len(m) > 1 {
			return m[1]
		}
	}

	return ""
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
		trace.QuickCheck.Reason = "No envelope format (/.AT1., .CR1., or .ADS) found"
		return trace
	}

	text := strings.TrimSpace(msg.Text)

	// Parse envelope components.
	station, msgType, tail, textPrefix, hexPayload := parseEnvelopeWithPrefix(text)

	// Add extractors for envelope parsing steps.
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "station",
		Pattern: "envelope header before IMI marker",
		Matched: station != "",
		Value:   station,
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "message_type",
		Pattern: ".AT1, .CR1, or .ADS",
		Matched: msgType != "",
		Value:   msgType,
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "tail",
		Pattern: "aircraft registration patterns",
		Matched: tail != "",
		Value:   tail,
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "text_prefix",
		Pattern: "10-char CRC prefix",
		Matched: textPrefix != "",
		Value:   textPrefix,
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "hex_payload",
		Pattern: "hex-encoded binary data",
		Matched: hexPayload != "",
		Value:   fmt.Sprintf("%d bytes", len(hexPayload)/2),
	})

	// Determine if overall match succeeds.
	trace.Matched = tail != "" || station != ""

	return trace
}
