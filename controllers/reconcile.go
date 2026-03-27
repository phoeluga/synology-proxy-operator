package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/phoeluga/synology-proxy-operator/pkg/synology"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// reconcileProxyRecord handles create/update of proxy record
func (r *IngressReconciler) reconcileProxyRecord(ctx context.Context, ingress *networkingv1.Ingress) (*ReconcileResult, error) {
	log := r.Logger.WithValues("ingress", client.ObjectKeyFromObject(ingress))

	// Extract frontend hostname
	hostname, err := extractHostname(ingress)
	if err != nil {
		return &ReconcileResult{Action: "Error", RequeueAfter: 30 * time.Second}, fmt.Errorf("failed to extract hostname: %w", err)
	}
	log.Debug("Extracted hostname", "hostname", hostname)

	// Discover backend
	backend, err := r.BackendDiscovery.Discover(ingress)
	if err != nil {
		return &ReconcileResult{Action: "Error", RequeueAfter: 30 * time.Second}, fmt.Errorf("failed to discover backend: %w", err)
	}
	log.Debug("Discovered backend", "fqdn", backend.FQDN, "port", backend.Port)

	// Match certificate
	cert, err := r.CertMatcher.Match(ctx, hostname)
	if err != nil {
		log.Warn("Certificate matching failed", "error", err)
	}
	if cert == nil {
		log.Warn("No certificate matched for hostname", "hostname", hostname)
	} else {
		log.Debug("Certificate matched", "cert_id", cert.ID, "cert_name", cert.Name)
	}

	// Determine ACL profile
	aclProfile := getACLProfile(ingress, r.Config.Synology.DefaultACLProfile)
	log.Debug("Using ACL profile", "profile", aclProfile)

	// Build desired proxy record
	desired := &synology.ProxyRecord{
		FrontendHostname: hostname,
		FrontendPort:     443,
		FrontendProtocol: "https",
		BackendHostname:  backend.FQDN,
		BackendPort:      backend.Port,
		BackendProtocol:  backend.Protocol,
		ACLProfileName:   aclProfile,
		Description:      formatDescription(ingress),
		Enabled:          true,
	}

	// Query existing record
	existing, err := r.findExistingRecord(ctx, ingress)
	if err != nil {
		return &ReconcileResult{Action: "Error", RequeueAfter: 30 * time.Second}, fmt.Errorf("failed to query existing record: %w", err)
	}

	var result *ReconcileResult

	if existing == nil {
		// Create new record
		log.Info("Creating new proxy record")
		created, err := r.SynologyClient.Proxy.Create(ctx, desired)
		if err != nil {
			return &ReconcileResult{Action: "Error", RequeueAfter: 30 * time.Second}, fmt.Errorf("failed to create proxy record: %w", err)
		}

		log.Info("Created proxy record", "uuid", created.UUID)
		result = &ReconcileResult{
			Action:          "Create",
			ProxyRecordUUID: created.UUID,
		}

		// Assign certificate if matched
		if cert != nil {
			log.Info("Assigning certificate to proxy record", "cert_id", cert.ID)
			if err := r.SynologyClient.Certificate.Assign(ctx, created.UUID, cert.ID); err != nil {
				log.Warn("Failed to assign certificate", "error", err)
			} else {
				result.CertificateID = cert.ID
				log.Info("Certificate assigned successfully")
			}
		}
	} else {
		// Update if different
		if needsUpdate(existing, desired) {
			log.Info("Updating proxy record", "uuid", existing.UUID)
			desired.UUID = existing.UUID
			if err := r.SynologyClient.Proxy.Update(ctx, desired); err != nil {
				return &ReconcileResult{Action: "Error", RequeueAfter: 30 * time.Second}, fmt.Errorf("failed to update proxy record: %w", err)
			}

			log.Info("Updated proxy record", "uuid", existing.UUID)
			result = &ReconcileResult{
				Action:          "Update",
				ProxyRecordUUID: existing.UUID,
			}

			// Update certificate if needed
			if cert != nil && existing.CertificateID != cert.ID {
				log.Info("Updating certificate assignment", "old_cert", existing.CertificateID, "new_cert", cert.ID)
				if err := r.SynologyClient.Certificate.Assign(ctx, existing.UUID, cert.ID); err != nil {
					log.Warn("Failed to assign certificate", "error", err)
				} else {
					result.CertificateID = cert.ID
					log.Info("Certificate updated successfully")
				}
			} else if cert != nil {
				result.CertificateID = existing.CertificateID
			}
		} else {
			log.Debug("Proxy record is up to date")
			result = &ReconcileResult{
				Action:          "NoOp",
				ProxyRecordUUID: existing.UUID,
				CertificateID:   existing.CertificateID,
			}
		}
	}

	return result, nil
}

// extractHostname extracts the frontend hostname from Ingress
func extractHostname(ingress *networkingv1.Ingress) (string, error) {
	if len(ingress.Spec.Rules) == 0 {
		return "", fmt.Errorf("no rules in Ingress spec")
	}

	hostname := ingress.Spec.Rules[0].Host
	if hostname == "" {
		return "", fmt.Errorf("no host in first Ingress rule")
	}

	return hostname, nil
}

// formatDescription formats the description field for proxy record identification
func formatDescription(ingress *networkingv1.Ingress) string {
	return fmt.Sprintf("k8s:%s/%s:%s", ingress.Namespace, ingress.Name, ingress.UID)
}

// parseDescription parses the description field to extract namespace, name, and UID
func parseDescription(description string) (namespace, name, uid string, ok bool) {
	if !strings.HasPrefix(description, "k8s:") {
		return "", "", "", false
	}

	parts := strings.Split(description[4:], ":")
	if len(parts) != 2 {
		return "", "", "", false
	}

	namespaceName := strings.Split(parts[0], "/")
	if len(namespaceName) != 2 {
		return "", "", "", false
	}

	return namespaceName[0], namespaceName[1], parts[1], true
}

// findExistingRecord finds an existing proxy record for the Ingress
func (r *IngressReconciler) findExistingRecord(ctx context.Context, ingress *networkingv1.Ingress) (*synology.ProxyRecord, error) {
	log := r.Logger.WithValues("ingress", client.ObjectKeyFromObject(ingress))

	// List all proxy records
	records, err := r.SynologyClient.Proxy.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list proxy records: %w", err)
	}

	// Search for record with matching description
	expectedDesc := formatDescription(ingress)
	for _, record := range records {
		if record.Description == expectedDesc {
			log.Debug("Found existing proxy record", "uuid", record.UUID)
			return &record, nil
		}
	}

	log.Debug("No existing proxy record found")
	return nil, nil
}

// needsUpdate checks if the proxy record needs to be updated
func needsUpdate(existing, desired *synology.ProxyRecord) bool {
	return existing.FrontendHostname != desired.FrontendHostname ||
		existing.FrontendPort != desired.FrontendPort ||
		existing.FrontendProtocol != desired.FrontendProtocol ||
		existing.BackendHostname != desired.BackendHostname ||
		existing.BackendPort != desired.BackendPort ||
		existing.BackendProtocol != desired.BackendProtocol ||
		existing.ACLProfileName != desired.ACLProfileName ||
		existing.Enabled != desired.Enabled
}

// getACLProfile gets the ACL profile from annotation or default
func getACLProfile(ingress *networkingv1.Ingress, defaultProfile string) string {
	if ingress.Annotations == nil {
		return defaultProfile
	}

	if profile, ok := ingress.Annotations[ACLProfileAnnotation]; ok && profile != "" {
		return profile
	}

	return defaultProfile
}
