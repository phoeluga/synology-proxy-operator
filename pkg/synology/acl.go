package synology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ACLOperations provides ACL profile operations
type ACLOperations struct {
	client *Client
}

// List lists all ACL profiles
func (a *ACLOperations) List(ctx context.Context) ([]*ACLProfile, error) {
	var profiles []*ACLProfile
	err := a.client.retryCoordinator.Execute(ctx, "ACL.List", func() error {
		var err error
		profiles, err = a.listOnce(ctx)
		return err
	})
	return profiles, err
}

// listOnce performs a single list attempt
func (a *ACLOperations) listOnce(ctx context.Context) ([]*ACLProfile, error) {
	// Check session validity
	if !a.client.sessionManager.IsValid() {
		if err := a.client.sessionManager.RefreshSession(ctx); err != nil {
			return nil, err
		}
	}

	// Wait for rate limit
	if err := a.client.rateLimiter.Wait(ctx); err != nil {
		return nil, &NetworkError{Message: "rate limit wait cancelled", Cause: err}
	}

	startTime := time.Now()
	a.client.metrics.IncrementActiveRequests()
	defer a.client.metrics.DecrementActiveRequests()

	sid, synoToken, _ := a.client.sessionManager.GetSession()

	// Build request
	listURL := fmt.Sprintf("%s/webapi/entry.cgi", a.client.baseURL)
	params := url.Values{}
	params.Set("api", "SYNO.Core.ACL")
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
	resp, err := a.client.httpClient.Do(req)
	if err != nil {
		a.client.circuitBreaker.RecordFailure()
		a.client.metrics.RecordAPIRequest("ACL.List", "error", time.Since(startTime).Seconds())
		return nil, &NetworkError{Message: "HTTP request failed", Cause: err}
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		a.client.sessionManager.MarkInvalid()
		a.client.metrics.RecordAPIRequest("ACL.List", "auth_error", time.Since(startTime).Seconds())
		return nil, &AuthError{Message: "session expired"}
	}

	if resp.StatusCode != http.StatusOK {
		a.client.circuitBreaker.RecordFailure()
		a.client.metrics.RecordAPIRequest("ACL.List", "error", time.Since(startTime).Seconds())
		return nil, &APIError{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Operation:  "ACL.List",
			Retryable:  classifyHTTPStatus(resp.StatusCode),
		}
	}

	// Parse response
	var listResp aclListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		a.client.metrics.RecordAPIRequest("ACL.List", "error", time.Since(startTime).Seconds())
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	// Check success
	if !listResp.Success {
		a.client.circuitBreaker.RecordFailure()
		a.client.metrics.RecordAPIRequest("ACL.List", "error", time.Since(startTime).Seconds())
		return nil, classifyAPIError(listResp.Error.Code, "ACL.List")
	}

	// Convert to domain entities
	profiles := make([]*ACLProfile, len(listResp.Data.Profiles))
	for i, apiProfile := range listResp.Data.Profiles {
		profiles[i] = apiProfile.toDomain()
	}

	a.client.circuitBreaker.RecordSuccess()
	a.client.metrics.RecordAPIRequest("ACL.List", "success", time.Since(startTime).Seconds())
	a.client.logger.Debug("Listed ACL profiles", "count", len(profiles))
	return profiles, nil
}

// Get retrieves an ACL profile by name
func (a *ACLOperations) Get(ctx context.Context, name string) (*ACLProfile, error) {
	profiles, err := a.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, profile := range profiles {
		if profile.Name == name {
			return profile, nil
		}
	}

	return nil, &NotFoundError{Resource: "ACLProfile", ID: name}
}
