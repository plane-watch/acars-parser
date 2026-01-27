// Package patterns provides extraction functions for ACARS message parsing.
package patterns

import (
	"strings"
)

// ExtractFlightNumber attempts to extract a flight number from text.
// Returns empty string if no valid flight number is found.
// Rejects garbage like ":PAD11" and hex-like patterns (Mode-S addresses).
func ExtractFlightNumber(text string, tokens []string) string {
	// Try ICAO-format patterns first (e.g., "ASA329 B738" or "UAL123 CLRD").
	// These are more reliable than numeric-only extractions.
	if m := FlightNumCtxPattern.FindStringSubmatch(text); len(m) > 1 {
		if isValidFlightNumber(m[1]) {
			return m[1]
		}
	}

	// Try "FLT XXX" pattern as fallback. Note: this extracts numeric-only,
	// which may not be the ICAO callsign (e.g., "FLT 329" vs "ASA329").
	if m := FlightNumFltPattern.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}

	// Look for flight number in first few tokens.
	for i, tok := range tokens {
		if i > 10 {
			break
		}
		// Extract the actual match from the token, not the whole token.
		// This prevents returning ":PAD11" when we only matched "PAD11".
		if m := FlightNumPattern.FindString(tok); m != "" {
			// Verify it looks like a flight number (not an airport or garbage).
			if isValidFlightNumber(m) {
				return m
			}
		}
	}

	// Try trailing pattern.
	if m := FlightNumTrailPattern.FindStringSubmatch(text); len(m) > 1 {
		if isValidFlightNumber(m[1]) {
			return m[1]
		}
	}

	return ""
}

// isValidFlightNumber checks if a string is a valid flight number.
// Rejects:
// - Strings that look like Mode-S hex addresses (e.g., A32AD, 8C3323)
// - Strings that look like ICAO airport codes
// - Strings that are too short or too long
func isValidFlightNumber(s string) bool {
	if len(s) < 3 || len(s) > 7 {
		return false
	}

	// Reject if it looks like an ICAO airport code.
	if ICAOPattern.MatchString(s) {
		return false
	}

	// Reject if it looks like a Mode-S hex address.
	// Hex addresses are 6 hex chars (0-9A-F), often starting with country prefix.
	// Valid flight numbers have format: 2-3 letters + 1-4 digits + optional letter.
	// Hex addresses often have letters mixed with numbers throughout.
	if looksLikeHexAddress(s) {
		return false
	}

	// Must match the expected flight number format.
	return FlightNumPattern.MatchString(s)
}

// looksLikeHexAddress checks if a string looks like a Mode-S hex address.
// Hex addresses are typically 6 hex characters and don't follow the
// airline code + flight number pattern.
func looksLikeHexAddress(s string) bool {
	// If it's all hex characters and 6 chars long, likely a hex address.
	if len(s) == 6 && isAllHex(s) {
		return true
	}

	// Check for patterns like A3xxxx (Greek), 8xxxxx (various), etc.
	// These have letters mixed throughout, unlike flight numbers which
	// have letters at start and optionally end, with digits in middle.
	if len(s) >= 5 {
		// Count transitions between letter and digit.
		// Flight numbers: AAA123 = 1 transition, AA123A = 2 transitions
		// Hex addresses: A32AD = 3 transitions, 8C3323 = 2+ transitions with digit start
		transitions := 0
		for i := 1; i < len(s); i++ {
			prevIsDigit := s[i-1] >= '0' && s[i-1] <= '9'
			currIsDigit := s[i] >= '0' && s[i] <= '9'
			if prevIsDigit != currIsDigit {
				transitions++
			}
		}
		// Flight numbers typically have 1-2 transitions (letters then digits, maybe trailing letter).
		// Hex with mixed patterns have more transitions or start with digit.
		startsWithDigit := s[0] >= '0' && s[0] <= '9'
		if startsWithDigit && transitions >= 2 {
			return true
		}
		if transitions >= 3 {
			return true
		}
	}

	return false
}

// isAllHex checks if a string contains only hexadecimal characters.
func isAllHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// findICAONextWord extracts the next word and checks if it's a valid ICAO code.
// This is more precise than FindValidICAO which scans the entire text.
func findICAONextWord(text string) string {
	// Skip leading whitespace.
	text = strings.TrimLeft(text, " \t\n\r")
	if text == "" {
		return ""
	}

	// Extract the next word (up to whitespace or punctuation).
	end := 0
	for end < len(text) && text[end] != ' ' && text[end] != '\t' && text[end] != '\n' &&
		text[end] != '\r' && text[end] != ',' && text[end] != '/' && text[end] != '-' {
		end++
	}

	word := text[:end]
	if IsValidICAO(word) {
		return word
	}
	return ""
}

