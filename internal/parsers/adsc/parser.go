// Package adsc parses ADS-C (Automatic Dependent Surveillance - Contract) messages.
// Based on libacars ADS-C decoder implementation.
package adsc

import (
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/airports"
	"acars_parser/internal/crc"
	"acars_parser/internal/patterns"
	"acars_parser/internal/registry"
)

// MeteoData contains meteorological information.
type MeteoData struct {
	WindSpeed      float64 `json:"wind_speed_kts"`     // Wind speed in knots.
	WindDirection  float64 `json:"wind_direction_deg"` // True wind direction in degrees.
	WindDirInvalid bool    `json:"wind_dir_invalid"`   // True if wind direction is invalid.
	Temperature    float64 `json:"temperature_c"`      // Temperature in Celsius.
}

// EarthRef contains earth-referenced velocity data (ground track).
type EarthRef struct {
	Track        float64 `json:"track_deg"`        // True track in degrees.
	TrackInvalid bool    `json:"track_invalid"`    // True if track is invalid.
	GroundSpeed  float64 `json:"ground_speed_kts"` // Ground speed in knots.
	VertSpeed    int     `json:"vert_speed_fpm"`   // Vertical speed in ft/min.
}

// AirRef contains air-referenced velocity data (heading/mach).
type AirRef struct {
	Heading        float64 `json:"heading_deg"`     // True heading in degrees.
	HeadingInvalid bool    `json:"heading_invalid"` // True if heading is invalid.
	Mach           float64 `json:"mach"`            // Mach number.
	VertSpeed      int     `json:"vert_speed_fpm"`  // Vertical speed in ft/min.
}

// Waypoint contains predicted waypoint data.
type Waypoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  int     `json:"altitude_ft"`
	ETA       int     `json:"eta_seconds,omitempty"` // ETA in seconds.
}

// PredictedRoute contains the predicted route waypoints.
type PredictedRoute struct {
	NextWaypoint     *Waypoint `json:"next_waypoint,omitempty"`
	NextNextWaypoint *Waypoint `json:"next_next_waypoint,omitempty"`
}

// ContractRequest contains uplink contract request data.
type ContractRequest struct {
	ContractNum  int `json:"contract_num"`
	IntervalSecs int `json:"interval_secs,omitempty"`
}

// Result represents a decoded ADS-C message (Label B6 or A6).
type Result struct {
	MsgID             int64            `json:"message_id"`
	Timestamp         string           `json:"timestamp"`
	Direction         string           `json:"direction,omitempty"` // "uplink" or "downlink"
	FlightID          string           `json:"flight_id,omitempty"`
	Registration      string           `json:"registration"`
	GroundStation     string           `json:"ground_station,omitempty"`
	GroundStationName string           `json:"ground_station_name,omitempty"`
	MessageType       string           `json:"message_type"`
	PayloadBytes      int              `json:"payload_bytes"` // Length of decoded payload.
	Latitude          float64          `json:"latitude,omitempty"`
	Longitude         float64          `json:"longitude,omitempty"`
	Altitude          int              `json:"altitude,omitempty"`
	ContractRequest   *ContractRequest `json:"contract_request,omitempty"`

	// Enhanced fields from tag parsing.
	ReportTime     float64         `json:"report_time_sec,omitempty"` // Seconds past the hour.
	Accuracy       int             `json:"accuracy,omitempty"`        // Position accuracy (0-7).
	NAVRedundancy  bool            `json:"nav_redundancy,omitempty"`  // NAV unit redundancy OK.
	TCASAvailable  bool            `json:"tcas_available,omitempty"`  // TCAS available.
	AirframeID     string          `json:"airframe_id,omitempty"`     // ICAO hex address.
	ADSCFlightID   string          `json:"adsc_flight_id,omitempty"`  // Flight ID from tag 12.
	Meteo          *MeteoData      `json:"meteo,omitempty"`           // Meteorological data.
	EarthRef       *EarthRef       `json:"earth_ref,omitempty"`       // Earth reference data.
	AirRef         *AirRef         `json:"air_ref,omitempty"`         // Air reference data.
	PredictedRoute *PredictedRoute `json:"predicted_route,omitempty"` // Predicted route.
	RawHex         string          `json:"raw_hex,omitempty"`
}

