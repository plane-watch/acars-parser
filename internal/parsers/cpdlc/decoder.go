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
	msg := &Message{
		Direction: d.direction,
	}

	// FANSATCDownlinkMessage/FANSATCUplinkMessage is a SEQUENCE with:
	// 1. aTCMessageheader (mandatory)
	// 2. aTCDownlinkmsgelementid / aTCUplinkmsgelementid (mandatory CHOICE)
	// 3. aTCdownlinkmsgelementid_seqOf / aTCuplinkmsgelementid_seqOf (OPTIONAL)
	//
	// In ASN.1 PER SEQUENCE encoding, presence bits for optional fields come first.
	// So the first bit indicates whether seqOf (multi-element) is present.

	// Read presence bit for optional seqOf field.
	hasSeqOf, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("seqOf presence: %w", err)
	}

	// Decode header (presence bits for optional header fields come first within header).
	header, err := d.decodeHeader()
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	msg.Header = *header

	// Decode primary message element.
	elem, err := d.decodeElement()
	if err != nil {
		return nil, fmt.Errorf("element: %w", err)
	}
	msg.Elements = append(msg.Elements, *elem)

	// Decode additional elements if seqOf is present.
	if hasSeqOf {
		// FANSATCDownlinkMsgElementIdSequence is SIZE(1..4) OF FANSATCDownlinkMsgElementId.
		// Length is encoded as 2 bits (for 1-4 range).
		count, err := d.br.ReadConstrainedInt(1, 4)
		if err != nil {
			return nil, fmt.Errorf("seqOf count: %w", err)
		}
		for i := 0; i < count; i++ {
			elem, err := d.decodeElement()
			if err != nil {
				return nil, fmt.Errorf("seqOf element %d: %w", i, err)
			}
			msg.Elements = append(msg.Elements, *elem)
		}
	}

	return msg, nil
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
		timestamp, err := d.decodeTimestamp()
		if err != nil {
			return nil, fmt.Errorf("timestamp: %w", err)
		}
		header.Timestamp = timestamp
	}

	return header, nil
}

// decodeTimestamp decodes a FANS timestamp (hours, minutes, seconds).
// FANSTimestamp is a SEQUENCE of hours (5 bits), minutes (6 bits), seconds (6 bits).
func (d *Decoder) decodeTimestamp() (*Time, error) {
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
	// Seconds (0-59) = 6 bits.
	seconds, err := d.br.ReadConstrainedInt(0, 59)
	if err != nil {
		return nil, err
	}
	return &Time{Hours: hours, Minutes: minutes, Seconds: seconds}, nil
}

