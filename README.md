# ACP Poc

An Elixir implementation of an AI agent using the Agent Client Protocol (ACP) using req_llm, plus a basic TUI ACP client using Go and Bubble Tea.

## Features

- **ACP Protocol Compliance**: Implements the Agent Client Protocol for editor-agent communication
- **Tool Integration**: Demonstrates function calling with weather-specific tools
- **Streaming Updates**: Real-time response streaming to clients

## Prerequisites

- Elixir 1.18 or later
- An OpenAI API key (or Anthropic API key)

## Getting started

1. Start the Elixir server: mix run --no-halt
2. Start the TUI: go run tui/main.go

## Architecture

### Components

**Elixir Server:**
- **TuiAcp.Agent**: Main agent module implementing the `ACPEx.Agent` behavior
- **TuiAcp.TcpServer**: ACP server handling JSON-RPC communication over TCP sockets
- **TuiAcp.Server**: Legacy stdio server (for backward compatibility)
- **TuiAcp.Application**: Configurable supervisor that starts TCP or stdio server

**Go TUI Client:**
- **main.go**: Bubbletea-based terminal UI for interactive chat
- **acp_client.go**: ACP client implementation that connects to the agent via TCP
- Supports real-time streaming of agent responses
- Default connection: `localhost:9090`

**Communication:**
- Protocol: Agent Client Protocol (ACP) over TCP sockets
- Format: Line-delimited JSON-RPC 2.0 messages
- The server and client run as separate, independent processes

## TODO
## v0.0.1
- [x] Refactor TUI and Elixir code
- [x] Remove debuggin info below input
- [x] Add dir list client capability
- [ ] Copy prompt

## v0.0.2
- [ ] Handle reconnection logic. Client should try to reconnect if the server is down. 
- [ ] Add MCP as client capability
