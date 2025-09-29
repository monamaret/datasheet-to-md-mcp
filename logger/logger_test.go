package logger

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		levelStr      string
		expectedLevel LogLevel
	}{
		{"debug", LogDebug}, {"info", LogInfo}, {"warn", LogWarn}, {"warning", LogWarn}, {"error", LogError}, {"fatal", LogFatal}, {"DEBUG", LogDebug}, {"INFO", LogInfo}, {"invalid", LogInfo}, {"", LogInfo},
	}
	for _, tt := range tests {
		t.Run("level_"+tt.levelStr, func(t *testing.T) {
			logger := NewLogger(tt.levelStr)
			if logger.level != tt.expectedLevel {
				t.Errorf("Expected level %v, got %v", tt.expectedLevel, logger.level)
			}
		})
	}
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogDebug, "DEBUG"}, {LogInfo, "INFO"}, {LogWarn, "WARN"}, {LogError, "ERROR"}, {LogFatal, "FATAL"}, {LogLevel(999), "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		levelStr string
		expected LogLevel
	}{
		{"debug", LogDebug}, {"DEBUG", LogDebug}, {"info", LogInfo}, {"INFO", LogInfo}, {"warn", LogWarn}, {"WARN", LogWarn}, {"warning", LogWarn}, {"WARNING", LogWarn}, {"error", LogError}, {"ERROR", LogError}, {"fatal", LogFatal}, {"FATAL", LogFatal}, {"invalid", LogInfo}, {"", LogInfo},
	}
	for _, tt := range tests {
		t.Run("parse_"+tt.levelStr, func(t *testing.T) {
			result := parseLogLevel(tt.levelStr)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLogger_LogLevels(t *testing.T) {
	var buf bytes.Buffer
	originalStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	logger := NewLogger("info")
	logger.logger = log.New(&buf, "", 0)

	tests := []struct {
		name      string
		logFunc   func(string, ...interface{})
		level     LogLevel
		shouldLog bool
	}{
		{"debug_message", logger.Debug, LogDebug, false}, {"info_message", logger.Info, LogInfo, true}, {"warn_message", logger.Warn, LogWarn, true}, {"error_message", logger.Error, LogError, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc("test message")
			output := buf.String()
			hasOutput := len(output) > 0
			if tt.shouldLog && !hasOutput {
				t.Errorf("Expected log output for %s level, got none", tt.level.String())
			}
			if !tt.shouldLog && hasOutput {
				t.Errorf("Expected no log output for %s level, got: %s", tt.level.String(), output)
			}
			if hasOutput {
				if !strings.Contains(output, tt.level.String()) {
					t.Errorf("Expected level '%s', got: %s", tt.level.String(), output)
				}
				if !strings.Contains(output, "test message") {
					t.Errorf("Expected 'test message', got: %s", output)
				}
				if !strings.Contains(output, "T") || !strings.Contains(output, ":") {
					t.Errorf("Expected timestamp, got: %s", output)
				}
			}
		})
	}
	w.Close()
	os.Stderr = originalStderr
	r.Close()
}

func TestLogger_LogFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger("debug")
	logger.logger = log.New(&buf, "", 0)

	t.Run("formatted message", func(t *testing.T) {
		logger.Info("Processing file %s with %d pages", "test.pdf", 42)
		output := buf.String()
		if !strings.Contains(output, "Processing file test.pdf with 42 pages") {
			t.Errorf("Expected formatted message, got: %s", output)
		}
		if !strings.Contains(output, "[INFO]") {
			t.Errorf("Expected [INFO], got: %s", output)
		}
	})

	t.Run("message with special characters", func(t *testing.T) {
		buf.Reset()
		logger.Warn("File path contains special chars: /test/file%%20name.pdf")
		output := buf.String()
		if !strings.Contains(output, "File path contains special chars: /test/file%20name.pdf") {
			t.Errorf("Expected special characters preserved, got: %s", output)
		}
		if !strings.Contains(output, "[WARN]") {
			t.Errorf("Expected [WARN], got: %s", output)
		}
	})
}

func TestLogger_Fatal(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger("debug")
	logger.logger = log.New(&buf, "", 0)
	logger.log(LogFatal, "Fatal error: %s", "system failure")
	output := buf.String()
	if !strings.Contains(output, "[FATAL]") {
		t.Errorf("Expected [FATAL], got: %s", output)
	}
	if !strings.Contains(output, "Fatal error: system failure") {
		t.Errorf("Expected fatal message, got: %s", output)
	}
}

func TestLogger_MultipleLoggers(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	debugLogger := NewLogger("debug")
	debugLogger.logger = log.New(&buf1, "", 0)
	errorLogger := NewLogger("error")
	errorLogger.logger = log.New(&buf2, "", 0)
	debugLogger.Error("Error from debug logger")
	errorLogger.Error("Error from error logger")
	if !strings.Contains(buf1.String(), "Error from debug logger") {
		t.Error("Debug logger should log error messages")
	}
	if !strings.Contains(buf2.String(), "Error from error logger") {
		t.Error("Error logger should log error messages")
	}
	buf1.Reset()
	buf2.Reset()
	debugLogger.Debug("Debug from debug logger")
	errorLogger.Debug("Debug from error logger")
	if !strings.Contains(buf1.String(), "Debug from debug logger") {
		t.Error("Debug logger should log debug messages")
	}
	if buf2.String() != "" {
		t.Errorf("Error logger should not log debug messages, got: %s", buf2.String())
	}
}

func TestLogger_TimestampFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger("info")
	logger.logger = log.New(&buf, "", 0)
	logger.Info("Test timestamp")
	output := buf.String()
	if !strings.Contains(output, "T") {
		t.Error("Expected 'T' in timestamp")
	}
	if strings.Count(output, ":") < 2 {
		t.Errorf("Expected at least 2 colons in timestamp, got: %s", output)
	}
	if strings.Count(output, "-") < 2 {
		t.Errorf("Expected at least 2 dashes in date, got: %s", output)
	}
}

func TestLogger_EdgeCases(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger("info")
	logger.logger = log.New(&buf, "", 0)
	t.Run("empty message", func(t *testing.T) {
		buf.Reset()
		logger.Info("")
		output := buf.String()
		if !strings.Contains(output, "[INFO]") {
			t.Error("Should include level for empty message")
		}
		if len(output) < 20 {
			t.Errorf("Expected timestamp and level, got: %s", output)
		}
	})
	t.Run("nil arguments", func(t *testing.T) {
		buf.Reset()
		logger.Info("Test with nil: %v", nil)
		output := buf.String()
		if !strings.Contains(output, "Test with nil: <nil>") {
			t.Errorf("Expected nil formatted '<nil>', got: %s", output)
		}
	})
	t.Run("many arguments", func(t *testing.T) {
		buf.Reset()
		buf.Reset()
		logger.Info("Args: %s %d %.2f %t", "string", 42, 3.14, true)
		output := buf.String()
		expected := "Args: string 42 3.14 true"
		if !strings.Contains(output, expected) {
			t.Errorf("Expected '%s', got: %s", expected, output)
		}
	})
}
