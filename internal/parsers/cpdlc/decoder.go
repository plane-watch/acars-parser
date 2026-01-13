package cpdlc

import (
	"fmt"
)

// Message represents a decoded CPDLC message.
type Message struct {
	Direction MessageDirection `json:"direction"`
	Header    MessageHeader    `json:"header"`
	Elements  []MessageElement `json:"elements"`
}

// MessageElement represents a single message element (uplink or downlink).
type MessageElement struct {
	ID    int         `json:"id"`             // Element ID (uM0-uM182 for uplink, dM0-dM128 for downlink).
	Label string      `json:"label"`          // Human-readable message template.
	Data  interface{} `json:"data,omitempty"` // Element-specific data.
	Text  string      `json:"text,omitempty"` // Formatted message text.
}

// Decoder decodes FANS-1/A CPDLC messages.
type Decoder struct {
	br        *BitReader
	direction MessageDirection
}

// NewDecoder creates a new CPDLC decoder.
func NewDecoder(data []byte, direction MessageDirection) *Decoder {
	return &Decoder{
		br:        NewBitReader(data),
		direction: direction,
	}
}

// Decode decodes the message.
func (d *Decoder) Decode() (*Message, error) {
	// We see both encodings in the wild:
	//  1) Standard UPER SEQUENCE optional field presence bit for the optional element_id_seq (multi-element messages)
	//  2) Legacy/non-standard encodings where that presence bit is omitted
	//
	// To be robust, try both and select the decode that succeeds with the least leftover bits.
	msgA, remA, errA := d.decodeAttempt(true, false) // standard: presence bit for optional element_id_seq
	msgB, remB, errB := d.decodeAttempt(false, true) // legacy: no presence bit; optionally try to decode trailing elements heuristically

	// Choose best successful attempt.
	if errA != nil && errB != nil {
		return nil, fmt.Errorf("decode failed (std): %v; (legacy): %v", errA, errB)
	}
	if errA == nil && errB != nil {
		return msgA, nil
	}
	if errB == nil && errA != nil {
		return msgB, nil
	}

	// Both succeeded: choose the one that consumes more bits (smaller remainder),
	// and as a tie-breaker, prefer the one with more elements.
	if remA < remB {
		return msgA, nil
	}
	if remB < remA {
		return msgB, nil
	}
	if len(msgA.Elements) > len(msgB.Elements) {
		return msgA, nil
	}
	if len(msgB.Elements) > len(msgA.Elements) {
		return msgB, nil
	}
	return msgA, nil
}

func (d *Decoder) decodeAttempt(withSeqPresenceBit bool, heuristicTailSeq bool) (*Message, int, error) {
	// reset reader
	_ = d.br.SetOffset(0)

	msg := &Message{
		Direction: d.direction,
	}

	hasSeq := false
	if withSeqPresenceBit {
		b, err := d.br.ReadBit()
		if err != nil {
			return nil, d.br.Remaining(), fmt.Errorf("seqPresent: %w", err)
		}
		hasSeq = b
	}

	// Decode header (presence bits for optional header fields come first within header).
	header, err := d.decodeHeader()
	if err != nil {
		return nil, d.br.Remaining(), fmt.Errorf("header: %w", err)
	}
	msg.Header = *header

	// Decode primary message element.
	elem, err := d.decodeElement()
	if err != nil {
		return nil, d.br.Remaining(), fmt.Errorf("element: %w", err)
	}
	msg.Elements = append(msg.Elements, *elem)

	// Optional additional elements (SEQUENCE OF SIZE(1..4)).
	if hasSeq {
		seqLen, err := d.br.ReadConstrainedInt(1, 4)
		if err != nil {
			return nil, d.br.Remaining(), fmt.Errorf("seqLen: %w", err)
		}
		for i := 0; i < seqLen; i++ {
			e, err := d.decodeElement()
			if err != nil {
				return nil, d.br.Remaining(), fmt.Errorf("seqElem[%d]: %w", i, err)
			}
			msg.Elements = append(msg.Elements, *e)
		}
	} else if heuristicTailSeq {
		// Legacy heuristic: if there is remaining data, try to interpret it as a constrained
		// SEQUENCE OF (1..4) message elements (length + elements). If it fails, roll back.
		off := d.br.Offset()
		if d.br.Remaining() >= 10 { // 2 bits len + >=8 bits element id
			seqLen, err := d.br.ReadConstrainedInt(1, 4)
			if err == nil {
				ok := true
				buf := make([]MessageElement, 0, seqLen)
				for i := 0; i < seqLen; i++ {
					e, err := d.decodeElement()
					if err != nil {
						ok = false
						break
					}
					buf = append(buf, *e)
				}
				if ok {
					msg.Elements = append(msg.Elements, buf...)
				} else {
					_ = d.br.SetOffset(off)
				}
			} else {
				_ = d.br.SetOffset(off)
			}
		}
	}

	return msg, d.br.Remaining(), nil
}

// decodeHeader decodes the ATC message header.
// In ASN.1 PER SEQUENCE encoding, presence bits for optional fields come FIRST,
// then the data fields in order.
func (d *Decoder) decodeHeader() (*MessageHeader, error) {
	header := &MessageHeader{}

	// FANSATCMessageHeader has 2 optional fields: msgReferenceNumber and timestamp.
	// In PER, presence bits come first (in order of the optional fields).
	hasRef, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasRef: %w", err)
	}
	hasTimestamp, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasTimestamp: %w", err)
	}

	// Message identification number (6 bits, 0-63) - mandatory, always present.
	msgID, err := d.br.ReadConstrainedInt(0, 63)
	if err != nil {
		return nil, fmt.Errorf("msgID: %w", err)
	}
	header.MsgID = msgID

	// Message reference number (6 bits, 0-63) - optional.
	if hasRef {
		ref, err := d.br.ReadConstrainedInt(0, 63)
		if err != nil {
			return nil, fmt.Errorf("msgRef: %w", err)
		}
		header.MsgRef = &ref
	}

	// Timestamp (optional).
	if hasTimestamp {
		timestamp, err := d.decodeHeaderTimestamp()
		if err != nil {
			return nil, fmt.Errorf("timestamp: %w", err)
		}
		header.Timestamp = timestamp
	}

	return header, nil
}

// decodeHeaderTimestamp decodes a FANS timestamp as used in CPDLC headers.
// NOTE: Header timestamps include seconds. We intentionally do NOT expose seconds
// in JSON (hours/minutes are enough for most UI), but we MUST consume them from
// the bitstream to keep decoding aligned.
func (d *Decoder) decodeHeaderTimestamp() (*Time, error) {
	// Hours (0-23) = 5 bits.
	hours, err := d.br.ReadConstrainedInt(0, 23)
	if err != nil {
		return nil, err
	}
	// Minutes (0-59) = 6 bits.
	minutes, err := d.br.ReadConstrainedInt(0, 59)
	if err != nil {
		return nil, err
	}
	// Seconds (0-59) = 6 bits (consume, do not expose).
	_, err = d.br.ReadConstrainedInt(0, 59)
	if err != nil {
		return nil, err
	}
	return &Time{Hours: hours, Minutes: minutes}, nil
}

