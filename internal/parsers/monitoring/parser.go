// Package monitoring parses the headers of engine and performance
// monitoring reports sent on label H1 in several layouts. Each header gives
// the aircraft, its callsign (or the flight number's digits), the route
// and usually the date and time; the report bodies are not parsed.
//
// The layouts, with their format names:
//
//	csv      R001V0106,180302,, N3118J,5318410-07,736738,736345,00118,KCHS,KBOS,18:31:51,2026:10:06,JBU568  ,...
//	apm      APM    1 N845MD         AAL86   KORDEGLL241026001522,
//	acm      ACM01ACM230FSN873FD  FDX48   KMEMEBLG041026  20ER...
//	engine   <330>KEG
//	         8  N27908UAL109    CL  41051026103207EDDMKIAD  1605  2
//	station  /ILNGE7X.CLB103.HL7624KE3KAL234    261023131018KSEARKSI+...
//	         /ILNGE7X.SCR005SVHZ-AR221SVA123    261102071957OEJNEGCC-...
//	         /ILNGE7X PARJBXH.6029631WSC-FHCM WJA2267 MPTOCYYC000028FF...
//	ge       /ILNGE7X.<103>GEG
//	         9   1VN-A862 HVN30     EDDFVVTS041126 392 6123313 1537
//	eb       EB0116AB_9 , 56, N29977,UAL7      ,   6,081026,19:09:41,KIAH,RJAA,...
//	e        E23C01209L819KIAHSAEZ  ...
//	abs      ABS001DA_N1811U  KSNAKSJC384
//	         ABS001DA_ N8756S KDALKAUS727 (the registration field is 8 wide)
//	         ABS001DA_N8906K  KDALKBWI 72 (a space before the digits)
//	         ABS071BA_
//	         N806AK  0764KSMFKSEA0311262092ER033030  554
//	         SB0122BA_F-GSQL  KIADLFPG176
//
// In the acm, engine, station and ge layouts the registration runs into the
// fields around it, so the transmitted tail is used to find it: without a
// tail, or when the report does not contain it where expected, they are not
// parsed. In the csv, apm and eb layouts the registration has its own
// field; a report naming another aircraft than the transmitted one is not
// parsed, so that its flight and route are not credited to the transmitting
// aircraft. The e layout gives only part of the registration, which is not
// reported. The abs layout's digits after the route are not the flight
// number (WN3202 sent 484), so it gives no flight; a blank registration
// field ("......."), which names no aircraft, gives no registration, and
// neither does a truncated field ("C   IP" for C-FFIP) or the aircraft
// address ("A2F584" for N290AK) consistent with the transmitted tail.
package monitoring

