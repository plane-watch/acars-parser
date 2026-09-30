// Package main provides the debug command for tracing parser pattern matching.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
	"acars_parser/internal/storage"
)

func runDebugCmd(args []string) {
	fs := flag.NewFlagSet("debug", flag.ExitOnError)

	// ClickHouse connection flags.
	chHost := fs.String("ch-host", defaultCHHost(), "ClickHouse host")
	chPort := fs.Int("ch-port", defaultCHPort(), "ClickHouse port")
	chUser := fs.String("ch-user", defaultCHUser(), "ClickHouse user")
	chPass := fs.String("ch-password", defaultCHPassword(), "ClickHouse password")
	chDB := fs.String("ch-db", defaultCHDatabase(), "ClickHouse database")

	msgID := fs.Uint64("id", 0, "Message ID to debug")
	rawText := fs.String("text", "", "Raw message text to debug (instead of -id)")
	labelFilter := fs.String("label", "", "ACARS label for raw text (e.g. 'H1', '16')")
	showAll := fs.Bool("all", false, "Show all pattern attempts, not just matches")
	parserFilter := fs.String("type", "", "Only show results for specific parser type (e.g. 'pdc', 'sq', 'atis')")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Get the message text.
	var text string
	var label string

	if *rawText != "" {
		text = *rawText
		label = *labelFilter // Use provided label for raw text.
	} else if *msgID > 0 {
		// Load from ClickHouse.
		ctx := context.Background()
		db, err := storage.OpenClickHouse(ctx, storage.ClickHouseConfig{
			Host:     *chHost,
			Port:     *chPort,
			Database: *chDB,
			User:     *chUser,
			Password: *chPass,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to ClickHouse: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = db.Close() }()

		msg, err := db.GetByID(ctx, *msgID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error querying: %v\n", err)
			os.Exit(1)
		}

		if msg == nil {
			fmt.Fprintf(os.Stderr, "Message ID %d not found\n", *msgID)
			os.Exit(1)
		}

		text = msg.RawText
		label = msg.Label
	} else {
		fmt.Fprintf(os.Stderr, "Must provide -text or -id\n")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Print header.
	fmt.Println("Debug Trace")
	fmt.Println("===========")
	if *msgID > 0 {
		fmt.Printf("Message ID: %d\n", *msgID)
	}
	if label != "" {
		fmt.Printf("Label: %s\n", label)
	}
	fmt.Println()
	fmt.Println("Raw Text:")
	printIndented(text, "  ")
	fmt.Println()

	// Get registry.
	reg := registry.Default()
	reg.Sort()

	// Create a mock message for dispatch.
	msg := &acars.Message{
		Label: label,
		Text:  text,
	}

	// Try all parsers and collect results.
	matches := reg.Dispatch(msg)

	if len(matches) == 0 {
		fmt.Println("No parser matched this message.")
	} else {
		fmt.Printf("Matched Parsers: %d\n", len(matches))
		for _, m := range matches {
			fmt.Printf("  - %s (%s)\n", m.Parser, m.Result.Type())
		}
	}

	fmt.Println()

	// Get all parsers that implement Traceable and show their traces.
	parsers := getTraceableParsers(reg, label)

	for _, tp := range parsers {
		// Filter by parser type if specified.
		if *parserFilter != "" && tp.parser.Name() != *parserFilter {
			continue
		}

		trace := tp.traceable.ParseWithTrace(msg)
		printParserTrace(trace, *showAll)
	}
}

// traceableParser holds a parser that implements both Parser and Traceable.
type traceableParser struct {
	parser    registry.Parser
	traceable registry.Traceable
}

// getTraceableParsers returns all parsers that implement Traceable.
// Filters by label if provided, otherwise returns all global parsers.
func getTraceableParsers(reg *registry.Registry, label string) []traceableParser {
	var result []traceableParser

	// Get all registered parsers (using the exported method).
	parsers := reg.AllParsers()

	for _, p := range parsers {
		// Check if parser implements Traceable.
		traceable, ok := p.(registry.Traceable)
		if !ok {
			continue
		}

		// If label is specified, only include parsers that handle this label
		// or are global (content-based) parsers.
		if label != "" {
			labels := p.Labels()
			if len(labels) > 0 {
				// Label-specific parser - check if it handles this label.
				found := false
				for _, l := range labels {
					if l == label {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			// Global parsers (empty labels) are always included.
		}

		result = append(result, traceableParser{
			parser:    p,
			traceable: traceable,
		})
	}

	return result
}

func printParserTrace(trace *registry.TraceResult, showAll bool) {
	fmt.Printf("--- %s Parser Trace ---\n", strings.ToUpper(trace.ParserName))

	// Print QuickCheck result.
	if trace.QuickCheck != nil {
		status := "PASSED"
		if !trace.QuickCheck.Passed {
			status = "FAILED"
		}
		fmt.Printf("QuickCheck: %s\n", status)
		if trace.QuickCheck.Reason != "" {
			fmt.Printf("  (%s)\n", trace.QuickCheck.Reason)
		}
		if !trace.QuickCheck.Passed {
			fmt.Println()
			return
		}
	}

	// Print format/pattern traces.
	if len(trace.Formats) > 0 {
		fmt.Println()
		fmt.Println("Grok Patterns:")
		for i, ft := range trace.Formats {
			if !showAll && !ft.Matched {
				continue
			}

			status := "NO MATCH"
			if ft.Matched {
				status = "MATCH"
			}

			fmt.Printf("  [%d] %s: %s\n", i+1, ft.Name, status)

			if ft.Matched && len(ft.Captures) > 0 {
				fmt.Println("      Captures:")
				for k, v := range ft.Captures {
					if v != "" {
						fmt.Printf("        %s: %q\n", k, v)
					}
				}
			}

			if showAll && !ft.Matched {
				// Show truncated pattern for debugging.
				pattern := ft.Pattern
				if len(pattern) > 100 {
					pattern = pattern[:100] + "..."
				}
				fmt.Printf("      Pattern: %s\n", pattern)
			}
		}
	}

	// Print extractor traces.
	if len(trace.Extractors) > 0 {
		fmt.Println()
		fmt.Println("Field Extractors:")
		for _, ext := range trace.Extractors {
			if !showAll && !ext.Matched {
				continue
			}

			status := "NO MATCH"
			if ext.Matched {
				status = fmt.Sprintf("MATCH -> %q", ext.Value)
			}
			fmt.Printf("  %s: %s\n", ext.Name, status)

			if showAll && !ext.Matched {
				pattern := ext.Pattern
				if len(pattern) > 80 {
					pattern = pattern[:80] + "..."
				}
				fmt.Printf("    Pattern: %s\n", pattern)
			}
		}
	}

	// Print overall match result.
	fmt.Println()
	if trace.Matched {
		fmt.Println("Result: MATCHED")
	} else {
		fmt.Println("Result: NO MATCH")
	}

	fmt.Println()
}

func printIndented(text, indent string) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		fmt.Printf("%s%s\n", indent, line)
	}
}
