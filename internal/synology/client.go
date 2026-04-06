// Package synology provides a Go client for the Synology DSM reverse proxy API.
// It mirrors the functionality of the Python SynologyProxyManager class.
package synology

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
)

// --------------------------------------------------------------------------
// Wire types (Synology DSM JSON shapes)
// --------------------------------------------------------------------------

type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *apiError       `json:"error,omitempty"`
}

type apiError struct {
	Code int `json:"code"`
}

// ProxyEntry is the full record sent to / received from the DSM API.
type ProxyEntry struct {
	UUID                 string         `json:"UUID,omitempty"`
	Key                  string         `json:"_key,omitempty"`
	Description          string         `json:"description"`
	ProxyConnectTimeout  int            `json:"proxy_connect_timeout"`
	ProxyReadTimeout     int            `json:"proxy_read_timeout"`
	ProxySendTimeout     int            `json:"proxy_send_timeout"`
	ProxyHTTPVersion     int            `json:"proxy_http_version"`
	ProxyInterceptErrors bool           `json:"proxy_intercept_errors"`
	Frontend             ProxyFrontend  `json:"frontend"`
	Backend              ProxyBackend   `json:"backend"`
	CustomizeHeaders     []CustomHeader `json:"customize_headers"`
}

// ProxyFrontend is the source (public) side of a reverse proxy rule.
type ProxyFrontend struct {
	ACL      string       `json:"acl,omitempty"`
	FQDN     string       `json:"fqdn"`
	Port     int          `json:"port"`
	Protocol int          `json:"protocol"` // 1 = HTTPS
	HTTPS    *HTTPSConfig `json:"https,omitempty"`
}

// ProxyBackend is the destination (private) side of a reverse proxy rule.
type ProxyBackend struct {
	FQDN     string `json:"fqdn"`
	Port     int    `json:"port"`
	Protocol int    `json:"protocol"` // 0 = HTTP, 1 = HTTPS
}

// HTTPSConfig holds HTTPS-specific frontend settings.
type HTTPSConfig struct {
	HSTS bool `json:"hsts"`
}

// CustomHeader is an HTTP request header injected by the reverse proxy.
type CustomHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Certificate holds the DSM certificate metadata.
type Certificate struct {
	ID        string      `json:"id"`
	Desc      string      `json:"desc"`
	IsDefault bool        `json:"is_default"`
	Subject   CertSubject `json:"subject"`
}

// CertSubject holds CN and SANs of a certificate.
type CertSubject struct {
	CommonName string   `json:"common_name"`
	SubAltName []string `json:"sub_alt_name"`
}

// ACLProfile is an Access Control entry from DSM.
type ACLProfile struct {
	UUID string `json:"UUID"`
	Name string `json:"name"`
}

// --------------------------------------------------------------------------
// Client
// --------------------------------------------------------------------------

// Config holds the configuration for the Synology DSM client.
type Config struct {
	// URL is the base URL of DSM, e.g. "https://diskstation.local:5001"
	URL string
	// Username used for DSM login.
	Username string
	// Password used for DSM login.
	Password string
	// SkipTLSVerify disables TLS certificate verification (useful for self-signed certs).
	SkipTLSVerify bool
	// SessionTimeout controls how long the cached SID is kept before re-login. Default 1h.
	SessionTimeout time.Duration
}

// Client is a thread-safe Synology DSM API client.
type Client struct {
	cfg        Config
	mu         sync.Mutex
	sid        string
	synoToken  string
	loginTime  time.Time
	httpClient *http.Client
	log        logr.Logger
}

// New creates a new Synology DSM client.
func New(cfg Config, log logr.Logger) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}

	if cfg.SessionTimeout == 0 {
		cfg.SessionTimeout = time.Hour
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.SkipTLSVerify, //nolint:gosec // intentional for local DSM
		},
	}

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Transport: transport,
			Jar:       jar,
			Timeout:   3 * time.Minute,
		},
		log: log,
	}, nil
}

// ensureLoggedIn logs in if no valid session exists.
func (c *Client) ensureLoggedIn(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sid != "" && time.Since(c.loginTime) < c.cfg.SessionTimeout {
		return nil
	}
	return c.login(ctx)
}

