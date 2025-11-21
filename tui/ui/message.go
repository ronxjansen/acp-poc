package ui

import (
	"github.com/muesli/reflow/wordwrap"
	"github.com/ron/tui_acp/tui/app"
)

// MessageRenderer handles rendering of conversation messages
type MessageRenderer struct {
	width int
	theme *MessageTheme
}

// NewMessageRenderer creates a new message renderer with the default theme
func NewMessageRenderer(width int) MessageRenderer {
	return MessageRenderer{
		width: width,
		theme: DefaultMessageTheme(),
	}
}

// NewMessageRendererWithTheme creates a new message renderer with a custom theme
func NewMessageRendererWithTheme(width int, theme *MessageTheme) MessageRenderer {
	return MessageRenderer{
		width: width,
		theme: theme,
	}
}

// SetWidth updates the width for word wrapping
func (r *MessageRenderer) SetWidth(width int) {
	r.width = width
}

// RenderConversation renders all messages in the conversation
func (r MessageRenderer) RenderConversation(messages []app.Message, currentResponse string) string {
	var output string

	for _, msg := range messages {
		output += r.RenderMessage(msg) + "\n"
	}

	// Render streaming response if present
	if currentResponse != "" {
		output += r.renderAssistantMessage(currentResponse) + "\n"
	}

	return output
}

// RenderMessage renders a single message based on its type
func (r MessageRenderer) RenderMessage(msg app.Message) string {
	style, label := r.theme.GetConfig(msg.Type)
	return r.renderWithStyle(style, label, msg.Content)
}

// renderWithStyle is a helper that renders content with a given style and label
func (r MessageRenderer) renderWithStyle(style interface{ Render(...string) string }, label, content string) string {
	wrapWidth := r.getWrapWidth()
	wrapped := wordwrap.String(content, wrapWidth)
	return style.Render(label) + wrapped + "\n"
}

// renderAssistantMessage renders an assistant message (used for streaming responses)
func (r MessageRenderer) renderAssistantMessage(content string) string {
	style, label := r.theme.GetConfig(app.MessageAssistant)
	return r.renderWithStyle(style, label, content)
}

// getWrapWidth calculates the appropriate width for word wrapping
func (r MessageRenderer) getWrapWidth() int {
	wrapWidth := r.width - 4
	if wrapWidth < 40 {
		wrapWidth = 40
	}
	return wrapWidth
}