// decodeElement decodes a single message element.
func (d *Decoder) decodeElement() (*MessageElement, error) {
	elem := &MessageElement{}

	// The element is a CHOICE with no extension marker (all alternatives are root).
	// Downlink: 8 bits for 0-128 (129 choices).
	// Uplink: 8 bits for 0-182 (183 choices).
	var maxChoice int
	if d.direction == DirectionUplink {
		maxChoice = 182 // uM0-uM182.
	} else {
		maxChoice = 128 // dM0-dM128.
	}

	// Read the element ID directly - no extension bit in FANS-1/A element choices.
	elemID, err := d.br.ReadConstrainedInt(0, maxChoice)
	if err != nil {
		return nil, fmt.Errorf("element ID: %w", err)
	}

	elem.ID = elemID

	// Get label and decode data based on direction and element ID.
	if d.direction == DirectionUplink {
		elem.Label = GetUplinkLabel(elemID)
		elem.Data, err = d.decodeUplinkData(elemID)
	} else {
		elem.Label = GetDownlinkLabel(elemID)
		elem.Data, err = d.decodeDownlinkData(elemID)
	}
	if err != nil {
		return nil, fmt.Errorf("element data: %w", err)
	}

	// Format the text.
	elem.Text = d.formatElementText(elem)

	return elem, nil
}

// decodeUplinkData decodes uplink element-specific data.
func (d *Decoder) decodeUplinkData(elemID int) (interface{}, error) {
	// Map element IDs to their data types and decode accordingly.
	switch elemID {
	case 0, 1, 2, 3, 4, 5, 67, 72, 96, 107, 116, 124, 125, 126, 127, 131, 132, 133,
		134, 135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147,
		154, 156, 161, 162, 164, 165, 166, 167, 168, 176, 177, 178, 179, 182:
		// NULL - no data.
		return nil, nil

	case 6, 19, 20, 23, 33, 34, 35, 36, 37, 38, 39, 40, 41, 128, 129, 148, 175:
		// Altitude.
		return d.decodeAltitude()

	case 7, 9, 11, 69, 71, 93:
		// Time.
		return d.decodeTime()

	case 8, 10, 12, 68, 70, 74, 75, 87, 130, 155:
		// Position.
		return d.decodePosition()

	case 13, 15, 17, 21, 24:
		// Time + Altitude.
		return d.decodeTimeAltitude()

	case 14, 16, 18, 22, 25, 42, 43, 44, 45, 46, 47, 48, 49, 92:
		// Position + Altitude.
		return d.decodePositionAltitude()

	case 30, 31, 32, 180:
		// Altitude + Altitude.
		return d.decodeAltitudeAltitude()

	case 106, 108, 109, 111, 112, 113, 114, 115, 151:
		// Speed.
		return d.decodeSpeed()

	case 110:
		// Speed + Speed.
		return d.decodeSpeedSpeed()

	case 94, 95, 98:
		// Direction + Degrees.
		return d.decodeDirectionDegrees()

	case 117, 120:
		// ICAO unit name + frequency.
		return d.decodeUnitNameFrequency()

	case 123:
		// Beacon code.
		return d.decodeBeaconCode()

	case 153:
		// Altimeter.
		return d.decodeAltimeter()

	case 157:
		// Frequency.
		return d.decodeFrequency()

	case 158:
		// ATIS code.
		return d.decodeATISCode()

	case 159:
		// Error information.
		return d.decodeErrorInfo()

	case 160:
		// ICAO facility designation.
		return d.decodeICAOFacility()

	case 169, 170:
		// Free text.
		return d.decodeFreeText()

	case 171, 172, 173, 174:
		// Vertical rate.
		return d.decodeVerticalRate()

	case 64, 82, 152:
		// Distance offset + direction.
		return d.decodeDistanceOffset()

	case 79, 83, 86:
		// Position + RouteClearance.
		return d.decodePositionRouteClearance()

	case 80, 85:
		// RouteClearance.
		return d.decodeRouteClearance()

	case 81, 84, 99:
		// ProcedureName (84 has position first).
		if elemID == 84 {
			return d.decodePositionProcedure()
		}
		return d.decodeProcedureName()

	default:
		// Unknown or complex type - skip.
		return nil, nil
	}
}

// decodeDownlinkData decodes downlink element-specific data.
func (d *Decoder) decodeDownlinkData(elemID int) (interface{}, error) {
	switch elemID {
	case 0, 1, 2, 3, 4, 5, 20, 25, 41, 51, 52, 53, 55, 56, 58, 63, 65, 66, 69, 74, 75:
		// NULL - no data.
		return nil, nil

	case 6, 8, 9, 10, 28, 29, 30, 32, 37, 38, 54, 61, 72:
		// Altitude.
		return d.decodeAltitude()

	case 7, 76, 77:
		// Altitude + Altitude.
		return d.decodeAltitudeAltitude()

	case 43, 46:
		// Time.
		return d.decodeTime()

	case 22, 31, 33, 42, 44, 45:
		// Position.
		return d.decodePosition()

	case 11, 12:
		// Position + Altitude.
		return d.decodePositionAltitude()

	case 13, 14:
		// Time + Altitude.
		return d.decodeTimeAltitude()

	case 18, 34, 39, 49:
		// Speed.
		return d.decodeSpeed()

	case 19, 50:
		// Speed + Speed.
		return d.decodeSpeedSpeed()

	case 35, 36, 70, 71:
		// Degrees.
		return d.decodeDegrees()

	case 21:
		// Frequency.
		return d.decodeFrequency()

	case 47:
		// Beacon code.
		return d.decodeBeaconCode()

	case 48:
		// POSITION REPORT [positionreport]
		return d.decodePositionReport()

	case 62:
		// Error information.
		return d.decodeErrorInfo()

	case 64:
		// ICAO facility designation.
		return d.decodeICAOFacility()

	case 67, 68:
		// Free text.
		return d.decodeFreeText()

	case 73:
		// Version number.
		return d.decodeVersionNumber()

	case 79:
		// ATIS code.
		return d.decodeATISCode()

	case 15, 27, 60, 80:
		// Distance offset + direction.
		return d.decodeDistanceOffset()

	case 23:
		// ProcedureName.
		return d.decodeProcedureName()

	case 24, 40:
		// RouteClearance.
		return d.decodeRouteClearance()

	case 26, 59:
		// Position + RouteClearance.
		return d.decodePositionRouteClearance()

	default:
		return nil, nil
	}
}

// Type-specific decoders.

func (d *Decoder) decodeAltitude() (*Altitude, error) {
	// FANSAltitude is a CHOICE with 8 alternatives (0-7), 3 bits, no extensions.
	// 0: altitudeQNH (12 bits, 0-2500, value x10 = feet)
	// 1: altitudeQNHMeters (14 bits, 0-16000, meters)
	// 2: altitudeQFE (12 bits, 0-2100, value x10 = feet)
	// 3: altitudeQFEMeters (13 bits, 0-7000, meters)
	// 4: altitudeGNSSFeet (18 bits, 0-150000, feet)
	// 5: altitudeGNSSMeters (16 bits, 0-50000, meters)
	// 6: altitudeFlightLevel (10 bits, 30-600, FL)
	// 7: altitudeFlightLevelMetric (11 bits, 100-2000, metric FL)

	choice, err := d.br.ReadConstrainedInt(0, 7)
	if err != nil {
		return nil, err
	}

	alt := &Altitude{}
	switch choice {
	case 0: // altitudeQNH (feet/10).
		v, err := d.br.ReadConstrainedInt(0, 2500)
		if err != nil {
			return nil, err
		}
		alt.Type = "feet"
		alt.Value = v * 10
	case 1: // altitudeQNHMeters.
		v, err := d.br.ReadConstrainedInt(0, 16000)
		if err != nil {
			return nil, err
		}
		alt.Type = "meters"
		alt.Value = v
	case 2: // altitudeQFE (feet/10).
		v, err := d.br.ReadConstrainedInt(0, 2100)
		if err != nil {
			return nil, err
		}
		alt.Type = "feet"
		alt.Value = v * 10
	case 3: // altitudeQFEMeters.
		v, err := d.br.ReadConstrainedInt(0, 7000)
		if err != nil {
			return nil, err
		}
		alt.Type = "meters"
		alt.Value = v
	case 4: // altitudeGNSSFeet.
		v, err := d.br.ReadConstrainedInt(0, 150000)
		if err != nil {
			return nil, err
		}
		alt.Type = "feet"
		alt.Value = v
	case 5: // altitudeGNSSMeters.
		v, err := d.br.ReadConstrainedInt(0, 50000)
		if err != nil {
			return nil, err
		}
		alt.Type = "meters"
		alt.Value = v
	case 6: // altitudeFlightLevel.
		v, err := d.br.ReadConstrainedInt(30, 600)
		if err != nil {
			return nil, err
		}
		alt.Type = "flight_level"
		alt.Value = v
	case 7: // altitudeFlightLevelMetric.
		v, err := d.br.ReadConstrainedInt(100, 2000)
		if err != nil {
			return nil, err
		}
		alt.Type = "flight_level_metric"
		alt.Value = v
	}

	return alt, nil
}

