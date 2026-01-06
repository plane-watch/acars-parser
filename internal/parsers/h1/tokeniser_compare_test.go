package h1

import (
	"testing"

	"acars_parser/internal/acars"
)

// TestTokeniserMatchesRegex verifies that the tokeniser-based FPN parser
// correctly extracts fields from known FPN messages.
func TestTokeniserMatchesRegex(t *testing.T) {
	tests := []struct {
		name string
		text string
		// Expected values.
		origin      string
		destination string
		flightNum   string
		hasRoute    bool
	}{
		{
			name:        "simple QFA flight",
			text:        "FPN/SN2993/FNQFA401/RI:DA:YSSY:CR:SYDMEL001:AA:YMML..WOL,S34334E150474.H65.LEECE.Q29.BOOINC817",
			origin:      "YSSY",
			destination: "YMML",
			flightNum:   "QFA401",
			hasRoute:    true, // Inline route after AA.
		},
		{
			name:        "US domestic with F section",
			text:        "FPN/RP:DA:KCLT:AA:KBOS:CR:KCLTKBOS(22L)..BESSI.Q22.RBV.Q419.JFK:A:ROBUC3.JFK:F:VECTOR..DISCO..EGGRL:AP:RNVY 22L.EGGRL:F:WINNI2DCD",
			origin:      "KCLT",
			destination: "KBOS",
			hasRoute:    true,
		},
		{
			name:        "Tampa to LaGuardia",
			text:        "FPN/RP:DA:KTPA:AA:KLGA:CR:KTPAKLGA(22O)..RID01..GEA01..HUR01..GRO01..HYTRA:A:PROUD2.HURTS:F:VECTOR..DISCO..YOMAN:AP:ILS 22:F:PROUD1C6D",
			origin:      "KTPA",
			destination: "KLGA",
			hasRoute:    true,
		},
		{
			name:        "Milwaukee to Chicago",
			text:        "4W092050/RA:DA:KMKE:AA:KORD754E",
			origin:      "KMKE",
			destination: "KORD",
			hasRoute:    false,
		},
	}

	// Initialise parser (now tokeniser-based).
	parser := &FPNParser{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse with tokeniser.
			tokens := TokeniseFPN(tt.text)

			// Parse with FPNParser.
			msg := &acars.Message{
				ID:    1,
				Label: "H1",
				Text:  tt.text,
			}
			result := parser.Parse(msg)

			// Compare origin.
			tokenOrigin := tokens.GetOrigin()
			if tokenOrigin != tt.origin {
				t.Errorf("Tokeniser origin = %q, want %q", tokenOrigin, tt.origin)
			}

			// Compare destination.
			tokenDest := tokens.GetDestination()
			if tokenDest != tt.destination {
				t.Errorf("Tokeniser destination = %q, want %q", tokenDest, tt.destination)
			}

			// Compare flight number.
			if tt.flightNum != "" && tokens.FlightNum != tt.flightNum {
				t.Errorf("Tokeniser flightNum = %q, want %q", tokens.FlightNum, tt.flightNum)
			}

			// Check route extraction.
			hasInlineRoute := tokens.GetInlineRoute() != ""
			hasRouteSection := tokens.GetRoute() != ""
			tokenHasRoute := hasInlineRoute || hasRouteSection
			if tokenHasRoute != tt.hasRoute {
				t.Errorf("Tokeniser hasRoute = %v, want %v (inline=%q, section=%q)",
					tokenHasRoute, tt.hasRoute, tokens.GetInlineRoute(), tokens.GetRoute())
			}

			// Verify FPNParser result matches tokeniser.
			if result != nil {
				fpn := result.(*FPNResult)

				if tokenOrigin != fpn.Origin {
					t.Errorf("Tokeniser origin %q != parser origin %q", tokenOrigin, fpn.Origin)
				}
				if tokenDest != fpn.Destination {
					t.Errorf("Tokeniser dest %q != parser dest %q", tokenDest, fpn.Destination)
				}

				// Log for visibility.
				t.Logf("Parser: origin=%s dest=%s flight=%s waypoints=%d",
					fpn.Origin, fpn.Destination, fpn.FlightNum, len(fpn.Waypoints))
				t.Logf("Token: origin=%s dest=%s flight=%s sections=%v",
					tokenOrigin, tokenDest, tokens.FlightNum, getSectionKeys(tokens))
			} else {
				t.Logf("Parser returned nil (tokeniser found origin=%s dest=%s)",
					tokenOrigin, tokenDest)
			}
		})
	}
}

// getSectionKeys returns the section marker keys for logging.
func getSectionKeys(tokens *FPNTokens) []string {
	keys := make([]string, 0, len(tokens.Sections))
	for k := range tokens.Sections {
		keys = append(keys, k)
	}
	return keys
}

// TestTokeniserHandlesNewlines verifies the tokeniser handles embedded newlines
// that can appear in ACARS transmission.
func TestTokeniserHandlesNewlines(t *testing.T) {
	// This message has a newline embedded in the coordinates.
	text := `FPN/SN2993/FNQFA401/RI:DA:YSSY:CR:SYDMEL001:AA:YMML..WOL,S34334E15
0474.H65.LEECE.Q29.BOOINC817`

	tokens := TokeniseFPN(text)

	if tokens.GetOrigin() != "YSSY" {
		t.Errorf("Origin = %q, want YSSY", tokens.GetOrigin())
	}
	if tokens.GetDestination() != "YMML" {
		t.Errorf("Destination = %q, want YMML", tokens.GetDestination())
	}

	// The inline route should have the newline stripped.
	inline := tokens.GetInlineRoute()
	if inline == "" {
		t.Error("Expected inline route to be extracted")
	}
	if containsNewline(inline) {
		t.Errorf("Inline route still contains newline: %q", inline)
	}

	// Should contain the full coordinate.
	if !containsSubstring(inline, "S34334E150474") {
		t.Errorf("Inline route should contain rejoined coordinate, got: %q", inline)
	}
}

func containsNewline(s string) bool {
	for _, c := range s {
		if c == '\n' || c == '\r' {
			return true
		}
	}
	return false
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestTokeniserExtraFieldsNotInRegex verifies the tokeniser can extract
// all ARINC 622/633 section markers.
func TestTokeniserExtraFieldsNotInRegex(t *testing.T) {
	// Complex message with many sections.
	text := "FPN/SN123:DA:KSFO:D:PORTE2:AA:KORD:A:BENKY4:F:MERIT..DIXIE:AP:ILS22L:R:22L:CR:SFOORD"

	tokens := TokeniseFPN(text)

	// Verify all sections extracted.
	expectedSections := map[string]string{
		"DA": "KSFO",
		"D":  "PORTE2",
		"AA": "KORD",
		"A":  "BENKY4",
		"F":  "MERIT..DIXIE",
		"AP": "ILS22L",
		"R":  "22L",
		"CR": "SFOORD",
	}

	for marker, expected := range expectedSections {
		if tokens.Sections[marker] != expected {
			t.Errorf("Section[%s] = %q, want %q", marker, tokens.Sections[marker], expected)
		}
	}

	// Verify getter methods work.
	if tokens.GetDeparture() != "PORTE2" {
		t.Errorf("GetDeparture() = %q, want PORTE2", tokens.GetDeparture())
	}
	if tokens.GetArrival() != "BENKY4" {
		t.Errorf("GetArrival() = %q, want BENKY4", tokens.GetArrival())
	}
	if tokens.GetRunway() != "22L" {
		t.Errorf("GetRunway() = %q, want 22L", tokens.GetRunway())
	}
}
