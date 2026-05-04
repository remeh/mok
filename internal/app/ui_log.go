package app

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/user/mmok/internal/agent"
	"github.com/user/mmok/internal/types"
)

// UILogWriter manages real-time and post-session logging of TUI activity.
//
// It supports two modes:
//   - Real-time: events are written as they arrive, buffered per-turn so
//     streaming deltas are written atomically at turn end.
//   - Export: all messages are written in expanded form (no collapsing) to
//     a file at a user-chosen path.
//
// The writer is safe for concurrent use (the agent goroutine and the
// bubbletea event loop may both call WriteEvent).
type UILogWriter struct {
	mu   sync.Mutex
	file *os.File

	// Session metadata written in the header.
	model    string
	endpoint string
	started  time.Time

	// Turn buffer: collects deltas during a turn so we can write them
	// atomically at the end of the turn without partial-streaming artifacts.
	turnBuf []string
}

// UILogWriterOption configures a UILogWriter.
type UILogWriterOption func(*UILogWriterOptions)

type UILogWriterOptions struct {
	truncate bool
}

// WithTruncate sets the file open mode to truncate (O_TRUNC) instead of append (O_APPEND).
func WithTruncate() UILogWriterOption {
	return func(o *UILogWriterOptions) { o.truncate = true }
}

// NewUILogWriter creates a log writer that appends to the given file path.
// If the file cannot be opened, a nil writer is returned along with an error.
func NewUILogWriter(path, model, endpoint string, opts ...UILogWriterOption) (*UILogWriter, error) {
	// Apply options.
	var cfg UILogWriterOptions
	for _, opt := range opts {
		opt(&cfg)
	}

	var flags int
	if cfg.truncate {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	} else {
		flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}

	f, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening ui log %s: %w", path, err)
	}

	w := &UILogWriter{
		file:     f,
		model:    model,
		endpoint: endpoint,
		started:  time.Now(),
	}
	w.WriteHeader()
	return w, nil
}

// WriteHeader writes the session header to the log.
func (w *UILogWriter) WriteHeader() {
	fmt.Fprintln(w.file, strings.Repeat("=", 60))
	fmt.Fprintln(w.file, "mmok TUI Session Log")
	fmt.Fprintln(w.file, strings.Repeat("=", 60))
	fmt.Fprintf(w.file, "Model:    %s\n", w.model)
	fmt.Fprintf(w.file, "Endpoint: %s\n", w.endpoint)
	fmt.Fprintf(w.file, "Started:  %s\n", w.started.Format(time.RFC3339))
	fmt.Fprintln(w.file, strings.Repeat("=", 60))
	fmt.Fprintln(w.file)
}

// ts returns a formatted timestamp for log entries.
func (w *UILogWriter) ts() string {
	return time.Now().Format("15:04:05.000")
}

// writeLine writes a single line to the log file.
func (w *UILogWriter) writeLine(s string) {
	fmt.Fprintln(w.file, s)
}

