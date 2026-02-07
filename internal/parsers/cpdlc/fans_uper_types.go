package cpdlc

// FANS-1/A CPDLC ASN.1 types for use with github.com/shaneshort/go-asn/uper.
// Based on ARINC 622 / FANS-1/A specification.
//
// This file uses UPER (Unaligned PER) encoding, which is the correct
// encoding for FANS-1/A CPDLC messages.

// =============================================================================
// Top-Level Message Structures
// =============================================================================

// UPERDownlinkMessage is the top-level downlink message structure.
type UPERDownlinkMessage struct {
	Header   UPERMessageHeader
	Element  UPERDownlinkElement
	Elements []UPERDownlinkElement `asn1:"optional,size:1..4"`
}

// UPERUplinkMessage is the top-level uplink message structure.
type UPERUplinkMessage struct {
	Header   UPERMessageHeader
	Element  UPERUplinkElement
	Elements []UPERUplinkElement `asn1:"optional,size:1..4"`
}

// UPERMessageHeader is the common header for uplink and downlink messages.
type UPERMessageHeader struct {
	MsgID     int  `asn1:"size:0..63"`
	MsgRef    *int `asn1:"optional,size:0..63"`
	Timestamp *UPERTimestamp `asn1:"optional"`
}

// UPERTimestamp is hours:minutes:seconds.
type UPERTimestamp struct {
	Hours   int `asn1:"size:0..23"`
	Minutes int `asn1:"size:0..59"`
	Seconds int `asn1:"size:0..59"`
}

// UPERTime is hours:minutes (no seconds).
type UPERTime struct {
	Hours   int `asn1:"size:0..23"`
	Minutes int `asn1:"size:0..59"`
}

// =============================================================================
// Downlink Element (CHOICE of 129 elements: dM0-dM128)
// =============================================================================

