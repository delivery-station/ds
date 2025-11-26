package log

import (
	"os"

	"github.com/hashicorp/go-hclog"
)

var logger hclog.Logger

func init() {
	logger = hclog.New(&hclog.LoggerOptions{
		Name:   "ds",
		Output: os.Stderr,
		Level:  hclog.Info,
	})
}

// SetLogger sets the global logger
func SetLogger(l hclog.Logger) {
	logger = l
}

// L returns the global logger
func L() hclog.Logger {
	return logger
}

// Debug logs a debug message
func Debug(msg string, args ...interface{}) {
	logger.Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...interface{}) {
	logger.Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...interface{}) {
	logger.Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...interface{}) {
	logger.Error(msg, args...)
}
