# Phase 5: MCP Server Support

## Goals
- Connect to MCP servers via stdio transport
- Discover tools and resources from MCP servers
- Bridge MCP tools into the harness tool registry
- Support multiple concurrent MCP servers

## What is MCP?

Model Context Protocol (MCP) is a standard for connecting AI models to external tools
and data sources. Servers expose tools and resources via JSON-RPC over stdio or SSE.

Key MCP concepts:
- **Tools**: Callable functions (like our built-in tools)
- **Resources**: Addressable data sources (files, API data, etc.)
- **Prompts**: Pre-built prompt templates

For mmok, we focus on **tools** first, then resources.

## MCP JSON-RPC Client

```go
package mcp

// Client connects to an MCP server via stdio.
type Client struct {
    server   *os.Process
    rw       *bufio.ReadWriter
    mux      *jsonrpc.Mux  // JSON-RPC message multiplexer
    tools    []MCPTool
    resources []MCPResource
}

// MCPServerConfig defines how to start an MCP server.
type MCPServerConfig struct {
    Name    string
    Command string
    Args    []string
    Env     []string
}

func NewClient(ctx context.Context, config MCPServerConfig) (*Client, error)
func (c *Client) Initialize(ctx context.Context) error
func (c *Client) ListTools(ctx context.Context) ([]MCPTool, error)
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error)
func (c *Client) ListResources(ctx context.Context) ([]MCPResource, error)
func (c *Client) ReadResource(ctx context.Context, uri string) (string, error)
func (c *Client) Close() error

type MCPTool struct {
    Name        string
    Description string
    InputSchema map[string]interface{} // JSON Schema
}

type MCPResource struct {
    URI         string
    Name        string
    Description string
    MIMEType    string
}

type ToolResult struct {
    Content []ContentBlock
    IsError bool
}

type ContentBlock struct {
    Type string // "text" | "image"
    Text string
}
```

### JSON-RPC Protocol

MCP uses JSON-RPC 2.0. Key methods:

```
initialize → {protocolVersion, capabilities, serverInfo}
initialized  → notification (client sends after initialize)
tools/list   → {tools: [...]}
tools/call   → {name, arguments} → {content: [...], isError}
resources/list → {resources: [...]}
resources/read → {uri} → {contents: [...]}
```

### Stdio Transport

```go
func startServer(command string, args []string, env []string) (*os.Process, io.WriteCloser, io.Reader, error) {
    cmd := exec.Command(command, args...)
    cmd.Env = append(os.Environ(), env...)
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    cmd.Stderr = os.Stderr  // Server errors go to stderr
    cmd.Start()
    return cmd, stdin, stdout, nil
}
```

Messages are sent as newline-delimited JSON:
```
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"mmok","version":"0.1.0"}}}
```

## Tool Bridge

Bridge MCP tools into the harness tool registry:

```go
package mcp

// ToolBridge adapts MCP tools to the harness Tool interface.
type ToolBridge struct {
    client *Client
    prefix string  // e.g., "filesystem_" to avoid name collisions
}

func (b *ToolBridge) ToHarnessTool(mcpTool MCPTool) tools.Tool {
    return &BridgedTool{
        Name:        b.prefix + mcpTool.Name,
        Description: mcpTool.Description,
        InputSchema: mcpTool.InputSchema,
        Execute: func(args json.RawMessage) (string, error) {
            var argsMap map[string]interface{}
            json.Unmarshal(args, &argsMap)
            result, err := b.client.CallTool(context.Background(), mcpTool.Name, argsMap)
            if err != nil {
                return "", err
            }
            return formatMCPResult(result), nil
        },
    }
}

func formatMCPResult(result *ToolResult) string {
    var buf strings.Builder
    for _, block := range result.Content {
        if block.Type == "text" {
            buf.WriteString(block.Text)
        }
    }
    return buf.String()
}
```

## Server Manager

Manage multiple MCP servers:

```go
package mcp

// Manager handles multiple MCP servers.
type Manager struct {
    servers map[string]*Client
}

func NewManager() *Manager

// Start starts an MCP server and discovers its tools.
func (m *Manager) Start(ctx context.Context, config MCPServerConfig) error {
    client, err := NewClient(ctx, config)
    if err != nil {
        return err
    }
    if err := client.Initialize(ctx); err != nil {
        return err
    }
    m.servers[config.Name] = client
    return nil
}

// DiscoverTools lists all tools from all servers.
func (m *Manager) DiscoverTools(ctx context.Context) ([]MCPTool, error) {
    var all []MCPTool
    for name, client := range m.servers {
        tools, err := client.ListTools(ctx)
        if err != nil {
            log.Printf("warning: failed to list tools from %s: %v", name, err)
            continue
        }
        all = append(all, tools...)
    }
    return all, nil
}

// Close shuts down all servers.
func (m *Manager) Close() error
```

## Integration with Tool Registry

```go
// In app startup:
func setupTools(config *Config) (*tools.Registry, *mcp.Manager, error) {
    registry := tools.NewRegistry()

    // Register built-in tools
    registry.Register(&tools.ReadTool{CWD: cwd})
    registry.Register(&tools.WriteTool{CWD: cwd})
    registry.Register(&tools.EditTool{CWD: cwd})
    registry.Register(&tools.BashTool{CWD: cwd})

    // Start MCP servers
    manager := mcp.NewManager()
    for _, serverConfig := range config.MCPServers {
        if err := manager.Start(ctx, serverConfig); err != nil {
            return nil, nil, fmt.Errorf("mcp server %s: %w", serverConfig.Name, err)
        }
    }

    // Bridge MCP tools
    mcpTools, err := manager.DiscoverTools(ctx)
    if err != nil {
        return nil, nil, err
    }

    bridge := &mcp.ToolBridge{Manager: manager}
    for _, tool := range mcpTools {
        registry.Register(bridge.ToHarnessTool(tool))
    }

    return registry, manager, nil
}
```

## Config Extension

```yaml
# Add to config.yaml
mcp_servers:
  - name: "filesystem"
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/home/user/projects"]
  - name: "github"
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-github"]
    env:
      - "GITHUB_TOKEN=${GITHUB_TOKEN}"
```

## Tasks

1. [ ] Implement `internal/mcp/client.go`: JSON-RPC client over stdio
2. [ ] Implement `internal/mcp/server.go`: Server manager (start/stop/discover)
3. [ ] Implement `internal/mcp/bridge.go`: MCP → harness tool bridge
4. [ ] Extend config to support MCP server definitions
5. [ ] Wire MCP tools into tool registry at startup
6. [ ] Test: Connect to an MCP server, discover tools, call a tool
