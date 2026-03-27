package synology

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// RetryCoordinator handles retry logic with exponential backoff
type RetryCoordinator struct {
	maxRetries    int
	initialDelay  time.Duration
	maxDelay      time.Duration
	backoffFactor float64
	jitterPercent float64
	logger        Logger
	metrics       MetricsRegistry
}

// NewRetryCoordinator creates a new retry coordinator
func NewRetryCoordinator(maxRetries int, logger Logger, metrics MetricsRegistry) *RetryCoordinator {
	return &RetryCoordinator{
		maxRetries:    maxRetries,
		initialDelay:  1 * time.Second,
		maxDelay:      5 * time.Minute,
		backoffFactor: 2.0,
		jitterPercent: 0.1,
		logger:        logger,
		metrics:       metrics,
	}
}

// Execute executes an operation with retry logic
func (r *RetryCoordinator) Execute(ctx context.Context, operation string, fn func() error) error {
	delay := r.initialDelay

	for attempt := 0; attempt < r.maxRetries; attempt++ {
		// Execute operation
		err := fn()

		// Success
		if err == nil {
			if attempt > 0 {
				r.logger.Info("Operation succeeded after retries",
					"operation", operation,
					"attempts", attempt+1)
			}
			return nil
		}

		// Check if retryable
		if !isRetryable(err) {
			r.logger.Warn("Non-retryable error, not retrying",
				"operation", operation,
				"error", err)
			r.metrics.RecordAPIError(operation, "non_retryable")
			return err
		}

		// Last attempt
		if attempt == r.maxRetries-1 {
			r.logger.Error("Max retries exceeded", err,
				"operation", operation,
				"attempts", r.maxRetries)
			r.metrics.RecordAPIError(operation, "max_retries")
			return fmt.Errorf("max retries exceeded: %w", err)
		}

		// Calculate delay with jitter
		jitter := time.Duration(rand.Float64() * r.jitterPercent * 2 * float64(delay))
		actualDelay := delay + jitter - time.Duration(r.jitterPercent*float64(delay))

		r.logger.Warn("Retrying operation",
			"operation", operation,
			"attempt", attempt+1,
			"delay", actualDelay,
			"error", err)

		// Wait with context cancellation support
		select {
		case <-time.After(actualDelay):
			// Continue to next attempt
		case <-ctx.Done():
			return ctx.Err()
		}

		// Exponential backoff
		delay = time.Duration(float64(delay) * r.backoffFactor)
		if delay > r.maxDelay {
			delay = r.maxDelay
		}
	}

	return fmt.Errorf("max retries exceeded")
}
