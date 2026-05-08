package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/user/mok/internal/agent"
	"github.com/user/mok/internal/llm"
	"github.com/user/mok/internal/tools"
	"github.com/user/mok/internal/tui"
	"github.com/user/mok/internal/types"
)

// agentEvent wraps a single agent event as a tea.Msg.
type agentEvent struct {
	Event agent.Event
	Done  bool // true when the agent turn is complete
}

// modelSelectorState constants
type modelSelectorStateEnum int

const (
	modelSelectorIdle = iota
	modelSelectorFetching
	modelSelectorWaitingInput
)

// modelSelectorState tracks the model selection process.
type modelSelectorState struct {
	selector   *tui.ModelSelector
	waitingFor modelSelectorStateEnum
}

// AppModel is the root bubbletea model for the application.
type AppModel struct {
	Config         *Config
	Screen         *tui.Screen
	Agent          *agent.Agent
	Messages       []*types.Message
	Debug          *agent.DebugLogger
	UILogWriter    *UILogWriter
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

	// Model selection
	modelSelector *modelSelectorState
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

	// Create command registry and register built-in commands
	cmdRegistry := tui.NewCommandRegistry()
	cmdRegistry.Register(tui.CommandDefinition{
		Name:        "model",
		Description: "Change model",
		HasArgs:     true,
		ArgValues:   []string{cfg.Model}, // Start with current model, can be expanded
	})
	cmdRegistry.Register(tui.CommandDefinition{
		Name:        "debug",
		Description: "Toggle debug mode",
		HasArgs:     true,
		ArgValues:   []string{"on", "off"},
	})
	cmdRegistry.Register(tui.CommandDefinition{
		Name:        "clear",
		Description: "Clear conversation",
		HasArgs:     false,
	})
	cmdRegistry.Register(tui.CommandDefinition{
		Name:        "quit",
		Description: "Exit application",
		HasArgs:     false,
	})
	cmdRegistry.Register(tui.CommandDefinition{
		Name:        "exit",
		Description: "Exit application",
		HasArgs:     false,
	})
	cmdRegistry.Register(tui.CommandDefinition{
		Name:        "help",
		Description: "Show available commands",
		HasArgs:     false,
	})
	cmdRegistry.Register(tui.CommandDefinition{
		Name:        "compact",
		Description: "Manually compact conversation history",
		HasArgs:     false,
	})

	// Pass command registry to input area
	inputArea := screen.GetInputArea()
	inputArea.SetCommandRegistry(cmdRegistry)

	agt := agent.NewAgent(client, agent.AgentConfig{
		Model:               cfg.Model,
		MaxTokens:           cfg.MaxTokens,
		CWD:                 cfg.CWD,
		MaxContextTokens:    cfg.MaxContextTokens,
		CompactionThreshold: cfg.CompactionThreshold,
		KeepRecentTokens:    cfg.KeepRecentTokens,
		SummarizationModel:  cfg.SummarizationModel,
	}, toolRegistry, debug)

	// Create UI log writer (always on, default path is "ui.log").
	var uiLogWriter *UILogWriter
	var err error
	uiLogWriter, err = NewUILogWriter(cfg.UILogPath, cfg.Model, cfg.Endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ui log: %v\n", err)
	} else if debug != nil {
		debug.Info("APP", "UI logging enabled, writing to %s", cfg.UILogPath)
	}

	return &AppModel{
		Config:      cfg,
		Screen:      screen,
		Agent:       agt,
		Debug:       debug,
		UILogWriter: uiLogWriter,
		Messages:    make([]*types.Message, 0),
		eventChan:   make(chan agentEvent, 128),
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
			if m.UILogWriter != nil {
				m.UILogWriter.Close()
			}
			return m, tea.Quit

		case tea.KeyEsc:
			if m.agentRunning {
				m.abortAgent()
			} else if m.modelSelector != nil && m.modelSelector.waitingFor == modelSelectorWaitingInput {
				// Cancel model selection: clear state, dismiss autocomplete, and clear input
				m.modelSelector = nil
				m.Screen.GetInputArea().DismissAutocomplete()
				m.Screen.GetInputArea().SetValue("")
				m.Screen.SetStatusMessage("")
			} else {
				// Let input area handle ESC (dismiss autocomplete, exit history mode)
				_ = m.Screen.GetInputArea().HandleKey(msg.Type)
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
				m.Screen.GetMessageView().ScrollToBottom()
				break
			}

			// Check if model selector is active and waiting for selection
			if m.modelSelector != nil && m.modelSelector.waitingFor == modelSelectorWaitingInput && m.Screen.GetInputArea().AutocompleteIsActive() {
				// Accept the selected model from input area's autocomplete
				selectedModel := m.Screen.GetInputArea().GetAutocompleteState().GetSelected()
				m.selectModel(selectedModel)
				// Deactivate autocomplete and clear input
				m.Screen.GetInputArea().DismissAutocomplete()
				m.Screen.GetInputArea().SetValue("")
				return m, nil
			}

			// Accept autocomplete if active (without inserting newline)
			if m.Screen.GetInputArea().AutocompleteIsActive() {
				m.Screen.GetInputArea().AcceptAutocomplete()
			}
			// Get the input value (autocomplete may have been accepted)
			input := m.Screen.GetInputArea().Value()
			if input != "" {
				if quitCmd := m.submitMessage(input); quitCmd != nil {
					return m, quitCmd
				}
			}

		case tea.KeyUp, tea.KeyDown:
			if m.agentRunning {
				break
			}
			// Let input area handle Up/Down first (it handles history navigation at line start)
			inputHandled := m.Screen.GetInputArea().HandleKey(msg.Type)

			// If input area didn't handle it (empty input, not at line start), scroll messages
			if !inputHandled && m.Screen.IsScrolledUp() {
				if msg.Type == tea.KeyUp {
					m.Screen.GetMessageView().ScrollUp()
				} else {
					m.Screen.GetMessageView().ScrollDown()
				}
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

	case tui.ModelSelectorMsg:
		m.handleModelSelection(msg)
		return m, nil

	case time.Time:
		// Spinner tick
		m.Screen.Tick()
		return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg { return t })
	}

	return m, nil
}

