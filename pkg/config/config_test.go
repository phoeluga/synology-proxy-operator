package config

import (
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: &Config{
				Synology: SynologyConfig{
					URL:             "https://synology.example.com:5001",
					SecretName:      "synology-credentials",
					SecretNamespace: "default",
					MaxRetries:      3,
					RetryDelay:      "1s",
				},
				Controller: ControllerConfig{
					WatchNamespaces:  "*",
					MetricsAddr:      ":8080",
					HealthProbeAddr:  ":8081",
					LeaderElectionID: "synology-proxy-operator",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid URL - HTTP not allowed",
			config: &Config{
				Synology: SynologyConfig{
					URL:             "http://synology.example.com",
					SecretName:      "synology-credentials",
					SecretNamespace: "default",
				},
			},
			wantErr: true,
			errMsg:  "synology URL must use HTTPS",
		},
		{
			name: "invalid URL - empty",
			config: &Config{
				Synology: SynologyConfig{
					URL:             "",
					SecretName:      "synology-credentials",
					SecretNamespace: "default",
				},
			},
			wantErr: true,
			errMsg:  "synology URL is required",
		},
		{
			name: "invalid URL - malformed",
			config: &Config{
				Synology: SynologyConfig{
					URL:             "not-a-url",
					SecretName:      "synology-credentials",
					SecretNamespace: "default",
				},
			},
			wantErr: true,
			errMsg:  "invalid synology URL",
		},
		{
			name: "missing secret name",
			config: &Config{
				Synology: SynologyConfig{
					URL:             "https://synology.example.com:5001",
					SecretName:      "",
					SecretNamespace: "default",
				},
			},
			wantErr: true,
			errMsg:  "secret name is required",
		},
		{
			name: "missing secret namespace",
			config: &Config{
				Synology: SynologyConfig{
					URL:             "https://synology.example.com:5001",
					SecretName:      "synology-credentials",
					SecretNamespace: "",
				},
			},
			wantErr: true,
			errMsg:  "secret namespace is required",
		},
		{
			name: "invalid max retries - negative",
			config: &Config{
				Synology: SynologyConfig{
					URL:             "https://synology.example.com:5001",
					SecretName:      "synology-credentials",
					SecretNamespace: "default",
					MaxRetries:      -1,
				},
			},
			wantErr: true,
			errMsg:  "max retries must be >= 0",
		},
		{
			name: "invalid max retries - too high",
			config: &Config{
				Synology: SynologyConfig{
					URL:             "https://synology.example.com:5001",
					SecretName:      "synology-credentials",
					SecretNamespace: "default",
					MaxRetries:      11,
				},
			},
			wantErr: true,
			errMsg:  "max retries must be <= 10",
		},
		{
			name: "invalid log level",
			config: &Config{
				Synology: SynologyConfig{
					URL:             "https://synology.example.com:5001",
					SecretName:      "synology-credentials",
					SecretNamespace: "default",
				},
				Logging: LoggingConfig{
					Level: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid log level",
		},
		{
			name: "invalid retry delay",
			config: &Config{
				Synology: SynologyConfig{
					URL:             "https://synology.example.com:5001",
					SecretName:      "synology-credentials",
					SecretNamespace: "default",
					RetryDelay:      "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid retry delay",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("Config.Validate() error message = %v, want %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	// Check Synology defaults
	if cfg.Synology.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries = 3, got %d", cfg.Synology.MaxRetries)
	}
	if cfg.Synology.RetryDelay != "1s" {
		t.Errorf("Expected RetryDelay = 1s, got %s", cfg.Synology.RetryDelay)
	}
	if cfg.Synology.Timeout != "30s" {
		t.Errorf("Expected Timeout = 30s, got %s", cfg.Synology.Timeout)
	}

	// Check Controller defaults
	if cfg.Controller.WatchNamespaces != "*" {
		t.Errorf("Expected WatchNamespaces = *, got %s", cfg.Controller.WatchNamespaces)
	}
	if cfg.Controller.MetricsAddr != ":8080" {
		t.Errorf("Expected MetricsAddr = :8080, got %s", cfg.Controller.MetricsAddr)
	}
	if cfg.Controller.HealthProbeAddr != ":8081" {
		t.Errorf("Expected HealthProbeAddr = :8081, got %s", cfg.Controller.HealthProbeAddr)
	}

	// Check Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected Level = info, got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Expected Format = json, got %s", cfg.Logging.Format)
	}
}
