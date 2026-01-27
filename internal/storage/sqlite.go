// Package storage provides persistent storage for parsed ACARS messages.
// This file contains read-only SQLite functions for migration and legacy data access.
package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Message represents a stored ACARS message with its parsed result.
type Message struct {
	ID            int64
	Timestamp     time.Time
	Label         string
	ParserType    string
	Flight        string
	Tail          string
	Origin        string
	Destination   string
	RawText       string
	ParsedJSON    string
	MissingFields string
	Confidence    float64
	IsGolden      bool
	Annotation    string
	ExpectedJSON  string
}

// SQLiteDB wraps a SQLite database connection for read-only message access.
// Used for migration and legacy data queries. New data goes to ClickHouse/PostgreSQL.
type SQLiteDB struct {
	db *sql.DB
}

// OpenSQLite opens an existing SQLite database in read-only mode.
func OpenSQLite(path string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return &SQLiteDB{db: db}, nil
}

// Close closes the database connection.
func (d *SQLiteDB) Close() error {
	return d.db.Close()
}

// QueryParams contains filtering options for querying messages.
type QueryParams struct {
	ID           int64  // Filter by specific message ID.
	ParserType   string // Filter by parser type (exact match).
	Label        string // Filter by ACARS label (exact match).
	Flight       string // Filter by flight number (LIKE match).
	MissingField string // Filter by specific missing field (LIKE match).
	HasMissing   bool   // Only show messages with any missing fields.
	FullText     string // FTS5 full-text search on raw_text.
	Limit        int    // Max results (default 100).
	Offset       int    // Pagination offset.
	OrderBy      string // Sort field (timestamp, parser_type, confidence).
	OrderDesc    bool   // Sort descending.
}

// Query retrieves messages matching the given parameters.
func (d *SQLiteDB) Query(p QueryParams) ([]Message, error) {
	var conditions []string
	var args []interface{}

	if p.ID != 0 {
		conditions = append(conditions, "id = ?")
		args = append(args, p.ID)
	}
	if p.ParserType != "" {
		conditions = append(conditions, "parser_type = ?")
		args = append(args, p.ParserType)
	}
	if p.Label != "" {
		conditions = append(conditions, "label = ?")
		args = append(args, p.Label)
	}
	if p.Flight != "" {
		conditions = append(conditions, "flight LIKE ?")
		args = append(args, "%"+p.Flight+"%")
	}
	if p.MissingField != "" {
		conditions = append(conditions, "missing_fields LIKE ?")
		args = append(args, "%"+p.MissingField+"%")
	}
	if p.HasMissing {
		conditions = append(conditions, "missing_fields != '' AND missing_fields IS NOT NULL")
	}

	// Handle FTS5 search - requires a JOIN with the FTS table.
	var query string
	if p.FullText != "" {
		query = `SELECT m.id, m.timestamp, m.label, m.parser_type, m.flight, m.tail,
				m.origin, m.destination, m.raw_text, m.parsed_json, m.missing_fields, m.confidence,
				m.is_golden, m.annotation, m.expected_json
				FROM messages m
				JOIN messages_fts fts ON m.id = fts.rowid
				WHERE messages_fts MATCH ?`
		args = append([]interface{}{p.FullText}, args...)
		if len(conditions) > 0 {
			query += " AND " + strings.Join(conditions, " AND ")
		}
	} else {
		query = `SELECT id, timestamp, label, parser_type, flight, tail,
				origin, destination, raw_text, parsed_json, missing_fields, confidence,
				is_golden, annotation, expected_json
				FROM messages`
		if len(conditions) > 0 {
			query += " WHERE " + strings.Join(conditions, " AND ")
		}
	}

	// Order by.
	orderField := "id"
	if p.OrderBy != "" {
		switch p.OrderBy {
		case "timestamp", "parser_type", "confidence", "label", "flight":
			orderField = p.OrderBy
		}
	}
	direction := "ASC"
	if p.OrderDesc {
		direction = "DESC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", orderField, direction)

	// Limit and offset.
	limit := 100
	if p.Limit > 0 {
		limit = p.Limit
	}
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, p.Offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []Message
	for rows.Next() {
		var m Message
		var ts, missing, annotation, expectedJSON sql.NullString
		var confidence sql.NullFloat64
		var isGolden sql.NullInt64

		err := rows.Scan(&m.ID, &ts, &m.Label, &m.ParserType, &m.Flight, &m.Tail,
			&m.Origin, &m.Destination, &m.RawText, &m.ParsedJSON, &missing, &confidence,
			&isGolden, &annotation, &expectedJSON)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		if ts.Valid {
			m.Timestamp, _ = time.Parse(time.RFC3339, ts.String)
		}
		if missing.Valid {
			m.MissingFields = missing.String
		}
		if confidence.Valid {
			m.Confidence = confidence.Float64
		}
		if isGolden.Valid {
			m.IsGolden = isGolden.Int64 == 1
		}
		if annotation.Valid {
			m.Annotation = annotation.String
		}
		if expectedJSON.Valid {
			m.ExpectedJSON = expectedJSON.String
		}

		messages = append(messages, m)
	}

	return messages, rows.Err()
}

// Stats returns aggregate statistics about stored messages.
type Stats struct {
	TotalMessages    int
	ByParserType     map[string]int
	ByLabel          map[string]int
	WithMissing      int
	TopMissingFields map[string]int
}

