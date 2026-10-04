package cpdlc

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"acars_parser/internal/acars"
)

// The messages in this file were checked against the libacars reference
// decoder (decode_acars_apps). The real messages are from the January 2026
// corpus; the libacars output for each is summarised in its comment. The
// synthetic payloads exercise types no corpus message uses: they were
// encoded with uper.Marshal from the types in fans_uper_types.go, wrapped
// in an AT1 message with a valid CRC and decoded by libacars, which gave
// the values in the comments.

// TestDecodeLibacarsVerifiedMessages decodes real messages and checks each
// element's label and formatted text, which carries its data.
func TestDecodeLibacarsVerifiedMessages(t *testing.T) {
	tests := []struct {
		name, label, text string
		wantDir           string
		wantLabels        []string
		wantTexts         []string
	}{
		{
			// libacars: SQUAWK 2215; MONITOR SBAO center HF 10096 kHz; AT
			// 15 deg south 035 deg west CONTACT SBRE center VHF 134.500
			// MHz; free text SECONDARY 126.55.
			name: "SQUAWK, MONITOR, AT position CONTACT and free text", label: "AA",
			text:    "/RECOEYA.AT1.TC-LGZA10CB7DED23678538506780E273B30F88EA70A9450C45CA90FA7161CF9D120D2B2818B26CB9AB57B4D",
			wantDir: "uplink",
			wantLabels: []string{"SQUAWK [beaconcode]", "MONITOR [icaounitname] [frequency]",
				"AT [position] CONTACT [icaounitname] [frequency]", "[freetext]"},
			wantTexts: []string{"SQUAWK 2215", "MONITOR SBAO CENTER 10096 kHz",
				"AT -15.0000,-35.0000 CONTACT SBRE CENTER 134.500 MHz", "SECONDARY 126.55"},
		},
		{
			// libacars: Msg Ref 5, ERROR unrecognizedMsgReferenceNumber.
			name: "ERROR", label: "AA", text: "/CCUCAYA.AT1.VH-OQJ428B3E205D4E", wantDir: "uplink",
			wantLabels: []string{"ERROR [errorinformation]"},
			wantTexts:  []string{"ERROR unrecognized msg reference number"},
		},
		{
			// libacars: position 66 deg north 120 deg west; route 57 deg
			// north 100 deg west.
			name: "uM79 CLEARED TO position VIA route", label: "AA",
			text: "/YEGE2YA.AT1.JA798A21CE7DD3DA10F10080139192ED77", wantDir: "uplink",
			wantLabels: []string{"CLEARED TO [position] VIA [routeclearance]"},
			wantTexts:  []string{"CLEARED TO 66.0000,-120.0000 VIA ROUTE 57.0000,-100.0000"},
		},
		{
			// libacars: destination KSFO; arrival procedure BDEGA4,
			// transition AMAKR; route: published identifier AMAKR.
			name: "uM80 CLEARED route", label: "AA",
			text: "/OAKODYA.AT1.VT-ALJ23D6B01410A974E34F961448B1E0B49066C1974800483360CBA4C7F2", wantDir: "uplink",
			wantLabels: []string{"CLEARED [routeclearance]"},
			wantTexts:  []string{"CLEARED ROUTE AMAKR STAR BDEGA4.AMAKR DEST KSFO"},
		},
		{
			// libacars: position 21 deg south 100 deg east; route SINAM,
			// IPMOR, PH (published identifiers).
			name: "uM83 AT position CLEARED route", label: "AA",
			text: "/MELCAYA.AT1.A7-BEL2404A894D8ACC8008104A7267419A124D09B3E901A12000DC8", wantDir: "uplink",
			wantLabels: []string{"AT [position] CLEARED [routeclearance]"},
			wantTexts:  []string{"AT -21.0000,100.0000 CLEARED ROUTE SINAM IPMOR PH"},
		},
		{
			// libacars: Facility Name DIAP (a name, not a designation),
			// function control, HF 8861 kHz.
			name: "facility name and HF frequency", label: "AA",
			text: "/ACCFAYA.AT1.N3895501BAC6249834385DECA52D", wantDir: "uplink",
			wantLabels: []string{"CONTACT [icaounitname] [frequency]"},
			wantTexts:  []string{"CONTACT DIAP CONTROL 8861 kHz"},
		},
		{
			// libacars: facility designation SBAO, TP4 table labelA.
			name: "uM163 facility designation and TP4 table", label: "AA",
			text: "/RECOEYA.AT1.D2-TES2881D7E8E9C2833C5D98", wantDir: "uplink",
			wantLabels: []string{"[icaofacilitydesignation] [tp4table]"},
			wantTexts:  []string{"SBAO labelA"},
		},
		{
			// libacars: a position report (26 54.5' south 152 35.9' east at
			// 05:10, FL265, next fix LAWNN at 05:14), DEVIATING 30 nm right
			// and CLIMBING TO FL320.
			name: "dM48, dM80 and dM29", label: "BA",
			text:    "/BNECAYA.AT1.9V-SHQA114A5CC3D9F3B9A8879859C52B1D624C835E74E29C257830E5CF622711B1D8BC2159CE6A243834A9CC28989954074477244DD7F",
			wantDir: "downlink",
			wantLabels: []string{"POSITION REPORT [positionreport]", "DEVIATING [distanceoffset][direction] OF ROUTE",
				"CLIMBING TO [altitude]"},
			wantTexts: []string{"POSITION REPORT -26.9083,152.5983 at 05:10 FL265 next LAWNN at 05:14",
				"DEVIATING 30 nm right OF ROUTE", "CLIMBING TO FL320"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := parseCPDLC(t, tt.label, tt.text)
			if r.Error != "" || r.Direction != tt.wantDir {
				t.Fatalf("error %q, direction %q; want no error, %q", r.Error, r.Direction, tt.wantDir)
			}
			if len(r.Elements) != len(tt.wantLabels) {
				t.Fatalf("%d elements, want %d: %+v", len(r.Elements), len(tt.wantLabels), r.Elements)
			}
			for i, e := range r.Elements {
				if e.Label != tt.wantLabels[i] || e.Text != tt.wantTexts[i] {
					t.Errorf("element %d: %q / %q; want %q / %q", i, e.Label, e.Text, tt.wantLabels[i], tt.wantTexts[i])
				}
			}
		})
	}
}

