// Package deltaheader parses the route header that Delta's aircraft put on
// many downlinks (labels 10 to 48). For example, on label 24:
//
//	041124 KATL KIAD7
//	/FN 1324
//
// The header line is six digits, the origin and destination, and one digit.
// The last two of the six digits echo the message's label, which Parse
// requires. The first four look like a day and hour, but in October 2026
// about a quarter of them differed from the day of receipt while the
// message's flight number was the current one, so their meaning is not
// established and they are not reported; nor is the final digit. Some
// messages have a "/FN" line with the flight number's digits.
//
// The body after the header differs by label and is not parsed.
package deltaheader

import (
	"regexp"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is the route header of a Delta downlink.
type Result struct {
	MsgID       int64  `json:"message_id"`
	Timestamp   string `json:"timestamp"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	// FlightNumberDigits is the flight number from the "/FN" line, without
	// the airline code, e.g. "1324".
	FlightNumberDigits string `json:"flight_number_digits,omitempty"`
}

func (r *Result) Type() string     { return "delta_header" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// headerRe matches the header line: four digits, the label echo, the
	// origin, the destination and a digit, alone on the first line.
	headerRe = regexp.MustCompile(`^\d{4}([0-9A-Z]{2}) ([A-Z]{4}) ([A-Z]{4})\d[ \t]*(?:\r?\n|$)`)

	// flightRe matches the "/FN" line on the second line.
	flightRe = regexp.MustCompile(`^/FN (\d{1,4})[ \t]*(?:\r?\n|$)`)
)

// Parser parses Delta route headers.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string { return "deltaheader" }

// Labels are the labels on which the header was seen in the archive.
func (p *Parser) Labels() []string {
	return []string{"10", "11", "12", "13", "14", "15", "17", "20", "21", "24", "26", "27",
		"30", "32", "36", "37", "38", "39", "44", "45", "48"}
}

func (p *Parser) Priority() int { return 60 }

func (p *Parser) QuickCheck(text string) bool {
	return len(text) >= 17 && text[6] == ' ' && text[11] == ' '
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	m := headerRe.FindStringSubmatchIndex(msg.Text)
	if m == nil {
		return nil
	}
	text := msg.Text
	label, origin, dest := text[m[2]:m[3]], text[m[4]:m[5]], text[m[6]:m[7]]
	if label != msg.Label || !patterns.IsValidICAO(origin) || !patterns.IsValidICAO(dest) {
		return nil
	}
	result := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Origin:      origin,
		Destination: dest,
	}
	if f := flightRe.FindStringSubmatch(text[m[1]:]); f != nil {
		result.FlightNumberDigits = f[1]
	}
	return result
}