func (r *Result) Type() string     { return "adsc" }
func (r *Result) MessageID() int64 { return r.MsgID }

// Parser parses ADS-C B6 messages.
type Parser struct{}

func init() {
	registry.Register(&Parser{})
}

func (p *Parser) Name() string     { return "adsc" }
func (p *Parser) Labels() []string { return []string{"B6", "A6"} }
func (p *Parser) Priority() int    { return 10 }

func (p *Parser) QuickCheck(text string) bool {
	return strings.Contains(text, ".ADS.")
}

func (p *Parser) Parse(msg *acars.Message) registry.Result {
	if msg.Text == "" {
		return nil
	}

	text := strings.TrimSpace(msg.Text)

	result := &Result{
		MsgID:     int64(msg.ID),
		Timestamp: msg.Timestamp,
	}

	// Find .ADS. marker.
	adsIdx := strings.Index(text, ".ADS.")
	if adsIdx < 0 {
		return nil
	}

	// Determine message direction from label.
	switch msg.Label {
	case "A6":
		result.Direction = "uplink"
	case "B6":
		result.Direction = "downlink"
	}

	// Parse prefix: [link][flight]/[station].
	prefix := text[:adsIdx]
	if idx := strings.LastIndex(prefix, "/"); idx >= 0 {
		result.GroundStation = prefix[idx+1:]
		result.GroundStationName = airports.GetGroundStationName(result.GroundStation)
		prefix = prefix[:idx]
	}
	// Extract flight ID (format like L46AKL0628 or J77ABA024R).
	if m := patterns.ADSCFlightPattern.FindStringSubmatch(prefix); len(m) >= 2 {
		result.FlightID = m[1]
	}

	// Extract the raw text prefix for CRC verification.
	// Format: IMI (3 chars) + separator/registration (7 chars) = 10 chars total.
	prefixStart := adsIdx + 1 // Skip the dot before "ADS".
	if len(text) < prefixStart+10 {
		return nil
	}

	textPrefix := text[prefixStart : prefixStart+10]
	hexPayload := text[prefixStart+10:]

	// Validate hex payload.
	if len(hexPayload) < 4 || len(hexPayload)%2 != 0 {
		return nil
	}
	data, err := hex.DecodeString(hexPayload)
	if err != nil || len(data) < 3 {
		return nil
	}

	// Verify CRC using the raw 10-char text prefix.
	if !crc.VerifyArincBinaryRaw(textPrefix, data) {
		return nil // CRC mismatch - reject message.
	}

	// Extract clean registration from text prefix (chars 4-10, after "ADS.").
	regPart := textPrefix[3:]                // Skip "ADS".
	regPart = strings.TrimLeft(regPart, ".") // Strip leading dots.
	result.Registration = regPart
	result.RawHex = hexPayload

	// Strip CRC from payload before decoding.
	data = data[:len(data)-2]

	// Handle uplink (Label A6) vs downlink (Label B6) differently.
	if msg.Label == "A6" {
		decodeUplinkPayload(result, data)
	} else {
		decodePayloadData(result, data)
	}

	return result
}

// decodeUplinkPayload decodes uplink (Label A6) contract request data.
// Format: byte[0]=header(0x07), byte[1]=contract_num, byte[2]=interval_modulus, byte[3+]=additional_data
// Interval is calculated as: modulus * 64 seconds
func decodeUplinkPayload(result *Result, data []byte) {
	if len(data) < 3 {
		return
	}

	result.PayloadBytes = len(data)
	result.MessageType = "uplink_contract_request"

	// Byte 0 is header (typically 0x07).
	// Byte 1 is contract number.
	// Byte 2 is interval modulus (multiply by 64 to get seconds).
	contractNum := int(data[1])
	intervalModulus := int(data[2])
	intervalSecs := intervalModulus * 64

	result.ContractRequest = &ContractRequest{
		ContractNum:  contractNum,
		IntervalSecs: intervalSecs,
	}
}

