package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"acars_parser/internal/reassembly"
	"acars_parser/internal/registry"
)

// ohmaReport is a real OHMA report (January 2026), which no parser
// recognises when it is split into blocks.
const ohmaReport = "OHMAeJy1Uk2PmzAU/CuVz8HCBgLhRgnsokKCCLv9okJu4qaWDImME2m1yn+vgbCbJkQ9FZ+e38y8eWNewZGKhu1q4AIMdTABFW0asqWqfi3AmjNay2hTALcAy8fEK8CkGCBzImnXwDqeajrSEM6R42LHNRGcWea3DrwhkiiUEiNM7DmpaaPK76qWhPHFofpJRafiaw/hMuwH7DaUd5dd+Yuz7W850DZ0T4Q8COopvZ2QvgL3Al+Dhw5PhGBHwm/7z9mF3sXoMA4cjC56Md2upJqSs+p2Rds1sItm7yvSowppsNcVPidNc45NuSUVlYKt38GjujfR9dCXPX2Lv0yCvH8DVjeS1Ou3MGtyVqxkqowPyymkSuLQw9o4f5wmf6N9Ivhz/w/cwE1ojTCyA6cNlfdItmF/SLwvrut6j4lm2MnH2NH09rM8dWm0ir3mf4sqT1Dpx1GaBPPIW5R+9hStgjJL8/If4SliHiQpUjnPrxYzDARNNLtK40zAYwRkQ2ya44R5FIYjHGRB3bauKGmC0ixYrZ6yYNSYDk3bvs8Z9abDmXNtLfX8T2G8/HzHmw6n1nSEEi3aSXdIGBptBMM5gdMfngFftg=="

// feedBlock returns a feed message (as Airframes sends it) for one block.
func feedBlock(t *testing.T, id int, number, text string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]interface{}{
		"station": map[string]string{"id": "237341468"},
		"message": map[string]interface{}{
			"id": id, "timestamp": "2026-10-23T15:05:00Z", "label": "H1", "tail": "C-GFOF",
			"message_number": number, "block_id": "5", "block_end": false, "text": text,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestLiveJoinsBlocks checks that live parses a message split into blocks
// once its blocks have settled, as well as each block as received.
func TestLiveJoinsBlocks(t *testing.T) {
	reg := registry.Default()
	reg.Sort()
	clock := time.Date(2026, 10, 23, 15, 5, 0, 0, time.UTC)
	h := &liveHandler{
		reg:          reg,
		stats:        &liveStats{typeCounts: map[string]int{}},
		debugLabels:  map[string]bool{},
		excludeTypes: map[string]bool{},
		assembler:    reassembly.New(reassemblySettle, reassemblyIdle, reassemblyMaxPending),
		now:          func() time.Time { return clock },
	}
	ctx := context.Background()
	h.handle(ctx, feedBlock(t, 100, "D47A", ohmaReport[:210]))
	h.handle(ctx, feedBlock(t, 101, "D47B", ohmaReport[210:420]))
	h.handle(ctx, feedBlock(t, 102, "D47C", ohmaReport[420:]))

	// The flush loop judges the blocks on its ticks, with no further message.
	// The tick channel is unbuffered and the loop handles one tick at a
	// time, so a second tick at the same time returns only once the first
	// tick's flush is done.
	tick := make(chan time.Time)
	loopCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		h.flushLoop(loopCtx, tick)
		close(done)
	}()
	start := clock
	joinedAt := func(at time.Time) int {
		h.mu.Lock()
		clock = at
		h.mu.Unlock()
		tick <- at
		tick <- at
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.stats.assembled
	}
	if n := joinedAt(start.Add(reassemblySettle - time.Second)); n != 0 {
		t.Errorf("before the blocks settled: %d joined, want 0", n)
	}
	if n := joinedAt(start.Add(reassemblySettle)); n != 1 {
		t.Errorf("after the blocks settled: %d joined, want 1", n)
	}
	stop()
	<-done
	if h.stats.typeCounts["ohma"] != 1 {
		t.Errorf("%d ohma results, want 1", h.stats.typeCounts["ohma"])
	}
}