// WriteEvent logs a single agent event in real-time.
// Streaming deltas (TextDelta, ThinkingDelta, ToolCallUpdate) are buffered
// and flushed at MessageEnd / TurnEnd to avoid partial artifacts.
func (w *UILogWriter) WriteEvent(event agent.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch ev := event.(type) {
	case agent.EventTurnStart:
		w.writeLine(fmt.Sprintf("\n[%s] ── TURN START ──", w.ts()))

	case agent.EventMessageStart:
		w.writeLine(fmt.Sprintf("[%s] ── ASSISTANT MESSAGE START ──", w.ts()))

	case agent.EventTextDelta:
		w.turnBuf = append(w.turnBuf, ev.Text)

	case agent.EventThinkingDelta:
		// Buffer thinking text separately; we'll write it at MessageEnd.
		w.turnBuf = append(w.turnBuf, fmt.Sprintf("[thinking:%s]", ev.Text))

	case agent.EventMessageEnd:
		w.flushTurnBuffer()
		w.writeLine(fmt.Sprintf("[%s] ── ASSISTANT MESSAGE END", w.ts()))
		if ev.Usage != nil {
			w.writeLine(fmt.Sprintf("  tokens: prompt=%d completion=%d total=%d",
				ev.Usage.PromptTokens, ev.Usage.CompletionTokens, ev.Usage.TotalTokens))
		}

	case agent.EventTurnEnd:
		w.flushTurnBuffer()
		w.writeLine(fmt.Sprintf("[%s] ── TURN END", w.ts()))
		if ev.Usage != nil {
			w.writeLine(fmt.Sprintf("  total tokens this turn: %d", ev.Usage.TotalTokens))
		}

	case agent.EventToolCallStart:
		w.writeLine(fmt.Sprintf("[%s] ── TOOL CALL: %s", w.ts(), ev.Name))

	case agent.EventToolCallUpdate:
		// Buffer tool call arg updates; flush at ToolCallEnd.
		w.turnBuf = append(w.turnBuf, fmt.Sprintf("[tool_args:%s]", ev.RawArgs))

	case agent.EventToolCallEnd:
		w.flushTurnBuffer()
		w.writeLine(fmt.Sprintf("[%s] ── TOOL CALL END: %s", w.ts(), ev.Name))
		w.writeLine(fmt.Sprintf("  Args: %s", ev.Args))

	case agent.EventToolResult:
		w.writeLine(fmt.Sprintf("[%s] ── TOOL RESULT: %s", w.ts(), ev.Name))
		if ev.IsError {
			w.writeLine("  [ERROR]")
		}
		// Truncate very long results for the real-time log.
		result := ev.Result
		if len(result) > 2000 {
			result = result[:2000] + "\n  ... (truncated, see export for full output)"
		}
		w.writeLine("  Result:")
		for _, line := range strings.Split(result, "\n") {
			w.writeLine("    " + line)
		}

	case agent.EventError:
		w.flushTurnBuffer()
		w.writeLine(fmt.Sprintf("[%s] ── ERROR: %v", w.ts(), ev.Err))

	case agent.EventCompactionStart:
		w.writeLine(fmt.Sprintf("[%s] ── COMPACTION START: %d tokens", w.ts(), ev.TokensBefore))

	case agent.EventCompactionEnd:
		w.writeLine(fmt.Sprintf("[%s] ── COMPACTION END: %d → %d tokens, %d messages summarized",
			w.ts(), ev.TokensBefore, ev.TokensAfter, ev.MessagesRemoved))

	case agent.EventCompactionError:
		w.writeLine(fmt.Sprintf("[%s] ── COMPACTION ERROR: %v", w.ts(), ev.Err))
	}
}

// flushTurnBuffer writes any buffered deltas as a single block.
func (w *UILogWriter) flushTurnBuffer() {
	if len(w.turnBuf) == 0 {
		return
	}

	// Separate thinking and text deltas for clean output.
	var thinkingParts []string
	var textParts []string

	for _, entry := range w.turnBuf {
		if strings.HasPrefix(entry, "[thinking:") && strings.HasSuffix(entry, "]") {
			thinkingParts = append(thinkingParts, strings.TrimSuffix(strings.TrimPrefix(entry, "[thinking:"), "]"))
		} else {
			textParts = append(textParts, entry)
		}
	}

	if len(thinkingParts) > 0 {
		w.writeLine("  [thinking]")
		w.writeLine(strings.Join(thinkingParts, " "))
		w.writeLine("  [/thinking]")
	}

	if len(textParts) > 0 {
		text := strings.Join(textParts, "")
		for _, line := range strings.Split(text, "\n") {
			w.writeLine("  " + line)
		}
	}

	w.turnBuf = nil
}

// LogUserInput logs a user message submission.
func (w *UILogWriter) LogUserInput(input string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writeLine(fmt.Sprintf("[%s] ── USER ──", w.ts()))
	for _, line := range strings.Split(input, "\n") {
		w.writeLine("  " + line)
	}
}

