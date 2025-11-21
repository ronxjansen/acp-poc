// Package client provides ACP (Agent Client Protocol) client implementation.
//
// # JSON-RPC Middleware for Extension Methods
//
// This file implements a middleware layer to handle ACP extension methods that are not
// part of the standard protocol. According to the ACP extensibility specification,
// any method name starting with an underscore (_) is reserved for custom extensions.
//
// ## Why This Middleware is Needed
//
// The acp-go-sdk (v0.6.3) generates a client-side request handler in client_gen.go that
// only routes standard ACP methods (like fs/read_text_file, fs/write_text_file, etc.) to
// the Client interface implementation. When an extension method request arrives (e.g.,
// _fs/grep_search), the generated handler returns "Method not found" because it's not
// in the hardcoded switch statement.
//
// ## How This Middleware Works
//
// The JSONRPCMiddleware wraps the TCP connection's io.Reader and intercepts incoming
// JSON-RPC requests before they reach the acp-go-sdk's connection handler:
//
//  1. Read incoming JSON-RPC request from the TCP connection
//  2. Parse the request to check if method starts with underscore (_)
//  3. If it's an extension method:
//     - Call our custom ExtensionMethodHandler.HandleExtensionMethod()
//     - Send the response directly back through the writer
//     - Continue reading (effectively "consuming" the request)
//  4. If it's a standard method:
//     - Pass the request through to the SDK's normal handling
//
// ## ACP Extensibility Protocol
//
// According to the ACP specification (https://agentclientprotocol.com/protocol/extensibility):
//   - Method names starting with _ are reserved for custom extensions
//   - Extension methods follow standard JSON-RPC 2.0 semantics
//   - Clients should respond with "Method not found" if they don't recognize an extension
//   - This mechanism allows adding custom functionality while maintaining protocol compatibility
//
// ## Usage
//
// The middleware is integrated in NewACPClient():
//
//	baseReader := bufio.NewReader(conn)
//	writer := &flushingWriter{bufio.NewWriter(conn)}
//	reader := NewJSONRPCMiddleware(ctx, baseReader, writer, client)
//	client.conn = acp.NewClientSideConnection(client, writer, reader)
//
// The ACPClient implements ExtensionMethodHandler to process custom methods like _fs/grep_search.
package client

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// ExtensionMethodHandler handles extension methods
type ExtensionMethodHandler interface {
	HandleExtensionMethod(ctx context.Context, method string, params map[string]interface{}) (interface{}, error)
}

// JSONRPCMiddleware wraps io.Reader to intercept and handle extension method requests
type JSONRPCMiddleware struct {
	underlying io.Reader
	handler    ExtensionMethodHandler
	writer     io.Writer
	buffer     []byte
	ctx        context.Context
	scanner    *bufio.Scanner // Persistent scanner to avoid recreation on each Read
}

// NewJSONRPCMiddleware creates a new JSON-RPC middleware
func NewJSONRPCMiddleware(ctx context.Context, reader io.Reader, writer io.Writer, handler ExtensionMethodHandler) *JSONRPCMiddleware {
	return &JSONRPCMiddleware{
		underlying: reader,
		handler:    handler,
		writer:     writer,
		ctx:        ctx,
		buffer:     make([]byte, 0),
		scanner:    bufio.NewScanner(reader), // Initialize scanner once
	}
}

// Read implements io.Reader
func (m *JSONRPCMiddleware) Read(p []byte) (n int, err error) {
	// If we have buffered data from a previous interception, return it first
	if len(m.buffer) > 0 {
		n = copy(p, m.buffer)
		m.buffer = m.buffer[n:]
		return n, nil
	}

	// Read from underlying reader using persistent scanner
	if !m.scanner.Scan() {
		if err := m.scanner.Err(); err != nil {
			return 0, err
		}
		return 0, io.EOF
	}

	line := m.scanner.Bytes()

	// Try to parse as JSON-RPC request
	var req JSONRPCRequest
	if err := json.Unmarshal(line, &req); err != nil {
		// Not a valid JSON-RPC request, pass through
		n = copy(p, line)
		n += copy(p[n:], []byte("\n"))
		return n, nil
	}

	// Check if this is an extension method (starts with underscore)
	if strings.HasPrefix(req.Method, "_") && m.handler != nil {
		// Handle extension method
		var params map[string]interface{}
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				// Log error but continue with nil params
				params = nil
			}
		}

		result, handlerErr := m.handler.HandleExtensionMethod(m.ctx, req.Method, params)

		// Create response
		var resp JSONRPCResponse
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		if handlerErr != nil {
			resp.Error = map[string]interface{}{
				"code":    -32000,
				"message": handlerErr.Error(),
			}
		} else {
			resp.Result = result
		}

		// Send response directly to writer
		respBytes, err := json.Marshal(resp)
		if err != nil {
			// If we can't marshal the response, send an error response
			resp.Result = nil
			resp.Error = map[string]interface{}{
				"code":    -32603,
				"message": "Internal error: failed to marshal response",
			}
			respBytes, _ = json.Marshal(resp)
		}
		respBytes = append(respBytes, '\n')
		if _, err := m.writer.Write(respBytes); err != nil {
			return 0, err
		}

		// Return empty to continue reading
		return m.Read(p)
	}

	// Not an extension method, pass through
	n = copy(p, line)
	n += copy(p[n:], []byte("\n"))
	return n, nil
}
