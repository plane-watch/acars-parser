package cpdlc

import (
	"fmt"
	"strings"
)

// This file defines the decoded form of a CPDLC message, which the parser
// reports as JSON. Values keep the precision the message encodes them with
// (for example, a distance in tenths of a nautical mile becomes a decimal
// number of nautical miles), and enumerations are reported by their
// identifiers in the FANS-1/A ASN.1 module (for example "eitherSide" or
// "clearanceDelivery").

// Message represents a decoded CPDLC message.
type Message struct {
	Direction MessageDirection `json:"direction"`
	Header    MessageHeader    `json:"header"`
	Elements  []MessageElement `json:"elements"`
}

// MessageElement represents a single message element (uplink or downlink).
//
// Data holds the element's parameters. An element with no parameters (for
// example uM3 ROGER) has no data. An element with one parameter holds it
// directly (for example *Altitude for uM20 CLIMB TO AND MAINTAIN
// [altitude]); an element with several holds a map from parameter names
// ("position", "altitude", "time", "unit", "frequency" and so on) to their
// values. Text is the label with each placeholder replaced by its value.
type MessageElement struct {
	ID    int         `json:"id"`             // Element ID (uM0-uM182 for uplink, dM0-dM128 for downlink).
	Label string      `json:"label"`          // Human-readable message template.
	Data  interface{} `json:"data,omitempty"` // Element-specific data.
	Text  string      `json:"text,omitempty"` // Formatted message text.
}

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
	MsgID     int        `json:"msg_id"`              // Message identification number.
	MsgRef    *int       `json:"msg_ref,omitempty"`   // Reference number (optional).
	Timestamp *Timestamp `json:"timestamp,omitempty"` // Timestamp (optional).
}

// Timestamp is the time in a message header (FANSTimestamp).
type Timestamp struct {
	Hours   int `json:"hours"`
	Minutes int `json:"minutes"`
	Seconds int `json:"seconds"`
}

func (t *Timestamp) String() string {
	if t == nil {
		return ""
	}
	return fmt.Sprintf("%02d:%02d:%02d", t.Hours, t.Minutes, t.Seconds)
}

// Time is a time of day in hours and minutes (FANSTime), which carries no
// seconds.
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

// Altitude represents an altitude (FANSAltitude). Type names the
// alternative: "flight_level" (Value in hundreds of feet),
// "flight_level_metric" (Value in tens of metres), "qnh_feet", "qfe_feet"
// and "gnss_feet" (Value in feet), or "qnh_metres", "qfe_metres" and
// "gnss_metres" (Value in metres).
type Altitude struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

func (a *Altitude) String() string {
	if a == nil {
		return ""
	}
	switch a.Type {
	case "flight_level":
		return fmt.Sprintf("FL%d", a.Value)
	case "flight_level_metric":
		// Metric flight levels are in tens of metres: 1000 is 10,000 m.
		return fmt.Sprintf("FL %d m", a.Value*10)
	case "qnh_feet", "qfe_feet", "gnss_feet":
		return fmt.Sprintf("%d ft", a.Value)
	default:
		return fmt.Sprintf("%d m", a.Value)
	}
}

// Speed represents a speed (FANSSpeed, or FANSSpeedGround in a position
// report). Type names the alternative: "indicated", "true" and "ground"
// (Value in knots), "indicated_metric", "true_metric" and "ground_metric"
// (Value in km/h), or "mach" (Value in hundredths of Mach, for both of the
// module's Mach alternatives).
type Speed struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

func (s *Speed) String() string {
	if s == nil {
		return ""
	}
	switch s.Type {
	case "mach":
		return fmt.Sprintf("M%d.%02d", s.Value/100, s.Value%100)
	case "indicated_metric", "true_metric", "ground_metric":
		return fmt.Sprintf("%d km/h", s.Value)
	default:
		return fmt.Sprintf("%d kt", s.Value)
	}
}

