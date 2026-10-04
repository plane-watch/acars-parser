package cpdlc

// This file implements the FANS-1/A CPDLC ASN.1 module
// (FANSACTwoWayDataLinkCommunications, the module dumpvdl2 publishes as
// asn1/fans-cpdlc.asn1) as Go types for the github.com/shaneshort/go-asn
// unaligned PER (UPER) decoder. Each type's comment names the ASN.1 type it
// implements; field order, OPTIONAL fields, CHOICE alternative order,
// enumeration sizes, INTEGER ranges and string sizes follow the module. The
// PER constraints were checked against the asn1c-generated C code in
// libacars (libacars/asn1/FANS*.c).
// The module is mapped as follows:
//   - SEQUENCE is a struct. OPTIONAL components are pointers tagged
//     `optional`; UPER encodes a presence bit for each at the start of the
//     SEQUENCE, in field order.
//   - CHOICE is a struct of pointers tagged `choice:N`; UPER encodes the
//     alternative's index in the fewest bits that hold the alternative count.
//   - NULL is *struct{}, which encodes no bits.
//   - INTEGER (a..b) is an int tagged `size:a..b`, encoded as the offset
//     from a. Like asn1c (and so libacars), the decoder does not check that
//     a value lies within the range.
//   - ENUMERATED is a named int type tagged `size:0..N-1`, encoded as the
//     enumeration index. The decoder rejects an index that names no
//     enumeration (see checkEnumerations), as asn1c does.
//   - IA5String (SIZE (a..b)) is a string tagged `ia5string,size:a..b`,
//     seven bits per character after a length unless the size is fixed.
//   - SEQUENCE SIZE (a..b) OF T is a slice tagged `size:a..b` (a pointer to
//     a slice when OPTIONAL, as go-asn requires of OPTIONAL fields; the
//     encoder treats a nil slice as present). A SEQUENCE OF
//     with a fixed size encodes no length, only the elements in turn, so it
//     is a struct with one field per element (for example, FANSAltitudeAltitude
//     is two FANSAltitude fields).
//   - NumericString (SIZE (12)) is a slice of twelve UPERNumericChar.
//   - A type that only renames another (FANSFixNext ::= FANSPosition) is
//     the Go type of the type it renames.
// The context tags ([0], [1], ...) in the module are not encoded in PER.
// Struct fields carry a `data` tag, which is not used by the UPER decoder.
// It names the key under which convertValue (decoder_uper.go) stores the
// field's value when it converts a SEQUENCE of several message element
// parameters into a map. A field tagged `data:"-"` is a fixed-size SEQUENCE
// OF embedded in its parent, whose own fields carry the keys.

// =============================================================================
// Messages and headers
// =============================================================================

// UPERUplinkMessage implements FANSATCUplinkMessage.
type UPERUplinkMessage struct {
	Header   UPERMessageHeader
	Element  UPERUplinkElement
	Elements *[]UPERUplinkElement `asn1:"optional,size:1..4"` // FANSATCUplinkMsgElementIdSequence.
}

// UPERDownlinkMessage implements FANSATCDownlinkMessage.
type UPERDownlinkMessage struct {
	Header   UPERMessageHeader
	Element  UPERDownlinkElement
	Elements *[]UPERDownlinkElement `asn1:"optional,size:1..4"` // FANSATCDownlinkMsgElementIdSequence.
}

// UPERMessageHeader implements FANSATCMessageHeader.
type UPERMessageHeader struct {
	MsgID     int            `asn1:"size:0..63"`          // FANSMsgIdentificationNumber.
	MsgRef    *int           `asn1:"optional,size:0..63"` // FANSMsgReferenceNumber.
	Timestamp *UPERTimestamp `asn1:"optional"`
}

// UPERTimestamp implements FANSTimestamp.
type UPERTimestamp struct {
	Hours   int `asn1:"size:0..23"` // FANSTimehours.
	Minutes int `asn1:"size:0..59"` // FANSTimeminutes.
	Seconds int `asn1:"size:0..59"` // FANSTimeSeconds.
}

// =============================================================================
// Uplink message elements
// =============================================================================

