package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// AutocompleteView renders the autocomplete suggestion panel.
type AutocompleteView struct {
	theme       Theme
	maxItems    int
	fullWidth   int
	descriptionWidth int
}

// NewAutocompleteView creates a new AutocompleteView.
func NewAutocompleteView(theme Theme) *AutocompleteView {
	return &AutocompleteView{
		theme:    theme,
		maxItems: 10,
	}
}

// SetDimensions sets the dimensions for the autocomplete view.
func (a *AutocompleteView) SetDimensions(fullWidth, descriptionWidth int) {
	a.fullWidth = fullWidth
	a.descriptionWidth = descriptionWidth
}

// Render renders the autocomplete panel with suggestions.
// Returns an empty string if no suggestions are available.
func (a *AutocompleteView) Render(state *AutocompleteState, prefix string) string {
	if state == nil || !state.IsActive() || !state.HasSuggestions() {
		return ""
	}

	suggestions := state.GetSuggestions()
	selectedIdx := state.GetSelectedIndex()

	// Limit to max items
	startIdx := 0
	if len(suggestions) > a.maxItems {
		// Try to center the selected item
		if selectedIdx > a.maxItems/2 && selectedIdx < len(suggestions)-a.maxItems/2 {
			startIdx = selectedIdx - a.maxItems/2
		} else if selectedIdx >= len(suggestions)-a.maxItems {
			startIdx = len(suggestions) - a.maxItems
		}
	}
	endIdx := startIdx + a.maxItems
	if endIdx > len(suggestions) {
		endIdx = len(suggestions)
	}

	var lines []string

	// Render suggestions with full width background
	for i := startIdx; i < endIdx; i++ {
		suggestion := suggestions[i]
		isSelected := (i == selectedIdx)

		var line string
		if isSelected {
			// Selected item: show with indicator and full width background
			content := fmt.Sprintf("▸ %s", suggestion)
			// Pad content to full width, then apply style with fixed width
			paddedContent := content + strings.Repeat(" ", max(0, a.fullWidth-lipgloss.Width(content)))
			line = a.theme.AutocompleteItemSelected.Width(a.fullWidth).Render(paddedContent)
		} else {
			// Normal item with full width background
			content := fmt.Sprintf("  %s", suggestion)
			// Pad content to full width, then apply style with fixed width
			paddedContent := content + strings.Repeat(" ", max(0, a.fullWidth-lipgloss.Width(content)))
			line = a.theme.AutocompleteItem.Width(a.fullWidth).Render(paddedContent)
		}

		lines = append(lines, line)
	}

	// Wrap in panel style
	if len(lines) > 0 {
		panel := a.theme.AutocompletePanel.Render(strings.Join(lines, "\n"))
		return panel
	}

	return ""
}

// GetHeight returns the approximate height of the rendered autocomplete panel.
func (a *AutocompleteView) GetHeight(state *AutocompleteState, prefix string) int {
	if state == nil || !state.IsActive() || !state.HasSuggestions() {
		return 0
	}

	suggestions := state.GetSuggestions()
	count := len(suggestions)
	if count > a.maxItems {
		count = a.maxItems
	}

	// Header + separator + suggestions
	if prefix != "" {
		return count + 2
	}
	return count
}

// GetMaxWidth returns the maximum width needed for the autocomplete panel.
func (a *AutocompleteView) GetMaxWidth(state *AutocompleteState) int {
	if state == nil || !state.IsActive() || !state.HasSuggestions() {
		return 0
	}

	maxWidth := 0
	for _, suggestion := range state.GetSuggestions() {
		width := lipgloss.Width(suggestion) + 3 // +3 for "  " prefix and space
		if width > maxWidth {
			maxWidth = width
		}
	}

	// Add border padding
	return maxWidth + 2
}