// UPERDownlinkElement is a CHOICE of 129 downlink message types.
// Only commonly used elements are fully typed; others are left as raw data.
type UPERDownlinkElement struct {
	// dM0-dM5: Simple acknowledgements (NULL)
	DM0Wilco    *struct{} `asn1:"choice:0"`
	DM1Unable   *struct{} `asn1:"choice:1"`
	DM2Standby  *struct{} `asn1:"choice:2"`
	DM3Roger    *struct{} `asn1:"choice:3"`
	DM4Affirm   *struct{} `asn1:"choice:4"`
	DM5Negative *struct{} `asn1:"choice:5"`

	// dM6-dM10: Altitude-related
	DM6RequestAltitude      *UPERAltitude `asn1:"choice:6"`
	DM7RequestBlock         *UPERAltitudeAltitude `asn1:"choice:7"`
	DM8RequestCruiseClimb   *UPERAltitude `asn1:"choice:8"`
	DM9RequestClimb         *UPERAltitude `asn1:"choice:9"`
	DM10RequestDescent      *UPERAltitude `asn1:"choice:10"`

	// dM11-dM12: Position + Altitude
	DM11AtPositionRequestClimb   *UPERPositionAltitude `asn1:"choice:11"`
	DM12AtPositionRequestDescent *UPERPositionAltitude `asn1:"choice:12"`

	// dM13-dM14: Time + Altitude
	DM13AtTimeRequestClimb   *UPERTimeAltitude `asn1:"choice:13"`
	DM14AtTimeRequestDescent *UPERTimeAltitude `asn1:"choice:14"`

	// dM15-dM17: Offset requests
	DM15RequestOffset         *UPERDistanceOffsetDirection `asn1:"choice:15"`
	DM16AtPositionRequestOffset *UPERPositionDistanceOffsetDirection `asn1:"choice:16"`
	DM17AtTimeRequestOffset   *UPERTimeDistanceOffsetDirection `asn1:"choice:17"`

	// dM18-dM19: Speed requests
	DM18RequestSpeed  *UPERSpeed `asn1:"choice:18"`
	DM19RequestSpeedRange *UPERSpeedSpeed `asn1:"choice:19"`

	// dM20: Request voice contact (NULL)
	DM20RequestVoiceContact *struct{} `asn1:"choice:20"`

	// dM21: Request voice contact with frequency
	DM21RequestVoiceFrequency *UPERFrequency `asn1:"choice:21"`

	// dM22-dM24: Position/procedure requests
	DM22RequestDirectTo *UPERPosition `asn1:"choice:22"`
	DM23RequestProcedure *UPERProcedureName `asn1:"choice:23"`
	DM24RequestRoute    *UPERRouteClearance `asn1:"choice:24"`

	// dM25: Request clearance (NULL)
	DM25RequestClearance *struct{} `asn1:"choice:25"`

	// dM26-dM27: Weather deviation
	DM26WeatherDeviationTo *UPERPositionRouteClearance `asn1:"choice:26"`
	DM27WeatherDeviationOffset *UPERDistanceOffsetDirection `asn1:"choice:27"`

	// dM28-dM40: Status reports
	DM28Leaving        *UPERAltitude `asn1:"choice:28"`
	DM29ClimbingTo     *UPERAltitude `asn1:"choice:29"`
	DM30DescendingTo   *UPERAltitude `asn1:"choice:30"`
	DM31Passing        *UPERPosition `asn1:"choice:31"`
	DM32PresentAltitude *UPERAltitude `asn1:"choice:32"`
	DM33PresentPosition *UPERPosition `asn1:"choice:33"`
	DM34PresentSpeed   *UPERSpeed `asn1:"choice:34"`
	DM35PresentHeading *UPERDegrees `asn1:"choice:35"`
	DM36PresentGroundTrack *UPERDegrees `asn1:"choice:36"`
	DM37Level          *UPERAltitude `asn1:"choice:37"`
	DM38AssignedAltitude *UPERAltitude `asn1:"choice:38"`
	DM39AssignedSpeed  *UPERSpeed `asn1:"choice:39"`
	DM40AssignedRoute  *UPERRouteClearance `asn1:"choice:40"`

	// dM41: Back on route (NULL)
	DM41BackOnRoute *struct{} `asn1:"choice:41"`

	// dM42-dM46: Waypoint reports
	DM42NextWaypoint     *UPERPosition `asn1:"choice:42"`
	DM43NextWaypointETA  *UPERTime `asn1:"choice:43"`
	DM44EnsuingWaypoint  *UPERPosition `asn1:"choice:44"`
	DM45ReportedWaypoint *UPERPosition `asn1:"choice:45"`
	DM46ReportedWaypointTime *UPERTime `asn1:"choice:46"`

	// dM47: Squawk code
	DM47Squawking *UPERBeaconCode `asn1:"choice:47"`

	// dM48: Position report - THE KEY ONE!
	DM48PositionReport *UPERPositionReport `asn1:"choice:48"`

	// dM49-dM54: Various requests
	DM49WhenCanWeExpectSpeed     *UPERSpeed `asn1:"choice:49"`
	DM50WhenCanWeExpectSpeedRange *UPERSpeedSpeed `asn1:"choice:50"`
	DM51WhenBackOnRoute          *struct{} `asn1:"choice:51"`
	DM52WhenLowerAltitude        *struct{} `asn1:"choice:52"`
	DM53WhenHigherAltitude       *struct{} `asn1:"choice:53"`
	DM54WhenCruiseClimb          *UPERAltitude `asn1:"choice:54"`

	// dM55-dM58: Emergency
	DM55PanPanPan  *struct{} `asn1:"choice:55"`
	DM56MaydayMayday *struct{} `asn1:"choice:56"`
	DM57FuelSouls  *UPERRemainingFuelSouls `asn1:"choice:57"`
	DM58CancelEmergency *struct{} `asn1:"choice:58"`

	// dM59-dM80: Various other messages
	DM59DivertingTo    *UPERPositionRouteClearance `asn1:"choice:59"`
	DM60Offsetting     *UPERDistanceOffsetDirection `asn1:"choice:60"`
	DM61DescendingTo2  *UPERAltitude `asn1:"choice:61"`
	DM62Error          *UPERErrorInformation `asn1:"choice:62"`
	DM63NotCurrentDataAuthority *struct{} `asn1:"choice:63"`
	DM64Facility       *string `asn1:"choice:64,ia5string,size:4..4"`
	DM65DueToWeather   *struct{} `asn1:"choice:65"`
	DM66DueToPerformance *struct{} `asn1:"choice:66"`
	DM67FreeTextLow    *string `asn1:"choice:67,ia5string,size:1..256"`
	DM68FreeTextDistress *string `asn1:"choice:68,ia5string,size:1..256"`
	DM69RequestVMCDescent *struct{} `asn1:"choice:69"`
	DM70RequestHeading *UPERDegrees `asn1:"choice:70"`
	DM71RequestGroundTrack *UPERDegrees `asn1:"choice:71"`
	DM72Reaching       *UPERAltitude `asn1:"choice:72"`
	DM73VersionNumber  *int `asn1:"choice:73,size:0..15"`
	DM74MaintainOwnSep *struct{} `asn1:"choice:74"`
	DM75AtPilotsDiscretion *struct{} `asn1:"choice:75"`
	DM76ReachingBlock  *UPERAltitudeAltitude `asn1:"choice:76"`
	DM77AssignedBlock  *UPERAltitudeAltitude `asn1:"choice:77"`
	DM78AtTimeDistance *UPERTimeDistanceToFromPosition `asn1:"choice:78"`
	DM79ATIS           *string `asn1:"choice:79,ia5string,size:1..1"`
	DM80Deviating      *UPERDistanceOffsetDirection `asn1:"choice:80"`

	// dM81-dM128: Reserved (NULL)
	DM81Reserved  *struct{} `asn1:"choice:81"`
	DM82Reserved  *struct{} `asn1:"choice:82"`
	DM83Reserved  *struct{} `asn1:"choice:83"`
	DM84Reserved  *struct{} `asn1:"choice:84"`
	DM85Reserved  *struct{} `asn1:"choice:85"`
	DM86Reserved  *struct{} `asn1:"choice:86"`
	DM87Reserved  *struct{} `asn1:"choice:87"`
	DM88Reserved  *struct{} `asn1:"choice:88"`
	DM89Reserved  *struct{} `asn1:"choice:89"`
	DM90Reserved  *struct{} `asn1:"choice:90"`
	DM91Reserved  *struct{} `asn1:"choice:91"`
	DM92Reserved  *struct{} `asn1:"choice:92"`
	DM93Reserved  *struct{} `asn1:"choice:93"`
	DM94Reserved  *struct{} `asn1:"choice:94"`
	DM95Reserved  *struct{} `asn1:"choice:95"`
	DM96Reserved  *struct{} `asn1:"choice:96"`
	DM97Reserved  *struct{} `asn1:"choice:97"`
	DM98Reserved  *struct{} `asn1:"choice:98"`
	DM99Reserved  *struct{} `asn1:"choice:99"`
	DM100Reserved *struct{} `asn1:"choice:100"`
	DM101Reserved *struct{} `asn1:"choice:101"`
	DM102Reserved *struct{} `asn1:"choice:102"`
	DM103Reserved *struct{} `asn1:"choice:103"`
	DM104Reserved *struct{} `asn1:"choice:104"`
	DM105Reserved *struct{} `asn1:"choice:105"`
	DM106Reserved *struct{} `asn1:"choice:106"`
	DM107Reserved *struct{} `asn1:"choice:107"`
	DM108Reserved *struct{} `asn1:"choice:108"`
	DM109Reserved *struct{} `asn1:"choice:109"`
	DM110Reserved *struct{} `asn1:"choice:110"`
	DM111Reserved *struct{} `asn1:"choice:111"`
	DM112Reserved *struct{} `asn1:"choice:112"`
	DM113Reserved *struct{} `asn1:"choice:113"`
	DM114Reserved *struct{} `asn1:"choice:114"`
	DM115Reserved *struct{} `asn1:"choice:115"`
	DM116Reserved *struct{} `asn1:"choice:116"`
	DM117Reserved *struct{} `asn1:"choice:117"`
	DM118Reserved *struct{} `asn1:"choice:118"`
	DM119Reserved *struct{} `asn1:"choice:119"`
	DM120Reserved *struct{} `asn1:"choice:120"`
	DM121Reserved *struct{} `asn1:"choice:121"`
	DM122Reserved *struct{} `asn1:"choice:122"`
	DM123Reserved *struct{} `asn1:"choice:123"`
	DM124Reserved *struct{} `asn1:"choice:124"`
	DM125Reserved *struct{} `asn1:"choice:125"`
	DM126Reserved *struct{} `asn1:"choice:126"`
	DM127Reserved *struct{} `asn1:"choice:127"`
	DM128Reserved *struct{} `asn1:"choice:128"`
}