// UPERUplinkElement implements FANSATCUplinkMsgElementId, the CHOICE of the
// 183 uplink message elements uM0 to uM182 (an eight-bit index). Each
// field's comment gives the element's template from the module.
type UPERUplinkElement struct {
	UM0NULL                              *struct{}                            `asn1:"choice:0"`                         // UNABLE
	UM1NULL                              *struct{}                            `asn1:"choice:1"`                         // STANDBY
	UM2NULL                              *struct{}                            `asn1:"choice:2"`                         // REQUEST DEFERRED
	UM3NULL                              *struct{}                            `asn1:"choice:3"`                         // ROGER
	UM4NULL                              *struct{}                            `asn1:"choice:4"`                         // AFFIRM
	UM5NULL                              *struct{}                            `asn1:"choice:5"`                         // NEGATIVE
	UM6Altitude                          *UPERAltitude                        `asn1:"choice:6"`                         // EXPECT [altitude]
	UM7Time                              *UPERTime                            `asn1:"choice:7"`                         // EXPECT CLIMB AT [time]
	UM8Position                          *UPERPosition                        `asn1:"choice:8"`                         // EXPECT CLIMB AT [position]
	UM9Time                              *UPERTime                            `asn1:"choice:9"`                         // EXPECT DESCENT AT [time]
	UM10Position                         *UPERPosition                        `asn1:"choice:10"`                        // EXPECT DESCENT AT [position]
	UM11Time                             *UPERTime                            `asn1:"choice:11"`                        // EXPECT CRUISE CLIMB AT [time]
	UM12Position                         *UPERPosition                        `asn1:"choice:12"`                        // EXPECT CRUISE CLIMB AT [position]
	UM13TimeAltitude                     *UPERTimeAltitude                    `asn1:"choice:13"`                        // AT [time] EXPECT CLIMB TO [altitude]
	UM14PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:14"`                        // AT [position] EXPECT CLIMB TO [altitude]
	UM15TimeAltitude                     *UPERTimeAltitude                    `asn1:"choice:15"`                        // AT [time] EXPECT DESCENT TO [altitude]
	UM16PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:16"`                        // AT [position] EXPECT DESCENT TO [altitude]
	UM17TimeAltitude                     *UPERTimeAltitude                    `asn1:"choice:17"`                        // AT [time] EXPECT CRUISE CLIMB TO [altitude]
	UM18PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:18"`                        // AT [position] EXPECT CRUISE CLIMB TO [altitude]
	UM19Altitude                         *UPERAltitude                        `asn1:"choice:19"`                        // MAINTAIN [altitude]
	UM20Altitude                         *UPERAltitude                        `asn1:"choice:20"`                        // CLIMB TO AND MAINTAIN [altitude]
	UM21TimeAltitude                     *UPERTimeAltitude                    `asn1:"choice:21"`                        // AT [time] CLIMB TO AND MAINTAIN [altitude]
	UM22PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:22"`                        // AT [position] CLIMB TO AND MAINTAIN [altitude]
	UM23Altitude                         *UPERAltitude                        `asn1:"choice:23"`                        // DESCEND TO AND MAINTAIN [altitude]
	UM24TimeAltitude                     *UPERTimeAltitude                    `asn1:"choice:24"`                        // AT [time] DESCEND TO AND MAINTAIN [altitude]
	UM25PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:25"`                        // AT [position] DESCEND TO AND MAINTAIN [altitude]
	UM26AltitudeTime                     *UPERAltitudeTime                    `asn1:"choice:26"`                        // CLIMB TO REACH [altitude] BY [time]
	UM27AltitudePosition                 *UPERAltitudePosition                `asn1:"choice:27"`                        // CLIMB TO REACH [altitude] BY [position]
	UM28AltitudeTime                     *UPERAltitudeTime                    `asn1:"choice:28"`                        // DESCEND TO REACH [altitude] BY [time]
	UM29AltitudePosition                 *UPERAltitudePosition                `asn1:"choice:29"`                        // DESCEND TO REACH [altitude] BY [position]
	UM30AltitudeAltitude                 *UPERAltitudeAltitude                `asn1:"choice:30"`                        // MAINTAIN BLOCK [altitude] TO [altitude]
	UM31AltitudeAltitude                 *UPERAltitudeAltitude                `asn1:"choice:31"`                        // CLIMB TO AND MAINTAIN BLOCK [altitude] TO [altitude]
	UM32AltitudeAltitude                 *UPERAltitudeAltitude                `asn1:"choice:32"`                        // DESCEND TO AND MAINTAIN BLOCK [altitude] TO [altitude]
	UM33Altitude                         *UPERAltitude                        `asn1:"choice:33"`                        // CRUISE [altitude]
	UM34Altitude                         *UPERAltitude                        `asn1:"choice:34"`                        // CRUISE CLIMB TO [altitude]
	UM35Altitude                         *UPERAltitude                        `asn1:"choice:35"`                        // CRUISE CLIMB ABOVE [altitude]
	UM36Altitude                         *UPERAltitude                        `asn1:"choice:36"`                        // EXPEDITE CLIMB TO [altitude]
	UM37Altitude                         *UPERAltitude                        `asn1:"choice:37"`                        // EXPEDITE DESCENT TO [altitude]
	UM38Altitude                         *UPERAltitude                        `asn1:"choice:38"`                        // IMMEDIATELY CLIMB TO [altitude]
	UM39Altitude                         *UPERAltitude                        `asn1:"choice:39"`                        // IMMEDIATELY DESCEND TO [altitude]
	UM40Altitude                         *UPERAltitude                        `asn1:"choice:40"`                        // IMMEDIATELY STOP CLIMB AT [altitude]
	UM41Altitude                         *UPERAltitude                        `asn1:"choice:41"`                        // IMMEDIATELY STOP DESCENT AT [altitude]
	UM42PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:42"`                        // EXPECT TO CROSS [position] AT [altitude]
	UM43PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:43"`                        // EXPECT TO CROSS [position] AT OR ABOVE [altitude]
	UM44PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:44"`                        // EXPECT TO CROSS [position] AT OR BELOW [altitude]
	UM45PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:45"`                        // EXPECT TO CROSS [position] AT AND MAINTAIN [altitude]
	UM46PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:46"`                        // CROSS [position] AT [altitude]
	UM47PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:47"`                        // CROSS [position] AT OR ABOVE [altitude]
	UM48PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:48"`                        // CROSS [position] AT OR BELOW [altitude]
	UM49PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:49"`                        // CROSS [position] AT AND MAINTAIN [altitude]
	UM50PositionAltitudeAltitude         *UPERPositionAltitudeAltitude        `asn1:"choice:50"`                        // CROSS [position] BETWEEN [altitude] AND [altitude]
	UM51PositionTime                     *UPERPositionTime                    `asn1:"choice:51"`                        // CROSS [position] AT [time]
	UM52PositionTime                     *UPERPositionTime                    `asn1:"choice:52"`                        // CROSS [position] AT OR BEFORE [time]
	UM53PositionTime                     *UPERPositionTime                    `asn1:"choice:53"`                        // CROSS [position] AT OR AFTER [time]
	UM54PositionTimeTime                 *UPERPositionTimeTime                `asn1:"choice:54"`                        // CROSS [position] BETWEEN [time] AND [time]
	UM55PositionSpeed                    *UPERPositionSpeed                   `asn1:"choice:55"`                        // CROSS [position] AT [speed]
	UM56PositionSpeed                    *UPERPositionSpeed                   `asn1:"choice:56"`                        // CROSS [position] AT OR LESS THAN [speed]
	UM57PositionSpeed                    *UPERPositionSpeed                   `asn1:"choice:57"`                        // CROSS [position] AT OR GREATER THAN [speed]
	UM58PositionTimeAltitude             *UPERPositionTimeAltitude            `asn1:"choice:58"`                        // CROSS [position] AT [time] AT [altitude]
	UM59PositionTimeAltitude             *UPERPositionTimeAltitude            `asn1:"choice:59"`                        // CROSS [position] AT OR BEFORE [time] AT [altitude]
	UM60PositionTimeAltitude             *UPERPositionTimeAltitude            `asn1:"choice:60"`                        // CROSS [position] AT OR AFTER [time] AT [altitude]
	UM61PositionAltitudeSpeed            *UPERPositionAltitudeSpeed           `asn1:"choice:61"`                        // CROSS [position] AT AND MAINTAIN [altitude] AT [speed]
	UM62TimePositionAltitude             *UPERTimePositionAltitude            `asn1:"choice:62"`                        // AT [time] CROSS [position] AT AND MAINTAIN [altitude]
	UM63TimePositionAltitudeSpeed        *UPERTimePositionAltitudeSpeed       `asn1:"choice:63"`                        // AT [time] CROSS [position] AT AND MAINTAIN [altitude] AT [speed]
	UM64DistanceOffsetDirection          *UPERDistanceOffsetDirection         `asn1:"choice:64"`                        // OFFSET [distanceoffset] [direction] OF ROUTE
	UM65PositionDistanceOffsetDirection  *UPERPositionDistanceOffsetDirection `asn1:"choice:65"`                        // AT [position] OFFSET [distanceoffset] [direction] OF ROUTE
	UM66TimeDistanceOffsetDirection      *UPERTimeDistanceOffsetDirection     `asn1:"choice:66"`                        // AT [time] OFFSET [distanceoffset] [direction] OF ROUTE
	UM67NULL                             *struct{}                            `asn1:"choice:67"`                        // PROCEED BACK ON ROUTE
	UM68Position                         *UPERPosition                        `asn1:"choice:68"`                        // REJOIN ROUTE BY [position]
	UM69Time                             *UPERTime                            `asn1:"choice:69"`                        // REJOIN ROUTE BY [time]
	UM70Position                         *UPERPosition                        `asn1:"choice:70"`                        // EXPECT BACK ON ROUTE BY [position]
	UM71Time                             *UPERTime                            `asn1:"choice:71"`                        // EXPECT BACK ON ROUTE BY [time]
	UM72NULL                             *struct{}                            `asn1:"choice:72"`                        // RESUME OWN NAVIGATION
	UM73PredepartureClearance            *UPERPredepartureClearance           `asn1:"choice:73"`                        // [predepartureclearance]
	UM74Position                         *UPERPosition                        `asn1:"choice:74"`                        // PROCEED DIRECT TO [position]
	UM75Position                         *UPERPosition                        `asn1:"choice:75"`                        // WHEN ABLE PROCEED DIRECT TO [position]
	UM76TimePosition                     *UPERTimePosition                    `asn1:"choice:76"`                        // AT [time] PROCEED DIRECT TO [position]
	UM77PositionPosition                 *UPERPositionPosition                `asn1:"choice:77"`                        // AT [position] PROCEED DIRECT TO [position]
	UM78AltitudePosition                 *UPERAltitudePosition                `asn1:"choice:78"`                        // AT [altitude] PROCEED DIRECT TO [position]
	UM79PositionRouteClearance           *UPERPositionRouteClearance          `asn1:"choice:79"`                        // CLEARED TO [position] VIA [routeclearance]
	UM80RouteClearance                   *UPERRouteClearance                  `asn1:"choice:80"`                        // CLEARED [routeclearance]
	UM81ProcedureName                    *UPERProcedureName                   `asn1:"choice:81"`                        // CLEARED [procedurename]
	UM82DistanceOffsetDirection          *UPERDistanceOffsetDirection         `asn1:"choice:82"`                        // CLEARED TO DEVIATE UP TO [distanceoffset] [direction] OF ROUTE
	UM83PositionRouteClearance           *UPERPositionRouteClearance          `asn1:"choice:83"`                        // AT [position] CLEARED [routeclearance]
	UM84PositionProcedureName            *UPERPositionProcedureName           `asn1:"choice:84"`                        // AT [position] CLEARED [procedurename]
	UM85RouteClearance                   *UPERRouteClearance                  `asn1:"choice:85"`                        // EXPECT [routeclearance]
	UM86PositionRouteClearance           *UPERPositionRouteClearance          `asn1:"choice:86"`                        // AT [position] EXPECT [routeclearance]
	UM87Position                         *UPERPosition                        `asn1:"choice:87"`                        // EXPECT DIRECT TO [position]
	UM88PositionPosition                 *UPERPositionPosition                `asn1:"choice:88"`                        // AT [position] EXPECT DIRECT TO [position]
	UM89TimePosition                     *UPERTimePosition                    `asn1:"choice:89"`                        // AT [time] EXPECT DIRECT TO [position]
	UM90AltitudePosition                 *UPERAltitudePosition                `asn1:"choice:90"`                        // AT [altitude] EXPECT DIRECT TO [position]
	UM91HoldClearance                    *UPERHoldClearance                   `asn1:"choice:91"`                        // HOLD AT [position] MAINTAIN [altitude] INBOUND TRACK [degrees] [direction] TURNS [legtype]
	UM92PositionAltitude                 *UPERPositionAltitude                `asn1:"choice:92"`                        // HOLD AT [position] AS PUBLISHED MAINTAIN [altitude]
	UM93Time                             *UPERTime                            `asn1:"choice:93"`                        // EXPECT FURTHER CLEARANCE AT [time]
	UM94DirectionDegrees                 *UPERDirectionDegrees                `asn1:"choice:94"`                        // TURN [direction] HEADING [degrees]
	UM95DirectionDegrees                 *UPERDirectionDegrees                `asn1:"choice:95"`                        // TURN [direction] GROUND TRACK [degrees]
	UM96NULL                             *struct{}                            `asn1:"choice:96"`                        // FLY PRESENT HEADING
	UM97PositionDegrees                  *UPERPositionDegrees                 `asn1:"choice:97"`                        // AT [position] FLY HEADING [degrees]
	UM98DirectionDegrees                 *UPERDirectionDegrees                `asn1:"choice:98"`                        // IMMEDIATELY TURN [direction] HEADING [degrees]
	UM99ProcedureName                    *UPERProcedureName                   `asn1:"choice:99"`                        // EXPECT [procedurename]
	UM100TimeSpeed                       *UPERTimeSpeed                       `asn1:"choice:100"`                       // AT [time] EXPECT [speed]
	UM101PositionSpeed                   *UPERPositionSpeed                   `asn1:"choice:101"`                       // AT [position] EXPECT [speed]
	UM102AltitudeSpeed                   *UPERAltitudeSpeed                   `asn1:"choice:102"`                       // AT [altitude] EXPECT [speed]
	UM103TimeSpeedSpeed                  *UPERTimeSpeedSpeed                  `asn1:"choice:103"`                       // AT [time] EXPECT [speed] TO [speed]
	UM104PositionSpeedSpeed              *UPERPositionSpeedSpeed              `asn1:"choice:104"`                       // AT [position] EXPECT [speed] TO [speed]
	UM105AltitudeSpeedSpeed              *UPERAltitudeSpeedSpeed              `asn1:"choice:105"`                       // AT [altitude] EXPECT [speed] TO [speed]
	UM106Speed                           *UPERSpeed                           `asn1:"choice:106"`                       // MAINTAIN [speed]
	UM107NULL                            *struct{}                            `asn1:"choice:107"`                       // MAINTAIN PRESENT SPEED
	UM108Speed                           *UPERSpeed                           `asn1:"choice:108"`                       // MAINTAIN [speed] OR GREATER
	UM109Speed                           *UPERSpeed                           `asn1:"choice:109"`                       // MAINTAIN [speed] OR LESS
	UM110SpeedSpeed                      *UPERSpeedSpeed                      `asn1:"choice:110"`                       // MAINTAIN [speed] TO [speed]
	UM111Speed                           *UPERSpeed                           `asn1:"choice:111"`                       // INCREASE SPEED TO [speed]
	UM112Speed                           *UPERSpeed                           `asn1:"choice:112"`                       // INCREASE SPEED TO [speed] OR GREATER
	UM113Speed                           *UPERSpeed                           `asn1:"choice:113"`                       // REDUCE SPEED TO [speed]
	UM114Speed                           *UPERSpeed                           `asn1:"choice:114"`                       // REDUCE SPEED TO [speed] OR LESS
	UM115Speed                           *UPERSpeed                           `asn1:"choice:115"`                       // DO NOT EXCEED [speed]
	UM116NULL                            *struct{}                            `asn1:"choice:116"`                       // RESUME NORMAL SPEED
	UM117ICAOUnitNameFrequency           *UPERICAOUnitNameFrequency           `asn1:"choice:117"`                       // CONTACT [icaounitname] [frequency]
	UM118PositionICAOUnitNameFrequency   *UPERPositionICAOUnitNameFrequency   `asn1:"choice:118"`                       // AT [position] CONTACT [icaounitname] [frequency]
	UM119TimeICAOUnitNameFrequency       *UPERTimeICAOUnitNameFrequency       `asn1:"choice:119"`                       // AT [time] CONTACT [icaounitname] [frequency]
	UM120ICAOUnitNameFrequency           *UPERICAOUnitNameFrequency           `asn1:"choice:120"`                       // MONITOR [icaounitname] [frequency]
	UM121PositionICAOUnitNameFrequency   *UPERPositionICAOUnitNameFrequency   `asn1:"choice:121"`                       // AT [position] MONITOR [icaounitname] [frequency]
	UM122TimeICAOUnitNameFrequency       *UPERTimeICAOUnitNameFrequency       `asn1:"choice:122"`                       // AT [time] MONITOR [icaounitname] [frequency]
	UM123BeaconCode                      *UPERBeaconCode                      `asn1:"choice:123"`                       // SQUAWK [beaconcode]
	UM124NULL                            *struct{}                            `asn1:"choice:124"`                       // STOP SQUAWK
	UM125NULL                            *struct{}                            `asn1:"choice:125"`                       // SQUAWK ALTITUDE
	UM126NULL                            *struct{}                            `asn1:"choice:126"`                       // STOP ALTITUDE SQUAWK
	UM127NULL                            *struct{}                            `asn1:"choice:127"`                       // REPORT BACK ON ROUTE
	UM128Altitude                        *UPERAltitude                        `asn1:"choice:128"`                       // REPORT LEAVING [altitude]
	UM129Altitude                        *UPERAltitude                        `asn1:"choice:129"`                       // REPORT LEVEL [altitude]
	UM130Position                        *UPERPosition                        `asn1:"choice:130"`                       // REPORT PASSING [position]
	UM131NULL                            *struct{}                            `asn1:"choice:131"`                       // REPORT REMAINING FUEL AND SOULS ON BOARD
	UM132NULL                            *struct{}                            `asn1:"choice:132"`                       // CONFIRM POSITION
	UM133NULL                            *struct{}                            `asn1:"choice:133"`                       // CONFIRM ALTITUDE
	UM134NULL                            *struct{}                            `asn1:"choice:134"`                       // CONFIRM SPEED
	UM135NULL                            *struct{}                            `asn1:"choice:135"`                       // CONFIRM ASSIGNED ALTITUDE
	UM136NULL                            *struct{}                            `asn1:"choice:136"`                       // CONFIRM ASSIGNED SPEED
	UM137NULL                            *struct{}                            `asn1:"choice:137"`                       // CONFIRM ASSIGNED ROUTE
	UM138NULL                            *struct{}                            `asn1:"choice:138"`                       // CONFIRM TIME OVER REPORTED WAYPOINT
	UM139NULL                            *struct{}                            `asn1:"choice:139"`                       // CONFIRM REPORTED WAYPOINT
	UM140NULL                            *struct{}                            `asn1:"choice:140"`                       // CONFIRM NEXT WAYPOINT
	UM141NULL                            *struct{}                            `asn1:"choice:141"`                       // CONFIRM NEXT WAYPOINT ETA
	UM142NULL                            *struct{}                            `asn1:"choice:142"`                       // CONFIRM ENSUING WAYPOINT
	UM143NULL                            *struct{}                            `asn1:"choice:143"`                       // CONFIRM REQUEST
	UM144NULL                            *struct{}                            `asn1:"choice:144"`                       // CONFIRM SQUAWK
	UM145NULL                            *struct{}                            `asn1:"choice:145"`                       // CONFIRM HEADING
	UM146NULL                            *struct{}                            `asn1:"choice:146"`                       // CONFIRM GROUND TRACK
	UM147NULL                            *struct{}                            `asn1:"choice:147"`                       // REQUEST POSITION REPORT
	UM148Altitude                        *UPERAltitude                        `asn1:"choice:148"`                       // WHEN CAN YOU ACCEPT [altitude]
	UM149AltitudePosition                *UPERAltitudePosition                `asn1:"choice:149"`                       // CAN YOU ACCEPT [altitude] AT [position]
	UM150AltitudeTime                    *UPERAltitudeTime                    `asn1:"choice:150"`                       // CAN YOU ACCEPT [altitude] AT [time]
	UM151Speed                           *UPERSpeed                           `asn1:"choice:151"`                       // WHEN CAN YOU ACCEPT [speed]
	UM152DistanceOffsetDirection         *UPERDistanceOffsetDirection         `asn1:"choice:152"`                       // WHEN CAN YOU ACCEPT [distanceoffset] [direction] OFFSET
	UM153Altimeter                       *UPERAltimeter                       `asn1:"choice:153"`                       // ALTIMETER [altimeter]
	UM154NULL                            *struct{}                            `asn1:"choice:154"`                       // RADAR SERVICES TERMINATED
	UM155Position                        *UPERPosition                        `asn1:"choice:155"`                       // RADAR CONTACT [position]
	UM156NULL                            *struct{}                            `asn1:"choice:156"`                       // RADAR CONTACT LOST
	UM157Frequency                       *UPERFrequency                       `asn1:"choice:157"`                       // CHECK STUCK MICROPHONE [frequency]
	UM158ATISCode                        *UPERATISCode                        `asn1:"choice:158,ia5string,size:1"`      // ATIS [atiscode]
	UM159ErrorInformation                *UPERErrorInformation                `asn1:"choice:159,size:0..16"`            // ERROR [errorinformation]
	UM160ICAOFacilityDesignation         *UPERICAOFacilityDesignation         `asn1:"choice:160,ia5string,size:4"`      // NEXT DATA AUTHORITY [icaofacilitydesignation]
	UM161NULL                            *struct{}                            `asn1:"choice:161"`                       // END SERVICE
	UM162NULL                            *struct{}                            `asn1:"choice:162"`                       // SERVICE UNAVAILABLE
	UM163ICAOFacilityDesignationTp4Table *UPERICAOFacilityDesignationTp4Table `asn1:"choice:163"`                       // [icaofacilitydesignation] [tp4table]
	UM164NULL                            *struct{}                            `asn1:"choice:164"`                       // WHEN READY
	UM165NULL                            *struct{}                            `asn1:"choice:165"`                       // THEN
	UM166NULL                            *struct{}                            `asn1:"choice:166"`                       // DUE TO TRAFFIC
	UM167NULL                            *struct{}                            `asn1:"choice:167"`                       // DUE TO AIRSPACE RESTRICTION
	UM168NULL                            *struct{}                            `asn1:"choice:168"`                       // DISREGARD
	UM169FreeText                        *UPERFreeText                        `asn1:"choice:169,ia5string,size:1..256"` // [freetext]
	UM170FreeText                        *UPERFreeText                        `asn1:"choice:170,ia5string,size:1..256"` // [freetext] (distress urgency)
	UM171VerticalRate                    *UPERVerticalRate                    `asn1:"choice:171"`                       // CLIMB AT [verticalrate] MINIMUM
	UM172VerticalRate                    *UPERVerticalRate                    `asn1:"choice:172"`                       // CLIMB AT [verticalrate] MAXIMUM
	UM173VerticalRate                    *UPERVerticalRate                    `asn1:"choice:173"`                       // DESCEND AT [verticalrate] MINIMUM
	UM174VerticalRate                    *UPERVerticalRate                    `asn1:"choice:174"`                       // DESCEND AT [verticalrate] MAXIMUM
	UM175Altitude                        *UPERAltitude                        `asn1:"choice:175"`                       // REPORT REACHING [altitude]
	UM176NULL                            *struct{}                            `asn1:"choice:176"`                       // MAINTAIN OWN SEPARATION AND VMC
	UM177NULL                            *struct{}                            `asn1:"choice:177"`                       // AT PILOTS DISCRETION
	UM178NULL                            *struct{}                            `asn1:"choice:178"`                       // [trackdetailmsg-deleted]
	UM179NULL                            *struct{}                            `asn1:"choice:179"`                       // SQUAWK IDENT
	UM180AltitudeAltitude                *UPERAltitudeAltitude                `asn1:"choice:180"`                       // REPORT REACHING BLOCK [altitude] TO [altitude]
	UM181ToFromPosition                  *UPERToFromPosition                  `asn1:"choice:181"`                       // REPORT DISTANCE [tofrom] [position]
	UM182NULL                            *struct{}                            `asn1:"choice:182"`                       // CONFIRM ATIS CODE
}

