package metrics

// RecordReconciliation records a reconciliation operation
func (r *Registry) RecordReconciliation(namespace, result string, duration float64) {
	r.reconcileTotal.WithLabelValues(namespace, result).Inc()
	r.reconcileDuration.WithLabelValues(namespace).Observe(duration)
}

// RecordReconciliationError records a reconciliation error
func (r *Registry) RecordReconciliationError(namespace, errorType string) {
	r.reconcileErrors.WithLabelValues(namespace, errorType).Inc()
}

// RecordReconcileError is an alias for RecordReconciliationError for interface compatibility
func (r *Registry) RecordReconcileError(namespace, errorType string) {
	r.RecordReconciliationError(namespace, errorType)
}

// RecordAPIRequest records a Synology API request
func (r *Registry) RecordAPIRequest(operation, status string, duration float64) {
	r.apiRequestsTotal.WithLabelValues(operation, status).Inc()
	r.apiRequestDuration.WithLabelValues(operation).Observe(duration)
}

// RecordAPIError records a Synology API error
func (r *Registry) RecordAPIError(operation, errorType string) {
	r.apiErrorsTotal.WithLabelValues(operation, errorType).Inc()
}

// IncrementActiveRequests increments the active API requests gauge
func (r *Registry) IncrementActiveRequests() {
	r.apiActiveRequests.Inc()
}

// DecrementActiveRequests decrements the active API requests gauge
func (r *Registry) DecrementActiveRequests() {
	r.apiActiveRequests.Dec()
}

// RecordCacheHit records a certificate cache hit or miss
func (r *Registry) RecordCacheHit(cacheType string, hit bool) {
	if hit {
		r.certCacheHits.Inc()
	} else {
		r.certCacheMisses.Inc()
	}
}

// RecordCertificateMatch records a certificate match result
// result should be one of: "exact", "wildcard", "none"
func (r *Registry) RecordCertificateMatch(result string) {
	r.certMatchesTotal.WithLabelValues(result).Inc()
}

// SetProxyRecordCount sets the proxy record count for a namespace
func (r *Registry) SetProxyRecordCount(namespace string, count int) {
	r.proxyRecordsTotal.WithLabelValues(namespace).Set(float64(count))
}

// IncrementProxyRecordCount increments the proxy record count for a namespace
func (r *Registry) IncrementProxyRecordCount(namespace string) {
	r.proxyRecordsTotal.WithLabelValues(namespace).Inc()
}

// DecrementProxyRecordCount decrements the proxy record count for a namespace
func (r *Registry) DecrementProxyRecordCount(namespace string) {
	r.proxyRecordsTotal.WithLabelValues(namespace).Dec()
}

// ClassifyError classifies an error into a type for metrics
func ClassifyError(err error) string {
	if err == nil {
		return "none"
	}

	errStr := err.Error()

	// API errors
	if contains(errStr, "connection refused") || contains(errStr, "connection reset") {
		return "connection_error"
	}
	if contains(errStr, "timeout") || contains(errStr, "deadline exceeded") {
		return "timeout"
	}
	if contains(errStr, "401") || contains(errStr, "unauthorized") {
		return "auth_error"
	}
	if contains(errStr, "403") || contains(errStr, "forbidden") {
		return "permission_error"
	}
	if contains(errStr, "404") || contains(errStr, "not found") {
		return "not_found"
	}
	if contains(errStr, "429") || contains(errStr, "rate limit") {
		return "rate_limit"
	}
	if contains(errStr, "500") || contains(errStr, "internal server error") {
		return "server_error"
	}

	// Validation errors
	if contains(errStr, "invalid") || contains(errStr, "validation") {
		return "validation_error"
	}

	// Configuration errors
	if contains(errStr, "config") || contains(errStr, "configuration") {
		return "config_error"
	}

	// Default
	return "unknown"
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
