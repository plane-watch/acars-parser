// Package main provides a corpus analyzer for ACARS messages.
// It analyzes message distribution, parsing coverage, and format patterns.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := flag.String("db", "messages.db", "SQLite database file")
	outputFormat := flag.String("format", "text", "Output format: text, json")
	showTemplates := flag.Bool("templates", false, "Include template analysis (slower)")
	topN := flag.Int("top", 20, "Show top N items in each category")
	label := flag.String("label", "", "Analyze specific label only")
	suggest := flag.Bool("suggest", false, "Generate pattern suggestions for a label (requires -label)")
	minCluster := flag.Int("min-cluster", 3, "Minimum cluster size for suggestions")
	testPattern := flag.String("test", "", "Test a regex pattern against the corpus")

	flag.Parse()

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Pattern testing mode.
	if *testPattern != "" {
		if *label == "" {
			fmt.Fprintf(os.Stderr, "Error: -test requires -label\n")
			os.Exit(1)
		}
		matches, total, matchIDs, nonMatchIDs := TestPattern(db, *testPattern, *label)
		fmt.Printf("Pattern: %s\n", *testPattern)
		fmt.Printf("Label: %s\n", *label)
		fmt.Printf("Result: %d/%d match (%.1f%%)\n\n", matches, total, float64(matches)/float64(total)*100)

		if len(matchIDs) > 0 {
			fmt.Printf("Sample matches: %v\n", matchIDs)
		}
		if len(nonMatchIDs) > 0 {
			fmt.Printf("Sample non-matches: %v\n", nonMatchIDs)
		}
		return
	}

	// Suggestion mode.
	if *suggest {
		if *label == "" {
			fmt.Fprintf(os.Stderr, "Error: -suggest requires -label\n")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Generating pattern suggestions for label %s...\n", *label)
		suggestions := SuggestPatterns(db, *label, *minCluster, *topN)

		if *outputFormat == "json" {
			data, _ := json.MarshalIndent(suggestions, "", "  ")
			fmt.Println(string(data))
		} else {
			PrintSuggestions(suggestions, db)
		}
		return
	}

	report := &AnalysisReport{}

	// Run all analyses.
	fmt.Fprintf(os.Stderr, "Analyzing corpus...\n")

	report.Summary = analyzeSummary(db)
	fmt.Fprintf(os.Stderr, "  - Summary complete\n")

	report.LabelDistribution = analyzeLabelDistribution(db, *topN)
	fmt.Fprintf(os.Stderr, "  - Label distribution complete\n")

	report.ParserCoverage = analyzeParserCoverage(db, *topN)
	fmt.Fprintf(os.Stderr, "  - Parser coverage complete\n")

	report.LabelParsing = analyzeLabelParsing(db, *label)
	fmt.Fprintf(os.Stderr, "  - Label parsing complete\n")

	report.ContentPatterns = analyzeContentPatterns(db, *label, *topN)
	fmt.Fprintf(os.Stderr, "  - Content patterns complete\n")

	report.FieldCoverage = analyzeFieldCoverage(db)
	fmt.Fprintf(os.Stderr, "  - Field coverage complete\n")

	if *showTemplates {
		report.TemplateAnalysis = analyzeTemplates(db, *label, *topN)
		fmt.Fprintf(os.Stderr, "  - Template analysis complete\n")
	}

	// Output.
	if *outputFormat == "json" {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
	} else {
		printTextReport(report, *topN)
	}
}

// AnalysisReport contains all analysis results.
type AnalysisReport struct {
	Summary          SummaryStats            `json:"summary"`
	LabelDistribution []LabelCount           `json:"label_distribution"`
	ParserCoverage   []ParserCount           `json:"parser_coverage"`
	LabelParsing     []LabelParseStats       `json:"label_parsing"`
	ContentPatterns  []LabelContentPatterns  `json:"content_patterns"`
	FieldCoverage    []FieldCoverageStats    `json:"field_coverage"`
	TemplateAnalysis []LabelTemplates        `json:"template_analysis,omitempty"`
}

type SummaryStats struct {
	TotalMessages   int     `json:"total_messages"`
	ParsedMessages  int     `json:"parsed_messages"`
	UnparsedMessages int    `json:"unparsed_messages"`
	ParseRate       float64 `json:"parse_rate"`
	UniqueLabels    int     `json:"unique_labels"`
	UniqueParserTypes int   `json:"unique_parser_types"`
	GoldenMessages  int     `json:"golden_messages"`
	FlaggedMessages int     `json:"flagged_messages"`
}

type LabelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
	Pct   float64 `json:"percentage"`
}

type ParserCount struct {
	ParserType string `json:"parser_type"`
	Count      int    `json:"count"`
	Pct        float64 `json:"percentage"`
}

type LabelParseStats struct {
	Label        string  `json:"label"`
	Total        int     `json:"total"`
	Parsed       int     `json:"parsed"`
	Unparsed     int     `json:"unparsed"`
	ParseRate    float64 `json:"parse_rate"`
	TopParsers   []string `json:"top_parsers"`
}

type LabelContentPatterns struct {
	Label      string         `json:"label"`
	Keywords   []KeywordCount `json:"keywords"`
	Structures []string       `json:"common_structures"`
}

type KeywordCount struct {
	Keyword string `json:"keyword"`
	Count   int    `json:"count"`
	Pct     float64 `json:"percentage"`
}

type FieldCoverageStats struct {
	ParserType string             `json:"parser_type"`
	Fields     []FieldCount       `json:"fields"`
}

type FieldCount struct {
	Field   string  `json:"field"`
	Present int     `json:"present"`
	Missing int     `json:"missing"`
	Pct     float64 `json:"percentage"`
}

type LabelTemplates struct {
	Label           string          `json:"label"`
	TotalMessages   int             `json:"total_messages"`
	UniqueTemplates int             `json:"unique_templates"`
	TopTemplates    []TemplateCount `json:"top_templates"`
}

type TemplateCount struct {
	Template string `json:"template"`
	Count    int    `json:"count"`
	Example  string `json:"example"`
}

func analyzeSummary(db *sql.DB) SummaryStats {
	var stats SummaryStats

	db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&stats.TotalMessages)
	db.QueryRow("SELECT COUNT(*) FROM messages WHERE parser_type != 'unparsed' AND parser_type != ''").Scan(&stats.ParsedMessages)
	stats.UnparsedMessages = stats.TotalMessages - stats.ParsedMessages
	if stats.TotalMessages > 0 {
		stats.ParseRate = float64(stats.ParsedMessages) / float64(stats.TotalMessages) * 100
	}
	db.QueryRow("SELECT COUNT(DISTINCT label) FROM messages").Scan(&stats.UniqueLabels)
	db.QueryRow("SELECT COUNT(DISTINCT parser_type) FROM messages WHERE parser_type != ''").Scan(&stats.UniqueParserTypes)
	db.QueryRow("SELECT COUNT(*) FROM messages WHERE is_golden = 1").Scan(&stats.GoldenMessages)
	db.QueryRow("SELECT COUNT(*) FROM messages WHERE annotation IS NOT NULL AND annotation != ''").Scan(&stats.FlaggedMessages)

	return stats
}

func analyzeLabelDistribution(db *sql.DB, topN int) []LabelCount {
	rows, err := db.Query(`
		SELECT label, COUNT(*) as cnt
		FROM messages
		GROUP BY label
		ORDER BY cnt DESC
		LIMIT ?`, topN)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var total int
	db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&total)

	var results []LabelCount
	for rows.Next() {
		var lc LabelCount
		rows.Scan(&lc.Label, &lc.Count)
		if total > 0 {
			lc.Pct = float64(lc.Count) / float64(total) * 100
		}
		results = append(results, lc)
	}
	return results
}

func analyzeParserCoverage(db *sql.DB, topN int) []ParserCount {
	rows, err := db.Query(`
		SELECT COALESCE(NULLIF(parser_type, ''), 'unparsed') as ptype, COUNT(*) as cnt
		FROM messages
		GROUP BY ptype
		ORDER BY cnt DESC
		LIMIT ?`, topN)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var total int
	db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&total)

	var results []ParserCount
	for rows.Next() {
		var pc ParserCount
		rows.Scan(&pc.ParserType, &pc.Count)
		if total > 0 {
			pc.Pct = float64(pc.Count) / float64(total) * 100
		}
		results = append(results, pc)
	}
	return results
}

