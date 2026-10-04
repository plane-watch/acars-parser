package label80

import (
	"testing"

	"acars_parser/internal/acars"
)

// TestParseTailStopsAtTheSlash checks that the tail does not include the
// field that follows it ("HK-5365/01F"). The message is real, from the
// January 2026 corpus.
func TestParseTailStopsAtTheSlash(t *testing.T) {
	text := "3N01 POSRPT 0254/04 SKBO/CYYZ HK-5365/01F 10:55\r\n/WYP VERKO"
	r, ok := (&Parser{}).Parse(&acars.Message{ID: 1, Label: "80", Text: text}).(*Result)
	if !ok {
		t.Fatal("Parse returned no result")
	}
	if r.Tail != "HK-5365" {
		t.Errorf("Tail = %q, want HK-5365", r.Tail)
	}
}
