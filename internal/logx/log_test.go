package logx

import (
	"log/slog"
	"os"
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
