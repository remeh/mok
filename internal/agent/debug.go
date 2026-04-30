package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// DebugLogger provides centralized debug logging for the agent system.
type DebugLogger struct {
	enabled bool
	out     io.Writer
	start   time.Time
}

// NewDebugLogger creates a debug logger. If enabled is false, all logging is a no-op.
func NewDebugLogger(enabled bool) *DebugLogger {
	return &DebugLogger{
		enabled: enabled,
		out:     os.Stderr,
		start:   time.Now(),
	}
}

// log formats and writes a debug line. Returns immediately if disabled.
func (d *DebugLogger) log(category, msg string) {
	if !d.enabled {
		return
	}
	elapsed := time.Since(d.start).Truncate(time.Millisecond)
	fmt.Fprintf(d.out, "[%s] %s: %s\n", elapsed, category, msg)
}

// Debug logs a verbose debug message.
func (d *DebugLogger) Debug(category, format string, args ...any) {
	d.log(category, fmt.Sprintf(format, args...))
}

// Info logs an informational message.
func (d *DebugLogger) Info(category, format string, args ...any) {
	d.log(category, fmt.Sprintf(format, args...))
}

// Request logs an HTTP/API request detail.
func (d *DebugLogger) Request(category, format string, args ...any) {
	d.log(category, fmt.Sprintf(format, args...))
}

// Response logs an HTTP/API response detail.
func (d *DebugLogger) Response(category, format string, args ...any) {
	d.log(category, fmt.Sprintf(format, args...))
}

// Event logs an agent event.
func (d *DebugLogger) Event(category, format string, args ...any) {
	d.log(category, fmt.Sprintf(format, args...))
}

// Tool logs a tool execution detail.
func (d *DebugLogger) Tool(category, format string, args ...any) {
	d.log(category, fmt.Sprintf(format, args...))
}

// JSON pretty-prints a value as JSON with indentation.
func (d *DebugLogger) JSON(category, label string, v any) {
	if !d.enabled {
		return
	}
	data, err := json.MarshalIndent(v, "  ", "  ")
	if err != nil {
		d.log(category, fmt.Sprintf("%s: <marshal error: %v>", label, err))
		return
	}
	d.log(category, fmt.Sprintf("%s:\n  %s", label, string(data)))
}

// Dump writes raw bytes with a label, truncating if too large.
func (d *DebugLogger) Dump(category, label string, data []byte) {
	if !d.enabled {
		return
	}
	maxDump := 2048
	if len(data) > maxDump {
		d.log(category, fmt.Sprintf("%s: %d bytes (truncated)\n%s...", label, len(data), string(data[:maxDump])))
	} else {
		d.log(category, fmt.Sprintf("%s: %d bytes\n%s", label, len(data), string(data)))
	}
}

// Separator prints a visual separator for log sections.
func (d *DebugLogger) Separator(category string) {
	d.log(category, strings.Repeat("---", 10))
}
