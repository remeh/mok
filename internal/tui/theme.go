package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds all lipgloss styles for the TUI.
type Theme struct {
	User        lipgloss.Style
	Assistant   lipgloss.Style
	ToolCall    lipgloss.Style
	ToolResult  lipgloss.Style
	Error       lipgloss.Style
	StatusBar   lipgloss.Style
	InputPrefix lipgloss.Style
	Border      lipgloss.Style
	Panel       lipgloss.Style
	Dim         lipgloss.Style
	Bold        lipgloss.Style
}

// DefaultTheme returns the default color scheme.
func DefaultTheme() Theme {
	return Theme{
		User:        lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true),        // Blue
		Assistant:   lipgloss.NewStyle().Foreground(lipgloss.Color("210")),                   // Pink
		ToolCall:    lipgloss.NewStyle().Foreground(lipgloss.Color("136")).Italic(true),     // Green
		ToolResult:  lipgloss.NewStyle().Foreground(lipgloss.Color("178")),                  // Orange
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),       // Red
		StatusBar:   lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("144")),
		InputPrefix: lipgloss.NewStyle().Foreground(lipgloss.Color("144")).Bold(true),       // Teal
		Border:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")),
		Panel:       lipgloss.NewStyle().MarginLeft(1).MarginRight(1),
		Dim:         lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		Bold:        lipgloss.NewStyle().Bold(true),
	}
}
