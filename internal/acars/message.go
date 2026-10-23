// Package acars provides ACARS message types and structures.
package acars

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// FlexInt64 handles JSON fields that can be either string or number.
type FlexInt64 int64

func (f *FlexInt64) UnmarshalJSON(data []byte) error {
	// Try as number first
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		*f = FlexInt64(i)
		return nil
	}

	// Try as string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "" {
			*f = 0
			return nil
		}
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			*f = 0
			return nil // Silently ignore unparseable IDs
		}
		*f = FlexInt64(i)
		return nil
	}

	*f = 0
	return nil
}

// Message represents the inner message from an ACARS feed.
// This can be populated directly from flat JSON or extracted from NATSWrapper.
type Message struct {
	ID        FlexInt64 `json:"id"`
	Source    string    `json:"source"`
	Timestamp string    `json:"timestamp"`
	Tail      string    `json:"tail"`
	Text      string    `json:"text"`
	Label     string    `json:"label"`
	Frequency float64   `json:"frequency"`

	// FlightNumber is the flight number as transmitted in the message, with
	// padding trimmed. It is distinct from Flight, which is Airframes' record.
	FlightNumber string `json:"flight_number,omitempty"`

	// Direction indicators from the transport layer.
	BlockID       string `json:"block_id,omitempty"`       // ACARS block ID ('0'-'9' = downlink, 'A'-'X' = uplink).
	LinkDirection string `json:"link_direction,omitempty"` // Explicit direction: "uplink" or "downlink".

	// MessageNumber is the downlink's message sequence number, e.g. "D47A":
	// three characters numbering the message, and a block letter (A, B,
	// ...) numbering the blocks of a multi-block message. A message joined
	// from its blocks has the three characters only.
	MessageNumber string `json:"message_number,omitempty"`

	// BlockEnd is the feed's block_end flag, nil when the feed did not
	// send it. It does not reliably mark a message's last block: in October
	// 2026 it was false on most single-block messages, and true or false on
	// every block of some multi-block messages.
	BlockEnd *bool `json:"block_end,omitempty"`

	// AssembledFrom lists, in order, the IDs of the blocks a message was
	// joined from (see internal/reassembly); it is empty for a message
	// received as sent.
	AssembledFrom []int64 `json:"assembled_from,omitempty"`

	// Link-layer (VDL) addresses of the sender and recipient: 24-bit ICAO
	// addresses as six upper-case hex digits. See AircraftAddress.
	FromHex string `json:"from_hex,omitempty"`
	ToHex   string `json:"to_hex,omitempty"`

	// Airframes' own metadata about the aircraft, flight and receiving
	// station. It is Airframes' enrichment, not transmitted data, so it must
	// not be used as a source of facts that acars_parser publishes (its
	// accuracy is unknown, and Airframes may itself draw on data that
	// acars_parser feeds). It may be shown for context in the console.
	Airframe *Airframe `json:"airframe,omitempty"`
	Flight   *Flight   `json:"flight,omitempty"`
	Station  *Station  `json:"station,omitempty"`
}

// h1PreambleRe matches the preamble of an H1 FMC downlink's text: an
// optional "- ", "#", a two-character sub-label (M1, DF, CF, 10, ...) and
// "B", as in "#M1BPOSN..." and "- #DFB++86501...". Other H1 forms, such as
// "- #MD/AA ..." (CPDLC and ADS-C) and "#M1FPN/...", are not matched: their
// parsers read the text whole.
var h1PreambleRe = regexp.MustCompile(`^(?:-\s*)?#[A-Z0-9]{2}B`)

// ForParsing returns the message as parsers see it: for an H1 message with
// an FMC preamble, a copy whose text starts after the preamble, so that a
// report reads the same whether or not the feeder kept the preamble;
// otherwise the message itself. The message's own text, which is stored,
// is not changed.
func (m *Message) ForParsing() *Message {
	preamble := m.H1Preamble()
	if preamble == "" {
		return m
	}
	c := *m
	c.Text = m.Text[len(preamble):]
	return &c
}

// H1Preamble returns the FMC preamble at the start of an H1 message's text
// (see h1PreambleRe), or nothing if the message has none.
func (m *Message) H1Preamble() string {
	if m.Label != "H1" {
		return ""
	}
	return h1PreambleRe.FindString(m.Text)
}

// AircraftAddress returns the aircraft's 24-bit ICAO address as carried by the
// link layer: the sender of a downlink or the recipient of an uplink. The
// direction comes from LinkDirection, or failing that the block ID (digits are
// downlinks, letters A to X are uplinks). It reports false if the direction or a
// valid, non-zero address is not known.
func (m *Message) AircraftAddress() (string, bool) {
	var addr string
	switch {
	case m.LinkDirection == "downlink":
		addr = m.FromHex
	case m.LinkDirection == "uplink":
		addr = m.ToHex
	case len(m.BlockID) == 1 && m.BlockID[0] >= '0' && m.BlockID[0] <= '9':
		addr = m.FromHex
	case len(m.BlockID) == 1 && m.BlockID[0] >= 'A' && m.BlockID[0] <= 'X':
		addr = m.ToHex
	default:
		return "", false
	}
	if !IsICAOAddress(addr) {
		return "", false
	}
	return addr, true
}

