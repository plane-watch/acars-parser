package cpdlc

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/shaneshort/go-asn/uper"
)

// Errors returned by DecodeWithUPER when the direction cannot be settled.
var (
	// ErrAmbiguousDirection means that both message sets decode the message
	// to valid elements and the direction is not known, e.g. element 0 is
	// dM0 WILCO as a downlink and uM0 UNABLE as an uplink.
	ErrAmbiguousDirection = errors.New("decodes validly as both an uplink and a downlink")

	// ErrNoValidElements means that the message decodes, but neither message
	// set gives valid (non-reserved) elements.
	ErrNoValidElements = errors.New("no valid elements as an uplink or a downlink")

	// errTrailingOctets means that the message decodes without its last
	// octet: it is longer than its encoding.
	errTrailingOctets = errors.New("octets after the end of the encoding")
)

// DecodeWithUPER decodes a CPDLC message using the go-asn UPER library.
// This is the correct decoder for FANS-1/A which uses Unaligned PER.
//
// The message is decoded with both the uplink and the downlink message set,
// and a decode is valid when all its elements are defined (not reserved).
// The ASN.1 schemas differ, but some messages decode without error in both,
// so the content alone does not always decide the direction:
//   - If one message set gives valid elements, it is used, whatever the
//     given direction (the content proves the direction).
//   - If both do, the given direction decides; with DirectionUnknown,
//     ErrAmbiguousDirection is returned rather than a guess.
//   - If neither does, ErrNoValidElements is returned (or the decode error,
//     if neither decodes at all), rather than reserved elements.
func DecodeWithUPER(data []byte, direction MessageDirection) (*Message, error) {
	up, upErr := decodeUplinkUPER(data, &Message{Direction: DirectionUplink})
	upValid := upErr == nil && validateUplinkElements(up)
	down, downErr := decodeDownlinkUPER(data, &Message{Direction: DirectionDownlink})
	downValid := downErr == nil && validateDownlinkElements(down)

	switch {
	case upValid && downValid:
		switch direction {
		case DirectionUplink:
			return up, nil
		case DirectionDownlink:
			return down, nil
		}
		return nil, ErrAmbiguousDirection
	case upValid:
		return up, nil
	case downValid:
		return down, nil
	case upErr != nil && downErr != nil:
		if direction == DirectionUplink {
			return nil, upErr
		}
		return nil, downErr
	}
	return nil, ErrNoValidElements
}

// placeholderUplink and placeholderDownlink are the element IDs whose UPER
// type in fans_uper_types.go is an empty struct (ASN.1 NULL) although the
// element's label has a "[" placeholder. A NULL element carries no data, so
// a template that asks for data cannot be filled; a decode containing one
// is not valid. With the types matching the module, the only such element
// is uM178, whose template "[trackdetailmsg-deleted]" marks an element the
// module has deleted. The check guards against a type being changed to
// NULL in error.
var (
	placeholderUplink   = placeholderElements(reflect.TypeOf(UPERUplinkElement{}), GetUplinkLabel)
	placeholderDownlink = placeholderElements(reflect.TypeOf(UPERDownlinkElement{}), GetDownlinkLabel)
)

// placeholderElements returns the choice numbers of an element choice type
// whose alternative is *struct{} but whose label has a "[" placeholder.
func placeholderElements(choice reflect.Type, label func(int) string) map[int]bool {
	empty := reflect.TypeOf(&struct{}{})
	out := make(map[int]bool)
	for i := 0; i < choice.NumField(); i++ {
		f := choice.Field(i)
		if f.Type != empty {
			continue
		}
		if n, ok := choiceNumber(f.Tag.Get("asn1")); ok && strings.Contains(label(n), "[") {
			out[n] = true
		}
	}
	return out
}

// validateUplinkElements checks if all decoded uplink elements are semantically valid.
// Returns false if any element has a "(reserved)" or empty label, or is a
// placeholder (see placeholderUplink).
func validateUplinkElements(msg *Message) bool {
	if msg == nil || len(msg.Elements) == 0 {
		return false
	}
	for _, elem := range msg.Elements {
		label := GetUplinkLabel(elem.ID)
		if label == "(reserved)" || label == "" || placeholderUplink[elem.ID] {
			return false
		}
	}
	return true
}

// validateDownlinkElements checks if all decoded downlink elements are semantically valid.
// Returns false if any element has a "(reserved)" or empty label, or is a
// placeholder (see placeholderDownlink).
func validateDownlinkElements(msg *Message) bool {
	if msg == nil || len(msg.Elements) == 0 {
		return false
	}
	for _, elem := range msg.Elements {
		label := GetDownlinkLabel(elem.ID)
		if label == "(reserved)" || label == "" || placeholderDownlink[elem.ID] {
			return false
		}
	}
	return true
}

// decodeUplinkUPER decodes an uplink message using UPER.
func decodeUplinkUPER(data []byte, msg *Message) (*Message, error) {
	var fansMsg UPERUplinkMessage
	if err := unmarshalComplete(data, &fansMsg); err != nil {
		return nil, fmt.Errorf("uper unmarshal uplink: %w", err)
	}
	msg.Header = convertUPERHeader(&fansMsg.Header)
	elements, err := convertElements(fansMsg.Element, deref(fansMsg.Elements), GetUplinkLabel)
	if err != nil {
		return nil, fmt.Errorf("convert uplink element: %w", err)
	}
	msg.Elements = elements
	return msg, nil
}