// UPERUplinkElement is a CHOICE of 183 uplink message types (uM0-uM182).
type UPERUplinkElement struct {
	// uM0-uM5: Simple responses (NULL).
	UM0Unable          *struct{} `asn1:"choice:0"`
	UM1Standby         *struct{} `asn1:"choice:1"`
	UM2RequestDeferred *struct{} `asn1:"choice:2"`
	UM3Roger           *struct{} `asn1:"choice:3"`
	UM4Affirm          *struct{} `asn1:"choice:4"`
	UM5Negative        *struct{} `asn1:"choice:5"`

	// uM6: EXPECT [altitude].
	UM6Altitude *UPERAltitude `asn1:"choice:6"`

	// uM7-uM12: Time/position expect messages.
	UM7Time      *UPERTime     `asn1:"choice:7"`
	UM8Position  *UPERPosition `asn1:"choice:8"`
	UM9Time      *UPERTime     `asn1:"choice:9"`
	UM10Position *UPERPosition `asn1:"choice:10"`
	UM11Time     *UPERTime     `asn1:"choice:11"`
	UM12Position *UPERPosition `asn1:"choice:12"`

	// uM13-uM18: Time/position + altitude.
	UM13TimeAltitude     *UPERTimeAltitude     `asn1:"choice:13"`
	UM14PositionAltitude *UPERPositionAltitude `asn1:"choice:14"`
	UM15TimeAltitude     *UPERTimeAltitude     `asn1:"choice:15"`
	UM16PositionAltitude *UPERPositionAltitude `asn1:"choice:16"`
	UM17TimeAltitude     *UPERTimeAltitude     `asn1:"choice:17"`
	UM18PositionAltitude *UPERPositionAltitude `asn1:"choice:18"`

	// uM19-uM25: Altitude commands.
	UM19Altitude         *UPERAltitude         `asn1:"choice:19"`
	UM20Altitude         *UPERAltitude         `asn1:"choice:20"`
	UM21TimeAltitude     *UPERTimeAltitude     `asn1:"choice:21"`
	UM22PositionAltitude *UPERPositionAltitude `asn1:"choice:22"`
	UM23Altitude         *UPERAltitude         `asn1:"choice:23"`
	UM24TimeAltitude     *UPERTimeAltitude     `asn1:"choice:24"`
	UM25PositionAltitude *UPERPositionAltitude `asn1:"choice:25"`

	// uM26-uM29: Altitude + time/position.
	UM26AltitudeTime     *UPERAltitudeTime     `asn1:"choice:26"`
	UM27AltitudePosition *UPERAltitudePosition `asn1:"choice:27"`
	UM28AltitudeTime     *UPERAltitudeTime     `asn1:"choice:28"`
	UM29AltitudePosition *UPERAltitudePosition `asn1:"choice:29"`

	// uM30-uM32: Block altitudes.
	UM30AltitudeAltitude *UPERAltitudeAltitude `asn1:"choice:30"`
	UM31AltitudeAltitude *UPERAltitudeAltitude `asn1:"choice:31"`
	UM32AltitudeAltitude *UPERAltitudeAltitude `asn1:"choice:32"`

	// uM33-uM41: More altitude commands.
	UM33Altitude *UPERAltitude `asn1:"choice:33"`
	UM34Altitude *UPERAltitude `asn1:"choice:34"`
	UM35Altitude *UPERAltitude `asn1:"choice:35"`
	UM36Altitude *UPERAltitude `asn1:"choice:36"`
	UM37Altitude *UPERAltitude `asn1:"choice:37"`
	UM38Altitude *UPERAltitude `asn1:"choice:38"`
	UM39Altitude *UPERAltitude `asn1:"choice:39"`
	UM40Altitude *UPERAltitude `asn1:"choice:40"`
	UM41Altitude *UPERAltitude `asn1:"choice:41"`

	// uM42-uM50: Cross position at altitude.
	UM42PositionAltitude         *UPERPositionAltitude         `asn1:"choice:42"`
	UM43PositionAltitude         *UPERPositionAltitude         `asn1:"choice:43"`
	UM44PositionAltitude         *UPERPositionAltitude         `asn1:"choice:44"`
	UM45PositionAltitude         *UPERPositionAltitude         `asn1:"choice:45"`
	UM46PositionAltitude         *UPERPositionAltitude         `asn1:"choice:46"`
	UM47PositionAltitude         *UPERPositionAltitude         `asn1:"choice:47"`
	UM48PositionAltitude         *UPERPositionAltitude         `asn1:"choice:48"`
	UM49PositionAltitude         *UPERPositionAltitude         `asn1:"choice:49"`
	UM50PositionAltitudeAltitude *UPERPositionAltitudeAltitude `asn1:"choice:50"`

	// uM51-uM54: Cross position at time.
	UM51PositionTime     *UPERPositionTime     `asn1:"choice:51"`
	UM52PositionTime     *UPERPositionTime     `asn1:"choice:52"`
	UM53PositionTime     *UPERPositionTime     `asn1:"choice:53"`
	UM54PositionTimeTime *UPERPositionTimeTime `asn1:"choice:54"`

	// uM55-uM57: Cross position at speed.
	UM55PositionSpeed *UPERPositionSpeed `asn1:"choice:55"`
	UM56PositionSpeed *UPERPositionSpeed `asn1:"choice:56"`
	UM57PositionSpeed *UPERPositionSpeed `asn1:"choice:57"`

	// uM58-uM63: Complex crossing instructions.
	UM58PositionTimeAltitude      *UPERPositionTimeAltitude      `asn1:"choice:58"`
	UM59PositionTimeAltitude      *UPERPositionTimeAltitude      `asn1:"choice:59"`
	UM60PositionTimeAltitude      *UPERPositionTimeAltitude      `asn1:"choice:60"`
	UM61PositionAltitudeSpeed     *UPERPositionAltitudeSpeed     `asn1:"choice:61"`
	UM62TimePositionAltitude      *UPERTimePositionAltitude      `asn1:"choice:62"`
	UM63TimePositionAltitudeSpeed *UPERTimePositionAltitudeSpeed `asn1:"choice:63"`

	// uM64-uM66: Offset instructions.
	UM64DistanceOffsetDirection         *UPERDistanceOffsetDirection         `asn1:"choice:64"`
	UM65PositionDistanceOffsetDirection *UPERPositionDistanceOffsetDirection `asn1:"choice:65"`
	UM66TimeDistanceOffsetDirection     *UPERTimeDistanceOffsetDirection     `asn1:"choice:66"`

	// uM67: PROCEED BACK ON ROUTE.
	UM67ProceedBackOnRoute *struct{} `asn1:"choice:67"`

	// uM68-uM71: Rejoin route.
	UM68Position *UPERPosition `asn1:"choice:68"`
	UM69Time     *UPERTime     `asn1:"choice:69"`
	UM70Position *UPERPosition `asn1:"choice:70"`
	UM71Time     *UPERTime     `asn1:"choice:71"`

	// uM72: RESUME OWN NAVIGATION.
	UM72ResumeOwnNav *struct{} `asn1:"choice:72"`

	// uM73: Pre-departure clearance (complex).
	UM73PDC *UPERPredepartureClearance `asn1:"choice:73"`

	// uM74-uM78: Direct to position.
	UM74Position         *UPERPosition         `asn1:"choice:74"`
	UM75Position         *UPERPosition         `asn1:"choice:75"`
	UM76TimePosition     *UPERTimePosition     `asn1:"choice:76"`
	UM77PositionPosition *UPERPositionPosition `asn1:"choice:77"`
	UM78AltitudePosition *UPERAltitudePosition `asn1:"choice:78"`

	// uM79-uM90: Route clearances.
	UM79PositionRouteClearance *UPERPositionRouteClearance `asn1:"choice:79"`
	UM80RouteClearance         *UPERRouteClearance         `asn1:"choice:80"`
	UM81ProcedureName          *UPERProcedureName          `asn1:"choice:81"`
	UM82DistanceOffsetDirection *UPERDistanceOffsetDirection `asn1:"choice:82"`
	UM83PositionRouteClearance *UPERPositionRouteClearance `asn1:"choice:83"`
	UM84PositionProcedureName  *UPERPositionProcedureName  `asn1:"choice:84"`
	UM85RouteClearance         *UPERRouteClearance         `asn1:"choice:85"`
	UM86PositionRouteClearance *UPERPositionRouteClearance `asn1:"choice:86"`
	UM87Position               *UPERPosition               `asn1:"choice:87"`
	UM88PositionPosition       *UPERPositionPosition       `asn1:"choice:88"`
	UM89TimePosition           *UPERTimePosition           `asn1:"choice:89"`
	UM90AltitudePosition       *UPERAltitudePosition       `asn1:"choice:90"`

	// uM91-uM93: Hold instructions.
	UM91HoldClearance    *UPERHoldClearance    `asn1:"choice:91"`
	UM92PositionAltitude *UPERPositionAltitude `asn1:"choice:92"`
	UM93Time             *UPERTime             `asn1:"choice:93"`

	// uM94-uM98: Heading/track instructions.
	UM94DirectionDegrees *UPERDirectionDegrees `asn1:"choice:94"`
	UM95DirectionDegrees *UPERDirectionDegrees `asn1:"choice:95"`
	UM96FlyPresentHdg    *struct{}             `asn1:"choice:96"`
	UM97PositionDegrees  *UPERPositionDegrees  `asn1:"choice:97"`
	UM98DirectionDegrees *UPERDirectionDegrees `asn1:"choice:98"`

	// uM99: EXPECT [procedurename].
	UM99ProcedureName *UPERProcedureName `asn1:"choice:99"`

	// uM100-uM105: Expect speed.
	UM100TimeSpeed         *UPERTimeSpeed         `asn1:"choice:100"`
	UM101PositionSpeed     *UPERPositionSpeed     `asn1:"choice:101"`
	UM102AltitudeSpeed     *UPERAltitudeSpeed     `asn1:"choice:102"`
	UM103TimeSpeedSpeed    *UPERTimeSpeedSpeed    `asn1:"choice:103"`
	UM104PositionSpeedSpeed *UPERPositionSpeedSpeed `asn1:"choice:104"`
	UM105AltitudeSpeedSpeed *UPERAltitudeSpeedSpeed `asn1:"choice:105"`

	// uM106-uM116: Speed instructions.
	UM106Speed        *UPERSpeed    `asn1:"choice:106"`
	UM107MaintainSpd  *struct{}     `asn1:"choice:107"`
	UM108Speed        *UPERSpeed    `asn1:"choice:108"`
	UM109Speed        *UPERSpeed    `asn1:"choice:109"`
	UM110SpeedSpeed   *UPERSpeedSpeed `asn1:"choice:110"`
	UM111Speed        *UPERSpeed    `asn1:"choice:111"`
	UM112Speed        *UPERSpeed    `asn1:"choice:112"`
	UM113Speed        *UPERSpeed    `asn1:"choice:113"`
	UM114Speed        *UPERSpeed    `asn1:"choice:114"`
	UM115Speed        *UPERSpeed    `asn1:"choice:115"`
	UM116ResumeNormal *struct{}     `asn1:"choice:116"`

	// uM117-uM122: Contact/monitor frequency.
	UM117UnitFreq         *UPERICAOUnitNameFrequency         `asn1:"choice:117"`
	UM118PositionUnitFreq *UPERPositionICAOUnitNameFrequency `asn1:"choice:118"`
	UM119TimeUnitFreq     *UPERTimeICAOUnitNameFrequency     `asn1:"choice:119"`
	UM120UnitFreq         *UPERICAOUnitNameFrequency         `asn1:"choice:120"`
	UM121PositionUnitFreq *UPERPositionICAOUnitNameFrequency `asn1:"choice:121"`
	UM122TimeUnitFreq     *UPERTimeICAOUnitNameFrequency     `asn1:"choice:122"`

	// uM123-uM127: Squawk instructions.
	UM123BeaconCode   *UPERBeaconCode `asn1:"choice:123"`
	UM124StopSquawk   *struct{}       `asn1:"choice:124"`
	UM125SquawkAlt    *struct{}       `asn1:"choice:125"`
	UM126StopAltSquawk *struct{}      `asn1:"choice:126"`
	UM127ReportBack   *struct{}       `asn1:"choice:127"`

	// uM128-uM130: Report instructions.
	UM128Altitude *UPERAltitude `asn1:"choice:128"`
	UM129Altitude *UPERAltitude `asn1:"choice:129"`
	UM130Position *UPERPosition `asn1:"choice:130"`

	// uM131-uM147: Confirm/report requests (NULL).
	UM131ReportFuelSouls  *struct{} `asn1:"choice:131"`
	UM132ConfirmPosition  *struct{} `asn1:"choice:132"`
	UM133ConfirmAltitude  *struct{} `asn1:"choice:133"`
	UM134ConfirmSpeed     *struct{} `asn1:"choice:134"`
	UM135ConfirmAssignAlt *struct{} `asn1:"choice:135"`
	UM136ConfirmAssignSpd *struct{} `asn1:"choice:136"`
	UM137ConfirmAssignRte *struct{} `asn1:"choice:137"`
	UM138ConfirmTimeWpt   *struct{} `asn1:"choice:138"`
	UM139ConfirmRptWpt    *struct{} `asn1:"choice:139"`
	UM140ConfirmNextWpt   *struct{} `asn1:"choice:140"`
	UM141ConfirmNextETA   *struct{} `asn1:"choice:141"`
	UM142ConfirmEnsuingWpt *struct{} `asn1:"choice:142"`
	UM143ConfirmRequest   *struct{} `asn1:"choice:143"`
	UM144ConfirmSquawk    *struct{} `asn1:"choice:144"`
	UM145ConfirmHeading   *struct{} `asn1:"choice:145"`
	UM146ConfirmTrack     *struct{} `asn1:"choice:146"`
	UM147RequestPosRpt    *struct{} `asn1:"choice:147"`

	// uM148-uM152: When can you accept.
	UM148Altitude               *UPERAltitude               `asn1:"choice:148"`
	UM149AltitudePosition       *UPERAltitudePosition       `asn1:"choice:149"`
	UM150AltitudeTime           *UPERAltitudeTime           `asn1:"choice:150"`
	UM151Speed                  *UPERSpeed                  `asn1:"choice:151"`
	UM152DistanceOffsetDirection *UPERDistanceOffsetDirection `asn1:"choice:152"`

	// uM153: ALTIMETER [altimeter].
	UM153Altimeter *UPERAltimeter `asn1:"choice:153"`

	// uM154-uM156: Radar.
	UM154RadarTerminated *struct{}     `asn1:"choice:154"`
	UM155Position        *UPERPosition `asn1:"choice:155"`
	UM156RadarLost       *struct{}     `asn1:"choice:156"`

	// uM157: CHECK STUCK MICROPHONE.
	UM157Frequency *UPERFrequency `asn1:"choice:157"`

	// uM158: ATIS.
	UM158ATISCode *string `asn1:"choice:158,ia5string,size:1..1"`

	// uM159: ERROR.
	UM159ErrorInfo *UPERErrorInformation `asn1:"choice:159"`

	// uM160: NEXT DATA AUTHORITY.
	UM160Facility *string `asn1:"choice:160,ia5string,size:4..4"`

	// uM161-uM168: Service messages (NULL).
	UM161EndService       *struct{} `asn1:"choice:161"`
	UM162ServiceUnavail   *struct{} `asn1:"choice:162"`
	UM163FacilityTP4      *UPERFacilityTP4 `asn1:"choice:163"`
	UM164WhenReady        *struct{} `asn1:"choice:164"`
	UM165Then             *struct{} `asn1:"choice:165"`
	UM166DueToTraffic     *struct{} `asn1:"choice:166"`
	UM167DueToAirspace    *struct{} `asn1:"choice:167"`
	UM168Disregard        *struct{} `asn1:"choice:168"`

	// uM169-uM170: Free text.
	UM169FreeText        *string `asn1:"choice:169,ia5string,size:1..256"`
	UM170FreeTextDistress *string `asn1:"choice:170,ia5string,size:1..256"`

	// uM171-uM174: Vertical rate.
	UM171VerticalRate *UPERVerticalRate `asn1:"choice:171"`
	UM172VerticalRate *UPERVerticalRate `asn1:"choice:172"`
	UM173VerticalRate *UPERVerticalRate `asn1:"choice:173"`
	UM174VerticalRate *UPERVerticalRate `asn1:"choice:174"`

	// uM175: REPORT REACHING [altitude].
	UM175Altitude *UPERAltitude `asn1:"choice:175"`

	// uM176-uM179: Misc (NULL).
	UM176MaintainOwnSep  *struct{} `asn1:"choice:176"`
	UM177PilotsDiscretion *struct{} `asn1:"choice:177"`
	UM178Deleted         *struct{} `asn1:"choice:178"`
	UM179SquawkIdent     *struct{} `asn1:"choice:179"`

	// uM180: REPORT REACHING BLOCK.
	UM180AltitudeAltitude *UPERAltitudeAltitude `asn1:"choice:180"`

	// uM181: REPORT DISTANCE.
	UM181ToFromPosition *UPERToFromPosition `asn1:"choice:181"`

	// uM182: CONFIRM ATIS CODE.
	UM182ConfirmATIS *struct{} `asn1:"choice:182"`
}