// ExtractAirports extracts origin and destination ICAO codes from text.
// Uses context-aware extraction - only extracts airports when there's clear
// contextual signal (keywords, route formats). Does NOT guess based on finding
// random 4-letter codes in the text.
func ExtractAirports(text string, tokens []string) (origin, destination string) {
	upperText := strings.ToUpper(text)

	// Destination patterns - extract destination from clearance context.
	destPatterns := []struct {
		keyword string
		offset  int
	}{
		{"CLRD TO ", 8},
		{"CLEARED TO ", 11},
		{"CLRD ", 5},
		{"DEST ", 5},
		{"DEST:", 5},
		{"DESTINATION ", 12},
		{"ARR ", 4},
		{"ARRIVING ", 9},
	}

	for _, p := range destPatterns {
		if destination != "" {
			break
		}
		if idx := strings.Index(upperText, p.keyword); idx >= 0 {
			after := upperText[idx+p.offset:]
			// Only look at the next word, not the entire remaining text.
			if m := findICAONextWord(after); m != "" {
				destination = m
			}
		}
	}

	// Origin patterns - extract origin from departure context.
	originPatterns := []struct {
		keyword string
		offset  int
	}{
		{"DEPART ", 7},
		{"DEPARTING ", 10},
		{"DEP ", 4},
		{"FROM ", 5},
		{"ORIGIN ", 7},
		{"ORG ", 4},
	}

	for _, p := range originPatterns {
		if origin != "" {
			break
		}
		if idx := strings.Index(upperText, p.keyword); idx >= 0 {
			after := upperText[idx+p.offset:]
			// Only look at the next word, not the entire remaining text.
			if m := findICAONextWord(after); m != "" {
				origin = m
			}
		}
	}

	// Route format: "XXXX-XXXX" or "XXXX/XXXX".
	if m := RoutePattern.FindStringSubmatch(upperText); len(m) == 3 {
		if IsValidICAO(m[1]) && IsValidICAO(m[2]) {
			if origin == "" {
				origin = m[1]
			}
			if destination == "" {
				destination = m[2]
			}
		}
	}

	// Slash-separated route format: "KORD/KJFK".
	if origin == "" || destination == "" {
		if m := routeSlashPattern.FindStringSubmatch(upperText); len(m) == 3 {
			if IsValidICAO(m[1]) && IsValidICAO(m[2]) {
				if origin == "" {
					origin = m[1]
				}
				if destination == "" {
					destination = m[2]
				}
			}
		}
	}

	// NOTE: Route strings like "KPHL DITCH LUIGI HNNAH CYUL" should be handled
	// by grok patterns in the appropriate parser, not by generic extraction.

	return
}

// ExtractRunway extracts departure runway from text.
func ExtractRunway(text string) string {
	upperText := strings.ToUpper(text)

	if m := RunwayOffPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}
	if m := RunwayRwyPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}
	if m := RunwayDepPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}

	return ""
}

// ExtractSID extracts Standard Instrument Departure from text.
func ExtractSID(text string) string {
	upperText := strings.ToUpper(text)

	if m := SIDViaPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}
	if m := SIDDepPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}
	if m := SIDNamedPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}
	if m := SIDClearedPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}
	// Handle word-based SID names like "SANEG TWO DEP" -> "SANEG2".
	if m := SIDWordPattern.FindStringSubmatch(upperText); len(m) > 2 {
		digit := WordToDigit(m[2])
		if digit != "" {
			return m[1] + digit
		}
	}

	return ""
}

// ExtractSquawk extracts transponder code from text.
func ExtractSquawk(text string) string {
	upperText := strings.ToUpper(text)

	if m := SquawkPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}

	return ""
}

// ExtractFrequency extracts departure frequency from text.
func ExtractFrequency(text string) string {
	upperText := strings.ToUpper(text)

	if m := FreqDepPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}
	if m := FreqPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}

	return ""
}

// ExtractAltitude extracts initial altitude and flight level from text.
func ExtractAltitude(text string) (altitude, flightLevel string) {
	upperText := strings.ToUpper(text)

	// Flight level.
	if m := FLPattern.FindStringSubmatch(upperText); len(m) > 1 {
		flightLevel = "FL" + m[1]
	} else if m := FLExpPattern.FindStringSubmatch(upperText); len(m) > 1 {
		flightLevel = "FL" + m[1]
	} else if m := FLRequestPattern.FindStringSubmatch(upperText); len(m) > 1 {
		flightLevel = "FL" + m[1]
	}

	// Altitude.
	if m := AltClimbPattern.FindStringSubmatch(upperText); len(m) > 1 {
		altitude = m[1]
	} else if m := AltPattern.FindStringSubmatch(upperText); len(m) > 1 {
		altitude = m[1]
	} else if m := AltMaintPattern.FindStringSubmatch(upperText); len(m) > 1 {
		altitude = m[1]
	} else if m := AltInitialPattern.FindStringSubmatch(upperText); len(m) > 1 {
		altitude = m[1]
	}

	return
}

// ExtractAircraftType extracts aircraft type code from text.
func ExtractAircraftType(text string) string {
	upperText := strings.ToUpper(text)

	// Try FAA pattern first (more specific).
	if m := AircraftFAAPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}

	// Try generic pattern - find all matches and return the first containing a digit.
	matches := AircraftDirectPattern.FindAllStringSubmatch(upperText, -1)
	for _, m := range matches {
		if len(m) > 1 && containsDigit(m[1]) {
			return m[1]
		}
	}

	return ""
}

// containsDigit checks if a string contains at least one digit.
func containsDigit(s string) bool {
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return true
		}
	}
	return false
}

// ExtractATIS extracts ATIS information letter from text.
func ExtractATIS(text string) string {
	upperText := strings.ToUpper(text)

	if m := ATISPattern.FindStringSubmatch(upperText); len(m) > 1 {
		return m[1]
	}

	return ""
}
