package controllers

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// handleDeletion handles Ingress deletion with finalizer
func (r *IngressReconciler) handleDeletion(ctx context.Context, ingress *networkingv1.Ingress) (ctrl.Result, error) {
	log := r.Logger.WithValues("ingress", client.ObjectKeyFromObject(ingress))

	if !hasFinalizer(ingress) {
		log.Debug("No finalizer present, allowing deletion")
		return ctrl.Result{}, nil
	}

	// Check deletion policy
	policy := getDeletionPolicy(ingress)
	log.Info("Processing deletion", "policy", policy)

	if policy == "delete" {
		// Find and delete proxy record
		existing, err := r.findExistingRecord(ctx, ingress)
		if err != nil {
			log.Error("Failed to query existing record during deletion", err)
			return ctrl.Result{}, fmt.Errorf("failed to query existing record: %w", err)
		}

		if existing != nil {
			log.Info("Deleting proxy record", "uuid", existing.UUID)
			if err := r.SynologyClient.Proxy.Delete(ctx, existing.UUID); err != nil {
				log.Error("Failed to delete proxy record", err)
				return ctrl.Result{}, fmt.Errorf("failed to delete proxy record: %w", err)
			}
			log.Info("Proxy record deleted successfully", "uuid", existing.UUID)
			r.Recorder.Eventf(ingress, "Normal", "ProxyRecordDeleted",
				"Proxy record %s deleted", existing.UUID)
		} else {
			log.Info("No proxy record found to delete")
		}
	} else {
		log.Info("Retaining proxy record per deletion policy")
		r.Recorder.Event(ingress, "Normal", "ProxyRecordRetained",
			"Proxy record retained per deletion policy")
	}

	// Remove finalizer
	log.Info("Removing finalizer")
	if err := r.removeFinalizer(ctx, ingress); err != nil {
		log.Error("Failed to remove finalizer", err)
		return ctrl.Result{}, err
	}

	log.Info("Finalizer removed, Ingress can be deleted")
	return ctrl.Result{}, nil
}

// addFinalizer adds the operator finalizer to the Ingress
func (r *IngressReconciler) addFinalizer(ctx context.Context, ingress *networkingv1.Ingress) error {
	controllerutil.AddFinalizer(ingress, FinalizerName)
	if err := r.Update(ctx, ingress); err != nil {
		return fmt.Errorf("failed to add finalizer: %w", err)
	}
	return nil
}

// removeFinalizer removes the operator finalizer from the Ingress
func (r *IngressReconciler) removeFinalizer(ctx context.Context, ingress *networkingv1.Ingress) error {
	controllerutil.RemoveFinalizer(ingress, FinalizerName)
	if err := r.Update(ctx, ingress); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}
	return nil
}

// hasFinalizer checks if the Ingress has the operator finalizer
func hasFinalizer(ingress *networkingv1.Ingress) bool {
	return controllerutil.ContainsFinalizer(ingress, FinalizerName)
}

// getDeletionPolicy gets the deletion policy from annotation
func getDeletionPolicy(ingress *networkingv1.Ingress) string {
	if ingress.Annotations == nil {
		return "delete"
	}

	policy, ok := ingress.Annotations[DeletionPolicyAnnotation]
	if !ok {
		return "delete"
	}

	// Validate policy value
	switch policy {
	case "delete", "retain":
		return policy
	default:
		// Invalid value defaults to delete
		return "delete"
	}
}
