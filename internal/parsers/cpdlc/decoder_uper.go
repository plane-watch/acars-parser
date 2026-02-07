package cpdlc

import (
	"fmt"

	"github.com/shaneshort/go-asn/uper"
)

// DecodeWithUPER decodes a CPDLC message using the go-asn UPER library.
// This is the correct decoder for FANS-1/A which uses Unaligned PER.
//
// Direction detection strategy:
// 1. Try the provided direction first
// 2. Validate decoded elements are semantically valid (not "(reserved)" or unknown)
// 3. If validation fails, try the opposite direction
// 4. Return whichever direction produces valid elements
//
// This approach is necessary because ACARS metadata (BlockID, Label, link_direction)
// is often unreliable for determining CPDLC direction. The ASN.1 schemas for uplink
// and downlink differ, but some messages can decode without error in both schemas.
// Semantic validation (checking element labels) resolves the ambiguity.
func DecodeWithUPER(data []byte, direction MessageDirection) (*Message, error) {
	// Try primary direction first.
	primaryMsg := &Message{Direction: direction}
	var primaryResult *Message
	var primaryErr error
	var primaryValid bool

	if direction == DirectionUplink {
		primaryResult, primaryErr = decodeUplinkUPER(data, primaryMsg)
		if primaryErr == nil {
			primaryValid = validateUplinkElements(primaryResult)
		}
	} else {
		primaryResult, primaryErr = decodeDownlinkUPER(data, primaryMsg)
		if primaryErr == nil {
			primaryValid = validateDownlinkElements(primaryResult)
		}
	}

	// If primary direction decoded and validated successfully, return it.
	if primaryErr == nil && primaryValid {
		return primaryResult, nil
	}

	// Try opposite direction as fallback.
	fallbackMsg := &Message{}
	var fallbackResult *Message
	var fallbackErr error
	var fallbackValid bool

	if direction == DirectionUplink {
		fallbackMsg.Direction = DirectionDownlink
		fallbackResult, fallbackErr = decodeDownlinkUPER(data, fallbackMsg)
		if fallbackErr == nil {
			fallbackValid = validateDownlinkElements(fallbackResult)
		}
	} else {
		fallbackMsg.Direction = DirectionUplink
		fallbackResult, fallbackErr = decodeUplinkUPER(data, fallbackMsg)
		if fallbackErr == nil {
			fallbackValid = validateUplinkElements(fallbackResult)
		}
	}

	// If fallback decoded and validated successfully, return it.
	if fallbackErr == nil && fallbackValid {
		return fallbackResult, nil
	}

	// Neither direction validated successfully.
	// Prefer returning a decoded result over an error, even if not validated.
	// This allows partial decodes to still provide some information.
	if primaryErr == nil {
		return primaryResult, nil
	}
	if fallbackErr == nil {
		return fallbackResult, nil
	}

	// Both failed to decode - return primary error.
	return nil, primaryErr
}

// validateUplinkElements checks if all decoded uplink elements are semantically valid.
// Returns false if any element has a "(reserved)" or empty label.
func validateUplinkElements(msg *Message) bool {
	if msg == nil || len(msg.Elements) == 0 {
		return false
	}
	for _, elem := range msg.Elements {
		label := GetUplinkLabel(elem.ID)
		if label == "(reserved)" || label == "" {
			return false
		}
	}
	return true
}

// validateDownlinkElements checks if all decoded downlink elements are semantically valid.
// Returns false if any element has a "(reserved)" or empty label.
func validateDownlinkElements(msg *Message) bool {
	if msg == nil || len(msg.Elements) == 0 {
		return false
	}
	for _, elem := range msg.Elements {
		label := GetDownlinkLabel(elem.ID)
		if label == "(reserved)" || label == "" {
			return false
		}
	}
	return true
}

// decodeUplinkUPER decodes an uplink message using UPER.
func decodeUplinkUPER(data []byte, msg *Message) (*Message, error) {
	var fansMsg UPERUplinkMessage
	if err := uper.Unmarshal(data, &fansMsg); err != nil {
		return nil, fmt.Errorf("uper unmarshal uplink: %w", err)
	}

	// Convert header.
	msg.Header = convertUPERHeader(&fansMsg.Header)

	// Convert primary element.
	elem, err := convertUPERUplinkElement(&fansMsg.Element)
	if err != nil {
		return nil, fmt.Errorf("convert uplink element: %w", err)
	}
	msg.Elements = append(msg.Elements, *elem)

	// Convert additional elements.
	for i, fansElem := range fansMsg.Elements {
		elem, err := convertUPERUplinkElement(&fansElem)
		if err != nil {
			return nil, fmt.Errorf("convert uplink element %d: %w", i, err)
		}
		msg.Elements = append(msg.Elements, *elem)
	}

	return msg, nil
}

// decodeDownlinkUPER decodes a downlink message using UPER.
func decodeDownlinkUPER(data []byte, msg *Message) (*Message, error) {
	var fansMsg UPERDownlinkMessage
	if err := uper.Unmarshal(data, &fansMsg); err != nil {
		return nil, fmt.Errorf("uper unmarshal downlink: %w", err)
	}

	// Convert header.
	msg.Header = convertUPERHeader(&fansMsg.Header)

	// Convert primary element.
	elem, err := convertUPERDownlinkElement(&fansMsg.Element)
	if err != nil {
		return nil, fmt.Errorf("convert downlink element: %w", err)
	}
	msg.Elements = append(msg.Elements, *elem)

	// Convert additional elements.
	for i, fansElem := range fansMsg.Elements {
		elem, err := convertUPERDownlinkElement(&fansElem)
		if err != nil {
			return nil, fmt.Errorf("convert downlink element %d: %w", i, err)
		}
		msg.Elements = append(msg.Elements, *elem)
	}

	return msg, nil
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
		header.Timestamp = &Time{
			Hours:   h.Timestamp.Hours,
			Minutes: h.Timestamp.Minutes,
			Seconds: h.Timestamp.Seconds,
		}
	}
	return header
}