func (d *Decoder) decodeTime() (*Time, error) {
	// Most CPDLC element "time" fields are HH:MM only (no seconds).
	hours, err := d.br.ReadConstrainedInt(0, 23)
	if err != nil {
		return nil, err
	}
	minutes, err := d.br.ReadConstrainedInt(0, 59)
	if err != nil {
		return nil, err
	}
	return &Time{Hours: hours, Minutes: minutes}, nil
}

func (d *Decoder) decodePosition() (*Position, error) {
	// FANSPosition is a CHOICE with 5 alternatives (0-4), 3 bits, no extensions.
	// 0: fixName
	// 1: navaid
	// 2: airport
	// 3: latitudeLongitude
	// 4: placeBearingDistance

	choice, err := d.br.ReadConstrainedInt(0, 4)
	if err != nil {
		return nil, err
	}

	pos := &Position{}
	switch choice {
	case 0: // Fix name (1-5 chars).
		name, err := d.decodeFixName()
		if err != nil {
			return nil, err
		}
		pos.Type = "fix"
		pos.Name = name
	case 1: // Navaid (1-4 chars).
		name, err := d.decodeNavaid()
		if err != nil {
			return nil, err
		}
		pos.Type = "navaid"
		pos.Name = name
	case 2: // Airport (4 chars).
		name, err := d.decodeAirport()
		if err != nil {
			return nil, err
		}
		pos.Type = "airport"
		pos.Name = name
	case 3: // Lat/lon.
		lat, lon, err := d.decodeLatLon()
		if err != nil {
			return nil, err
		}
		pos.Type = "latlon"
		pos.Latitude = &lat
		pos.Longitude = &lon
	case 4: // PlaceBearingDistance.
		pbd, err := d.decodePlaceBearingDistance()
		if err != nil {
			return nil, err
		}
		pos.Type = "place_bearing_distance"
		pos.Name = pbd.FixName
		pos.Bearing = pbd.Bearing
		pos.Distance = pbd.Distance
		pos.DistanceUnit = pbd.DistanceUnit
		if pbd.Latitude != nil && pbd.Longitude != nil {
			pos.Latitude = pbd.Latitude
			pos.Longitude = pbd.Longitude
		}
	}

	return pos, nil
}

func (d *Decoder) decodeTimeAltitude() (map[string]interface{}, error) {
	time, err := d.decodeTime()
	if err != nil {
		return nil, err
	}
	alt, err := d.decodeAltitude()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"time": time, "altitude": alt}, nil
}

func (d *Decoder) decodePositionAltitude() (map[string]interface{}, error) {
	pos, err := d.decodePosition()
	if err != nil {
		return nil, err
	}
	alt, err := d.decodeAltitude()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"position": pos, "altitude": alt}, nil
}

func (d *Decoder) decodePositionRouteClearance() (map[string]interface{}, error) {
	pos, err := d.decodePosition()
	if err != nil {
		return nil, err
	}
	rc, err := d.decodeRouteClearance()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"position": pos, "route_clearance": rc}, nil
}

func (d *Decoder) decodePositionProcedure() (map[string]interface{}, error) {
	pos, err := d.decodePosition()
	if err != nil {
		return nil, err
	}
	proc, err := d.decodeProcedureName()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"position": pos, "procedure": proc}, nil
}

func (d *Decoder) decodeAltitudeAltitude() (map[string]interface{}, error) {
	alt1, err := d.decodeAltitude()
	if err != nil {
		return nil, err
	}
	alt2, err := d.decodeAltitude()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"altitude1": alt1, "altitude2": alt2}, nil
}

// decodePositionReport decodes dM48 POSITION REPORT data.
// In the wild, the POS-CURRENT block is often preceded by a 20-bit presence bitmap.
// Some aircraft omit the trailing "reported waypoint" triplet; we treat it as optional.
func (d *Decoder) decodePositionReport() (*PositionReport, error) {
	// Keep this tolerant: try the common 20-bit bitmap form first, then fall back.
	start := d.br.Offset()

	pr, err := d.decodePositionReportImpl(true)
	if err == nil {
		return pr, nil
	}

	_ = d.br.SetOffset(start)
	pr2, err2 := d.decodePositionReportImpl(false)
	if err2 == nil {
		return pr2, nil
	}

	// Prefer the first error (bitmap form) because it's usually closer to what aircraft send.
	return nil, err
}

func (d *Decoder) decodePositionReportImpl(withBitmap bool) (*PositionReport, error) {
	pr := &PositionReport{}

	if withBitmap {
		if _, err := d.br.ReadBits(20); err != nil {
			return nil, fmt.Errorf("presence bitmap: %w", err)
		}
	}

	// Current position (often encoded as latitude/longitude with minutes*10 precision).
	pos, err := d.decodePositionReportPosCurrent()
	if err != nil {
		return nil, fmt.Errorf("pos_current: %w", err)
	}
	pr.PosCurrent = pos

	// Time at current position: hours are commonly encoded 0..47 (6 bits) in FANS-1/A reports.
	tCur, err := d.decodeTime48()
	if err != nil {
		return nil, fmt.Errorf("time_at_pos_current: %w", err)
	}
	pr.TimeAtPosCurrent = tCur

	// Altitude.
	alt, err := d.decodeAltitude()
	if err != nil {
		return nil, fmt.Errorf("alt: %w", err)
	}
	pr.Alt = alt

	// Next fix.
	nextFix, err := d.decodePosition()
	if err != nil {
		return nil, fmt.Errorf("next_fix: %w", err)
	}
	pr.NextFix = nextFix

	// ETA at next fix (HH:MM).
	etaNext, err := d.decodeTime()
	if err != nil {
		return nil, fmt.Errorf("eta_at_fix_next: %w", err)
	}
	pr.EtaAtFixNext = etaNext

	// Next-next fix.
	nextNextFix, err := d.decodePosition()
	if err != nil {
		return nil, fmt.Errorf("next_next_fix: %w", err)
	}
	pr.NextNextFix = nextNextFix

	// ETA at destination (HH:MM).
	etaDest, err := d.decodeTime()
	if err != nil {
		return nil, fmt.Errorf("eta_at_dest: %w", err)
	}
	pr.EtaAtDest = etaDest

	// Temperature.
	temp, err := d.decodeTemperature()
	if err != nil {
		return nil, fmt.Errorf("temp: %w", err)
	}
	pr.Temp = temp

	// Winds.
	winds, err := d.decodeWinds()
	if err != nil {
		return nil, fmt.Errorf("winds: %w", err)
	}
	pr.Winds = winds

	// Speed.
	spd, err := d.decodeSpeed()
	if err != nil {
		return nil, fmt.Errorf("speed: %w", err)
	}
	pr.Speed = spd

	// Optional: reported waypoint (pos/time/alt).
	// Some aircraft don't include it; if it doesn't fit, roll back and leave it empty.
	off := d.br.Offset()
	if d.br.Remaining() > 0 {
		repPos, err1 := d.decodePosition()
		repTime, err2 := d.decodeTime()
		repAlt, err3 := d.decodeAltitude()
		if err1 == nil && err2 == nil && err3 == nil {
			pr.ReportedWptPos = repPos
			pr.ReportedWptTime = repTime
			pr.ReportedWptAlt = repAlt
		} else {
			_ = d.br.SetOffset(off)
		}
	}

	return pr, nil
}