// =============================================================================
// Downlink message elements
// =============================================================================

// UPERDownlinkElement implements FANSATCDownlinkMsgElementId, the CHOICE of
// the 129 downlink message elements dM0 to dM128 (an eight-bit index).
// dM81 to dM128 are NULL elements the module reserves so that the index is
// eight bits wide; they have no template. Each other field's comment gives
// the element's template from the module.
type UPERDownlinkElement struct {
	DM0NULL                             *struct{}                            `asn1:"choice:0"`                        // WILCO
	DM1NULL                             *struct{}                            `asn1:"choice:1"`                        // UNABLE
	DM2NULL                             *struct{}                            `asn1:"choice:2"`                        // STANDBY
	DM3NULL                             *struct{}                            `asn1:"choice:3"`                        // ROGER
	DM4NULL                             *struct{}                            `asn1:"choice:4"`                        // AFFIRM
	DM5NULL                             *struct{}                            `asn1:"choice:5"`                        // NEGATIVE
	DM6Altitude                         *UPERAltitude                        `asn1:"choice:6"`                        // REQUEST [altitude]
	DM7AltitudeAltitude                 *UPERAltitudeAltitude                `asn1:"choice:7"`                        // REQUEST BLOCK [altitude] TO [altitude]
	DM8Altitude                         *UPERAltitude                        `asn1:"choice:8"`                        // REQUEST CRUISE CLIMB TO [altitude]
	DM9Altitude                         *UPERAltitude                        `asn1:"choice:9"`                        // REQUEST CLIMB TO [altitude]
	DM10Altitude                        *UPERAltitude                        `asn1:"choice:10"`                       // REQUEST DESCENT TO [altitude]
	DM11PositionAltitude                *UPERPositionAltitude                `asn1:"choice:11"`                       // AT [position] REQUEST CLIMB TO [altitude]
	DM12PositionAltitude                *UPERPositionAltitude                `asn1:"choice:12"`                       // AT [position] REQUEST DESCENT TO [altitude]
	DM13TimeAltitude                    *UPERTimeAltitude                    `asn1:"choice:13"`                       // AT [time] REQUEST CLIMB TO [altitude]
	DM14TimeAltitude                    *UPERTimeAltitude                    `asn1:"choice:14"`                       // AT [time] REQUEST DESCENT TO [altitude]
	DM15DistanceOffsetDirection         *UPERDistanceOffsetDirection         `asn1:"choice:15"`                       // REQUEST OFFSET [distanceoffset] [direction] OF ROUTE
	DM16PositionDistanceOffsetDirection *UPERPositionDistanceOffsetDirection `asn1:"choice:16"`                       // AT [position] REQUEST OFFSET [distanceoffset] [direction] OF ROUTE
	DM17TimeDistanceOffsetDirection     *UPERTimeDistanceOffsetDirection     `asn1:"choice:17"`                       // AT [time] REQUEST OFFSET [distanceoffset] [direction] OF ROUTE
	DM18Speed                           *UPERSpeed                           `asn1:"choice:18"`                       // REQUEST [speed]
	DM19SpeedSpeed                      *UPERSpeedSpeed                      `asn1:"choice:19"`                       // REQUEST [speed] TO [speed]
	DM20NULL                            *struct{}                            `asn1:"choice:20"`                       // REQUEST VOICE CONTACT
	DM21Frequency                       *UPERFrequency                       `asn1:"choice:21"`                       // REQUEST VOICE CONTACT [frequency]
	DM22Position                        *UPERPosition                        `asn1:"choice:22"`                       // REQUEST DIRECT TO [position]
	DM23ProcedureName                   *UPERProcedureName                   `asn1:"choice:23"`                       // REQUEST [procedurename]
	DM24RouteClearance                  *UPERRouteClearance                  `asn1:"choice:24"`                       // REQUEST [routeclearance]
	DM25NULL                            *struct{}                            `asn1:"choice:25"`                       // REQUEST CLEARANCE
	DM26PositionRouteClearance          *UPERPositionRouteClearance          `asn1:"choice:26"`                       // REQUEST WEATHER DEVIATION TO [position] VIA [routeclearance]
	DM27DistanceOffsetDirection         *UPERDistanceOffsetDirection         `asn1:"choice:27"`                       // REQUEST WEATHER DEVIATION UP TO [distanceoffset] [direction] OF ROUTE
	DM28Altitude                        *UPERAltitude                        `asn1:"choice:28"`                       // LEAVING [altitude]
	DM29Altitude                        *UPERAltitude                        `asn1:"choice:29"`                       // CLIMBING TO [altitude]
	DM30Altitude                        *UPERAltitude                        `asn1:"choice:30"`                       // DESCENDING TO [altitude]
	DM31Position                        *UPERPosition                        `asn1:"choice:31"`                       // PASSING [position]
	DM32Altitude                        *UPERAltitude                        `asn1:"choice:32"`                       // PRESENT ALTITUDE [altitude]
	DM33Position                        *UPERPosition                        `asn1:"choice:33"`                       // PRESENT POSITION [position]
	DM34Speed                           *UPERSpeed                           `asn1:"choice:34"`                       // PRESENT SPEED [speed]
	DM35Degrees                         *UPERDegrees                         `asn1:"choice:35"`                       // PRESENT HEADING [degrees]
	DM36Degrees                         *UPERDegrees                         `asn1:"choice:36"`                       // PRESENT GROUND TRACK [degrees]
	DM37Altitude                        *UPERAltitude                        `asn1:"choice:37"`                       // LEVEL [altitude]
	DM38Altitude                        *UPERAltitude                        `asn1:"choice:38"`                       // ASSIGNED ALTITUDE [altitude]
	DM39Speed                           *UPERSpeed                           `asn1:"choice:39"`                       // ASSIGNED SPEED [speed]
	DM40RouteClearance                  *UPERRouteClearance                  `asn1:"choice:40"`                       // ASSIGNED ROUTE [routeclearance]
	DM41NULL                            *struct{}                            `asn1:"choice:41"`                       // BACK ON ROUTE
	DM42Position                        *UPERPosition                        `asn1:"choice:42"`                       // NEXT WAYPOINT [position]
	DM43Time                            *UPERTime                            `asn1:"choice:43"`                       // NEXT WAYPOINT ETA [time]
	DM44Position                        *UPERPosition                        `asn1:"choice:44"`                       // ENSUING WAYPOINT [position]
	DM45Position                        *UPERPosition                        `asn1:"choice:45"`                       // REPORTED WAYPOINT [position]
	DM46Time                            *UPERTime                            `asn1:"choice:46"`                       // REPORTED WAYPOINT [time]
	DM47BeaconCode                      *UPERBeaconCode                      `asn1:"choice:47"`                       // SQUAWKING [beaconcode]
	DM48PositionReport                  *UPERPositionReport                  `asn1:"choice:48"`                       // POSITION REPORT [positionreport]
	DM49Speed                           *UPERSpeed                           `asn1:"choice:49"`                       // WHEN CAN WE EXPECT [speed]
	DM50SpeedSpeed                      *UPERSpeedSpeed                      `asn1:"choice:50"`                       // WHEN CAN WE EXPECT [speed] TO [speed]
	DM51NULL                            *struct{}                            `asn1:"choice:51"`                       // WHEN CAN WE EXPECT BACK ON ROUTE
	DM52NULL                            *struct{}                            `asn1:"choice:52"`                       // WHEN CAN WE EXPECT LOWER ALTITUDE
	DM53NULL                            *struct{}                            `asn1:"choice:53"`                       // WHEN CAN WE EXPECT HIGHER ALTITUDE
	DM54Altitude                        *UPERAltitude                        `asn1:"choice:54"`                       // WHEN CAN WE EXPECT CRUISE CLIMB TO [altitude]
	DM55NULL                            *struct{}                            `asn1:"choice:55"`                       // PAN PAN PAN
	DM56NULL                            *struct{}                            `asn1:"choice:56"`                       // MAYDAY MAYDAY MAYDAY
	DM57RemainingFuelRemainingSouls     *UPERRemainingFuelRemainingSouls     `asn1:"choice:57"`                       // [remainingfuel] OF FUEL REMAINING AND [remainingsouls] SOULS ON BOARD
	DM58NULL                            *struct{}                            `asn1:"choice:58"`                       // CANCEL EMERGENCY
	DM59PositionRouteClearance          *UPERPositionRouteClearance          `asn1:"choice:59"`                       // DIVERTING TO [position] VIA [routeclearance]
	DM60DistanceOffsetDirection         *UPERDistanceOffsetDirection         `asn1:"choice:60"`                       // OFFSETTING [distanceoffset] [direction] OF ROUTE
	DM61Altitude                        *UPERAltitude                        `asn1:"choice:61"`                       // DESCENDING TO [altitude]
	DM62ErrorInformation                *UPERErrorInformation                `asn1:"choice:62,size:0..16"`            // ERROR [errorinformation]
	DM63NULL                            *struct{}                            `asn1:"choice:63"`                       // NOT CURRENT DATA AUTHORITY
	DM64ICAOFacilityDesignation         *UPERICAOFacilityDesignation         `asn1:"choice:64,ia5string,size:4"`      // [icaofacilitydesignation]
	DM65NULL                            *struct{}                            `asn1:"choice:65"`                       // DUE TO WEATHER
	DM66NULL                            *struct{}                            `asn1:"choice:66"`                       // DUE TO AIRCRAFT PERFORMANCE
	DM67FreeText                        *UPERFreeText                        `asn1:"choice:67,ia5string,size:1..256"` // [freetext]
	DM68FreeText                        *UPERFreeText                        `asn1:"choice:68,ia5string,size:1..256"` // [freetext] (distress urgency)
	DM69NULL                            *struct{}                            `asn1:"choice:69"`                       // REQUEST VMC DESCENT
	DM70Degrees                         *UPERDegrees                         `asn1:"choice:70"`                       // REQUEST HEADING [degrees]
	DM71Degrees                         *UPERDegrees                         `asn1:"choice:71"`                       // REQUEST GROUND TRACK [degrees]
	DM72Altitude                        *UPERAltitude                        `asn1:"choice:72"`                       // REACHING [altitude]
	DM73VersionNumber                   *UPERVersionNumber                   `asn1:"choice:73,size:0..15"`            // [versionnumber]
	DM74NULL                            *struct{}                            `asn1:"choice:74"`                       // MAINTAIN OWN SEPARATION AND VMC
	DM75NULL                            *struct{}                            `asn1:"choice:75"`                       // AT PILOTS DISCRETION
	DM76AltitudeAltitude                *UPERAltitudeAltitude                `asn1:"choice:76"`                       // REACHING BLOCK [altitude] TO [altitude]
	DM77AltitudeAltitude                *UPERAltitudeAltitude                `asn1:"choice:77"`                       // ASSIGNED BLOCK [altitude] TO [altitude]
	DM78TimeDistanceToFromPosition      *UPERTimeDistanceToFromPosition      `asn1:"choice:78"`                       // AT [time] [distance] [tofrom] [position]
	DM79ATISCode                        *UPERATISCode                        `asn1:"choice:79,ia5string,size:1"`      // ATIS [atiscode]
	DM80DistanceOffsetDirection         *UPERDistanceOffsetDirection         `asn1:"choice:80"`                       // DEVIATING [distanceoffset] [direction] OF ROUTE
	DM81NULL                            *struct{}                            `asn1:"choice:81"`
	DM82NULL                            *struct{}                            `asn1:"choice:82"`
	DM83NULL                            *struct{}                            `asn1:"choice:83"`
	DM84NULL                            *struct{}                            `asn1:"choice:84"`
	DM85NULL                            *struct{}                            `asn1:"choice:85"`
	DM86NULL                            *struct{}                            `asn1:"choice:86"`
	DM87NULL                            *struct{}                            `asn1:"choice:87"`
	DM88NULL                            *struct{}                            `asn1:"choice:88"`
	DM89NULL                            *struct{}                            `asn1:"choice:89"`
	DM90NULL                            *struct{}                            `asn1:"choice:90"`
	DM91NULL                            *struct{}                            `asn1:"choice:91"`
	DM92NULL                            *struct{}                            `asn1:"choice:92"`
	DM93NULL                            *struct{}                            `asn1:"choice:93"`
	DM94NULL                            *struct{}                            `asn1:"choice:94"`
	DM95NULL                            *struct{}                            `asn1:"choice:95"`
	DM96NULL                            *struct{}                            `asn1:"choice:96"`
	DM97NULL                            *struct{}                            `asn1:"choice:97"`
	DM98NULL                            *struct{}                            `asn1:"choice:98"`
	DM99NULL                            *struct{}                            `asn1:"choice:99"`
	DM100NULL                           *struct{}                            `asn1:"choice:100"`
	DM101NULL                           *struct{}                            `asn1:"choice:101"`
	DM102NULL                           *struct{}                            `asn1:"choice:102"`
	DM103NULL                           *struct{}                            `asn1:"choice:103"`
	DM104NULL                           *struct{}                            `asn1:"choice:104"`
	DM105NULL                           *struct{}                            `asn1:"choice:105"`
	DM106NULL                           *struct{}                            `asn1:"choice:106"`
	DM107NULL                           *struct{}                            `asn1:"choice:107"`
	DM108NULL                           *struct{}                            `asn1:"choice:108"`
	DM109NULL                           *struct{}                            `asn1:"choice:109"`
	DM110NULL                           *struct{}                            `asn1:"choice:110"`
	DM111NULL                           *struct{}                            `asn1:"choice:111"`
	DM112NULL                           *struct{}                            `asn1:"choice:112"`
	DM113NULL                           *struct{}                            `asn1:"choice:113"`
	DM114NULL                           *struct{}                            `asn1:"choice:114"`
	DM115NULL                           *struct{}                            `asn1:"choice:115"`
	DM116NULL                           *struct{}                            `asn1:"choice:116"`
	DM117NULL                           *struct{}                            `asn1:"choice:117"`
	DM118NULL                           *struct{}                            `asn1:"choice:118"`
	DM119NULL                           *struct{}                            `asn1:"choice:119"`
	DM120NULL                           *struct{}                            `asn1:"choice:120"`
	DM121NULL                           *struct{}                            `asn1:"choice:121"`
	DM122NULL                           *struct{}                            `asn1:"choice:122"`
	DM123NULL                           *struct{}                            `asn1:"choice:123"`
	DM124NULL                           *struct{}                            `asn1:"choice:124"`
	DM125NULL                           *struct{}                            `asn1:"choice:125"`
	DM126NULL                           *struct{}                            `asn1:"choice:126"`
	DM127NULL                           *struct{}                            `asn1:"choice:127"`
	DM128NULL                           *struct{}                            `asn1:"choice:128"`
}

