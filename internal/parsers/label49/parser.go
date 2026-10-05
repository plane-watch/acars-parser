// Package label49 parses the header of label 49 downlinks, which many
// airlines send, for example:
//
//	01ERDC    ETD103/311802OMAALEMD
//	+38996265.0- 30.7
//
// The header is "01", a four-letter report code (ERDC, TATO, DCAP, ICCL and
// others, whose meanings are not established), the callsign padded with
// spaces on either side, "/", six digits that look like the day, hour and
// minute (DDHHMM), and the origin and destination. The lines after the
// header are not parsed.
package label49

import (
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is the header of a label 49 downlink.
type Result struct {
	MsgID       int64  `json:"message_id"`
	Timestamp   string `json:"timestamp"`
	Report      string `json:"report"`   // e.g. ERDC.
	Flight      string `json:"flight"`   // The callsign, e.g. ETD103.
	DayTime     string `json:"day_time"` // DDHHMM, as sent.
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
}

func (r *Result) Type() string     { return "label49" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// headerRe matches the header line.
	headerRe = regexp.MustCompile(`^01([A-Z]{4})([ A-Z0-9]{3,12})/(\d{6})([A-Z]{4})([A-Z]{4})[ \t]*(?:\r?\n|$)`)

	// callsignRe matches the trimmed callsign: an ICAO or IATA airline code
	// and a flight number with an optional suffix.
	callsignRe = regexp.MustCompile(`^(?:[A-Z]{3}|[A-Z0-9]{2})\d{1,4}[A-Z]{0,2}$`)
)

// Parser parses label 49 headers.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "label49" }
func (p *Parser) Labels() []string { return []string{"49"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "01")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	m := headerRe.FindStringSubmatch(msg.Text)
	if m == nil {
		return nil
	}
	flight := strings.TrimSpace(m[2])
	if !callsignRe.MatchString(flight) || !patterns.IsValidICAO(m[4]) || !patterns.IsValidICAO(m[5]) {
		return nil
	}
	return &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Report:      m[1],
		Flight:      flight,
		DayTime:     m[3],
		Origin:      m[4],
		Destination: m[5],
	}
}
