package relay

import (
	"testing"
	"time"
)

func TestTTLCache_AddAndContains(t *testing.T) {
	c := NewTTLCache[string](100, 10*time.Second)

	// First add should return false (not a duplicate).
	if c.Add("msg-1") {
		t.Error("expected first add to return false (not duplicate)")
	}

	// Second add of the same key should return true (duplicate).
	if !c.Add("msg-1") {
		t.Error("expected second add to return true (duplicate)")
	}

	// Different key should return false.
	if c.Add("msg-2") {
		t.Error("expected new key to return false")
	}
}

func TestTTLCache_Expiry(t *testing.T) {
	c := NewTTLCache[string](100, 50*time.Millisecond)

	c.Add("msg-1")

	// Should be a duplicate immediately.
	if !c.Add("msg-1") {
		t.Error("expected duplicate before expiry")
	}

	// Wait for expiry.
	time.Sleep(60 * time.Millisecond)

	// Sweep expired entries.
	c.Sweep()

	// Should no longer be a duplicate after expiry.
	if c.Add("msg-1") {
		t.Error("expected non-duplicate after expiry and sweep")
	}
}

func TestTTLCache_MaxSize(t *testing.T) {
	c := NewTTLCache[int](5, 10*time.Second)

	// Fill the cache to capacity.
	for i := 0; i < 5; i++ {
		c.Add(i)
	}

	if c.Len() != 5 {
		t.Errorf("expected cache length 5, got %d", c.Len())
	}

	// Adding one more should trigger eviction of expired entries,
	// and if none are expired, allow the add anyway (bounded, not blocking).
	c.Add(5)

	// Cache should not exceed max size after eviction attempt.
	if c.Len() > 5 {
		t.Errorf("expected cache length <= 5 after eviction, got %d", c.Len())
	}
}

func TestTTLCache_SweepRemovesExpired(t *testing.T) {
	c := NewTTLCache[string](100, 50*time.Millisecond)

	c.Add("a")
	c.Add("b")

	time.Sleep(60 * time.Millisecond)

	// Add a fresh entry.
	c.Add("c")

	swept := c.Sweep()

	if swept != 2 {
		t.Errorf("expected 2 swept entries, got %d", swept)
	}

	if c.Len() != 1 {
		t.Errorf("expected 1 remaining entry, got %d", c.Len())
	}
}

func TestTTLCache_Uint64Key(t *testing.T) {
	// Verify the cache works with uint64 keys (for content hashes).
	c := NewTTLCache[uint64](100, 10*time.Second)

	if c.Add(0xDEADBEEF) {
		t.Error("expected first add to return false")
	}

	if !c.Add(0xDEADBEEF) {
		t.Error("expected second add to return true (duplicate)")
	}
}

func TestContentHash(t *testing.T) {
	// Same inputs produce the same hash.
	h1 := ContentHash("H1", "VH-ABC", "SOME MESSAGE TEXT")
	h2 := ContentHash("H1", "VH-ABC", "SOME MESSAGE TEXT")
	if h1 != h2 {
		t.Error("expected identical inputs to produce identical hash")
	}

	// Different inputs produce different hashes.
	h3 := ContentHash("H1", "VH-ABC", "DIFFERENT TEXT")
	if h1 == h3 {
		t.Error("expected different inputs to produce different hash")
	}

	// Order matters - swapping fields changes the hash.
	h4 := ContentHash("VH-ABC", "H1", "SOME MESSAGE TEXT")
	if h1 == h4 {
		t.Error("expected swapped fields to produce different hash")
	}
}

func TestExtractDedupFields(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		wantLabel string
		wantTail  string
		wantText  string
		wantErr   bool
	}{
		{
			name:      "standard NATS wrapper message",
			data:      `{"message":{"label":"H1","tail":"VH-ABC","text":"SOME DATA"}}`,
			wantLabel: "H1",
			wantTail:  "VH-ABC",
			wantText:  "SOME DATA",
		},
		{
			name:      "message with extra fields ignored",
			data:      `{"station":{"ident":"YSSY"},"message":{"label":"80","tail":"N12345","text":"POS DATA","id":12345}}`,
			wantLabel: "80",
			wantTail:  "N12345",
			wantText:  "POS DATA",
		},
		{
			name:      "empty message object",
			data:      `{"message":{}}`,
			wantLabel: "",
			wantTail:  "",
			wantText:  "",
		},
		{
			name:    "invalid JSON",
			data:    `not json`,
			wantErr: true,
		},
		{
			name: "no message field",
			data: `{"station":{"ident":"YSSY"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, tail, text, err := ExtractDedupFields([]byte(tt.data))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if label != tt.wantLabel {
				t.Errorf("label: got %q, want %q", label, tt.wantLabel)
			}
			if tail != tt.wantTail {
				t.Errorf("tail: got %q, want %q", tail, tt.wantTail)
			}
			if text != tt.wantText {
				t.Errorf("text: got %q, want %q", text, tt.wantText)
			}
		})
	}
}
