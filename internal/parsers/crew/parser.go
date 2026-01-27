// Package crew parses crew list messages.
package crew

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// CrewMember represents a single crew member.
type CrewMember struct {
	Position   string `json:"position"`            // CA, FO, FA, FM, etc.
	Name       string `json:"name"`
	EmployeeID string `json:"employee_id,omitempty"`
}

// Result represents a parsed crew list.
type Result struct {
	MsgID        int64        `json:"message_id,omitempty"`
	FlightNumber string       `json:"flight_number,omitempty"`
	FlightDate   string       `json:"flight_date,omitempty"`
	Origin       string       `json:"origin,omitempty"`
	Destination  string       `json:"destination,omitempty"`
	SentTime     string       `json:"sent_time,omitempty"`
	GateETA      string       `json:"gate_eta,omitempty"`
	CockpitCrew  []CrewMember `json:"cockpit_crew,omitempty"`
	CabinCrew    []CrewMember `json:"cabin_crew,omitempty"`
	MinCrew      int          `json:"min_crew,omitempty"`
}

func (r *Result) Type() string     { return "crew_list" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// UA475/10 CYEG KDEN
	flightRouteRe = regexp.MustCompile(`([A-Z]{2}\d+)/(\d+)\s+([A-Z]{4})\s+([A-Z]{4})`)

	// SENT:  21:47:04z
	sentTimeRe = regexp.MustCompile(`SENT:\s*(\d{2}:\d{2}:\d{2}z?)`)

	// GATE ETA 0054
	gateETARe = regexp.MustCompile(`GATE ETA\s*(\d{4})`)

	// 1.CA CLARKE   DOMINIC or CA CLARKE DOMINIC
	cockpitCrewRe = regexp.MustCompile(`(?:\d+\.)?(CA|FO|SO|FE)\s+([A-Z]+)\s+([A-Z ]+?)\s*\n\s*([A-Z]?\d+)`)

	// FA HAY        DUSTIN or FM FERRERI    CAROLINE E
	cabinCrewRe = regexp.MustCompile(`(FA|FM|FP|FS)\s+([A-Z]+)\s+([A-Z ]+?)\s*\n\s*([A-Z]?\d+)`)

	// FLIGHT ATTENDANT MIN:4
	minCrewRe = regexp.MustCompile(`(?:FLIGHT ATTENDANT|FA)\s*MIN[:\s]*(\d+)`)
)

// Parser parses crew list messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string           { return "crew_list" }
func (p *Parser) Labels() []string       { return []string{"RA"} }
func (p *Parser) Priority() int          { return 55 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "CREW LIST")
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
		MsgID: int64(msg.ID),
	}

	// Parse flight/route.
	if m := flightRouteRe.FindStringSubmatch(text); m != nil {
		result.FlightNumber = m[1]
		result.FlightDate = m[2]
		result.Origin = m[3]
		result.Destination = m[4]
	}

	// Parse sent time.
	if m := sentTimeRe.FindStringSubmatch(text); m != nil {
		result.SentTime = m[1]
	}

	// Parse gate ETA.
	if m := gateETARe.FindStringSubmatch(text); m != nil {
		result.GateETA = m[1]
	}

	// Parse cockpit crew.
	cockpitMatches := cockpitCrewRe.FindAllStringSubmatch(text, -1)
	for _, m := range cockpitMatches {
		crew := CrewMember{
			Position:   m[1],
			Name:       strings.TrimSpace(m[2] + " " + strings.TrimSpace(m[3])),
			EmployeeID: m[4],
		}
		result.CockpitCrew = append(result.CockpitCrew, crew)
	}

	// Parse cabin crew.
	cabinMatches := cabinCrewRe.FindAllStringSubmatch(text, -1)
	for _, m := range cabinMatches {
		crew := CrewMember{
			Position:   m[1],
			Name:       strings.TrimSpace(m[2] + " " + strings.TrimSpace(m[3])),
			EmployeeID: m[4],
		}
		result.CabinCrew = append(result.CabinCrew, crew)
	}

	// Parse minimum crew.
	if m := minCrewRe.FindStringSubmatch(text); m != nil {
		result.MinCrew, _ = strconv.Atoi(m[1])
	}

	// Must have some crew data.
	if len(result.CockpitCrew) == 0 && len(result.CabinCrew) == 0 {
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
		trace.QuickCheck.Reason = "No CREW LIST keyword found"
		return trace
	}

	text := msg.Text

	// Add extractors for key patterns.
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "flight_route",
		Pattern: flightRouteRe.String(),
		Matched: flightRouteRe.MatchString(text),
		Value: func() string {
			if m := flightRouteRe.FindStringSubmatch(text); len(m) > 1 {
				return m[1]
			}
			return ""
		}(),
	})

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "sent_time",
		Pattern: sentTimeRe.String(),
		Matched: sentTimeRe.MatchString(text),
	})

	cockpitMatches := cockpitCrewRe.FindAllString(text, -1)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "cockpit_crew",
		Pattern: cockpitCrewRe.String(),
		Matched: len(cockpitMatches) > 0,
		Value:   strconv.Itoa(len(cockpitMatches)) + " found",
	})

	cabinMatches := cabinCrewRe.FindAllString(text, -1)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "cabin_crew",
		Pattern: cabinCrewRe.String(),
		Matched: len(cabinMatches) > 0,
		Value:   strconv.Itoa(len(cabinMatches)) + " found",
	})

	trace.Matched = len(cockpitMatches) > 0 || len(cabinMatches) > 0
	return trace
}