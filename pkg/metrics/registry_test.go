package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.apiCallsTotal)
	assert.NotNil(t, registry.apiCallDuration)
	assert.NotNil(t, registry.reconciliationsTotal)
	assert.NotNil(t, registry.reconciliationDuration)
	assert.NotNil(t, registry.proxyRecordsTotal)
	assert.NotNil(t, registry.certificateAssignmentsTotal)
	assert.NotNil(t, registry.errorsTotal)
	assert.NotNil(t, registry.retryAttemptsTotal)
	assert.NotNil(t, registry.circuitBreakerState)
	assert.NotNil(t, registry.cacheHitsTotal)
}

func TestRegistry_RecordAPICall(t *testing.T) {
	registry := NewRegistry()

	// Record successful API call
	registry.RecordAPICall("create", true, 100*time.Millisecond)

	// Verify counter incremented
	count := testutil.ToFloat64(registry.apiCallsTotal.WithLabelValues("create", "success"))
	assert.Equal(t, float64(1), count)

	// Record failed API call
	registry.RecordAPICall("create", false, 50*time.Millisecond)

	// Verify failure counter incremented
	failCount := testutil.ToFloat64(registry.apiCallsTotal.WithLabelValues("create", "failure"))
	assert.Equal(t, float64(1), failCount)
}

func TestRegistry_RecordReconciliation(t *testing.T) {
	registry := NewRegistry()

	// Record successful reconciliation
	registry.RecordReconciliation("default", "test-ingress", true, 200*time.Millisecond)

	// Verify counter incremented
	count := testutil.ToFloat64(registry.reconciliationsTotal.WithLabelValues("default", "success"))
	assert.Equal(t, float64(1), count)

	// Record failed reconciliation
	registry.RecordReconciliation("default", "test-ingress", false, 100*time.Millisecond)

	// Verify failure counter incremented
	failCount := testutil.ToFloat64(registry.reconciliationsTotal.WithLabelValues("default", "failure"))
	assert.Equal(t, float64(1), failCount)
}

func TestRegistry_SetProxyRecordCount(t *testing.T) {
	registry := NewRegistry()

	// Set proxy record count
	registry.SetProxyRecordCount(5)

	// Verify gauge value
	count := testutil.ToFloat64(registry.proxyRecordsTotal)
	assert.Equal(t, float64(5), count)

	// Update count
	registry.SetProxyRecordCount(10)

	// Verify updated value
	count = testutil.ToFloat64(registry.proxyRecordsTotal)
	assert.Equal(t, float64(10), count)
}

func TestRegistry_RecordCertificateAssignment(t *testing.T) {
	registry := NewRegistry()

	// Record successful assignment
	registry.RecordCertificateAssignment("example.com", "cert1", true)

	// Verify counter incremented
	count := testutil.ToFloat64(registry.certificateAssignmentsTotal.WithLabelValues("success"))
	assert.Equal(t, float64(1), count)

	// Record failed assignment
	registry.RecordCertificateAssignment("example.com", "cert1", false)

	// Verify failure counter incremented
	failCount := testutil.ToFloat64(registry.certificateAssignmentsTotal.WithLabelValues("failure"))
	assert.Equal(t, float64(1), failCount)
}

func TestRegistry_RecordError(t *testing.T) {
	registry := NewRegistry()

	// Record error
	registry.RecordError("auth", "authentication_failed")

	// Verify counter incremented
	count := testutil.ToFloat64(registry.errorsTotal.WithLabelValues("auth", "authentication_failed"))
	assert.Equal(t, float64(1), count)

	// Record another error
	registry.RecordError("api", "timeout")

	// Verify counter incremented
	count = testutil.ToFloat64(registry.errorsTotal.WithLabelValues("api", "timeout"))
	assert.Equal(t, float64(1), count)
}

func TestRegistry_RecordRetry(t *testing.T) {
	registry := NewRegistry()

	// Record retry
	registry.RecordRetry("create")

	// Verify counter incremented
	count := testutil.ToFloat64(registry.retryAttemptsTotal.WithLabelValues("create"))
	assert.Equal(t, float64(1), count)

	// Record multiple retries
	registry.RecordRetry("create")
	registry.RecordRetry("create")

	// Verify counter incremented
	count = testutil.ToFloat64(registry.retryAttemptsTotal.WithLabelValues("create"))
	assert.Equal(t, float64(3), count)
}

func TestRegistry_RecordCircuitBreakerState(t *testing.T) {
	registry := NewRegistry()

	// Record closed state
	registry.RecordCircuitBreakerState("closed")

	// Verify gauge value
	value := testutil.ToFloat64(registry.circuitBreakerState.WithLabelValues("closed"))
	assert.Equal(t, float64(1), value)

	// Record open state
	registry.RecordCircuitBreakerState("open")

	// Verify gauge values
	closedValue := testutil.ToFloat64(registry.circuitBreakerState.WithLabelValues("closed"))
	openValue := testutil.ToFloat64(registry.circuitBreakerState.WithLabelValues("open"))
	assert.Equal(t, float64(0), closedValue)
	assert.Equal(t, float64(1), openValue)
}

func TestRegistry_RecordCacheHit(t *testing.T) {
	registry := NewRegistry()

	// Record cache hit
	registry.RecordCacheHit("certificates", true)

	// Verify counter incremented
	count := testutil.ToFloat64(registry.cacheHitsTotal.WithLabelValues("certificates", "hit"))
	assert.Equal(t, float64(1), count)

	// Record cache miss
	registry.RecordCacheHit("certificates", false)

	// Verify miss counter incremented
	missCount := testutil.ToFloat64(registry.cacheHitsTotal.WithLabelValues("certificates", "miss"))
	assert.Equal(t, float64(1), missCount)
}

func TestRegistry_PrometheusRegistration(t *testing.T) {
	registry := NewRegistry()

	// Create a new Prometheus registry
	promRegistry := prometheus.NewRegistry()

	// Register all metrics
	err := promRegistry.Register(registry.apiCallsTotal)
	assert.NoError(t, err)

	err = promRegistry.Register(registry.apiCallDuration)
	assert.NoError(t, err)

	err = promRegistry.Register(registry.reconciliationsTotal)
	assert.NoError(t, err)

	err = promRegistry.Register(registry.reconciliationDuration)
	assert.NoError(t, err)

	err = promRegistry.Register(registry.proxyRecordsTotal)
	assert.NoError(t, err)

	err = promRegistry.Register(registry.certificateAssignmentsTotal)
	assert.NoError(t, err)

	err = promRegistry.Register(registry.errorsTotal)
	assert.NoError(t, err)

	err = promRegistry.Register(registry.retryAttemptsTotal)
	assert.NoError(t, err)

	err = promRegistry.Register(registry.circuitBreakerState)
	assert.NoError(t, err)

	err = promRegistry.Register(registry.cacheHitsTotal)
	assert.NoError(t, err)
}
