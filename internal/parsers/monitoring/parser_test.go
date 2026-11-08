package monitoring

import (
	"testing"

	"acars_parser/internal/acars"
)

// The messages are real H1 monitoring reports from the archive (October
// 2026), cut after their header lines.
func TestParse(t *testing.T) {
	tests := []struct {
		name, tail, text string
		want             *Result
	}{
		{
			name: "CSV report with an ICAO callsign",
			tail: "N3118J",
			text: "R001V0106,180302,, N3118J,5318410-07,736738,736345,00118,KCHS,KBOS,18:31:51,2026:10:06,JBU568  ,000010,260307,5000(18:35:04),/Q,0.15",
			want: &Result{Format: "csv", Report: "R001V0106", Registration: "N3118J", Flight: "JBU568",
				Origin: "KCHS", Destination: "KBOS", Date: "2026-10-06", Time: "18:31:51"},
		},
		{
			name: "CSV report with the flight number's digits",
			tail: "N113DQ",
			text: "R001V0106,170300,, N113DQ,5318410-07,736171,736028,07739,KMSP,KEWR,14:53:28,2026:10:05,2895    ,000010,130114",
			want: &Result{Format: "csv", Report: "R001V0106", Registration: "N113DQ", FlightNumberDigits: "2895",
				Origin: "KMSP", Destination: "KEWR", Date: "2026-10-05", Time: "14:53:28"},
		},
		{
			name: "APM report",
			tail: "N845MD",
			text: "APM    1 N845MD         AAL86   KORDEGLL241026001522,\r\n,ACMF0105,.841,,,274.7",
			want: &Result{Format: "apm", Report: "APM", Registration: "N845MD", Flight: "AAL86",
				Origin: "KORD", Destination: "EGLL", Date: "2026-10-24", Time: "00:15:22"},
		},
		{
			name: "APM report under a B03A line",
			tail: "N17417",
			text: "B03A      \r\nAPM     N17417          UAL1680 KLAXKIAH241026052726,737789     ,BCG3BL2000003,.786",
			want: &Result{Format: "apm", Report: "APM", Registration: "N17417", Flight: "UAL1680",
				Origin: "KLAX", Destination: "KIAH", Date: "2026-10-24", Time: "05:27:26"},
		},
		{
			name: "ACM report",
			tail: "N873FD",
			text: "ACM01ACM230FSN873FD  FDX48   KMEMEBLG041026  20ER235115632673162FEDAP107\r\n 229015940A02",
			want: &Result{Format: "acm", Report: "ACM230", Registration: "N873FD", Flight: "FDX48",
				Origin: "KMEM", Destination: "EBLG", Date: "2026-10-04"},
		},
		{
			name: "ACM report with the callsign against the tail",
			tail: "A6-ECJ",
			text: "ACM01ACM220DSEKA6-ECJUAE795  GOBDOMDB041026 111CL191551 6308317CEAD11709\r\n 1639012821012",
			want: &Result{Format: "acm", Report: "ACM220", Registration: "A6-ECJ", Flight: "UAE795",
				Origin: "GOBD", Destination: "OMDB", Date: "2026-10-04"},
		},
		{
			name: "engine report with the date before the route",
			tail: "N27908",
			text: "<330>KEG\r\n8  N27908UAL109    CL  41051026103207EDDMKIAD  1605  2\r\n 23818 -25 -279XXX",
			want: &Result{Format: "engine", Report: "<330>KEG", Registration: "N27908", Flight: "UAL109",
				Origin: "EDDM", Destination: "KIAD", Date: "2026-10-05", Time: "10:32:07"},
		},
		{
			name: "engine report with the route before the date",
			tail: "A6-BNA",
			text: "<310>JEG\r\n9  A6-BNAETD7NW    WMKKOMAA021026023518 463TO461927  4\r\n    401073162",
			want: &Result{Format: "engine", Report: "<310>JEG", Registration: "A6-BNA", Flight: "ETD7NW",
				Origin: "WMKK", Destination: "OMAA", Date: "2026-10-02", Time: "02:35:18"},
		},
		{
			name: "engine report without a station digit",
			tail: "HL8732",
			text: "<701>IEG\r\n HL8732KAL969    ER  25231026145243RKSILKPR39770010C  7\r\n  339972960849",
			want: &Result{Format: "engine", Report: "<701>IEG", Registration: "HL8732", Flight: "KAL969",
				Origin: "RKSI", Destination: "LKPR", Date: "2026-10-23", Time: "14:52:43"},
		},
		{
			name: "station report",
			tail: "HL7624",
			text: "/ILNGE7X.CLB103.HL7624KE3KAL234    261023131018KSEARKSI+00007  8F959638670001",
			want: &Result{Format: "station", Report: "CLB103", Registration: "HL7624", Flight: "KAL234",
				Origin: "KSEA", Destination: "RKSI", Date: "2026-10-23", Time: "13:10:18"},
		},
		{
			name: "station report with an airline code before the tail",
			tail: "N446AM",
			text: "/ILNGE7X.CLB006AM N446AM2AMX021    261008040732MMMXLEMD+006050295897086",
			want: &Result{Format: "station", Report: "CLB006", Registration: "N446AM", Flight: "AMX021",
				Origin: "MMMX", Destination: "LEMD", Date: "2026-10-08", Time: "04:07:32"},
		},
		// Live traffic (October and November 2026).
		{
			name: "station report with an airline code right before the tail",
			tail: "HZ-AR22",
			text: "/ILNGE7X.SCR005SVHZ-AR221SVA123    261102071957OEJNEGCC-026070295690373000013000022447M85P04",
			want: &Result{Format: "station", Report: "SCR005", Registration: "HZ-AR22", Flight: "SVA123",
				Origin: "OEJN", Destination: "EGCC", Date: "2026-11-02", Time: "07:19:57"},
		},
		{
			name: "station report with two dots before the tail",
			tail: "B-LJC",
			text: "/ILNGE7X.TOF103..B-LJCCX1CPA020    261102031356YSSYYMML+00005  8F959356670001000022124M70P04",
			want: &Result{Format: "station", Report: "TOF103", Registration: "B-LJC", Flight: "CPA020",
				Origin: "YSSY", Destination: "YMML", Date: "2026-11-02", Time: "03:13:56"},
		},
		{
			name: "station report named with four digits",
			tail: "LX-VCN",
			text: "/ILNGE7X.EL1101.LX-VCNCV2CLX5485   261030090453ZHCCRJNK+00014  8F959219670000E00022124M70P04",
			want: &Result{Format: "station", Report: "EL1101", Registration: "LX-VCN", Flight: "CLX5485",
				Origin: "ZHCC", Destination: "RJNK", Date: "2026-10-30", Time: "09:04:53"},
		},
		{
			name: "station report after a ground address",
			tail: "C-FHCM",
			text: "/ILNGE7X PARJBXH.6029631WSC-FHCM WJA2267 MPTOCYYC000028FF046PFS57409092611012233442628M86P12",
			want: &Result{Format: "station", Registration: "C-FHCM", Flight: "WJA2267", Origin: "MPTO", Destination: "CYYC"},
		},
		{
			name: "GE engine report",
			tail: "VN-A862",
			text: "/ILNGE7X.<103>GEG\r\n9   1VN-A862 HVN30     EDDFVVTS041126 392 6123313 1537\r\nBSM5A70990054\r\n",
			want: &Result{Format: "ge", Report: "103", Registration: "VN-A862", Flight: "HVN30", Origin: "EDDF", Destination: "VVTS"},
		},
		{
			name: "GE engine report with the callsign after the tail",
			tail: "N826AN",
			text: "/ILNGE7X.<102>HEG9   1\r\n N826ANAAL169    KLAXRJTT1449TA261026081422  759\r\nAAL46ACMF0102524806\r\n",
			want: &Result{Format: "ge", Report: "102", Registration: "N826AN", Flight: "AAL169", Origin: "KLAX", Destination: "RJTT"},
		},
		{
			name: "EB report",
			tail: "N29977",
			text: "EB0116AB_9 , 56, N29977,UAL7      ,   6,081026,19:09:41,KIAH,RJAA,13198\r\nBSM5B70000022",
			want: &Result{Format: "eb", Report: "EB0116AB", Registration: "N29977", Flight: "UAL7",
				Origin: "KIAH", Destination: "RJAA", Date: "2026-10-08", Time: "19:09:41"},
		},
		{
			name: "ABS report",
			tail: "N1811U",
			text: "ABS001DA_N1811U  KSNAKSJC384\r\n 33989-1191700452027099-248270025XA0W/X0150200000--",
			want: &Result{Format: "abs", Report: "ABS001DA", Registration: "N1811U", Origin: "KSNA", Destination: "KSJC"},
		},
		{
			name: "SB report",
			tail: "F-GSQL",
			text: "SB0122BA_F-GSQL  KIADLFPG176\r\n  51169-0010260453 38002-530018029098/9W028040",
			want: &Result{Format: "abs", Report: "SB0122BA", Registration: "F-GSQL", Origin: "KIAD", Destination: "LFPG"},
		},
		{
			// Live traffic (November 2026): a space before the digits.
			name: "ABS report with a space before the digits",
			tail: "N8906K",
			text: "ABS001DA_N8906K  KDALKBWI 72\r\n 38702 -767840301009613+05-214004X2005Z0110130000--\r\n",
			want: &Result{Format: "abs", Report: "ABS001DA", Registration: "N8906K", Origin: "KDAL", Destination: "KBWI"},
		},
		{
			// Live traffic (November 2026): a space before the registration.
			name: "ABS report with a space before the registration",
			tail: "N8756S",
			text: "ABS001DA_ N8756S KDALKAUS727\r\n 30313 -975671424006592+09-266018XA8YY,0210360000--\r\n",
			want: &Result{Format: "abs", Report: "ABS001DA", Registration: "N8756S", Origin: "KDAL", Destination: "KAUS"},
		},
		{
			// A truncated field (the first character and the last ones)
			// consistent with the tail is the tail's, but gives no
			// registration (live traffic, November 2026).
			name: "ABS report with a truncated registration",
			tail: "C-FFIP",
			text: "ABS001DA_C   IP  MMTLCYYZ970\r\n  37663",
			want: &Result{Format: "abs", Report: "ABS001DA", Origin: "MMTL", Destination: "CYYZ"},
		},
		{
			name: "ABS report with the last characters of the registration",
			tail: "TC-VFR",
			text: "ABS001DA_    FR  LFSBLTFJ 68\r\n 46554  140431708036997-52-068010XA0/9W0",
			want: &Result{Format: "abs", Report: "ABS001DA", Origin: "LFSB", Destination: "LTFJ"},
		},
		{
			name: "ABS report with the first character of the registration",
			tail: "C-FNWD",
			text: "ABS011DA_C       KSFOCYYC1\r\n  37663",
			want: &Result{Format: "abs", Report: "ABS011DA", Origin: "KSFO", Destination: "CYYC"},
		},
		{
			name: "ABS report with dots after the first character",
			tail: "VH-8VG",
			text: "ABS001DA_V...... YBCGYMML159\r\n  37663",
			want: &Result{Format: "abs", Report: "ABS001DA", Origin: "YBCG", Destination: "YMML"},
		},
		{
			// Alaska sends the 24-bit address, here N290AK's.
			name: "ABS report with the aircraft address",
			tail: "N290AK",
			text: "ABS001DA_A2F584  KLAXKPDX833\r\n  37663",
			want: &Result{Format: "abs", Report: "ABS001DA", Origin: "KLAX", Destination: "KPDX"},
		},
		{
			// Live traffic (November 2026): report 071, the header on the
			// second line.
			name: "ABS report 071",
			tail: "N806AK",
			text: "ABS071BA_\r\nN806AK  0764KSMFKSEA0311262092ER033030  554\r\nBCG2F-S200-0008 26002+\r\n",
			want: &Result{Format: "abs", Report: "ABS071BA", Registration: "N806AK", FlightNumberDigits: "0764",
				Origin: "KSMF", Destination: "KSEA", Date: "2026-11-03"},
		},
		{
			name: "ABS report 071 with a truncated registration",
			tail: "TC-SLA",
			text: "ABS071BA_\r\nT   LA  0003LTAUEHAM081126 231ER101033 2564\r\nBCG27-S200-0011 33981+\r\n",
			want: &Result{Format: "abs", Report: "ABS071BA", FlightNumberDigits: "0003",
				Origin: "LTAU", Destination: "EHAM", Date: "2026-11-08"},
		},
		{
			// A blank registration field names no aircraft.
			name: "ABS report without a registration",
			tail: "G-EUPA",
			text: "ABS011DA_....... LGRPELLX1\r\n  37639  219100936036003-56-070058XNI:9W1563760000--",
			want: &Result{Format: "abs", Report: "ABS011DA", Origin: "LGRP", Destination: "ELLX"},
		},
		{
			name: "E report with the flight number's digits",
			tail: "N209UA",
			text: "E23C01209L819KIAHSAEZ  37252261004260448ER\r\n157 284484030043005",
			want: &Result{Format: "e", Report: "E23C", FlightNumberDigits: "819", Origin: "KIAH", Destination: "SAEZ"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&Parser{}).Parse(&acars.Message{ID: 6, Label: "H1", Tail: tt.tail, Text: tt.text})
			r, ok := got.(*Result)
			if !ok {
				t.Fatalf("got %T, want *Result", got)
			}
			tt.want.MsgID = 6
			if *r != *tt.want {
				t.Errorf("got  %+v\nwant %+v", *r, *tt.want)
			}
		})
	}
}

