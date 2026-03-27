package health

import (
	"context"
	"testing"
	"time"
)

func TestHealthChecker_Liveness(t *testing.T) {
	checker := NewHealthChecker()
	ctx := context.Background()

	// Liveness should always return true
	if !checker.CheckLiveness(ctx) {
		t.Error("Expected liveness check to return true")
	}
}

func TestHealthChecker_ReadinessNoChecks(t *testing.T) {
	checker := NewHealthChecker()
	ctx := context.Background()

	status := checker.CheckReadiness(ctx)

	// With no checks registered, should be ready
	if !status.Ready {
		t.Error("Expected readiness to be true with no checks")
	}
	if !status.Live {
		t.Error("Expected live to be true")
	}
	if status.Message != "All checks passed" {
		t.Errorf("Expected message 'All checks passed', got %s", status.Message)
	}
	if len(status.Checks) != 0 {
		t.Errorf("Expected 0 checks, got %d", len(status.Checks))
	}
}

func TestHealthChecker_ReadinessWithPassingChecks(t *testing.T) {
	checker := NewHealthChecker()
	ctx := context.Background()

	// Register passing checks
	checker.RegisterCheck(&mockCheck{name: "check1", pass: true})
	checker.RegisterCheck(&mockCheck{name: "check2", pass: true})

	status := checker.CheckReadiness(ctx)

	// All checks pass, should be ready
	if !status.Ready {
		t.Error("Expected readiness to be true with all checks passing")
	}
	if status.Message != "All checks passed" {
		t.Errorf("Expected message 'All checks passed', got %s", status.Message)
	}
	if len(status.Checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(status.Checks))
	}

	// Verify individual check results
	if !status.Checks["check1"].Passed {
		t.Error("Expected check1 to pass")
	}
	if !status.Checks["check2"].Passed {
		t.Error("Expected check2 to pass")
	}
}

func TestHealthChecker_ReadinessWithFailingCheck(t *testing.T) {
	checker := NewHealthChecker()
	ctx := context.Background()

	// Register one passing and one failing check
	checker.RegisterCheck(&mockCheck{name: "check1", pass: true})
	checker.RegisterCheck(&mockCheck{name: "check2", pass: false, message: "check failed"})

	status := checker.CheckReadiness(ctx)

	// One check fails, should not be ready
	if status.Ready {
		t.Error("Expected readiness to be false with failing check")
	}
	if status.Message != "One or more checks failed" {
		t.Errorf("Expected message 'One or more checks failed', got %s", status.Message)
	}
	if len(status.Checks) != 2 {
		t.Errorf("Expected 2 checks, got %d", len(status.Checks))
	}

	// Verify individual check results
	if !status.Checks["check1"].Passed {
		t.Error("Expected check1 to pass")
	}
	if status.Checks["check2"].Passed {
		t.Error("Expected check2 to fail")
	}
	if status.Checks["check2"].Message != "check failed" {
		t.Errorf("Expected check2 message 'check failed', got %s", status.Checks["check2"].Message)
	}
}

func TestHealthChecker_RegisterCheck(t *testing.T) {
	checker := NewHealthChecker()
	ctx := context.Background()

	// Initially no checks
	status := checker.CheckReadiness(ctx)
	if len(status.Checks) != 0 {
		t.Errorf("Expected 0 checks initially, got %d", len(status.Checks))
	}

	// Register a check
	checker.RegisterCheck(&mockCheck{name: "test", pass: true})

	// Should now have 1 check
	status = checker.CheckReadiness(ctx)
	if len(status.Checks) != 1 {
		t.Errorf("Expected 1 check after registration, got %d", len(status.Checks))
	}
}

func TestHealthChecker_Timestamp(t *testing.T) {
	checker := NewHealthChecker()
	ctx := context.Background()

	before := time.Now()
	status := checker.CheckReadiness(ctx)
	after := time.Now()

	// Timestamp should be between before and after
	if status.Timestamp.Before(before) || status.Timestamp.After(after) {
		t.Error("Timestamp not within expected range")
	}
}

func TestHealthChecker_ContextTimeout(t *testing.T) {
	checker := NewHealthChecker()
	
	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Register slow check
	checker.RegisterCheck(&slowCheck{delay: 100 * time.Millisecond})

	// Check should still complete (though the slow check might not finish)
	status := checker.CheckReadiness(ctx)
	
	// Status should be returned even if context times out
	if status.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp even with timeout")
	}
}

// mockCheck is a test implementation of ReadinessCheck
type mockCheck struct {
	name    string
	pass    bool
	message string
}

func (c *mockCheck) Name() string {
	return c.name
}

func (c *mockCheck) Check(ctx context.Context) CheckResult {
	return CheckResult{
		Passed:  c.pass,
		Message: c.message,
	}
}

// slowCheck simulates a slow health check
type slowCheck struct {
	delay time.Duration
}

func (c *slowCheck) Name() string {
	return "slow"
}

func (c *slowCheck) Check(ctx context.Context) CheckResult {
	select {
	case <-time.After(c.delay):
		return CheckResult{Passed: true, Message: "completed"}
	case <-ctx.Done():
		return CheckResult{Passed: false, Message: "timeout"}
	}
}
