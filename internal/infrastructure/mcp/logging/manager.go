// Package logging provides MCP logging manager implementation.
package logging

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// LogEntry represents a log entry.
type LogEntry struct {
	Level     shared.MCPLogLevel `json:"level"`
	Message   string             `json:"message"`
	Logger    string             `json:"logger,omitempty"`
	Data      interface{}        `json:"data,omitempty"`
	Timestamp int64              `json:"timestamp"`
}

// LogHandler is a callback for log messages.
type LogHandler func(entry LogEntry)

// LogManager manages logging configuration and output.
type LogManager struct {
	mu         sync.RWMutex
	level      shared.MCPLogLevel
	handlers   []LogHandler
	entries    []LogEntry
	maxEntries int
}

func normalizeLogLevel(level string) shared.MCPLogLevel {
	return shared.MCPLogLevel(strings.ToLower(strings.TrimSpace(level)))
}

func safeInvokeLogHandler(handler LogHandler, entry LogEntry) {
	if handler == nil {
		return
	}

	defer func() {
		_ = recover()
	}()
	handler(entry)
}

// NewLogManager creates a new LogManager.
func NewLogManager(level shared.MCPLogLevel, maxEntries int) *LogManager {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &LogManager{
		level:      level,
		handlers:   make([]LogHandler, 0),
		entries:    make([]LogEntry, 0),
		maxEntries: maxEntries,
	}
}

// NewLogManagerWithDefaults creates a LogManager with default settings.
func NewLogManagerWithDefaults() *LogManager {
	return NewLogManager(shared.MCPLogLevelInfo, 1000)
}

// SetLevel sets the log level.
func (lm *LogManager) SetLevel(level string) error {
	if lm == nil {
		return fmt.Errorf("log manager is required")
	}

	logLevel := normalizeLogLevel(level)

	// Validate level
	switch logLevel {
	case shared.MCPLogLevelDebug,
		shared.MCPLogLevelInfo,
		shared.MCPLogLevelNotice,
		shared.MCPLogLevelWarning,
		shared.MCPLogLevelError,
		shared.MCPLogLevelCritical,
		shared.MCPLogLevelAlert,
		shared.MCPLogLevelEmergency:
		// Valid
	default:
		return fmt.Errorf("invalid log level: %s", level)
	}

	lm.mu.Lock()
	lm.level = logLevel
	lm.mu.Unlock()

	return nil
}

// GetLevel returns the current log level.
func (lm *LogManager) GetLevel() shared.MCPLogLevel {
	if lm == nil {
		return shared.MCPLogLevelInfo
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.level
}

// AddHandler adds a log handler.
func (lm *LogManager) AddHandler(handler LogHandler) {
	if lm == nil || handler == nil {
		return
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.handlers = append(lm.handlers, handler)
}

// Log logs a message at the specified level.
func (lm *LogManager) Log(level shared.MCPLogLevel, message string, data interface{}) {
	if lm == nil {
		return
	}

	if !lm.shouldLog(level) {
		return
	}

	entry := LogEntry{
		Level:     level,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}

	lm.mu.Lock()
	// Store entry
	lm.entries = append(lm.entries, entry)
	if len(lm.entries) > lm.maxEntries {
		lm.entries = lm.entries[1:]
	}
	handlers := make([]LogHandler, len(lm.handlers))
	copy(handlers, lm.handlers)
	lm.mu.Unlock()

	// Notify handlers
	for _, handler := range handlers {
		go safeInvokeLogHandler(handler, entry)
	}
}

// LogWithLogger logs a message with a specific logger name.
func (lm *LogManager) LogWithLogger(level shared.MCPLogLevel, logger, message string, data interface{}) {
	if lm == nil {
		return
	}

	if !lm.shouldLog(level) {
		return
	}

	entry := LogEntry{
		Level:     level,
		Message:   message,
		Logger:    logger,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}

	lm.mu.Lock()
	lm.entries = append(lm.entries, entry)
	if len(lm.entries) > lm.maxEntries {
		lm.entries = lm.entries[1:]
	}
	handlers := make([]LogHandler, len(lm.handlers))
	copy(handlers, lm.handlers)
	lm.mu.Unlock()

	for _, handler := range handlers {
		go safeInvokeLogHandler(handler, entry)
	}
}

// shouldLog checks if a message at the given level should be logged.
func (lm *LogManager) shouldLog(level shared.MCPLogLevel) bool {
	if lm == nil {
		return false
	}

	lm.mu.RLock()
	currentLevel := lm.level
	lm.mu.RUnlock()

	return levelPriority(level) >= levelPriority(currentLevel)
}

// levelPriority returns the priority of a log level.
func levelPriority(level shared.MCPLogLevel) int {
	switch level {
	case shared.MCPLogLevelDebug:
		return 0
	case shared.MCPLogLevelInfo:
		return 1
	case shared.MCPLogLevelNotice:
		return 2
	case shared.MCPLogLevelWarning:
		return 3
	case shared.MCPLogLevelError:
		return 4
	case shared.MCPLogLevelCritical:
		return 5
	case shared.MCPLogLevelAlert:
		return 6
	case shared.MCPLogLevelEmergency:
		return 7
	default:
		return 1 // Default to info level
	}
}

// Convenience methods

// Debug logs a debug message.
func (lm *LogManager) Debug(message string, data interface{}) {
	lm.Log(shared.MCPLogLevelDebug, message, data)
}

// Info logs an info message.
func (lm *LogManager) Info(message string, data interface{}) {
	lm.Log(shared.MCPLogLevelInfo, message, data)
}

// Notice logs a notice message.
func (lm *LogManager) Notice(message string, data interface{}) {
	lm.Log(shared.MCPLogLevelNotice, message, data)
}

// Warning logs a warning message.
func (lm *LogManager) Warning(message string, data interface{}) {
	lm.Log(shared.MCPLogLevelWarning, message, data)
}

// Error logs an error message.
func (lm *LogManager) Error(message string, data interface{}) {
	lm.Log(shared.MCPLogLevelError, message, data)
}

// Critical logs a critical message.
func (lm *LogManager) Critical(message string, data interface{}) {
	lm.Log(shared.MCPLogLevelCritical, message, data)
}

// GetEntries returns recent log entries.
func (lm *LogManager) GetEntries(limit int) []LogEntry {
	if lm == nil {
		return []LogEntry{}
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if limit <= 0 || limit > len(lm.entries) {
		limit = len(lm.entries)
	}

	start := len(lm.entries) - limit
	if start < 0 {
		start = 0
	}

	result := make([]LogEntry, limit)
	copy(result, lm.entries[start:])
	return result
}

// GetEntriesByLevel returns log entries filtered by level.
func (lm *LogManager) GetEntriesByLevel(level shared.MCPLogLevel, limit int) []LogEntry {
	if lm == nil {
		return []LogEntry{}
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make([]LogEntry, 0)
	for i := len(lm.entries) - 1; i >= 0 && len(result) < limit; i-- {
		if lm.entries[i].Level == level {
			result = append(result, lm.entries[i])
		}
	}

	// Reverse to maintain chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// Clear clears all log entries.
func (lm *LogManager) Clear() {
	if lm == nil {
		return
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.entries = make([]LogEntry, 0)
}

// Count returns the number of log entries.
func (lm *LogManager) Count() int {
	if lm == nil {
		return 0
	}

	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return len(lm.entries)
}
