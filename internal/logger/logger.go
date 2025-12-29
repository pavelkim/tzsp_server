package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Logger handles application logging
type Logger struct {
	fileLogger     *logrus.Logger
	consoleLogger  *logrus.Logger
	fileEnabled    bool
	consoleEnabled bool
}

// FileConfig contains file logger configuration
type FileConfig struct {
	Enabled bool
	Level   string
	Format  string
	Path    string
}

// ConsoleConfig contains console logger configuration
type ConsoleConfig struct {
	Enabled bool
	Level   string
	Format  string
}

// Config contains logger configuration
type Config struct {
	File    FileConfig
	Console ConsoleConfig
}

// NewLogger creates a new application logger with multiple outputs
func NewLogger(cfg *Config) (*Logger, error) {
	l := &Logger{}

	// Setup file logger if enabled
	if cfg.File.Enabled && cfg.File.Path != "" {
		fileLog := logrus.New()

		// Set file log level
		fileLvl := cfg.File.Level
		if fileLvl == "" {
			fileLvl = "info"
		}
		lvl, err := logrus.ParseLevel(fileLvl)
		if err != nil {
			lvl = logrus.InfoLevel
		}
		fileLog.SetLevel(lvl)

		// Set file format
		if cfg.File.Format == "json" {
			fileLog.SetFormatter(&logrus.JSONFormatter{
				TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			})
		} else {
			fileLog.SetFormatter(&logrus.TextFormatter{
				FullTimestamp:   true,
				TimestampFormat: "2006-01-02 15:04:05",
				DisableColors:   true,
			})
		}

		// Open log file
		logFile, err := os.OpenFile(cfg.File.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		fileLog.SetOutput(logFile)

		l.fileLogger = fileLog
		l.fileEnabled = true
	}

	// Setup console logger if enabled
	if cfg.Console.Enabled {
		consoleLog := logrus.New()

		// Set console log level
		consoleLvl := cfg.Console.Level
		if consoleLvl == "" {
			consoleLvl = "info"
		}
		lvl, err := logrus.ParseLevel(consoleLvl)
		if err != nil {
			lvl = logrus.InfoLevel
		}
		consoleLog.SetLevel(lvl)

		// Set console format (default to text for readability)
		consoleFormat := cfg.Console.Format
		if consoleFormat == "" {
			consoleFormat = "text"
		}

		if consoleFormat == "json" {
			consoleLog.SetFormatter(&logrus.JSONFormatter{
				TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			})
		} else {
			consoleLog.SetFormatter(&logrus.TextFormatter{
				FullTimestamp:   true,
				TimestampFormat: "2006-01-02 15:04:05",
				ForceColors:     true,
			})
		}

		consoleLog.SetOutput(os.Stdout)

		l.consoleLogger = consoleLog
		l.consoleEnabled = true
	}

	// Ensure at least one logger is configured
	if !l.fileEnabled && !l.consoleEnabled {
		// Default to console if nothing specified
		consoleLog := logrus.New()
		consoleLog.SetLevel(logrus.InfoLevel)
		consoleLog.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			ForceColors:     true,
		})
		consoleLog.SetOutput(os.Stdout)
		l.consoleLogger = consoleLog
		l.consoleEnabled = true
	}

	return l, nil
}

// Info logs an info message to both outputs
func (l *Logger) Info(msg string, fields ...interface{}) {
	logFields := l.parseFields(fields...)

	if l.fileEnabled {
		if len(fields) > 0 {
			l.fileLogger.WithFields(logFields).Info(msg)
		} else {
			l.fileLogger.Info(msg)
		}
	}

	if l.consoleEnabled {
		if len(fields) > 0 {
			l.consoleLogger.WithFields(logFields).Info(msg)
		} else {
			l.consoleLogger.Info(msg)
		}
	}
}

// Warn logs a warning message to both outputs
func (l *Logger) Warn(msg string, fields ...interface{}) {
	logFields := l.parseFields(fields...)

	if l.fileEnabled {
		if len(fields) > 0 {
			l.fileLogger.WithFields(logFields).Warn(msg)
		} else {
			l.fileLogger.Warn(msg)
		}
	}

	if l.consoleEnabled {
		if len(fields) > 0 {
			l.consoleLogger.WithFields(logFields).Warn(msg)
		} else {
			l.consoleLogger.Warn(msg)
		}
	}
}

// Error logs an error message to both outputs
func (l *Logger) Error(msg string, fields ...interface{}) {
	logFields := l.parseFields(fields...)

	if l.fileEnabled {
		if len(fields) > 0 {
			l.fileLogger.WithFields(logFields).Error(msg)
		} else {
			l.fileLogger.Error(msg)
		}
	}

	if l.consoleEnabled {
		if len(fields) > 0 {
			l.consoleLogger.WithFields(logFields).Error(msg)
		} else {
			l.consoleLogger.Error(msg)
		}
	}
}

// Debug logs a debug message to both outputs
func (l *Logger) Debug(msg string, fields ...interface{}) {
	logFields := l.parseFields(fields...)

	if l.fileEnabled {
		if len(fields) > 0 {
			l.fileLogger.WithFields(logFields).Debug(msg)
		} else {
			l.fileLogger.Debug(msg)
		}
	}

	if l.consoleEnabled {
		if len(fields) > 0 {
			l.consoleLogger.WithFields(logFields).Debug(msg)
		} else {
			l.consoleLogger.Debug(msg)
		}
	}
}

// parseFields converts variadic arguments to logrus.Fields
func (l *Logger) parseFields(fields ...interface{}) logrus.Fields {
	result := make(logrus.Fields)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			result[key] = fields[i+1]
		}
	}
	return result
}
