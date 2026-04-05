package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	proxyv1alpha1 "github.com/phoeluga/synology-proxy-operator/api/v1alpha1"
)

// ServiceIngressReconciler watches Services and Ingresses annotated with
// synology.proxy/enabled=true and auto-creates/deletes SynologyProxyRule objects.
//
// Annotation contract on the Service or Ingress:
//
//	synology.proxy/enabled: "true"            — opt in
//	synology.proxy/source-host: "foo.example" — hostname override (optional)
//	synology.proxy/acl-profile: "LAN Only"    — ACL profile (optional)
//	synology.proxy/destination-protocol: "https" — backend protocol (optional)
//	synology.proxy/assign-certificate: "false"   — disable cert (optional)
type ServiceIngressReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
	RuleNamespace string
	// WatchNamespace is an optional glob pattern (e.g. "app-*"). When set, all
	// Services and Ingresses in matching namespaces are auto-managed without
	// requiring the synology.proxy/enabled annotation.
	WatchNamespace string
	// DisableAutoDiscoveryIfSPRExists suppresses glob-based auto-discovery for a
	// namespace when at least one manually-created SynologyProxyRule already
	// exists there. Resources with an explicit synology.proxy/enabled: "true"
	// annotation are still managed regardless of this flag.
	DisableAutoDiscoveryIfSPRExists bool
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch

// SetupWithManager registers two controllers — one for Services, one for Ingresses.
func (r *ServiceIngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		Named("service-proxy").
		For(&corev1.Service{}).
		Complete(&serviceReconcileAdapter{r}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("ingress-proxy").
		For(&networkingv1.Ingress{}).
		Complete(&ingressReconcileAdapter{r})
}

// ── Service adapter ──────────────────────────────────────────────────────────

type serviceReconcileAdapter struct{ r *ServiceIngressReconciler }

func (a *serviceReconcileAdapter) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	svc := &corev1.Service{}
	if err := a.r.Get(ctx, req.NamespacedName, svc); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	return a.r.reconcileObject(ctx, svc.Name, svc.Namespace, svc.Annotations, svc.DeletionTimestamp != nil,
		&proxyv1alpha1.ObjectRef{Name: svc.Name, Namespace: svc.Namespace}, nil)
}

// ── Ingress adapter ──────────────────────────────────────────────────────────

type ingressReconcileAdapter struct{ r *ServiceIngressReconciler }

func (a *ingressReconcileAdapter) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ing := &networkingv1.Ingress{}
	if err := a.r.Get(ctx, req.NamespacedName, ing); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	return a.r.reconcileObject(ctx, ing.Name, ing.Namespace, ing.Annotations, ing.DeletionTimestamp != nil,
		nil, &proxyv1alpha1.ObjectRef{Name: ing.Name, Namespace: ing.Namespace})
}

// ── Shared reconcile logic ───────────────────────────────────────────────────

func (r *ServiceIngressReconciler) reconcileObject(
	ctx context.Context,
	name, namespace string,
	annotations map[string]string,
	deleting bool,
	serviceRef *proxyv1alpha1.ObjectRef,
	ingressRef *proxyv1alpha1.ObjectRef,
) (ctrl.Result, error) {
	log := r.Log.WithValues("name", name, "namespace", namespace)

	ruleNS := r.RuleNamespace
	if ruleNS == "" {
		ruleNS = namespace
	}
	ruleName := ruleNameForObject(name, namespace)

	// Fetch namespace annotations to check synology.proxy/auto-discovery.
	var nsAnnotations map[string]string
	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: namespace}, ns); err == nil {
		nsAnnotations = ns.Annotations
	}

	// Not opted-in or being deleted → clean up if we own the rule.
	if deleting || !r.isResourceEnabled(namespace, annotations, nsAnnotations) {
		return ctrl.Result{}, r.deleteOwnedRule(ctx, log, ruleName, ruleNS, name, namespace)
	}

	// Glob-only auto-discovery: if a manual SPR exists in the source namespace,
	// back off and remove any rule we previously created for this object.
	if r.DisableAutoDiscoveryIfSPRExists && !isEnabled(annotations) {
		if hasManualSPRInNamespace(ctx, r.Client, namespace) {
			log.V(1).Info("Manual SPR found in namespace, suppressing auto-discovery", "namespace", namespace)
			return ctrl.Result{}, r.deleteOwnedRule(ctx, log, ruleName, ruleNS, name, namespace)
		}
	}

	return ctrl.Result{}, r.upsertRule(ctx, log, ruleName, ruleNS, name, namespace, annotations, serviceRef, ingressRef)
}

