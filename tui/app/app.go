package app

import (
	"context"
	"sync"

	"github.com/ron/tui_acp/tui/client"
	"github.com/ron/tui_acp/tui/logger"
)

// MessageType defines types of messages
type MessageType string

const (
	MessageUser       MessageType = "user"
	MessageAssistant  MessageType = "assistant"
	MessageToolInput  MessageType = "tool_input"
	MessageToolOutput MessageType = "tool_output"
	MessageSystem     MessageType = "system"
	MessageError      MessageType = "error"
	MessageDebug      MessageType = "debug"
	MessageInfo       MessageType = "info"
)

// Message represents a conversation message
type Message struct {
	Type    MessageType
	Content string
	Data    interface{} // Optional structured data
}

// App manages the business logic for the chat application
type App struct {
	mu             sync.RWMutex
	client         *client.ACPClient
	conversation   *ConversationManager
	logger         logger.Logger
	updateCallback func(string)
}

// Config contains configuration for creating an App
type Config struct {
	Logger         logger.Logger
	UpdateCallback func(string) // Called when a message chunk is received
}

// New creates a new App instance
func New(cfg Config) *App {
	if cfg.Logger == nil {
		cfg.Logger = logger.NewNoopLogger()
	}

	return &App{
		logger:         cfg.Logger,
		updateCallback: cfg.UpdateCallback,
		conversation:   NewConversationManager(),
	}
}

// Connect establishes a connection to the ACP server
func (a *App) Connect(ctx context.Context, address string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	acpClient, err := client.NewACPClient(client.Config{
		Address: address,
		Logger:  a.logger,
		Handler: a,
	})
	if err != nil {
		return err
	}

	a.client = acpClient
	a.logger.Info("Connected to ACP server at %s", address)
	return nil
}

// AddUserMessage adds a user message to the conversation without sending it
func (a *App) AddUserMessage(text string) {
	a.conversation.AddUserMessage(text)
}

// SendPromptToAgent sends a prompt to the agent (without adding to messages)
func (a *App) SendPromptToAgent(ctx context.Context, text string) error {
	a.mu.RLock()
	client := a.client
	a.mu.RUnlock()

	if client != nil {
		return client.SendPrompt(ctx, text)
	}

	return nil
}

// SendMessage sends a user message to the agent
func (a *App) SendMessage(ctx context.Context, text string) error {
	a.conversation.AddUserMessage(text)

	a.mu.RLock()
	client := a.client
	a.mu.RUnlock()

	if client != nil {
		return client.SendPrompt(ctx, text)
	}

	return nil
}

// OnMessageChunk implements the MessageHandler interface
func (a *App) OnMessageChunk(ctx context.Context, text string) error {
	a.conversation.AppendToCurrentResponse(text)

	if a.updateCallback != nil {
		a.updateCallback(text)
	}

	return nil
}

// OnMessageComplete implements the MessageHandler interface
// Called when the agent has finished sending a response
func (a *App) OnMessageComplete(ctx context.Context) error {
	a.conversation.FlushCurrentResponse()

	if a.updateCallback != nil {
		a.updateCallback("")
	}

	return nil
}

// GetMessages returns the messages slice (not a copy for efficiency).
// Callers should not modify the returned slice.
func (a *App) GetMessages() []Message {
	return a.conversation.GetMessages()
}

// GetCurrentResponse returns the current incomplete response from the agent
func (a *App) GetCurrentResponse() string {
	return a.conversation.GetCurrentResponse()
}

// GetState returns both messages and current response in a single lock acquisition.
// This is more efficient than calling GetMessages and GetCurrentResponse separately.
// The returned messages slice should not be modified.
func (a *App) GetState() (messages []Message, currentResponse string) {
	return a.conversation.GetState()
}

// AddMessage adds a message of a specific type to the conversation
func (a *App) AddMessage(msgType string, content string, data ...interface{}) {
	msg := Message{
		Type:    MessageType(msgType),
		Content: content,
	}

	if len(data) > 0 {
		msg.Data = data[0]
	}

	a.conversation.AddMessage(msg)
}

// Close closes the ACP client connection
func (a *App) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		return a.client.Close()
	}
	return nil
}

// IsConnected returns whether the app is connected to an ACP server
func (a *App) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.client != nil
}

// SetLogger updates the logger instance
func (a *App) SetLogger(log logger.Logger) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logger = log
}
