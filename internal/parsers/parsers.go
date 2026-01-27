// Package parsers imports all parser packages to trigger their init() registration.
// Import this package for side effects only.
package parsers

import (
	// Import all parser packages to register them with the registry.
	_ "acars_parser/internal/parsers/adsc"
	_ "acars_parser/internal/parsers/agfsr"
	_ "acars_parser/internal/parsers/atis"
	_ "acars_parser/internal/parsers/cpdlc"
	_ "acars_parser/internal/parsers/envelope"
	_ "acars_parser/internal/parsers/eta"
	_ "acars_parser/internal/parsers/fst"
	_ "acars_parser/internal/parsers/gateassign"
	_ "acars_parser/internal/parsers/h1"
	_ "acars_parser/internal/parsers/h2wind"
	_ "acars_parser/internal/parsers/label10"
	_ "acars_parser/internal/parsers/label16"
	_ "acars_parser/internal/parsers/label21"
	_ "acars_parser/internal/parsers/label22"
	_ "acars_parser/internal/parsers/label44"
	_ "acars_parser/internal/parsers/label4j"
	_ "acars_parser/internal/parsers/label5l"
	_ "acars_parser/internal/parsers/label80"
	_ "acars_parser/internal/parsers/labelrf"
	_ "acars_parser/internal/parsers/landingdata"
	_ "acars_parser/internal/parsers/label83"
	_ "acars_parser/internal/parsers/labelb2"
	_ "acars_parser/internal/parsers/loadsheet"
	_ "acars_parser/internal/parsers/labelb3"
	_ "acars_parser/internal/parsers/mediaadv"
	_ "acars_parser/internal/parsers/pdc"
	_ "acars_parser/internal/parsers/sq"
	_ "acars_parser/internal/parsers/turbulence"
	_ "acars_parser/internal/parsers/weather"
)
