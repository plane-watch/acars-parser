package aircrafttype

// designators is the set of ICAO aircraft type designators (ICAO Doc 8643)
// that Normalise will return. It covers the types seen in the January 2026
// corpus and common airliner, regional, business and military types.
//
// The list is deliberately explicit: older parser output shows that values
// shaped like designators are often something else (flight levels such as
// "F240", checksum fragments such as "C8AE", callsign fragments such as
// "BAW8"), so a value that is not listed is not normalised. Callers keep the
// raw value. Add a designator here when traffic shows it.
var designators = toSet(
	// Airbus.
	"A306", "A30B", "A310", "A318", "A319", "A320", "A321", "A19N", "A20N", "A21N",
	"A332", "A333", "A337", "A338", "A339", "A342", "A343", "A345", "A346",
	"A359", "A35K", "A388", "A400", "BCS1", "BCS3",
	// Boeing.
	"B712", "B721", "B722", "B731", "B732", "B733", "B734", "B735", "B736", "B737",
	"B738", "B739", "B37M", "B38M", "B39M", "B3XM", "B741", "B742", "B743", "B744",
	"B748", "B74S", "B752", "B753", "B762", "B763", "B764", "B772", "B773", "B77L",
	"B77W", "B778", "B779", "B788", "B789", "B78X", "MD11", "MD82", "MD83", "MD87",
	"MD88", "MD90", "DC10", "DC93",
	// Embraer.
	"E135", "E145", "E170", "E175", "E75L", "E75S", "E190", "E195", "E290", "E295",
	"E275", "E35L", "E50P", "E55P", "E545", "E550",
	// Bombardier and De Havilland Canada.
	"CRJ1", "CRJ2", "CRJ7", "CRJ9", "CRJX", "DH8A", "DH8B", "DH8C", "DH8D",
	"CL30", "CL35", "CL60", "GLEX", "GL5T", "GL7T", "GL6T",
	// ATR, Saab, Fokker and others.
	"AT43", "AT45", "AT72", "AT75", "AT76", "SF34", "SB20", "F70", "F100",
	"BA46", "RJ85", "RJ1H", "SU95", "C919", "AJ27", "J328",
	// Business jets.
	"GLF4", "GLF5", "GLF6", "G280", "GA5C", "GA6C", "GA7C", "GA8C",
	"C25A", "C25B", "C25C", "C510", "C525", "C550", "C560", "C56X", "C680", "C68A",
	"C700", "C750", "F2TH", "F900", "FA7X", "FA8X", "FA6X", "FA50",
	"LJ35", "LJ45", "LJ60", "LJ75", "H25B", "PC12", "PC24", "B350", "BE20",
	// Military and cargo.
	"C130", "C30J", "C17", "K35R", "A124", "IL76", "B52", "P8",
)

func toSet(values ...string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}