// Position represents a position (FANSPosition). Type is "fix", "navaid",
// "airport", "latlon" or "place_bearing_distance". Name is the fix, navaid
// or airport. Latitude and Longitude are decimal degrees (north and east
// positive); a place/bearing/distance position carries them when the
// message gives the fix's coordinates.
type Position struct {
	Type      string    `json:"type"`
	Latitude  *float64  `json:"latitude,omitempty"`
	Longitude *float64  `json:"longitude,omitempty"`
	Name      string    `json:"name,omitempty"`
	Bearing   *Degrees  `json:"bearing,omitempty"`  // Bearing from the fix (place_bearing_distance).
	Distance  *Distance `json:"distance,omitempty"` // Distance from the fix (place_bearing_distance).
}

func (p *Position) String() string {
	if p == nil {
		return ""
	}
	if p.Type == "place_bearing_distance" && p.Bearing != nil && p.Distance != nil {
		return fmt.Sprintf("%s %s %s", p.Name, p.Bearing, p.Distance)
	}
	if p.Name != "" {
		return p.Name
	}
	return formatLatLon(p.Latitude, p.Longitude)
}

// formatLatLon formats decimal degrees for the formatted text.
func formatLatLon(lat, lon *float64) string {
	if lat == nil || lon == nil {
		return ""
	}
	return fmt.Sprintf("%.4f,%.4f", *lat, *lon)
}

// Degrees represents a heading, track or bearing (FANSDegrees).
type Degrees struct {
	Magnetic bool `json:"magnetic,omitempty"` // True if magnetic, false if true north.
	Value    int  `json:"value"`              // Degrees, 1 to 360.
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

// Distance represents a distance (FANSDistance): Value is in nautical miles
// (to a tenth) when Unit is "nm", or in kilometres when Unit is "km".
type Distance struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

func (d *Distance) String() string {
	if d == nil {
		return ""
	}
	if d.Unit == "nm" {
		return fmt.Sprintf("%.1f nm", d.Value)
	}
	return fmt.Sprintf("%.0f km", d.Value)
}

// DistanceOffset represents a lateral offset from route and its direction
// (FANSDistanceOffsetDirection).
type DistanceOffset struct {
	Distance  int    `json:"distance"`  // Distance value.
	Unit      string `json:"unit"`      // "nm" or "km".
	Direction string `json:"direction"` // A FANSDirection identifier, e.g. "left" or "eitherSide".
}

func (d *DistanceOffset) String() string {
	if d == nil {
		return ""
	}
	return fmt.Sprintf("%d %s %s", d.Distance, d.Unit, enumText(d.Direction))
}

// Frequency represents a radio frequency (FANSFrequency). Type is "hf",
// "vhf" or "uhf", with Value in kHz, or "satcom", with Channel the
// twelve-character satellite channel (a telephone number).
type Frequency struct {
	Type    string `json:"type"`
	Value   int    `json:"value,omitempty"`
	Channel string `json:"channel,omitempty"`
}

func (f *Frequency) String() string {
	if f == nil {
		return ""
	}
	switch f.Type {
	case "hf":
		return fmt.Sprintf("%d kHz", f.Value)
	case "satcom":
		return "SATCOM " + strings.TrimSpace(f.Channel)
	default:
		return fmt.Sprintf("%.3f MHz", float64(f.Value)/1000.0)
	}
}

// ICAOUnitName represents an air traffic unit (FANSICAOUnitName), named
// either by its four-letter ICAO facility designation or by a facility name,
// with its function.
type ICAOUnitName struct {
	FacilityDesignation string `json:"facility_designation,omitempty"`
	FacilityName        string `json:"facility_name,omitempty"`
	FacilityFunction    string `json:"facility_function"` // A FANSICAOFacilityFunction identifier, e.g. "center".
}

func (u *ICAOUnitName) String() string {
	if u == nil {
		return ""
	}
	name := u.FacilityDesignation
	if name == "" {
		name = u.FacilityName
	}
	return name + " " + strings.ToUpper(enumText(u.FacilityFunction))
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

// ErrorInfo represents CPDLC error information (FANSErrorInformation).
// Code is the enumeration index and Desc its identifier in the module, e.g.
// "unrecognizedMsgReferenceNumber".
type ErrorInfo struct {
	Code int    `json:"code"`
	Desc string `json:"description,omitempty"`
}

// VerticalRate represents a climb or descent rate (FANSVerticalRate): Value
// is in feet per minute when Unit is "ft/min", or in metres per minute when
// Unit is "m/min".
type VerticalRate struct {
	Value int    `json:"value"`
	Unit  string `json:"unit"`
}

func (v *VerticalRate) String() string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%d %s", v.Value, v.Unit)
}

// Altimeter represents an altimeter setting (FANSAltimeter): Value is in
// inches of mercury when Unit is "inHg", or in hectopascals when Unit is
// "hPa".
type Altimeter struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

func (a *Altimeter) String() string {
	if a == nil {
		return ""
	}
	if a.Unit == "inHg" {
		return fmt.Sprintf("%.2f inHg", a.Value)
	}
	return fmt.Sprintf("%.1f hPa", a.Value)
}

// RemainingFuel represents fuel remaining as a time duration.
type RemainingFuel struct {
	Hours   int `json:"hours"`
	Minutes int `json:"minutes"`
}

func (r *RemainingFuel) String() string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("%dh%02dm", r.Hours, r.Minutes)
}

