package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	proxyv1alpha1 "github.com/phoeluga/synology-proxy-operator/api/v1alpha1"
	"github.com/phoeluga/synology-proxy-operator/internal/argo"
)

// Annotation keys recognised on ArgoCD Application objects.
const (
	// AnnotationEnabled enables proxy management for an application.
	// Value: "true" | "false"
	AnnotationEnabled = "synology.proxy/enabled"

	// AnnotationSourceHost overrides the public FQDN (frontend hostname).
	// When absent, the operator constructs it as <appName>.<defaultDomain>.
	AnnotationSourceHost = "synology.proxy/source-host"

	// AnnotationACLProfile overrides the ACL profile name for this application.
	AnnotationACLProfile = "synology.proxy/acl-profile"

	// AnnotationDestProtocol overrides the backend protocol ("http" or "https").
	AnnotationDestProtocol = "synology.proxy/destination-protocol"

	// AnnotationDestHost overrides the backend hostname / IP.
	AnnotationDestHost = "synology.proxy/destination-host"

	// AnnotationDestPort overrides the backend port.
	AnnotationDestPort = "synology.proxy/destination-port"

	// AnnotationAssignCert controls certificate assignment ("true"/"false").
	AnnotationAssignCert = "synology.proxy/assign-certificate"

	// AnnotationServiceRef selects a specific Service for backend discovery: "<namespace>/<name>"
	AnnotationServiceRef = "synology.proxy/service-ref"

	// AnnotationIngressRef selects a specific Ingress for backend discovery: "<namespace>/<name>"
	AnnotationIngressRef = "synology.proxy/ingress-ref"
)

// ArgoApplicationReconciler watches ArgoCD Application objects and creates or
// deletes SynologyProxyRule objects to reflect the desired proxy configuration.
type ArgoApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	// DefaultDomain is appended to the app name when no source-host annotation is present.
	// E.g. "example.com" produces "myapp.example.com".
	DefaultDomain string
	// WatchNamespace restricts the ArgoCD Applications this controller watches.
	// Empty string means all namespaces.
	WatchNamespace string
	// RuleNamespace is the namespace where SynologyProxyRule objects are created.
	RuleNamespace string
}

// +kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch

// Reconcile is the main loop for ArgoApplicationReconciler.
func (r *ArgoApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("application", req.NamespacedName)

	app := &argo.Application{}
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Only manage apps explicitly opted-in via annotation.
	if !isProxyEnabled(app) {
		// Clean up only rules this operator created (owner reference present).
		// Rules created manually via resources.yaml are left untouched.
		return ctrl.Result{}, r.deleteRuleIfExists(ctx, log, app)
	}

	if !app.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.deleteRuleIfExists(ctx, log, app)
	}

	return ctrl.Result{}, r.reconcileRule(ctx, log, app)
}

// reconcileRule creates or updates the SynologyProxyRule owned by this Application.
func (r *ArgoApplicationReconciler) reconcileRule(ctx context.Context, log logr.Logger, app *argo.Application) error {
	ruleName := ruleNameForApp(app)
	ruleNS := r.RuleNamespace
	if ruleNS == "" {
		ruleNS = app.Namespace
	}

	desired := r.buildRule(app, ruleName, ruleNS)

	existing := &proxyv1alpha1.SynologyProxyRule{}
	err := r.Get(ctx, client.ObjectKey{Name: ruleName, Namespace: ruleNS}, existing)

	if apierrors.IsNotFound(err) {
		log.Info("Creating SynologyProxyRule for Application", "rule", ruleName)
		// Owner references are only valid within the same namespace.
		// When app and rule are in different namespaces, ownership is tracked via labels.
		if app.Namespace == ruleNS {
			if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
				return fmt.Errorf("setting owner reference: %w", err)
			}
		}
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update spec if it differs.
	existing.Spec = desired.Spec
	log.Info("Updating SynologyProxyRule for Application", "rule", ruleName)
	return r.Update(ctx, existing)
}