func analyzeLabelParsing(db *sql.DB, filterLabel string) []LabelParseStats {
	query := `
		SELECT
			label,
			COUNT(*) as total,
			SUM(CASE WHEN parser_type != 'unparsed' AND parser_type != '' THEN 1 ELSE 0 END) as parsed
		FROM messages
	`
	if filterLabel != "" {
		query += " WHERE label = ?"
	}
	query += " GROUP BY label ORDER BY total DESC LIMIT 30"

	var rows *sql.Rows
	var err error
	if filterLabel != "" {
		rows, err = db.Query(query, filterLabel)
	} else {
		rows, err = db.Query(query)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []LabelParseStats
	for rows.Next() {
		var ls LabelParseStats
		rows.Scan(&ls.Label, &ls.Total, &ls.Parsed)
		ls.Unparsed = ls.Total - ls.Parsed
		if ls.Total > 0 {
			ls.ParseRate = float64(ls.Parsed) / float64(ls.Total) * 100
		}

		// Get top parsers for this label.
		prows, _ := db.Query(`
			SELECT parser_type, COUNT(*) as cnt
			FROM messages
			WHERE label = ? AND parser_type != '' AND parser_type != 'unparsed'
			GROUP BY parser_type
			ORDER BY cnt DESC
			LIMIT 3`, ls.Label)
		if prows != nil {
			for prows.Next() {
				var pt string
				var cnt int
				prows.Scan(&pt, &cnt)
				ls.TopParsers = append(ls.TopParsers, fmt.Sprintf("%s(%d)", pt, cnt))
			}
			prows.Close()
		}

		results = append(results, ls)
	}
	return results
}

// Keywords to look for in messages - these indicate potential data value.
var interestingKeywords = []string{
	// Clearances/PDC.
	"PDC", "CLEARANCE", "CLEARED", "CLRD",
	// Position/Navigation.
	"POSITION", "POSN", "POS", "ETA", "ETD", "ETO",
	"WAYPOINT", "DIRECT", "DCT",
	// Weather.
	"METAR", "TAF", "ATIS", "WIND", "TEMP", "QNH",
	// Flight operations.
	"DEPARTURE", "ARRIVAL", "LANDING", "TAKEOFF",
	"GATE", "TAXI", "PUSHBACK",
	// Route.
	"ROUTE", "VIA", "FILED",
	// Aircraft state.
	"FUEL", "FOB", "WEIGHT", "LOAD",
	// Comms.
	"FREQ", "CONTACT", "SELCAL",
	// Identifiers.
	"SQUAWK", "XPNDR", "FLIGHT", "FLT",
}

func analyzeContentPatterns(db *sql.DB, filterLabel string, topN int) []LabelContentPatterns {
	// Get labels to analyze.
	query := "SELECT DISTINCT label FROM messages"
	if filterLabel != "" {
		query += " WHERE label = '" + filterLabel + "'"
	}
	query += " ORDER BY label"

	labelRows, err := db.Query(query)
	if err != nil {
		return nil
	}
	defer labelRows.Close()

	var labels []string
	for labelRows.Next() {
		var l string
		labelRows.Scan(&l)
		labels = append(labels, l)
	}

	var results []LabelContentPatterns
	for _, lbl := range labels {
		// Get sample of messages for this label.
		rows, err := db.Query(`
			SELECT raw_text FROM messages
			WHERE label = ?
			LIMIT 1000`, lbl)
		if err != nil {
			continue
		}

		keywordCounts := make(map[string]int)
		var total int

		for rows.Next() {
			var text string
			rows.Scan(&text)
			total++
			upper := strings.ToUpper(text)

			for _, kw := range interestingKeywords {
				if strings.Contains(upper, kw) {
					keywordCounts[kw]++
				}
			}
		}
		rows.Close()

		if total == 0 {
			continue
		}

		// Sort keywords by count.
		var keywords []KeywordCount
		for kw, cnt := range keywordCounts {
			if cnt > 0 {
				keywords = append(keywords, KeywordCount{
					Keyword: kw,
					Count:   cnt,
					Pct:     float64(cnt) / float64(total) * 100,
				})
			}
		}
		sort.Slice(keywords, func(i, j int) bool {
			return keywords[i].Count > keywords[j].Count
		})
		if len(keywords) > topN {
			keywords = keywords[:topN]
		}

		if len(keywords) > 0 {
			results = append(results, LabelContentPatterns{
				Label:    lbl,
				Keywords: keywords,
			})
		}
	}

	return results
}

func analyzeFieldCoverage(db *sql.DB) []FieldCoverageStats {
	// Get parser types with parsed_json.
	rows, err := db.Query(`
		SELECT DISTINCT parser_type
		FROM messages
		WHERE parser_type != '' AND parser_type != 'unparsed' AND parsed_json != ''
		ORDER BY parser_type`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var parserTypes []string
	for rows.Next() {
		var pt string
		rows.Scan(&pt)
		parserTypes = append(parserTypes, pt)
	}

	var results []FieldCoverageStats
	for _, pt := range parserTypes {
		// Sample parsed_json for this parser type.
		jrows, err := db.Query(`
			SELECT parsed_json FROM messages
			WHERE parser_type = ? AND parsed_json != ''
			LIMIT 500`, pt)
		if err != nil {
			continue
		}

		fieldPresent := make(map[string]int)
		fieldMissing := make(map[string]int)
		var total int

		for jrows.Next() {
			var jsonStr string
			jrows.Scan(&jsonStr)
			total++

			var data map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
				continue
			}

			// Track all fields seen.
			for k, v := range data {
				// Skip metadata fields.
				if k == "message_id" || k == "timestamp" || k == "raw_text" || k == "parse_confidence" {
					continue
				}

				isEmpty := false
				switch val := v.(type) {
				case string:
					isEmpty = val == ""
				case float64:
					isEmpty = val == 0
				case []interface{}:
					isEmpty = len(val) == 0
				case nil:
					isEmpty = true
				}

				if isEmpty {
					fieldMissing[k]++
				} else {
					fieldPresent[k]++
				}
			}
		}
		jrows.Close()

		if total == 0 {
			continue
		}

		// Combine present and missing for all fields.
		allFields := make(map[string]bool)
		for f := range fieldPresent {
			allFields[f] = true
		}
		for f := range fieldMissing {
			allFields[f] = true
		}

		var fields []FieldCount
		for f := range allFields {
			present := fieldPresent[f]
			missing := fieldMissing[f]
			fields = append(fields, FieldCount{
				Field:   f,
				Present: present,
				Missing: missing,
				Pct:     float64(present) / float64(total) * 100,
			})
		}
		sort.Slice(fields, func(i, j int) bool {
			return fields[i].Present > fields[j].Present
		})

		results = append(results, FieldCoverageStats{
			ParserType: pt,
			Fields:     fields,
		})
	}

	return results
}

// Template analysis - reuses logic from templates command.
var tokenPatterns = []struct {
	Name    string
	Pattern *regexp.Regexp
}{
	{"<FREQ>", regexp.MustCompile(`^\d{2,3}\.\d{1,3}$`)},
	{"<TIME>", regexp.MustCompile(`^[0-2]\d[0-5]\d$`)},
	{"<SQWK>", regexp.MustCompile(`^[0-7]{4}$`)},
	{"<FL>", regexp.MustCompile(`^FL\d{2,3}$`)},
	{"<RWY>", regexp.MustCompile(`^\d{1,2}[LCR]?$`)},
	{"<ICAO>", regexp.MustCompile(`^[A-Z]{4}$`)},
	{"<FLIGHT>", regexp.MustCompile(`^[A-Z]{2,3}\d{1,4}[A-Z]?$`)},
	{"<TAIL>", regexp.MustCompile(`^[A-Z]{1,2}-?[A-Z]{0,3}\d{1,5}[A-Z]{0,2}$`)},
	{"<ACFT>", regexp.MustCompile(`^[A-Z]\d{2,3}[A-Z]?$`)},
	{"<NUM>", regexp.MustCompile(`^\d+$`)},
	{"<WPT5>", regexp.MustCompile(`^[A-Z]{5}$`)},
	{"<CODE>", regexp.MustCompile(`^[A-Z]{3,4}$`)},
	{"<ALNUM>", regexp.MustCompile(`^[A-Z0-9]{6,}$`)},
}

var literalKeywords = map[string]bool{
	"PDC": true, "CLRD": true, "CLEARED": true, "TO": true, "VIA": true,
	"OFF": true, "RWY": true, "RUNWAY": true, "SID": true, "DEP": true,
	"SQUAWK": true, "XPNDR": true, "FREQ": true, "ATIS": true,
	"CLIMB": true, "MAINTAIN": true, "EXPECT": true, "CONTACT": true,
	"FROM": true, "AT": true, "ON": true, "FOR": true, "WITH": true,
	"POS": true, "POSITION": true, "ETA": true, "ETD": true,
	"ROUTE": true, "DIRECT": true, "DCT": true, "ALT": true, "FL": true,
}

func analyzeTemplates(db *sql.DB, filterLabel string, topN int) []LabelTemplates {
	// Get labels to analyze.
	query := `SELECT label, COUNT(*) as cnt FROM messages GROUP BY label HAVING cnt >= 10 ORDER BY cnt DESC LIMIT 20`
	if filterLabel != "" {
		query = `SELECT label, COUNT(*) as cnt FROM messages WHERE label = '` + filterLabel + `' GROUP BY label`
	}

	labelRows, err := db.Query(query)
	if err != nil {
		return nil
	}
	defer labelRows.Close()

	var labels []string
	for labelRows.Next() {
		var l string
		var cnt int
		labelRows.Scan(&l, &cnt)
		labels = append(labels, l)
	}

	var results []LabelTemplates
	for _, lbl := range labels {
		rows, err := db.Query(`SELECT raw_text FROM messages WHERE label = ? LIMIT 2000`, lbl)
		if err != nil {
			continue
		}

		templates := make(map[string][]string) // template -> examples
		var total int

		for rows.Next() {
			var text string
			rows.Scan(&text)
			total++

			tmpl := normaliseToTemplate(text)
			if len(templates[tmpl]) < 2 {
				templates[tmpl] = append(templates[tmpl], text)
			} else {
				templates[tmpl] = templates[tmpl] // Just increment count implicitly.
			}
		}
		rows.Close()

		// Count templates.
		type tmplCount struct {
			tmpl    string
			count   int
			example string
		}
		var counts []tmplCount
		for tmpl, examples := range templates {
			counts = append(counts, tmplCount{tmpl, len(examples), examples[0]})
		}
		// Recount properly.
		templateCounts := make(map[string]int)
		templateExamples := make(map[string]string)
		rows2, _ := db.Query(`SELECT raw_text FROM messages WHERE label = ? LIMIT 5000`, lbl)
		if rows2 != nil {
			for rows2.Next() {
				var text string
				rows2.Scan(&text)
				tmpl := normaliseToTemplate(text)
				templateCounts[tmpl]++
				if _, ok := templateExamples[tmpl]; !ok {
					templateExamples[tmpl] = text
				}
			}
			rows2.Close()
		}

		var topTemplates []TemplateCount
		for tmpl, cnt := range templateCounts {
			topTemplates = append(topTemplates, TemplateCount{
				Template: truncate(tmpl, 100),
				Count:    cnt,
				Example:  truncate(templateExamples[tmpl], 200),
			})
		}
		sort.Slice(topTemplates, func(i, j int) bool {
			return topTemplates[i].Count > topTemplates[j].Count
		})
		if len(topTemplates) > topN {
			topTemplates = topTemplates[:topN]
		}

		results = append(results, LabelTemplates{
			Label:           lbl,
			TotalMessages:   total,
			UniqueTemplates: len(templateCounts),
			TopTemplates:    topTemplates,
		})
	}

	return results
}

func normaliseToTemplate(text string) string {
	text = strings.ToUpper(text)
	lines := strings.Split(text, "\n")

	var normalisedLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		tokens := strings.Fields(line)
		var normalisedTokens []string

		for _, tok := range tokens {
			norm := classifyToken(tok)
			normalisedTokens = append(normalisedTokens, norm)
		}

		if len(normalisedTokens) > 0 {
			normalisedLines = append(normalisedLines, strings.Join(normalisedTokens, " "))
		}
	}

	return strings.Join(normalisedLines, " | ")
}

