package synology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const certEndpoint = "/webapi/entry.cgi"

// listCertificates returns all certificates from DSM.
func (c *Client) listCertificates(ctx context.Context) ([]Certificate, error) {
	data, err := c.post(ctx, certEndpoint, url.Values{
		"api":     {"SYNO.Core.Certificate.CRT"},
		"method":  {"list"},
		"version": {"1"},
	})
	if err != nil {
		return nil, fmt.Errorf("listing certificates: %w", err)
	}

	var result struct {
		Certificates []Certificate `json:"certificates"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing certificate list: %w", err)
	}
	return result.Certificates, nil
}

// FindMatchingCertID returns the ID and description of the best certificate for hostname.
// Priority:
//  1. A certificate whose CN or SAN matches the hostname (including wildcards).
//  2. The DSM default certificate (is_default=true) as fallback.
//
// Returns empty strings only if no certificates exist at all.
func (c *Client) FindMatchingCertID(ctx context.Context, hostname string) (id, desc string, err error) {
	certs, err := c.listCertificates(ctx)
	if err != nil {
		return "", "", err
	}

	var defaultID, defaultDesc string
	for _, cert := range certs {
		if cert.IsDefault {
			defaultID = cert.ID
			defaultDesc = cert.Desc
		}
		patterns := append([]string{cert.Subject.CommonName}, cert.Subject.SubAltName...)
		for _, pattern := range patterns {
			if matchesCertPattern(pattern, hostname) {
				c.log.Info("Found matching certificate", "cert", cert.Desc, "pattern", pattern, "hostname", hostname)
				return cert.ID, cert.Desc, nil
			}
		}
	}

	if defaultID != "" {
		c.log.Info("No specific certificate matched, using DSM default", "cert", defaultDesc, "hostname", hostname)
		return defaultID, defaultDesc, nil
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
// Falls back to the DSM default certificate when no specific match is found.
// proxyUUID is the DSM UUID of the proxy record; hostname is the public FQDN.
func (c *Client) AssignCertificate(ctx context.Context, proxyUUID, hostname string) error {
	certID, certDesc, err := c.FindMatchingCertID(ctx, hostname)
	if err != nil {
		return fmt.Errorf("finding certificate for %s: %w", hostname, err)
	}
	if certID == "" {
		c.log.Info("No certificates found in DSM, skipping assignment", "hostname", hostname)
		return nil
	}

	c.log.Info("Assigning certificate to proxy record",
		"cert", certDesc, "hostname", hostname, "proxyUUID", proxyUUID)

	settings := []map[string]any{
		{
			"service": map[string]any{
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