// decodeDownlinkUPER decodes a downlink message using UPER.
func decodeDownlinkUPER(data []byte, msg *Message) (*Message, error) {
	var fansMsg UPERDownlinkMessage
	if err := unmarshalComplete(data, &fansMsg); err != nil {
		return nil, fmt.Errorf("uper unmarshal downlink: %w", err)
	}
	msg.Header = convertUPERHeader(&fansMsg.Header)
	elements, err := convertElements(fansMsg.Element, deref(fansMsg.Elements), GetDownlinkLabel)
	if err != nil {
		return nil, fmt.Errorf("convert downlink element: %w", err)
	}
	msg.Elements = elements
	return msg, nil
}

// unmarshalComplete decodes data into v, which points to a message type,
// and rejects the decode where libacars (asn1c's uper_decode_complete)
// would:
//   - an ENUMERATED index that names no enumeration (see checkEnumerations);
//   - an encoding that ends before the last octet of data. The decoder
//     reads the same bits in the same order whatever the length of its
//     input, so the encoding ends before the last octet exactly when data
//     without its last octet also decodes.
func unmarshalComplete(data []byte, v interface{}) error {
	if err := uper.Unmarshal(data, v); err != nil {
		return err
	}
	if err := checkEnumerations(reflect.ValueOf(v)); err != nil {
		return err
	}
	if len(data) > 0 {
		probe := reflect.New(reflect.TypeOf(v).Elem()).Interface()
		if uper.Unmarshal(data[:len(data)-1], probe) == nil {
			return errTrailingOctets
		}
	}
	return nil
}

// checkEnumerations returns an error if a decoded value holds an
// enumeration index (a value of a type implementing enumerated) that names
// no enumeration. UPER encodes the index of an ENUMERATED value in the
// fewest bits that hold the largest index, so an enumeration whose size is
// not a power of two has encodable indexes that name nothing; go-asn
// decodes them as integers without checking them.
func checkEnumerations(v reflect.Value) error {
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return checkEnumerations(v.Elem())
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := checkEnumerations(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := checkEnumerations(v.Field(i)); err != nil {
				return err
			}
		}
	case reflect.Int:
		if e, ok := v.Interface().(enumerated); ok {
			if n := v.Int(); n < 0 || n >= int64(len(e.names())) {
				return fmt.Errorf("%s: index %d names no enumeration", v.Type().Name(), n)
			}
		}
	}
	return nil
}

// convertUPERHeader converts a UPER header to the output format.
func convertUPERHeader(h *UPERMessageHeader) MessageHeader {
	header := MessageHeader{
		MsgID: h.MsgID,
	}
	if h.MsgRef != nil {
		ref := *h.MsgRef
		header.MsgRef = &ref
	}
	if h.Timestamp != nil {
		header.Timestamp = &Timestamp{
			Hours:   h.Timestamp.Hours,
			Minutes: h.Timestamp.Minutes,
			Seconds: h.Timestamp.Seconds,
		}
	}
	return header
}

// convertElements converts a message's primary element and its additional
// elements (UPERUplinkElement or UPERDownlinkElement) to the output format,
// labelling them with label.
func convertElements[E UPERUplinkElement | UPERDownlinkElement](first E, rest []E, label func(int) string) ([]MessageElement, error) {
	all := append([]E{first}, rest...)
	out := make([]MessageElement, 0, len(all))
	for i := range all {
		elem, err := convertElement(reflect.ValueOf(&all[i]).Elem(), label)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		out = append(out, elem)
	}
	return out, nil
}

// convertElement converts a decoded element CHOICE to a MessageElement: its
// ID is the chosen alternative's number, its data the alternative's value
// (see convertValue) and its text the label with each placeholder replaced
// by the value of the parameter in the same place. An element without
// parameters has no data and no text.
func convertElement(choice reflect.Value, label func(int) string) (MessageElement, error) {
	id, value, err := chosenAlternative(choice)
	if err != nil {
		return MessageElement{}, err
	}
	elem := MessageElement{ID: id, Label: label(id)}
	data, fills, err := convertValue(value)
	if err != nil {
		return MessageElement{}, fmt.Errorf("%s: %w", elem.Label, err)
	}
	if data != nil {
		elem.Data = data
		elem.Text = fillTemplate(elem.Label, fills)
	}
	return elem, nil
}

// chosenAlternative returns the choice number and the value of the
// alternative that a decoded CHOICE struct holds.
func chosenAlternative(choice reflect.Value) (int, reflect.Value, error) {
	t := choice.Type()
	for i := 0; i < t.NumField(); i++ {
		f := choice.Field(i)
		if f.IsNil() {
			continue
		}
		n, ok := choiceNumber(t.Field(i).Tag.Get("asn1"))
		if !ok {
			return 0, reflect.Value{}, fmt.Errorf("%s.%s has no choice number", t.Name(), t.Field(i).Name)
		}
		return n, f.Elem(), nil
	}
	return 0, reflect.Value{}, fmt.Errorf("%s holds no alternative", t.Name())
}

