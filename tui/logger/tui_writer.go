package logger

import "time"

// TUIWriter is a custom io.Writer that sends log messages to a channel (non-blocking)
type TUIWriter struct {
	logChan chan<- LogMessage
}

// NewTUIWriter creates a new TUI writer
func NewTUIWriter(logChan chan<- LogMessage) *TUIWriter {
	return &TUIWriter{logChan: logChan}
}

// Write implements io.Writer interface
func (w *TUIWriter) Write(p []byte) (n int, err error) {
	select {
	case w.logChan <- LogMessage{
		Message: string(p),
		Time:    time.Now(),
	}:
	default:
		// Channel full, drop message to avoid blocking
	}
	return len(p), nil
}
