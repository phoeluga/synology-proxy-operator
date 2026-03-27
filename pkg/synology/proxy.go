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

// ProxyOperations provides proxy record operations
type ProxyOperations struct {
	client *Client
}

// List lists all proxy records
func (p *ProxyOperations) List(ctx context.Context) ([]*ProxyRecord, error) {
	var records []*ProxyRecord
	err := p.client.retryCoordinator.Execute(ctx, "Proxy.List", func() error {
		var err error
		records, err = p.listOnce(ctx)
		return err
	})
	return records, err
}

// listOnce performs a single list attempt
func (p *ProxyOperations) listOnce(ctx context.Context) ([]*ProxyRecord, error) {
	// Check session validity
	if !p.client.sessionManager.IsValid() {
		if err := p.client.sessionManager.RefreshSession(ctx); err != nil {
			return nil, err
		}
	}

	// Wait for rate limit
	if err := p.client.rateLimiter.Wait(ctx); err != nil {
		return nil, &NetworkError{Message: "rate limit wait cancelled", Cause: err}
	}

	startTime := time.Now()
	p.client.metrics.IncrementActiveRequests()
	defer p.client.metrics.DecrementActiveRequests()

	sid, synoToken, _ := p.client.sessionManager.GetSession()

	// Build request
	listURL := fmt.Sprintf("%s/webapi/entry.cgi", p.client.baseURL)
	params := url.Values{}
	params.Set("api", "SYNO.Core.ReverseProxy")
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
	resp, err := p.client.httpClient.Do(req)
	if err != nil {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.List", "error", time.Since(startTime).Seconds())
		return nil, &NetworkError{Message: "HTTP request failed", Cause: err}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		p.client.sessionManager.MarkInvalid()
		p.client.metrics.RecordAPIRequest("Proxy.List", "auth_error", time.Since(startTime).Seconds())
		return nil, &AuthError{Message: "session expired"}
	}

	if resp.StatusCode != http.StatusOK {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.List", "error", time.Since(startTime).Seconds())
		return nil, &APIError{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Operation:  "Proxy.List",
			Retryable:  classifyHTTPStatus(resp.StatusCode),
		}
	}

	// Parse response
	var listResp proxyListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		p.client.metrics.RecordAPIRequest("Proxy.List", "error", time.Since(startTime).Seconds())
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	// Check success
	if !listResp.Success {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.List", "error", time.Since(startTime).Seconds())
		return nil, classifyAPIError(listResp.Error.Code, "Proxy.List")
	}

	// Convert to domain entities
	records := make([]*ProxyRecord, len(listResp.Data.Records))
	for i, apiRecord := range listResp.Data.Records {
		records[i] = apiRecord.toDomain()
	}

	p.client.circuitBreaker.RecordSuccess()
	p.client.metrics.RecordAPIRequest("Proxy.List", "success", time.Since(startTime).Seconds())
	p.client.logger.Debug("Listed proxy records", "count", len(records))
	return records, nil
}

// Get retrieves a proxy record by UUID
func (p *ProxyOperations) Get(ctx context.Context, uuid string) (*ProxyRecord, error) {
	records, err := p.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if record.UUID == uuid {
			return record, nil
		}
	}

	return nil, &NotFoundError{Resource: "ProxyRecord", ID: uuid}
}

// Create creates a new proxy record
func (p *ProxyOperations) Create(ctx context.Context, record *ProxyRecord) (*ProxyRecord, error) {
	var created *ProxyRecord
	err := p.client.retryCoordinator.Execute(ctx, "Proxy.Create", func() error {
		var err error
		created, err = p.createOnce(ctx, record)
		return err
	})
	return created, err
}