// =============================================================================
// Simple types
// =============================================================================

// UPERFreeText implements FANSFreeText, IA5String (SIZE (1..256)); a field
// of this type is tagged `ia5string,size:1..256`.
type UPERFreeText string

// UPERATISCode implements FANSATISCode, IA5String (SIZE (1)).
type UPERATISCode string

// UPERICAOFacilityDesignation implements FANSICAOfacilityDesignation,
// IA5String (SIZE (4)), a four-letter ICAO location indicator.
type UPERICAOFacilityDesignation string

// UPERVersionNumber implements FANSVersionNumber, INTEGER (0..15).
type UPERVersionNumber int

// UPERRemainingSouls implements FANSRemainingSouls, INTEGER (1..1024).
type UPERRemainingSouls int

// UPERTime implements FANSTime: hours and minutes, without seconds.
type UPERTime struct {
	Hours   int `asn1:"size:0..23"` // FANSTimehours.
	Minutes int `asn1:"size:0..59"` // FANSTimeminutes.
}

// UPERTimeTime implements FANSTimeTime, SEQUENCE SIZE (2) OF FANSTime.
type UPERTimeTime struct {
	Time1 UPERTime `data:"time1"`
	Time2 UPERTime `data:"time2"`
}

// UPERAltitude implements FANSAltitude.
type UPERAltitude struct {
	AltitudeQNH               *int `asn1:"choice:0,size:0..2500"`   // FANSAltitudeQNH, units of 10 ft.
	AltitudeQNHMeters         *int `asn1:"choice:1,size:0..16000"`  // FANSAltitudeQNHMeters, metres.
	AltitudeQFE               *int `asn1:"choice:2,size:0..2100"`   // FANSAltitudeQFE, units of 10 ft.
	AltitudeQFEMeters         *int `asn1:"choice:3,size:0..7000"`   // FANSAltitudeQFEMeters, metres.
	AltitudeGNSSFeet          *int `asn1:"choice:4,size:0..150000"` // FANSAltitudeGNSSFeet, feet.
	AltitudeGNSSMeters        *int `asn1:"choice:5,size:0..50000"`  // FANSAltitudeGNSSMeters, metres.
	AltitudeFlightLevel       *int `asn1:"choice:6,size:30..600"`   // FANSAltitudeFlightLevel, units of 100 ft.
	AltitudeFlightLevelMetric *int `asn1:"choice:7,size:100..2000"` // FANSAltitudeFlightLevelMetric, units of 10 m.
}

