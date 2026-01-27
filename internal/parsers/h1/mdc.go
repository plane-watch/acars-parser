// Package h1 contains parsers for H1 label messages.
package h1

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// MDCResult represents a parsed MDC (Maintenance Data Computer) report.
type MDCResult struct {
	MsgID          int64              `json:"message_id,omitempty"`
	ReportType     string             `json:"report_type"`
	WriteOption    string             `json:"write_option,omitempty"`
	Filename       string             `json:"filename,omitempty"`
	Time           string             `json:"time,omitempty"`
	Date           string             `json:"date,omitempty"`
	ApplicationPN  string             `json:"application_pn,omitempty"`
	TablesPN       string             `json:"tables_pn,omitempty"`
	LegNumber      string             `json:"leg_number,omitempty"`
	EngineTrend    *EngineTrendData   `json:"engine_trend,omitempty"`
	Faults         []FaultEntry       `json:"faults,omitempty"`
}

// EngineTrendData contains engine performance parameters.
type EngineTrendData struct {
	LeftN1         float64 `json:"left_n1,omitempty"`          // % RPM
	RightN1        float64 `json:"right_n1,omitempty"`
	LeftN2         float64 `json:"left_n2,omitempty"`
	RightN2        float64 `json:"right_n2,omitempty"`
	LeftITT        int     `json:"left_itt,omitempty"`         // Inter-Turbine Temperature (C)
	RightITT       int     `json:"right_itt,omitempty"`
	LeftPS3        int     `json:"left_ps3,omitempty"`         // Compressor discharge pressure (PSI)
	RightPS3       int     `json:"right_ps3,omitempty"`
	LeftN1Vibes    float64 `json:"left_n1_vibes,omitempty"`    // Vibration (MIL)
	RightN1Vibes   float64 `json:"right_n1_vibes,omitempty"`
	LeftN2Vibes    float64 `json:"left_n2_vibes,omitempty"`
	RightN2Vibes   float64 `json:"right_n2_vibes,omitempty"`
	LeftOilTemp    int     `json:"left_oil_temp,omitempty"`    // Oil temperature (C)
	RightOilTemp   int     `json:"right_oil_temp,omitempty"`
	LeftOilPress   int     `json:"left_oil_press,omitempty"`   // Oil pressure (PSI)
	RightOilPress  int     `json:"right_oil_press,omitempty"`
	LeftPLA        float64 `json:"left_pla,omitempty"`         // Power Lever Angle (DEG)
	RightPLA       float64 `json:"right_pla,omitempty"`
	LeftFuelFlow   int     `json:"left_fuel_flow,omitempty"`   // Fuel flow (PPH)
	RightFuelFlow  int     `json:"right_fuel_flow,omitempty"`
	LeftVGPos      float64 `json:"left_vg_pos,omitempty"`      // Variable Geometry position (DEG)
	RightVGPos     float64 `json:"right_vg_pos,omitempty"`
	FADECControl   string  `json:"fadec_control,omitempty"`    // Which FADEC is in control
	Airspeed       float64 `json:"airspeed,omitempty"`         // Computed airspeed (KT)
	Altitude       int     `json:"altitude,omitempty"`         // Altitude (FT)
	TotalAirTemp   float64 `json:"total_air_temp,omitempty"`   // Total air temperature (C)
}

// FaultEntry represents a single fault from an MDC fault report.
type FaultEntry struct {
	ATA         string `json:"ata"`
	System      string `json:"system"`
	LRU         string `json:"lru,omitempty"`
	Status      string `json:"status,omitempty"`
	Message     string `json:"message,omitempty"`
	EquationID  string `json:"equation_id,omitempty"`
}

func (r *MDCResult) Type() string     { return "mdc" }
func (r *MDCResult) MessageID() int64 { return r.MsgID }

