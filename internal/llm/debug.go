package llm

// DebugLogger is the interface for debug logging used by the LLM client.
// The agent package's DebugLogger implements this interface.
type DebugLogger interface {
	Debug(category, format string, args ...any)
	Info(category, format string, args ...any)
	Request(category, format string, args ...any)
	Response(category, format string, args ...any)
	Event(category, format string, args ...any)
	Tool(category, format string, args ...any)
	JSON(category, label string, v any)
	Dump(category, label string, data []byte)
}
