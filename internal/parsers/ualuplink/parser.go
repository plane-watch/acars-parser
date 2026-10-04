// Package ualuplink parses the header of United Airlines uplinks (label RA)
// from its operations system, address QUNDCULUA. The first line names the
// message ("TURB SIGMET", "GATE ASSIGN", "EPNF INFO") and an early line names
// the flight it is for: the flight number, its day of the month, and the
// origin and destination. For example:
//
//	QUNDCULUA~1TURB SIGMET
//		UAL252-04 PHNL KIAH
//		YOUR FLIGHT IS WITHIN A
//
// ACARS uplinks carry no flight ID field (acarsdec reports one only for
// downlink blocks), so this header is the only transmitted statement of the
// flight on these messages. The day was checked against the January 2026
// corpus: in 99.5% of messages it is the day of the message or the day
// before.
package ualuplink

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is the header of a United uplink.
type Result struct {
	MsgID       int64  `json:"message_id"`
	Timestamp   string `json:"timestamp"`
	Title       string `json:"title"`  // e.g. TURB SIGMET.
	Flight      string `json:"flight"` // As transmitted: UA0187 (IATA) or UAL252 (ICAO).
	Day         int    `json:"day"`    // The flight's day of the month.
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
}

func (r *Result) Type() string     { return "united_uplink" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// titleRe matches the first line: the sender's address, "~", a digit
	// and the title.
	titleRe = regexp.MustCompile(`^QUNDCULUA~\d([^\r\n]*)`)

	// flightRe matches the flight line, which may be indented with a tab:
	// the flight number, "/" or "-", the day, the origin and destination.
	flightRe = regexp.MustCompile(`^\t?(UAL?\d{1,4})[/-](\d{2}) +([A-Z]{4}) ([A-Z]{4})\s*$`)
)

// maxHeaderLines is how many lines after the title are searched for the
// flight line; it is the first or, after a "** PART 01 OF 01 **" line, the
// second.
const maxHeaderLines = 3

// Parser parses United uplink headers.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "ualuplink" }
func (p *Parser) Labels() []string { return []string{"RA"} }
func (p *Parser) Priority() int    { return 60 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "QUNDCULUA~")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	t := titleRe.FindStringSubmatch(msg.Text)
	if t == nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(msg.Text, "\r", ""), "\n")
	for i := 1; i < len(lines) && i <= maxHeaderLines; i++ {
		f := flightRe.FindStringSubmatch(lines[i])
		if f == nil {
			continue
		}
		day, _ := strconv.Atoi(f[2])
		if day < 1 || day > 31 || !patterns.IsValidICAO(f[3]) || !patterns.IsValidICAO(f[4]) {
			return nil
		}
		return &Result{
			MsgID:       int64(msg.ID),
			Timestamp:   msg.Timestamp,
			Title:       strings.TrimSpace(t[1]),
			Flight:      f[1],
			Day:         day,
			Origin:      f[3],
			Destination: f[4],
		}
	}
	return nil
}
