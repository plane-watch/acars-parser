// Package loadsheet provides Grok-like pattern composition for loadsheet parsing.
package loadsheet

import (
	"regexp"
	"strings"
)

// LoadsheetFormat represents a specific loadsheet message format.
type LoadsheetFormat struct {
	Name     string
	Labels   []string       // Which ACARS labels this format appears on.
	Pattern  *regexp.Regexp // Compiled regex with named capture groups.
	WeightUnit string       // "kg" or "tonnes" - affects how we interpret numbers.
}

// LoadsheetFormats defines the known loadsheet message formats.
// Order matters - more specific patterns should come first.
var LoadsheetFormats = []LoadsheetFormat{
	// Format A: Swiss/Lufthansa/Edelweiss/LOT/Saudia standard format with PAX line.
	// This is the most common format with weights in KG.
	// Example:
	// LOADSHEET FINAL 1736 EDNO1
	// LX1376/21     21JAN26
	// ZRH WRO HB-AZH   2/3
	// ZFW 39754  MAX 46700
	// TOF 4800
	// TOW 44554  MAX 54000
	// TIF 2000
	// LAW 42554  MAX 49050   L
	// ...
	// PAX/6/59 TTL 65
	{
		Name:   "standard_kg",
		Labels: []string{"C1", "RA", "H1", "30", "31", "2A", "22", "35", "45", "13", "42"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<status>FINAL|PRELIM)\s+(?P<time>\d{4})\s+(?:EDNO?\s*(?P<edition>\d+))?` +
			`.*?` +
			`(?P<flight>[A-Z]{2}\d{1,4}[A-Z]?)/\d+\s+\d+[A-Z]{3}\d+\s*\n` +
			`\s*(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<tail>[A-Z0-9-]+)\s+(?P<crew>\d+/\d+)` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)` +
			`(?:.*?TIF\s+(?P<tif>\d+))?` +
			`.*?` +
			`LAW\s+(?P<law>\d+)\s+MAX\s+(?P<law_max>\d+)` +
			`.*?` +
			`PAX/(?P<pax_breakdown>[\d/]+)\s+TTL\s+(?P<pax_total>\d+)` +
			`(?:.*?MACZFW\s+(?P<mac_zfw>[\d.]+))?` +
			`(?:.*?MACTOW\s+(?P<mac_tow>[\d.]+))?`),
		WeightUnit: "kg",
	},

	// Format A2: Standard format without PAX line (partial messages or cargo).
	// Same as standard but LAW is the endpoint.
	// Example:
	// -  LOADSHEET FINAL 1902 EDNO1
	// AT969/31      31DEC25
	// VLC CMN CN-RGQ   2/3
	// ZFW 33901  MAX 40900   L
	// TOF 6100
	// TOW 40001  MAX 51800
	{
		Name:   "standard_kg_minimal",
		Labels: []string{"C1", "RA", "H1", "30", "31", "2A", "22", "35", "45", "13", "42"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<status>FINAL|PRELIM)\s+(?P<time>\d{4})\s+(?:EDNO?\s*(?P<edition>\d+))?` +
			`.*?` +
			`(?P<flight>[A-Z]{2}\d{1,4}[A-Z]?)/\d+\s+\d+[A-Z]{3}\d+\s*\n` +
			`\s*(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<tail>[A-Z0-9-]+)\s+(?P<crew>\d+/\d+)` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)`),
		WeightUnit: "kg",
	},

	// Format B: Qantas format with weights in tonnes (decimal).
	// Example:
	// FINAL LOADSHEET
	// ...
	// QF009  24JAN PER VH-ZNL 4/10 1900
	// ZFW   150.6
	// TOF   100.1
	// TOW   250.7
	// TIF    94.6
	// LAW   156.1
	{
		Name:   "qantas_tonnes",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`(?P<status>FINAL|PRELIM(?:INARY)?)\s+LOADSHEET` +
			`.*?` +
			`(?:REGO|REG):\s*(?P<tail>[A-Z0-9-]+)` +
			`.*?` +
			`FLIGHT:\s*(?P<flight>[A-Z]{2}\d{1,4})` +
			`.*?` +
			// Summary line repeats flight/tail - match generically since Go doesn't support backreferences.
			`[A-Z]{2}\d{1,4}\s+\d+[A-Z]{3}\s+(?P<origin>[A-Z]{3})\s+[A-Z0-9-]+\s+(?P<crew>\d+/\d+)` +
			`.*?` +
			`ZFW\s+(?P<zfw>[\d.]+)` +
			`.*?` +
			`TOF\s+(?P<tof>[\d.]+)` +
			`.*?` +
			`TOW\s+(?P<tow>[\d.]+)` +
			`.*?` +
			`TIF\s+(?P<tif>[\d.]+)` +
			`.*?` +
			`LAW\s+(?P<law>[\d.]+)`),
		WeightUnit: "tonnes",
	},

	// Format C: British Airways format with full field names.
	// Example:
	// L O A D S H E E T           CHECKED      APPROVED         EDNO
	// ...
	// EZE BCN LL 2604      ECNNH   C42Y269      3/08    18JAN26 1918
	// ...
	// ZERO FUEL WEIGHT ACTUAL 149904 MAX 166000
	// TAKE OFF FUEL            75982
	// TAKE OFF WEIGHT  ACTUAL 225886 MAX 242000
	{
		Name:   "ba_full_names",
		Labels: []string{"10", "14"},
		Pattern: regexp.MustCompile(`(?s)` +
			`L\s+O\s+A\s+D\s+S\s+H\s+E\s+E\s+T` +
			`.*?` +
			`(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<flight>[A-Z]{2}\s*\d{1,4})\s+(?P<tail>[A-Z0-9]+)\s+[A-Z0-9]+\s+(?P<crew>\d+/\d+)\s+(?P<date>\d+[A-Z]{3}\d+)` +
			`.*?` +
			`ZERO\s+FUEL\s+WEIGHT\s+ACTUAL\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TAKE\s+OFF\s+FUEL\s+(?P<tof>\d+)` +
			`.*?` +
			`TAKE\s+OFF\s+WEIGHT\s+ACTUAL\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)` +
			`.*?` +
			`(?:TRIP\s+FUEL\s+(?P<tif>\d+))?` +
			`.*?` +
			`LANDING\s+WEIGHT\s+ACTUAL\s+(?P<law>\d+)\s+MAX\s+(?P<law_max>\d+)`),
		WeightUnit: "kg",
	},

	// Format D: JetSmart format with DOW/TTL fields.
	// Example:
	// JETSMART LOADSHEET
	// WJ3814/31     31DEC25     EDNO 001
	// AEP GIG   CCAWA 2/4   31DEC25 1517
	// TTL  16346
	// DOW  42412
	// ZFW  58758  MAX  61000
	// TOF  11787
	// TOW  70545  MAX  78000
	// TIF   6308
	// LAW  64237  MAX  64500 L
	// PAX/184 TTL 185
	{
		Name:   "jetsmart",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`JETSMART\s+LOADSHEET` +
			`.*?` +
			`(?P<flight>[A-Z]{2}\d{1,4})/\d+\s+\d+[A-Z]{3}\d+\s+(?:EDNO?\s*(?P<edition>\d+))?` +
			`.*?` +
			`(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<tail>[A-Z0-9]+)\s+(?P<crew>\d+/\d+)` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)` +
			`(?:.*?TIF\s+(?P<tif>\d+))?` +
			`(?:.*?LAW\s+(?P<law>\d+)\s+MAX\s+(?P<law_max>\d+))?` +
			`(?:.*?PAX/(?P<pax_breakdown>[\d/]+)\s+TTL\s+(?P<pax_total>\d+))?`),
		WeightUnit: "kg",
	},

	// Format D1b: JetSmart minimal (for truncated messages with only ZFW).
	// Example:
	// JETSMART LOADSHEET
	// WJ3814/31     31DEC25     EDNO 001
	// AEP GIG   CCAWA 2/4   31DEC25 1517
	// ZFW  58758  MAX  61000
	// TOF
	{
		Name:   "jetsmart_minimal",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`JETSMART\s+LOADSHEET` +
			`.*?` +
			`(?P<flight>[A-Z]{2}\d{1,4}[A-Z]?)/\d+\s+\d+[A-Z]{3}\d+\s+(?:EDNO?\s*(?P<edition>\d+))?` +
			`.*?` +
			`(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<tail>[A-Z0-9]+)\s+(?P<crew>\d+/\d+)` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)`),
		WeightUnit: "kg",
	},

	// Format D2: Chinese airlines format (Tibet, Sichuan, etc.) with structured layout.
	// Example:
	// TIBET AIRLINES (or SI CHUAN AIRLINES)
	// LOADSHEET           EDNO 01
	// FLIGHT:TV9780/03JAN26   CZXYBP B6440
	// VERSION:C4Y132     CREW:3/6/0    CAB:0
	// ZFW       050059  MACZFW: 30.94
	//    MZFW    58500  L
	// TOF        11020
	// TOW        61079  MACTOW: 28.91
	{
		Name:   "chinese_airlines",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`(?:TIBET|SI\s+CHUAN|SICHUAN|CHINA\s+EASTERN|CHINA\s+SOUTHERN|HAINAN|XIAMEN|OKAY|AIR\s+CHINA|SHAN\s*DONG|SPRING)\s+(?:AIRLINES?|AIRWAYS)?(?:\s+CO\.?\s*(?:LTD)?)?` +
			`.*?` +
			`LOADSHEET\s+EDNO?\s*(?P<edition>\d+)` +
			`.*?` +
			// Flight can be 2-letter or number+letter code (e.g., 3U6904 for Sichuan).
			`FLIGHT:\s*(?P<flight>[A-Z0-9]{2}\d{1,4})/\d+[A-Z]{3}\d+\s+(?P<origin>[A-Z]{3,6})\s+(?P<tail>[A-Z0-9]+)` +
			`.*?` +
			`CREW:\s*(?P<crew>\d+/\d+(?:/\d+)?)` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+MACZFW:\s*(?P<mac_zfw>[\d.]+)` +
			`.*?` +
			`MZFW\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+MACTOW:\s*(?P<mac_tow>[\d.]+)` +
			`.*?` +
			`MTOW\s+(?P<tow_max>\d+)` +
			`(?:.*?LDW\s+(?P<law>\d+))?` +
			`(?:.*?MLDW\s+(?P<law_max>\d+))?` +
			`(?:.*?TTL:\s*(?P<pax_total>\d+))?`),
		WeightUnit: "kg",
	},

	// Format E: Jat Airways with BW/DOW fields.
	// Example:
	// -    LOADSHEET FINAL 001 2348
	// JU1353/10 10JAN26
	// HHN INI YUAPF 2/3
	// BW     40516
	// DOW    41355
	// ZFW   47433 MAX 57000
	{
		Name:   "jat_bw_dow",
		Labels: []string{"3S"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<status>FINAL|PRELIM)\s+(?:00)?(?P<edition>\d+)\s+(?P<time>\d{4})` +
			`.*?` +
			`(?P<flight>[A-Z]{2}\d{1,4}[A-Z]?)/\d+\s+\d+[A-Z]{3}\d+\s*\n` +
			`\s*(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<tail>[A-Z0-9]+)\s+(?P<crew>\d+/\d+)` +
			`.*?` +
			`(?:BW\s+(?P<bw>\d+))?` +
			`.*?` +
			`(?:DOW\s+(?P<dow>\d+))?` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)` +
			`.*?` +
			`(?:TIF\s+(?P<tif>\d+))?` +
			`.*?` +
			`LAW\s+(?P<law>\d+)\s+MAX\s+(?P<law_max>\d+)`),
		WeightUnit: "kg",
	},

	// Format E2: Air Malta/European style with FINAL EDN format.
	// Example:
	// -  LOADSHEET FINAL EDN02
	// KM478/03 03JAN26 0555
	// MLA CDG 9HNEE 2/5
	// ZFW   58492 MAX 64300 L
	// TOF    8100
	// TOW   66592 MAX 73500
	// TIF    5157
	// LAW   61435 MAX 67400
	{
		Name:   "european_edn",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<status>FINAL|PRELIM)\s+EDN(?P<edition>\d+)` +
			`.*?` +
			`(?P<flight>[A-Z]{2}\d{1,4})/\d+\s+\d+[A-Z]{3}\d+` +
			`.*?` +
			`(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<tail>[A-Z0-9]+)\s+(?P<crew>\d+/\d+)` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)` +
			`(?:.*?TIF\s+(?P<tif>\d+))?` +
			`(?:.*?LAW\s+(?P<law>\d+)\s+MAX\s+(?P<law_max>\d+))?` +
			`(?:.*?MACZFW\s+(?P<mac_zfw>[\d.]+))?` +
			`(?:.*?MACTOW\s+(?P<mac_tow>[\d.]+))?` +
			`(?:.*?PAX/(?P<pax_breakdown>[\d/]+)\s+TTL\s+(?P<pax_total>\d+))?`),
		WeightUnit: "kg",
	},

	// Format F: Cathay Pacific format with ACT weights.
	// Example:
	// - LOADSHEET                    PRELIM 01
	// NRT HKG      CX0527/03          B-LQE
	// J38W28Y214         2/11        03JAN26
	// ZFW ACT   174249      MAX 195700  L
	// TO FUEL    33300
	// TOW ACT   207549      MAX 240000
	{
		Name:   "cathay_act",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<status>FINAL|PRELIM)\s+(?P<edition>\d+)` +
			`.*?` +
			`(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<flight>[A-Z]{2}\d{1,4})/\d+\s+(?P<tail>[A-Z0-9-]+)` +
			`.*?` +
			`(?P<crew>\d+/\d+)` +
			`.*?` +
			`ZFW\s+ACT\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TO\s+FUEL\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+ACT\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)` +
			`(?:.*?TRIP\s+FUEL\s+(?P<tif>\d+))?` +
			`(?:.*?LAW\s+ACT\s+(?P<law>\d+)\s+MAX\s+(?P<law_max>\d+))?` +
			`(?:.*?MACZFW\s+(?P<mac_zfw>[\d.]+))?` +
			`(?:.*?MACTOW\s+(?P<mac_tow>[\d.]+))?` +
			`(?:.*?TTL\s+PAX\s+(?P<pax_total>\d+))?`),
		WeightUnit: "kg",
	},

	// Format G: TUI/Thomson format with EDN header.
	// Example:
	// LOADSHEET EDN01
	// TOM13/02   02JAN26 2240
	// BGI LGW GTUIM 2/8
	// ZFW 157832 MAX 181436 L
	{
		Name:   "tui_edn",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+EDN(?P<edition>\d+)` +
			`.*?` +
			`(?P<flight>[A-Z]{2,3}\d{1,4})/\d+\s+\d+[A-Z]{3}\d+` +
			`.*?` +
			`(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<tail>[A-Z0-9]+)\s+(?P<crew>\d+/\d+)` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)` +
			`(?:.*?TIF\s+(?P<tif>\d+))?` +
			`(?:.*?LAW\s+(?P<law>\d+)\s+MAX\s+(?P<law_max>\d+))?` +
			`(?:.*?MACZFW\s+(?P<mac_zfw>[\d.]+))?` +
			`(?:.*?MACTOW\s+(?P<mac_tow>[\d.]+))?`),
		WeightUnit: "kg",
	},

	// Format H: EAT/Cargo format with weights in pounds.
	// Example:
	// LOADSHEET  QY5547   REF 2F2QV
	// --- FOR INFORMATION ONLY ---
	// FLT  WAW-0LEJ  A306EAT / DAEAK
	//       WEIGHT  INDEX  ALL WGHTS LB
	// ZFW   219777  30,28 286600 Max
	// TOF    37350  13,37
	// TOW   257127  43,65 321750 Op. Max
	// LW    244027  50,03 308650 Max
	{
		Name:   "eat_cargo_lb",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<flight>[A-Z0-9]+)\s+REF` +
			`.*?` +
			`FLT\s+(?P<origin>[A-Z]{3})-\d?(?P<destination>[A-Z]{3})\s+[A-Z0-9]+\s*/\s*(?P<tail>[A-Z0-9]+)` +
			`.*?` +
			`ALL\s+WGHTS\s+LB` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+[\d,.]+\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+[\d,.]+\s+(?P<tow_max>\d+)` +
			`.*?` +
			`(?:LW|LAW)\s+(?P<law>\d+)\s+[\d,.]+\s+(?P<law_max>\d+)`),
		WeightUnit: "lb",
	},

	// Format H1a: Ethiopian Airlines format with EDNO-N dash.
	// Example:
	// LOADSHEET FINAL 1133
	// ET318/07JAN26 07JAN26 EDNO-1
	// ADD NBO ETAZN C30Y325    2/15
	// ZFW  177624 MAX 195700
	// TOF   18900
	// TOW  196524 MAX 196700 L
	{
		Name:   "ethiopian",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<status>FINAL|PRELIM)\s+(?P<time>\d{4})` +
			`.*?` +
			`(?P<flight>[A-Z]{2}\d{1,4})/\d+[A-Z]{3}\d+\s+\d+[A-Z]{3}\d+\s+EDNO-(?P<edition>\d+)` +
			`.*?` +
			`(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<tail>[A-Z0-9]+)\s+[A-Z0-9]+\s+(?P<crew>\d+/\d+)` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)` +
			`(?:.*?TIF\s+(?P<tif>\d+))?` +
			`(?:.*?LAW\s+(?P<law>\d+)\s+MAX\s+(?P<law_max>\d+))?` +
			`(?:.*?MACZFW\s+(?P<mac_zfw>[\d.]+))?` +
			`(?:.*?MACTOW\s+(?P<mac_tow>[\d.]+))?` +
			`(?:.*?PAX/(?P<pax_breakdown>[\d/]+)\s+TTL\s+(?P<pax_total>\d+))?`),
		WeightUnit: "kg",
	},

	// Format H1b: Kalitta/Cargo format with weights in LB (no INDEX column).
	// Example:
	// LOADSHEET  K4235   REF 2FHQY
	// FLT  HKG-ANC  B777F-CKS / N794CK
	//       WEIGHT  ALL WGHTS LB
	// ZFW   491211 547000 Max
	// TOF   191000
	// TOW   682211 738000 Op. Max
	// LW    517621 575000 Max
	{
		Name:   "kalitta_cargo_lb",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<flight>[A-Z0-9]+)\s+REF` +
			`.*?` +
			`FLT\s+(?P<origin>[A-Z]{3})-(?P<destination>[A-Z]{3})\s+[A-Z0-9-]+\s*/\s*(?P<tail>[A-Z0-9]+)` +
			`.*?` +
			`ALL\s+WGHTS\s+LB` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+(?P<tow_max>\d+)` +
			`.*?` +
			`(?:LW|LAW)\s+(?P<law>\d+)\s+(?P<law_max>\d+)`),
		WeightUnit: "lb",
	},

	// Format H1c: Kalitta older format (no REF in header).
	// Example:
	// LOADSHEET CKS605
	// FLT FAI-ORD Boeing 747-400F / N716CK
	//        WEIGHT  ALL WGHTS  LB
	// ZFW    510500 610000 Max
	// TOF    157000
	// TOW    667500 870000 Max
	// LW     553217 652000 Op. Max
	{
		Name:   "kalitta_old_lb",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<flight>[A-Z0-9]+)\s*\n` +
			`.*?` +
			`FLT\s+(?P<origin>[A-Z]{3})-(?P<destination>[A-Z]{3})\s+[A-Za-z0-9 -]+\s*/\s*(?P<tail>[A-Z0-9]+)` +
			`.*?` +
			`ALL\s+WGHTS\s+LB` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+(?P<tow_max>\d+)` +
			`.*?` +
			`(?:LW|LAW)\s+(?P<law>\d+)\s+(?P<law_max>\d+)`),
		WeightUnit: "lb",
	},

	// Format H2: DHL/Cargo format with weights in KG (not LB).
	// Example:
	// LOADSHEET  D0522   REF 2F4RX
	// FLT  EMA-LEJ  B777-DHK1 / GDHLY
	//       WEIGHT  INDEX  ALL WGHTS KG
	// ZFW   192374  48,41 248115 Max
	// TOF    20300  -0,29
	// TOW   212674  48,12 268415 Op. Max
	// LW    203974  48,06 260815 Max
	{
		Name:   "dhl_cargo_kg",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<flight>[A-Z0-9]+)\s+REF` +
			`.*?` +
			`FLT\s+(?P<origin>[A-Z]{3})-\d?(?P<destination>[A-Z]{3})\s+[A-Z0-9-]+\s*/\s*(?P<tail>[A-Z0-9]+)` +
			`.*?` +
			`ALL\s+WGHTS\s+KG` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+[\d,.-]+\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOF\s+(?P<tof>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+[\d,.-]+\s+(?P<tow_max>\d+)` +
			`.*?` +
			`(?:LW|LAW)\s+(?P<law>\d+)\s+[\d,.-]+\s+(?P<law_max>\d+)`),
		WeightUnit: "kg",
	},

	// Format I: French Bee/Corsair short format (no TOF line).
	// Example:
	// LOADSHEET PRELIM 1032
	// SS926/04 04JAN26
	// ORY PTP F-HRNB 2/8
	// ZFW 171572  MAX 181000  L
	// TOW 224086  MAX 251000
	// LAW 179782  MAX 191000
	// PAX/20/21/312 TTL 355
	{
		Name:   "french_bee_short",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`LOADSHEET\s+(?P<status>FINAL|PRELIM)\s+(?P<time>\d{4})` +
			`.*?` +
			`(?P<flight>[A-Z]{2}\d{1,4})/\d+\s+\d+[A-Z]{3}\d+\s*\n` +
			`\s*(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<tail>[A-Z0-9-]+)\s+(?P<crew>\d+/\d+)` +
			`.*?` +
			`ZFW\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TOW\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)` +
			`.*?` +
			`LAW\s+(?P<law>\d+)\s+MAX\s+(?P<law_max>\d+)` +
			`(?:.*?PAX/(?P<pax_breakdown>[\d/]+)\s+TTL\s+(?P<pax_total>\d+))?`),
		WeightUnit: "kg",
	},

	// Format J: VIC/Corsair format with full weight names and EST values.
	// Example:
	// VIC                     01
	// FROM/TO FLIGHT        A/C REG VERSION      CREW    DATE    TIME
	// ORY CAY TX570/03JAN   FOFDF   J12W24Y267   2/8/0   03JAN26 1250
	// ZERO FUEL WEIGHT   EST  157825 MAX 170000
	// TAKE OFF FUEL            65037
	// TAKE OFF WEIGHT    EST  222862 MAX 230000  L
	// LANDING WEIGHT     EST  168906 MAX 182000
	{
		Name:   "vic_corsair",
		Labels: []string{"C1"},
		Pattern: regexp.MustCompile(`(?s)` +
			`VIC\s+(?P<edition>\d+)` +
			`.*?` +
			`FROM/TO\s+FLIGHT` +
			`.*?` +
			`(?P<origin>[A-Z]{3})\s+(?P<destination>[A-Z]{3})\s+(?P<flight>[A-Z]{2}\d{1,4})/\d+[A-Z]{3}\s+(?P<tail>[A-Z0-9-]+)\s+[A-Z0-9]+\s+(?P<crew>\d+/\d+(?:/\d+)?)` +
			`.*?` +
			`ZERO\s+FUEL\s+WEIGHT\s+EST\s+(?P<zfw>\d+)\s+MAX\s+(?P<zfw_max>\d+)` +
			`.*?` +
			`TAKE\s+OFF\s+FUEL\s+(?P<tof>\d+)` +
			`.*?` +
			`TAKE\s+OFF\s+WEIGHT\s+EST\s+(?P<tow>\d+)\s+MAX\s+(?P<tow_max>\d+)` +
			`(?:.*?TRIP\s+FUEL\s+(?P<tif>\d+))?` +
			`.*?` +
			`LANDING\s+WEIGHT\s+EST\s+(?P<law>\d+)\s+MAX\s+(?P<law_max>\d+)` +
			`(?:.*?MACZFW\s+(?P<mac_zfw>[\d.]+))?` +
			`(?:.*?MACTOW\s+(?P<mac_tow>[\d.]+))?`),
		WeightUnit: "kg",
	},
}

// MatchFormat tries to match a message against known loadsheet formats.
// Returns the matching format and captured values, or nil if no match.
func MatchFormat(text, label string) (*LoadsheetFormat, map[string]string) {
	// Normalise text - handle both \r\n and \n, and \t characters.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\t", " ")

	for i := range LoadsheetFormats {
		format := &LoadsheetFormats[i]

		// Check if this format applies to the given label.
		labelMatch := false
		for _, l := range format.Labels {
			if l == label {
				labelMatch = true
				break
			}
		}
		if !labelMatch {
			continue
		}

		// Try to match the pattern.
		match := format.Pattern.FindStringSubmatch(text)
		if match == nil {
			continue
		}

		// Extract named groups into a map.
		result := make(map[string]string)
		for i, name := range format.Pattern.SubexpNames() {
			if name != "" && i < len(match) && match[i] != "" {
				result[name] = strings.TrimSpace(match[i])
			}
		}

		return format, result
	}

	return nil, nil
}