// convertUPERUplinkElement converts a UPER uplink element to the output format.
func convertUPERUplinkElement(e *UPERUplinkElement) (*MessageElement, error) {
	// Simple acknowledgements (NULL types).
	if e.UM0Unable != nil {
		return &MessageElement{ID: 0, Label: GetUplinkLabel(0)}, nil
	}
	if e.UM1Standby != nil {
		return &MessageElement{ID: 1, Label: GetUplinkLabel(1)}, nil
	}
	if e.UM2RequestDeferred != nil {
		return &MessageElement{ID: 2, Label: GetUplinkLabel(2)}, nil
	}
	if e.UM3Roger != nil {
		return &MessageElement{ID: 3, Label: GetUplinkLabel(3)}, nil
	}
	if e.UM4Affirm != nil {
		return &MessageElement{ID: 4, Label: GetUplinkLabel(4)}, nil
	}
	if e.UM5Negative != nil {
		return &MessageElement{ID: 5, Label: GetUplinkLabel(5)}, nil
	}

	// Altitude-related messages.
	if e.UM6Altitude != nil {
		return makeUplinkElement(6, convertUPERAltitude(e.UM6Altitude)), nil
	}
	if e.UM19Altitude != nil {
		return makeUplinkElement(19, convertUPERAltitude(e.UM19Altitude)), nil
	}
	if e.UM20Altitude != nil {
		return makeUplinkElement(20, convertUPERAltitude(e.UM20Altitude)), nil
	}
	if e.UM23Altitude != nil {
		return makeUplinkElement(23, convertUPERAltitude(e.UM23Altitude)), nil
	}

	// Time messages.
	if e.UM7Time != nil {
		return makeUplinkElement(7, convertUPERTime(e.UM7Time)), nil
	}
	if e.UM9Time != nil {
		return makeUplinkElement(9, convertUPERTime(e.UM9Time)), nil
	}
	if e.UM11Time != nil {
		return makeUplinkElement(11, convertUPERTime(e.UM11Time)), nil
	}

	// Position messages.
	if e.UM8Position != nil {
		return makeUplinkElement(8, convertUPERPosition(e.UM8Position)), nil
	}
	if e.UM10Position != nil {
		return makeUplinkElement(10, convertUPERPosition(e.UM10Position)), nil
	}
	if e.UM12Position != nil {
		return makeUplinkElement(12, convertUPERPosition(e.UM12Position)), nil
	}
	if e.UM68Position != nil {
		return makeUplinkElement(68, convertUPERPosition(e.UM68Position)), nil
	}
	if e.UM70Position != nil {
		return makeUplinkElement(70, convertUPERPosition(e.UM70Position)), nil
	}
	if e.UM74Position != nil {
		return makeUplinkElement(74, convertUPERPosition(e.UM74Position)), nil
	}
	if e.UM75Position != nil {
		return makeUplinkElement(75, convertUPERPosition(e.UM75Position)), nil
	}
	if e.UM87Position != nil {
		return makeUplinkElement(87, convertUPERPosition(e.UM87Position)), nil
	}

	// Distance offset + direction messages.
	if e.UM64DistanceOffsetDirection != nil {
		return makeUplinkElement(64, convertUPERDistanceOffsetDirection(e.UM64DistanceOffsetDirection)), nil
	}
	if e.UM82DistanceOffsetDirection != nil {
		return makeUplinkElement(82, convertUPERDistanceOffsetDirection(e.UM82DistanceOffsetDirection)), nil
	}
	if e.UM152DistanceOffsetDirection != nil {
		return makeUplinkElement(152, convertUPERDistanceOffsetDirection(e.UM152DistanceOffsetDirection)), nil
	}

	// Speed messages.
	if e.UM106Speed != nil {
		return makeUplinkElement(106, convertUPERSpeed(e.UM106Speed)), nil
	}
	if e.UM108Speed != nil {
		return makeUplinkElement(108, convertUPERSpeed(e.UM108Speed)), nil
	}
	if e.UM109Speed != nil {
		return makeUplinkElement(109, convertUPERSpeed(e.UM109Speed)), nil
	}

	// Beacon code.
	if e.UM123BeaconCode != nil {
		return makeUplinkElement(123, convertUPERBeaconCode(e.UM123BeaconCode)), nil
	}

	// ICAO facility designation.
	if e.UM160Facility != nil {
		return makeUplinkElement(160, *e.UM160Facility), nil
	}

	// ATIS code.
	if e.UM158ATISCode != nil {
		return makeUplinkElement(158, *e.UM158ATISCode), nil
	}

	// Frequency.
	if e.UM157Frequency != nil {
		return makeUplinkElement(157, convertUPERFrequency(e.UM157Frequency)), nil
	}

	// Free text messages.
	if e.UM169FreeText != nil {
		elem := &MessageElement{
			ID:    169,
			Label: GetUplinkLabel(169),
			Data:  &FreeText{Text: *e.UM169FreeText},
		}
		elem.Text = *e.UM169FreeText
		return elem, nil
	}
	if e.UM170FreeTextDistress != nil {
		elem := &MessageElement{
			ID:    170,
			Label: GetUplinkLabel(170),
			Data:  &FreeText{Text: *e.UM170FreeTextDistress},
		}
		elem.Text = *e.UM170FreeTextDistress
		return elem, nil
	}

	// For other elements, find which one is set and return basic info.
	id := findUPERUplinkElementID(e)
	elem := &MessageElement{
		ID:    id,
		Label: GetUplinkLabel(id),
	}
	// Try to extract data for remaining types.
	elem.Data = extractUplinkData(e, id)
	if elem.Data != nil {
		elem.Text = FormatElementText(elem)
	}
	return elem, nil
}

// makeUplinkElement creates an uplink element with formatted text.
func makeUplinkElement(id int, data interface{}) *MessageElement {
	elem := &MessageElement{
		ID:    id,
		Label: GetUplinkLabel(id),
		Data:  data,
	}
	elem.Text = FormatElementText(elem)
	return elem
}