func (r *ServiceIngressReconciler) upsertRule(
	ctx context.Context,
	log logr.Logger,
	ruleName, ruleNS string,
	objectName, objectNamespace string,
	annotations map[string]string,
	serviceRef *proxyv1alpha1.ObjectRef,
	ingressRef *proxyv1alpha1.ObjectRef,
) error {
	desired := r.buildRule(ruleName, ruleNS, objectName, objectNamespace, annotations, serviceRef, ingressRef)

	existing := &proxyv1alpha1.SynologyProxyRule{}
	err := r.Get(ctx, client.ObjectKey{Name: ruleName, Namespace: ruleNS}, existing)
	if apierrors.IsNotFound(err) {
		log.Info("Creating SynologyProxyRule", "rule", ruleName)
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Only update if spec actually changed.
	if reflect.DeepEqual(existing.Spec, desired.Spec) {
		log.V(1).Info("SynologyProxyRule unchanged, skipping update", "rule", ruleName)
		return nil
	}
	existing.Spec = desired.Spec
	log.Info("Updating SynologyProxyRule", "rule", ruleName)
	return r.Update(ctx, existing)
}

func (r *ServiceIngressReconciler) buildRule(
	ruleName, ruleNS string,
	objectName, objectNamespace string,
	annotations map[string]string,
	serviceRef *proxyv1alpha1.ObjectRef,
	ingressRef *proxyv1alpha1.ObjectRef,
) *proxyv1alpha1.SynologyProxyRule {
	if annotations == nil {
		annotations = map[string]string{}
	}

	sourceHost := annotations[AnnotationSourceHost]
	aclProfile := annotations[AnnotationACLProfile]
	destProtocol := annotations[AnnotationDestProtocol]
	if destProtocol == "" {
		destProtocol = "http"
	}

	assignCert := true
	if v := annotations[AnnotationAssignCert]; strings.ToLower(v) == "false" {
		assignCert = false
	}

	return &proxyv1alpha1.SynologyProxyRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ruleName,
			Namespace: ruleNS,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":           "synology-proxy-operator",
				"proxy.synology.io/managed-by-object":    objectName,
				"proxy.synology.io/managed-by-object-ns": objectNamespace,
			},
		},
		Spec: proxyv1alpha1.SynologyProxyRuleSpec{
			SourceHost:          sourceHost, // empty = auto-derived by rule controller
			DestinationProtocol: destProtocol,
			ACLProfile:          aclProfile,
			AssignCertificate:   &assignCert,
			ServiceRef:          serviceRef,
			IngressRef:          ingressRef,
			Description:         ruleName,
		},
	}
}

// deleteOwnedRule deletes the SynologyProxyRule only if it carries our managed-by labels.
func (r *ServiceIngressReconciler) deleteOwnedRule(
	ctx context.Context,
	log logr.Logger,
	ruleName, ruleNS string,
	objectName, objectNamespace string,
) error {
	rule := &proxyv1alpha1.SynologyProxyRule{}
	err := r.Get(ctx, client.ObjectKey{Name: ruleName, Namespace: ruleNS}, rule)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// Only delete rules we created.
	if rule.Labels["proxy.synology.io/managed-by-object"] != objectName ||
		rule.Labels["proxy.synology.io/managed-by-object-ns"] != objectNamespace {
		log.V(1).Info("Rule not owned by this object, skipping delete", "rule", ruleName)
		return nil
	}

	log.Info("Deleting SynologyProxyRule for disabled/deleted object", "rule", ruleName)
	if err := r.Delete(ctx, rule); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// ruleNameForObject returns a stable rule name for a Service or Ingress.
// Format: "<namespace>--<name>" to avoid collisions across namespaces.
func ruleNameForObject(name, namespace string) string {
	return fmt.Sprintf("%s--%s", namespace, name)
}

// isResourceEnabled returns true when the resource should be managed.
//
// Decision order:
//  1. synology.proxy/enabled: "false" on the resource — always opts out, even
//     when the namespace glob matches.
//  2. synology.proxy/enabled: "true" on the resource — always opts in.
//  3. WatchNamespace glob matches AND the namespace does not carry
//     synology.proxy/auto-discovery: "false" — auto-managed via glob.
func (r *ServiceIngressReconciler) isResourceEnabled(namespace string, annotations map[string]string, namespaceAnnotations map[string]string) bool {
	// Explicit opt-out on the resource wins over everything.
	if strings.ToLower(annotations[AnnotationEnabled]) == "false" {
		return false
	}
	// Explicit opt-in on the resource always works.
	if isEnabled(annotations) {
		return true
	}
	// Namespace glob — only applies when the namespace has not disabled auto-discovery.
	return namespaceMatches(namespace, r.WatchNamespace) &&
		strings.ToLower(namespaceAnnotations[AnnotationAutoDiscovery]) != "false"
}

// isEnabled returns true when synology.proxy/enabled=true is set.
func isEnabled(annotations map[string]string) bool {
	return strings.ToLower(annotations[AnnotationEnabled]) == "true"
}
