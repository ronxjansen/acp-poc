package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ron/tui_acp/tui/app"
)

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAssistant)).
			Bold(true)

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorGray))

	tuiErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorError)).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorGray))
)

// Model represents the TUI state
type Model struct {
	inputBox        InputBox
	messageRenderer MessageRenderer
	app             *app.App
	updateChan      chan string
	address         string
	err             error
	connecting      bool
	loading         bool
	spinner         HexSpinner
}

type acpUpdateMsg struct {
	text string
}

type acpErrorMsg struct {
	err error
}

type connectMsg struct {
	err error
}

// NewModel creates a new TUI model
func NewModel(application *app.App, updateChan chan string, address string) Model {
	return Model{
		inputBox:        NewInputBox("Type a message..."),
		messageRenderer: NewMessageRenderer(80), // Default width
		app:             application,
		updateChan:      updateChan,
		address:         address,
		connecting:      true,
		loading:         false,
		spinner:         NewHexSpinner(),
	}
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return Connect(m.address, m.updateChan, m.app)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case connectMsg:
		m.connecting = false
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		return m, waitForUpdate(m.updateChan)

	case acpUpdateMsg:
		// Response received - stop loading
		m.loading = false
		return m, waitForUpdate(m.updateChan)

	case acpErrorMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case TickMsg:
		// Update spinner animation
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		default:
			// Delegate input handling to InputBox component
			submitted, userMessage := m.inputBox.Update(msg)
			if submitted {
				// Add message to conversation immediately (synchronous)
				m.app.AddUserMessage(userMessage)

				// Start loading indicator
				m.loading = true

				// Send to server asynchronously
				go func() {
					if err := m.app.SendPromptToAgent(context.Background(), userMessage); err != nil {
						// Error handling could be improved here
						_ = err
					}
				}()

				// Return to trigger re-render with the user message now visible and start spinner
				return m, m.spinner.Init()
			}
		}

	case tea.WindowSizeMsg:
		m.messageRenderer.SetWidth(msg.Width)
	}

	return m, nil
}

// View renders the TUI
func (m Model) View() string {
	header := headerStyle.Render("Weather Agent TUI") + "\n"
	separator := separatorStyle.Render("─────────────────────────────────────") + "\n\n"

	if m.connecting {
		return header + separator + "Connecting to server...\n"
	}

	if !m.app.IsConnected() && m.err != nil {
		return header + separator + tuiErrorStyle.Render(fmt.Sprintf("Failed to connect: %v\nPress Ctrl+C to exit", m.err))
	}

	help := helpStyle.Render("Enter: send • Ctrl+C: quit")

	// Use InputBox component for rendering input
	inputView := m.inputBox.View()

	var errorView string
	if m.err != nil {
		errorView = tuiErrorStyle.Render(fmt.Sprintf("\nError: %v\n", m.err))
	}

	// Show loading spinner just above input when waiting for response
	var spinnerView string
	if m.loading {
		spinnerView = m.spinner.View() + " Processing...\n"
	}

	// Use MessageRenderer component for rendering conversation
	messages := m.app.GetMessages()
	currentResponse := m.app.GetCurrentResponse()
	conversationView := m.messageRenderer.RenderConversation(messages, currentResponse)

	return header + separator + conversationView + errorView + spinnerView + inputView + "\n" + help
}

// waitForUpdate waits for updates from the app layer
func waitForUpdate(updateChan chan string) tea.Cmd {
	return func() tea.Msg {
		text, ok := <-updateChan
		if !ok {
			return nil
		}
		return acpUpdateMsg{text: text}
	}
}

// Connect initiates connection to the server
func Connect(address string, updateChan chan string, application *app.App) tea.Cmd {
	return func() tea.Msg {
		err := application.Connect(context.Background(), address)
		return connectMsg{err: err}
	}
}
