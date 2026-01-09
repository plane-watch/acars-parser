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
	Type         string   `json:"type"`                    // "latlon", "fix", "navaid", "place_bearing_distance", etc.
	Latitude     *float64 `json:"latitude,omitempty"`      // Decimal degrees.
	Longitude    *float64 `json:"longitude,omitempty"`     // Decimal degrees.
	Name         string   `json:"name,omitempty"`          // Fix/navaid name.
	Bearing      *int     `json:"bearing,omitempty"`       // Bearing in degrees (for place_bearing_distance).
	Distance     *int     `json:"distance,omitempty"`      // Distance value (for place_bearing_distance).
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
	Type  string  `json:"type"`  // "vhf", "uhf", "hf", "satcom".
	Value float64 `json:"value"` // For VHF/UHF value is MHz; for HF value is kHz; for satcom channel it's an integer channel number.
}

func (f *Frequency) String() string {
	if f == nil {
		return ""
	}
	switch f.Type {
	case "vhf":
		return fmt.Sprintf("%.3f MHz", f.Value)
	case "uhf":
		return fmt.Sprintf("%.3f MHz", f.Value)
	case "hf":
		// HF in kHz
		return fmt.Sprintf("%.0f kHz", f.Value)
	case "satcom":
		return fmt.Sprintf("SATCOM ch %.0f", f.Value)
	default:
		return fmt.Sprintf("%v", f.Value)
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

// Temperature represents an air temperature.
type Temperature struct {
	Type  string  `json:"type"`  // "C" or "F"
	Value float64 `json:"value"` // degrees
}

func (t *Temperature) String() string {
	if t == nil {
		return ""
	}
	unit := t.Type
	if unit == "" {
		unit = "C"
	}
	// Keep one decimal max, but avoid trailing .0 for integer-ish values.
	if t.Value == float64(int(t.Value)) {
		return fmt.Sprintf("%d %s", int(t.Value), unit)
	}
	return fmt.Sprintf("%.1f %s", t.Value, unit)
}

// WindSpeed represents wind speed.
type WindSpeed struct {
	Type  string `json:"type"`  // "kts" or "kmh"
	Value int    `json:"value"` // speed
}

func (w *WindSpeed) String() string {
	if w == nil {
		return ""
	}
	suffix := w.Type
	if suffix == "" {
		suffix = "kts"
	}
	return fmt.Sprintf("%d %s", w.Value, suffix)
}

// Winds represents wind direction and speed.
type Winds struct {
	Direction int        `json:"direction"` // degrees
	Speed     *WindSpeed `json:"speed,omitempty"`
}

func (w *Winds) String() string {
	if w == nil {
		return ""
	}
	if w.Speed != nil {
		return fmt.Sprintf("%d°/%s", w.Direction, w.Speed.String())
	}
	return fmt.Sprintf("%d°", w.Direction)
}

// PositionReport represents a downlink POSITION REPORT (dM48).
// Field set matches what is commonly seen in FANS-1/A position reports.
type PositionReport struct {
	PosCurrent       *Position    `json:"pos_current,omitempty"`
	TimeAtPosCurrent *Time        `json:"time_at_pos_current,omitempty"`
	Alt              *Altitude    `json:"alt,omitempty"`
	NextFix          *Position    `json:"next_fix,omitempty"`
	EtaAtFixNext     *Time        `json:"eta_at_fix_next,omitempty"`
	NextNextFix      *Position    `json:"next_next_fix,omitempty"`
	EtaAtDest        *Time        `json:"eta_at_dest,omitempty"`
	Temp             *Temperature `json:"temp,omitempty"`
	Winds            *Winds       `json:"winds,omitempty"`
	Speed            *Speed       `json:"speed,omitempty"`
	ReportedWptPos   *Position    `json:"reported_wpt_pos,omitempty"`
	ReportedWptTime  *Time        `json:"reported_wpt_time,omitempty"`
	ReportedWptAlt   *Altitude    `json:"reported_wpt_alt,omitempty"`
}

func (p *PositionReport) String() string {
	if p == nil {
		return ""
	}
	// Compact summary for label substitution.
	parts := []string{}
	if p.PosCurrent != nil {
		parts = append(parts, p.PosCurrent.String())
	}
	if p.TimeAtPosCurrent != nil {
		parts = append(parts, p.TimeAtPosCurrent.String())
	}
	if p.Alt != nil {
		parts = append(parts, p.Alt.String())
	}
	if p.NextFix != nil && p.EtaAtFixNext != nil {
		parts = append(parts, fmt.Sprintf("next %s %s", p.NextFix.String(), p.EtaAtFixNext.String()))
	} else if p.NextFix != nil {
		parts = append(parts, fmt.Sprintf("next %s", p.NextFix.String()))
	}
	if p.Winds != nil {
		parts = append(parts, "wind "+p.Winds.String())
	}
	if p.Speed != nil {
		parts = append(parts, "spd "+p.Speed.String())
	}
	if len(parts) == 0 {
		return "(position report)"
	}
	return fmt.Sprintf("%s", joinNonEmpty(parts, "; "))
}

func joinNonEmpty(parts []string, sep string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return ""
	}
	res := out[0]
	for i := 1; i < len(out); i++ {
		res += sep + out[i]
	}
	return res
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
