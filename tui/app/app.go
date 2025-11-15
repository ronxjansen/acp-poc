package app

import (
	"context"
	"strings"
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
	mu              sync.RWMutex
	client          *client.ACPClient
	messages        []Message
	currentResponse *strings.Builder
	logger          logger.Logger
	updateCallback  func(string)
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
		logger:          cfg.Logger,
		updateCallback:  cfg.UpdateCallback,
		messages:        make([]Message, 0),
		currentResponse: &strings.Builder{},
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

// flushCurrentResponse adds any pending response to messages (must hold lock)
func (a *App) flushCurrentResponse() {
	if a.currentResponse.Len() > 0 {
		a.messages = append(a.messages, Message{
			Type:    MessageAssistant,
			Content: a.currentResponse.String(),
		})
		a.currentResponse.Reset()
	}
}

// AddUserMessage adds a user message to the conversation without sending it
func (a *App) AddUserMessage(text string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.flushCurrentResponse()

	a.messages = append(a.messages, Message{
		Type:    MessageUser,
		Content: text,
	})
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
	a.mu.Lock()

	a.flushCurrentResponse()

	a.messages = append(a.messages, Message{
		Type:    MessageUser,
		Content: text,
	})

	client := a.client
	a.mu.Unlock()

	if client != nil {
		return client.SendPrompt(ctx, text)
	}

	return nil
}

// OnMessageChunk implements the MessageHandler interface
func (a *App) OnMessageChunk(ctx context.Context, text string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.currentResponse.WriteString(text)

	if a.updateCallback != nil {
		a.updateCallback(text)
	}

	return nil
}

// GetMessages returns a copy of all messages
func (a *App) GetMessages() []Message {
	a.mu.RLock()
	defer a.mu.RUnlock()

	messages := make([]Message, len(a.messages))
	copy(messages, a.messages)
	return messages
}

// GetCurrentResponse returns the current incomplete response from the agent
func (a *App) GetCurrentResponse() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.currentResponse == nil {
		return ""
	}
	return a.currentResponse.String()
}

// AddMessage adds a message of a specific type to the conversation
func (a *App) AddMessage(msgType string, content string, data ...interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()

	msg := Message{
		Type:    MessageType(msgType),
		Content: content,
	}

	if len(data) > 0 {
		msg.Data = data[0]
	}

	a.messages = append(a.messages, msg)
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
