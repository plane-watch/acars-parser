// Package storage provides persistent storage for parsed ACARS messages.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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
// MessagesTable is the message archive table.
const MessagesTable = "messages"

// tableNameRe matches the table names the message-table functions accept.
// They are interpolated into SQL, so only plain identifiers are allowed.
var tableNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func checkTableName(name string) error {
	if !tableNameRe.MatchString(name) {
		return fmt.Errorf("invalid table name %q", name)
	}
	return nil
}

// messagesDDL returns the CREATE TABLE statement of a messages table.
func messagesDDL(name string) string {
	return `CREATE TABLE IF NOT EXISTS ` + name + ` (
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
			message_number  LowCardinality(String),
			block_id        LowCardinality(String),
			block_end       Nullable(Bool),
			assembled_from  Array(UInt64),
			link_direction  LowCardinality(String),
			created_at      DateTime64(3) DEFAULT now64(3)
		)
		ENGINE = MergeTree()
		PARTITION BY toYYYYMM(timestamp)
		ORDER BY (parser_type, label, timestamp, id)
		SETTINGS index_granularity = 8192`
}

// blockColumns are the definitions of the block columns, which CreateSchema
// adds to an archive created before them; messagesDDL has them too.
var blockColumns = []string{
	"message_number LowCardinality(String) AFTER confidence",
	"block_id LowCardinality(String) AFTER message_number",
	"block_end Nullable(Bool) AFTER block_id",
	"assembled_from Array(UInt64) AFTER block_end",
	"link_direction LowCardinality(String) AFTER assembled_from",
}

// ErrSchemaOutdated is returned by CheckSchema when the messages table lacks
// columns that the code reads and writes.
var ErrSchemaOutdated = errors.New("the messages table is missing columns: update it with " +
	"\"acars_parser migrate -skip-messages -skip-state\" (live also updates it when it starts)")

// CheckSchema returns ErrSchemaOutdated if the messages table lacks the
// block columns. Commands that read the archive call it first, so that they
// fail before changing anything rather than part-way (a rebuild would
// otherwise leave its staging table behind). It does not change the table:
// that is left to CreateSchema, which live and migrate run.
func (d *ClickHouseDB) CheckSchema(ctx context.Context) error {
	var n uint64
	err := d.conn.QueryRow(ctx, `SELECT count() FROM system.columns WHERE database = currentDatabase()
		AND table = ? AND name IN ('message_number', 'block_id', 'block_end', 'assembled_from', 'link_direction')`, MessagesTable).Scan(&n)
	if err != nil {
		return fmt.Errorf("check schema: %w", err)
	}
	if n != uint64(len(blockColumns)) {
		return ErrSchemaOutdated
	}
	return nil
}

// BlockFields are a message's ACARS block and link fields (see
// acars.Message): its message number, block ID and the feed's block_end
// flag (nil when not sent), for a message joined from its blocks the
// blocks' IDs, and the link direction the feed sent ("uplink",
// "downlink" or empty). Parsers need the block ID or link direction to
// tell an uplink from a downlink, so a rebuild must have them.
type BlockFields struct {
	MessageNumber string
	BlockID       string
	BlockEnd      *bool
	AssembledFrom []uint64
	LinkDirection string
}

// assembledFrom returns the IDs as stored: an empty array, not NULL.
func (b BlockFields) assembledFrom() []uint64 {
	if b.AssembledFrom == nil {
		return []uint64{}
	}
	return b.AssembledFrom
}