import (
	"regexp"
	"strings"
	"time"

	"acars_parser/internal/acars"
	"acars_parser/internal/nnumber"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is the header of a monitoring report.
type Result struct {
	MsgID     int64  `json:"message_id"`
	Timestamp string `json:"timestamp"`
	Format    string `json:"format"` // csv, apm, acm, engine, station, ge, eb, e or abs.
	Report    string `json:"report"` // The report's name, e.g. R001V0106, ACM230, <330>KEG.

	// Registration is the transmitted tail, which the report names.
	Registration string `json:"registration,omitempty"`

	// Flight is the callsign or flight number as sent (UAL109, BT132);
	// FlightNumberDigits is set instead when the report gives only digits.
	Flight             string `json:"flight,omitempty"`
	FlightNumberDigits string `json:"flight_number_digits,omitempty"`

	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Date        string `json:"date,omitempty"` // YYYY-MM-DD.
	Time        string `json:"time,omitempty"` // HH:MM:SS.
}

func (r *Result) Type() string     { return "monitoring_report" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	quickRe = regexp.MustCompile(`^(?:R\d{3}V|APM |B03A|ACM\d|<\d|/[A-Z0-9]{7}[ .]|EB\d|E\d{2}C|ABS\d|SB\d)`)

	csvRe = regexp.MustCompile(`^(R\d{3}V\d{4}),[^,]*,[^,]*,\s*([A-Z0-9-]{2,8})\s*,[^,]*,[^,]*,[^,]*,[^,]*,` +
		`([A-Z]{4}),([A-Z]{4}),(\d\d:\d\d:\d\d),(\d{4}:\d\d:\d\d),\s*([A-Z0-9]*?)\s*,`)
	apmRe = regexp.MustCompile(`^(?:B03A\s*\r?\n)?APM +(?:\d+ +)?([A-Z0-9-]{2,8}) +([A-Z0-9]{3,8}) +` +
		`([A-Z]{4})([A-Z]{4})(\d{6})(\d{6}),`)
	ebRe = regexp.MustCompile(`^(EB\d{4}AB)_\s*\d+\s*,\s*\d+\s*,\s*([A-Z0-9-]{2,8})\s*,\s*([A-Z0-9]{2,8})\s*,\s*\d+\s*,` +
		`(\d{6}),(\d\d:\d\d:\d\d),([A-Z]{4}),([A-Z]{4}),`)
	eRe   = regexp.MustCompile(`^(E\d{2}C)\d{5}L(\d{1,4})([A-Z]{4})([A-Z]{4})`)
	absRe = regexp.MustCompile(`^(ABS\d{3}DA|SB\d{3,4}BA)_([A-Z0-9. -]{8})([A-Z]{4})([A-Z]{4})[ \t]*\d*[ \t]*(?:\r?\n|$)`)
	// abs071Re matches report 071's header, on the second line: the
	// registration field (eight wide), the flight number's digits, the
	// route and the date (DDMMYY).
	abs071Re = regexp.MustCompile(`^(ABS071BA)_\r?\n([A-Z0-9. -]{8})(\d{4})([A-Z]{4})([A-Z]{4})(\d{6})`)

	// acmRe matches the start of an acm report; the registration follows
	// within a few letters.
	acmRe = regexp.MustCompile(`^ACM\d{2}(ACM\d{3})`)
	// engineTitleRe matches the first line of an engine report.
	engineTitleRe = regexp.MustCompile(`^<\d{2,3}>[A-Z0-9]{3,4}$`)
	// engineLeadRe matches what precedes the registration on its line.
	engineLeadRe = regexp.MustCompile(`^\s*\d{0,2}\s*`)
	// stationRe matches the start of a station report: the station's
	// address and the report's name.
	stationRe = regexp.MustCompile(`^/[A-Z0-9]{7}\.([A-Z]{2,3}\d{3,4})`)
	// geRe matches the first line of a GE engine report sent to a station:
	// the station's address and the report's number in angle brackets.
	geRe = regexp.MustCompile(`^/[A-Z0-9]{7}\.<(\d{3})>`)
	// groundStationRe matches a station report after a ground address: the
	// station's address and the originator, then a ".".
	groundStationRe = regexp.MustCompile(`^/[A-Z0-9]{7} [A-Z0-9]{7}\.`)
	// stationCallsignRouteRe matches what follows the registration in a GE
	// report and after a ground address: at most a space, the callsign,
	// then the route. The callsign must follow the registration directly,
	// so that a tail that is part of a longer registration is not matched.
	stationCallsignRouteRe = regexp.MustCompile(`^ ?([A-Z]{3}\d{1,4}[A-Z]{0,2})\s+([A-Z]{4})([A-Z]{4})`)

	// After the registration: the callsign and the route with the date.
	acmTailRe     = regexp.MustCompile(`^\s*([A-Z]{3}\d{1,4}[A-Z]{0,2})\s+([A-Z]{4})([A-Z]{4})(\d{6})`)
	engineTailRe  = regexp.MustCompile(`^([A-Z]{3}\d{1,4}[A-Z]{0,2})\s`)
	engineRouteRe = regexp.MustCompile(`(\d{6})(\d{6})([A-Z]{4})([A-Z]{4})|([A-Z]{4})([A-Z]{4})(\d{6})(\d{6})`)
	// After a station report's registration: when the registration follows
	// a ".", an airline code and a digit ("HL7624KE3KAL234"); when it follows
	// an airline code and a space ("AM N446AM2AMX021"), a digit. Then the
	// callsign, the date (YYMMDD) and time, and the route.
	stationAfterDotRe   = regexp.MustCompile(`^[A-Z0-9]{2}\d([A-Z]{3}\d{1,4}[A-Z]{0,2})\s+(\d{6})(\d{6})([A-Z]{4})([A-Z]{4})`)
	stationAfterSpaceRe = regexp.MustCompile(`^\d([A-Z]{3}\d{1,4}[A-Z]{0,2})\s+(\d{6})(\d{6})([A-Z]{4})([A-Z]{4})`)
	// stationCodeRe matches the airline code and space before a station
	// report's registration.
	stationCodeRe = regexp.MustCompile(`^[A-Z0-9]{2} `)
	// stationCodeNoSpaceRe matches the airline code right before a station
	// report's registration ("SCR005SVHZ-AR22").
	stationCodeNoSpaceRe = regexp.MustCompile(`^[A-Z0-9]{2}`)

	icaoCallsignRe = regexp.MustCompile(`^[A-Z]{3}\d{1,4}[A-Z]{0,2}$`)
	iataFlightRe   = regexp.MustCompile(`^[A-Z0-9]{2}\d{1,4}[A-Z]?$`)
	digitsRe       = regexp.MustCompile(`^\d{1,4}$`)
)

// Date layouts, in Go's notation.
const (
	ddmmyy   = "020106"
	yymmdd   = "060102"
	yyyymmdd = "2006:01:02"
)

// Parser parses monitoring report headers.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "monitoring" }
func (p *Parser) Labels() []string { return []string{"H1"} }
func (p *Parser) Priority() int    { return 60 }

func (p *Parser) QuickCheck(text string) bool {
	return quickRe.MatchString(text)
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	for _, parse := range []func(*acars.Message) *Result{parseCSV, parseAPM, parseACM, parseEngine, parseStation, parseEB, parseE, parseABS} {
		r := parse(msg)
		if r == nil {
			continue
		}
		if !patterns.IsValidICAO(r.Origin) || !patterns.IsValidICAO(r.Destination) {
			return nil
		}
		r.MsgID, r.Timestamp = int64(msg.ID), msg.Timestamp
		return r
	}
	return nil
}

func parseCSV(msg *acars.Message) *Result {
	m := csvRe.FindStringSubmatch(msg.Text)
	if m == nil {
		return nil
	}
	r := &Result{Format: "csv", Report: m[1], Origin: m[3], Destination: m[4], Time: m[5]}
	if !setDate(r, yyyymmdd, m[6]) || !setRegistration(r, m[2], msg.Tail) || !setFlight(r, m[7]) {
		return nil
	}
	return r
}

func parseAPM(msg *acars.Message) *Result {
	m := apmRe.FindStringSubmatch(msg.Text)
	if m == nil {
		return nil
	}
	r := &Result{Format: "apm", Report: "APM", Origin: m[3], Destination: m[4]}
	if !setDate(r, ddmmyy, m[5]) || !setTime(r, m[6]) || !setRegistration(r, m[1], msg.Tail) || !setFlight(r, m[2]) {
		return nil
	}
	return r
}

func parseEB(msg *acars.Message) *Result {
	m := ebRe.FindStringSubmatch(msg.Text)
	if m == nil {
		return nil
	}
	r := &Result{Format: "eb", Report: m[1], Time: m[5], Origin: m[6], Destination: m[7]}
	if !setDate(r, ddmmyy, m[4]) || !setRegistration(r, m[2], msg.Tail) || !setFlight(r, m[3]) {
		return nil
	}
	return r
}

func parseE(msg *acars.Message) *Result {
	m := eRe.FindStringSubmatch(msg.Text)
	if m == nil {
		return nil
	}
	return &Result{Format: "e", Report: m[1], FlightNumberDigits: m[2], Origin: m[3], Destination: m[4]}
}

func parseABS(msg *acars.Message) *Result {
	var r *Result
	var field string
	if m := absRe.FindStringSubmatch(msg.Text); m != nil {
		r = &Result{Format: "abs", Report: m[1], Origin: m[3], Destination: m[4]}
		field = m[2]
	} else if m := abs071Re.FindStringSubmatch(msg.Text); m != nil {
		r = &Result{Format: "abs", Report: m[1], Origin: m[4], Destination: m[5]}
		field = m[2]
		if !setDate(r, ddmmyy, m[6]) || !setFlight(r, m[3]) {
			return nil
		}
	} else {
		return nil
	}
	// The registration field is eight characters wide; its leading blanks
	// show where a truncated registration resumes.
	trimmed := strings.TrimSpace(field)
	switch {
	case strings.Trim(trimmed, ". ") == "":
		// A blank field names no aircraft.
	case msg.Tail == "" || acars.NormaliseRegistration(trimmed) == acars.NormaliseRegistration(msg.Tail):
		setRegistration(r, trimmed, msg.Tail)
	case acars.TruncatedRegistrationOf(field, msg.Tail) || addressOfTail(trimmed, msg.Tail):
		// The tail's, but the field does not give the registration.
	default:
		return nil
	}
	return r
}

func parseACM(msg *acars.Message) *Result {
	line := firstLine(msg.Text)
	m := acmRe.FindStringSubmatchIndex(line)
	if m == nil {
		return nil
	}
	// The registration follows two or four letters ("FS", "DSEK"); a tail
	// found after anything else is part of another registration.
	head := line[m[1]:]
	i := strings.Index(head, msg.Tail)
	if msg.Tail == "" || (i != 2 && i != 4) || !isLetters(head[:i]) {
		return nil
	}
	t := acmTailRe.FindStringSubmatch(head[i+len(msg.Tail):])
	if t == nil {
		return nil
	}
	r := &Result{Format: "acm", Report: line[m[2]:m[3]], Registration: msg.Tail, Flight: t[1], Origin: t[2], Destination: t[3]}
	if !setDate(r, ddmmyy, t[4]) {
		return nil
	}
	return r
}

func parseEngine(msg *acars.Message) *Result {
	lines := strings.SplitN(strings.ReplaceAll(msg.Text, "\r", ""), "\n", 3)
	if len(lines) < 2 || !engineTitleRe.MatchString(strings.TrimSpace(lines[0])) {
		return nil
	}
	line := lines[1]
	line = line[len(engineLeadRe.FindString(line)):]
	if msg.Tail == "" || !strings.HasPrefix(line, msg.Tail) {
		return nil
	}
	rest := line[len(msg.Tail):]
	c := engineTailRe.FindStringSubmatch(rest)
	if c == nil {
		return nil
	}
	m := engineRouteRe.FindStringSubmatch(rest[len(c[1]):])
	if m == nil {
		return nil
	}
	r := &Result{Format: "engine", Report: strings.TrimSpace(lines[0]), Registration: msg.Tail, Flight: c[1]}
	date, clock := m[1], m[2]
	r.Origin, r.Destination = m[3], m[4]
	if m[1] == "" {
		r.Origin, r.Destination, date, clock = m[5], m[6], m[7], m[8]
	}
	if !setDate(r, ddmmyy, date) || !setTime(r, clock) {
		return nil
	}
	return r
}

func parseStation(msg *acars.Message) *Result {
	if msg.Tail == "" {
		return nil
	}
	line := firstLine(msg.Text)
	if g := groundStationRe.FindStringIndex(line); g != nil {
		return parseGroundStation(line[g[1]:], msg.Tail)
	}
	if g := geRe.FindStringSubmatch(line); g != nil {
		return parseGE(msg.Text, g[1], msg.Tail)
	}
	m := stationRe.FindStringSubmatchIndex(line)
	if m == nil {
		return nil
	}
	// The registration follows the report's name, after a "." or two
	// ("TOF103..B-LJC"), or after an airline code and a space ("CLB006AM
	// N446AM") or no space ("SCR005SVHZ-AR22"); what comes after it
	// depends on which (see stationAfterDotRe).
	rest := line[m[1]:]
	var after *regexp.Regexp
	switch {
	case strings.HasPrefix(rest, "."+msg.Tail):
		rest, after = rest[1+len(msg.Tail):], stationAfterDotRe
	case strings.HasPrefix(rest, ".."+msg.Tail):
		rest, after = rest[2+len(msg.Tail):], stationAfterDotRe
	case stationCodeRe.MatchString(rest) && strings.HasPrefix(rest[3:], msg.Tail):
		rest, after = rest[3+len(msg.Tail):], stationAfterSpaceRe
	case stationCodeNoSpaceRe.MatchString(rest) && strings.HasPrefix(rest[2:], msg.Tail):
		rest, after = rest[2+len(msg.Tail):], stationAfterSpaceRe
	default:
		return nil
	}
	s := after.FindStringSubmatch(rest)
	if s == nil {
		return nil
	}
	r := &Result{Format: "station", Report: line[m[2]:m[3]], Registration: msg.Tail, Flight: s[1], Origin: s[4], Destination: s[5]}
	if !setDate(r, yymmdd, s[2]) || !setTime(r, s[3]) {
		return nil
	}
	return r
}

// groundStationPrefixLen is the length of the field between a ground
// address and the registration: seven characters and the airline's IATA
// code ("6029631" and "WS" before C-FHCM). In every report seen in live
// traffic (October and November 2026) it had this length.
const groundStationPrefixLen = 9

// parseGroundStation parses a station report after a ground address: a
// fixed field, the registration, then the callsign and the route, as in
// "6029631WSC-FHCM WJA2267 MPTOCYYC...". It names no report. The fixed
// field keeps a tail from matching the end of a longer registration
// ("...WSHB-1234" for B-1234).
func parseGroundStation(rest, tail string) *Result {
	if len(rest) < groundStationPrefixLen || !isAlnum(rest[:groundStationPrefixLen]) ||
		!strings.HasPrefix(rest[groundStationPrefixLen:], tail) {
		return nil
	}
	s := stationCallsignRouteRe.FindStringSubmatch(rest[groundStationPrefixLen+len(tail):])
	if s == nil {
		return nil
	}
	return &Result{Format: "station", Registration: tail, Flight: s[1], Origin: s[2], Destination: s[3]}
}

// parseGE parses a GE engine report sent to a station: the report's number
// on the first line ("<103>GEG"), then on the second the registration,
// after digits and spaces, followed by the callsign and the route, as in
// "9   1VN-A862 HVN30     EDDFVVTS041126".
func parseGE(text, report, tail string) *Result {
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\r' || r == '\n' })
	if len(lines) < 2 {
		return nil
	}
	second := lines[1]
	// The registration may start with a digit ("9H-WDN"), so each start
	// within the leading digits and spaces is tried.
	lead := len(second) - len(strings.TrimLeft(second, "0123456789 "))
	for j := 0; j <= lead; j++ {
		if !strings.HasPrefix(second[j:], tail) {
			continue
		}
		if s := stationCallsignRouteRe.FindStringSubmatch(second[j+len(tail):]); s != nil {
			return &Result{Format: "ge", Report: report, Registration: tail, Flight: s[1], Origin: s[2], Destination: s[3]}
		}
	}
	return nil
}

func isAlnum(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] < 'A' || s[i] > 'Z') && (s[i] < '0' || s[i] > '9') {
			return false
		}
	}
	return true
}

