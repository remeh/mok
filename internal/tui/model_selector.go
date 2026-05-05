package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
)

// ModelInfo holds information about an available model.
type ModelInfo struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Created     int64  `json:"created"`
	OwnedBy     string `json:"owned_by"`
	Description string `json:"description,omitempty"`
}

// ModelSelector manages model selection with autocomplete-like UI.
type ModelSelector struct {
	theme        Theme
	autocomplete *AutocompleteState
	loading      bool
	errorMsg     string
	currentModel string
	endpoint     string
	bearerToken  string
}

// NewModelSelector creates a new ModelSelector.
func NewModelSelector(theme Theme, currentModel, endpoint, bearerToken string) *ModelSelector {
	return &ModelSelector{
		theme:        theme,
		autocomplete: NewAutocompleteState(),
		currentModel: currentModel,
		endpoint:     endpoint,
		bearerToken:  bearerToken,
	}
}

// ModelSelectorMsg is a tea.Msg for model selector events.
type ModelSelectorMsg struct {
	Models    []ModelInfo
	Error     error
	Selected  string
	Cancelled bool
}

// FetchModelsCmd fetches available models from the API.
func (ms *ModelSelector) FetchModelsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Build the models endpoint URL
		modelsURL := ms.endpoint
		if !strings.HasSuffix(modelsURL, "/") {
			modelsURL += "/"
		}
		modelsURL += "models"

		req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
		if err != nil {
			return ModelSelectorMsg{Error: fmt.Errorf("failed to create request: %w", err)}
		}

		if ms.bearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+ms.bearerToken)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return ModelSelectorMsg{Error: fmt.Errorf("failed to fetch models: %w", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ModelSelectorMsg{Error: fmt.Errorf("API returned status %d", resp.StatusCode)}
		}

		// Parse JSON response
		var result struct {
			Data []ModelInfo `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return ModelSelectorMsg{Error: fmt.Errorf("failed to decode response: %w", err)}
		}

		return ModelSelectorMsg{Models: result.Data}
	}
}

// Activate starts the model selector with fetched models.
func (ms *ModelSelector) Activate(models []ModelInfo) {
	ms.loading = false
	ms.errorMsg = ""

	if len(models) == 0 {
		ms.errorMsg = "No models available"
		return
	}

	// Build suggestion list with model IDs
	suggestions := make([]string, len(models))
	for i, model := range models {
		suggestions[i] = model.ID
	}

	// Activate autocomplete with model suggestions
	ms.autocomplete.active = true
	ms.autocomplete.suggestions = suggestions
	ms.autocomplete.selectedIndex = 0
	ms.autocomplete.insertPos = 0
	ms.autocomplete.completionType = CompletionCommand
	ms.autocomplete.prefix = ""
	ms.autocomplete.cmdName = ""
	ms.autocomplete.argPrefix = ""
}

// GetSelectedModel returns the currently selected model ID.
func (ms *ModelSelector) GetSelectedModel() string {
	if ms.autocomplete.IsActive() && ms.autocomplete.HasSuggestions() {
		return ms.autocomplete.GetSelected()
	}
	return ms.currentModel
}

// IsLoading returns whether models are being fetched.
func (ms *ModelSelector) IsLoading() bool {
	return ms.loading
}

// GetError returns any error message.
func (ms *ModelSelector) GetError() string {
	return ms.errorMsg
}

// IsActive returns whether the selector is showing suggestions.
func (ms *ModelSelector) IsActive() bool {
	return ms.autocomplete.IsActive()
}

// GetAutocompleteState returns the internal autocomplete state for rendering.
func (ms *ModelSelector) GetAutocompleteState() *AutocompleteState {
	return ms.autocomplete
}

// Render displays the model selector UI.
func (ms *ModelSelector) Render(width int) string {
	if ms.loading {
		return ms.theme.Dim.Render("Loading models...")
	}

	if ms.errorMsg != "" {
		return ms.theme.Error.Render("Error: " + ms.errorMsg)
	}

	if !ms.autocomplete.IsActive() {
		return ""
	}

	// Render autocomplete suggestions
	autocompleteView := NewAutocompleteView(ms.theme)
	autocompleteView.SetDimensions(width, 0)
	return autocompleteView.Render(ms.autocomplete, "")
}

// GetHeight returns the height needed for the model selector UI.
func (ms *ModelSelector) GetHeight() int {
	if ms.loading {
		return 1
	}
	if ms.errorMsg != "" {
		return 1
	}
	if !ms.autocomplete.IsActive() {
		return 0
	}
	return NewAutocompleteView(ms.theme).GetHeight(ms.autocomplete, "")
}

// SetError sets the error message and clears loading state.
func (ms *ModelSelector) SetError(errMsg string) {
	ms.loading = false
	ms.errorMsg = errMsg
}

// SetLoading sets the loading state.
func (ms *ModelSelector) SetLoading(loading bool) {
	ms.loading = loading
}
