package certificate

import (
	"context"
	"fmt"
	"strings"

	"github.com/phoeluga/synology-proxy-operator/pkg/logging"
	"github.com/phoeluga/synology-proxy-operator/pkg/synology"
)

// Matcher matches hostnames to certificates
type Matcher struct {
	synologyClient *synology.Client
	logger         logging.Logger
	// TODO: Add MetricsRegistry when Unit 4 is implemented
}

// NewMatcher creates a new certificate matcher
func NewMatcher(synologyClient *synology.Client, logger logging.Logger) *Matcher {
	return &Matcher{
		synologyClient: synologyClient,
		logger:         logger,
	}
}

// Match finds a certificate matching the hostname
func (m *Matcher) Match(ctx context.Context, hostname string) (*synology.Certificate, error) {
	log := m.logger.WithValues("hostname", hostname)

	// Get certificates (cached by SynologyClient)
	certs, err := m.synologyClient.Certificate.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}

	log.Debug("Matching certificate", "total_certs", len(certs))

	// Try exact match first
	for _, cert := range certs {
		if exactMatch(hostname, &cert) {
			// TODO: Record metric when Unit 4 is implemented
			log.Debug("Certificate matched (exact)", "cert_id", cert.ID, "cert_name", cert.Name)
			return &cert, nil
		}
	}

	// Try wildcard match
	for _, cert := range certs {
		if wildcardMatch(hostname, &cert) {
			// TODO: Record metric when Unit 4 is implemented
			log.Debug("Certificate matched (wildcard)", "cert_id", cert.ID, "cert_name", cert.Name)
			return &cert, nil
		}
	}

	// No match
	// TODO: Record metric when Unit 4 is implemented
	log.Debug("No certificate matched")
	return nil, nil
}

// exactMatch checks for exact CN or SAN match
func exactMatch(hostname string, cert *synology.Certificate) bool {
	// Check Common Name
	if cert.CommonName == hostname {
		return true
	}

	// Check Subject Alternative Names
	for _, san := range cert.SubjectAltNames {
		if san == hostname {
			return true
		}
	}

	return false
}

// wildcardMatch checks for wildcard match
func wildcardMatch(hostname string, cert *synology.Certificate) bool {
	// Check if certificate is a wildcard certificate
	if !cert.IsWildcard {
		return false
	}

	// Check CN
	if strings.HasPrefix(cert.CommonName, "*.") && matchWildcardPattern(hostname, cert.CommonName) {
		return true
	}

	// Check SANs
	for _, san := range cert.SubjectAltNames {
		if strings.HasPrefix(san, "*.") && matchWildcardPattern(hostname, san) {
			return true
		}
	}

	return false
}

// matchWildcardPattern matches hostname against wildcard pattern
// Pattern must be in format "*.example.com"
// Hostname must be in format "subdomain.example.com" (exactly one additional label)
func matchWildcardPattern(hostname, pattern string) bool {
	if !strings.HasPrefix(pattern, "*.") {
		return false
	}

	baseDomain := pattern[2:] // Remove "*."

	// Hostname must end with base domain
	if !strings.HasSuffix(hostname, baseDomain) {
		return false
	}

	// Extract prefix (everything before base domain)
	prefix := strings.TrimSuffix(hostname, "."+baseDomain)

	// Prefix must not contain dots (exactly one label)
	if strings.Contains(prefix, ".") {
		return false // More than one additional label
	}

	// Prefix must not be empty
	if prefix == "" {
		return false
	}

	return true
}

// MatchMultiple finds all certificates matching the hostname
// Useful for debugging or when multiple matches are acceptable
func (m *Matcher) MatchMultiple(ctx context.Context, hostname string) ([]*synology.Certificate, error) {
	log := m.logger.WithValues("hostname", hostname)

	// Get certificates (cached by SynologyClient)
	certs, err := m.synologyClient.Certificate.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}

	var matches []*synology.Certificate

	// Find all exact matches
	for i := range certs {
		if exactMatch(hostname, &certs[i]) {
			matches = append(matches, &certs[i])
		}
	}

	// Find all wildcard matches
	for i := range certs {
		if wildcardMatch(hostname, &certs[i]) {
			// Check if already added (avoid duplicates)
			found := false
			for _, m := range matches {
				if m.ID == certs[i].ID {
					found = true
					break
				}
			}
			if !found {
				matches = append(matches, &certs[i])
			}
		}
	}

	log.Debug("Multiple certificate matches", "count", len(matches))
	return matches, nil
}

// ValidatePattern validates a wildcard pattern
func ValidatePattern(pattern string) error {
	if !strings.HasPrefix(pattern, "*.") {
		return fmt.Errorf("wildcard pattern must start with '*.'")
	}

	baseDomain := pattern[2:]
	if baseDomain == "" {
		return fmt.Errorf("wildcard pattern must have a base domain")
	}

	if strings.Contains(baseDomain, "*") {
		return fmt.Errorf("wildcard pattern can only have one wildcard at the beginning")
	}

	return nil
}
