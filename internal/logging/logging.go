package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

func ParseLevel(value string) (Level, error) {
	level := Level(strings.ToLower(strings.TrimSpace(value)))
	switch level {
	case LevelDebug, LevelInfo, LevelWarn, LevelError:
		return level, nil
	default:
		return "", fmt.Errorf(
			"invalid value %q; accepted values: debug, info, warn, error",
			value,
		)
	}
}

func New(output io.Writer, level Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(output, handlerOptions(level)))
}

func handlerOptions(level Level) *slog.HandlerOptions {
	return &slog.HandlerOptions{
		Level: level.slogLevel(),
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.String(
					slog.TimeKey,
					attr.Value.Time().Format("2006-01-02T15:04:05Z07:00"),
				)
			}
			return attr
		},
	}
}

func (l Level) slogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
