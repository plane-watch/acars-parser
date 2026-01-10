// Command-line entry point for ACARS Parser (extract-focused).
//
// Note about input formats
// ------------------------
// The upstream parsers in this repo expect an "acars.Message" object with at least:
//   - label (e.g. "H1", "B6", "AA"...)
//   - text  (the ACARS/ARINC message text payload)
//
// In the real world, you may have any of these inputs:
//  1. NATS feed wrapper: {"message":{...}, "airframe":{...}, ...}
//  2. Flat message:      {"label":"H1","text":"...", ...}
//  3. Decoder logs:      dumpvdl2 / dumphfdl JSON where ACARS is nested deep.
//
// This CLI tries to autodetect all three. Use -all to keep messages even if no parser matched.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"acars_parser/internal/acars"
	_ "acars_parser/internal/parsers" // register all parsers via init()
	"acars_parser/internal/registry"
)

type ExtractOut struct {
	Message *acars.Message `json:"message"`
	Results []any          `json:"results,omitempty"`
}

type Stats struct {
	Lines          int
	ParsedNATS     int
	ParsedFlat     int
	ParsedNested   int
	SkippedNoLabel int
	Emitted        int
	Matched        int
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "acars_parser (extract) - commands:")
	fmt.Fprintln(w, "  extract  - parse JSONL file and output JSON")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  acars_parser extract -input messages.jsonl [-output out.json] [-pretty] [-all] [-stats]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Notes:")
	fmt.Fprintln(w, "  - Input must be JSONL (one JSON object per line).")
	fmt.Fprintln(w, "  - For dumpvdl2/dumphfdl logs, the tool will try to find label/text in nested paths.")
	fmt.Fprintln(w, "")
}

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "extract":
		runExtract(os.Args[2:])
	case "-h", "--help", "help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage(os.Stderr)
		os.Exit(2)
	}
}

func runExtract(args []string) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	inPath := fs.String("input", "", "Input JSONL file (default: stdin)")
	outPath := fs.String("output", "", "Output JSON file (default: stdout)")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	includeAll := fs.Bool("all", false, "Include messages even if no parser matched")
	showStats := fs.Bool("stats", false, "Print basic counters to stderr")
	_ = fs.Parse(args)

	// Ensure parsers priority ordering is stable.
	registry.Default().Sort()

	var r io.Reader = os.Stdin
	if *inPath != "" {
		f, err := os.Open(*inPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open input: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		r = f
	}

	scanner := bufio.NewScanner(r)
	// JSON lines can be long; bump buffer (20MB).
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 60*1024*1024)

	out := make([]ExtractOut, 0, 1024)
	st := &Stats{}

	for scanner.Scan() {
		st.Lines++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		b := []byte(line)

		msgs, kind := decodeToMessage(b)
		if len(msgs) == 0 {
			st.SkippedNoLabel++
			continue
		}
		switch kind {
		case "nats":
			st.ParsedNATS++
		case "flat":
			st.ParsedFlat++
		case "nested":
			st.ParsedNested++
		}

		// Process all messages extracted from this line
		for _, msg := range msgs {
			if msg == nil || (strings.TrimSpace(msg.Label) == "" && strings.TrimSpace(msg.Text) == "") {
				continue
			}
			var appended bool
			var matched bool
			out, appended, matched = appendOut(out, msg, *includeAll)
			if appended {
				st.Emitted++
			}
			if matched {
				st.Matched++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Input read error: %v\n", err)
		os.Exit(1)
	}

	var wout io.Writer = os.Stdout
	if *outPath != "" {
		f, err := os.Create(*outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create output: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		wout = f
	}

	enc, err := marshalJSON(out, *pretty)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON encode error: %v\n", err)
		os.Exit(1)
	}
	_, _ = wout.Write(enc)
	if wout == os.Stdout {
		_, _ = wout.Write([]byte("\n"))
	}

	if *showStats {
		fmt.Fprintf(os.Stderr,
			"stats: lines=%d parsed(nats=%d flat=%d nested=%d) skipped(no_label_text)=%d emitted=%d matched=%d\n",
			st.Lines, st.ParsedNATS, st.ParsedFlat, st.ParsedNested, st.SkippedNoLabel, st.Emitted, st.Matched,
		)
	}
}

func appendOut(out []ExtractOut, msg *acars.Message, includeAll bool) ([]ExtractOut, bool, bool) {
	results := registry.Default().Dispatch(msg)
	if !includeAll && len(results) == 0 {
		return out, false, false
	}
	rany := make([]any, 0, len(results))
	for _, r := range results {
		rany = append(rany, r) // keep concrete types for JSON marshal
	}
	out = append(out, ExtractOut{Message: msg, Results: rany})
	return out, true, len(results) > 0
}

func marshalJSON(v any, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(v, "", "  ")
	}
	return json.Marshal(v)
}

func decodeToMessage(b []byte) ([]*acars.Message, string) {
	// 1) NATS wrapper
	var w acars.NATSWrapper
	if err := json.Unmarshal(b, &w); err == nil && w.Message != nil {
		if msg := w.ToMessage(); msg != nil && (msg.Label != "" || msg.Text != "") {
			return []*acars.Message{msg}, "nats"
		}
	}

	// 2) Flat message (only accept if it actually contains label/text)
	var m acars.Message
	if err := json.Unmarshal(b, &m); err == nil {
		if strings.TrimSpace(m.Label) != "" || strings.TrimSpace(m.Text) != "" {
			return []*acars.Message{&m}, "flat"
		}
	}

	// 3) Nested formats (dumpvdl2/dumphfdl, etc.)
	var anyObj any
	if err := json.Unmarshal(b, &anyObj); err != nil {
		return nil, ""
	}
	msgs := buildMessagesFromNested(anyObj)
	if len(msgs) > 0 {
		return msgs, "nested"
	}
	return nil, ""
}

