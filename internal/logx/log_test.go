package logx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"other":   slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Fatalf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestLoggingWrappers(t *testing.T) {
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")
}

func TestInitFromEnv(t *testing.T) {
	once = sync.Once{}
	t.Setenv("FOUNDRY_LOG", "debug")
	InitFromEnv()

	if os.Getenv("FOUNDRY_LOG") != "debug" {
		t.Fatal("expected env to remain available")
	}
}

func TestJSONHandler(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(newHandler(&out, "warn", " JSON "))
	logger.Info("filtered")
	logger.Warn("backup failed", "operation", "backup", "count", 2)
	var entry map[string]any
	if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "WARN" || entry["operation"] != "backup" || entry["count"] != float64(2) || entry["time"] == nil {
		t.Fatalf("unexpected JSON: %s", out.String())
	}
	out.Reset()
	slog.New(newHandler(&out, "info", "unknown")).Info("text fallback")
	if !strings.Contains(out.String(), "level=INFO") {
		t.Fatal(out.String())
	}
}
