// Package storage provides persistent storage for parsed ACARS messages.
package storage

import (
	"database/sql"
	"encoding/json"
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

// DB wraps a SQLite database connection for message storage.
type DB struct {
	db *sql.DB
}

// Open opens or creates a SQLite database at the given path.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent access.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	// Create schema.
	if err := createSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// createSchema creates the database tables and indices.
func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL,
		label TEXT NOT NULL,
		parser_type TEXT NOT NULL,
		flight TEXT,
		tail TEXT,
		origin TEXT,
		destination TEXT,
		raw_text TEXT NOT NULL,
		parsed_json TEXT NOT NULL,
		missing_fields TEXT,
		confidence REAL,
		created_at TEXT DEFAULT (datetime('now')),
		is_golden INTEGER DEFAULT 0,
		annotation TEXT,
		expected_json TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_messages_parser_type ON messages(parser_type);
	CREATE INDEX IF NOT EXISTS idx_messages_label ON messages(label);
	CREATE INDEX IF NOT EXISTS idx_messages_flight ON messages(flight);
	CREATE INDEX IF NOT EXISTS idx_messages_missing ON messages(missing_fields);
	CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
	-- Note: idx_messages_golden created by migration for existing DBs

	-- FTS5 virtual table for full-text search on raw message text.
	CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
		raw_text,
		content='messages',
		content_rowid='id'
	);

	-- Triggers to keep FTS index in sync.
	CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
		INSERT INTO messages_fts(rowid, raw_text) VALUES (new.id, new.raw_text);
	END;

	CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
		INSERT INTO messages_fts(messages_fts, rowid, raw_text) VALUES('delete', old.id, old.raw_text);
	END;

	CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
		INSERT INTO messages_fts(messages_fts, rowid, raw_text) VALUES('delete', old.id, old.raw_text);
		INSERT INTO messages_fts(rowid, raw_text) VALUES (new.id, new.raw_text);
	END;
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Run migrations for existing databases.
	return migrateSchema(db)
}

// migrateSchema adds new columns to existing databases.
func migrateSchema(db *sql.DB) error {
	// Check if is_golden column exists.
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name='is_golden'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Add new columns for review functionality.
		migrations := []string{
			`ALTER TABLE messages ADD COLUMN is_golden INTEGER DEFAULT 0`,
			`ALTER TABLE messages ADD COLUMN annotation TEXT`,
			`ALTER TABLE messages ADD COLUMN expected_json TEXT`,
		}
		for _, m := range migrations {
			if _, err := db.Exec(m); err != nil {
				// Ignore "duplicate column" errors for idempotency.
				if !strings.Contains(err.Error(), "duplicate column") {
					return err
				}
			}
		}
		// Create index.
		_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_golden ON messages(is_golden)`)
	}

	return nil
}

// InsertParams contains the parameters for inserting a message.
type InsertParams struct {
	Timestamp     string
	Label         string
	ParserType    string
	Flight        string
	Tail          string
	Origin        string
	Destination   string
	RawText       string
	ParsedData    interface{}
	MissingFields []string
	Confidence    float64
}

// Insert stores a parsed message in the database.
func (d *DB) Insert(p InsertParams) (int64, error) {
	parsedJSON, err := json.Marshal(p.ParsedData)
	if err != nil {
		return 0, fmt.Errorf("marshal parsed data: %w", err)
	}

	missingFields := strings.Join(p.MissingFields, ",")

	result, err := d.db.Exec(`
		INSERT INTO messages (timestamp, label, parser_type, flight, tail, origin, destination, raw_text, parsed_json, missing_fields, confidence)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.Timestamp, p.Label, p.ParserType, p.Flight, p.Tail, p.Origin, p.Destination, p.RawText, string(parsedJSON), missingFields, p.Confidence)
	if err != nil {
		return 0, fmt.Errorf("insert message: %w", err)
	}

	return result.LastInsertId()
}

// QueryParams contains filtering options for querying messages.
type QueryParams struct {
	ID            int64    // Filter by specific message ID.
	ParserType    string   // Filter by parser type (exact match).
	Label         string   // Filter by ACARS label (exact match).
	Flight        string   // Filter by flight number (LIKE match).
	MissingField  string   // Filter by specific missing field (LIKE match).
	HasMissing    bool     // Only show messages with any missing fields.
	FullText      string   // FTS5 full-text search on raw_text.
	Limit         int      // Max results (default 100).
	Offset        int      // Pagination offset.
	OrderBy       string   // Sort field (timestamp, parser_type, confidence).
	OrderDesc     bool     // Sort descending.
}

// Query retrieves messages matching the given parameters.
func (d *DB) Query(p QueryParams) ([]Message, error) {
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
	TotalMessages   int
	ByParserType    map[string]int
	ByLabel         map[string]int
	WithMissing     int
	TopMissingFields map[string]int
}

// GetStats returns statistics about the stored messages.
func (d *DB) GetStats() (*Stats, error) {
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
func (d *DB) Distinct(column string) ([]string, error) {
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
func (d *DB) GetByID(id int64) (*Message, error) {
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
func (d *DB) GetByAcarsID(acarsID int64) (*Message, error) {
	// Use SQLite's JSON extraction to find the message_id in parsed_json.
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

// SetGolden marks or unmarks a message as golden.
func (d *DB) SetGolden(id int64, golden bool) error {
	val := 0
	if golden {
		val = 1
	}
	_, err := d.db.Exec(`UPDATE messages SET is_golden = ? WHERE id = ?`, val, id)
	return err
}

// SetAnnotation sets the annotation for a message.
func (d *DB) SetAnnotation(id int64, annotation string) error {
	_, err := d.db.Exec(`UPDATE messages SET annotation = ? WHERE id = ?`, annotation, id)
	return err
}

// SetExpectedJSON sets the expected JSON for a message.
func (d *DB) SetExpectedJSON(id int64, expectedJSON string) error {
	_, err := d.db.Exec(`UPDATE messages SET expected_json = ? WHERE id = ?`, expectedJSON, id)
	return err
}

// UpdateParsedParams contains parameters for updating a parsed message.
type UpdateParsedParams struct {
	ID            int64
	ParserType    string
	ParsedData    interface{}
	MissingFields []string
}

// UpdateParsed updates the parsed result for an existing message.
func (d *DB) UpdateParsed(p UpdateParsedParams) error {
	parsedJSON, err := json.Marshal(p.ParsedData)
	if err != nil {
		return fmt.Errorf("marshal parsed data: %w", err)
	}

	missingFields := strings.Join(p.MissingFields, ",")

	_, err = d.db.Exec(`UPDATE messages SET parser_type = ?, parsed_json = ?, missing_fields = ? WHERE id = ?`,
		p.ParserType, string(parsedJSON), missingFields, p.ID)
	if err != nil {
		return fmt.Errorf("update message: %w", err)
	}

	return nil
}

// GetGoldenMessages retrieves all messages marked as golden.
func (d *DB) GetGoldenMessages() ([]Message, error) {
	return d.Query(QueryParams{
		Limit: 100000,
	})
}

// CountByType returns message counts grouped by parser type.
func (d *DB) CountByType() (map[string]int, error) {
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