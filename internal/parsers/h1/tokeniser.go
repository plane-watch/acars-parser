// Package h1 provides tokenisation functions for ARINC 622/633 style messages.
package h1

import (
	"regexp"
	"strings"
)

// FPNTokens represents the tokenised sections of an FPN message.
type FPNTokens struct {
	// Header contains the portion before the first section marker.
	// Example: "FPN/SN2993/FNQFA401/RI" from "FPN/SN2993/FNQFA401/RI:DA:YSSY..."
	Header string

	// SerialNum is extracted from /SNxxxx in the header.
	SerialNum string

	// FlightNum is extracted from /FNxxxx in the header.
	FlightNum string

	// Sections maps section marker to its value.
	// Example: {"DA": "YSSY", "AA": "YMML", "F": "WOL..LEECE"}
	Sections map[string]string
}

// sectionMarkerRe matches ARINC 622/633 section markers like :DA:, :AA:, :F:, etc.
// Markers are 1-2 uppercase letters surrounded by colons.
var sectionMarkerRe = regexp.MustCompile(`:([A-Z]{1,2}):`)

// headerFlightRe extracts flight number from header.
// Format: /FN + airline (2-3 letters) + flight number (1-4 digits) + optional suffix (1-2 letters) + optional checksum.
// Examples: /FNAAL123, /FNSWR4WF, /FNQFA401
var headerFlightRe = regexp.MustCompile(`/FN([A-Z]{2,3}\d{1,4}[A-Z]{0,2})`)

// headerSerialRe extracts serial number from header.
// Format: /SN + digits.
var headerSerialRe = regexp.MustCompile(`/SN(\d+)`)

// NormaliseFPN strips transmission artefacts from FPN message text.
// This handles embedded newlines, carriage returns, and tabs that can
// appear mid-field due to ACARS transmission characteristics.
func NormaliseFPN(text string) string {
	// Strip carriage returns.
	text = strings.ReplaceAll(text, "\r", "")
	// Strip newlines.
	text = strings.ReplaceAll(text, "\n", "")
	// Strip tabs.
	text = strings.ReplaceAll(text, "\t", "")
	return text
}

// TokeniseFPN splits an FPN message into its component sections.
// It extracts the header (everything before the first :XX: marker) and
// builds a map of section markers to their values.
//
// Example input:
//
//	"FPN/SN2993/FNQFA401/RI:DA:YSSY:CR:SYDMEL001:AA:YMML..WOL"
//
// Example output:
//
//	FPNTokens{
//	    Header: "FPN/SN2993/FNQFA401/RI",
//	    SerialNum: "2993",
//	    FlightNum: "QFA401",
//	    Sections: {"DA": "YSSY", "CR": "SYDMEL001", "AA": "YMML..WOL"},
//	}
func TokeniseFPN(text string) *FPNTokens {
	// Normalise first to handle transmission artefacts.
	text = NormaliseFPN(text)

	tokens := &FPNTokens{
		Sections: make(map[string]string),
	}

	// Find all section marker positions.
	matches := sectionMarkerRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		// No section markers found - entire text is the header.
		tokens.Header = text
		parseHeader(tokens)
		return tokens
	}

	// Extract header (everything before the first marker).
	tokens.Header = text[:matches[0][0]]
	parseHeader(tokens)

	// If flight number not in header, search entire message.
	// This handles cases like "...:F:ROUTE/FNAAL123" where /FN is at the end.
	if tokens.FlightNum == "" {
		tokens.FlightNum = extractFlightNum(text)
	}

	// Extract each section.
	for i, match := range matches {
		// match[0]:match[1] is the full match ":XX:"
		// match[2]:match[3] is the captured group "XX"
		marker := text[match[2]:match[3]]

		// Value runs from end of this marker to start of next marker (or end of string).
		valueStart := match[1]
		var valueEnd int
		if i+1 < len(matches) {
			valueEnd = matches[i+1][0]
		} else {
			valueEnd = len(text)
		}

		value := text[valueStart:valueEnd]

		// Keep first occurrence only. Later occurrences are typically checksums
		// or suffixes (e.g., ":F:ROUTE:AP:ILS22:F:CHECKSUM").
		if _, exists := tokens.Sections[marker]; !exists {
			tokens.Sections[marker] = value
		}
	}

	return tokens
}

