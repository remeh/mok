# mok

mok is a terminal-based coding agent harness with multi-agent flow orchestration
for complex executions.

Developed in Go, uses [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI.

Talks to OpenAI-compatible LLM endpoints with SSE streaming.
Handles tool calls (read, write, edit, bash), thinking and loops. Strong focus
on running perfectly with non-frontier models.


## Features

* Interactive TUI with markdown rendering and scrollable message history.
* Big focus on local models quirk handling (Gemma, Qwen XML tool calls, empty responses, JSON repair).
* Multi-agent flow orchestration, chain agents with isolated context and handoff summaries.
* Non-interactive prompt mode (`-p`) for one-shot queries and scripting.
* Streaming text and reasoning/thinking tokens.
* Built-in tools: file read, write, edit (unified diff), bash (timeout, truncation).
* Configurable allowlist or blocklist for bash command execution.
* Automatic context compaction with LLM-driven summarization and hard cut fallback.
* Session save/restore (`-session`).
* Slash commands: model switching, debug toggle, compaction, flow selection, YOLO mode.

![Screenshot](https://raw.githubusercontent.com/remeh/mok/refs/heads/main/screenshot.png)

## Build

```
$ go build -o mok cmd/mok/main.go
```

## Quick Start

```
$ ./mok -t 120 -p "Write a Python function to sort a list" \
    -endpoint http://localhost:8000/v1 -model gemma4-e4b
```

Or run the TUI:

```
$ ./mok
```

## Configuration

Copy the example config to your config directory and customise it:

```
$ mkdir -p ~/.config/mok
$ cp mok.example.yaml ~/.config/mok/config.yaml
```

`config.yaml` can be stored in these locations:

- `./mok.yaml` (current directory)
- `~/.config/mok/config.yaml` (user config)

See `mok.example.yaml` for all supported options: global settings, bash confirm policies, multi-agent definitions, and flows...

## Tests

```
$ go test ./...
```

Copyright (c) 2025 Remy Mathieu
