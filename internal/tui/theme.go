package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds all lipgloss styles for the TUI.
type Theme struct {
	User                lipgloss.Style
	System              lipgloss.Style
	Assistant           lipgloss.Style
	AssistantLabel      lipgloss.Style
	ToolCall            lipgloss.Style
	ToolResult          lipgloss.Style
	ToolResultCollapsed lipgloss.Style
	Error               lipgloss.Style
	StatusBar           lipgloss.Style
	StatusBarActive     lipgloss.Style
	StatusBarIdle       lipgloss.Style
	StatusBarError      lipgloss.Style
	InputPrefix         lipgloss.Style
	Border              lipgloss.Style
	Panel               lipgloss.Style
	Dim                 lipgloss.Style
	Bold                lipgloss.Style

	// Autocomplete styles
	AutocompletePanel        lipgloss.Style
	AutocompleteItem         lipgloss.Style
	AutocompleteItemSelected lipgloss.Style
	AutocompleteDescription  lipgloss.Style
	AutocompletePrefix       lipgloss.Style
}

// DefaultTheme returns the default color scheme.
func DefaultTheme() Theme {
	return Theme{
		User:                lipgloss.NewStyle().Background(lipgloss.Color("236")),
		System:              lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("103")),
		Assistant:           lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
		AssistantLabel:      lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		ToolCall:            lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Italic(true),
		ToolResult:          lipgloss.NewStyle().Foreground(lipgloss.Color("81")),
		ToolResultCollapsed: lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Italic(true),
		Error:               lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		StatusBar:           lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("144")),
		StatusBarActive:     lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("226")).Bold(true),
		StatusBarIdle:       lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("65")),
		StatusBarError:      lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("196")).Bold(true),
		InputPrefix:         lipgloss.NewStyle().Foreground(lipgloss.Color("144")).Bold(true),
		Border:              lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")),
		Panel:               lipgloss.NewStyle().MarginLeft(1).MarginRight(1),
		Dim:                 lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		Bold:                lipgloss.NewStyle().Bold(true),

		// Autocomplete styles
		AutocompletePanel:        lipgloss.NewStyle().Background(lipgloss.Color("236")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")),
		AutocompleteItem:         lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252")).PaddingLeft(2).PaddingRight(2),
		AutocompleteItemSelected: lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("232")).Bold(true).PaddingLeft(2).PaddingRight(2),
		AutocompleteDescription:  lipgloss.NewStyle().Foreground(lipgloss.Color("243")).PaddingLeft(2),
		AutocompletePrefix:       lipgloss.NewStyle().Foreground(lipgloss.Color("144")).Bold(true),
	}
}