// =============================================================================
// Supporting Types
// =============================================================================

// UPERAltitude is a CHOICE of 8 altitude representations.
type UPERAltitude struct {
	AltitudeQNH               *int `asn1:"choice:0,size:0..2500"`   // units=10ft
	AltitudeQNHMeters         *int `asn1:"choice:1,size:0..16000"`
	AltitudeQFE               *int `asn1:"choice:2,size:0..2100"`   // units=10ft
	AltitudeQFEMeters         *int `asn1:"choice:3,size:0..7000"`
	AltitudeGNSSFeet          *int `asn1:"choice:4,size:0..150000"`
	AltitudeGNSSMeters        *int `asn1:"choice:5,size:0..50000"`
	AltitudeFlightLevel       *int `asn1:"choice:6,size:30..600"`
	AltitudeFlightLevelMetric *int `asn1:"choice:7,size:100..2000"`
}

// UPERAltitudeAltitude is two altitudes.
type UPERAltitudeAltitude struct {
	Altitude1 UPERAltitude
	Altitude2 UPERAltitude
}

// UPERSpeed is a CHOICE of 8 speed representations.
type UPERSpeed struct {
	SpeedIndicated       *int `asn1:"choice:0,size:7..38"`   // units=10kt
	SpeedIndicatedMetric *int `asn1:"choice:1,size:10..137"` // units=10km/h
	SpeedTrue            *int `asn1:"choice:2,size:7..70"`   // units=10kt
	SpeedTrueMetric      *int `asn1:"choice:3,size:10..137"`
	SpeedGround          *int `asn1:"choice:4,size:7..70"`
	SpeedGroundMetric    *int `asn1:"choice:5,size:10..265"`
	SpeedMach            *int `asn1:"choice:6,size:61..92"`  // M.61-M.92
	SpeedMachLarge       *int `asn1:"choice:7,size:93..604"` // M.093-M6.04
}