// A report about another aircraft (a registration that is not the
// transmitted tail) is not parsed: its flight and route would be credited
// to the transmitting aircraft. The formats that need the transmitted tail
// to find their fields are not parsed without one.
func TestParseRejects(t *testing.T) {
	tests := []struct{ name, tail, text string }{
		{"CSV, another aircraft", "N3118K",
			"R001V0106,180302,, N3118J,5318410-07,736738,736345,00118,KCHS,KBOS,18:31:51,2026:10:06,JBU568  ,000010"},
		{"APM, another aircraft", "N845ME", "APM    1 N845MD         AAL86   KORDEGLL241026001522,"},
		{"ACM, no tail", "", "ACM01ACM230FSN873FD  FDX48   KMEMEBLG041026  20ER235115632673162FEDAP107"},
		{"engine, another aircraft", "N27909", "<330>KEG\r\n8  N27908UAL109    CL  41051026103207EDDMKIAD  1605  2"},
		{"engine, no callsign", "N845MD", "<202>JEG\r\n9  N845MD0000000000PO 2910710260704080000000025328 12"},
		{"invalid airport", "N29977", "EB0116AB_9 , 56, N29977,UAL7      ,   6,081026,19:09:41,KIAH,QQQQ,13198"},
		{"invalid date", "N845MD", "APM    1 N845MD         AAL86   KORDEGLL321326001522,"},
		{"ABS, another aircraft", "N1811V", "ABS001DA_N1811U  KSNAKSJC384\r\n 33989"},
		{"ABS, a truncated field of another aircraft", "C-FNWD", "ABS001DA_C   IP  MMTLCYYZ970\r\n  37663"},
		{"ABS, another aircraft's address", "N290AK", "ABS001DA_A3133C  KLAXKSEA494\r\n  37663"},
		// A transmitted tail that is part of the report's registration.
		{"station, tail inside the registration", "N446", "/ILNGE7X.CLB006AM N446AM2AMX021    261008040732MMMXLEMD+006050295"},
		{"station, tail inside the registration after a dot", "HL762", "/ILNGE7X.CLB103.HL7624KE3KAL234    261023131018KSEARKSI+00007"},
		{"station, tail inside the registration after a code", "HZ-AR2", "/ILNGE7X.SCR005SVHZ-AR221SVA123    261102071957OEJNEGCC-026070"},
		{"station, tail inside the registration after a ground address", "C-FHC", "/ILNGE7X PARJBXH.6029631WSC-FHCM WJA2267 MPTOCYYC000028FF046PFS"},
		{"GE, tail inside the registration", "N826A", "/ILNGE7X.<102>HEG9   1\r\n N826ANAAL169    KLAXRJTT1449TA261026081422  759"},
		{"GE, tail inside the registration before a space", "VN-A86", "/ILNGE7X.<103>GEG\r\n9   1VN-A862 HVN30     EDDFVVTS041126 392"},
		{"acm, tail at the end of another registration", "N123", "ACM01ACM230F9H-N123FDX48   KMEMEBLG041026  20ER235115632673162FEDAP107"},
		// A flight field that names no flight this parser can read.
		{"csv, unreadable flight", "N3118J",
			"R001V0106,180302,, N3118J,5318410-07,736738,736345,00118,KCHS,KBOS,18:31:51,2026:10:06,UAL1A2,000010"},
		{"not a report", "N845MD", "POSN39006W075260,OYVAY,014938,340"},
	}
	for _, tt := range tests {
		if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: tt.tail, Text: tt.text}); r != nil {
			t.Errorf("%s: got %+v, want nil", tt.name, r)
		}
	}
}
