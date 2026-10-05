package app

import (
	"fmt"
	"strings"
	"sync"
	"time"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// LogEntry is one captured log line shown on the dashboard Logs page.
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Module  string    `json:"module"`
	Message string    `json:"message"`
}

// levelRank orders levels so a logger can drop everything below its minimum.
var levelRank = map[string]int{"DEBUG": 0, "INFO": 1, "WARN": 2, "ERROR": 3}

// LogBuffer is a bounded, in-memory ring of recent log lines. It caps how much
// is kept (and thus how much the UI can request) so the page stays fast.
type LogBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	max     int
}

// NewLogBuffer returns a buffer that keeps at most max entries.
func NewLogBuffer(max int) *LogBuffer {
	if max < 1 {
		max = 1
	}
	return &LogBuffer{max: max}
}

// Add appends one entry, dropping the oldest when the buffer is full.
func (b *LogBuffer) Add(level, module, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, LogEntry{Time: time.Now(), Level: level, Module: module, Message: message})
	if len(b.entries) > b.max {
		b.entries = append(b.entries[:0], b.entries[len(b.entries)-b.max:]...)
	}
}

// Recent returns up to limit newest entries, oldest first. limit<=0 or larger
// than the buffer returns everything held.
func (b *LogBuffer) Recent(limit int) []LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(b.entries)
	if limit <= 0 || limit > n {
		limit = n
	}
	out := make([]LogEntry, limit)
	copy(out, b.entries[n-limit:])
	return out
}

// Logger returns a whatsmeow logger that records into the buffer and still
// writes to stdout, so the Logs page and the process log stay in sync. Only
// lines at or above minLevel are recorded, matching what stdout shows.
func (b *LogBuffer) Logger(module, minLevel string, color bool) waLog.Logger {
	return &bufferLogger{
		buf: b,
		mod: module,
		min: levelRank[strings.ToUpper(minLevel)],
		out: waLog.Stdout(module, minLevel, color),
	}
}

type bufferLogger struct {
	buf *LogBuffer
	mod string
	min int
	out waLog.Logger
}

func (l *bufferLogger) record(level, msg string, args ...any) {
	if levelRank[level] >= l.min {
		l.buf.Add(level, l.mod, fmt.Sprintf(msg, args...))
	}
}

func (l *bufferLogger) Errorf(msg string, args ...any) {
	l.record("ERROR", msg, args...)
	l.out.Errorf(msg, args...)
}
func (l *bufferLogger) Warnf(msg string, args ...any) {
	l.record("WARN", msg, args...)
	l.out.Warnf(msg, args...)
}
func (l *bufferLogger) Infof(msg string, args ...any) {
	l.record("INFO", msg, args...)
	l.out.Infof(msg, args...)
}
func (l *bufferLogger) Debugf(msg string, args ...any) {
	l.record("DEBUG", msg, args...)
	l.out.Debugf(msg, args...)
}
func (l *bufferLogger) Sub(mod string) waLog.Logger {
	return &bufferLogger{buf: l.buf, mod: l.mod + "/" + mod, min: l.min, out: l.out.Sub(mod)}
}
