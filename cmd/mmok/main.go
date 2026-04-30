package main

import (
	"bufio"
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

	flags := map[string]string{
		"model":              *model,
		"endpoint":           *endpoint,
		"max-context-tokens": fmt.Sprintf("%d", *maxContext),
		"temperature":        fmt.Sprintf("%f", *temperature),
		"max-tokens":         fmt.Sprintf("%d", *maxTokens),
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

	if err := app.Run(cfg); err != nil {
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

	client := llm.NewClient(cfg.Endpoint, cfg.BearerToken)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	req := &llm.ChatRequest{
		Model:       cfg.Model,
		Messages:    messages,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	}

	startTime := time.Now()
	var charCount int
	var usage *llm.Usage

	fmt.Fprintf(os.Stderr, "\n[mmok] model=%s endpoint=%s timeout=%ds\n", cfg.Model, cfg.Endpoint, timeoutSec)
	fmt.Fprintln(os.Stderr, strings.Repeat("-", 60))

	eventChan, err := client.Stream(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[mmok] error: %v\n", err)
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	go func() {
		for scanner.Scan() {
			cancel()
		}
	}()

	for event := range eventChan {
		switch event.Type {
		case "text":
			fmt.Print(event.Text)
			charCount += len(event.Text)
		case "thinking":
			// Skip thinking tokens in prompt mode
		case "done":
			usage = event.Usage
		case "error":
			fmt.Fprintf(os.Stderr, "\n[mmok] stream error: %v\n", event.Err)
		}
	}

	elapsed := time.Since(startTime)
	fmt.Fprint(os.Stderr, strings.Repeat("-", 60)+"\n")
	if usage != nil {
		fmt.Fprintf(os.Stderr, "[mmok] done in %s | tokens: prompt=%d completion=%d total=%d\n",
			elapsed.Round(time.Millisecond), usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	} else {
		// llama-server doesn't report usage in streaming mode; fall back to local estimate
		estimatedTokens := llm.EstimateTokens(prompt) + llm.EstimateTokens(strings.Repeat(" ", charCount))
		fmt.Fprintf(os.Stderr, "[mmok] done in %s (%d chars, ~%d tokens estimated)\n",
			elapsed.Round(time.Millisecond), charCount, estimatedTokens)
	}

	return nil
}
