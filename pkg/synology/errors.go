package synology

import "fmt"

// APIError represents an error from Synology API
type APIError struct {
	Code       int
	Message    string
	HTTPStatus int
	Operation  string
	Retryable  bool
	Context    map[string]interface{}
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s (operation: %s)", e.Code, e.Message, e.Operation)
}

// NotFoundError represents a resource not found error
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// AuthError represents an authentication error
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication failed: %s", e.Message)
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}

// NetworkError represents a network-level error
type NetworkError struct {
	Message string
	Cause   error
}

func (e *NetworkError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("network error: %s (cause: %v)", e.Message, e.Cause)
	}
	return fmt.Sprintf("network error: %s", e.Message)
}

// isRetryable determines if an error should be retried
func isRetryable(err error) bool {
	switch e := err.(type) {
	case *NetworkError:
		return true
	case *APIError:
		return e.Retryable
	case *AuthError:
		return false
	case *ValidationError:
		return false
	case *NotFoundError:
		return false
	default:
		return true // Unknown errors are retryable
	}
}

// classifyHTTPStatus classifies HTTP status code for retryability
func classifyHTTPStatus(status int) bool {
	switch {
	case status >= 500: // 5xx errors
		return true
	case status == 429: // Rate limit
		return true
	case status == 408: // Request timeout
		return true
	default:
		return false
	}
}

// classifyAPIError creates an APIError from API response
func classifyAPIError(code int, operation string) *APIError {
	var message string
	var retryable bool

	switch code {
	case 100:
		message = "Unknown error"
		retryable = true
	case 101:
		message = "Invalid parameter"
		retryable = false
	case 102:
		message = "API does not exist"
		retryable = false
	case 103:
		message = "Method does not exist"
		retryable = false
	case 104:
		message = "Not supported in current version"
		retryable = false
	case 105:
		message = "Insufficient user privilege"
		retryable = false
	case 106:
		message = "Connection timeout"
		retryable = true
	case 107:
		message = "Multiple login detected"
		retryable = false
	case 400:
		message = "Invalid request"
		retryable = false
	case 401:
		message = "Not authorized"
		retryable = false
	case 403:
		message = "Forbidden"
		retryable = false
	case 404:
		message = "Not found"
		retryable = false
	case 500:
		message = "Internal server error"
		retryable = true
	default:
		message = fmt.Sprintf("Unknown error code: %d", code)
		retryable = true
	}

	return &APIError{
		Code:      code,
		Message:   message,
		Operation: operation,
		Retryable: retryable,
	}
}
