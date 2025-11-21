package logger

import "time"

// LogMessage represents a log message sent to the TUI
type LogMessage struct {
	Level   string
	Message string
	Time    time.Time
}

// Logger defines an interface for logging debug messages
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Error(format string, args ...interface{})
}
