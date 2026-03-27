package synology

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestClient(t *testing.T) (*Client, *MockServer) {
	mock := NewMockServer()

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

	return client, mock
}

func TestProxy_List(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	records, err := client.Proxy.List(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, records)
}

func TestProxy_Get(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	// Create a record first
	record := &ProxyRecord{
		FrontendHostname: "test.example.com",
		FrontendPort:     443,
		FrontendProtocol: "https",
		BackendHostname:  "backend.local",
		BackendPort:      8080,
		BackendProtocol:  "http",
	}

	created, err := client.Proxy.Create(context.Background(), record)
	require.NoError(t, err)

	// Get the record
	retrieved, err := client.Proxy.Get(context.Background(), created.UUID)
	assert.NoError(t, err)
	assert.Equal(t, created.UUID, retrieved.UUID)
	assert.Equal(t, "test.example.com", retrieved.FrontendHostname)
}

func TestProxy_Get_NotFound(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	_, err := client.Proxy.Get(context.Background(), "nonexistent-uuid")
	assert.Error(t, err)
	assert.IsType(t, &NotFoundError{}, err)
}

func TestProxy_Create(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	record := &ProxyRecord{
		FrontendHostname: "test.example.com",
		FrontendPort:     443,
		FrontendProtocol: "https",
		BackendHostname:  "backend.local",
		BackendPort:      8080,
		BackendProtocol:  "http",
	}

	created, err := client.Proxy.Create(context.Background(), record)
	assert.NoError(t, err)
	assert.NotEmpty(t, created.UUID)
	assert.Equal(t, "test.example.com", created.FrontendHostname)
}

func TestProxy_Create_ValidationError(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	record := &ProxyRecord{
		// Missing required fields
		FrontendPort: 443,
	}

	_, err := client.Proxy.Create(context.Background(), record)
	assert.Error(t, err)
	assert.IsType(t, &ValidationError{}, err)
}

func TestProxy_Update(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	// Create a record
	record := &ProxyRecord{
		FrontendHostname: "test.example.com",
		FrontendPort:     443,
		FrontendProtocol: "https",
		BackendHostname:  "backend.local",
		BackendPort:      8080,
		BackendProtocol:  "http",
	}

	created, err := client.Proxy.Create(context.Background(), record)
	require.NoError(t, err)

	// Update the record
	created.BackendPort = 9090
	updated, err := client.Proxy.Update(context.Background(), created)
	assert.NoError(t, err)
	assert.Equal(t, 9090, updated.BackendPort)
}

func TestProxy_Update_NotFound(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	record := &ProxyRecord{
		UUID:             "nonexistent-uuid",
		FrontendHostname: "test.example.com",
		FrontendPort:     443,
		FrontendProtocol: "https",
		BackendHostname:  "backend.local",
		BackendPort:      8080,
		BackendProtocol:  "http",
	}

	_, err := client.Proxy.Update(context.Background(), record)
	assert.Error(t, err)
	assert.IsType(t, &NotFoundError{}, err)
}

func TestProxy_Delete(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	// Create a record
	record := &ProxyRecord{
		FrontendHostname: "test.example.com",
		FrontendPort:     443,
		FrontendProtocol: "https",
		BackendHostname:  "backend.local",
		BackendPort:      8080,
		BackendProtocol:  "http",
	}

	created, err := client.Proxy.Create(context.Background(), record)
	require.NoError(t, err)

	// Delete the record
	err = client.Proxy.Delete(context.Background(), created.UUID)
	assert.NoError(t, err)

	// Verify it's deleted
	_, err = client.Proxy.Get(context.Background(), created.UUID)
	assert.Error(t, err)
	assert.IsType(t, &NotFoundError{}, err)
}

func TestProxy_Delete_NotFound(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	err := client.Proxy.Delete(context.Background(), "nonexistent-uuid")
	assert.Error(t, err)
	assert.IsType(t, &NotFoundError{}, err)
}

func TestProxy_WithRetry(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	// Set failure mode to simulate transient error
	mock.SetFailureMode("500")

	record := &ProxyRecord{
		FrontendHostname: "test.example.com",
		FrontendPort:     443,
		FrontendProtocol: "https",
		BackendHostname:  "backend.local",
		BackendPort:      8080,
		BackendProtocol:  "http",
	}

	// Should fail with retries
	_, err := client.Proxy.Create(context.Background(), record)
	assert.Error(t, err)

	// Reset failure mode
	mock.SetFailureMode("none")

	// Should succeed now
	created, err := client.Proxy.Create(context.Background(), record)
	assert.NoError(t, err)
	assert.NotEmpty(t, created.UUID)
}

func TestProxy_ContextCancellation(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	record := &ProxyRecord{
		FrontendHostname: "test.example.com",
		FrontendPort:     443,
		FrontendProtocol: "https",
		BackendHostname:  "backend.local",
		BackendPort:      8080,
		BackendProtocol:  "http",
	}

	_, err := client.Proxy.Create(ctx, record)
	assert.Error(t, err)
}
