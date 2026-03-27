package synology

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_Types(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "auth error",
			err:  &AuthError{Message: "invalid credentials"},
			want: "authentication failed: invalid credentials",
		},
		{
			name: "rate limit error",
			err:  &RateLimitError{RetryAfter: 60},
			want: "rate limit exceeded, retry after 60 seconds",
		},
		{
			name: "not found error",
			err:  &NotFoundError{Resource: "proxy record", ID: "123"},
			want: "proxy record not found: 123",
		},
		{
			name: "validation error",
			err:  &ValidationError{Field: "hostname", Message: "invalid format"},
			want: "validation error on field 'hostname': invalid format",
		},
		{
			name: "api error",
			err:  &APIError{Code: 400, Message: "bad request"},
			want: "API error (400): bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "rate limit error - retryable",
			err:  &RateLimitError{RetryAfter: 60},
			want: true,
		},
		{
			name: "timeout error - retryable",
			err:  &TimeoutError{Operation: "create"},
			want: true,
		},
		{
			name: "auth error - not retryable",
			err:  &AuthError{Message: "invalid credentials"},
			want: false,
		},
		{
			name: "validation error - not retryable",
			err:  &ValidationError{Field: "hostname"},
			want: false,
		},
		{
			name: "not found error - not retryable",
			err:  &NotFoundError{Resource: "record"},
			want: false,
		},
		{
			name: "unknown error - retryable",
			err:  errors.New("unknown error"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClassifyHTTPStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "500 - retryable", status: 500, want: true},
		{name: "502 - retryable", status: 502, want: true},
		{name: "503 - retryable", status: 503, want: true},
		{name: "429 - retryable", status: 429, want: true},
		{name: "400 - not retryable", status: 400, want: false},
		{name: "401 - not retryable", status: 401, want: false},
		{name: "404 - not retryable", status: 404, want: false},
		{name: "200 - not retryable", status: 200, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyHTTPStatus(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}
