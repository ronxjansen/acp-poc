package client

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	acp "github.com/coder/acp-go-sdk"
	"github.com/ron/tui_acp/tui/logger"
)

// MessageHandler defines the interface for handling message chunks
type MessageHandler interface {
	OnMessageChunk(ctx context.Context, text string) error
}

// GrepResult represents a single match from a grep search
type GrepResult struct {
	Path       string // File path
	LineNumber int    // Line number (1-indexed)
	Line       string // The matching line
	Match      string // The matched text
}

// DirectoryEntry represents a file or directory in a listing
type DirectoryEntry struct {
	Path  string      // Full path
	Name  string      // Base name
	IsDir bool        // Whether it's a directory
	Size  int64       // File size in bytes
	Mode  fs.FileMode // File mode and permissions
}

// Config contains configuration for creating an ACPClient
type Config struct {
	Address string
	Logger  logger.Logger
	Handler MessageHandler
}

// ACPClient implements the acp.Client interface and manages communication with an ACP agent
type ACPClient struct {
	mu sync.Mutex

	logger  logger.Logger
	handler MessageHandler

	sessionID  acp.SessionId
	conn       *acp.ClientSideConnection
	tcpConn    net.Conn
	tcpAddress string
	cwd        string // Working directory for resolving relative paths
}

// flushingWriter wraps a bufio.Writer and flushes after every write
type flushingWriter struct {
	*bufio.Writer
}

func (fw *flushingWriter) Write(p []byte) (n int, err error) {
	n, err = fw.Writer.Write(p)
	if err != nil {
		return n, err
	}
	err = fw.Writer.Flush()
	return n, err
}

// NewACPClient creates a new ACP client and connects to the specified TCP address
func NewACPClient(cfg Config) (*ACPClient, error) {
	if cfg.Logger == nil {
		cfg.Logger = logger.NewNoopLogger()
	}

	client := &ACPClient{
		logger:     cfg.Logger,
		handler:    cfg.Handler,
		tcpAddress: cfg.Address,
	}

	client.logger.Debug("Connecting to %s...", cfg.Address)
	conn, err := net.Dial("tcp", cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", cfg.Address, err)
	}
	client.logger.Debug("TCP connected")

	client.tcpConn = conn

	// Wrap TCP connection with buffered I/O for proper line-based communication
	// Use auto-flushing writer to ensure messages are sent immediately
	baseReader := bufio.NewReader(conn)
	writer := &flushingWriter{bufio.NewWriter(conn)}

	// Wrap reader with middleware to intercept extension method requests
	ctx := context.Background()
	reader := NewJSONRPCMiddleware(ctx, baseReader, writer, client)

	client.conn = acp.NewClientSideConnection(client, writer, reader)

	client.logger.Debug("Initializing ACP connection...")
	_, err = client.conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs:       acp.FileSystemCapability{ReadTextFile: true, WriteTextFile: true},
			Terminal: false,
		},
	})
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}
	client.logger.Debug("ACP initialized")

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	// Make sure cwd is an absolute path
	absCwd, err := filepath.Abs(cwd)
	if err == nil {
		cwd = absCwd
	}

	client.cwd = cwd
	client.logger.Debug("Working directory: %s", cwd)

	client.logger.Debug("Creating new session...")
	sessionResp, err := client.conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        cwd,
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	client.sessionID = sessionResp.SessionId
	client.logger.Debug("Session created: %s", sessionResp.SessionId)

	return client, nil
}

// SendPrompt sends a prompt to the agent and streams the response
func (c *ACPClient) SendPrompt(ctx context.Context, prompt string) error {
	c.mu.Lock()
	sessionID := c.sessionID
	c.mu.Unlock()

	c.logger.Info("Sending prompt: %s", prompt)
	_, err := c.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	})

	return err
}

// Close closes the ACP client and TCP connection
func (c *ACPClient) Close() error {
	if c.tcpConn != nil {
		c.tcpConn.Close()
	}
	return nil
}