// decodePayloadData decodes the binary ADS-C payload using tag-based parsing.
// Based on libacars ADS-C decoder: https://github.com/szpajder/libacars
// Message types from ARINC 745 / EUROCAE ED-100A.
func decodePayloadData(result *Result, data []byte) {
	if len(data) < 1 {
		return
	}

	result.PayloadBytes = len(data)

	// The first byte indicates the message type/first tag.
	// Parse tags in sequence until we run out of data.
	offset := 0
	firstTag := true

	for offset < len(data) {
		tag := data[offset]
		offset++

		consumed := parseTag(result, tag, data[offset:], firstTag)
		if consumed < 0 {
			// Parsing error or unknown tag - stop processing.
			break
		}
		offset += consumed
		firstTag = false
	}
}

// parseTag parses a single ADS-C tag and returns bytes consumed, or -1 on error.
func parseTag(result *Result, tag byte, data []byte, isFirst bool) int {
	switch tag {
	// Acknowledgment.
	case 0x03:
		if isFirst {
			result.MessageType = "acknowledgment"
		}
		if len(data) < 1 {
			return -1
		}
		return 1 // Contract number.

	// Negative acknowledgment.
	case 0x04:
		if isFirst {
			result.MessageType = "nack"
		}
		if len(data) < 2 {
			return -1
		}
		return 2 // Contract number + reason.

	// Noncompliance notification.
	case 0x05:
		if isFirst {
			result.MessageType = "noncompliance"
		}
		if len(data) < 2 {
			return -1
		}
		groupCnt := int(data[1])
		return 2 + groupCnt*2 // Approximate size.

	// Cancel emergency mode.
	case 0x06:
		if isFirst {
			result.MessageType = "cancel_emergency"
		}
		return 0

	// Basic report (Tag 7) - 10 bytes.
	case 0x07:
		if isFirst {
			result.MessageType = "basic"
		}
		if len(data) < 10 {
			return -1
		}
		decodeBasicReportTag(result, data[:10])
		return 10

	// Emergency basic report (Tag 9).
	case 0x09:
		if isFirst {
			result.MessageType = "emergency"
		}
		if len(data) < 10 {
			return -1
		}
		decodeBasicReportTag(result, data[:10])
		return 10

	// Lateral deviation change event (Tag 10).
	case 0x0A:
		if isFirst {
			result.MessageType = "lateral_deviation"
		}
		if len(data) < 10 {
			return -1
		}
		decodeBasicReportTag(result, data[:10])
		return 10

	// Flight ID data (Tag 12) - 6 bytes.
	case 0x0C:
		if len(data) < 6 {
			return -1
		}
		result.ADSCFlightID = decodeFlightID(data[:6])
		return 6

	// Predicted route (Tag 13) - 17 bytes.
	case 0x0D:
		if len(data) < 17 {
			return -1
		}
		result.PredictedRoute = decodePredictedRoute(data[:17])
		return 17

	// Earth reference data (Tag 14) - 5 bytes.
	case 0x0E:
		if len(data) < 5 {
			return -1
		}
		result.EarthRef = decodeEarthRef(data[:5])
		return 5

	// Air reference data (Tag 15) - 5 bytes.
	case 0x0F:
		if len(data) < 5 {
			return -1
		}
		result.AirRef = decodeAirRef(data[:5])
		return 5

	// Meteo data (Tag 16) - 4 bytes.
	case 0x10:
		if len(data) < 4 {
			return -1
		}
		result.Meteo = decodeMeteo(data[:4])
		return 4

	// Airframe ID (Tag 17) - 3 bytes.
	case 0x11:
		if len(data) < 3 {
			return -1
		}
		result.AirframeID = fmt.Sprintf("%02X%02X%02X", data[0], data[1], data[2])
		return 3

	// Vertical rate change event (Tag 18).
	case 0x12:
		if isFirst {
			result.MessageType = "vert_rate_change"
		}
		if len(data) < 10 {
			return -1
		}
		decodeBasicReportTag(result, data[:10])
		return 10

	// Altitude range event (Tag 19).
	case 0x13:
		if isFirst {
			result.MessageType = "altitude_range"
		}
		if len(data) < 10 {
			return -1
		}
		decodeBasicReportTag(result, data[:10])
		return 10

	// Waypoint change event (Tag 20).
	case 0x14:
		if isFirst {
			result.MessageType = "waypoint_change"
		}
		if len(data) < 10 {
			return -1
		}
		decodeBasicReportTag(result, data[:10])
		return 10

	// Intermediate projection (Tag 22) - 8 bytes.
	case 0x16:
		// TODO: Implement intermediate projection parsing.
		if len(data) < 8 {
			return -1
		}
		return 8

	// Fixed projection (Tag 23) - 9 bytes.
	case 0x17:
		// TODO: Implement fixed projection parsing.
		if len(data) < 9 {
			return -1
		}
		return 9

	default:
		// Unknown tag - can't continue parsing safely.
		if isFirst {
			result.MessageType = fmt.Sprintf("unknown_%02x", tag)
		}
		return -1
	}
}

