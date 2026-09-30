// Package main provides the templates command for discovering message format templates.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"acars_parser/internal/storage"
)

// Token classification patterns - order matters (more specific first).
var tokenPatterns = []struct {
	Name    string
	Pattern *regexp.Regexp
}{
	// Frequencies: 118.800, 121.5, etc.
	{"<FREQ>", regexp.MustCompile(`^\d{2,3}\.\d{1,3}$`)},

	// Time: 4 digits, often HHMM format.
	{"<TIME>", regexp.MustCompile(`^[0-2]\d[0-5]\d$`)},

	// Squawk: exactly 4 digits, octal-ish (0-7).
	{"<SQWK>", regexp.MustCompile(`^[0-7]{4}$`)},

	// Flight level: FL followed by digits.
	{"<FL>", regexp.MustCompile(`^FL\d{2,3}$`)},

	// Runway: 1-2 digits optionally followed by L/C/R.
	{"<RWY>", regexp.MustCompile(`^\d{1,2}[LCR]?$`)},

	// ICAO airport: exactly 4 uppercase letters starting with valid prefixes.
	{"<ICAO>", regexp.MustCompile(`^[A-Z]{4}$`)},

	// Flight number: 2-3 letters followed by 1-4 digits.
	{"<FLIGHT>", regexp.MustCompile(`^[A-Z]{2,3}\d{1,4}[A-Z]?$`)},

	// Tail number: starts with country prefix, has digits.
	{"<TAIL>", regexp.MustCompile(`^[A-Z]{1,2}-?[A-Z]{0,3}\d{1,5}[A-Z]{0,2}$`)},

	// Aircraft type: common patterns like A320, B738, E190.
	{"<ACFT>", regexp.MustCompile(`^[A-Z]\d{2,3}[A-Z]?$`)},

	// Generic number sequences.
	{"<NUM>", regexp.MustCompile(`^\d+$`)},

	// Alphanumeric codes (potential waypoints, SIDs, etc.) - 5 letters.
	{"<WPT5>", regexp.MustCompile(`^[A-Z]{5}$`)},

	// Alphanumeric codes - 3-4 letters.
	{"<CODE>", regexp.MustCompile(`^[A-Z]{3,4}$`)},

	// Mixed alphanumeric - likely identifiers.
	{"<ALNUM>", regexp.MustCompile(`^[A-Z0-9]{6,}$`)},
}

// Keywords that should remain literal (structural markers).
var literalKeywords = map[string]bool{
	// PDC-specific.
	"PDC": true, "CLRD": true, "CLEARED": true, "TO": true, "VIA": true,
	"OFF": true, "RWY": true, "RUNWAY": true, "SID": true, "DEP": true,
	"SQUAWK": true, "XPNDR": true, "FREQ": true, "ATIS": true,
	"CLIMB": true, "MAINTAIN": true, "EXPECT": true, "CONTACT": true,
	"DEPARTURE": true, "INITIAL": true, "ALTITUDE": true,

	// Common structural words.
	"FROM": true, "AT": true, "ON": true, "FOR": true, "WITH": true,
	"AND": true, "OR": true, "THE": true, "OF": true, "IN": true,
	"NO": true, "NOT": true, "END": true, "START": true,

	// Message type indicators.
	"UPLINK": true, "DOWNLINK": true, "REQUEST": true, "REPLY": true,
	"ACK": true, "NAK": true, "WILCO": true, "UNABLE": true, "ROGER": true,

	// Position/weather related.
	"POS": true, "POSITION": true, "ETA": true, "ETD": true, "UTC": true,
	"ROUTE": true, "DIRECT": true, "DCT": true, "HDG": true, "HEADING": true,
	"ALT": true, "FL": true, "FT": true, "FEET": true,
	"TEMP": true, "WIND": true, "QNH": true, "ALTIMETER": true,

	// Aircraft state.
	"LANDING": true, "TAKEOFF": true, "TAXI": true, "GATE": true,
	"ARRIVED": true, "DEPARTED": true, "AIRBORNE": true, "GROUND": true,

	// OOOI events (OUT already covered above).
	"OOOI": true,

	// Common labels/headers.
	"FLT": true, "FLIGHT": true, "A/C": true, "ACFT": true,
	"ORIG": true, "DEST": true, "ORIGIN": true, "DESTINATION": true,
}

