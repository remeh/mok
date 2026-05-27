package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/mok/internal/tools"
)

// PromptConfig holds parameters for building the system prompt.
type PromptConfig struct {
	CWD   string
	Tools *tools.Registry
}

// BuildSystemPrompt returns a system prompt optimized for local LLMs.
func BuildSystemPrompt(cfg *PromptConfig) string {
	cwd := cfg.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	date := time.Now().Format("2006-01-02")

	// Build tool listing for the prompt
	toolsList := "(none)"
	if cfg.Tools != nil {
		var lines []string
		for _, t := range cfg.Tools.All() {
			def := t.Definition()
			lines = append(lines, fmt.Sprintf("- %s: %s", def.Name, def.Snippet))
		}
		if len(lines) > 0 {
			toolsList = strings.Join(lines, "\n")
		}
	}

	// Read context files: AGENTS.md, CLAUDE.md
	var contextSection string
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		path := filepath.Join(cwd, name)
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			contextSection += fmt.Sprintf("\n\n# %s\n\n%s", name, string(data))
		}
	}

	return fmt.Sprintf(`You are an expert coding assistant. You help users by reading files, executing commands, editing code, and writing new files.

# Available tools

%s

# Guidelines

- Read files before editing them. Understand existing code before modifying it.
- Use bash for file operations like ls, grep, find, git.
- Prefer the read tool over bash cat/head/tail for reading files (it gives you line numbers and handles large files).
- Prefer the edit tool over bash sed/awk for modifying files (it validates changes and shows diffs).
- Use the write tool only for creating new files. Use edit for modifying existing files.
- When reading large files, use offset and limit to read in chunks rather than loading the entire file.
- Be concise in your responses.
- Show file paths clearly when working with files.
- When an edit fails because oldText was not found, re-read the file to see its current contents before retrying.
- Do not make changes beyond what was requested.
- Take that time to ask questions or to ask if it's time to make changes, but do not make changes until asked.

Current date: %s
Working directory: %s%s`, toolsList, date, cwd, contextSection)
}
