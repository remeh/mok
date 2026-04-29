# Phase 1: App — TUI + Configuration

## Goals
- Working TUI application with message display and input
- Configuration system: YAML file + env vars + CLI flags (flags > env > file)
- Message rendering: user, assistant text, tool calls/results
- Status bar: model, token count, streaming state
- Keybindings: Enter to submit, Ctrl+C to abort, arrow keys for history

## Config Schema

```yaml
# ~/.config/mmok/config.yaml or ./mmok.yaml (project-local)
model: "qwen3-8b"
endpoint: "http://localhost:8080/v1"
api_key: ""
max_context_tokens: 131072
compaction_threshold: 0.8
keep_recent_tokens: 16384
temperature: 0.0
max_tokens: 0
```

## Config Loading

Precedence: CLI flags > env vars > YAML file > defaults

```go
type Config struct {
    Model              string  `yaml:"model"`
    Endpoint           string  `yaml:"endpoint"`
    APIKey             string  `yaml:"api_key"`
    MaxContextTokens   int     `yaml:"max_context_tokens"`
    CompactionThreshold float64 `yaml:"compaction_threshold"`
    KeepRecentTokens   int     `yaml:"keep_recent_tokens"`
    Temperature        float32 `yaml:"temperature"`
    MaxTokens          int     `yaml:"max_tokens"`
}

// LoadConfig reads config from: defaults → file → env → flags
func LoadConfig() (*Config, error)
```

Env vars: `MMOK_MODEL`, `MMOK_ENDPOINT`, `MMOK_API_KEY`, etc.

## TUI Architecture (bubbletea Elm pattern)

```go
// AppModel is the root bubbletea model
type AppModel struct {
    Config      *Config
    Agent       *agent.Agent        // nil until phase 2
    Messages    []Message           // Conversation history
    InputBuffer string              // Current user input
    InputCursor int                 // Cursor position in input
    Streaming   bool                // LLM is streaming
    PartialText string              // Partial assistant text
    History     []string            // Input history
    HistoryIdx  int                 // History navigation index
    Width, Height int              // Terminal dimensions
}

func (m AppModel) Init() tea.Cmd
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m AppModel) View() string
```

### Message Types

```go
type MessageType string

const (
    MsgSystem   MessageType = "system"
    MsgUser     MessageType = "user"
    MsgAssistant MessageType = "assistant"
    MsgToolCall MessageType = "tool_call"
    MsgToolResult MessageType = "tool_result"
)

type Message struct {
    ID        string      // Unique identifier
    Type      MessageType
    Content   string      // Text content or tool result output
    ToolName  string      // For tool_call / tool_result
    ToolArgs  string      // For tool_call: JSON args
    IsError   bool        // For tool_result
    Timestamp time.Time
}
```

### TUI Layout

```
┌─────────────────────────────────────────────────────────────┐
│ User: How do I fix the edit tool?                            │
├─────────────────────────────────────────────────────────────┤
│ Assistant: Let me check the edit tool implementation.        │
│                                                             │
│ [tool_call] read(path="internal/tools/edit.go")             │
│ [tool_result] 307 lines                                     │
│                                                             │
│ Assistant: I found the issue. The edit tool has a bug in    │
│ the diff generation logic. Let me fix it.                    │
│                                                             │
│ [tool_call] edit(path="...", oldText="...", newText="...")  │
│ [tool_result] Applied 1 edit                                │
│                                                             │
│ Assistant: Done! The fix applies edits against the original │
│ file content, not incrementally.                             │
├─────────────────────────────────────────────────────────────┤
│ >                                                           │
│ qwen3-8b | 12,847 tokens | streaming...                    │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

1. **MessageView** — Scrollable message list
   - Renders each message with appropriate styling
   - Auto-scrolls to bottom on new content
   - Collapsible tool results (show first N lines, expand on click)

2. **InputArea** — Multi-line input
   - Single line with Alt+Enter for newline
   - Enter to submit
   - Arrow up/down for history
   - Shows cursor position

3. **StatusBar** — Bottom bar
   - Left: Model name
   - Center: Token count / context usage percentage
   - Right: Status (idle, streaming, compacting, error)

### Styling (lipgloss)

```go
var (
    StyleUser     = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true)     // Blue
    StyleAssistant = lipgloss.NewStyle().Foreground(lipgloss.Color("210"))                // Pink
    StyleToolCall  = lipgloss.NewStyle().Foreground(lipgloss.Color("136")).Italic(true)  // Green
    StyleToolResult = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))              // Orange
    StyleError     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))               // Red
    StyleStatusBar = lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("144"))
)
```

### Tasks

1. [ ] `go mod init github.com/user/mmok`
2. [ ] Create directory structure under `internal/`
3. [ ] Implement `internal/app/config.go`: LoadConfig with YAML + env + defaults
4. [ ] Implement `internal/app/config_types.go`: Config struct
5. [ ] Implement `internal/tui/theme.go`: Lipgloss styles
6. [ ] Implement `internal/tui/statusbar.go`: Status bar component
7. [ ] Implement `internal/tui/input.go`: Input area with history
8. [ ] Implement `internal/tui/message_view.go`: Message list with rendering
9. [ ] Implement `internal/tui/screen.go`: Main layout composition
10. [ ] Implement `internal/app/app.go`: Bubbletea app model, update, view
11. [ ] Implement `cmd/mmok/main.go`: CLI entry point with flag parsing
12. [ ] Test: Run the app, type messages, see them rendered (no LLM yet)