func isLetters(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}

func firstLine(text string) string {
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		return text[:i]
	}
	return text
}

// setRegistration sets the report's registration, and returns false if it
// names another aircraft than the transmitted tail. Without a transmitted
// tail, no registration is reported.
func setRegistration(r *Result, reg, tail string) bool {
	t := acars.NormaliseRegistration(tail)
	if t == "" {
		return true
	}
	if acars.NormaliseRegistration(reg) != t {
		return false
	}
	r.Registration = reg
	return true
}

// addressOfTail reports whether an ABS registration field is the aircraft
// address (Alaska sends it: "A2F584" for N290AK) of a US registration,
// derived from the tail.
func addressOfTail(field, tail string) bool {
	addr, ok := nnumber.ICAOAddress(acars.NormaliseRegistration(tail))
	return ok && field == addr
}

// setFlight sets the callsign or flight number, or its digits, and returns
// false if the field holds something else: the report names a flight that
// cannot be compared with the transmitted one, so its route must not be
// credited to that flight. An empty field names none.
func setFlight(r *Result, f string) bool {
	switch {
	case f == "":
	case digitsRe.MatchString(f):
		r.FlightNumberDigits = f
	case icaoCallsignRe.MatchString(f), iataFlightRe.MatchString(f):
		r.Flight = f
	default:
		return false
	}
	return true
}

// setDate sets the date from s in layout, and returns false if s is not a
// date.
func setDate(r *Result, layout, s string) bool {
	d, err := time.Parse(layout, s)
	if err != nil {
		return false
	}
	r.Date = d.Format("2006-01-02")
	return true
}

// setTime sets the time from HHMMSS, and returns false if it is not a time.
func setTime(r *Result, s string) bool {
	t, err := time.Parse("150405", s)
	if err != nil {
		return false
	}
	r.Time = t.Format("15:04:05")
	return true
}
