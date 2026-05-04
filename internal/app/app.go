package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/user/mmok/internal/agent"
	"github.com/user/mmok/internal/llm"
	"github.com/user/mmok/internal/tools"
	"github.com/user/mmok/internal/tui"
	"github.com/user/mmok/internal/types"
)

// agentEvent wraps a single agent event as a tea.Msg.
type agentEvent struct {
	Event agent.Event
	Done  bool // true when the agent turn is complete
}

// AppModel is the root bubbletea model for the application.
type AppModel struct {
	Config         *Config
	Screen         *tui.Screen
	Agent          *agent.Agent
	Messages       []*types.Message
	Debug          *agent.DebugLogger
	width          int
	height         int
	editorTempFile string // path to temp file used by Ctrl-G editor
	quitting       bool

	// Agent event handling
	agentRunning  bool
	streamMsg     *types.Message
	cancel        context.CancelFunc
	eventChan     chan agentEvent
	spinnerTicker tea.Cmd
}

// NewAppModel creates a new AppModel with the given config.
func NewAppModel(cfg *Config) (*AppModel, error) {
	theme := tui.DefaultTheme()
	screen := tui.NewScreen(theme)

	screen.SetModel(cfg.Model)
	screen.SetMaxTokens(cfg.MaxContextTokens)
	screen.SetStatusBarState(tui.StatusIdle)

	// Create debug logger for TUI mode (writes to debug.log file)
	var debug *agent.DebugLogger
	if cfg.Debug {
		debug = agent.NewDebugLoggerFile(true, "debug.log")
		debug.Info("APP", "Debug logging enabled, writing to debug.log")
	}

	client := llm.NewClient(cfg.Endpoint, cfg.BearerToken)
	client.WithDebug(debug)

	// Create tool registry and register built-in tools
	toolRegistry := tools.NewRegistry()
	toolRegistry.Add(&tools.ReadTool{CWD: cfg.CWD})
	toolRegistry.Add(&tools.WriteTool{CWD: cfg.CWD})
	toolRegistry.Add(&tools.EditTool{CWD: cfg.CWD})
	toolRegistry.Add(&tools.BashTool{CWD: cfg.CWD})

	agt := agent.NewAgent(client, agent.AgentConfig{
		Model:               cfg.Model,
		MaxTokens:           cfg.MaxTokens,
		CWD:                 cfg.CWD,
		MaxContextTokens:    cfg.MaxContextTokens,
		CompactionThreshold: cfg.CompactionThreshold,
		KeepRecentTokens:    cfg.KeepRecentTokens,
		SummarizationModel:  cfg.SummarizationModel,
	}, toolRegistry, debug)

	return &AppModel{
		Config:    cfg,
		Screen:    screen,
		Agent:     agt,
		Debug:     debug,
		Messages:  make([]*types.Message, 0),
		eventChan: make(chan agentEvent, 128),
	}, nil
}

// Init implements tea.Model.
func (m *AppModel) Init() tea.Cmd {
	return tea.Batch(
		tea.WindowSize(),
		tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg { return t }),
	)
}

