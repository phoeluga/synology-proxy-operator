package config

import (
	"context"
	"fmt"
	
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Load loads configuration from flags, environment variables, and defaults
func Load(cmd *cobra.Command, kubeClient client.Client) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()
	
	// Bind flags to viper
	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return nil, fmt.Errorf("failed to bind flags: %w", err)
	}
	
	// Load from environment variables (with prefix)
	viper.SetEnvPrefix("SYNOLOGY_OPERATOR")
	viper.AutomaticEnv()
	
	// Load Synology config
	cfg.Synology.URL = viper.GetString("synology-url")
	cfg.Synology.SecretName = viper.GetString("synology-secret-name")
	cfg.Synology.SecretNamespace = viper.GetString("synology-secret-namespace")
	cfg.Synology.TLSVerify = viper.GetBool("tls-verify")
	cfg.Synology.CACertPath = viper.GetString("ca-cert-path")
	
	// Load Controller config
	cfg.Controller.WatchNamespaces = viper.GetString("watch-namespaces")
	cfg.Controller.DefaultACLProfile = viper.GetString("default-acl-profile")
	cfg.Controller.MaxRetries = viper.GetInt("max-retries")
	cfg.Controller.MetricsAddr = viper.GetString("metrics-addr")
	cfg.Controller.HealthProbeAddr = viper.GetString("health-probe-addr")
	
	// Load Observability config
	cfg.Observability.LogLevel = viper.GetString("log-level")
	cfg.Observability.LogFormat = viper.GetString("log-format")
	cfg.Observability.MetricsEnabled = viper.GetBool("metrics-enabled")
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	
	// Load credentials from Kubernetes Secret
	if err := loadCredentials(context.Background(), kubeClient, cfg); err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}
	
	return cfg, nil
}

// loadCredentials loads Synology credentials from Kubernetes Secret
func loadCredentials(ctx context.Context, kubeClient client.Client, cfg *Config) error {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      cfg.Synology.SecretName,
		Namespace: cfg.Synology.SecretNamespace,
	}
	
	if err := kubeClient.Get(ctx, secretKey, secret); err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", 
			cfg.Synology.SecretNamespace, cfg.Synology.SecretName, err)
	}
	
	username, ok := secret.Data["username"]
	if !ok {
		return fmt.Errorf("secret %s/%s missing 'username' key", 
			cfg.Synology.SecretNamespace, cfg.Synology.SecretName)
	}
	
	password, ok := secret.Data["password"]
	if !ok {
		return fmt.Errorf("secret %s/%s missing 'password' key", 
			cfg.Synology.SecretNamespace, cfg.Synology.SecretName)
	}
	
	cfg.Synology.UpdateCredentials(string(username), string(password))
	
	return nil
}

// SetupFlags sets up CLI flags for configuration
func SetupFlags(cmd *cobra.Command) {
	// Synology flags
	cmd.Flags().String("synology-url", "", "Synology NAS URL (HTTPS required)")
	cmd.Flags().String("synology-secret-name", "synology-credentials", "Kubernetes Secret name containing credentials")
	cmd.Flags().String("synology-secret-namespace", "default", "Kubernetes Secret namespace")
	cmd.Flags().Bool("tls-verify", true, "Verify TLS certificates")
	cmd.Flags().String("ca-cert-path", "", "Path to custom CA certificate")
	
	// Controller flags
	cmd.Flags().String("watch-namespaces", "*", "Comma-separated namespace patterns to watch")
	cmd.Flags().String("default-acl-profile", "default", "Default ACL profile name")
	cmd.Flags().Int("max-retries", 10, "Maximum API retry attempts")
	cmd.Flags().String("metrics-addr", ":8080", "Metrics endpoint address")
	cmd.Flags().String("health-probe-addr", ":8081", "Health probe endpoint address")
	
	// Observability flags
	cmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.Flags().String("log-format", "json", "Log format (json, text)")
	cmd.Flags().Bool("metrics-enabled", true, "Enable Prometheus metrics")
}
