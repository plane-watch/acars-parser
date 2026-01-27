package cpdlc

import (
	"encoding/hex"
	"fmt"
	"testing"
)

// TestValidCPDLCSamples tests samples that are known to be valid according to libacars.
func TestValidCPDLCSamples(t *testing.T) {
	samples := []struct {
		hexStr       string
		direction    MessageDirection
		desc         string
		wantElemID   int
		wantMsgID    int
		wantHasTime  bool
	}{
		// libacars sample (from cpdlc_get_position.c) - dM48 Position Report
		// Verified to decode correctly in libacars with:
		//   Msg ID: 8, Timestamp: 15:56:32, Element: dM48PositionReport
		{
			hexStr:      "243F880C3D903BB412903604FE326C2479F4A64F7F62528B1A9CF8382738186AC28B16668E013DF464D8",
			direction:   DirectionDownlink,
			desc:        "libacars sample - dM48 Position Report",
			wantElemID:  48,
			wantMsgID:   8,
			wantHasTime: true,
		},
	}

	for _, s := range samples {
		t.Run(s.desc, func(t *testing.T) {
			data, err := hex.DecodeString(s.hexStr)
			if err != nil {
				t.Fatalf("Hex decode error: %v", err)
			}

			decoder := NewDecoder(data, s.direction)
			msg, err := decoder.Decode()
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}

			// Check header.
			if msg.Header.MsgID != s.wantMsgID {
				t.Errorf("MsgID = %d, want %d", msg.Header.MsgID, s.wantMsgID)
			}
			if s.wantHasTime && msg.Header.Timestamp == nil {
				t.Error("Expected timestamp but got nil")
			}

			// Check element.
			if len(msg.Elements) == 0 {
				t.Fatal("No elements decoded")
			}
			elem := msg.Elements[0]
			if elem.ID != s.wantElemID {
				t.Errorf("Element ID = %d, want %d", elem.ID, s.wantElemID)
			}

			fmt.Printf("\n=== %s ===\n", s.desc)
			fmt.Printf("Hex: %s\n", s.hexStr)
			fmt.Printf("MsgID: %d\n", msg.Header.MsgID)
			if msg.Header.Timestamp != nil {
				fmt.Printf("Timestamp: %s\n", msg.Header.Timestamp)
			}
			fmt.Printf("Element ID: %d\n", elem.ID)
			fmt.Printf("Label: %s\n", elem.Label)
			fmt.Printf("Text: %s\n", elem.Text)
		})
	}
}

// TestMalformedCPDLCSamples tests samples that are known to be malformed/truncated.
// These samples fail to decode in libacars and should produce invalid element IDs.
// The purpose of this test is to document known-bad samples and verify we don't crash.
func TestMalformedCPDLCSamples(t *testing.T) {
	samples := []struct {
		hexStr    string
		direction MessageDirection
		desc      string
	}{
		// These samples were previously thought to be valid but fail in libacars.
		// They likely represent truncated or corrupted CPDLC messages.
		{"1FD08019F3", DirectionDownlink, "truncated - was decoded as dM80 with old workaround"},
		{"00D0569F3630EADB", DirectionDownlink, "truncated - was decoded as dM80 with old workaround"},
		{"01BA005617", DirectionDownlink, "truncated - was decoded as dM58 with old workaround"},
		{"E102044A521D01FC9C34", DirectionDownlink, "truncated - was decoded as dM20 with old workaround"},
		{"E184074E1ACB902C2072E4F321", DirectionDownlink, "truncated - was decoded as dM28 with old workaround"},
	}

	for _, s := range samples {
		t.Run(s.desc, func(t *testing.T) {
			data, err := hex.DecodeString(s.hexStr)
			if err != nil {
				t.Fatalf("Hex decode error: %v", err)
			}

			decoder := NewDecoder(data, s.direction)
			msg, err := decoder.Decode()

			// These may or may not decode without error, but the element ID should be invalid.
			// We're mainly verifying we don't crash on malformed input.
			if err != nil {
				t.Logf("Decode error (expected for malformed data): %v", err)
				return
			}

			if len(msg.Elements) > 0 {
				elem := msg.Elements[0]
				t.Logf("Malformed sample decoded to element ID %d (%s) - likely invalid",
					elem.ID, elem.Label)
			}
		})
	}
}