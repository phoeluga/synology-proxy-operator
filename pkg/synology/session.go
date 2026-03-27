package synology

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SessionManager manages Synology API session
type SessionManager struct {
	mu        sync.RWMutex
	sid       string
	synoToken string
	createdAt time.Time
	isValid   bool
	username  string
	password  string
	authFunc  func(context.Context, string, string) (*Session, error)
	logger    Logger
}

// NewSessionManager creates a new session manager
func NewSessionManager(username, password string, logger Logger) *SessionManager {
	return &SessionManager{
		username: username,
		password: password,
		isValid:  false,
		logger:   logger,
	}
}

// SetAuthFunc sets the authentication function
func (sm *SessionManager) SetAuthFunc(authFunc func(context.Context, string, string) (*Session, error)) {
	sm.authFunc = authFunc
}

// GetSession returns the current session (read lock)
func (sm *SessionManager) GetSession() (sid, token string, valid bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sid, sm.synoToken, sm.isValid
}

// RefreshSession refreshes the session (write lock with double-check)
func (sm *SessionManager) RefreshSession(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Double-check: another goroutine may have already refreshed
	if sm.isValid && time.Since(sm.createdAt) < 1*time.Second {
		sm.logger.Debug("Session already refreshed by another goroutine")
		return nil
	}

	sm.logger.Info("Refreshing session")

	if sm.authFunc == nil {
		return fmt.Errorf("auth function not set")
	}

	// Call auth function
	session, err := sm.authFunc(ctx, sm.username, sm.password)
	if err != nil {
		sm.isValid = false
		return fmt.Errorf("session refresh failed: %w", err)
	}

	// Update session
	sm.sid = session.SID
	sm.synoToken = session.SynoToken
	sm.createdAt = time.Now()
	sm.isValid = true

	sm.logger.Info("Session refreshed successfully")
	return nil
}

// MarkInvalid marks the session as invalid
func (sm *SessionManager) MarkInvalid() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.isValid = false
	sm.logger.Info("Session marked as invalid")
}

// IsValid checks if session is valid
func (sm *SessionManager) IsValid() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isValid
}

// UpdateCredentials updates username and password
func (sm *SessionManager) UpdateCredentials(username, password string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.username = username
	sm.password = password
	sm.isValid = false // Force re-authentication
	sm.logger.Info("Credentials updated, session invalidated")
}
