package main

import (
	"time"

	"acars_parser/internal/acars"
	"acars_parser/internal/extractor"
	"acars_parser/internal/registry"
	"acars_parser/internal/storage"
)

// resultRow returns the archive row that stores one parse result of a
// message.
func resultRow(msg *acars.Message, result registry.Result) storage.CHInsertParams {
	origin, dest := extractRouteFromResult(result)
	return storage.CHInsertParams{
		ID:            uint64(msg.ID),
		Timestamp:     parseTimestamp(msg.Timestamp),
		Label:         msg.Label,
		ParserType:    result.Type(),
		Flight:        msg.FlightNumber,
		Tail:          msg.Tail,
		Origin:        origin,
		Destination:   dest,
		RawText:       msg.Text,
		ParsedData:    result,
		MissingFields: getMissingFields(result),
		Confidence:    float32(extractConfidenceFromResult(result)),
		BlockFields:   blockFields(msg),
	}
}

// unparsedRow returns the archive row that stores a message no parser
// matched.
func unparsedRow(msg *acars.Message) storage.CHInsertParams {
	return storage.CHInsertParams{
		ID:          uint64(msg.ID),
		Timestamp:   parseTimestamp(msg.Timestamp),
		Label:       msg.Label,
		ParserType:  "unparsed",
		Flight:      msg.FlightNumber,
		Tail:        msg.Tail,
		RawText:     msg.Text,
		ParsedData:  map[string]string{"label": msg.Label},
		BlockFields: blockFields(msg),
	}
}

// blockFields returns a message's block fields as the archive stores them.
func blockFields(msg *acars.Message) storage.BlockFields {
	b := storage.BlockFields{MessageNumber: msg.MessageNumber, BlockID: msg.BlockID, BlockEnd: msg.BlockEnd}
	for _, id := range msg.AssembledFrom {
		b.AssembledFrom = append(b.AssembledFrom, uint64(id))
	}
	return b
}

// setBlockFields sets a message's block fields from those stored.
func setBlockFields(msg *acars.Message, b storage.BlockFields) {
	msg.MessageNumber, msg.BlockID, msg.BlockEnd = b.MessageNumber, b.BlockID, b.BlockEnd
	msg.AssembledFrom = nil
	for _, id := range b.AssembledFrom {
		msg.AssembledFrom = append(msg.AssembledFrom, int64(id))
	}
}

// messageRows returns the archive rows of a message: one per parse result,
// or one "unparsed" row if there is none. A message without text has no
// rows.
func messageRows(msg *acars.Message, results []registry.Result) []storage.CHInsertParams {
	if len(results) == 0 {
		if msg.Text == "" {
			return nil
		}
		return []storage.CHInsertParams{unparsedRow(msg)}
	}
	rows := make([]storage.CHInsertParams, 0, len(results))
	for _, result := range results {
		rows = append(rows, resultRow(msg, result))
	}
	return rows
}

// flightRoute returns the route row for a flight update, and false if there
// is none. The routes table maps a flight number to its ICAO origin and
// destination, so a route needs both ICAO endpoints (IATA pairs are kept by
// the extractor for storage v2) and a flight number: a route seen without
// one is an aircraft's movement, not a flight's route, and is not recorded
// (it used to be recorded under the registration).
func flightRoute(f *extractor.FlightUpdate, seen time.Time) (storage.Route, bool) {
	if f == nil || f.AirportCodes != extractor.AirportCodesICAO || f.FlightNumber == "" {
		return storage.Route{}, false
	}
	return storage.Route{
		FlightPattern:    f.FlightNumber,
		OriginICAO:       f.Origin,
		DestICAO:         f.Destination,
		ObservationCount: 1,
		FirstSeen:        seen,
		LastSeen:         seen,
	}, true
}
