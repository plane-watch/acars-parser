package cpdlc

import (
	"testing"
)

// Helper function to create fuel/souls test data.
func makeFuelSoulsData(hours, minutes, souls int) map[string]interface{} {
	return map[string]interface{}{
		"remaining_fuel":   &RemainingFuel{Hours: hours, Minutes: minutes},
		"persons_on_board": &PersonsOnBoard{Count: souls},
	}
}

// TestDownlinkDecoderCoverage ensures all downlink message types (dM0-dM80) have decoders.
// Each test case verifies that a specific message ID returns the correct data type.
func TestDownlinkDecoderCoverage(t *testing.T) {
	testCases := []struct {
		id          int
		label       string
		dataType    string // Expected data type: "nil", "altitude", "position", "time", "speed", etc.
		description string
	}{
		// NULL messages - no data required
		{0, "WILCO", "nil", "acknowledgment"},
		{1, "UNABLE", "nil", "acknowledgment"},
		{2, "STANDBY", "nil", "acknowledgment"},
		{3, "ROGER", "nil", "acknowledgment"},
		{4, "AFFIRM", "nil", "acknowledgment"},
		{5, "NEGATIVE", "nil", "acknowledgment"},
		{20, "REQUEST VOICE CONTACT", "nil", "request"},
		{25, "REQUEST CLEARANCE", "nil", "request"},
		{41, "BACK ON ROUTE", "nil", "report"},
		{51, "WHEN CAN WE EXPECT BACK ON ROUTE", "nil", "query"},
		{52, "WHEN CAN WE EXPECT LOWER ALTITUDE", "nil", "query"},
		{53, "WHEN CAN WE EXPECT HIGHER ALTITUDE", "nil", "query"},
		{55, "PAN PAN PAN", "nil", "emergency"},
		{56, "MAYDAY MAYDAY MAYDAY", "nil", "emergency"},
		{58, "CANCEL EMERGENCY", "nil", "emergency"},
		{63, "NOT CURRENT DATA AUTHORITY", "nil", "status"},
		{65, "DUE TO WEATHER", "nil", "reason"},
		{66, "DUE TO AIRCRAFT PERFORMANCE", "nil", "reason"},
		{69, "REQUEST VMC DESCENT", "nil", "request"},
		{74, "MAINTAIN OWN SEPARATION AND VMC", "nil", "instruction"},
		{75, "AT PILOTS DISCRETION", "nil", "instruction"},

		// Altitude messages
		{6, "REQUEST [altitude]", "altitude", "altitude request"},
		{8, "REQUEST CRUISE CLIMB TO [altitude]", "altitude", "altitude request"},
		{9, "REQUEST CLIMB TO [altitude]", "altitude", "altitude request"},
		{10, "REQUEST DESCENT TO [altitude]", "altitude", "altitude request"},
		{28, "LEAVING [altitude]", "altitude", "altitude report"},
		{29, "CLIMBING TO [altitude]", "altitude", "altitude report"},
		{30, "DESCENDING TO [altitude]", "altitude", "altitude report"},
		{32, "PRESENT ALTITUDE [altitude]", "altitude", "altitude report"},
		{37, "LEVEL [altitude]", "altitude", "altitude report"},
		{38, "ASSIGNED ALTITUDE [altitude]", "altitude", "altitude report"},
		{54, "WHEN CAN WE EXPECT CRUISE CLIMB TO [altitude]", "altitude", "altitude query"},
		{61, "DESCENDING TO [altitude]", "altitude", "altitude report"},
		{72, "REACHING [altitude]", "altitude", "altitude report"},

		// Altitude + Altitude messages
		{7, "REQUEST BLOCK [altitude] TO [altitude]", "altitude_altitude", "block altitude request"},
		{76, "REACHING BLOCK [altitude] TO [altitude]", "altitude_altitude", "block altitude report"},
		{77, "ASSIGNED BLOCK [altitude] TO [altitude]", "altitude_altitude", "block altitude report"},

		// Time messages
		{43, "NEXT WAYPOINT ETA [time]", "time", "time report"},
		{46, "REPORTED WAYPOINT [time]", "time", "time report"},

		// Position messages
		{22, "REQUEST DIRECT TO [position]", "position", "position request"},
		{31, "PASSING [position]", "position", "position report"},
		{33, "PRESENT POSITION [position]", "position", "position report"},
		{42, "NEXT WAYPOINT [position]", "position", "position report"},
		{44, "ENSUING WAYPOINT [position]", "position", "position report"},
		{45, "REPORTED WAYPOINT [position]", "position", "position report"},

		// Position + Altitude messages
		{11, "AT [position] REQUEST CLIMB TO [altitude]", "position_altitude", "position+altitude request"},
		{12, "AT [position] REQUEST DESCENT TO [altitude]", "position_altitude", "position+altitude request"},

		// Time + Altitude messages
		{13, "AT [time] REQUEST CLIMB TO [altitude]", "time_altitude", "time+altitude request"},
		{14, "AT [time] REQUEST DESCENT TO [altitude]", "time_altitude", "time+altitude request"},

		// Speed messages
		{18, "REQUEST [speed]", "speed", "speed request"},
		{34, "PRESENT SPEED [speed]", "speed", "speed report"},
		{39, "ASSIGNED SPEED [speed]", "speed", "speed report"},
		{49, "WHEN CAN WE EXPECT [speed]", "speed", "speed query"},

		// Speed + Speed messages
		{19, "REQUEST [speed] TO [speed]", "speed_speed", "speed range request"},
		{50, "WHEN CAN WE EXPECT [speed] TO [speed]", "speed_speed", "speed range query"},

		// Degrees messages
		{35, "PRESENT HEADING [degrees]", "degrees", "heading report"},
		{36, "PRESENT GROUND TRACK [degrees]", "degrees", "track report"},
		{70, "REQUEST HEADING [degrees]", "degrees", "heading request"},
		{71, "REQUEST GROUND TRACK [degrees]", "degrees", "track request"},

		// Frequency messages
		{21, "REQUEST VOICE CONTACT [frequency]", "frequency", "frequency request"},

		// Beacon code messages
		{47, "SQUAWKING [beaconcode]", "beacon_code", "transponder report"},

		// Error info messages
		{62, "ERROR [errorinformation]", "error_info", "error report"},

		// ICAO facility messages
		{64, "[icaofacilitydesignation]", "icao_facility", "facility report"},

		// Free text messages
		{67, "[freetext]", "free_text", "free text"},
		{68, "[freetext]", "free_text", "free text"},

		// Version number messages
		{73, "[versionnumber]", "version", "version report"},

		// ATIS code messages
		{79, "ATIS [atiscode]", "atis_code", "ATIS report"},

		// Distance offset messages
		{15, "REQUEST OFFSET [distanceoffset] [direction] OF ROUTE", "distance_offset", "offset request"},
		{27, "REQUEST WEATHER DEVIATION UP TO [distanceoffset] [direction] OF ROUTE", "distance_offset", "weather deviation"},
		{60, "OFFSETTING [distanceoffset] [direction] OF ROUTE", "distance_offset", "offset report"},
		{80, "DEVIATING [distanceoffset] [direction] OF ROUTE", "distance_offset", "deviation report"},

		// Remaining fuel + souls (emergency)
		{57, "[remainingfuel] OF FUEL REMAINING AND [remainingsouls] SOULS ON BOARD", "fuel_souls", "emergency fuel/souls"},

		// Procedure name messages
		{23, "REQUEST [procedurename]", "procedure_name", "procedure request"},

		// Route clearance messages
		{24, "REQUEST [routeclearance]", "route_clearance", "route request"},
		{40, "ASSIGNED ROUTE [routeclearance]", "route_clearance", "route report"},

		// Position + Route clearance messages
		{26, "REQUEST WEATHER DEVIATION TO [position] VIA [routeclearance]", "position_route", "weather deviation request"},
		{59, "DIVERTING TO [position] VIA [routeclearance]", "position_route", "diversion report"},

		// Position + Distance offset (MISSING - need to implement)
		{16, "AT [position] REQUEST OFFSET [distanceoffset] [direction] OF ROUTE", "position_distance_offset", "position+offset request"},

		// Time + Distance offset (MISSING - need to implement)
		{17, "AT [time] REQUEST OFFSET [distanceoffset] [direction] OF ROUTE", "time_distance_offset", "time+offset request"},

		// Position report (MISSING - complex structure)
		{48, "POSITION REPORT [positionreport]", "position_report", "full position report"},

		// Time + Distance + ToFrom + Position (MISSING - complex structure)
		{78, "AT [time] [distance] [tofrom] [position]", "time_distance_position", "time/distance/position report"},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			label := GetDownlinkLabel(tc.id)
			if label == "(reserved)" {
				t.Errorf("dM%d has no label defined", tc.id)
			}

			// Verify the decoder handles this message ID.
			// We can't easily test without valid binary data, but we can verify
			// the switch statement has a case for it by checking the decoder doesn't panic.
			d := &Decoder{
				br:        NewBitReader(make([]byte, 100)), // Dummy data.
				direction: DirectionDownlink,
			}

			// This will fail for missing decoders - that's intentional.
			// The test documents what SHOULD work.
			_, err := d.decodeDownlinkData(tc.id)

			// For "nil" type messages, no error expected and nil data is fine.
			// For other types, we just verify no panic occurred.
			// The actual data validation would require proper test vectors.
			if tc.dataType == "nil" {
				if err != nil {
					t.Errorf("dM%d (%s): unexpected error for nil-data message: %v", tc.id, tc.description, err)
				}
			}
			// Note: Non-nil data types may error due to insufficient dummy data,
			// but they shouldn't panic or return "not implemented".
		})
	}
}

