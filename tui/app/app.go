package app

import (
	"context"
	"encoding/json"
	"fmt"
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

// OnToolInput implements the ToolMessageHandler interface
// Called when a tool is about to be executed
func (a *App) OnToolInput(ctx context.Context, method string, params map[string]interface{}) error {
	// Flush any pending response before showing tool call
	a.conversation.FlushCurrentResponse()

	// Format tool input message
	content := formatToolInput(method, params)
	a.conversation.AddMessage(Message{
		Type:    MessageToolInput,
		Content: content,
		Data:    params,
	})

	if a.updateCallback != nil {
		a.updateCallback(content)
	}

	return nil
}

// OnToolOutput implements the ToolMessageHandler interface
// Called when a tool has finished executing
func (a *App) OnToolOutput(ctx context.Context, method string, result interface{}, err error) error {
	// Format tool output message
	content := formatToolOutput(method, result, err)
	a.conversation.AddMessage(Message{
		Type:    MessageToolOutput,
		Content: content,
		Data:    result,
	})

	if a.updateCallback != nil {
		a.updateCallback(content)
	}

	return nil
}

// formatToolInput formats tool input for display
func formatToolInput(method string, params map[string]interface{}) string {
	// Create a concise summary based on tool type
	switch method {
	case "_fs/grep_search":
		pattern, _ := params["pattern"].(string)
		path, _ := params["path"].(string)
		if path == "" {
			path = "."
		}
		return fmt.Sprintf("%s: pattern=%q path=%q", method, pattern, path)
	case "_fs/list_dirs":
		path, _ := params["path"].(string)
		if path == "" {
			path = "."
		}
		recursive, _ := params["recursive"].(bool)
		return fmt.Sprintf("%s: path=%q recursive=%v", method, path, recursive)
	default:
		// Fallback to JSON
		paramsJSON, _ := json.Marshal(params)
		return fmt.Sprintf("%s: %s", method, string(paramsJSON))
	}
}

// formatToolOutput formats tool output for display
func formatToolOutput(method string, result interface{}, err error) string {
	if err != nil {
		return fmt.Sprintf("%s error: %v", method, err)
	}

	// Create a concise summary based on tool type
	switch method {
	case "_fs/grep_search":
		if res, ok := result.(map[string]interface{}); ok {
			matches, _ := res["matches"].([]map[string]interface{})
			truncated, _ := res["truncated"].(bool)
			if truncated {
				return fmt.Sprintf("%s: %d matches (truncated)", method, len(matches))
			}
			return fmt.Sprintf("%s: %d matches", method, len(matches))
		}
	case "_fs/list_dirs":
		if res, ok := result.(map[string]interface{}); ok {
			count, _ := res["count"].(int)
			truncated, _ := res["truncated"].(bool)
			if truncated {
				return fmt.Sprintf("%s: %d entries (truncated)", method, count)
			}
			return fmt.Sprintf("%s: %d entries", method, count)
		}
	}

	// Fallback to JSON (truncated if too long)
	resultJSON, _ := json.Marshal(result)
	summary := string(resultJSON)
	if len(summary) > 100 {
		summary = summary[:100] + "..."
	}
	return fmt.Sprintf("%s: %s", method, summary)
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