// UPERAltitudeAltitude implements FANSAltitudeAltitude, SEQUENCE SIZE (2)
// OF FANSAltitude.
type UPERAltitudeAltitude struct {
	Altitude1 UPERAltitude `data:"altitude1"`
	Altitude2 UPERAltitude `data:"altitude2"`
}

// UPERSpeed implements FANSSpeed.
type UPERSpeed struct {
	SpeedIndicated       *int `asn1:"choice:0,size:7..38"`   // FANSSpeedIndicated, units of 10 kt.
	SpeedIndicatedMetric *int `asn1:"choice:1,size:10..137"` // FANSSpeedIndicatedMetric, units of 10 km/h.
	SpeedTrue            *int `asn1:"choice:2,size:7..70"`   // FANSSpeedTrue, units of 10 kt.
	SpeedTrueMetric      *int `asn1:"choice:3,size:10..137"` // FANSSpeedTrueMetric, units of 10 km/h.
	SpeedGround          *int `asn1:"choice:4,size:7..70"`   // FANSSpeedGround, units of 10 kt.
	SpeedGroundMetric    *int `asn1:"choice:5,size:10..265"` // FANSSpeedGroundMetric, units of 10 km/h.
	SpeedMach            *int `asn1:"choice:6,size:61..92"`  // FANSSpeedMach, units of 0.01 Mach.
	SpeedMachLarge       *int `asn1:"choice:7,size:93..604"` // FANSSpeedMachLarge, units of 0.01 Mach.
}

// UPERSpeedSpeed implements FANSSpeedSpeed, SEQUENCE SIZE (2) OF FANSSpeed.
type UPERSpeedSpeed struct {
	Speed1 UPERSpeed `data:"speed1"`
	Speed2 UPERSpeed `data:"speed2"`
}

// UPERDegrees implements FANSDegrees.
type UPERDegrees struct {
	DegreesMagnetic *int `asn1:"choice:0,size:1..360"` // FANSDegreesMagnetic.
	DegreesTrue     *int `asn1:"choice:1,size:1..360"` // FANSDegreesTrue.
}

// UPERFrequency implements FANSFrequency.
type UPERFrequency struct {
	FrequencyHF         *int               `asn1:"choice:0,size:2850..28000"`    // FANSFrequencyhf, kHz.
	FrequencyVHF        *int               `asn1:"choice:1,size:117000..138000"` // FANSFrequencyvhf, kHz.
	FrequencyUHF        *int               `asn1:"choice:2,size:225000..399975"` // FANSFrequencyuhf, kHz.
	FrequencySatChannel *[]UPERNumericChar `asn1:"choice:3,size:12"`             // FANSFrequencysatchannel.
}

// UPERNumericChar is one character of a NumericString, which this module
// uses only for FANSFrequencysatchannel (NumericString (SIZE (12)), a
// twelve-digit telephone number). go-asn has no NumericString, so each
// character is decoded as its four-bit PER index into the NumericString
// alphabet " 0123456789" (X.691 30.5.4: the index is used because the
// largest character code does not fit in four bits): 0 is a space and 1 to
// 10 are the digits 0 to 9. Indexes 11 to 15 are not characters; see
// checkEnumerations.
type UPERNumericChar struct {
	Index numericIndex `asn1:"size:0..10"`
}

// UPERBeaconCode implements FANSBeaconCode, SEQUENCE SIZE (4) OF
// FANSBeaconCodeOctalDigit (INTEGER (0..7)).
type UPERBeaconCode struct {
	Digit1 int `asn1:"size:0..7"`
	Digit2 int `asn1:"size:0..7"`
	Digit3 int `asn1:"size:0..7"`
	Digit4 int `asn1:"size:0..7"`
}

// UPERAltimeter implements FANSAltimeter.
type UPERAltimeter struct {
	AltimeterEnglish *int `asn1:"choice:0,size:2200..3200"`  // FANSAltimeterEnglish, units of 0.01 inHg.
	AltimeterMetric  *int `asn1:"choice:1,size:7500..12500"` // FANSAltimeterMetric, units of 0.1 hPa.
}

// UPERVerticalRate implements FANSVerticalRate.
type UPERVerticalRate struct {
	VerticalRateEnglish *int `asn1:"choice:0,size:0..60"`  // FANSVerticalRateEnglish, units of 100 ft/min.
	VerticalRateMetric  *int `asn1:"choice:1,size:0..200"` // FANSVerticalRateMetric, units of 10 m/min.
}

// UPERDistance implements FANSDistance.
type UPERDistance struct {
	DistanceNm *int `asn1:"choice:0,size:0..9999"` // FANSDistanceNm, units of 0.1 NM.
	DistanceKm *int `asn1:"choice:1,size:1..1024"` // FANSDistanceKm, km.
}

// UPERDistanceOffset implements FANSDistanceOffset.
type UPERDistanceOffset struct {
	DistanceOffsetNm *int `asn1:"choice:0,size:1..128"` // FANSDistanceOffsetNm, NM.
	DistanceOffsetKm *int `asn1:"choice:1,size:1..256"` // FANSDistanceOffsetKm, km.
}

// UPERDistanceOffsetDirection implements FANSDistanceOffsetDirection.
type UPERDistanceOffsetDirection struct {
	DistanceOffset UPERDistanceOffset
	Direction      UPERDirection `asn1:"size:0..10"`
}

// UPERRemainingFuel implements FANSRemainingFuel: hours (FANSTimehours) and
// minutes (FANSTimeminutes) of fuel.
type UPERRemainingFuel struct {
	Hours   int `asn1:"size:0..23"`
	Minutes int `asn1:"size:0..59"`
}

// =============================================================================
// Positions
// =============================================================================

// UPERPosition implements FANSPosition (and the types that rename it:
// FANSPositionCurrent, FANSFixNext, FANSFixNextPlusOne and
// FANSReportedWaypointPosition).
type UPERPosition struct {
	FixName              *string                   `asn1:"choice:0,ia5string,size:1..5"` // FANSFixName.
	Navaid               *string                   `asn1:"choice:1,ia5string,size:1..4"` // FANSNavaid.
	Airport              *string                   `asn1:"choice:2,ia5string,size:4"`    // FANSAirport.
	LatitudeLongitude    *UPERLatitudeLongitude    `asn1:"choice:3"`
	PlaceBearingDistance *UPERPlaceBearingDistance `asn1:"choice:4"`
}

