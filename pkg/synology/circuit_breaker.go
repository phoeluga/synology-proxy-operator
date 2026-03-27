package synology

import (
	"sync/atomic"
	"time"
)

// Circuit breaker states
const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half-open"
)

// CircuitBreaker implements failure threshold pattern
type CircuitBreaker struct {
	failureCount    atomic.Int32
	threshold       int32
	recoveryTimeout time.Duration
	lastCheckTime   atomic.Value // time.Time
	logger          Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(threshold int, recoveryTimeout time.Duration, logger Logger) *CircuitBreaker {
	cb := &CircuitBreaker{
		threshold:       int32(threshold),
		recoveryTimeout: recoveryTimeout,
		logger:          logger,
	}
	cb.lastCheckTime.Store(time.Now())
	return cb
}

// RecordSuccess resets failure count
func (cb *CircuitBreaker) RecordSuccess() {
	oldCount := cb.failureCount.Swap(0)
	if oldCount >= cb.threshold {
		cb.logger.Info("Circuit breaker recovered", "previous_failures", oldCount)
	}
}

// RecordFailure increments failure count
func (cb *CircuitBreaker) RecordFailure() {
	newCount := cb.failureCount.Add(1)
	if newCount == cb.threshold {
		cb.logger.Warn("Circuit breaker opened", "failures", newCount)
	}
}

// IsHealthy checks if circuit breaker is healthy
func (cb *CircuitBreaker) IsHealthy() bool {
	return cb.failureCount.Load() < cb.threshold
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() string {
	count := cb.failureCount.Load()
	if count < cb.threshold {
		return StateClosed
	}
	
	lastCheck := cb.lastCheckTime.Load().(time.Time)
	if time.Since(lastCheck) > cb.recoveryTimeout {
		return StateHalfOpen
	}
	
	return StateOpen
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.failureCount.Store(0)
	cb.lastCheckTime.Store(time.Now())
	cb.logger.Info("Circuit breaker reset")
}

// ShouldAttemptRecovery checks if recovery attempt is allowed
func (cb *CircuitBreaker) ShouldAttemptRecovery() bool {
	if cb.IsHealthy() {
		return false
	}

	lastCheck := cb.lastCheckTime.Load().(time.Time)
	if time.Since(lastCheck) > cb.recoveryTimeout {
		cb.lastCheckTime.Store(time.Now())
		return true
	}

	return false
}

// GetFailureCount returns current failure count
func (cb *CircuitBreaker) GetFailureCount() int {
	return int(cb.failureCount.Load())
}
