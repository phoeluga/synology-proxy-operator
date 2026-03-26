// Package synology provides a client for the Synology WebAPI,
// specifically targeting the reverse proxy and certificate management APIs.
package synology

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

const (
	apiAuth         = "SYNO.API.Auth"
	apiReverseProxy = "SYNO.Core.ReverseProxy.Rule"
	apiCertificate  = "SYNO.Core.Certificate"
	apiACL          = "SYNO.Core.ReverseProxy.ACL"

	entryPoint = "/webapi/entry.cgi"
	authPath   = "/webapi/auth.cgi"
)

// ReverseProxyEntry represents a single reverse proxy record returned by the Synology API.
type ReverseProxyEntry struct {
	UUID           string `json:"id"`
	Description    string `json:"description"`
	SourceScheme   string `json:"src_scheme"`
	SourceHost     string `json:"src_hostname"`
	SourcePort     int    `json:"src_port"`
	DestScheme     string `json:"dst_scheme"`
	DestHost       string `json:"dst_hostname"`
	DestPort       int    `json:"dst_port"`
	ACLProfileID   string `json:"acl_profile_id,omitempty"`
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
	ID   string `json:"id"`
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
	params.Set("version", "6")
	params.Set("method", "login")
	params.Set("account", c.username)
	params.Set("passwd", c.password)
	params.Set("session", "SynologyReverseProxyOperator")
	params.Set("format", "sid")
	params.Set("enable_syno_token", "yes")

	resp, err := c.httpClient.PostForm(c.baseURL+authPath, params)
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
	params.Set("version", "6")
	params.Set("method", "logout")
	params.Set("session", "SynologyReverseProxyOperator")
	c.addAuth(&params)

	resp, err := c.httpClient.PostForm(c.baseURL+authPath, params)
	if err != nil {
		return fmt.Errorf("synology logout request failed: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// listReverseProxyRecords returns all existing reverse proxy records.
func (c *Client) listReverseProxyRecords() ([]ReverseProxyEntry, error) {
	params := url.Values{}
	params.Set("api", apiReverseProxy)
	params.Set("version", "1")
	params.Set("method", "list")
	c.addAuth(&params)

	resp, err := c.httpClient.PostForm(c.baseURL+entryPoint, params)
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
		List []ReverseProxyEntry `json:"list"`
	}
	if err := json.Unmarshal(result.Data, &data); err != nil {
		return nil, fmt.Errorf("list reverse proxy data parse failed: %w", err)
	}
	return data.List, nil
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

	params := url.Values{}
	params.Set("api", apiReverseProxy)
	params.Set("version", "1")
	params.Set("description", spec.Description)
	params.Set("src_scheme", spec.SourceScheme)
	params.Set("src_hostname", spec.SourceHostname)
	params.Set("src_port", fmt.Sprintf("%d", spec.SourcePort))
	params.Set("dst_scheme", spec.DestScheme)
	params.Set("dst_hostname", spec.DestHostname)
	params.Set("dst_port", fmt.Sprintf("%d", spec.DestPort))
	if spec.ACLProfileID != "" {
		params.Set("acl_profile_id", spec.ACLProfileID)
	}
	c.addAuth(&params)

	if existing != nil {
		params.Set("method", "set")
		params.Set("id", existing.UUID)
	} else {
		params.Set("method", "create")
	}

	resp, err := c.httpClient.PostForm(c.baseURL+entryPoint, params)
	if err != nil {
		return "", fmt.Errorf("upsert reverse proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("upsert reverse proxy read body failed: %w", err)
	}

	var result synologyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("upsert reverse proxy decode failed: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("upsert reverse proxy failed: %s", errorMsg(result.Error))
	}

	// On update the UUID is already known; on create the API returns it.
	if existing != nil {
		return existing.UUID, nil
	}

	var data struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(result.Data, &data); err != nil {
		// Some DSM versions return the id at the top level of data as a plain string.
		var rawID string
		if err2 := json.Unmarshal(result.Data, &rawID); err2 == nil {
			return rawID, nil
		}
		return "", fmt.Errorf("upsert reverse proxy parse uuid failed: %w", err)
	}
	return data.ID, nil
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

	params := url.Values{}
	params.Set("api", apiReverseProxy)
	params.Set("version", "1")
	params.Set("method", "delete")
	params.Set("id", existing.UUID)
	c.addAuth(&params)

	resp, err := c.httpClient.PostForm(c.baseURL+entryPoint, params)
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
	params := url.Values{}
	params.Set("api", apiCertificate)
	params.Set("version", "1")
	params.Set("method", "list")
	c.addAuth(&params)

	resp, err := c.httpClient.PostForm(c.baseURL+entryPoint, params)
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

// AssignCertificate assigns a certificate to a reverse proxy record.
// It finds the matching cert for the given hostname and updates the Synology
// certificate service binding so the proxy uses that cert.
func (c *Client) AssignCertificate(proxyUUID, hostname string) error {
	certID, _, err := c.FindMatchingCert(hostname)
	if err != nil {
		return err
	}
	if certID == "" {
		return fmt.Errorf("no matching certificate found for hostname %q", hostname)
	}

	// Fetch current certificate info to get the existing services list.
	params := url.Values{}
	params.Set("api", apiCertificate)
	params.Set("version", "1")
	params.Set("method", "get")
	params.Set("id", certID)
	c.addAuth(&params)

	resp, err := c.httpClient.PostForm(c.baseURL+entryPoint, params)
	if err != nil {
		return fmt.Errorf("get certificate request failed: %w", err)
	}
	defer resp.Body.Close()

	var getResult synologyResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResult); err != nil {
		return fmt.Errorf("get certificate decode failed: %w", err)
	}
	if !getResult.Success {
		return fmt.Errorf("get certificate failed: %s", errorMsg(getResult.Error))
	}

	// Build the service binding payload.
	// Synology expects a JSON array of service objects under the "services" key.
	type serviceEntry struct {
		Owner       string `json:"owner"`
		Service     string `json:"service"`
		DisplayName string `json:"display_name"`
		Subscriber  string `json:"subscriber"`
	}

	newService := serviceEntry{
		Owner:       "ReverseProxy",
		Service:     proxyUUID,
		DisplayName: hostname,
		Subscriber:  "ReverseProxy",
	}

	servicesJSON, err := json.Marshal([]serviceEntry{newService})
	if err != nil {
		return fmt.Errorf("marshal certificate services failed: %w", err)
	}

	setParams := url.Values{}
	setParams.Set("api", apiCertificate)
	setParams.Set("version", "1")
	setParams.Set("method", "set")
	setParams.Set("id", certID)
	setParams.Set("services", string(servicesJSON))
	c.addAuth(&setParams)

	setResp, err := c.httpClient.PostForm(c.baseURL+entryPoint, setParams)
	if err != nil {
		return fmt.Errorf("assign certificate request failed: %w", err)
	}
	defer setResp.Body.Close()

	var setResult synologyResponse
	if err := json.NewDecoder(setResp.Body).Decode(&setResult); err != nil {
		return fmt.Errorf("assign certificate decode failed: %w", err)
	}
	if !setResult.Success {
		return fmt.Errorf("assign certificate failed: %s", errorMsg(setResult.Error))
	}
	return nil
}

// GetACLProfileID returns the ID of an ACL profile by name.
// Returns an empty string (and no error) when the profile is not found.
func (c *Client) GetACLProfileID(profileName string) (string, error) {
	if profileName == "" {
		return "", nil
	}

	params := url.Values{}
	params.Set("api", apiACL)
	params.Set("version", "1")
	params.Set("method", "list")
	c.addAuth(&params)

	resp, err := c.httpClient.PostForm(c.baseURL+entryPoint, params)
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
		List []aclProfile `json:"list"`
	}
	if err := json.Unmarshal(result.Data, &data); err != nil {
		return "", fmt.Errorf("list ACL profiles data parse failed: %w", err)
	}

	for _, p := range data.List {
		if p.Name == profileName {
			return p.ID, nil
		}
	}
	return "", nil
}

// addAuth injects the SID and SynoToken into a request's form values.
func (c *Client) addAuth(params *url.Values) {
	if c.sid != "" {
		params.Set("_sid", c.sid)
	}
	if c.synoToken != "" {
		params.Set("SynoToken", c.synoToken)
	}
}

// certMatchesHostname checks whether a certificate covers the given hostname,
// including wildcard matching (*.example.com matches sub.example.com).
func certMatchesHostname(cert CertEntry, hostname string) bool {
	candidates := append([]string{cert.Subject.CommonName}, cert.Subject.AltNames...)
	for _, name := range candidates {
		if name == hostname {
			return true
		}
		// Wildcard match: *.example.com should match sub.example.com
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
