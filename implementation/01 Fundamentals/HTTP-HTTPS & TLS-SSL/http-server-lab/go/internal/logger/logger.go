package logger

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Level represents log severity. Ordered so that level comparisons work correctly.
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger is a minimal structured logger.
// In production you'd use slog (Go 1.21+) or zap. This teaches the mechanics.
type Logger struct {
	minLevel Level
	out      io.Writer
}

func New(minLevel Level) *Logger {
	return &Logger{
		minLevel: minLevel,
		out:      os.Stdout,
	}
}

// WithWriter allows injecting a writer — critical for testability.
// Tests can pass a bytes.Buffer and assert on log output.
func (l *Logger) WithWriter(w io.Writer) *Logger {
	return &Logger{minLevel: l.minLevel, out: w}
}

func (l *Logger) log(level Level, msg string, fields map[string]any) {
	if level < l.minLevel {
		return
	}
	// Structured log line: timestamp level msg key=value ...
	// Real production: encode as JSON for log aggregators (ELK, Loki)
	line := fmt.Sprintf("time=%s level=%s msg=%q", time.Now().Format(time.RFC3339), level, msg)
	for k, v := range fields {
		line += fmt.Sprintf(" %s=%v", k, v)
	}
	fmt.Fprintln(l.out, line)
}

func (l *Logger) Info(msg string, fields map[string]any)  { l.log(INFO, msg, fields) }
func (l *Logger) Debug(msg string, fields map[string]any) { l.log(DEBUG, msg, fields) }
func (l *Logger) Warn(msg string, fields map[string]any)  { l.log(WARN, msg, fields) }
func (l *Logger) Error(msg string, fields map[string]any) { l.log(ERROR, msg, fields) }