package client

import (
	"context"
	"io/fs"

	acp "github.com/coder/acp-go-sdk"
	"github.com/ron/tui_acp/tui/logger"
)

// MessageHandler defines the interface for handling message chunks
type MessageHandler interface {
	OnMessageChunk(ctx context.Context, text string) error
	OnMessageComplete(ctx context.Context) error
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

// ACPClient is a facade that composes protocol, capability, and extension components
// to provide a unified interface for ACP communication.
type ACPClient struct {
	protocol   *ProtocolClient
	capability *CapabilityHandler
	extension  *ExtensionRouter
	fs         *FileSystemAdapter
	handler    MessageHandler
	logger     logger.Logger
}

// NewACPClient creates a new ACP client and connects to the specified TCP address
func NewACPClient(cfg Config) (*ACPClient, error) {
	if cfg.Logger == nil {
		cfg.Logger = logger.NewNoopLogger()
	}

	// Create the client shell first (needed for acp.Client interface)
	client := &ACPClient{
		handler: cfg.Handler,
		logger:  cfg.Logger,
	}

	// Create filesystem adapter (will be initialized with cwd after protocol connects)
	// For now use "." as placeholder - will be updated after connection
	client.fs = NewFileSystemAdapter(".", cfg.Logger)

	// Create capability handler
	client.capability = NewCapabilityHandler(client.fs, cfg.Handler, cfg.Logger)

	// Create extension router
	client.extension = NewExtensionRouter(client.fs, cfg.Logger)

	// Create protocol client (this establishes the connection)
	protocol, err := NewProtocolClient(ProtocolConfig{
		Address:          cfg.Address,
		Logger:           cfg.Logger,
		ACPClient:        client, // ACPClient implements acp.Client via delegation
		ExtensionHandler: client.extension,
	})
	if err != nil {
		return nil, err
	}
	client.protocol = protocol

	// Update filesystem adapter with actual working directory
	client.fs = NewFileSystemAdapter(protocol.GetCwd(), cfg.Logger)
	client.capability = NewCapabilityHandler(client.fs, cfg.Handler, cfg.Logger)
	client.extension = NewExtensionRouter(client.fs, cfg.Logger)

	return client, nil
}

// SendPrompt sends a prompt to the agent and streams the response
func (c *ACPClient) SendPrompt(ctx context.Context, prompt string) error {
	err := c.protocol.SendPrompt(ctx, prompt)

	// Signal that the message is complete
	if c.handler != nil {
		c.handler.OnMessageComplete(ctx)
	}

	return err
}

// Close closes the ACP client and TCP connection
func (c *ACPClient) Close() error {
	if c.protocol != nil {
		return c.protocol.Close()
	}
	return nil
}

// acp.Client interface implementation - delegates to CapabilityHandler

// SessionUpdate handles session update notifications from the agent
func (c *ACPClient) SessionUpdate(ctx context.Context, n acp.SessionNotification) error {
	return c.capability.SessionUpdate(ctx, n)
}

// RequestPermission handles permission requests from the agent
func (c *ACPClient) RequestPermission(ctx context.Context, p acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return c.capability.RequestPermission(ctx, p)
}

// WriteTextFile handles file write requests from the agent
func (c *ACPClient) WriteTextFile(ctx context.Context, p acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return c.capability.WriteTextFile(ctx, p)
}

// ReadTextFile handles file read requests from the agent
func (c *ACPClient) ReadTextFile(ctx context.Context, p acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return c.capability.ReadTextFile(ctx, p)
}

// CreateTerminal is not supported in this client
func (c *ACPClient) CreateTerminal(ctx context.Context, p acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return c.capability.CreateTerminal(ctx, p)
}

// KillTerminalCommand is not supported in this client
func (c *ACPClient) KillTerminalCommand(ctx context.Context, p acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return c.capability.KillTerminalCommand(ctx, p)
}

// ReleaseTerminal is not supported in this client
func (c *ACPClient) ReleaseTerminal(ctx context.Context, p acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return c.capability.ReleaseTerminal(ctx, p)
}

// TerminalOutput is not supported in this client
func (c *ACPClient) TerminalOutput(ctx context.Context, p acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return c.capability.TerminalOutput(ctx, p)
}

// WaitForTerminalExit is not supported in this client
func (c *ACPClient) WaitForTerminalExit(ctx context.Context, p acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return c.capability.WaitForTerminalExit(ctx, p)
}

// Filesystem delegation methods for external use

// GrepSearch delegates to the FileSystemAdapter
func (c *ACPClient) GrepSearch(ctx context.Context, pattern string, paths []string, recursive bool, caseSensitive bool) ([]GrepResult, error) {
	return c.fs.GrepSearch(ctx, pattern, paths, recursive, caseSensitive)
}

// ListDirectories delegates to the FileSystemAdapter
func (c *ACPClient) ListDirectories(ctx context.Context, path string, recursive bool) ([]DirectoryEntry, error) {
	return c.fs.ListDirectories(ctx, path, recursive)
}