// Update implements tea.Model.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.Screen.SetDimensions(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.agentRunning {
				m.abortAgent()
			}
			m.quitting = true
			if m.Debug != nil {
				m.Debug.Close()
			}
			return m, tea.Quit

		case tea.KeyEsc:
			if m.agentRunning {
				m.abortAgent()
			}

		case tea.KeyCtrlZ:
			return m, tea.Suspend

		case tea.KeyCtrlG:
			if m.agentRunning {
				break
			}
			if cmd := m.openEditor(); cmd != nil {
				return m, cmd
			}

		case tea.KeyCtrlO:
			// Expand all collapsed tool results
			m.expandAllToolResults()

		case tea.KeyEnter:
			if msg.Alt {
				m.Screen.GetInputArea().HandleRune('\n')
				break
			}
			if m.agentRunning {
				m.abortAgent()
				break
			}
			input := m.Screen.GetInputArea().Value()
			if input != "" {
				if quitCmd := m.submitMessage(input); quitCmd != nil {
					return m, quitCmd
				}
			}

		case tea.KeyUp, tea.KeyDown:
			if m.Screen.GetInputArea().Value() == "" || m.Screen.IsScrolledUp() {
				if msg.Type == tea.KeyUp {
					m.Screen.GetMessageView().ScrollUp()
				} else {
					m.Screen.GetMessageView().ScrollDown()
				}
			} else {
				m.Screen.GetInputArea().HandleKey(msg.Type)
			}

		case tea.KeyPgUp, tea.KeyPgDown, tea.KeyCtrlU, tea.KeyCtrlD:
			if msg.Type == tea.KeyPgUp || msg.Type == tea.KeyCtrlU {
				m.Screen.GetMessageView().ScrollPageUp()
			} else {
				m.Screen.GetMessageView().ScrollPageDown()
			}

		case tea.KeyCtrlT, tea.KeyCtrlB:
			if msg.Type == tea.KeyCtrlT {
				m.Screen.GetMessageView().ScrollToTop()
			} else {
				m.Screen.GetMessageView().ScrollToBottom()
			}

		default:
			if m.agentRunning {
				break
			}
			handled := m.Screen.GetInputArea().HandleKey(msg.Type)
			if !handled && msg.Type == tea.KeyRunes {
				for _, r := range msg.Runes {
					m.Screen.GetInputArea().HandleRune(r)
				}
			} else if !handled && msg.String() != "" {
				m.Screen.GetInputArea().HandleRune(rune(msg.String()[0]))
			}
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.Screen.GetMessageView().ScrollUp()
		case tea.MouseButtonWheelDown:
			m.Screen.GetMessageView().ScrollDown()
		case tea.MouseButtonLeft:
			// Only toggle on press, not release or motion
			if msg.Action != tea.MouseActionPress {
				break
			}
			// Click within the message view area to toggle expand/collapse
			if msg.Y < m.height-2 {
				idx := m.Screen.GetMessageView().MessageAtY(msg.Y)
				if idx >= 0 && idx < len(m.Messages) {
					msgAtClick := m.Messages[idx]
					switch {
					case msgAtClick.Type == types.MsgToolResult && msgAtClick.Summary != "":
						msgAtClick.Collapsed = !msgAtClick.Collapsed
						m.Screen.GetMessageView().MessageGrew()
					case msgAtClick.ThinkingText != "":
						msgAtClick.ThinkingExpanded = !msgAtClick.ThinkingExpanded
						m.Screen.GetMessageView().MessageGrew()
					}
				}
			}
		}

	case agentEvent:
		m.handleAgentEvent(msg.Event)
		// Keep polling for more events until the turn is done
		if !msg.Done {
			return m, m.readAgentEvent
		}

	case time.Time:
		// Spinner tick
		m.Screen.Tick()
		return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg { return t })
	}

	return m, nil
}