// extractUplinkData extracts data from uplink elements not handled above.
func extractUplinkData(e *UPERUplinkElement, id int) interface{} {
	// Position + Altitude combinations.
	if e.UM14PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM14PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM14PositionAltitude.Altitude),
		}
	}
	if e.UM16PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM16PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM16PositionAltitude.Altitude),
		}
	}
	if e.UM18PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM18PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM18PositionAltitude.Altitude),
		}
	}
	if e.UM22PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM22PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM22PositionAltitude.Altitude),
		}
	}
	if e.UM25PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM25PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM25PositionAltitude.Altitude),
		}
	}

	// Time + Altitude combinations.
	if e.UM13TimeAltitude != nil {
		return map[string]interface{}{
			"time":     convertUPERTime(&e.UM13TimeAltitude.Time),
			"altitude": convertUPERAltitude(&e.UM13TimeAltitude.Altitude),
		}
	}
	if e.UM15TimeAltitude != nil {
		return map[string]interface{}{
			"time":     convertUPERTime(&e.UM15TimeAltitude.Time),
			"altitude": convertUPERAltitude(&e.UM15TimeAltitude.Altitude),
		}
	}
	if e.UM17TimeAltitude != nil {
		return map[string]interface{}{
			"time":     convertUPERTime(&e.UM17TimeAltitude.Time),
			"altitude": convertUPERAltitude(&e.UM17TimeAltitude.Altitude),
		}
	}
	if e.UM21TimeAltitude != nil {
		return map[string]interface{}{
			"time":     convertUPERTime(&e.UM21TimeAltitude.Time),
			"altitude": convertUPERAltitude(&e.UM21TimeAltitude.Altitude),
		}
	}
	if e.UM24TimeAltitude != nil {
		return map[string]interface{}{
			"time":     convertUPERTime(&e.UM24TimeAltitude.Time),
			"altitude": convertUPERAltitude(&e.UM24TimeAltitude.Altitude),
		}
	}

	// Altitude + Altitude (block altitudes).
	if e.UM30AltitudeAltitude != nil {
		return map[string]interface{}{
			"altitude1": convertUPERAltitude(&e.UM30AltitudeAltitude.Altitude1),
			"altitude2": convertUPERAltitude(&e.UM30AltitudeAltitude.Altitude2),
		}
	}
	if e.UM31AltitudeAltitude != nil {
		return map[string]interface{}{
			"altitude1": convertUPERAltitude(&e.UM31AltitudeAltitude.Altitude1),
			"altitude2": convertUPERAltitude(&e.UM31AltitudeAltitude.Altitude2),
		}
	}
	if e.UM32AltitudeAltitude != nil {
		return map[string]interface{}{
			"altitude1": convertUPERAltitude(&e.UM32AltitudeAltitude.Altitude1),
			"altitude2": convertUPERAltitude(&e.UM32AltitudeAltitude.Altitude2),
		}
	}

	// Speed + Speed.
	if e.UM110SpeedSpeed != nil {
		return map[string]interface{}{
			"speed1": convertUPERSpeed(&e.UM110SpeedSpeed.Speed1),
			"speed2": convertUPERSpeed(&e.UM110SpeedSpeed.Speed2),
		}
	}

	// More altitude elements.
	if e.UM33Altitude != nil {
		return convertUPERAltitude(e.UM33Altitude)
	}
	if e.UM34Altitude != nil {
		return convertUPERAltitude(e.UM34Altitude)
	}
	if e.UM35Altitude != nil {
		return convertUPERAltitude(e.UM35Altitude)
	}
	if e.UM36Altitude != nil {
		return convertUPERAltitude(e.UM36Altitude)
	}
	if e.UM37Altitude != nil {
		return convertUPERAltitude(e.UM37Altitude)
	}
	if e.UM38Altitude != nil {
		return convertUPERAltitude(e.UM38Altitude)
	}
	if e.UM39Altitude != nil {
		return convertUPERAltitude(e.UM39Altitude)
	}
	if e.UM40Altitude != nil {
		return convertUPERAltitude(e.UM40Altitude)
	}
	if e.UM41Altitude != nil {
		return convertUPERAltitude(e.UM41Altitude)
	}
	if e.UM128Altitude != nil {
		return convertUPERAltitude(e.UM128Altitude)
	}
	if e.UM129Altitude != nil {
		return convertUPERAltitude(e.UM129Altitude)
	}
	if e.UM148Altitude != nil {
		return convertUPERAltitude(e.UM148Altitude)
	}
	if e.UM175Altitude != nil {
		return convertUPERAltitude(e.UM175Altitude)
	}

	// More position elements.
	if e.UM130Position != nil {
		return convertUPERPosition(e.UM130Position)
	}
	if e.UM155Position != nil {
		return convertUPERPosition(e.UM155Position)
	}

	// More time elements.
	if e.UM69Time != nil {
		return convertUPERTime(e.UM69Time)
	}
	if e.UM71Time != nil {
		return convertUPERTime(e.UM71Time)
	}
	if e.UM93Time != nil {
		return convertUPERTime(e.UM93Time)
	}

	// More speed elements.
	if e.UM111Speed != nil {
		return convertUPERSpeed(e.UM111Speed)
	}
	if e.UM112Speed != nil {
		return convertUPERSpeed(e.UM112Speed)
	}
	if e.UM113Speed != nil {
		return convertUPERSpeed(e.UM113Speed)
	}
	if e.UM114Speed != nil {
		return convertUPERSpeed(e.UM114Speed)
	}
	if e.UM115Speed != nil {
		return convertUPERSpeed(e.UM115Speed)
	}
	if e.UM151Speed != nil {
		return convertUPERSpeed(e.UM151Speed)
	}

	// More Position + Altitude.
	if e.UM42PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM42PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM42PositionAltitude.Altitude),
		}
	}
	if e.UM43PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM43PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM43PositionAltitude.Altitude),
		}
	}
	if e.UM44PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM44PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM44PositionAltitude.Altitude),
		}
	}
	if e.UM45PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM45PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM45PositionAltitude.Altitude),
		}
	}
	if e.UM46PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM46PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM46PositionAltitude.Altitude),
		}
	}
	if e.UM47PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM47PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM47PositionAltitude.Altitude),
		}
	}
	if e.UM48PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM48PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM48PositionAltitude.Altitude),
		}
	}
	if e.UM49PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM49PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM49PositionAltitude.Altitude),
		}
	}
	if e.UM92PositionAltitude != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM92PositionAltitude.Position),
			"altitude": convertUPERAltitude(&e.UM92PositionAltitude.Altitude),
		}
	}

	// Position + Time.
	if e.UM51PositionTime != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM51PositionTime.Position),
			"time":     convertUPERTime(&e.UM51PositionTime.Time),
		}
	}
	if e.UM52PositionTime != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM52PositionTime.Position),
			"time":     convertUPERTime(&e.UM52PositionTime.Time),
		}
	}
	if e.UM53PositionTime != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM53PositionTime.Position),
			"time":     convertUPERTime(&e.UM53PositionTime.Time),
		}
	}

	// Position + Speed.
	if e.UM55PositionSpeed != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM55PositionSpeed.Position),
			"speed":    convertUPERSpeed(&e.UM55PositionSpeed.Speed),
		}
	}
	if e.UM56PositionSpeed != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM56PositionSpeed.Position),
			"speed":    convertUPERSpeed(&e.UM56PositionSpeed.Speed),
		}
	}
	if e.UM57PositionSpeed != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM57PositionSpeed.Position),
			"speed":    convertUPERSpeed(&e.UM57PositionSpeed.Speed),
		}
	}
	if e.UM101PositionSpeed != nil {
		return map[string]interface{}{
			"position": convertUPERPosition(&e.UM101PositionSpeed.Position),
			"speed":    convertUPERSpeed(&e.UM101PositionSpeed.Speed),
		}
	}

	// Altitude + Time.
	if e.UM26AltitudeTime != nil {
		return map[string]interface{}{
			"altitude": convertUPERAltitude(&e.UM26AltitudeTime.Altitude),
			"time":     convertUPERTime(&e.UM26AltitudeTime.Time),
		}
	}
	if e.UM28AltitudeTime != nil {
		return map[string]interface{}{
			"altitude": convertUPERAltitude(&e.UM28AltitudeTime.Altitude),
			"time":     convertUPERTime(&e.UM28AltitudeTime.Time),
		}
	}
	if e.UM150AltitudeTime != nil {
		return map[string]interface{}{
			"altitude": convertUPERAltitude(&e.UM150AltitudeTime.Altitude),
			"time":     convertUPERTime(&e.UM150AltitudeTime.Time),
		}
	}

	// Altitude + Position.
	if e.UM27AltitudePosition != nil {
		return map[string]interface{}{
			"altitude": convertUPERAltitude(&e.UM27AltitudePosition.Altitude),
			"position": convertUPERPosition(&e.UM27AltitudePosition.Position),
		}
	}
	if e.UM29AltitudePosition != nil {
		return map[string]interface{}{
			"altitude": convertUPERAltitude(&e.UM29AltitudePosition.Altitude),
			"position": convertUPERPosition(&e.UM29AltitudePosition.Position),
		}
	}
	if e.UM78AltitudePosition != nil {
		return map[string]interface{}{
			"altitude": convertUPERAltitude(&e.UM78AltitudePosition.Altitude),
			"position": convertUPERPosition(&e.UM78AltitudePosition.Position),
		}
	}
	if e.UM90AltitudePosition != nil {
		return map[string]interface{}{
			"altitude": convertUPERAltitude(&e.UM90AltitudePosition.Altitude),
			"position": convertUPERPosition(&e.UM90AltitudePosition.Position),
		}
	}
	if e.UM149AltitudePosition != nil {
		return map[string]interface{}{
			"altitude": convertUPERAltitude(&e.UM149AltitudePosition.Altitude),
			"position": convertUPERPosition(&e.UM149AltitudePosition.Position),
		}
	}

	// Altitude + Speed.
	if e.UM102AltitudeSpeed != nil {
		return map[string]interface{}{
			"altitude": convertUPERAltitude(&e.UM102AltitudeSpeed.Altitude),
			"speed":    convertUPERSpeed(&e.UM102AltitudeSpeed.Speed),
		}
	}

	// Time + Position.
	if e.UM76TimePosition != nil {
		return map[string]interface{}{
			"time":     convertUPERTime(&e.UM76TimePosition.Time),
			"position": convertUPERPosition(&e.UM76TimePosition.Position),
		}
	}
	if e.UM89TimePosition != nil {
		return map[string]interface{}{
			"time":     convertUPERTime(&e.UM89TimePosition.Time),
			"position": convertUPERPosition(&e.UM89TimePosition.Position),
		}
	}

	// Position + Position.
	if e.UM77PositionPosition != nil {
		return map[string]interface{}{
			"position1": convertUPERPosition(&e.UM77PositionPosition.Position1),
			"position2": convertUPERPosition(&e.UM77PositionPosition.Position2),
		}
	}
	if e.UM88PositionPosition != nil {
		return map[string]interface{}{
			"position1": convertUPERPosition(&e.UM88PositionPosition.Position1),
			"position2": convertUPERPosition(&e.UM88PositionPosition.Position2),
		}
	}

	// Time + Speed.
	if e.UM100TimeSpeed != nil {
		return map[string]interface{}{
			"time":  convertUPERTime(&e.UM100TimeSpeed.Time),
			"speed": convertUPERSpeed(&e.UM100TimeSpeed.Speed),
		}
	}

	// More Altitude + Altitude.
	if e.UM180AltitudeAltitude != nil {
		return map[string]interface{}{
			"altitude1": convertUPERAltitude(&e.UM180AltitudeAltitude.Altitude1),
			"altitude2": convertUPERAltitude(&e.UM180AltitudeAltitude.Altitude2),
		}
	}

	// Direction + Degrees (heading instructions).
	if e.UM94DirectionDegrees != nil {
		return convertUPERDirectionDegrees(e.UM94DirectionDegrees)
	}
	if e.UM95DirectionDegrees != nil {
		return convertUPERDirectionDegrees(e.UM95DirectionDegrees)
	}
	if e.UM98DirectionDegrees != nil {
		return convertUPERDirectionDegrees(e.UM98DirectionDegrees)
	}

	return nil
}

