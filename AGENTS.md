# mok — Terminal Coding Agent

## What It Is

A terminal-based coding agent harness built with Go and [bubbletea](https://github.com/charmbracelet/bubbletea).
Supports both interactive TUI mode and non-interactive prompt mode (single-shot, streaming to stdout).

## Build

```
go build -o mok cmd/mok/main.go
```

## Test

```
go test -v ./...
```

After building, test with:
```
./mok -t 120 -p "here is a prompt" -endpoint http://localhost:8000/v1 -model gemma4-e4b
```
`-p` is the prompt, `-t` is timeout in seconds. Models `gemma4-e4b` and `qwen3.5-9b-thinking` are available.

## Architecture

```
cmd/mok/main.go            — CLI entry point, flags, prompt mode runner
internal/agent/
  agent.go                  — Agent struct, conversation history, context tracker
  events.go                 — Agent event types (TurnStart, TextDelta, ThinkingDelta, ToolCall*, ToolResult)
  loop.go                   — Agent loop: build context → stream → collect → execute tools → repeat
  prompt.go                 — System prompt builder (date, CWD)
  debug.go                  — DebugLogger for agent subsystem
internal/app/
  config_types.go           — Config struct + defaults
  config.go                 — YAML file → env vars → CLI flags precedence chain
  app.go                    — bubbletea Model (Update/View), agent event wiring
internal/llm/
  client.go                 — OpenAI-compatible API client, SSE streaming parser
  accumulator.go            — Tool call accumulator (map+slice, index-first matching, ID fallback)
  json_repair.go            — JSON repair (control chars, invalid escapes, unclosed braces)
  tokenizer.go              — Token estimation + ContextTracker
  debug.go                  — DebugLogger interface + NopLogger
internal/quirks/
  empty_response.go         — Detects empty responses (stop with no content)
  sanitize.go               — Strips leaked reasoning/thinking XML tags from content
  thinking.go               — Uses thinking as content when content is empty
  xml_tool_call.go          — Extracts XML-style tool calls (e.g. Qwen \u2573<function>...\u2581)
  malformed_tool_calls.go   — Sanitizes/repairs tool call args; drops unrepairable ones
internal/tools/
  tool.go                   — Tool interface + ToolDefinition + Registry
  validator.go              — ValidateAndCoerce with type coercion
  path_utils.go             — Path resolution (~, relative, absolute) + safety checks
  read.go                   — Read tool (offset/limit/truncation, image support)
  write.go                  — Write tool (auto-create parent dirs)
  edit.go                   — Edit tool (multi-edit, unified diff output)
  bash.go                   — Bash tool (timeout, output truncation)
internal/tui/
  screen.go                 — Composes MessageView + InputArea + StatusBar
  message_view.go           — Scrollable message list with pinning, word wrap, glamour markdown rendering
  input.go                  — Text input with cursor, history, line editing, focus
  statusbar.go              — Bottom bar: model name, token count, state, scroll hint
  theme.go                  — Lipgloss color/style definitions
  utils.go                  — Shared helpers (StringsRepeat)
  markdown.go               — Glamour-based markdown renderer
internal/types/
  message.go                — Message type, tool call/result constructors
```

## Dependencies

- **bubbletea** (v1.3.10) — TUI framework
- **lipgloss** (v1.1.1) — Styling
- **glamour** (v1.0.0) — Markdown rendering (indirect, via lipgloss)
- **muesli/reflow** — Word wrapping
- **gopkg.in/yaml.v3** — Config file parsing
- **sergi/go-diff** (v1.4.0) — Unified diff generation for edit tool

## Configuration

Precedence: defaults → YAML file → env vars → CLI flags.

**Config keys** (`config_types.go`): `model`, `endpoint`, `bearer_token`, `cwd`, `max_context_tokens`, `compaction_threshold`, `keep_recent_tokens`, `max_tokens`, `debug`.

**Env vars**: `MMOK_MODEL`, `MMOK_ENDPOINT`, `MMOK_BEARER_TOKEN`, `MMOK_MAX_CONTEXT_TOKENS`, `MMOK_COMPACTION_THRESHOLD`, `MMOK_KEEP_RECENT_TOKENS`, `MMOK_MAX_TOKENS`, `MMOK_DEBUG`.

**CLI flags**: `-model`, `-endpoint`, `-bearer-token`, `-max-context-tokens`, `-max-tokens`, `-debug`, `-p` (prompt), `-t` (timeout), `-version`.

**File locations**: `./mok.yaml`, `./config.yaml`, `~/.config/mok/config.yaml`.

## Current State

### Working
- **Agent loop**: builds context (system prompt + history), streams text/thinking separately, collects assistant message, tracks tokens, executes tool calls in a retry loop (up to 5000 iterations)
- **Events**: `EventTurnStart`, `EventMessageStart`, `EventTextDelta`, `EventThinkingDelta`, `EventMessageEnd`, `EventTurnEnd`, `EventError`, `EventToolCallStart`, `EventToolCallUpdate`, `EventToolCallEnd`, `EventToolResult`
- **LLM client**: OpenAI-compatible SSE streaming, context-aware abort via `http.NewRequestWithContext`, `reasoning_content` for thinking tokens, `X-Client-ID` header for session affinity
- **Tool calls**: JSON accumulation with index-first + ID fallback (Gemma quirk), XML tool call detection (Qwen quirk), argument sanitization/retry (malformed_tool_calls quirk), empty response retry
- **JSON repair**: three-layer fallback (direct parse → repair control chars/escapes → close unclosed braces)
- **Built-in tools**: read (offset/limit/truncation, images), write (auto-create dirs), edit (multi-edit, unified diff), bash (timeout, output truncation)
- **Tool registry**: `Registry` with `Add`/`Get`/`All`/`Has`/`ToSpecs`, `ValidateAndCoerce` with type coercion
- **TUI**: scrollable message view with pinning, glamour markdown rendering for assistant messages, collapsed tool results, `[thinking]` collapsed indicator, animated cursor during streaming, status bar with dot animation (`streaming`, `processing`, `compacting`, `executing: <tool>`, `● ready`, `✗ error`), scroll hint (`↓N`), input with history and line editing
- **Prompt mode**: single-shot via full agent loop (text + thinking + tool execution), streaming to stdout, abort via signal, shows token usage or local estimate
- **Context tracker**: `ContextTracker` with `EstimateTokens` for token estimation
- **System prompt**: includes current date and working directory
- **Config**: YAML + env + flags with proper precedence, `bearer_token` and `debug` support
- **Debug**: `DebugLogger` with categories (AGENT, STREAM, EVENT, TOOL, HTTP, SSE, QUIRK, CONTEXT), optional file output

### Not Yet Implemented
- No conversation history persistence (in-memory only)
- No context compaction (config fields exist but logic is absent)
- No file attachment / context file support

## Next Steps

See [PLAN.md](./PLAN.md) for the full phase breakdown.

### Phase 4 — Compaction
Implement context compaction with LLM-driven summarization.

### Phase 5 — MCP
Add Model Context Protocol support for external tool providers.
