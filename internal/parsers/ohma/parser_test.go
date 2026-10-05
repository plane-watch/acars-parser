package ohma

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"io"
	"testing"

	"acars_parser/internal/acars"
)

// flairReport is a real OHMA report (January 2026) from C-GFOF, Flair
// flight FLE821 from CYEG to CYVR.
const flairReport = "OHMAeJy1Uk2PmzAU/CuVz8HCBgLhRgnsokKCCLv9okJu4qaWDImME2m1yn+vgbCbJkQ9FZ+e38y8eWNewZGKhu1q4AIMdTABFW0asqWqfi3AmjNay2hTALcAy8fEK8CkGCBzImnXwDqeajrSEM6R42LHNRGcWea3DrwhkiiUEiNM7DmpaaPK76qWhPHFofpJRafiaw/hMuwH7DaUd5dd+Yuz7W850DZ0T4Q8COopvZ2QvgL3Al+Dhw5PhGBHwm/7z9mF3sXoMA4cjC56Md2upJqSs+p2Rds1sItm7yvSowppsNcVPidNc45NuSUVlYKt38GjujfR9dCXPX2Lv0yCvH8DVjeS1Ou3MGtyVqxkqowPyymkSuLQw9o4f5wmf6N9Ivhz/w/cwE1ojTCyA6cNlfdItmF/SLwvrut6j4lm2MnH2NH09rM8dWm0ir3mf4sqT1Dpx1GaBPPIW5R+9hStgjJL8/If4SliHiQpUjnPrxYzDARNNLtK40zAYwRkQ2ya44R5FIYjHGRB3bauKGmC0ixYrZ6yYNSYDk3bvs8Z9abDmXNtLfX8T2G8/HzHmw6n1nSEEi3aSXdIGBptBMM5gdMfngFftg=="

func TestParse(t *testing.T) {
	got, ok := (&Parser{}).Parse(&acars.Message{ID: 9, Label: "H1", Tail: "C-GFOF", Text: flairReport}).(*Result)
	if !ok {
		t.Fatal("Parse returned no result")
	}
	want := Result{MsgID: 9, Registration: "C-GFOF", Flight: "FLE821", Origin: "CYEG", Destination: "CYVR",
		MessageDate: "2026-01-12T18:28:41.954Z"}
	if *got != want {
		t.Errorf("got  %+v\nwant %+v", *got, want)
	}
}

// A report about another aircraft is not parsed: its flight and route would
// otherwise be credited to the transmitting aircraft. Without a transmitted
// tail the report is parsed, without a registration.
func TestParseOtherTail(t *testing.T) {
	if r := (&Parser{}).Parse(&acars.Message{ID: 9, Label: "H1", Tail: "C-GFOE", Text: flairReport}); r != nil {
		t.Errorf("got %+v, want nil", r)
	}
	got, ok := (&Parser{}).Parse(&acars.Message{ID: 9, Label: "H1", Text: flairReport}).(*Result)
	if !ok {
		t.Fatal("Parse returned no result without a tail")
	}
	if got.Registration != "" || got.Flight != "FLE821" {
		t.Errorf("got %+v, want flight FLE821 and no registration", *got)
	}
}

// A report that decompresses to more than the limit is rejected, even when
// its first part is valid JSON.
func TestParseRejectsOversized(t *testing.T) {
	var inner bytes.Buffer
	zr, err := zlib.NewReader(bytes.NewReader(mustBase64(t, flairReport[4:])))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(&inner, zr); err != nil {
		t.Fatal(err)
	}
	inner.Write(bytes.Repeat([]byte(" "), maxDecompressed))
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(inner.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	text := "OHMA" + base64.StdEncoding.EncodeToString(compressed.Bytes())
	if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "C-GFOF", Text: text}); r != nil {
		t.Errorf("got %+v, want nil", r)
	}
}

func mustBase64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseRejects(t *testing.T) {
	for name, text := range map[string]string{
		"truncated":   flairReport[:300],
		"not base64":  "OHMA!!!!",
		"not OHMA":    "POSN39006W075260,OYVAY,014938,340",
		"empty after": "OHMA",
	} {
		if r := (&Parser{}).Parse(&acars.Message{Label: "H1", Tail: "C-GFOF", Text: text}); r != nil {
			t.Errorf("%s: got %+v, want nil", name, r)
		}
	}
}
