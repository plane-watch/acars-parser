// Package fuel parses fuel delivery receipt messages.
package fuel

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// Result represents a parsed fuel delivery receipt.
type Result struct {
	MsgID           int64   `json:"message_id,omitempty"`
	FlightNumber    string  `json:"flight_number"`
	Tail            string  `json:"tail"`
	Date            string  `json:"date"`
	Destination     string  `json:"destination,omitempty"`
	FuelCompany     string  `json:"fuel_company,omitempty"`
	FuelGrade       string  `json:"fuel_grade,omitempty"`
	TruckID         string  `json:"truck_id,omitempty"`
	StartTime       string  `json:"start_time,omitempty"`
	EndTime         string  `json:"end_time,omitempty"`
	AmountLitres    int     `json:"amount_litres,omitempty"`
	DensityKgM3     int     `json:"density_kg_m3,omitempty"`
	QtyBeforeKg     int     `json:"qty_before_kg,omitempty"`
}

func (r *Result) Type() string     { return "fuel_delivery" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// AY571 OHLVL 22.01.2026
	flightLineRe = regexp.MustCompile(`([A-Z]{2}\d+)\s+([A-Z0-9-]+)\s+(\d{2}\.\d{2}\.\d{4})`)
	destRe       = regexp.MustCompile(`DEST:\s*([A-Z]{3,4})`)
	companyRe    = regexp.MustCompile(`FUEL COMPANY:\s*([A-Z0-9]+)`)
	gradeRe      = regexp.MustCompile(`FUEL GRADE:\s*([A-Z0-9]+)`)
	startRe      = regexp.MustCompile(`START:\s*(\d{2}:\d{2})`)
	endRe        = regexp.MustCompile(`END:\s*(\d{2}:\d{2})`)
	amountRe     = regexp.MustCompile(`AMOUNT:\s*(\d+)\s*LTRS`)
	densityRe    = regexp.MustCompile(`DENSITY:\s*(\d+)\s*KG/M3`)
	qtyBeforeRe  = regexp.MustCompile(`QTY BEFORE[^:]*:\s*(\d+)\s*KG`)
	truckRe      = regexp.MustCompile(`TRUCK:\s*([A-Z0-9]+)`)
)

// Parser parses fuel delivery receipt messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string           { return "fuel_delivery" }
func (p *Parser) Labels() []string       { return []string{"3E", "RA"} }
func (p *Parser) Priority() int          { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, "FUEL DELIVERY")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := msg.Text

	if !strings.Contains(text, "FUEL DELIVERY") {
		return nil
	}

	result := &Result{
		MsgID: int64(msg.ID),
	}

	// Parse flight line: AY571 OHLVL 22.01.2026
	if m := flightLineRe.FindStringSubmatch(text); m != nil {
		result.FlightNumber = m[1]
		result.Tail = m[2]
		result.Date = m[3]
	}

	if m := destRe.FindStringSubmatch(text); m != nil {
		result.Destination = m[1]
	}
	if m := companyRe.FindStringSubmatch(text); m != nil {
		result.FuelCompany = m[1]
	}
	if m := gradeRe.FindStringSubmatch(text); m != nil {
		result.FuelGrade = m[1]
	}
	if m := startRe.FindStringSubmatch(text); m != nil {
		result.StartTime = m[1]
	}
	if m := endRe.FindStringSubmatch(text); m != nil {
		result.EndTime = m[1]
	}
	if m := amountRe.FindStringSubmatch(text); m != nil {
		result.AmountLitres, _ = strconv.Atoi(m[1])
	}
	if m := densityRe.FindStringSubmatch(text); m != nil {
		result.DensityKgM3, _ = strconv.Atoi(m[1])
	}
	if m := qtyBeforeRe.FindStringSubmatch(text); m != nil {
		result.QtyBeforeKg, _ = strconv.Atoi(m[1])
	}
	if m := truckRe.FindStringSubmatch(text); m != nil {
		result.TruckID = m[1]
	}

	// Must have at least flight info to be valid.
	if result.FlightNumber == "" {
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
		trace.QuickCheck.Reason = "No FUEL DELIVERY keyword found"
		return trace
	}

	text := msg.Text

	// Add extractors for key patterns.
	extractors := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"flight_line", flightLineRe},
		{"dest", destRe},
		{"company", companyRe},
		{"grade", gradeRe},
		{"start", startRe},
		{"end", endRe},
		{"amount", amountRe},
		{"density", densityRe},
		{"qty_before", qtyBeforeRe},
		{"truck", truckRe},
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

	trace.Matched = flightLineRe.MatchString(text)
	return trace
}