// GetStats returns statistics about the stored messages.
func (d *SQLiteDB) GetStats() (*Stats, error) {
	stats := &Stats{
		ByParserType:     make(map[string]int),
		ByLabel:          make(map[string]int),
		TopMissingFields: make(map[string]int),
	}

	// Total messages.
	row := d.db.QueryRow("SELECT COUNT(*) FROM messages")
	if err := row.Scan(&stats.TotalMessages); err != nil {
		return nil, err
	}

	// By parser type.
	rows, err := d.db.Query("SELECT parser_type, COUNT(*) FROM messages GROUP BY parser_type ORDER BY COUNT(*) DESC")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var typ string
		var count int
		if err := rows.Scan(&typ, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		stats.ByParserType[typ] = count
	}
	_ = rows.Close()

	// By label.
	rows, err = d.db.Query("SELECT label, COUNT(*) FROM messages GROUP BY label ORDER BY COUNT(*) DESC LIMIT 20")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		stats.ByLabel[label] = count
	}
	_ = rows.Close()

	// With missing fields.
	row = d.db.QueryRow("SELECT COUNT(*) FROM messages WHERE missing_fields != '' AND missing_fields IS NOT NULL")
	if err := row.Scan(&stats.WithMissing); err != nil {
		return nil, err
	}

	// Top missing fields - requires parsing the comma-separated values.
	rows, err = d.db.Query("SELECT missing_fields FROM messages WHERE missing_fields != '' AND missing_fields IS NOT NULL")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var fields string
		if err := rows.Scan(&fields); err != nil {
			_ = rows.Close()
			return nil, err
		}
		for _, f := range strings.Split(fields, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				stats.TopMissingFields[f]++
			}
		}
	}
	_ = rows.Close()

	return stats, nil
}

// Distinct returns distinct values for a given column.
func (d *SQLiteDB) Distinct(column string) ([]string, error) {
	// Validate column name to prevent SQL injection.
	validColumns := map[string]bool{
		"parser_type": true,
		"label":       true,
		"flight":      true,
		"origin":      true,
		"destination": true,
	}
	if !validColumns[column] {
		return nil, fmt.Errorf("invalid column: %s", column)
	}

	query := fmt.Sprintf("SELECT DISTINCT %s FROM messages WHERE %s IS NOT NULL AND %s != '' ORDER BY %s", column, column, column, column)
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

// GetByID retrieves a single message by ID.
func (d *SQLiteDB) GetByID(id int64) (*Message, error) {
	query := `SELECT id, timestamp, label, parser_type, flight, tail,
			origin, destination, raw_text, parsed_json, missing_fields, confidence,
			is_golden, annotation, expected_json
			FROM messages WHERE id = ?`

	var m Message
	var ts, missing, annotation, expectedJSON sql.NullString
	var confidence sql.NullFloat64
	var isGolden sql.NullInt64

	err := d.db.QueryRow(query, id).Scan(&m.ID, &ts, &m.Label, &m.ParserType, &m.Flight, &m.Tail,
		&m.Origin, &m.Destination, &m.RawText, &m.ParsedJSON, &missing, &confidence,
		&isGolden, &annotation, &expectedJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if ts.Valid {
		m.Timestamp, _ = time.Parse(time.RFC3339, ts.String)
	}
	if missing.Valid {
		m.MissingFields = missing.String
	}
	if confidence.Valid {
		m.Confidence = confidence.Float64
	}
	if isGolden.Valid {
		m.IsGolden = isGolden.Int64 == 1
	}
	if annotation.Valid {
		m.Annotation = annotation.String
	}
	if expectedJSON.Valid {
		m.ExpectedJSON = expectedJSON.String
	}

	return &m, nil
}

// GetByAcarsID retrieves a message by the ACARS message ID stored in parsed_json.
func (d *SQLiteDB) GetByAcarsID(acarsID int64) (*Message, error) {
	query := `SELECT id, timestamp, label, parser_type, flight, tail,
			origin, destination, raw_text, parsed_json, missing_fields, confidence,
			is_golden, annotation, expected_json
			FROM messages WHERE json_extract(parsed_json, '$.message_id') = ?`

	var m Message
	var ts, missing, annotation, expectedJSON sql.NullString
	var confidence sql.NullFloat64
	var isGolden sql.NullInt64

	err := d.db.QueryRow(query, acarsID).Scan(&m.ID, &ts, &m.Label, &m.ParserType, &m.Flight, &m.Tail,
		&m.Origin, &m.Destination, &m.RawText, &m.ParsedJSON, &missing, &confidence,
		&isGolden, &annotation, &expectedJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if ts.Valid {
		m.Timestamp, _ = time.Parse(time.RFC3339, ts.String)
	}
	if missing.Valid {
		m.MissingFields = missing.String
	}
	if confidence.Valid {
		m.Confidence = confidence.Float64
	}
	if isGolden.Valid {
		m.IsGolden = isGolden.Int64 == 1
	}
	if annotation.Valid {
		m.Annotation = annotation.String
	}
	if expectedJSON.Valid {
		m.ExpectedJSON = expectedJSON.String
	}

	return &m, nil
}

// CountByType returns message counts grouped by parser type.
func (d *SQLiteDB) CountByType() (map[string]int, error) {
	counts := make(map[string]int)
	rows, err := d.db.Query("SELECT parser_type, COUNT(*) FROM messages GROUP BY parser_type")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var typ string
		var count int
		if err := rows.Scan(&typ, &count); err != nil {
			return nil, err
		}
		counts[typ] = count
	}
	return counts, rows.Err()
}

// Count returns the total number of messages, optionally filtered by parser type.
func (d *SQLiteDB) Count(parserType string) (int, error) {
	var count int
	var err error
	if parserType != "" {
		err = d.db.QueryRow("SELECT COUNT(*) FROM messages WHERE parser_type = ?", parserType).Scan(&count)
	} else {
		err = d.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
	}
	return count, err
}
