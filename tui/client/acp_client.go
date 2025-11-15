package client

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	acp "github.com/coder/acp-go-sdk"
	"github.com/ron/tui_acp/tui/logger"
)

// MessageHandler defines the interface for handling message chunks
type MessageHandler interface {
	OnMessageChunk(ctx context.Context, text string) error
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

	client.conn = acp.NewClientSideConnection(client, conn, conn)

	client.logger.Debug("Initializing ACP connection...")
	ctx := context.Background()
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
	return acp.WriteTextFileResponse{}, unsupportedMethodError("WriteTextFile")
}

func (c *ACPClient) ReadTextFile(ctx context.Context, p acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, unsupportedMethodError("ReadTextFile")
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