// handleAgentEvent processes a single event from the agent loop.
func (m *AppModel) handleAgentEvent(event agent.Event) {
	// Log the event to the UI log if enabled.
	if m.UILogWriter != nil {
		m.UILogWriter.WriteEvent(event)
	}

	switch ev := event.(type) {
	case agent.EventTurnStart:
		m.Screen.SetStatusBarState(tui.StatusProcessing)
		// Store the turn start time on the last user message
		if len(m.Messages) > 0 {
			m.Messages[len(m.Messages)-1].Timestamp = ev.StartTime
		}

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
			// If the assistant message was only a placeholder (no content and
			// no thinking), the response was purely tool calls. Remove the
			// empty message to avoid a duplicate entry in the UI.
			if m.streamMsg.Content == "" && m.streamMsg.ThinkingText == "" {
				for i, msg := range m.Messages {
					if msg == m.streamMsg {
						m.Messages = append(m.Messages[:i], m.Messages[i+1:]...)
						m.Screen.GetMessageView().MessageGrew()
						break
					}
				}
			}
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

		// Append a dim stats line at the end of the turn
		if ev.Duration > 0 {
			statsLine := formatTurnStats(ev.EndTime, ev.Duration, ev.Usage)
			statsMsg := types.NewMessage(types.MsgSystem, statsLine)
			statsMsg.IsTurnStats = true
			m.Messages = append(m.Messages, statsMsg)
			m.Screen.GetMessageView().MessageGrew()
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
		m.Screen.SetStatusMessage("")
		m.Screen.SetBlocked(true)

	case agent.EventCompactionEnd:
		m.Screen.SetBlocked(false)
		m.Screen.SetStatusBarState(tui.StatusIdle)
		m.Screen.SetStatusMessage("")
		// Update token count to reflect the compacted context
		m.Screen.SetTokenCount(ev.TokensAfter)
		// Optionally show a system message about compaction
		summaryMsg := types.NewMessage(types.MsgAssistant,
			fmt.Sprintf("[Compaction: %d → %d tokens, %d messages summarized]",
				ev.TokensBefore, ev.TokensAfter, ev.MessagesRemoved))
		m.Messages = append(m.Messages, summaryMsg)
		m.Screen.GetMessageView().MessageGrew()

	case agent.EventCompactionError:
		m.Screen.SetBlocked(false)
		m.Screen.SetStatusBarState(tui.StatusIdle)
		m.Screen.SetStatusMessage("")
		// Check if it was a cancellation
		if ev.Err != nil && strings.Contains(ev.Err.Error(), "cancelled") {
			// Show cancellation message but don't treat as error
			cancelMsg := types.NewSystemMessage("Compaction cancelled")
			m.Messages = append(m.Messages, cancelMsg)
			m.Screen.GetMessageView().MessageGrew()
		}
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

	tmp, err := os.CreateTemp("", "mok-prompt-*.txt")
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
	m.Screen.SetStreaming(m.streamMsg != nil && m.streamMsg.Streaming)

	return m.Screen.Render()
}

// handleCommand processes slash commands. Returns a tea.Cmd if handled.
func (m *AppModel) handleCommand(text string) (tea.Cmd, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return nil, false
	}

	// Parse command and arguments
	parts := strings.Fields(text[1:]) // Remove leading slash and split
	if len(parts) == 0 {
		return nil, false
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "exit", "quit":
		m.quitting = true
		if m.Debug != nil {
			m.Debug.Close()
		}
		if m.UILogWriter != nil {
			m.UILogWriter.Close()
		}
		return tea.Quit, true

	case "clear":
		m.Messages = make([]*types.Message, 0)
		m.Screen.GetMessageView().Clear()
		return nil, true

	case "model":
		return m.handleModelCommand(), true

	case "debug":
		return m.handleDebugCommand(parts), true

	case "help":
		// Show help message
		helpText := "Available commands:\n"
		helpText += "  /model         - Interactively select a model from available options\n"
		helpText += "  /debug on|off  - Toggle debug mode\n"
		helpText += "  /clear         - Clear conversation\n"
		helpText += "  /compact       - Manually compact conversation history\n"
		helpText += "  /quit | /exit  - Exit application\n"
		helpText += "  /help          - Show this help\n"

		helpMsg := types.NewSystemMessage(helpText)
		m.Messages = append(m.Messages, helpMsg)
		m.Screen.GetMessageView().MessageGrew()
		return nil, true

	case "compact":
		return m.handleCompactCommand(), true

	default:
		msg := types.NewSystemMessage("unknown command: " + text)
		m.Messages = append(m.Messages, msg)
		m.Screen.GetMessageView().MessageGrew()
		return nil, true
	}

	return nil, false
}

// handleModelCommand initiates model selection by fetching available models.
func (m *AppModel) handleModelCommand() tea.Cmd {
	// Create model selector if not exists
	if m.modelSelector == nil {
		m.modelSelector = &modelSelectorState{
			selector: tui.NewModelSelector(
				tui.DefaultTheme(),
				m.Config.Model,
				m.Config.Endpoint,
				m.Config.BearerToken,
			),
			waitingFor: modelSelectorFetching, // fetching
		}
		m.Screen.SetStatusMessage("Fetching models...")
	}

	// Start fetching models
	return m.modelSelector.selector.FetchModelsCmd()
}

// handleDebugCommand toggles debug mode based on the argument.
func (m *AppModel) handleDebugCommand(parts []string) tea.Cmd {
	if len(parts) < 2 {
		msg := types.NewSystemMessage("Usage: /debug on|off")
		m.Messages = append(m.Messages, msg)
		m.Screen.GetMessageView().MessageGrew()
		return nil
	}

	action := strings.ToLower(parts[1])

	if action != "on" && action != "off" {
		msg := types.NewSystemMessage("Usage: /debug on|off")
		m.Messages = append(m.Messages, msg)
		m.Screen.GetMessageView().MessageGrew()
		return nil
	}

	// Toggle debug mode
	m.Config.Debug = (action == "on")

	// Update debug logger if needed
	if m.Config.Debug {
		if m.Debug == nil {
			m.Debug = agent.NewDebugLoggerFile(true, "debug.log")
			m.Debug.Info("APP", "Debug mode enabled")
		}
	} else {
		if m.Debug != nil {
			m.Debug.Close()
			m.Debug = nil
		}
	}

	// Update the agent's debug logger immediately
	if m.Agent != nil {
		m.Agent.SetDebug(m.Debug)
		m.Agent.SetClientDebug(m.Debug)
	}

	// Show confirmation
	confirmMsg := fmt.Sprintf("Debug mode %s", action)
	sysMsg := types.NewSystemMessage(confirmMsg)
	m.Messages = append(m.Messages, sysMsg)
	m.Screen.GetMessageView().MessageGrew()

	return nil
}

// handleCompactCommand triggers manual compaction of the conversation history.
func (m *AppModel) handleCompactCommand() tea.Cmd {
	if m.Agent == nil {
		msg := types.NewSystemMessage("Agent not initialized")
		m.Messages = append(m.Messages, msg)
		m.Screen.GetMessageView().MessageGrew()
		return nil
	}

	// Show status and block input
	m.Screen.SetStatusBarState(tui.StatusCompacting)
	m.Screen.SetStatusMessage("")
	m.Screen.SetBlocked(true)

	// Perform compaction in a goroutine to avoid blocking the UI
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := m.Agent.ManualCompact(ctx)
		if err != nil {
			return agentEvent{
				Event: agent.EventError{Err: fmt.Errorf("compaction failed: %w", err)},
				Done:  true,
			}
		}

		return agentEvent{
			Event: agent.EventCompactionEnd{
				TokensBefore:     result.TokensBefore,
				TokensAfter:      result.TokensAfter,
				MessagesRemoved:  result.MessagesRemoved,
				SummaryAvailable: result.CompactSummary != nil,
			},
			Done: true,
		}
	}
}

