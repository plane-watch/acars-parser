package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"acars_parser/internal/acars"
	"acars_parser/internal/registry"
)

// ExtractedData contains all extracted information from ACARS messages.
type ExtractedData struct {
	Stats   ExtractStats             `json:"stats"`
	Results map[string][]interface{} `json:"results,omitempty"`
}

// ExtractStats contains statistics about the extraction.
type ExtractStats struct {
	TotalMessages int            `json:"total_messages"`
	ParsedByType  map[string]int `json:"parsed_by_type"`
	TopOrigins    []CountEntry   `json:"top_origins,omitempty"`
	TopDests      []CountEntry   `json:"top_destinations,omitempty"`
}

// CountEntry represents a count for a code.
type CountEntry struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

func runExtractCmd(args []string) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	inputFile := fs.String("input", "", "Input JSONL file (default: stdin)")
	outputFile := fs.String("output", "", "Output JSON file (default: stdout)")
	prettyPrint := fs.Bool("pretty", false, "Pretty print JSON output")
	includeAll := fs.Bool("all", false, "Include all parsed data types")
	_ = fs.Parse(args) // ExitOnError handles parse failures.

	// Silence unused variable warning - all types included by default.
	// TODO: The -all flag is accepted but has no effect; every result type is
	// always included. Either implement a filter or remove the flag.
	_ = includeAll

	if err := runExtract(*inputFile, *outputFile, *prettyPrint); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runExtract(inputPath, outputPath string, prettyPrint bool) error {
	var input io.Reader
	if inputPath != "" {
		f, err := os.Open(inputPath)
		if err != nil {
			return fmt.Errorf("opening input: %w", err)
		}
		defer func() { _ = f.Close() }()
		input = f
	} else {
		input = os.Stdin
	}

	data, err := extractAll(input)
	if err != nil {
		return fmt.Errorf("extracting data: %w", err)
	}

	var output io.Writer
	if outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("creating output: %w", err)
		}
		defer func() { _ = f.Close() }()
		output = f
	} else {
		output = os.Stdout
	}

	enc := json.NewEncoder(output)
	if prettyPrint {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(data)
}

func extractAll(reader io.Reader) (*ExtractedData, error) {
	reg := registry.Default()
	reg.Sort()

	data := &ExtractedData{
		Stats: ExtractStats{
			ParsedByType: make(map[string]int),
		},
		Results: make(map[string][]interface{}),
	}

	originCounts := make(map[string]int)
	destCounts := make(map[string]int)

	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		data.Stats.TotalMessages++

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Try to parse as NATS wrapper first, then as direct message.
		msg := parseMessage(line)
		if msg == nil {
			continue
		}

		// Dispatch to all matching parsers.
		results := reg.Dispatch(msg)
		for _, result := range results {
			typeName := result.Type()
			data.Stats.ParsedByType[typeName]++
			data.Results[typeName] = append(data.Results[typeName], result)

			// Extract origin/destination for stats if available.
			extractRouteStats(result, originCounts, destCounts)
		}
	}

	// Build top origins/destinations.
	data.Stats.TopOrigins = topN(originCounts, 10)
	data.Stats.TopDests = topN(destCounts, 10)

	return data, scanner.Err()
}

func parseMessage(line []byte) *acars.Message {
	// Try NATS wrapper format first.
	var wrapper acars.NATSWrapper
	if err := json.Unmarshal(line, &wrapper); err == nil && wrapper.Message != nil {
		return wrapper.ToMessage()
	}

	// Try direct message format.
	var msg acars.Message
	if err := json.Unmarshal(line, &msg); err == nil && msg.Label != "" {
		return &msg
	}

	return nil
}

// extractRouteStats extracts origin/destination from results that have them.
func extractRouteStats(result registry.Result, origins, dests map[string]int) {
	// Use type assertion to check for common fields.
	// This is a bit ugly but avoids adding methods to every result type.
	if v, ok := result.(interface{ GetOrigin() string }); ok {
		if o := v.GetOrigin(); o != "" {
			origins[o]++
		}
	}
	if v, ok := result.(interface{ GetDestination() string }); ok {
		if d := v.GetDestination(); d != "" {
			dests[d]++
		}
	}

	// Fallback: try to extract from JSON representation.
	b, err := json.Marshal(result)
	if err != nil {
		return
	}
	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return
	}

	if o, ok := m["origin"].(string); ok && o != "" {
		origins[o]++
	}
	if o, ok := m["origin_icao"].(string); ok && o != "" {
		origins[o]++
	}
	if d, ok := m["destination"].(string); ok && d != "" {
		dests[d]++
	}
	if d, ok := m["dest_icao"].(string); ok && d != "" {
		dests[d]++
	}
}

func topN(m map[string]int, n int) []CountEntry {
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].v > sorted[j].v
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	result := make([]CountEntry, len(sorted))
	for i, kv := range sorted {
		result[i] = CountEntry{kv.k, kv.v}
	}
	return result
}
