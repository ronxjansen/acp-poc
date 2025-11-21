package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

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