// UPERLatitude implements FANSLatitude.
type UPERLatitude struct {
	Degrees       int                   `asn1:"size:0..90"`           // FANSLatitudeDegrees.
	MinutesTenths *int                  `asn1:"optional,size:0..599"` // FANSMinutesLatLon, units of 0.1 minute.
	Direction     UPERLatitudeDirection `asn1:"size:0..1"`
}

// UPERLongitude implements FANSLongitude.
type UPERLongitude struct {
	Degrees       int                    `asn1:"size:0..180"`          // FANSLongitudeDegrees.
	MinutesTenths *int                   `asn1:"optional,size:0..599"` // FANSMinutesLatLon, units of 0.1 minute.
	Direction     UPERLongitudeDirection `asn1:"size:0..1"`
}

// UPERLatitudeLongitude implements FANSLatitudeLongitude.
type UPERLatitudeLongitude struct {
	Latitude  UPERLatitude
	Longitude UPERLongitude
}

// UPERPlaceBearingDistance implements FANSPlaceBearingDistance.
type UPERPlaceBearingDistance struct {
	FixName           string                 `asn1:"ia5string,size:1..5"` // FANSFixName.
	LatitudeLongitude *UPERLatitudeLongitude `asn1:"optional"`
	Degrees           UPERDegrees
	Distance          UPERDistance
}

// UPERPositionPosition implements FANSPositionPosition, SEQUENCE SIZE (2)
// OF FANSPosition.
type UPERPositionPosition struct {
	Position1 UPERPosition `data:"position1"`
	Position2 UPERPosition `data:"position2"`
}

// =============================================================================
// Sequences of message element parameters
// =============================================================================

// UPERAltitudePosition implements FANSAltitudePosition.
type UPERAltitudePosition struct {
	Altitude UPERAltitude `data:"altitude"`
	Position UPERPosition `data:"position"`
}

// UPERAltitudeSpeed implements FANSAltitudeSpeed.
type UPERAltitudeSpeed struct {
	Altitude UPERAltitude `data:"altitude"`
	Speed    UPERSpeed    `data:"speed"`
}

// UPERAltitudeSpeedSpeed implements FANSAltitudeSpeedSpeed.
type UPERAltitudeSpeedSpeed struct {
	Altitude       UPERAltitude `data:"altitude"`
	UPERSpeedSpeed `data:"-"`   // speed-seqOf.
}

// UPERAltitudeTime implements FANSAltitudeTime.
type UPERAltitudeTime struct {
	Altitude UPERAltitude `data:"altitude"`
	Time     UPERTime     `data:"time"`
}

// UPERDirectionDegrees implements FANSDirectionDegrees.
type UPERDirectionDegrees struct {
	Direction UPERDirection `asn1:"size:0..10" data:"direction"`
	Degrees   UPERDegrees   `data:"degrees"`
}

// UPERICAOUnitNameFrequency implements FANSICAOUnitNameFrequency.
type UPERICAOUnitNameFrequency struct {
	ICAOUnitName UPERICAOUnitName `data:"unit"`
	Frequency    UPERFrequency    `data:"frequency"`
}

// UPERICAOFacilityDesignationTp4Table implements
// FANSICAOFacilityDesignationTp4Table.
type UPERICAOFacilityDesignationTp4Table struct {
	ICAOFacilityDesignation UPERICAOFacilityDesignation `asn1:"ia5string,size:4" data:"facility_designation"`
	Tp4Table                UPERTp4Table                `asn1:"size:0..1" data:"tp4_table"`
}

// UPERPositionAltitude implements FANSPositionAltitude.
type UPERPositionAltitude struct {
	Position UPERPosition `data:"position"`
	Altitude UPERAltitude `data:"altitude"`
}

// UPERPositionAltitudeAltitude implements FANSPositionAltitudeAltitude.
type UPERPositionAltitudeAltitude struct {
	Position             UPERPosition `data:"position"`
	UPERAltitudeAltitude `data:"-"`   // altitude-seqOf.
}

// UPERPositionAltitudeSpeed implements FANSPositionAltitudeSpeed.
type UPERPositionAltitudeSpeed struct {
	Position UPERPosition `data:"position"`
	Altitude UPERAltitude `data:"altitude"`
	Speed    UPERSpeed    `data:"speed"`
}

// UPERPositionDegrees implements FANSPositionDegrees.
type UPERPositionDegrees struct {
	Position UPERPosition `data:"position"`
	Degrees  UPERDegrees  `data:"degrees"`
}

// UPERPositionDistanceOffsetDirection implements
// FANSPositionDistanceOffsetDirection. The offset and direction are one
// field (UPERDistanceOffsetDirection) here, which encodes the same bits as
// the module's two consecutive components and converts to one
// DistanceOffset.
type UPERPositionDistanceOffsetDirection struct {
	Position                UPERPosition                `data:"position"`
	DistanceOffsetDirection UPERDistanceOffsetDirection `data:"distance_offset"`
}

// UPERPositionICAOUnitNameFrequency implements
// FANSPositionICAOUnitNameFrequency.
type UPERPositionICAOUnitNameFrequency struct {
	Position     UPERPosition     `data:"position"`
	ICAOUnitName UPERICAOUnitName `data:"unit"`
	Frequency    UPERFrequency    `data:"frequency"`
}

// UPERPositionProcedureName implements FANSPositionProcedureName.
type UPERPositionProcedureName struct {
	Position      UPERPosition      `data:"position"`
	ProcedureName UPERProcedureName `data:"procedure"`
}

// UPERPositionRouteClearance implements FANSPositionRouteClearance.
type UPERPositionRouteClearance struct {
	Position       UPERPosition       `data:"position"`
	RouteClearance UPERRouteClearance `data:"route_clearance"`
}

// UPERPositionSpeed implements FANSPositionSpeed.
type UPERPositionSpeed struct {
	Position UPERPosition `data:"position"`
	Speed    UPERSpeed    `data:"speed"`
}

// UPERPositionSpeedSpeed implements FANSPositionSpeedSpeed.
type UPERPositionSpeedSpeed struct {
	Position       UPERPosition `data:"position"`
	UPERSpeedSpeed `data:"-"`   // speed-seqOf.
}

// UPERPositionTime implements FANSPositionTime.
type UPERPositionTime struct {
	Position UPERPosition `data:"position"`
	Time     UPERTime     `data:"time"`
}

// UPERPositionTimeAltitude implements FANSPositionTimeAltitude.
type UPERPositionTimeAltitude struct {
	Position UPERPosition `data:"position"`
	Time     UPERTime     `data:"time"`
	Altitude UPERAltitude `data:"altitude"`
}

// UPERPositionTimeTime implements FANSPositionTimeTime.
type UPERPositionTimeTime struct {
	Position     UPERPosition `data:"position"`
	UPERTimeTime `data:"-"`   // time-seqOf.
}

// UPERRemainingFuelRemainingSouls implements
// FANSRemainingFuelRemainingSouls.
type UPERRemainingFuelRemainingSouls struct {
	RemainingFuel  UPERRemainingFuel  `data:"remaining_fuel"`
	RemainingSouls UPERRemainingSouls `asn1:"size:1..1024" data:"persons_on_board"`
}

// UPERTimeAltitude implements FANSTimeAltitude.
type UPERTimeAltitude struct {
	Time     UPERTime     `data:"time"`
	Altitude UPERAltitude `data:"altitude"`
}

// UPERTimeDistanceOffsetDirection implements
// FANSTimeDistanceOffsetDirection. As in
// UPERPositionDistanceOffsetDirection, the offset and direction are one
// field.
type UPERTimeDistanceOffsetDirection struct {
	Time                    UPERTime                    `data:"time"`
	DistanceOffsetDirection UPERDistanceOffsetDirection `data:"distance_offset"`
}

// UPERTimeDistanceToFromPosition implements FANSTimeDistanceToFromPosition.
type UPERTimeDistanceToFromPosition struct {
	Time     UPERTime     `data:"time"`
	Distance UPERDistance `data:"distance"`
	ToFrom   UPERToFrom   `asn1:"size:0..1" data:"to_from"`
	Position UPERPosition `data:"position"`
}

// UPERTimeICAOUnitNameFrequency implements FANSTimeICAOunitnameFrequency.
type UPERTimeICAOUnitNameFrequency struct {
	Time         UPERTime         `data:"time"`
	ICAOUnitName UPERICAOUnitName `data:"unit"`
	Frequency    UPERFrequency    `data:"frequency"`
}

// UPERTimePosition implements FANSTimePosition.
type UPERTimePosition struct {
	Time     UPERTime     `data:"time"`
	Position UPERPosition `data:"position"`
}

// UPERTimePositionAltitude implements FANSTimePositionAltitude.
type UPERTimePositionAltitude struct {
	Time     UPERTime     `data:"time"`
	Position UPERPosition `data:"position"`
	Altitude UPERAltitude `data:"altitude"`
}

// UPERTimePositionAltitudeSpeed implements FANSTimePositionAltitudeSpeed.
type UPERTimePositionAltitudeSpeed struct {
	Time     UPERTime     `data:"time"`
	Position UPERPosition `data:"position"`
	Altitude UPERAltitude `data:"altitude"`
	Speed    UPERSpeed    `data:"speed"`
}

// UPERTimeSpeed implements FANSTimeSpeed.
type UPERTimeSpeed struct {
	Time  UPERTime  `data:"time"`
	Speed UPERSpeed `data:"speed"`
}

// UPERTimeSpeedSpeed implements FANSTimeSpeedSpeed.
type UPERTimeSpeedSpeed struct {
	Time           UPERTime   `data:"time"`
	UPERSpeedSpeed `data:"-"` // speed-seqOf.
}

// UPERToFromPosition implements FANSToFromPosition.
type UPERToFromPosition struct {
	ToFrom   UPERToFrom   `asn1:"size:0..1" data:"to_from"`
	Position UPERPosition `data:"position"`
}

// =============================================================================
// Facilities
// =============================================================================

// UPERICAOUnitName implements FANSICAOUnitName.
type UPERICAOUnitName struct {
	ICAOFacilityIdentification UPERICAOFacilityIdentification
	ICAOFacilityFunction       UPERICAOFacilityFunction `asn1:"size:0..7"`
}

// UPERICAOFacilityIdentification implements FANSICAOFacilityIdentification:
// a facility is identified either by its four-letter ICAO location
// indicator or by its name.
type UPERICAOFacilityIdentification struct {
	ICAOFacilityDesignation *string `asn1:"choice:0,ia5string,size:4"`     // FANSICAOfacilityDesignation.
	ICAOFacilityName        *string `asn1:"choice:1,ia5string,size:3..18"` // FANSICAOFacilityName.
}

// =============================================================================
// Holding (uM91)
// =============================================================================