// decodeBasicReportTag decodes a 10-byte basic report tag.
// Format: lat(21) + lon(21) + alt(16) + timestamp(15) + flags(7).
func decodeBasicReportTag(result *Result, data []byte) {
	if len(data) < 10 {
		return
	}

	// Read 80 bits (10 bytes) into a bitstream.
	bits := uint64(0)
	for i := 0; i < 8; i++ {
		bits = (bits << 8) | uint64(data[i])
	}

	// Latitude: bits 0-20 (21 bits).
	latRaw := uint32((bits >> (64 - 21)) & 0x1FFFFF)
	result.Latitude = decodeCoordinate(latRaw)

	// Longitude: bits 21-41 (21 bits).
	lonRaw := uint32((bits >> (64 - 42)) & 0x1FFFFF)
	result.Longitude = decodeCoordinate(lonRaw)

	// Altitude: bits 42-57 (16 bits).
	// Need to read more bytes for this.
	altBits := uint64(0)
	for i := 0; i < 8; i++ {
		altBits = (altBits << 8) | uint64(data[i])
	}
	altRaw := uint32((altBits >> (64 - 58)) & 0xFFFF)
	result.Altitude = decodeAltitude(altRaw)

	// Timestamp: bits 58-72 (15 bits).
	// Read remaining bytes.
	var allBits [10]byte
	copy(allBits[:], data)

	// Extract timestamp from bits 58-72.
	tsBits := (uint32(data[7])<<16 | uint32(data[8])<<8 | uint32(data[9])) >> 7
	tsBits &= 0x7FFF
	result.ReportTime = float64(tsBits) * 0.125

	// Flags in last 7 bits.
	flags := data[9] & 0x7F
	result.NAVRedundancy = (flags & 0x01) != 0
	result.Accuracy = int((flags >> 1) & 0x07)
	result.TCASAvailable = (flags & 0x10) != 0

	// Validate coordinates.
	if result.Latitude < -90 || result.Latitude > 90 {
		result.Latitude = 0
	}
	if result.Longitude < -180 || result.Longitude > 180 {
		result.Longitude = 0
	}
}

// decodeCoordinate decodes a 21-bit signed coordinate value.
// Field range is -180 to 180 degrees.
// MSB weight is 90 degrees, LSB weight is 90/(2^19).
func decodeCoordinate(raw uint32) float64 {
	// Sign extend 21-bit value.
	if raw&0x100000 != 0 {
		raw |= 0xFFE00000
	}
	signed := int32(raw)
	if signed > 0x0FFFFF {
		signed = int32(int64(raw) - 0x200000)
	}

	maxVal := 180.0 - 90.0/math.Pow(2, 19)
	return maxVal * float64(signed) / float64(0xFFFFF)
}

// Per ADS-C Basic Report encoding, altitude is (altitude_ft - 20000) with 2 ft resolution.
// altitude_ft = signed(raw) * 2 + 20000.
func decodeAltitude(raw uint32) int {
	if raw&0x8000 != 0 {
		raw |= 0xFFFF0000
	}
	return int(int32(raw))*2 + 20000
}