// decodeElement decodes a single message element.
func (d *Decoder) decodeElement() (*MessageElement, error) {
	elem := &MessageElement{}

	// ASN.1 PER CHOICE encoding for FANS-1/A element IDs.
	// According to libacars asn1c-generated constraints:
	// - FANSATCDownlinkMsgElementId: APC_CONSTRAINED, 8, 8, 0, 128 (8 bits, no extension)
	// - FANSATCUplinkMsgElementId: APC_CONSTRAINED, 8, 8, 0, 182 (8 bits, no extension)
	//
	// Both use straight 8-bit encoding with NO extension bit.

	var elemID int
	var err error
	var maxChoice int

	if d.direction == DirectionUplink {
		maxChoice = 182
	} else {
		maxChoice = 128
	}

	elemID, err = d.br.ReadConstrainedInt(0, maxChoice)
	if err != nil {
		return nil, fmt.Errorf("element ID: %w", err)
	}

	// Validate element ID is within the valid range for this direction.
	// Malformed/truncated messages may produce bit patterns that decode to values > maxChoice.
	if elemID > maxChoice {
		return nil, fmt.Errorf("element ID %d exceeds maximum %d for %s (malformed message)",
			elemID, maxChoice, d.direction)
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

	case 57:
		// Remaining fuel and souls on board (emergency message).
		return d.decodeRemainingFuelSouls()

	case 23:
		// ProcedureName.
		return d.decodeProcedureName()

	case 24, 40:
		// RouteClearance.
		return d.decodeRouteClearance()

	case 26, 59:
		// Position + RouteClearance.
		return d.decodePositionRouteClearance()

	case 16:
		// AT [position] REQUEST OFFSET [distanceoffset] [direction] OF ROUTE.
		return d.decodePositionDistanceOffset()

	case 17:
		// AT [time] REQUEST OFFSET [distanceoffset] [direction] OF ROUTE.
		return d.decodeTimeDistanceOffset()

	case 48:
		// POSITION REPORT [positionreport].
		return d.decodePositionReport()

	case 78:
		// AT [time] [distance] [tofrom] [position].
		return d.decodeTimeDistanceToFromPosition()

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
	return d.decodeTimestamp()
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
		freq.Value = v
	case 1: // VHF (kHz).
		v, err := d.br.ReadConstrainedInt(117000, 138000)
		if err != nil {
			return nil, err
		}
		freq.Type = "vhf"
		freq.Value = v
	case 2: // UHF (kHz).
		v, err := d.br.ReadConstrainedInt(225000, 399975)
		if err != nil {
			return nil, err
		}
		freq.Type = "uhf"
		freq.Value = v
	case 3: // SATCOM channel (string).
		// Skip the satellite channel for now - requires string decoding.
		freq.Type = "satcom"
		freq.Value = 0
	}

	return freq, nil
}

