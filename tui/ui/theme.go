package ui

import (
	"github.com/charmbracelet/lipgloss"
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

// MessageTheme defines the visual styling for different message types
type MessageTheme struct {
	configs map[app.MessageType]messageConfig
}

// messageConfig defines the style and label for a message type
type messageConfig struct {
	style lipgloss.Style
	label string
}

// DefaultMessageTheme creates the default message theme
func DefaultMessageTheme() *MessageTheme {
	return &MessageTheme{
		configs: map[app.MessageType]messageConfig{
			app.MessageUser:       {style: createMessageStyle(ColorUser, true, false), label: "You: "},
			app.MessageAssistant:  {style: createMessageStyle(ColorAssistant, false, false), label: "Agent: "},
			app.MessageToolInput:  {style: createMessageStyle(ColorToolInput, false, false), label: "Tool Input: "},
			app.MessageToolOutput: {style: createMessageStyle(ColorToolOutput, false, false), label: "Tool Output: "},
			app.MessageSystem:     {style: createMessageStyle(ColorSystem, false, true), label: "System: "},
			app.MessageError:      {style: createMessageStyle(ColorError, true, false), label: "Error: "},
			app.MessageDebug:      {style: createMessageStyle(ColorDebug, false, true), label: "Debug: "},
			app.MessageInfo:       {style: createMessageStyle(ColorInfo, false, false), label: "Info: "},
		},
	}
}

// GetConfig returns the message config for a given message type
func (t *MessageTheme) GetConfig(msgType app.MessageType) (lipgloss.Style, string) {
	cfg, ok := t.configs[msgType]
	if !ok {
		// Default to assistant style for unknown types
		cfg = t.configs[app.MessageAssistant]
	}
	return cfg.style, cfg.label
}

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
