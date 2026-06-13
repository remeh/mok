# mok — Terminal Coding Agent

A terminal-based coding agent harness built with Go and [bubbletea](https://github.com/charmbracelet/bubbletea).
Supports interactive TUI mode, non-interactive prompt mode (single-shot, streaming to stdout),
and multi-agent flow orchestration.

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
`-p` is the prompt, `-t` is timeout in seconds.

## Architecture

```
cmd/mok/main.go            — CLI entry point, flags, prompt mode runner
internal/agent/
  agent.go                  — Agent struct, conversation history, context tracker, compaction
  events.go                 — Agent event types (TurnStart, TextDelta, ThinkingDelta, ToolCall*, ToolResult, Compaction*, Flow*)
  loop.go                   — Agent loop: build context → stream → collect → execute tools → repeat
  prompt.go                 — System prompt builder (date, CWD, PromptPrefix for role-specific prompting)
  debug.go                  — DebugLogger for agent subsystem
internal/app/
  config_types.go           — Config struct + defaults
  config.go                 — YAML file → env vars → CLI flags precedence chain
  app.go                    — bubbletea Model (Update/View), agent event wiring, slash commands
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
  xml_tool_call.go          — Extracts XML-style tool calls (e.g. Qwen <function>...)
  malformed_tool_calls.go   — Sanitizes/repairs tool call args; drops unrepairable ones
internal/tools/
  tool.go                   — Tool interface + ToolDefinition + Registry
  validator.go              — ValidateAndCoerce with type coercion
  path_utils.go             — Path resolution (~, relative, absolute) + safety checks
  read.go                   — Read tool (offset/limit/truncation, image support)
  write.go                  — Write tool (auto-create parent dirs)
  edit.go                   — Edit tool (multi-edit, unified diff output)
  bash.go                   — Bash tool (timeout, output truncation)
internal/compaction/
  compaction.go             — Compactor orchestrator, LLM-driven summarization, hard cut fallback
  summarizer.go             — Hybrid summarization (programmatic extraction + LLM refinement)
  cutpoint.go               — Smart cut point detection based on token thresholds
  context.go                — Token estimation for message lists
  file_ops.go               — Programmatic file operation extraction from messages
  prompts.go                — Summarization prompt templates
internal/tui/
  screen.go                 — Composes MessageView + InputArea + StatusBar + AutocompleteView
  message_view.go           — Scrollable message list with pinning, word wrap, glamour markdown rendering
  input.go                  — Text input with cursor, history, line editing, focus, command autocomplete
  statusbar.go              — Bottom bar: model name, token usage, state, scroll hint
  theme.go                  — Lipgloss color/style definitions
  utils.go                  — Shared helpers (StringsRepeat)
  markdown.go               — Glamour-based markdown renderer
  autocomplete_view.go      — Command/model suggestion dropdown
  model_selector.go         — Interactive model selection from API
internal/types/
  message.go                — Message type, tool call/result constructors
internal/session/
  session.go                — Session data structures, serialization, conversion (app↔LLM)
  storage.go                — Session file management (~/.mok/sessions/)
internal/flow/
  types.go                  — AgentDefinition, FlowDefinition
  config.go                 — AgentConfig with Resolved*() methods (per-agent vs global precedence)
  factory.go                — AgentFactory: builds *agent.Agent from AgentDefinition; LLM client cache
  handoff.go                — BuildHandoffMessage, HandoffOptions, BuildHandoffSummary
  result.go                 — AgentRunResult: captures agent output (messages, tokens, error, timing)
  orchestrator.go           — FlowOrchestrator: sequential flow execution, event bridging, handoff loop
```

## Dependencies

- **bubbletea** (v1.3.10) — TUI framework
- **lipgloss** (v1.1.1) — TUI styling
- **glamour** (v1.0.0) — Markdown rendering (indirect, via lipgloss)
- **muesli/reflow** — Word wrapping
- **gopkg.in/yaml.v3** — Config file parsing
- **sergi/go-diff** (v1.4.0) — Unified diff generation for edit tool

## Configuration

Precedence: defaults → YAML file → env vars → CLI flags.

**Config keys** (`config_types.go`):
- `model` — LLM model name
- `endpoint` — OpenAI-compatible API endpoint
- `bearer_token` — API authentication token
- `cwd` — Working directory
- `system_prompt` — Custom system prompt (for one-shot runs)
- `max_context_tokens` — Maximum context window size
- `compaction_threshold` — Trigger compaction at this fraction of max context (e.g., 0.8)
- `keep_recent_tokens` — Minimum tokens to preserve at end of history after compaction
- `summarization_model` — Optional separate model for summarization (defaults to main model)
- `max_tokens` — Maximum response tokens
- `debug` — Enable debug logging
- `ui_log_path` — Path for UI session logs (requires debug mode to be enabled)
- `enable_multiline` — Enable multi-line editing (default: true)
- `enable_autocomplete` — Enable command autocomplete (default: true)
- `autocomplete_max_items` — Max suggestions to show (default: 10)
- `tab_completes` — Enable Tab for completion (default: true)
- `agents` — Named agent definitions for multi-agent mode (optional, absent = single-agent)
- `flows` — Named flow definitions (ordered lists of agent names)
- `default_flow` — Default flow to run when none specified

**Env vars**: `MOK_MODEL`, `MOK_ENDPOINT`, `MOK_BEARER_TOKEN`, `MOK_SYSTEM_PROMPT`, `MOK_MAX_CONTEXT_TOKENS`, `MOK_COMPACTION_THRESHOLD`, `MOK_KEEP_RECENT_TOKENS`, `MOK_MAX_TOKENS`, `MOK_DEBUG`, `MOK_UI_LOG_PATH`, `MOK_ENABLE_MULTILINE`, `MOK_ENABLE_AUTOCOMPLETE`, `MOK_AUTOCOMPLETE_MAX_ITEMS`, `MOK_TAB_COMPLETES`.

**CLI flags**: `-model`, `-endpoint`, `-bearer-token`, `-system-prompt`, `-max-context-tokens`, `-max-tokens`, `-debug`, `-p` (prompt), `-t` (timeout), `-version`, `-ui-log-path`, `-session` (restore session).

**File locations**: `./mok.yaml`, `./config.yaml`, `~/.config/mok/config.yaml`.

### Multi-Agent Flow Configuration

When `agents` and `flows` are defined in the config, mok operates in **multi-agent mode**.
Each agent can have its own model, system prompt, and config overrides. Flows define ordered
sequences of agents that execute sequentially with context handoffs.

```yaml
# Global defaults (fallback for all agents)
model: qwen3.6-35b-a3b-coder
endpoint: http://localhost:8080/v1
max_context_tokens: 131072
compaction_threshold: 0.8
keep_recent_tokens: 16384

agents:
  senior:
    model: qwen3.5-122b-coder
    prompt: "You are a senior software developer and architect."
    max_context_tokens: 131072
  coder:
    model: qwen3.6-27b-coder
    prompt: "You are an expert developer. Implement code according to specifications."
  reviewer:
    model: qwen3.5-122b-coder
    prompt: "You are an expert code reviewer. Provide actionable feedback."

flows:
  implementation: [senior, coder, reviewer, coder]
  review: [reviewer, coder]
  quick-fix: [coder]

default_flow: "implementation"
```

**Agent field precedence** (per-agent): agent-specific value → global config → env vars → CLI flags → defaults.

**Required fields per agent**: `model` and `prompt`. All other fields (`endpoint`, `max_tokens`,
`max_context_tokens`, `compaction_threshold`, `keep_recent_tokens`, `summarization_model`) inherit
from the global config.

**Handoff protocol**: When agent N finishes, its output is programmatically summarized (key points,
file operations) and injected as a handoff message into agent N+1. Each agent runs with its own
isolated context — its system prompt + the handoff message. Full context history is never mixed
between agents.

**TUI commands**: `/flow` lists available flows, `/flow <name>` selects a flow and prompts for
the user request, `/flow stop` cancels the active flow. The status bar shows flow progress:
`flow: implementation [2/5] coder`.

## One-Shot Runs

Use the `-system-prompt` flag with `-p` to run mok as a one-shot LLM client without implementing your own API client library:

```bash
./mok -p "Write a Python function to sort a list" \
  -system-prompt "You are a Python coding assistant. Provide concise, correct code with brief explanations." \
  -endpoint http://localhost:8000/v1 \
  -model gemma4-e4b \
  -t 120
```

This is useful for:
- Quick LLM queries without TUI overhead
- Scripting and automation
- Testing different system prompts
- Integrating mok as a lightweight LLM client

When `-system-prompt` is provided, it overrides the default coding assistant prompt. Without it, the default prompt (with tools, date, CWD, and context files) is used.

## Current State

### Working

**Agent loop**:
- Builds context (system prompt + conversation history)
- Streams text/thinking separately
- Collects assistant message
- Tracks tokens (estimated + server-reported)
- Executes tool calls in retry loop (up to 5000 iterations)
- Supports tool call → execute → retry cycle
- Handles cancellation with cleanup

**Events** (typed event stream):
- `EventTurnStart`, `EventTurnEnd`
- `EventMessageStart`, `EventMessageEnd`
- `EventTextDelta`, `EventThinkingDelta`
- `EventToolCallStart`, `EventToolCallUpdate`, `EventToolCallEnd`
- `EventToolResult`
- `EventCompactionStart`, `EventCompactionEnd`, `EventCompactionError`
- `EventFlowStart`, `EventFlowStepStart`, `EventFlowStepEnd`, `EventFlowEnd`
- `EventError`

**LLM client**:
- OpenAI-compatible SSE streaming
- Context-aware abort via `http.NewRequestWithContext`
- `reasoning_content` for thinking tokens
- `X-Client-ID` header for session affinity
- No client-side timeout (bounded by caller context)

**Tool calls**:
- JSON accumulation with index-first + ID fallback (Gemma quirk)
- XML tool call detection (Qwen quirk)
- Argument sanitization/retry (malformed_tool_calls quirk)
- Empty response retry
- ValidateAndCoerce with type coercion against schema

**JSON repair** (three-layer fallback):
1. Direct parse
2. Repair control chars/invalid escapes
3. Close unclosed braces

**Built-in tools**:
- `read` — File reads with offset/limit/truncation, image support
- `write` — File writes with auto-create parent dirs
- `edit` — Multi-edit with unified diff output
- `bash` — Command execution with timeout and output truncation

**Tool registry**:
- `Registry` with `Add`/`Get`/`All`/`Has`/`ToSpecs`
- Sorted by name for deterministic ordering (prompt cache affinity)
- `ValidateAndCoerce` with type coercion

**Context management**:
- `ContextTracker` with estimated + server-reported token counts
- Automatic compaction when threshold reached
- Manual compaction via `/compact` command
- LLM-driven summarization with programmatic file operation extraction
- Hybrid summarization: extract key points + LLM refinement
- Hard cut fallback if summarization fails
- Preserves recent context (`keep_recent_tokens`)

**Compaction events**:
- `EventCompactionStart` — Emitted when compaction begins
- `EventCompactionEnd` — Emitted with token reduction stats
- `EventCompactionError` — Emitted on failure (including cancellation)

**TUI**:
- Scrollable message view with pinning (Ctrl+T/B, PgUp/PgDn, mouse wheel)
- Glamour markdown rendering for assistant messages
- Collapsed tool results (click or Ctrl+O to expand all)
- `[thinking]` collapsed indicator (expandable)
- Animated cursor during streaming
- Status bar with dot animation (`streaming`, `processing`, `compacting`, `executing: <tool>`, `● ready`, `✗ error`)
- Scroll hint (`↓N`) when scrolled above bottom
- Input with history navigation (Up/Down), line editing
- Command autocomplete (Tab or Ctrl+Space)
- Model selector (`/model` command) with interactive selection
- Slash commands: `/model`, `/debug on|off`, `/clear`, `/compact`, `/flow [name]`, `/yolo`, `/quit`, `/exit`, `/help`
- External editor support (Ctrl+G, uses `$EDITOR`)
- Turn stats display (timestamp, duration, token count)
- Multi-agent flow orchestration with context handoff between agents

**Prompt mode** (non-interactive):
- Single-shot via full agent loop (text + thinking + tool execution)
- Streaming to stdout
- Abort via signal (Ctrl+C, SIGTERM)
- Shows token usage or local estimate
- Tool execution status to stderr

**Debug logging**:
- `DebugLogger` with categories (AGENT, STREAM, EVENT, TOOL, HTTP, SSE, QUIRK, CONTEXT)
- Optional file output (`debug.log`)
- Toggle via `/debug on|off` command or `-debug` flag

**UI session logging**:
- Persistent conversation logs when debug mode is enabled
- Includes model, endpoint, full message history
- Configurable via `MOK_UI_LOG_PATH` or `-ui-log-path`

**Quit summary**: When quitting the TUI (via `/quit`, `/exit`, or Ctrl+C), the conversation history is automatically printed to stdout in a collapsed, scrollable format. This allows you to review the entire session after exiting:
- User messages shown with `>` prefix and timestamps
- Assistant messages with `[thinking] (collapsed)` indicator
- Tool calls and results shown in collapsed form with summaries
- Plain text output (no ANSI codes) for easy redirection
- Accessible via both interactive quit commands and Ctrl+C

**Session restoration**:
- Automatic session saving when quitting after user activity
- Session files stored in `~/.mok/sessions/` as JSON (e.g., `session_20240115_143022.json`)
- Restore with `-session <path>` flag
- Restores full conversation history, model/endpoint config, token count, agent LLM history
- CLI flags take precedence over restored session config
- No session file created if user quits without sending any prompts
- Restore instruction printed to stderr on quit

### Not Yet Implemented

- Session listing/selection UI (`/sessions` command)
- Manual session save (`/save` command)
- Session naming/categorization
- Flow session persistence (flow metadata not yet stored in session files)
- `-flow` CLI flag for non-interactive flow execution
- Per-agent tool filtering (all agents share the same tool registry)
- MCP (Model Context Protocol) support for external tool providers
- File attachment / context file support
