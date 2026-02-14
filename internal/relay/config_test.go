package relay

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear any env vars that might interfere.
	envVars := []string{
		"AIRFRAMES_NATS_URL", "AIRFRAMES_NATS_CREDS", "AIRFRAMES_NATS_SUBJECT",
		"INTERNAL_NATS_URL", "INTERNAL_NATS_SUBJECT",
		"DEDUP_TTL", "DEDUP_MAX_SIZE", "METRICS_ADDR", "LOG_LEVEL",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg := LoadConfig()

	if cfg.AirframesURL != "nats://157.90.242.138:4222" {
		t.Errorf("AirframesURL: got %q, want default", cfg.AirframesURL)
	}
	if cfg.AirframesSubject != "v1.aircraft.ingest.*.message.*.created" {
		t.Errorf("AirframesSubject: got %q, want default", cfg.AirframesSubject)
	}
	if cfg.InternalURL != "nats://localhost:4222" {
		t.Errorf("InternalURL: got %q, want default", cfg.InternalURL)
	}
	if cfg.InternalSubject != "acars.messages" {
		t.Errorf("InternalSubject: got %q, want default", cfg.InternalSubject)
	}
	if cfg.DedupTTL != 10*time.Second {
		t.Errorf("DedupTTL: got %v, want 10s", cfg.DedupTTL)
	}
	if cfg.DedupMaxSize != 100000 {
		t.Errorf("DedupMaxSize: got %d, want 100000", cfg.DedupMaxSize)
	}
	if cfg.MetricsAddr != ":9090" {
		t.Errorf("MetricsAddr: got %q, want :9090", cfg.MetricsAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want info", cfg.LogLevel)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	t.Setenv("AIRFRAMES_NATS_URL", "nats://custom:4222")
	t.Setenv("AIRFRAMES_NATS_CREDS", "/path/to/creds")
	t.Setenv("INTERNAL_NATS_SUBJECT", "custom.subject")
	t.Setenv("DEDUP_TTL", "5s")
	t.Setenv("DEDUP_MAX_SIZE", "50000")
	t.Setenv("LOG_LEVEL", "debug")

	cfg := LoadConfig()

	if cfg.AirframesURL != "nats://custom:4222" {
		t.Errorf("AirframesURL: got %q, want custom", cfg.AirframesURL)
	}
	if cfg.AirframesCreds != "/path/to/creds" {
		t.Errorf("AirframesCreds: got %q, want /path/to/creds", cfg.AirframesCreds)
	}
	if cfg.InternalSubject != "custom.subject" {
		t.Errorf("InternalSubject: got %q, want custom.subject", cfg.InternalSubject)
	}
	if cfg.DedupTTL != 5*time.Second {
		t.Errorf("DedupTTL: got %v, want 5s", cfg.DedupTTL)
	}
	if cfg.DedupMaxSize != 50000 {
		t.Errorf("DedupMaxSize: got %d, want 50000", cfg.DedupMaxSize)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel: got %q, want debug", cfg.LogLevel)
	}
}

func TestLoadConfig_InvalidDedupTTL(t *testing.T) {
	t.Setenv("DEDUP_TTL", "not-a-duration")

	cfg := LoadConfig()

	// Should fall back to default on invalid input.
	if cfg.DedupTTL != 10*time.Second {
		t.Errorf("DedupTTL: got %v, want 10s (default) on invalid input", cfg.DedupTTL)
	}
}

func TestLoadConfig_InvalidDedupMaxSize(t *testing.T) {
	t.Setenv("DEDUP_MAX_SIZE", "not-a-number")

	cfg := LoadConfig()

	if cfg.DedupMaxSize != 100000 {
		t.Errorf("DedupMaxSize: got %d, want 100000 (default) on invalid input", cfg.DedupMaxSize)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				AirframesURL:     "nats://host:4222",
				AirframesCreds:   "/path/to/creds",
				AirframesSubject: "v1.aircraft.ingest.*.message.*.created",
				InternalURL:      "nats://localhost:4222",
				InternalSubject:  "acars.messages",
				DedupTTL:         10 * time.Second,
				DedupMaxSize:     100000,
				MetricsAddr:      ":9090",
				LogLevel:         "info",
			},
			wantErr: false,
		},
		{
			name: "missing credentials",
			cfg: Config{
				AirframesURL:    "nats://host:4222",
				AirframesCreds:  "",
				InternalURL:     "nats://localhost:4222",
				InternalSubject: "acars.messages",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveCredsFile_FilePath(t *testing.T) {
	// Create a temporary credentials file.
	tmpFile, err := os.CreateTemp("", "test-creds-*.creds")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("-----BEGIN NATS USER JWT-----\ntest\n-----END NATS USER JWT-----\n")
	tmpFile.Close()

	path, cleanup, err := ResolveCredsFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if path != tmpFile.Name() {
		t.Errorf("expected path %q, got %q", tmpFile.Name(), path)
	}
}

func TestResolveCredsFile_InlineContent(t *testing.T) {
	inlineCreds := "-----BEGIN NATS USER JWT-----\ntest-jwt\n-----END NATS USER JWT-----\n-----BEGIN USER NKEY SEED-----\ntest-seed\n-----END USER NKEY SEED-----\n"

	path, cleanup, err := ResolveCredsFile(inlineCreds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	// Should have written to a temp file.
	if path == inlineCreds {
		t.Error("expected temp file path, got inline content back")
	}

	// The temp file should contain the inline content.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("error reading temp file: %v", err)
	}
	if string(content) != inlineCreds {
		t.Errorf("temp file content mismatch")
	}
}
