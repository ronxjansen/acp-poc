package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	debug   bool
	trace   bool
	logFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tui_acp",
	Short: "A TUI client for ACP (Agent Communication Protocol)",
	Long: `A terminal user interface for communicating with ACP agents.
This application provides an interactive chat interface to communicate
with agents using the Agent Communication Protocol.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVarP(&trace, "trace", "t", false, "Enable trace logging (includes debug)")
	rootCmd.PersistentFlags().StringVarP(&logFile, "log-file", "l", "tui.log", "Path to log file")
}

// GetDebug returns the debug flag value
func GetDebug() bool {
	return debug
}

// GetTrace returns the trace flag value
func GetTrace() bool {
	return trace
}

// GetLogFile returns the log file path
func GetLogFile() string {
	return logFile
}
