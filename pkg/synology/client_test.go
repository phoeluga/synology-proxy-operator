package synology

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Success(t *testing.T) {
	mock := NewMockServer()
	defer mock.Close()

	config := Config{
		BaseURL:        mock.URL(),
		Username:       "admin",
		Password:       "password",
		TLSVerify:      false,
		MaxRetries:     3,
		RateLimit:      10.0,
		RateBurst:      5,
		RequestTimeout: 30 * time.Second,
	}

	client, err := New(config, &testLogger{}, &testMetrics{})
	require.NoError(t, err)
	assert.NotNil(t, client)

	err = client.Close()
	assert.NoError(t, err)
}

func TestNew_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "missing base URL",
			config: Config{
				Username: "admin",
				Password: "password",
			},
		},
		{
			name: "missing username",
			config: Config{
				BaseURL:  "https://nas.example.com",
				Password: "password",
			},
		},
		{
			name: "missing password",
			config: Config{
				BaseURL:  "https://nas.example.com",
				Username: "admin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config, &testLogger{}, &testMetrics{})
			assert.Error(t, err)
			assert.Nil(t, client)
		})
	}
}

func TestNew_AuthenticationFailure(t *testing.T) {
	mock := NewMockServer()
	defer mock.Close()

	mock.SetFailureMode("auth")

	config := Config{
		BaseURL:        mock.URL(),
		Username:       "admin",
		Password:       "wrong",
		TLSVerify:      false,
		MaxRetries:     1,
		RequestTimeout: 5 * time.Second,
	}

	client, err := New(config, &testLogger{}, &testMetrics{})
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestClient_HealthCheck(t *testing.T) {
	mock := NewMockServer()
	defer mock.Close()

	config := Config{
		BaseURL:        mock.URL(),
		Username:       "admin",
		Password:       "password",
		TLSVerify:      false,
		MaxRetries:     3,
		RequestTimeout: 30 * time.Second,
	}

	client, err := New(config, &testLogger{}, &testMetrics{})
	require.NoError(t, err)
	defer client.Close()

	// Should be healthy
	healthy := client.HealthCheck(context.Background())
	assert.True(t, healthy)

	// Trip circuit breaker
	for i := 0; i < 5; i++ {
		client.circuitBreaker.RecordFailure()
	}

	// Should be unhealthy
	healthy = client.HealthCheck(context.Background())
	assert.False(t, healthy)
}

func TestClient_Close(t *testing.T) {
	mock := NewMockServer()
	defer mock.Close()

	config := Config{
		BaseURL:        mock.URL(),
		Username:       "admin",
		Password:       "password",
		TLSVerify:      false,
		MaxRetries:     3,
		RequestTimeout: 30 * time.Second,
	}

	client, err := New(config, &testLogger{}, &testMetrics{})
	require.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				BaseURL:        "https://nas.example.com",
				Username:       "admin",
				Password:       "password",
				MaxRetries:     3,
				RequestTimeout: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing base URL",
			config: Config{
				Username: "admin",
				Password: "password",
			},
			wantErr: true,
		},
		{
			name: "missing username",
			config: Config{
				BaseURL:  "https://nas.example.com",
				Password: "password",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			config: Config{
				BaseURL:  "https://nas.example.com",
				Username: "admin",
			},
			wantErr: true,
		},
		{
			name: "invalid max retries",
			config: Config{
				BaseURL:    "https://nas.example.com",
				Username:   "admin",
				Password:   "password",
				MaxRetries: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
