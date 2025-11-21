package client

import (
	"context"
	"fmt"

	acp "github.com/coder/acp-go-sdk"
	"github.com/ron/tui_acp/tui/logger"
)

// CapabilityHandler implements the acp.Client interface methods for handling
// agent requests (file operations, permissions, terminal stubs).
type CapabilityHandler struct {
	fs      *FileSystemAdapter
	handler MessageHandler
	logger  logger.Logger
}

// NewCapabilityHandler creates a new capability handler
func NewCapabilityHandler(fs *FileSystemAdapter, handler MessageHandler, log logger.Logger) *CapabilityHandler {
	if log == nil {
		log = logger.NewNoopLogger()
	}
	return &CapabilityHandler{
		fs:      fs,
		handler: handler,
		logger:  log,
	}
}

// SetMessageHandler updates the message handler
func (c *CapabilityHandler) SetMessageHandler(handler MessageHandler) {
	c.handler = handler
}

// SessionUpdate handles session update notifications from the agent
func (c *CapabilityHandler) SessionUpdate(ctx context.Context, n acp.SessionNotification) error {
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

// handleMessageChunk processes message chunks and forwards them to the handler
func (c *CapabilityHandler) handleMessageChunk(ctx context.Context, content *acp.ContentBlock, messageType string) error {
	if content == nil || content.Text == nil {
		return nil
	}

	textChunk := content.Text.Text
	c.logger.Info("Received %s message chunk: %s", messageType, textChunk)

	if c.handler != nil {
		return c.handler.OnMessageChunk(ctx, textChunk)
	}
	return nil
}

// RequestPermission handles permission requests from the agent
func (c *CapabilityHandler) RequestPermission(ctx context.Context, p acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
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

// WriteTextFile handles file write requests from the agent
func (c *CapabilityHandler) WriteTextFile(ctx context.Context, p acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	c.logger.Info("WriteTextFile called for path: %s", p.Path)

	if err := c.fs.WriteTextFile(p.Path, p.Content); err != nil {
		return acp.WriteTextFileResponse{}, err
	}

	return acp.WriteTextFileResponse{}, nil
}

// ReadTextFile handles file read requests from the agent
func (c *CapabilityHandler) ReadTextFile(ctx context.Context, p acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	c.logger.Info("ReadTextFile called for path: %s", p.Path)

	content, err := c.fs.ReadTextFile(p.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}

	return acp.ReadTextFileResponse{
		Content: content,
	}, nil
}

// unsupportedMethodError creates an error for unsupported client methods
func unsupportedMethodError(methodName string) error {
	return fmt.Errorf("%s not supported in this client", methodName)
}

// Terminal stubs - these methods return errors as terminal operations are not supported

// CreateTerminal is not supported in this client
func (c *CapabilityHandler) CreateTerminal(ctx context.Context, p acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, unsupportedMethodError("CreateTerminal")
}

// KillTerminalCommand is not supported in this client
func (c *CapabilityHandler) KillTerminalCommand(ctx context.Context, p acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, unsupportedMethodError("KillTerminalCommand")
}

// ReleaseTerminal is not supported in this client
func (c *CapabilityHandler) ReleaseTerminal(ctx context.Context, p acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, unsupportedMethodError("ReleaseTerminal")
}

// TerminalOutput is not supported in this client
func (c *CapabilityHandler) TerminalOutput(ctx context.Context, p acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, unsupportedMethodError("TerminalOutput")
}

// WaitForTerminalExit is not supported in this client
func (c *CapabilityHandler) WaitForTerminalExit(ctx context.Context, p acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, unsupportedMethodError("WaitForTerminalExit")
}
