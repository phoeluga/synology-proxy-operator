package logging

import (
	"encoding/json"
	"strings"
)

// Sensitive keys that should be redacted in logs
var sensitiveKeys = []string{
	"password", "passwd", "pwd",
	"token", "sid", "synotoken",
	"secret", "key", "credential",
	"authorization", "auth",
	"cookie", "session",
	"api_key", "apikey",
}

// FilterSensitiveData removes sensitive data from a map
func FilterSensitiveData(data map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})
	for k, v := range data {
		if isSensitiveKey(k) {
			filtered[k] = "[REDACTED]"
		} else {
			// Recursively filter nested maps
			if nested, ok := v.(map[string]interface{}); ok {
				filtered[k] = FilterSensitiveData(nested)
			} else {
				filtered[k] = v
			}
		}
	}
	return filtered
}

// FilterSensitiveJSON filters sensitive data from JSON string
func FilterSensitiveJSON(jsonStr string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// If not valid JSON, return truncated string
		return TruncateString(jsonStr, 200)
	}

	filtered := FilterSensitiveData(data)
	result, err := json.Marshal(filtered)
	if err != nil {
		return TruncateString(jsonStr, 200)
	}

	return string(result)
}

// isSensitiveKey checks if a key contains sensitive terms
func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}

// TruncateString truncates a string to max length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}

// FilterKeyValues filters sensitive data from key-value pairs
// Used for logging key-value pairs
func FilterKeyValues(keysAndValues ...interface{}) []interface{} {
	if len(keysAndValues) == 0 {
		return keysAndValues
	}

	filtered := make([]interface{}, len(keysAndValues))
	copy(filtered, keysAndValues)

	// Process pairs (key, value, key, value, ...)
	for i := 0; i < len(filtered)-1; i += 2 {
		key, ok := filtered[i].(string)
		if !ok {
			continue
		}

		if isSensitiveKey(key) {
			filtered[i+1] = "[REDACTED]"
		} else {
			// Check if value is a map and filter it
			if m, ok := filtered[i+1].(map[string]interface{}); ok {
				filtered[i+1] = FilterSensitiveData(m)
			}
			// Check if value is a string that might be JSON
			if s, ok := filtered[i+1].(string); ok && (strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")) {
				filtered[i+1] = FilterSensitiveJSON(s)
			}
		}
	}

	return filtered
}

// SanitizeError sanitizes error messages to remove sensitive data
func SanitizeError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Replace common sensitive patterns
	patterns := []struct {
		pattern     string
		replacement string
	}{
		{"password=", "password=[REDACTED]"},
		{"token=", "token=[REDACTED]"},
		{"sid=", "sid=[REDACTED]"},
		{"synotoken=", "synotoken=[REDACTED]"},
		{"Authorization: Bearer ", "Authorization: Bearer [REDACTED]"},
		{"Authorization: Basic ", "Authorization: Basic [REDACTED]"},
	}

	for _, p := range patterns {
		if strings.Contains(errStr, p.pattern) {
			// Find the pattern and redact until next space or end
			idx := strings.Index(errStr, p.pattern)
			if idx != -1 {
				start := idx + len(p.pattern)
				end := start
				for end < len(errStr) && errStr[end] != ' ' && errStr[end] != '&' && errStr[end] != '\n' {
					end++
				}
				errStr = errStr[:start] + "[REDACTED]" + errStr[end:]
			}
		}
	}

	return &sanitizedError{original: err, sanitized: errStr}
}

// sanitizedError wraps an error with sanitized message
type sanitizedError struct {
	original  error
	sanitized string
}

func (e *sanitizedError) Error() string {
	return e.sanitized
}

func (e *sanitizedError) Unwrap() error {
	return e.original
}

// AddSensitiveKey adds a custom sensitive key to the filter list
func AddSensitiveKey(key string) {
	sensitiveKeys = append(sensitiveKeys, strings.ToLower(key))
}

// GetSensitiveKeys returns the list of sensitive keys
func GetSensitiveKeys() []string {
	return append([]string{}, sensitiveKeys...)
}
