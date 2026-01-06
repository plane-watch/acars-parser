// Package patterns provides extraction functions for ACARS message parsing.
package patterns

import (
	"strings"
)

// ExtractFlightNumber attempts to extract a flight number from text.
func ExtractFlightNumber(text string, tokens []string) string {
	// Try labeled patterns first.
	if m := FlightNumFltPattern.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	if m := FlightNumCtxPattern.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}

	// Look for flight number in first few tokens.
	for i, tok := range tokens {
		if i > 10 {
			break
		}
		if FlightNumPattern.MatchString(tok) {
			// Verify it looks like a flight number (not an airport).
			if !ICAOPattern.MatchString(tok) && len(tok) >= 4 && len(tok) <= 7 {
				return tok
			}
		}
	}

	// Try trailing pattern.
	if m := FlightNumTrailPattern.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}

	return ""
}

// ExtractAirports extracts origin and destination ICAO codes from text.
func ExtractAirports(text string, tokens []string) (origin, destination string) {
	upperText := strings.ToUpper(text)

	// Pattern 1: "CLRD TO XXXX" - destination after CLRD TO.
	if idx := strings.Index(upperText, "CLRD TO "); idx >= 0 {
		after := upperText[idx+8:]
		if m := FindValidICAO(after); m != "" {
			destination = m
		}
	}

	// Pattern 2: "CLEARED TO XXXX".
	if destination == "" {
		if idx := strings.Index(upperText, "CLEARED TO "); idx >= 0 {
			after := upperText[idx+11:]
			if m := FindValidICAO(after); m != "" {
				destination = m
			}
		}
	}

	// Pattern 3: Look for "XXXX-XXXX" route format.
	if m := RoutePattern.FindStringSubmatch(upperText); len(m) == 3 {
		if IsValidICAO(m[1]) && IsValidICAO(m[2]) {
			origin = m[1]
			destination = m[2]
			return
		}
	}

	// Pattern 4: Look for "XXXX ... XXXX" where first is origin and last is destination.
	// Common in PDC route lines like "KPHL DITCH LUIGI HNNAH CYUL".
	allICAO := FindAllValidICAO(upperText)
	if len(allICAO) >= 2 {
		origin = allICAO[0]
		// Use the LAST valid ICAO as destination (more reliable for route lines).
		destination = allICAO[len(allICAO)-1]
		return
	}

	// Single ICAO found.
	if len(allICAO) == 1 {
		if strings.Contains(upperText, "CLRD TO") || strings.Contains(upperText, "CLEARED TO") {
			destination = allICAO[0]
		} else {
			origin = allICAO[0]
		}
	}

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
