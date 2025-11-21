package app

import (
	"strings"
	"sync"
)

// ConversationManager handles message storage and state for the conversation
type ConversationManager struct {
	mu              sync.RWMutex
	messages        []Message
	currentResponse *strings.Builder
}

// NewConversationManager creates a new ConversationManager
func NewConversationManager() *ConversationManager {
	return &ConversationManager{
		messages:        make([]Message, 0),
		currentResponse: &strings.Builder{},
	}
}

// AddMessage adds a message to the conversation
func (c *ConversationManager) AddMessage(msg Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msg)
}

// AddUserMessage adds a user message, flushing any pending response first
func (c *ConversationManager) AddUserMessage(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.flushCurrentResponse()

	c.messages = append(c.messages, Message{
		Type:    MessageUser,
		Content: text,
	})
}

// AppendToCurrentResponse appends text to the current streaming response
func (c *ConversationManager) AppendToCurrentResponse(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentResponse.WriteString(text)
}

// FlushCurrentResponse flushes the current response to messages
func (c *ConversationManager) FlushCurrentResponse() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flushCurrentResponse()
}

// flushCurrentResponse adds any pending response to messages (must hold lock)
func (c *ConversationManager) flushCurrentResponse() {
	if c.currentResponse.Len() > 0 {
		c.messages = append(c.messages, Message{
			Type:    MessageAssistant,
			Content: c.currentResponse.String(),
		})
		c.currentResponse.Reset()
	}
}

// GetMessages returns the messages slice (not a copy for efficiency).
// Callers should not modify the returned slice.
func (c *ConversationManager) GetMessages() []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.messages
}

// GetCurrentResponse returns the current incomplete response
func (c *ConversationManager) GetCurrentResponse() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.currentResponse == nil {
		return ""
	}
	return c.currentResponse.String()
}

// GetState returns both messages and current response in a single lock acquisition.
// This is more efficient than calling GetMessages and GetCurrentResponse separately.
func (c *ConversationManager) GetState() (messages []Message, currentResponse string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.currentResponse != nil {
		currentResponse = c.currentResponse.String()
	}
	return c.messages, currentResponse
}
