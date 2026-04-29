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

	"github.com/user/mmok/internal/app"
	"github.com/user/mmok/internal/llm"
)

var (
	version = "0.1.0"
	commit  = "dev"
	date    = "unknown"
)

func main() {
	// Define CLI flags
	showVersion := flag.Bool("version", false, "Show version")
	prompt := flag.String("p", "", "Prompt to execute (non-interactive mode)")
	timeout := flag.Int("t", 0, "Timeout in seconds for prompt mode (0=no limit)")
	model := flag.String("model", "", "LLM model name")
	endpoint := flag.String("endpoint", "", "API endpoint URL")
	maxContext := flag.Int("max-context-tokens", 0, "Max context tokens")
	temperature := flag.Float64("temperature", 0, "Sampling temperature")
	maxTokens := flag.Int("max-tokens", 0, "Max response tokens")

	flag.Parse()

	if *showVersion {
		fmt.Printf("mmok %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Build flags map for config loading
	flags := map[string]string{
		"model":              *model,
		"endpoint":           *endpoint,
		"max-context-tokens": fmt.Sprintf("%d", *maxContext),
		"temperature":        fmt.Sprintf("%f", *temperature),
		"max-tokens":         fmt.Sprintf("%d", *maxTokens),
	}

	// Load configuration
	cfg, err := app.LoadConfig(flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: config error: %v\n", err)
		cfg = app.DefaultConfig()
	}

	// Prompt mode (non-interactive)
	if *prompt != "" {
		if err := runPrompt(cfg, *prompt, *timeout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Interactive TUI mode
	if err := app.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runPrompt executes a single prompt and streams the response to stdout.
func runPrompt(cfg *app.Config, prompt string, timeoutSec int) error {
	// Build context with timeout
	ctx := context.Background()
	cancel := func() {}

	if timeoutSec > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	}
	defer cancel()

	// Handle SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create LLM client
	client := llm.New(cfg)

	// Build messages
	messages := []llm.ChatMsg{
		{Role: "user", Content: prompt},
	}

	// Track stats
	startTime := time.Now()
	var tokenCount int

	// Stream handler: print each chunk as it arrives
	handler := func(chunk string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			fmt.Print(chunk)
			tokenCount++
			return nil
		}
	}

	// Execute
	fmt.Fprintf(os.Stderr, "\n[mmok] model=%s endpoint=%s timeout=%ds\n", cfg.Model, cfg.Endpoint, timeoutSec)
	fmt.Fprintln(os.Stderr, strings.Repeat("-", 60))

	_, err := client.Chat(messages, handler)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[mmok] error: %v\n", err)
		return err
	}

	elapsed := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "\n%s\n[mmok] done in %s (%d chars)\n",
		strings.Repeat("-", 60), elapsed.Round(time.Millisecond), tokenCount)

	return nil
}
