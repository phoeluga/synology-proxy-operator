package synology

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// CertificateOperations provides certificate operations
type CertificateOperations struct {
	client *Client
}

// List lists all certificates with caching
func (c *CertificateOperations) List(ctx context.Context) ([]*Certificate, error) {
	// Check cache first
	if cached, hit := c.client.certCache.Get(); hit {
		c.client.metrics.RecordCacheHit("certificates", true)
		c.client.logger.Info("Certificate cache hit", "count", len(cached))
		return cached, nil
	}

	c.client.metrics.RecordCacheHit("certificates", false)

	// Fetch from API
	var certs []*Certificate
	err := c.client.retryCoordinator.Execute(ctx, "Certificate.List", func() error {
		var err error
		certs, err = c.listOnce(ctx)
		return err
	})

	if err != nil {
		return nil, err
	}

	// Update cache
	c.client.certCache.Set(certs)

	return certs, nil
}

// listOnce performs a single list attempt
func (c *CertificateOperations) listOnce(ctx context.Context) ([]*Certificate, error) {
	// Check session validity
	if !c.client.sessionManager.IsValid() {
		if err := c.client.sessionManager.RefreshSession(ctx); err != nil {
			return nil, err
		}
	}

	// Wait for rate limit
	if err := c.client.rateLimiter.Wait(ctx); err != nil {
		return nil, &NetworkError{Message: "rate limit wait cancelled", Cause: err}
	}

	startTime := time.Now()
	c.client.metrics.IncrementActiveRequests()
	defer c.client.metrics.DecrementActiveRequests()

	sid, synoToken, _ := c.client.sessionManager.GetSession()

	// Build request
	listURL := fmt.Sprintf("%s/webapi/entry.cgi", c.client.baseURL)
	params := url.Values{}
	params.Set("api", "SYNO.Core.Certificate")
	params.Set("version", "1")
	params.Set("method", "list")
	params.Set("_sid", sid)

	fullURL := fmt.Sprintf("%s?%s", listURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, &NetworkError{Message: "failed to create request", Cause: err}
	}

	req.Header.Set("X-SYNO-TOKEN", synoToken)

	// Make HTTP request
	resp, err := c.client.httpClient.Do(req)
	if err != nil {
		c.client.circuitBreaker.RecordFailure()
		c.client.metrics.RecordAPIRequest("Certificate.List", "error", time.Since(startTime).Seconds())
		return nil, &NetworkError{Message: "HTTP request failed", Cause: err}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.client.sessionManager.MarkInvalid()
		c.client.metrics.RecordAPIRequest("Certificate.List", "auth_error", time.Since(startTime).Seconds())
		return nil, &AuthError{Message: "session expired"}
	}

	if resp.StatusCode != http.StatusOK {
		c.client.circuitBreaker.RecordFailure()
		c.client.metrics.RecordAPIRequest("Certificate.List", "error", time.Since(startTime).Seconds())
		return nil, &APIError{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Operation:  "Certificate.List",
			Retryable:  classifyHTTPStatus(resp.StatusCode),
		}
	}

	// Parse response
	var listResp certificateListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		c.client.metrics.RecordAPIRequest("Certificate.List", "error", time.Since(startTime).Seconds())
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	// Check success
	if !listResp.Success {
		c.client.circuitBreaker.RecordFailure()
		c.client.metrics.RecordAPIRequest("Certificate.List", "error", time.Since(startTime).Seconds())
		return nil, classifyAPIError(listResp.Error.Code, "Certificate.List")
	}

	// Convert to domain entities
	certs := make([]*Certificate, len(listResp.Data.Certificates))
	for i, apiCert := range listResp.Data.Certificates {
		certs[i] = apiCert.toDomain()
	}

	c.client.circuitBreaker.RecordSuccess()
	c.client.metrics.RecordAPIRequest("Certificate.List", "success", time.Since(startTime).Seconds())
	c.client.logger.Debug("Listed certificates", "count", len(certs))
	return certs, nil
}

// listCertificatesInternal is used internally (e.g., for health checks)
func (c *Client) listCertificatesInternal(ctx context.Context) ([]*Certificate, error) {
	cert := &CertificateOperations{client: c}
	return cert.List(ctx)
}

// Assign assigns a certificate to a proxy record
func (c *CertificateOperations) Assign(ctx context.Context, proxyUUID, certificateID string) error {
	return c.client.retryCoordinator.Execute(ctx, "Certificate.Assign", func() error {
		return c.assignOnce(ctx, proxyUUID, certificateID)
	})
}

// assignOnce performs a single assign attempt
func (c *CertificateOperations) assignOnce(ctx context.Context, proxyUUID, certificateID string) error {
	// Check session validity
	if !c.client.sessionManager.IsValid() {
		if err := c.client.sessionManager.RefreshSession(ctx); err != nil {
			return err
		}
	}

	// Wait for rate limit
	if err := c.client.rateLimiter.Wait(ctx); err != nil {
		return &NetworkError{Message: "rate limit wait cancelled", Cause: err}
	}

	startTime := time.Now()
	c.client.metrics.IncrementActiveRequests()
	defer c.client.metrics.DecrementActiveRequests()

	sid, synoToken, _ := c.client.sessionManager.GetSession()

	// Build request
	assignURL := fmt.Sprintf("%s/webapi/entry.cgi", c.client.baseURL)
	
	// Build form data
	data := url.Values{}
	data.Set("api", "SYNO.Core.ReverseProxy")
	data.Set("version", "1")
	data.Set("method", "assign_certificate")
	data.Set("_sid", sid)
	data.Set("uuid", proxyUUID)
	data.Set("certificate_id", certificateID)

	req, err := http.NewRequestWithContext(ctx, "POST", assignURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return &NetworkError{Message: "failed to create request", Cause: err}
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-SYNO-TOKEN", synoToken)

	// Make HTTP request
	resp, err := c.client.httpClient.Do(req)
	if err != nil {
		c.client.circuitBreaker.RecordFailure()
		c.client.metrics.RecordAPIRequest("Certificate.Assign", "error", time.Since(startTime).Seconds())
		return &NetworkError{Message: "HTTP request failed", Cause: err}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.client.sessionManager.MarkInvalid()
		c.client.metrics.RecordAPIRequest("Certificate.Assign", "auth_error", time.Since(startTime).Seconds())
		return &AuthError{Message: "session expired"}
	}

	if resp.StatusCode != http.StatusOK {
		c.client.circuitBreaker.RecordFailure()
		c.client.metrics.RecordAPIRequest("Certificate.Assign", "error", time.Since(startTime).Seconds())
		return &APIError{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Operation:  "Certificate.Assign",
			Retryable:  classifyHTTPStatus(resp.StatusCode),
		}
	}

	// Parse response
	var assignResp struct {
		Success bool `json:"success"`
		Error   struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&assignResp); err != nil {
		c.client.metrics.RecordAPIRequest("Certificate.Assign", "error", time.Since(startTime).Seconds())
		return fmt.Errorf("failed to parse assign response: %w", err)
	}

	// Check success
	if !assignResp.Success {
		c.client.circuitBreaker.RecordFailure()
		c.client.metrics.RecordAPIRequest("Certificate.Assign", "error", time.Since(startTime).Seconds())
		return classifyAPIError(assignResp.Error.Code, "Certificate.Assign")
	}

	c.client.circuitBreaker.RecordSuccess()
	c.client.metrics.RecordAPIRequest("Certificate.Assign", "success", time.Since(startTime).Seconds())
	c.client.logger.Info("Assigned certificate", "proxy_uuid", proxyUUID, "certificate_id", certificateID)
	return nil
}
