// Package relay implements a NATS message relay with deduplication.
//
// The relay connects to an upstream NATS server (Airframes), deduplicates
// messages using a two-layer strategy (subject ID + content hash), and
// republishes unique messages to an internal NATS server.
package relay

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sync"
	"time"
)

// TTLCache is a thread-safe cache with time-to-live expiry and bounded size.
// It supports any comparable key type, allowing reuse for both string-based
// subject ID dedup and uint64-based content hash dedup.
type TTLCache[K comparable] struct {
	entries map[K]int64 // key -> expiry timestamp (unix nanoseconds)
	maxSize int
	ttl     time.Duration
	mu      sync.Mutex
}

// NewTTLCache creates a new TTL cache with the given maximum size and TTL duration.
func NewTTLCache[K comparable](maxSize int, ttl time.Duration) *TTLCache[K] {
	return &TTLCache[K]{
		entries: make(map[K]int64, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Add inserts a key into the cache. Returns true if the key was already present
// and not expired (i.e., this is a duplicate). Returns false if the key is new
// or had expired.
func (c *TTLCache[K]) Add(key K) bool {
	now := time.Now().UnixNano()
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if the key exists and hasn't expired.
	if expiry, exists := c.entries[key]; exists && now < expiry {
		return true // Duplicate.
	}

	// Evict expired entries if at capacity.
	if len(c.entries) >= c.maxSize {
		c.sweepLocked(now)
	}

	// If still at capacity after sweeping (no expired entries), evict the
	// oldest entry to maintain the size bound.
	if len(c.entries) >= c.maxSize {
		c.evictOldestLocked()
	}

	c.entries[key] = now + int64(c.ttl)
	return false
}

// Len returns the current number of entries in the cache.
func (c *TTLCache[K]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Sweep removes all expired entries and returns the number removed.
func (c *TTLCache[K]) Sweep() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sweepLocked(time.Now().UnixNano())
}

// sweepLocked removes expired entries. Must be called with the lock held.
func (c *TTLCache[K]) sweepLocked(now int64) int {
	swept := 0
	for key, expiry := range c.entries {
		if now >= expiry {
			delete(c.entries, key)
			swept++
		}
	}
	return swept
}

// evictOldestLocked removes the entry with the earliest expiry time.
// Must be called with the lock held.
func (c *TTLCache[K]) evictOldestLocked() {
	var oldestKey K
	var oldestExpiry int64
	first := true
	for key, expiry := range c.entries {
		if first || expiry < oldestExpiry {
			oldestKey = key
			oldestExpiry = expiry
			first = false
		}
	}
	if !first {
		delete(c.entries, oldestKey)
	}
}

// ContentHash computes an FNV-64a hash over the given label, tail, and text fields.
// A null byte separator is used between fields to prevent collisions from
// concatenation (e.g., label="AB" + tail="C" vs label="A" + tail="BC").
func ContentHash(label, tail, text string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(label))
	h.Write([]byte{0})
	h.Write([]byte(tail))
	h.Write([]byte{0})
	h.Write([]byte(text))
	return h.Sum64()
}

// dedupPayload is a minimal struct for extracting only the fields needed
// for content-based deduplication from the Airframes NATS JSON payload.
// This avoids coupling to the full acars.NATSWrapper type.
type dedupPayload struct {
	Message *dedupMessage `json:"message"`
}

type dedupMessage struct {
	Label string `json:"label"`
	Tail  string `json:"tail"`
	Text  string `json:"text"`
}

// ExtractDedupFields parses the minimum fields needed for content deduplication
// from a raw Airframes NATS message payload. Returns the label, tail, and text
// fields from the nested message object.
func ExtractDedupFields(data []byte) (label, tail, text string, err error) {
	var payload dedupPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", "", "", fmt.Errorf("unmarshal dedup fields: %w", err)
	}
	if payload.Message == nil {
		return "", "", "", nil
	}
	return payload.Message.Label, payload.Message.Tail, payload.Message.Text, nil
}
