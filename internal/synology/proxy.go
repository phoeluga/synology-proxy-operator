package synology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

const (
	proxyEndpoint = "/webapi/entry.cgi/SYNO.Core.AppPortal.ReverseProxy"
	proxyAPI      = "SYNO.Core.AppPortal.ReverseProxy"
)

// ProxyRule is the operator's internal representation of a desired proxy rule.
// It is translated into a ProxyEntry before being sent to the DSM API.
type ProxyRule struct {
	// Description is the unique record identifier stored in DSM (maps to "description").
	Description string
	// SourceHost is the public FQDN (frontend.fqdn).
	SourceHost string
	// SourcePort is the HTTPS port (frontend.port). Defaults to 443.
	SourcePort int
	// DestinationHost is the backend IP or FQDN (backend.fqdn).
	DestinationHost string
	// DestinationPort is the backend port (backend.port).
	DestinationPort int
	// DestinationHTTPS selects HTTPS for the backend protocol.
	DestinationHTTPS bool
	// ACLProfileID is the UUID of the DSM access control profile.
	ACLProfileID string
	// CustomHeaders overrides the default WebSocket upgrade headers.
	CustomHeaders []CustomHeader
	// ConnectTimeout in seconds. Defaults to 60.
	ConnectTimeout int
	// ReadTimeout in seconds. Defaults to 60.
	ReadTimeout int
	// SendTimeout in seconds. Defaults to 60.
	SendTimeout int
}

func (r *ProxyRule) withDefaults() ProxyRule {
	out := *r
	if out.SourcePort == 0 {
		out.SourcePort = 443
	}
	if out.ConnectTimeout == 0 {
		out.ConnectTimeout = 60
	}
	if out.ReadTimeout == 0 {
		out.ReadTimeout = 60
	}
	if out.SendTimeout == 0 {
		out.SendTimeout = 60
	}
	if out.CustomHeaders == nil {
		out.CustomHeaders = DefaultWebSocketHeaders()
	}
	return out
}

// ListProxyRecords returns all reverse proxy records from DSM.
func (c *Client) ListProxyRecords(ctx context.Context) ([]ProxyEntry, error) {
	data, err := c.post(ctx, proxyEndpoint, url.Values{
		"api":     {proxyAPI},
		"method":  {"list"},
		"version": {"1"},
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Entries []ProxyEntry `json:"entries"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing proxy list: %w", err)
	}
	return result.Entries, nil
}

// GetProxyRecord returns the first DSM proxy record whose description equals name,
// or nil if no such record exists.
func (c *Client) GetProxyRecord(ctx context.Context, name string) (*ProxyEntry, error) {
	entries, err := c.ListProxyRecords(ctx)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if entries[i].Description == name {
			return &entries[i], nil
		}
	}
	return nil, nil
}

// UpsertProxyRule creates or updates a DSM reverse proxy record based on rule.
// It returns the record UUID and whether the source hostname changed (which
// triggers certificate reassignment).
func (c *Client) UpsertProxyRule(ctx context.Context, rule ProxyRule) (uuid string, hostnameChanged bool, err error) {
	r := rule.withDefaults()

	existing, err := c.GetProxyRecord(ctx, r.Description)
	if err != nil {
		return "", false, fmt.Errorf("looking up existing record: %w", err)
	}

	backendProtocol := 0
	if r.DestinationHTTPS {
		backendProtocol = 1
	}

	entry := ProxyEntry{
		Description:          r.Description,
		ProxyConnectTimeout:  r.ConnectTimeout,
		ProxyReadTimeout:     r.ReadTimeout,
		ProxySendTimeout:     r.SendTimeout,
		ProxyHTTPVersion:     1,
		ProxyInterceptErrors: false,
		Frontend: ProxyFrontend{
			ACL:      r.ACLProfileID,
			FQDN:     r.SourceHost,
			Port:     r.SourcePort,
			Protocol: 1,
			HTTPS:    &HTTPSConfig{HSTS: true},
		},
		Backend: ProxyBackend{
			FQDN:     r.DestinationHost,
			Port:     r.DestinationPort,
			Protocol: backendProtocol,
		},
		CustomizeHeaders: r.CustomHeaders,
	}

	method := "create"
	if existing != nil {
		method = "set"
		entry.UUID = existing.UUID
		hostnameChanged = existing.Frontend.FQDN != r.SourceHost
		c.log.Info("Updating existing proxy record", "description", r.Description, "uuid", existing.UUID)
	} else {
		hostnameChanged = true
		c.log.Info("Creating new proxy record", "description", r.Description)
	}

	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return "", false, fmt.Errorf("marshalling entry: %w", err)
	}

	data, err := c.post(ctx, proxyEndpoint, url.Values{
		"api":     {proxyAPI},
		"method":  {method},
		"version": {"1"},
		"entry":   {string(entryJSON)},
	})
	if err != nil {
		return "", false, fmt.Errorf("DSM %s proxy record: %w", method, err)
	}

	// For create, the UUID is returned in the response; for set we already have it.
	if existing != nil {
		return existing.UUID, hostnameChanged, nil
	}

	var createResp struct {
		UUID string `json:"UUID"`
	}
	_ = json.Unmarshal(data, &createResp)
	if createResp.UUID == "" {
		// Fallback: re-fetch to obtain UUID.
		rec, err := c.GetProxyRecord(ctx, r.Description)
		if err != nil || rec == nil {
			return "", hostnameChanged, fmt.Errorf("created record but could not retrieve UUID")
		}
		return rec.UUID, hostnameChanged, nil
	}
	return createResp.UUID, hostnameChanged, nil
}

// DeleteProxyRecord deletes the DSM reverse proxy record with the given description.
// Returns false (no error) if the record does not exist.
func (c *Client) DeleteProxyRecord(ctx context.Context, name string) (bool, error) {
	existing, err := c.GetProxyRecord(ctx, name)
	if err != nil {
		return false, fmt.Errorf("looking up record to delete: %w", err)
	}
	if existing == nil {
		c.log.Info("Proxy record not found, nothing to delete", "description", name)
		return false, nil
	}

	uuids, err := json.Marshal([]string{existing.UUID})
	if err != nil {
		return false, err
	}

	_, err = c.post(ctx, proxyEndpoint, url.Values{
		"api":     {proxyAPI},
		"method":  {"delete"},
		"version": {"1"},
		"uuids":   {string(uuids)},
	})
	if err != nil {
		return false, fmt.Errorf("deleting proxy record %q: %w", name, err)
	}

	c.log.Info("Deleted proxy record", "description", name, "uuid", existing.UUID)
	return true, nil
}