func (d *ClickHouseDB) CreateSchema(ctx context.Context) error {
	queries := []string{
		messagesDDL(MessagesTable),

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

	// Add the block columns to an archive created before they existed. The
	// rows already stored have no block fields: they read as empty, NULL
	// and an empty array.
	for _, column := range blockColumns {
		if err := d.conn.Exec(ctx, `ALTER TABLE messages ADD COLUMN IF NOT EXISTS `+column); err != nil {
			return fmt.Errorf("add column: %w", err)
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
	BlockFields
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
	BlockFields
}

// Insert stores a single message in ClickHouse.
func (d *ClickHouseDB) Insert(ctx context.Context, p CHInsertParams) error {
	parsedJSON, err := json.Marshal(p.ParsedData)
	if err != nil {
		return fmt.Errorf("marshal parsed data: %w", err)
	}

	missingFields := strings.Join(p.MissingFields, ",")

	err = d.conn.Exec(ctx, `
		INSERT INTO messages (id, timestamp, label, parser_type, flight, tail, origin, destination, raw_text, parsed_json, missing_fields, confidence,
			message_number, block_id, block_end, assembled_from, link_direction)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.Timestamp, p.Label, p.ParserType, p.Flight, p.Tail, p.Origin, p.Destination, p.RawText, string(parsedJSON), missingFields, p.Confidence,
		p.MessageNumber, p.BlockID, p.BlockEnd, p.assembledFrom(), p.LinkDirection)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	return nil
}

// InsertBatch stores multiple messages in ClickHouse efficiently.
func (d *ClickHouseDB) InsertBatch(ctx context.Context, messages []CHInsertParams) error {
	return d.InsertBatchInto(ctx, MessagesTable, messages)
}

// InsertBatchInto stores multiple messages in the named messages table.
func (d *ClickHouseDB) InsertBatchInto(ctx context.Context, table string, messages []CHInsertParams) error {
	if len(messages) == 0 {
		return nil
	}
	if err := checkTableName(table); err != nil {
		return err
	}

	batch, err := d.conn.PrepareBatch(ctx, `
		INSERT INTO `+table+` (id, timestamp, label, parser_type, flight, tail, origin, destination, raw_text, parsed_json, missing_fields, confidence,
			message_number, block_id, block_end, assembled_from, link_direction)
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

		err = batch.Append(p.ID, p.Timestamp, p.Label, p.ParserType, p.Flight, p.Tail, p.Origin, p.Destination, p.RawText, string(parsedJSON), missingFields, p.Confidence,
			p.MessageNumber, p.BlockID, p.BlockEnd, p.assembledFrom(), p.LinkDirection)
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
	ID         uint64
	ParserType string
	Label      string
	Flight     string
	HasMissing bool
	FullText   string // LIKE match on raw_text.
	Limit      int
	Offset     int
	OrderBy    string
	OrderDesc  bool
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

	query := `SELECT ` + messageColumns + ` FROM messages`
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
	if orderField != "id" {
		// The ID breaks ties, so that the order is stable across pages
		// and a message's rows (one per parse result) stay together.
		query += " , id " + direction
	}

	// Limit and offset.
	limit := 100
	if p.Limit > 0 {
		limit = p.Limit
	}
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, p.Offset)

	return d.queryMessages(ctx, query, args...)
}

// messageColumns are the columns of a CHMessage, in the order
// queryMessages scans them.
const messageColumns = `id, timestamp, label, parser_type, flight, tail, origin, destination, raw_text, parsed_json, missing_fields, confidence, created_at,
	message_number, block_id, block_end, assembled_from, link_direction`

// queryMessages runs a query selecting messageColumns and returns its rows.
func (d *ClickHouseDB) queryMessages(ctx context.Context, query string, args ...interface{}) ([]CHMessage, error) {
	rows, err := d.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []CHMessage
	for rows.Next() {
		var m CHMessage
		err := rows.Scan(&m.ID, &m.Timestamp, &m.Label, &m.ParserType, &m.Flight, &m.Tail,
			&m.Origin, &m.Destination, &m.RawText, &m.ParsedJSON, &m.MissingFields, &m.Confidence, &m.CreatedAt,
			&m.MessageNumber, &m.BlockID, &m.BlockEnd, &m.AssembledFrom, &m.LinkDirection)
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

// MessageKey is a message's place in time order: its timestamp, then its
// ID. The zero key is before every message.
type MessageKey struct {
	Timestamp time.Time
	ID        uint64
}

// ServerTime returns the ClickHouse server's current time, the clock that
// fills created_at.
func (d *ClickHouseDB) ServerTime(ctx context.Context) (time.Time, error) {
	var t time.Time
	err := d.conn.QueryRow(ctx, "SELECT now64(3)").Scan(&t)
	return t, err
}

// MessagesAfter returns the rows of the messages after the key, in time
// order, with parserType's rows only when it is set, and only rows stored
// (created_at) before insertedBefore when it is not zero: whole messages (all
// of a message's rows, one per parse result), as many as fit in limit rows
// (or the first message's, if it alone has more), and the key of the last.
// It returns no rows after the last message.
//
// Paging by key, unlike by offset, neither repeats nor skips a message
// when rows are inserted meanwhile: a row inserted before the key (such as
// a message live joins from its blocks, which carries its first block's
// time) is not read.
func (d *ClickHouseDB) MessagesAfter(ctx context.Context, after MessageKey, parserType string, insertedBefore time.Time, limit int) ([]CHMessage, MessageKey, error) {
	limit = max(limit, 1)
	var conditions []string
	var args []interface{}
	if !insertedBefore.IsZero() {
		// Bound as text at millisecond precision, as the key is.
		conditions = append(conditions, "created_at < toDateTime64(?, 3, 'UTC')")
		args = append(args, insertedBefore.UTC().Format("2006-01-02 15:04:05.000"))
	}
	if after != (MessageKey{}) {
		// The key's time is bound as text at the column's millisecond
		// precision: a time.Time parameter is bound at whole seconds, which
		// would read the key's message again.
		conditions = append(conditions, "(timestamp, id) > (toDateTime64(?, 3, 'UTC'), ?)")
		args = append(args, after.Timestamp.UTC().Format("2006-01-02 15:04:05.000"), after.ID)
	}
	if parserType != "" {
		conditions = append(conditions, "parser_type = ?")
		args = append(args, parserType)
	}
	query := `SELECT ` + messageColumns + ` FROM messages`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY timestamp, id LIMIT %d"

	for n := limit; ; n *= 2 {
		rows, err := d.queryMessages(ctx, fmt.Sprintf(query, n), args...)
		if err != nil || len(rows) == 0 {
			return nil, after, err
		}
		if end := pageEnd(rows, len(rows) < n, limit); end > 0 {
			m := rows[end-1]
			return rows[:end], MessageKey{m.Timestamp, m.ID}, nil
		}
		// The first message has more rows than n: read more.
	}
}

// pageEnd returns how many of rows (in time order, read with a limit that
// all reached unless atEnd) form whole messages within limit rows, or the
// first message alone if it has more; or 0 if the first message may
// continue past rows. A message is the rows sharing an ID and timestamp.
func pageEnd(rows []CHMessage, atEnd bool, limit int) int {
	same := func(a, b CHMessage) bool { return a.ID == b.ID && a.Timestamp.Equal(b.Timestamp) }
	// The rows of whole messages: the last message's may continue past
	// rows unless they reached the end.
	whole := len(rows)
	if !atEnd {
		for whole > 0 && same(rows[whole-1], rows[len(rows)-1]) {
			whole--
		}
	}
	end := 0
	for end < whole {
		next := end + 1
		for next < whole && same(rows[next], rows[end]) {
			next++
		}
		if end > 0 && next > limit {
			break
		}
		end = next
	}
	return end
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

// CreateMessagesTable creates an empty messages table with the given name,
// laid out as the message archive.
func (d *ClickHouseDB) CreateMessagesTable(ctx context.Context, name string) error {
	if err := checkTableName(name); err != nil {
		return err
	}
	return d.conn.Exec(ctx, messagesDDL(name))
}

// TableExists reports whether a table exists in the connection's database.
func (d *ClickHouseDB) TableExists(ctx context.Context, name string) (bool, error) {
	var n uint64
	err := d.conn.QueryRow(ctx, `SELECT count() FROM system.tables WHERE database = currentDatabase() AND name = ?`, name).Scan(&n)
	return n > 0, err
}

// TableUUID returns a table's UUID, which stays with the table when it is
// renamed or exchanged ("" if the table does not exist).
func (d *ClickHouseDB) TableUUID(ctx context.Context, name string) (string, error) {
	var uuid string
	err := d.conn.QueryRow(ctx, `SELECT toString(uuid) FROM system.tables WHERE database = currentDatabase() AND name = ?`, name).Scan(&uuid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return uuid, err
}

// DropTable drops a table if it exists.
func (d *ClickHouseDB) DropTable(ctx context.Context, name string) error {
	if err := checkTableName(name); err != nil {
		return err
	}
	return d.conn.Exec(ctx, `DROP TABLE IF EXISTS `+name)
}

// ExchangeTables atomically swaps the names of two tables.
func (d *ClickHouseDB) ExchangeTables(ctx context.Context, a, b string) error {
	if err := checkTableName(a); err != nil {
		return err
	}
	if err := checkTableName(b); err != nil {
		return err
	}
	return d.conn.Exec(ctx, `EXCHANGE TABLES `+a+` AND `+b)
}

// RenameTable renames a table.
func (d *ClickHouseDB) RenameTable(ctx context.Context, from, to string) error {
	if err := checkTableName(from); err != nil {
		return err
	}
	if err := checkTableName(to); err != nil {
		return err
	}
	return d.conn.Exec(ctx, `RENAME TABLE `+from+` TO `+to)
}

// CountDistinctIDs returns the number of distinct message IDs in a messages
// table (a message has one row per parse result).
func (d *ClickHouseDB) CountDistinctIDs(ctx context.Context, table string) (uint64, error) {
	if err := checkTableName(table); err != nil {
		return 0, err
	}
	var n uint64
	err := d.conn.QueryRow(clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{"max_execution_time": 0})),
		`SELECT uniqExact(id) FROM `+table).Scan(&n)
	return n, err
}

// RawMessage is a stored message's transmitted content and its block and
// link fields, with the direction its parse results recorded, but not the
// results themselves.
type RawMessage struct {
	ID        uint64
	Timestamp time.Time
	Label     string
	Tail      string
	Flight    string
	RawText   string
	BlockFields

	// StoredDirection is the "direction" a parse result stored for the
	// message recorded ("uplink" or "downlink"; empty if none did). Rows
	// stored before the block ID and link direction were have neither, but
	// the CPDLC result recorded the direction live worked out from them.
	StoredDirection string
}

// StreamRawMessages calls fn once for each distinct message ID in a
// messages table, in ID order, with the message's transmitted content
// (taken from one of its rows; they hold the same content). The IDs are
// read first (8 bytes each: about 96 MB for 12 million messages), then the
// messages in ranges of chunk IDs, one query per range, so that no query
// stays open for long (on the 11.9M-message archive, each took about 0.2 s).
// fn's error stops the stream and is returned.
func (d *ClickHouseDB) StreamRawMessages(ctx context.Context, table string, chunk int, fn func(RawMessage) error) error {
	if err := checkTableName(table); err != nil {
		return err
	}
	if chunk <= 0 {
		return fmt.Errorf("chunk size %d: must be positive", chunk)
	}
	qctx := clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{"max_execution_time": 0}))

	var ids []uint64
	rows, err := d.conn.Query(qctx, `SELECT DISTINCT id FROM `+table+` ORDER BY id`)
	if err != nil {
		return fmt.Errorf("query message ids: %w", err)
	}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan message id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()

	for start := 0; start < len(ids); start += chunk {
		end := min(start+chunk, len(ids)) - 1
		if err := d.streamRange(qctx, table, ids[start], ids[end], fn); err != nil {
			return err
		}
	}
	return nil
}

// streamRange calls fn for each distinct message with an ID from lo to hi.
// The rows of a message hold the same content; should rows of one ID hold
// different text, the first text in order is streamed, and its fields and
// stored direction are taken only from the rows with that text.
func (d *ClickHouseDB) streamRange(ctx context.Context, table string, lo, hi uint64, fn func(RawMessage) error) error {
	rows, err := d.conn.Query(ctx,
		`SELECT id, any(timestamp), any(label), any(tail), any(flight), raw_text,
			any(message_number), any(block_id), any(block_end), any(assembled_from), any(link_direction),
			anyIf(JSONExtractString(parsed_json, 'direction'), JSONExtractString(parsed_json, 'direction') IN ('uplink', 'downlink'))
		FROM `+table+` WHERE id >= ? AND id <= ?
		GROUP BY id, raw_text ORDER BY id, raw_text LIMIT 1 BY id`, lo, hi)
	if err != nil {
		return fmt.Errorf("query messages %d to %d: %w", lo, hi, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var m RawMessage
		if err := rows.Scan(&m.ID, &m.Timestamp, &m.Label, &m.Tail, &m.Flight, &m.RawText,
			&m.MessageNumber, &m.BlockID, &m.BlockEnd, &m.AssembledFrom, &m.LinkDirection, &m.StoredDirection); err != nil {
			return fmt.Errorf("scan message: %w", err)
		}
		if err := fn(m); err != nil {
			return err
		}
	}
	return rows.Err()
}

// ErrSwapRename is returned by SwapInTable when the swap itself succeeded
// but the rename of the old table was not confirmed: the old table is under
// the staging name or, if the rename happened and only its reply was lost,
// the previous name.
var ErrSwapRename = errors.New("tables exchanged, but the rename of the old table was not confirmed")

// SwapInTable replaces the current table with the staging table and keeps
// the current one as previous. The swap is an EXCHANGE TABLES, which is
// atomic (a multi-table RENAME is not: ClickHouse can apply part of it);
// the old table, now under the staging name, is then renamed to previous.
// If that rename is not confirmed, the error wraps ErrSwapRename: the
// current name holds the new table, and the old one is under the staging
// or the previous name.
func (d *ClickHouseDB) SwapInTable(ctx context.Context, current, staging, previous string) error {
	for _, name := range []string{current, staging, previous} {
		if err := checkTableName(name); err != nil {
			return err
		}
	}
	if err := d.conn.Exec(ctx, `EXCHANGE TABLES `+current+` AND `+staging); err != nil {
		return fmt.Errorf("exchange %s and %s: %w", current, staging, err)
	}
	if err := d.conn.Exec(ctx, `RENAME TABLE `+staging+` TO `+previous); err != nil {
		return fmt.Errorf("%w: rename %s to %s: %v", ErrSwapRename, staging, previous, err)
	}
	return nil
}
