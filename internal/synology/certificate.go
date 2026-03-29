package synology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const certEndpoint = "/webapi/entry.cgi"

// FindMatchingCertID returns the id and description of the first DSM certificate
// whose CN or SAN matches hostname (including wildcard patterns like *.example.com).
func (c *Client) FindMatchingCertID(ctx context.Context, hostname string) (id, desc string, err error) {
	data, err := c.post(ctx, certEndpoint, url.Values{
		"api":     {"SYNO.Core.Certificate.CRT"},
		"method":  {"list"},
		"version": {"1"},
	})
	if err != nil {
		return "", "", fmt.Errorf("listing certificates: %w", err)
	}

	var result struct {
		Certificates []Certificate `json:"certificates"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", "", fmt.Errorf("parsing certificate list: %w", err)
	}

	for _, cert := range result.Certificates {
		patterns := append([]string{cert.Subject.CommonName}, cert.Subject.SubAltName...)
		for _, pattern := range patterns {
			if matchesCertPattern(pattern, hostname) {
				return cert.ID, cert.Desc, nil
			}
		}
	}
	return "", "", nil
}

// matchesCertPattern returns true when pattern covers hostname.
// Supports wildcard patterns like *.example.com.
func matchesCertPattern(pattern, hostname string) bool {
	if pattern == hostname {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[2:] // e.g. "example.com"
		return strings.HasSuffix(hostname, "."+suffix) || hostname == suffix
	}
	return false
}

// AssignCertificate assigns the best matching certificate to a reverse proxy record.
// proxyUUID is the DSM UUID of the proxy record; hostname is the public FQDN.
func (c *Client) AssignCertificate(ctx context.Context, proxyUUID, hostname string) error {
	certID, certDesc, err := c.FindMatchingCertID(ctx, hostname)
	if err != nil {
		return fmt.Errorf("finding certificate for %s: %w", hostname, err)
	}
	if certID == "" {
		c.log.Info("No matching certificate found, skipping assignment", "hostname", hostname)
		return nil
	}

	c.log.Info("Assigning certificate to proxy record",
		"cert", certDesc, "hostname", hostname, "proxyUUID", proxyUUID)

	settings := []map[string]interface{}{
		{
			"service": map[string]interface{}{
				"display_name":  hostname,
				"isPkg":         false,
				"multiple_cert": true,
				"owner":         "root",
				"service":       proxyUUID,
				"subscriber":    "ReverseProxy",
				"user_setable":  true,
			},
			"old_id": "",
			"id":     certID,
		},
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshalling certificate settings: %w", err)
	}

	_, err = c.post(ctx, certEndpoint, url.Values{
		"api":      {"SYNO.Core.Certificate.Service"},
		"method":   {"set"},
		"version":  {"1"},
		"settings": {string(settingsJSON)},
	})
	if err != nil {
		return fmt.Errorf("assigning certificate to %s: %w", hostname, err)
	}

	c.log.Info("Certificate assigned successfully", "cert", certDesc, "hostname", hostname)
	return nil
}
