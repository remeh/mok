package tui

import (
	"strings"
	"unicode"
)

// CompletionType indicates what kind of completion is being performed.
type CompletionType int

const (
	CompletionNone CompletionType = iota
	CompletionCommand
	CompletionValue
)

// CommandDefinition defines a slash command and its metadata.
type CommandDefinition struct {
	Name        string
	Description string
	HasArgs     bool
	ArgValues   []string // predefined values for autocomplete
}

// CommandRegistry manages command definitions and provides completion matching.
type CommandRegistry struct {
	commands map[string]*CommandDefinition
}

// NewCommandRegistry creates a new CommandRegistry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]*CommandDefinition),
	}
}

// Register adds a command definition to the registry.
func (r *CommandRegistry) Register(cmd CommandDefinition) {
	r.commands[cmd.Name] = &cmd
}

// Get returns a command definition by name, or nil if not found.
func (r *CommandRegistry) Get(name string) *CommandDefinition {
	return r.commands[name]
}

// All returns all registered commands.
func (r *CommandRegistry) All() []*CommandDefinition {
	result := make([]*CommandDefinition, 0, len(r.commands))
	for _, cmd := range r.commands {
		result = append(result, cmd)
	}
	return result
}

// Complete returns matching command names for the given prefix.
func (r *CommandRegistry) Complete(prefix string) []string {
	if prefix == "" {
		// Return all commands sorted
		result := make([]string, 0, len(r.commands))
		for name := range r.commands {
			result = append(result, name)
		}
		return sortStrings(result)
	}

	prefix = strings.ToLower(prefix)
	result := make([]string, 0)
	for name := range r.commands {
		if strings.HasPrefix(name, prefix) {
			result = append(result, name)
		}
	}
	return sortStrings(result)
}

// CompleteValues returns matching argument values for a command.
func (r *CommandRegistry) CompleteValues(cmdName, argPrefix string) []string {
	cmd := r.Get(cmdName)
	if cmd == nil || !cmd.HasArgs || len(cmd.ArgValues) == 0 {
		return nil
	}

	if argPrefix == "" {
		return sortStrings(cmd.ArgValues)
	}

	argPrefix = strings.ToLower(argPrefix)
	result := make([]string, 0)
	for _, val := range cmd.ArgValues {
		if strings.HasPrefix(strings.ToLower(val), argPrefix) {
			result = append(result, val)
		}
	}
	return sortStrings(result)
}

// HasCommand checks if a command exists.
func (r *CommandRegistry) HasCommand(name string) bool {
	_, ok := r.commands[name]
	return ok
}

