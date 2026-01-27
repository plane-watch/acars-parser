package cpdlc

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestDebugAltitude(t *testing.T) {
	hexStr := "E184074E1ACB902C2072E4F321"
	data, _ := hex.DecodeString(hexStr)
	
	br := NewBitReader(data)
	
	// Decode header manually
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
	fmt.Printf("extBit=%v, elemID=%d (should be 28 LEAVING)\n", extBit, elemID)
	
	// Now decode altitude
	// Altitude CHOICE: 3 bits (0-7)
	altChoice, _ := br.ReadConstrainedInt(0, 7)
	fmt.Printf("altChoice=%d\n", altChoice)
	
	switch altChoice {
	case 0:
		v, _ := br.ReadConstrainedInt(0, 2500)
		fmt.Printf("altitudeQNH: %d x10 = %d feet\n", v, v*10)
	case 1:
		v, _ := br.ReadConstrainedInt(0, 16000)
		fmt.Printf("altitudeQNHMeters: %d meters\n", v)
	case 2:
		v, _ := br.ReadConstrainedInt(0, 2100)
		fmt.Printf("altitudeQFE: %d x10 = %d feet\n", v, v*10)
	case 3:
		v, _ := br.ReadConstrainedInt(0, 7000)
		fmt.Printf("altitudeQFEMeters: %d meters\n", v)
	case 4:
		v, _ := br.ReadConstrainedInt(0, 150000)
		fmt.Printf("altitudeGNSSFeet: %d feet\n", v)
	case 5:
		v, _ := br.ReadConstrainedInt(0, 50000)
		fmt.Printf("altitudeGNSSMeters: %d meters\n", v)
	case 6:
		v, _ := br.ReadConstrainedInt(30, 600)
		fmt.Printf("altitudeFlightLevel: FL%d\n", v)
	case 7:
		v, _ := br.ReadConstrainedInt(100, 2000)
		fmt.Printf("altitudeFlightLevelMetric: %d\n", v)
	}
}
