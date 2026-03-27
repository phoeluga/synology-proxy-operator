package synology

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCertificateCache_GetSet(t *testing.T) {
	cache := NewCertificateCache(5 * time.Minute)

	// Initially empty
	certs, found := cache.Get()
	assert.False(t, found)
	assert.Nil(t, certs)

	// Set certificates
	testCerts := []*Certificate{
		{ID: "cert1", Desc: "example.com"},
		{ID: "cert2", Desc: "*.example.com"},
	}
	cache.Set(testCerts)

	// Should retrieve certificates
	certs, found = cache.Get()
	assert.True(t, found)
	assert.Len(t, certs, 2)
	assert.Equal(t, "cert1", certs[0].ID)
}

func TestCertificateCache_Invalidate(t *testing.T) {
	cache := NewCertificateCache(5 * time.Minute)

	// Set certificates
	testCerts := []*Certificate{
		{ID: "cert1", Desc: "example.com"},
	}
	cache.Set(testCerts)

	// Verify cached
	_, found := cache.Get()
	assert.True(t, found)

	// Invalidate
	cache.Invalidate()

	// Should be empty
	_, found = cache.Get()
	assert.False(t, found)
}

func TestCertificateCache_TTLExpiry(t *testing.T) {
	cache := NewCertificateCache(100 * time.Millisecond)

	// Set certificates
	testCerts := []*Certificate{
		{ID: "cert1", Desc: "example.com"},
	}
	cache.Set(testCerts)

	// Should be cached
	_, found := cache.Get()
	assert.True(t, found)

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, found = cache.Get()
	assert.False(t, found)
}

func TestCertificateCache_UpdateRefreshesTTL(t *testing.T) {
	cache := NewCertificateCache(200 * time.Millisecond)

	// Set certificates
	testCerts := []*Certificate{
		{ID: "cert1", Desc: "example.com"},
	}
	cache.Set(testCerts)

	// Wait half the TTL
	time.Sleep(100 * time.Millisecond)

	// Update cache (refreshes TTL)
	cache.Set(testCerts)

	// Wait another half TTL
	time.Sleep(100 * time.Millisecond)

	// Should still be cached (total 200ms but TTL was refreshed at 100ms)
	_, found := cache.Get()
	assert.True(t, found)
}

func TestCertificateCache_ConcurrentAccess(t *testing.T) {
	cache := NewCertificateCache(5 * time.Minute)

	testCerts := []*Certificate{
		{ID: "cert1", Desc: "example.com"},
	}

	done := make(chan bool)

	// Concurrent reads and writes
	for i := 0; i < 10; i++ {
		go func() {
			cache.Get()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			cache.Set(testCerts)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		go func() {
			cache.Invalidate()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 25; i++ {
		<-done
	}

	// Should not panic
	assert.True(t, true)
}

func TestCertificateCache_EmptyList(t *testing.T) {
	cache := NewCertificateCache(5 * time.Minute)

	// Set empty list
	cache.Set([]*Certificate{})

	// Should be cached (even though empty)
	certs, found := cache.Get()
	assert.True(t, found)
	assert.Empty(t, certs)
}

func TestCertificateCache_NilList(t *testing.T) {
	cache := NewCertificateCache(5 * time.Minute)

	// Set nil list
	cache.Set(nil)

	// Should be cached
	certs, found := cache.Get()
	assert.True(t, found)
	assert.Nil(t, certs)
}
