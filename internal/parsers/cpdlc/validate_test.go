package cpdlc

import (
	"encoding/hex"
	"testing"
)

// TestValidCPDLCSamples decodes payloads (without the ARINC CRC) whose
// decoding libacars confirms (decode_acars_apps, with the payload wrapped
// in an AT1 message with a valid CRC).
func TestValidCPDLCSamples(t *testing.T) {
	samples := []struct {
		hexStr    string
		direction MessageDirection
		desc      string
		wantMsgID int
		wantTexts []string
	}{
		{
			// libacars sample (from cpdlc_get_position.c).
			hexStr:    "243F880C3D903BB412903604FE326C2479F4A64F7F62528B1A9CF8382738186AC28B16668E013DF464D8",
			direction: DirectionDownlink,
			desc:      "dM48 POSITION REPORT",
			wantMsgID: 8,
		},
		{
			hexStr:    "23BF9A682CCD9B34",
			direction: DirectionUplink,
			desc:      "uM160 NEXT DATA AUTHORITY",
			wantMsgID: 7,
			wantTexts: []string{"NEXT DATA AUTHORITY YMMM"},
		},
		{
			hexStr:    "E3BF000F520E21FC",
			direction: DirectionUplink,
			desc:      "uM82 CLEARED TO DEVIATE and uM127 REPORT BACK ON ROUTE",
			wantMsgID: 7,
			wantTexts: []string{"CLEARED TO DEVIATE UP TO 15 nm either side OF ROUTE", ""},
		},
		{
			hexStr:    "22C0659D52E9C69E01CC40",
			direction: DirectionUplink,
			desc:      "uM117 CONTACT with an HF frequency",
			wantMsgID: 5,
			wantTexts: []string{"CONTACT KSFO CENTER 6532 kHz"},
		},
	}

	for _, s := range samples {
		t.Run(s.desc, func(t *testing.T) {
			data, err := hex.DecodeString(s.hexStr)
			if err != nil {
				t.Fatalf("Hex decode error: %v", err)
			}
			msg, err := DecodeWithUPER(data, s.direction)
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}
			if msg.Direction != s.direction {
				t.Errorf("direction %v, want %v", msg.Direction, s.direction)
			}
			if msg.Header.MsgID != s.wantMsgID {
				t.Errorf("MsgID = %d, want %d", msg.Header.MsgID, s.wantMsgID)
			}
			if s.wantTexts == nil {
				return
			}
			if len(msg.Elements) != len(s.wantTexts) {
				t.Fatalf("%d elements, want %d", len(msg.Elements), len(s.wantTexts))
			}
			for i, want := range s.wantTexts {
				if got := msg.Elements[i].Text; got != want {
					t.Errorf("element %d text %q, want %q", i, got, want)
				}
			}
		})
	}
}

// TestMalformedCPDLCSamples checks that payloads which libacars reports as
// unparseable in both directions are not decoded. They were once decoded
// (as dM80, dM58, dM20 and dM28) by a workaround in an earlier decoder.
func TestMalformedCPDLCSamples(t *testing.T) {
	for _, hexStr := range []string{"1FD08019F3", "00D0569F3630EADB", "01BA005617", "E102044A521D01FC9C34", "E184074E1ACB902C2072E4F321"} {
		data, err := hex.DecodeString(hexStr)
		if err != nil {
			t.Fatalf("Hex decode error: %v", err)
		}
		for _, dir := range []MessageDirection{DirectionUplink, DirectionDownlink, DirectionUnknown} {
			if msg, err := DecodeWithUPER(data, dir); err == nil {
				t.Errorf("%s (%v): decoded %+v; want an error", hexStr, dir, msg.Elements)
			}
		}
	}
}
