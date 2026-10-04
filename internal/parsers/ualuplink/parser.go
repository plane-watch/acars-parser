// Package ualuplink parses the header of United Airlines uplinks (label RA)
// from its operations system, address QUNDCULUA. The first line names the
// message ("TURB SIGMET", "GATE ASSIGN", "EPNF INFO") and an early line names
// the flight it is for: the flight number, its day of the month, and the
// origin and destination. The flight line is the line after the title, or
// after a part marker ("** PART 01 OF 01 **") that follows it. For example:
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

	// flightRe matches the flight line, which may be indented: the flight
	// number, "/" or "-", the day, the origin and destination.
	flightRe = regexp.MustCompile(`^[ \t]*(UAL?\d{1,4})[/-](\d{2})[ \t]+([A-Z]{4})[ \t]+([A-Z]{4})[ \t]*$`)

	// partRe matches a part marker, such as "** PART 01 OF 01 **".
	partRe = regexp.MustCompile(`^[ \t]*\*\* PART \d+ OF \d+ \*\*[ \t]*$`)
)

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
	// The flight line is the line after the title, or after a part marker
	// that follows the title. A flight-like line anywhere else is body text
	// and is not read.
	text := strings.ReplaceAll(msg.Text, "\r\n", "\n")
	lines := strings.Split(strings.ReplaceAll(text, "\r", "\n"), "\n")
	i := 1
	if i < len(lines) && partRe.MatchString(lines[i]) {
		i++
	}
	if i >= len(lines) {
		return nil
	}
	f := flightRe.FindStringSubmatch(lines[i])
	if f == nil {
		return nil
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
