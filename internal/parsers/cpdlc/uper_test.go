package cpdlc

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/shaneshort/go-asn/uper"
)

// TestUPERDecode tests decoding with the new UPER library.
func TestUPERDecode(t *testing.T) {
	// Known good sample: dM48 Position Report with MsgID=8, Timestamp=15:56:32
	// Verified against libacars.
	hexStr := "243F880C3D903BB412903604FE326C2479F4A64F7F62528B1A9CF8382738186AC28B16668E013DF464D8"
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("hex decode: %v", err)
	}

	fmt.Println("=== UPER Decode Test ===")
	fmt.Printf("Input hex: %s\n", hexStr)
	fmt.Printf("Input length: %d bytes\n", len(data))

	var msg UPERDownlinkMessage
	err = uper.Unmarshal(data, &msg)
	if err != nil {
		t.Fatalf("UPER unmarshal: %v", err)
	}

	// Verify header
	fmt.Printf("\n=== Header ===\n")
	fmt.Printf("MsgID: %d (expected 8)\n", msg.Header.MsgID)
	if msg.Header.MsgRef != nil {
		fmt.Printf("MsgRef: %d\n", *msg.Header.MsgRef)
	} else {
		fmt.Printf("MsgRef: nil\n")
	}
	if msg.Header.Timestamp != nil {
		fmt.Printf("Timestamp: %02d:%02d:%02d (expected 15:56:32)\n",
			msg.Header.Timestamp.Hours,
			msg.Header.Timestamp.Minutes,
			msg.Header.Timestamp.Seconds)
	} else {
		fmt.Printf("Timestamp: nil\n")
	}

	if msg.Header.MsgID != 8 {
		t.Errorf("MsgID = %d, want 8", msg.Header.MsgID)
	}
	if msg.Header.Timestamp == nil {
		t.Error("Expected timestamp, got nil")
	} else {
		if msg.Header.Timestamp.Hours != 15 {
			t.Errorf("Hours = %d, want 15", msg.Header.Timestamp.Hours)
		}
		if msg.Header.Timestamp.Minutes != 56 {
			t.Errorf("Minutes = %d, want 56", msg.Header.Timestamp.Minutes)
		}
		if msg.Header.Timestamp.Seconds != 32 {
			t.Errorf("Seconds = %d, want 32", msg.Header.Timestamp.Seconds)
		}
	}

	// Check which element was decoded
	fmt.Printf("\n=== Element ===\n")
	elem := msg.Element
	if elem.DM48PositionReport != nil {
		fmt.Println("Element: dM48 Position Report")
		pr := elem.DM48PositionReport

		// Print position (mandatory).
		fmt.Printf("Position: ")
		printPosition(&pr.PositionCurrent)

		// Print time at position (mandatory).
		fmt.Printf("Time at position: %02d:%02d\n", pr.TimeAtPositionCurrent.Hours, pr.TimeAtPositionCurrent.Minutes)

		// Print altitude (mandatory).
		fmt.Printf("Altitude: ")
		printAltitude(&pr.Altitude)

		if pr.FixNext != nil {
			fmt.Printf("Fix next: ")
			printPosition(pr.FixNext)
		}
		if pr.TimeEtaAtFixNext != nil {
			fmt.Printf("ETA at fix next: %02d:%02d\n", pr.TimeEtaAtFixNext.Hours, pr.TimeEtaAtFixNext.Minutes)
		}
	} else {
		// Find which element is set
		fmt.Println("Element type not dM48")
		if elem.DM0Wilco != nil {
			fmt.Println("  DM0 Wilco")
		}
		// Add more checks as needed
	}
}

func printPosition(p *UPERPosition) {
	if p == nil {
		fmt.Println("nil")
		return
	}
	switch {
	case p.FixName != nil:
		fmt.Printf("Fix: %s\n", *p.FixName)
	case p.Navaid != nil:
		fmt.Printf("Navaid: %s\n", *p.Navaid)
	case p.Airport != nil:
		fmt.Printf("Airport: %s\n", *p.Airport)
	case p.LatitudeLongitude != nil:
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

		fmt.Printf("LatLon: %.4f, %.4f\n", lat, lon)
	case p.PlaceBearingDistance != nil:
		pbd := p.PlaceBearingDistance
		fmt.Printf("PlaceBearingDistance: %s ", pbd.FixName)
		if pbd.Degrees.DegreesMagnetic != nil {
			fmt.Printf("%03d°M ", *pbd.Degrees.DegreesMagnetic)
		} else if pbd.Degrees.DegreesTrue != nil {
			fmt.Printf("%03d°T ", *pbd.Degrees.DegreesTrue)
		}
		if pbd.Distance.DistanceNm != nil {
			fmt.Printf("%.1fnm\n", float64(*pbd.Distance.DistanceNm)/10.0)
		} else if pbd.Distance.DistanceKm != nil {
			fmt.Printf("%dkm\n", *pbd.Distance.DistanceKm)
		}
	default:
		fmt.Println("(unknown)")
	}
}

func printAltitude(a *UPERAltitude) {
	if a == nil {
		fmt.Println("nil")
		return
	}
	switch {
	case a.AltitudeQNH != nil:
		fmt.Printf("%d ft QNH\n", *a.AltitudeQNH * 10)
	case a.AltitudeQNHMeters != nil:
		fmt.Printf("%d m QNH\n", *a.AltitudeQNHMeters)
	case a.AltitudeQFE != nil:
		fmt.Printf("%d ft QFE\n", *a.AltitudeQFE * 10)
	case a.AltitudeQFEMeters != nil:
		fmt.Printf("%d m QFE\n", *a.AltitudeQFEMeters)
	case a.AltitudeGNSSFeet != nil:
		fmt.Printf("%d ft GNSS\n", *a.AltitudeGNSSFeet)
	case a.AltitudeGNSSMeters != nil:
		fmt.Printf("%d m GNSS\n", *a.AltitudeGNSSMeters)
	case a.AltitudeFlightLevel != nil:
		fmt.Printf("FL%d\n", *a.AltitudeFlightLevel)
	case a.AltitudeFlightLevelMetric != nil:
		fmt.Printf("FL%d metric\n", *a.AltitudeFlightLevelMetric)
	default:
		fmt.Println("(unknown)")
	}
}
