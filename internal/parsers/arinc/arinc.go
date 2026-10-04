// Package arinc implements ARINC 622/623 message parsing with CRC validation.
// This layer sits between raw ACARS messages and protocol-specific decoders (CPDLC, ADS-C, etc.).
package arinc

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"acars_parser/internal/crc"
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

// messagePattern matches an ARINC 622 binary message in envelope form:
// "/", the ground station (4 to 7 characters), ".", the IMI (AT1, CR1, CC1
// or DR1), the registration field and the hex payload. The registration
// field is seven characters, padded on the left with dots (".N514DN",
// "..N17RX"; "B-18772" has none), so it is read by its length: the payload
// cannot be told from the registration's last characters ("EC-NMZ" ends in
// hex digits).
var messagePattern = regexp.MustCompile(`^/([A-Z0-9]{4,7})\.([A-Z]{2}[0-9])([A-Z0-9.-]{7})([0-9A-F]*)$`)

// relayedPattern matches an ARINC 622 message relayed in label H1 with its
// original label, e.g. "- #MD/AA PIKCPYA.AT1.N657UA...": a "- #" sublabel,
// "/", the original label (AA for CPDLC, A6 for ADS-C, A0 for AFN) and a
// space. In binary applications (AT1, ADS) the IMI is followed by the
// seven-character registration field, which starts with "." unless the
// registration has seven characters; in character-oriented ones (AFN), by
// "/".
var relayedPattern = regexp.MustCompile(`^- #[A-Z0-9]{2}/([A-Z0-9]{2}) ([A-Z0-9]{4,7}\.[A-Z]{2}[A-Z0-9][A-Z0-9./-].+)$`)

// barePattern matches an ARINC 622 message without its leading "/", as
// label H1 also carries it, e.g. "USADCXA.AT1.N200WN...".
var barePattern = regexp.MustCompile(`^[A-Z0-9]{4,7}\.[A-Z]{2}[A-Z0-9][A-Z0-9./-]`)

// envelopePattern matches an ARINC 622 message in envelope form.
var envelopePattern = regexp.MustCompile(`^/[A-Z0-9]{4,7}\.[A-Z]{2}[A-Z0-9][A-Z0-9./-]`)

// Unwrap returns an ARINC 622 message in envelope form
// ("/<ground_station>.<IMI>.<registration><hex>" for binary applications,
// which Parse reads, or "/<ground_station>.<IMI>/<text>" for character ones)
// from the forms in which label H1 carries it: relayed with its original
// label ("- #MD/AA ..."), whose label is returned, or without the leading
// "/". A message already in envelope form is returned unchanged. ok is false
// for any other text.
func Unwrap(text string) (msg, label string, ok bool) {
	text = strings.TrimSpace(text)
	if m := relayedPattern.FindStringSubmatch(text); m != nil {
		return "/" + m[2], m[1], true
	}
	if barePattern.MatchString(text) {
		return "/" + text, "", true
	}
	if envelopePattern.MatchString(text) {
		return text, "", true
	}
	return "", "", false
}

// Parse parses an ARINC binary message, validates CRC, and returns the payload.
// Returns ErrCRCFailed if CRC validation fails.
// Returns other errors for format/parsing issues.
func Parse(text string) (*Result, error) {
	matches := messagePattern.FindStringSubmatch(text)
	if matches == nil {
		return nil, fmt.Errorf("%w: does not match ARINC format", ErrUnknownFormat)
	}
	groundStation, imi, regField, hexStr := matches[1], matches[2], matches[3], matches[4]

	hexData, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidHex, err)
	}
	if len(hexData) < 2 {
		return nil, fmt.Errorf("%w: need at least 2 bytes for CRC", ErrTooShort)
	}

	// The CRC covers the IMI and the registration field as transmitted
	// (ten characters), then the payload.
	if !crc.VerifyArincBinaryRaw(imi+regField, hexData) {
		return nil, ErrCRCFailed
	}

	return &Result{
		GroundStation: groundStation,
		IMI:           imi,
		Registration:  strings.TrimLeft(regField, "."),
		Payload:       hexData[:len(hexData)-2],
		RawHex:        hexStr,
	}, nil
}

// splitRegistrationAndHex separates the aircraft registration from the hex payload.
// According to ARINC 622, the registration field is 6 characters after the dot following the IMI.
// The hex payload starts immediately after.
//
// Example: "HL8251243F880C..." -> registration "HL8251", hex "243F880C..."
//
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
