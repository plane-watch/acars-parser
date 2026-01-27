// Package paxconn parses passenger connection status messages.
package paxconn

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// Connection represents a connecting flight and its passengers.
type Connection struct {
	FlightNumber string `json:"flight_number"`
	Date         string `json:"date,omitempty"`
	Time         string `json:"time,omitempty"`
	Destination  string `json:"destination,omitempty"`
	Gate         string `json:"gate,omitempty"`
	Decision     string `json:"decision"`              // MISSEDCONNECTION, PENDING, WILLWAIT
	Class        string `json:"class,omitempty"`
	Passengers   int    `json:"passengers"`
	Bags         int    `json:"bags"`
}

// Result represents a parsed passenger connection status.
type Result struct {
	MsgID             int64        `json:"message_id,omitempty"`
	CurrentFlight     string       `json:"current_flight,omitempty"`
	Connections       []Connection `json:"connections,omitempty"`
	MissedCount       int          `json:"missed_count"`
	PendingCount      int          `json:"pending_count"`
	WillWaitCount     int          `json:"will_wait_count"`
	TotalConnecting   int          `json:"total_connecting"`
}

func (r *Result) Type() string     { return "pax_conn_status" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// CURRENT FLIGHT: AY1434
	currentFlightRe = regexp.MustCompile(`CURRENT FLIGHT[:\s]*\n[^\n]*\n\s*([A-Z]{2}\d+)`)

	// AY1075 20260120 14:30 RIX
	connectionFlightRe = regexp.MustCompile(`([A-Z]{2}\d+)\s+(\d{8})\s+(\d{2}:\d{2})\s+([A-Z]{3})`)

	// DECISION          CLASS PAX BAGS
	// MISSEDCONNECTION  Y     1   0
	decisionRe = regexp.MustCompile(`(MISSEDCONNECTION|PENDING|WILLWAIT)\s+([A-Z])\s+(\d+)\s+(\d+)`)
)

// Parser parses passenger connection status messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string           { return "pax_conn_status" }
func (p *Parser) Labels() []string       { return []string{"3E", "RA"} }
func (p *Parser) Priority() int          { return 55 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "PAX CONN STATUS")
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

	// Parse current flight.
	if m := currentFlightRe.FindStringSubmatch(text); m != nil {
		result.CurrentFlight = m[1]
	}

	// Parse connections by finding flight info followed by decision lines.
	// Split by the dashed separators.
	sections := strings.Split(text, "--------------------------------")

	for _, section := range sections {
		// Find flight info in this section.
		flightMatch := connectionFlightRe.FindStringSubmatch(section)
		decisionMatch := decisionRe.FindStringSubmatch(section)

		if flightMatch != nil && decisionMatch != nil {
			pax, _ := strconv.Atoi(decisionMatch[3])
			bags, _ := strconv.Atoi(decisionMatch[4])

			conn := Connection{
				FlightNumber: flightMatch[1],
				Date:         flightMatch[2],
				Time:         flightMatch[3],
				Destination:  flightMatch[4],
				Decision:     decisionMatch[1],
				Class:        decisionMatch[2],
				Passengers:   pax,
				Bags:         bags,
			}
			result.Connections = append(result.Connections, conn)

			// Update counts.
			switch conn.Decision {
			case "MISSEDCONNECTION":
				result.MissedCount += pax
			case "PENDING":
				result.PendingCount += pax
			case "WILLWAIT":
				result.WillWaitCount += pax
			}
			result.TotalConnecting += pax
		}
	}

	// Must have at least one connection.
	if len(result.Connections) == 0 {
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
		trace.QuickCheck.Reason = "No PAX CONN STATUS keyword found"
		return trace
	}

	text := msg.Text

	// Add extractors for key patterns.
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "current_flight",
		Pattern: currentFlightRe.String(),
		Matched: currentFlightRe.MatchString(text),
		Value: func() string {
			if m := currentFlightRe.FindStringSubmatch(text); len(m) > 1 {
				return m[1]
			}
			return ""
		}(),
	})

	connFlightMatches := connectionFlightRe.FindAllString(text, -1)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "connection_flights",
		Pattern: connectionFlightRe.String(),
		Matched: len(connFlightMatches) > 0,
		Value:   strconv.Itoa(len(connFlightMatches)) + " found",
	})

	decisionMatches := decisionRe.FindAllString(text, -1)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "decisions",
		Pattern: decisionRe.String(),
		Matched: len(decisionMatches) > 0,
		Value:   strconv.Itoa(len(decisionMatches)) + " found",
	})

	trace.Matched = len(connFlightMatches) > 0 && len(decisionMatches) > 0
	return trace
}