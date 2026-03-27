package synology

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "URL with password in query",
			input: "https://nas.example.com:5001/api?username=admin&password=secret123",
			want:  "https://nas.example.com:5001/api?username=admin&password=***",
		},
		{
			name:  "URL with sid in query",
			input: "https://nas.example.com:5001/api?sid=abc123def456",
			want:  "https://nas.example.com:5001/api?sid=***",
		},
		{
			name:  "URL with synotoken in query",
			input: "https://nas.example.com:5001/api?synotoken=token123",
			want:  "https://nas.example.com:5001/api?synotoken=***",
		},
		{
			name:  "URL without sensitive data",
			input: "https://nas.example.com:5001/api?method=list",
			want:  "https://nas.example.com:5001/api?method=list",
		},
		{
			name:  "URL with multiple sensitive params",
			input: "https://nas.example.com:5001/api?username=admin&password=secret&sid=abc123",
			want:  "https://nas.example.com:5001/api?username=admin&password=***&sid=***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeURL(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSanitizeJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "JSON with password field",
			input: `{"username":"admin","password":"secret123"}`,
			want:  `{"username":"admin","password":"***"}`,
		},
		{
			name:  "JSON with sid field",
			input: `{"sid":"abc123def456","data":{}}`,
			want:  `{"sid":"***","data":{}}`,
		},
		{
			name:  "JSON with synotoken field",
			input: `{"synotoken":"token123","success":true}`,
			want:  `{"synotoken":"***","success":true}`,
		},
		{
			name:  "JSON without sensitive data",
			input: `{"method":"list","success":true}`,
			want:  `{"method":"list","success":true}`,
		},
		{
			name:  "JSON with nested sensitive data",
			input: `{"data":{"password":"secret","username":"admin"}}`,
			want:  `{"data":{"password":"***","username":"admin"}}`,
		},
		{
			name:  "JSON with multiple sensitive fields",
			input: `{"password":"secret","sid":"abc","synotoken":"token"}`,
			want:  `{"password":"***","sid":"***","synotoken":"***"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeJSON(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "string with password keyword",
			input: "password=secret123 username=admin",
			want:  "password=*** username=admin",
		},
		{
			name:  "string with sid keyword",
			input: "sid=abc123def456",
			want:  "sid=***",
		},
		{
			name:  "string with synotoken keyword",
			input: "synotoken=token123",
			want:  "synotoken=***",
		},
		{
			name:  "string without sensitive data",
			input: "method=list success=true",
			want:  "method=list success=true",
		},
		{
			name:  "string with multiple sensitive keywords",
			input: "password=secret sid=abc123 synotoken=token",
			want:  "password=*** sid=*** synotoken=***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSanitizeHeaders(t *testing.T) {
	tests := []struct {
		name  string
		input map[string][]string
		want  map[string][]string
	}{
		{
			name: "headers with authorization",
			input: map[string][]string{
				"Authorization": {"Bearer token123"},
				"Content-Type":  {"application/json"},
			},
			want: map[string][]string{
				"Authorization": {"***"},
				"Content-Type":  {"application/json"},
			},
		},
		{
			name: "headers with cookie",
			input: map[string][]string{
				"Cookie":       {"session=abc123"},
				"Content-Type": {"application/json"},
			},
			want: map[string][]string{
				"Cookie":       {"***"},
				"Content-Type": {"application/json"},
			},
		},
		{
			name: "headers without sensitive data",
			input: map[string][]string{
				"Content-Type": {"application/json"},
				"Accept":       {"application/json"},
			},
			want: map[string][]string{
				"Content-Type": {"application/json"},
				"Accept":       {"application/json"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeHeaders(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name  string
		input error
		want  string
	}{
		{
			name:  "error with password",
			input: &AuthError{Message: "authentication failed with password=secret123"},
			want:  "authentication failed with password=***",
		},
		{
			name:  "error with sid",
			input: &APIError{Message: "invalid sid=abc123def456"},
			want:  "invalid sid=***",
		},
		{
			name:  "error without sensitive data",
			input: &ValidationError{Message: "invalid hostname format"},
			want:  "invalid hostname format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeError(tt.input)
			assert.Contains(t, got, tt.want)
		})
	}
}