func (d *Decoder) decodeUnitNameFrequency() (map[string]interface{}, error) {
	// ICAO unit name.
	name, err := d.decodeICAOFacility()
	if err != nil {
		return nil, err
	}
	// Frequency.
	freq, err := d.decodeFrequency()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"unit": name, "frequency": freq}, nil
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
	// Length-prefixed IA5 string.
	length, err := d.br.ReadLength()
	if err != nil {
		return nil, err
	}
	if length > 256 {
		length = 256 // Cap at reasonable max.
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
		if fuel, ok := data["remaining_fuel"].(*RemainingFuel); ok {
			text = substituteText(text, "[remainingfuel]", fuel.String())
		}
		if souls, ok := data["persons_on_board"].(*PersonsOnBoard); ok {
			text = substituteText(text, "[remainingsouls]", souls.String())
		}
		// Handle distance_offset for dM16/dM17.
		if offset, ok := data["distance_offset"].(*DistanceOffset); ok {
			text = substituteText(text, "[distanceoffset]", fmt.Sprintf("%d %s", offset.Distance, offset.Unit))
			text = substituteText(text, "[direction]", offset.Direction)
		}
		// Handle distance + to_from for dM78.
		if dist, ok := data["distance"].(*Distance); ok {
			text = substituteText(text, "[distance]", dist.String())
		}
		if toFrom, ok := data["to_from"].(string); ok {
			text = substituteText(text, "[tofrom]", toFrom)
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
// decodeRemainingFuelSouls decodes dM57: remaining fuel and persons on board.
func (d *Decoder) decodeRemainingFuelSouls() (map[string]interface{}, error) {
	// FANSRemainingFuel: hours (0-99) and minutes (0-59).
	fuelHours, err := d.br.ReadConstrainedInt(0, 99)
	if err != nil {
		return nil, fmt.Errorf("fuel hours: %w", err)
	}
	fuelMinutes, err := d.br.ReadConstrainedInt(0, 59)
	if err != nil {
		return nil, fmt.Errorf("fuel minutes: %w", err)
	}

	// FANSPersonsOnBoard: 0-1023 (10 bits).
	souls, err := d.br.ReadConstrainedInt(0, 1023)
	if err != nil {
		return nil, fmt.Errorf("souls: %w", err)
	}

	return map[string]interface{}{
		"remaining_fuel":   &RemainingFuel{Hours: fuelHours, Minutes: fuelMinutes},
		"persons_on_board": &PersonsOnBoard{Count: souls},
	}, nil
}

// decodePositionDistanceOffset decodes dM16: position + distance offset.
func (d *Decoder) decodePositionDistanceOffset() (map[string]interface{}, error) {
	pos, err := d.decodePosition()
	if err != nil {
		return nil, fmt.Errorf("position: %w", err)
	}
	offset, err := d.decodeDistanceOffset()
	if err != nil {
		return nil, fmt.Errorf("distance offset: %w", err)
	}
	return map[string]interface{}{
		"position":        pos,
		"distance_offset": offset,
	}, nil
}

// decodeTimeDistanceOffset decodes dM17: time + distance offset.
func (d *Decoder) decodeTimeDistanceOffset() (map[string]interface{}, error) {
	time, err := d.decodeTime()
	if err != nil {
		return nil, fmt.Errorf("time: %w", err)
	}
	offset, err := d.decodeDistanceOffset()
	if err != nil {
		return nil, fmt.Errorf("distance offset: %w", err)
	}
	return map[string]interface{}{
		"time":            time,
		"distance_offset": offset,
	}, nil
}

// decodePositionReport decodes dM48: full position report.
// FANSPositionReport is a complex SEQUENCE with multiple optional fields.
func (d *Decoder) decodePositionReport() (*PositionReport, error) {
	pr := &PositionReport{}

	// FANSPositionReport has 10 optional fields. Read presence bits first.
	hasTime, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasTime: %w", err)
	}
	hasFixNext, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasFixNext: %w", err)
	}
	hasFixNextETA, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasFixNextETA: %w", err)
	}
	hasFixNextPlusOne, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasFixNextPlusOne: %w", err)
	}
	hasAltitude, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasAltitude: %w", err)
	}
	hasSpeed, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasSpeed: %w", err)
	}
	hasTemperature, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasTemperature: %w", err)
	}
	hasWind, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasWind: %w", err)
	}
	hasTurbulence, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasTurbulence: %w", err)
	}
	hasIcing, err := d.br.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("hasIcing: %w", err)
	}

	// Mandatory: current position.
	pos, err := d.decodePosition()
	if err != nil {
		return nil, fmt.Errorf("position: %w", err)
	}
	pr.Position = pos

	// Decode optional fields in order.
	if hasTime {
		time, err := d.decodeTime()
		if err != nil {
			return nil, fmt.Errorf("time: %w", err)
		}
		pr.Time = time
	}

	if hasFixNext {
		fix, err := d.decodePosition()
		if err != nil {
			return nil, fmt.Errorf("fixNext: %w", err)
		}
		pr.FixNext = fix
	}

	if hasFixNextETA {
		eta, err := d.decodeTime()
		if err != nil {
			return nil, fmt.Errorf("fixNextETA: %w", err)
		}
		pr.FixNextETA = eta
	}

	if hasFixNextPlusOne {
		fix, err := d.decodePosition()
		if err != nil {
			return nil, fmt.Errorf("fixNextPlusOne: %w", err)
		}
		pr.FixNextPlusOne = fix
	}

	if hasAltitude {
		alt, err := d.decodeAltitude()
		if err != nil {
			return nil, fmt.Errorf("altitude: %w", err)
		}
		pr.Altitude = alt
	}

	if hasSpeed {
		spd, err := d.decodeSpeed()
		if err != nil {
			return nil, fmt.Errorf("speed: %w", err)
		}
		pr.Speed = spd
	}

	if hasTemperature {
		// FANSTemperature is a constrained integer (-100 to +100 degrees C).
		temp, err := d.br.ReadConstrainedInt(-100, 100)
		if err != nil {
			return nil, fmt.Errorf("temperature: %w", err)
		}
		pr.Temperature = &temp
	}

	if hasWind {
		wind, err := d.decodeWind()
		if err != nil {
			return nil, fmt.Errorf("wind: %w", err)
		}
		pr.Wind = wind
	}

	if hasTurbulence {
		// FANSTurbulence is an enum (0-3: none, light, moderate, severe).
		turb, err := d.br.ReadConstrainedInt(0, 3)
		if err != nil {
			return nil, fmt.Errorf("turbulence: %w", err)
		}
		turbNames := []string{"none", "light", "moderate", "severe"}
		if turb < len(turbNames) {
			pr.Turbulence = turbNames[turb]
		}
	}

	if hasIcing {
		// FANSIcing is an enum (0-3: none, light, moderate, severe).
		icing, err := d.br.ReadConstrainedInt(0, 3)
		if err != nil {
			return nil, fmt.Errorf("icing: %w", err)
		}
		icingNames := []string{"none", "light", "moderate", "severe"}
		if icing < len(icingNames) {
			pr.Icing = icingNames[icing]
		}
	}

	return pr, nil
}