// convertUPERDirectionDegrees converts direction + degrees for heading instructions.
func convertUPERDirectionDegrees(d *UPERDirectionDegrees) map[string]interface{} {
	if d == nil {
		return nil
	}
	direction := "left"
	if d.Direction == 1 {
		direction = "right"
	}
	deg := &Degrees{}
	if d.Degrees.DegreesMagnetic != nil {
		deg.Value = *d.Degrees.DegreesMagnetic
		deg.Magnetic = true
	} else if d.Degrees.DegreesTrue != nil {
		deg.Value = *d.Degrees.DegreesTrue
		deg.Magnetic = false
	}
	return map[string]interface{}{
		"direction": direction,
		"degrees":   deg,
	}
}

// convertUPERTime converts a UPER time to the output format.
func convertUPERTime(t *UPERTime) *Time {
	if t == nil {
		return nil
	}
	return &Time{
		Hours:   t.Hours,
		Minutes: t.Minutes,
	}
}

// convertUPERDistanceOffsetDirection converts distance offset + direction.
func convertUPERDistanceOffsetDirection(d *UPERDistanceOffsetDirection) *DistanceOffset {
	if d == nil {
		return nil
	}
	offset := &DistanceOffset{}

	if d.DistanceOffset.DistanceOffsetNm != nil {
		offset.Distance = *d.DistanceOffset.DistanceOffsetNm
		offset.Unit = "nm"
	} else if d.DistanceOffset.DistanceOffsetKm != nil {
		offset.Distance = *d.DistanceOffset.DistanceOffsetKm
		offset.Unit = "km"
	}

	dirNames := []string{"left", "right", "either side", "north", "south", "east", "west", "north-east", "north-west", "south-east", "south-west"}
	if d.Direction.Value >= 0 && d.Direction.Value < len(dirNames) {
		offset.Direction = dirNames[d.Direction.Value]
	}

	return offset
}

// convertUPERBeaconCode converts a beacon code.
func convertUPERBeaconCode(b *UPERBeaconCode) *BeaconCode {
	if b == nil {
		return nil
	}
	code := fmt.Sprintf("%d%d%d%d", b.Digit1, b.Digit2, b.Digit3, b.Digit4)
	return &BeaconCode{Code: code}
}

// convertUPERFrequency converts a frequency.
func convertUPERFrequency(f *UPERFrequency) *Frequency {
	if f == nil {
		return nil
	}
	freq := &Frequency{}
	switch {
	case f.FrequencyHF != nil:
		freq.Type = "hf"
		freq.Value = *f.FrequencyHF
	case f.FrequencyVHF != nil:
		freq.Type = "vhf"
		freq.Value = *f.FrequencyVHF
	case f.FrequencyUHF != nil:
		freq.Type = "uhf"
		freq.Value = *f.FrequencyUHF
	case f.FrequencySatChan != nil:
		freq.Type = "satcom"
		// For satcom, we don't have a numeric value.
	}
	return freq
}

