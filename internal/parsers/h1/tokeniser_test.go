package h1

import (
	"testing"
)

func TestNormaliseFPN(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no changes needed",
			input:    "FPN/SN123:DA:YSSY:AA:YMML",
			expected: "FPN/SN123:DA:YSSY:AA:YMML",
		},
		{
			name:     "strip carriage returns",
			input:    "FPN/SN123\r:DA:YSSY\r:AA:YMML",
			expected: "FPN/SN123:DA:YSSY:AA:YMML",
		},
		{
			name:     "strip newlines",
			input:    "FPN/SN123\n:DA:YSSY\n:AA:YMML",
			expected: "FPN/SN123:DA:YSSY:AA:YMML",
		},
		{
			name:     "strip tabs",
			input:    "FPN/SN123\t:DA:YSSY\t:AA:YMML",
			expected: "FPN/SN123:DA:YSSY:AA:YMML",
		},
		{
			name:     "strip CRLF combination",
			input:    "FPN/SN123\r\n:DA:YSSY\r\n:AA:YMML",
			expected: "FPN/SN123:DA:YSSY:AA:YMML",
		},
		{
			name:     "coordinate with embedded newline",
			input:    ":AA:YMML..WOL,S34334E15\n0474",
			expected: ":AA:YMML..WOL,S34334E150474",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormaliseFPN(tt.input)
			if result != tt.expected {
				t.Errorf("NormaliseFPN() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTokeniseFPN(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedHeader  string
		expectedSerial  string
		expectedFlight  string
		expectedOrigin  string
		expectedDest    string
		expectedRoute   string
		expectedSections map[string]string
	}{
		{
			name:           "simple flight plan",
			input:          "FPN/SN2993/FNQFA401:DA:YSSY:AA:YMML",
			expectedHeader: "FPN/SN2993/FNQFA401",
			expectedSerial: "2993",
			expectedFlight: "QFA401",
			expectedOrigin: "YSSY",
			expectedDest:   "YMML",
			expectedSections: map[string]string{
				"DA": "YSSY",
				"AA": "YMML",
			},
		},
		{
			name:           "with company route between DA and AA",
			input:          "FPN/SN2993/FNQFA401/RI:DA:YSSY:CR:SYDMEL001:AA:YMML",
			expectedHeader: "FPN/SN2993/FNQFA401/RI",
			expectedSerial: "2993",
			expectedFlight: "QFA401",
			expectedOrigin: "YSSY",
			expectedDest:   "YMML",
			expectedSections: map[string]string{
				"DA": "YSSY",
				"CR": "SYDMEL001",
				"AA": "YMML",
			},
		},
		{
			name:           "with inline waypoints after AA",
			input:          "FPN/SN2993/FNQFA401:DA:YSSY:AA:YMML..WOL,S34334E150474",
			expectedHeader: "FPN/SN2993/FNQFA401",
			expectedSerial: "2993",
			expectedFlight: "QFA401",
			expectedOrigin: "YSSY",
			expectedDest:   "YMML",
			expectedSections: map[string]string{
				"DA": "YSSY",
				"AA": "YMML..WOL,S34334E150474",
			},
		},
		{
			name:           "with F route section",
			input:          "FPN/SN123/FNAAL92:DA:KJFK:AA:KLAX:F:MERIT..DIXIE..WHALE",
			expectedHeader: "FPN/SN123/FNAAL92",
			expectedSerial: "123",
			expectedFlight: "AAL92",
			expectedOrigin: "KJFK",
			expectedDest:   "KLAX",
			expectedRoute:  "MERIT..DIXIE..WHALE",
			expectedSections: map[string]string{
				"DA": "KJFK",
				"AA": "KLAX",
				"F":  "MERIT..DIXIE..WHALE",
			},
		},
		{
			name:           "with SID and STAR",
			input:          "FPN/SN456/FNUAL123:DA:KSFO:D:PORTE2:AA:KORD:A:BENKY4:F:ROUTE",
			expectedHeader: "FPN/SN456/FNUAL123",
			expectedSerial: "456",
			expectedFlight: "UAL123",
			expectedOrigin: "KSFO",
			expectedDest:   "KORD",
			expectedRoute:  "ROUTE",
			expectedSections: map[string]string{
				"DA": "KSFO",
				"D":  "PORTE2",
				"AA": "KORD",
				"A":  "BENKY4",
				"F":  "ROUTE",
			},
		},
		{
			name:           "with approach",
			input:          "FPN/SN789:DA:KBOS:AA:KJFK:AP:ILS22L..ZIGEE,N37312W102468",
			expectedHeader: "FPN/SN789",
			expectedSerial: "789",
			expectedOrigin: "KBOS",
			expectedDest:   "KJFK",
			expectedSections: map[string]string{
				"DA": "KBOS",
				"AA": "KJFK",
				"AP": "ILS22L..ZIGEE,N37312W102468",
			},
		},
		{
			name:           "complex real-world message",
			input:          "FPN/RP:DA:KCLT:AA:KBOS:CR:KCLTKBOS(22L)..BESSI.Q22.RBV.Q419.JFK:A:ROBUC3.JFK:F:VECTOR..DISCO..EGGRL:AP:RNVY 22L.EGGRL",
			expectedHeader: "FPN/RP",
			expectedOrigin: "KCLT",
			expectedDest:   "KBOS",
			expectedSections: map[string]string{
				"DA": "KCLT",
				"AA": "KBOS",
				"CR": "KCLTKBOS(22L)..BESSI.Q22.RBV.Q419.JFK",
				"A":  "ROBUC3.JFK",
				"F":  "VECTOR..DISCO..EGGRL",
				"AP": "RNVY 22L.EGGRL",
			},
		},
		{
			name:           "with embedded newline in coordinates",
			input:          "FPN/SN2993/FNQFA401:DA:YSSY:AA:YMML..WOL,S34334E15\n0474",
			expectedHeader: "FPN/SN2993/FNQFA401",
			expectedSerial: "2993",
			expectedFlight: "QFA401",
			expectedOrigin: "YSSY",
			expectedDest:   "YMML",
			expectedSections: map[string]string{
				"DA": "YSSY",
				"AA": "YMML..WOL,S34334E150474", // Newline stripped.
			},
		},
		{
			name:           "no section markers",
			input:          "FPN/SN123/FNQFA401",
			expectedHeader: "FPN/SN123/FNQFA401",
			expectedSerial: "123",
			expectedFlight: "QFA401",
			expectedSections: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := TokeniseFPN(tt.input)

			if tokens.Header != tt.expectedHeader {
				t.Errorf("Header = %q, want %q", tokens.Header, tt.expectedHeader)
			}
			if tokens.SerialNum != tt.expectedSerial {
				t.Errorf("SerialNum = %q, want %q", tokens.SerialNum, tt.expectedSerial)
			}
			if tokens.FlightNum != tt.expectedFlight {
				t.Errorf("FlightNum = %q, want %q", tokens.FlightNum, tt.expectedFlight)
			}
			if tt.expectedOrigin != "" && tokens.GetOrigin() != tt.expectedOrigin {
				t.Errorf("GetOrigin() = %q, want %q", tokens.GetOrigin(), tt.expectedOrigin)
			}
			if tt.expectedDest != "" && tokens.GetDestination() != tt.expectedDest {
				t.Errorf("GetDestination() = %q, want %q", tokens.GetDestination(), tt.expectedDest)
			}
			if tt.expectedRoute != "" && tokens.GetRoute() != tt.expectedRoute {
				t.Errorf("GetRoute() = %q, want %q", tokens.GetRoute(), tt.expectedRoute)
			}

			// Check all expected sections are present.
			for marker, expectedValue := range tt.expectedSections {
				if tokens.Sections[marker] != expectedValue {
					t.Errorf("Sections[%q] = %q, want %q", marker, tokens.Sections[marker], expectedValue)
				}
			}

			// Check no unexpected sections.
			for marker := range tokens.Sections {
				if _, ok := tt.expectedSections[marker]; !ok {
					t.Errorf("Unexpected section %q = %q", marker, tokens.Sections[marker])
				}
			}
		})
	}
}