// TestDecodeErrorInformation checks the data of a uM159 ERROR: libacars
// gives unrecognizedMsgReferenceNumber, the third enumeration (index 2).
func TestDecodeErrorInformation(t *testing.T) {
	r := parseCPDLC(t, "AA", "/CCUCAYA.AT1.VH-OQJ428B3E205D4E")
	if len(r.Elements) != 1 {
		t.Fatalf("elements %+v", r.Elements)
	}
	want := &ErrorInfo{Code: 2, Desc: "unrecognizedMsgReferenceNumber"}
	if got, ok := r.Elements[0].Data.(*ErrorInfo); !ok || *got != *want {
		t.Errorf("data %#v, want %#v", r.Elements[0].Data, want)
	}
	if r.Header == nil || r.Header.MsgRef == nil || *r.Header.MsgRef != 5 || r.Header.Timestamp != nil {
		t.Errorf("header %+v; want message reference 5 and no timestamp", r.Header)
	}
}

// TestDecodeFacilityName checks that a facility given by name is reported
// as a name, not as a designation.
func TestDecodeFacilityName(t *testing.T) {
	r := parseCPDLC(t, "AA", "/ACCFAYA.AT1.N3895501BAC6249834385DECA52D")
	data, ok := r.Elements[0].Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data %#v", r.Elements[0].Data)
	}
	wantUnit := ICAOUnitName{FacilityName: "DIAP", FacilityFunction: "control"}
	if unit, ok := data["unit"].(*ICAOUnitName); !ok || *unit != wantUnit {
		t.Errorf("unit %#v, want %#v", data["unit"], wantUnit)
	}
	wantFreq := Frequency{Type: "hf", Value: 8861}
	if freq, ok := data["frequency"].(*Frequency); !ok || *freq != wantFreq {
		t.Errorf("frequency %#v, want %#v", data["frequency"], wantFreq)
	}
}

