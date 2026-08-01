package logging

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestParseLevelNormalizesSupportedValues(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{input: "DEBUG", want: LevelDebug},
		{input: " info ", want: LevelInfo},
		{input: "Warn", want: LevelWarn},
		{input: "error", want: LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLevel(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("ParseLevel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseLevelRejectsUnsupportedValue(t *testing.T) {
	_, err := ParseLevel("verbose")
	if err == nil {
		t.Fatal("ParseLevel accepted an unsupported value")
	}
	for _, want := range []string{"verbose", "debug, info, warn, error"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestLevelMapsToSlogLevel(t *testing.T) {
	tests := []struct {
		level Level
		want  slog.Level
	}{
		{level: LevelDebug, want: slog.LevelDebug},
		{level: LevelInfo, want: slog.LevelInfo},
		{level: LevelWarn, want: slog.LevelWarn},
		{level: LevelError, want: slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			if got := tt.level.slogLevel(); got != tt.want {
				t.Fatalf("%q maps to %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestHandlerOptionsUseCompactMetadata(t *testing.T) {
	options := handlerOptions(LevelDebug)
	if options.AddSource {
		t.Fatal("source metadata is enabled")
	}

	timestamp := time.Date(2026, time.August, 1, 12, 1, 47, 473000000, time.FixedZone("BST", 3600))
	formatted := options.ReplaceAttr(nil, slog.Time(slog.TimeKey, timestamp))
	if got, want := formatted.Value.String(), "2026-08-01T12:01:47+01:00"; got != want {
		t.Fatalf("formatted timestamp = %q, want %q", got, want)
	}
}