// decodePositionReportPosCurrent decodes the "pos_current" field inside dM48.
// This position CHOICE differs from the generic FANS position used elsewhere:
// aircraft commonly use a lat/lon encoding with minutes*10 precision.
func (d *Decoder) decodePositionReportPosCurrent() (*Position, error) {
	choice, err := d.br.ReadConstrainedInt(0, 7)
	if err != nil {
		return nil, err
	}

	pos := &Position{}

	switch choice {
	case 0: // Fix name.
		name, err := d.decodeFixName()
		if err != nil {
			return nil, err
		}
		pos.Type = "fix"
		pos.Name = name

	case 1: // Navaid.
		name, err := d.decodeNavaid()
		if err != nil {
			return nil, err
		}
		pos.Type = "navaid"
		pos.Name = name

	case 2: // Airport.
		name, err := d.decodeAirport()
		if err != nil {
			return nil, err
		}
		pos.Type = "airport"
		pos.Name = name

	case 3, 7: // Latitude/Longitude (minutes*10).
		lat, lon, err := d.decodeLatLonMin10()
		if err != nil {
			return nil, err
		}
		pos.Type = "latlon"
		pos.Latitude = &lat
		pos.Longitude = &lon

	case 4: // Place/Bearing/Distance.
		pbd, err := d.decodePlaceBearingDistance()
		if err != nil {
			return nil, err
		}
		pos.Type = "place_bearing_distance"
		pos.Name = pbd.FixName
		pos.Bearing = pbd.Bearing
		pos.Distance = pbd.Distance
		pos.DistanceUnit = pbd.DistanceUnit
		if pbd.Latitude != nil && pbd.Longitude != nil {
			pos.Latitude = pbd.Latitude
			pos.Longitude = pbd.Longitude
		}

	default:
		return nil, fmt.Errorf("unsupported pos_current choice %d", choice)
	}

	return pos, nil
}

// decodeLatLonMin10 decodes lat/lon as (deg, minutes*10) with hemisphere bits.
// Observed layout (FANS-1/A position reports):
//  - lat_deg: 0..90
//  - lat_min10: 0..599 (minutes*10)
//  - lat_dir: 0=north, 1=south
//  - lon_dir: 0=west, 1=east
//  - lon_deg: 0..180
//  - lon_min10: 0..599 (minutes*10)
func (d *Decoder) decodeLatLonMin10() (float64, float64, error) {
	latDeg, err := d.br.ReadConstrainedInt(0, 90)
	if err != nil {
		return 0, 0, err
	}
	latMin10, err := d.br.ReadConstrainedInt(0, 599)
	if err != nil {
		return 0, 0, err
	}
	latSouth, err := d.br.ReadBit()
	if err != nil {
		return 0, 0, err
	}

	lonEast, err := d.br.ReadBit()
	if err != nil {
		return 0, 0, err
	}
	lonDeg, err := d.br.ReadConstrainedInt(0, 180)
	if err != nil {
		return 0, 0, err
	}
	lonMin10, err := d.br.ReadConstrainedInt(0, 599)
	if err != nil {
		return 0, 0, err
	}

	lat := float64(latDeg) + (float64(latMin10)/10.0)/60.0
	lon := float64(lonDeg) + (float64(lonMin10)/10.0)/60.0
	if latSouth {
		lat = -lat
	}
	if !lonEast {
		lon = -lon
	}

	return lat, lon, nil
}

// decodeTime48 decodes a time HH:MM where hours are encoded as 0..47.
func (d *Decoder) decodeTime48() (*Time, error) {
	hours, err := d.br.ReadConstrainedInt(0, 47)
	if err != nil {
		return nil, err
	}
	minutes, err := d.br.ReadConstrainedInt(0, 59)
	if err != nil {
		return nil, err
	}
	return &Time{Hours: hours, Minutes: minutes}, nil
}

func (d *Decoder) decodeTemperature() (*Temperature, error) {
	// FANSTemperature is a CHOICE.
	// Observed in FANS-1/A position reports:
	//  0: temperatureC  INTEGER(-80..47)   (7 bits)
	//  1: temperatureF  INTEGER(-100..100) (fallback)
	choice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, err
	}

	t := &Temperature{}
	if choice == 0 {
		v, err := d.br.ReadConstrainedInt(-80, 47)
		if err != nil {
			return nil, err
		}
		t.Type = "C"
		t.Value = float64(v)
		return t, nil
	}

	v, err := d.br.ReadConstrainedInt(-100, 100)
	if err != nil {
		return nil, err
	}
	t.Type = "F"
	t.Value = float64(v)
	return t, nil
}

func (d *Decoder) decodeWindSpeed() (*WindSpeed, error) {
	// FANSWindSpeed is a CHOICE:
	//  0: windSpeedEnglish (kts)
	//  1: windSpeedMetric (km/h)
	choice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, err
	}

	ws := &WindSpeed{}
	if choice == 0 {
		v, err := d.br.ReadConstrainedInt(0, 255)
		if err != nil {
			return nil, err
		}
		ws.Type = "kts"
		ws.Value = v
		return ws, nil
	}

	// Metric. Keep the range generous to avoid rejecting valid implementations.
	v, err := d.br.ReadConstrainedInt(0, 511)
	if err != nil {
		return nil, err
	}
	ws.Type = "kmh"
	ws.Value = v
	return ws, nil
}

func (d *Decoder) decodeWinds() (*Winds, error) {
	// Wind direction in degrees.
	dir, err := d.br.ReadConstrainedInt(1, 360)
	if err != nil {
		return nil, err
	}
	ws, err := d.decodeWindSpeed()
	if err != nil {
		return nil, err
	}
	return &Winds{Direction: dir, Speed: ws}, nil
}