// buildMessagesFromNested tries common paths used by dumpvdl2 / dumphfdl logs.
// It returns multiple messages if MIAM decoded content is present (both outer and inner).
func buildMessagesFromNested(obj any) []*acars.Message {
	root, ok := obj.(map[string]any)
	if !ok {
		return nil
	}

	var msgs []*acars.Message

	// First, try to extract outer ACARS message (e.g., MA with compressed text)
	outerLabel := firstString(root,
		"label",
		"message.label",
		"acars.label",
		"vdl2.avlc.acars.label",
		"vdl2.avlc.acars.lbl",
		"hfdl.lpdu.hfnpdu.acars.label",
		"hfdl.lpdu.hfnpdu.acars.acars_label",
	)

	outerText := firstString(root,
		"text",
		"message.text",
		"msg_text",
		"message.msg_text",
		"acars.text",
		"acars.message.text",
		"vdl2.avlc.acars.text",
		"vdl2.avlc.acars.msg_text",
		"vdl2.avlc.acars.message.text",
		"hfdl.lpdu.hfnpdu.acars.text",
		"hfdl.lpdu.hfnpdu.acars.msg_text",
	)

	// Check if MIAM decoded content exists
	miamLabel := firstString(root,
		"vdl2.avlc.acars.miam.single_transfer.miam_core.data.acars.label",
		"hfdl.lpdu.hfnpdu.acars.miam.single_transfer.miam_core.data.acars.label",
	)

	miamText := firstString(root,
		"vdl2.avlc.acars.miam.single_transfer.miam_core.data.acars.message.text",
		"hfdl.lpdu.hfnpdu.acars.miam.single_transfer.miam_core.data.acars.message.text",
	)

	// If we have both outer and MIAM content, create both messages
	if strings.TrimSpace(outerLabel) != "" || strings.TrimSpace(outerText) != "" {
		// Extract common metadata
		tail := firstString(root,
			"tail",
			"airframe.tail",
			"vdl2.avlc.acars.reg",
			"vdl2.avlc.acars.tail",
			"hfdl.lpdu.hfnpdu.acars.reg",
		)

		ts := firstString(root,
			"timestamp",
			"message.timestamp",
		)

		// If no timestamp string, try epoch seconds found in dumpvdl2/dumphfdl logs.
		if ts == "" {
			sec := firstInt64(root,
				"vdl2.t.sec",
				"hfdl.t.sec",
				"t.sec",
			)
			usec := firstInt64(root,
				"vdl2.t.usec",
				"hfdl.t.usec",
				"t.usec",
			)
			if sec > 0 {
				t := time.Unix(sec, usec*1000).UTC()
				ts = t.Format(time.RFC3339Nano)
			}
		}

		freq := firstFloat64(root,
			"frequency",
			"message.frequency",
			"vdl2.freq",
			"hfdl.freq",
		)
		// Many decoder logs store Hz as int (e.g. 136975000). Convert to MHz if large.
		if freq > 1_000_000 {
			freq = freq / 1_000_000.0
		}

		src := firstString(root,
			"source",
			"vdl2.app.name",
			"hfdl.app.name",
			"app.name",
		)

		// Create outer message (e.g., MA with compressed text)
		outerMsg := &acars.Message{
			Label:     outerLabel,
			Text:      outerText,
			Tail:      tail,
			Timestamp: ts,
			Frequency: freq,
			Source:    src,
		}
		msgs = append(msgs, outerMsg)

		// If MIAM decoded content exists, create second message with decoded content
		if strings.TrimSpace(miamLabel) != "" || strings.TrimSpace(miamText) != "" {
			miamMsg := &acars.Message{
				Label:     miamLabel,
				Text:      miamText,
				Tail:      tail,
				Timestamp: ts,
				Frequency: freq,
				Source:    src,
			}
			msgs = append(msgs, miamMsg)
		}
	}

	return msgs
}

func firstString(root map[string]any, paths ...string) string {
	for _, p := range paths {
		if v, ok := deepGet(root, p); ok {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return t
				}
			case float64:
				// Sometimes labels are numeric; preserve as int string where possible.
				if t == float64(int64(t)) {
					return strconv.FormatInt(int64(t), 10)
				}
				return strconv.FormatFloat(t, 'f', -1, 64)
			case bool:
				if t {
					return "true"
				}
				return "false"
			}
		}
	}
	return ""
}

func firstInt64(root map[string]any, paths ...string) int64 {
	for _, p := range paths {
		if v, ok := deepGet(root, p); ok {
			switch t := v.(type) {
			case float64:
				return int64(t)
			case string:
				if i, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
					return i
				}
			}
		}
	}
	return 0
}

func firstFloat64(root map[string]any, paths ...string) float64 {
	for _, p := range paths {
		if v, ok := deepGet(root, p); ok {
			switch t := v.(type) {
			case float64:
				return t
			case string:
				if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
					return f
				}
			}
		}
	}
	return 0
}

// deepGet walks a map[string]any using a dotted path: "a.b.c".
func deepGet(root map[string]any, dotted string) (any, bool) {
	parts := strings.Split(dotted, ".")
	var cur any = root
	for _, part := range parts {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[part]
			if !ok {
				return nil, false
			}
			cur = v
		default:
			return nil, false
		}
	}
	return cur, true
}