// handleAgentEvent processes a single event from the agent loop.
func (m *AppModel) handleAgentEvent(event agent.Event) {
	switch ev := event.(type) {
	case agent.EventTurnStart:
		m.Screen.SetStatusBarState(tui.StatusProcessing)

	case agent.EventMessageStart:
		m.streamMsg = types.NewMessage(types.MsgAssistant, "")
		m.streamMsg.Streaming = true
		m.Messages = append(m.Messages, m.streamMsg)
		m.Screen.GetMessageView().MessageGrew()
		m.Screen.SetStatusBarState(tui.StatusStreaming)

	case agent.EventTextDelta:
		if m.streamMsg != nil {
			m.streamMsg.Content += ev.Text
			m.Screen.GetMessageView().MessageGrew()
		}

	case agent.EventThinkingDelta:
		if m.streamMsg != nil {
			m.streamMsg.ThinkingText += ev.Text
			m.Screen.GetMessageView().MessageGrew()
		}

	case agent.EventMessageEnd:
		if m.streamMsg != nil {
			m.streamMsg.Streaming = false
		}
		if ev.Usage != nil {
			m.Screen.SetTokenCount(ev.Usage.TotalTokens)
		}
		// Agent is deciding its next action (more LLM calls or turn end)
		m.Screen.SetStatusBarState(tui.StatusProcessing)

	case agent.EventTurnEnd:
		if ev.Cancelled {
			cancelMsg := types.NewSystemMessage("Cancelled")
			m.Messages = append(m.Messages, cancelMsg)
			m.Screen.GetMessageView().MessageGrew()
		}
		m.agentRunning = false
		m.streamMsg = nil
		m.cancel = nil
		m.Screen.GetInputArea().SetFocused(true)
		m.Screen.SetBlocked(false)

		if ev.Usage != nil {
			m.Screen.SetTokenCount(ev.Usage.TotalTokens)
		}

		m.Screen.SetStatusBarState(tui.StatusIdle)
		m.Screen.SetStatusMessage("") // Clear custom status message

	case agent.EventToolCallStart:
		// Show tool call start with "executing..."
		toolCallMsg := types.NewToolCall(ev.Name, ev.RawArgs)
		m.Messages = append(m.Messages, toolCallMsg)
		m.Screen.GetMessageView().MessageGrew()
		m.Screen.SetStatusBarState(tui.StatusToolCall)
		m.Screen.SetToolName(ev.Name)

	case agent.EventToolCallUpdate:
		// Update the last tool call message's args (streaming args)
		if len(m.Messages) > 0 {
			last := m.Messages[len(m.Messages)-1]
			if last.Type == types.MsgToolCall {
				last.ToolArgs = ev.RawArgs
				m.Screen.GetMessageView().MessageGrew()
			}
		}

	case agent.EventToolCallEnd:
		// Finalize the tool call display
		if len(m.Messages) > 0 {
			last := m.Messages[len(m.Messages)-1]
			if last.Type == types.MsgToolCall {
				last.ToolArgs = ev.Args
				m.Screen.GetMessageView().MessageGrew()
			}
		}

	case agent.EventToolResult:
		// Show tool result
		resultMsg := types.NewToolResult(ev.Name, ev.Result, ev.IsError)
		m.Messages = append(m.Messages, resultMsg)
		m.Screen.GetMessageView().MessageGrew()
		m.Screen.SetToolName("")
		// Reset status to thinking since agent will decide next action
		m.Screen.SetStatusBarState(tui.StatusProcessing)

	case agent.EventError:
		m.agentRunning = false
		m.streamMsg = nil
		m.cancel = nil
		m.Screen.GetInputArea().SetFocused(true)
		m.Screen.SetBlocked(false)
		m.Screen.SetStatusBarState(tui.StatusError)
		errMsg := types.NewMessage(types.MsgAssistant, "Error: "+ev.Err.Error())
		m.Messages = append(m.Messages, errMsg)
		m.Screen.GetMessageView().MessageGrew()

	case agent.EventCompactionStart:
		m.Screen.SetStatusBarState(tui.StatusCompacting)
		m.Screen.SetStatusMessage("Compacting context...")

	case agent.EventCompactionEnd:
		m.Screen.SetStatusBarState(tui.StatusProcessing)
		m.Screen.SetStatusMessage("Compaction complete")
		// Optionally show a system message about compaction
		summaryMsg := types.NewMessage(types.MsgAssistant,
			fmt.Sprintf("[Compaction: %d → %d tokens, %d messages summarized]",
				ev.TokensBefore, ev.TokensAfter, ev.MessagesRemoved))
		m.Messages = append(m.Messages, summaryMsg)
		m.Screen.GetMessageView().MessageGrew()

	case agent.EventCompactionError:
		m.Screen.SetStatusBarState(tui.StatusProcessing)
		m.Screen.SetStatusMessage("Compaction skipped")
	}
}

// abortAgent cancels the current agent run.
func (m *AppModel) abortAgent() {
	if m.cancel != nil {
		m.cancel()
	}
}

