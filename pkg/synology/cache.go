package synology

import (
	"sync"
	"time"
)

// CertificateCache caches certificates with TTL
type CertificateCache struct {
	mu           sync.RWMutex
	certificates []*Certificate
	cachedAt     time.Time
	ttl          time.Duration
}

// NewCertificateCache creates a new certificate cache
func NewCertificateCache(ttl time.Duration) *CertificateCache {
	return &CertificateCache{
		ttl: ttl,
	}
}

// Get returns cached certificates if valid
func (cc *CertificateCache) Get() ([]*Certificate, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	if cc.certificates == nil {
		return nil, false
	}

	if time.Since(cc.cachedAt) > cc.ttl {
		return nil, false // Cache expired
	}

	return cc.certificates, true
}

// Set updates the cache
func (cc *CertificateCache) Set(certs []*Certificate) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.certificates = certs
	cc.cachedAt = time.Now()
}

// Invalidate clears the cache
func (cc *CertificateCache) Invalidate() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.certificates = nil
}

// IsValid checks if cache is valid
func (cc *CertificateCache) IsValid() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	if cc.certificates == nil {
		return false
	}

	return time.Since(cc.cachedAt) <= cc.ttl
}

// Age returns the age of the cache
func (cc *CertificateCache) Age() time.Duration {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	if cc.certificates == nil {
		return 0
	}

	return time.Since(cc.cachedAt)
}