func runTemplatesCmd(args []string) {
	fs := flag.NewFlagSet("templates", flag.ExitOnError)

	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPassword := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")
	chDB := fs.String("ch-db", defaultCHDatabase(), "ClickHouse database")
	parserType := fs.String("type", "", "Filter by parser type")
	label := fs.String("label", "", "Filter by ACARS label")
	limit := fs.Int("limit", 0, "Limit number of messages (0 = all)")
	minCount := fs.Int("min", 2, "Minimum messages per template to show")
	showExamples := fs.Int("examples", 1, "Number of example messages per template")
	verbose := fs.Bool("v", false, "Verbose output: show full template strings")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Open database.
	db, err := storage.OpenClickHouse(ctx, storage.ClickHouseConfig{
		Host:     *chHost,
		Port:     *chPort,
		Database: *chDB,
		User:     *chUser,
		Password: *chPassword,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	// Query messages.
	params := storage.CHQueryParams{
		ParserType: *parserType,
		Label:      *label,
		OrderBy:    "id",
		OrderDesc:  false,
	}
	if *limit > 0 {
		params.Limit = *limit
	} else {
		// TODO: The -limit help text says 0 means all, but 0 is capped here.
		params.Limit = 100000
	}

	messages, err := db.Query(ctx, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying messages: %v\n", err)
		os.Exit(1)
	}

	// Group by template.
	type templateGroup struct {
		Template string
		Count    int
		Examples []string
		IDs      []uint64
	}

	groups := make(map[string]*templateGroup)

	for _, msg := range messages {
		template := normaliseToTemplate(msg.RawText)

		if g, ok := groups[template]; ok {
			g.Count++
			if len(g.Examples) < *showExamples {
				g.Examples = append(g.Examples, msg.RawText)
				g.IDs = append(g.IDs, msg.ID)
			}
		} else {
			groups[template] = &templateGroup{
				Template: template,
				Count:    1,
				Examples: []string{msg.RawText},
				IDs:      []uint64{msg.ID},
			}
		}
	}

	// Sort by count descending.
	var sorted []*templateGroup
	for _, g := range groups {
		if g.Count >= *minCount {
			sorted = append(sorted, g)
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	// Output.
	fmt.Printf("Template Analysis\n")
	fmt.Printf("=================\n")
	fmt.Printf("Total messages: %d\n", len(messages))
	fmt.Printf("Unique templates: %d\n", len(groups))
	fmt.Printf("Templates with >= %d messages: %d\n\n", *minCount, len(sorted))

	for i, g := range sorted {
		fmt.Printf("--- Template %d (%d messages) ---\n", i+1, g.Count)

		if *verbose {
			fmt.Printf("Pattern: %s\n", g.Template)
		} else {
			// Truncate long templates.
			tmpl := g.Template
			if len(tmpl) > 100 {
				tmpl = tmpl[:100] + "..."
			}
			fmt.Printf("Pattern: %s\n", tmpl)
		}

		fmt.Println("Examples:")
		for j, ex := range g.Examples {
			fmt.Printf("  [ID %d]\n", g.IDs[j])
			printIndentedTrunc(ex, "    ", 500)
		}
		fmt.Println()
	}

	// Show singleton stats.
	singletons := 0
	for _, g := range groups {
		if g.Count == 1 {
			singletons++
		}
	}
	fmt.Printf("Summary: %d singletons (unique formats), %d repeated templates\n",
		singletons, len(groups)-singletons)
}

// normaliseToTemplate converts a message to its template form.
func normaliseToTemplate(text string) string {
	// Normalise whitespace and case.
	text = strings.ToUpper(text)
	lines := strings.Split(text, "\n")

	var normalisedLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		tokens := tokenise(line)
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

// tokenise splits a line into tokens, preserving some punctuation.
func tokenise(line string) []string {
	// Split on whitespace and common delimiters, but keep some structure.
	var tokens []string
	var current strings.Builder

	for _, r := range line {
		switch r {
		case ' ', '\t':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case '/', '-', ':', '=', ',', '.':
			// Flush current token.
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			// Add punctuation as separate token (helps identify structure).
			tokens = append(tokens, string(r))
		case '(', ')', '[', ']':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(r))
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// classifyToken returns the token or its placeholder classification.
func classifyToken(tok string) string {
	// Check if it's a literal keyword first.
	if literalKeywords[tok] {
		return tok
	}

	// Try pattern matches.
	for _, tp := range tokenPatterns {
		if tp.Pattern.MatchString(tok) {
			return tp.Name
		}
	}

	// Keep short tokens literal (likely structural).
	if len(tok) <= 2 {
		return tok
	}

	// Default: keep as-is if it looks structural, otherwise genericise.
	// If it's all letters and short-ish, might be a keyword we missed.
	if regexp.MustCompile(`^[A-Z]{3,8}$`).MatchString(tok) {
		// Could be a keyword - keep it to see patterns.
		return tok
	}

	return "<OTHER>"
}

func printIndentedTrunc(text, indent string, maxLen int) {
	if len(text) > maxLen {
		text = text[:maxLen] + "..."
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		fmt.Printf("%s%s\n", indent, line)
	}
}
