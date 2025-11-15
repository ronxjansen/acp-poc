package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

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

// Config contains configuration for the logger
type Config struct {
	Debug      bool
	Trace      bool
	LogFile    string
	TUILogChan chan<- LogMessage // Optional channel for TUI output
}

// ZerologAdapter adapts zerolog.Logger to the Logger interface
type ZerologAdapter struct {
	logger zerolog.Logger
}

// NewZerologLogger creates a new zerolog-based logger with multiple transports
func NewZerologLogger(cfg Config) Logger {
	var logLevel zerolog.Level
	if cfg.Trace {
		logLevel = zerolog.TraceLevel
	} else if cfg.Debug {
		logLevel = zerolog.DebugLevel
	} else {
		logLevel = zerolog.InfoLevel
	}

	var writers []io.Writer

	if cfg.LogFile != "" {
		logPath := cfg.LogFile
		if !filepath.IsAbs(logPath) {
			execPath, err := os.Executable()
			if err == nil {
				execDir := filepath.Dir(execPath)
				logPath = filepath.Join(execDir, cfg.LogFile)
			}
		}

		fileLogger := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    10, // megabytes
			MaxBackups: 3,
			MaxAge:     28, // days
		}
		writers = append(writers, fileLogger)
	}

	if cfg.TUILogChan != nil {
		tuiWriter := NewTUIWriter(cfg.TUILogChan)

		consoleWriter := zerolog.ConsoleWriter{
			Out:        tuiWriter,
			TimeFormat: "15:04:05",
			NoColor:    true, // TUI will handle coloring
			PartsOrder: []string{
				zerolog.LevelFieldName,
				zerolog.MessageFieldName,
			},
		}
		writers = append(writers, consoleWriter)
	}

	if len(writers) == 0 {
		writers = append(writers, io.Discard)
	}

	multi := io.MultiWriter(writers...)

	logger := zerolog.New(multi).
		With().
		Timestamp().
		Logger().
		Level(logLevel)

	return &ZerologAdapter{logger: logger}
}

func (z *ZerologAdapter) Debug(format string, args ...interface{}) {
	z.logger.Debug().Msgf(format, args...)
}

func (z *ZerologAdapter) Info(format string, args ...interface{}) {
	z.logger.Info().Msgf(format, args...)
}

func (z *ZerologAdapter) Error(format string, args ...interface{}) {
	z.logger.Error().Msgf(format, args...)
}

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

// NoopLogger is a logger that doesn't output anything
type NoopLogger struct{}

// NewNoopLogger creates a logger that discards all output
func NewNoopLogger() Logger {
	return &NoopLogger{}
}

func (l *NoopLogger) Debug(format string, args ...interface{}) {}
func (l *NoopLogger) Info(format string, args ...interface{})  {}
func (l *NoopLogger) Error(format string, args ...interface{}) {}