// sortStrings sorts strings alphabetically (case-insensitive).
func sortStrings(s []string) []string {
	// Simple bubble sort for small lists
	result := make([]string, len(s))
	copy(result, s)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if strings.ToLower(result[i]) > strings.ToLower(result[j]) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// AutocompleteState holds the state for autocomplete functionality.
type AutocompleteState struct {
	active          bool
	suggestions     []string
	selectedIndex   int
	prefix          string        // text before cursor to match against
	insertPos       int           // position where completion will be inserted
	completionType  CompletionType // Command, Value, or None
	cmdName         string        // current command name (for value completion)
	argPrefix       string        // current argument prefix (for value completion)
}

// NewAutocompleteState creates a new AutocompleteState.
func NewAutocompleteState() *AutocompleteState {
	return &AutocompleteState{
		active:         false,
		suggestions:    make([]string, 0),
		selectedIndex:  0,
		completionType: CompletionNone,
	}
}

// IsActive returns whether autocomplete is currently active.
func (a *AutocompleteState) IsActive() bool {
	return a.active
}

// GetSuggestions returns the current suggestions.
func (a *AutocompleteState) GetSuggestions() []string {
	return a.suggestions
}

// GetSelected returns the currently selected suggestion.
func (a *AutocompleteState) GetSelected() string {
	if len(a.suggestions) == 0 {
		return ""
	}
	return a.suggestions[a.selectedIndex]
}

// GetSelectedIndex returns the currently selected index.
func (a *AutocompleteState) GetSelectedIndex() int {
	return a.selectedIndex
}

// GetCompletionType returns the type of completion.
func (a *AutocompleteState) GetCompletionType() CompletionType {
	return a.completionType
}

// GetPrefix returns the prefix being matched.
func (a *AutocompleteState) GetPrefix() string {
	return a.prefix
}

// GetInsertPos returns the position where completion will be inserted.
func (a *AutocompleteState) GetInsertPos() int {
	return a.insertPos
}

// ActivateCommandCompletion activates command completion with the given prefix.
func (a *AutocompleteState) ActivateCommandCompletion(prefix string, insertPos int, registry *CommandRegistry) {
	a.active = true
	a.prefix = prefix
	a.insertPos = insertPos
	a.completionType = CompletionCommand
	a.cmdName = ""
	a.argPrefix = ""

	if prefix == "" {
		a.suggestions = registry.Complete("")
	} else {
		a.suggestions = registry.Complete(prefix)
	}
	a.selectedIndex = 0
}

// ActivateValueCompletion activates value completion for a command.
func (a *AutocompleteState) ActivateValueCompletion(cmdName, argPrefix string, insertPos int, registry *CommandRegistry) {
	a.active = true
	a.cmdName = cmdName
	a.argPrefix = argPrefix
	a.insertPos = insertPos
	a.completionType = CompletionValue
	a.prefix = argPrefix

	a.suggestions = registry.CompleteValues(cmdName, argPrefix)
	a.selectedIndex = 0
}

// Deactivate deactivates autocomplete.
func (a *AutocompleteState) Deactivate() {
	a.active = false
	a.suggestions = make([]string, 0)
	a.selectedIndex = 0
	a.prefix = ""
	a.insertPos = 0
	a.completionType = CompletionNone
	a.cmdName = ""
	a.argPrefix = ""
}

// SelectNext moves to the next suggestion (wraps around).
func (a *AutocompleteState) SelectNext() {
	if len(a.suggestions) == 0 {
		return
	}
	a.selectedIndex = (a.selectedIndex + 1) % len(a.suggestions)
}

// SelectPrevious moves to the previous suggestion (wraps around).
func (a *AutocompleteState) SelectPrevious() {
	if len(a.suggestions) == 0 {
		return
	}
	a.selectedIndex = (a.selectedIndex - 1 + len(a.suggestions)) % len(a.suggestions)
}

// Filter updates suggestions based on a new prefix.
func (a *AutocompleteState) Filter(newPrefix string, registry *CommandRegistry) {
	a.prefix = newPrefix

	switch a.completionType {
	case CompletionCommand:
		a.suggestions = registry.Complete(newPrefix)
	case CompletionValue:
		a.argPrefix = newPrefix
		a.suggestions = registry.CompleteValues(a.cmdName, newPrefix)
	}

	if len(a.suggestions) == 0 {
		a.Deactivate()
	} else {
		a.selectedIndex = 0
	}
}

// HasSuggestions returns whether there are any suggestions.
func (a *AutocompleteState) HasSuggestions() bool {
	return len(a.suggestions) > 0
}

// Count returns the number of suggestions.
func (a *AutocompleteState) Count() int {
	return len(a.suggestions)
}

// GetPrefixAtCursor extracts the prefix at the current cursor position from the text.
// Returns the prefix string, command name (if applicable), and insert position.
func GetPrefixAtCursor(text string, cursorPos int) (prefix string, cmdName string, insertPos int, completionType CompletionType) {
	if cursorPos == 0 || cursorPos > len(text) {
		return "", "", 0, CompletionNone
	}

	// Find the start of the current word (or /word)
	start := cursorPos
	for start > 0 {
		ch := rune(text[start-1])
		if unicode.IsSpace(ch) || ch == '/' {
			break
		}
		start--
	}

	prefix = text[start:cursorPos]

	// Check if we're after a command
	if start > 0 && text[start-1] == '/' {
		// We're completing a command
		return prefix, "", start, CompletionCommand
	}

	// Check if we're after a command + space
	if start > 0 {
		// Look back for command pattern: /word followed by space
		cmdEnd := start - 1
		for cmdEnd > 0 && !unicode.IsSpace(rune(text[cmdEnd-1])) && text[cmdEnd-1] != '/' {
			cmdEnd--
		}

		if cmdEnd > 0 && text[cmdEnd-1] == '/' {
			// Check if there's a space between command and cursor
			hasSpace := false
			for i := cmdEnd; i < start; i++ {
				if unicode.IsSpace(rune(text[i])) {
					hasSpace = true
					break
				}
			}

			if hasSpace {
				// We're completing a value for this command
				cmdName = text[cmdEnd : start-1] // exclude the slash
				return prefix, cmdName, start, CompletionValue
			}
		}
	}

	return "", "", 0, CompletionNone
}