// UPERSpeedSpeed is two speeds.
type UPERSpeedSpeed struct {
	Speed1 UPERSpeed
	Speed2 UPERSpeed
}

// UPERDegrees is a CHOICE of magnetic or true degrees.
type UPERDegrees struct {
	DegreesMagnetic *int `asn1:"choice:0,size:1..360"`
	DegreesTrue     *int `asn1:"choice:1,size:1..360"`
}

// UPERDirection is an enumerated direction (0-10).
type UPERDirection struct {
	Value int `asn1:"size:0..10"`
}

// UPERFrequency is a CHOICE of 4 frequency types.
type UPERFrequency struct {
	FrequencyHF      *int    `asn1:"choice:0,size:2850..28000"`  // kHz
	FrequencyVHF     *int    `asn1:"choice:1,size:117000..138000"` // kHz
	FrequencyUHF     *int    `asn1:"choice:2,size:225000..399975"` // kHz
	FrequencySatChan *string `asn1:"choice:3,ia5string,size:1..12"`
}

// UPERDistance is a CHOICE of nm or km.
type UPERDistance struct {
	DistanceNm *int `asn1:"choice:0,size:0..9999"` // units=0.1nm
	DistanceKm *int `asn1:"choice:1,size:1..1024"`
}

// UPERDistanceOffset is a CHOICE of offset distance.
type UPERDistanceOffset struct {
	DistanceOffsetNm *int `asn1:"choice:0,size:1..128"`
	DistanceOffsetKm *int `asn1:"choice:1,size:1..256"`
}

