package ui

import "github.com/ron/tui_acp/tui/app"

// ChatState holds the pure state of the chat UI, separate from rendering and input handling.
// This makes state changes explicit and testable.
type ChatState struct {
	// Connection state
	Connecting bool
	Connected  bool
	Error      error

	// Message tracking
	PrintedMsgCount int

	// Loading state
	Loading bool
}

// NewChatState creates a new chat state in connecting mode
func NewChatState() ChatState {
	return ChatState{
		Connecting:      true,
		Connected:       false,
		PrintedMsgCount: 0,
		Loading:         false,
	}
}

// SetConnected updates state after successful connection
func (s *ChatState) SetConnected() {
	s.Connecting = false
	s.Connected = true
}

// SetConnectionError updates state after connection failure
func (s *ChatState) SetConnectionError(err error) {
	s.Connecting = false
	s.Connected = false
	s.Error = err
}

// SetError sets an error state
func (s *ChatState) SetError(err error) {
	s.Error = err
	s.Loading = false
}

// ClearError clears the error state
func (s *ChatState) ClearError() {
	s.Error = nil
}

// SetLoading sets the loading state
func (s *ChatState) SetLoading(loading bool) {
	s.Loading = loading
}

// UpdatePrintedCount updates the count of printed messages and returns the new messages to print
func (s *ChatState) UpdatePrintedCount(messages []app.Message) []app.Message {
	if s.PrintedMsgCount >= len(messages) {
		return nil
	}
	newMessages := messages[s.PrintedMsgCount:]
	s.PrintedMsgCount = len(messages)
	return newMessages
}
