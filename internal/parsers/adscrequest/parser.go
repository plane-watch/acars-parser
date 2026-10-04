// Package adscrequest parses ADS-C uplinks: the contract requests and
// cancellations that an air traffic services unit sends to an aircraft
// (ARINC 745), on label A6 and relayed on label H1 ("- #MD/A6 ..."). The
// aircraft's reports, on B6, are parsed by the adsc package.
//
// A message is a sequence of tags: 1 cancels all contracts; 2 cancels a
// contract and 6 its emergency mode (each with the contract number); 7, 8
// and 9 request a periodic, event or emergency periodic contract (the
// contract number, then the request's own tags, up to the first tag that is
// not a request tag). The encoding follows libacars (adsc.c), against which
// the corpus decodes were checked.
package adscrequest

import (
	"errors"
	"fmt"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/parsers/arinc"
	"acars_parser/internal/registry"
)

// Intent is an aircraft intent data request (tag 21).
type Intent struct {
	Modulus           int `json:"modulus"`
	ProjectionMinutes int `json:"projection_minutes"`
}

// Request is one request in an ADS-C uplink.
type Request struct {
	// Kind is cancel_all, cancel, cancel_emergency, periodic, event or
	// emergency_periodic.
	Kind     string `json:"kind"`
	Contract *int   `json:"contract,omitempty"`

	IntervalSecs       *int     `json:"interval_secs,omitempty"`        // Reporting interval (periodic).
	LateralDeviationNM *float64 `json:"lateral_deviation_nm,omitempty"` // Report when exceeded (event).
	VerticalSpeedFPM   *int     `json:"vertical_speed_fpm,omitempty"`   // Report when reached (event).
	AltitudeFloorFt    *int     `json:"altitude_floor_ft,omitempty"`    // Report when outside the range (event).
	AltitudeCeilingFt  *int     `json:"altitude_ceiling_ft,omitempty"`
	WaypointChanges    bool     `json:"waypoint_changes,omitempty"` // Report waypoint changes (event).

	// Groups maps each requested data group (flight_id, predicted_route,
	// earth_reference, air_reference, meteo, airframe_id) to its modulus:
	// it is sent with every modulus-th report.
	Groups         map[string]int `json:"groups,omitempty"`
	AircraftIntent *Intent        `json:"aircraft_intent,omitempty"`
}

// Result is an ADS-C uplink.
type Result struct {
	MsgID         int64     `json:"message_id"`
	Timestamp     string    `json:"timestamp"`
	GroundStation string    `json:"ground_station"`
	Registration  string    `json:"registration"`
	Requests      []Request `json:"requests"`
}

func (r *Result) Type() string     { return "adsc_request" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses ADS-C uplinks.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "adscrequest" }
func (p *Parser) Labels() []string { return []string{"A6", "H1"} }
func (p *Parser) Priority() int    { return 50 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, ".ADS")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if !p.QuickCheck(msg.Text) || !isUplink(msg) {
		return nil
	}
	text, relayed, ok := arinc.Unwrap(msg.Text)
	if !ok || (msg.Label == "H1" && relayed != "A6") {
		// On H1, only a message relayed with its original label (A6) is
		// known to be an uplink.
		return nil
	}
	env, err := arinc.Parse(text)
	if err != nil || env.IMI != arinc.IMIADS {
		return nil
	}
	requests, err := decode(env.Payload)
	if err != nil {
		return nil
	}
	return &Result{
		MsgID:         int64(msg.ID),
		Timestamp:     msg.Timestamp,
		GroundStation: env.GroundStation,
		Registration:  env.Registration,
		Requests:      requests,
	}
}

// isUplink reports whether the message can be an uplink: the link layer or
// block ID does not say it is a downlink.
func isUplink(msg *acars.Message) bool {
	if msg.LinkDirection == "downlink" {
		return false
	}
	if msg.BlockID != "" && msg.BlockID[0] >= '0' && msg.BlockID[0] <= '9' {
		return false
	}
	return true
}

