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
	printedMsgCount int // Track how many messages have been printed to stdout
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
		// Print welcome header on successful connection
		header := headerStyle.Render("Weather Agent TUI")
		separator := separatorStyle.Render("─────────────────────────────────────")
		welcome := helpStyle.Render("Connected to " + m.address)
		return m, tea.Batch(
			tea.Println(header),
			tea.Println(separator),
			tea.Println(welcome),
			tea.Println(""), // blank line
			waitForUpdate(m.updateChan),
		)

	case acpUpdateMsg:
		// Check if there are new messages to print to stdout
		messages := m.app.GetMessages()
		var cmds []tea.Cmd

		// Print any new completed messages to stdout
		for i := m.printedMsgCount; i < len(messages); i++ {
			rendered := m.messageRenderer.RenderMessage(messages[i])
			cmds = append(cmds, tea.Println(rendered))
		}
		m.printedMsgCount = len(messages)

		// If no current response, we're done loading
		if m.app.GetCurrentResponse() == "" {
			m.loading = false
		}

		cmds = append(cmds, waitForUpdate(m.updateChan))
		return m, tea.Batch(cmds...)

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
				// Note: AddUserMessage flushes any pending assistant response first
				m.app.AddUserMessage(userMessage)

				// Print ALL new messages (including any flushed assistant response + user message)
				messages := m.app.GetMessages()
				var cmds []tea.Cmd
				for i := m.printedMsgCount; i < len(messages); i++ {
					rendered := m.messageRenderer.RenderMessage(messages[i])
					cmds = append(cmds, tea.Println(rendered))
				}
				m.printedMsgCount = len(messages)

				// Start loading indicator
				m.loading = true

				// Send to server asynchronously
				go func() {
					if err := m.app.SendPromptToAgent(context.Background(), userMessage); err != nil {
						// Error handling could be improved here
						_ = err
					}
				}()

				// Print messages and start spinner
				cmds = append(cmds, m.spinner.Init())
				return m, tea.Batch(cmds...)
			}
		}

	case tea.WindowSizeMsg:
		m.messageRenderer.SetWidth(msg.Width)
	}

	return m, nil
}

// View renders the TUI - only the input area and streaming response
// Completed messages are printed to stdout via tea.Println for scrollback
func (m Model) View() string {
	if m.connecting {
		return "Connecting to server...\n"
	}

	if !m.app.IsConnected() && m.err != nil {
		return tuiErrorStyle.Render(fmt.Sprintf("Failed to connect: %v\nPress Ctrl+C to exit", m.err))
	}

	help := helpStyle.Render("Enter: send • Ctrl+C: quit")

	// Use InputBox component for rendering input
	inputView := m.inputBox.View()

	var errorView string
	if m.err != nil {
		errorView = tuiErrorStyle.Render(fmt.Sprintf("Error: %v\n", m.err))
	}

	// Show current streaming response (not yet complete)
	var streamingView string
	currentResponse := m.app.GetCurrentResponse()
	if currentResponse != "" {
		streamingView = m.messageRenderer.RenderMessage(app.Message{
			Type:    app.MessageAssistant,
			Content: currentResponse,
		}) + "\n"
	}

	// Show loading spinner when waiting for response
	var spinnerView string
	if m.loading {
		spinnerView = m.spinner.View() + " Processing...\n"
	}

	return streamingView + errorView + spinnerView + inputView + "\n" + help
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
