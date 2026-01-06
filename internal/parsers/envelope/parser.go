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
	MsgID        int64   `json:"message_id"`
	Timestamp    string  `json:"timestamp"`
	Tail         string  `json:"tail,omitempty"`
	Station      string  `json:"station,omitempty"`
	MessageType  string  `json:"message_type,omitempty"` // AT1, CR1, ADS
	PayloadBytes int     `json:"payload_bytes,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`  // From ADS-C position reports.
	Longitude    float64 `json:"longitude,omitempty"` // From ADS-C position reports.
	Altitude     string  `json:"altitude,omitempty"`  // Flight level from ADS-C.
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
	if result.Tail == "" && msg.Airframe != nil && msg.Airframe.Tail != "" {
		result.Tail = msg.Airframe.Tail
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

		// Strip CRC from payload and decode.
		data = data[:len(data)-2]
		result.PayloadBytes = len(data)

		// Decode ADS-C payload if present.
		if result.MessageType == "ADS" && len(data) >= 3 {
			decodeADSCData(result, data)
		}
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
	regPart := textPrefix[3:] // Skip IMI (3 chars).
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

// decodeADSCData extracts position and altitude from ADS-C binary payload.
// ADS-C uses a TLV (tag-length-value) structure embedded in the envelope.
func decodeADSCData(result *Result, data []byte) {
	if len(data) < 1 {
		return
	}

	msgType := data[0]

	// Type 0x07: Basic group with altitude in TLV format.
	if msgType == 0x07 {
		decodeADSCBasic(result, data)
	}

	// Type 0x08: Earth reference group with position in TLV format.
	if msgType == 0x08 {
		decodeADSCPosition(result, data)
	}
}

// decodeADSCBasic decodes type 0x07 basic group containing altitude.
// TLV tags in this group:
//   - 0x0B: Altitude (1 byte) - flight level.
//   - 0x0C: Vertical rate (1 byte) - climb/descend indicator.
//   - 0x0D: Track (1 byte) - heading scaled to 360°.
//   - 0x0E, 0x0F, 0x10, 0x15: Other 1-byte fields.
func decodeADSCBasic(result *Result, data []byte) {
	i := 2 // Skip type and group bytes.
	for i < len(data)-1 {
		tag := data[i]

		switch {
		case tag == 0x0B && i+1 < len(data):
			// Altitude tag: next byte is flight level.
			fl := int(data[i+1])
			if fl > 0 && fl <= 600 {
				result.Altitude = fmt.Sprintf("FL%03d", fl)
			}
			i += 2

		case (tag == 0x0C || tag == 0x0D || tag == 0x0E ||
			tag == 0x0F || tag == 0x10 || tag == 0x15) && i+1 < len(data):
			// Other 1-byte fields. Skip without extracting.
			i += 2

		default:
			i++
		}
	}
}

// decodeADSCPosition decodes type 0x08 earth reference group containing lat/lon.
// TLV tags in this group:
//   - 0x0A: Ground speed (2 bytes) - not extracted but must skip correctly.
//   - 0x0B: Altitude (1 byte) - flight level.
//   - 0x12: Latitude (2 bytes) - 16-bit signed, scaled to ±90°.
//   - 0x13: Longitude (2 bytes) - 16-bit signed, scaled to ±180°.
//   - 0x14: FOM/timestamp (2 bytes) - not extracted but must skip correctly.
func decodeADSCPosition(result *Result, data []byte) {
	i := 2 // Skip type and group bytes.
	for i < len(data)-1 {
		tag := data[i]

		switch {
		case tag == 0x0A && i+2 < len(data):
			// Ground speed tag: 2 bytes. Skip without extracting.
			i += 3

		case tag == 0x0B && i+1 < len(data):
			// Altitude tag: 1 byte flight level.
			fl := int(data[i+1])
			if fl > 0 && fl <= 600 {
				result.Altitude = fmt.Sprintf("FL%03d", fl)
			}
			i += 2

		case tag == 0x12 && i+2 < len(data):
			// Latitude tag: 2 bytes are 16-bit signed value.
			raw := int(data[i+1])<<8 | int(data[i+2])
			if raw > 32767 {
				raw -= 65536
			}
			lat := float64(raw) * 90.0 / 32768.0
			if lat >= -90 && lat <= 90 {
				result.Latitude = lat
			}
			i += 3

		case tag == 0x13 && i+2 < len(data):
			// Longitude tag: 2 bytes are 16-bit signed value.
			raw := int(data[i+1])<<8 | int(data[i+2])
			if raw > 32767 {
				raw -= 65536
			}
			lon := float64(raw) * 180.0 / 32768.0
			if lon >= -180 && lon <= 180 {
				result.Longitude = lon
			}
			i += 3

		case tag == 0x14 && i+2 < len(data):
			// FOM/timestamp tag: 2 bytes. Skip without extracting.
			i += 3

		default:
			i++
		}
	}
}