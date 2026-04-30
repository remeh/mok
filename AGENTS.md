# mmok — Terminal Coding Agent

## What It Is

A terminal-based coding agent harness built with Go and [bubbletea](https://github.com/charmbracelet/bubbletea).
Supports both interactive TUI mode and non-interactive prompt mode (single-shot, streaming to stdout).

## Build

```
go build -o mmok cmd/mmok/main.go
```

## Test

```
go test -v ./...
```

After having build the `mmok` binary, you can run `./mmok  -t 120 -p "here is a prompt" -endpoint http://localhost:8000/v1 -model gemma4-e4b`
to test the execution / interpretation of a prompt: `-p` is the flag for the prompt, `-t` is a timeout in seconds.
Use it while developing changes to validate your work. Note that model `qwen3.5-9b-thinking` is available as well.

## Architecture

```
cmd/mmok/main.go            — CLI entry point, flags, prompt mode runner
internal/agent/
  agent.go                  — Agent struct, conversation history, context tracker
  events.go                 — Agent event types (TurnStart, TextDelta, ThinkingDelta, etc.)
  loop.go                   — Agent loop: build context → stream → collect → repeat
  prompt.go                 — System prompt builder (date, CWD)
internal/app/
  config_types.go           — Config struct + defaults
  config.go                 — YAML file → env vars → CLI flags precedence chain
  app.go                    — bubbletea Model (Update/View), agent event wiring
internal/llm/
  client.go                 — OpenAI-compatible API client, SSE streaming parser
  tokenizer.go              — Token estimation + ContextTracker
internal/tui/
  screen.go                 — Composes MessageView + InputArea + StatusBar
  message_view.go           — Scrollable message list with word wrap, thinking indicator
  input.go                  — Text input with cursor, history, line editing, focus
  statusbar.go              — Bottom bar: model name, token count, state
  theme.go                  — Lipgloss color/style definitions
  utils.go                  — Shared helpers (StringsRepeat)
internal/types/
  message.go                — Message type, tool call/result constructors
```

## Dependencies

- **bubbletea** (v1.3.10) — TUI framework
- **lipgloss** (v1.1.0) — Styling
- **muesli/reflow** — Word wrapping
- **gopkg.in/yaml.v3** — Config file parsing
- **caarlos0/env/v10** — Env struct tags (declared but not actively used)

## Configuration

Precedence: defaults → YAML file → env vars → CLI flags.

Supported config keys: `model`, `endpoint`, `bearer_token`, `max_context_tokens`, `compaction_threshold`, `keep_recent_tokens`, `temperature`, `max_tokens`, `model_quirks`.

Env vars: `MMOK_MODEL`, `MMOK_ENDPOINT`, `MMOK_BEARER_TOKEN`, `MMOK_MAX_CONTEXT_TOKENS`, `MMOK_COMPACTION_THRESHOLD`, `MMOK_KEEP_RECENT_TOKENS`, `MMOK_TEMPERATURE`, `MMOK_MAX_TOKENS`, `MMOK_MODEL_QUIRKS`.

File locations searched: `./mmok.yaml`, `./config.yaml`, `~/.config/mmok/config.yaml`.

## Current State

### Working
- CLI flags: `-p` (prompt mode), `-model`, `-endpoint`, `-bearer-token`, `-temperature`, `-max-tokens`, `-max-context-tokens`, `-t` (timeout), `-version`
- Prompt mode: single-shot streaming via `client.Stream()`, handles text + thinking events, shows token usage or local estimate, abort via stdin
- TUI: fully wired to LLM client via `internal/agent` package
- Agent loop: builds context (system prompt + history), streams text/thinking separately, collects assistant message, tracks tokens
- Agent events: `EventTurnStart`, `EventMessageStart`, `EventTextDelta`, `EventThinkingDelta`, `EventMessageEnd`, `EventTurnEnd`, `EventError`
- TUI message view: scrollable, word wrap, `[thinking]` collapsed indicator, cursor during streaming
- Abort: Ctrl+C / Esc aborts running agent, then quits
- Input disabled during agent running
- Context tracker: `ContextTracker` with `EstimateTokens` for token estimation
- Config: YAML + env + flags with proper precedence, `bearer_token` and `model_quirks` support
- System prompt: includes current date and working directory
- LLM client: OpenAI-compatible SSE streaming, context-aware abort via `http.NewRequestWithContext`, handles `reasoning_content` for thinking tokens
- Tool call parsing scaffolding in SSE parser (for Phase 2B)

### Not Yet Implemented
- No conversation history persistence (in-memory only)
- No context compaction (config fields exist but logic is absent)
- No tool call execution (parsing scaffolding exists in SSE client)
- No file attachment / context file support

## Next Steps

See [PLAN.md](./PLAN.md) for the full phase breakdown.

### Phase 2B — Tool Calls + JSON Repair
Add tool call accumulation from SSE, JSON repair, schema validation, tool interface/registry,
and extend the agent loop with tool call → execute → retry. Handle model quirks (Gemma, Qwen).

### Phase 3 — Built-in Tools
Implement actual tools: bash, read, edit, write.

### Phase 4 — Compaction
Implement context compaction with LLM-driven summarization.

### Phase 5 — MCP
Add Model Context Protocol support for external tool providers.
