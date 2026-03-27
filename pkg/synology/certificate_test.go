package synology

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCertificate_List(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	certs, err := client.Certificate.List(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, certs)
	assert.Greater(t, len(certs), 0)
}

func TestCertificate_List_Cached(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	// First call - should hit API
	certs1, err := client.Certificate.List(context.Background())
	require.NoError(t, err)

	// Second call - should use cache
	certs2, err := client.Certificate.List(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, len(certs1), len(certs2))
}

func TestCertificate_List_CacheInvalidation(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	// Get certificates (cached)
	_, err := client.Certificate.List(context.Background())
	require.NoError(t, err)

	// Invalidate cache
	client.certCache.Invalidate()

	// Should fetch from API again
	certs, err := client.Certificate.List(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, certs)
}

func TestCertificate_Assign(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	// Get available certificates
	certs, err := client.Certificate.List(context.Background())
	require.NoError(t, err)
	require.Greater(t, len(certs), 0)

	// Assign certificate
	err = client.Certificate.Assign(context.Background(), "test.example.com", certs[0].ID)
	assert.NoError(t, err)
}

func TestCertificate_Assign_InvalidCertID(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	err := client.Certificate.Assign(context.Background(), "test.example.com", "nonexistent-cert")
	assert.Error(t, err)
}

func TestCertificate_Assign_EmptyHostname(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	err := client.Certificate.Assign(context.Background(), "", "cert-id")
	assert.Error(t, err)
	assert.IsType(t, &ValidationError{}, err)
}

func TestCertificate_Assign_EmptyCertID(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	err := client.Certificate.Assign(context.Background(), "test.example.com", "")
	assert.Error(t, err)
	assert.IsType(t, &ValidationError{}, err)
}

func TestCertificate_WithRetry(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	// Set failure mode
	mock.SetFailureMode("500")

	// Should fail with retries
	_, err := client.Certificate.List(context.Background())
	assert.Error(t, err)

	// Reset failure mode
	mock.SetFailureMode("none")

	// Should succeed now
	certs, err := client.Certificate.List(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, certs)
}

func TestCertificate_ContextCancellation(t *testing.T) {
	client, mock := setupTestClient(t)
	defer mock.Close()
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Certificate.List(ctx)
	assert.Error(t, err)
}

func TestCertificate_CacheTTL(t *testing.T) {
	// Create client with short cache TTL
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

	// Override cache with short TTL
	client.certCache = NewCertificateCache(100 * time.Millisecond)

	// First call
	certs1, err := client.Certificate.List(context.Background())
	require.NoError(t, err)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Second call - should fetch from API again
	certs2, err := client.Certificate.List(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, len(certs1), len(certs2))
}