// handleModelSelection processes the result of fetching models.
func (m *AppModel) handleModelSelection(msg tui.ModelSelectorMsg) tea.Cmd {
	if m.modelSelector == nil {
		return nil
	}

	if msg.Error != nil {
		m.modelSelector.selector.SetError(msg.Error.Error())
		m.modelSelector.waitingFor = modelSelectorIdle
		m.Screen.SetStatusMessage("")

		// Show error message
		errMsg := types.NewSystemMessage("Failed to fetch models: " + msg.Error.Error())
		m.Messages = append(m.Messages, errMsg)
		m.Screen.GetMessageView().MessageGrew()
		return nil
	}

	// Activate selector with fetched models
	m.modelSelector.selector.Activate(msg.Models)
	m.modelSelector.waitingFor = modelSelectorWaitingInput // waiting for user selection

	// Show prompt in input area
	input := m.Screen.GetInputArea()
	input.SetValue("Select model: ")

	// Programmatically activate autocomplete with model suggestions
	models := msg.Models
	suggestions := make([]string, len(models))
	for i, model := range models {
		suggestions[i] = model.ID
	}
	input.GetAutocompleteState().ActivateSimpleCompletion(suggestions, len("Select model: "))

	return nil
}

// selectModel updates the config with the selected model and recreates the agent.
func (m *AppModel) selectModel(modelID string) {
	if m.modelSelector == nil {
		return
	}

	// Save conversation history before recreating agent
	conversationHistory := m.Messages

	// Update config with new model
	m.Config.Model = modelID

	// Recreate the agent with the new model
	m.recreateAgent()

	// Restore conversation history
	m.Messages = conversationHistory

	// Update screen with new model
	m.Screen.SetModel(modelID)

	// Show confirmation
	confirmMsg := fmt.Sprintf("Switched to model: %s", modelID)
	sysMsg := types.NewSystemMessage(confirmMsg)
	m.Messages = append(m.Messages, sysMsg)
	m.Screen.GetMessageView().MessageGrew()

	// Clear selector state
	m.modelSelector = nil
	m.Screen.SetStatusMessage("")
}

