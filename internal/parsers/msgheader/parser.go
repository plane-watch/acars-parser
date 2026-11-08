// Package msgheader parses the header line that many airlines' aircraft put
// on their downlinks, on any label:
//
//	  01 WXRQ   0800/06 OMSJ/WMKK .A6-ATC
//	3N01 POSRPT UAE0069   /05 OMDB/EGKK .A6-JHB
//	1103 OFFRP  7953  /07 KLAX/EHAM .9V-SFP 9999 07112026 0748
//
// The header is a sequence code, the message type (a weather request, a
// position report, an off report, ...), the flight, the day of the month,
// the origin and destination, and the registration after a ".". The flight
// is the flight number's digits, sometimes with a suffix ("038A"), or the
// ICAO callsign ("UAE0069"). The body after the header differs by type and
// is not parsed.
package msgheader

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is a downlink's header line.
type Result struct {
	MsgID       int64  `json:"message_id"`
	Timestamp   string `json:"timestamp"`
	Sequence    string `json:"sequence"`     // e.g. "01", "3N01".
	MessageType string `json:"message_type"` // e.g. "WXRQ", "POSRPT".

	// FlightNumberDigits is the flight number without its airline code
	// ("0800"); the extractor uses the route only for a transmitted flight
	// with this number. Flight is the ICAO callsign ("UAE0069") instead.
	FlightNumberDigits string `json:"flight_number_digits,omitempty"`
	Flight             string `json:"flight,omitempty"`

	DayOfMonth  int    `json:"day_of_month"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`

	// Registration is reported only when it is the transmitted tail.
	Registration string `json:"registration,omitempty"`
}

func (r *Result) Type() string     { return "message_header" }
func (r *Result) MessageID() int64 { return r.MsgID }

// headerRe matches the header at the start of the message: the sequence
// code, the message type, the flight field (padded), the day, the route and
// the registration, which may be followed by other fields after a space, a
// "," (",2135") or a "/" ("/01F 02:03").
var headerRe = regexp.MustCompile(`^\s*([0-9A-Z]{2,4})\s+([A-Z]{3,6})\s+([A-Z0-9]{1,7})\s*/(\d{2})\s+([A-Z]{4})/([A-Z]{4})\s+\.([A-Z0-9-]{3,8})(?:[\s,/]|$)`)

// digitsRe matches a flight field that is the flight number's digits, with
// an optional suffix, which is not kept.
var digitsRe = regexp.MustCompile(`^(\d{1,4})[A-Z]{0,2}$`)

// callsignRe matches a flight field that is an ICAO callsign: three letters
// and a number, with an optional suffix, or a letter and a digit after the
// first digit ("UAE8UW").
var callsignRe = regexp.MustCompile(`^[A-Z]{3}\d[A-Z0-9]{0,3}$`)

// Parser parses downlink header lines.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string { return "msgheader" }

// Labels is empty: the header was seen on many labels (5U, 80, 10, 82, 88,
// 28, 3E, ...), so every message is checked.
func (p *Parser) Labels() []string { return nil }

func (p *Parser) Priority() int { return 90 }

// QuickCheck looks for the route and the registration's "." that follow the
// day.
func (p *Parser) QuickCheck(text string) bool {
	if len(text) < 30 {
		return false
	}
	head := text[:min(len(text), 60)]
	return strings.Contains(head, "/") && strings.Contains(head, " .")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	m := headerRe.FindStringSubmatch(msg.Text)
	if m == nil || !patterns.IsValidICAO(m[5]) || !patterns.IsValidICAO(m[6]) {
		return nil
	}
	day, _ := strconv.Atoi(m[4])
	if day < 1 || day > 31 {
		return nil
	}
	r := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Sequence:    m[1],
		MessageType: m[2],
		DayOfMonth:  day,
		Origin:      m[5],
		Destination: m[6],
	}
	switch flight := m[3]; {
	case digitsRe.MatchString(flight):
		r.FlightNumberDigits = digitsRe.FindStringSubmatch(flight)[1]
	case callsignRe.MatchString(flight):
		r.Flight = flight
	default:
		// A field that is not a flight names one that cannot be compared
		// with the transmitted flight.
		return nil
	}
	// A header naming another aircraft than the transmitted tail is not
	// parsed, so that its route is not credited to the transmitting
	// aircraft; without a transmitted tail, no registration is reported.
	if msg.Tail != "" {
		if acars.NormaliseRegistration(m[7]) != acars.NormaliseRegistration(msg.Tail) {
			return nil
		}
		r.Registration = m[7]
	}
	return r
}
