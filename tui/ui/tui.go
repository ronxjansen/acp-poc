package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ron/tui_acp/tui/app"
)

// Message types for tea.Model communication
type (
	acpUpdateMsg struct{ text string }
	acpErrorMsg  struct{ err error }
	connectMsg   struct{ err error }
)

// Model represents the TUI state - a thin coordinator that composes
// state management, input handling, and view rendering.
type Model struct {
	// State
	state ChatState

	// Components
	inputBox InputBox
	view     ViewRenderer
	spinner  HexSpinner

	// External dependencies
	app        *app.App
	updateChan chan string
	errChan    chan error
	address    string
}

// NewModel creates a new TUI model
func NewModel(application *app.App, updateChan chan string, address string) Model {
	return Model{
		state:      NewChatState(),
		inputBox:   NewInputBox("Type a message..."),
		view:       NewViewRenderer(80),
		spinner:    NewHexSpinner(),
		app:        application,
		updateChan: updateChan,
		errChan:    make(chan error, 10),
		address:    address,
	}
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		Connect(m.address, m.updateChan, m.app),
		waitForError(m.errChan),
	)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case connectMsg:
		return m.handleConnect(msg)
	case acpUpdateMsg:
		return m.handleACPUpdate(msg)
	case acpErrorMsg:
		return m.handleACPError(msg)
	case TickMsg:
		return m.handleTick(msg)
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		m.view.SetWidth(msg.Width)
	}
	return m, nil
}

// View renders the TUI
func (m Model) View() string {
	if m.state.Connecting {
		return m.view.RenderConnecting()
	}

	if !m.app.IsConnected() && m.state.Error != nil {
		return m.view.RenderConnectionError(m.state.Error)
	}

	return m.view.RenderMainView(
		m.state,
		m.app.GetCurrentResponse(),
		m.spinner,
		m.inputBox.View(),
	)
}

// handleConnect handles connection result messages
func (m Model) handleConnect(msg connectMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.state.SetConnectionError(msg.err)
		return m, tea.Quit
	}

	m.state.SetConnected()

	// Print welcome header
	header, separator, welcome := m.view.RenderWelcome(m.address)
	return m, tea.Batch(
		tea.Println(header),
		tea.Println(separator),
		tea.Println(welcome),
		tea.Println(""),
		waitForUpdate(m.updateChan),
	)
}

// handleACPUpdate handles update messages from the ACP layer
func (m Model) handleACPUpdate(msg acpUpdateMsg) (tea.Model, tea.Cmd) {
	messages, _ := m.app.GetState()
	var cmds []tea.Cmd

	// Print any new completed messages
	newMessages := m.state.UpdatePrintedCount(messages)
	for _, newMsg := range newMessages {
		rendered := m.view.RenderMessage(newMsg)
		cmds = append(cmds, tea.Println(rendered))
	}

	// OnMessageComplete sends empty string as explicit completion signal
	// This is the most reliable way to detect when the response is done
	if msg.text == "" {
		m.state.SetLoading(false)
	}

	cmds = append(cmds, waitForUpdate(m.updateChan))
	return m, tea.Batch(cmds...)
}

// handleACPError handles error messages from async operations
func (m Model) handleACPError(msg acpErrorMsg) (tea.Model, tea.Cmd) {
	m.state.SetError(msg.err)
	return m, waitForError(m.errChan)
}

// handleTick handles spinner animation tick messages
func (m Model) handleTick(msg TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// handleKeyMsg handles keyboard input messages
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	default:
		return m.handleTextInput(msg)
	}
}

// handleTextInput handles regular text input and submission
func (m Model) handleTextInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	submitted, userMessage := m.inputBox.Update(msg)
	if !submitted {
		return m, nil
	}

	// Add message to conversation
	m.app.AddUserMessage(userMessage)

	// Print new messages
	messages := m.app.GetMessages()
	var cmds []tea.Cmd
	newMessages := m.state.UpdatePrintedCount(messages)
	for _, msg := range newMessages {
		rendered := m.view.RenderMessage(msg)
		cmds = append(cmds, tea.Println(rendered))
	}

	// Start loading
	m.state.SetLoading(true)

	// Send to server asynchronously
	errChan := m.errChan
	go func() {
		if err := m.app.SendPromptToAgent(context.Background(), userMessage); err != nil {
			select {
			case errChan <- err:
			default:
			}
		}
	}()

	cmds = append(cmds, m.spinner.Init())
	return m, tea.Batch(cmds...)
}

// Channel monitoring commands

func waitForUpdate(updateChan chan string) tea.Cmd {
	return func() tea.Msg {
		text, ok := <-updateChan
		if !ok {
			return nil
		}
		return acpUpdateMsg{text: text}
	}
}

func waitForError(errChan chan error) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-errChan
		if !ok {
			return nil
		}
		return acpErrorMsg{err: err}
	}
}

// Connect initiates connection to the server
func Connect(address string, updateChan chan string, application *app.App) tea.Cmd {
	return func() tea.Msg {
		err := application.Connect(context.Background(), address)
		return connectMsg{err: err}
	}
}