// UPERHoldClearance implements FANSHoldClearance.
type UPERHoldClearance struct {
	Position  UPERPosition
	Altitude  UPERAltitude
	Degrees   UPERDegrees
	Direction UPERDirection `asn1:"size:0..10"`
	LegType   *UPERLegType  `asn1:"optional"`
}

// UPERLegType implements FANSLegType.
type UPERLegType struct {
	LegDistance *UPERLegDistance `asn1:"choice:0"`
	LegTime     *int             `asn1:"choice:1,size:1..99"` // FANSLegTime, units of 0.1 minute.
}

// UPERLegDistance implements FANSLegDistance.
type UPERLegDistance struct {
	LegDistanceEnglish *int `asn1:"choice:0,size:1..999"` // FANSLegDistanceEnglish, units of 0.1 NM.
	LegDistanceMetric  *int `asn1:"choice:1,size:1..128"` // FANSLegDistanceMetric, km.
}

// =============================================================================
// Procedures and route clearances
// =============================================================================

// UPERProcedureName implements FANSProcedureName (and the types that rename
// it: FANSProcedureDeparture, FANSProcedureApproach and
// FANSProcedureArrival).
type UPERProcedureName struct {
	ProcedureType       UPERProcedureType `asn1:"size:0..2"`
	Procedure           string            `asn1:"ia5string,size:1..6"`          // FANSProcedure.
	ProcedureTransition *string           `asn1:"optional,ia5string,size:1..5"` // FANSProcedureTransition.
}

// UPERRouteClearance implements FANSRouteClearance.
type UPERRouteClearance struct {
	AirportDeparture           *string                         `asn1:"optional,ia5string,size:4"`    // FANSAirportDeparture.
	AirportDestination         *string                         `asn1:"optional,ia5string,size:4"`    // FANSAirportDestination.
	RunwayDeparture            *UPERRunway                     `asn1:"optional"`                     // FANSRunwayDeparture.
	ProcedureDeparture         *UPERProcedureName              `asn1:"optional"`                     // FANSProcedureDeparture.
	RunwayArrival              *UPERRunway                     `asn1:"optional"`                     // FANSRunwayArrival.
	ProcedureApproach          *UPERProcedureName              `asn1:"optional"`                     // FANSProcedureApproach.
	ProcedureArrival           *UPERProcedureName              `asn1:"optional"`                     // FANSProcedureArrival.
	AirwayIntercept            *string                         `asn1:"optional,ia5string,size:1..5"` // FANSAirwayIntercept.
	RouteInformation           *[]UPERRouteInformation         `asn1:"optional,size:1..128"`         // FANSRouteInformationSequence.
	RouteInformationAdditional *UPERRouteInformationAdditional `asn1:"optional"`
}

// UPERRunway implements FANSRunway.
type UPERRunway struct {
	Direction     int                     `asn1:"size:1..36"` // FANSRunwayDirection.
	Configuration UPERRunwayConfiguration `asn1:"size:0..3"`
}

// UPERRouteInformation implements FANSRouteInformation.
type UPERRouteInformation struct {
	PublishedIdentifier      *UPERPublishedIdentifier      `asn1:"choice:0"`
	LatitudeLongitude        *UPERLatitudeLongitude        `asn1:"choice:1"`
	PlaceBearingPlaceBearing *UPERPlaceBearingPlaceBearing `asn1:"choice:2"`
	PlaceBearingDistance     *UPERPlaceBearingDistance     `asn1:"choice:3"`
	AirwayIdentifier         *string                       `asn1:"choice:4,ia5string,size:1..5"` // FANSAirwayIdentifier.
	TrackDetail              *UPERTrackDetail              `asn1:"choice:5"`
}

// UPERPublishedIdentifier implements FANSPublishedIdentifier.
type UPERPublishedIdentifier struct {
	FixName           string                 `asn1:"ia5string,size:1..5"` // FANSFixName.
	LatitudeLongitude *UPERLatitudeLongitude `asn1:"optional"`
}

// UPERPlaceBearing implements FANSPlaceBearing.
type UPERPlaceBearing struct {
	FixName           string                 `asn1:"ia5string,size:1..5"` // FANSFixName.
	LatitudeLongitude *UPERLatitudeLongitude `asn1:"optional"`
	Degrees           UPERDegrees
}

// UPERPlaceBearingPlaceBearing implements FANSPlaceBearingPlaceBearing,
// SEQUENCE SIZE (2) OF FANSPlaceBearing.
type UPERPlaceBearingPlaceBearing struct {
	PlaceBearing1 UPERPlaceBearing
	PlaceBearing2 UPERPlaceBearing
}

// UPERTrackDetail implements FANSTrackDetail.
type UPERTrackDetail struct {
	TrackName         string                  `asn1:"ia5string,size:3..6"` // FANSTrackName.
	LatitudeLongitude []UPERLatitudeLongitude `asn1:"size:1..128"`         // FANSLatitudeLongitudeSequence.
}

// UPERRouteInformationAdditional implements FANSRouteInformationAdditional.
type UPERRouteInformationAdditional struct {
	ATWAlongTrackWaypoints        *[]UPERATWAlongTrackWaypoint  `asn1:"optional,size:1..8"` // FANSATWAlongTrackWaypointSequence.
	ReportingPoints               *UPERReportingPoints          `asn1:"optional"`
	InterceptCourseFromSequence   *[]UPERInterceptCourseFrom    `asn1:"optional,size:1..4"`  // FANSInterceptCourseFromSequence.
	HoldAtWaypointSequence        *[]UPERHoldAtWaypoint         `asn1:"optional,size:1..8"`  // FANSHoldatwaypointSequence.
	WaypointSpeedAltitudeSequence *[]UPERWaypointSpeedAltitude  `asn1:"optional,size:1..32"` // FANSWaypointSpeedAltitudesequence.
	RTARequiredTimeArrivals       *[]UPERRTARequiredTimeArrival `asn1:"optional,size:1..32"` // FANSRTARequiredTimeArrivalSequence.
}

// UPERATWAlongTrackWaypoint implements FANSATWAlongTrackWaypoint.
type UPERATWAlongTrackWaypoint struct {
	Position     UPERPosition
	ATWDistance  UPERATWDistance
	Speed        *UPERSpeed         `asn1:"optional"`
	ATWAltitudes *[]UPERATWAltitude `asn1:"optional,size:1..2"` // FANSATWAltitudeSequence.
}

// UPERATWDistance implements FANSATWDistance.
type UPERATWDistance struct {
	ATWDistanceTolerance UPERATWDistanceTolerance `asn1:"size:0..1"`
	Distance             UPERDistance
}

// UPERATWAltitude implements FANSATWAltitude.
type UPERATWAltitude struct {
	ATWAltitudeTolerance UPERATWAltitudeTolerance `asn1:"size:0..2"`
	Altitude             UPERAltitude
}

// UPERReportingPoints implements FANSReportingPoints.
type UPERReportingPoints struct {
	LatLonReportingPoints UPERLatLonReportingPoints
	DegreeIncrement       *int `asn1:"optional,size:1..20"` // FANSDegreeIncrement, degrees.
}

// UPERLatLonReportingPoints implements FANSLatLonReportingPoints.
type UPERLatLonReportingPoints struct {
	LatitudeReportingPoints  *UPERLatitudeReportingPoints  `asn1:"choice:0"`
	LongitudeReportingPoints *UPERLongitudeReportingPoints `asn1:"choice:1"`
}

// UPERLatitudeReportingPoints implements FANSLatitudeReportingPoints.
type UPERLatitudeReportingPoints struct {
	Direction UPERLatitudeDirection `asn1:"size:0..1"`
	Degrees   int                   `asn1:"size:0..90"` // FANSLatitudeDegrees.
}

// UPERLongitudeReportingPoints implements FANSLongitudeReportingPoints.
type UPERLongitudeReportingPoints struct {
	Direction UPERLongitudeDirection `asn1:"size:0..1"`
	Degrees   int                    `asn1:"size:0..180"` // FANSLongitudeDegrees.
}

// UPERInterceptCourseFrom implements FANSInterceptCourseFrom.
type UPERInterceptCourseFrom struct {
	Selection UPERInterceptCourseFromSelection
	Degrees   UPERDegrees
}

// UPERInterceptCourseFromSelection implements
// FANSInterceptCourseFromSelection.
type UPERInterceptCourseFromSelection struct {
	PublishedIdentifier      *UPERPublishedIdentifier      `asn1:"choice:0"`
	LatitudeLongitude        *UPERLatitudeLongitude        `asn1:"choice:1"`
	PlaceBearingPlaceBearing *UPERPlaceBearingPlaceBearing `asn1:"choice:2"`
	PlaceBearingDistance     *UPERPlaceBearingDistance     `asn1:"choice:3"`
}

// UPERHoldAtWaypoint implements FANSHoldatwaypoint.
type UPERHoldAtWaypoint struct {
	Position    UPERPosition
	SpeedLow    *UPERSpeed       `asn1:"optional"` // FANSHoldatwaypointSpeedLow.
	ATWAltitude *UPERATWAltitude `asn1:"optional"`
	SpeedHigh   *UPERSpeed       `asn1:"optional"` // FANSHoldatwaypointSpeedHigh.
	Direction   *UPERDirection   `asn1:"optional,size:0..10"`
	Degrees     *UPERDegrees     `asn1:"optional"`
	EFCTime     *UPERTime        `asn1:"optional"` // FANSEFCtime.
	LegType     *UPERLegType     `asn1:"optional"`
}

// UPERWaypointSpeedAltitude implements FANSWaypointSpeedAltitude.
type UPERWaypointSpeedAltitude struct {
	Position     UPERPosition
	Speed        *UPERSpeed         `asn1:"optional"`
	ATWAltitudes *[]UPERATWAltitude `asn1:"optional,size:1..2"` // FANSATWAltitudeSequence.
}

// UPERRTARequiredTimeArrival implements FANSRTARequiredTimeArrival.
type UPERRTARequiredTimeArrival struct {
	Position     UPERPosition
	RTATime      UPERRTATime
	RTATolerance *int `asn1:"optional,size:1..150"` // FANSRTATolerance, units of 0.1 minute.
}

// UPERRTATime implements FANSRTATime.
type UPERRTATime struct {
	Time          UPERTime
	TimeTolerance UPERTimeTolerance `asn1:"size:0..2"`
}

// =============================================================================
// Pre-departure clearance (uM73)
// =============================================================================

