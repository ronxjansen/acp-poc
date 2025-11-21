package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ron/tui_acp/tui/app"
	"github.com/ron/tui_acp/tui/logger"
	"github.com/ron/tui_acp/tui/ui"
)

// ApplicationBuilder handles the construction of the chat application components
type ApplicationBuilder struct {
	serverAddress string
	debug         bool
	trace         bool
	logFile       string

	// Channels
	updateChan chan string
	logChan    chan logger.LogMessage

	// Components
	log         logger.Logger
	application *app.App
}

// NewApplicationBuilder creates a new ApplicationBuilder with configuration
func NewApplicationBuilder(serverAddress string) *ApplicationBuilder {
	return &ApplicationBuilder{
		serverAddress: serverAddress,
		debug:         GetDebug(),
		trace:         GetTrace(),
		logFile:       GetLogFile(),
		updateChan:    make(chan string, 100),
		logChan:       make(chan logger.LogMessage, 100),
	}
}

// BuildLogger creates and returns the logger
func (b *ApplicationBuilder) BuildLogger() logger.Logger {
	var tuiLogChan chan<- logger.LogMessage
	if b.debug || b.trace {
		tuiLogChan = b.logChan
	}

	b.log = logger.NewZerologLogger(logger.Config{
		Debug:      b.debug,
		Trace:      b.trace,
		LogFile:    b.logFile,
		TUILogChan: tuiLogChan,
	})

	return b.log
}

// BuildApp creates and returns the application instance
func (b *ApplicationBuilder) BuildApp() *app.App {
	if b.log == nil {
		b.BuildLogger()
	}

	b.application = app.New(app.Config{
		Logger: b.log,
		UpdateCallback: func(text string) {
			select {
			case b.updateChan <- text:
			default:
				// Channel full, skip update
			}
		},
	})

	return b.application
}

// BuildModel creates and returns the TUI model
func (b *ApplicationBuilder) BuildModel() ui.Model {
	if b.application == nil {
		b.BuildApp()
	}

	return ui.NewModel(b.application, b.updateChan, b.serverAddress)
}

// BuildProgram creates and returns the Bubble Tea program
func (b *ApplicationBuilder) BuildProgram() *tea.Program {
	model := b.BuildModel()
	return tea.NewProgram(model)
}

// StartLogConsumer starts the goroutine that consumes log messages
func (b *ApplicationBuilder) StartLogConsumer() {
	if b.application == nil {
		return
	}

	go func() {
		for logMsg := range b.logChan {
			msg := logMsg.Message
			if len(msg) > 0 {
				b.application.AddMessage("debug", msg)
			}
		}
	}()
}

// Cleanup closes all resources
func (b *ApplicationBuilder) Cleanup() {
	if b.application != nil {
		b.application.Close()
	}
}

// GetApp returns the application instance
func (b *ApplicationBuilder) GetApp() *app.App {
	return b.application
}
