package logger

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/susamn/obsidian-web/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// Log is the global logger instance
	Log *logrus.Logger
)

// init initializes the logger with a basic configuration
// This ensures the logger is always usable, even before Initialize() is called
func init() {
	Log = logrus.New()
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	Log.SetOutput(os.Stdout)
	Log.SetLevel(logrus.InfoLevel)
}

// Initialize sets up the logger based on configuration
func Initialize(cfg *config.LoggingConfig) error {
	Log = logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
		Log.Warnf("Invalid log level '%s', defaulting to 'info'", cfg.Level)
	}
	Log.SetLevel(level)

	// Set log format
	switch strings.ToLower(cfg.Format) {
	case "json":
		Log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	case "text":
		Log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	default:
		Log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// Set output destination
	var output io.Writer
	switch strings.ToLower(cfg.Output) {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	case "file":
		if cfg.File.Path == "" {
			return fmt.Errorf("log file path is required when output is 'file'")
		}

		// Use lumberjack for log rotation
		output = &lumberjack.Logger{
			Filename:   cfg.File.Path,
			MaxSize:    cfg.File.MaxSize,    // megabytes
			MaxBackups: cfg.File.MaxBackups, // number of old files to keep
			MaxAge:     cfg.File.MaxAge,     // days
			Compress:   cfg.File.Compress,   // compress rotated files
		}

		Log.Infof("Logging to file: %s (max_size: %dMB, max_backups: %d, max_age: %dd, compress: %v)",
			cfg.File.Path, cfg.File.MaxSize, cfg.File.MaxBackups, cfg.File.MaxAge, cfg.File.Compress)
	default:
		output = os.Stdout
	}

	Log.SetOutput(output)

	Log.WithFields(logrus.Fields{
		"level":  cfg.Level,
		"format": cfg.Format,
		"output": cfg.Output,
	}).Info("Logger initialized")

	return nil
}

// WithField creates an entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return Log.WithField(key, value)
}

// WithFields creates an entry with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Log.WithFields(fields)
}

// WithError creates an entry with an error field
func WithError(err error) *logrus.Entry {
	return Log.WithError(err)
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	Log.Debug(args...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	Log.Debugf(format, args...)
}

// Info logs an info message
func Info(args ...interface{}) {
	Log.Info(args...)
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	Log.Infof(format, args...)
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	Log.Warn(args...)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	Log.Warnf(format, args...)
}

// Error logs an error message
func Error(args ...interface{}) {
	Log.Error(args...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	Log.Errorf(format, args...)
}

// Fatal logs a fatal message and exits
func Fatal(args ...interface{}) {
	Log.Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(format string, args ...interface{}) {
	Log.Fatalf(format, args...)
}

// Panic logs a panic message and panics
func Panic(args ...interface{}) {
	Log.Panic(args...)
}

// Panicf logs a formatted panic message and panics
func Panicf(format string, args ...interface{}) {
	Log.Panicf(format, args...)
}
