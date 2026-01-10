package label27

import (
	"acars_parser/internal/acars"
	"encoding/json"
	"fmt"
	"testing"
)

func TestManualMessage(t *testing.T) {
	msg := &acars.Message{
		ID:        1,
		Label:     "27",
		Tail:      "RA-12345",
		Timestamp: "2026-01-10T18:19:01Z",
		Text:      "POS01AFL2827 /18190121USSSULLI FUEL 122 TEMP- 13 WDIR26384 WSPD 20 LATN 56.832 LONE 60.195 ETA0359 TUR ALT 11741",
	}

	parser := &Parser{}

	if !parser.QuickCheck(msg.Text) {
		t.Fatal("QuickCheck failed!")
	}

	result := parser.Parse(msg)
	if result == nil {
		t.Fatal("Parse returned nil!")
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("Error marshaling JSON: %v", err)
	}

	fmt.Println(string(jsonData))
}
