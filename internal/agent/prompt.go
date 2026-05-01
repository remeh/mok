package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PromptConfig holds parameters for building the system prompt.
type PromptConfig struct {
	CWD string
}

// BuildSystemPrompt returns a system prompt optimized for local LLMs.
func BuildSystemPrompt(cfg *PromptConfig) string {
	cwd := cfg.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	date := time.Now().Format("2006-01-02")

	// Read AGENT.md if it exists
	var agentMd string
	agentMdPath := filepath.Join(cwd, "AGENTS.md")
	if data, err := os.ReadFile(agentMdPath); err == nil && len(data) > 0 {
		agentMd = "\nAGENTS.md:\n\n" + string(data)
	}

	return fmt.Sprintf(`You are an expert coding assistant. You help users by reading files,
executing commands, editing code, and writing new files.

Guidelines:
- Use bash for file operations like ls, rg, find
- Be concise in your responses
- Show file paths clearly when working with files
- Read files in chunks when possible

Current date: %s
Working directory: %s%s`, date, cwd, agentMd)
}