var errTruncated = errors.New("truncated")

// groupNames are the data group request tags.
var groupNames = map[byte]string{
	12: "flight_id",
	13: "predicted_route",
	14: "earth_reference",
	15: "air_reference",
	16: "meteo",
	17: "airframe_id",
}

// decode decodes the requests in an ADS-C uplink payload (without its CRC).
func decode(b []byte) ([]Request, error) {
	if len(b) == 0 {
		return nil, errTruncated
	}
	var out []Request
	for len(b) > 0 {
		tag := b[0]
		b = b[1:]
		var req Request
		switch tag {
		case 1:
			req.Kind = "cancel_all"
		case 2, 6:
			req.Kind = map[byte]string{2: "cancel", 6: "cancel_emergency"}[tag]
			if len(b) < 1 {
				return nil, errTruncated
			}
			req.Contract = intPtr(int(b[0]))
			b = b[1:]
		case 7, 8, 9:
			req.Kind = map[byte]string{7: "periodic", 8: "event", 9: "emergency_periodic"}[tag]
			if len(b) < 1 {
				return nil, errTruncated
			}
			req.Contract = intPtr(int(b[0]))
			var err error
			if b, err = decodeRequestTags(&req, b[1:]); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown tag %d", tag)
		}
		out = append(out, req)
	}
	return out, nil
}

// decodeRequestTags decodes a contract request's tags into req, up to the
// first byte that is not a request tag (the next request, or the end), and
// returns the rest.
func decodeRequestTags(req *Request, b []byte) ([]byte, error) {
	need := func(n int) error {
		if len(b) < 1+n {
			return errTruncated
		}
		return nil
	}
	for len(b) > 0 {
		tag := b[0]
		switch {
		case tag == 10: // Lateral deviation threshold, in eighths of a nautical mile.
			if err := need(1); err != nil {
				return nil, err
			}
			req.LateralDeviationNM = floatPtr(float64(b[1]) / 8)
			b = b[2:]
		case tag == 11: // Reporting interval: a 2-bit scaling factor and a 6-bit rate.
			if err := need(1); err != nil {
				return nil, err
			}
			sf := int(b[1] >> 6)
			switch sf {
			case 2:
				sf = 8
			case 3:
				sf = 64
			}
			req.IntervalSecs = intPtr(sf * (int(b[1]&0x3f) + 1))
			b = b[2:]
		case groupNames[tag] != "": // A data group and its modulus.
			if err := need(1); err != nil {
				return nil, err
			}
			if req.Groups == nil {
				req.Groups = map[string]int{}
			}
			req.Groups[groupNames[tag]] = int(b[1])
			b = b[2:]
		case tag == 18: // Vertical speed threshold, signed, in 64 ft/min.
			if err := need(1); err != nil {
				return nil, err
			}
			req.VerticalSpeedFPM = intPtr(int(int8(b[1])) * 64)
			b = b[2:]
		case tag == 19: // Altitude range: ceiling then floor, signed, in 4 ft.
			if err := need(4); err != nil {
				return nil, err
			}
			req.AltitudeCeilingFt = intPtr(int(int16(uint16(b[1])<<8|uint16(b[2]))) * 4)
			req.AltitudeFloorFt = intPtr(int(int16(uint16(b[3])<<8|uint16(b[4]))) * 4)
			b = b[5:]
		case tag == 20: // Report waypoint changes.
			req.WaypointChanges = true
			b = b[1:]
		case tag == 21: // Aircraft intent: modulus and projection time.
			if err := need(2); err != nil {
				return nil, err
			}
			req.AircraftIntent = &Intent{Modulus: int(b[1]), ProjectionMinutes: int(b[2])}
			b = b[3:]
		default:
			return b, nil
		}
	}
	return b, nil
}

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }
