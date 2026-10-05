// Package unitedheader parses the header line that United's aircraft put
// on many downlinks, for example on label 14:
//
//	/14 OFF EVENT      / KFSD KDEN 19 153705/TIME 1537
//
// The header is a two-character code, a title padded to the "/", the
// origin and destination, the day of the month and the time (HHMMSS). On
// the labels parsed here the code is the label, which Parse requires. The
// same header on label 5Z, where the code is a sub-label (C3, R3, ...), is
// parsed by the eta parser. The body after the header differs by code and
// is not parsed.
package unitedheader

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is the header of a United downlink.
type Result struct {
	MsgID       int64  `json:"message_id"`
	Timestamp   string `json:"timestamp"`
	Code        string `json:"code"`  // The label, e.g. "14".
	Title       string `json:"title"` // e.g. "OFF EVENT".
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	DayOfMonth  int    `json:"day_of_month"`
	ReportTime  string `json:"report_time"` // HHMMSS.
}

func (r *Result) Type() string     { return "united_header" }
func (r *Result) MessageID() int64 { return r.MsgID }

// headerRe matches the header at the start of the message.
var headerRe = regexp.MustCompile(`^/([0-9A-Z]{2}) ([A-Z0-9 ]{14,16})/ ([A-Z]{4}) ([A-Z]{4}) (\d{2}) (\d{6})`)

// Parser parses United downlink headers.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string { return "unitedheader" }

// Labels are the labels on which the header, with the label as its code,
// was seen in the archive (other than 5Z, which eta parses).
func (p *Parser) Labels() []string {
	return []string{"11", "12", "13", "14", "15", "16", "17", "18", "19", "1E", "1G", "1M", "1R",
		"22", "23", "27", "2R", "33"}
}

func (p *Parser) Priority() int { return 60 }

func (p *Parser) QuickCheck(text string) bool {
	return len(text) > 20 && text[0] == '/' && text[3] == ' '
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	m := headerRe.FindStringSubmatch(msg.Text)
	if m == nil || m[1] != msg.Label || !patterns.IsValidICAO(m[3]) || !patterns.IsValidICAO(m[4]) {
		return nil
	}
	day, _ := strconv.Atoi(m[5])
	return &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Code:        m[1],
		Title:       strings.TrimSpace(m[2]),
		Origin:      m[3],
		Destination: m[4],
		DayOfMonth:  day,
		ReportTime:  m[6],
	}
}
