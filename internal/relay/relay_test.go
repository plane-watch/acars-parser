package relay

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// mockPublisher records all published messages for assertion.
type mockPublisher struct {
	mu       sync.Mutex
	messages []publishedMsg
}

type publishedMsg struct {
	subject string
	data    []byte
}

func (m *mockPublisher) Publish(subject string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, publishedMsg{subject: subject, data: data})
	return nil
}

func (m *mockPublisher) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func TestHandleMessage_PublishesUniqueMessage(t *testing.T) {
	pub := &mockPublisher{}
	r := newTestRelay(pub)

	data := []byte(`{"message":{"label":"H1","tail":"VH-ABC","text":"POSITION REPORT"}}`)
	subject := "v1.aircraft.ingest.source1.message.msg-001.created"

	r.handleMessage(subject, data)

	if pub.count() != 1 {
		t.Errorf("expected 1 published message, got %d", pub.count())
	}
}

func TestHandleMessage_SubjectIDDedup(t *testing.T) {
	pub := &mockPublisher{}
	r := newTestRelay(pub)

	data := []byte(`{"message":{"label":"H1","tail":"VH-ABC","text":"POSITION REPORT"}}`)
	subject := "v1.aircraft.ingest.source1.message.msg-001.created"

	// First message should be published.
	r.handleMessage(subject, data)
	// Same subject (same message ID) should be deduplicated.
	r.handleMessage(subject, data)

	if pub.count() != 1 {
		t.Errorf("expected 1 published message (subject dedup), got %d", pub.count())
	}
}

func TestHandleMessage_ContentHashDedup(t *testing.T) {
	pub := &mockPublisher{}
	r := newTestRelay(pub)

	data := []byte(`{"message":{"label":"H1","tail":"VH-ABC","text":"POSITION REPORT"}}`)

	// Same content but different message IDs (different ground stations).
	subject1 := "v1.aircraft.ingest.source1.message.msg-001.created"
	subject2 := "v1.aircraft.ingest.source2.message.msg-002.created"

	r.handleMessage(subject1, data)
	r.handleMessage(subject2, data)

	if pub.count() != 1 {
		t.Errorf("expected 1 published message (content dedup), got %d", pub.count())
	}
}

func TestHandleMessage_DifferentContentPublished(t *testing.T) {
	pub := &mockPublisher{}
	r := newTestRelay(pub)

	data1 := []byte(`{"message":{"label":"H1","tail":"VH-ABC","text":"REPORT ONE"}}`)
	data2 := []byte(`{"message":{"label":"H1","tail":"VH-ABC","text":"REPORT TWO"}}`)
	subject1 := "v1.aircraft.ingest.source1.message.msg-001.created"
	subject2 := "v1.aircraft.ingest.source1.message.msg-002.created"

	r.handleMessage(subject1, data1)
	r.handleMessage(subject2, data2)

	if pub.count() != 2 {
		t.Errorf("expected 2 published messages, got %d", pub.count())
	}
}

func TestHandleMessage_InvalidJSON(t *testing.T) {
	pub := &mockPublisher{}
	r := newTestRelay(pub)

	// Invalid JSON should still be published (layer 1 dedup only, layer 2 skipped).
	data := []byte(`not valid json`)
	subject := "v1.aircraft.ingest.source1.message.msg-001.created"

	r.handleMessage(subject, data)

	// Message should be published even if JSON parsing fails - the relay
	// should not drop messages it can't parse.
	if pub.count() != 1 {
		t.Errorf("expected 1 published message (invalid JSON still relayed), got %d", pub.count())
	}
}

func TestHandleMessage_ShortSubjectNoDedup(t *testing.T) {
	pub := &mockPublisher{}
	r := newTestRelay(pub)

	// Subject too short to extract message ID - layer 1 skipped.
	data := []byte(`{"message":{"label":"H1","tail":"VH-ABC","text":"DATA"}}`)
	subject := "some.short.subject"

	r.handleMessage(subject, data)
	r.handleMessage(subject, data)

	// Without subject ID dedup, content hash dedup should still catch the duplicate.
	if pub.count() != 1 {
		t.Errorf("expected 1 published message (content dedup fallback), got %d", pub.count())
	}
}

func TestExtractSubjectMsgID(t *testing.T) {
	tests := []struct {
		subject string
		wantID  string
		wantOK  bool
	}{
		{
			subject: "v1.aircraft.ingest.source1.message.12345.created",
			wantID:  "12345",
			wantOK:  true,
		},
		{
			subject: "v1.aircraft.ingest.feedA.message.abc-def.created",
			wantID:  "abc-def",
			wantOK:  true,
		},
		{
			subject: "too.short",
			wantID:  "",
			wantOK:  false,
		},
		{
			subject: "",
			wantID:  "",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.subject, func(t *testing.T) {
			id, ok := ExtractSubjectMsgID(tt.subject)
			if ok != tt.wantOK {
				t.Errorf("ok: got %v, want %v", ok, tt.wantOK)
			}
			if id != tt.wantID {
				t.Errorf("id: got %q, want %q", id, tt.wantID)
			}
		})
	}
}

// newTestRelay creates a Relay with test-friendly settings.
func newTestRelay(pub Publisher) *Relay {
	return &Relay{
		cfg: Config{
			InternalSubject: "test.messages",
			DedupTTL:        10 * time.Second,
			DedupMaxSize:    1000,
		},
		downstream:   pub,
		subjectCache: NewTTLCache[string](1000, 10*time.Second),
		contentCache: NewTTLCache[uint64](1000, 10*time.Second),
		metrics:      NewMetricsWithRegistry(prometheus.NewRegistry()),
		logger:       newLogger("debug"),
	}
}
