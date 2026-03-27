package synology

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLogger struct{}

func (l *testLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Error(msg string, err error, keysAndValues ...interface{}) {}
func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) WithValues(keysAndValues ...interface{}) Logger { return l }
func (l *testLogger) WithName(name string) Logger                    { return l }

type testMetrics struct{}

func (m *testMetrics) RecordAPIRequest(operation, status string, duration float64) {}
func (m *testMetrics) RecordCacheHit(cacheType string, hit bool)                   {}
func (m *testMetrics) IncrementActiveRequests()                                    {}
func (m *testMetrics) DecrementActiveRequests()                                    {}
func (m *testMetrics) RecordReconciliation(namespace, result string, duration float64) {}
func (m *testMetrics) RecordReconcileError(namespace, errorType string)            {}
func (m *testMetrics) RecordAPIError(operation, errorType string)                  {}
func (m *testMetrics) RecordCertificateMatch(result string)                        {}
func (m *testMetrics) SetProxyRecordCount(namespace string, count int)             {}

func TestRetryCoordinator_Execute_Success(t *testing.T) {
	logger := &testLogger{}
	metrics := &testMetrics{}
	rc := NewRetryCoordinator(3, logger, metrics)

	callCount := 0
	operation := func(ctx context.Context) error {
		callCount++
		return nil
	}

	err := rc.Execute(context.Background(), "test", operation)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRetryCoordinator_Execute_RetryableError(t *testing.T) {
	logger := &testLogger{}
	metrics := &testMetrics{}
	rc := NewRetryCoordinator(3, logger, metrics)

	callCount := 0
	operation := func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return &RateLimitError{RetryAfter: 1}
		}
		return nil
	}

	err := rc.Execute(context.Background(), "test", operation)
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestRetryCoordinator_Execute_NonRetryableError(t *testing.T) {
	logger := &testLogger{}
	metrics := &testMetrics{}
	rc := NewRetryCoordinator(3, logger, metrics)

	callCount := 0
	operation := func(ctx context.Context) error {
		callCount++
		return &AuthError{Message: "invalid credentials"}
	}

	err := rc.Execute(context.Background(), "test", operation)
	assert.Error(t, err)
	assert.Equal(t, 1, callCount)
	assert.IsType(t, &AuthError{}, err)
}

func TestRetryCoordinator_Execute_MaxRetriesExceeded(t *testing.T) {
	logger := &testLogger{}
	metrics := &testMetrics{}
	rc := NewRetryCoordinator(3, logger, metrics)

	callCount := 0
	operation := func(ctx context.Context) error {
		callCount++
		return &RateLimitError{RetryAfter: 1}
	}

	err := rc.Execute(context.Background(), "test", operation)
	assert.Error(t, err)
	assert.Equal(t, 4, callCount) // Initial + 3 retries
}

func TestRetryCoordinator_Execute_ContextCanceled(t *testing.T) {
	logger := &testLogger{}
	metrics := &testMetrics{}
	rc := NewRetryCoordinator(3, logger, metrics)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	operation := func(ctx context.Context) error {
		return &RateLimitError{RetryAfter: 1}
	}

	err := rc.Execute(ctx, "test", operation)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name    string
		attempt int
		min     time.Duration
		max     time.Duration
	}{
		{name: "first retry", attempt: 1, min: 1 * time.Second, max: 2 * time.Second},
		{name: "second retry", attempt: 2, min: 2 * time.Second, max: 4 * time.Second},
		{name: "third retry", attempt: 3, min: 4 * time.Second, max: 8 * time.Second},
		{name: "max backoff", attempt: 10, min: 30 * time.Second, max: 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := calculateBackoff(tt.attempt)
			assert.GreaterOrEqual(t, backoff, tt.min)
			assert.LessOrEqual(t, backoff, tt.max)
		})
	}
}

func TestRetryCoordinator_Execute_BackoffTiming(t *testing.T) {
	logger := &testLogger{}
	metrics := &testMetrics{}
	rc := NewRetryCoordinator(2, logger, metrics)

	callCount := 0
	start := time.Now()
	operation := func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return &TimeoutError{Operation: "test"}
		}
		return nil
	}

	err := rc.Execute(context.Background(), "test", operation)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
	// Should have at least 2 backoffs (1s + 2s = 3s minimum)
	assert.GreaterOrEqual(t, duration, 3*time.Second)
}