func (d *Decoder) decodeSpeed() (*Speed, error) {
	// FANSSpeed is a CHOICE with 8 alternatives (0-7), 3 bits, no extensions.
	// 0: speedIndicated (5 bits, 7-38, knots x10 = 70-380 kt)
	// 1: speedIndicatedMetric (7 bits, 10-137, km/h x10 = 100-1370 km/h)
	// 2: speedTrue (6 bits, 7-70, knots x10 = 70-700 kt)
	// 3: speedTrueMetric (7 bits, 10-137, km/h x10 = 100-1370 km/h)
	// 4: speedGround (6 bits, 7-70, knots x10 = 70-700 kt)
	// 5: speedGroundMetric (8 bits, 10-265, km/h x10 = 100-2650 km/h)
	// 6: speedMach (5 bits, 61-92, Mach/100 = M.61-M.92)
	// 7: speedMachLarge (9 bits, 93-604, Mach/1000 = M.093-M.604)

	choice, err := d.br.ReadConstrainedInt(0, 7)
	if err != nil {
		return nil, err
	}

	spd := &Speed{}
	switch choice {
	case 0: // speedIndicated (knots x10).
		v, err := d.br.ReadConstrainedInt(7, 38)
		if err != nil {
			return nil, err
		}
		spd.Type = "knots"
		spd.Value = v * 10
	case 1: // speedIndicatedMetric (km/h x10).
		v, err := d.br.ReadConstrainedInt(10, 137)
		if err != nil {
			return nil, err
		}
		spd.Type = "kph"
		spd.Value = v * 10
	case 2: // speedTrue (knots x10).
		v, err := d.br.ReadConstrainedInt(7, 70)
		if err != nil {
			return nil, err
		}
		spd.Type = "knots"
		spd.Value = v * 10
	case 3: // speedTrueMetric (km/h x10).
		v, err := d.br.ReadConstrainedInt(10, 137)
		if err != nil {
			return nil, err
		}
		spd.Type = "kph"
		spd.Value = v * 10
	case 4: // speedGround (knots x10).
		v, err := d.br.ReadConstrainedInt(7, 70)
		if err != nil {
			return nil, err
		}
		spd.Type = "knots"
		spd.Value = v * 10
	case 5: // speedGroundMetric (km/h x10).
		v, err := d.br.ReadConstrainedInt(10, 265)
		if err != nil {
			return nil, err
		}
		spd.Type = "kph"
		spd.Value = v * 10
	case 6: // speedMach (Mach/100).
		v, err := d.br.ReadConstrainedInt(61, 92)
		if err != nil {
			return nil, err
		}
		spd.Type = "mach"
		spd.Value = v // M.61-M.92.
	case 7: // speedMachLarge (Mach/1000).
		v, err := d.br.ReadConstrainedInt(93, 604)
		if err != nil {
			return nil, err
		}
		spd.Type = "mach"
		spd.Value = v // M.093-M.604 (stored as 93-604).
	}

	return spd, nil
}

func (d *Decoder) decodeSpeedSpeed() (map[string]interface{}, error) {
	spd1, err := d.decodeSpeed()
	if err != nil {
		return nil, err
	}
	spd2, err := d.decodeSpeed()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"speed1": spd1, "speed2": spd2}, nil
}

func (d *Decoder) decodeDegrees() (*Degrees, error) {
	// FANSDegrees is a CHOICE with 2 alternatives (0-1), 1 bit, no extensions.
	// 0: degreesMagnetic (9 bits, 1-360)
	// 1: degreesTrue (9 bits, 1-360)

	choice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, err
	}

	deg := &Degrees{}
	deg.Magnetic = (choice == 0)

	// Degrees value (9 bits, 1-360).
	v, err := d.br.ReadConstrainedInt(1, 360)
	if err != nil {
		return nil, err
	}
	deg.Value = v

	return deg, nil
}

func (d *Decoder) decodeDirectionDegrees() (map[string]interface{}, error) {
	// Direction (left/right).
	dir, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, err
	}
	direction := "left"
	if dir == 1 {
		direction = "right"
	}

	// Degrees.
	deg, err := d.decodeDegrees()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{"direction": direction, "degrees": deg}, nil
}

func (d *Decoder) decodeDistanceOffset() (*DistanceOffset, error) {
	// FANSDistanceOffsetDirection is a SEQUENCE of:
	// 1. distanceOffset (CHOICE, 1 bit, no extensions): nm or km
	// 2. direction (ENUMERATED, 4 bits, 0-10)

	offset := &DistanceOffset{}

	// Read distance offset choice (1 bit, no extension).
	choice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, err
	}

	switch choice {
	case 0: // Nautical miles (7 bits, 1-128).
		v, err := d.br.ReadConstrainedInt(1, 128)
		if err != nil {
			return nil, err
		}
		offset.Distance = v
		offset.Unit = "nm"
	case 1: // Kilometres (8 bits, 1-256).
		v, err := d.br.ReadConstrainedInt(1, 256)
		if err != nil {
			return nil, err
		}
		offset.Distance = v
		offset.Unit = "km"
	}

	// Direction (4 bits, 0-10).
	dir, err := d.br.ReadConstrainedInt(0, 10)
	if err != nil {
		return nil, err
	}
	dirNames := []string{"left", "right", "either side", "north", "south", "east", "west", "north-east", "north-west", "south-east", "south-west"}
	if dir < len(dirNames) {
		offset.Direction = dirNames[dir]
	}

	return offset, nil
}

func (d *Decoder) decodeFrequency() (*Frequency, error) {
	// FANSFrequency is a CHOICE with 4 alternatives (0-3), 2 bits, no extensions.
	// 0: frequencyhf (15 bits, 2850-28000 kHz)
	// 1: frequencyvhf (15 bits, 117000-138000 kHz)
	// 2: frequencyuhf (18 bits, 225000-399975 kHz)
	// 3: frequencysatchannel (satellite channel string)

	choice, err := d.br.ReadConstrainedInt(0, 3)
	if err != nil {
		return nil, err
	}

	freq := &Frequency{}
	switch choice {
	case 0: // HF (kHz).
		v, err := d.br.ReadConstrainedInt(2850, 28000)
		if err != nil {
			return nil, err
		}
		freq.Type = "hf"
		freq.Value = float64(v)
	case 1: // VHF (kHz).
		v, err := d.br.ReadConstrainedInt(117000, 138000)
		if err != nil {
			return nil, err
		}
		freq.Type = "vhf"
		freq.Value = float64(v) / 1000.0
	case 2: // UHF (kHz).
		v, err := d.br.ReadConstrainedInt(225000, 399975)
		if err != nil {
			return nil, err
		}
		freq.Type = "uhf"
		freq.Value = float64(v) / 1000.0
	case 3: // SATCOM channel (string).
		// Skip the satellite channel for now - requires string decoding.
		freq.Type = "satcom"
		freq.Value = 0
	}

	return freq, nil
}

func (d *Decoder) decodeUnitNameFrequency() (map[string]interface{}, error) {
	// uM117/uM120 in FANS-1/A uses ICAOUnitNameFrequency:
	//   ICAOUnitName ::= SEQUENCE {
	//     icaoFacilityId      CHOICE { icaoFacilityDesignation IA5String(SIZE(4)), ... },
	//     icaoFacilityFunction ENUMERATED { center(0), approach(1), tower(2), final(3), groundControl(4),
	//                                      clearanceDelivery(5), departure(6), control(7) }
	//   }
	//   Frequency ::= CHOICE { hf, vhf, uhf, satcom }
	//
	// NOTE: The choice bit + facility-function bits MUST be consumed, otherwise the decoder becomes
	// misaligned and the following SEQUENCE OF elements gets decoded as garbage.
	unit, err := d.decodeICAOUnitName()
	if err != nil {
		return nil, err
	}

	// Frequency.
	freq, err := d.decodeFrequency()
	if err != nil {
		return nil, err
	}

	// Keep backward-compat with existing JSON consumers:
	//  - expose "frequency" as {type,value}
	//  - include "unit" (ICAO facility designation) as a plain string
	return map[string]interface{}{
		"unit":      unit.Designation,
		"unit_type": unit.Function,
		"frequency": freq,
	}, nil
}

// ICAOUnitName represents an ICAO unit name (facility designation + function).
type ICAOUnitName struct {
	Designation string
	Function    string
}

func (d *Decoder) decodeICAOUnitName() (*ICAOUnitName, error) {
	// icaoFacilityId choice (observed: 0 => ICAO facility designation).
	// In practice this is 1 bit in most on-air FANS messages.
	choice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, err
	}

	// ICAO facility designation (4 chars, IA5 7-bit each).
	// If the choice is not 0, we still try to read 4 chars to stay aligned.
	des, err := d.decodeIA5String(4)
	if err != nil {
		return nil, err
	}
	_ = choice

	// Facility function (ENUM 0..7 => 3 bits).
	fn, err := d.br.ReadConstrainedInt(0, 7)
	if err != nil {
		return nil, err
	}
	fnStr := ""
	switch fn {
	case 0:
		fnStr = "center"
	case 1:
		fnStr = "approach"
	case 2:
		fnStr = "tower"
	case 3:
		fnStr = "final"
	case 4:
		fnStr = "ground"
	case 5:
		fnStr = "clearance"
	case 6:
		fnStr = "departure"
	case 7:
		fnStr = "control"
	default:
		fnStr = ""
	}

	return &ICAOUnitName{Designation: des, Function: fnStr}, nil
}

