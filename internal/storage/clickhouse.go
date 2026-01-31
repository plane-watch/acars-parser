// Package storage provides persistent storage for parsed ACARS messages.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ClickHouseConfig holds ClickHouse connection settings.
type ClickHouseConfig struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
}

// ClickHouseDB wraps a ClickHouse connection for message storage.
type ClickHouseDB struct {
	conn driver.Conn
}

// Conn returns the underlying ClickHouse connection for direct queries.
func (d *ClickHouseDB) Conn() driver.Conn {
	return d.conn
}

// OpenClickHouse opens a connection to ClickHouse.
func OpenClickHouse(ctx context.Context, cfg ClickHouseConfig) (*ClickHouseDB, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:     10 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}

	// Test the connection.
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	return &ClickHouseDB{conn: conn}, nil
}

// Close closes the ClickHouse connection.
func (d *ClickHouseDB) Close() error {
	return d.conn.Close()
}

// CreateSchema creates the ClickHouse tables.
func (d *ClickHouseDB) CreateSchema(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id              UInt64,
			timestamp       DateTime64(3),
			label           LowCardinality(String),
			parser_type     LowCardinality(String),
			flight          LowCardinality(String),
			tail            LowCardinality(String),
			origin          LowCardinality(String),
			destination     LowCardinality(String),
			raw_text        String,
			parsed_json     String,
			missing_fields  String,
			confidence      Float32,
			created_at      DateTime64(3) DEFAULT now64(3)
		)
		ENGINE = MergeTree()
		PARTITION BY toYYYYMM(timestamp)
		ORDER BY (parser_type, label, timestamp, id)
		SETTINGS index_granularity = 8192`,

		`CREATE TABLE IF NOT EXISTS atis_history (
			id              UInt64,
			airport_icao    LowCardinality(String),
			letter          LowCardinality(String),
			atis_type       LowCardinality(Nullable(String)),
			atis_time       String,
			raw_text        String,
			runways         String,
			approaches      String,
			wind            String,
			visibility      String,
			clouds          String,
			temperature     String,
			dew_point       String,
			qnh             String,
			remarks         String,
			recorded_at     DateTime64(3) DEFAULT now64(3)
		)
		ENGINE = MergeTree()
		PARTITION BY toYYYYMM(recorded_at)
		ORDER BY (airport_icao, recorded_at, id)`,
	}

	for _, q := range queries {
		if err := d.conn.Exec(ctx, q); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}

	// Add bloom filter index for full-text search (ignore error if already exists).
	_ = d.conn.Exec(ctx, `ALTER TABLE messages ADD INDEX IF NOT EXISTS idx_raw_text_bloom raw_text TYPE tokenbf_v1(32768, 3, 0) GRANULARITY 1`)

	return nil
}

// CHMessage represents a message stored in ClickHouse.
type CHMessage struct {
	ID            uint64
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
	Confidence    float32
	CreatedAt     time.Time
}

// CHInsertParams contains parameters for inserting a message.
type CHInsertParams struct {
	ID            uint64
	Timestamp     time.Time
	Label         string
	ParserType    string
	Flight        string
	Tail          string
	Origin        string
	Destination   string
	RawText       string
	ParsedData    interface{}
	MissingFields []string
	Confidence    float32
}

// Insert stores a single message in ClickHouse.
func (d *ClickHouseDB) Insert(ctx context.Context, p CHInsertParams) error {
	parsedJSON, err := json.Marshal(p.ParsedData)
	if err != nil {
		return fmt.Errorf("marshal parsed data: %w", err)
	}

	missingFields := strings.Join(p.MissingFields, ",")

	err = d.conn.Exec(ctx, `
		INSERT INTO messages (id, timestamp, label, parser_type, flight, tail, origin, destination, raw_text, parsed_json, missing_fields, confidence)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.Timestamp, p.Label, p.ParserType, p.Flight, p.Tail, p.Origin, p.Destination, p.RawText, string(parsedJSON), missingFields, p.Confidence)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	return nil
}

