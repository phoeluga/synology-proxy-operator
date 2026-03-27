package synology

import "strings"

var sensitiveKeys = []string{
	"password", "passwd", "pwd",
	"token", "sid", "synotoken",
	"secret", "key", "credential",
	"authorization", "auth",
}

// sanitizeMap removes sensitive fields from map
func sanitizeMap(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		if isSensitiveKey(k) {
			result[k] = "[REDACTED]"
		} else {
			// Recursively filter nested maps
			if nested, ok := v.(map[string]interface{}); ok {
				result[k] = sanitizeMap(nested)
			} else {
				result[k] = v
			}
		}
	}
	return result
}

// isSensitiveKey checks if key contains sensitive terms
func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}

// truncateString truncates string to max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}

// sanitizeError removes sensitive data from error messages
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	
	errStr := err.Error()
	
	// Replace common sensitive patterns
	for _, sensitive := range sensitiveKeys {
		// Simple pattern matching - in production, use more sophisticated regex
		if strings.Contains(strings.ToLower(errStr), sensitive) {
			errStr = strings.ReplaceAll(errStr, sensitive, "[REDACTED]")
		}
	}
	
	return truncateString(errStr, 200)
}