// decodeHeading decodes a 12-bit signed heading/track value.
// Format is same as lat/lon but 12-bit with LSB weight 90/(2^10).
func decodeHeading(raw uint32) float64 {
	// Sign extend 12-bit value.
	if raw&0x800 != 0 {
		raw |= 0xFFFFF000
	}
	signed := int32(raw)

	maxVal := 180.0 - 90.0/math.Pow(2, 10)
	result := maxVal * float64(signed) / float64(0x7FF)
	if result < 0 {
		result += 360.0
	}
	return result
}

// decodeWindDir decodes a 9-bit signed wind direction value.
// Format is same as lat/lon but 9-bit with LSB weight 90/(2^7).
func decodeWindDir(raw uint32) float64 {
	// Sign extend 9-bit value.
	if raw&0x100 != 0 {
		raw |= 0xFFFFFE00
	}
	signed := int32(raw)

	maxVal := 180.0 - 90.0/math.Pow(2, 7)
	result := maxVal * float64(signed) / float64(0xFF)
	if result < 0 {
		result += 360.0
	}
	return result
}

// decodeTemperature decodes a 12-bit signed temperature value.
// Field range is -512 to 512 degrees C (but realistically -100 to +60).
func decodeTemperature(raw uint32) float64 {
	// Sign extend 12-bit value.
	if raw&0x800 != 0 {
		raw |= 0xFFFFF000
	}
	signed := int32(raw)

	maxVal := 512.0 - 256.0/math.Pow(2, 10)
	return maxVal * float64(signed) / float64(0x7FF)
}

// decodeVertSpeed decodes a 12-bit signed vertical speed value.
// Resolution is 16 ft/min.
func decodeVertSpeed(raw uint32) int {
	// Sign extend 12-bit value.
	if raw&0x800 != 0 {
		raw |= 0xFFFFF000
	}
	return int(int32(raw)) * 16
}

// decodeFlightID decodes a 6-byte ISO5-encoded flight ID (8 chars).
func decodeFlightID(data []byte) string {
	if len(data) < 6 {
		return ""
	}

	// 48 bits = 8 x 6-bit characters.
	bits := uint64(0)
	for i := 0; i < 6; i++ {
		bits = (bits << 8) | uint64(data[i])
	}

	var id [8]byte
	for i := 0; i < 8; i++ {
		// Extract 6-bit character (MSB first).
		shift := uint(48 - (i+1)*6)
		c := byte((bits >> shift) & 0x3F)

		// ISO5 alphabet:
		// 0x20 (space) when MSB = 1, bit 5 = 0
		// 0x30-0x39 (digits) when bits 5-4 = 11
		// 0x41-0x5A (letters) when bit 5 = 0
		if (c & 0x20) == 0 {
			c += 0x40 // Convert to ASCII letter range.
		}
		id[i] = c
	}

	// Trim trailing spaces.
	result := string(id[:])
	return strings.TrimRight(result, " ")
}

// decodeMeteo decodes a 4-byte meteo data tag.
// Format: wind_speed(9) + wind_dir_invalid(1) + wind_dir(9) + temp(12) = 31 bits.
func decodeMeteo(data []byte) *MeteoData {
	if len(data) < 4 {
		return nil
	}

	bits := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])

	// Wind speed: bits 0-8 (9 bits), resolution 0.5 kt.
	windSpeedRaw := (bits >> 23) & 0x1FF
	windSpeed := float64(windSpeedRaw) / 2.0

	// Wind direction invalid: bit 9.
	windDirInvalid := (bits >> 22) & 0x01

	// Wind direction: bits 10-18 (9 bits).
	windDirRaw := (bits >> 13) & 0x1FF
	windDir := decodeWindDir(windDirRaw)

	// Temperature: bits 19-30 (12 bits).
	tempRaw := (bits >> 1) & 0xFFF
	temp := decodeTemperature(tempRaw)

	return &MeteoData{
		WindSpeed:      windSpeed,
		WindDirection:  windDir,
		WindDirInvalid: windDirInvalid != 0,
		Temperature:    temp,
	}
}

