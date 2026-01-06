// Package turbulence parses turbulence advisory and SIGMET messages from ACARS.
package turbulence

import (
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// Result represents parsed turbulence data.
type Result struct {
	MsgID       int64  `json:"message_id"`
	Timestamp   string `json:"timestamp"`
	Tail        string `json:"tail,omitempty"`
	TurbType    string `json:"turb_type,omitempty"`    // TURB CAT, TURB NORMAL, etc.
	ID          string `json:"id,omitempty"`           // SIGMET/advisory ID
	Severity    string `json:"severity,omitempty"`     // LGT, MOD, SEV, MDT
	AltitudeLow string `json:"altitude_low,omitempty"` // FL300
	AltitudeHi  string `json:"altitude_hi,omitempty"`  // FL380
	ValidFrom   string `json:"valid_from,omitempty"`
	ValidTo     string `json:"valid_to,omitempty"`
	Movement    string `json:"movement,omitempty"`
	Description string `json:"description,omitempty"`
	EntryPoint  string `json:"entry_point,omitempty"`
	ExitPoint   string `json:"exit_point,omitempty"`
}

func (r *Result) Type() string     { return "turbulence" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses turbulence messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "turbulence" }
func (p *Parser) Labels() []string { return []string{"C1"} }
func (p *Parser) Priority() int    { return 65 } // Higher priority than weather.

// QuickCheck looks for turbulence keywords.
func (p *Parser) QuickCheck(text string) bool {
	upper := strings.ToUpper(text)
	return strings.Contains(upper, "TURB") &&
		(strings.Contains(upper, "SIGMET") ||
			strings.Contains(upper, "ADVISORY") ||
			strings.Contains(upper, "WSI"))
}

// Pattern matchers.
var (
	// Type pattern.
	typeRe = regexp.MustCompile(`\bTYPE[:\s]+([A-Z\s]+?)(?:\n|$)`)

	// ID pattern.
	idRe = regexp.MustCompile(`\bID[:\s]+(\d+)`)

	// Severity patterns.
	severityRe = regexp.MustCompile(`\b(?:SEVERITY|INTST)[:\s]+([A-Z\s]+?)(?:\n|$)`)
	sevRe2     = regexp.MustCompile(`\b(LGT|MOD|MDT|SEV|OCNL\s+MOD)\s+(?:\(\d+\)\s+)?INTENSITY`)

	// Altitude pattern.
	altRe = regexp.MustCompile(`\bALT[:\s]+FL(\d{3})(?:\s*[-TO]+\s*FL?(\d{3}))?`)

	// Valid period pattern.
	validRe = regexp.MustCompile(`\bVALID[:\s]+(\d{6}Z?)(?:\s*[-TO]+\s*(\d{6}Z?))?`)

	// Movement pattern.
	mvtRe = regexp.MustCompile(`\bMVT[:\s]+([A-Z0-9\s]+?)(?:\n|$)`)

	// Description pattern.
	discRe = regexp.MustCompile(`\bDISC[:\s]+(.+?)(?:\n|$)`)

	// Entry/exit pattern.
	entryExitRe = regexp.MustCompile(`\bENTRY/EXIT[:\s]+(\d+[NS]\d+[EW])\s*/\s*(\d+[NS]\d+[EW])`)
)

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	text := msg.Text

	// Extract type.
	if m := typeRe.FindStringSubmatch(text); len(m) > 1 {
		result.TurbType = strings.TrimSpace(m[1])
	}

	// Extract ID.
	if m := idRe.FindStringSubmatch(text); len(m) > 1 {
		result.ID = m[1]
	}

	// Extract severity.
	if m := severityRe.FindStringSubmatch(text); len(m) > 1 {
		result.Severity = strings.TrimSpace(m[1])
	} else if m := sevRe2.FindStringSubmatch(text); len(m) > 1 {
		result.Severity = strings.TrimSpace(m[1])
	}

	// Extract altitude range.
	if m := altRe.FindStringSubmatch(text); len(m) > 1 {
		result.AltitudeLow = "FL" + m[1]
		if len(m) > 2 && m[2] != "" {
			result.AltitudeHi = "FL" + m[2]
		}
	}

	// Extract valid period.
	if m := validRe.FindStringSubmatch(text); len(m) > 1 {
		result.ValidFrom = m[1]
		if len(m) > 2 && m[2] != "" {
			result.ValidTo = m[2]
		}
	}

	// Extract movement.
	if m := mvtRe.FindStringSubmatch(text); len(m) > 1 {
		result.Movement = strings.TrimSpace(m[1])
	}

	// Extract description.
	if m := discRe.FindStringSubmatch(text); len(m) > 1 {
		result.Description = strings.TrimSpace(m[1])
	}

	// Extract entry/exit points.
	if m := entryExitRe.FindStringSubmatch(text); len(m) > 2 {
		result.EntryPoint = m[1]
		result.ExitPoint = m[2]
	}

	// Only return if we got useful data.
	if result.Severity == "" && result.AltitudeLow == "" && result.ID == "" {
		return nil
	}

	return result
}