func classifyToken(tok string) string {
	if literalKeywords[tok] {
		return tok
	}

	for _, tp := range tokenPatterns {
		if tp.Pattern.MatchString(tok) {
			return tp.Name
		}
	}

	if len(tok) <= 2 {
		return tok
	}

	if regexp.MustCompile(`^[A-Z]{3,8}$`).MatchString(tok) {
		return tok
	}

	return "<OTHER>"
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func printTextReport(report *AnalysisReport, topN int) {
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("                    ACARS CORPUS ANALYSIS")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	// Summary.
	fmt.Println("SUMMARY")
	fmt.Println("───────")
	s := report.Summary
	fmt.Printf("Total Messages:     %d\n", s.TotalMessages)
	fmt.Printf("Parsed:             %d (%.1f%%)\n", s.ParsedMessages, s.ParseRate)
	fmt.Printf("Unparsed:           %d (%.1f%%)\n", s.UnparsedMessages, 100-s.ParseRate)
	fmt.Printf("Unique Labels:      %d\n", s.UniqueLabels)
	fmt.Printf("Unique Parser Types: %d\n", s.UniqueParserTypes)
	fmt.Printf("Golden Messages:    %d\n", s.GoldenMessages)
	fmt.Printf("Flagged Messages:   %d\n", s.FlaggedMessages)
	fmt.Println()

	// Label distribution.
	fmt.Println("LABEL DISTRIBUTION (Top messages by ACARS label)")
	fmt.Println("─────────────────")
	fmt.Printf("%-10s %10s %8s\n", "Label", "Count", "Pct")
	for _, lc := range report.LabelDistribution {
		label := lc.Label
		if label == "" {
			label = "(empty)"
		}
		fmt.Printf("%-10s %10d %7.1f%%\n", label, lc.Count, lc.Pct)
	}
	fmt.Println()

	// Parser coverage.
	fmt.Println("PARSER COVERAGE (Messages by parser type)")
	fmt.Println("───────────────")
	fmt.Printf("%-20s %10s %8s\n", "Parser", "Count", "Pct")
	for _, pc := range report.ParserCoverage {
		fmt.Printf("%-20s %10d %7.1f%%\n", pc.ParserType, pc.Count, pc.Pct)
	}
	fmt.Println()

	// Label parsing stats.
	fmt.Println("PARSING BY LABEL (Coverage per ACARS label)")
	fmt.Println("────────────────")
	fmt.Printf("%-10s %8s %8s %8s %8s  %s\n", "Label", "Total", "Parsed", "Unparsed", "Rate", "Top Parsers")
	for _, ls := range report.LabelParsing {
		label := ls.Label
		if label == "" {
			label = "(empty)"
		}
		parsers := strings.Join(ls.TopParsers, ", ")
		fmt.Printf("%-10s %8d %8d %8d %7.1f%%  %s\n", label, ls.Total, ls.Parsed, ls.Unparsed, ls.ParseRate, parsers)
	}
	fmt.Println()

	// Content patterns.
	fmt.Println("CONTENT PATTERNS (Keywords found per label)")
	fmt.Println("────────────────")
	for _, cp := range report.ContentPatterns {
		if len(cp.Keywords) == 0 {
			continue
		}
		label := cp.Label
		if label == "" {
			label = "(empty)"
		}
		var kwStrs []string
		for _, kw := range cp.Keywords {
			if len(kwStrs) >= 8 {
				break
			}
			kwStrs = append(kwStrs, fmt.Sprintf("%s(%.0f%%)", kw.Keyword, kw.Pct))
		}
		fmt.Printf("%-10s: %s\n", label, strings.Join(kwStrs, ", "))
	}
	fmt.Println()

	// Field coverage.
	fmt.Println("FIELD COVERAGE (Extraction rate per parser)")
	fmt.Println("──────────────")
	for _, fc := range report.FieldCoverage {
		fmt.Printf("\n%s:\n", fc.ParserType)
		for _, f := range fc.Fields {
			bar := strings.Repeat("█", int(f.Pct/5))
			fmt.Printf("  %-20s %5.1f%% %s\n", f.Field, f.Pct, bar)
		}
	}
	fmt.Println()

	// Template analysis.
	if len(report.TemplateAnalysis) > 0 {
		fmt.Println("TEMPLATE ANALYSIS (Format patterns per label)")
		fmt.Println("─────────────────")
		for _, lt := range report.TemplateAnalysis {
			label := lt.Label
			if label == "" {
				label = "(empty)"
			}
			fmt.Printf("\n%s: %d messages, %d unique templates\n", label, lt.TotalMessages, lt.UniqueTemplates)
			for i, t := range lt.TopTemplates {
				if i >= 5 {
					break
				}
				fmt.Printf("  [%d] %s\n", t.Count, t.Template)
			}
		}
	}
}