// choiceNumber returns N from the "choice:N" option of an asn1 struct tag.
func choiceNumber(tag string) (int, bool) {
	for _, opt := range strings.Split(tag, ",") {
		if s, ok := strings.CutPrefix(opt, "choice:"); ok {
			n, err := strconv.Atoi(s)
			return n, err == nil
		}
	}
	return 0, false
}

// fillTemplate replaces the placeholders of an element template ("[...]")
// in turn with fills. An empty fill (an absent OPTIONAL parameter) removes
// its placeholder and the space before it. Placeholders with no space
// between them (dM80 DEVIATING [distanceoffset][direction] OF ROUTE) are
// separated by one.
func fillTemplate(template string, fills []string) string {
	var b strings.Builder
	rest := template
	for _, fill := range fills {
		start := strings.IndexByte(rest, '[')
		end := strings.IndexByte(rest, ']')
		if start < 0 || end < start {
			break
		}
		before := rest[:start]
		if fill == "" {
			before = strings.TrimSuffix(before, " ")
		} else if before == "" && b.Len() > 0 {
			before = " "
		}
		b.WriteString(before)
		b.WriteString(fill)
		rest = rest[end+1:]
	}
	b.WriteString(rest)
	return b.String()
}

// convertValue converts the value of an element CHOICE alternative (or one
// of its parameters) to the output format. It returns the value, the text
// for each placeholder the value fills in the element's template, in order,
// and an error if the value's type has no conversion (a programming error,
// which TestConvertEveryElementType catches).
//
// A parameter type (UPERAltitude, UPERPosition and so on) converts to its
// output type. A SEQUENCE of several parameters (UPERPositionAltitude and
// so on) converts to a map from each field's `data` tag to the field's
// value; see fans_uper_types.go. NULL converts to nil.
func convertValue(v reflect.Value) (interface{}, []string, error) {
	if !v.CanAddr() {
		// Values reached through pointers and struct fields are
		// addressable; copy any other so that its address can be taken.
		c := reflect.New(v.Type()).Elem()
		c.Set(v)
		v = c
	}
	one := func(value interface{}, text string) (interface{}, []string, error) {
		return value, []string{text}, nil
	}
	switch x := v.Addr().Interface().(type) {
	case *struct{}:
		return nil, nil, nil
	case *UPERAltitude:
		a := convertUPERAltitude(x)
		return one(a, a.String())
	case *UPERTime:
		t := convertUPERTime(x)
		return one(t, t.String())
	case *UPERPosition:
		p := convertUPERPosition(x)
		return one(p, p.String())
	case *UPERSpeed:
		s := convertUPERSpeed(x)
		return one(s, s.String())
	case *UPERDegrees:
		d := convertUPERDegrees(x)
		return one(d, d.String())
	case *UPERDistance:
		d := convertUPERDistance(x)
		return one(d, d.String())
	case *UPERFrequency:
		f := convertUPERFrequency(x)
		return one(f, f.String())
	case *UPERBeaconCode:
		b := convertUPERBeaconCode(x)
		return one(b, b.String())
	case *UPERAltimeter:
		a := convertUPERAltimeter(x)
		return one(a, a.String())
	case *UPERVerticalRate:
		r := convertUPERVerticalRate(x)
		return one(r, r.String())
	case *UPERICAOUnitName:
		u := convertUPERICAOUnitName(x)
		return one(u, u.String())
	case *UPERRemainingFuel:
		f := &RemainingFuel{Hours: x.Hours, Minutes: x.Minutes}
		return one(f, f.String())
	case *UPERRemainingSouls:
		p := &PersonsOnBoard{Count: int(*x)}
		return one(p, p.String())
	case *UPERFreeText:
		return one(&FreeText{Text: string(*x)}, string(*x))
	case *UPERATISCode:
		return one(string(*x), string(*x))
	case *UPERICAOFacilityDesignation:
		return one(string(*x), string(*x))
	case *UPERVersionNumber:
		return one(int(*x), strconv.Itoa(int(*x)))
	case *UPERErrorInformation:
		e := &ErrorInfo{Code: int(*x), Desc: enumName(*x)}
		return one(e, enumText(e.Desc))
	case *UPERDirection:
		return one(enumName(*x), enumText(enumName(*x)))
	case *UPERToFrom:
		return one(enumName(*x), enumName(*x))
	case *UPERTp4Table:
		return one(enumName(*x), enumName(*x))
	case *UPERDistanceOffsetDirection:
		d := convertUPERDistanceOffsetDirection(x)
		// The offset and the direction fill two placeholders.
		return d, []string{fmt.Sprintf("%d %s", d.Distance, d.Unit), enumText(d.Direction)}, nil
	case *UPERProcedureName:
		p := convertUPERProcedureName(x)
		return one(p, p.String())
	case *UPERRouteClearance:
		r := convertUPERRouteClearance(x)
		return one(r, r.String())
	case *UPERHoldClearance:
		h := convertUPERHoldClearance(x)
		return h, []string{h.Position.String(), h.Altitude.String(), h.Degrees.String(),
			enumText(h.Direction), h.LegType.String()}, nil
	case *UPERPositionReport:
		r := convertUPERPositionReport(x)
		return one(r, r.String())
	case *UPERPredepartureClearance:
		p := convertUPERPredepartureClearance(x)
		return one(p, p.String())
	}
	if v.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("no conversion for %s", v.Type())
	}
	data := make(map[string]interface{})
	var fills []string
	if err := convertParameters(v, data, &fills); err != nil {
		return nil, nil, err
	}
	return data, fills, nil
}

