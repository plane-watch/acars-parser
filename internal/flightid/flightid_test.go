package flightid

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		in, tail string
		want     ID
		ok       bool
	}{
		{"QFA001", "", ID{Airline: "QFA", ICAO: true, Number: "1"}, true},
		{"QF0001", "", ID{Airline: "QF", Number: "1"}, true},
		{"B6123", "", ID{Airline: "B6", Number: "123"}, true},
		{"LOT49H", "", ID{Airline: "LOT", ICAO: true, Number: "49", Suffix: "H"}, true},
		{"SWR4WF", "", ID{Airline: "SWR", ICAO: true, Number: "4", Suffix: "WF"}, true},
		{" UAL839 ", "", ID{Airline: "UAL", ICAO: true, Number: "839"}, true},
		// A registration used as the callsign is not a flight identifier.
		{"N123AB", "N123AB", ID{}, false},
		// An IATA code must contain a letter.
		{"12345", "", ID{}, false},
		{"QANTAS", "", ID{}, false},
	}
	for _, tt := range tests {
		got, ok := Parse(tt.in, tt.tail)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Parse(%q) = %+v, %v; want %+v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestString(t *testing.T) {
	for in, want := range map[string]string{"QFA001": "QFA1", "QF0001": "QF1", "LOT049H": "LOT49H", "QF000": "QF0"} {
		id, _ := Parse(in, "")
		if got := id.String(); got != want {
			t.Errorf("Parse(%q).String() = %q, want %q", in, got, want)
		}
	}
}

func TestSame(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"QF1255", "QFA1255", true},
		{"QFA008", "QFA8", true},
		{"QF1", "QF01", true},
		// Another number, or the same number of another airline in the
		// same form.
		{"B61", "JBU61", false},
		{"QF11", "QF1", false},
		{"QFA1", "UAL1", false},
		{"LOT49H", "LOT49", false},
		// Not flight identifiers.
		{"N123AB", "N123AB", false},
	}
	for _, tt := range tests {
		a, okA := Parse(tt.a, "N123AB")
		b, okB := Parse(tt.b, "N123AB")
		if got := okA && okB && Same(a, b); got != tt.want {
			t.Errorf("Same(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