// resolvePath resolves a path relative to the client's working directory
// If the path is already absolute, it returns it unchanged
func (c *ACPClient) resolvePath(path string) string {
	// If path is already absolute, return it as-is
	if filepath.IsAbs(path) {
		return path
	}

	// Resolve relative to the working directory
	resolved := filepath.Join(c.cwd, path)
	c.logger.Debug("Resolved path '%s' -> '%s'", path, resolved)
	return resolved
}

// acp.Client interface implementation

// handleMessageChunk processes message chunks and forwards them to the handler
func (c *ACPClient) handleMessageChunk(ctx context.Context, content *acp.ContentBlock, messageType string) error {
	if content != nil && content.Text != nil {
		textChunk := content.Text.Text
		c.logger.Info("Received %s message chunk: %s", messageType, textChunk)
		if c.handler != nil {
			return c.handler.OnMessageChunk(ctx, textChunk)
		}
	}
	return nil
}

// handleToolMessageChunk processes tool-related message chunks with special formatting
func (c *ACPClient) handleToolMessageChunk(ctx context.Context, content *acp.ContentBlock) error {
	if content == nil || content.Text == nil {
		return nil
	}

	textChunk := content.Text.Text
	c.logger.Info("Received tool message chunk: %s", textChunk)

	// For now, just pass it through as a regular message
	// The formatting from the agent already has emojis and structure
	if c.handler != nil {
		return c.handler.OnMessageChunk(ctx, textChunk)
	}
	return nil
}

func (c *ACPClient) SessionUpdate(ctx context.Context, n acp.SessionNotification) error {
	u := n.Update

	c.logger.Debug("SessionUpdate called")

	if u.UserMessageChunk != nil {
		c.logger.Debug("UserMessageChunk: %+v", u.UserMessageChunk)
		return c.handleMessageChunk(ctx, &u.UserMessageChunk.Content, "user")
	}

	if u.AgentMessageChunk != nil {
		c.logger.Debug("AgentMessageChunk: %+v", u.AgentMessageChunk)
		return c.handleMessageChunk(ctx, &u.AgentMessageChunk.Content, "agent")
	}

	return nil
}

// unsupportedMethodError creates an error for unsupported client methods
func unsupportedMethodError(methodName string) error {
	return fmt.Errorf("%s not supported in this client", methodName)
}

func (c *ACPClient) RequestPermission(ctx context.Context, p acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	if len(p.Options) > 0 {
		return acp.RequestPermissionResponse{
			Outcome: acp.RequestPermissionOutcome{
				Selected: &acp.RequestPermissionOutcomeSelected{
					OptionId: p.Options[0].OptionId,
				},
			},
		}, nil
	}
	return acp.RequestPermissionResponse{}, fmt.Errorf("no options provided")
}

func (c *ACPClient) WriteTextFile(ctx context.Context, p acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	c.logger.Info("WriteTextFile called for path: %s", p.Path)

	// Resolve the path relative to working directory
	resolvedPath := c.resolvePath(p.Path)

	// Create parent directories if they don't exist
	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.logger.Error("Failed to create directory %s: %v", dir, err)
		return acp.WriteTextFileResponse{}, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the file content
	err := os.WriteFile(resolvedPath, []byte(p.Content), 0644)
	if err != nil {
		c.logger.Error("Failed to write file %s: %v", resolvedPath, err)
		return acp.WriteTextFileResponse{}, fmt.Errorf("failed to write file: %w", err)
	}

	c.logger.Debug("Successfully wrote %d bytes to %s", len(p.Content), resolvedPath)

	return acp.WriteTextFileResponse{}, nil
}

func (c *ACPClient) ReadTextFile(ctx context.Context, p acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	c.logger.Info("ReadTextFile called for path: %s", p.Path)

	// Resolve the path relative to working directory
	resolvedPath := c.resolvePath(p.Path)

	// Read the file content
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		c.logger.Error("Failed to read file %s: %v", resolvedPath, err)
		return acp.ReadTextFileResponse{}, fmt.Errorf("failed to read file: %w", err)
	}

	c.logger.Debug("Successfully read %d bytes from %s", len(content), resolvedPath)

	return acp.ReadTextFileResponse{
		Content: string(content),
	}, nil
}

