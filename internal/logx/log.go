package logx

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var once sync.Once

func InitFromEnv() {
	once.Do(func() {
		slog.SetDefault(slog.New(newHandler(os.Stderr, os.Getenv("FOUNDRY_LOG"), os.Getenv("FOUNDRY_LOG_FORMAT"))))
	})
}

func newHandler(w io.Writer, level, format string) slog.Handler {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	if strings.EqualFold(strings.TrimSpace(format), "json") {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

func parseLevel(v string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}
