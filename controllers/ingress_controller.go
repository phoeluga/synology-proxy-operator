package controllers

import (
	"context"
	"time"

	"github.com/phoeluga/synology-proxy-operator/pkg/certificate"
	"github.com/phoeluga/synology-proxy-operator/pkg/config"
	"github.com/phoeluga/synology-proxy-operator/pkg/filter"
	"github.com/phoeluga/synology-proxy-operator/pkg/logging"
	"github.com/phoeluga/synology-proxy-operator/pkg/synology"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// OperatorAnnotation is the annotation that enables operator management
	OperatorAnnotation = "synology.io/enabled"

	// FinalizerName is the finalizer added to managed Ingresses
	FinalizerName = "synology.io/proxy-record"

	// DeletionPolicyAnnotation specifies what to do with proxy record on deletion
	DeletionPolicyAnnotation = "synology.io/deletion-policy"

	// ACLProfileAnnotation specifies the ACL profile to use
	ACLProfileAnnotation = "synology.io/acl-profile"

	// BackendProtocolAnnotation specifies the backend protocol (http/https)
	BackendProtocolAnnotation = "synology.io/backend-protocol"
)

// IngressReconciler reconciles Ingress resources
type IngressReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	SynologyClient   *synology.Client
	CertMatcher      *certificate.Matcher
	BackendDiscovery *BackendDiscovery
	Config           *config.Config
	NamespaceFilter  filter.NamespaceFilter
	Logger           logging.Logger
	Recorder         record.EventRecorder
	// TODO: Add MetricsRegistry when Unit 4 is implemented
}

// ReconcileResult represents the result of a reconciliation
type ReconcileResult struct {
	Action          string // Create, Update, Delete, NoOp
	ProxyRecordUUID string
	CertificateID   string
	Error           error
	RequeueAfter    time.Duration
}

// Reconcile implements the reconciliation loop
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Logger.WithValues("ingress", req.NamespacedName)
	startTime := time.Now()

	// Fetch Ingress
	var ingress networkingv1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if errors.IsNotFound(err) {
			// Ingress deleted, nothing to do
			return ctrl.Result{}, nil
		}
		log.Error("Failed to fetch Ingress", err)
		return ctrl.Result{}, err
	}

	// Check if Ingress has operator annotation
	if !hasOperatorAnnotation(&ingress) {
		log.Debug("Ingress does not have operator annotation, ignoring")
		return ctrl.Result{}, nil
	}

	// Handle deletion
	if !ingress.DeletionTimestamp.IsZero() {
		log.Info("Ingress is being deleted")
		return r.handleDeletion(ctx, &ingress)
	}

	// Add finalizer if not present
	if !hasFinalizer(&ingress) {
		log.Info("Adding finalizer")
		if err := r.addFinalizer(ctx, &ingress); err != nil {
			log.Error("Failed to add finalizer", err)
			return ctrl.Result{}, err
		}
	}

	// Reconcile proxy record
	result, err := r.reconcileProxyRecord(ctx, &ingress)

	// Update status
	if statusErr := r.updateStatus(ctx, &ingress, result, err); statusErr != nil {
		log.Error("Failed to update status", statusErr)
	}

	// Record metrics
	duration := time.Since(startTime)
	// TODO: Record metrics when Unit 4 is implemented
	log.Debug("Reconciliation completed", "duration", duration, "action", result.Action)

	// Create event
	r.createEvent(&ingress, result, err)

	if err != nil {
		log.Error("Reconciliation failed", err)
		return ctrl.Result{RequeueAfter: result.RequeueAfter}, err
	}

	log.Info("Reconciliation successful", "action", result.Action, "uuid", result.ProxyRecordUUID)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			// Filter by namespace patterns
			return r.NamespaceFilter.Matches(object.GetNamespace())
		})).
		Complete(r)
}

// hasOperatorAnnotation checks if Ingress has the operator annotation
func hasOperatorAnnotation(ingress *networkingv1.Ingress) bool {
	if ingress.Annotations == nil {
		return false
	}
	value, ok := ingress.Annotations[OperatorAnnotation]
	return ok && value == "true"
}

// createEvent creates a Kubernetes Event for the Ingress
func (r *IngressReconciler) createEvent(ingress *networkingv1.Ingress, result *ReconcileResult, err error) {
	if err != nil {
		r.Recorder.Event(ingress, "Warning", "ReconciliationFailed", err.Error())
		return
	}

	switch result.Action {
	case "Create":
		r.Recorder.Eventf(ingress, "Normal", "ProxyRecordCreated",
			"Proxy record created with UUID %s", result.ProxyRecordUUID)
	case "Update":
		r.Recorder.Eventf(ingress, "Normal", "ProxyRecordUpdated",
			"Proxy record updated with UUID %s", result.ProxyRecordUUID)
	case "Delete":
		r.Recorder.Eventf(ingress, "Normal", "ProxyRecordDeleted",
			"Proxy record deleted with UUID %s", result.ProxyRecordUUID)
	case "NoOp":
		// Don't create event for no-op
	}

	if result.CertificateID != "" {
		r.Recorder.Eventf(ingress, "Normal", "CertificateAssigned",
			"Certificate %s assigned to proxy record", result.CertificateID)
	}
}
