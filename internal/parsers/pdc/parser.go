// Package pdc parses Pre-Departure Clearance messages.
package pdc

import (
	"strings"
	"sync"

	"acars_parser/internal/acars"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// Result represents a parsed Pre-Departure Clearance.
type Result struct {
	MsgID           int64    `json:"message_id"`
	Timestamp       string   `json:"timestamp"`
	FlightNumber    string   `json:"flight_number,omitempty"`
	Tail            string   `json:"tail,omitempty"`
	AircraftICAO    string   `json:"aircraft_icao,omitempty"`
	Origin          string   `json:"origin,omitempty"`
	Destination     string   `json:"destination,omitempty"`
	DepartureTime   string   `json:"departure_time,omitempty"`
	Runway          string   `json:"runway,omitempty"`
	SID             string   `json:"sid,omitempty"`
	Route           string   `json:"route,omitempty"`
	RouteWaypoints  []string `json:"route_waypoints,omitempty"`
	Squawk          string   `json:"squawk,omitempty"`
	DepartureFreq   string   `json:"departure_freq,omitempty"`
	InitialAltitude string   `json:"initial_altitude,omitempty"`
	FlightLevel     string   `json:"flight_level,omitempty"`
	AircraftType    string   `json:"aircraft_type,omitempty"`
	ATIS            string   `json:"atis,omitempty"`
	PDCFormat       string   `json:"pdc_format,omitempty"`
	RawText         string   `json:"raw_text,omitempty"`
	ParseConfidence float64  `json:"parse_confidence"`
}

func (r *Result) Type() string     { return "pdc" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses Pre-Departure Clearance messages.
type Parser struct {
	IncludeRawText bool
}

// Grok compiler singleton - compiled once and reused.
var (
	grokCompiler *Compiler
	grokOnce     sync.Once
	grokErr      error
)

// getCompiler returns the singleton grok compiler.
func getCompiler() (*Compiler, error) {
	grokOnce.Do(func() {
		grokCompiler = NewCompiler()
		grokErr = grokCompiler.Compile()
	})
	return grokCompiler, grokErr
}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "pdc" }
func (p *Parser) Labels() []string { return nil } // Content-based, checks all labels.
func (p *Parser) Priority() int    { return 500 } // Run after label-specific parsers.

func (p *Parser) QuickCheck(text string) bool {
	upper := strings.ToUpper(text)

	// Reject "failed" PDC messages that indicate no clearance is available.
	// These should not be parsed as valid PDCs.
	if strings.Contains(upper, "NO DEPARTURE CLEARANCE") ||
		strings.Contains(upper, "NO PDC ON FILE") ||
		strings.Contains(upper, "NO PDC AVAILABLE") ||
		strings.Contains(upper, "PDC NOT AVAILABLE") {
		return false
	}

	// Require "PDC" as a standalone term - not embedded in route strings like "KPDXKMSPDC311225".
	// Check for PDC at word boundaries or preceded by common prefixes (space, newline, tab).
	if strings.Contains(upper, " PDC") || strings.Contains(upper, "\nPDC") ||
		strings.Contains(upper, "\tPDC") || strings.HasPrefix(upper, "PDC") ||
		strings.Contains(upper, "/PDC") || strings.Contains(upper, "~1PDC") {
		return true
	}

	// Also match APCDC format (e.g., "1APCDC").
	if strings.Contains(upper, "APCDC") {
		return true
	}

	// Match PRE-DEPARTURE CLEARANCE (used by some carriers).
	if strings.Contains(upper, "PRE-DEPARTURE CLEARANCE") ||
		strings.Contains(upper, "PREDEPARTURE CLEARANCE") {
		return true
	}

	return false
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
	}

	if p.IncludeRawText {
		result.RawText = msg.Text
	}

	// Get tail from message or airframe.
	result.Tail = msg.Tail
	if result.Tail == "" && msg.Airframe != nil {
		result.Tail = msg.Airframe.Tail
	}

	// Get ICAO hex from airframe.
	if msg.Airframe != nil {
		result.AircraftICAO = msg.Airframe.ICAO
	}

	// Try grok-based parsing first for format-specific extraction.
	compiler, err := getCompiler()
	if err == nil {
		if grokResult := compiler.Parse(msg.Text); grokResult != nil {
			// Use grok results as primary source.
			result.PDCFormat = grokResult.FormatName
			result.FlightNumber = grokResult.FlightNumber
			result.Origin = grokResult.Origin
			result.Destination = grokResult.Destination
			result.Runway = grokResult.Runway
			result.SID = grokResult.SID
			result.Route = grokResult.Route
			result.Squawk = grokResult.Squawk
			result.AircraftType = grokResult.Aircraft
			result.DepartureFreq = grokResult.Frequency
			result.ATIS = grokResult.ATIS
			if grokResult.Altitude != "" {
				result.InitialAltitude = grokResult.Altitude
			}
			if grokResult.DepartureTime != "" {
				result.DepartureTime = grokResult.DepartureTime
			}
		}
	}

	// Use ACARS envelope flight number only as a fallback if not parsed from PDC text.
	if result.FlightNumber == "" && msg.Flight != nil && msg.Flight.Flight != "" {
		result.FlightNumber = msg.Flight.Flight
	}

	// Fallback extraction for fields not captured by grok.
	tokens := strings.Fields(strings.ToUpper(msg.Text))

	if result.FlightNumber == "" {
		result.FlightNumber = patterns.ExtractFlightNumber(msg.Text, tokens)
	}

	if result.Origin == "" || result.Destination == "" {
		origin, dest := patterns.ExtractAirports(msg.Text, tokens)
		if result.Origin == "" {
			result.Origin = origin
		}
		if result.Destination == "" {
			result.Destination = dest
		}
	}

	// If origin still not found, try station's nearest airport.
	if result.Origin == "" && msg.Station != nil {
		result.Origin = msg.Station.NearestAirportIcao
	}

	if result.Runway == "" {
		result.Runway = patterns.ExtractRunway(msg.Text)
	}
	if result.SID == "" {
		result.SID = patterns.ExtractSID(msg.Text)
	}
	if result.Squawk == "" {
		result.Squawk = patterns.ExtractSquawk(msg.Text)
	}
	if result.DepartureFreq == "" {
		result.DepartureFreq = patterns.ExtractFrequency(msg.Text)
	}
	if result.InitialAltitude == "" || result.FlightLevel == "" {
		alt, fl := patterns.ExtractAltitude(msg.Text)
		if result.InitialAltitude == "" {
			result.InitialAltitude = alt
		}
		if result.FlightLevel == "" {
			result.FlightLevel = fl
		}
	}
	if result.AircraftType == "" {
		result.AircraftType = patterns.ExtractAircraftType(msg.Text)
	}
	if result.ATIS == "" {
		result.ATIS = patterns.ExtractATIS(msg.Text)
	}

	// Extract route waypoints from structured text.
	if len(result.RouteWaypoints) == 0 {
		result.RouteWaypoints = ExtractRouteWaypoints(msg.Text)
	}

	// Extract departure time.
	if result.DepartureTime == "" {
		result.DepartureTime = ExtractDepartureTime(msg.Text)
	}

	// Calculate confidence.
	result.ParseConfidence = calculateConfidence(result)

	return result
}

func calculateConfidence(pdc *Result) float64 {
	score := 0.0
	maxScore := 11.0

	// Core fields worth more.
	if pdc.FlightNumber != "" {
		score += 2
	}
	if pdc.Origin != "" {
		score += 2
	}
	if pdc.Destination != "" {
		score += 2
	}

	// Supporting fields.
	if pdc.Runway != "" {
		score += 1
	}
	if pdc.SID != "" {
		score += 1
	}
	if pdc.Squawk != "" {
		score += 1
	}
	if pdc.DepartureFreq != "" {
		score += 0.5
	}
	if pdc.AircraftType != "" {
		score += 0.5
	}

	// Bonus for grok format match (indicates high-confidence structured parse).
	if pdc.PDCFormat != "" {
		score += 1
	}

	return score / maxScore
}