// TestDecodePositionReport checks every component of a real dM48. libacars
// gives: 26 34.3' south 153 09.6' east at 03:06, FL191, next fix LOAFA at
// 03:21, next+1 navaid GLA, ETA at destination 09:15, -7 C, wind 63 deg 21
// kts, indicated airspeed 290 kts, ground speed 400 kts, up 1300 ft/min,
// track angle 328 deg true, true heading 331 deg true; then CLIMBING TO
// FL320.
func TestDecodePositionReport(t *testing.T) {
	r := parseCPDLC(t, "BA", "/BNECAYA.AT1.VH-OYXA10C6A8C3D9F039A55F9918031B14224C9F063411AA68F320A4F491F0545A10DD1F4A0772440B0E1")
	if r.Error != "" || len(r.Elements) != 2 || r.Elements[1].Text != "CLIMBING TO FL320" {
		t.Fatalf("error %q, elements %+v", r.Error, r.Elements)
	}
	assertJSON(t, r.Elements[0].Data, `{
		"position": {"type": "latlon", "latitude": -26.571666666666665, "longitude": 153.16},
		"time": {"hours": 3, "minutes": 6},
		"altitude": {"type": "flight_level", "value": 191},
		"fix_next": {"type": "fix", "name": "LOAFA"},
		"fix_next_eta": {"hours": 3, "minutes": 21},
		"fix_next_plus_one": {"type": "navaid", "name": "GLA"},
		"destination_eta": {"hours": 9, "minutes": 15},
		"temperature": {"value": -7, "unit": "C"},
		"wind": {"direction": 63, "speed": 21, "unit": "kt"},
		"speed": {"type": "indicated", "value": 290},
		"ground_speed": {"type": "ground", "value": 400},
		"vertical_change": {"direction": "up", "rate": {"value": 1300, "unit": "ft/min"}},
		"track_angle": {"value": 328},
		"true_heading": {"value": 331}
	}`)
}

// TestDecodeRejectsTrailingOctets checks a real message that libacars
// reports as unparseable: its encoding ends a whole octet before the end of
// the payload. An earlier decoder, which did not check this, decoded it.
func TestDecodeRejectsTrailingOctets(t *testing.T) {
	r := parseCPDLC(t, "AA", "/ANCXFXA.AT1.B-2075813DF440F0A106D4109B8A0023FA")
	if !strings.HasPrefix(r.Error, "decode_failed") || len(r.Elements) != 0 {
		t.Errorf("error %q, elements %+v; want decode_failed", r.Error, r.Elements)
	}
}

// TestDecodeRejectsInvalidEnumeration checks that an ENUMERATED index that
// names no enumeration is rejected, as libacars rejects it. The payload is
// the uM159 ERROR above (Msg ID 5, Msg Ref 5) with its five-bit error index
// changed from 2 to 31; FANSErrorInformation has indexes 0 to 16.
func TestDecodeRejectsInvalidEnumeration(t *testing.T) {
	valid, _ := hex.DecodeString("428B3E20")
	invalid, _ := hex.DecodeString("428B3FF0")
	if _, err := decodeUplinkUPER(valid, &Message{}); err != nil {
		t.Fatalf("valid payload: %v", err)
	}
	if msg, err := decodeUplinkUPER(invalid, &Message{}); err == nil {
		t.Errorf("decoded %+v; want an error", msg.Elements)
	}
}