// decodeEarthRef decodes a 5-byte earth reference data tag.
// Format: track_invalid(1) + track(12) + speed(13) + vert_speed(12) = 38 bits.
func decodeEarthRef(data []byte) *EarthRef {
	if len(data) < 5 {
		return nil
	}

	bits := uint64(data[0])<<32 | uint64(data[1])<<24 | uint64(data[2])<<16 | uint64(data[3])<<8 | uint64(data[4])

	// Track invalid: bit 0.
	trackInvalid := (bits >> 39) & 0x01

	// Track: bits 1-12 (12 bits).
	trackRaw := uint32((bits >> 27) & 0xFFF)
	track := decodeHeading(trackRaw)

	// Ground speed: bits 13-25 (13 bits), resolution 0.5 kt.
	speedRaw := (bits >> 14) & 0x1FFF
	speed := float64(speedRaw) / 2.0

	// Vertical speed: bits 26-37 (12 bits).
	vsRaw := uint32((bits >> 2) & 0xFFF)
	vs := decodeVertSpeed(vsRaw)

	return &EarthRef{
		Track:        track,
		TrackInvalid: trackInvalid != 0,
		GroundSpeed:  speed,
		VertSpeed:    vs,
	}
}

// decodeAirRef decodes a 5-byte air reference data tag.
// Format: heading_invalid(1) + heading(12) + speed(13) + vert_speed(12) = 38 bits.
func decodeAirRef(data []byte) *AirRef {
	if len(data) < 5 {
		return nil
	}

	bits := uint64(data[0])<<32 | uint64(data[1])<<24 | uint64(data[2])<<16 | uint64(data[3])<<8 | uint64(data[4])

	// Heading invalid: bit 0.
	headingInvalid := (bits >> 39) & 0x01

	// Heading: bits 1-12 (12 bits).
	headingRaw := uint32((bits >> 27) & 0xFFF)
	heading := decodeHeading(headingRaw)

	// Mach speed: bits 13-25 (13 bits), stored as mach * 1000.
	speedRaw := (bits >> 14) & 0x1FFF
	mach := float64(speedRaw) / 1000.0

	// Vertical speed: bits 26-37 (12 bits).
	vsRaw := uint32((bits >> 2) & 0xFFF)
	vs := decodeVertSpeed(vsRaw)

	return &AirRef{
		Heading:        heading,
		HeadingInvalid: headingInvalid != 0,
		Mach:           mach,
		VertSpeed:      vs,
	}
}

// decodePredictedRoute decodes a 17-byte predicted route tag.
// Format: lat_next(21) + lon_next(21) + alt_next(16) + eta_next(14) +
//
//	lat_next_next(21) + lon_next_next(21) + alt_next_next(16) = 130 bits.
func decodePredictedRoute(data []byte) *PredictedRoute {
	if len(data) < 17 {
		return nil
	}

	// Read all 136 bits (17 bytes).
	var bits [17]byte
	copy(bits[:], data)

	// Helper to read N bits starting at bit offset.
	readBits := func(startBit, numBits int) uint32 {
		var result uint32
		for i := 0; i < numBits; i++ {
			byteIdx := (startBit + i) / 8
			bitIdx := 7 - ((startBit + i) % 8)
			if bits[byteIdx]&(1<<bitIdx) != 0 {
				result |= 1 << (numBits - 1 - i)
			}
		}
		return result
	}

	// Next waypoint.
	latNextRaw := readBits(0, 21)
	lonNextRaw := readBits(21, 21)
	altNextRaw := readBits(42, 16)
	etaNextRaw := readBits(58, 14)

	// Next+1 waypoint.
	latNextNextRaw := readBits(72, 21)
	lonNextNextRaw := readBits(93, 21)
	altNextNextRaw := readBits(114, 16)

	return &PredictedRoute{
		NextWaypoint: &Waypoint{
			Latitude:  decodeCoordinate(latNextRaw),
			Longitude: decodeCoordinate(lonNextRaw),
			Altitude:  decodeAltitude(altNextRaw),
			ETA:       int(etaNextRaw),
		},
		NextNextWaypoint: &Waypoint{
			Latitude:  decodeCoordinate(latNextNextRaw),
			Longitude: decodeCoordinate(lonNextNextRaw),
			Altitude:  decodeAltitude(altNextNextRaw),
		},
	}
}
