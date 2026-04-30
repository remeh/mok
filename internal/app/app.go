package app

import (
	"context"
	"os"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/user/mmok/internal/agent"
	"github.com/user/mmok/internal/llm"
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
	Config    *Config
	Screen    *tui.Screen
	Agent     *agent.Agent
	Messages  []*types.Message
	width     int
	height    int
	quitting  bool

	// Agent event handling
	agentRunning bool
	streamMsg    *types.Message
	cancel       context.CancelFunc
	eventChan    chan agentEvent
}

// NewAppModel creates a new AppModel with the given config.
func NewAppModel(cfg *Config) (*AppModel, error) {
	theme := tui.DefaultTheme()
	screen := tui.NewScreen(theme)

	screen.SetModel(cfg.Model)
	screen.SetMaxTokens(cfg.MaxContextTokens)
	screen.SetStatusBarState(tui.StatusIdle)

	client := llm.NewClient(cfg.Endpoint, cfg.BearerToken)
	agt := agent.NewAgent(client, agent.AgentConfig{
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	})

	return &AppModel{
		Config:    cfg,
		Screen:    screen,
		Agent:     agt,
		Messages:  make([]*types.Message, 0),
		eventChan: make(chan agentEvent, 128),
	}, nil
}

// Init implements tea.Model.
func (m *AppModel) Init() tea.Cmd {
	return tea.WindowSize()
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
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.agentRunning {
				m.abortAgent()
			}
			m.quitting = true
			return m, tea.Quit

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
			if m.Screen.GetInputArea().Value() == "" {
				if msg.Type == tea.KeyUp {
					m.Screen.GetMessageView().ScrollUp()
				} else {
					m.Screen.GetMessageView().ScrollDown()
				}
			} else {
				m.Screen.GetInputArea().HandleKey(msg.Type)
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

	case agentEvent:
		m.handleAgentEvent(msg.Event)
		// Keep polling for more events until the turn is done
		if !msg.Done {
			return m, m.readAgentEvent
		}
	}

	return m, nil
}

// handleAgentEvent processes a single event from the agent loop.
func (m *AppModel) handleAgentEvent(event agent.Event) {
	switch ev := event.(type) {
	case agent.EventTurnStart:
		m.Screen.SetStatusBarState(tui.StatusThinking)

	case agent.EventMessageStart:
		m.streamMsg = types.NewMessage(types.MsgAssistant, "")
		m.streamMsg.Streaming = true
		m.Messages = append(m.Messages, m.streamMsg)

	case agent.EventTextDelta:
		if m.streamMsg != nil {
			m.streamMsg.Content += ev.Text
		}

	case agent.EventThinkingDelta:
		if m.streamMsg != nil {
			m.streamMsg.ThinkingText += ev.Text
		}

	case agent.EventMessageEnd:
		if m.streamMsg != nil {
			m.streamMsg.Streaming = false
		}
		if ev.Usage != nil {
			m.Screen.SetTokenCount(ev.Usage.TotalTokens)
		}

	case agent.EventTurnEnd:
		m.agentRunning = false
		m.streamMsg = nil
		m.cancel = nil
		m.Screen.GetInputArea().SetFocused(true)
		m.Screen.SetStatusBarState(tui.StatusIdle)

	case agent.EventError:
		m.agentRunning = false
		m.streamMsg = nil
		m.cancel = nil
		m.Screen.GetInputArea().SetFocused(true)
		m.Screen.SetStatusBarState(tui.StatusError)
		errMsg := types.NewMessage(types.MsgAssistant, "Error: "+ev.Err.Error())
		m.Messages = append(m.Messages, errMsg)
	}
}

// abortAgent cancels the current agent run.
func (m *AppModel) abortAgent() {
	if m.cancel != nil {
		m.cancel()
	}
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
	m.Screen.GetMessageView().ScrollToBottom()

	m.agentRunning = true
	m.Screen.GetInputArea().SetFocused(false)
	m.Screen.SetStatusBarState(tui.StatusThinking)

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
		tea.WithOutput(os.Stdout),
	)

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
