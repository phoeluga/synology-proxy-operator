package synology

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/time/rate"
)

// Logger interface for structured logging
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, err error, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	WithValues(keysAndValues ...interface{}) Logger
	WithName(name string) Logger
}

// MetricsRegistry interface for recording metrics
type MetricsRegistry interface {
	RecordAPIRequest(operation, status string, duration float64)
	RecordCacheHit(cacheType string, hit bool)
	IncrementActiveRequests()
	DecrementActiveRequests()
	RecordReconciliation(namespace, result string, duration float64)
	RecordReconcileError(namespace, errorType string)
	RecordAPIError(operation, errorType string)
	RecordCertificateMatch(result string)
	SetProxyRecordCount(namespace string, count int)
}

// Client is the main Synology API client
type Client struct {
	baseURL          string
	httpClient       *http.Client
	sessionManager   *SessionManager
	rateLimiter      *rate.Limiter
	circuitBreaker   *CircuitBreaker
	retryCoordinator *RetryCoordinator
	certCache        *CertificateCache
	logger           Logger
	metrics          MetricsRegistry
	
	// API operations
	Proxy       *ProxyOperations
	Certificate *CertificateOperations
	ACL         *ACLOperations
}

// Config holds client configuration
type Config struct {
	BaseURL        string
	Username       string
	Password       string
	TLSVerify      bool
	CACertPath     string
	MaxRetries     int
	RateLimit      float64
	RateBurst      int
	RequestTimeout time.Duration
}

// New creates a new Synology API client
func New(config Config, logger Logger, metrics MetricsRegistry) (*Client, error) {
	// Validate config
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	// Create TLS config
	tlsConfig, err := createTLSConfig(config)
	if err != nil {
		return nil, err
	}

	// Create HTTP client with connection pooling
	httpClient := &http.Client{
		Timeout: config.RequestTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			MaxConnsPerHost:     10,
			IdleConnTimeout:     90 * time.Second,
			TLSClientConfig:     tlsConfig,
		},
	}

	// Create rate limiter
	rateLimiter := rate.NewLimiter(rate.Limit(config.RateLimit), config.RateBurst)

	// Create circuit breaker
	circuitBreaker := NewCircuitBreaker(5, 60*time.Second, logger)

	// Create certificate cache
	certCache := NewCertificateCache(5 * time.Minute)

	// Create session manager
	sessionManager := NewSessionManager(config.Username, config.Password, logger)

	// Create retry coordinator
	retryCoordinator := NewRetryCoordinator(config.MaxRetries, logger, metrics)

	client := &Client{
		baseURL:          config.BaseURL,
		httpClient:       httpClient,
		sessionManager:   sessionManager,
		rateLimiter:      rateLimiter,
		circuitBreaker:   circuitBreaker,
		retryCoordinator: retryCoordinator,
		certCache:        certCache,
		logger:           logger,
		metrics:          metrics,
	}

	// Initialize API operations
	client.Proxy = &ProxyOperations{client: client}
	client.Certificate = &CertificateOperations{client: client}
	client.ACL = &ACLOperations{client: client}

	// Set auth function for session manager
	sessionManager.SetAuthFunc(client.authenticateInternal)

	// Initial authentication
	if err := client.authenticate(context.Background()); err != nil {
		return nil, fmt.Errorf("initial authentication failed: %w", err)
	}

	return client, nil
}

// validateConfig validates client configuration
func validateConfig(config Config) error {
	if config.BaseURL == "" {
		return &ValidationError{Field: "BaseURL", Message: "cannot be empty"}
	}
	if config.Username == "" {
		return &ValidationError{Field: "Username", Message: "cannot be empty"}
	}
	if config.Password == "" {
		return &ValidationError{Field: "Password", Message: "cannot be empty"}
	}
	if config.MaxRetries < 0 {
		return &ValidationError{Field: "MaxRetries", Message: "cannot be negative"}
	}
	if config.RateLimit <= 0 {
		return &ValidationError{Field: "RateLimit", Message: "must be positive"}
	}
	if config.RateBurst <= 0 {
		return &ValidationError{Field: "RateBurst", Message: "must be positive"}
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 30 * time.Second
	}
	return nil
}

// createTLSConfig creates TLS configuration
func createTLSConfig(config Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !config.TLSVerify,
	}

	// Load CA certificate if provided
	if config.CACertPath != "" {
		caCert, err := os.ReadFile(config.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// Close gracefully shuts down the client
func (c *Client) Close() error {
	c.logger.Info("Synology client closing")
	// Wait for in-flight requests (with timeout)
	// Close HTTP client
	// Clear cache
	c.certCache.Invalidate()
	c.logger.Info("Synology client closed")
	return nil
}

// HealthCheck performs a lightweight health check
func (c *Client) HealthCheck(ctx context.Context) bool {
	if !c.circuitBreaker.IsHealthy() {
		c.logger.Warn("Health check failed: circuit breaker open")
		return false
	}

	// Make lightweight API call (list certificates)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.listCertificatesInternal(ctx)
	if err != nil {
		c.logger.Warn("Health check failed", "error", err)
		return false
	}

	return true
}

// UpdateCredentials updates the client credentials
func (c *Client) UpdateCredentials(username, password string) {
	c.sessionManager.UpdateCredentials(username, password)
	c.logger.Info("Client credentials updated")
}
