package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/phoeluga/synology-proxy-operator/pkg/config"
	"github.com/phoeluga/synology-proxy-operator/pkg/logging"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "synology-proxy-operator",
		Short: "Kubernetes operator for Synology reverse proxy management",
		Long:  `Synology Proxy Operator automates reverse proxy record management on Synology NAS based on Kubernetes Ingress resources.`,
		RunE:  run,
	}

	// Setup configuration flags
	config.SetupFlags(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Initialize logger first (with defaults until config is loaded)
	logger, err := logging.NewLogger("info", "json")
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("Starting Synology Proxy Operator",
		"version", version,
		"commit", commit,
		"date", date,
	)

	// Create Kubernetes client
	kubeConfig, err := ctrlconfig.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	kubeClient, err := client.New(kubeConfig, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Load configuration
	logger.Info("Loading configuration")
	cfg, err := config.Load(cmd, kubeClient)
	if err != nil {
		logger.Error("Failed to load configuration", err)
		return err
	}

	// Reinitialize logger with configured log level and format
	logger, err = logging.NewLogger(cfg.Observability.LogLevel, cfg.Observability.LogFormat)
	if err != nil {
		return fmt.Errorf("failed to reinitialize logger: %w", err)
	}

	logger.Info("Configuration loaded successfully",
		"synology_url", cfg.Synology.URL,
		"watch_namespaces", cfg.Controller.WatchNamespaces,
		"log_level", cfg.Observability.LogLevel,
	)

	// TODO: Initialize remaining components
	// - Secret Watcher (for credential reload)
	// - Health Checker (with readiness checks)
	// - Health check HTTP server
	// - Namespace Filter

	logger.Info("Operator started successfully")

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Wait for shutdown signal
	sig := <-sigCh
	logger.Info("Shutdown signal received", "signal", sig.String())

	// Graceful shutdown
	return gracefulShutdown(logger)
}

func gracefulShutdown(logger logging.Logger) error {
	logger.Info("Starting graceful shutdown")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// TODO: Stop components
	// - Stop Secret Watcher
	// - Stop health check HTTP server
	// - Flush logger

	select {
	case <-ctx.Done():
		logger.Warn("Shutdown timeout exceeded, forcing exit")
		return fmt.Errorf("shutdown timeout exceeded")
	default:
		logger.Info("Shutdown complete")
		return nil
	}
}
