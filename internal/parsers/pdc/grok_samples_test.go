package pdc

import (
	"fmt"
	"testing"
)

// TestSampleMessages tests a variety of real-world PDC message formats.
func TestSampleMessages(t *testing.T) {
	c := NewCompiler()
	if err := c.Compile(); err != nil {
		t.Fatalf("failed to compile patterns: %v", err)
	}

	samples := []struct {
		name string
		text string
	}{
		{
			name: "Helsinki FIN5LA to ESSA",
			text: `/HELCLXA.DC1/CLD 1832 251229 EFHK PDC
728
FIN5LA CLRD TO ESSA OFF
04R VIA ADIVO5C
SQUAWK 0437 NEXT FREQ
121.800
QNH 992
TSAT 1900
CLIMB TO 4000 FT`,
		},
		{
			name: "London Heathrow QTR58U to OTHH",
			text: `/LHRCDYA.DC1/CLD 1835 251229 EGLL PDC 607
	QTR58U CLRD TO OTHH OFF 09R VIA DET1J
	SQUAWK 3403 ADT 1855 ATIS U
	NO CTOT; 0148E`,
		},
		{
			name: "London Heathrow KAL908 to RKSI",
			text: `/LHRDCXA.DC1/CLD 1838 251229 EGLL PDC 070
	KAL908 CLRD TO RKSI OFF 09R VIA DET1J
	SQUAWK 4622 ADT 1850 ATIS U
	NO CTOT; 04669`,
		},
		{
			name: "Warsaw LOT3859 to EPWR",
			text: `/WAWDLYA.DC1/CLD 1840 251229 EPWA
PDC 002
LOT3859 CLRD TO EPWR
OFF 29 VIA SOXER7G INITIAL
CLIMB ALTITUDE 6000 FEET
SQUAWK 1000 ATIS T`,
		},
		{
			name: "US Delta DAL2699 full routing",
			text: `42 PDC 2699 MSP RDU
***DATE/TIME OF PDC RECEIPT: 29DEC 1827Z

**** PREDEPARTURE  CLEARANCE ****

DAL2699 DEPARTING KMSP  TRANSPONDER 3631
SKED DEP TIME 1857   EQUIP  A320/L
FILED FLT LEVEL 350
     ROUTING
****************************************
 -CLEARED AS FILED-
KMSP ZMBRO7 ODI JVL CGT BONNT
DQN HNN FRIKY ALDAN4 KRDU
****************************************
CLEARED ZMBRO7 DEPARTURE ODI TRSN
CLB VIA SID EXC MAINT 7000FT
EXP 350 10 MIN AFT DP,DPFRQ 124.7
XPCT RWY 30L`,
		},
		{
			name: "Australian Jetstar truncated runway",
			text: `.MELOJJQ 291828
AGM
AN VH-X3N/MA 365A
-  /
PDC 291826
JST400 A320 YSSY 1910
CLEARED TO YBCG VIA
16LKEVIN7 DEP OLSEM:TRAN
ROUTE:DCT OLSEM Y193 BANDA Y43 BERNI DCT
CLIMB VIA SID TO: 5000
DEP FREQ: 123.000
SQUAWK 1234`,
		},
		// Additional global DC1 samples.
		{
			name: "Frankfurt DAL15 to KATL",
			text: `/FRADFYA.DC1/CLD 0839 230201 EDDF PDC 881
DLA15 CLRD TO KATL OFF 25C VIA OBOKA2G
SQUAWK 0122 ADT MDI NEXT FREQ 121.905 ATIS R
REQ STARTUP ACC TSAT ON 121.905`,
		},
		{
			name: "Hong Kong CPA729 to WMKK",
			text: `/HKGCPYA.DC1/CLD 0809 230201 VHHH PDC 354
	CPA729 CLRD TO WMKK OFF 25L VIA PECAN1B
	SQUAWK 5132 NEXT FREQ 122.150 ATIS V
	CLIMB VIA SID TO 5000FT. ACK PDC. CTC DELIVERY ON 122.15 WHEN READY TO START`,
		},
		{
			name: "Singapore SIA964 to WIII",
			text: `/SINCXYA.DC1/CLD 0909 230201 WSSS PDC 001
	SIA964 CLRD TO WIII OFF 02R VIA CHANGI1C
	SQUAWK 2214 NEXT FREQ 121.650 ATIS K
	X ANITO FL350 OR ABOVE`,
		},
		{
			name: "Bangkok KLM803 to RPLL",
			text: `/BKKDCXA.DC1/CLD 0743 230201 VTBS PDC 301
	KLM803 CLRD TO RPLL OFF 01R VIA DOSBU3K L880 ALT060
	SQUAWK 0747
	SELECT ACCEPT FUNCTION TO ACK CLR & CTC GROUND FREQ FOR PUSH BACK & START UP`,
		},
		{
			name: "Sao Paulo QTR8155 to SEQM",
			text: `/RIOCGYA.DC1/CLD 0820 230201 SBGR PDC 551
	QTR8155 CLRD TO SEQM OFF 10L VIA
	UKBEV1D/UKBEV/F400/UL201 ASTOB UM417 RORIT UZ8
	CIA UM775 ANRIR UL655 AKTOR UM665 IQT UM776
	QIT DCT
	SQUAWK 4327 ADT 0845 NEXT FREQ 126.900 ATIS P
	APP SP 120,45`,
		},
		{
			name: "Seoul UAL892 to KSFO",
			text: `/ICNDLXA.DC1/CLD 0845 230201 RKSI PDC 159
	UAL892 CLRD TO KSFO OFF 34R VIA EGOBA2Y Y697 FL 230
	SQUAWK 7135 CONTACT APRON 121.65 ATIS C`,
		},
	}

	for _, s := range samples {
		t.Run(s.name, func(t *testing.T) {
			result := c.Parse(s.text)
			if result == nil {
				t.Errorf("FAILED TO PARSE - no result returned")
				return
			}

			// Print what we extracted.
			fmt.Printf("\n=== %s ===\n", s.name)
			fmt.Printf("  Format:      %s\n", result.FormatName)
			fmt.Printf("  Flight:      %s\n", result.FlightNumber)
			fmt.Printf("  Origin:      %s\n", result.Origin)
			fmt.Printf("  Destination: %s\n", result.Destination)
			fmt.Printf("  Aircraft:    %s\n", result.Aircraft)
			fmt.Printf("  Runway:      %s\n", result.Runway)
			fmt.Printf("  SID:         %s\n", result.SID)
			fmt.Printf("  Squawk:      %s\n", result.Squawk)
			fmt.Printf("  Altitude:    %s\n", result.Altitude)
			fmt.Printf("  Frequency:   %s\n", result.Frequency)
			fmt.Printf("  ATIS:        %s\n", result.ATIS)
			fmt.Printf("  Route:       %s\n", result.Route)
		})
	}
}