// decodeWind decodes a FANSWind structure.
func (d *Decoder) decodeWind() (*Wind, error) {
	// FANSWind is a SEQUENCE of direction and speed.
	// Direction: degrees (0-359), 9 bits.
	direction, err := d.br.ReadConstrainedInt(0, 359)
	if err != nil {
		return nil, fmt.Errorf("direction: %w", err)
	}

	// Speed: FANSWindSpeed is a CHOICE (0=knots, 1=kph).
	speedChoice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, fmt.Errorf("speed choice: %w", err)
	}

	var speed int
	var unit string
	if speedChoice == 0 {
		// Knots (0-255).
		speed, err = d.br.ReadConstrainedInt(0, 255)
		if err != nil {
			return nil, fmt.Errorf("speed knots: %w", err)
		}
		unit = "kt"
	} else {
		// KPH (0-511).
		speed, err = d.br.ReadConstrainedInt(0, 511)
		if err != nil {
			return nil, fmt.Errorf("speed kph: %w", err)
		}
		unit = "km/h"
	}

	return &Wind{
		Direction: direction,
		Speed:     speed,
		Unit:      unit,
	}, nil
}

// decodeDistance decodes a FANSDistance structure.
func (d *Decoder) decodeDistance() (*Distance, error) {
	// FANSDistance is a CHOICE (0=nm, 1=km).
	choice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return nil, fmt.Errorf("distance choice: %w", err)
	}

	dist := &Distance{}
	if choice == 0 {
		// Nautical miles (0-9999).
		v, err := d.br.ReadConstrainedInt(0, 9999)
		if err != nil {
			return nil, fmt.Errorf("distance nm: %w", err)
		}
		dist.Value = v
		dist.Unit = "nm"
	} else {
		// Kilometres (0-16000).
		v, err := d.br.ReadConstrainedInt(0, 16000)
		if err != nil {
			return nil, fmt.Errorf("distance km: %w", err)
		}
		dist.Value = v
		dist.Unit = "km"
	}

	return dist, nil
}

// decodeToFrom decodes a FANSToFrom enum.
func (d *Decoder) decodeToFrom() (string, error) {
	// FANSToFrom is an enum (0=to, 1=from).
	choice, err := d.br.ReadConstrainedInt(0, 1)
	if err != nil {
		return "", err
	}
	if choice == 0 {
		return "to", nil
	}
	return "from", nil
}

// decodeTimeDistanceToFromPosition decodes dM78: time + distance + toFrom + position.
func (d *Decoder) decodeTimeDistanceToFromPosition() (map[string]interface{}, error) {
	time, err := d.decodeTime()
	if err != nil {
		return nil, fmt.Errorf("time: %w", err)
	}
	dist, err := d.decodeDistance()
	if err != nil {
		return nil, fmt.Errorf("distance: %w", err)
	}
	toFrom, err := d.decodeToFrom()
	if err != nil {
		return nil, fmt.Errorf("toFrom: %w", err)
	}
	pos, err := d.decodePosition()
	if err != nil {
		return nil, fmt.Errorf("position: %w", err)
	}
	return map[string]interface{}{
		"time":     time,
		"distance": dist,
		"to_from":  toFrom,
		"position": pos,
	}, nil
}

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
