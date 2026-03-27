package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Registry holds all Prometheus metrics for the operator
type Registry struct {
	// Reconciliation metrics
	reconcileTotal    *prometheus.CounterVec
	reconcileDuration *prometheus.HistogramVec
	reconcileErrors   *prometheus.CounterVec

	// API metrics
	apiRequestsTotal   *prometheus.CounterVec
	apiRequestDuration *prometheus.HistogramVec
	apiErrorsTotal     *prometheus.CounterVec
	apiActiveRequests  prometheus.Gauge

	// Certificate metrics
	certCacheHits    prometheus.Counter
	certCacheMisses  prometheus.Counter
	certMatchesTotal *prometheus.CounterVec

	// Proxy record metrics
	proxyRecordsTotal *prometheus.GaugeVec
}

// NewRegistry creates and registers all metrics
func NewRegistry() *Registry {
	r := &Registry{
		// Reconciliation metrics
		reconcileTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "synology_operator_reconcile_total",
				Help: "Total number of reconciliations",
			},
			[]string{"namespace", "result"},
		),
		reconcileDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "synology_operator_reconcile_duration_seconds",
				Help:    "Reconciliation duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
			},
			[]string{"namespace"},
		),
		reconcileErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "synology_operator_reconcile_errors_total",
				Help: "Total number of reconciliation errors",
			},
			[]string{"namespace", "error_type"},
		),

		// API metrics
		apiRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "synology_api_requests_total",
				Help: "Total number of Synology API requests",
			},
			[]string{"operation", "status"},
		),
		apiRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "synology_api_request_duration_seconds",
				Help:    "Synology API request duration in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
			},
			[]string{"operation"},
		),
		apiErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "synology_api_errors_total",
				Help: "Total number of Synology API errors",
			},
			[]string{"operation", "error_type"},
		),
		apiActiveRequests: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "synology_api_active_requests",
				Help: "Number of active Synology API requests",
			},
		),

		// Certificate metrics
		certCacheHits: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "synology_certificate_cache_hits_total",
				Help: "Total number of certificate cache hits",
			},
		),
		certCacheMisses: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "synology_certificate_cache_misses_total",
				Help: "Total number of certificate cache misses",
			},
		),
		certMatchesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "synology_certificate_matches_total",
				Help: "Total number of certificate matches by result type",
			},
			[]string{"result"},
		),

		// Proxy record metrics
		proxyRecordsTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "synology_operator_proxy_records_total",
				Help: "Total number of managed proxy records",
			},
			[]string{"namespace"},
		),
	}

	// Register all metrics with controller-runtime's registry
	metrics.Registry.MustRegister(
		r.reconcileTotal,
		r.reconcileDuration,
		r.reconcileErrors,
		r.apiRequestsTotal,
		r.apiRequestDuration,
		r.apiErrorsTotal,
		r.apiActiveRequests,
		r.certCacheHits,
		r.certCacheMisses,
		r.certMatchesTotal,
		r.proxyRecordsTotal,
	)

	return r
}

// Reset resets all metrics (useful for testing)
func (r *Registry) Reset() {
	r.reconcileTotal.Reset()
	r.reconcileDuration.Reset()
	r.reconcileErrors.Reset()
	r.apiRequestsTotal.Reset()
	r.apiRequestDuration.Reset()
	r.apiErrorsTotal.Reset()
	r.apiActiveRequests.Set(0)
	r.certCacheHits.Add(0) // Counter doesn't have Reset, this is a no-op
	r.certCacheMisses.Add(0)
	r.certMatchesTotal.Reset()
	r.proxyRecordsTotal.Reset()
}
