// Pattern suggestion logic for generating regex candidates from message clusters.
package main

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"acars_parser/internal/storage"
)

// PatternSuggestion represents a suggested regex pattern for a message cluster.
type PatternSuggestion struct {
	ClusterID       int      `json:"cluster_id"`
	MessageCount    int      `json:"message_count"`
	Label           string   `json:"label"`
	SuggestedRegex  string   `json:"suggested_regex"`
	NamedGroups     []string `json:"named_groups"`
	Examples        []string `json:"examples"`
	ExampleIDs      []uint64 `json:"example_ids"`
	TemplatePattern string   `json:"template_pattern"`
}

// msgInfo holds message ID and text for clustering.
type msgInfo struct {
	id   uint64
	text string
}

// SuggestPatterns analyzes messages and suggests regex patterns for clusters.
func SuggestPatterns(ctx context.Context, ch *storage.ClickHouseDB, label string, minClusterSize int, maxSuggestions int) []PatternSuggestion {
	conn := ch.Conn()

	// Get messages for the label.
	rows, err := conn.Query(ctx, `SELECT id, raw_text FROM messages WHERE label = ? LIMIT 5000`, label)
	if err != nil {
		return nil
	}
	defer rows.Close()

	// Group by template.
	clusters := make(map[string][]msgInfo)

	for rows.Next() {
		var id uint64
		var text string
		_ = rows.Scan(&id, &text)

		template := normaliseToTemplate(text)
		clusters[template] = append(clusters[template], msgInfo{id, text})
	}

	// Sort clusters by size.
	type clusterInfo struct {
		template string
		messages []msgInfo
	}
	var sortedClusters []clusterInfo
	for tmpl, msgs := range clusters {
		if len(msgs) >= minClusterSize {
			sortedClusters = append(sortedClusters, clusterInfo{tmpl, msgs})
		}
	}
	sort.Slice(sortedClusters, func(i, j int) bool {
		return len(sortedClusters[i].messages) > len(sortedClusters[j].messages)
	})

	if len(sortedClusters) > maxSuggestions {
		sortedClusters = sortedClusters[:maxSuggestions]
	}

	// Generate suggestions for each cluster.
	var suggestions []PatternSuggestion
	for i, cluster := range sortedClusters {
		suggestion := generatePatternSuggestion(cluster.messages, cluster.template, label, i+1)
		suggestions = append(suggestions, suggestion)
	}

	return suggestions
}

func generatePatternSuggestion(messages []msgInfo, template, label string, clusterID int) PatternSuggestion {
	suggestion := PatternSuggestion{
		ClusterID:       clusterID,
		MessageCount:    len(messages),
		Label:           label,
		TemplatePattern: template,
	}

	// Get examples (up to 3).
	for i, msg := range messages {
		if i >= 3 {
			break
		}
		suggestion.Examples = append(suggestion.Examples, msg.text)
		suggestion.ExampleIDs = append(suggestion.ExampleIDs, msg.id)
	}

	// Generate regex from the first message as reference.
	if len(messages) > 0 {
		regex, groups := generateRegexFromMessage(messages[0].text, template)
		suggestion.SuggestedRegex = regex
		suggestion.NamedGroups = groups
	}

	return suggestion
}

// generateRegexFromMessage creates a regex pattern from a message and its template.
func generateRegexFromMessage(text, template string) (string, []string) {
	// Split template into tokens.
	templateTokens := strings.Fields(strings.ReplaceAll(template, "|", " | "))

	// Split message into lines for line-aware processing.
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	// Build regex by processing template tokens.
	var regexParts []string
	var namedGroups []string
	groupCounts := make(map[string]int)

	lineIdx := 0
	tokenIdx := 0

	for _, tok := range templateTokens {
		if tok == "|" {
			regexParts = append(regexParts, `\s*`)
			lineIdx++
			tokenIdx = 0
			continue
		}

		// Get unique group name.
		baseName := tokenToGroupName(tok)
		if baseName != "" {
			groupCounts[baseName]++
			if groupCounts[baseName] > 1 {
				baseName = fmt.Sprintf("%s%d", baseName, groupCounts[baseName])
			}
		}

		switch tok {
		case "<ICAO>":
			namedGroups = append(namedGroups, baseName)
			regexParts = append(regexParts, fmt.Sprintf(`(?P<%s>[A-Z]{4})`, baseName))
		case "<FLIGHT>":
			namedGroups = append(namedGroups, baseName)
			regexParts = append(regexParts, fmt.Sprintf(`(?P<%s>[A-Z]{2,3}\d{1,4}[A-Z]?)`, baseName))
		case "<TIME>":
			namedGroups = append(namedGroups, baseName)
			regexParts = append(regexParts, fmt.Sprintf(`(?P<%s>\d{4})`, baseName))
		case "<NUM>":
			regexParts = append(regexParts, `\d+`)
		case "<SQWK>":
			namedGroups = append(namedGroups, baseName)
			regexParts = append(regexParts, fmt.Sprintf(`(?P<%s>[0-7]{4})`, baseName))
		case "<FREQ>":
			namedGroups = append(namedGroups, baseName)
			regexParts = append(regexParts, fmt.Sprintf(`(?P<%s>\d{2,3}\.\d{1,3})`, baseName))
		case "<RWY>":
			namedGroups = append(namedGroups, baseName)
			regexParts = append(regexParts, fmt.Sprintf(`(?P<%s>\d{1,2}[LCR]?)`, baseName))
		case "<FL>":
			namedGroups = append(namedGroups, baseName)
			regexParts = append(regexParts, fmt.Sprintf(`(?P<%s>FL\d{2,3})`, baseName))
		case "<TAIL>":
			namedGroups = append(namedGroups, baseName)
			regexParts = append(regexParts, fmt.Sprintf(`(?P<%s>[A-Z0-9]{4,7})`, baseName))
		case "<ACFT>":
			namedGroups = append(namedGroups, baseName)
			regexParts = append(regexParts, fmt.Sprintf(`(?P<%s>[A-Z]\d{2,3}[A-Z]?)`, baseName))
		case "<WPT5>":
			namedGroups = append(namedGroups, baseName)
			regexParts = append(regexParts, fmt.Sprintf(`(?P<%s>[A-Z]{5})`, baseName))
		case "<CODE>":
			regexParts = append(regexParts, `[A-Z]{3,4}`)
		case "<ALNUM>":
			regexParts = append(regexParts, `[A-Z0-9]+`)
		case "<OTHER>":
			regexParts = append(regexParts, `\S+`)
		default:
			// Literal token - escape regex special characters.
			escaped := regexp.QuoteMeta(tok)
			regexParts = append(regexParts, escaped)
		}

		regexParts = append(regexParts, `\s*`)
		tokenIdx++
	}

	// Join and clean up the regex.
	regex := strings.Join(regexParts, "")
	// Remove trailing \s*
	regex = strings.TrimSuffix(regex, `\s*`)
	// Collapse multiple \s* into one
	regex = regexp.MustCompile(`(\\s\*)+`).ReplaceAllString(regex, `\s+`)
	// Make whitespace more flexible
	regex = strings.ReplaceAll(regex, `\s+`, `[\s\t]+`)
	// Add start anchor but not end (messages may have trailing content)
	regex = `(?s)` + regex

	return regex, namedGroups
}