// convertParameters converts each field of a SEQUENCE of several element
// parameters into data, under the field's `data` tag, and appends the
// fields' placeholder texts to fills. A field tagged `data:"-"` (a
// fixed-size SEQUENCE OF) contributes its own fields.
func convertParameters(v reflect.Value, data map[string]interface{}, fills *[]string) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		key := t.Field(i).Tag.Get("data")
		switch key {
		case "":
			return fmt.Errorf("no conversion for %s (field %s has no data tag)", t, t.Field(i).Name)
		case "-":
			if err := convertParameters(v.Field(i), data, fills); err != nil {
				return err
			}
		default:
			value, f, err := convertValue(v.Field(i))
			if err != nil {
				return err
			}
			data[key] = value
			*fills = append(*fills, f...)
		}
	}
	return nil
}

// deref returns the elements of an OPTIONAL SEQUENCE OF, or nil if it is
// absent.
func deref[T any](seq *[]T) []T {
	if seq == nil {
		return nil
	}
	return *seq
}

// enumName returns the module's identifier for an enumeration index; the
// index has been checked by checkEnumerations.
func enumName[E interface {
	~int
	enumerated
}](e E) string {
	names := e.names()
	if int(e) < 0 || int(e) >= len(names) {
		return strconv.Itoa(int(e))
	}
	return names[e]
}

// convertUPERTime converts a UPER time to the output format.
func convertUPERTime(t *UPERTime) *Time {
	if t == nil {
		return nil
	}
	return &Time{Hours: t.Hours, Minutes: t.Minutes}
}

// convertUPERAltitude converts a UPER altitude to the output format.
func convertUPERAltitude(a *UPERAltitude) *Altitude {
	if a == nil {
		return nil
	}
	alt := &Altitude{}
	switch {
	case a.AltitudeQNH != nil:
		alt.Type, alt.Value = "qnh_feet", *a.AltitudeQNH*10
	case a.AltitudeQNHMeters != nil:
		alt.Type, alt.Value = "qnh_metres", *a.AltitudeQNHMeters
	case a.AltitudeQFE != nil:
		alt.Type, alt.Value = "qfe_feet", *a.AltitudeQFE*10
	case a.AltitudeQFEMeters != nil:
		alt.Type, alt.Value = "qfe_metres", *a.AltitudeQFEMeters
	case a.AltitudeGNSSFeet != nil:
		alt.Type, alt.Value = "gnss_feet", *a.AltitudeGNSSFeet
	case a.AltitudeGNSSMeters != nil:
		alt.Type, alt.Value = "gnss_metres", *a.AltitudeGNSSMeters
	case a.AltitudeFlightLevel != nil:
		alt.Type, alt.Value = "flight_level", *a.AltitudeFlightLevel
	case a.AltitudeFlightLevelMetric != nil:
		alt.Type, alt.Value = "flight_level_metric", *a.AltitudeFlightLevelMetric
	}
	return alt
}

// convertUPERSpeed converts a UPER speed to the output format. The
// module encodes the knot and km/h alternatives in units of ten and both
// Mach alternatives in hundredths of Mach.
func convertUPERSpeed(s *UPERSpeed) *Speed {
	if s == nil {
		return nil
	}
	speed := &Speed{}
	switch {
	case s.SpeedIndicated != nil:
		speed.Type, speed.Value = "indicated", *s.SpeedIndicated*10
	case s.SpeedIndicatedMetric != nil:
		speed.Type, speed.Value = "indicated_metric", *s.SpeedIndicatedMetric*10
	case s.SpeedTrue != nil:
		speed.Type, speed.Value = "true", *s.SpeedTrue*10
	case s.SpeedTrueMetric != nil:
		speed.Type, speed.Value = "true_metric", *s.SpeedTrueMetric*10
	case s.SpeedGround != nil:
		speed.Type, speed.Value = "ground", *s.SpeedGround*10
	case s.SpeedGroundMetric != nil:
		speed.Type, speed.Value = "ground_metric", *s.SpeedGroundMetric*10
	case s.SpeedMach != nil:
		speed.Type, speed.Value = "mach", *s.SpeedMach
	case s.SpeedMachLarge != nil:
		speed.Type, speed.Value = "mach", *s.SpeedMachLarge
	}
	return speed
}

// convertUPERDegrees converts a UPER degrees value to the output format.
func convertUPERDegrees(d *UPERDegrees) *Degrees {
	if d == nil {
		return nil
	}
	switch {
	case d.DegreesMagnetic != nil:
		return &Degrees{Magnetic: true, Value: *d.DegreesMagnetic}
	case d.DegreesTrue != nil:
		return &Degrees{Value: *d.DegreesTrue}
	}
	return &Degrees{}
}

// convertUPERDistance converts a UPER distance (tenths of a nautical mile
// or kilometres) to the output format.
func convertUPERDistance(d *UPERDistance) *Distance {
	if d == nil {
		return nil
	}
	switch {
	case d.DistanceNm != nil:
		return &Distance{Value: float64(*d.DistanceNm) / 10, Unit: "nm"}
	case d.DistanceKm != nil:
		return &Distance{Value: float64(*d.DistanceKm), Unit: "km"}
	}
	return &Distance{}
}