// exportMessages writes a complete, expanded export of all messages to the
// given writer function. This is shared between Export and ExportToFile.
func exportMessages(w func(string), messages []*types.Message) (toolCallCount, toolResultCount, errorCount, assistantMsgCount, userMsgCount int) {
	for _, msg := range messages {
		ts := msg.Timestamp.Format("15:04:05.000")

		switch msg.Type {
		case types.MsgUser:
			userMsgCount++
			w(fmt.Sprintf("[%s] ── USER ──", ts))
			w(msg.Content)
			w("")

		case types.MsgAssistant:
			assistantMsgCount++
			w(fmt.Sprintf("[%s] ── ASSISTANT ──", ts))
			if msg.ThinkingText != "" {
				w("[thinking]")
				for _, line := range strings.Split(msg.ThinkingText, "\n") {
					w("  " + line)
				}
				w("[/thinking]")
			}
			w(msg.Content)
			w("")

		case types.MsgToolCall:
			toolCallCount++
			w(fmt.Sprintf("[%s] ── TOOL CALL: %s ──", ts, msg.ToolName))
			w(fmt.Sprintf("Args: %s", msg.ToolArgs))
			w("")

		case types.MsgToolResult:
			toolResultCount++
			if msg.IsError {
				errorCount++
			}
			w(fmt.Sprintf("[%s] ── TOOL RESULT: %s", ts, msg.ToolName))
			if msg.IsError {
				w("  [ERROR]")
			}
			w(msg.Content)
			w("")

		}
	}
	return
}

// writeStatistics writes session statistics to the given writer function.
func writeStatistics(w func(string), messages []*types.Message, totalTokens, toolCallCount, toolResultCount, errorCount, assistantMsgCount, userMsgCount int) {
	w(strings.Repeat("=", 60))
	w("Session Statistics")
	w(strings.Repeat("=", 60))
	w(fmt.Sprintf("Total messages:   %d", len(messages)))
	w(fmt.Sprintf("User messages:    %d", userMsgCount))
	w(fmt.Sprintf("Assistant msgs:   %d", assistantMsgCount))
	w(fmt.Sprintf("Tool calls:       %d", toolCallCount))
	w(fmt.Sprintf("Tool results:     %d", toolResultCount))
	w(fmt.Sprintf("Tool errors:      %d", errorCount))
	w(fmt.Sprintf("Total tokens:     %d", totalTokens))
	w(fmt.Sprintf("Exported at:      %s", time.Now().Format(time.RFC3339)))
	w(strings.Repeat("=", 60))
}

// Export writes a complete, expanded export of all messages to the log file.
// This is the post-session export (Approach 1 from the plan): all collapsed
// content is expanded, thinking is shown, and full tool results are included.
func (w *UILogWriter) Export(messages []*types.Message) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.writeLine("")
	w.writeLine(strings.Repeat("=", 60))
	w.writeLine("EXPORT — Complete Conversation (Expanded)")
	w.writeLine(strings.Repeat("=", 60))
	w.writeLine("")

	var totalTokens int
	var toolCallCount, toolResultCount, errorCount, assistantMsgCount, userMsgCount int
	toolCallCount, toolResultCount, errorCount, assistantMsgCount, userMsgCount = exportMessages(w.writeLine, messages)

	writeStatistics(w.writeLine, messages, totalTokens, toolCallCount, toolResultCount, errorCount, assistantMsgCount, userMsgCount)
	w.writeLine("")
}

// ExportToFile writes a complete, expanded export of all messages to a
// separate file path (not the real-time log). This is used for the /export
// slash command.
func ExportToFile(path string, messages []*types.Message, model, endpoint string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating export file %s: %w", path, err)
	}
	defer f.Close()

	var toolCallCount, toolResultCount, errorCount, assistantMsgCount, userMsgCount int

	fprintln := func(s string) { fmt.Fprintln(f, s) }

	fmt.Fprintln(f, strings.Repeat("=", 60))
	fmt.Fprintln(f, "mmok Conversation Export")
	fmt.Fprintln(f, strings.Repeat("=", 60))
	fmt.Fprintf(f, "Model:    %s\n", model)
	fmt.Fprintf(f, "Endpoint: %s\n", endpoint)
	fmt.Fprintf(f, "Exported: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintln(f, strings.Repeat("=", 60))
	fmt.Fprintln(f)

	toolCallCount, toolResultCount, errorCount, assistantMsgCount, userMsgCount = exportMessages(fprintln, messages)

	writeStatistics(fprintln, messages, 0, toolCallCount, toolResultCount, errorCount, assistantMsgCount, userMsgCount)

	return nil
}

// Close flushes and closes the log file.
func (w *UILogWriter) Close() {
	if w == nil || w.file == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushTurnBuffer()
	w.writeLine("")
	w.writeLine(strings.Repeat("=", 60))
	w.writeLine(fmt.Sprintf("Session ended: %s", time.Now().Format(time.RFC3339)))
	w.writeLine(strings.Repeat("=", 60))
	w.file.Sync()
	w.file.Close()
}