func (d *Decoder) decodeBeaconCode() (*BeaconCode, error) {
	// 4 octal digits (0-7 each).
	code := ""
	for i := 0; i < 4; i++ {
		digit, err := d.br.ReadConstrainedInt(0, 7)
		if err != nil {
			return nil, err
		}
		code += string(byte('0' + digit))
	}
	return &BeaconCode{Code: code}, nil
}

func (d *Decoder) decodeAltimeter() (map[string]interface{}, error) {
	// FANSAltimeter is a CHOICE with 2 alternatives (0-1), 1 bit, no extensions.
	// 0: altimeterEnglish (10 bits, 2200-3200, inHg x100)
	// 1: altimeterMetric (13 bits, 7500-12500, hPa x10)

	choice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, err
	}

	switch choice {
	case 0: // English (inHg x100).
		v, err := d.br.ReadConstrainedInt(2200, 3200)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"type":  "inhg",
			"value": float64(v) / 100.0,
		}, nil
	case 1: // Metric (hPa x10).
		v, err := d.br.ReadConstrainedInt(7500, 12500)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"type":  "hpa",
			"value": float64(v) / 10.0,
		}, nil
	default:
		return nil, nil
	}
}

func (d *Decoder) decodeATISCode() (string, error) {
	// Single letter A-Z.
	v, err := d.br.ReadConstrainedInt(0, 25)
	if err != nil {
		return "", err
	}
	return string(byte('A' + v)), nil
}

func (d *Decoder) decodeErrorInfo() (*ErrorInfo, error) {
	// Error code (0-6).
	code, err := d.br.ReadConstrainedInt(0, 6)
	if err != nil {
		return nil, err
	}
	errorDescs := []string{
		"unrecognized message reference number",
		"logon data not accepted",
		"insufficient resources",
		"service unavailable",
		"duplicate message reference number",
		"no operational PDC",
		"unexpected request reference",
	}
	desc := ""
	if code < len(errorDescs) {
		desc = errorDescs[code]
	}
	return &ErrorInfo{Code: code, Desc: desc}, nil
}

func (d *Decoder) decodeICAOFacility() (string, error) {
	// 4-character ICAO facility designator.
	data, err := d.decodeIA5String(4)
	if err != nil {
		return "", err
	}
	return data, nil
}

func (d *Decoder) decodeFreeText() (*FreeText, error) {
	// IA5String with SIZE(1..256) -> constrained length (8 bits) + 7-bit IA5 chars.
	length, err := d.br.ReadConstrainedInt(1, 256)
	if err != nil {
		return nil, err
	}
	text, err := d.decodeIA5String(length)
	if err != nil {
		return nil, err
	}
	return &FreeText{Text: text}, nil
}

func (d *Decoder) decodeVersionNumber() (int, error) {
	// Version number (0-15).
	return d.br.ReadConstrainedInt(0, 15)
}

func (d *Decoder) decodeVerticalRate() (*VerticalRate, error) {
	// FANSVerticalRate is a CHOICE with 2 alternatives (0-1), 1 bit, no extensions.
	// 0: verticalRateEnglish (6 bits, 0-60, ft/min x100 = 0-6000 ft/min)
	// 1: verticalRateMetric (8 bits, 0-200, m/min x10 = 0-2000 m/min)

	choice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, err
	}

	vr := &VerticalRate{}
	switch choice {
	case 0: // ft/min x100.
		v, err := d.br.ReadConstrainedInt(0, 60)
		if err != nil {
			return nil, err
		}
		vr.Value = v * 100
	case 1: // m/min x10.
		v, err := d.br.ReadConstrainedInt(0, 200)
		if err != nil {
			return nil, err
		}
		// Convert m/min to ft/min (x 3.28084).
		vr.Value = int(float64(v) * 10 * 3.28084)
	}

	return vr, nil
}

func (d *Decoder) decodeFixName() (string, error) {
	// 1-5 character fix name.
	length, err := d.br.ReadConstrainedInt(1, 5)
	if err != nil {
		return "", err
	}
	return d.decodeIA5String(length)
}

func (d *Decoder) decodeNavaid() (string, error) {
	// 1-4 character navaid.
	length, err := d.br.ReadConstrainedInt(1, 4)
	if err != nil {
		return "", err
	}
	return d.decodeIA5String(length)
}

func (d *Decoder) decodeAirport() (string, error) {
	// 4 character airport code.
	return d.decodeIA5String(4)
}

func (d *Decoder) decodePlaceBearingDistance() (*PlaceBearingDistance, error) {
	pbd := &PlaceBearingDistance{}

	// FANSPlaceBearingDistance is a SEQUENCE with 1 optional field.
	// In PER, presence bit comes first.
	hasLatLon, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasLatLon: %w", err)
	}

	// Decode fixName (SIZE 1..5, IA5String).
	// Length is 3 bits (for 1-5 range: log2(5) = ~3 bits).
	fixLen, err := d.br.ReadConstrainedInt(1, 5)
	if err != nil {
		return nil, fmt.Errorf("fixName length: %w", err)
	}
	fixName, err := d.decodeIA5String(fixLen)
	if err != nil {
		return nil, fmt.Errorf("fixName: %w", err)
	}
	pbd.FixName = fixName

	// If latLon is present, decode it.
	if hasLatLon {
		lat, lon, err := d.decodeLatLon()
		if err != nil {
			return nil, fmt.Errorf("latLon: %w", err)
		}
		pbd.Latitude = &lat
		pbd.Longitude = &lon
	}

	// Decode degrees (CHOICE: 0=magnetic, 1=true).
	degChoice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, fmt.Errorf("degrees choice: %w", err)
	}
	pbd.Magnetic = (degChoice == 0)

	// Degrees value (9 bits, 1-360).
	degValue, err := d.br.ReadConstrainedInt(1, 360)
	if err != nil {
		return nil, fmt.Errorf("degrees value: %w", err)
	}
	pbd.Bearing = &degValue

	// Decode distance (CHOICE: 0=nm, 1=km).
	distChoice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, fmt.Errorf("distance choice: %w", err)
	}

	var distValue int
	if distChoice == 0 {
		// Nautical miles (14 bits, 0-9999).
		distValue, err = d.br.ReadConstrainedInt(0, 9999)
		if err != nil {
			return nil, fmt.Errorf("distance nm: %w", err)
		}
		pbd.DistanceUnit = "nm"
	} else {
		// Kilometres (10 bits, 1-1024).
		distValue, err = d.br.ReadConstrainedInt(1, 1024)
		if err != nil {
			return nil, fmt.Errorf("distance km: %w", err)
		}
		pbd.DistanceUnit = "km"
	}
	pbd.Distance = &distValue

	return pbd, nil
}