func (c *ACPClient) CreateTerminal(ctx context.Context, p acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, unsupportedMethodError("CreateTerminal")
}

func (c *ACPClient) KillTerminalCommand(ctx context.Context, p acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, unsupportedMethodError("KillTerminalCommand")
}

func (c *ACPClient) ReleaseTerminal(ctx context.Context, p acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, unsupportedMethodError("ReleaseTerminal")
}

func (c *ACPClient) TerminalOutput(ctx context.Context, p acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, unsupportedMethodError("TerminalOutput")
}

func (c *ACPClient) WaitForTerminalExit(ctx context.Context, p acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, unsupportedMethodError("WaitForTerminalExit")
}

// HandleExtensionMethod handles custom extension methods that start with underscore
// This is called by a custom wrapper to handle methods like _fs/grep_search
func (c *ACPClient) HandleExtensionMethod(ctx context.Context, method string, params map[string]interface{}) (interface{}, error) {
	switch method {
	case "_fs/grep_search":
		return c.handleGrepSearch(ctx, params)
	default:
		return nil, fmt.Errorf("extension method not supported: %s", method)
	}
}

// handleGrepSearch handles the _fs/grep_search extension method
func (c *ACPClient) handleGrepSearch(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	c.logger.Info("HandleGrepSearch called with params: %+v", params)

	// Extract parameters
	pattern, _ := params["pattern"].(string)
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	path, _ := params["path"].(string)
	if path == "" {
		path = "."
	}

	caseSensitive, _ := params["caseSensitive"].(bool)
	filePattern, _ := params["filePattern"].(string)

	// Resolve the path relative to working directory
	resolvedPath := c.resolvePath(path)

	c.logger.Debug("Grep search: pattern=%s, path=%s, caseSensitive=%v, filePattern=%s",
		pattern, resolvedPath, caseSensitive, filePattern)

	// Build the list of paths to search
	paths := []string{resolvedPath}

	// Perform the grep search (recursive by default)
	results, err := c.GrepSearch(ctx, pattern, paths, true, caseSensitive)
	if err != nil {
		c.logger.Error("GrepSearch failed: %v", err)
		return nil, err
	}

	// Convert results to the expected format and limit to 20 results
	const maxResults = 20
	matches := make([]map[string]interface{}, 0, len(results))
	truncated := false

	for _, result := range results {
		// Stop if we've reached the limit
		if len(matches) >= maxResults {
			truncated = true
			break
		}

		// Apply file pattern filter if specified
		if filePattern != "" {
			matched, err := filepath.Match(filePattern, filepath.Base(result.Path))
			if err != nil || !matched {
				continue
			}
		}

		// Truncate long lines to avoid huge JSON responses
		line := result.Line
		const maxLineLength = 200
		if len(line) > maxLineLength {
			line = line[:maxLineLength] + "..."
		}

		matches = append(matches, map[string]interface{}{
			"path":       result.Path,
			"lineNumber": result.LineNumber,
			"line":       line,
			"match":      result.Match,
		})
	}

	c.logger.Debug("Grep search found %d matches (truncated: %v)", len(matches), truncated)

	response := map[string]interface{}{
		"matches":   matches,
		"truncated": truncated,
	}

	if truncated {
		response["message"] = fmt.Sprintf("Results limited to %d matches. Refine your search for more specific results.", maxResults)
	}

	return response, nil
}

