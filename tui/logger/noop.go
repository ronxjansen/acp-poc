package logger

// NoopLogger is a logger that doesn't output anything
type NoopLogger struct{}

// NewNoopLogger creates a logger that discards all output
func NewNoopLogger() Logger {
	return &NoopLogger{}
}

func (l *NoopLogger) Debug(format string, args ...interface{}) {}
func (l *NoopLogger) Info(format string, args ...interface{})  {}
func (l *NoopLogger) Error(format string, args ...interface{}) {}