func (d *Decoder) decodeLatLon() (float64, float64, error) {
	// Latitude degrees (0-90).
	latDeg, err := d.br.ReadConstrainedInt(0, 90)
	if err != nil {
		return 0, 0, err
	}
	// Latitude minutes (0-59).
	latMin, err := d.br.ReadConstrainedInt(0, 59)
	if err != nil {
		return 0, 0, err
	}
	// Latitude seconds (0-59).
	latSec, err := d.br.ReadConstrainedInt(0, 59)
	if err != nil {
		return 0, 0, err
	}
	// Latitude direction (0=N, 1=S).
	latDir, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return 0, 0, err
	}

	lat := float64(latDeg) + float64(latMin)/60.0 + float64(latSec)/3600.0
	if latDir == 1 {
		lat = -lat
	}

	// Longitude degrees (0-180).
	lonDeg, err := d.br.ReadConstrainedInt(0, 180)
	if err != nil {
		return 0, 0, err
	}
	// Longitude minutes (0-59).
	lonMin, err := d.br.ReadConstrainedInt(0, 59)
	if err != nil {
		return 0, 0, err
	}
	// Longitude seconds (0-59).
	lonSec, err := d.br.ReadConstrainedInt(0, 59)
	if err != nil {
		return 0, 0, err
	}
	// Longitude direction (0=E, 1=W).
	lonDir, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return 0, 0, err
	}

	lon := float64(lonDeg) + float64(lonMin)/60.0 + float64(lonSec)/3600.0
	if lonDir == 1 {
		lon = -lon
	}

	return lat, lon, nil
}

func (d *Decoder) decodeIA5String(length int) (string, error) {
	// IA5 characters are 7-bit ASCII.
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		v, err := d.br.ReadBits(7)
		if err != nil {
			return "", err
		}
		result[i] = byte(v)
	}
	return string(result), nil
}

// formatElementText formats the element into human-readable text.
func (d *Decoder) formatElementText(elem *MessageElement) string {
	// For now, just return the label template.
	// A full implementation would substitute the data values.
	text := elem.Label

	// Simple substitutions based on data type.
	if data, ok := elem.Data.(*Altitude); ok && data != nil {
		text = substituteText(text, "[altitude]", data.String())
	}
	if data, ok := elem.Data.(*Speed); ok && data != nil {
		text = substituteText(text, "[speed]", data.String())
	}
	if data, ok := elem.Data.(*Position); ok && data != nil {
		text = substituteText(text, "[position]", data.String())
	}
	if data, ok := elem.Data.(*Time); ok && data != nil {
		text = substituteText(text, "[time]", data.String())
	}
	if data, ok := elem.Data.(*Degrees); ok && data != nil {
		text = substituteText(text, "[degrees]", data.String())
	}
	if data, ok := elem.Data.(*BeaconCode); ok && data != nil {
		text = substituteText(text, "[beaconcode]", data.String())
	}
	if data, ok := elem.Data.(*FreeText); ok && data != nil {
		text = substituteText(text, "[freetext]", data.Text)
	}
	if data, ok := elem.Data.(*Frequency); ok && data != nil {
		text = substituteText(text, "[frequency]", data.String())
	}
	if data, ok := elem.Data.(*VerticalRate); ok && data != nil {
		text = substituteText(text, "[verticalrate]", data.String())
	}
	if data, ok := elem.Data.(*DistanceOffset); ok && data != nil {
		text = substituteText(text, "[distanceoffset]", fmt.Sprintf("%d %s", data.Distance, data.Unit))
		text = substituteText(text, "[direction]", data.Direction)
	}
	if data, ok := elem.Data.(string); ok && data != "" {
		text = substituteText(text, "[atiscode]", data)
		text = substituteText(text, "[icaofacilitydesignation]", data)
	}
	if data, ok := elem.Data.(*ErrorInfo); ok && data != nil {
		text = substituteText(text, "[errorinformation]", data.Desc)
	}
	if data, ok := elem.Data.(*RouteClearance); ok && data != nil {
		text = substituteText(text, "[routeclearance]", data.String())
	}
	if data, ok := elem.Data.(*ProcedureName); ok && data != nil {
		text = substituteText(text, "[procedurename]", data.String())
	}
	if data, ok := elem.Data.(*PositionReport); ok && data != nil {
		text = substituteText(text, "[positionreport]", data.String())
	}

	// Handle map types for compound data.
	if data, ok := elem.Data.(map[string]interface{}); ok {
		if alt, ok := data["altitude"].(*Altitude); ok {
			text = substituteText(text, "[altitude]", alt.String())
		}
		if alt1, ok := data["altitude1"].(*Altitude); ok {
			text = substituteFirst(text, "[altitude]", alt1.String())
		}
		if alt2, ok := data["altitude2"].(*Altitude); ok {
			text = substituteText(text, "[altitude]", alt2.String())
		}
		if pos, ok := data["position"].(*Position); ok {
			text = substituteText(text, "[position]", pos.String())
		}
		if t, ok := data["time"].(*Time); ok {
			text = substituteText(text, "[time]", t.String())
		}
		if spd, ok := data["speed1"].(*Speed); ok {
			text = substituteFirst(text, "[speed]", spd.String())
		}
		if spd, ok := data["speed2"].(*Speed); ok {
			text = substituteText(text, "[speed]", spd.String())
		}
		if deg, ok := data["degrees"].(*Degrees); ok {
			text = substituteText(text, "[degrees]", deg.String())
		}
		if dir, ok := data["direction"].(string); ok {
			text = substituteText(text, "[direction]", dir)
		}
		if unit, ok := data["unit"].(string); ok {
			text = substituteText(text, "[icaounitname]", unit)
		}
		if freq, ok := data["frequency"].(*Frequency); ok {
			text = substituteText(text, "[frequency]", freq.String())
		}
		if rc, ok := data["route_clearance"].(*RouteClearance); ok {
			text = substituteText(text, "[routeclearance]", rc.String())
		}
		if proc, ok := data["procedure"].(*ProcedureName); ok {
			text = substituteText(text, "[procedurename]", proc.String())
		}
	}

	return text
}

// substituteText replaces all occurrences of pattern with replacement.
func substituteText(text, pattern, replacement string) string {
	result := ""
	for i := 0; i < len(text); {
		if i+len(pattern) <= len(text) && text[i:i+len(pattern)] == pattern {
			result += replacement
			i += len(pattern)
		} else {
			result += string(text[i])
			i++
		}
	}
	return result
}

// substituteFirst replaces only the first occurrence of pattern.
func substituteFirst(text, pattern, replacement string) string {
	for i := 0; i < len(text); {
		if i+len(pattern) <= len(text) && text[i:i+len(pattern)] == pattern {
			return text[:i] + replacement + text[i+len(pattern):]
		}
		i++
	}
	return text
}

