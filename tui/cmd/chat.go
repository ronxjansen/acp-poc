package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ron/tui_acp/tui/app"
	"github.com/ron/tui_acp/tui/logger"
	"github.com/ron/tui_acp/tui/ui"
	"github.com/spf13/cobra"
)

var (
	address string
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat [address]",
	Short: "Start the chat interface with an ACP agent",
	Long: `Start an interactive chat session with an ACP agent.
The address should be in the format host:port (e.g., localhost:9090).
If no address is provided, it defaults to localhost:9090.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Use address from args if provided, otherwise use flag value
		serverAddress := address
		if len(args) > 0 {
			serverAddress = args[0]
		}

		// Create channels for updates and logs
		updateChan := make(chan string, 100)
		logChan := make(chan logger.LogMessage, 100)

		// Get global flags
		debugEnabled := GetDebug()
		traceEnabled := GetTrace()
		logFilePath := GetLogFile()

		// Only send logs to TUI if debug or trace mode is enabled
		var tuiLogChan chan<- logger.LogMessage
		if debugEnabled || traceEnabled {
			tuiLogChan = logChan
		}

		// Create zerolog logger with file and TUI output
		log := logger.NewZerologLogger(logger.Config{
			Debug:      debugEnabled,
			Trace:      traceEnabled,
			LogFile:    logFilePath,
			TUILogChan: tuiLogChan,
		})

		// Create the application
		application := app.New(app.Config{
			Logger: log,
			UpdateCallback: func(text string) {
				select {
				case updateChan <- text:
				default:
					// Channel full, skip update
				}
			},
		})

		// Start goroutine to consume log messages and add them to the app
		go func() {
			for logMsg := range logChan {
				msg := logMsg.Message
				if len(msg) > 0 {
					application.AddMessage("debug", msg)
				}
			}
		}()

		// Create the TUI model
		model := ui.NewModel(application, updateChan, serverAddress)

		// Start the Bubble Tea program
		// Using inline mode (no AltScreen) so messages print to stdout with scrollback
		p := tea.NewProgram(model)

		_, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Clean up
		application.Close()
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Local flags for the chat command
	chatCmd.Flags().StringVarP(&address, "address", "a", "localhost:9090", "ACP server address (host:port)")
}
