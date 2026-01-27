// Package review provides a web UI for reviewing and annotating parsed messages.
package review

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"

	"acars_parser/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

// Server provides the review web UI.
type Server struct {
	ch     *storage.ClickHouseDB
	pg     *storage.PostgresDB
	port   int
	filter string // Optional parser type filter.
}

// NewServer creates a new review server.
func NewServer(ch *storage.ClickHouseDB, pg *storage.PostgresDB, port int, filter string) *Server {
	return &Server{
		ch:     ch,
		pg:     pg,
		port:   port,
		filter: filter,
	}
}

// Run starts the HTTP server.
func (s *Server) Run() error {
	mux := http.NewServeMux()

	// API routes.
	mux.HandleFunc("/api/messages", s.handleMessages)
	mux.HandleFunc("/api/messages/", s.handleMessage) // /api/messages/{id}
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/types", s.handleTypes)
	mux.HandleFunc("/api/export/json", s.handleExportJSON)
	mux.HandleFunc("/api/export/go", s.handleExportGo)

	// Static files.
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("embed static files: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Review UI starting at http://localhost%s", addr)
	if s.filter != "" {
		log.Printf("Filtering to parser type: %s", s.filter)
	}

	return http.ListenAndServe(addr, mux)
}

// APIMessage is the JSON representation of a message.
type APIMessage struct {
	ID            int64             `json:"id"`
	Timestamp     string            `json:"timestamp"`
	Label         string            `json:"label"`
	ParserType    string            `json:"parser_type"`
	Flight        string            `json:"flight"`
	Tail          string            `json:"tail"`
	Origin        string            `json:"origin"`
	Destination   string            `json:"destination"`
	RawText       string            `json:"raw_text"`
	Parsed        map[string]interface{} `json:"parsed"`
	MissingFields []string          `json:"missing_fields"`
	Confidence    float64           `json:"confidence"`
	IsGolden      bool              `json:"is_golden"`
	Annotation    string            `json:"annotation"`
	Expected      map[string]interface{} `json:"expected,omitempty"`
}

