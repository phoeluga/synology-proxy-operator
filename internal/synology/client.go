// Package synology provides a client for the Synology WebAPI,
// specifically targeting the AppPortal reverse proxy and certificate management APIs.
package synology

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

const (
	apiAuth        = "SYNO.API.Auth"
	apiReverseProxy = "SYNO.Core.AppPortal.ReverseProxy"
	apiCertCRT     = "SYNO.Core.Certificate.CRT"
	apiCertService = "SYNO.Core.Certificate.Service"
	apiACL         = "SYNO.Core.AppPortal.AccessControl"

	entryPoint = "/webapi/entry.cgi"
	authPath   = "/webapi/auth.cgi"
)

// reverseProxyFrontend mirrors the "frontend" object in the Synology AppPortal API.
type reverseProxyFrontend struct {
	FQDN     string              `json:"fqdn"`
	Port     int                 `json:"port"`
	Protocol int                 `json:"protocol"` // 0=http, 1=https
	ACL      string              `json:"acl,omitempty"`
	HTTPS    *reverseProxyHTTPS  `json:"https,omitempty"`
}

type reverseProxyHTTPS struct {
	HSTS bool `json:"hsts"`
}

// reverseProxyBackend mirrors the "backend" object in the Synology AppPortal API.
type reverseProxyBackend struct {
	FQDN     string `json:"fqdn"`
	Port     int    `json:"port"`
	Protocol int    `json:"protocol"` // 0=http, 1=https
}

// reverseProxyCustomHeader is a custom header forwarded by the proxy.
type reverseProxyCustomHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ReverseProxyEntry represents a single reverse proxy record returned by the Synology AppPortal API.
type ReverseProxyEntry struct {
	UUID        string               `json:"UUID"`
	Description string               `json:"description"`
	Frontend    reverseProxyFrontend `json:"frontend"`
	Backend     reverseProxyBackend  `json:"backend"`
}

// ReverseProxySpec is the desired state for a reverse proxy record.
type ReverseProxySpec struct {
	Description    string
	SourceScheme   string
	SourceHostname string
	SourcePort     int
	DestScheme     string
	DestHostname   string
	DestPort       int
	ACLProfileID   string
}

// CertEntry represents a certificate record from the Synology API.
type CertEntry struct {
	ID          string   `json:"id"`
	Description string   `json:"desc"`
	Subject     certSubj `json:"subject"`
}

type certSubj struct {
	CommonName string   `json:"common_name"`
	AltNames   []string `json:"sub_alt_name"`
}

// aclProfile is a minimal ACL profile record.
type aclProfile struct {
	UUID string `json:"UUID"`
	Name string `json:"name"`
}

// synologyResponse is the generic envelope returned by every Synology API call.
type synologyResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *synologyError  `json:"error,omitempty"`
}

type synologyError struct {
	Code int `json:"code"`
}

// Client is a Synology WebAPI client that manages reverse proxy records.
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	sid        string
	synoToken  string
}

// NewClient creates a new Synology API client.
// Set skipTLSVerify=true when the NAS uses a self-signed certificate.
func NewClient(baseURL, username, password string, skipTLSVerify bool) *Client {
	jar, _ := cookiejar.New(nil)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerify}, //nolint:gosec
	}
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Jar:       jar,
			Transport: transport,
		},
	}
}

// Login authenticates against the Synology API and stores the SID + SynoToken.
func (c *Client) Login() error {
	params := url.Values{}
	params.Set("api", apiAuth)
	params.Set("version", "3")
	params.Set("method", "login")
	params.Set("account", c.username)
	params.Set("passwd", c.password)
	params.Set("session", "AppPortal")
	params.Set("format", "sid")
	params.Set("enable_syno_token", "yes")

	resp, err := c.postForm(c.baseURL+authPath, params)
	if err != nil {
		return fmt.Errorf("synology login request failed: %w", err)
	}
	defer resp.Body.Close()

	var result synologyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("synology login decode failed: %w", err)
	}
	if !result.Success {
		code := 0
		if result.Error != nil {
			code = result.Error.Code
		}
		return fmt.Errorf("synology login failed with error code %d", code)
	}

	var data struct {
		SID       string `json:"sid"`
		SynoToken string `json:"synotoken"`
	}
	if err := json.Unmarshal(result.Data, &data); err != nil {
		return fmt.Errorf("synology login data parse failed: %w", err)
	}
	c.sid = data.SID
	c.synoToken = data.SynoToken
	return nil
}

