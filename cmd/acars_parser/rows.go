package main

import (
	"acars_parser/internal/acars"
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
	}
}

// unparsedRow returns the archive row that stores a message no parser
// matched.
func unparsedRow(msg *acars.Message) storage.CHInsertParams {
	return storage.CHInsertParams{
		ID:         uint64(msg.ID),
		Timestamp:  parseTimestamp(msg.Timestamp),
		Label:      msg.Label,
		ParserType: "unparsed",
		Flight:     msg.FlightNumber,
		Tail:       msg.Tail,
		RawText:    msg.Text,
		ParsedData: map[string]string{"label": msg.Label},
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
