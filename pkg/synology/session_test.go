package synology

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSessionManager_GetSession(t *testing.T) {
	logger := &testLogger{}
	sm := NewSessionManager("admin", "password", logger)

	// Initially no session
	sid, token := sm.GetSession()
	assert.Empty(t, sid)
	assert.Empty(t, token)

	// Set session
	sm.UpdateSession("test-sid", "test-token")

	// Should return session
	sid, token = sm.GetSession()
	assert.Equal(t, "test-sid", sid)
	assert.Equal(t, "test-token", token)
}

func TestSessionManager_UpdateSession(t *testing.T) {
	logger := &testLogger{}
	sm := NewSessionManager("admin", "password", logger)

	sm.UpdateSession("sid1", "token1")
	sid, token := sm.GetSession()
	assert.Equal(t, "sid1", sid)
	assert.Equal(t, "token1", token)
	assert.True(t, sm.IsValid())

	// Update with new session
	sm.UpdateSession("sid2", "token2")
	sid, token = sm.GetSession()
	assert.Equal(t, "sid2", sid)
	assert.Equal(t, "token2", token)
}

func TestSessionManager_Invalidate(t *testing.T) {
	logger := &testLogger{}
	sm := NewSessionManager("admin", "password", logger)

	sm.UpdateSession("test-sid", "test-token")
	assert.True(t, sm.IsValid())

	sm.Invalidate()
	assert.False(t, sm.IsValid())

	sid, token := sm.GetSession()
	assert.Empty(t, sid)
	assert.Empty(t, token)
}

func TestSessionManager_IsValid(t *testing.T) {
	logger := &testLogger{}
	sm := NewSessionManager("admin", "password", logger)

	// Initially invalid
	assert.False(t, sm.IsValid())

	// Valid after update
	sm.UpdateSession("test-sid", "test-token")
	assert.True(t, sm.IsValid())

	// Invalid after invalidation
	sm.Invalidate()
	assert.False(t, sm.IsValid())
}

func TestSessionManager_NeedsRefresh(t *testing.T) {
	logger := &testLogger{}
	sm := NewSessionManager("admin", "password", logger)

	// Needs refresh when no session
	assert.True(t, sm.NeedsRefresh())

	// Set session with recent timestamp
	sm.UpdateSession("test-sid", "test-token")
	assert.False(t, sm.NeedsRefresh())

	// Manually set old timestamp to simulate expiry
	sm.mu.Lock()
	sm.lastRefresh = time.Now().Add(-25 * time.Minute)
	sm.mu.Unlock()

	assert.True(t, sm.NeedsRefresh())
}

func TestSessionManager_GetCredentials(t *testing.T) {
	logger := &testLogger{}
	sm := NewSessionManager("testuser", "testpass", logger)

	username, password := sm.GetCredentials()
	assert.Equal(t, "testuser", username)
	assert.Equal(t, "testpass", password)
}

func TestSessionManager_ConcurrentAccess(t *testing.T) {
	logger := &testLogger{}
	sm := NewSessionManager("admin", "password", logger)

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			sm.GetSession()
			sm.IsValid()
			sm.NeedsRefresh()
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(n int) {
			sm.UpdateSession("sid", "token")
			if n%2 == 0 {
				sm.Invalidate()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should not panic
	assert.True(t, true)
}
