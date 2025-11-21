package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/ron/tui_acp/tui/app"
)

// TUIStyles holds all the styles used by the TUI view
type TUIStyles struct {
	Header    lipgloss.Style
	Separator lipgloss.Style
	Error     lipgloss.Style
	Help      lipgloss.Style
}

// DefaultTUIStyles returns the default TUI styles
func DefaultTUIStyles() TUIStyles {
	return TUIStyles{
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAssistant)).
			Bold(true),
		Separator: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorGray)),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorError)).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorGray)),
	}
}

// ViewRenderer handles all view composition for the TUI.
// It takes state and components and produces the final view string.
type ViewRenderer struct {
	styles          TUIStyles
	messageRenderer MessageRenderer
}

// NewViewRenderer creates a new view renderer
func NewViewRenderer(width int) ViewRenderer {
	return ViewRenderer{
		styles:          DefaultTUIStyles(),
		messageRenderer: NewMessageRenderer(width),
	}
}

// SetWidth updates the width for the message renderer
func (v *ViewRenderer) SetWidth(width int) {
	v.messageRenderer.SetWidth(width)
}

// RenderConnecting renders the connecting state view
func (v ViewRenderer) RenderConnecting() string {
	return "Connecting to server...\n"
}

// RenderConnectionError renders the connection error view
func (v ViewRenderer) RenderConnectionError(err error) string {
	return v.styles.Error.Render(fmt.Sprintf("Failed to connect: %v\nPress Ctrl+C to exit", err))
}

// RenderWelcome returns the welcome header components for printing
func (v ViewRenderer) RenderWelcome(address string) (header, separator, welcome string) {
	header = v.styles.Header.Render("Weather Agent TUI")
	separator = v.styles.Separator.Render("─────────────────────────────────────")
	welcome = v.styles.Help.Render("Connected to " + address)
	return
}

// RenderMessage renders a single message
func (v ViewRenderer) RenderMessage(msg app.Message) string {
	return v.messageRenderer.RenderMessage(msg)
}

// RenderStreamingResponse renders the current streaming response
func (v ViewRenderer) RenderStreamingResponse(response string) string {
	if response == "" {
		return ""
	}
	return v.messageRenderer.RenderMessage(app.Message{
		Type:    app.MessageAssistant,
		Content: response,
	}) + "\n"
}

// RenderError renders an error message
func (v ViewRenderer) RenderError(err error) string {
	if err == nil {
		return ""
	}
	return v.styles.Error.Render(fmt.Sprintf("Error: %v\n", err))
}

// RenderSpinner renders the loading spinner
func (v ViewRenderer) RenderSpinner(spinner HexSpinner) string {
	return spinner.View() + " Processing...\n"
}

// RenderHelp renders the help text
func (v ViewRenderer) RenderHelp() string {
	return v.styles.Help.Render("Enter: send • Ctrl+C: quit")
}

// RenderMainView composes the main chat view from all components
func (v ViewRenderer) RenderMainView(
	state ChatState,
	currentResponse string,
	spinner HexSpinner,
	inputView string,
) string {
	streamingView := v.RenderStreamingResponse(currentResponse)

	var errorView string
	if state.Error != nil {
		errorView = v.RenderError(state.Error)
	}

	var spinnerView string
	if state.Loading {
		spinnerView = v.RenderSpinner(spinner)
	}

	help := v.RenderHelp()

	return streamingView + errorView + spinnerView + inputView + "\n" + help
}
