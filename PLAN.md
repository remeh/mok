# mok — Go Coding Harness

A terminal-based coding agent harness in Go for local LLMs via llama-server (OpenAI-compatible endpoints).

## Design Principles
1. **Interfaces first** — Every subsystem defined by interface; implementations swappable
2. **Stream-first** — All LLM interaction is streaming
3. **Event-driven agent loop** — Typed event stream; UI and side-effects subscribe
4. **Chunked file I/O** — offset/limit for file reads
5. **Automatic compaction** — LLM-driven summarization at smart cut points
6. **MCP support** — Model Context Protocol as first-class tool providers

## Phase Overview

| Phase | What | Files |
|-------|------|-------|
| 1 | TUI + Config | See [phase-1-app.md](./phase-1-app.md) |
| 2A | Agent — Streaming, Thinking, TUI Wiring | See [phase-2a-agent.md](./phase-2a-agent.md) |
| 2B | Agent — Tool Calls, JSON Repair, Quirks | See [phase-2b-agent.md](./phase-2b-agent.md) |
| 3 | Tool Calling (built-in tools) | See [phase-3-tools.md](./phase-3-tools.md) |
| 4 | Compaction | See [phase-4-compaction.md](./phase-4-compaction.md) |
| 5 | MCP Servers | See [phase-5-mcp.md](./phase-5-mcp.md) |

## Project Structure

```
mok/
├── cmd/mok/main.go
├── internal/
│   ├── app/          # TUI app, config loading
│   ├── agent/        # Agent loop, events, messages, prompt builder
│   ├── llm/          # OpenAI-compatible client, SSE parser, token estimation
│   ├── tools/        # Tool registry, parser, validator, built-in tools
│   ├── compaction/   # Compaction orchestrator, summarizer, cut points
│   ├── mcp/          # MCP client, server manager, tool bridge
│   └── tui/          # TUI components (screen, messages, input, statusbar)
├── go.mod
└── README.md
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `charmbracelet/bubbletea` | Elm-architecture TUI framework |
| `charmbracelet/lipgloss` | TUI styling |
| `muesli/reflow` | Text wrapping |
| `ghodss/yaml` | YAML config parsing |