// TestDecodeSyntheticElements decodes synthetic payloads (see the comment at
// the top of the file) for elements and types that no corpus message uses.
func TestDecodeSyntheticElements(t *testing.T) {
	tests := []struct {
		name, hexStr string
		dir          MessageDirection
		wantTexts    []string
	}{
		{
			// libacars: HOLD AT fix TAPAS, FL350, 90 deg magnetic, right,
			// leg distance 10.5 nm.
			name: "uM91 with a leg distance", hexStr: "22b22e16e25483420d3ca00b220d00", dir: DirectionUplink,
			wantTexts: []string{"HOLD AT TAPAS MAINTAIN FL350 INBOUND TRACK 090M right TURNS 10.5 nm"},
		},
		{
			// libacars: 45 30.0' north 030 00.0' west, QNH 8000 ft, 270 deg
			// true, left, leg time 1.5 min.
			name: "uM91 with a leg time", hexStr: "22b22e16eeb52c478008320c342380", dir: DirectionUplink,
			wantTexts: []string{"HOLD AT 45.5000,-30.0000 MAINTAIN 8000 ft INBOUND TRACK 270T left TURNS 1.5 min"},
		},
		{
			// libacars: CONTACT Facility Name GANDER, control, Satcom
			// channel 008816312345; CHECK STUCK MICROPHONE UHF 251.000 MHz;
			// ALTIMETER 1013.2 hPa; ERROR reservedErrorMsg6 (index 16).
			name:   "satellite channel, UHF, metric altimeter, error index 16",
			hexStr: "a2b22e1d671e0ce891697c46649d08d15a9d8659099a9227e0", dir: DirectionUplink,
			wantTexts: []string{"CONTACT GANDER CONTROL SATCOM 008816312345", "CHECK STUCK MICROPHONE 251.000 MHz",
				"ALTIMETER 1013.2 hPa", "ERROR reserved error msg"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := decodeHex(t, tt.hexStr, tt.dir)
			if len(msg.Elements) != len(tt.wantTexts) {
				t.Fatalf("%d elements, want %d", len(msg.Elements), len(tt.wantTexts))
			}
			for i, want := range tt.wantTexts {
				if got := msg.Elements[i].Text; got != want {
					t.Errorf("element %d: %q, want %q", i, got, want)
				}
			}
		})
	}
}

