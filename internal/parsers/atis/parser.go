// Package atis parses ATIS (Automatic Terminal Information Service) broadcast messages.
// These contain weather, runway, and approach information for airports.
// Label A9 typically contains D-ATIS (digital ATIS) messages.
package atis

import (
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// Result represents parsed ATIS data.
type Result struct {
	MsgID       int64    `json:"message_id"`
	Timestamp   string   `json:"timestamp"`
	RawText     string   `json:"raw_text,omitempty"` // Full raw ATIS text.
	Airport     string   `json:"airport,omitempty"`
	ATISLetter  string   `json:"atis_letter,omitempty"`
	ATISType    string   `json:"atis_type,omitempty"` // ARR, DEP, or empty for combined.
	ATISTime    string   `json:"atis_time,omitempty"` // Zulu time of ATIS.
	Runways     []string `json:"runways,omitempty"`
	Approaches  []string `json:"approaches,omitempty"` // ILS, RNAV, etc.
	Wind        string   `json:"wind,omitempty"`
	Visibility  string   `json:"visibility,omitempty"`
	Clouds      string   `json:"clouds,omitempty"`
	Temperature string   `json:"temperature,omitempty"`
	DewPoint    string   `json:"dew_point,omitempty"`
	QNH         string   `json:"qnh,omitempty"`
	Remarks     []string `json:"remarks,omitempty"`
}

func (r *Result) Type() string     { return "atis" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser extracts ATIS information using token-based parsing.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "atis" }
func (p *Parser) Labels() []string { return []string{"A9"} }
func (p *Parser) Priority() int    { return 100 }

// Patterns for ATIS parsing.
var (
	// Envelope: /ICNDLXA.TI2/RKSI ARR ATIS O
	envelopeRe = regexp.MustCompile(`/([A-Z0-9]+)\.TI2/([A-Z]{4})\s+(ARR|DEP)?\s*ATIS\s+([A-Z])`)

	// Time: 0500Z or 1806Z
	timeRe = regexp.MustCompile(`\b(\d{4})Z\b`)

	// Wind: WIND 180/4KT, WIND 360/15KT VRB BTN 130/ AND 200/
	windRe = regexp.MustCompile(`WIND\s+(\d{3}/\d{1,3}KT(?:\s+(?:VRB|GUST)[^.]*)?|VRB\s+\d{1,3}KT)`)

	// QNH: QNH 1027, QNH 1015HPA
	qnhRe = regexp.MustCompile(`QNH\s+(\d{3,4})(?:HPA)?`)

	// Temperature: T MS 8, T 18, T MS 1
	tempRe = regexp.MustCompile(`\bT\s+(MS\s*)?\s*(\d{1,2})\b`)

	// Dew point: DP MS 17, DP 14, DP MS 8
	dewPointRe = regexp.MustCompile(`\bDP\s*(MS\s*)?\s*(\d{1,2})\b`)

	// Visibility: VIS 10KM, VIS 10KM OR MORE
	visRe = regexp.MustCompile(`VIS\s+(\d+KM(?:\s+OR\s+MORE)?)`)

	// Clouds: CLD BKN 3500FT, CLD FEW 2000FT, CAVOK
	cloudRe = regexp.MustCompile(`(?:CLD\s+([A-Z]+\s+\d+FT)|CAVOK)`)

	// Runway: RWY 15L, RWY 34, RWY 07C
	runwayRe = regexp.MustCompile(`RWY\s+(\d{1,2}[LCR]?)`)

	// Approach: ILS APCH, ILS Z APCH, RNAV APCH
	approachRe = regexp.MustCompile(`(ILS(?:\s+[A-Z])?\s+APCH|RNAV\s+APCH|VOR\s+APCH)`)
)

func (p *Parser) QuickCheck(text string) bool {
	upper := strings.ToUpper(text)
	return strings.Contains(upper, "ATIS") &&
		(strings.Contains(upper, ".TI2/") || strings.Contains(upper, "QNH") ||
			strings.Contains(upper, "WIND"))
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		RawText:   msg.Text, // Preserve original text.
	}

	text := strings.ToUpper(msg.Text)

	// Extract envelope info.
	if m := envelopeRe.FindStringSubmatch(text); len(m) >= 5 {
		result.Airport = m[2]
		result.ATISType = m[3] // May be empty.
		result.ATISLetter = m[4]
	}

	// Extract time.
	if m := timeRe.FindStringSubmatch(text); len(m) > 1 {
		result.ATISTime = m[1] + "Z"
	}

	// Extract wind.
	if m := windRe.FindStringSubmatch(text); len(m) > 1 {
		result.Wind = strings.TrimSpace(m[1])
	}

	// Extract QNH.
	if m := qnhRe.FindStringSubmatch(text); len(m) > 1 {
		result.QNH = m[1]
	}

	// Extract temperature.
	if m := tempRe.FindStringSubmatch(text); len(m) > 2 {
		temp := m[2]
		if strings.Contains(m[1], "MS") {
			temp = "-" + temp
		}
		result.Temperature = temp
	}

	// Extract dew point.
	if m := dewPointRe.FindStringSubmatch(text); len(m) > 2 {
		dp := m[2]
		if strings.Contains(m[1], "MS") {
			dp = "-" + dp
		}
		result.DewPoint = dp
	}

	// Extract visibility.
	if m := visRe.FindStringSubmatch(text); len(m) > 1 {
		result.Visibility = m[1]
	}

	// Check for CAVOK.
	if strings.Contains(text, "CAVOK") {
		result.Visibility = "CAVOK"
	}

	// Extract clouds.
	if m := cloudRe.FindStringSubmatch(text); len(m) > 1 && m[1] != "" {
		result.Clouds = m[1]
	}

	// Extract runways (may have multiple).
	runwayMatches := runwayRe.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	for _, m := range runwayMatches {
		if len(m) > 1 && !seen[m[1]] {
			result.Runways = append(result.Runways, m[1])
			seen[m[1]] = true
		}
	}

	// Extract approaches.
	approachMatches := approachRe.FindAllStringSubmatch(text, -1)
	seenApch := make(map[string]bool)
	for _, m := range approachMatches {
		if len(m) > 1 && !seenApch[m[1]] {
			result.Approaches = append(result.Approaches, m[1])
			seenApch[m[1]] = true
		}
	}

	// Extract remarks (cautions, warnings, etc.).
	result.Remarks = extractRemarks(text)

	// Only return if we got meaningful data.
	if result.Airport == "" && result.QNH == "" && len(result.Runways) == 0 {
		return nil
	}

	return result
}

// extractRemarks finds noteworthy items in the ATIS.
func extractRemarks(text string) []string {
	var remarks []string

	patterns := []struct {
		re   *regexp.Regexp
		name string
	}{
		{regexp.MustCompile(`CAUTION\s+([^.]+)`), ""},
		{regexp.MustCompile(`(RWY\s+\d{1,2}[LCR]?\s+(?:CLSD|CLOSED|UNUSABLE)[^.]*)`), ""},
		{regexp.MustCompile(`(BIRD\s+ACTIVITY)`), ""},
		{regexp.MustCompile(`(GPS\s+(?:SIGNAL\s+)?UNRELIABLE[^.]*)`), ""},
		{regexp.MustCompile(`(LOW\s+VISIBILITY[^.]*)`), ""},
		{regexp.MustCompile(`(WORK\s+IN\s+PROGRESS)`), ""},
	}

	for _, p := range patterns {
		if m := p.re.FindStringSubmatch(text); len(m) > 1 {
			remark := strings.TrimSpace(m[1])
			if remark != "" {
				remarks = append(remarks, remark)
			}
		}
	}

	return remarks
}