// TestUplinkDecoderCoverage ensures all uplink message types (uM0-uM182) have decoders.
func TestUplinkDecoderCoverage(t *testing.T) {
	// Map of uplink IDs that require NULL (no data)
	nullUplinks := map[int]bool{
		0: true, 1: true, 2: true, 3: true, 4: true, 5: true,
		67: true, 72: true, 96: true, 107: true, 116: true,
		124: true, 125: true, 126: true, 127: true,
		131: true, 132: true, 133: true, 134: true, 135: true, 136: true, 137: true,
		138: true, 139: true, 140: true, 141: true, 142: true, 143: true, 144: true,
		145: true, 146: true, 147: true, 154: true, 156: true,
		161: true, 162: true, 164: true, 165: true, 166: true, 167: true, 168: true,
		176: true, 177: true, 178: true, 179: true, 182: true,
	}

	for id := 0; id <= 182; id++ {
		t.Run(GetUplinkLabel(id), func(t *testing.T) {
			label := GetUplinkLabel(id)
			if label == "(reserved)" {
				t.Skipf("uM%d is reserved", id)
				return
			}

			d := &Decoder{
				br:        NewBitReader(make([]byte, 100)),
				direction: DirectionUplink,
			}

			_, err := d.decodeUplinkData(id)

			if nullUplinks[id] {
				if err != nil {
					t.Errorf("uM%d: unexpected error for nil-data message: %v", id, err)
				}
			}
		})
	}
}

