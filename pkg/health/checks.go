package health

import (
	"context"

	"github.com/phoeluga/synology-proxy-operator/pkg/config"
)

// ConfigCheck verifies configuration is valid
type ConfigCheck struct {
	Config *config.Config
}

// Name returns the check name
func (c *ConfigCheck) Name() string {
	return "config"
}

// Check validates the configuration
func (c *ConfigCheck) Check(ctx context.Context) CheckResult {
	if c.Config == nil {
		return CheckResult{
			Passed:  false,
			Message: "Configuration not loaded",
		}
	}

	if err := c.Config.Validate(); err != nil {
		return CheckResult{
			Passed:  false,
			Message: "Configuration validation failed",
			Error:   err.Error(),
		}
	}

	return CheckResult{
		Passed:  true,
		Message: "Configuration is valid",
	}
}

// SynologyCheck verifies Synology API is reachable
// Note: Requires SynologyClient from Unit 2
type SynologyCheck struct {
	// TODO: Add SynologyClient field when Unit 2 is implemented
}

// Name returns the check name
func (c *SynologyCheck) Name() string {
	return "synology"
}

// Check validates Synology API connectivity
func (c *SynologyCheck) Check(ctx context.Context) CheckResult {
	// TODO: Implement when SynologyClient is available (Unit 2)
	// For now, return placeholder
	return CheckResult{
		Passed:  true,
		Message: "Synology check not yet implemented (requires Unit 2)",
	}
}

// KubernetesCheck verifies Kubernetes API is accessible
type KubernetesCheck struct {
	// TODO: Add Kubernetes client field
}

// Name returns the check name
func (c *KubernetesCheck) Name() string {
	return "kubernetes"
}

// Check validates Kubernetes API connectivity
func (c *KubernetesCheck) Check(ctx context.Context) CheckResult {
	// TODO: Implement Kubernetes API check
	// Try to list namespaces to verify API access
	return CheckResult{
		Passed:  true,
		Message: "Kubernetes API check not yet implemented",
	}
}
