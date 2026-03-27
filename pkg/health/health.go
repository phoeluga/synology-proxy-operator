package health

import (
	"context"
	"time"
)

// HealthChecker manages health checks
type HealthChecker interface {
	CheckLiveness(ctx context.Context) bool
	CheckReadiness(ctx context.Context) HealthStatus
	RegisterCheck(check ReadinessCheck)
}

// ReadinessCheck interface for individual checks
type ReadinessCheck interface {
	Name() string
	Check(ctx context.Context) CheckResult
}

// HealthStatus represents overall health status
type HealthStatus struct {
	Ready     bool                   `json:"ready"`
	Live      bool                   `json:"live"`
	Message   string                 `json:"message"`
	Checks    map[string]CheckResult `json:"checks"`
	Timestamp time.Time              `json:"timestamp"`
}

// CheckResult represents individual check result
type CheckResult struct {
	Passed     bool   `json:"passed"`
	Message    string `json:"message"`
	Error      string `json:"error,omitempty"`
	StackTrace string `json:"stackTrace,omitempty"`
}

type healthChecker struct {
	checks []ReadinessCheck
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() HealthChecker {
	return &healthChecker{
		checks: make([]ReadinessCheck, 0),
	}
}

// CheckLiveness checks if the process is alive
func (h *healthChecker) CheckLiveness(ctx context.Context) bool {
	// Always true if process is running
	return true
}

// CheckReadiness checks if the service is ready to accept traffic
func (h *healthChecker) CheckReadiness(ctx context.Context) HealthStatus {
	results := make(map[string]CheckResult)
	allPassed := true

	for _, check := range h.checks {
		result := check.Check(ctx)
		results[check.Name()] = result
		if !result.Passed {
			allPassed = false
		}
	}

	message := "All checks passed"
	if !allPassed {
		message = "One or more checks failed"
	}

	return HealthStatus{
		Ready:     allPassed,
		Live:      true,
		Message:   message,
		Checks:    results,
		Timestamp: time.Now(),
	}
}

// RegisterCheck registers a readiness check
func (h *healthChecker) RegisterCheck(check ReadinessCheck) {
	h.checks = append(h.checks, check)
}
