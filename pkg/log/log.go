package log

import (
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
)

var (
	loggerMu sync.RWMutex
	logger   hclog.Logger = hclog.NewNullLogger()
)

// SetLogger sets the global logger.
func SetLogger(l hclog.Logger) {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if l == nil {
		logger = hclog.NewNullLogger()
		return
	}

	logger = l
}

// L returns the current global logger.
func L() hclog.Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return logger
}

// Debug logs a debug message.
func Debug(msg string, args ...interface{}) {
	L().Debug(msg, args...)
}

// Info logs an info message.
func Info(msg string, args ...interface{}) {
	L().Info(msg, args...)
}

// Warn logs a warning message.
func Warn(msg string, args ...interface{}) {
	L().Warn(msg, args...)
}

// Error logs an error message.
func Error(msg string, args ...interface{}) {
	L().Error(msg, args...)
}

// Named returns a child logger that inherits the configured options.
func Named(name string) hclog.Logger {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return L()
	}

	return L().Named(trimmed)
}
