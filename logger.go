// Package main - Logging utilities for the PDF to Markdown MCP server.
// This file provides a structured logging system with different log levels and formatted output.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Logger provides structured logging with configurable log levels for the MCP server.
// It supports different severity levels (debug, info, warn, error, fatal) and formats
// log messages with timestamps and severity indicators.
type Logger struct {
	level  LogLevel    // Current minimum log level to output
	logger *log.Logger // Underlying Go standard library logger
}

// LogLevel represents the severity level of log messages.
// Higher numeric values indicate more severe log levels.
type LogLevel int

const (
	// LogDebug represents detailed debugging information, typically only of interest
	// when diagnosing problems or understanding detailed program flow.
	LogDebug LogLevel = iota

	// LogInfo represents general informational messages that highlight the progress
	// of the application at a coarse-grained level.
	LogInfo

	// LogWarn represents potentially harmful situations or unexpected conditions
	// that don't prevent the application from continuing.
	LogWarn

	// LogError represents error events that might still allow the application
	// to continue running but indicate a significant problem.
	LogError

	// LogFatal represents very severe error events that will presumably lead
	// the application to abort or terminate.
	LogFatal
)

// String returns the string representation of a LogLevel for display purposes.
// This is used in log message formatting to show the severity level.
//
// Returns:
//   - string: Human-readable log level name (DEBUG, INFO, WARN, ERROR, FATAL)
func (l LogLevel) String() string {
	switch l {
	case LogDebug:
		return "DEBUG"
	case LogInfo:
		return "INFO"
	case LogWarn:
		return "WARN"
	case LogError:
		return "ERROR"
	case LogFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// NewLogger creates a new Logger instance with the specified minimum log level.
// Messages below this level will be filtered out and not displayed.
//
// Parameters:
//   - levelStr: String representation of the desired log level (debug, info, warn, error, fatal)
//
// Returns:
//   - *Logger: Configured logger instance ready for use
func NewLogger(levelStr string) *Logger {
	level := parseLogLevel(levelStr)

	return &Logger{
		level:  level,
		logger: log.New(os.Stderr, "", 0), // No default flags, we'll format ourselves
	}
}

// parseLogLevel converts a string log level name to a LogLevel enum value.
// The comparison is case-insensitive for user convenience.
//
// Parameters:
//   - levelStr: String representation of log level (debug, info, warn, error, fatal)
//
// Returns:
//   - LogLevel: Corresponding LogLevel enum value, defaults to LogInfo for invalid input
func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToLower(levelStr) {
	case "debug":
		return LogDebug
	case "info":
		return LogInfo
	case "warn", "warning":
		return LogWarn
	case "error":
		return LogError
	case "fatal":
		return LogFatal
	default:
		return LogInfo // Default to info level
	}
}

// log is the internal method that handles the actual logging logic.
// It checks if the message level meets the minimum threshold and formats the output.
//
// Parameters:
//   - level: Severity level of this log message
//   - format: Printf-style format string for the message
//   - args: Arguments to substitute into the format string
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	// Only log if the message level is at or above our configured minimum level
	if level < l.level {
		return
	}

	// Format the timestamp in ISO 8601 format for consistency
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")

	// Create the full log message with timestamp, level, and formatted content
	message := fmt.Sprintf(format, args...)
	fullMessage := fmt.Sprintf("%s [%s] %s", timestamp, level.String(), message)

	// Output the formatted message
	l.logger.Println(fullMessage)
}

// Debug logs a debug-level message. These messages are typically used for detailed
// diagnostic information that's only useful when diagnosing problems.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments to substitute into the format string
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LogDebug, format, args...)
}

// Info logs an informational message. These messages highlight the general flow
// and progress of the application.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments to substitute into the format string
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LogInfo, format, args...)
}

// Warn logs a warning message. These indicate potentially harmful situations
// or unexpected conditions that don't prevent continued operation.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments to substitute into the format string
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LogWarn, format, args...)
}

// Error logs an error message. These indicate significant problems that occurred
// but may allow the application to continue running.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments to substitute into the format string
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LogError, format, args...)
}

// Fatal logs a fatal error message and then terminates the application.
// This should only be used for critical errors that make continued operation impossible.
//
// Parameters:
//   - format: Printf-style format string
//   - args: Arguments to substitute into the format string
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(LogFatal, format, args...)
	os.Exit(1)
}