// expandAllToolResults expands all collapsed tool result messages.
func (m *AppModel) expandAllToolResults() {
	changed := false
	for _, msg := range m.Messages {
		if msg.Type == types.MsgToolResult && msg.Collapsed {
			msg.Collapsed = false
			changed = true
		}
	}
	if changed {
		m.Screen.GetMessageView().MessageGrew()
	}
}

// openEditor creates a temp file with the current input, launches $EDITOR
// (falling back to vi), and on exit reads the file back into the input area.
func (m *AppModel) openEditor() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmp, err := os.CreateTemp("", "mmok-prompt-*.txt")
	if err != nil {
		return nil
	}
	if _, err := tmp.WriteString(m.Screen.GetInputArea().Value()); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil
	}
	tmp.Close()

	m.editorTempFile = tmp.Name()

	cmd := exec.Command(editor, tmp.Name())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer func() {
			os.Remove(tmp.Name())
			m.editorTempFile = ""
		}()

		if err != nil {
			return nil
		}

		content, readErr := os.ReadFile(tmp.Name())
		if readErr == nil {
			m.Screen.GetInputArea().SetValue(string(content))
		}
		return nil
	})
}

// readAgentEvent is a tea.Cmd that reads the next event from the bridge channel.
// Blocks until an event arrives or the channel closes.
func (m *AppModel) readAgentEvent() tea.Msg {
	event, ok := <-m.eventChan
	if !ok {
		return agentEvent{Event: agent.EventTurnEnd{}, Done: true}
	}
	return event
}

// View implements tea.Model.
func (m *AppModel) View() string {
	if m.quitting {
		return ""
	}

	m.Screen.SetMessages(m.Messages)
	m.Screen.SetInputValue(m.Screen.GetInputArea().Value())
	m.Screen.SetStreaming(m.streamMsg != nil && m.streamMsg.Streaming)

	return m.Screen.Render()
}

// handleCommand processes slash commands. Returns a tea.Cmd if handled.
func (m *AppModel) handleCommand(text string) tea.Cmd {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return nil
	}

	cmd := strings.ToLower(strings.TrimSpace(text[1:]))
	switch cmd {
	case "exit", "quit":
		m.quitting = true
		if m.Debug != nil {
			m.Debug.Close()
		}
		return tea.Quit
	default:
		return nil
	}
}

// submitMessage adds a user message and starts the agent loop.
// Returns the first polling cmd to kick off the event pipeline.
func (m *AppModel) submitMessage(text string) tea.Cmd {
	if quitCmd := m.handleCommand(text); quitCmd != nil {
		m.Screen.GetInputArea().SetValue("")
		m.Screen.GetInputArea().PushHistory()
		return quitCmd
	}

	userMsg := types.NewMessage(types.MsgUser, text)
	m.Messages = append(m.Messages, userMsg)
	m.Screen.GetInputArea().SetValue("")
	m.Screen.GetInputArea().PushHistory()
	m.Screen.SetMessages(m.Messages)
	m.Screen.GetMessageView().ScrollToBottom()

	m.agentRunning = true
	m.Screen.GetInputArea().SetFocused(false)
	m.Screen.SetBlocked(true)
	m.Screen.SetStatusBarState(tui.StatusProcessing)

	// Create a fresh event channel for this turn.
	m.eventChan = make(chan agentEvent, 128)

	// Agent writes events here; a bridge goroutine forwards them to m.eventChan.
	agentEvents := make(chan agent.Event, 64)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	go func() {
		defer close(agentEvents)
		_ = m.Agent.Run(ctx, text, agentEvents)
	}()

	// Bridge: forward each agent event to bubbletea's channel.
	go func() {
		defer close(m.eventChan)
		for event := range agentEvents {
			m.eventChan <- agentEvent{Event: event, Done: false}
		}
	}()

	return m.readAgentEvent
}

// Run starts the bubbletea program.
func Run(cfg *Config) error {
	model, err := NewAppModel(cfg)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
		tea.WithOutput(os.Stdout),
	)

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
