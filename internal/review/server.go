// Package review provides a web UI for reviewing and annotating parsed messages.
package review

import (
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
	db     *storage.DB
	port   int
	filter string // Optional parser type filter
}

// NewServer creates a new review server.
func NewServer(db *storage.DB, port int, filter string) *Server {
	return &Server{
		db:     db,
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

func messageToAPI(m *storage.Message) APIMessage {
	api := APIMessage{
		ID:          m.ID,
		Timestamp:   m.Timestamp.Format("2006-01-02 15:04:05"),
		Label:       m.Label,
		ParserType:  m.ParserType,
		Flight:      m.Flight,
		Tail:        m.Tail,
		Origin:      m.Origin,
		Destination: m.Destination,
		RawText:     m.RawText,
		Confidence:  m.Confidence,
		IsGolden:    m.IsGolden,
		Annotation:  m.Annotation,
	}

	// Parse missing fields.
	if m.MissingFields != "" {
		api.MissingFields = strings.Split(m.MissingFields, ",")
	}

	// Parse JSON fields.
	if m.ParsedJSON != "" {
		_ = json.Unmarshal([]byte(m.ParsedJSON), &api.Parsed)
	}
	if m.ExpectedJSON != "" {
		_ = json.Unmarshal([]byte(m.ExpectedJSON), &api.Expected)
	}

	return api
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters.
	q := r.URL.Query()
	params := storage.QueryParams{
		ParserType:   q.Get("type"),
		Label:        q.Get("label"),
		MissingField: q.Get("missing"),
		HasMissing:   q.Get("has_missing") == "true",
		FullText:     q.Get("search"),
		OrderBy:      q.Get("order"),
		OrderDesc:    q.Get("desc") != "false",
	}

	// Apply server-level filter.
	if s.filter != "" && params.ParserType == "" {
		params.ParserType = s.filter
	}

	// Golden filter.
	// Note: We'd need to add this to QueryParams, but for now we'll filter in code.
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

	messages, err := s.db.Query(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to API format.
	var result []APIMessage
	for _, m := range messages {
		if goldenOnly && !m.IsGolden {
			continue
		}
		result = append(result, messageToAPI(&m))
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
	msg, err := s.db.GetByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if msg == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(messageToAPI(msg))
}

func (s *Server) setGolden(w http.ResponseWriter, r *http.Request, id int64) {
	var req struct {
		Golden bool `json:"golden"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.db.SetGolden(id, req.Golden); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) setAnnotation(w http.ResponseWriter, r *http.Request, id int64) {
	var req struct {
		Annotation string `json:"annotation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.db.SetAnnotation(id, req.Annotation); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) setExpected(w http.ResponseWriter, r *http.Request, id int64) {
	var req struct {
		Expected map[string]interface{} `json:"expected"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	expectedJSON, err := json.Marshal(req.Expected)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.db.SetExpectedJSON(id, string(expectedJSON)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := s.db.GetStats()
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

	types, err := s.db.Distinct("parser_type")
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

	// Get all golden messages.
	messages, err := s.db.Query(storage.QueryParams{Limit: 100000})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var exports []GoldenExport
	for _, m := range messages {
		if !m.IsGolden {
			continue
		}

		export := GoldenExport{
			ID:         m.ID,
			RawText:    m.RawText,
			Label:      m.Label,
			ParserType: m.ParserType,
			Annotation: m.Annotation,
		}

		// Use expected_json if set, otherwise use parsed_json.
		if m.ExpectedJSON != "" {
			_ = json.Unmarshal([]byte(m.ExpectedJSON), &export.Expected)
		} else if m.ParsedJSON != "" {
			_ = json.Unmarshal([]byte(m.ParsedJSON), &export.Expected)
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

	// Get all golden messages.
	messages, err := s.db.Query(storage.QueryParams{Limit: 100000})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Group by parser type.
	byType := make(map[string][]storage.Message)
	for _, m := range messages {
		if !m.IsGolden {
			continue
		}
		byType[m.ParserType] = append(byType[m.ParserType], m)
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
