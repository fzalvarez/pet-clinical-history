package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return Debug
	case "info", "":
		return Info
	case "warn", "warning":
		return Warn
	case "error":
		return Error
	default:
		return Info
	}
}

func (l Level) String() string {
	switch l {
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Warn:
		return "warn"
	case Error:
		return "error"
	default:
		return "info"
	}
}

type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

func ParseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return FormatJSON
	default:
		return FormatText
	}
}

type Logger interface {
	With(fields map[string]any) Logger

	Debug(msg string, fields map[string]any)
	Info(msg string, fields map[string]any)
	Warn(msg string, fields map[string]any)
	Error(msg string, fields map[string]any)
}

// StdLogger es un logger minimalista sin deps externas (sirve como base para Odin/Plans).
type StdLogger struct {
	mu     sync.Mutex
	std    *log.Logger
	level  Level
	format Format
	base   map[string]any
}

type Options struct {
	Level  Level
	Format Format
	App    string
}

func New(opts Options) Logger {
	l := log.New(os.Stdout, "", 0)

	base := map[string]any{}
	if strings.TrimSpace(opts.App) != "" {
		base["app"] = strings.TrimSpace(opts.App)
	}

	return &StdLogger{
		std:   l,
		level: opts.Level,
		format: func() Format {
			if opts.Format == "" {
				return FormatText
			}
			return opts.Format
		}(),
		base: base,
	}
}

// NewFromEnv crea logger desde env:
// - LOG_LEVEL=debug|info|warn|error (default info)
// - LOG_FORMAT=text|json (default text)
// - APP_NAME=pet-clinical-history (opcional)
func NewFromEnv() Logger {
	return New(Options{
		Level:  ParseLevel(os.Getenv("LOG_LEVEL")),
		Format: ParseFormat(os.Getenv("LOG_FORMAT")),
		App:    os.Getenv("APP_NAME"),
	})
}

func (l *StdLogger) With(fields map[string]any) Logger {
	if len(fields) == 0 {
		return l
	}

	merged := map[string]any{}
	for k, v := range l.base {
		merged[k] = v
	}
	for k, v := range fields {
		if strings.TrimSpace(k) == "" {
			continue
		}
		merged[k] = v
	}

	// shallow copy del logger (comparte std, level, format)
	return &StdLogger{
		std:    l.std,
		level:  l.level,
		format: l.format,
		base:   merged,
	}
}

func (l *StdLogger) Debug(msg string, fields map[string]any) { l.log(Debug, msg, fields) }
func (l *StdLogger) Info(msg string, fields map[string]any)  { l.log(Info, msg, fields) }
func (l *StdLogger) Warn(msg string, fields map[string]any)  { l.log(Warn, msg, fields) }
func (l *StdLogger) Error(msg string, fields map[string]any) { l.log(Error, msg, fields) }

func (l *StdLogger) log(lvl Level, msg string, fields map[string]any) {
	if lvl < l.level {
		return
	}

	entry := map[string]any{
		"ts":    time.Now().Format(time.RFC3339Nano),
		"level": lvl.String(),
		"msg":   msg,
	}

	for k, v := range l.base {
		entry[k] = v
	}
	for k, v := range fields {
		if strings.TrimSpace(k) == "" {
			continue
		}
		entry[k] = v
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	switch l.format {
	case FormatJSON:
		b, _ := json.Marshal(entry)
		l.std.Println(string(b))
	default:
		l.std.Println(formatText(entry))
	}
}

func formatText(m map[string]any) string {
	// Ordenar keys para salida estable (útil en tests/logs).
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, m[k]))
	}
	return strings.Join(parts, " ")
}
