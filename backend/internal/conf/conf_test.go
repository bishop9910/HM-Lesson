package conf

import (
	"log/slog"
	"testing"
)

func TestLogLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
	}
	for _, c := range cases {
		cfg := Config{LogLevelName: c.in}
		if got := cfg.LogLevel(); got != c.want {
			t.Fatalf("LogLevel(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestEnvFallback(t *testing.T) {
	t.Setenv("HM_ENV", "")
	if got := Load().Env; got != "dev" {
		t.Fatalf("Env = %q, want dev", got)
	}
	t.Setenv("HM_ENV", "prod")
	if got := Load().Env; got != "prod" {
		t.Fatalf("Env = %q, want prod", got)
	}
}
