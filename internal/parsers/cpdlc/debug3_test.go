package cpdlc

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestCurrentDecoderOutput(t *testing.T) {
	samples := []struct {
		hexStr string
		dir    MessageDirection
		desc   string
	}{
		{"6184241F01DF74", DirectionDownlink, "Was FL16390m in old DB"},
		{"E184074E1ACB902C2072E4F321", DirectionDownlink, "Was 11054m"},
	}
	
	for _, s := range samples {
		data, _ := hex.DecodeString(s.hexStr)
		decoder := NewDecoder(data, s.dir)
		msg, err := decoder.Decode()
		
		fmt.Printf("\n=== %s ===\n", s.desc)
		fmt.Printf("Hex: %s\n", s.hexStr)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		
		if len(msg.Elements) > 0 {
			elem := msg.Elements[0]
			fmt.Printf("Element ID: %d\n", elem.ID)
			fmt.Printf("Label: %s\n", elem.Label)
			fmt.Printf("Text: %s\n", elem.Text)
			
			if alt, ok := elem.Data.(*Altitude); ok {
				fmt.Printf("Altitude: %d %s\n", alt.Value, alt.Type)
			}
		}
	}
}
