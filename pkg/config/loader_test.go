package config

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestLoadConfig_FromFlags(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	// Create test command
	cmd := &cobra.Command{}
	AddFlags(cmd)

	// Set flag values
	cmd.Flags().Set("synology-url", "https://test.example.com:5001")
	cmd.Flags().Set("secret-name", "test-secret")
	cmd.Flags().Set("secret-namespace", "test-namespace")
	cmd.Flags().Set("max-retries", "5")
	cmd.Flags().Set("watch-namespaces", "app-*,prod-*")
	cmd.Flags().Set("log-level", "debug")

	// Load configuration
	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify values from flags
	if cfg.Synology.URL != "https://test.example.com:5001" {
		t.Errorf("Expected URL from flag, got %s", cfg.Synology.URL)
	}
	if cfg.Synology.SecretName != "test-secret" {
		t.Errorf("Expected SecretName from flag, got %s", cfg.Synology.SecretName)
	}
	if cfg.Synology.SecretNamespace != "test-namespace" {
		t.Errorf("Expected SecretNamespace from flag, got %s", cfg.Synology.SecretNamespace)
	}
	if cfg.Synology.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries = 5, got %d", cfg.Synology.MaxRetries)
	}
	if cfg.Controller.WatchNamespaces != "app-*,prod-*" {
		t.Errorf("Expected WatchNamespaces from flag, got %s", cfg.Controller.WatchNamespaces)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected Level from flag, got %s", cfg.Logging.Level)
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	// Set environment variables
	os.Setenv("SYNOLOGY_URL", "https://env.example.com:5001")
	os.Setenv("SECRET_NAME", "env-secret")
	os.Setenv("SECRET_NAMESPACE", "env-namespace")
	os.Setenv("LOG_LEVEL", "warn")
	defer func() {
		os.Unsetenv("SYNOLOGY_URL")
		os.Unsetenv("SECRET_NAME")
		os.Unsetenv("SECRET_NAMESPACE")
		os.Unsetenv("LOG_LEVEL")
	}()

	// Create test command
	cmd := &cobra.Command{}
	AddFlags(cmd)

	// Load configuration
	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify values from environment
	if cfg.Synology.URL != "https://env.example.com:5001" {
		t.Errorf("Expected URL from env, got %s", cfg.Synology.URL)
	}
	if cfg.Synology.SecretName != "env-secret" {
		t.Errorf("Expected SecretName from env, got %s", cfg.Synology.SecretName)
	}
	if cfg.Synology.SecretNamespace != "env-namespace" {
		t.Errorf("Expected SecretNamespace from env, got %s", cfg.Synology.SecretNamespace)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Expected Level from env, got %s", cfg.Logging.Level)
	}
}

func TestLoadConfig_Precedence(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	// Set environment variable
	os.Setenv("SYNOLOGY_URL", "https://env.example.com:5001")
	defer os.Unsetenv("SYNOLOGY_URL")

	// Create test command
	cmd := &cobra.Command{}
	AddFlags(cmd)

	// Set flag value (should override env)
	cmd.Flags().Set("synology-url", "https://flag.example.com:5001")

	// Load configuration
	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify flag takes precedence over env
	if cfg.Synology.URL != "https://flag.example.com:5001" {
		t.Errorf("Expected URL from flag (precedence), got %s", cfg.Synology.URL)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	// Create test command with minimal required flags
	cmd := &cobra.Command{}
	AddFlags(cmd)
	cmd.Flags().Set("synology-url", "https://test.example.com:5001")
	cmd.Flags().Set("secret-name", "test-secret")
	cmd.Flags().Set("secret-namespace", "test-namespace")

	// Load configuration
	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify defaults are applied
	if cfg.Synology.MaxRetries != 3 {
		t.Errorf("Expected default MaxRetries = 3, got %d", cfg.Synology.MaxRetries)
	}
	if cfg.Synology.RetryDelay != "1s" {
		t.Errorf("Expected default RetryDelay = 1s, got %s", cfg.Synology.RetryDelay)
	}
	if cfg.Controller.WatchNamespaces != "*" {
		t.Errorf("Expected default WatchNamespaces = *, got %s", cfg.Controller.WatchNamespaces)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default Level = info, got %s", cfg.Logging.Level)
	}
}

func TestLoadConfig_ValidationError(t *testing.T) {
	// Reset viper for clean test
	viper.Reset()

	// Create test command with invalid URL
	cmd := &cobra.Command{}
	AddFlags(cmd)
	cmd.Flags().Set("synology-url", "http://insecure.example.com")
	cmd.Flags().Set("secret-name", "test-secret")
	cmd.Flags().Set("secret-namespace", "test-namespace")

	// Load configuration - should fail validation
	_, err := LoadConfig(cmd)
	if err == nil {
		t.Error("Expected validation error for HTTP URL, got nil")
	}
}