// convertUPERDistanceOffsetDirection converts distance offset + direction.
func convertUPERDistanceOffsetDirection(d *UPERDistanceOffsetDirection) *DistanceOffset {
	offset := &DistanceOffset{Direction: enumName(d.Direction)}
	switch {
	case d.DistanceOffset.DistanceOffsetNm != nil:
		offset.Distance, offset.Unit = *d.DistanceOffset.DistanceOffsetNm, "nm"
	case d.DistanceOffset.DistanceOffsetKm != nil:
		offset.Distance, offset.Unit = *d.DistanceOffset.DistanceOffsetKm, "km"
	}
	return offset
}

// convertUPERFrequency converts a frequency.
func convertUPERFrequency(f *UPERFrequency) *Frequency {
	if f == nil {
		return nil
	}
	freq := &Frequency{}
	switch {
	case f.FrequencyHF != nil:
		freq.Type, freq.Value = "hf", *f.FrequencyHF
	case f.FrequencyVHF != nil:
		freq.Type, freq.Value = "vhf", *f.FrequencyVHF
	case f.FrequencyUHF != nil:
		freq.Type, freq.Value = "uhf", *f.FrequencyUHF
	case f.FrequencySatChannel != nil:
		freq.Type = "satcom"
		var b strings.Builder
		for _, c := range *f.FrequencySatChannel {
			b.WriteByte(numericAlphabet[c.Index])
		}
		freq.Channel = b.String()
	}
	return freq
}

// convertUPERBeaconCode converts a beacon code.
func convertUPERBeaconCode(b *UPERBeaconCode) *BeaconCode {
	if b == nil {
		return nil
	}
	return &BeaconCode{Code: fmt.Sprintf("%d%d%d%d", b.Digit1, b.Digit2, b.Digit3, b.Digit4)}
}

// convertUPERAltimeter converts an altimeter setting (hundredths of an inch
// of mercury or tenths of a hectopascal).
func convertUPERAltimeter(a *UPERAltimeter) *Altimeter {
	switch {
	case a.AltimeterEnglish != nil:
		return &Altimeter{Value: float64(*a.AltimeterEnglish) / 100, Unit: "inHg"}
	case a.AltimeterMetric != nil:
		return &Altimeter{Value: float64(*a.AltimeterMetric) / 10, Unit: "hPa"}
	}
	return &Altimeter{}
}

// convertUPERVerticalRate converts a vertical rate (hundreds of feet or
// tens of metres per minute).
func convertUPERVerticalRate(r *UPERVerticalRate) *VerticalRate {
	switch {
	case r.VerticalRateEnglish != nil:
		return &VerticalRate{Value: *r.VerticalRateEnglish * 100, Unit: "ft/min"}
	case r.VerticalRateMetric != nil:
		return &VerticalRate{Value: *r.VerticalRateMetric * 10, Unit: "m/min"}
	}
	return &VerticalRate{}
}

// convertUPERICAOUnitName converts an ICAO unit name.
func convertUPERICAOUnitName(u *UPERICAOUnitName) *ICAOUnitName {
	unit := &ICAOUnitName{FacilityFunction: enumName(u.ICAOFacilityFunction)}
	id := u.ICAOFacilityIdentification
	switch {
	case id.ICAOFacilityDesignation != nil:
		unit.FacilityDesignation = *id.ICAOFacilityDesignation
	case id.ICAOFacilityName != nil:
		unit.FacilityName = *id.ICAOFacilityName
	}
	return unit
}

// convertUPERLatLon converts a latitude and longitude (degrees and optional
// tenths of a minute) to decimal degrees, north and east positive.
func convertUPERLatLon(ll *UPERLatitudeLongitude) (lat, lon float64) {
	lat = float64(ll.Latitude.Degrees)
	if ll.Latitude.MinutesTenths != nil {
		lat += float64(*ll.Latitude.MinutesTenths) / 600.0
	}
	if enumName(ll.Latitude.Direction) == "south" {
		lat = -lat
	}
	lon = float64(ll.Longitude.Degrees)
	if ll.Longitude.MinutesTenths != nil {
		lon += float64(*ll.Longitude.MinutesTenths) / 600.0
	}
	if enumName(ll.Longitude.Direction) == "west" {
		lon = -lon
	}
	return lat, lon
}

// convertUPERPosition converts a UPER position to the output format.
func convertUPERPosition(p *UPERPosition) *Position {
	if p == nil {
		return nil
	}
	pos := &Position{}
	switch {
	case p.FixName != nil:
		pos.Type, pos.Name = "fix", *p.FixName
	case p.Navaid != nil:
		pos.Type, pos.Name = "navaid", *p.Navaid
	case p.Airport != nil:
		pos.Type, pos.Name = "airport", *p.Airport
	case p.LatitudeLongitude != nil:
		pos.Type = "latlon"
		lat, lon := convertUPERLatLon(p.LatitudeLongitude)
		pos.Latitude, pos.Longitude = &lat, &lon
	case p.PlaceBearingDistance != nil:
		return convertUPERPlaceBearingDistance(p.PlaceBearingDistance)
	}
	return pos
}

