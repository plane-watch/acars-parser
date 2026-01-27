// Package weather parses METAR and TAF weather messages from ACARS.
// Handles labels RA and C1 which often contain weather data.
package weather

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// MetarReport represents a single parsed METAR.
type MetarReport struct {
	Airport     string `json:"airport"`
	Time        string `json:"time,omitempty"`
	Wind        string `json:"wind,omitempty"`
	WindDir     int    `json:"wind_dir,omitempty"`
	WindSpeed   int    `json:"wind_speed,omitempty"`
	WindGust    int    `json:"wind_gust,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	Weather     string `json:"weather,omitempty"`
	Clouds      string `json:"clouds,omitempty"`
	Temperature int    `json:"temperature,omitempty"`
	DewPoint    int    `json:"dew_point,omitempty"`
	QNH         int    `json:"qnh,omitempty"`
	Raw         string `json:"raw"`
}

// TafReport represents a single parsed TAF.
type TafReport struct {
	Airport string `json:"airport"`
	Issued  string `json:"issued,omitempty"`
	Valid   string `json:"valid,omitempty"`
	Raw     string `json:"raw"`
}

// SigmetReport represents a parsed SIGMET.
type SigmetReport struct {
	ID         string `json:"id"`
	ValidFrom  string `json:"valid_from,omitempty"`
	ValidTo    string `json:"valid_to,omitempty"`
	Originator string `json:"originator,omitempty"`
	FIR        string `json:"fir,omitempty"`
	Phenomenon string `json:"phenomenon,omitempty"`
	Altitude   string `json:"altitude,omitempty"`
	Movement   string `json:"movement,omitempty"`
	Raw        string `json:"raw"`
}

// Result represents parsed weather data.
type Result struct {
	MsgID     int64          `json:"message_id"`
	Timestamp string         `json:"timestamp"`
	Tail      string         `json:"tail,omitempty"`
	Metars    []MetarReport  `json:"metars,omitempty"`
	Tafs      []TafReport    `json:"tafs,omitempty"`
	Sigmets   []SigmetReport `json:"sigmets,omitempty"`
}

func (r *Result) Type() string     { return "weather" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses weather messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "weather" }
func (p *Parser) Labels() []string { return []string{"RA", "C1", "21", "H1", "3W", "27", "31", "34", "3T", "23"} }
func (p *Parser) Priority() int    { return 50 } // Lower priority, run after more specific parsers.

// QuickCheck looks for weather keywords.
func (p *Parser) QuickCheck(text string) bool {
	upper := strings.ToUpper(text)
	return strings.Contains(upper, "METAR") ||
		strings.Contains(upper, " TAF ") ||
		strings.Contains(upper, "SIGMET")
}

// METAR pattern components.
var (
	// Match a METAR line: METAR [COR] ICAO DDHHMMZ ...
	metarRe = regexp.MustCompile(`(?m)^(?:METAR\s+)?(?:COR\s+)?([A-Z]{4})\s+(\d{6}Z)\s+(.+?)(?:\s*=|$)`)

	// Wind pattern: DDDSPKT or DDDSPGGGKT or VRBSPKT or DDDSPMPSORKT
	windRe = regexp.MustCompile(`\b(\d{3}|VRB)(\d{2,3})(?:G(\d{2,3}))?(KT|MPS)\b`)

	// Visibility: 9999 or ####SM or ####
	visRe = regexp.MustCompile(`\b(\d{4})\b|\b(\d+)SM\b`)

	// Temperature/dewpoint: TT/DD or MTT/MDD (M = minus)
	tempRe = regexp.MustCompile(`\b(M?\d{2})/(M?\d{2})\b`)

	// QNH: Q#### or A####
	qnhRe = regexp.MustCompile(`\b([QA])(\d{4})\b`)

	// Clouds: FEW/SCT/BKN/OVC followed by height, or CAVOK/NCD/NSC.
	cloudRe = regexp.MustCompile(`\b(FEW|SCT|BKN|OVC)(\d{3})(?:CB|TCU)?\b|\b(CAVOK|NCD|NSC|SKC|CLR)\b`)

	// TAF pattern.
	tafRe = regexp.MustCompile(`(?m)TAF\s+(?:AMD\s+)?(?:COR\s+)?([A-Z]{4})\s+(\d{6}Z)\s+(\d{4}/\d{4})\s+(.+?)(?:\s*=|$)`)

	// SIGMET pattern.
	// Example: SIGMET 7 VALID 040330/040730 SBAO- SBAO ATLANTICO FIR SEV TURB FCST WI ... FL300/380 STNR NC=
	// Example: SIGMET A01 VALID 040235/040635 VCBI- VCCF COLOMBO FIR EMBD TS OBS WI ... TOP FL600 STNR NC=
	sigmetRe = regexp.MustCompile(`SIGMET\s+(\w+)\s+VALID\s+(\d{6})/(\d{6})\s+([A-Z]{4})-\s+([A-Z]{4})\s+(\w+(?:\s+\w+)?)\s+FIR\s+(.+?)(?:\s*=|$)`)

	// SIGMET altitude patterns.
	sigmetAltRe = regexp.MustCompile(`(?:FL(\d{3})/(\d{3})|TOP\s+FL(\d{3})|SFC/FL(\d{3}))`)

	// SIGMET movement patterns.
	sigmetMvtRe = regexp.MustCompile(`\b(STNR|MOV\s+[NESW]+\s+\d+KT)\b`)
)

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
		Tail:      msg.Tail,
	}

	text := msg.Text

	// Extract METARs.
	result.Metars = extractMetars(text)

	// Extract TAFs.
	result.Tafs = extractTafs(text)

	// Extract SIGMETs.
	result.Sigmets = extractSigmets(text)

	// Only return if we found something.
	if len(result.Metars) == 0 && len(result.Tafs) == 0 && len(result.Sigmets) == 0 {
		return nil
	}

	return result
}

// extractMetars parses all METAR reports from the text.
func extractMetars(text string) []MetarReport {
	var metars []MetarReport

	// Find all METAR matches.
	matches := metarRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}

		metar := MetarReport{
			Airport: m[1],
			Time:    m[2],
			Raw:     strings.TrimSpace(m[0]),
		}

		body := m[3]

		// Parse wind.
		if wm := windRe.FindStringSubmatch(body); len(wm) > 0 {
			metar.Wind = wm[0]
			if wm[1] != "VRB" {
				metar.WindDir, _ = strconv.Atoi(wm[1])
			}
			metar.WindSpeed, _ = strconv.Atoi(wm[2])
			if wm[3] != "" {
				metar.WindGust, _ = strconv.Atoi(wm[3])
			}
			// Convert MPS to KT if needed (roughly *2).
			if wm[4] == "MPS" {
				metar.WindSpeed = int(float64(metar.WindSpeed) * 1.944)
				if metar.WindGust > 0 {
					metar.WindGust = int(float64(metar.WindGust) * 1.944)
				}
			}
		}

		// Parse visibility.
		if vm := visRe.FindStringSubmatch(body); len(vm) > 0 {
			if vm[1] != "" {
				metar.Visibility = vm[1]
			} else if vm[2] != "" {
				metar.Visibility = vm[2] + "SM"
			}
		}

		// Parse temperature/dewpoint.
		if tm := tempRe.FindStringSubmatch(body); len(tm) > 0 {
			metar.Temperature = parseTemp(tm[1])
			metar.DewPoint = parseTemp(tm[2])
		}

		// Parse QNH.
		if qm := qnhRe.FindStringSubmatch(body); len(qm) > 0 {
			metar.QNH, _ = strconv.Atoi(qm[2])
			// Convert inHg to hPa if A prefix (US format).
			if qm[1] == "A" {
				// A2992 = 29.92 inHg = ~1013 hPa
				metar.QNH = int(float64(metar.QNH) * 0.338639)
			}
		}

		// Extract cloud info.
		clouds := cloudRe.FindAllString(body, -1)
		if len(clouds) > 0 {
			metar.Clouds = strings.Join(clouds, " ")
		}

		metars = append(metars, metar)
	}

	return metars
}

// extractTafs parses all TAF reports from the text.
func extractTafs(text string) []TafReport {
	var tafs []TafReport

	matches := tafRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) < 5 {
			continue
		}

		taf := TafReport{
			Airport: m[1],
			Issued:  m[2],
			Valid:   m[3],
			Raw:     strings.TrimSpace(m[0]),
		}

		tafs = append(tafs, taf)
	}

	return tafs
}

// parseTemp converts temperature string (e.g., "M05" or "12") to int.
func parseTemp(s string) int {
	if s == "" {
		return 0
	}
	neg := strings.HasPrefix(s, "M")
	s = strings.TrimPrefix(s, "M")
	val, _ := strconv.Atoi(s)
	if neg {
		val = -val
	}
	return val
}

// extractSigmets parses all SIGMET reports from the text.
func extractSigmets(text string) []SigmetReport {
	var sigmets []SigmetReport

	matches := sigmetRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) < 8 {
			continue
		}

		sigmet := SigmetReport{
			ID:         m[1],
			ValidFrom:  m[2],
			ValidTo:    m[3],
			Originator: m[4],
			FIR:        m[5] + " " + m[6],
			Raw:        strings.TrimSpace(m[0]),
		}

		body := m[7]

		// Extract phenomenon (first few words before WI or OBS or FCST).
		if idx := strings.Index(body, " WI "); idx > 0 {
			sigmet.Phenomenon = strings.TrimSpace(body[:idx])
		} else if idx := strings.Index(body, " OBS "); idx > 0 {
			sigmet.Phenomenon = strings.TrimSpace(body[:idx])
		} else if idx := strings.Index(body, " FCST "); idx > 0 {
			sigmet.Phenomenon = strings.TrimSpace(body[:idx])
		}

		// Extract altitude.
		if am := sigmetAltRe.FindStringSubmatch(body); len(am) > 0 {
			// Find which group matched.
			for i := 1; i < len(am); i++ {
				if am[i] != "" {
					sigmet.Altitude = am[0]
					break
				}
			}
		}

		// Extract movement.
		if mm := sigmetMvtRe.FindString(body); mm != "" {
			sigmet.Movement = mm
		}

		sigmets = append(sigmets, sigmet)
	}

	return sigmets
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
		trace.QuickCheck.Reason = "No METAR, TAF, or SIGMET keyword found"
		return trace
	}

	text := msg.Text

	// Add extractors for each weather type.
	metarMatches := metarRe.FindAllString(text, -1)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "metar",
		Pattern: metarRe.String(),
		Matched: len(metarMatches) > 0,
		Value:   strconv.Itoa(len(metarMatches)) + " found",
	})

	tafMatches := tafRe.FindAllString(text, -1)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "taf",
		Pattern: tafRe.String(),
		Matched: len(tafMatches) > 0,
		Value:   strconv.Itoa(len(tafMatches)) + " found",
	})

	sigmetMatches := sigmetRe.FindAllString(text, -1)
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "sigmet",
		Pattern: sigmetRe.String(),
		Matched: len(sigmetMatches) > 0,
		Value:   strconv.Itoa(len(sigmetMatches)) + " found",
	})

	trace.Matched = len(metarMatches) > 0 || len(tafMatches) > 0 || len(sigmetMatches) > 0

	return trace
}