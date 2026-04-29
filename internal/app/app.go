package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/user/mmok/internal/tui"
	"github.com/user/mmok/internal/types"
)

// AppModel is the root bubbletea model for the application.
type AppModel struct {
	Config      *Config
	Screen      *tui.Screen
	Messages    []*types.Message
	InputBuffer string
	Streaming   bool
	PartialText string
	width       int
	height      int
	quitting    bool
}

// NewAppModel creates a new AppModel with the given config.
func NewAppModel(cfg *Config) (*AppModel, error) {
	theme := tui.DefaultTheme()
	screen := tui.NewScreen(theme)

	screen.SetModel(cfg.Model)
	screen.SetMaxTokens(cfg.MaxContextTokens)
	screen.SetStatusBarState(tui.StatusIdle)

	return &AppModel{
		Config:   cfg,
		Screen:   screen,
		Messages: make([]*types.Message, 0),
	}, nil
}

// Init implements tea.Model.
func (m *AppModel) Init() tea.Cmd {
	return tea.WindowSize()
}

// Update implements tea.Model.
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.Screen.SetDimensions(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			if msg.Alt {
				// Alt+Enter: insert newline in input
				m.Screen.GetInputArea().HandleRune('\n')
				break
			}
			// Submit message
			input := m.Screen.GetInputArea().Value()
			if input != "" {
				if quitCmd := m.submitMessage(input); quitCmd != nil {
					return m, quitCmd
				}
			}

		case tea.KeyUp, tea.KeyDown:
			// Only handle if input is empty (otherwise it's for history)
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
			handled := m.Screen.GetInputArea().HandleKey(msg.Type)
			if !handled && msg.Type == tea.KeyRunes {
				for _, r := range msg.Runes {
					m.Screen.GetInputArea().HandleRune(r)
				}
			} else if !handled && msg.String() != "" {
				m.Screen.GetInputArea().HandleRune(rune(msg.String()[0]))
			}
		}
	}

	return m, cmd
}

// View implements tea.Model.
func (m *AppModel) View() string {
	if m.quitting {
		return ""
	}

	// Update screen with current state
	m.Screen.SetMessages(m.Messages)
	m.Screen.SetInputValue(m.Screen.GetInputArea().Value())
	m.Screen.SetStreaming(m.Streaming)
	m.Screen.SetPartialText(m.PartialText)

	return m.Screen.Render()
}

// handleCommand processes slash commands. Returns a tea.Cmd if a command was handled.
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

// submitMessage adds a user message and starts a (future) agent response.
// Returns a tea.Cmd if a command needs to be executed (e.g., tea.Quit).
func (m *AppModel) submitMessage(text string) tea.Cmd {
	// Check for slash commands
	if quitCmd := m.handleCommand(text); quitCmd != nil {
		m.Screen.GetInputArea().SetValue("")
		m.Screen.GetInputArea().PushHistory()
		return quitCmd
	}

	// Add user message
	userMsg := types.NewMessage(types.MsgUser, text)
	m.Messages = append(m.Messages, userMsg)
	m.Screen.GetInputArea().SetValue("")
	m.Screen.GetInputArea().PushHistory()

	// For now, echo back (Phase 2 will connect the agent)
	// This is a placeholder to show the UI working
	assistantMsg := types.NewMessage(types.MsgAssistant, fmt.Sprintf("(echo) %s", text))
	m.Messages = append(m.Messages, assistantMsg)
	m.Screen.GetMessageView().ScrollToBottom()
	return nil
}

// Run starts the bubbletea program.
func Run(cfg *Config) error {
	model, err := NewAppModel(cfg)
	if err != nil {
		return fmt.Errorf("creating app model: %w", err)
	}

	p := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithOutput(os.Stdout),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}
