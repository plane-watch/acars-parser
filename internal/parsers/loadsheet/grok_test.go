package loadsheet

import (
	"testing"
)

func TestMatchFormat_StandardKG(t *testing.T) {
	// Swiss/Lufthansa format.
	text := `LOADSHEET FINAL 1736 EDNO1
LX1376/21     21JAN26
ZRH WRO HB-AZH   2/3
ZFW 39754  MAX 46700
TOF 4800
TOW 44554  MAX 54000
TIF 2000
LAW 42554  MAX 49050   L
UNDLD 6496
PAX/6/59 TTL 65`

	format, fields := MatchFormat(text, "C1")
	if format == nil {
		t.Fatal("Expected standard_kg format to match")
	}

	if format.Name != "standard_kg" {
		t.Errorf("Expected format name 'standard_kg', got '%s'", format.Name)
	}

	tests := map[string]string{
		"status":      "FINAL",
		"time":        "1736",
		"edition":     "1",
		"flight":      "LX1376",
		"origin":      "ZRH",
		"destination": "WRO",
		"tail":        "HB-AZH",
		"crew":        "2/3",
		"zfw":         "39754",
		"zfw_max":     "46700",
		"tof":         "4800",
		"tow":         "44554",
		"tow_max":     "54000",
		"tif":         "2000",
		"law":         "42554",
		"law_max":     "49050",
		"pax_total":   "65",
	}

	for field, expected := range tests {
		if fields[field] != expected {
			t.Errorf("Field %s: expected '%s', got '%s'", field, expected, fields[field])
		}
	}
}

func TestMatchFormat_QantasTonnes(t *testing.T) {
	// Qantas format with weights in tonnes.
	text := `FINAL LOADSHEET
ISSUE DATE TIME: 24JAN  1900
REGO: VH-ZNL
FLIGHT: QF009
=========================
QF009  24JAN PER VH-ZNL 4/10 1900
.....................
QF009
NIL SIGNIFICANT CHANGE FROM PROV EDNO 1
ZFW   150.6
TOF   100.1
TOW   250.7
TIF    94.6
LAW   156.1`

	format, fields := MatchFormat(text, "C1")
	if format == nil {
		t.Fatal("Expected qantas_tonnes format to match")
	}

	if format.Name != "qantas_tonnes" {
		t.Errorf("Expected format name 'qantas_tonnes', got '%s'", format.Name)
	}

	if fields["tail"] != "VH-ZNL" {
		t.Errorf("Expected tail 'VH-ZNL', got '%s'", fields["tail"])
	}
	if fields["flight"] != "QF009" {
		t.Errorf("Expected flight 'QF009', got '%s'", fields["flight"])
	}
	if fields["origin"] != "PER" {
		t.Errorf("Expected origin 'PER', got '%s'", fields["origin"])
	}
	if fields["zfw"] != "150.6" {
		t.Errorf("Expected zfw '150.6', got '%s'", fields["zfw"])
	}

	// Test weight conversion.
	zfw := parseWeight(fields["zfw"], format.WeightUnit)
	if zfw != 150600 {
		t.Errorf("Expected ZFW 150600 kg, got %d", zfw)
	}
}

func TestMatchFormat_BA(t *testing.T) {
	// British Airways format with spaced header.
	text := `L O A D S H E E T           CHECKED      APPROVED         EDNO
ALL WEIGHTS IN KG      LIC 12406//J.M.G                    01
.
FROM/TO FLIGHT       A/C REG VERSION      CREW    DATE    TIME
EZE BCN LL 2604      ECNNH   C42Y269      3/08    18JAN26 1918
                        WEIGHT           DISTRIBUTION
LOAD IN COMPARTMENTS      5358 1/1430 3/1110 4/2722 5/96
.
PASSENGER/CABIN BAG      21945 140/126/23/7    TTL 296 CAB 0
                               PAX 38/251      SOC
                               BLK
TOTAL TRAFFIC LOAD       27303
DRY OPERATING WEIGHT    122601
ZERO FUEL WEIGHT ACTUAL 149904 MAX 166000  L   ADJ
TAKE OFF FUEL            75982
TAKE OFF WEIGHT  ACTUAL 225886 MAX 242000      ADJ
TRIP FUEL                66530
LANDING WEIGHT   ACTUAL 159356 MAX 182000      ADJ`

	format, fields := MatchFormat(text, "10")
	if format == nil {
		t.Fatal("Expected ba_full_names format to match")
	}

	if format.Name != "ba_full_names" {
		t.Errorf("Expected format name 'ba_full_names', got '%s'", format.Name)
	}

	if fields["origin"] != "EZE" {
		t.Errorf("Expected origin 'EZE', got '%s'", fields["origin"])
	}
	if fields["destination"] != "BCN" {
		t.Errorf("Expected destination 'BCN', got '%s'", fields["destination"])
	}
	if fields["zfw"] != "149904" {
		t.Errorf("Expected zfw '149904', got '%s'", fields["zfw"])
	}
	if fields["tow"] != "225886" {
		t.Errorf("Expected tow '225886', got '%s'", fields["tow"])
	}
}

func TestMatchFormat_WrongLabel(t *testing.T) {
	// Standard format but wrong label - should not match.
	text := `LOADSHEET FINAL 1736 EDNO1
LX1376/21     21JAN26
ZRH WRO HB-AZH   2/3
ZFW 39754  MAX 46700`

	format, _ := MatchFormat(text, "99") // Invalid label.
	if format != nil {
		t.Error("Expected no match for invalid label")
	}
}

func TestParseWeight(t *testing.T) {
	tests := []struct {
		value    string
		unit     string
		expected int
	}{
		{"39754", "kg", 39754},
		{"150.6", "tonnes", 150600},
		{"", "kg", 0},
		{"1,234", "kg", 1234},
		{"12.345", "tonnes", 12345},
	}

	for _, tc := range tests {
		result := parseWeight(tc.value, tc.unit)
		if result != tc.expected {
			t.Errorf("parseWeight(%q, %q) = %d, expected %d", tc.value, tc.unit, result, tc.expected)
		}
	}
}