package client

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	acp "github.com/coder/acp-go-sdk"
	"github.com/ron/tui_acp/tui/logger"
)

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

// ProtocolClient handles the core ACP protocol communication:
// connection setup, initialization, session management, and sending prompts.
type ProtocolClient struct {
	mu sync.Mutex

	sessionID  acp.SessionId
	conn       *acp.ClientSideConnection
	tcpConn    net.Conn
	tcpAddress string
	cwd        string
	logger     logger.Logger
}

// ProtocolConfig contains configuration for creating a ProtocolClient
type ProtocolConfig struct {
	Address string
	Logger  logger.Logger
	// ACPClient is the acp.Client implementation that handles agent requests
	ACPClient acp.Client
	// ExtensionHandler handles custom extension methods (methods starting with _)
	ExtensionHandler ExtensionMethodHandler
}

// NewProtocolClient creates a new protocol client and establishes connection
func NewProtocolClient(cfg ProtocolConfig) (*ProtocolClient, error) {
	if cfg.Logger == nil {
		cfg.Logger = logger.NewNoopLogger()
	}

	client := &ProtocolClient{
		logger:     cfg.Logger,
		tcpAddress: cfg.Address,
	}

	cfg.Logger.Debug("Connecting to %s...", cfg.Address)
	conn, err := net.Dial("tcp", cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", cfg.Address, err)
	}
	cfg.Logger.Debug("TCP connected")

	client.tcpConn = conn

	// Wrap TCP connection with buffered I/O for proper line-based communication
	// Use auto-flushing writer to ensure messages are sent immediately
	baseReader := bufio.NewReader(conn)
	writer := &flushingWriter{bufio.NewWriter(conn)}

	// Wrap reader with middleware to intercept extension method requests
	ctx := context.Background()
	reader := NewJSONRPCMiddleware(ctx, baseReader, writer, cfg.ExtensionHandler)

	client.conn = acp.NewClientSideConnection(cfg.ACPClient, writer, reader)

	cfg.Logger.Debug("Initializing ACP connection...")
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
	cfg.Logger.Debug("ACP initialized")

	// Determine working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	if absCwd, err := filepath.Abs(cwd); err == nil {
		cwd = absCwd
	}
	client.cwd = cwd
	cfg.Logger.Debug("Working directory: %s", cwd)

	// Create new session
	cfg.Logger.Debug("Creating new session...")
	sessionResp, err := client.conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        cwd,
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	client.sessionID = sessionResp.SessionId
	cfg.Logger.Debug("Session created: %s", sessionResp.SessionId)

	return client, nil
}

// SendPrompt sends a prompt to the agent
func (p *ProtocolClient) SendPrompt(ctx context.Context, prompt string) error {
	p.mu.Lock()
	sessionID := p.sessionID
	p.mu.Unlock()

	p.logger.Info("Sending prompt: %s", prompt)
	_, err := p.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	})

	return err
}

// GetCwd returns the working directory
func (p *ProtocolClient) GetCwd() string {
	return p.cwd
}

// Close closes the protocol client and TCP connection
func (p *ProtocolClient) Close() error {
	if p.tcpConn != nil {
		return p.tcpConn.Close()
	}
	return nil
}