var (
	// Header patterns.
	mdcReportTypeRe   = regexp.MustCompile(`MDC REPORT:\s*([A-Z ]+?)\s*[\r\n]`)
	mdcWriteOptionRe  = regexp.MustCompile(`WRITE OPTION:\s*([^\r\n]+)`)
	mdcFilenameRe     = regexp.MustCompile(`FILENAME:\s*([A-Z0-9._]+)`)
	mdcTimeRe         = regexp.MustCompile(`TIME:\s*(\d{2}:\d{2})`)
	mdcDateRe         = regexp.MustCompile(`DATE:\s*(\d{2}[A-Za-z]{3}\d{4})`)
	mdcAppPNRe        = regexp.MustCompile(`MDC APPLICATION PN:\s*([0-9-]+)`)
	mdcTablesPNRe     = regexp.MustCompile(`MDC TABLES PN:\s*([0-9-]+)`)
	mdcLegRe          = regexp.MustCompile(`LEG:\s*(\d+)`)

	// Engine trend patterns (L/R prefix for left/right engine).
	engineParamRe = regexp.MustCompile(`([LR])\s+([A-Z0-9 ]+?)\s+([\d.-]+)\s*([A-Z%]+)`)

	// Fault patterns.
	faultATARe      = regexp.MustCompile(`ATA(\d{2}-\d{2})\s+([A-Z /-]+)`)
	faultEquationRe = regexp.MustCompile(`Equation ID:\s*([A-Z0-9-]+)`)
)

// MDCParser parses MDC (Maintenance Data Computer) reports.
type MDCParser struct{}

func init() {
	registry.Register(&MDCParser{})
}

// Name returns the parser's unique identifier.
func (p *MDCParser) Name() string { return "mdc" }

// Labels returns which ACARS labels this parser handles.
func (p *MDCParser) Labels() []string { return []string{"H1"} }

// Priority determines order when multiple parsers match. Lower than trajectory.
func (p *MDCParser) Priority() int { return 40 }

// QuickCheck performs a fast string check before expensive regex.
func (p *MDCParser) QuickCheck(text string) bool {
	return strings.Contains(text, "MDC REPORT:")
}

// Parse extracts MDC report data from the message.
func (p *MDCParser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := msg.Text

	// Must have MDC REPORT header.
	reportMatch := mdcReportTypeRe.FindStringSubmatch(text)
	if reportMatch == nil {
		return nil
	}

	result := &MDCResult{
		MsgID:      int64(msg.ID),
		ReportType: strings.TrimSpace(reportMatch[1]),
	}

	// Extract common header fields.
	if m := mdcWriteOptionRe.FindStringSubmatch(text); m != nil {
		result.WriteOption = strings.TrimSpace(m[1])
	}
	if m := mdcFilenameRe.FindStringSubmatch(text); m != nil {
		result.Filename = m[1]
	}
	if m := mdcTimeRe.FindStringSubmatch(text); m != nil {
		result.Time = m[1]
	}
	if m := mdcDateRe.FindStringSubmatch(text); m != nil {
		result.Date = m[1]
	}
	if m := mdcAppPNRe.FindStringSubmatch(text); m != nil {
		result.ApplicationPN = m[1]
	}
	if m := mdcTablesPNRe.FindStringSubmatch(text); m != nil {
		result.TablesPN = m[1]
	}
	if m := mdcLegRe.FindStringSubmatch(text); m != nil {
		result.LegNumber = m[1]
	}

	// Parse report-specific data.
	switch result.ReportType {
	case "ENGINE TREND":
		result.EngineTrend = p.parseEngineTrend(text)
	case "CURRENT FAULTS", "FAULT HISTORY":
		result.Faults = p.parseFaults(text)
	}

	return result
}