// PersonsOnBoard represents the number of people on the aircraft.
type PersonsOnBoard struct {
	Count int `json:"count"`
}

func (p *PersonsOnBoard) String() string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", p.Count)
}

// ProcedureName represents a procedure (FANSProcedureName): Type is
// "arrival", "approach" or "departure".
type ProcedureName struct {
	Type       string `json:"type"`
	Name       string `json:"name"`
	Transition string `json:"transition,omitempty"`
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

// Runway represents a runway designation (FANSRunway).
type Runway struct {
	Direction     int    `json:"direction"`     // 1-36.
	Configuration string `json:"configuration"` // "left", "right", "center" or "none" (the module's identifiers).
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

// RouteClearance represents a route clearance (FANSRouteClearance).
type RouteClearance struct {
	AirportDeparture    string                      `json:"airport_departure,omitempty"`
	AirportDestination  string                      `json:"airport_destination,omitempty"`
	RunwayDeparture     *Runway                     `json:"runway_departure,omitempty"`
	ProcedureDeparture  *ProcedureName              `json:"procedure_departure,omitempty"`
	RunwayArrival       *Runway                     `json:"runway_arrival,omitempty"`
	ProcedureApproach   *ProcedureName              `json:"procedure_approach,omitempty"`
	ProcedureArrival    *ProcedureName              `json:"procedure_arrival,omitempty"`
	AirwayIntercept     string                      `json:"airway_intercept,omitempty"`
	RouteInformation    []RouteInformation          `json:"route_information,omitempty"`
	RouteInfoAdditional *RouteInformationAdditional `json:"route_info_additional,omitempty"`
}

func (r *RouteClearance) String() string {
	if r == nil {
		return ""
	}
	var parts []string
	add := func(prefix, value string) {
		if value != "" {
			parts = append(parts, prefix+value)
		}
	}
	add("DEP ", r.AirportDeparture)
	add("RWY ", r.RunwayDeparture.String())
	add("SID ", r.ProcedureDeparture.String())
	add("AWY ", r.AirwayIntercept)
	route := make([]string, 0, len(r.RouteInformation))
	for i := range r.RouteInformation {
		route = append(route, r.RouteInformation[i].String())
	}
	add("ROUTE ", strings.Join(route, " "))
	add("STAR ", r.ProcedureArrival.String())
	add("APP ", r.ProcedureApproach.String())
	add("RWY ", r.RunwayArrival.String())
	add("DEST ", r.AirportDestination)
	if r.RouteInfoAdditional != nil {
		parts = append(parts, "(additional route information)")
	}
	return strings.Join(parts, " ")
}

// RouteInformation is one element of a route (FANSRouteInformation). Type
// names the alternative:
//   - "published_identifier": Position is the fix (type "fix"), with its
//     coordinates when the message gives them.
//   - "latitude_longitude": Position is the point (type "latlon").
//   - "place_bearing_place_bearing": PlaceBearings holds the two fixes and
//     bearings whose intersection is the point.
//   - "place_bearing_distance": Position is the point (type
//     "place_bearing_distance").
//   - "airway_identifier": Airway is the airway.
//   - "track_detail": Track is the track.
type RouteInformation struct {
	Type          string         `json:"type"`
	Position      *Position      `json:"position,omitempty"`
	PlaceBearings []PlaceBearing `json:"place_bearings,omitempty"`
	Airway        string         `json:"airway,omitempty"`
	Track         *TrackDetail   `json:"track,omitempty"`
}

func (r *RouteInformation) String() string {
	switch {
	case r.Position != nil:
		return r.Position.String()
	case len(r.PlaceBearings) == 2:
		return r.PlaceBearings[0].String() + "/" + r.PlaceBearings[1].String()
	case r.Track != nil:
		return "TRACK " + r.Track.Name
	default:
		return r.Airway
	}
}

// PlaceBearing is a fix and a bearing from it (FANSPlaceBearing).
type PlaceBearing struct {
	FixName   string   `json:"fix_name"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Bearing   *Degrees `json:"bearing"`
}

func (p *PlaceBearing) String() string {
	return p.FixName + " " + p.Bearing.String()
}

// LatLon is a point in decimal degrees (FANSLatitudeLongitude).
type LatLon struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// TrackDetail is a named track and its points (FANSTrackDetail).
type TrackDetail struct {
	Name   string   `json:"name"`
	Points []LatLon `json:"points"`
}

// RouteInformationAdditional holds the additional route constraints of a
// route clearance (FANSRouteInformationAdditional).
type RouteInformationAdditional struct {
	AlongTrackWaypoints    []AlongTrackWaypoint    `json:"along_track_waypoints,omitempty"`
	ReportingPoints        *ReportingPoints        `json:"reporting_points,omitempty"`
	InterceptCourses       []InterceptCourse       `json:"intercept_courses,omitempty"`
	HoldsAtWaypoint        []HoldAtWaypoint        `json:"holds_at_waypoint,omitempty"`
	WaypointSpeedAltitudes []WaypointSpeedAltitude `json:"waypoint_speed_altitudes,omitempty"`
	RequiredArrivalTimes   []RequiredArrivalTime   `json:"required_arrival_times,omitempty"`
}

// ToleranceAltitude is an altitude with a tolerance (FANSATWAltitude):
// Tolerance is "at", "atorabove" or "atorbelow".
type ToleranceAltitude struct {
	Tolerance string    `json:"tolerance"`
	Altitude  *Altitude `json:"altitude"`
}

// AlongTrackWaypoint is a waypoint defined by a distance along the track
// from a position (FANSATWAlongTrackWaypoint): DistanceTolerance is "plus"
// or "minus".
type AlongTrackWaypoint struct {
	Position          *Position           `json:"position"`
	DistanceTolerance string              `json:"distance_tolerance"`
	Distance          *Distance           `json:"distance"`
	Speed             *Speed              `json:"speed,omitempty"`
	Altitudes         []ToleranceAltitude `json:"altitudes,omitempty"`
}

// ReportingPoints asks for reports at every latitude or longitude
// (FANSReportingPoints). Type is "latitude" or "longitude"; Direction is
// the hemisphere ("north", "south", "east" or "west").
type ReportingPoints struct {
	Type            string `json:"type"`
	Direction       string `json:"direction"`
	Degrees         int    `json:"degrees"`
	DegreeIncrement *int   `json:"degree_increment,omitempty"`
}

// InterceptCourse is a course to intercept from a point
// (FANSInterceptCourseFrom). From has the form of a RouteInformation of type
// "published_identifier", "latitude_longitude",
// "place_bearing_place_bearing" or "place_bearing_distance".
type InterceptCourse struct {
	From    RouteInformation `json:"from"`
	Degrees *Degrees         `json:"degrees"`
}

// HoldAtWaypoint is a hold in a route clearance (FANSHoldatwaypoint).
type HoldAtWaypoint struct {
	Position  *Position          `json:"position"`
	SpeedLow  *Speed             `json:"speed_low,omitempty"`
	Altitude  *ToleranceAltitude `json:"altitude,omitempty"`
	SpeedHigh *Speed             `json:"speed_high,omitempty"`
	Direction string             `json:"direction,omitempty"`
	Degrees   *Degrees           `json:"degrees,omitempty"`
	EFCTime   *Time              `json:"efc_time,omitempty"`
	LegType   *LegType           `json:"leg_type,omitempty"`
}

// WaypointSpeedAltitude is a speed or altitude constraint at a waypoint
// (FANSWaypointSpeedAltitude).
type WaypointSpeedAltitude struct {
	Position  *Position           `json:"position"`
	Speed     *Speed              `json:"speed,omitempty"`
	Altitudes []ToleranceAltitude `json:"altitudes,omitempty"`
}

// RequiredArrivalTime is a required time of arrival at a position
// (FANSRTARequiredTimeArrival): TimeTolerance is "at", "atorafter" or
// "atorbefore", and ToleranceMinutes, when present, is the accepted error
// in minutes (to a tenth).
type RequiredArrivalTime struct {
	Position         *Position `json:"position"`
	Time             *Time     `json:"time"`
	TimeTolerance    string    `json:"time_tolerance"`
	ToleranceMinutes *float64  `json:"tolerance_minutes,omitempty"`
}

// LegType is the length of a holding pattern's inbound leg (FANSLegType):
// either a distance or a time in minutes (to a tenth).
type LegType struct {
	Distance *Distance `json:"distance,omitempty"`
	Minutes  *float64  `json:"minutes,omitempty"`
}

func (l *LegType) String() string {
	if l == nil {
		return ""
	}
	if l.Distance != nil {
		return l.Distance.String()
	}
	if l.Minutes != nil {
		return fmt.Sprintf("%.1f min", *l.Minutes)
	}
	return ""
}

// HoldClearance is the clearance of uM91 (FANSHoldClearance): hold at
// Position, maintain Altitude, inbound track Degrees, Direction turns
// ("left" or "right", or another FANSDirection identifier), and the
// optional leg type.
type HoldClearance struct {
	Position  *Position `json:"position"`
	Altitude  *Altitude `json:"altitude"`
	Degrees   *Degrees  `json:"degrees"`
	Direction string    `json:"direction"`
	LegType   *LegType  `json:"leg_type,omitempty"`
}

// EquipmentCode is the aircraft's equipment in a pre-departure clearance
// (FANSAircraftEquipmentCode). COMNAVStatus and SSR hold the module's
// identifiers, e.g. "ggnss" and "stransponderModeSPAID".
type EquipmentCode struct {
	COMNAVApproachAvailable bool     `json:"comnav_approach_available"`
	COMNAVStatus            []string `json:"comnav_status,omitempty"`
	SSR                     string   `json:"ssr"`
}

// PredepartureClearance is the clearance of uM73
// (FANSPredepartureClearance).
type PredepartureClearance struct {
	FlightID            string          `json:"flight_id"`
	AircraftType        string          `json:"aircraft_type,omitempty"`
	EquipmentCode       *EquipmentCode  `json:"equipment_code,omitempty"`
	DepartureTime       *Time           `json:"departure_time"`
	RouteClearance      *RouteClearance `json:"route_clearance"`
	AltitudeRestriction *Altitude       `json:"altitude_restriction,omitempty"`
	DepartureFrequency  *Frequency      `json:"departure_frequency"`
	BeaconCode          *BeaconCode     `json:"beacon_code"`
	Revision            int             `json:"revision"`
}

func (p *PredepartureClearance) String() string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%s %s DEP %s SQUAWK %s", p.FlightID, p.RouteClearance, p.DepartureTime, p.BeaconCode)
}

// Temperature is an air temperature in a position report (FANSTemperature):
// Unit is "C" or "F".
type Temperature struct {
	Value int    `json:"value"`
	Unit  string `json:"unit"`
}

// Wind represents wind information (FANSWinds).
type Wind struct {
	Direction int    `json:"direction"` // Degrees, 1 to 360.
	Speed     int    `json:"speed"`     // Wind speed value.
	Unit      string `json:"unit"`      // "kt" or "km/h".
}

func (w *Wind) String() string {
	if w == nil {
		return ""
	}
	return fmt.Sprintf("%03d/%d%s", w.Direction, w.Speed, w.Unit)
}

// VerticalChange is the vertical direction and rate in a position report
// (FANSVerticalChange): Direction is "up" or "down".
type VerticalChange struct {
	Direction string        `json:"direction"`
	Rate      *VerticalRate `json:"rate"`
}

// PositionReport represents a position report (dM48, FANSPositionReport),
// with the components in the module's order.
type PositionReport struct {
	Position                 *Position       `json:"position"`
	Time                     *Time           `json:"time"`
	Altitude                 *Altitude       `json:"altitude"`
	FixNext                  *Position       `json:"fix_next,omitempty"`
	FixNextETA               *Time           `json:"fix_next_eta,omitempty"`
	FixNextPlusOne           *Position       `json:"fix_next_plus_one,omitempty"`
	DestinationETA           *Time           `json:"destination_eta,omitempty"`
	RemainingFuel            *RemainingFuel  `json:"remaining_fuel,omitempty"`
	Temperature              *Temperature    `json:"temperature,omitempty"`
	Wind                     *Wind           `json:"wind,omitempty"`
	Turbulence               string          `json:"turbulence,omitempty"`
	Icing                    string          `json:"icing,omitempty"`
	Speed                    *Speed          `json:"speed,omitempty"`
	GroundSpeed              *Speed          `json:"ground_speed,omitempty"`
	VerticalChange           *VerticalChange `json:"vertical_change,omitempty"`
	TrackAngle               *Degrees        `json:"track_angle,omitempty"`
	TrueHeading              *Degrees        `json:"true_heading,omitempty"`
	Distance                 *Distance       `json:"distance,omitempty"`
	SupplementaryInformation string          `json:"supplementary_information,omitempty"`
	ReportedWaypointPosition *Position       `json:"reported_waypoint_position,omitempty"`
	ReportedWaypointTime     *Time           `json:"reported_waypoint_time,omitempty"`
	ReportedWaypointAltitude *Altitude       `json:"reported_waypoint_altitude,omitempty"`
}

func (pr *PositionReport) String() string {
	if pr == nil {
		return ""
	}
	result := pr.Position.String() + " at " + pr.Time.String() + " " + pr.Altitude.String()
	if pr.FixNext != nil {
		result += " next " + pr.FixNext.String()
		if pr.FixNextETA != nil {
			result += " at " + pr.FixNextETA.String()
		}
	}
	return result
}

// enumText converts an enumeration identifier from the module ("eitherSide",
// "clearanceDelivery") into words for the formatted text ("either side",
// "clearance delivery").
func enumText(identifier string) string {
	var b strings.Builder
	for i, r := range identifier {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte(' ')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}