// UPERDistanceOffsetDirection is offset + direction.
type UPERDistanceOffsetDirection struct {
	DistanceOffset UPERDistanceOffset
	Direction      UPERDirection
}

// UPERPosition is a CHOICE of 5 position representations.
type UPERPosition struct {
	FixName              *string `asn1:"choice:0,ia5string,size:1..5"`
	Navaid               *string `asn1:"choice:1,ia5string,size:1..4"`
	Airport              *string `asn1:"choice:2,ia5string,size:4..4"`
	LatitudeLongitude    *UPERLatitudeLongitude `asn1:"choice:3"`
	PlaceBearingDistance *UPERPlaceBearingDistance `asn1:"choice:4"`
}

// UPERLatitude is latitude with degrees, optional tenths of minutes, and direction.
type UPERLatitude struct {
	Degrees        int  `asn1:"size:0..90"`
	MinutesTenths  *int `asn1:"optional,size:0..599"` // units=0.1min
	Direction      int  `asn1:"size:0..1"`            // 0=north, 1=south
}

// UPERLongitude is longitude with degrees, optional tenths of minutes, and direction.
type UPERLongitude struct {
	Degrees        int  `asn1:"size:0..180"`
	MinutesTenths  *int `asn1:"optional,size:0..599"`
	Direction      int  `asn1:"size:0..1"` // 0=east, 1=west
}

// UPERLatitudeLongitude is a lat/lon pair.
type UPERLatitudeLongitude struct {
	Latitude  UPERLatitude
	Longitude UPERLongitude
}

// UPERPlaceBearingDistance is a fix with optional lat/lon, bearing and distance.
type UPERPlaceBearingDistance struct {
	FixName           string `asn1:"ia5string,size:1..5"`
	LatitudeLongitude *UPERLatitudeLongitude `asn1:"optional"`
	Degrees           UPERDegrees
	Distance          UPERDistance
}

// UPERPositionAltitude is position + altitude.
type UPERPositionAltitude struct {
	Position UPERPosition
	Altitude UPERAltitude
}

// UPERTimeAltitude is time + altitude.
type UPERTimeAltitude struct {
	Time     UPERTime
	Altitude UPERAltitude
}

// UPERPositionDistanceOffsetDirection is position + offset + direction.
type UPERPositionDistanceOffsetDirection struct {
	Position       UPERPosition
	DistanceOffset UPERDistanceOffset
	Direction      UPERDirection
}

// UPERTimeDistanceOffsetDirection is time + offset + direction.
type UPERTimeDistanceOffsetDirection struct {
	Time           UPERTime
	DistanceOffset UPERDistanceOffset
	Direction      UPERDirection
}

// UPERTimeDistanceToFromPosition is time + distance + to/from + position.
type UPERTimeDistanceToFromPosition struct {
	Time     UPERTime
	Distance UPERDistance
	ToFrom   int `asn1:"size:0..1"` // 0=to, 1=from
	Position UPERPosition
}

// UPERBeaconCode is a 4-digit octal squawk code.
type UPERBeaconCode struct {
	Digit1 int `asn1:"size:0..7"`
	Digit2 int `asn1:"size:0..7"`
	Digit3 int `asn1:"size:0..7"`
	Digit4 int `asn1:"size:0..7"`
}

// UPERErrorInformation is error code + optional supplementary.
type UPERErrorInformation struct {
	ErrorCode int `asn1:"size:0..6"`
	// 0=unrecognizedMsgRef, 1=logonDataNotAccepted, 2=insufficientResources,
	// 3=serviceUnavailable, 4=duplicateMsgRef, 5=noOperationalPDC, 6=unexpectedRequestRef
}

// UPERRemainingFuelSouls is fuel remaining + persons on board.
type UPERRemainingFuelSouls struct {
	RemainingFuel UPERRemainingFuel
	RemainingSouls int `asn1:"size:0..1023"`
}

// UPERRemainingFuel is hours + minutes of fuel remaining.
type UPERRemainingFuel struct {
	Hours   int `asn1:"size:0..99"`
	Minutes int `asn1:"size:0..59"`
}

// =============================================================================
// Route/Procedure Types
// =============================================================================

// UPERProcedureName is procedure type + name + optional transition.
type UPERProcedureName struct {
	ProcedureType int    `asn1:"size:0..2"` // 0=arrival, 1=approach, 2=departure
	Procedure     UPERProcedure
}

// UPERProcedure is procedure identifier + optional transition.
type UPERProcedure struct {
	Name       string  `asn1:"ia5string,size:1..6"`
	Transition *string `asn1:"optional,ia5string,size:1..5"`
}

// UPERRouteClearance has many optional fields for route information.
type UPERRouteClearance struct {
	AirportDeparture    *string `asn1:"optional,ia5string,size:4..4"`
	AirportDestination  *string `asn1:"optional,ia5string,size:4..4"`
	RunwayDeparture     *UPERRunway `asn1:"optional"`
	ProcedureDeparture  *UPERProcedureName `asn1:"optional"`
	RunwayArrival       *UPERRunway `asn1:"optional"`
	ProcedureApproach   *UPERProcedureName `asn1:"optional"`
	ProcedureArrival    *UPERProcedureName `asn1:"optional"`
	AirwayIntercept     *string `asn1:"optional,ia5string,size:2..7"`
	RouteInformation    []UPERRouteInformationElement `asn1:"optional,size:1..128"`
	RouteInfoAdditional *string `asn1:"optional,ia5string,size:1..256"`
}

