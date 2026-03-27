package config

import (
	"fmt"
	"net/url"
	"strings"
)

// Config aggregates all operator configuration
type Config struct {
	Synology      SynologyConfig
	Controller    ControllerConfig
	Observability ObservabilityConfig
}

// SynologyConfig contains Synology NAS connection configuration
type SynologyConfig struct {
	URL             string
	SecretName      string
	SecretNamespace string
	TLSVerify       bool
	CACertPath      string
	
	// Credentials loaded from Secret (not exported)
	username string
	password string
}

// ControllerConfig contains controller behavior configuration
type ControllerConfig struct {
	WatchNamespaces   string
	DefaultACLProfile string
	MaxRetries        int
	MetricsAddr       string
	HealthProbeAddr   string
}

// ObservabilityConfig contains logging and metrics configuration
type ObservabilityConfig struct {
	LogLevel       string
	LogFormat      string
	MetricsEnabled bool
}

// Validate validates the entire configuration
func (c *Config) Validate() error {
	var errs []string
	
	// Validate Synology config
	if err := c.Synology.Validate(); err != nil {
		errs = append(errs, fmt.Sprintf("synology: %v", err))
	}
	
	// Validate Controller config
	if err := c.Controller.Validate(); err != nil {
		errs = append(errs, fmt.Sprintf("controller: %v", err))
	}
	
	// Validate Observability config
	if err := c.Observability.Validate(); err != nil {
		errs = append(errs, fmt.Sprintf("observability: %v", err))
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errs, "; "))
	}
	
	return nil
}

// Validate validates Synology configuration
func (sc *SynologyConfig) Validate() error {
	// Validate URL
	if sc.URL == "" {
		return fmt.Errorf("URL is required")
	}
	
	parsedURL, err := url.Parse(sc.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use HTTPS, got %s", parsedURL.Scheme)
	}
	
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must include host")
	}
	
	// Validate Secret
	if sc.SecretName == "" {
		return fmt.Errorf("secret name is required")
	}
	
	if sc.SecretNamespace == "" {
		return fmt.Errorf("secret namespace is required")
	}
	
	// Validate CA cert path if provided
	if sc.CACertPath != "" {
		// Note: File existence check would happen at runtime
	}
	
	return nil
}

// Validate validates Controller configuration
func (cc *ControllerConfig) Validate() error {
	// Validate namespace patterns
	if cc.WatchNamespaces != "" && cc.WatchNamespaces != "*" {
		patterns := strings.Split(cc.WatchNamespaces, ",")
		for _, pattern := range patterns {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				return fmt.Errorf("namespace pattern cannot be empty")
			}
			// Basic validation - no invalid characters
			if strings.ContainsAny(pattern, " \t\n") {
				return fmt.Errorf("invalid namespace pattern: %s", pattern)
			}
		}
	}
	
	// Validate ACL profile
	if cc.DefaultACLProfile == "" {
		return fmt.Errorf("default ACL profile is required")
	}
	
	// Validate max retries
	if cc.MaxRetries < 1 || cc.MaxRetries > 100 {
		return fmt.Errorf("max retries must be between 1 and 100, got %d", cc.MaxRetries)
	}
	
	// Validate addresses (basic check)
	if cc.MetricsAddr == "" {
		return fmt.Errorf("metrics address is required")
	}
	
	if cc.HealthProbeAddr == "" {
		return fmt.Errorf("health probe address is required")
	}
	
	return nil
}

// Validate validates Observability configuration
func (oc *ObservabilityConfig) Validate() error {
	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	
	if !validLevels[oc.LogLevel] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", oc.LogLevel)
	}
	
	// Validate log format
	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	
	if !validFormats[oc.LogFormat] {
		return fmt.Errorf("invalid log format: %s (must be json or text)", oc.LogFormat)
	}
	
	return nil
}

// UpdateCredentials updates Synology credentials (called by SecretWatcher)
func (sc *SynologyConfig) UpdateCredentials(username, password string) {
	sc.username = username
	sc.password = password
}

// GetCredentials returns Synology credentials
func (sc *SynologyConfig) GetCredentials() (username, password string) {
	return sc.username, sc.password
}

// DefaultConfig returns configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Synology: SynologyConfig{
			TLSVerify: true,
		},
		Controller: ControllerConfig{
			WatchNamespaces:   "*",
			DefaultACLProfile: "default",
			MaxRetries:        10,
			MetricsAddr:       ":8080",
			HealthProbeAddr:   ":8081",
		},
		Observability: ObservabilityConfig{
			LogLevel:       "info",
			LogFormat:      "json",
			MetricsEnabled: true,
		},
	}
}