// NormaliseRegistration returns a registration in a form for comparison:
// upper case, without surrounding space, the leading dots with which ARINC
// pads registrations to seven characters ("..N17RX"), or dashes. Two forms
// of one registration ("VH-EBO", ".VHEBO") normalise to the same value.
func NormaliseRegistration(s string) string {
	s = strings.TrimLeft(strings.TrimSpace(s), ".")
	return strings.ToUpper(strings.ReplaceAll(s, "-", ""))
}

// IsICAOAddress reports whether s is a usable 24-bit ICAO aircraft address:
// six upper-case hex digits, and neither all zeros nor all ones (which are
// not assigned to aircraft).
func IsICAOAddress(s string) bool {
	if len(s) != 6 || s == "000000" || s == "FFFFFF" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// Airframe contains aircraft identification data.
type Airframe struct {
	ID                string `json:"id,omitempty"`
	Tail              string `json:"tail"`
	ICAO              string `json:"icao"`
	IATA              string `json:"iata,omitempty"`
	Manufacturer      string `json:"manufacturer,omitempty"`
	ManufacturerModel string `json:"manufacturer_model,omitempty"`
	Owner             string `json:"owner,omitempty"`
	Military          bool   `json:"military,omitempty"`
}

// Flight contains flight identification and route data.
type Flight struct {
	ID                 string  `json:"id,omitempty"`
	Flight             string  `json:"flight"`
	Status             string  `json:"status,omitempty"`
	DepartingAirport   string  `json:"departing_airport,omitempty"`
	DestinationAirport string  `json:"destination_airport,omitempty"`
	Latitude           float64 `json:"latitude,omitempty"`
	Longitude          float64 `json:"longitude,omitempty"`
	Altitude           int     `json:"altitude,omitempty"`
}

// Station contains ground station data.
type Station struct {
	ID                 string  `json:"id,omitempty"`
	Ident              string  `json:"ident,omitempty"`
	NearestAirportIcao string  `json:"nearest_airport_icao,omitempty"`
	Latitude           float64 `json:"latitude,omitempty"`
	Longitude          float64 `json:"longitude,omitempty"`
}

// NATSWrapper represents the NATS feed message format where the ACARS
// message is nested inside a "message" field with metadata at the top level.
type NATSWrapper struct {
	Source   *NATSSource `json:"source,omitempty"`
	Station  *Station    `json:"station,omitempty"`
	Airframe *Airframe   `json:"airframe,omitempty"`
	Flight   *Flight     `json:"flight,omitempty"`
	Message  *NATSInner  `json:"message,omitempty"`
}

// NATSSource contains source metadata from the NATS feed.
type NATSSource struct {
	Name        string `json:"name,omitempty"`
	Application string `json:"application,omitempty"`
}

// NATSInner is the inner message structure from NATS feed.
type NATSInner struct {
	ID            FlexInt64 `json:"id"`
	Timestamp     string    `json:"timestamp"`
	Label         string    `json:"label"`
	Text          string    `json:"text"`
	Tail          string    `json:"tail"`
	Flight        string    `json:"flight"`
	Frequency     float64   `json:"frequency"`
	FromHex       string    `json:"from_hex,omitempty"`
	ToHex         string    `json:"to_hex,omitempty"`
	BlockID       string    `json:"block_id,omitempty"`       // ACARS block ID ('0'-'9' = downlink, 'A'-'X' = uplink).
	LinkDirection string    `json:"link_direction,omitempty"` // Explicit direction: "uplink" or "downlink".
	MessageNumber string    `json:"message_number,omitempty"`
	BlockEnd      *bool     `json:"block_end,omitempty"`
}

// ToMessage converts a NATSWrapper to a unified Message.
func (w *NATSWrapper) ToMessage() *Message {
	if w.Message == nil {
		return nil
	}

	msg := &Message{
		ID:            w.Message.ID,
		Timestamp:     w.Message.Timestamp,
		Label:         w.Message.Label,
		Text:          w.Message.Text,
		Tail:          w.Message.Tail,
		Frequency:     w.Message.Frequency,
		FlightNumber:  strings.TrimSpace(w.Message.Flight),
		BlockID:       w.Message.BlockID,
		LinkDirection: w.Message.LinkDirection,
		MessageNumber: strings.TrimSpace(w.Message.MessageNumber),
		BlockEnd:      w.Message.BlockEnd,
		FromHex:       strings.ToUpper(strings.TrimSpace(w.Message.FromHex)),
		ToHex:         strings.ToUpper(strings.TrimSpace(w.Message.ToHex)),
		Airframe:      w.Airframe,
		Flight:        w.Flight,
		Station:       w.Station,
	}

	// The tail is only what was transmitted: an empty tail is not filled in
	// from Airframes' airframe record.
	return msg
}
