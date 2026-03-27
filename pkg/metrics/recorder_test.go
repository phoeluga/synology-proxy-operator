package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecorder_RecordAPICall(t *testing.T) {
	registry := NewRegistry()
	recorder := NewRecorder(registry)

	// Record API call
	recorder.RecordAPICall("list", true, 150*time.Millisecond)

	// Verify metric recorded
	count := testutil.ToFloat64(registry.apiCallsTotal.WithLabelValues("list", "success"))
	assert.Equal(t, float64(1), count)
}

func TestRecorder_RecordReconciliation(t *testing.T) {
	registry := NewRegistry()
	recorder := NewRecorder(registry)

	// Record reconciliation
	recorder.RecordReconciliation("default", "test-ingress", true, 250*time.Millisecond)

	// Verify metric recorded
	count := testutil.ToFloat64(registry.reconciliationsTotal.WithLabelValues("default", "success"))
	assert.Equal(t, float64(1), count)
}

func TestRecorder_SetProxyRecordCount(t *testing.T) {
	registry := NewRegistry()
	recorder := NewRecorder(registry)

	// Set count
	recorder.SetProxyRecordCount(7)

	// Verify metric set
	count := testutil.ToFloat64(registry.proxyRecordsTotal)
	assert.Equal(t, float64(7), count)
}

func TestRecorder_RecordCertificateAssignment(t *testing.T) {
	registry := NewRegistry()
	recorder := NewRecorder(registry)

	// Record assignment
	recorder.RecordCertificateAssignment("test.com", "cert1", true)

	// Verify metric recorded
	count := testutil.ToFloat64(registry.certificateAssignmentsTotal.WithLabelValues("success"))
	assert.Equal(t, float64(1), count)
}

func TestRecorder_RecordError(t *testing.T) {
	registry := NewRegistry()
	recorder := NewRecorder(registry)

	// Record error
	recorder.RecordError("synology", "connection_failed")

	// Verify metric recorded
	count := testutil.ToFloat64(registry.errorsTotal.WithLabelValues("synology", "connection_failed"))
	assert.Equal(t, float64(1), count)
}

func TestRecorder_RecordRetry(t *testing.T) {
	registry := NewRegistry()
	recorder := NewRecorder(registry)

	// Record retry
	recorder.RecordRetry("update")

	// Verify metric recorded
	count := testutil.ToFloat64(registry.retryAttemptsTotal.WithLabelValues("update"))
	assert.Equal(t, float64(1), count)
}

func TestRecorder_RecordCircuitBreakerState(t *testing.T) {
	registry := NewRegistry()
	recorder := NewRecorder(registry)

	// Record state
	recorder.RecordCircuitBreakerState("half-open")

	// Verify metric recorded
	value := testutil.ToFloat64(registry.circuitBreakerState.WithLabelValues("half-open"))
	assert.Equal(t, float64(1), value)
}

func TestRecorder_RecordCacheHit(t *testing.T) {
	registry := NewRegistry()
	recorder := NewRecorder(registry)

	// Record cache hit
	recorder.RecordCacheHit("certificates", true)

	// Verify metric recorded
	count := testutil.ToFloat64(registry.cacheHitsTotal.WithLabelValues("certificates", "hit"))
	assert.Equal(t, float64(1), count)
}

func TestRecorder_MultipleOperations(t *testing.T) {
	registry := NewRegistry()
	recorder := NewRecorder(registry)

	// Record multiple operations
	recorder.RecordAPICall("create", true, 100*time.Millisecond)
	recorder.RecordAPICall("create", true, 150*time.Millisecond)
	recorder.RecordAPICall("create", false, 50*time.Millisecond)

	// Verify counters
	successCount := testutil.ToFloat64(registry.apiCallsTotal.WithLabelValues("create", "success"))
	failureCount := testutil.ToFloat64(registry.apiCallsTotal.WithLabelValues("create", "failure"))

	assert.Equal(t, float64(2), successCount)
	assert.Equal(t, float64(1), failureCount)
}

func TestRecorder_ConcurrentAccess(t *testing.T) {
	registry := NewRegistry()
	recorder := NewRecorder(registry)

	done := make(chan bool)

	// Concurrent metric recording
	for i := 0; i < 10; i++ {
		go func() {
			recorder.RecordAPICall("test", true, 100*time.Millisecond)
			recorder.RecordRetry("test")
			recorder.RecordError("test", "test_error")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify metrics recorded correctly
	apiCount := testutil.ToFloat64(registry.apiCallsTotal.WithLabelValues("test", "success"))
	retryCount := testutil.ToFloat64(registry.retryAttemptsTotal.WithLabelValues("test"))
	errorCount := testutil.ToFloat64(registry.errorsTotal.WithLabelValues("test", "test_error"))

	assert.Equal(t, float64(10), apiCount)
	assert.Equal(t, float64(10), retryCount)
	assert.Equal(t, float64(10), errorCount)
}
