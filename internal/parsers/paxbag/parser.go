// Package paxbag parses passenger and baggage detail messages for weight and balance.
package paxbag

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// ZoneCount contains passenger counts by zone.
type ZoneCount struct {
	Zone    string `json:"zone"`
	Adults  int    `json:"adults"`
	Male    int    `json:"male"`
	Female  int    `json:"female"`
	Children int   `json:"children"`
	Infants int    `json:"infants"`
}

// Result represents parsed passenger and baggage data.
type Result struct {
	MsgID         int64       `json:"message_id,omitempty"`
	AircraftType  string      `json:"aircraft_type,omitempty"`
	Registration  string      `json:"registration,omitempty"`
	Configuration string      `json:"configuration,omitempty"` // e.g., "68Y"
	FlightNumber  string      `json:"flight_number,omitempty"`
	Date          string      `json:"date,omitempty"`
	Origin        string      `json:"origin,omitempty"`
	Destination   string      `json:"destination,omitempty"`
	STD           string      `json:"std,omitempty"`           // Scheduled departure
	BoardingTime  string      `json:"boarding_time,omitempty"`
	Gate          string      `json:"gate,omitempty"`
	TotalPax      int         `json:"total_pax,omitempty"`
	Infants       int         `json:"infants,omitempty"`
	Adults        int         `json:"adults,omitempty"`
	Male          int         `json:"male,omitempty"`
	Female        int         `json:"female,omitempty"`
	Children      int         `json:"children,omitempty"`
	BagCount      int         `json:"bag_count,omitempty"`
	BagWeight     int         `json:"bag_weight,omitempty"`    // kg
	Zones         []ZoneCount `json:"zones,omitempty"`
	IsFinalised   bool        `json:"is_finalised"`
}

func (r *Result) Type() string     { return "pax_bag" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// FLIGHT INFO  AT7       REG OH-ATG   68Y
	flightInfoRe = regexp.MustCompile(`FLIGHT INFO\s+([A-Z0-9]+)\s+REG\s+([A-Z0-9-]+)\s+(\d+[A-Z])`)

	// AY1031   31DEC  HEL STD2120             BOARD 2050 GATE 9
	flightLineRe = regexp.MustCompile(`([A-Z]{2}\d+)\s+(\d{2}[A-Z]{3})\s+([A-Z]{3})\s+STD(\d{4})\s+BOARD\s+(\d{4})\s+GATE\s+(\S+)`)

	// TOTAL PASSENGERS 12 PLUS INFANTS 0
	totalPaxRe = regexp.MustCompile(`TOTAL PASSENGERS\s+(\d+)\s+PLUS INFANTS\s+(\d+)`)

	// JOINING  Y      0    9    3    0    0     3@56
	joiningRe = regexp.MustCompile(`JOINING\s+[A-Z]\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)@(\d+)`)

	// ZONE A      0    1    0    0    0
	zoneRe = regexp.MustCompile(`ZONE\s+([A-Z])\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(\d+)`)

	// Destination line: TLL or destination code
	destLineRe = regexp.MustCompile(`DEST\s+CABIN.*\n([A-Z]{3})`)
)

// Parser parses passenger and baggage detail messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string           { return "pax_bag" }
func (p *Parser) Labels() []string       { return []string{"RA"} }
func (p *Parser) Priority() int          { return 55 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "PAX AND BAG DETAILS")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := msg.Text

	if !p.QuickCheck(text) {
		return nil
	}

	result := &Result{
		MsgID:       int64(msg.ID),
		IsFinalised: !strings.Contains(text, "NOT FINALISED"),
	}

	// Parse flight info line.
	if m := flightInfoRe.FindStringSubmatch(text); m != nil {
		result.AircraftType = m[1]
		result.Registration = m[2]
		result.Configuration = m[3]
	}

	// Parse flight details line.
	if m := flightLineRe.FindStringSubmatch(text); m != nil {
		result.FlightNumber = m[1]
		result.Date = m[2]
		result.Origin = m[3]
		result.STD = m[4]
		result.BoardingTime = m[5]
		result.Gate = m[6]
	}

	// Parse destination.
	if m := destLineRe.FindStringSubmatch(text); m != nil {
		result.Destination = m[1]
	}

	// Parse total passengers.
	if m := totalPaxRe.FindStringSubmatch(text); m != nil {
		result.TotalPax, _ = strconv.Atoi(m[1])
		result.Infants, _ = strconv.Atoi(m[2])
	}

	// Parse joining passengers and bags.
	if m := joiningRe.FindStringSubmatch(text); m != nil {
		result.Adults, _ = strconv.Atoi(m[1])
		result.Male, _ = strconv.Atoi(m[2])
		result.Female, _ = strconv.Atoi(m[3])
		result.Children, _ = strconv.Atoi(m[4])
		result.Infants, _ = strconv.Atoi(m[5])
		result.BagCount, _ = strconv.Atoi(m[6])
		result.BagWeight, _ = strconv.Atoi(m[7])
	}

	// Parse zone breakdown.
	zoneMatches := zoneRe.FindAllStringSubmatch(text, -1)
	for _, m := range zoneMatches {
		zone := ZoneCount{
			Zone: m[1],
		}
		zone.Adults, _ = strconv.Atoi(m[2])
		zone.Male, _ = strconv.Atoi(m[3])
		zone.Female, _ = strconv.Atoi(m[4])
		zone.Children, _ = strconv.Atoi(m[5])
		zone.Infants, _ = strconv.Atoi(m[6])
		result.Zones = append(result.Zones, zone)
	}

	// Must have some data.
	if result.FlightNumber == "" && result.TotalPax == 0 {
		return nil
	}

	return result
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	quickCheckPassed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{
		Passed: quickCheckPassed,
	}

	if !quickCheckPassed {
		trace.QuickCheck.Reason = "No PAX AND BAG DETAILS keyword found"
		return trace
	}

	text := msg.Text

	// Add extractors for key patterns.
	extractors := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"flight_info", flightInfoRe},
		{"flight_line", flightLineRe},
		{"total_pax", totalPaxRe},
		{"joining", joiningRe},
		{"zone", zoneRe},
		{"dest_line", destLineRe},
	}

	for _, e := range extractors {
		ext := registry.Extractor{
			Name:    e.name,
			Pattern: e.pattern.String(),
		}
		if m := e.pattern.FindStringSubmatch(text); len(m) > 1 {
			ext.Matched = true
			ext.Value = m[1]
		}
		trace.Extractors = append(trace.Extractors, ext)
	}

	// Determine if overall match succeeds.
	hasFlightNum := flightLineRe.MatchString(text)
	hasTotalPax := totalPaxRe.MatchString(text)
	trace.Matched = hasFlightNum || hasTotalPax

	return trace
}