// Logout invalidates the current session.
func (c *Client) Logout() error {
	params := url.Values{}
	params.Set("api", apiAuth)
	params.Set("version", "3")
	params.Set("method", "logout")
	params.Set("session", "AppPortal")
	c.addAuth(&params)

	resp, err := c.postForm(c.baseURL+authPath, params)
	if err != nil {
		return fmt.Errorf("synology logout request failed: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// listReverseProxyRecords returns all existing reverse proxy records.
func (c *Client) listReverseProxyRecords() ([]ReverseProxyEntry, error) {
	payload := url.Values{}
	payload.Set("api", apiReverseProxy)
	payload.Set("method", "list")
	payload.Set("version", "1")
	c.addAuth(&payload)

	resp, err := c.postForm(c.baseURL+entryPoint+"/"+apiReverseProxy, payload)
	if err != nil {
		return nil, fmt.Errorf("list reverse proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	var result synologyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("list reverse proxy decode failed: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("list reverse proxy failed: %s", errorMsg(result.Error))
	}

	var data struct {
		Entries []ReverseProxyEntry `json:"entries"`
	}
	if err := json.Unmarshal(result.Data, &data); err != nil {
		return nil, fmt.Errorf("list reverse proxy data parse failed: %w", err)
	}
	return data.Entries, nil
}

// GetExistingRecord finds a reverse proxy record by its description field.
// Returns nil, nil when no matching record is found.
func (c *Client) GetExistingRecord(description string) (*ReverseProxyEntry, error) {
	records, err := c.listReverseProxyRecords()
	if err != nil {
		return nil, err
	}
	for i := range records {
		if records[i].Description == description {
			return &records[i], nil
		}
	}
	return nil, nil
}

// UpsertReverseProxy creates or updates a reverse proxy record.
// It matches on Description; if a record with that description already exists it is updated.
// Returns the UUID of the created/updated record.
func (c *Client) UpsertReverseProxy(spec ReverseProxySpec) (string, error) {
	existing, err := c.GetExistingRecord(spec.Description)
	if err != nil {
		return "", err
	}

	method := "create"
	if existing != nil {
		method = "set"
	}

	srcProto := 0
	if strings.ToLower(spec.SourceScheme) == "https" {
		srcProto = 1
	}
	dstProto := 0
	if strings.ToLower(spec.DestScheme) == "https" {
		dstProto = 1
	}

	entry := map[string]interface{}{
		"description":             spec.Description,
		"proxy_connect_timeout":   60,
		"proxy_read_timeout":      60,
		"proxy_send_timeout":      60,
		"proxy_http_version":      1,
		"proxy_intercept_errors":  false,
		"frontend": reverseProxyFrontend{
			FQDN:     spec.SourceHostname,
			Port:     spec.SourcePort,
			Protocol: srcProto,
			ACL:      spec.ACLProfileID,
			HTTPS:    &reverseProxyHTTPS{HSTS: srcProto == 1},
		},
		"backend": reverseProxyBackend{
			FQDN:     spec.DestHostname,
			Port:     spec.DestPort,
			Protocol: dstProto,
		},
		"customize_headers": []reverseProxyCustomHeader{
			{Name: "Upgrade", Value: "$http_upgrade"},
			{Name: "Connection", Value: "$connection_upgrade"},
		},
	}
	if existing != nil {
		entry["UUID"] = existing.UUID
	}

	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshal reverse proxy entry failed: %w", err)
	}

	payload := url.Values{}
	payload.Set("api", apiReverseProxy)
	payload.Set("method", method)
	payload.Set("version", "1")
	payload.Set("entry", string(entryJSON))
	c.addAuth(&payload)

	resp, err := c.postForm(c.baseURL+entryPoint+"/"+apiReverseProxy, payload)
	if err != nil {
		return "", fmt.Errorf("upsert reverse proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	var result synologyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("upsert reverse proxy decode failed: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("upsert reverse proxy failed: %s", errorMsg(result.Error))
	}

	if existing != nil {
		return existing.UUID, nil
	}

	// On create, fetch the newly created record to get its UUID.
	created, err := c.GetExistingRecord(spec.Description)
	if err != nil {
		return "", fmt.Errorf("upsert reverse proxy: failed to retrieve created record: %w", err)
	}
	if created == nil {
		return "", fmt.Errorf("upsert reverse proxy: created record not found after creation")
	}
	return created.UUID, nil
}

// DeleteRecord removes a reverse proxy record identified by its description.
// Returns nil if no matching record exists (idempotent).
func (c *Client) DeleteRecord(description string) error {
	existing, err := c.GetExistingRecord(description)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}

	uuidsJSON, err := json.Marshal([]string{existing.UUID})
	if err != nil {
		return fmt.Errorf("marshal delete uuids failed: %w", err)
	}

	payload := url.Values{}
	payload.Set("api", apiReverseProxy)
	payload.Set("method", "delete")
	payload.Set("version", "1")
	payload.Set("uuids", string(uuidsJSON))
	c.addAuth(&payload)

	resp, err := c.postForm(c.baseURL+entryPoint+"/"+apiReverseProxy, payload)
	if err != nil {
		return fmt.Errorf("delete reverse proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	var result synologyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("delete reverse proxy decode failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("delete reverse proxy failed: %s", errorMsg(result.Error))
	}
	return nil
}

// FindMatchingCert searches for a certificate whose CN or SANs match the given hostname.
// It supports wildcard certificates (e.g. *.hnet.io matches myapp.hnet.io).
// Returns certID, certDescription, error.
func (c *Client) FindMatchingCert(hostname string) (string, string, error) {
	payload := url.Values{}
	payload.Set("api", apiCertCRT)
	payload.Set("method", "list")
	payload.Set("version", "1")
	c.addAuth(&payload)

	resp, err := c.postForm(c.baseURL+entryPoint, payload)
	if err != nil {
		return "", "", fmt.Errorf("list certificates request failed: %w", err)
	}
	defer resp.Body.Close()

	var result synologyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("list certificates decode failed: %w", err)
	}
	if !result.Success {
		return "", "", fmt.Errorf("list certificates failed: %s", errorMsg(result.Error))
	}

	var data struct {
		Certificates []CertEntry `json:"certificates"`
	}
	if err := json.Unmarshal(result.Data, &data); err != nil {
		return "", "", fmt.Errorf("list certificates data parse failed: %w", err)
	}

	for _, cert := range data.Certificates {
		if certMatchesHostname(cert, hostname) {
			return cert.ID, cert.Description, nil
		}
	}
	return "", "", nil
}

// AssignCertificate assigns a matching wildcard certificate to a reverse proxy record.
func (c *Client) AssignCertificate(proxyUUID, hostname string) error {
	certID, _, err := c.FindMatchingCert(hostname)
	if err != nil {
		return err
	}
	if certID == "" {
		return fmt.Errorf("no matching certificate found for hostname %q", hostname)
	}

	type certService struct {
		Service     map[string]interface{} `json:"service"`
		OldID       string                 `json:"old_id"`
		ID          string                 `json:"id"`
	}

	settings := []certService{
		{
			Service: map[string]interface{}{
				"display_name":  hostname,
				"isPkg":         false,
				"multiple_cert": true,
				"owner":         "root",
				"service":       proxyUUID,
				"subscriber":    "ReverseProxy",
				"user_setable":  true,
			},
			OldID: "",
			ID:    certID,
		},
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal certificate settings failed: %w", err)
	}

	payload := url.Values{}
	payload.Set("api", apiCertService)
	payload.Set("method", "set")
	payload.Set("version", "1")
	payload.Set("settings", string(settingsJSON))
	c.addAuth(&payload)

	resp, err := c.postForm(c.baseURL+entryPoint, payload)
	if err != nil {
		return fmt.Errorf("assign certificate request failed: %w", err)
	}
	defer resp.Body.Close()

	var result synologyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("assign certificate decode failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("assign certificate failed: %s", errorMsg(result.Error))
	}
	return nil
}

// GetACLProfileID returns the UUID of an ACL profile by name.
// Returns an empty string (and no error) when the profile is not found.
func (c *Client) GetACLProfileID(profileName string) (string, error) {
	if profileName == "" {
		return "", nil
	}

	payload := url.Values{}
	payload.Set("api", apiACL)
	payload.Set("method", "list")
	payload.Set("version", "1")
	c.addAuth(&payload)

	resp, err := c.postForm(c.baseURL+entryPoint+"/"+apiACL, payload)
	if err != nil {
		return "", fmt.Errorf("list ACL profiles request failed: %w", err)
	}
	defer resp.Body.Close()

	var result synologyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("list ACL profiles decode failed: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("list ACL profiles failed: %s", errorMsg(result.Error))
	}

	var data struct {
		Entries []aclProfile `json:"entries"`
	}
	if err := json.Unmarshal(result.Data, &data); err != nil {
		return "", fmt.Errorf("list ACL profiles data parse failed: %w", err)
	}

	for _, p := range data.Entries {
		if p.Name == profileName {
			return p.UUID, nil
		}
	}
	return "", nil
}

// addAuth injects the SID into a request's form values.
// The SynoToken is sent as an HTTP header via postForm.
func (c *Client) addAuth(params *url.Values) {
	if c.sid != "" {
		params.Set("_sid", c.sid)
	}
}

// postForm is a helper that POSTs form data and injects the X-SYNO-TOKEN header when available.
func (c *Client) postForm(endpoint string, params url.Values) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.synoToken != "" {
		req.Header.Set("X-SYNO-TOKEN", c.synoToken)
	}
	return c.httpClient.Do(req)
}

// certMatchesHostname checks whether a certificate covers the given hostname,
// including wildcard matching (*.example.com matches sub.example.com).
func certMatchesHostname(cert CertEntry, hostname string) bool {
	candidates := append([]string{cert.Subject.CommonName}, cert.Subject.AltNames...)
	for _, name := range candidates {
		if name == hostname {
			return true
		}
		if strings.HasPrefix(name, "*.") {
			suffix := name[1:] // ".example.com"
			if strings.HasSuffix(hostname, suffix) && !strings.Contains(strings.TrimSuffix(hostname, suffix), ".") {
				return true
			}
		}
	}
	return false
}

// errorMsg formats a Synology error for display.
func errorMsg(e *synologyError) string {
	if e == nil {
		return "unknown error"
	}
	return fmt.Sprintf("error code %d", e.Code)
}