// UPERRunway is runway direction + configuration.
type UPERRunway struct {
	Direction     int `asn1:"size:1..36"`
	Configuration int `asn1:"size:0..3"` // 0=left, 1=right, 2=centre, 3=none
}

// UPERRouteInformationElement is a CHOICE of route info types.
type UPERRouteInformationElement struct {
	PublicationIdentifier *string `asn1:"choice:0,ia5string,size:1..6"`
	LatitudeLongitude     *UPERLatitudeLongitude `asn1:"choice:1"`
	PlaceBearingPlaceBearing *UPERPlaceBearingPlaceBearing `asn1:"choice:2"`
	PlaceBearingDistance  *UPERPlaceBearingDistance `asn1:"choice:3"`
	AirwayIdentifier      *string `asn1:"choice:4,ia5string,size:1..5"`
	TrackDetail           *UPERTrackDetail `asn1:"choice:5"`
	Airport               *string `asn1:"choice:6,ia5string,size:4..4"`
	RNPRequirements       *int `asn1:"choice:7,size:1..10"` // simplified
	Fix                   *string `asn1:"choice:8,ia5string,size:1..5"`
	Navaid                *string `asn1:"choice:9,ia5string,size:1..4"`
	HoldAtWaypoint        *UPERHoldAtWaypoint `asn1:"choice:10"`
}

// UPERPlaceBearingPlaceBearing is two place-bearing pairs.
type UPERPlaceBearingPlaceBearing struct {
	FixName1 string `asn1:"ia5string,size:1..5"`
	Degrees1 UPERDegrees
	FixName2 string `asn1:"ia5string,size:1..5"`
	Degrees2 UPERDegrees
}

// UPERTrackDetail is track name + optional lat/lon.
type UPERTrackDetail struct {
	TrackName         string `asn1:"ia5string,size:3..6"`
	LatitudeLongitude *UPERLatitudeLongitude `asn1:"optional"`
}

// UPERHoldAtWaypoint is hold position + optional details.
type UPERHoldAtWaypoint struct {
	Position UPERPosition
	// Simplified - full version has more optional fields
}

// UPERPositionRouteClearance is position + route clearance.
type UPERPositionRouteClearance struct {
	Position       UPERPosition
	RouteClearance UPERRouteClearance
}

// =============================================================================
// Position Report (dM48)
// =============================================================================

// UPERPositionReport is the full position report structure.
// Has 3 mandatory fields (Position, Time, Altitude) and 19 optional fields.
// Field order must match the FANSPositionReport ASN.1 definition.
type UPERPositionReport struct {
	// Mandatory fields.
	PositionCurrent       UPERPosition
	TimeAtPositionCurrent UPERTime     // Hours + Minutes only (no seconds).
	Altitude              UPERAltitude

	// Optional fields (19 total).
	FixNext                  *UPERPosition `asn1:"optional"`
	TimeEtaAtFixNext         *UPERTime `asn1:"optional"`
	FixNextPlusOne           *UPERPosition `asn1:"optional"`
	TimeEtaDestination       *UPERTime `asn1:"optional"`
	RemainingFuel            *UPERRemainingFuel `asn1:"optional"`
	Temperature              *UPERTemperature `asn1:"optional"`
	Winds                    *UPERWinds `asn1:"optional"`
	Turbulence               *int `asn1:"optional,size:0..3"` // 0=nil, 1=light, 2=mod, 3=severe
	Icing                    *int `asn1:"optional,size:0..3"`
	Speed                    *UPERSpeed `asn1:"optional"`
	SpeedGround              *UPERSpeedGround `asn1:"optional"`
	VerticalChange           *int `asn1:"optional,size:0..3"` // 0=level, 1=climb, 2=descent, 3=unknown
	TrackAngle               *UPERDegrees `asn1:"optional"`
	TrueHeading              *UPERDegrees `asn1:"optional"`
	Distance                 *UPERDistance `asn1:"optional"`
	SupplementaryInformation *string `asn1:"optional,ia5string,size:1..256"`
	ReportedWaypointPosition *UPERPosition `asn1:"optional"`
	ReportedWaypointTime     *UPERTime `asn1:"optional"`
	ReportedWaypointAltitude *UPERAltitude `asn1:"optional"`
}

// UPERTemperature is a CHOICE of temperature types.
type UPERTemperature struct {
	TemperatureC      *int `asn1:"choice:0,size:-100..100"`
	TemperatureFahren *int `asn1:"choice:1,size:-148..212"`
}

// UPERWinds is a SEQUENCE of wind direction and speed.
type UPERWinds struct {
	Direction UPERWindDirection
	Speed     UPERWindSpeed
}

// UPERWindDirection is a CHOICE of wind direction types.
type UPERWindDirection struct {
	DegreesMagnetic *int `asn1:"choice:0,size:1..360"`
	DegreesTrue     *int `asn1:"choice:1,size:1..360"`
}

// UPERSpeedGround is a CHOICE of ground speed types.
type UPERSpeedGround struct {
	SpeedGroundKt  *int `asn1:"choice:0,size:0..2000"`
	SpeedGroundKmh *int `asn1:"choice:1,size:0..3700"`
}

// UPERWindSpeed is a CHOICE of wind speed in kt or km/h.
type UPERWindSpeed struct {
	SpeedKt  *int `asn1:"choice:0,size:0..255"`
	SpeedKmh *int `asn1:"choice:1,size:0..511"`
}

// =============================================================================
// Additional Uplink Supporting Types
// =============================================================================

// UPERAltitudeTime is altitude + time.
type UPERAltitudeTime struct {
	Altitude UPERAltitude
	Time     UPERTime
}

// UPERAltitudePosition is altitude + position.
type UPERAltitudePosition struct {
	Altitude UPERAltitude
	Position UPERPosition
}

// UPERPositionAltitudeAltitude is position + two altitudes.
type UPERPositionAltitudeAltitude struct {
	Position  UPERPosition
	Altitude1 UPERAltitude
	Altitude2 UPERAltitude
}

// UPERPositionTime is position + time.
type UPERPositionTime struct {
	Position UPERPosition
	Time     UPERTime
}

// UPERPositionTimeTime is position + two times.
type UPERPositionTimeTime struct {
	Position UPERPosition
	Time1    UPERTime
	Time2    UPERTime
}

// UPERPositionSpeed is position + speed.
type UPERPositionSpeed struct {
	Position UPERPosition
	Speed    UPERSpeed
}

