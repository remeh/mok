package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/user/mok/internal/agent"
	"github.com/user/mok/internal/app"
	"github.com/user/mok/internal/llm"
	"github.com/user/mok/internal/session"
	"github.com/user/mok/internal/tools"
)

var (
	version = "0.1.0"
	commit  = "dev"
	date    = "unknown"
)

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	prompt := flag.String("p", "", "Prompt to execute (non-interactive mode)")
	timeout := flag.Int("t", 0, "Timeout in seconds for prompt mode (0=no limit)")
	model := flag.String("model", "", "LLM model name")
	endpoint := flag.String("endpoint", "", "API endpoint URL")
	bearerToken := flag.String("bearer-token", "", "Bearer token for API authentication")
	systemPrompt := flag.String("system-prompt", "", "Custom system prompt (for one-shot runs with -p)")
	maxContext := flag.Int("max-context-tokens", 0, "Max context tokens")
	maxTokens := flag.Int("max-tokens", 0, "Max response tokens")
	debug := flag.Bool("debug", false, "Enable debug logging to stderr")
	uiLogPath := flag.String("ui-log-path", "", "Path for UI session log (requires -debug flag)")
	sessionPath := flag.String("session", "", "Path to session file to restore")

	flag.Parse()

	if *showVersion {
		fmt.Printf("mok %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	flags := map[string]string{
		"model":              *model,
		"endpoint":           *endpoint,
		"bearer-token":       *bearerToken,
		"system-prompt":      *systemPrompt,
		"max-context-tokens": fmt.Sprintf("%d", *maxContext),
		"max-tokens":         fmt.Sprintf("%d", *maxTokens),
		"debug":              fmt.Sprintf("%t", *debug),
		"ui-log-path":        *uiLogPath,
	}

	cfg, err := app.LoadConfig(flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: config error: %v\n", err)
		cfg = app.DefaultConfig()
	}

	if *prompt != "" {
		if err := runPrompt(cfg, *prompt, *timeout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle session restore (TUI mode only)
	if *sessionPath != "" {
		// Validate session file exists
		if _, err := os.Stat(*sessionPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: session file not found: %s\n", *sessionPath)
			os.Exit(1)
		}

		// Load session to extract config
		sess, err := session.LoadSession(*sessionPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to load session: %v\n", err)
			os.Exit(1)
		}

		// Merge session config with CLI flags (CLI flags have highest priority).
		// Session values override config file/env defaults since the user is
		// explicitly restoring this session.
		if *model == "" {
			cfg.Model = sess.Metadata.Model
		}
		if *endpoint == "" {
			cfg.Endpoint = sess.Metadata.Endpoint
		}
		// Always use session config values (they were set by the user during
		// the original session). CLI flags already applied via LoadConfig.
		if sess.Config.MaxContextTokens > 0 {
			cfg.MaxContextTokens = sess.Config.MaxContextTokens
		}
		if sess.Config.CompactionThreshold > 0 {
			cfg.CompactionThreshold = sess.Config.CompactionThreshold
		}
		if sess.Config.KeepRecentTokens > 0 {
			cfg.KeepRecentTokens = sess.Config.KeepRecentTokens
		}
		if sess.Config.MaxTokens > 0 {
			cfg.MaxTokens = sess.Config.MaxTokens
		}
		if sess.Config.SummarizationModel != "" {
			cfg.SummarizationModel = sess.Config.SummarizationModel
		}
	}

	if err := app.Run(cfg, *sessionPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runPrompt(cfg *app.Config, prompt string, timeoutSec int) error {
	ctx := context.Background()
	cancel := func() {}

	if timeoutSec > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	}
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create debug logger
	debug := agent.NewDebugLogger(cfg.Debug)
	if cfg.Debug {
		debug.Info("CONFIG", "Debug mode enabled")
	}

	client := llm.NewClient(cfg.Endpoint, cfg.BearerToken)
	client.WithDebug(debug)

	// Create tool registry (same as TUI mode)
	toolRegistry := tools.NewRegistry()
	toolRegistry.Add(&tools.ReadTool{CWD: cfg.CWD})
	toolRegistry.Add(&tools.WriteTool{CWD: cfg.CWD})
	toolRegistry.Add(&tools.EditTool{CWD: cfg.CWD})
	toolRegistry.Add(&tools.BashTool{CWD: cfg.CWD})

	agt := agent.NewAgent(client, agent.AgentConfig{
		Model:        cfg.Model,
		MaxTokens:    cfg.MaxTokens,
		CWD:          cfg.CWD,
		SystemPrompt: cfg.SystemPrompt,
	}, toolRegistry, debug)

	startTime := time.Now()
	var charCount int
	var lastUsage *llm.Usage

	fmt.Fprintf(os.Stderr, "\n[mok] model=%s endpoint=%s timeout=%ds\n", cfg.Model, cfg.Endpoint, timeoutSec)
	fmt.Fprintln(os.Stderr, strings.Repeat("-", 60))

	events := make(chan agent.Event, 128)

	// Run agent loop in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- agt.Run(ctx, prompt, events)
		close(events)
	}()

	for ev := range events {
		switch e := ev.(type) {
		case agent.EventTextDelta:
			fmt.Print(e.Text)
			charCount += len(e.Text)
		case agent.EventThinkingDelta:
			// Skip thinking tokens in prompt mode
		case agent.EventToolCallStart:
			fmt.Fprintf(os.Stderr, "\n[tool] %s\n", e.Name)
		case agent.EventToolResult:
			if e.IsError {
				fmt.Fprintf(os.Stderr, "[tool] %s error: %s\n", e.Name, e.Result)
			} else {
				// Truncate long tool results in stderr output
				result := e.Result
				if len(result) > 200 {
					result = result[:200] + "..."
				}
				fmt.Fprintf(os.Stderr, "[tool] %s done (%d bytes)\n", e.Name, len(e.Result))
			}
		case agent.EventMessageEnd:
			lastUsage = e.Usage
		case agent.EventTurnEnd:
			if e.Usage != nil {
				lastUsage = e.Usage
			}
		case agent.EventCompactionStart:
			fmt.Fprintf(os.Stderr, "\n[compaction] starting: %d tokens\n", e.TokensBefore)
		case agent.EventCompactionEnd:
			fmt.Fprintf(os.Stderr, "[compaction] complete: %d → %d tokens, %d messages summarized\n",
				e.TokensBefore, e.TokensAfter, e.MessagesRemoved)
		case agent.EventCompactionError:
			fmt.Fprintf(os.Stderr, "[compaction] error: %v\n", e.Err)
		case agent.EventError:
			fmt.Fprintf(os.Stderr, "\n[mok] error: %v\n", e.Err)
		}
	}

	agentErr := <-errCh

	elapsed := time.Since(startTime)
	fmt.Fprintln(os.Stderr)
	fmt.Fprint(os.Stderr, strings.Repeat("-", 60)+"\n")
	if lastUsage != nil {
		fmt.Fprintf(os.Stderr, "[mok] done in %s | tokens: prompt=%d completion=%d total=%d\n",
			elapsed.Round(time.Millisecond), lastUsage.PromptTokens, lastUsage.CompletionTokens, lastUsage.TotalTokens)
	} else {
		estimatedTokens := llm.EstimateTokens(prompt) + llm.EstimateTokens(strings.Repeat(" ", charCount))
		fmt.Fprintf(os.Stderr, "[mok] done in %s (%d chars, ~%d tokens estimated)\n",
			elapsed.Round(time.Millisecond), charCount, estimatedTokens)
	}

	return agentErr
}
