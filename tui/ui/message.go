package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/ron/tui_acp/tui/app"
)

// Color constants for consistent theming
const (
	ColorUser        = "81"
	ColorAssistant   = "86"
	ColorToolInput   = "141"
	ColorToolOutput  = "99"
	ColorSystem      = "208"
	ColorError       = "196"
	ColorDebug       = "240"
	ColorInfo        = "39"
	ColorCaret       = "62"
	ColorPlaceholder = "240"
	ColorGray        = "240"
)

// createMessageStyle creates a lipgloss style for message rendering
func createMessageStyle(color string, bold, italic bool) lipgloss.Style {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		MarginLeft(2)

	if bold {
		style = style.Bold(true)
	}
	if italic {
		style = style.Italic(true)
	}

	return style
}

var (
	userStyle       = createMessageStyle(ColorUser, true, false)
	assistantStyle  = createMessageStyle(ColorAssistant, false, false)
	toolInputStyle  = createMessageStyle(ColorToolInput, false, false)
	toolOutputStyle = createMessageStyle(ColorToolOutput, false, false)
	systemStyle     = createMessageStyle(ColorSystem, false, true)
	errorStyle      = createMessageStyle(ColorError, true, false)
	debugStyle      = createMessageStyle(ColorDebug, false, true)
	infoStyle       = createMessageStyle(ColorInfo, false, false)
)

// messageConfig defines the style and label for a message type
type messageConfig struct {
	style lipgloss.Style
	label string
}

// messageConfigs maps message types to their rendering configuration
var messageConfigs = map[app.MessageType]messageConfig{
	app.MessageUser:       {style: userStyle, label: "You: "},
	app.MessageAssistant:  {style: assistantStyle, label: "Agent: "},
	app.MessageToolInput:  {style: toolInputStyle, label: "Tool Input: "},
	app.MessageToolOutput: {style: toolOutputStyle, label: "Tool Output: "},
	app.MessageSystem:     {style: systemStyle, label: "System: "},
	app.MessageError:      {style: errorStyle, label: "Error: "},
	app.MessageDebug:      {style: debugStyle, label: "Debug: "},
	app.MessageInfo:       {style: infoStyle, label: "Info: "},
}

// MessageRenderer handles rendering of conversation messages
type MessageRenderer struct {
	width int
}

// NewMessageRenderer creates a new message renderer
func NewMessageRenderer(width int) MessageRenderer {
	return MessageRenderer{
		width: width,
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
	// Look up the configuration for this message type
	cfg, ok := messageConfigs[msg.Type]
	if !ok {
		// Default to assistant style for unknown types
		cfg = messageConfigs[app.MessageAssistant]
	}

	return r.renderWithStyle(cfg.style, cfg.label, msg.Content)
}

// renderWithStyle is a helper that renders content with a given style and label
func (r MessageRenderer) renderWithStyle(style lipgloss.Style, label, content string) string {
	wrapWidth := r.getWrapWidth()
	wrapped := wordwrap.String(content, wrapWidth)
	return style.Render(label) + wrapped + "\n"
}

// renderAssistantMessage renders an assistant message (used for streaming responses)
func (r MessageRenderer) renderAssistantMessage(content string) string {
	cfg := messageConfigs[app.MessageAssistant]
	return r.renderWithStyle(cfg.style, cfg.label, content)
}

// getWrapWidth calculates the appropriate width for word wrapping
func (r MessageRenderer) getWrapWidth() int {
	wrapWidth := r.width - 4
	if wrapWidth < 40 {
		wrapWidth = 40
	}
	return wrapWidth
}
