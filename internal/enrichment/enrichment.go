// Package enrichment extracts flight enrichment data from parsed ACARS messages.
// This data is used to enhance ADS-B tracking with route, ETA, runway, and PAX info.
package enrichment

import (
	"encoding/json"
	"strings"
	"time"

	"acars_parser/internal/extractor"
	"acars_parser/internal/registry"
	"acars_parser/internal/storage"
)

// ExtractEnrichment extracts enrichment data from parsed results.
// Returns nil if no enrichable data is found or if key fields are missing.
//
// Parameters:
//   - icaoHex: The aircraft's ICAO 24-bit address (from NATS envelope or aircraft lookup)
//   - callsign: The flight number/callsign (from message or envelope)
//   - timestamp: Message timestamp for determining flight date
//   - results: Parsed results from the registry
func ExtractEnrichment(icaoHex, callsign string, timestamp time.Time, results []registry.Result) *storage.FlightEnrichmentUpdate {
	if icaoHex == "" {
		return nil // Can't enrich without aircraft identifier
	}

	// Calculate flight date (UTC midnight).
	flightDate := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, time.UTC)

	update := &storage.FlightEnrichmentUpdate{
		ICAOHex:    strings.ToUpper(icaoHex),
		Callsign:   extractor.NormaliseFlightNumber(callsign),
		FlightDate: flightDate,
	}

	// Process each parsed result.
	for _, result := range results {
		extractFromResult(update, result)
	}

	// If no callsign found, can't create a useful enrichment record.
	if update.Callsign == "" {
		return nil
	}

	// Only return if we have something to enrich.
	if !hasEnrichmentData(update) {
		return nil
	}

	return update
}

// extractFromResult extracts enrichment fields from a single parser result.
func extractFromResult(update *storage.FlightEnrichmentUpdate, result registry.Result) {
	// Convert result to map for generic field access.
	data := resultToMap(result)
	if data == nil {
		return
	}

	// Try to extract callsign from various field names.
	if update.Callsign == "" {
		if v := getStringField(data, "flight_number", "flight_num", "flight"); v != "" {
			update.Callsign = extractor.NormaliseFlightNumber(v)
		}
	}

	switch result.Type() {
	case "pdc":
		extractPDC(update, data)
	case "flight_plan":
		extractFlightPlan(update, data)
	case "loadsheet":
		extractLoadsheet(update, data)
	case "eta":
		extractETA(update, data)
	}
}

// extractPDC extracts enrichment data from a PDC (Pre-Departure Clearance) result.
func extractPDC(update *storage.FlightEnrichmentUpdate, data map[string]interface{}) {
	if v := getStringField(data, "origin"); v != "" {
		update.Origin = &v
	}
	if v := getStringField(data, "destination"); v != "" {
		update.Destination = &v
	}
	if v := getStringField(data, "runway"); v != "" {
		update.DepartureRunway = &v
	}
	if v := getStringField(data, "sid"); v != "" {
		update.SID = &v
	}
	if v := getStringField(data, "squawk"); v != "" {
		update.Squawk = &v
	}

	// Extract route waypoints if present.
	if waypoints, ok := data["route_waypoints"].([]interface{}); ok && len(waypoints) > 0 {
		var route []string
		for _, wp := range waypoints {
			if s, ok := wp.(string); ok && s != "" {
				route = append(route, s)
			}
		}
		if len(route) > 0 {
			update.Route = route
		}
	}
}

// extractFlightPlan extracts enrichment data from an FPN (Flight Plan) result.
func extractFlightPlan(update *storage.FlightEnrichmentUpdate, data map[string]interface{}) {
	if v := getStringField(data, "origin"); v != "" {
		update.Origin = &v
	}
	if v := getStringField(data, "destination"); v != "" {
		update.Destination = &v
	}

	// Extract waypoints as route.
	if waypoints, ok := data["waypoints"].([]interface{}); ok && len(waypoints) > 0 {
		var route []string
		for _, wp := range waypoints {
			if wpMap, ok := wp.(map[string]interface{}); ok {
				if name, ok := wpMap["name"].(string); ok && name != "" {
					route = append(route, name)
				}
			}
		}
		if len(route) > 0 {
			update.Route = route
		}
	}
}

// extractLoadsheet extracts enrichment data from a loadsheet result.
func extractLoadsheet(update *storage.FlightEnrichmentUpdate, data map[string]interface{}) {
	if v := getStringField(data, "origin"); v != "" {
		update.Origin = &v
	}
	if v := getStringField(data, "destination"); v != "" {
		update.Destination = &v
	}

	// Extract PAX count.
	if pax, ok := data["pax"].(float64); ok && pax > 0 {
		paxInt := int(pax)
		update.PaxCount = &paxInt
	}

	// TODO: Extract pax_breakdown when loadsheet parser supports class breakdown.
}

// extractETA extracts enrichment data from an ETA result.
func extractETA(update *storage.FlightEnrichmentUpdate, data map[string]interface{}) {
	if v := getStringField(data, "origin"); v != "" {
		update.Origin = &v
	}
	if v := getStringField(data, "destination"); v != "" {
		update.Destination = &v
	}

	// TODO: Parse and store ETA time when we have a consistent format.
	// The ETA field in eta.Result is a string like "1830" (HHMM).
}

// resultToMap converts a registry.Result to a map via JSON for generic field access.
func resultToMap(result registry.Result) map[string]interface{} {
	data, err := json.Marshal(result)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

// getStringField returns the first non-empty string value for any of the given keys.
func getStringField(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := data[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// hasEnrichmentData checks if the update has any enrichable data beyond the key fields.
func hasEnrichmentData(u *storage.FlightEnrichmentUpdate) bool {
	return u.Origin != nil || u.Destination != nil || len(u.Route) > 0 ||
		u.ETA != nil || u.DepartureRunway != nil || u.ArrivalRunway != nil || u.SID != nil || u.Squawk != nil ||
		u.PaxCount != nil || len(u.PaxBreakdown) > 0
}