// findUPERUplinkElementID finds the element ID by checking which field is set.
func findUPERUplinkElementID(e *UPERUplinkElement) int {
	// Check all fields and return the ID of the one that's set.
	if e.UM7Time != nil {
		return 7
	}
	if e.UM8Position != nil {
		return 8
	}
	if e.UM9Time != nil {
		return 9
	}
	if e.UM10Position != nil {
		return 10
	}
	if e.UM11Time != nil {
		return 11
	}
	if e.UM12Position != nil {
		return 12
	}
	if e.UM13TimeAltitude != nil {
		return 13
	}
	if e.UM14PositionAltitude != nil {
		return 14
	}
	if e.UM15TimeAltitude != nil {
		return 15
	}
	if e.UM16PositionAltitude != nil {
		return 16
	}
	if e.UM17TimeAltitude != nil {
		return 17
	}
	if e.UM18PositionAltitude != nil {
		return 18
	}
	if e.UM19Altitude != nil {
		return 19
	}
	if e.UM20Altitude != nil {
		return 20
	}
	if e.UM21TimeAltitude != nil {
		return 21
	}
	if e.UM22PositionAltitude != nil {
		return 22
	}
	if e.UM23Altitude != nil {
		return 23
	}
	if e.UM24TimeAltitude != nil {
		return 24
	}
	if e.UM25PositionAltitude != nil {
		return 25
	}
	if e.UM26AltitudeTime != nil {
		return 26
	}
	if e.UM27AltitudePosition != nil {
		return 27
	}
	if e.UM28AltitudeTime != nil {
		return 28
	}
	if e.UM29AltitudePosition != nil {
		return 29
	}
	if e.UM30AltitudeAltitude != nil {
		return 30
	}
	if e.UM31AltitudeAltitude != nil {
		return 31
	}
	if e.UM32AltitudeAltitude != nil {
		return 32
	}
	if e.UM33Altitude != nil {
		return 33
	}
	if e.UM34Altitude != nil {
		return 34
	}
	if e.UM35Altitude != nil {
		return 35
	}
	if e.UM36Altitude != nil {
		return 36
	}
	if e.UM37Altitude != nil {
		return 37
	}
	if e.UM38Altitude != nil {
		return 38
	}
	if e.UM39Altitude != nil {
		return 39
	}
	if e.UM40Altitude != nil {
		return 40
	}
	if e.UM41Altitude != nil {
		return 41
	}
	if e.UM42PositionAltitude != nil {
		return 42
	}
	if e.UM43PositionAltitude != nil {
		return 43
	}
	if e.UM44PositionAltitude != nil {
		return 44
	}
	if e.UM45PositionAltitude != nil {
		return 45
	}
	if e.UM46PositionAltitude != nil {
		return 46
	}
	if e.UM47PositionAltitude != nil {
		return 47
	}
	if e.UM48PositionAltitude != nil {
		return 48
	}
	if e.UM49PositionAltitude != nil {
		return 49
	}
	if e.UM50PositionAltitudeAltitude != nil {
		return 50
	}
	if e.UM51PositionTime != nil {
		return 51
	}
	if e.UM52PositionTime != nil {
		return 52
	}
	if e.UM53PositionTime != nil {
		return 53
	}
	if e.UM54PositionTimeTime != nil {
		return 54
	}
	if e.UM55PositionSpeed != nil {
		return 55
	}
	if e.UM56PositionSpeed != nil {
		return 56
	}
	if e.UM57PositionSpeed != nil {
		return 57
	}
	if e.UM58PositionTimeAltitude != nil {
		return 58
	}
	if e.UM59PositionTimeAltitude != nil {
		return 59
	}
	if e.UM60PositionTimeAltitude != nil {
		return 60
	}
	if e.UM61PositionAltitudeSpeed != nil {
		return 61
	}
	if e.UM62TimePositionAltitude != nil {
		return 62
	}
	if e.UM63TimePositionAltitudeSpeed != nil {
		return 63
	}
	if e.UM64DistanceOffsetDirection != nil {
		return 64
	}
	if e.UM65PositionDistanceOffsetDirection != nil {
		return 65
	}
	if e.UM66TimeDistanceOffsetDirection != nil {
		return 66
	}
	if e.UM67ProceedBackOnRoute != nil {
		return 67
	}
	if e.UM68Position != nil {
		return 68
	}
	if e.UM69Time != nil {
		return 69
	}
	if e.UM70Position != nil {
		return 70
	}
	if e.UM71Time != nil {
		return 71
	}
	if e.UM72ResumeOwnNav != nil {
		return 72
	}
	if e.UM73PDC != nil {
		return 73
	}
	if e.UM74Position != nil {
		return 74
	}
	if e.UM75Position != nil {
		return 75
	}
	if e.UM76TimePosition != nil {
		return 76
	}
	if e.UM77PositionPosition != nil {
		return 77
	}
	if e.UM78AltitudePosition != nil {
		return 78
	}
	if e.UM79PositionRouteClearance != nil {
		return 79
	}
	if e.UM80RouteClearance != nil {
		return 80
	}
	if e.UM81ProcedureName != nil {
		return 81
	}
	if e.UM82DistanceOffsetDirection != nil {
		return 82
	}
	if e.UM83PositionRouteClearance != nil {
		return 83
	}
	if e.UM84PositionProcedureName != nil {
		return 84
	}
	if e.UM85RouteClearance != nil {
		return 85
	}
	if e.UM86PositionRouteClearance != nil {
		return 86
	}
	if e.UM87Position != nil {
		return 87
	}
	if e.UM88PositionPosition != nil {
		return 88
	}
	if e.UM89TimePosition != nil {
		return 89
	}
	if e.UM90AltitudePosition != nil {
		return 90
	}
	if e.UM91HoldClearance != nil {
		return 91
	}
	if e.UM92PositionAltitude != nil {
		return 92
	}
	if e.UM93Time != nil {
		return 93
	}
	if e.UM94DirectionDegrees != nil {
		return 94
	}
	if e.UM95DirectionDegrees != nil {
		return 95
	}
	if e.UM96FlyPresentHdg != nil {
		return 96
	}
	if e.UM97PositionDegrees != nil {
		return 97
	}
	if e.UM98DirectionDegrees != nil {
		return 98
	}
	if e.UM99ProcedureName != nil {
		return 99
	}
	if e.UM100TimeSpeed != nil {
		return 100
	}
	if e.UM101PositionSpeed != nil {
		return 101
	}
	if e.UM102AltitudeSpeed != nil {
		return 102
	}
	if e.UM103TimeSpeedSpeed != nil {
		return 103
	}
	if e.UM104PositionSpeedSpeed != nil {
		return 104
	}
	if e.UM105AltitudeSpeedSpeed != nil {
		return 105
	}
	if e.UM106Speed != nil {
		return 106
	}
	if e.UM107MaintainSpd != nil {
		return 107
	}
	if e.UM108Speed != nil {
		return 108
	}
	if e.UM109Speed != nil {
		return 109
	}
	if e.UM110SpeedSpeed != nil {
		return 110
	}
	if e.UM111Speed != nil {
		return 111
	}
	if e.UM112Speed != nil {
		return 112
	}
	if e.UM113Speed != nil {
		return 113
	}
	if e.UM114Speed != nil {
		return 114
	}
	if e.UM115Speed != nil {
		return 115
	}
	if e.UM116ResumeNormal != nil {
		return 116
	}
	if e.UM117UnitFreq != nil {
		return 117
	}
	if e.UM118PositionUnitFreq != nil {
		return 118
	}
	if e.UM119TimeUnitFreq != nil {
		return 119
	}
	if e.UM120UnitFreq != nil {
		return 120
	}
	if e.UM121PositionUnitFreq != nil {
		return 121
	}
	if e.UM122TimeUnitFreq != nil {
		return 122
	}
	if e.UM123BeaconCode != nil {
		return 123
	}
	if e.UM124StopSquawk != nil {
		return 124
	}
	if e.UM125SquawkAlt != nil {
		return 125
	}
	if e.UM126StopAltSquawk != nil {
		return 126
	}
	if e.UM127ReportBack != nil {
		return 127
	}
	if e.UM128Altitude != nil {
		return 128
	}
	if e.UM129Altitude != nil {
		return 129
	}
	if e.UM130Position != nil {
		return 130
	}
	if e.UM131ReportFuelSouls != nil {
		return 131
	}
	if e.UM132ConfirmPosition != nil {
		return 132
	}
	if e.UM133ConfirmAltitude != nil {
		return 133
	}
	if e.UM134ConfirmSpeed != nil {
		return 134
	}
	if e.UM135ConfirmAssignAlt != nil {
		return 135
	}
	if e.UM136ConfirmAssignSpd != nil {
		return 136
	}
	if e.UM137ConfirmAssignRte != nil {
		return 137
	}
	if e.UM138ConfirmTimeWpt != nil {
		return 138
	}
	if e.UM139ConfirmRptWpt != nil {
		return 139
	}
	if e.UM140ConfirmNextWpt != nil {
		return 140
	}
	if e.UM141ConfirmNextETA != nil {
		return 141
	}
	if e.UM142ConfirmEnsuingWpt != nil {
		return 142
	}
	if e.UM143ConfirmRequest != nil {
		return 143
	}
	if e.UM144ConfirmSquawk != nil {
		return 144
	}
	if e.UM145ConfirmHeading != nil {
		return 145
	}
	if e.UM146ConfirmTrack != nil {
		return 146
	}
	if e.UM147RequestPosRpt != nil {
		return 147
	}
	if e.UM148Altitude != nil {
		return 148
	}
	if e.UM149AltitudePosition != nil {
		return 149
	}
	if e.UM150AltitudeTime != nil {
		return 150
	}
	if e.UM151Speed != nil {
		return 151
	}
	if e.UM152DistanceOffsetDirection != nil {
		return 152
	}
	if e.UM153Altimeter != nil {
		return 153
	}
	if e.UM154RadarTerminated != nil {
		return 154
	}
	if e.UM155Position != nil {
		return 155
	}
	if e.UM156RadarLost != nil {
		return 156
	}
	if e.UM157Frequency != nil {
		return 157
	}
	if e.UM158ATISCode != nil {
		return 158
	}
	if e.UM159ErrorInfo != nil {
		return 159
	}
	if e.UM160Facility != nil {
		return 160
	}
	if e.UM161EndService != nil {
		return 161
	}
	if e.UM162ServiceUnavail != nil {
		return 162
	}
	if e.UM163FacilityTP4 != nil {
		return 163
	}
	if e.UM164WhenReady != nil {
		return 164
	}
	if e.UM165Then != nil {
		return 165
	}
	if e.UM166DueToTraffic != nil {
		return 166
	}
	if e.UM167DueToAirspace != nil {
		return 167
	}
	if e.UM168Disregard != nil {
		return 168
	}
	if e.UM171VerticalRate != nil {
		return 171
	}
	if e.UM172VerticalRate != nil {
		return 172
	}
	if e.UM173VerticalRate != nil {
		return 173
	}
	if e.UM174VerticalRate != nil {
		return 174
	}
	if e.UM175Altitude != nil {
		return 175
	}
	if e.UM176MaintainOwnSep != nil {
		return 176
	}
	if e.UM177PilotsDiscretion != nil {
		return 177
	}
	if e.UM178Deleted != nil {
		return 178
	}
	if e.UM179SquawkIdent != nil {
		return 179
	}
	if e.UM180AltitudeAltitude != nil {
		return 180
	}
	if e.UM181ToFromPosition != nil {
		return 181
	}
	if e.UM182ConfirmATIS != nil {
		return 182
	}

	return -1 // Unknown element.
}