// parseEngineTrend extracts engine parameters from ENGINE TREND reports.
func (p *MDCParser) parseEngineTrend(text string) *EngineTrendData {
	data := &EngineTrendData{}
	hasData := false

	// Parse left/right engine parameters.
	matches := engineParamRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		side := m[1]       // L or R
		param := strings.TrimSpace(m[2])
		valueStr := m[3]
		// unit := m[4]    // Available if needed

		value, _ := strconv.ParseFloat(valueStr, 64)
		intValue := int(value)

		switch param {
		case "N1":
			if side == "L" {
				data.LeftN1 = value
			} else {
				data.RightN1 = value
			}
			hasData = true
		case "N2":
			if side == "L" {
				data.LeftN2 = value
			} else {
				data.RightN2 = value
			}
			hasData = true
		case "ITT":
			if side == "L" {
				data.LeftITT = intValue
			} else {
				data.RightITT = intValue
			}
			hasData = true
		case "PS3":
			if side == "L" {
				data.LeftPS3 = intValue
			} else {
				data.RightPS3 = intValue
			}
			hasData = true
		case "N1 VIBES":
			if side == "L" {
				data.LeftN1Vibes = value
			} else {
				data.RightN1Vibes = value
			}
			hasData = true
		case "N2 VIBES":
			if side == "L" {
				data.LeftN2Vibes = value
			} else {
				data.RightN2Vibes = value
			}
			hasData = true
		case "OIL TEMP":
			if side == "L" {
				data.LeftOilTemp = intValue
			} else {
				data.RightOilTemp = intValue
			}
			hasData = true
		case "OIL PRESSURE":
			if side == "L" {
				data.LeftOilPress = intValue
			} else {
				data.RightOilPress = intValue
			}
			hasData = true
		case "PLA":
			if side == "L" {
				data.LeftPLA = value
			} else {
				data.RightPLA = value
			}
			hasData = true
		case "FUEL FLOW":
			if side == "L" {
				data.LeftFuelFlow = intValue
			} else {
				data.RightFuelFlow = intValue
			}
			hasData = true
		case "VG POSITION":
			if side == "L" {
				data.LeftVGPos = value
			} else {
				data.RightVGPos = value
			}
			hasData = true
		}
	}

	// Parse non-sided parameters.
	if m := regexp.MustCompile(`FADEC IN CONTROL\s+([A-Z0-9 ]+)`).FindStringSubmatch(text); m != nil {
		data.FADECControl = strings.TrimSpace(m[1])
		hasData = true
	}
	if m := regexp.MustCompile(`COMPUTED AIRSPEED\s+([\d.]+)`).FindStringSubmatch(text); m != nil {
		data.Airspeed, _ = strconv.ParseFloat(m[1], 64)
		hasData = true
	}
	if m := regexp.MustCompile(`ALTITUDE\s+(\d+)`).FindStringSubmatch(text); m != nil {
		data.Altitude, _ = strconv.Atoi(m[1])
		hasData = true
	}
	if m := regexp.MustCompile(`TOTAL AIR TEMP\s+([+-]?[\d.]+)`).FindStringSubmatch(text); m != nil {
		data.TotalAirTemp, _ = strconv.ParseFloat(m[1], 64)
		hasData = true
	}

	if !hasData {
		return nil
	}
	return data
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *MDCParser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{
		ParserName: p.Name(),
	}

	quickCheckPassed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{
		Passed: quickCheckPassed,
	}

	if !quickCheckPassed {
		trace.QuickCheck.Reason = "No MDC REPORT: marker found"
		return trace
	}

	text := msg.Text

	// Add extractors for key patterns.
	extractors := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"report_type", mdcReportTypeRe},
		{"write_option", mdcWriteOptionRe},
		{"filename", mdcFilenameRe},
		{"time", mdcTimeRe},
		{"date", mdcDateRe},
		{"application_pn", mdcAppPNRe},
		{"tables_pn", mdcTablesPNRe},
		{"leg", mdcLegRe},
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

	// Check for engine parameters.
	engineMatches := engineParamRe.FindAllString(text, -1)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "engine_params",
		Pattern: engineParamRe.String(),
		Matched: len(engineMatches) > 0,
		Value:   strconv.Itoa(len(engineMatches)) + " found",
	})

	// Check for fault entries.
	faultMatches := faultATARe.FindAllString(text, -1)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "faults",
		Pattern: faultATARe.String(),
		Matched: len(faultMatches) > 0,
		Value:   strconv.Itoa(len(faultMatches)) + " found",
	})

	trace.Matched = mdcReportTypeRe.MatchString(text)
	return trace
}

// parseFaults extracts fault entries from CURRENT FAULTS or FAULT HISTORY reports.
func (p *MDCParser) parseFaults(text string) []FaultEntry {
	var faults []FaultEntry

	// Find all ATA entries.
	ataMatches := faultATARe.FindAllStringSubmatchIndex(text, -1)
	for i, match := range ataMatches {
		ata := text[match[2]:match[3]]
		system := strings.TrimSpace(text[match[4]:match[5]])

		// Extract the section between this ATA and the next (or end of text).
		sectionEnd := len(text)
		if i+1 < len(ataMatches) {
			sectionEnd = ataMatches[i+1][0]
		}
		section := text[match[0]:sectionEnd]

		fault := FaultEntry{
			ATA:    ata,
			System: system,
		}

		// Look for equation ID in this section.
		if m := faultEquationRe.FindStringSubmatch(section); m != nil {
			fault.EquationID = m[1]
		}

		faults = append(faults, fault)
	}

	return faults
}