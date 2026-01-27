package cpdlc

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestDebugBadAltitude(t *testing.T) {
	// This message was decoded as "PRESENT ALTITUDE FL16390m" which is nonsense
	hexStr := "6184241F01DF74"
	data, _ := hex.DecodeString(hexStr)
	
	fmt.Printf("Decoding: %s\n", hexStr)
	fmt.Printf("Raw bytes: %v\n", data)
	
	br := NewBitReader(data)
	
	// Decode header
	hasRef, _ := br.ReadBit()
	hasTimestamp, _ := br.ReadBit()
	msgID, _ := br.ReadConstrainedInt(0, 63)
	fmt.Printf("hasRef=%v, hasTimestamp=%v, msgID=%d\n", hasRef, hasTimestamp, msgID)
	
	if hasRef {
		ref, _ := br.ReadConstrainedInt(0, 63)
		fmt.Printf("msgRef=%d\n", ref)
	}
	if hasTimestamp {
		hours, _ := br.ReadConstrainedInt(0, 23)
		minutes, _ := br.ReadConstrainedInt(0, 59)
		fmt.Printf("timestamp=%02d:%02d\n", hours, minutes)
	}
	
	// Extension bit + element ID
	extBit, _ := br.ReadBit()
	elemID, _ := br.ReadConstrainedInt(0, 127)
	fmt.Printf("extBit=%v, elemID=%d\n", extBit, elemID)
	fmt.Printf("Remaining bits: %d\n", br.Remaining())
	
	// Altitude CHOICE: 3 bits (0-7)
	altChoice, _ := br.ReadConstrainedInt(0, 7)
	fmt.Printf("altChoice=%d\n", altChoice)
	
	// What does each choice need?
	choiceNames := []string{
		"0: altitudeQNH (12 bits, 0-2500, feet/10)",
		"1: altitudeQNHMeters (14 bits, 0-16000)",
		"2: altitudeQFE (12 bits, 0-2100, feet/10)",
		"3: altitudeQFEMeters (13 bits, 0-7000)",
		"4: altitudeGNSSFeet (18 bits, 0-150000)",
		"5: altitudeGNSSMeters (16 bits, 0-50000)",
		"6: altitudeFlightLevel (10 bits, 30-600)",
		"7: altitudeFlightLevelMetric (11 bits, 100-2000)",
	}
	fmt.Printf("Choice type: %s\n", choiceNames[altChoice])
	fmt.Printf("Remaining bits after choice: %d\n", br.Remaining())
}
