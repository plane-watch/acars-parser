// Package acars provides ACARS message types and structures.
package acars

import (
	"encoding/json"
	"strconv"
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

	// Direction indicators from the transport layer.
	BlockID       string `json:"block_id,omitempty"`       // ACARS block ID ('0'-'9' = downlink, 'A'-'X' = uplink).
	LinkDirection string `json:"link_direction,omitempty"` // Explicit direction: "uplink" or "downlink".

	// These may be present in the message itself (old format) or at wrapper level (NATS)
	Airframe *Airframe `json:"airframe,omitempty"`
	Flight   *Flight   `json:"flight,omitempty"`
	Station  *Station  `json:"station,omitempty"`
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
		BlockID:       w.Message.BlockID,
		LinkDirection: w.Message.LinkDirection,
		Airframe:      w.Airframe,
		Flight:        w.Flight,
		Station:       w.Station,
	}

	// Use tail from airframe if not in message
	if msg.Tail == "" && w.Airframe != nil {
		msg.Tail = w.Airframe.Tail
	}

	return msg
}