func tokenToGroupName(token string) string {
	switch token {
	case "<ICAO>":
		return "icao"
	case "<FLIGHT>":
		return "flight"
	case "<TIME>":
		return "time"
	case "<SQWK>":
		return "squawk"
	case "<FREQ>":
		return "freq"
	case "<RWY>":
		return "runway"
	case "<FL>":
		return "flight_level"
	case "<TAIL>":
		return "tail"
	case "<ACFT>":
		return "aircraft"
	case "<WPT5>":
		return "waypoint"
	default:
		return ""
	}
}

// TestPattern tests a regex pattern against the corpus and returns match statistics.
func TestPattern(ctx context.Context, ch *storage.ClickHouseDB, pattern string, label string) (matches int, total int, sampleMatches []uint64, sampleNonMatches []uint64) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, 0, nil, nil
	}

	conn := ch.Conn()
	rows, err := conn.Query(ctx, `SELECT id, raw_text FROM messages WHERE label = ? LIMIT 2000`, label)
	if err != nil {
		return 0, 0, nil, nil
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		var text string
		_ = rows.Scan(&id, &text)
		total++

		if re.MatchString(text) {
			matches++
			if len(sampleMatches) < 5 {
				sampleMatches = append(sampleMatches, id)
			}
		} else {
			if len(sampleNonMatches) < 5 {
				sampleNonMatches = append(sampleNonMatches, id)
			}
		}
	}

	return matches, total, sampleMatches, sampleNonMatches
}

// PrintSuggestions outputs pattern suggestions in a readable format.
func PrintSuggestions(ctx context.Context, suggestions []PatternSuggestion, ch *storage.ClickHouseDB, label string) {
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("                    PATTERN SUGGESTIONS")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	for _, s := range suggestions {
		fmt.Printf("───────────────────────────────────────────────────────────────\n")
		fmt.Printf("CLUSTER %d: %d messages (Label: %s)\n", s.ClusterID, s.MessageCount, s.Label)
		fmt.Printf("───────────────────────────────────────────────────────────────\n")
		fmt.Println()

		fmt.Println("Template:")
		fmt.Printf("  %s\n", s.TemplatePattern)
		fmt.Println()

		fmt.Println("Suggested Regex:")
		// Print regex in a more readable format.
		printFormattedRegex(s.SuggestedRegex)
		fmt.Println()

		if len(s.NamedGroups) > 0 {
			fmt.Printf("Capture Groups: %s\n", strings.Join(s.NamedGroups, ", "))
			fmt.Println()
		}

		fmt.Println("Examples:")
		for i, ex := range s.Examples {
			fmt.Printf("  [ID %d]\n", s.ExampleIDs[i])
			printIndentedTrunc(ex, "    ", 300)
			fmt.Println()
		}

		// Test the pattern.
		if ch != nil && s.SuggestedRegex != "" {
			matches, total, _, _ := TestPattern(ctx, ch, s.SuggestedRegex, label)
			fmt.Printf("Test Results: %d/%d messages match (%.1f%%)\n", matches, total, float64(matches)/float64(total)*100)
		}

		fmt.Println()
	}
}

func printFormattedRegex(regex string) {
	// Break long regex into readable chunks.
	if len(regex) <= 80 {
		fmt.Printf("  %s\n", regex)
		return
	}

	// Try to break at logical points.
	parts := strings.Split(regex, `[\s\t]+`)
	var line strings.Builder
	line.WriteString("  ")

	for i, part := range parts {
		if i > 0 {
			if line.Len()+len(part)+10 > 80 {
				fmt.Println(line.String() + `[\s\t]+`)
				line.Reset()
				line.WriteString("    ")
			} else {
				line.WriteString(`[\s\t]+`)
			}
		}
		line.WriteString(part)
	}
	if line.Len() > 2 {
		fmt.Println(line.String())
	}
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
