// Package swareport parses the header of Southwest Airlines' ACMS reports
// (label H1): engine, performance and system reports sent under some fifty
// report codes. For example:
//
//	72740,8668,B737-800,260107,WN0183,KLGA,KHOU,1101,SW2501
//	19.10.46,CR,0871,17170,301.8,.621,-21.7,...
//
// The header is the report code, Southwest's fleet number, the aircraft
// type, the date (YYMMDD), the flight, origin and destination, an
// unidentified number and the system ID. The flight data reports that
// start with "++" have the same header and are parsed by trajectory.
//
// The fleet number is not reported: it is the airline's own number, which
// is not always the registration's digits (N500WR reports 8636), so the
// transmitted tail is what identifies the aircraft. In the January 2026
// corpus, no tail reported two types. The report body is not parsed.
package swareport

import (
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is the header of a Southwest ACMS report.
type Result struct {
	MsgID        int64  `json:"message_id"`
	Timestamp    string `json:"timestamp"`
	Report       string `json:"report"`        // The report code, e.g. 72740.
	AircraftType string `json:"aircraft_type"` // As transmitted, e.g. B737-800, B737-8MAX.
	Date         string `json:"date"`          // YYMMDD.
	Flight       string `json:"flight"`        // IATA flight number, e.g. WN0183.
	Origin       string `json:"origin"`
	Destination  string `json:"destination"`
	SystemID     string `json:"system_id"` // e.g. SW2501.
}

func (r *Result) Type() string     { return "swa_report" }
func (r *Result) MessageID() int64 { return r.MsgID }

// headerRe matches the header line.
var headerRe = regexp.MustCompile(`^([0-9A-Z]{5}),[0-9A-Z]{3,4},(B7[0-9]7-[0-9]{3}|B737-[789]MAX),(\d{6}),([A-Z0-9]{2}\d{1,4}[A-Z]?),([A-Z]{4}),([A-Z]{4}),\d+,([A-Z0-9-]+)\s*(?:\r?\n|$)`)

// Parser parses Southwest ACMS report headers.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "swareport" }
func (p *Parser) Labels() []string { return []string{"H1"} }
func (p *Parser) Priority() int    { return 60 }

func (p *Parser) QuickCheck(text string) bool {
	return len(text) > 12 && text[5] == ',' && strings.Contains(text[:min(len(text), 20)], ",B7")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	m := headerRe.FindStringSubmatch(msg.Text)
	if m == nil || !patterns.IsValidICAO(m[5]) || !patterns.IsValidICAO(m[6]) {
		return nil
	}
	return &Result{
		MsgID:        int64(msg.ID),
		Timestamp:    msg.Timestamp,
		Report:       m[1],
		AircraftType: m[2],
		Date:         m[3],
		Flight:       m[4],
		Origin:       m[5],
		Destination:  m[6],
		SystemID:     m[7],
	}
}
