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

// NopLogger is a no-op implementation of DebugLogger.
type NopLogger struct{}

func (NopLogger) Debug(string, string, ...any)    {}
func (NopLogger) Info(string, string, ...any)     {}
func (NopLogger) Request(string, string, ...any)  {}
func (NopLogger) Response(string, string, ...any) {}
func (NopLogger) Event(string, string, ...any)    {}
func (NopLogger) Tool(string, string, ...any)     {}
func (NopLogger) JSON(string, string, any)        {}
func (NopLogger) Dump(string, string, []byte)     {}