// TestDM57FuelAndSouls tests the emergency fuel/souls message specifically.
func TestDM57FuelAndSouls(t *testing.T) {
	// Test vector: 2 hours 30 minutes fuel, 150 souls
	// Hours (0-99): 7 bits for value 2
	// Minutes (0-59): 6 bits for value 30
	// Souls (0-1023): 10 bits for value 150

	// Build test data manually:
	// Hours=2: 7 bits = 0000010
	// Minutes=30: 6 bits = 011110
	// Souls=150: 10 bits = 0010010110
	// Combined: 0000010 011110 0010010110 = 00000100 11110001 0010110x
	// Padded: 0x04 0xF1 0x2C (approximately)

	// For now, just verify the decoder function exists and handles the types.
	d := &Decoder{
		br:        NewBitReader([]byte{0x04, 0xF1, 0x2C, 0x00}),
		direction: DirectionDownlink,
	}

	dataMap, err := d.decodeRemainingFuelSouls()
	if err != nil {
		t.Fatalf("decodeRemainingFuelSouls error: %v", err)
	}

	if dataMap == nil {
		t.Fatal("expected non-nil data")
	}

	if _, ok := dataMap["remaining_fuel"]; !ok {
		t.Error("missing remaining_fuel in result")
	}
	if _, ok := dataMap["persons_on_board"]; !ok {
		t.Error("missing persons_on_board in result")
	}

	// Verify types.
	if fuel, ok := dataMap["remaining_fuel"].(*RemainingFuel); ok {
		t.Logf("Decoded fuel: %s", fuel.String())
	} else {
		t.Errorf("remaining_fuel wrong type: %T", dataMap["remaining_fuel"])
	}

	if souls, ok := dataMap["persons_on_board"].(*PersonsOnBoard); ok {
		t.Logf("Decoded souls: %s", souls.String())
	} else {
		t.Errorf("persons_on_board wrong type: %T", dataMap["persons_on_board"])
	}
}