// convertUPERPlaceBearingDistance converts a fix, its optional coordinates,
// a bearing and a distance to a position of type "place_bearing_distance".
func convertUPERPlaceBearingDistance(pbd *UPERPlaceBearingDistance) *Position {
	pos := &Position{
		Type:     "place_bearing_distance",
		Name:     pbd.FixName,
		Bearing:  convertUPERDegrees(&pbd.Degrees),
		Distance: convertUPERDistance(&pbd.Distance),
	}
	if pbd.LatitudeLongitude != nil {
		lat, lon := convertUPERLatLon(pbd.LatitudeLongitude)
		pos.Latitude, pos.Longitude = &lat, &lon
	}
	return pos
}

// convertUPERPublishedIdentifier converts a fix and its optional
// coordinates to a position of type "fix".
func convertUPERPublishedIdentifier(p *UPERPublishedIdentifier) *Position {
	pos := &Position{Type: "fix", Name: p.FixName}
	if p.LatitudeLongitude != nil {
		lat, lon := convertUPERLatLon(p.LatitudeLongitude)
		pos.Latitude, pos.Longitude = &lat, &lon
	}
	return pos
}

// convertUPERPlaceBearings converts the two fixes and bearings of a
// FANSPlaceBearingPlaceBearing.
func convertUPERPlaceBearings(pbpb *UPERPlaceBearingPlaceBearing) []PlaceBearing {
	out := make([]PlaceBearing, 0, 2)
	for _, pb := range []*UPERPlaceBearing{&pbpb.PlaceBearing1, &pbpb.PlaceBearing2} {
		item := PlaceBearing{FixName: pb.FixName, Bearing: convertUPERDegrees(&pb.Degrees)}
		if pb.LatitudeLongitude != nil {
			lat, lon := convertUPERLatLon(pb.LatitudeLongitude)
			item.Latitude, item.Longitude = &lat, &lon
		}
		out = append(out, item)
	}
	return out
}

// convertUPERProcedureName converts a procedure name.
func convertUPERProcedureName(p *UPERProcedureName) *ProcedureName {
	if p == nil {
		return nil
	}
	proc := &ProcedureName{Type: enumName(p.ProcedureType), Name: p.Procedure}
	if p.ProcedureTransition != nil {
		proc.Transition = *p.ProcedureTransition
	}
	return proc
}

// convertUPERRunway converts a runway.
func convertUPERRunway(r *UPERRunway) *Runway {
	if r == nil {
		return nil
	}
	return &Runway{Direction: r.Direction, Configuration: enumName(r.Configuration)}
}

// convertUPERRouteClearance converts a route clearance.
func convertUPERRouteClearance(r *UPERRouteClearance) *RouteClearance {
	rc := &RouteClearance{
		RunwayDeparture:     convertUPERRunway(r.RunwayDeparture),
		ProcedureDeparture:  convertUPERProcedureName(r.ProcedureDeparture),
		RunwayArrival:       convertUPERRunway(r.RunwayArrival),
		ProcedureApproach:   convertUPERProcedureName(r.ProcedureApproach),
		ProcedureArrival:    convertUPERProcedureName(r.ProcedureArrival),
		RouteInfoAdditional: convertUPERRouteInformationAdditional(r.RouteInformationAdditional),
	}
	if r.AirportDeparture != nil {
		rc.AirportDeparture = *r.AirportDeparture
	}
	if r.AirportDestination != nil {
		rc.AirportDestination = *r.AirportDestination
	}
	if r.AirwayIntercept != nil {
		rc.AirwayIntercept = *r.AirwayIntercept
	}
	route := deref(r.RouteInformation)
	for i := range route {
		rc.RouteInformation = append(rc.RouteInformation, convertUPERRouteInformation(&route[i]))
	}
	return rc
}

// convertUPERRouteInformation converts one element of a route.
func convertUPERRouteInformation(r *UPERRouteInformation) RouteInformation {
	switch {
	case r.PublishedIdentifier != nil:
		return RouteInformation{Type: "published_identifier", Position: convertUPERPublishedIdentifier(r.PublishedIdentifier)}
	case r.LatitudeLongitude != nil:
		return RouteInformation{Type: "latitude_longitude", Position: convertUPERPosition(&UPERPosition{LatitudeLongitude: r.LatitudeLongitude})}
	case r.PlaceBearingPlaceBearing != nil:
		return RouteInformation{Type: "place_bearing_place_bearing", PlaceBearings: convertUPERPlaceBearings(r.PlaceBearingPlaceBearing)}
	case r.PlaceBearingDistance != nil:
		return RouteInformation{Type: "place_bearing_distance", Position: convertUPERPlaceBearingDistance(r.PlaceBearingDistance)}
	case r.AirwayIdentifier != nil:
		return RouteInformation{Type: "airway_identifier", Airway: *r.AirwayIdentifier}
	case r.TrackDetail != nil:
		track := &TrackDetail{Name: r.TrackDetail.TrackName}
		for i := range r.TrackDetail.LatitudeLongitude {
			lat, lon := convertUPERLatLon(&r.TrackDetail.LatitudeLongitude[i])
			track.Points = append(track.Points, LatLon{Latitude: lat, Longitude: lon})
		}
		return RouteInformation{Type: "track_detail", Track: track}
	}
	return RouteInformation{}
}