func messageToAPI(m *storage.CHMessage, annotation *storage.GoldenAnnotation) APIMessage {
	api := APIMessage{
		ID:          int64(m.ID),
		Timestamp:   m.Timestamp.Format("2006-01-02 15:04:05"),
		Label:       m.Label,
		ParserType:  m.ParserType,
		Flight:      m.Flight,
		Tail:        m.Tail,
		Origin:      m.Origin,
		Destination: m.Destination,
		RawText:     m.RawText,
		Confidence:  float64(m.Confidence),
	}

	// Parse missing fields.
	if m.MissingFields != "" {
		api.MissingFields = strings.Split(m.MissingFields, ",")
	}

	// Parse JSON fields.
	if m.ParsedJSON != "" {
		_ = json.Unmarshal([]byte(m.ParsedJSON), &api.Parsed)
	}

	// Add annotation data if available.
	if annotation != nil {
		api.IsGolden = annotation.IsGolden
		api.Annotation = annotation.Annotation
		api.Expected = annotation.ExpectedJSON
	}

	return api
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()

	// Parse query parameters.
	q := r.URL.Query()
	params := storage.CHQueryParams{
		ParserType: q.Get("type"),
		Label:      q.Get("label"),
		HasMissing: q.Get("has_missing") == "true",
		FullText:   q.Get("search"),
		OrderBy:    q.Get("order"),
		OrderDesc:  q.Get("desc") != "false",
	}

	// Apply server-level filter.
	if s.filter != "" && params.ParserType == "" {
		params.ParserType = s.filter
	}

	// Golden filter.
	goldenOnly := q.Get("golden") == "true"

	// Pagination.
	if limit, err := strconv.Atoi(q.Get("limit")); err == nil && limit > 0 {
		params.Limit = limit
	} else {
		params.Limit = 50
	}
	if offset, err := strconv.Atoi(q.Get("offset")); err == nil {
		params.Offset = offset
	}

	messages, err := s.ch.Query(ctx, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to API format.
	var result []APIMessage
	for _, m := range messages {
		// Fetch annotation from PostgreSQL if available.
		var annotation *storage.GoldenAnnotation
		if s.pg != nil {
			annotation, _ = s.pg.GetGoldenAnnotation(ctx, int64(m.ID))
		}

		if goldenOnly && (annotation == nil || !annotation.IsGolden) {
			continue
		}
		result = append(result, messageToAPI(&m, annotation))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /api/messages/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/messages/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing message ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getMessage(w, id)
	case http.MethodPost, http.MethodPatch:
		// Check for sub-action.
		if len(parts) > 1 {
			switch parts[1] {
			case "golden":
				s.setGolden(w, r, id)
			case "annotation":
				s.setAnnotation(w, r, id)
			case "expected":
				s.setExpected(w, r, id)
			default:
				http.Error(w, "Unknown action", http.StatusBadRequest)
			}
		} else {
			http.Error(w, "No action specified", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getMessage(w http.ResponseWriter, id int64) {
	ctx := context.Background()

	msg, err := s.ch.GetByID(ctx, uint64(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if msg == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Fetch annotation from PostgreSQL.
	var annotation *storage.GoldenAnnotation
	if s.pg != nil {
		annotation, _ = s.pg.GetGoldenAnnotation(ctx, id)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(messageToAPI(msg, annotation))
}

func (s *Server) setGolden(w http.ResponseWriter, r *http.Request, id int64) {
	if s.pg == nil {
		http.Error(w, "PostgreSQL not configured", http.StatusServiceUnavailable)
		return
	}

	ctx := context.Background()

	var body struct {
		Golden bool `json:"golden"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := s.pg.SetGolden(ctx, id, body.Golden); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) setAnnotation(w http.ResponseWriter, r *http.Request, id int64) {
	if s.pg == nil {
		http.Error(w, "PostgreSQL not configured", http.StatusServiceUnavailable)
		return
	}

	ctx := context.Background()

	var body struct {
		Annotation string `json:"annotation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := s.pg.SetAnnotation(ctx, id, body.Annotation); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) setExpected(w http.ResponseWriter, r *http.Request, id int64) {
	if s.pg == nil {
		http.Error(w, "PostgreSQL not configured", http.StatusServiceUnavailable)
		return
	}

	ctx := context.Background()

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get existing annotation or create new one.
	existing, _ := s.pg.GetGoldenAnnotation(ctx, id)
	annotation := storage.GoldenAnnotation{
		MessageID:    id,
		ExpectedJSON: body,
	}
	if existing != nil {
		annotation.IsGolden = existing.IsGolden
		annotation.Annotation = existing.Annotation
	}

	if err := s.pg.UpsertGoldenAnnotation(ctx, annotation); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	stats, err := s.ch.GetStats(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()
	types, err := s.ch.Distinct(ctx, "parser_type")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(types)
}

// GoldenExport represents a golden message for export.
type GoldenExport struct {
	ID         int64                  `json:"id"`
	RawText    string                 `json:"raw_text"`
	Label      string                 `json:"label"`
	ParserType string                 `json:"parser_type"`
	Expected   map[string]interface{} `json:"expected"`
	Annotation string                 `json:"annotation,omitempty"`
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()

	// Get all golden annotations from PostgreSQL.
	if s.pg == nil {
		http.Error(w, "PostgreSQL not configured", http.StatusServiceUnavailable)
		return
	}

	annotations, err := s.pg.GetGoldenMessages(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var exports []GoldenExport
	for _, a := range annotations {
		// Fetch message from ClickHouse.
		msg, err := s.ch.GetByID(ctx, uint64(a.MessageID))
		if err != nil || msg == nil {
			continue
		}

		export := GoldenExport{
			ID:         a.MessageID,
			RawText:    msg.RawText,
			Label:      msg.Label,
			ParserType: msg.ParserType,
			Annotation: a.Annotation,
		}

		// Use expected_json if set, otherwise use parsed_json.
		if a.ExpectedJSON != nil && len(a.ExpectedJSON) > 0 {
			export.Expected = a.ExpectedJSON
		} else if msg.ParsedJSON != "" {
			_ = json.Unmarshal([]byte(msg.ParsedJSON), &export.Expected)
		}

		exports = append(exports, export)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=golden_messages.json")
	_ = json.NewEncoder(w).Encode(exports)
}

func (s *Server) handleExportGo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()

	// Get all golden annotations from PostgreSQL.
	if s.pg == nil {
		http.Error(w, "PostgreSQL not configured", http.StatusServiceUnavailable)
		return
	}

	annotations, err := s.pg.GetGoldenMessages(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Group by parser type.
	type goldenMsg struct {
		ID      int64
		RawText string
		Label   string
		Flight  string
	}
	byType := make(map[string][]goldenMsg)
	for _, a := range annotations {
		// Fetch message from ClickHouse.
		msg, err := s.ch.GetByID(ctx, uint64(a.MessageID))
		if err != nil || msg == nil {
			continue
		}
		byType[msg.ParserType] = append(byType[msg.ParserType], goldenMsg{
			ID:      a.MessageID,
			RawText: msg.RawText,
			Label:   msg.Label,
			Flight:  msg.Flight,
		})
	}

	// Generate Go test code.
	var code strings.Builder
	code.WriteString("// Code generated from golden messages. DO NOT EDIT.\n\n")
	code.WriteString("package parsers_test\n\n")
	code.WriteString("import (\n")
	code.WriteString("\t\"testing\"\n\n")
	code.WriteString("\t\"acars_parser/internal/acars\"\n")
	code.WriteString("\t\"acars_parser/internal/registry\"\n")
	code.WriteString(")\n\n")

	for parserType, msgs := range byType {
		code.WriteString(fmt.Sprintf("func TestGolden_%s(t *testing.T) {\n", strings.ReplaceAll(parserType, "_", "")))
		code.WriteString("\treg := registry.Default()\n")
		code.WriteString("\treg.Sort()\n\n")
		code.WriteString("\tcases := []struct {\n")
		code.WriteString("\t\tname    string\n")
		code.WriteString("\t\traw     string\n")
		code.WriteString("\t\tlabel   string\n")
		code.WriteString("\t\twantType string\n")
		code.WriteString("\t}{\n")

		for _, m := range msgs {
			name := fmt.Sprintf("msg_%d", m.ID)
			if m.Flight != "" {
				name = m.Flight
			}
			// Escape backticks in raw text.
			rawText := strings.ReplaceAll(m.RawText, "`", "` + \"`\" + `")
			code.WriteString(fmt.Sprintf("\t\t{%q, `%s`, %q, %q},\n", name, rawText, m.Label, parserType))
		}

		code.WriteString("\t}\n\n")
		code.WriteString("\tfor _, tc := range cases {\n")
		code.WriteString("\t\tt.Run(tc.name, func(t *testing.T) {\n")
		code.WriteString("\t\t\tmsg := &acars.Message{Label: tc.label, Text: tc.raw}\n")
		code.WriteString("\t\t\tresults := reg.Dispatch(msg)\n")
		code.WriteString("\t\t\tif len(results) == 0 {\n")
		code.WriteString("\t\t\t\tt.Errorf(\"expected parser match, got none\")\n")
		code.WriteString("\t\t\t\treturn\n")
		code.WriteString("\t\t\t}\n")
		code.WriteString("\t\t\tif results[0].Type() != tc.wantType {\n")
		code.WriteString("\t\t\t\tt.Errorf(\"got type %q, want %q\", results[0].Type(), tc.wantType)\n")
		code.WriteString("\t\t\t}\n")
		code.WriteString("\t\t})\n")
		code.WriteString("\t}\n")
		code.WriteString("}\n\n")
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=golden_test.go")
	_, _ = w.Write([]byte(code.String()))
}
