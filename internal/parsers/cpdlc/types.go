package cpdlc

import "fmt"

// MessageDirection indicates whether the message is uplink (ground to air) or downlink (air to ground).
type MessageDirection int

const (
	// DirectionUnknown indicates the direction could not be determined.
	DirectionUnknown MessageDirection = iota
	// DirectionUplink is a ground-to-air message from ATC to the aircraft.
	DirectionUplink
	// DirectionDownlink is an air-to-ground message from the aircraft to ATC.
	DirectionDownlink
)

func (d MessageDirection) String() string {
	switch d {
	case DirectionUplink:
		return "uplink"
	case DirectionDownlink:
		return "downlink"
	default:
		return "unknown"
	}
}

// MessageHeader contains the CPDLC message header fields.
type MessageHeader struct {
	MsgID     int   `json:"msg_id"`              // Message identification number.
	MsgRef    *int  `json:"msg_ref,omitempty"`   // Reference number (optional).
	Timestamp *Time `json:"timestamp,omitempty"` // Timestamp (optional).
}

// Time represents a FANS timestamp (hours, minutes).
type Time struct {
	Hours   int `json:"hours"`
	Minutes int `json:"minutes"`
}

func (t *Time) String() string {
	if t == nil {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", t.Hours, t.Minutes)
}

// Altitude represents an altitude value with its type.
type Altitude struct {
	Type  string `json:"type"`  // "flight_level", "feet", "meters", etc.
	Value int    `json:"value"` // The altitude value.
}

func (a *Altitude) String() string {
	if a == nil {
		return ""
	}
	switch a.Type {
	case "flight_level":
		return fmt.Sprintf("FL%d", a.Value)
	case "flight_level_metric":
		return fmt.Sprintf("FL%dm", a.Value*10) // Value is in 10s of metres.
	case "feet":
		return fmt.Sprintf("%d ft", a.Value)
	case "meters":
		return fmt.Sprintf("%d m", a.Value)
	default:
		return fmt.Sprintf("%d %s", a.Value, a.Type)
	}
}

// Speed represents a speed value with its type.
type Speed struct {
	Type  string `json:"type"`  // "knots", "mach", etc.
	Value int    `json:"value"` // The speed value (mach is scaled by 1000).
}

func (s *Speed) String() string {
	if s == nil {
		return ""
	}
	switch s.Type {
	case "mach":
		return fmt.Sprintf("M.%02d", s.Value) // Value is mach * 100.
	case "knots":
		return fmt.Sprintf("%d kt", s.Value)
	case "kph":
		return fmt.Sprintf("%d km/h", s.Value)
	default:
		return fmt.Sprintf("%d %s", s.Value, s.Type)
	}
}

// Position represents a geographic position.
type Position struct {
	Type         string   `json:"type"`                   // "latlon", "fix", "navaid", "place_bearing_distance", etc.
	Latitude     *float64 `json:"latitude,omitempty"`     // Decimal degrees.
	Longitude    *float64 `json:"longitude,omitempty"`    // Decimal degrees.
	Name         string   `json:"name,omitempty"`         // Fix/navaid name.
	Bearing      *int     `json:"bearing,omitempty"`      // Bearing in degrees (for place_bearing_distance).
	Distance     *int     `json:"distance,omitempty"`     // Distance value (for place_bearing_distance).
	DistanceUnit string   `json:"distance_unit,omitempty"` // "nm" or "km" (for place_bearing_distance).
}

func (p *Position) String() string {
	if p == nil {
		return ""
	}
	if p.Type == "place_bearing_distance" && p.Name != "" && p.Bearing != nil && p.Distance != nil {
		return fmt.Sprintf("%s %03d/%d%s", p.Name, *p.Bearing, *p.Distance, p.DistanceUnit)
	}
	if p.Name != "" {
		return p.Name
	}
	if p.Latitude != nil && p.Longitude != nil {
		return fmt.Sprintf("%.4f,%.4f", *p.Latitude, *p.Longitude)
	}
	return ""
}

// PlaceBearingDistance represents a position defined by fix, bearing, and distance.
type PlaceBearingDistance struct {
	FixName      string   `json:"fix_name"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	Bearing      *int     `json:"bearing"`       // Degrees 1-360.
	Distance     *int     `json:"distance"`      // Distance value.
	DistanceUnit string   `json:"distance_unit"` // "nm" or "km".
	Magnetic     bool     `json:"magnetic"`      // True if bearing is magnetic.
}

// RouteClearance represents a route clearance with departure/arrival information.
type RouteClearance struct {
	AirportDeparture    string         `json:"airport_departure,omitempty"`
	AirportDestination  string         `json:"airport_destination,omitempty"`
	RunwayDeparture     *Runway        `json:"runway_departure,omitempty"`
	ProcedureDeparture  *ProcedureName `json:"procedure_departure,omitempty"`
	RunwayArrival       *Runway        `json:"runway_arrival,omitempty"`
	ProcedureApproach   *ProcedureName `json:"procedure_approach,omitempty"`
	ProcedureArrival    *ProcedureName `json:"procedure_arrival,omitempty"`
	AirwayIntercept     string         `json:"airway_intercept,omitempty"`
	RouteInformation    []string       `json:"route_information,omitempty"`
	RouteInfoAdditional string         `json:"route_info_additional,omitempty"`
}

func (r *RouteClearance) String() string {
	if r == nil {
		return ""
	}
	parts := []string{}
	if r.AirportDeparture != "" {
		parts = append(parts, "DEP:"+r.AirportDeparture)
	}
	if r.AirportDestination != "" {
		parts = append(parts, "DEST:"+r.AirportDestination)
	}
	if r.RunwayDeparture != nil {
		parts = append(parts, "RWY:"+r.RunwayDeparture.String())
	}
	if r.ProcedureDeparture != nil {
		parts = append(parts, "SID:"+r.ProcedureDeparture.String())
	}
	if r.AirwayIntercept != "" {
		parts = append(parts, "AWY:"+r.AirwayIntercept)
	}
	if len(parts) == 0 {
		return "(route clearance)"
	}
	return fmt.Sprintf("%v", parts)
}

// Runway represents a runway designation.
type Runway struct {
	Direction     int    `json:"direction"`     // 1-36.
	Configuration string `json:"configuration"` // "left", "right", "center", "none".
}

func (r *Runway) String() string {
	if r == nil {
		return ""
	}
	dir := fmt.Sprintf("%02d", r.Direction)
	switch r.Configuration {
	case "left":
		return dir + "L"
	case "right":
		return dir + "R"
	case "center":
		return dir + "C"
	default:
		return dir
	}
}

// ProcedureName represents a procedure (SID/STAR/approach).
type ProcedureName struct {
	Type       string `json:"type"`                 // "arrival", "approach", "departure".
	Name       string `json:"name"`                 // Procedure name.
	Transition string `json:"transition,omitempty"` // Optional transition.
}

func (p *ProcedureName) String() string {
	if p == nil {
		return ""
	}
	if p.Transition != "" {
		return p.Name + "." + p.Transition
	}
	return p.Name
}

// Frequency represents a radio frequency.
type Frequency struct {
	Type  string `json:"type"`  // "vhf", "uhf", "hf", "satcom".
	Value int    `json:"value"` // Frequency value (encoding depends on type).
}

func (f *Frequency) String() string {
	if f == nil {
		return ""
	}
	switch f.Type {
	case "vhf":
		// Value is in kHz, display as MHz.
		return fmt.Sprintf("%.3f MHz", float64(f.Value)/1000.0)
	case "uhf":
		return fmt.Sprintf("%.3f MHz", float64(f.Value)/1000.0)
	case "hf":
		return fmt.Sprintf("%d kHz", f.Value)
	case "satcom":
		return fmt.Sprintf("SATCOM ch %d", f.Value)
	default:
		return fmt.Sprintf("%d", f.Value)
	}
}

// Degrees represents a heading or track value.
type Degrees struct {
	Magnetic bool `json:"magnetic,omitempty"` // True if magnetic, false if true.
	Value    int  `json:"value"`              // Degrees 0-359.
}

func (d *Degrees) String() string {
	if d == nil {
		return ""
	}
	suffix := "T"
	if d.Magnetic {
		suffix = "M"
	}
	return fmt.Sprintf("%03d%s", d.Value, suffix)
}

// DistanceOffset represents a lateral offset from route.
type DistanceOffset struct {
	Distance  int    `json:"distance"`  // Distance value.
	Unit      string `json:"unit"`      // "nm" or "km".
	Direction string `json:"direction"` // "left" or "right".
}

func (d *DistanceOffset) String() string {
	if d == nil {
		return ""
	}
	return fmt.Sprintf("%d %s %s", d.Distance, d.Unit, d.Direction)
}

// BeaconCode represents a transponder code.
type BeaconCode struct {
	Code string `json:"code"` // 4-digit octal code.
}

func (b *BeaconCode) String() string {
	if b == nil {
		return ""
	}
	return b.Code
}

// FreeText represents free-form text.
type FreeText struct {
	Text string `json:"text"`
}

// ErrorInfo represents CPDLC error information.
type ErrorInfo struct {
	Code int    `json:"code"`
	Desc string `json:"description,omitempty"`
}

// VerticalRate represents a climb/descent rate.
type VerticalRate struct {
	Value int `json:"value"` // ft/min.
}

func (v *VerticalRate) String() string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%d ft/min", v.Value)
}