// convertUPERDownlinkElement converts a UPER downlink element to the output format.
func convertUPERDownlinkElement(e *UPERDownlinkElement) (*MessageElement, error) {
	// Find which element is set by checking each pointer field.
	// This is a CHOICE type - exactly one field will be non-nil.

	// dM0-dM5: Simple acknowledgements.
	if e.DM0Wilco != nil {
		return &MessageElement{ID: 0, Label: GetDownlinkLabel(0)}, nil
	}
	if e.DM1Unable != nil {
		return &MessageElement{ID: 1, Label: GetDownlinkLabel(1)}, nil
	}
	if e.DM2Standby != nil {
		return &MessageElement{ID: 2, Label: GetDownlinkLabel(2)}, nil
	}
	if e.DM3Roger != nil {
		return &MessageElement{ID: 3, Label: GetDownlinkLabel(3)}, nil
	}
	if e.DM4Affirm != nil {
		return &MessageElement{ID: 4, Label: GetDownlinkLabel(4)}, nil
	}
	if e.DM5Negative != nil {
		return &MessageElement{ID: 5, Label: GetDownlinkLabel(5)}, nil
	}

	// dM6-dM10: Altitude-related.
	if e.DM6RequestAltitude != nil {
		return &MessageElement{
			ID:    6,
			Label: GetDownlinkLabel(6),
			Data:  convertUPERAltitude(e.DM6RequestAltitude),
		}, nil
	}
	if e.DM7RequestBlock != nil {
		return &MessageElement{
			ID:    7,
			Label: GetDownlinkLabel(7),
			Data: map[string]interface{}{
				"altitude1": convertUPERAltitude(&e.DM7RequestBlock.Altitude1),
				"altitude2": convertUPERAltitude(&e.DM7RequestBlock.Altitude2),
			},
		}, nil
	}

	// dM48: Position Report - the key one!
	if e.DM48PositionReport != nil {
		elem := &MessageElement{
			ID:    48,
			Label: GetDownlinkLabel(48),
			Data:  convertUPERPositionReport(e.DM48PositionReport),
		}
		elem.Text = FormatElementText(elem)
		return elem, nil
	}

	// For other elements, return a basic element with the ID.
	// Find which one is set by checking all fields.
	id := findUPERDownlinkElementID(e)
	return &MessageElement{
		ID:    id,
		Label: GetDownlinkLabel(id),
	}, nil
}

