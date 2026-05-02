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

// NewDebugLoggerFile creates a debug logger that writes to the given file path.
// The file is created or truncated on open. If enabled is false, all logging is a no-op.
func NewDebugLoggerFile(enabled bool, path string) *DebugLogger {
	if !enabled {
		return &DebugLogger{
			enabled: false,
			out:     io.Discard,
			start:   time.Now(),
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		// Fall back to stderr if file open fails
		return &DebugLogger{
			enabled: true,
			out:     os.Stderr,
			start:   time.Now(),
		}
	}
	return &DebugLogger{
		enabled: true,
		out:     f,
		start:   time.Now(),
	}
}

// log formats and writes a debug line. Returns immediately if disabled or nil.
func (d *DebugLogger) log(category, msg string) {
	if d == nil || !d.enabled {
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
	if d == nil || !d.enabled {
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
	if d == nil || !d.enabled {
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

// Close releases any resources held by the debug logger.
func (d *DebugLogger) Close() {
	if d == nil || d.out == nil {
		return
	}
	if closer, ok := d.out.(io.Closer); ok {
		_ = closer.Close()
	}
}