// convertUPERATWAltitudes converts altitudes with tolerances.
func convertUPERATWAltitudes(seq *[]UPERATWAltitude) []ToleranceAltitude {
	var out []ToleranceAltitude
	alts := deref(seq)
	for i := range alts {
		out = append(out, convertUPERATWAltitude(&alts[i]))
	}
	return out
}

// convertUPERATWAltitude converts an altitude with a tolerance.
func convertUPERATWAltitude(a *UPERATWAltitude) ToleranceAltitude {
	return ToleranceAltitude{Tolerance: enumName(a.ATWAltitudeTolerance), Altitude: convertUPERAltitude(&a.Altitude)}
}

// convertUPERRouteInformationAdditional converts the additional route
// constraints of a route clearance.
func convertUPERRouteInformationAdditional(r *UPERRouteInformationAdditional) *RouteInformationAdditional {
	if r == nil {
		return nil
	}
	out := &RouteInformationAdditional{}
	waypoints := deref(r.ATWAlongTrackWaypoints)
	for i := range waypoints {
		w := &waypoints[i]
		out.AlongTrackWaypoints = append(out.AlongTrackWaypoints, AlongTrackWaypoint{
			Position:          convertUPERPosition(&w.Position),
			DistanceTolerance: enumName(w.ATWDistance.ATWDistanceTolerance),
			Distance:          convertUPERDistance(&w.ATWDistance.Distance),
			Speed:             convertUPERSpeed(w.Speed),
			Altitudes:         convertUPERATWAltitudes(w.ATWAltitudes),
		})
	}
	if rp := r.ReportingPoints; rp != nil {
		points := &ReportingPoints{DegreeIncrement: rp.DegreeIncrement}
		switch ll := rp.LatLonReportingPoints; {
		case ll.LatitudeReportingPoints != nil:
			points.Type = "latitude"
			points.Direction = enumName(ll.LatitudeReportingPoints.Direction)
			points.Degrees = ll.LatitudeReportingPoints.Degrees
		case ll.LongitudeReportingPoints != nil:
			points.Type = "longitude"
			points.Direction = enumName(ll.LongitudeReportingPoints.Direction)
			points.Degrees = ll.LongitudeReportingPoints.Degrees
		}
		out.ReportingPoints = points
	}
	courses := deref(r.InterceptCourseFromSequence)
	for i := range courses {
		c := &courses[i]
		from := convertUPERRouteInformation(&UPERRouteInformation{
			PublishedIdentifier:      c.Selection.PublishedIdentifier,
			LatitudeLongitude:        c.Selection.LatitudeLongitude,
			PlaceBearingPlaceBearing: c.Selection.PlaceBearingPlaceBearing,
			PlaceBearingDistance:     c.Selection.PlaceBearingDistance,
		})
		out.InterceptCourses = append(out.InterceptCourses, InterceptCourse{From: from, Degrees: convertUPERDegrees(&c.Degrees)})
	}
	holds := deref(r.HoldAtWaypointSequence)
	for i := range holds {
		h := &holds[i]
		hold := HoldAtWaypoint{
			Position:  convertUPERPosition(&h.Position),
			SpeedLow:  convertUPERSpeed(h.SpeedLow),
			SpeedHigh: convertUPERSpeed(h.SpeedHigh),
			Degrees:   convertUPERDegrees(h.Degrees),
			EFCTime:   convertUPERTime(h.EFCTime),
			LegType:   convertUPERLegType(h.LegType),
		}
		if h.ATWAltitude != nil {
			a := convertUPERATWAltitude(h.ATWAltitude)
			hold.Altitude = &a
		}
		if h.Direction != nil {
			hold.Direction = enumName(*h.Direction)
		}
		out.HoldsAtWaypoint = append(out.HoldsAtWaypoint, hold)
	}
	constraints := deref(r.WaypointSpeedAltitudeSequence)
	for i := range constraints {
		w := &constraints[i]
		out.WaypointSpeedAltitudes = append(out.WaypointSpeedAltitudes, WaypointSpeedAltitude{
			Position:  convertUPERPosition(&w.Position),
			Speed:     convertUPERSpeed(w.Speed),
			Altitudes: convertUPERATWAltitudes(w.ATWAltitudes),
		})
	}
	arrivals := deref(r.RTARequiredTimeArrivals)
	for i := range arrivals {
		a := &arrivals[i]
		rta := RequiredArrivalTime{
			Position:      convertUPERPosition(&a.Position),
			Time:          convertUPERTime(&a.RTATime.Time),
			TimeTolerance: enumName(a.RTATime.TimeTolerance),
		}
		if a.RTATolerance != nil {
			minutes := float64(*a.RTATolerance) / 10
			rta.ToleranceMinutes = &minutes
		}
		out.RequiredArrivalTimes = append(out.RequiredArrivalTimes, rta)
	}
	return out
}

