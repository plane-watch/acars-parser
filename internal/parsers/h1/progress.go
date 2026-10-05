package h1

import (
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/crc"
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
// and, like other FMC downlinks, usually ends with a four-character
// checksum appended to its last field. Examples:
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
	progressDestRe     = regexp.MustCompile(`^DT([A-Z]{4}),`)
	progressCallsignRe = regexp.MustCompile(`^[A-Z]{3}\d{1,4}[A-Z]{0,2}$`)
	progressTimeRe     = regexp.MustCompile(`^TS(\d{6}),`)
	progressLRRe       = regexp.MustCompile(`^PRG/LR,(\d{6}),([A-Z]{3}\d{1,4}[A-Z]{0,2}),([A-Z]{4}),([A-Z]{4}),`)
)

// progressChecksumLen is the length of the checksum that usually ends the
// report.
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

	fields := strings.Split(text, "/")[1:]
	for i, field := range fields {
		switch {
		case strings.HasPrefix(field, "FN"):
			flight, ok := progressCallsign(field[2:], i == len(fields)-1)
			if !ok {
				// The flight is unknown, so the destination cannot be
				// credited to any flight.
				return nil
			}
			result.Flight = flight
		default:
			if m := progressDestRe.FindStringSubmatch(field); m != nil {
				result.Destination = m[1]
			} else if m := progressTimeRe.FindStringSubmatch(field); m != nil {
				result.ReportTime = m[1]
			}
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

// progressCallsign returns the callsign in an FN field's value, or nothing,
// and false when the value is ambiguous.
// In the last field the value may end with the checksum: four hex
// characters, as in "UAE34YB474" (UAE34Y). When the value ends with four
// hex characters, the checksum is removed only if that leaves the one valid
// callsign; when both readings are callsigns ("UAL1234AB" or "UAL12"), the
// checksum cannot be told from the callsign without verifying it, which is
// not done (its algorithm is not established), so the value is ambiguous.
func progressCallsign(value string, last bool) (string, bool) {
	if !last || len(value) <= progressChecksumLen || !isHex(value[len(value)-progressChecksumLen:]) {
		if progressCallsignRe.MatchString(value) {
			return value, true
		}
		return "", true
	}
	stripped := value[:len(value)-progressChecksumLen]
	whole, cut := progressCallsignRe.MatchString(value), progressCallsignRe.MatchString(stripped)
	switch {
	case cut && whole:
		return "", false
	case cut:
		return stripped, true
	case whole:
		return value, true
	}
	return "", true
}

// isHex reports whether s is made of hex digits.
func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if !crc.IsHexDigit(s[i]) {
			return false
		}
	}
	return true
}