// recreateAgent creates a new agent with the current config values.
func (m *AppModel) recreateAgent() {
	// Create new LLM client
	client := llm.NewClient(m.Config.Endpoint, m.Config.BearerToken)
	client.WithDebug(m.Debug)

	// Re-register tools (they need CWD from config)
	toolRegistry := tools.NewRegistry()
	toolRegistry.Add(&tools.ReadTool{CWD: m.Config.CWD})
	toolRegistry.Add(&tools.WriteTool{CWD: m.Config.CWD})
	toolRegistry.Add(&tools.EditTool{CWD: m.Config.CWD})
	toolRegistry.Add(&tools.BashTool{CWD: m.Config.CWD})

	// Create new agent with updated config
	m.Agent = agent.NewAgent(client, agent.AgentConfig{
		Model:               m.Config.Model,
		MaxTokens:           m.Config.MaxTokens,
		CWD:                 m.Config.CWD,
		MaxContextTokens:    m.Config.MaxContextTokens,
		CompactionThreshold: m.Config.CompactionThreshold,
		KeepRecentTokens:    m.Config.KeepRecentTokens,
		SummarizationModel:  m.Config.SummarizationModel,
	}, toolRegistry, m.Debug)
}

// submitMessage adds a user message and starts the agent loop.
// Returns the first polling cmd to kick off the event pipeline.
func (m *AppModel) submitMessage(text string) tea.Cmd {
	quitCmd, interpreted := m.handleCommand(text)
	if quitCmd != nil {
		m.Screen.GetInputArea().SetValue("")
		m.Screen.GetInputArea().PushHistory()
		return quitCmd
	}
	// the command has been interpreted, we don't have to go further
	// but we have to clear the prompt
	if interpreted {
		m.Screen.GetInputArea().SetValue("")
		return nil
	}

	userMsg := types.NewMessage(types.MsgUser, text)
	m.Messages = append(m.Messages, userMsg)

	// Log user input to the UI log.
	if m.UILogWriter != nil {
		m.UILogWriter.LogUserInput(text)
	}
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

// formatTurnStats formats a turn's end time, duration, and token usage into a short stats line.
func formatTurnStats(endTime time.Time, duration time.Duration, usage *llm.Usage) string {
	parts := []string{fmt.Sprintf("%s", endTime.Format("15:04:05"))}
	dur := duration.Round(time.Millisecond)
	if dur > time.Minute {
		dur = dur.Round(time.Second)
	}
	parts = append(parts, fmt.Sprintf("⏱ %s", dur))
	if usage != nil && usage.TotalTokens > 0 {
		parts = append(parts, fmt.Sprintf("%d tokens", usage.TotalTokens))
	}
	return strings.Join(parts, " · ")
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