// GrepSearch searches for a pattern in the specified files or directories
// pattern: regex pattern to search for
// paths: list of file or directory paths to search in
// recursive: if true, search directories recursively
// caseSensitive: if true, perform case-sensitive matching
func (c *ACPClient) GrepSearch(ctx context.Context, pattern string, paths []string, recursive bool, caseSensitive bool) ([]GrepResult, error) {
	c.logger.Info("GrepSearch called with pattern: %s, paths: %v", pattern, paths)

	// Compile the regex pattern
	var re *regexp.Regexp
	var err error
	if caseSensitive {
		re, err = regexp.Compile(pattern)
	} else {
		re, err = regexp.Compile("(?i)" + pattern)
	}
	if err != nil {
		c.logger.Error("Invalid regex pattern %s: %v", pattern, err)
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var results []GrepResult

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			c.logger.Error("Failed to stat path %s: %v", path, err)
			continue
		}

		if info.IsDir() {
			if recursive {
				// Walk the directory recursively
				filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
					if err != nil {
						return nil // Continue on error
					}
					if !d.IsDir() {
						matches, _ := c.grepFile(filePath, re)
						results = append(results, matches...)
					}
					return nil
				})
			} else {
				// Only search files in the immediate directory
				entries, err := os.ReadDir(path)
				if err != nil {
					c.logger.Error("Failed to read directory %s: %v", path, err)
					continue
				}
				for _, entry := range entries {
					if !entry.IsDir() {
						filePath := filepath.Join(path, entry.Name())
						matches, _ := c.grepFile(filePath, re)
						results = append(results, matches...)
					}
				}
			}
		} else {
			// It's a file, search it directly
			matches, _ := c.grepFile(path, re)
			results = append(results, matches...)
		}
	}

	c.logger.Debug("GrepSearch found %d matches", len(results))
	return results, nil
}

// grepFile searches for pattern matches in a single file
func (c *ACPClient) grepFile(filePath string, re *regexp.Regexp) ([]GrepResult, error) {
	// Skip binary files by checking if file is likely text
	if !isTextFile(filePath) {
		return nil, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var results []GrepResult
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		if match := re.FindString(line); match != "" {
			results = append(results, GrepResult{
				Path:       filePath,
				LineNumber: lineNumber,
				Line:       line,
				Match:      match,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return results, err
	}

	return results, nil
}

// isTextFile checks if a file is likely a text file by reading the first 512 bytes
// and checking for null bytes or excessive non-printable characters
func isTextFile(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read first 512 bytes
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return false
	}

	buf = buf[:n]

	// Check for null bytes (strong indicator of binary file)
	for _, b := range buf {
		if b == 0 {
			return false
		}
	}

	// Count non-printable characters (excluding common whitespace)
	nonPrintable := 0
	for _, b := range buf {
		// Allow common whitespace: tab (9), newline (10), carriage return (13)
		if b < 32 && b != 9 && b != 10 && b != 13 {
			nonPrintable++
		} else if b > 126 && b < 128 {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider it binary
	threshold := len(buf) * 30 / 100
	return nonPrintable < threshold
}

// ListDirectories lists files and directories at the specified path
// path: the directory path to list
// recursive: if true, list entries recursively
func (c *ACPClient) ListDirectories(ctx context.Context, path string, recursive bool) ([]DirectoryEntry, error) {
	c.logger.Info("ListDirectories called for path: %s, recursive: %v", path, recursive)

	// Check if the path exists and is a directory
	info, err := os.Stat(path)
	if err != nil {
		c.logger.Error("Failed to stat path %s: %v", path, err)
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", path)
	}

	var entries []DirectoryEntry

	if recursive {
		// Walk the directory recursively
		err = filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
			if err != nil {
				c.logger.Error("Error walking path %s: %v", filePath, err)
				return nil // Continue on error
			}

			// Skip the root directory itself
			if filePath == path {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				c.logger.Error("Failed to get info for %s: %v", filePath, err)
				return nil // Continue on error
			}

			entries = append(entries, DirectoryEntry{
				Path:  filePath,
				Name:  d.Name(),
				IsDir: d.IsDir(),
				Size:  info.Size(),
				Mode:  info.Mode(),
			})

			return nil
		})
	} else {
		// Only list immediate children
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			c.logger.Error("Failed to read directory %s: %v", path, err)
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range dirEntries {
			info, err := entry.Info()
			if err != nil {
				c.logger.Error("Failed to get info for %s: %v", entry.Name(), err)
				continue
			}

			fullPath := filepath.Join(path, entry.Name())
			entries = append(entries, DirectoryEntry{
				Path:  fullPath,
				Name:  entry.Name(),
				IsDir: entry.IsDir(),
				Size:  info.Size(),
				Mode:  info.Mode(),
			})
		}
	}

	c.logger.Debug("ListDirectories found %d entries", len(entries))
	return entries, nil
}
