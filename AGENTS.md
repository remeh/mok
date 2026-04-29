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

You can run `./mmok -t 120 -p "here is a prompt"` to test the execution / interpretation
of a prompt. `-p` is the flag for the prompt, `-t` is a timeout in seconds. Use it while
developing to validate your work.

## Architecture

```
cmd/mmok/main.go            — CLI entry point, flags, prompt mode runner
internal/app/
  config_types.go           — Config struct + defaults
  config.go                 — YAML file → env vars → CLI flags precedence chain
  app.go                    — bubbletea Model (Update/View), message submission
internal/llm/
  client.go                 — OpenAI-compatible API client, SSE streaming parser
internal/tui/
  screen.go                 — Composes MessageView + InputArea + StatusBar
  message_view.go           — Scrollable message list with word wrap
  input.go                  — Text input with cursor, history, line editing
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

Precedence: defaults → YAML file → `MMOK_*` env vars → CLI flags.

Supported config keys: `model`, `endpoint`, `max_context_tokens`, `compaction_threshold`, `keep_recent_tokens`, `temperature`, `max_tokens`.

File locations searched: `./mmok.yaml`, `./config.yaml`, `~/.config/mmok/config.yaml`.

## Current State

### Working
- CLI flags: `-p` (prompt mode), `-model`, `-endpoint`, `-temperature`, `-max-tokens`, `-max-context-tokens`, `-t` (timeout), `-version`
- Prompt mode: single-shot non-interactive streaming to stdout
- TUI: message view with scroll, input with cursor/history/line editing, status bar
- Slash commands: `/exit`, `/quit`
- LLM client: OpenAI-compatible SSE streaming
- Config: YAML + env + flags with proper precedence

### Not Yet Implemented
- TUI does not yet connect to the LLM client (messages echo back instead of streaming real responses)
- No conversation history persistence
- No context compaction (config fields exist but logic is absent)
- No tool call execution
- No file attachment / context file support

## Next Steps

### 1. Wire TUI → LLM streaming
The `llm.Client` works (used in prompt mode). Connect it to the TUI so that submitting a message triggers a real streaming response instead of the `(echo)` placeholder. This means:
- Track conversation history in `AppModel`
- Build `[]llm.ChatMsg` from `[]*types.Message`
- Stream chunks back into the TUI via `SetPartialText` / `SetStreaming`
- Handle errors gracefully in the status bar

### 2. Conversation persistence
Save/load conversation history to a file so sessions survive restarts.

### 3. Context compaction
Implement the compaction logic hinted at by `CompactionThreshold` and `KeepRecentTokens` in config — summarize older messages when context is near capacity.