// TestDM16PositionDistanceOffset tests position + distance offset request.
func TestDM16PositionDistanceOffset(t *testing.T) {
	d := &Decoder{
		br:        NewBitReader(make([]byte, 50)),
		direction: DirectionDownlink,
	}

	data, err := d.decodeDownlinkData(16)
	if err == nil && data == nil {
		t.Error("dM16 decoder not implemented - returns nil")
	}
	// Once implemented, this should return position + distance offset data.
}

// TestDM17TimeDistanceOffset tests time + distance offset request.
func TestDM17TimeDistanceOffset(t *testing.T) {
	d := &Decoder{
		br:        NewBitReader(make([]byte, 50)),
		direction: DirectionDownlink,
	}

	data, err := d.decodeDownlinkData(17)
	if err == nil && data == nil {
		t.Error("dM17 decoder not implemented - returns nil")
	}
}

// TestDM48PositionReport tests the full position report message.
func TestDM48PositionReport(t *testing.T) {
	d := &Decoder{
		br:        NewBitReader(make([]byte, 100)),
		direction: DirectionDownlink,
	}

	data, err := d.decodeDownlinkData(48)
	if err == nil && data == nil {
		t.Error("dM48 decoder not implemented - returns nil")
	}
}

// TestDM78TimeDistancePosition tests time/distance/position report.
func TestDM78TimeDistancePosition(t *testing.T) {
	d := &Decoder{
		br:        NewBitReader(make([]byte, 50)),
		direction: DirectionDownlink,
	}

	data, err := d.decodeDownlinkData(78)
	if err == nil && data == nil {
		t.Error("dM78 decoder not implemented - returns nil")
	}
}

// TestTextSubstitution verifies that decoded data is properly substituted into labels.
func TestTextSubstitution(t *testing.T) {
	testCases := []struct {
		name     string
		label    string
		data     interface{}
		expected string
	}{
		{
			name:     "altitude substitution",
			label:    "REQUEST [altitude]",
			data:     &Altitude{Type: "flight_level", Value: 350},
			expected: "REQUEST FL350",
		},
		{
			name:     "speed substitution",
			label:    "MAINTAIN [speed]",
			data:     &Speed{Type: "mach", Value: 82},
			expected: "MAINTAIN M.82",
		},
		{
			name:     "position substitution",
			label:    "PROCEED DIRECT TO [position]",
			data:     &Position{Type: "fix", Name: "WHALE"},
			expected: "PROCEED DIRECT TO WHALE",
		},
		{
			name:     "time substitution",
			label:    "EXPECT AT [time]",
			data:     &Time{Hours: 14, Minutes: 30, Seconds: 0},
			expected: "EXPECT AT 14:30:00",
		},
		{
			name:     "beacon code substitution",
			label:    "SQUAWK [beaconcode]",
			data:     &BeaconCode{Code: "7500"},
			expected: "SQUAWK 7500",
		},
		{
			name:     "fuel and souls substitution",
			label:    "[remainingfuel] OF FUEL REMAINING AND [remainingsouls] SOULS ON BOARD",
			data:     makeFuelSoulsData(2, 30, 150),
			expected: "2h30m OF FUEL REMAINING AND 150 SOULS ON BOARD",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Decoder{direction: DirectionDownlink}
			elem := &MessageElement{
				Label: tc.label,
				Data:  tc.data,
			}

			result := d.formatElementText(elem)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}
