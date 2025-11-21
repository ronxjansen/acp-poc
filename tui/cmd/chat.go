package cmd

import (
	"fmt"
	"os"

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
	Run:  runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Local flags for the chat command
	chatCmd.Flags().StringVarP(&address, "address", "a", "localhost:9090", "ACP server address (host:port)")
}

func runChat(cmd *cobra.Command, args []string) {
	// Use address from args if provided, otherwise use flag value
	serverAddress := address
	if len(args) > 0 {
		serverAddress = args[0]
	}

	// Build the application using the builder pattern
	builder := NewApplicationBuilder(serverAddress)
	defer builder.Cleanup()

	// Build components
	builder.BuildLogger()
	builder.BuildApp()
	builder.StartLogConsumer()

	// Create and run the program
	program := builder.BuildProgram()

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
