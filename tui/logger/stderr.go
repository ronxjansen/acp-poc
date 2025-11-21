package logger

import (
	"fmt"
	"os"
)

// StderrLogger writes log messages to stderr
type StderrLogger struct{}

// NewStderrLogger creates a new logger that writes to stderr
func NewStderrLogger() Logger {
	return &StderrLogger{}
}

func (l *StderrLogger) Debug(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
}

func (l *StderrLogger) Info(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[INFO] "+format+"\n", args...)
}

func (l *StderrLogger) Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", args...)
}
