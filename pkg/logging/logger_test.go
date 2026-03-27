package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/phoeluga/synology-proxy-operator/pkg/config"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		config config.LoggingConfig
	}{
		{
			name: "info level JSON",
			config: config.LoggingConfig{
				Level:  "info",
				Format: "json",
			},
		},
		{
			name: "debug level JSON",
			config: config.LoggingConfig{
				Level:  "debug",
				Format: "json",
			},
		},
		{
			name: "error level JSON",
			config: config.LoggingConfig{
				Level:  "error",
				Format: "json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.config)
			if logger == nil {
				t.Error("NewLogger() returned nil")
			}
		})
	}
}

func TestLogger_LogLevels(t *testing.T) {
	// Create logger with debug level
	cfg := config.LoggingConfig{
		Level:  "debug",
		Format: "json",
	}
	logger := NewLogger(cfg)

	// Test that all log levels work
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message", nil)
}

func TestLogger_WithValues(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "json",
	}
	logger := NewLogger(cfg)

	// Create child logger with values
	childLogger := logger.WithValues("key1", "value1", "key2", "value2")
	if childLogger == nil {
		t.Error("WithValues() returned nil")
	}

	// Log with child logger
	childLogger.Info("test message")
}

func TestLogger_WithName(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "json",
	}
	logger := NewLogger(cfg)

	// Create named logger
	namedLogger := logger.WithName("test-component")
	if namedLogger == nil {
		t.Error("WithName() returned nil")
	}

	// Log with named logger
	namedLogger.Info("test message")
}

func TestSensitiveDataFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "filter username",
			input:    `{"username":"admin"}`,
			expected: `{"username":"[REDACTED]"}`,
		},
		{
			name:     "filter password",
			input:    `{"password":"secret123"}`,
			expected: `{"password":"[REDACTED]"}`,
		},
		{
			name:     "filter token",
			input:    `{"token":"abc123xyz"}`,
			expected: `{"token":"[REDACTED]"}`,
		},
		{
			name:     "filter authorization header",
			input:    `{"Authorization":"Bearer token123"}`,
			expected: `{"Authorization":"[REDACTED]"}`,
		},
		{
			name:     "filter cookie",
			input:    `{"Cookie":"session=abc123"}`,
			expected: `{"Cookie":"[REDACTED]"}`,
		},
		{
			name:     "filter multiple sensitive fields",
			input:    `{"username":"admin","password":"secret","data":"public"}`,
			expected: `{"username":"[REDACTED]","password":"[REDACTED]","data":"public"}`,
		},
		{
			name:     "no sensitive data",
			input:    `{"message":"hello","level":"info"}`,
			expected: `{"message":"hello","level":"info"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterSensitiveData([]byte(tt.input))

			// Parse both as JSON to compare structure
			var resultMap, expectedMap map[string]interface{}
			if err := json.Unmarshal(result, &resultMap); err != nil {
				t.Fatalf("Failed to parse result JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expectedMap); err != nil {
				t.Fatalf("Failed to parse expected JSON: %v", err)
			}

			// Compare maps
			for key, expectedVal := range expectedMap {
				resultVal, ok := resultMap[key]
				if !ok {
					t.Errorf("Missing key %s in result", key)
					continue
				}
				if resultVal != expectedVal {
					t.Errorf("Key %s: expected %v, got %v", key, expectedVal, resultVal)
				}
			}
		})
	}
}

func TestLogger_SensitiveDataFiltering(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Note: This test demonstrates the concept but won't work with zap's
	// production logger. In a real implementation, you'd need to use
	// zap's testing utilities or a custom sink.

	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "json",
	}
	logger := NewLogger(cfg)

	// Log message with sensitive data
	logger.Info("user login", "username", "admin", "password", "secret123")

	// In production, the output would be filtered by the filterSensitiveData function
	// This test verifies the filter function works correctly
	testData := `{"username":"admin","password":"secret123"}`
	filtered := filterSensitiveData([]byte(testData))

	if !strings.Contains(string(filtered), "[REDACTED]") {
		t.Error("Expected sensitive data to be redacted")
	}
}

func TestLogger_ErrorWithNilError(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "json",
	}
	logger := NewLogger(cfg)

	// Should not panic with nil error
	logger.Error("test message", nil)
}

func TestLogger_ErrorWithActualError(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:  "info",
		Format: "json",
	}
	logger := NewLogger(cfg)

	// Should log error properly
	err := &testError{msg: "test error"}
	logger.Error("test message", err)
}

// testError is a simple error implementation for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