// createOnce performs a single create attempt
func (p *ProxyOperations) createOnce(ctx context.Context, record *ProxyRecord) (*ProxyRecord, error) {
	// Check session validity
	if !p.client.sessionManager.IsValid() {
		if err := p.client.sessionManager.RefreshSession(ctx); err != nil {
			return nil, err
		}
	}

	// Wait for rate limit
	if err := p.client.rateLimiter.Wait(ctx); err != nil {
		return nil, &NetworkError{Message: "rate limit wait cancelled", Cause: err}
	}

	startTime := time.Now()
	p.client.metrics.IncrementActiveRequests()
	defer p.client.metrics.DecrementActiveRequests()

	sid, synoToken, _ := p.client.sessionManager.GetSession()

	// Build request
	createURL := fmt.Sprintf("%s/webapi/entry.cgi", p.client.baseURL)
	
	// Build form data
	data := url.Values{}
	data.Set("api", "SYNO.Core.ReverseProxy")
	data.Set("version", "1")
	data.Set("method", "create")
	data.Set("_sid", sid)
	data.Set("frontend_hostname", record.FrontendHostname)
	data.Set("frontend_port", fmt.Sprintf("%d", record.FrontendPort))
	data.Set("frontend_protocol", record.FrontendProtocol)
	data.Set("backend_hostname", record.BackendHostname)
	data.Set("backend_port", fmt.Sprintf("%d", record.BackendPort))
	data.Set("backend_protocol", record.BackendProtocol)
	data.Set("description", record.Description)
	data.Set("enabled", fmt.Sprintf("%t", record.Enabled))
	
	if record.ACLProfileName != "" {
		data.Set("acl_profile_name", record.ACLProfileName)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", createURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, &NetworkError{Message: "failed to create request", Cause: err}
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-SYNO-TOKEN", synoToken)

	// Make HTTP request
	resp, err := p.client.httpClient.Do(req)
	if err != nil {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.Create", "error", time.Since(startTime).Seconds())
		return nil, &NetworkError{Message: "HTTP request failed", Cause: err}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		p.client.sessionManager.MarkInvalid()
		p.client.metrics.RecordAPIRequest("Proxy.Create", "auth_error", time.Since(startTime).Seconds())
		return nil, &AuthError{Message: "session expired"}
	}

	if resp.StatusCode != http.StatusOK {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.Create", "error", time.Since(startTime).Seconds())
		return nil, &APIError{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Operation:  "Proxy.Create",
			Retryable:  classifyHTTPStatus(resp.StatusCode),
		}
	}

	// Parse response
	var createResp struct {
		Success bool `json:"success"`
		Data    struct {
			UUID string `json:"uuid"`
		} `json:"data"`
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		p.client.metrics.RecordAPIRequest("Proxy.Create", "error", time.Since(startTime).Seconds())
		return nil, fmt.Errorf("failed to parse create response: %w", err)
	}

	// Check success
	if !createResp.Success {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.Create", "error", time.Since(startTime).Seconds())
		return nil, classifyAPIError(createResp.Error.Code, "Proxy.Create")
	}

	// Set UUID and return
	record.UUID = createResp.Data.UUID
	record.CreatedAt = time.Now()
	record.UpdatedAt = time.Now()

	p.client.circuitBreaker.RecordSuccess()
	p.client.metrics.RecordAPIRequest("Proxy.Create", "success", time.Since(startTime).Seconds())
	p.client.logger.Info("Created proxy record", "uuid", record.UUID, "hostname", record.FrontendHostname)
	return record, nil
}

// Update updates an existing proxy record
func (p *ProxyOperations) Update(ctx context.Context, record *ProxyRecord) error {
	return p.client.retryCoordinator.Execute(ctx, "Proxy.Update", func() error {
		return p.updateOnce(ctx, record)
	})
}

// updateOnce performs a single update attempt
func (p *ProxyOperations) updateOnce(ctx context.Context, record *ProxyRecord) error {
	// Check session validity
	if !p.client.sessionManager.IsValid() {
		if err := p.client.sessionManager.RefreshSession(ctx); err != nil {
			return err
		}
	}

	// Wait for rate limit
	if err := p.client.rateLimiter.Wait(ctx); err != nil {
		return &NetworkError{Message: "rate limit wait cancelled", Cause: err}
	}

	startTime := time.Now()
	p.client.metrics.IncrementActiveRequests()
	defer p.client.metrics.DecrementActiveRequests()

	sid, synoToken, _ := p.client.sessionManager.GetSession()

	// Build request
	updateURL := fmt.Sprintf("%s/webapi/entry.cgi", p.client.baseURL)
	
	// Build form data
	data := url.Values{}
	data.Set("api", "SYNO.Core.ReverseProxy")
	data.Set("version", "1")
	data.Set("method", "update")
	data.Set("_sid", sid)
	data.Set("uuid", record.UUID)
	data.Set("frontend_hostname", record.FrontendHostname)
	data.Set("frontend_port", fmt.Sprintf("%d", record.FrontendPort))
	data.Set("frontend_protocol", record.FrontendProtocol)
	data.Set("backend_hostname", record.BackendHostname)
	data.Set("backend_port", fmt.Sprintf("%d", record.BackendPort))
	data.Set("backend_protocol", record.BackendProtocol)
	data.Set("description", record.Description)
	data.Set("enabled", fmt.Sprintf("%t", record.Enabled))
	
	if record.ACLProfileName != "" {
		data.Set("acl_profile_name", record.ACLProfileName)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", updateURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return &NetworkError{Message: "failed to create request", Cause: err}
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-SYNO-TOKEN", synoToken)

	// Make HTTP request
	resp, err := p.client.httpClient.Do(req)
	if err != nil {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.Update", "error", time.Since(startTime).Seconds())
		return &NetworkError{Message: "HTTP request failed", Cause: err}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		p.client.sessionManager.MarkInvalid()
		p.client.metrics.RecordAPIRequest("Proxy.Update", "auth_error", time.Since(startTime).Seconds())
		return &AuthError{Message: "session expired"}
	}

	if resp.StatusCode != http.StatusOK {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.Update", "error", time.Since(startTime).Seconds())
		return &APIError{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Operation:  "Proxy.Update",
			Retryable:  classifyHTTPStatus(resp.StatusCode),
		}
	}

	// Parse response
	var updateResp struct {
		Success bool `json:"success"`
		Error   struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&updateResp); err != nil {
		p.client.metrics.RecordAPIRequest("Proxy.Update", "error", time.Since(startTime).Seconds())
		return fmt.Errorf("failed to parse update response: %w", err)
	}

	// Check success
	if !updateResp.Success {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.Update", "error", time.Since(startTime).Seconds())
		return classifyAPIError(updateResp.Error.Code, "Proxy.Update")
	}

	record.UpdatedAt = time.Now()

	p.client.circuitBreaker.RecordSuccess()
	p.client.metrics.RecordAPIRequest("Proxy.Update", "success", time.Since(startTime).Seconds())
	p.client.logger.Info("Updated proxy record", "uuid", record.UUID, "hostname", record.FrontendHostname)
	return nil
}

// Delete deletes a proxy record by UUID
func (p *ProxyOperations) Delete(ctx context.Context, uuid string) error {
	return p.client.retryCoordinator.Execute(ctx, "Proxy.Delete", func() error {
		return p.deleteOnce(ctx, uuid)
	})
}

// deleteOnce performs a single delete attempt
func (p *ProxyOperations) deleteOnce(ctx context.Context, uuid string) error {
	// Check session validity
	if !p.client.sessionManager.IsValid() {
		if err := p.client.sessionManager.RefreshSession(ctx); err != nil {
			return err
		}
	}

	// Wait for rate limit
	if err := p.client.rateLimiter.Wait(ctx); err != nil {
		return &NetworkError{Message: "rate limit wait cancelled", Cause: err}
	}

	startTime := time.Now()
	p.client.metrics.IncrementActiveRequests()
	defer p.client.metrics.DecrementActiveRequests()

	sid, synoToken, _ := p.client.sessionManager.GetSession()

	// Build request
	deleteURL := fmt.Sprintf("%s/webapi/entry.cgi", p.client.baseURL)
	
	// Build form data
	data := url.Values{}
	data.Set("api", "SYNO.Core.ReverseProxy")
	data.Set("version", "1")
	data.Set("method", "delete")
	data.Set("_sid", sid)
	data.Set("uuid", uuid)

	req, err := http.NewRequestWithContext(ctx, "POST", deleteURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return &NetworkError{Message: "failed to create request", Cause: err}
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-SYNO-TOKEN", synoToken)

	// Make HTTP request
	resp, err := p.client.httpClient.Do(req)
	if err != nil {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.Delete", "error", time.Since(startTime).Seconds())
		return &NetworkError{Message: "HTTP request failed", Cause: err}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		p.client.sessionManager.MarkInvalid()
		p.client.metrics.RecordAPIRequest("Proxy.Delete", "auth_error", time.Since(startTime).Seconds())
		return &AuthError{Message: "session expired"}
	}

	if resp.StatusCode != http.StatusOK {
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.Delete", "error", time.Since(startTime).Seconds())
		return &APIError{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Operation:  "Proxy.Delete",
			Retryable:  classifyHTTPStatus(resp.StatusCode),
		}
	}

	// Parse response
	var deleteResp struct {
		Success bool `json:"success"`
		Error   struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&deleteResp); err != nil {
		p.client.metrics.RecordAPIRequest("Proxy.Delete", "error", time.Since(startTime).Seconds())
		return fmt.Errorf("failed to parse delete response: %w", err)
	}

	// Check success
	if !deleteResp.Success {
		// 404 is acceptable for delete (already deleted)
		if deleteResp.Error.Code == 404 {
			p.client.logger.Info("Proxy record already deleted", "uuid", uuid)
			p.client.metrics.RecordAPIRequest("Proxy.Delete", "success", time.Since(startTime).Seconds())
			return nil
		}
		
		p.client.circuitBreaker.RecordFailure()
		p.client.metrics.RecordAPIRequest("Proxy.Delete", "error", time.Since(startTime).Seconds())
		return classifyAPIError(deleteResp.Error.Code, "Proxy.Delete")
	}

	p.client.circuitBreaker.RecordSuccess()
	p.client.metrics.RecordAPIRequest("Proxy.Delete", "success", time.Since(startTime).Seconds())
	p.client.logger.Info("Deleted proxy record", "uuid", uuid)
	return nil
}