func TestGetInlineRoute(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no inline route",
			input:    "FPN:DA:YSSY:AA:YMML",
			expected: "",
		},
		{
			name:     "inline route present",
			input:    "FPN:DA:YSSY:AA:YMML..WOL,S34334E150474.H65.LEECE",
			expected: "WOL,S34334E150474.H65.LEECE",
		},
		{
			name:     "AA section too short",
			input:    "FPN:DA:YSSY:AA:YM",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := TokeniseFPN(tt.input)
			result := tokens.GetInlineRoute()
			if result != tt.expected {
				t.Errorf("GetInlineRoute() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHasSection(t *testing.T) {
	tokens := TokeniseFPN("FPN:DA:YSSY:AA:YMML:F:ROUTE")

	if !tokens.HasSection("DA") {
		t.Error("HasSection(DA) should be true")
	}
	if !tokens.HasSection("AA") {
		t.Error("HasSection(AA) should be true")
	}
	if !tokens.HasSection("F") {
		t.Error("HasSection(F) should be true")
	}
	if tokens.HasSection("D") {
		t.Error("HasSection(D) should be false")
	}
	if tokens.HasSection("AP") {
		t.Error("HasSection(AP) should be false")
	}
}

func TestGetters(t *testing.T) {
	input := "FPN/SN123:DA:KSFO:D:PORTE2:AA:KORD:A:BENKY4:F:ROUTE..WPTS:AP:ILS22L:R:22L:CR:SFOORD"
	tokens := TokeniseFPN(input)

	tests := []struct {
		name   string
		getter func() string
		want   string
	}{
		{"GetOrigin", tokens.GetOrigin, "KSFO"},
		{"GetDestination", tokens.GetDestination, "KORD"},
		{"GetDeparture", tokens.GetDeparture, "PORTE2"},
		{"GetArrival", tokens.GetArrival, "BENKY4"},
		{"GetRoute", tokens.GetRoute, "ROUTE..WPTS"},
		{"GetApproach", tokens.GetApproach, "ILS22L"},
		{"GetRunway", tokens.GetRunway, "22L"},
		{"GetCompanyRoute", tokens.GetCompanyRoute, "SFOORD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.getter()
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