// login authenticates against the DSM API and stores the session SID + SynoToken.
// Caller must hold c.mu.
func (c *Client) login(ctx context.Context) error {
	authURL := strings.TrimSuffix(c.cfg.URL, "/") + "/webapi/auth.cgi"
	params := url.Values{
		"api":               {"SYNO.API.Auth"},
		"version":           {"3"},
		"method":            {"login"},
		"account":           {c.cfg.Username},
		"passwd":            {c.cfg.Password},
		"session":           {"AppPortal"},
		"format":            {"sid"},
		"enable_syno_token": {"yes"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("building auth request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			SID       string `json:"sid"`
			SynoToken string `json:"synotoken"`
		} `json:"data"`
		Error *apiError `json:"error,omitempty"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parsing auth response: %w", err)
	}
	if !result.Success {
		code := 0
		if result.Error != nil {
			code = result.Error.Code
		}
		return fmt.Errorf("DSM login failed (error code %d)", code)
	}

	c.sid = result.Data.SID
	c.synoToken = result.Data.SynoToken
	c.loginTime = time.Now()
	c.log.Info("DSM login successful")
	return nil
}

// post sends a POST request to entry.cgi with the given form values and the current SID.
func (c *Client) post(ctx context.Context, endpoint string, form url.Values) (json.RawMessage, error) {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return nil, err
	}

	// Read SID/SynoToken under lock to avoid a race with concurrent re-logins.
	c.mu.Lock()
	form.Set("_sid", c.sid)
	synoToken := c.synoToken
	c.mu.Unlock()

	if synoToken != "" {
		form.Set("SynoToken", synoToken)
	}

	apiURL := strings.TrimSuffix(c.cfg.URL, "/") + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building request to %s: %w", endpoint, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if synoToken != "" {
		req.Header.Set("X-SYNO-TOKEN", synoToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", endpoint, err)
	}

	// DSM occasionally returns an HTML error page (e.g. on session expiry or
	// transient server errors) instead of a JSON response. Detect this and
	// force a re-login + single retry so the caller doesn't get a parse error.
	if len(body) > 0 && body[0] == '<' {
		c.log.Info("DSM returned HTML instead of JSON, re-authenticating and retrying",
			"endpoint", endpoint,
			"http_status", resp.StatusCode,
			"body", string(body),
		)
		c.mu.Lock()
		c.sid = ""
		c.mu.Unlock()
		if err := c.ensureLoggedIn(ctx); err != nil {
			return nil, fmt.Errorf("re-login after HTML response: %w", err)
		}
		c.mu.Lock()
		form.Set("_sid", c.sid)
		freshToken := c.synoToken
		c.mu.Unlock()
		if freshToken != "" {
			form.Set("SynoToken", freshToken)
			req.Header.Set("X-SYNO-TOKEN", freshToken)
		}
		return c.post(ctx, endpoint, form)
	}

	var result apiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response from %s: %w", endpoint, err)
	}
	if !result.Success {
		// If session expired, force a re-login and retry once with fresh credentials.
		if result.Error != nil && result.Error.Code == 119 {
			c.mu.Lock()
			c.sid = ""
			c.mu.Unlock()
			if err := c.ensureLoggedIn(ctx); err != nil {
				return nil, err
			}
			// Re-set _sid and SynoToken in the form with the new session values.
			c.mu.Lock()
			form.Set("_sid", c.sid)
			newToken := c.synoToken
			c.mu.Unlock()
			if newToken != "" {
				form.Set("SynoToken", newToken)
			}
			return c.post(ctx, endpoint, form)
		}
		code := 0
		if result.Error != nil {
			code = result.Error.Code
		}
		return nil, fmt.Errorf("DSM API error %d at %s", code, endpoint)
	}

	return result.Data, nil
}

// defaultWebSocketHeaders returns the default custom headers used for WebSocket proxying.
func DefaultWebSocketHeaders() []CustomHeader {
	return []CustomHeader{
		{Name: "Upgrade", Value: "$http_upgrade"},
		{Name: "Connection", Value: "$connection_upgrade"},
	}
}
