package cpdlc

import "fmt"

// FormatElementText formats the element into human-readable text by substituting
// placeholders in the label template with actual data values.
func FormatElementText(elem *MessageElement) string {
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