// decodeRouteClearance decodes a FANSRouteClearance structure.
// FANSRouteClearance is a SEQUENCE with 10 ALL OPTIONAL fields.
func (d *Decoder) decodeRouteClearance() (*RouteClearance, error) {
	rc := &RouteClearance{}

	// Read 10 presence bits for the optional fields.
	hasAirportDeparture, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasAirportDeparture: %w", err)
	}
	hasAirportDestination, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasAirportDestination: %w", err)
	}
	hasRunwayDeparture, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasRunwayDeparture: %w", err)
	}
	hasProcedureDeparture, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasProcedureDeparture: %w", err)
	}
	hasRunwayArrival, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasRunwayArrival: %w", err)
	}
	hasProcedureApproach, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasProcedureApproach: %w", err)
	}
	hasProcedureArrival, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasProcedureArrival: %w", err)
	}
	hasAirwayIntercept, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasAirwayIntercept: %w", err)
	}
	hasRouteInformation, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasRouteInformation: %w", err)
	}
	hasRouteInfoAdditional, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasRouteInfoAdditional: %w", err)
	}

	// Decode present fields in order.
	if hasAirportDeparture {
		airport, err := d.decodeAirport()
		if err != nil {
			return nil, fmt.Errorf("airportDeparture: %w", err)
		}
		rc.AirportDeparture = airport
	}

	if hasAirportDestination {
		airport, err := d.decodeAirport()
		if err != nil {
			return nil, fmt.Errorf("airportDestination: %w", err)
		}
		rc.AirportDestination = airport
	}

	if hasRunwayDeparture {
		runway, err := d.decodeRunway()
		if err != nil {
			return nil, fmt.Errorf("runwayDeparture: %w", err)
		}
		rc.RunwayDeparture = runway
	}

	if hasProcedureDeparture {
		proc, err := d.decodeProcedureName()
		if err != nil {
			return nil, fmt.Errorf("procedureDeparture: %w", err)
		}
		rc.ProcedureDeparture = proc
	}

	if hasRunwayArrival {
		runway, err := d.decodeRunway()
		if err != nil {
			return nil, fmt.Errorf("runwayArrival: %w", err)
		}
		rc.RunwayArrival = runway
	}

	if hasProcedureApproach {
		proc, err := d.decodeProcedureName()
		if err != nil {
			return nil, fmt.Errorf("procedureApproach: %w", err)
		}
		rc.ProcedureApproach = proc
	}

	if hasProcedureArrival {
		proc, err := d.decodeProcedureName()
		if err != nil {
			return nil, fmt.Errorf("procedureArrival: %w", err)
		}
		rc.ProcedureArrival = proc
	}

	if hasAirwayIntercept {
		airway, err := d.decodeAirwayIdentifier()
		if err != nil {
			return nil, fmt.Errorf("airwayIntercept: %w", err)
		}
		rc.AirwayIntercept = airway
	}

	if hasRouteInformation {
		// FANSRouteInformation is a SEQUENCE (SIZE 1..128) OF FANSRouteInformationSequence.
		// Each element is a CHOICE with position types.
		// For now, decode as a sequence of position strings.
		count, err := d.br.ReadConstrainedInt(1, 128)
		if err != nil {
			return nil, fmt.Errorf("routeInformation count: %w", err)
		}
		rc.RouteInformation = make([]string, 0, count)
		for i := 0; i < count; i++ {
			pos, err := d.decodeRouteInformationElement()
			if err != nil {
				return nil, fmt.Errorf("routeInformation[%d]: %w", i, err)
			}
			rc.RouteInformation = append(rc.RouteInformation, pos)
		}
	}

	if hasRouteInfoAdditional {
		// FANSRouteInformationAdditional is a variable length IA5 string.
		length, err := d.br.ReadConstrainedInt(1, 256)
		if err != nil {
			return nil, fmt.Errorf("routeInfoAdditional length: %w", err)
		}
		text, err := d.decodeIA5String(length)
		if err != nil {
			return nil, fmt.Errorf("routeInfoAdditional: %w", err)
		}
		rc.RouteInfoAdditional = text
	}

	return rc, nil
}

// decodeRunway decodes a FANSRunway structure.
// FANSRunway is a SEQUENCE of direction (1-36) and configuration (enum 0-3).
func (d *Decoder) decodeRunway() (*Runway, error) {
	// Direction: 6 bits (1-36).
	direction, err := d.br.ReadConstrainedInt(1, 36)
	if err != nil {
		return nil, fmt.Errorf("direction: %w", err)
	}

	// Configuration: 2 bits (0-3: left, right, center, none).
	config, err := d.br.ReadConstrainedInt(0, 3)
	if err != nil {
		return nil, fmt.Errorf("configuration: %w", err)
	}

	configNames := []string{"left", "right", "center", "none"}
	configName := "none"
	if config < len(configNames) {
		configName = configNames[config]
	}

	return &Runway{
		Direction:     direction,
		Configuration: configName,
	}, nil
}

// decodeProcedureName decodes a FANSProcedureName structure.
// FANSProcedureName is a SEQUENCE of type (enum 0-2) and procedure (with optional transition).
func (d *Decoder) decodeProcedureName() (*ProcedureName, error) {
	// FANSProcedureType: 2 bits (0-2: arrival, approach, departure).
	procType, err := d.br.ReadConstrainedInt(0, 2)
	if err != nil {
		return nil, fmt.Errorf("procedureType: %w", err)
	}

	typeNames := []string{"arrival", "approach", "departure"}
	typeName := "unknown"
	if procType < len(typeNames) {
		typeName = typeNames[procType]
	}

	// FANSProcedure has 1 optional field (transition).
	hasTransition, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasTransition: %w", err)
	}

	// Procedure identifier (SIZE 1..6).
	nameLen, err := d.br.ReadConstrainedInt(1, 6)
	if err != nil {
		return nil, fmt.Errorf("procedure length: %w", err)
	}
	name, err := d.decodeIA5String(nameLen)
	if err != nil {
		return nil, fmt.Errorf("procedure name: %w", err)
	}

	proc := &ProcedureName{
		Type: typeName,
		Name: name,
	}

	if hasTransition {
		// Transition identifier (SIZE 1..5).
		transLen, err := d.br.ReadConstrainedInt(1, 5)
		if err != nil {
			return nil, fmt.Errorf("transition length: %w", err)
		}
		transition, err := d.decodeIA5String(transLen)
		if err != nil {
			return nil, fmt.Errorf("transition: %w", err)
		}
		proc.Transition = transition
	}

	return proc, nil
}

// decodeAirwayIdentifier decodes a FANSAirwayIdentifier (SIZE 2..7).
func (d *Decoder) decodeAirwayIdentifier() (string, error) {
	length, err := d.br.ReadConstrainedInt(2, 7)
	if err != nil {
		return "", fmt.Errorf("airway length: %w", err)
	}
	return d.decodeIA5String(length)
}

// decodeRouteInformationElement decodes a single route information element.
// FANSRouteInformationSequence is a CHOICE of various position types.
func (d *Decoder) decodeRouteInformationElement() (string, error) {
	// FANSRouteInformationSequence has 11 alternatives (0-10), 4 bits.
	// 0: publicationIdentifier
	// 1: latitudeLongitude
	// 2: placeBearingPlaceBearing (2x place-bearing pairs)
	// 3: placeBearingDistance
	// 4: airwayIdentifier
	// 5: trackDetail
	// 6: airport
	// 7: rnpRequirements
	// 8: fix
	// 9: navaid
	// 10: holdAtWaypoint

	choice, err := d.br.ReadConstrainedInt(0, 10)
	if err != nil {
		return "", err
	}

	switch choice {
	case 0: // publicationIdentifier (SIZE 1..6).
		length, err := d.br.ReadConstrainedInt(1, 6)
		if err != nil {
			return "", err
		}
		return d.decodeIA5String(length)
	case 1: // latitudeLongitude.
		lat, lon, err := d.decodeLatLon()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%.4f,%.4f", lat, lon), nil
	case 2: // placeBearingPlaceBearing (complex, skip for now).
		return "(place-bearing-place-bearing)", nil
	case 3: // placeBearingDistance.
		pbd, err := d.decodePlaceBearingDistance()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s %03d/%d%s", pbd.FixName, *pbd.Bearing, *pbd.Distance, pbd.DistanceUnit), nil
	case 4: // airwayIdentifier.
		return d.decodeAirwayIdentifier()
	case 5: // trackDetail (complex, skip for now).
		return "(track-detail)", nil
	case 6: // airport.
		return d.decodeAirport()
	case 7: // rnpRequirements (skip for now).
		return "(rnp)", nil
	case 8: // fix.
		return d.decodeFixName()
	case 9: // navaid.
		return d.decodeNavaid()
	case 10: // holdAtWaypoint (complex, skip for now).
		return "(hold)", nil
	default:
		return "", fmt.Errorf("unknown route element choice: %d", choice)
	}
}