// convertUPERLegType converts the leg type of a hold (tenths of a nautical
// mile, kilometres or tenths of a minute).
func convertUPERLegType(l *UPERLegType) *LegType {
	if l == nil {
		return nil
	}
	switch {
	case l.LegDistance != nil && l.LegDistance.LegDistanceEnglish != nil:
		return &LegType{Distance: &Distance{Value: float64(*l.LegDistance.LegDistanceEnglish) / 10, Unit: "nm"}}
	case l.LegDistance != nil && l.LegDistance.LegDistanceMetric != nil:
		return &LegType{Distance: &Distance{Value: float64(*l.LegDistance.LegDistanceMetric), Unit: "km"}}
	case l.LegTime != nil:
		minutes := float64(*l.LegTime) / 10
		return &LegType{Minutes: &minutes}
	}
	return &LegType{}
}

// convertUPERHoldClearance converts the hold clearance of uM91.
func convertUPERHoldClearance(h *UPERHoldClearance) *HoldClearance {
	return &HoldClearance{
		Position:  convertUPERPosition(&h.Position),
		Altitude:  convertUPERAltitude(&h.Altitude),
		Degrees:   convertUPERDegrees(&h.Degrees),
		Direction: enumName(h.Direction),
		LegType:   convertUPERLegType(h.LegType),
	}
}

// convertUPERPredepartureClearance converts the pre-departure clearance of
// uM73.
func convertUPERPredepartureClearance(p *UPERPredepartureClearance) *PredepartureClearance {
	pdc := &PredepartureClearance{
		FlightID:            p.AircraftFlightIdentification,
		DepartureTime:       convertUPERTime(&p.TimeDepartureEdct),
		RouteClearance:      convertUPERRouteClearance(&p.RouteClearance),
		AltitudeRestriction: convertUPERAltitude(p.AltitudeRestriction),
		DepartureFrequency:  &Frequency{Type: "vhf", Value: p.FrequencyDeparture},
		BeaconCode:          convertUPERBeaconCode(&p.BeaconCode),
		Revision:            p.PDCRevision,
	}
	if p.AircraftType != nil {
		pdc.AircraftType = *p.AircraftType
	}
	if e := p.AircraftEquipmentCode; e != nil {
		code := &EquipmentCode{COMNAVApproachAvailable: e.COMNAVApproachEquipmentAvailable, SSR: enumName(e.SSREquipmentAvailable)}
		for _, s := range deref(e.COMNAVEquipmentStatus) {
			code.COMNAVStatus = append(code.COMNAVStatus, enumName(s.Status))
		}
		pdc.EquipmentCode = code
	}
	return pdc
}

// convertUPERPositionReport converts a UPER position report to the output format.
func convertUPERPositionReport(pr *UPERPositionReport) *PositionReport {
	report := &PositionReport{
		Position:                 convertUPERPosition(&pr.PositionCurrent),
		Time:                     convertUPERTime(&pr.TimeAtPositionCurrent),
		Altitude:                 convertUPERAltitude(&pr.Altitude),
		FixNext:                  convertUPERPosition(pr.FixNext),
		FixNextETA:               convertUPERTime(pr.TimeEtaAtFixNext),
		FixNextPlusOne:           convertUPERPosition(pr.FixNextPlusOne),
		DestinationETA:           convertUPERTime(pr.TimeEtaDestination),
		Speed:                    convertUPERSpeed(pr.Speed),
		TrackAngle:               convertUPERDegrees(pr.TrackAngle),
		TrueHeading:              convertUPERDegrees(pr.TrueHeading),
		Distance:                 convertUPERDistance(pr.Distance),
		ReportedWaypointPosition: convertUPERPosition(pr.ReportedWaypointPosition),
		ReportedWaypointTime:     convertUPERTime(pr.ReportedWaypointTime),
		ReportedWaypointAltitude: convertUPERAltitude(pr.ReportedWaypointAltitude),
	}
	if pr.RemainingFuel != nil {
		report.RemainingFuel = &RemainingFuel{Hours: pr.RemainingFuel.Hours, Minutes: pr.RemainingFuel.Minutes}
	}
	if t := pr.Temperature; t != nil {
		switch {
		case t.TemperatureC != nil:
			report.Temperature = &Temperature{Value: *t.TemperatureC, Unit: "C"}
		case t.TemperatureF != nil:
			report.Temperature = &Temperature{Value: *t.TemperatureF, Unit: "F"}
		}
	}
	if w := pr.Winds; w != nil {
		wind := &Wind{Direction: w.WindDirection}
		switch {
		case w.WindSpeed.WindSpeedEnglish != nil:
			wind.Speed, wind.Unit = *w.WindSpeed.WindSpeedEnglish, "kt"
		case w.WindSpeed.WindSpeedMetric != nil:
			wind.Speed, wind.Unit = *w.WindSpeed.WindSpeedMetric, "km/h"
		}
		report.Wind = wind
	}
	if pr.Turbulence != nil {
		report.Turbulence = enumName(*pr.Turbulence)
	}
	if pr.Icing != nil {
		report.Icing = enumName(*pr.Icing)
	}
	if pr.SpeedGround != nil {
		report.GroundSpeed = &Speed{Type: "ground", Value: *pr.SpeedGround * 10}
	}
	if vc := pr.VerticalChange; vc != nil {
		report.VerticalChange = &VerticalChange{
			Direction: enumName(vc.VerticalDirection),
			Rate:      convertUPERVerticalRate(&vc.VerticalRate),
		}
	}
	if pr.SupplementaryInformation != nil {
		report.SupplementaryInformation = *pr.SupplementaryInformation
	}
	return report
}
