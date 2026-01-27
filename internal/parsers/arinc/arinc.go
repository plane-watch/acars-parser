// Package arinc implements ARINC 622/623 message parsing with CRC validation.
// This layer sits between raw ACARS messages and protocol-specific decoders (CPDLC, ADS-C, etc.).
package arinc

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// IMI (Imbedded Message Identifier) types for ARINC binary messages.
const (
	IMIAT1 = "AT1" // CPDLC Air-to-Ground.
	IMICR1 = "CR1" // CPDLC Connect Request.
	IMICC1 = "CC1" // CPDLC Connect Confirm.
	IMIDR1 = "DR1" // CPDLC Disconnect Request.
	IMIADS = "ADS" // ADS-C.
	IMIDIS = "DIS" // ADS-C Disconnect.
)

// Error types for distinguishing failure modes.
var (
	ErrCRCFailed     = errors.New("crc_failed")
	ErrParseFailed   = errors.New("parse_failed")
	ErrTooShort      = errors.New("message_too_short")
	ErrInvalidHex    = errors.New("invalid_hex")
	ErrUnknownFormat = errors.New("unknown_format")
)

// Result contains the parsed ARINC message components.
type Result struct {
	GroundStation string // e.g., "SOUCAYA".
	IMI           string // e.g., "AT1", "CR1".
	Registration  string // e.g., "HL8251".
	Payload       []byte // CRC-stripped binary payload.
	RawHex        string // Original hex including CRC (for diagnostics).
}

// messagePattern matches ARINC binary message format.
// Format: /<ground_station>.<IMI>.<registration><hex_payload>
// Ground station is 4-7 uppercase alphanumeric chars.
// IMI is 2-3 chars (AT1, CR1, CC1, DR1, ADS, DIS).
// Registration + hex follows.
var messagePattern = regexp.MustCompile(`^/([A-Z0-9]{4,7})\.([A-Z]{2,3}[0-9])\.(.+)$`)

// Parse parses an ARINC binary message, validates CRC, and returns the payload.
// Returns ErrCRCFailed if CRC validation fails.
// Returns other errors for format/parsing issues.
func Parse(text string) (*Result, error) {
	matches := messagePattern.FindStringSubmatch(text)
	if matches == nil {
		return nil, fmt.Errorf("%w: does not match ARINC format", ErrUnknownFormat)
	}

	groundStation := matches[1]
	imi := matches[2]
	regAndHex := matches[3]

	// Split registration from hex payload.
	// Registration is variable length (up to 7 chars), hex starts at first valid hex sequence.
	registration, hexStr := splitRegistrationAndHex(regAndHex)
	if hexStr == "" {
		return nil, fmt.Errorf("%w: no hex payload found", ErrParseFailed)
	}

	// Decode hex to binary.
	hexData, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidHex, err)
	}

	// Need at least 2 bytes for CRC.
	if len(hexData) < 2 {
		return nil, fmt.Errorf("%w: need at least 2 bytes for CRC", ErrTooShort)
	}

	// Validate CRC.
	if !validateCRC(imi, registration, hexData) {
		return nil, ErrCRCFailed
	}

	// Strip CRC (last 2 bytes) from payload.
	payload := hexData[:len(hexData)-2]

	return &Result{
		GroundStation: groundStation,
		IMI:           imi,
		Registration:  registration,
		Payload:       payload,
		RawHex:        hexStr,
	}, nil
}

// splitRegistrationAndHex separates the aircraft registration from the hex payload.
// According to ARINC 622, the registration field is 6 characters after the dot following the IMI.
// The hex payload starts immediately after.
//
// Example: "HL8251243F880C..." -> registration "HL8251", hex "243F880C..."
//
// If the registration is shorter than 6 chars (rare), the hex starts earlier.
// We detect this by checking if the 6-char split produces valid hex.
func splitRegistrationAndHex(s string) (registration, hexStr string) {
	s = strings.ToUpper(s)

	// Standard case: 6-char registration field.
	if len(s) > 6 {
		reg := s[:6]
		hex := s[6:]
		if len(hex)%2 == 0 && isValidHex(hex) {
			return reg, hex
		}
	}

	// Fallback: try different registration lengths (5, 7, etc.).
	// This handles edge cases where registration is not exactly 6 chars.
	for regLen := 5; regLen <= 8 && regLen < len(s); regLen++ {
		if regLen == 6 {
			continue // Already tried.
		}
		reg := s[:regLen]
		hex := s[regLen:]
		if len(hex)%2 == 0 && isValidHex(hex) && hasLetter(reg) {
			return reg, hex
		}
	}

	// Last resort: find any valid hex suffix.
	for i := 1; i < len(s); i++ {
		candidate := s[i:]
		if len(candidate)%2 == 0 && isValidHex(candidate) && hasLetter(s[:i]) {
			return s[:i], candidate
		}
	}

	return s, ""
}

// isValidHex checks if a string is valid hexadecimal with even length.
func isValidHex(s string) bool {
	if len(s) == 0 || len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// hasLetter checks if string contains at least one letter.
func hasLetter(s string) bool {
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	return false
}

// IsCPDLC returns true if the IMI indicates a CPDLC message type.
func IsCPDLC(imi string) bool {
	switch imi {
	case IMIAT1, IMICR1, IMICC1, IMIDR1:
		return true
	}
	return false
}

// IsADSC returns true if the IMI indicates an ADS-C message type.
func IsADSC(imi string) bool {
	switch imi {
	case IMIADS, IMIDIS:
		return true
	}
	return false
}