// deleteRuleIfExists deletes a SynologyProxyRule only if it was created by this
// operator. Same-namespace rules are identified by owner reference; cross-namespace
// rules (Application in argocd, rule in synology-proxy-operator) by managed-by labels.
// Rules added manually have neither and are left alone.
func (r *ArgoApplicationReconciler) deleteRuleIfExists(ctx context.Context, log logr.Logger, app *argo.Application) error {
	rule := &proxyv1alpha1.SynologyProxyRule{}
	err := r.Get(ctx, client.ObjectKey{Name: ruleNameForApp(app), Namespace: r.RuleNamespace}, rule)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// Same-namespace: check owner reference. Cross-namespace: check labels.
	ownedByRef := metav1.IsControlledBy(rule, app)
	ownedByLabel := rule.Labels["proxy.synology.io/managed-by-argo-app"] == app.Name &&
		rule.Labels["proxy.synology.io/managed-by-argo-app-ns"] == app.Namespace
	if !ownedByRef && !ownedByLabel {
		log.V(1).Info("SynologyProxyRule not owned by this Application, skipping delete", "rule", rule.Name)
		return nil
	}
	log.Info("Deleting SynologyProxyRule for removed/disabled Application", "rule", rule.Name)
	if err := r.Delete(ctx, rule); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// buildRule constructs the desired SynologyProxyRule from an ArgoCD Application.
func (r *ArgoApplicationReconciler) buildRule(app *argo.Application, name, ns string) *proxyv1alpha1.SynologyProxyRule {
	annotations := app.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	// --- Source host ---
	sourceHost := annotations[AnnotationSourceHost]
	if sourceHost == "" && r.DefaultDomain != "" {
		sourceHost = fmt.Sprintf("%s.%s", app.Name, r.DefaultDomain)
	}

	// --- ACL profile ---
	aclProfile := annotations[AnnotationACLProfile]

	// --- Destination protocol ---
	destProtocol := annotations[AnnotationDestProtocol]
	if destProtocol == "" {
		destProtocol = "http"
	}

	// --- Assign certificate ---
	assignCert := true
	if v, ok := annotations[AnnotationAssignCert]; ok {
		assignCert = strings.ToLower(v) != "false"
	}

	rule := &proxyv1alpha1.SynologyProxyRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":             "synology-proxy-operator",
				"proxy.synology.io/managed-by-argo-app":    app.Name,
				"proxy.synology.io/managed-by-argo-app-ns": app.Namespace,
			},
		},
		Spec: proxyv1alpha1.SynologyProxyRuleSpec{
			SourceHost:          sourceHost,
			SourcePort:          443,
			DestinationHost:     annotations[AnnotationDestHost],
			DestinationProtocol: destProtocol,
			ACLProfile:          aclProfile,
			AssignCertificate:   &assignCert,
			ManagedByApp:        app.Name,
			Description:         name,
		},
	}

	// --- Destination port override ---
	if v := annotations[AnnotationDestPort]; v != "" {
		var p int
		if _, err := fmt.Sscanf(v, "%d", &p); err == nil {
			rule.Spec.DestinationPort = p
		}
	}

	// --- Service / Ingress refs ---
	if v := annotations[AnnotationServiceRef]; v != "" {
		parts := strings.SplitN(v, "/", 2)
		ref := &proxyv1alpha1.ObjectRef{}
		if len(parts) == 2 {
			ref.Namespace = parts[0]
			ref.Name = parts[1]
		} else {
			ref.Name = parts[0]
			ref.Namespace = app.Spec.Destination.Namespace
		}
		rule.Spec.ServiceRef = ref
	} else if v := annotations[AnnotationIngressRef]; v != "" {
		parts := strings.SplitN(v, "/", 2)
		ref := &proxyv1alpha1.ObjectRef{}
		if len(parts) == 2 {
			ref.Namespace = parts[0]
			ref.Name = parts[1]
		} else {
			ref.Name = parts[0]
			ref.Namespace = app.Spec.Destination.Namespace
		}
		rule.Spec.IngressRef = ref
	} else if app.Spec.Destination.Namespace != "" {
		// No explicit ref — let the rule controller auto-scan the destination namespace.
		// We convey the namespace by setting a ServiceRef with just the namespace.
		// The rule controller will auto-scan for a LoadBalancer in that namespace.
		// We achieve this by leaving ServiceRef nil and setting the rule namespace to match.
		// The rule's namespace defaults to the operator's RuleNamespace, so we propagate
		// the destination namespace through a label that the user can reference if needed.
		rule.Labels["proxy.synology.io/target-namespace"] = app.Spec.Destination.Namespace
	}

	return rule
}

// isProxyEnabled returns true if the application carries the enabled annotation.
func isProxyEnabled(app *argo.Application) bool {
	if app.Annotations == nil {
		return false
	}
	return strings.ToLower(app.Annotations[AnnotationEnabled]) == "true"
}

// ruleNameForApp returns the SynologyProxyRule name for an ArgoCD Application.
func ruleNameForApp(app *argo.Application) string {
	return app.Name
}

// SetupWithManager registers the controller with the Manager.
func (r *ArgoApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&argo.Application{}).
		Owns(&proxyv1alpha1.SynologyProxyRule{}).
		Complete(r)
}