// UPERPositionTimeAltitude is position + time + altitude.
type UPERPositionTimeAltitude struct {
	Position UPERPosition
	Time     UPERTime
	Altitude UPERAltitude
}

// UPERPositionAltitudeSpeed is position + altitude + speed.
type UPERPositionAltitudeSpeed struct {
	Position UPERPosition
	Altitude UPERAltitude
	Speed    UPERSpeed
}

// UPERTimePositionAltitude is time + position + altitude.
type UPERTimePositionAltitude struct {
	Time     UPERTime
	Position UPERPosition
	Altitude UPERAltitude
}

// UPERTimePositionAltitudeSpeed is time + position + altitude + speed.
type UPERTimePositionAltitudeSpeed struct {
	Time     UPERTime
	Position UPERPosition
	Altitude UPERAltitude
	Speed    UPERSpeed
}

// UPERTimePosition is time + position.
type UPERTimePosition struct {
	Time     UPERTime
	Position UPERPosition
}

// UPERPositionPosition is two positions.
type UPERPositionPosition struct {
	Position1 UPERPosition
	Position2 UPERPosition
}

// UPERAltitudeSpeed is altitude + speed.
type UPERAltitudeSpeed struct {
	Altitude UPERAltitude
	Speed    UPERSpeed
}

// UPERAltitudeSpeedSpeed is altitude + two speeds.
type UPERAltitudeSpeedSpeed struct {
	Altitude UPERAltitude
	Speed1   UPERSpeed
	Speed2   UPERSpeed
}

// UPERTimeSpeed is time + speed.
type UPERTimeSpeed struct {
	Time  UPERTime
	Speed UPERSpeed
}

// UPERTimeSpeedSpeed is time + two speeds.
type UPERTimeSpeedSpeed struct {
	Time   UPERTime
	Speed1 UPERSpeed
	Speed2 UPERSpeed
}

// UPERPositionSpeedSpeed is position + two speeds.
type UPERPositionSpeedSpeed struct {
	Position UPERPosition
	Speed1   UPERSpeed
	Speed2   UPERSpeed
}

// UPERDirectionDegrees is direction + degrees (for turn instructions).
type UPERDirectionDegrees struct {
	Direction int `asn1:"size:0..1"` // 0=left, 1=right
	Degrees   UPERDegrees
}

// UPERPositionDegrees is position + degrees.
type UPERPositionDegrees struct {
	Position UPERPosition
	Degrees  UPERDegrees
}

// UPERHoldClearance is the hold clearance structure.
type UPERHoldClearance struct {
	Position         UPERPosition
	Altitude         *UPERAltitude `asn1:"optional"`
	Speed            *UPERSpeed `asn1:"optional"`
	ATCDirection     *int `asn1:"optional,size:0..1"` // 0=left, 1=right
	LegType          *UPERHoldLegType `asn1:"optional"`
	DistanceTime     *UPERDistanceTime `asn1:"optional"`
	EFCTime          *UPERTime `asn1:"optional"`
}

// UPERHoldLegType is a CHOICE of distance or time based leg.
type UPERHoldLegType struct {
	LegDistance *int `asn1:"choice:0,size:1..128"` // nm
	LegTime     *int `asn1:"choice:1,size:1..60"`  // minutes
}

// UPERDistanceTime is distance + time.
type UPERDistanceTime struct {
	Distance UPERDistance
	Time     UPERTime
}

// UPERPredepartureClearance is the PDC (pre-departure clearance) structure.
type UPERPredepartureClearance struct {
	AircraftFlightID   string `asn1:"ia5string,size:2..8"`
	AirportDeparture   string `asn1:"ia5string,size:4..4"`
	AirportDestination *string `asn1:"optional,ia5string,size:4..4"`
	ClearedFlightLevel *UPERAltitude `asn1:"optional"`
	RouteClearance     *UPERRouteClearance `asn1:"optional"`
	DepartureTime      *UPERTime `asn1:"optional"`
	Squawk             *UPERBeaconCode `asn1:"optional"`
	Frequency          *UPERFrequency `asn1:"optional"`
}

// UPERICAOUnitNameFrequency is unit name + frequency.
type UPERICAOUnitNameFrequency struct {
	ICAOUnitName     UPERICAOUnitName
	Frequency        UPERFrequency
}

// UPERICAOUnitName is facility designation + name.
type UPERICAOUnitName struct {
	FacilityDesignation string `asn1:"ia5string,size:4..8"`
	FacilityName        *string `asn1:"optional,ia5string,size:1..24"`
	FacilityFunction    int `asn1:"size:0..15"` // enumerated
}

// UPERPositionICAOUnitNameFrequency is position + unit + frequency.
type UPERPositionICAOUnitNameFrequency struct {
	Position         UPERPosition
	ICAOUnitName     UPERICAOUnitName
	Frequency        UPERFrequency
}

// UPERTimeICAOUnitNameFrequency is time + unit + frequency.
type UPERTimeICAOUnitNameFrequency struct {
	Time             UPERTime
	ICAOUnitName     UPERICAOUnitName
	Frequency        UPERFrequency
}

// UPERAltimeter is a CHOICE of altimeter settings.
type UPERAltimeter struct {
	AltimeterInHg     *int `asn1:"choice:0,size:2200..3200"` // units=0.01 inHg (22.00-32.00)
	AltimeterHectopascals *int `asn1:"choice:1,size:750..1100"` // hPa
}

// UPERVerticalRate is a CHOICE of vertical rate units.
type UPERVerticalRate struct {
	VerticalRateFt *int `asn1:"choice:0,size:100..20000"` // ft/min, units=100
	VerticalRateM  *int `asn1:"choice:1,size:30..6000"`   // m/min, units=10
}

// UPERToFromPosition is to/from indicator + position.
type UPERToFromPosition struct {
	ToFrom   int `asn1:"size:0..1"` // 0=to, 1=from
	Position UPERPosition
}

// UPERFacilityTP4 is the facility identification for TP4 handoff.
type UPERFacilityTP4 struct {
	FacilityDesignation string `asn1:"ia5string,size:4..8"`
	Address             *UPERFacilityTP4Address `asn1:"optional"`
}

// UPERFacilityTP4Address is a TP4 address structure.
type UPERFacilityTP4Address struct {
	NSAP *string `asn1:"optional,ia5string,size:1..40"`
}

// UPERPositionProcedureName is position + procedure name.
type UPERPositionProcedureName struct {
	Position      UPERPosition
	ProcedureName UPERProcedureName
}