// TestDecodeSyntheticPredepartureClearance decodes a uM73 with every
// optional component, including all the additional route information.
// libacars gives each value below (see the comment at the top of the file).
func TestDecodeSyntheticPredepartureClearance(t *testing.T) {
	msg := decodeHex(t, "22b22e127aa31a0b1a13768d315cc93ff7b34e9d99d6a0c184d65c19b424b59061428b656cca53ab3266e63564c420c8b4e245994400f300110d3b231d579f328805e5806458c60c159e18c705a36696a94a5b062068c00e800052001d5001fc65a76601f7830c0c0e7d524650004280041237a9d6a0c10e98e12004bf69304158b369c6042a5520842863b8e326453840", DirectionUplink)
	if len(msg.Elements) != 1 || msg.Elements[0].ID != 73 {
		t.Fatalf("elements %+v", msg.Elements)
	}
	assertJSON(t, msg.Elements[0].Data, `{
		"flight_id": "QFA1",
		"aircraft_type": "B744",
		"equipment_code": {"comnav_approach_available": true, "comnav_status": ["ggnss", "rrnavRouteEquipment"], "ssr": "stransponderModeSPAID"},
		"departure_time": {"hours": 9, "minutes": 15},
		"route_clearance": {
			"airport_departure": "YSSY",
			"airport_destination": "NZAA",
			"runway_departure": {"direction": 34, "configuration": "left"},
			"procedure_departure": {"type": "departure", "name": "KAMPI5", "transition": "ABBEY"},
			"runway_arrival": {"direction": 23, "configuration": "none"},
			"procedure_approach": {"type": "approach", "name": "RNV23"},
			"airway_intercept": "L521",
			"route_information": [
				{"type": "published_identifier", "position": {"type": "fix", "name": "ESDEL", "latitude": -34.001666666666665, "longitude": 152.00333333333333}},
				{"type": "place_bearing_place_bearing", "place_bearings": [
					{"fix_name": "SY", "bearing": {"magnetic": true, "value": 100}},
					{"fix_name": "WOL", "latitude": -34.00833333333333, "longitude": 150.01, "bearing": {"value": 45}}]},
				{"type": "place_bearing_distance", "position": {"type": "place_bearing_distance", "name": "AA", "bearing": {"magnetic": true, "value": 360}, "distance": {"value": 50, "unit": "km"}}},
				{"type": "airway_identifier", "airway": "A464"},
				{"type": "track_detail", "track": {"name": "TRK01", "points": [{"latitude": -35, "longitude": 160}, {"latitude": -36, "longitude": -170}]}}
			],
			"route_info_additional": {
				"along_track_waypoints": [{"position": {"type": "navaid", "name": "SY"}, "distance_tolerance": "minus", "distance": {"value": 12.5, "unit": "nm"}, "speed": {"type": "mach", "value": 105}, "altitudes": [{"tolerance": "atorabove", "altitude": {"type": "gnss_feet", "value": 12345}}]}],
				"reporting_points": {"type": "longitude", "direction": "west", "degrees": 170, "degree_increment": 10},
				"intercept_courses": [{"from": {"type": "latitude_longitude", "position": {"type": "latlon", "latitude": 20, "longitude": 10}}, "degrees": {"value": 10}}],
				"holds_at_waypoint": [{"position": {"type": "airport", "name": "NZAA"}, "speed_low": {"type": "indicated", "value": 210}, "altitude": {"tolerance": "atorbelow", "altitude": {"type": "qfe_metres", "value": 900}}, "direction": "northWest", "degrees": {"magnetic": true, "value": 5}, "efc_time": {"hours": 23, "minutes": 59}, "leg_type": {"distance": {"value": 20, "unit": "km"}}}],
				"waypoint_speed_altitudes": [{"position": {"type": "fix", "name": "XYZ"}, "speed": {"type": "true_metric", "value": 800}}],
				"required_arrival_times": [{"position": {"type": "fix", "name": "RTA"}, "time": {"hours": 1, "minutes": 2}, "time_tolerance": "atorbefore", "tolerance_minutes": 2.5}]
			}
		},
		"altitude_restriction": {"type": "flight_level_metric", "value": 1010},
		"departure_frequency": {"type": "vhf", "value": 123450},
		"beacon_code": {"code": "1234"},
		"revision": 3
	}`)
}

