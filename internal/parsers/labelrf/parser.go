// Package labelrf parses Label RF flight subscription messages.
// These messages contain flight route data from the SITA FDA (Flight Data Application)
// system, commonly used by airlines for flight tracking and subscription services.
package labelrf

import (
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents parsed flight subscription data from Label RF messages.
type Result struct {
	MsgID        int64  `json:"message_id"`
	Timestamp    string `json:"timestamp"`
	MsgType      string `json:"msg_type,omitempty"`      // FDASUB, FDACOM, FSTREQ, FDAACK.
	OriginIATA   string `json:"origin_iata,omitempty"`   // 3-letter IATA code.
	OriginICAO   string `json:"origin_icao,omitempty"`   // 4-letter ICAO code (converted).
	DestIATA     string `json:"dest_iata,omitempty"`     // 3-letter IATA code.
	DestICAO     string `json:"dest_icao,omitempty"`     // 4-letter ICAO code (converted).
	FlightNum    string `json:"flight_num,omitempty"`    // Flight number (e.g., QF001).
	Date         string `json:"date,omitempty"`          // Flight date (YYMMDD).
	Time         string `json:"time,omitempty"`          // Scheduled time (HHMM).
	AircraftType string `json:"aircraft_type,omitempty"` // Aircraft type code (e.g., 388 = A380-800).
	Registration string `json:"registration,omitempty"`  // Aircraft registration.
}

func (r *Result) Type() string     { return "flight_subscription" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Grok compiler singleton.
var (
	grokCompiler *patterns.Compiler
	grokOnce     sync.Once
	grokErr      error
)

// getCompiler returns the singleton grok compiler.
func getCompiler() (*patterns.Compiler, error) {
	grokOnce.Do(func() {
		grokCompiler = patterns.NewCompiler(Formats, nil)
		grokErr = grokCompiler.Compile()
	})
	return grokCompiler, grokErr
}

// Parser parses Label RF flight subscription messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "labelrf" }
func (p *Parser) Labels() []string { return []string{"RF"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	// Only process messages that contain flight subscription data.
	return strings.Contains(text, "FDASUB") ||
		strings.Contains(text, "FDACOM") ||
		strings.Contains(text, "FSTREQ")
}

// Parse extracts flight subscription data from Label RF messages.
func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	compiler, err := getCompiler()
	if err != nil {
		return nil
	}

	match := compiler.Parse(msg.Text)
	if match == nil {
		return nil
	}

	// Validate we have actual flight data.
	origin := match.Captures["origin"]
	dest := match.Captures["dest"]
	flight := match.Captures["flight"]

	if origin == "" || dest == "" || flight == "" {
		return nil
	}

	result := &Result{
		MsgID:        int64(msg.ID),
		Timestamp:    msg.Timestamp,
		MsgType:      match.Captures["msg_type"],
		OriginIATA:   origin,
		DestIATA:     dest,
		FlightNum:    flight,
		Date:         match.Captures["date"],
		Time:         match.Captures["time"],
		AircraftType: match.Captures["actype"],
		Registration: match.Captures["reg"],
	}

	// Convert IATA to ICAO if possible.
	if icao, ok := iataToICAO[origin]; ok {
		result.OriginICAO = icao
	}
	if icao, ok := iataToICAO[dest]; ok {
		result.DestICAO = icao
	}

	return result
}

// iataToICAO maps common IATA airport codes to their ICAO equivalents.
var iataToICAO = map[string]string{
	// Australia & New Zealand.
	"SYD": "YSSY", "MEL": "YMML", "BNE": "YBBN", "PER": "YPPH", "ADL": "YPAD",
	"CBR": "YSCB", "HBA": "YMHB", "AKL": "NZAA", "WLG": "NZWN", "CHC": "NZCH",
	// Asia - Major Hubs.
	"SIN": "WSSS", "HKG": "VHHH", "NRT": "RJAA", "HND": "RJTT", "ICN": "RKSI",
	"PEK": "ZBAA", "PKX": "ZBAD", "PVG": "ZSPD", "SHA": "ZSSS", "CAN": "ZGGG",
	"TPE": "RCTP", "KUL": "WMKK", "BKK": "VTBS", "DEL": "VIDP", "BOM": "VABB",
	"BLR": "VOBL", "MNL": "RPLL", "CGK": "WIII", "DPS": "WADD", "CTS": "RJCC",
	// Europe - Major Hubs.
	"LHR": "EGLL", "LGW": "EGKK", "CDG": "LFPG", "FRA": "EDDF", "AMS": "EHAM",
	"FCO": "LIRF", "MAD": "LEMD", "BCN": "LEBL", "MUC": "EDDM", "ZRH": "LSZH",
	"VIE": "LOWW", "IST": "LTFM",
	// North America - Major Hubs.
	"LAX": "KLAX", "SFO": "KSFO", "JFK": "KJFK", "EWR": "KEWR", "ORD": "KORD",
	"DFW": "KDFW", "ATL": "KATL", "MIA": "KMIA", "SEA": "KSEA", "YVR": "CYVR",
	"YYZ": "CYYZ", "YUL": "CYUL",
	// Middle East.
	"DXB": "OMDB", "AUH": "OMAA", "DOH": "OTHH", "RUH": "OERK", "JED": "OEJN",
	// South America.
	"GRU": "SBGR", "EZE": "SAEZ", "SCL": "SCEL",
	// Africa.
	"JNB": "FAOR", "CPT": "FACT",
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
		trace.QuickCheck.Reason = "No FDASUB, FDACOM, or FSTREQ keyword found"
		return trace
	}

	// Get the compiler trace.
	compiler, err := getCompiler()
	if err != nil {
		trace.QuickCheck.Reason = "Failed to get compiler: " + err.Error()
		return trace
	}

	// Get detailed trace from compiler.
	compilerTrace := compiler.ParseWithTrace(msg.Text)

	// Convert format traces to generic format traces.
	for _, ft := range compilerTrace.Formats {
		trace.Formats = append(trace.Formats, registry.FormatTrace{
			Name:     ft.Name,
			Matched:  ft.Matched,
			Pattern:  ft.Pattern,
			Captures: ft.Captures,
		})
	}

	trace.Matched = compilerTrace.Match != nil
	return trace
}
