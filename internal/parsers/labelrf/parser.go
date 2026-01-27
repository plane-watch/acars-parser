// Package labelrf parses Label RF flight subscription messages.
// These messages contain flight route data from the SITA FDA (Flight Data Application)
// system, commonly used by airlines for flight tracking and subscription services.
package labelrf

import (
	"fmt"
	"regexp"
	"strings"

	"acars_parser/internal/acars"
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

// Parser parses Label RF flight subscription messages.
type Parser struct{}

// msgTypeRe extracts the message type prefix (FDASUB, FDACOM, FSTREQ, FDAACK).
var msgTypeRe = regexp.MustCompile(`^(FD(?:ASUB|ACOM|AACK)|FSTREQ)`)

// flightNumRe validates flight numbers (2-3 letter airline code + 1-4 digits).
var flightNumRe = regexp.MustCompile(`^[A-Z]{2,3}\d{1,4}$`)

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "labelrf" }
func (p *Parser) Labels() []string { return []string{"RF"} }
func (p *Parser) Priority() int    { return 100 }

func (p *Parser) QuickCheck(text string) bool {
	// Only process messages that contain flight subscription data.
	// FDASUB = Flight Data Subscription
	// FDACOM = Flight Data Communication
	// FSTREQ = Flight Status Request
	return strings.Contains(text, "FDASUB") ||
		strings.Contains(text, "FDACOM") ||
		strings.Contains(text, "FSTREQ")
}

// Parse extracts flight subscription data from Label RF messages.
// Format: ,<msg_type><timestamp>,<origin>,,<dest>,,<flight>,<date>,<time>,<type>,,<reg>,/<data>
// Example: ,FDASUB01260107143821,SIN,,LHR,,QF001,260107,1535,388,,VH-OQI,/358301,349,,1,4EC0
func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)

	// Split on the first slash to separate header from tracking data.
	parts := strings.SplitN(text, "/", 2)
	header := parts[0]

	// Split the header by commas.
	fields := strings.Split(header, ",")
	if len(fields) < 12 {
		return nil // Not enough fields for flight data.
	}

	// First field is empty (leading comma), second is message type + timestamp.
	msgTypeField := fields[1]
	if msgTypeField == "" {
		return nil
	}

	// Extract the message type.
	msgMatch := msgTypeRe.FindStringSubmatch(msgTypeField)
	if len(msgMatch) < 2 {
		return nil
	}
	msgType := msgMatch[1]

	// Fields layout for flight subscription messages:
	// [0] = empty (leading comma)
	// [1] = msg_type + timestamp
	// [2] = origin IATA
	// [3] = empty
	// [4] = dest IATA
	// [5] = empty
	// [6] = flight number
	// [7] = date (YYMMDD)
	// [8] = time (HHMM)
	// [9] = aircraft type
	// [10] = empty
	// [11] = registration

	originIATA := strings.TrimSpace(fields[2])
	destIATA := strings.TrimSpace(fields[4])
	flightNum := strings.TrimSpace(fields[6])

	// Validate we have actual flight data (not just position updates).
	if originIATA == "" || destIATA == "" || flightNum == "" {
		return nil
	}

	// Validate flight number format.
	if !flightNumRe.MatchString(flightNum) {
		return nil
	}

	result := &Result{
		MsgID:      int64(msg.ID),
		Timestamp:  msg.Timestamp,
		MsgType:    msgType,
		OriginIATA: originIATA,
		DestIATA:   destIATA,
		FlightNum:  flightNum,
	}

	// Convert IATA to ICAO if possible.
	if icao, ok := iataToICAO[originIATA]; ok {
		result.OriginICAO = icao
	}
	if icao, ok := iataToICAO[destIATA]; ok {
		result.DestICAO = icao
	}

	// Extract date if available.
	if len(fields) > 7 {
		result.Date = strings.TrimSpace(fields[7])
	}

	// Extract time if available.
	if len(fields) > 8 {
		result.Time = strings.TrimSpace(fields[8])
	}

	// Extract aircraft type if available.
	if len(fields) > 9 {
		result.AircraftType = strings.TrimSpace(fields[9])
	}

	// Extract registration if available.
	if len(fields) > 11 {
		result.Registration = strings.TrimSpace(fields[11])
	}

	return result
}