// InsertBatch stores multiple messages in ClickHouse efficiently.
func (d *ClickHouseDB) InsertBatch(ctx context.Context, messages []CHInsertParams) error {
	if len(messages) == 0 {
		return nil
	}

	batch, err := d.conn.PrepareBatch(ctx, `
		INSERT INTO messages (id, timestamp, label, parser_type, flight, tail, origin, destination, raw_text, parsed_json, missing_fields, confidence)
	`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, p := range messages {
		parsedJSON, err := json.Marshal(p.ParsedData)
		if err != nil {
			return fmt.Errorf("marshal parsed data: %w", err)
		}
		missingFields := strings.Join(p.MissingFields, ",")

		err = batch.Append(p.ID, p.Timestamp, p.Label, p.ParserType, p.Flight, p.Tail, p.Origin, p.Destination, p.RawText, string(parsedJSON), missingFields, p.Confidence)
		if err != nil {
			return fmt.Errorf("append to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	return nil
}

// CHQueryParams contains filtering options for querying messages.
type CHQueryParams struct {
	ID           uint64
	ParserType   string
	Label        string
	Flight       string
	HasMissing   bool
	FullText     string // LIKE match on raw_text.
	Limit        int
	Offset       int
	OrderBy      string
	OrderDesc    bool
}

// Query retrieves messages matching the given parameters.
func (d *ClickHouseDB) Query(ctx context.Context, p CHQueryParams) ([]CHMessage, error) {
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
	if p.HasMissing {
		conditions = append(conditions, "missing_fields != ''")
	}
	if p.FullText != "" {
		conditions = append(conditions, "raw_text LIKE ?")
		args = append(args, "%"+p.FullText+"%")
	}

	query := `SELECT id, timestamp, label, parser_type, flight, tail, origin, destination, raw_text, parsed_json, missing_fields, confidence, created_at FROM messages`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
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

	rows, err := d.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []CHMessage
	for rows.Next() {
		var m CHMessage
		err := rows.Scan(&m.ID, &m.Timestamp, &m.Label, &m.ParserType, &m.Flight, &m.Tail,
			&m.Origin, &m.Destination, &m.RawText, &m.ParsedJSON, &m.MissingFields, &m.Confidence, &m.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		messages = append(messages, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return messages, nil
}

// GetByID retrieves a single message by ID.
func (d *ClickHouseDB) GetByID(ctx context.Context, id uint64) (*CHMessage, error) {
	messages, err := d.Query(ctx, CHQueryParams{ID: id, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, nil
	}
	return &messages[0], nil
}

// CHStats contains aggregate statistics about stored messages.
type CHStats struct {
	TotalMessages    uint64
	ByParserType     map[string]uint64
	ByLabel          map[string]uint64
	WithMissing      uint64
	TopMissingFields map[string]uint64
}

// GetStats returns statistics about stored messages.
func (d *ClickHouseDB) GetStats(ctx context.Context) (*CHStats, error) {
	stats := &CHStats{
		ByParserType:     make(map[string]uint64),
		ByLabel:          make(map[string]uint64),
		TopMissingFields: make(map[string]uint64),
	}

	// Total messages.
	row := d.conn.QueryRow(ctx, "SELECT count() FROM messages")
	if err := row.Scan(&stats.TotalMessages); err != nil {
		return nil, err
	}

	// By parser type.
	rows, err := d.conn.Query(ctx, "SELECT parser_type, count() FROM messages GROUP BY parser_type ORDER BY count() DESC")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var typ string
		var count uint64
		if err := rows.Scan(&typ, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan parser type stats: %w", err)
		}
		stats.ByParserType[typ] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate parser type stats: %w", err)
	}
	rows.Close()

	// By label.
	rows, err = d.conn.Query(ctx, "SELECT label, count() FROM messages GROUP BY label ORDER BY count() DESC LIMIT 20")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var label string
		var count uint64
		if err := rows.Scan(&label, &count); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan label stats: %w", err)
		}
		stats.ByLabel[label] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate label stats: %w", err)
	}
	rows.Close()

	// With missing fields.
	row = d.conn.QueryRow(ctx, "SELECT count() FROM messages WHERE missing_fields != ''")
	if err := row.Scan(&stats.WithMissing); err != nil {
		return nil, err
	}

	return stats, nil
}

// Count returns the total number of messages, optionally filtered by parser type.
func (d *ClickHouseDB) Count(ctx context.Context, parserType string) (uint64, error) {
	var count uint64
	var err error
	if parserType != "" {
		row := d.conn.QueryRow(ctx, "SELECT count() FROM messages WHERE parser_type = ?", parserType)
		err = row.Scan(&count)
	} else {
		row := d.conn.QueryRow(ctx, "SELECT count() FROM messages")
		err = row.Scan(&count)
	}
	return count, err
}

// CountByType returns message counts grouped by parser type.
func (d *ClickHouseDB) CountByType(ctx context.Context) (map[string]uint64, error) {
	counts := make(map[string]uint64)
	rows, err := d.conn.Query(ctx, "SELECT parser_type, count() FROM messages GROUP BY parser_type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var typ string
		var count uint64
		if err := rows.Scan(&typ, &count); err != nil {
			return nil, fmt.Errorf("scan count by type: %w", err)
		}
		counts[typ] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate count by type: %w", err)
	}
	return counts, nil
}

// Distinct returns distinct values for a given column.
func (d *ClickHouseDB) Distinct(ctx context.Context, column string) ([]string, error) {
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

	query := fmt.Sprintf("SELECT DISTINCT %s FROM messages WHERE %s != '' ORDER BY %s", column, column, column)
	rows, err := d.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan distinct value: %w", err)
		}
		values = append(values, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate distinct values: %w", err)
	}
	return values, nil
}

// MaxID returns the maximum message ID in the table.
func (d *ClickHouseDB) MaxID(ctx context.Context) (uint64, error) {
	var maxID uint64
	row := d.conn.QueryRow(ctx, "SELECT max(id) FROM messages")
	if err := row.Scan(&maxID); err != nil {
		return 0, err
	}
	return maxID, nil
}
