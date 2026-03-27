package synology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// authenticate performs authentication with retry logic
func (c *Client) authenticate(ctx context.Context) error {
	return c.retryCoordinator.Execute(ctx, "Authenticate", func() error {
		return c.authenticateOnce(ctx)
	})
}

// authenticateOnce performs a single authentication attempt
func (c *Client) authenticateOnce(ctx context.Context) error {
	// Wait for rate limit
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return &NetworkError{Message: "rate limit wait cancelled", Cause: err}
	}

	startTime := time.Now()
	c.metrics.IncrementActiveRequests()
	defer c.metrics.DecrementActiveRequests()

	// Build request
	authURL := fmt.Sprintf("%s/webapi/auth.cgi", c.baseURL)
	params := url.Values{}
	params.Set("api", "SYNO.API.Auth")
	params.Set("version", "3")
	params.Set("method", "login")
	params.Set("account", c.sessionManager.username)
	params.Set("passwd", c.sessionManager.password)
	params.Set("session", "SynologyProxyOperator")
	params.Set("format", "sid")

	fullURL := fmt.Sprintf("%s?%s", authURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return &NetworkError{Message: "failed to create request", Cause: err}
	}

	// Make HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		c.metrics.RecordAPIRequest("Authenticate", "error", time.Since(startTime).Seconds())
		return &NetworkError{Message: "HTTP request failed", Cause: err}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		c.circuitBreaker.RecordFailure()
		c.metrics.RecordAPIRequest("Authenticate", "error", time.Since(startTime).Seconds())
		return &APIError{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Operation:  "Authenticate",
			Retryable:  classifyHTTPStatus(resp.StatusCode),
		}
	}

	// Parse response
	var authResp authResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		c.metrics.RecordAPIRequest("Authenticate", "error", time.Since(startTime).Seconds())
		return fmt.Errorf("failed to parse auth response: %w", err)
	}

	// Check success
	if !authResp.Success {
		c.circuitBreaker.RecordFailure()
		c.metrics.RecordAPIRequest("Authenticate", "error", time.Since(startTime).Seconds())
		apiErr := classifyAPIError(authResp.Error.Code, "Authenticate")
		return &AuthError{Message: apiErr.Message}
	}

	// Update session in session manager
	c.sessionManager.mu.Lock()
	c.sessionManager.sid = authResp.Data.SID
	c.sessionManager.synoToken = authResp.Data.SynoToken
	c.sessionManager.createdAt = time.Now()
	c.sessionManager.isValid = true
	c.sessionManager.mu.Unlock()

	c.circuitBreaker.RecordSuccess()
	c.metrics.RecordAPIRequest("Authenticate", "success", time.Since(startTime).Seconds())
	c.logger.Info("Authentication successful")
	return nil
}

// authenticateInternal is used by session manager for lazy refresh
func (c *Client) authenticateInternal(ctx context.Context, username, password string) (*Session, error) {
	// Wait for rate limit
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, &NetworkError{Message: "rate limit wait cancelled", Cause: err}
	}

	startTime := time.Now()
	c.metrics.IncrementActiveRequests()
	defer c.metrics.DecrementActiveRequests()

	// Build request
	authURL := fmt.Sprintf("%s/webapi/auth.cgi", c.baseURL)
	params := url.Values{}
	params.Set("api", "SYNO.API.Auth")
	params.Set("version", "3")
	params.Set("method", "login")
	params.Set("account", username)
	params.Set("passwd", password)
	params.Set("session", "SynologyProxyOperator")
	params.Set("format", "sid")

	fullURL := fmt.Sprintf("%s?%s", authURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, &NetworkError{Message: "failed to create request", Cause: err}
	}

	// Make HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		c.metrics.RecordAPIRequest("Authenticate", "error", time.Since(startTime).Seconds())
		return nil, &NetworkError{Message: "HTTP request failed", Cause: err}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		c.circuitBreaker.RecordFailure()
		c.metrics.RecordAPIRequest("Authenticate", "error", time.Since(startTime).Seconds())
		return nil, &APIError{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Operation:  "Authenticate",
			Retryable:  classifyHTTPStatus(resp.StatusCode),
		}
	}

	// Parse response
	var authResp authResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		c.metrics.RecordAPIRequest("Authenticate", "error", time.Since(startTime).Seconds())
		return nil, fmt.Errorf("failed to parse auth response: %w", err)
	}

	// Check success
	if !authResp.Success {
		c.circuitBreaker.RecordFailure()
		c.metrics.RecordAPIRequest("Authenticate", "error", time.Since(startTime).Seconds())
		apiErr := classifyAPIError(authResp.Error.Code, "Authenticate")
		return nil, &AuthError{Message: apiErr.Message}
	}

	c.circuitBreaker.RecordSuccess()
	c.metrics.RecordAPIRequest("Authenticate", "success", time.Since(startTime).Seconds())

	return &Session{
		SID:       authResp.Data.SID,
		SynoToken: authResp.Data.SynoToken,
		CreatedAt: time.Now(),
		IsValid:   true,
	}, nil
}

// logout performs logout (optional, for cleanup)
func (c *Client) logout(ctx context.Context) error {
	sid, _, valid := c.sessionManager.GetSession()
	if !valid {
		return nil // Already logged out
	}

	logoutURL := fmt.Sprintf("%s/webapi/auth.cgi", c.baseURL)
	params := url.Values{}
	params.Set("api", "SYNO.API.Auth")
	params.Set("version", "3")
	params.Set("method", "logout")
	params.Set("session", "SynologyProxyOperator")
	params.Set("_sid", sid)

	fullURL := fmt.Sprintf("%s?%s", logoutURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	c.sessionManager.MarkInvalid()
	c.logger.Info("Logged out successfully")
	return nil
}
