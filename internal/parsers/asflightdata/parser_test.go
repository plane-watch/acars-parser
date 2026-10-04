package asflightdata

import (
	"math"
	"testing"

	"acars_parser/internal/acars"
)

// The report is a real Alaska Airlines report (label H1) from the January
// 2026 corpus.
const report = "D3M207KSANKATLN33216W11201120073500M052215102G0009\r\nN33232W1115623500M052214100G0009\r\nN33249W1115113501M052214100G0009\r\nN33265W1114623499M052215099G0009\r\nN33281W1114123500M052215099G0009\r\n-0016\r\n"

func TestParse(t *testing.T) {
	r, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Text: report}).(*Result)
	if !ok {
		t.Fatal("Parse returned no result")
	}
	if r.Origin != "KSAN" || r.Destination != "KATL" || r.Time != "2007" || len(r.Samples) != 5 {
		t.Fatalf("got route %s-%s, time %q, %d samples", r.Origin, r.Destination, r.Time, len(r.Samples))
	}
	first := r.Samples[0]
	want := Sample{Latitude: 33.36, Longitude: -112.018333, AltitudeFt: 35000, TemperatureC: -52, WindDirection: 215, WindSpeedKt: 102}
	first.Latitude = math.Round(first.Latitude*1e6) / 1e6
	first.Longitude = math.Round(first.Longitude*1e6) / 1e6
	if first != want {
		t.Errorf("first sample %+v, want %+v", first, want)
	}
	if last := r.Samples[4]; last.AltitudeFt != 35000 || last.WindSpeedKt != 99 {
		t.Errorf("last sample %+v", last)
	}
}

// TestParsePositiveTemperature checks the "P" sign and a sample with
// another status letter ("B").
func TestParsePositiveTemperature(t *testing.T) {
	text := "D3M720KATLKSEAN47320W12208505390858P006312012G0009\r\nN47343W1220840770P008313013B0009\r\n"
	r, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Text: text}).(*Result)
	if !ok {
		t.Fatal("Parse returned no result")
	}
	if len(r.Samples) != 2 || r.Samples[0].TemperatureC != 6 || r.Samples[0].AltitudeFt != 8580 || r.Samples[1].TemperatureC != 8 {
		t.Errorf("got %+v", r.Samples)
	}
}

func TestParseRejectsOtherText(t *testing.T) {
	for _, text := range []string{
		"D3M207KSANXXXXN33216W11201120073500M052215102G0009\r\n",
		"D3M207KSANKATL garbage",
		"POSN31211W097249,ACT,052904",
		// Other variants, whose layouts are not established.
		"D1M207KSANKATLN33216W11201120073500M052215102G0009\r\n",
		"D2M207KSANKATLN33216W11201120073500M052215102G0009\r\n",
		// A latitude beyond 90 degrees.
		"D3M207KSANKATLS99216W11201120073500M052215102G0009\r\n",
		// Minutes of 60 or more, and an impossible time.
		"D3M207KSANKATLN33999W11201199603500M052215102G0009\r\n",
		"D3M207KSANKATLN33216W11201125073500M052215102G0009\r\n",
	} {
		if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Text: text}); r != nil {
			t.Errorf("Parse(%q) = %+v, want nil", text, r)
		}
	}
}

// TestParseSkipsInvalidSamples checks that a later sample with impossible
// minutes is skipped while the report is kept.
func TestParseSkipsInvalidSamples(t *testing.T) {
	text := "D3M207KSANKATLN33216W11201120073500M052215102G0009\r\nN33999W1115623500M052214100G0009\r\nN33249W1115113501M052214100G0009\r\n"
	r, ok := (&Parser{}).Parse(&acars.Message{Label: "H1", Text: text}).(*Result)
	if !ok || len(r.Samples) != 2 {
		t.Errorf("got %+v, want 2 samples", r)
	}
}
