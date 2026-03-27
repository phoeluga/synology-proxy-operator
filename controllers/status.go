package controllers

import (
	"context"
	"fmt"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IngressStatus represents the status we want to set on the Ingress
// Note: We use LoadBalancer status as a workaround since Ingress doesn't have custom status fields
type IngressStatus struct {
	Ready           bool
	Message         string
	ProxyRecordUUID string
	CertificateID   string
	LastUpdateTime  time.Time
}

// updateStatus updates the Ingress status based on reconciliation result
func (r *IngressReconciler) updateStatus(ctx context.Context, ingress *networkingv1.Ingress, result *ReconcileResult, err error) error {
	log := r.Logger.WithValues("ingress", client.ObjectKeyFromObject(ingress))

	// Build status
	status := buildStatus(result, err)

	// Update status annotation (since Ingress doesn't have custom status fields)
	if ingress.Annotations == nil {
		ingress.Annotations = make(map[string]string)
	}

	// Store status in annotations
	ingress.Annotations["synology.io/status"] = status.Message
	ingress.Annotations["synology.io/proxy-uuid"] = status.ProxyRecordUUID
	if status.CertificateID != "" {
		ingress.Annotations["synology.io/certificate-id"] = status.CertificateID
	}
	ingress.Annotations["synology.io/last-update"] = status.LastUpdateTime.Format(time.RFC3339)

	// Also update LoadBalancer status to show ready state
	if status.Ready {
		ingress.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{
			{
				Hostname: "synology-proxy",
			},
		}
	} else {
		ingress.Status.LoadBalancer.Ingress = nil
	}

	// Update the Ingress
	if err := r.Status().Update(ctx, ingress); err != nil {
		log.Error("Failed to update Ingress status", err)
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Also update annotations (separate from status)
	if err := r.Update(ctx, ingress); err != nil {
		log.Error("Failed to update Ingress annotations", err)
		return fmt.Errorf("failed to update annotations: %w", err)
	}

	log.Debug("Status updated successfully", "ready", status.Ready, "message", status.Message)
	return nil
}

// buildStatus builds the status from reconciliation result
func buildStatus(result *ReconcileResult, err error) *IngressStatus {
	status := &IngressStatus{
		LastUpdateTime: time.Now(),
	}

	if err != nil {
		status.Ready = false
		status.Message = fmt.Sprintf("Error: %s", err.Error())
		return status
	}

	if result == nil {
		status.Ready = false
		status.Message = "Unknown"
		return status
	}

	status.ProxyRecordUUID = result.ProxyRecordUUID
	status.CertificateID = result.CertificateID

	switch result.Action {
	case "Create":
		status.Ready = true
		status.Message = fmt.Sprintf("Proxy record created (UUID: %s)", result.ProxyRecordUUID)
		if result.CertificateID != "" {
			status.Message += fmt.Sprintf(", certificate assigned (ID: %s)", result.CertificateID)
		}
	case "Update":
		status.Ready = true
		status.Message = fmt.Sprintf("Proxy record updated (UUID: %s)", result.ProxyRecordUUID)
		if result.CertificateID != "" {
			status.Message += fmt.Sprintf(", certificate assigned (ID: %s)", result.CertificateID)
		}
	case "NoOp":
		status.Ready = true
		status.Message = fmt.Sprintf("Proxy record up to date (UUID: %s)", result.ProxyRecordUUID)
		if result.CertificateID != "" {
			status.Message += fmt.Sprintf(", certificate: %s", result.CertificateID)
		}
	case "Delete":
		status.Ready = false
		status.Message = "Proxy record deleted"
	case "Error":
		status.Ready = false
		if result.Error != nil {
			status.Message = fmt.Sprintf("Error: %s", result.Error.Error())
		} else {
			status.Message = "Error occurred"
		}
	default:
		status.Ready = false
		status.Message = "Unknown status"
	}

	return status
}

// GetIngressStatus retrieves the status from Ingress annotations
func GetIngressStatus(ingress *networkingv1.Ingress) *IngressStatus {
	if ingress.Annotations == nil {
		return &IngressStatus{
			Ready:   false,
			Message: "No status available",
		}
	}

	status := &IngressStatus{
		Message:         ingress.Annotations["synology.io/status"],
		ProxyRecordUUID: ingress.Annotations["synology.io/proxy-uuid"],
		CertificateID:   ingress.Annotations["synology.io/certificate-id"],
	}

	// Parse last update time
	if lastUpdate, ok := ingress.Annotations["synology.io/last-update"]; ok {
		if t, err := time.Parse(time.RFC3339, lastUpdate); err == nil {
			status.LastUpdateTime = t
		}
	}

	// Determine ready state from LoadBalancer status
	status.Ready = len(ingress.Status.LoadBalancer.Ingress) > 0

	return status
}

// SetIngressCondition sets a condition on the Ingress status
// Note: This is a helper for future use if we add custom conditions
func SetIngressCondition(ingress *networkingv1.Ingress, conditionType string, status metav1.ConditionStatus, reason, message string) {
	// Ingress doesn't have conditions in the standard API
	// This is a placeholder for potential future use with custom resources
	// For now, we use annotations to store status information
}