// findUPERDownlinkElementID finds the element ID by checking which field is set.
func findUPERDownlinkElementID(e *UPERDownlinkElement) int {
	// Check all fields and return the ID of the one that's set.
	// This is a brute-force approach but works for now.

	if e.DM8RequestCruiseClimb != nil {
		return 8
	}
	if e.DM9RequestClimb != nil {
		return 9
	}
	if e.DM10RequestDescent != nil {
		return 10
	}
	if e.DM11AtPositionRequestClimb != nil {
		return 11
	}
	if e.DM12AtPositionRequestDescent != nil {
		return 12
	}
	if e.DM13AtTimeRequestClimb != nil {
		return 13
	}
	if e.DM14AtTimeRequestDescent != nil {
		return 14
	}
	if e.DM15RequestOffset != nil {
		return 15
	}
	if e.DM16AtPositionRequestOffset != nil {
		return 16
	}
	if e.DM17AtTimeRequestOffset != nil {
		return 17
	}
	if e.DM18RequestSpeed != nil {
		return 18
	}
	if e.DM19RequestSpeedRange != nil {
		return 19
	}
	if e.DM20RequestVoiceContact != nil {
		return 20
	}
	if e.DM21RequestVoiceFrequency != nil {
		return 21
	}
	if e.DM22RequestDirectTo != nil {
		return 22
	}
	if e.DM23RequestProcedure != nil {
		return 23
	}
	if e.DM24RequestRoute != nil {
		return 24
	}
	if e.DM25RequestClearance != nil {
		return 25
	}
	if e.DM26WeatherDeviationTo != nil {
		return 26
	}
	if e.DM27WeatherDeviationOffset != nil {
		return 27
	}
	if e.DM28Leaving != nil {
		return 28
	}
	if e.DM29ClimbingTo != nil {
		return 29
	}
	if e.DM30DescendingTo != nil {
		return 30
	}
	if e.DM31Passing != nil {
		return 31
	}
	if e.DM32PresentAltitude != nil {
		return 32
	}
	if e.DM33PresentPosition != nil {
		return 33
	}
	if e.DM34PresentSpeed != nil {
		return 34
	}
	if e.DM35PresentHeading != nil {
		return 35
	}
	if e.DM36PresentGroundTrack != nil {
		return 36
	}
	if e.DM37Level != nil {
		return 37
	}
	if e.DM38AssignedAltitude != nil {
		return 38
	}
	if e.DM39AssignedSpeed != nil {
		return 39
	}
	if e.DM40AssignedRoute != nil {
		return 40
	}
	if e.DM41BackOnRoute != nil {
		return 41
	}
	if e.DM42NextWaypoint != nil {
		return 42
	}
	if e.DM43NextWaypointETA != nil {
		return 43
	}
	if e.DM44EnsuingWaypoint != nil {
		return 44
	}
	if e.DM45ReportedWaypoint != nil {
		return 45
	}
	if e.DM46ReportedWaypointTime != nil {
		return 46
	}
	if e.DM47Squawking != nil {
		return 47
	}
	if e.DM49WhenCanWeExpectSpeed != nil {
		return 49
	}
	if e.DM50WhenCanWeExpectSpeedRange != nil {
		return 50
	}
	if e.DM51WhenBackOnRoute != nil {
		return 51
	}
	if e.DM52WhenLowerAltitude != nil {
		return 52
	}
	if e.DM53WhenHigherAltitude != nil {
		return 53
	}
	if e.DM54WhenCruiseClimb != nil {
		return 54
	}
	if e.DM55PanPanPan != nil {
		return 55
	}
	if e.DM56MaydayMayday != nil {
		return 56
	}
	if e.DM57FuelSouls != nil {
		return 57
	}
	if e.DM58CancelEmergency != nil {
		return 58
	}
	if e.DM59DivertingTo != nil {
		return 59
	}
	if e.DM60Offsetting != nil {
		return 60
	}
	if e.DM61DescendingTo2 != nil {
		return 61
	}
	if e.DM62Error != nil {
		return 62
	}
	if e.DM63NotCurrentDataAuthority != nil {
		return 63
	}
	if e.DM64Facility != nil {
		return 64
	}
	if e.DM65DueToWeather != nil {
		return 65
	}
	if e.DM66DueToPerformance != nil {
		return 66
	}
	if e.DM67FreeTextLow != nil {
		return 67
	}
	if e.DM68FreeTextDistress != nil {
		return 68
	}
	if e.DM69RequestVMCDescent != nil {
		return 69
	}
	if e.DM70RequestHeading != nil {
		return 70
	}
	if e.DM71RequestGroundTrack != nil {
		return 71
	}
	if e.DM72Reaching != nil {
		return 72
	}
	if e.DM73VersionNumber != nil {
		return 73
	}
	if e.DM74MaintainOwnSep != nil {
		return 74
	}
	if e.DM75AtPilotsDiscretion != nil {
		return 75
	}
	if e.DM76ReachingBlock != nil {
		return 76
	}
	if e.DM77AssignedBlock != nil {
		return 77
	}
	if e.DM78AtTimeDistance != nil {
		return 78
	}
	if e.DM79ATIS != nil {
		return 79
	}
	if e.DM80Deviating != nil {
		return 80
	}

	// Reserved elements dM81-dM128.
	if e.DM81Reserved != nil {
		return 81
	}
	if e.DM82Reserved != nil {
		return 82
	}
	if e.DM83Reserved != nil {
		return 83
	}
	if e.DM84Reserved != nil {
		return 84
	}
	if e.DM85Reserved != nil {
		return 85
	}
	if e.DM86Reserved != nil {
		return 86
	}
	if e.DM87Reserved != nil {
		return 87
	}
	if e.DM88Reserved != nil {
		return 88
	}
	if e.DM89Reserved != nil {
		return 89
	}
	if e.DM90Reserved != nil {
		return 90
	}
	if e.DM91Reserved != nil {
		return 91
	}
	if e.DM92Reserved != nil {
		return 92
	}
	if e.DM93Reserved != nil {
		return 93
	}
	if e.DM94Reserved != nil {
		return 94
	}
	if e.DM95Reserved != nil {
		return 95
	}
	if e.DM96Reserved != nil {
		return 96
	}
	if e.DM97Reserved != nil {
		return 97
	}
	if e.DM98Reserved != nil {
		return 98
	}
	if e.DM99Reserved != nil {
		return 99
	}
	if e.DM100Reserved != nil {
		return 100
	}
	if e.DM101Reserved != nil {
		return 101
	}
	if e.DM102Reserved != nil {
		return 102
	}
	if e.DM103Reserved != nil {
		return 103
	}
	if e.DM104Reserved != nil {
		return 104
	}
	if e.DM105Reserved != nil {
		return 105
	}
	if e.DM106Reserved != nil {
		return 106
	}
	if e.DM107Reserved != nil {
		return 107
	}
	if e.DM108Reserved != nil {
		return 108
	}
	if e.DM109Reserved != nil {
		return 109
	}
	if e.DM110Reserved != nil {
		return 110
	}
	if e.DM111Reserved != nil {
		return 111
	}
	if e.DM112Reserved != nil {
		return 112
	}
	if e.DM113Reserved != nil {
		return 113
	}
	if e.DM114Reserved != nil {
		return 114
	}
	if e.DM115Reserved != nil {
		return 115
	}
	if e.DM116Reserved != nil {
		return 116
	}
	if e.DM117Reserved != nil {
		return 117
	}
	if e.DM118Reserved != nil {
		return 118
	}
	if e.DM119Reserved != nil {
		return 119
	}
	if e.DM120Reserved != nil {
		return 120
	}
	if e.DM121Reserved != nil {
		return 121
	}
	if e.DM122Reserved != nil {
		return 122
	}
	if e.DM123Reserved != nil {
		return 123
	}
	if e.DM124Reserved != nil {
		return 124
	}
	if e.DM125Reserved != nil {
		return 125
	}
	if e.DM126Reserved != nil {
		return 126
	}
	if e.DM127Reserved != nil {
		return 127
	}
	if e.DM128Reserved != nil {
		return 128
	}

	return -1 // Unknown element.
}

// convertUPERAltitude converts a UPER altitude to the output format.
func convertUPERAltitude(a *UPERAltitude) *Altitude {
	if a == nil {
		return nil
	}

	alt := &Altitude{}
	switch {
	case a.AltitudeQNH != nil:
		alt.Type = "feet"
		alt.Value = *a.AltitudeQNH * 10
	case a.AltitudeQNHMeters != nil:
		alt.Type = "meters"
		alt.Value = *a.AltitudeQNHMeters
	case a.AltitudeQFE != nil:
		alt.Type = "feet"
		alt.Value = *a.AltitudeQFE * 10
	case a.AltitudeQFEMeters != nil:
		alt.Type = "meters"
		alt.Value = *a.AltitudeQFEMeters
	case a.AltitudeGNSSFeet != nil:
		alt.Type = "feet"
		alt.Value = *a.AltitudeGNSSFeet
	case a.AltitudeGNSSMeters != nil:
		alt.Type = "meters"
		alt.Value = *a.AltitudeGNSSMeters
	case a.AltitudeFlightLevel != nil:
		alt.Type = "flight_level"
		alt.Value = *a.AltitudeFlightLevel
	case a.AltitudeFlightLevelMetric != nil:
		alt.Type = "flight_level_metric"
		alt.Value = *a.AltitudeFlightLevelMetric
	}

	return alt
}

