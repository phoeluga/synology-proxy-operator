package synology

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	logger := &testLogger{}
	cb := NewCircuitBreaker(5, 10*time.Second, logger)

	assert.True(t, cb.IsHealthy())
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_RecordSuccess(t *testing.T) {
	logger := &testLogger{}
	cb := NewCircuitBreaker(5, 10*time.Second, logger)

	// Record multiple successes
	for i := 0; i < 10; i++ {
		cb.RecordSuccess()
	}

	assert.True(t, cb.IsHealthy())
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_RecordFailure_BelowThreshold(t *testing.T) {
	logger := &testLogger{}
	cb := NewCircuitBreaker(5, 10*time.Second, logger)

	// Record failures below threshold
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}

	assert.True(t, cb.IsHealthy())
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_RecordFailure_ExceedsThreshold(t *testing.T) {
	logger := &testLogger{}
	cb := NewCircuitBreaker(5, 10*time.Second, logger)

	// Record failures exceeding threshold
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	assert.False(t, cb.IsHealthy())
	assert.Equal(t, StateOpen, cb.GetState())
}

func TestCircuitBreaker_Recovery(t *testing.T) {
	logger := &testLogger{}
	cb := NewCircuitBreaker(3, 100*time.Millisecond, logger)

	// Trip the circuit breaker
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)

	// Should transition to half-open
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Record success to close circuit
	cb.RecordSuccess()
	assert.Equal(t, StateClosed, cb.GetState())
	assert.True(t, cb.IsHealthy())
}

func TestCircuitBreaker_HalfOpen_FailureReopens(t *testing.T) {
	logger := &testLogger{}
	cb := NewCircuitBreaker(3, 100*time.Millisecond, logger)

	// Trip the circuit breaker
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Wait for recovery
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Record failure in half-open state
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.GetState())
	assert.False(t, cb.IsHealthy())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	logger := &testLogger{}
	cb := NewCircuitBreaker(3, 10*time.Second, logger)

	// Trip the circuit breaker
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Reset
	cb.Reset()
	assert.Equal(t, StateClosed, cb.GetState())
	assert.True(t, cb.IsHealthy())
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	logger := &testLogger{}
	cb := NewCircuitBreaker(10, 1*time.Second, logger)

	done := make(chan bool)

	// Concurrent operations
	for i := 0; i < 20; i++ {
		go func(n int) {
			if n%2 == 0 {
				cb.RecordSuccess()
			} else {
				cb.RecordFailure()
			}
			cb.IsHealthy()
			cb.GetState()
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

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	logger := &testLogger{}
	cb := NewCircuitBreaker(5, 10*time.Second, logger)

	// Record some failures
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Record success
	cb.RecordSuccess()

	// Should still be closed
	assert.Equal(t, StateClosed, cb.GetState())

	// Can record more failures before opening
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	assert.Equal(t, StateClosed, cb.GetState())

	// One more failure should open
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.GetState())
}
