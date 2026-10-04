package cpdlc

import (
	"encoding/hex"
	"errors"
	"testing"
)

// TestDirectionDetection verifies that the decoder correctly determines message direction
// even when both schemas can decode the message without ASN.1 errors.
func TestDirectionDetection(t *testing.T) {
	tests := []struct {
		name          string
		hexStr        string
		inputDir      MessageDirection
		wantDirection MessageDirection
		wantElemID    int
		wantLabel     string
	}{
		{
			// This message decodes in both directions, but only uplink produces valid labels.
			// As uplink: element 127 = "REPORT BACK ON ROUTE" (valid)
			// As downlink: element 127 = "(reserved)" (invalid)
			name:          "Ambiguous message - should detect uplink",
			hexStr:        "262d8d9fc0",
			inputDir:      DirectionDownlink, // Wrong input direction
			wantDirection: DirectionUplink,   // Should correct to uplink
			wantElemID:    127,
			wantLabel:     "REPORT BACK ON ROUTE",
		},
		{
			// uM160 NEXT DATA AUTHORITY - only valid as uplink.
			name:          "uM160 NEXT DATA AUTHORITY",
			hexStr:        "232d8c2829509124",
			inputDir:      DirectionDownlink, // Wrong input direction
			wantDirection: DirectionUplink,   // Should correct to uplink
			wantElemID:    160,
			wantLabel:     "NEXT DATA AUTHORITY [icaofacilitydesignation]",
		},
		{
			// dM48 Position Report - only valid as downlink.
			name:          "dM48 Position Report",
			hexStr:        "243F880C3D903BB412903604FE326C2479F4A64F7F62528B1A9CF8382738186AC28B16668E013DF464D8",
			inputDir:      DirectionUplink,   // Wrong input direction
			wantDirection: DirectionDownlink, // Should correct to downlink
			wantElemID:    48,
			wantLabel:     "POSITION REPORT [positionreport]",
		},
		{
			// dM0 WILCO - only valid as downlink.
			name:          "dM0 WILCO with correct input",
			hexStr:        "6310A9940038D2", // From NATS sample
			inputDir:      DirectionDownlink,
			wantDirection: DirectionDownlink,
			wantElemID:    0,
			wantLabel:     "WILCO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := hex.DecodeString(tt.hexStr)
			if err != nil {
				t.Fatalf("Hex decode error: %v", err)
			}

			msg, err := DecodeWithUPER(data, tt.inputDir)
			if err != nil {
				t.Fatalf("DecodeWithUPER error: %v", err)
			}

			if msg.Direction != tt.wantDirection {
				t.Errorf("Direction = %v, want %v", msg.Direction, tt.wantDirection)
			}

			if len(msg.Elements) == 0 {
				t.Fatal("No elements decoded")
			}

			elem := msg.Elements[0]
			if elem.ID != tt.wantElemID {
				t.Errorf("Element ID = %d, want %d", elem.ID, tt.wantElemID)
			}

			if elem.Label != tt.wantLabel {
				t.Errorf("Label = %q, want %q", elem.Label, tt.wantLabel)
			}

			t.Logf("Correctly detected as %s: ID=%d Label=%s",
				msg.Direction, elem.ID, elem.Label)
		})
	}
}

// TestDirectionValidation verifies the validation functions work correctly.
func TestDirectionValidation(t *testing.T) {
	tests := []struct {
		name      string
		direction MessageDirection
		elemID    int
		wantValid bool
	}{
		// Valid uplink elements
		{"uM0 UNABLE", DirectionUplink, 0, true},
		{"uM127 REPORT BACK ON ROUTE", DirectionUplink, 127, true},
		{"uM160 NEXT DATA AUTHORITY", DirectionUplink, 160, true},
		{"uM182 CONFIRM ATIS", DirectionUplink, 182, true},

		// Invalid uplink elements (reserved or unknown)
		{"uM178 reserved", DirectionUplink, 178, false},
		{"uM200 unknown", DirectionUplink, 200, false},

		// Valid downlink elements
		{"dM0 WILCO", DirectionDownlink, 0, true},
		{"dM48 POSITION REPORT", DirectionDownlink, 48, true},
		{"dM80 DEVIATING", DirectionDownlink, 80, true},

		// Invalid downlink elements (reserved or unknown)
		// Note: dM87-88, dM90-97, dM101-106, dM108+ are reserved in downlink
		{"dM87 reserved", DirectionDownlink, 87, false},
		{"dM127 reserved", DirectionDownlink, 127, false},
		{"dM200 unknown", DirectionDownlink, 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				Direction: tt.direction,
				Elements:  []MessageElement{{ID: tt.elemID}},
			}

			var valid bool
			if tt.direction == DirectionUplink {
				valid = validateUplinkElements(msg)
			} else {
				valid = validateDownlinkElements(msg)
			}

			if valid != tt.wantValid {
				label := ""
				if tt.direction == DirectionUplink {
					label = GetUplinkLabel(tt.elemID)
				} else {
					label = GetDownlinkLabel(tt.elemID)
				}
				t.Errorf("valid = %v, want %v (label=%q)", valid, tt.wantValid, label)
			}
		})
	}
}

// TestDecodeWithUnknownDirection checks decoding when the direction is not
// known from the link layer, block ID or label: the message set that gives
// valid elements decides it, and a message both decode validly is ambiguous.
func TestDecodeWithUnknownDirection(t *testing.T) {
	// uM160 NEXT DATA AUTHORITY: valid only as an uplink.
	data, _ := hex.DecodeString("232d8c2829509124")
	msg, err := DecodeWithUPER(data, DirectionUnknown)
	if err != nil || msg.Direction != DirectionUplink {
		t.Errorf("uM160: direction %v, error %v; want uplink", msgDirection(msg), err)
	}

	// Element 0 is dM0 WILCO as a downlink and uM0 UNABLE as an uplink:
	// the two meanings are opposite, so neither is reported.
	data, _ = hex.DecodeString("6310A9940038D2")
	if msg, err := DecodeWithUPER(data, DirectionUnknown); !errors.Is(err, ErrAmbiguousDirection) {
		t.Errorf("element 0: direction %v, error %v; want ErrAmbiguousDirection", msgDirection(msg), err)
	}
	// With the direction known, it is decoded.
	if msg, err := DecodeWithUPER(data, DirectionDownlink); err != nil || msg.Elements[0].Label != "WILCO" {
		t.Errorf("element 0 as a downlink: %v, %v; want WILCO", msg, err)
	}
}

// TestDecodeRejectsInvalidElements checks that a message neither message set
// decodes to valid elements is an error, rather than a result with reserved
// elements. The payload is a real label H1 message from the January 2026
// corpus (N8830Q): the uplink decode fails and the downlink decode gives
// the reserved dM117.
func TestDecodeRejectsInvalidElements(t *testing.T) {
	data, _ := hex.DecodeString("22B432DD6F1A7D2A882BCFA5524062218018D4")
	data = data[:len(data)-2] // Strip the CRC.
	for _, dir := range []MessageDirection{DirectionUplink, DirectionDownlink, DirectionUnknown} {
		if msg, err := DecodeWithUPER(data, dir); !errors.Is(err, ErrNoValidElements) {
			t.Errorf("direction %v: got %+v, %v; want ErrNoValidElements", dir, msg, err)
		}
	}
}

func msgDirection(msg *Message) MessageDirection {
	if msg == nil {
		return DirectionUnknown
	}
	return msg.Direction
}
