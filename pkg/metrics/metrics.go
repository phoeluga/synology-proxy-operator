package metrics

// This file contains metric name constants and documentation

const (
	// Reconciliation metric names
	MetricReconcileTotal    = "synology_operator_reconcile_total"
	MetricReconcileDuration = "synology_operator_reconcile_duration_seconds"
	MetricReconcileErrors   = "synology_operator_reconcile_errors_total"

	// API metric names
	MetricAPIRequestsTotal   = "synology_api_requests_total"
	MetricAPIRequestDuration = "synology_api_request_duration_seconds"
	MetricAPIErrorsTotal     = "synology_api_errors_total"
	MetricAPIActiveRequests  = "synology_api_active_requests"

	// Certificate metric names
	MetricCertCacheHits     = "synology_certificate_cache_hits_total"
	MetricCertCacheMisses   = "synology_certificate_cache_misses_total"
	MetricCertMatchesTotal  = "synology_certificate_matches_total"

	// Proxy record metric names
	MetricProxyRecordsTotal = "synology_operator_proxy_records_total"
)

// Metric label names
const (
	LabelNamespace  = "namespace"
	LabelResult     = "result"
	LabelErrorType  = "error_type"
	LabelOperation  = "operation"
	LabelStatus     = "status"
)

// Result values for reconciliation metrics
const (
	ResultSuccess = "success"
	ResultError   = "error"
)

// Status values for API metrics
const (
	StatusSuccess = "success"
	StatusError   = "error"
)

// Certificate match result values
const (
	MatchResultExact    = "exact"
	MatchResultWildcard = "wildcard"
	MatchResultNone     = "none"
)

// API operation names
const (
	OperationAuth           = "auth"
	OperationProxyList      = "proxy_list"
	OperationProxyGet       = "proxy_get"
	OperationProxyCreate    = "proxy_create"
	OperationProxyUpdate    = "proxy_update"
	OperationProxyDelete    = "proxy_delete"
	OperationCertList       = "cert_list"
	OperationCertAssign     = "cert_assign"
	OperationACLList        = "acl_list"
	OperationACLGet         = "acl_get"
)

// Error type values
const (
	ErrorTypeConnection   = "connection_error"
	ErrorTypeTimeout      = "timeout"
	ErrorTypeAuth         = "auth_error"
	ErrorTypePermission   = "permission_error"
	ErrorTypeNotFound     = "not_found"
	ErrorTypeRateLimit    = "rate_limit"
	ErrorTypeServer       = "server_error"
	ErrorTypeValidation   = "validation_error"
	ErrorTypeConfig       = "config_error"
	ErrorTypeUnknown      = "unknown"
)

// MetricDefinitions provides documentation for all metrics
var MetricDefinitions = map[string]string{
	MetricReconcileTotal: `
		Counter tracking total number of reconciliations.
		Labels: namespace, result (success/error)
		Use: Monitor reconciliation activity and success rate
	`,
	MetricReconcileDuration: `
		Histogram tracking reconciliation duration in seconds.
		Labels: namespace
		Buckets: [0.1, 0.5, 1, 2, 5, 10, 30]
		Use: Monitor reconciliation performance and identify slow operations
	`,
	MetricReconcileErrors: `
		Counter tracking total number of reconciliation errors.
		Labels: namespace, error_type
		Use: Monitor error patterns and troubleshoot issues
	`,
	MetricAPIRequestsTotal: `
		Counter tracking total number of Synology API requests.
		Labels: operation, status (success/error)
		Use: Monitor API usage and success rate
	`,
	MetricAPIRequestDuration: `
		Histogram tracking Synology API request duration in seconds.
		Labels: operation
		Buckets: [0.1, 0.5, 1, 2, 5, 10]
		Use: Monitor API performance and identify slow operations
	`,
	MetricAPIErrorsTotal: `
		Counter tracking total number of Synology API errors.
		Labels: operation, error_type
		Use: Monitor API error patterns and troubleshoot connectivity issues
	`,
	MetricAPIActiveRequests: `
		Gauge tracking number of active Synology API requests.
		Use: Monitor concurrent API usage and detect potential bottlenecks
	`,
	MetricCertCacheHits: `
		Counter tracking total number of certificate cache hits.
		Use: Monitor cache effectiveness
	`,
	MetricCertCacheMisses: `
		Counter tracking total number of certificate cache misses.
		Use: Monitor cache effectiveness and tune TTL
	`,
	MetricCertMatchesTotal: `
		Counter tracking total number of certificate matches by result type.
		Labels: result (exact/wildcard/none)
		Use: Monitor certificate matching patterns
	`,
	MetricProxyRecordsTotal: `
		Gauge tracking total number of managed proxy records.
		Labels: namespace
		Use: Monitor operator workload and resource usage
	`,
}

// PrometheusQueries provides example Prometheus queries
var PrometheusQueries = map[string]string{
	"reconciliation_rate": `
		rate(synology_operator_reconcile_total[5m])
	`,
	"reconciliation_error_rate": `
		rate(synology_operator_reconcile_total{result="error"}[5m])
	`,
	"reconciliation_success_rate": `
		rate(synology_operator_reconcile_total{result="success"}[5m]) / 
		rate(synology_operator_reconcile_total[5m])
	`,
	"reconciliation_duration_p95": `
		histogram_quantile(0.95, 
			rate(synology_operator_reconcile_duration_seconds_bucket[5m]))
	`,
	"reconciliation_duration_p99": `
		histogram_quantile(0.99, 
			rate(synology_operator_reconcile_duration_seconds_bucket[5m]))
	`,
	"api_request_rate": `
		rate(synology_api_requests_total[5m])
	`,
	"api_error_rate": `
		rate(synology_api_errors_total[5m])
	`,
	"api_duration_p95": `
		histogram_quantile(0.95, 
			rate(synology_api_request_duration_seconds_bucket[5m]))
	`,
	"cache_hit_rate": `
		rate(synology_certificate_cache_hits_total[5m]) / 
		(rate(synology_certificate_cache_hits_total[5m]) + 
		 rate(synology_certificate_cache_misses_total[5m]))
	`,
	"total_proxy_records": `
		sum(synology_operator_proxy_records_total)
	`,
	"proxy_records_by_namespace": `
		synology_operator_proxy_records_total
	`,
}
