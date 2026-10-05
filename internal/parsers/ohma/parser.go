// Package ohma parses reports from Boeing's onboard health management
// application (OHMA), sent on label H1 by 737 MAX aircraft. After "OHMA"
// the message is base64 of zlib-compressed JSON:
//
//	{"version":"2.0","message":"{\"clientId\":\"OHMA\",\"messageDate\":\"2026-01-12T18:28:41.954Z\",
//	 \"data\":{\"airplanes\":[{\"tailNumber\":\"C-GFOF\",\"model\":\"\",\"flights\":[{
//	 \"departureAirportCode\":\"CYEG\",\"arrivalAirportCode\":\"CYVR\",\"flightNumber\":\"FLE821\",...
//
// The inner "message" is itself JSON, as a string. The parser reports the
// tail, flight and route of a report about one aircraft and one flight; the
// health events are not parsed. The "model" field was empty in every report
// in the archive. Longer reports are split into segments (with "msg_seq"
// and "msg_total") or across ACARS blocks; a part on its own does not
// decode, and is not parsed.
package ohma

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result is the aircraft, flight and route of an OHMA report.
type Result struct {
	MsgID     int64  `json:"message_id"`
	Timestamp string `json:"timestamp"`

	// Registration is reported only when it is the transmitted tail.
	Registration string `json:"registration,omitempty"`
	Flight       string `json:"flight"` // ICAO callsign, e.g. FLE821.
	Origin       string `json:"origin"`
	Destination  string `json:"destination"`
	MessageDate  string `json:"message_date"` // As sent, e.g. 2026-01-12T18:28:41.954Z.
}

func (r *Result) Type() string     { return "ohma" }
func (r *Result) MessageID() int64 { return r.MsgID }

// envelope is the outer JSON object.
type envelope struct {
	Message string `json:"message"`
}

// report is the inner JSON object, with only the fields parsed.
type report struct {
	MessageDate string `json:"messageDate"`
	Data        struct {
		Airplanes []struct {
			TailNumber string `json:"tailNumber"`
			Flights    []struct {
				Departure    string `json:"departureAirportCode"`
				Arrival      string `json:"arrivalAirportCode"`
				FlightNumber string `json:"flightNumber"`
			} `json:"flights"`
		} `json:"airplanes"`
	} `json:"data"`
}

// callsignRe matches an ICAO callsign.
var callsignRe = regexp.MustCompile(`^[A-Z]{3}\d{1,4}[A-Z]{0,2}$`)

// maxDecompressed bounds the decompressed size: the largest report in the
// archive is far smaller.
const maxDecompressed = 1 << 20

// Parser parses OHMA reports.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "ohma" }
func (p *Parser) Labels() []string { return []string{"H1"} }
func (p *Parser) Priority() int    { return 60 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.HasPrefix(text, "OHMA")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	r, ok := decode(msg.Text)
	if !ok || len(r.Data.Airplanes) != 1 || len(r.Data.Airplanes[0].Flights) != 1 {
		return nil
	}
	plane := r.Data.Airplanes[0]
	flight := plane.Flights[0]
	if !callsignRe.MatchString(flight.FlightNumber) ||
		!patterns.IsValidICAO(flight.Departure) || !patterns.IsValidICAO(flight.Arrival) {
		return nil
	}
	result := &Result{
		MsgID:       int64(msg.ID),
		Timestamp:   msg.Timestamp,
		Flight:      flight.FlightNumber,
		Origin:      flight.Departure,
		Destination: flight.Arrival,
		MessageDate: r.MessageDate,
	}
	if tail := acars.NormaliseRegistration(msg.Tail); tail != "" && acars.NormaliseRegistration(plane.TailNumber) == tail {
		result.Registration = plane.TailNumber
	}
	return result
}

// decode returns the inner report of an OHMA message, and false if the
// message is not a complete report.
func decode(text string) (report, bool) {
	var r report
	payload := strings.NewReplacer("\r", "", "\n", "").Replace(strings.TrimPrefix(text, "OHMA"))
	compressed, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return r, false
	}
	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return r, false
	}
	defer func() { _ = zr.Close() }()
	data, err := io.ReadAll(io.LimitReader(zr, maxDecompressed))
	if err != nil {
		return r, false
	}
	var env envelope
	if json.Unmarshal(data, &env) != nil || json.Unmarshal([]byte(env.Message), &r) != nil {
		return r, false
	}
	return r, true
}