// UPERPredepartureClearance implements FANSPredepartureClearance.
type UPERPredepartureClearance struct {
	AircraftFlightIdentification string                     `asn1:"ia5string,size:2..7"`          // FANSAircraftFlightIdentification.
	AircraftType                 *string                    `asn1:"optional,ia5string,size:2..5"` // FANSAircraftType.
	AircraftEquipmentCode        *UPERAircraftEquipmentCode `asn1:"optional"`
	TimeDepartureEdct            UPERTime                   // FANSTimeDepartureEdct.
	RouteClearance               UPERRouteClearance
	AltitudeRestriction          *UPERAltitude `asn1:"optional"`            // FANSAltitudeRestriction.
	FrequencyDeparture           int           `asn1:"size:117000..138000"` // FANSFrequencyDeparture (FANSFrequencyvhf), kHz.
	BeaconCode                   UPERBeaconCode
	PDCRevision                  int `asn1:"size:1..16"` // FANSPDCrevision.
}

// UPERAircraftEquipmentCode implements FANSAircraftEquipmentCode.
type UPERAircraftEquipmentCode struct {
	COMNAVApproachEquipmentAvailable bool                             // FANSCOMNAVApproachEquipmentAvailable (BOOLEAN).
	COMNAVEquipmentStatus            *[]UPERCOMNAVEquipmentStatusItem `asn1:"optional,size:1..16"` // FANSCOMNAVEquipmentStatusSequence.
	SSREquipmentAvailable            UPERSSREquipmentAvailable        `asn1:"size:0..6"`
}

// UPERCOMNAVEquipmentStatusItem is one element of
// FANSCOMNAVEquipmentStatusSequence. go-asn takes the constraints of a
// SEQUENCE OF element from its struct tags, so the enumeration is wrapped
// in a struct; the wrapper encodes no bits of its own.
type UPERCOMNAVEquipmentStatusItem struct {
	Status UPERCOMNAVEquipmentStatus `asn1:"size:0..15"`
}

// =============================================================================
// Position report (dM48)
// =============================================================================

// UPERPositionReport implements FANSPositionReport: three mandatory
// components and nineteen optional ones, in the module's order.
type UPERPositionReport struct {
	PositionCurrent          UPERPosition // FANSPositionCurrent.
	TimeAtPositionCurrent    UPERTime     // FANSTimeAtPositionCurrent.
	Altitude                 UPERAltitude
	FixNext                  *UPERPosition       `asn1:"optional"` // FANSFixNext.
	TimeEtaAtFixNext         *UPERTime           `asn1:"optional"` // FANSTimeEtaAtFixNext.
	FixNextPlusOne           *UPERPosition       `asn1:"optional"` // FANSFixNextPlusOne.
	TimeEtaDestination       *UPERTime           `asn1:"optional"` // FANSTimeEtaDestination.
	RemainingFuel            *UPERRemainingFuel  `asn1:"optional"`
	Temperature              *UPERTemperature    `asn1:"optional"`
	Winds                    *UPERWinds          `asn1:"optional"`
	Turbulence               *UPERTurbulence     `asn1:"optional,size:0..2"`
	Icing                    *UPERIcing          `asn1:"optional,size:0..3"`
	Speed                    *UPERSpeed          `asn1:"optional"`
	SpeedGround              *int                `asn1:"optional,size:7..70"` // FANSSpeedGround, units of 10 kt.
	VerticalChange           *UPERVerticalChange `asn1:"optional"`
	TrackAngle               *UPERDegrees        `asn1:"optional"` // FANSTrackAngle.
	TrueHeading              *UPERDegrees        `asn1:"optional"` // FANSTrueheading.
	Distance                 *UPERDistance       `asn1:"optional"`
	SupplementaryInformation *string             `asn1:"optional,ia5string,size:1..256"` // FANSSupplementaryInformation.
	ReportedWaypointPosition *UPERPosition       `asn1:"optional"`                       // FANSReportedWaypointPosition.
	ReportedWaypointTime     *UPERTime           `asn1:"optional"`                       // FANSReportedWaypointTime.
	ReportedWaypointAltitude *UPERAltitude       `asn1:"optional"`                       // FANSReportedWaypointAltitude.
}

// UPERTemperature implements FANSTemperature.
type UPERTemperature struct {
	TemperatureC *int `asn1:"choice:0,size:-80..47"`   // FANSTemperatureC, degrees Celsius.
	TemperatureF *int `asn1:"choice:1,size:-105..150"` // FANSTemperatureF, degrees Fahrenheit.
}

// UPERWinds implements FANSWinds.
type UPERWinds struct {
	WindDirection int `asn1:"size:1..360"` // FANSWindDirection, degrees.
	WindSpeed     UPERWindSpeed
}

// UPERWindSpeed implements FANSWindSpeed.
type UPERWindSpeed struct {
	WindSpeedEnglish *int `asn1:"choice:0,size:0..255"` // FANSWindSpeedEnglish, kt.
	WindSpeedMetric  *int `asn1:"choice:1,size:0..511"` // FANSWindSpeedMetric, km/h.
}

// UPERVerticalChange implements FANSVerticalChange.
type UPERVerticalChange struct {
	VerticalDirection UPERVerticalDirection `asn1:"size:0..1"`
	VerticalRate      UPERVerticalRate
}

// =============================================================================
// Enumerations
// =============================================================================

// enumerated is implemented by the Go types of the module's ENUMERATED
// types. names returns the enumeration identifiers in index order, which
// is the order of their values in the module.
type enumerated interface {
	names() []string
}

// UPERDirection implements FANSDirection.
type UPERDirection int

func (UPERDirection) names() []string {
	return []string{"left", "right", "eitherSide", "north", "south", "east", "west",
		"northEast", "northWest", "southEast", "southWest"}
}

// UPERErrorInformation implements FANSErrorInformation. The module defines
// eleven errors (0 to 10) and reservedErrorMsg (16), "to ensure 5 bit
// FANSErrorInformation field". An X.691 encoder would encode the twelve
// enumerations' index in four bits; asn1c (and so libacars and dumpvdl2)
// takes the range of the values, 0..16, and reads five bits, which is the
// width the module's comment intends. libacars' module names indexes 11
// to 16 reservedErrorMsg1 to reservedErrorMsg6 and accepts them, so they
// are accepted here too, all as reservedErrorMsg.
type UPERErrorInformation int

func (UPERErrorInformation) names() []string {
	return []string{"applicationError", "duplicateMsgIdentificationNumber",
		"unrecognizedMsgReferenceNumber", "endServiceWithPendingMsgs",
		"endServiceWithNoValidResponse", "insufficientMsgStorageCapacity",
		"noAvailableMsgIdentificationNumber", "commandedTermination",
		"insufficientData", "unexpectedData", "invalidData",
		"reservedErrorMsg", "reservedErrorMsg", "reservedErrorMsg",
		"reservedErrorMsg", "reservedErrorMsg", "reservedErrorMsg"}
}

// UPERICAOFacilityFunction implements FANSICAOFacilityFunction.
type UPERICAOFacilityFunction int

func (UPERICAOFacilityFunction) names() []string {
	return []string{"center", "approach", "tower", "final", "groundControl",
		"clearanceDelivery", "departure", "control"}
}

// UPERProcedureType implements FANSProcedureType.
type UPERProcedureType int

func (UPERProcedureType) names() []string { return []string{"arrival", "approach", "departure"} }

// UPERRunwayConfiguration implements FANSRunwayConfiguration.
type UPERRunwayConfiguration int

func (UPERRunwayConfiguration) names() []string { return []string{"left", "right", "center", "none"} }

// UPERToFrom implements FANSToFrom.
type UPERToFrom int

func (UPERToFrom) names() []string { return []string{"to", "from"} }

// UPERTp4Table implements FANSTp4table.
type UPERTp4Table int

func (UPERTp4Table) names() []string { return []string{"labelA", "labelB"} }

// UPERTurbulence implements FANSTurbulence.
type UPERTurbulence int

func (UPERTurbulence) names() []string { return []string{"light", "moderate", "severe"} }

// UPERIcing implements FANSIcing.
type UPERIcing int

func (UPERIcing) names() []string { return []string{"trace", "light", "moderate", "severe"} }

// UPERVerticalDirection implements FANSVerticalDirection.
type UPERVerticalDirection int

func (UPERVerticalDirection) names() []string { return []string{"up", "down"} }

// UPERLatitudeDirection implements FANSLatitudeDirection.
type UPERLatitudeDirection int

func (UPERLatitudeDirection) names() []string { return []string{"north", "south"} }

// UPERLongitudeDirection implements FANSLongitudeDirection.
type UPERLongitudeDirection int

func (UPERLongitudeDirection) names() []string { return []string{"east", "west"} }

// UPERATWAltitudeTolerance implements FANSATWAltitudeTolerance.
type UPERATWAltitudeTolerance int

func (UPERATWAltitudeTolerance) names() []string { return []string{"at", "atorabove", "atorbelow"} }

// UPERATWDistanceTolerance implements FANSATWDistanceTolerance.
type UPERATWDistanceTolerance int

func (UPERATWDistanceTolerance) names() []string { return []string{"plus", "minus"} }

// UPERTimeTolerance implements FANSTimeTolerance.
type UPERTimeTolerance int

func (UPERTimeTolerance) names() []string { return []string{"at", "atorafter", "atorbefore"} }

// UPERCOMNAVEquipmentStatus implements FANSCOMNAVEquipmentStatus.
type UPERCOMNAVEquipmentStatus int

func (UPERCOMNAVEquipmentStatus) names() []string {
	return []string{"aloranA", "cloranC", "ddme", "edecca", "fadf", "ggnss", "hhfRTF",
		"iinertialNavigation", "lils", "momega", "ovor", "pdoppler",
		"rrnavRouteEquipment", "ttacan", "uuhfRTF", "vvhfRTF"}
}

// UPERSSREquipmentAvailable implements FANSSSREquipmentAvailable.
type UPERSSREquipmentAvailable int

func (UPERSSREquipmentAvailable) names() []string {
	return []string{"nnil", "atransponderModeA", "ctransponderModeAandC",
		"xtransponderModeS", "ptransponderModeSPA", "itransponderModeSID",
		"stransponderModeSPAID"}
}

// numericIndex is the PER index of a NumericString character (see
// UPERNumericChar). It is checked like an enumeration: the eleven indexes
// name the characters of the alphabet.
type numericIndex int

// numericAlphabet is the NumericString alphabet in PER index order.
const numericAlphabet = " 0123456789"

func (numericIndex) names() []string {
	names := make([]string, len(numericAlphabet))
	for i := range numericAlphabet {
		names[i] = numericAlphabet[i : i+1]
	}
	return names
}
