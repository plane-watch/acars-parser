package h1

import (
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// ProgressResult is an H1 PRG progress report: the destination, and when
// the report gives them, the ICAO callsign, the time and the origin.
type ProgressResult struct {
	MsgID       int64  `json:"message_id"`
	Timestamp   string `json:"timestamp"`
	Origin      string `json:"origin,omitempty"`
	Destination string `json:"destination"`
	Flight      string `json:"flight,omitempty"`      // ICAO callsign, e.g. DAL162.
	ReportTime  string `json:"report_time,omitempty"` // HHMMSS.
}

func (r *ProgressResult) Type() string     { return "progress_report" }
func (r *ProgressResult) MessageID() int64 { return r.MsgID }

// A progress report is a list of fields separated by "/", in any order,
// and, like other FMC downlinks, ends with a four-character checksum
// appended to its last field. Examples:
//
//	PRG/DTEHAM,18R,207,085235,041/FNDAL162/TS080747,090223AE30
//	PRG/TS093622,041026/DTEGCC,23R,218,110622,043/FNUAE34YB474
//	PRG/DTLEMG,12O,63,092914/PR1380,.../RP:DA:LSGG:AA:LEMG:A:BLN1A:...
//
// DT starts with the destination; the runway (whose suffix, as in "12O",
// is not established) and the numbers after it are not parsed. FN is the
// callsign, TS the time and date. An RP section gives the route from the
// origin (DA) to the arrival airport (AA). Southwest sends another layout,
// "PRG/LR,<time>,<callsign>,<origin>,<destination>,...", parsed by
// progressLRRe.
var (
	progressDestRe   = regexp.MustCompile(`^DT([A-Z]{4}),`)
	progressFlightRe = regexp.MustCompile(`^FN([A-Z]{3}\d{1,4}[A-Z]{0,2})$`)
	progressTimeRe   = regexp.MustCompile(`^TS(\d{6}),`)
	progressLRRe     = regexp.MustCompile(`^PRG/LR,(\d{6}),([A-Z]{3}\d{1,4}[A-Z]{0,2}),([A-Z]{4}),([A-Z]{4}),`)
)

// progressChecksumLen is the length of the checksum that ends the report.
const progressChecksumLen = 4

// ProgressParser parses H1 PRG progress reports.
type ProgressParser struct{}

func init() {
	registry.Register(&ProgressParser{})
}

func (p *ProgressParser) Name() string     { return "progress" }
func (p *ProgressParser) Labels() []string { return []string{"H1"} }
func (p *ProgressParser) Priority() int    { return 60 }

func (p *ProgressParser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "PRG/")
}

func (p *ProgressParser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	text := strings.TrimSpace(NormaliseFPN(msg.Text))
	result := &ProgressResult{MsgID: int64(msg.ID), Timestamp: msg.Timestamp}

	if m := progressLRRe.FindStringSubmatch(text); m != nil {
		if !patterns.IsValidICAO(m[3]) || !patterns.IsValidICAO(m[4]) {
			return nil
		}
		result.ReportTime, result.Flight, result.Origin, result.Destination = m[1], m[2], m[3], m[4]
		return result
	}

	// Remove the checksum, so that the last field reads as sent.
	if len(text) <= progressChecksumLen {
		return nil
	}
	body := text[:len(text)-progressChecksumLen]
	for _, field := range strings.Split(body, "/")[1:] {
		if m := progressDestRe.FindStringSubmatch(field); m != nil {
			result.Destination = m[1]
		} else if m := progressFlightRe.FindStringSubmatch(field); m != nil {
			result.Flight = m[1]
		} else if m := progressTimeRe.FindStringSubmatch(field); m != nil {
			result.ReportTime = m[1]
		}
	}
	if !patterns.IsValidICAO(result.Destination) {
		return nil
	}

	// The RP route is used only when its arrival airport is the DT
	// destination, so that the two fields agree on the flight.
	if i := strings.Index(text, "/RP:"); i >= 0 {
		route := TokeniseFPN(text[i+1:])
		if origin := route.GetOrigin(); patterns.IsValidICAO(origin) && route.GetDestination() == result.Destination {
			result.Origin = origin
		}
	}
	return result
}