// convertUPERPositionReport converts a UPER position report to the output format.
func convertUPERPositionReport(pr *UPERPositionReport) *PositionReport {
	if pr == nil {
		return nil
	}

	report := &PositionReport{
		Position: convertUPERPosition(&pr.PositionCurrent),
	}

	// TimeAtPositionCurrent is mandatory.
	report.Time = &Time{
		Hours:   pr.TimeAtPositionCurrent.Hours,
		Minutes: pr.TimeAtPositionCurrent.Minutes,
	}

	// Altitude is mandatory.
	report.Altitude = convertUPERAltitude(&pr.Altitude)

	if pr.FixNext != nil {
		report.FixNext = convertUPERPosition(pr.FixNext)
	}

	if pr.TimeEtaAtFixNext != nil {
		report.FixNextETA = &Time{
			Hours:   pr.TimeEtaAtFixNext.Hours,
			Minutes: pr.TimeEtaAtFixNext.Minutes,
		}
	}

	if pr.FixNextPlusOne != nil {
		report.FixNextPlusOne = convertUPERPosition(pr.FixNextPlusOne)
	}

	if pr.Speed != nil {
		report.Speed = convertUPERSpeed(pr.Speed)
	}

	if pr.Temperature != nil {
		temp := convertUPERTemperature(pr.Temperature)
		report.Temperature = &temp
	}

	if pr.Winds != nil {
		report.Wind = &Wind{
			Direction: convertUPERWindDirection(&pr.Winds.Direction),
			Speed:     convertUPERWindSpeed(&pr.Winds.Speed),
			Unit:      "kt",
		}
	}

	if pr.Turbulence != nil {
		turbulenceLabels := []string{"nil", "light", "moderate", "severe"}
		if *pr.Turbulence >= 0 && *pr.Turbulence < len(turbulenceLabels) {
			report.Turbulence = turbulenceLabels[*pr.Turbulence]
		}
	}

	if pr.Icing != nil {
		icingLabels := []string{"nil", "light", "moderate", "severe"}
		if *pr.Icing >= 0 && *pr.Icing < len(icingLabels) {
			report.Icing = icingLabels[*pr.Icing]
		}
	}

	return report
}

// convertUPERTemperature converts a UPER temperature to an int value.
func convertUPERTemperature(t *UPERTemperature) int {
	if t == nil {
		return 0
	}
	if t.TemperatureC != nil {
		return *t.TemperatureC
	}
	if t.TemperatureFahren != nil {
		// Convert Fahrenheit to Celsius.
		return (*t.TemperatureFahren - 32) * 5 / 9
	}
	return 0
}

// convertUPERWindDirection converts a UPER wind direction to degrees.
func convertUPERWindDirection(d *UPERWindDirection) int {
	if d == nil {
		return 0
	}
	if d.DegreesMagnetic != nil {
		return *d.DegreesMagnetic
	}
	if d.DegreesTrue != nil {
		return *d.DegreesTrue
	}
	return 0
}

// convertUPERPosition converts a UPER position to the output format.
func convertUPERPosition(p *UPERPosition) *Position {
	if p == nil {
		return nil
	}

	pos := &Position{}
	switch {
	case p.FixName != nil:
		pos.Type = "fix"
		pos.Name = *p.FixName
	case p.Navaid != nil:
		pos.Type = "navaid"
		pos.Name = *p.Navaid
	case p.Airport != nil:
		pos.Type = "airport"
		pos.Name = *p.Airport
	case p.LatitudeLongitude != nil:
		pos.Type = "latlon"
		ll := p.LatitudeLongitude
		lat := float64(ll.Latitude.Degrees)
		if ll.Latitude.MinutesTenths != nil {
			lat += float64(*ll.Latitude.MinutesTenths) / 600.0
		}
		if ll.Latitude.Direction == 1 {
			lat = -lat
		}

		lon := float64(ll.Longitude.Degrees)
		if ll.Longitude.MinutesTenths != nil {
			lon += float64(*ll.Longitude.MinutesTenths) / 600.0
		}
		if ll.Longitude.Direction == 1 {
			lon = -lon
		}

		pos.Latitude = &lat
		pos.Longitude = &lon
	case p.PlaceBearingDistance != nil:
		pos.Type = "place_bearing_distance"
		pbd := p.PlaceBearingDistance
		pos.Name = pbd.FixName
		var bearing int
		if pbd.Degrees.DegreesMagnetic != nil {
			bearing = *pbd.Degrees.DegreesMagnetic
		} else if pbd.Degrees.DegreesTrue != nil {
			bearing = *pbd.Degrees.DegreesTrue
		}
		pos.Bearing = &bearing
		var distance int
		if pbd.Distance.DistanceNm != nil {
			distance = *pbd.Distance.DistanceNm / 10 // Convert from 0.1nm to nm.
			pos.DistanceUnit = "nm"
		} else if pbd.Distance.DistanceKm != nil {
			distance = *pbd.Distance.DistanceKm
			pos.DistanceUnit = "km"
		}
		pos.Distance = &distance
	}

	return pos
}

// convertUPERSpeed converts a UPER speed to the output format.
func convertUPERSpeed(s *UPERSpeed) *Speed {
	if s == nil {
		return nil
	}

	speed := &Speed{}
	switch {
	case s.SpeedIndicated != nil:
		speed.Type = "knots"
		speed.Value = *s.SpeedIndicated * 10
	case s.SpeedIndicatedMetric != nil:
		speed.Type = "kph"
		speed.Value = *s.SpeedIndicatedMetric * 10
	case s.SpeedTrue != nil:
		speed.Type = "knots"
		speed.Value = *s.SpeedTrue * 10
	case s.SpeedTrueMetric != nil:
		speed.Type = "kph"
		speed.Value = *s.SpeedTrueMetric * 10
	case s.SpeedGround != nil:
		speed.Type = "knots"
		speed.Value = *s.SpeedGround * 10
	case s.SpeedGroundMetric != nil:
		speed.Type = "kph"
		speed.Value = *s.SpeedGroundMetric * 10
	case s.SpeedMach != nil:
		speed.Type = "mach"
		speed.Value = *s.SpeedMach // Already in M.xx format.
	case s.SpeedMachLarge != nil:
		speed.Type = "mach"
		speed.Value = *s.SpeedMachLarge / 10 // Convert from M.xxx to M.xx format.
	}

	return speed
}

// convertUPERWindSpeed converts a UPER wind speed to the output format.
func convertUPERWindSpeed(s *UPERWindSpeed) int {
	if s == nil {
		return 0
	}
	if s.SpeedKt != nil {
		return *s.SpeedKt
	}
	if s.SpeedKmh != nil {
		// Convert km/h to kt (approximate).
		return int(float64(*s.SpeedKmh) * 0.539957)
	}
	return 0
}