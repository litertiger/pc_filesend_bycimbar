package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel controls verbosity.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = [...]string{"DEBUG", "INFO ", "WARN ", "ERROR"}

func ParseLogLevel(s string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Logger writes timestamped, levelled messages to one or more writers.
type Logger struct {
	level LogLevel
	mu    sync.Mutex
	out   io.Writer
}

// NewLogger creates a logger that writes to all provided writers simultaneously.
// Pass os.Stderr for console output and a *os.File for file output.
func NewLogger(level LogLevel, writers ...io.Writer) *Logger {
	var w io.Writer
	switch len(writers) {
	case 0:
		w = os.Stderr
	case 1:
		w = writers[0]
	default:
		w = io.MultiWriter(writers...)
	}
	return &Logger{level: level, out: w}
}

func (l *Logger) log(level LogLevel, format string, args ...any) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.out, "%s [%s] %s\n", ts, levelNames[level], msg)
}

func (l *Logger) Debug(format string, args ...any) { l.log(LevelDebug, format, args...) }
func (l *Logger) Info(format string, args ...any)  { l.log(LevelInfo, format, args...) }
func (l *Logger) Warn(format string, args ...any)  { l.log(LevelWarn, format, args...) }
func (l *Logger) Error(format string, args ...any) { l.log(LevelError, format, args...) }