// TestDecodeSyntheticPositionReport decodes a dM48 with every optional
// component, followed by dM57, dM73 and dM78. libacars gives each value
// below (see the comment at the top of the file).
func TestDecodeSyntheticPositionReport(t *testing.T) {
	msg := decodeHex(t, "a2b22e0c3ffffca9f3e2450038a0059e80e40c4a4e206830a19055666cd9861d04d06cf9646bdbcb18f647e708a75685041267469ec1406883412d1c85e4ad24938e4000181040", DirectionDownlink)
	if len(msg.Elements) != 4 {
		t.Fatalf("elements %+v", msg.Elements)
	}
	assertJSON(t, msg.Elements[0].Data, `{
		"position": {"type": "place_bearing_distance", "latitude": -10.001666666666667, "longitude": -20.003333333333334, "name": "OOD", "bearing": {"value": 123}, "distance": {"value": 45.6, "unit": "nm"}},
		"time": {"hours": 3, "minutes": 4},
		"altitude": {"type": "gnss_metres", "value": 10000},
		"fix_next": {"type": "navaid", "name": "ABC"},
		"fix_next_eta": {"hours": 4, "minutes": 5},
		"fix_next_plus_one": {"type": "airport", "name": "YMML"},
		"destination_eta": {"hours": 6, "minutes": 7},
		"remaining_fuel": {"hours": 8, "minutes": 9},
		"temperature": {"value": -40, "unit": "F"},
		"wind": {"direction": 360, "speed": 300, "unit": "km/h"},
		"turbulence": "severe",
		"icing": "trace",
		"speed": {"type": "mach", "value": 84},
		"ground_speed": {"type": "ground", "value": 520},
		"vertical_change": {"direction": "down", "rate": {"value": 1500, "unit": "m/min"}},
		"track_angle": {"magnetic": true, "value": 200},
		"true_heading": {"value": 201},
		"distance": {"value": 1000, "unit": "km"},
		"supplementary_information": "SUPP INFO",
		"reported_waypoint_position": {"type": "latlon", "latitude": 5, "longitude": -6},
		"reported_waypoint_time": {"hours": 2, "minutes": 3},
		"reported_waypoint_altitude": {"type": "qfe_feet", "value": 1500}
	}`)
	wantTexts := []string{"", "1h30m OF FUEL REMAINING AND 300 SOULS ON BOARD", "2", "AT 07:08 0.1 nm from A"}
	for i := 1; i < 4; i++ {
		if got := msg.Elements[i].Text; got != wantTexts[i] {
			t.Errorf("element %d: %q, want %q", i, got, wantTexts[i])
		}
	}
}

// TestConvertEveryElementType converts a zero value of every element
// alternative's type and checks that the conversion exists and fills each
// of the template's placeholders.
func TestConvertEveryElementType(t *testing.T) {
	placeholder := regexp.MustCompile(`\[[^\]]+\]`)
	for _, c := range []struct {
		choice reflect.Type
		label  func(int) string
	}{
		{reflect.TypeOf(UPERUplinkElement{}), GetUplinkLabel},
		{reflect.TypeOf(UPERDownlinkElement{}), GetDownlinkLabel},
	} {
		for i := 0; i < c.choice.NumField(); i++ {
			f := c.choice.Field(i)
			n, ok := choiceNumber(f.Tag.Get("asn1"))
			if !ok {
				t.Fatalf("%s.%s has no choice number", c.choice.Name(), f.Name)
			}
			label := c.label(n)
			data, fills, err := convertValue(reflect.New(f.Type.Elem()).Elem())
			if err != nil {
				t.Errorf("%s (%s): %v", f.Name, label, err)
				continue
			}
			want := len(placeholder.FindAllString(label, -1))
			if data == nil {
				// NULL: only uM178's template has a placeholder (see
				// placeholderUplink).
				want = 0
			}
			if len(fills) != want {
				t.Errorf("%s (%s): %d placeholder values, want %d", f.Name, label, len(fills), want)
			}
		}
	}
}

// parseCPDLC parses an ACARS message with the cpdlc parser.
func parseCPDLC(t *testing.T, label, text string) *Result {
	t.Helper()
	r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: label, Text: text}).(*Result)
	if !ok {
		t.Fatal("Parse returned no result")
	}
	return r
}

// decodeHex decodes a hex payload (without the CRC) in the given direction.
func decodeHex(t *testing.T, hexStr string, dir MessageDirection) *Message {
	t.Helper()
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := DecodeWithUPER(data, dir)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if msg.Direction != dir {
		t.Fatalf("direction %v, want %v", msg.Direction, dir)
	}
	return msg
}

// assertJSON checks that v marshals to the same JSON as want.
func assertJSON(t *testing.T, v interface{}, want string) {
	t.Helper()
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var gotValue, wantValue interface{}
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("expected JSON: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("got  %s\nwant %s", got, strings.Join(strings.Fields(want), " "))
	}
}