// parseHeader extracts serial number and flight number from the header portion.
func parseHeader(tokens *FPNTokens) {
	if tokens.Header == "" {
		return
	}

	// Extract serial number (only from header).
	if m := headerSerialRe.FindStringSubmatch(tokens.Header); len(m) > 1 {
		tokens.SerialNum = m[1]
	}

	// Extract flight number (from header).
	if m := headerFlightRe.FindStringSubmatch(tokens.Header); len(m) > 1 {
		tokens.FlightNum = m[1]
	}
}

// extractFlightNum searches the entire message for a flight number pattern.
// This handles cases where /FN appears at the end of the message (e.g., after the route).
func extractFlightNum(text string) string {
	if m := headerFlightRe.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
}

// GetOrigin returns the departure airport from the DA section.
// Returns empty string if not present.
func (t *FPNTokens) GetOrigin() string {
	da := t.Sections["DA"]
	if len(da) < 4 {
		return ""
	}
	// The DA section should start with a 4-letter ICAO code.
	// It may have additional content after (e.g., ":DA:YSSY:CR:..." would give "YSSY").
	return da[:4]
}

// GetDestination returns the arrival airport from the AA section.
// Handles inline waypoints that may follow the ICAO code.
// Example: "YMML..WOL,S34334E150474" returns "YMML".
func (t *FPNTokens) GetDestination() string {
	aa := t.Sections["AA"]
	if len(aa) < 4 {
		return ""
	}
	// Extract just the ICAO code (first 4 letters).
	return aa[:4]
}

// GetRoute returns the flight route from the F section.
// Strips any trailing /FN... suffix (flight number/checksum).
// Returns empty string if not present.
func (t *FPNTokens) GetRoute() string {
	route := t.Sections["F"]
	// Strip trailing /FN... (flight number appears at end of some messages).
	if idx := strings.Index(route, "/FN"); idx >= 0 {
		route = route[:idx]
	}
	// Strip trailing /RP, /SN, etc. (other header-style suffixes).
	if idx := strings.Index(route, "/RP"); idx >= 0 {
		route = route[:idx]
	}
	if idx := strings.Index(route, "/SN"); idx >= 0 {
		route = route[:idx]
	}
	return route
}

// GetInlineRoute extracts waypoints that appear after the destination ICAO
// in the AA section (format: ":AA:YMML..WOL,coords..NEXT").
// Returns empty string if no inline route present.
func (t *FPNTokens) GetInlineRoute() string {
	aa := t.Sections["AA"]
	if len(aa) <= 4 {
		return ""
	}

	// Look for ".." after the ICAO code indicating inline waypoints.
	remainder := aa[4:]
	if strings.HasPrefix(remainder, "..") {
		return remainder[2:] // Strip leading ".."
	}
	return ""
}

// GetDeparture returns the SID (departure procedure) from the D section.
func (t *FPNTokens) GetDeparture() string {
	return t.Sections["D"]
}

// GetArrival returns the STAR (arrival procedure) from the A section.
func (t *FPNTokens) GetArrival() string {
	return t.Sections["A"]
}

// GetApproach returns the approach procedure from the AP section.
// Strips any trailing /XX suffixes (e.g., /RA for route amendments).
func (t *FPNTokens) GetApproach() string {
	ap := t.Sections["AP"]
	// Strip trailing slash-prefixed sections (e.g., /RA, /FN, /RP).
	if idx := strings.Index(ap, "/"); idx >= 0 {
		ap = ap[:idx]
	}
	return ap
}

// GetRunway returns the runway from the R section.
func (t *FPNTokens) GetRunway() string {
	return t.Sections["R"]
}

// GetCompanyRoute returns the company route identifier from the CR section.
func (t *FPNTokens) GetCompanyRoute() string {
	return t.Sections["CR"]
}

// HasSection checks if a section marker is present in the message.
func (t *FPNTokens) HasSection(marker string) bool {
	_, ok := t.Sections[marker]
	return ok
}