// iataToICAO maps common IATA airport codes to their ICAO equivalents.
// This is a subset covering major airports seen in ACARS traffic.
var iataToICAO = map[string]string{
	// Australia & New Zealand.
	"SYD": "YSSY", // Sydney
	"MEL": "YMML", // Melbourne
	"BNE": "YBBN", // Brisbane
	"PER": "YPPH", // Perth
	"ADL": "YPAD", // Adelaide
	"CBR": "YSCB", // Canberra
	"HBA": "YMHB", // Hobart
	"AKL": "NZAA", // Auckland
	"WLG": "NZWN", // Wellington
	"CHC": "NZCH", // Christchurch

	// Asia - Major Hubs.
	"SIN": "WSSS", // Singapore Changi
	"HKG": "VHHH", // Hong Kong
	"NRT": "RJAA", // Tokyo Narita
	"HND": "RJTT", // Tokyo Haneda
	"ICN": "RKSI", // Seoul Incheon
	"PEK": "ZBAA", // Beijing Capital
	"PKX": "ZBAD", // Beijing Daxing
	"PVG": "ZSPD", // Shanghai Pudong
	"SHA": "ZSSS", // Shanghai Hongqiao
	"CAN": "ZGGG", // Guangzhou
	"TPE": "RCTP", // Taipei Taoyuan
	"KUL": "WMKK", // Kuala Lumpur
	"BKK": "VTBS", // Bangkok Suvarnabhumi
	"DEL": "VIDP", // Delhi
	"BOM": "VABB", // Mumbai
	"BLR": "VOBL", // Bangalore
	"MNL": "RPLL", // Manila
	"CGK": "WIII", // Jakarta
	"DPS": "WADD", // Bali Denpasar
	"CTS": "RJCC", // Sapporo New Chitose

	// Europe - Major Hubs.
	"LHR": "EGLL", // London Heathrow
	"LGW": "EGKK", // London Gatwick
	"CDG": "LFPG", // Paris Charles de Gaulle
	"FRA": "EDDF", // Frankfurt
	"AMS": "EHAM", // Amsterdam
	"FCO": "LIRF", // Rome Fiumicino
	"MAD": "LEMD", // Madrid
	"BCN": "LEBL", // Barcelona
	"MUC": "EDDM", // Munich
	"ZRH": "LSZH", // Zurich
	"VIE": "LOWW", // Vienna
	"IST": "LTFM", // Istanbul

	// North America - Major Hubs.
	"LAX": "KLAX", // Los Angeles
	"SFO": "KSFO", // San Francisco
	"JFK": "KJFK", // New York JFK
	"EWR": "KEWR", // Newark
	"ORD": "KORD", // Chicago O'Hare
	"DFW": "KDFW", // Dallas Fort Worth
	"ATL": "KATL", // Atlanta
	"MIA": "KMIA", // Miami
	"SEA": "KSEA", // Seattle
	"YVR": "CYVR", // Vancouver
	"YYZ": "CYYZ", // Toronto
	"YUL": "CYUL", // Montreal

	// Middle East.
	"DXB": "OMDB", // Dubai
	"AUH": "OMAA", // Abu Dhabi
	"DOH": "OTHH", // Doha
	"RUH": "OERK", // Riyadh
	"JED": "OEJN", // Jeddah

	// South America.
	"GRU": "SBGR", // Sao Paulo Guarulhos
	"EZE": "SAEZ", // Buenos Aires Ezeiza
	"SCL": "SCEL", // Santiago

	// Africa.
	"JNB": "FAOR", // Johannesburg
	"CPT": "FACT", // Cape Town
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

	text := strings.TrimSpace(msg.Text)

	// Add extractors for each parsing step.
	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "msg_type",
		Pattern: msgTypeRe.String(),
		Matched: msgTypeRe.MatchString(text),
		Value:   func() string {
			if m := msgTypeRe.FindStringSubmatch(text); len(m) > 1 {
				return m[1]
			}
			return ""
		}(),
	})

	// Parse fields.
	parts := strings.SplitN(text, "/", 2)
	fields := strings.Split(parts[0], ",")
	hasEnoughFields := len(fields) >= 12

	trace.Extractors = append(trace.Extractors, registry.Extractor{
		Name:    "field_count",
		Pattern: "at least 12 comma-separated fields",
		Matched: hasEnoughFields,
		Value:   fmt.Sprintf("%d fields", len(fields)),
	})

	if hasEnoughFields {
		origin := strings.TrimSpace(fields[2])
		dest := strings.TrimSpace(fields[4])
		flight := strings.TrimSpace(fields[6])

		trace.Extractors = append(trace.Extractors, registry.Extractor{
			Name:    "origin",
			Pattern: "field[2]: 3-letter IATA code",
			Matched: origin != "",
			Value:   origin,
		})

		trace.Extractors = append(trace.Extractors, registry.Extractor{
			Name:    "destination",
			Pattern: "field[4]: 3-letter IATA code",
			Matched: dest != "",
			Value:   dest,
		})

		trace.Extractors = append(trace.Extractors, registry.Extractor{
			Name:    "flight_num",
			Pattern: flightNumRe.String(),
			Matched: flightNumRe.MatchString(flight),
			Value:   flight,
		})

		trace.Matched = origin != "" && dest != "" && flightNumRe.MatchString(flight)
	}

	return trace
}
