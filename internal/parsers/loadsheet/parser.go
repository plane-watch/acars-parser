// Package loadsheet parses aircraft loadsheet messages from ACARS.
// These contain weight and balance data for flight operations.
// Uses grok-style pattern matching for validated parsing.
package loadsheet

import (
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// Result represents parsed loadsheet data.
type Result struct {
	MsgID        int64  `json:"message_id"`
	FormatName   string `json:"format_name,omitempty"`   // Which grok format matched.
	Status       string `json:"status,omitempty"`        // FINAL or PRELIM.
	Timestamp    string `json:"timestamp"`
	Tail         string `json:"tail,omitempty"`
	Flight       string `json:"flight,omitempty"`
	Origin       string `json:"origin,omitempty"`
	Destination  string `json:"destination,omitempty"`
	AircraftType string `json:"aircraft_type,omitempty"`
	ZFW          int    `json:"zfw,omitempty"`           // Zero Fuel Weight (kg).
	ZFWMax       int    `json:"zfw_max,omitempty"`       // Maximum ZFW (kg).
	TOW          int    `json:"tow,omitempty"`           // Take Off Weight (kg).
	TOWMax       int    `json:"tow_max,omitempty"`       // Maximum TOW (kg).
	LAW          int    `json:"law,omitempty"`           // Landing Weight (kg).
	LAWMax       int    `json:"law_max,omitempty"`       // Maximum LAW (kg).
	TOF          int    `json:"tof,omitempty"`           // Take Off Fuel (kg).
	TIF          int    `json:"tif,omitempty"`           // Trip Fuel (kg).
	PAX          int    `json:"pax,omitempty"`           // Passenger count.
	Crew         string `json:"crew,omitempty"`          // Crew configuration (e.g., "2/4").
	MACZFW       string `json:"mac_zfw,omitempty"`       // MAC at ZFW.
	MACTOW       string `json:"mac_tow,omitempty"`       // MAC at TOW.
	Edition      string `json:"edition,omitempty"`       // Loadsheet edition number.
}

func (r *Result) Type() string     { return "loadsheet" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses loadsheet messages using grok-style pattern matching.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string { return "loadsheet" }

// Labels returns all labels that may contain loadsheets.
// This is the union of all labels from the grok formats.
func (p *Parser) Labels() []string {
	// Collect unique labels from all formats.
	labelSet := make(map[string]bool)
	for _, format := range LoadsheetFormats {
		for _, label := range format.Labels {
			labelSet[label] = true
		}
	}

	labels := make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}
	return labels
}

func (p *Parser) Priority() int { return 60 } // Higher priority than weather.

// QuickCheck looks for loadsheet keywords.
func (p *Parser) QuickCheck(text string) bool {
	upper := strings.ToUpper(text)
	return strings.Contains(upper, "LOADSHEET")
}

// Parse extracts loadsheet data using validated grok patterns.
func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	// Try to match against known formats.
	format, fields := MatchFormat(msg.Text, msg.Label)
	if format == nil {
		return nil // No validated format matched.
	}

	result := &Result{
		MsgID:      int64(msg.ID),
		FormatName: format.Name,
		Timestamp:  msg.Timestamp,
		Tail:       msg.Tail,
	}

	// Extract common fields.
	if v, ok := fields["status"]; ok {
		result.Status = v
	}
	if v, ok := fields["flight"]; ok {
		// Normalise flight number (remove spaces).
		result.Flight = strings.ReplaceAll(v, " ", "")
	}
	if v, ok := fields["tail"]; ok {
		result.Tail = v
	}
	if v, ok := fields["origin"]; ok {
		result.Origin = v
	}
	if v, ok := fields["destination"]; ok {
		result.Destination = v
	}
	if v, ok := fields["crew"]; ok {
		result.Crew = v
	}
	if v, ok := fields["edition"]; ok {
		result.Edition = v
	}

	// Extract weights - handle both kg and tonnes.
	result.ZFW = parseWeight(fields["zfw"], format.WeightUnit)
	result.ZFWMax = parseWeight(fields["zfw_max"], format.WeightUnit)
	result.TOW = parseWeight(fields["tow"], format.WeightUnit)
	result.TOWMax = parseWeight(fields["tow_max"], format.WeightUnit)
	result.LAW = parseWeight(fields["law"], format.WeightUnit)
	result.LAWMax = parseWeight(fields["law_max"], format.WeightUnit)
	result.TOF = parseWeight(fields["tof"], format.WeightUnit)
	result.TIF = parseWeight(fields["tif"], format.WeightUnit)

	// Extract passenger count.
	if v, ok := fields["pax_total"]; ok {
		result.PAX, _ = strconv.Atoi(v)
	}

	// Extract MAC values (these are percentages, not weights).
	if v, ok := fields["mac_zfw"]; ok {
		result.MACZFW = v
	}
	if v, ok := fields["mac_tow"]; ok {
		result.MACTOW = v
	}

	// Validate that we have essential data.
	if result.ZFW == 0 && result.TOW == 0 {
		return nil // No weight data extracted.
	}

	return result
}

// parseWeight parses a weight value, converting from tonnes or pounds to kg if needed.
func parseWeight(value, unit string) int {
	if value == "" {
		return 0
	}

	// Remove any thousands separators.
	value = strings.ReplaceAll(value, ",", "")

	switch unit {
	case "tonnes":
		// Value is in tonnes (e.g., "150.6" = 150,600 kg).
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0
		}
		return int(f * 1000)
	case "lb":
		// Value is in pounds - convert to kg (1 lb = 0.453592 kg).
		v, _ := strconv.Atoi(value)
		return int(float64(v) * 0.453592)
	default:
		// Value is in kg.
		v, _ := strconv.Atoi(value)
		return v
	}
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
		trace.QuickCheck.Reason = "No LOADSHEET keyword found"
		return trace
	}

	// Try to match against known formats.
	format, fields := MatchFormat(msg.Text, msg.Label)

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "format_match",
		Pattern: "loadsheet format patterns",
		Matched: format != nil,
		Value: func() string {
			if format != nil {
				return format.Name
			}
			return ""
		}(),
	})

	if format != nil && fields != nil {
		// Report which fields were extracted.
		for key, value := range fields {
			if value != "" {
				trace.Extractors = append(trace.Extractors, registry.Extractor{
					Name:    key,
					Pattern: "format field: " + key,
					Matched: true,
					Value:   value,
				})
			}
		}
	}

	trace.Matched = format != nil
	return trace
}
