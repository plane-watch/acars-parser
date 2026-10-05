package acmsreport

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is the header and CC block of an Airbus ACMS report.
type Result struct {
	MsgID     int64  `json:"message_id"`
	Timestamp string `json:"timestamp"`

	// AircraftSeries is the series as the report gives it, e.g. "A321". It
	// is not an ICAO type designator: the same value is sent by the ceo
	// and neo models (A321 and A21N), so it is deliberately not reported
	// as "aircraft_type".
	AircraftSeries string `json:"aircraft_series"`
	Report         string `json:"report"` // The report number, e.g. "019".

	// Registration is reported only when it is the transmitted tail.
	Registration string `json:"registration,omitempty"`
	ReportDate   string `json:"report_date,omitempty"` // MMMDD, e.g. JAN20; no year.
	ReportTime   string `json:"report_time,omitempty"` // HHMMSS.
	Origin       string `json:"origin"`
	Destination  string `json:"destination"`

	// FlightNumberDigits is the flight number without its airline code,
	// e.g. "0816". The extractor uses the route only for a transmitted
	// flight with this number. A flight number of 0000 is not reported.
	FlightNumberDigits string `json:"flight_number_digits,omitempty"`

	// Flight is the ICAO callsign from a C2 block, e.g. "UAL787", in
	// reports that do not give the flight number's digits.
	Flight string `json:"flight,omitempty"`

	// Latitude and Longitude are the position given by report 281.
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

func (r *Result) Type() string     { return "acms_report" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Airbus ACMS report headers.
type Parser struct{}

var (
	grokCompiler *patterns.Compiler
	grokOnce     sync.Once
	grokErr      error
)

func getCompiler() (*patterns.Compiler, error) {
	grokOnce.Do(func() {
		grokCompiler = patterns.NewCompiler(Formats, nil)
		grokErr = grokCompiler.Compile()
	})
	return grokCompiler, grokErr
}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "acmsreport" }
func (p *Parser) Labels() []string { return []string{"H1"} }
func (p *Parser) Priority() int    { return 60 }

// QuickCheck looks for the series prefix ("A3xx,") and a CC or C1 block,
// or one of the reports 239, 281 and 291.
func (p *Parser) QuickCheck(text string) bool {
	return len(text) > 5 && strings.HasPrefix(text, "A3") && text[4] == ',' &&
		(strings.Contains(text, "/CC") || strings.Contains(text, "/C1") ||
			strings.Contains(text, "/REP239,") || strings.Contains(text, "/REP281,") || strings.Contains(text, "/REP291,"))
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	if r := parseReport239(msg); r != nil {
		return r
	}
	compiler, err := getCompiler()
	if err != nil {
		return nil
	}
	match := compiler.Parse(msg.Text)
	if match == nil {
		return nil
	}
	c := match.Captures
	// The pattern checks the airports' shape only; reject codes that are
	// not plausible ICAO airport codes.
	if !patterns.IsValidICAO(c["origin"]) || !patterns.IsValidICAO(c["dest"]) {
		return nil
	}

	result := &Result{
		MsgID:              int64(msg.ID),
		Timestamp:          msg.Timestamp,
		AircraftSeries:     c["series"],
		Report:             c["report"],
		ReportDate:         c["month"] + c["day"],
		ReportTime:         c["time"],
		Origin:             c["origin"],
		Destination:        c["dest"],
		FlightNumberDigits: flightDigits(c["flight_digits"]),
		Flight:             c["callsign"],
	}
	result.Latitude, result.Longitude = position(c["lat_hemi"], c["lat"], c["lon_hemi"], c["lon"])
	if corruptDestination[result.Report] {
		result.Destination = ""
	}
	reg := strings.TrimLeft(c["reg_field"], ".")
	if sameRegistration(reg, msg.Tail) {
		result.Registration = reg
	}
	return result
}

// corruptDestination lists the reports whose destination is corrupted: the
// last letter is replaced by the third ("KLAA" for KLAS, "KORR" for KORD,
// "MMUU" for MMUN). In the archive, the destination of most reports 032 and
// 037, from several airlines, was corrupted this way, while their origins
// were real airports. Without an airport list the corrupted values cannot
// be told from real ones (KFLL), so these reports' destination is not
// reported, and they give no route.
var corruptDestination = map[string]bool{"032": true, "037": true}

// report239Re matches the header of report 239 up to its record, which
// starts with the report number again: "A321,037018,1,1,TB000000/REP239,
// 00,00,4/239".
var report239Re = regexp.MustCompile(`^(A3\d{2}),\d+,\d,\d,TB\d+/REP239,[^/]*/239`)

// Offsets in the report 239 record, counted from the end of the
// registration once line breaks are removed (the record is wrapped across
// lines). The registration has no fixed width, so it is located by the
// transmitted tail. In the archive, 2,990 of 3,000 records had the route at
// offset 83.
const (
	r239Flight = 0  // The flight number's digits (4).
	r239Date   = 4  // MMDDYY.
	r239Time   = 10 // HHMMSS.
	r239Route  = 83 // Origin and destination, ICAO (4 + 4).
)

// parseReport239 parses the fixed-width record of report 239:
//
//	A319,060733,1,1,TB000000/REP239,00,00,4/239N366NB2975123125181051192N45602W122616  2  5  2341  1T 0512  72
//	00 128 126 0000260500J8IH-KLAXKPDX
//
// After "239" come the registration (N366NB), the flight number's digits
// (2975), the date (123125, MMDDYY) and time (181051), a position and other
// fields not parsed, and the route (KLAX, KPDX). The record is used only
// when its registration is the transmitted tail and its route is two
// plausible ICAO codes: without a tail, the fields cannot be located.
func parseReport239(msg *acars.Message) *Result {
	m := report239Re.FindStringSubmatchIndex(msg.Text)
	if m == nil {
		return nil
	}
	tail := strings.TrimLeft(msg.Tail, ".")
	if tail == "" {
		return nil
	}
	record := strings.NewReplacer("\r", "", "\n", "").Replace(msg.Text[m[1]:])
	if !strings.HasPrefix(record, tail) {
		return nil
	}
	record = record[len(tail):]
	if len(record) < r239Route+8 {
		return nil
	}
	digits := record[r239Flight : r239Flight+4]
	date, err := time.Parse("010206", record[r239Date:r239Date+6])
	clock := record[r239Time : r239Time+6]
	origin, dest := record[r239Route:r239Route+4], record[r239Route+4:r239Route+8]
	if err != nil || !allDigits(digits) || !allDigits(clock) ||
		!patterns.IsValidICAO(origin) || !patterns.IsValidICAO(dest) {
		return nil
	}
	return &Result{
		MsgID:              int64(msg.ID),
		Timestamp:          msg.Timestamp,
		AircraftSeries:     msg.Text[m[2]:m[3]],
		Report:             "239",
		Registration:       tail,
		ReportDate:         strings.ToUpper(date.Format("Jan02")),
		ReportTime:         clock,
		Origin:             origin,
		Destination:        dest,
		FlightNumberDigits: flightDigits(digits),
	}
}

// position returns the position given in thousandths of a degree by report
// 281 ("N", "42191", "W", "072884" is 42.191, -72.884), or zeros when there
// is none or it is out of range.
func position(latHemi, lat, lonHemi, lon string) (float64, float64) {
	if latHemi == "" || lonHemi == "" {
		return 0, 0
	}
	la, errLat := strconv.Atoi(lat)
	lo, errLon := strconv.Atoi(lon)
	if errLat != nil || errLon != nil || la > 90000 || lo > 180000 {
		return 0, 0
	}
	latitude, longitude := float64(la)/1000, float64(lo)/1000
	if latHemi == "S" {
		latitude = -latitude
	}
	if lonHemi == "W" {
		longitude = -longitude
	}
	return latitude, longitude
}

// flightDigits returns the flight number's digits, or nothing for 0000,
// which reports send when no flight number is set.
func flightDigits(s string) string {
	if strings.Trim(s, "0") == "" {
		return ""
	}
	return s
}

func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// sameRegistration reports whether the report's registration is the
// transmitted tail (see acars.NormaliseRegistration). An empty tail matches
// nothing.
func sameRegistration(reg, tail string) bool {
	t := acars.NormaliseRegistration(tail)
	return t != "" && acars.NormaliseRegistration(reg) == t
}

// ParseWithTrace implements registry.Traceable for detailed debugging.
func (p *Parser) ParseWithTrace(msg *acars.Message) *registry.TraceResult {
	trace := &registry.TraceResult{ParserName: p.Name()}
	passed := p.QuickCheck(msg.Text)
	trace.QuickCheck = &registry.QuickCheck{Passed: passed}
	if !passed {
		trace.QuickCheck.Reason = "text does not start with an A3xx series and contain a CC block"
		return trace
	}
	compiler, err := getCompiler()
	if err != nil {
		trace.QuickCheck.Reason = "failed to get compiler: " + err.Error()
		return trace
	}
	compilerTrace := compiler.ParseWithTrace(msg.Text)
	for _, ft := range compilerTrace.Formats {
		trace.Formats = append(trace.Formats, registry.FormatTrace{
			Name:     ft.Name,
			Matched:  ft.Matched,
			Pattern:  ft.Pattern,
			Captures: ft.Captures,
		})
	}
	trace.Matched = p.Parse(msg) != nil
	return trace
}
