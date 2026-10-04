// Package afn parses ARINC 622 ATS facilities notification (AFN) messages,
// with which an aircraft logs on to an air traffic services unit for CPDLC
// and ADS-C, on label A0 (and relayed on label H1).
//
// Example:
//
//	/OAKODYA.AFN/FMHTZP16,.JA822J,86D5BE,031321/FAK0,KZAK/FARADS,0/FARATC,0A0F8
//
// The message is the ground station ("OAKODYA"), the IMI "AFN", a header
// (FMH: the flight ID, the registration, the 24-bit aircraft address and a
// time, the last two optional), segments, and a CRC (the last four hex
// digits, CRC-16/ARINC over "AFN/" and the text before it). The segments are
// an acknowledgement (FAK: a code and an ATS facility), a contact advisory
// (FCA: the next ground station and a code) and application statuses (FAR:
// the application, a code and an optional ground station). The codes are
// reported as transmitted; their meanings are not decoded.
package afn

import (
	"regexp"
	"strconv"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/crc"
	"acars_parser/internal/parsers/arinc"
	"acars_parser/internal/registry"
)

// Acknowledgement is an FAK segment.
type Acknowledgement struct {
	Code     string `json:"code"`
	Facility string `json:"facility,omitempty"` // ICAO ATS facility, e.g. KZAK.
}

// ContactAdvisory is an FCA segment: the ground station to contact next.
type ContactAdvisory struct {
	Station string `json:"station"`
	Code    string `json:"code"`
}

// Application is an FAR segment.
type Application struct {
	Name    string `json:"name"` // e.g. ADS, ATC.
	Code    string `json:"code"`
	Station string `json:"station,omitempty"`
}

// Result is an AFN message.
type Result struct {
	MsgID         int64  `json:"message_id"`
	Timestamp     string `json:"timestamp"`
	GroundStation string `json:"ground_station"`
	Callsign      string `json:"callsign"`
	Registration  string `json:"registration"` // Without ARINC's leading padding dots.

	// AircraftAddress is the 24-bit ICAO address the aircraft reports, when
	// the header carries one.
	AircraftAddress string           `json:"aircraft_address,omitempty"`
	Time            string           `json:"time,omitempty"` // HHMMSS.
	Acknowledgement *Acknowledgement `json:"acknowledgement,omitempty"`
	ContactAdvisory *ContactAdvisory `json:"contact_advisory,omitempty"`
	Applications    []Application    `json:"applications,omitempty"`
}

func (r *Result) Type() string     { return "afn" }
func (r *Result) MessageID() int64 { return r.MsgID }

var (
	// messageRe splits an AFN message into its ground station, its text
	// (covered by the CRC with "AFN/") and its CRC.
	messageRe = regexp.MustCompile(`^/([A-Z0-9]{4,7})\.AFN/(.+)([0-9A-F]{4})$`)

	// headerRe matches the FMH header.
	headerRe = regexp.MustCompile(`^FMH([A-Z0-9]{2,8}),(\.*[A-Z0-9-]{2,8})(?:,([0-9A-F]{6})?,(\d{6})?)?$`)

	// The segments.
	ackRe     = regexp.MustCompile(`^FAK(\d),([A-Z]{4})?$`)
	contactRe = regexp.MustCompile(`^FCA([A-Z0-9]{7}),(\d)$`)
	appRe     = regexp.MustCompile(`^FAR([A-Z]{3}),(\d)(?:,([A-Z0-9]{7}))?$`)
)

// Parser parses AFN messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "afn" }
func (p *Parser) Labels() []string { return []string{"A0", "H1"} }
func (p *Parser) Priority() int    { return 50 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, ".AFN/FMH")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) {
		return nil
	}
	text, _, ok := arinc.Unwrap(msg.Text)
	if !ok {
		return nil
	}
	m := messageRe.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	want, err := strconv.ParseUint(m[3], 16, 16)
	if err != nil || crc.CRC16Arinc([]byte("AFN/"+m[2]), 0xFFFF)^0xFFFF != uint16(want) {
		return nil
	}

	segments := strings.Split(m[2], "/")
	h := headerRe.FindStringSubmatch(segments[0])
	if h == nil {
		return nil
	}
	result := &Result{
		MsgID:         int64(msg.ID),
		Timestamp:     msg.Timestamp,
		GroundStation: m[1],
		Callsign:      h[1],
		Registration:  strings.TrimLeft(h[2], "."),
		Time:          h[4],
	}
	if acars.IsICAOAddress(h[3]) {
		result.AircraftAddress = h[3]
	}

	for _, s := range segments[1:] {
		if a := ackRe.FindStringSubmatch(s); a != nil {
			result.Acknowledgement = &Acknowledgement{Code: a[1], Facility: a[2]}
		} else if c := contactRe.FindStringSubmatch(s); c != nil {
			result.ContactAdvisory = &ContactAdvisory{Station: c[1], Code: c[2]}
		} else if ap := appRe.FindStringSubmatch(s); ap != nil {
			result.Applications = append(result.Applications, Application{Name: ap[1], Code: ap[2], Station: ap[3]})
		}
	}
	return result
}
