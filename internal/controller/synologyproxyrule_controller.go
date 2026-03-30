// Package controller contains the Kubernetes controllers for this operator.
package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	proxyv1alpha1 "github.com/phoeluga/synology-proxy-operator/api/v1alpha1"
	"github.com/phoeluga/synology-proxy-operator/internal/synology"
)

const (
	finalizerName = "proxy.synology.io/finalizer"
	requeueAfter  = 30 * time.Second
)

// annotationSourceHost is the annotation key on Service/Ingress objects that
// overrides the auto-derived source hostname for a proxy record.
// Value: full FQDN, e.g. "myapp.example.com"
const annotationSourceHost = "synology.proxy/source-host"

// SynologyProxyRuleReconciler reconciles SynologyProxyRule objects.
// It is responsible for syncing each rule's desired state with the Synology DSM API.
type SynologyProxyRuleReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Log            logr.Logger
	SynologyClient *synology.Client
	// DefaultACLProfile is applied when spec.aclProfile is empty.
	DefaultACLProfile string
	// DefaultDomain is used to derive a source hostname when spec.sourceHost is empty.
	// E.g. "example.com" → service "nginx" → "nginx.example.com".
	DefaultDomain string
}

// +kubebuilder:rbac:groups=proxy.synology.io,resources=synologyproxyrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=proxy.synology.io,resources=synologyproxyrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=proxy.synology.io,resources=synologyproxyrules/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch

// Reconcile is the main reconciliation loop for SynologyProxyRule.
func (r *SynologyProxyRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("synologyproxyrule", req.NamespacedName)

	rule := &proxyv1alpha1.SynologyProxyRule{}
	if err := r.Get(ctx, req.NamespacedName, rule); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion.
	if !rule.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, log, rule)
	}

	// Ensure finalizer is present.
	if !controllerutil.ContainsFinalizer(rule, finalizerName) {
		controllerutil.AddFinalizer(rule, finalizerName)
		if err := r.Update(ctx, rule); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcileUpsert(ctx, log, rule)
}

// reconcileDelete handles cleanup when a SynologyProxyRule is deleted.
// It removes all DSM records tracked in status.ManagedRecords, falling back
// to the primary description if status is empty (e.g. pre-migration objects).
func (r *SynologyProxyRuleReconciler) reconcileDelete(ctx context.Context, log logr.Logger, rule *proxyv1alpha1.SynologyProxyRule) (ctrl.Result, error) {
	descriptions := managedDescriptions(rule)
	for _, desc := range descriptions {
		log.Info("Deleting proxy record from DSM", "description", desc)
		if _, err := r.SynologyClient.DeleteProxyRecord(ctx, desc); err != nil {
			log.Error(err, "Failed to delete proxy record from DSM")
			r.setCondition(rule, proxyv1alpha1.ConditionSynced, metav1.ConditionFalse,
				proxyv1alpha1.ReasonSyncFailed, err.Error())
			_ = r.Status().Update(ctx, rule)
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}
	}

	controllerutil.RemoveFinalizer(rule, finalizerName)
	if err := r.Update(ctx, rule); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// managedDescriptions returns the DSM description keys tracked in status.
func managedDescriptions(rule *proxyv1alpha1.SynologyProxyRule) []string {
	descs := make([]string, len(rule.Status.ManagedRecords))
	for i, r := range rule.Status.ManagedRecords {
		descs[i] = r.Description
	}
	return descs
}

// reconcileUpsert handles create/update of a SynologyProxyRule.
// It manages one DSM proxy record per source host (sourceHost + additionalSourceHosts).
func (r *SynologyProxyRuleReconciler) reconcileUpsert(ctx context.Context, log logr.Logger, rule *proxyv1alpha1.SynologyProxyRule) (ctrl.Result, error) {
	spec := rule.Spec

	// --- Backend discovery (shared across all source hosts) ---
	destHost := spec.DestinationHost
	destPort := spec.DestinationPort

	if destHost == "" || destPort == 0 {
		h, p, err := r.discoverBackend(ctx, log, rule)
		if err != nil {
			log.Error(err, "Backend discovery failed")
			r.setCondition(rule, proxyv1alpha1.ConditionReady, metav1.ConditionFalse,
				proxyv1alpha1.ReasonDiscoveryFailed, err.Error())
			_ = r.Status().Update(ctx, rule)
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}
		if destHost == "" {
			destHost = h
		}
		if destPort == 0 {
			destPort = p
		}
	}

	if destHost == "" || destPort == 0 {
		msg := "destination host or port could not be determined; set spec.destinationHost/destinationPort or provide a serviceRef/ingressRef"
		log.Info(msg)
		r.setCondition(rule, proxyv1alpha1.ConditionReady, metav1.ConditionFalse,
			proxyv1alpha1.ReasonDiscoveryFailed, msg)
		_ = r.Status().Update(ctx, rule)
		return ctrl.Result{RequeueAfter: 2 * requeueAfter}, nil
	}

	// --- ACL profile resolution ---
	aclProfileName := spec.ACLProfile
	if aclProfileName == "" {
		aclProfileName = r.DefaultACLProfile
	}
	aclID := ""
	if aclProfileName != "" {
		var err error
		aclID, err = r.SynologyClient.GetACLProfileID(ctx, aclProfileName)
		if err != nil {
			log.Error(err, "Failed to resolve ACL profile", "profile", aclProfileName)
			r.setCondition(rule, proxyv1alpha1.ConditionSynced, metav1.ConditionFalse,
				proxyv1alpha1.ReasonSyncFailed, fmt.Sprintf("ACL profile: %v", err))
			_ = r.Status().Update(ctx, rule)
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}
	}

	// --- Build base proxy rule template (backend + timeouts + headers are shared) ---
	baseRule := synology.ProxyRule{
		SourcePort:       spec.SourcePort,
		DestinationHost:  destHost,
		DestinationPort:  destPort,
		DestinationHTTPS: spec.DestinationProtocol == "https",
		ACLProfileID:     aclID,
	}
	if spec.Timeouts != nil {
		baseRule.ConnectTimeout = spec.Timeouts.Connect
		baseRule.ReadTimeout = spec.Timeouts.Read
		baseRule.SendTimeout = spec.Timeouts.Send
	}
	for _, h := range spec.CustomHeaders {
		baseRule.CustomHeaders = append(baseRule.CustomHeaders, synology.CustomHeader{
			Name:  h.Name,
			Value: h.Value,
		})
	}

	// --- Derive source hostname (may be auto-computed from Service/Ingress name + defaultDomain) ---
	primarySourceHost, err := r.deriveSourceHost(ctx, rule)
	if err != nil {
		log.Error(err, "Cannot determine source hostname")
		r.setCondition(rule, proxyv1alpha1.ConditionReady, metav1.ConditionFalse,
			proxyv1alpha1.ReasonDiscoveryFailed, err.Error())
		_ = r.Status().Update(ctx, rule)
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	// --- Build desired host→description mapping ---
	// Primary host keeps the base description.
	// Additional hosts get "<baseDesc>/<host>" so they're identifiable in the DSM UI.
	baseDesc := r.descriptionFor(rule)
	type hostEntry struct {
		desc       string
		sourceHost string
	}
	allHosts := make([]hostEntry, 0, 1+len(spec.AdditionalSourceHosts))
	allHosts = append(allHosts, hostEntry{baseDesc, primarySourceHost})
	for _, h := range spec.AdditionalSourceHosts {
		allHosts = append(allHosts, hostEntry{baseDesc + "/" + h, h})
	}

	// --- Clean up DSM records for hosts that were removed from the spec ---
	desiredDescs := make(map[string]struct{}, len(allHosts))
	for _, e := range allHosts {
		desiredDescs[e.desc] = struct{}{}
	}
	for _, managed := range rule.Status.ManagedRecords {
		if _, stillWanted := desiredDescs[managed.Description]; !stillWanted {
			log.Info("Removing stale DSM record for removed host", "description", managed.Description)
			if _, err := r.SynologyClient.DeleteProxyRecord(ctx, managed.Description); err != nil {
				log.Error(err, "Failed to delete stale proxy record (will retry)", "description", managed.Description)
			}
		}
	}

	// --- Upsert one DSM record per source host ---
	assignCert := spec.AssignCertificate == nil || *spec.AssignCertificate
	managedRecords := make([]proxyv1alpha1.ManagedRecord, 0, len(allHosts))

	for _, entry := range allHosts {
		pr := baseRule
		pr.Description = entry.desc
		pr.SourceHost = entry.sourceHost

		uuid, written, err := r.SynologyClient.UpsertProxyRule(ctx, pr)
		if err != nil {
			log.Error(err, "Failed to upsert proxy record", "description", entry.desc)
			r.setCondition(rule, proxyv1alpha1.ConditionSynced, metav1.ConditionFalse,
				proxyv1alpha1.ReasonSyncFailed, err.Error())
			_ = r.Status().Update(ctx, rule)
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}

		if assignCert && written && uuid != "" {
			if err := r.SynologyClient.AssignCertificate(ctx, uuid, entry.sourceHost); err != nil {
				log.Error(err, "Certificate assignment failed (non-fatal)", "hostname", entry.sourceHost)
			}
		}

		managedRecords = append(managedRecords, proxyv1alpha1.ManagedRecord{
			Description: entry.desc,
			UUID:        uuid,
			SourceHost:  entry.sourceHost,
		})
	}

	// --- Update status (only when something changed) ---
	statusChanged := rule.Status.ResolvedDestinationHost != destHost ||
		rule.Status.ResolvedDestinationPort != destPort ||
		!rule.Status.Synced ||
		!managedRecordsEqual(rule.Status.ManagedRecords, managedRecords)

	if statusChanged {
		now := metav1.Now()
		rule.Status.ManagedRecords = managedRecords
		rule.Status.Synced = true
		rule.Status.LastSyncTime = &now
		rule.Status.ResolvedDestinationHost = destHost
		rule.Status.ResolvedDestinationPort = destPort
		r.setCondition(rule, proxyv1alpha1.ConditionSynced, metav1.ConditionTrue,
			proxyv1alpha1.ReasonSyncSuccess, "Proxy rule synced with Synology DSM")
		r.setCondition(rule, proxyv1alpha1.ConditionReady, metav1.ConditionTrue,
			proxyv1alpha1.ReasonSyncSuccess, "Backend discovered and proxy rule active")

		if err := r.Status().Update(ctx, rule); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("Successfully reconciled proxy rule",
		"sourceHost", primarySourceHost,
		"additionalHosts", len(spec.AdditionalSourceHosts),
		"destination", fmt.Sprintf("%s:%d", destHost, destPort),
		"records", len(managedRecords))

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// discoverBackend attempts to find the external IP and port from a referenced
// Service (LoadBalancer) or Ingress in the cluster.
func (r *SynologyProxyRuleReconciler) discoverBackend(ctx context.Context, log logr.Logger, rule *proxyv1alpha1.SynologyProxyRule) (host string, port int, err error) {
	spec := rule.Spec
	ns := rule.Namespace

	// 1. Try ServiceRef first.
	if spec.ServiceRef != nil {
		svcNS := spec.ServiceRef.Namespace
		if svcNS == "" {
			svcNS = ns
		}
		svc := &corev1.Service{}
		if err := r.Get(ctx, types.NamespacedName{Name: spec.ServiceRef.Name, Namespace: svcNS}, svc); err != nil {
			return "", 0, fmt.Errorf("getting service %s/%s: %w", svcNS, spec.ServiceRef.Name, err)
		}
		h, p := extractFromService(svc)
		if h != "" && p != 0 {
			log.Info("Discovered backend from Service", "service", spec.ServiceRef.Name, "host", h, "port", p)
			return h, p, nil
		}
		log.Info("Service found but has no external IP yet, will retry", "service", spec.ServiceRef.Name)
		return "", 0, nil
	}

	// 2. Try IngressRef.
	if spec.IngressRef != nil {
		ingNS := spec.IngressRef.Namespace
		if ingNS == "" {
			ingNS = ns
		}
		ing := &networkingv1.Ingress{}
		if err := r.Get(ctx, types.NamespacedName{Name: spec.IngressRef.Name, Namespace: ingNS}, ing); err != nil {
			return "", 0, fmt.Errorf("getting ingress %s/%s: %w", ingNS, spec.IngressRef.Name, err)
		}
		h, p := extractFromIngress(ing)
		if h != "" {
			log.Info("Discovered backend from Ingress", "ingress", spec.IngressRef.Name, "host", h, "port", p)
			return h, p, nil
		}
		log.Info("Ingress found but has no external IP yet, will retry", "ingress", spec.IngressRef.Name)
		return "", 0, nil
	}

	// 3. Auto-scan: find any LoadBalancer in same namespace with an ExternalIP.
	svcList := &corev1.ServiceList{}
	if err := r.List(ctx, svcList, client.InNamespace(ns)); err != nil {
		return "", 0, fmt.Errorf("listing services in %s: %w", ns, err)
	}
	for i := range svcList.Items {
		svc := &svcList.Items[i]
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}
		h, p := extractFromService(svc)
		if h != "" && p != 0 {
			log.Info("Auto-discovered backend from LoadBalancer service",
				"service", svc.Name, "host", h, "port", p)
			return h, p, nil
		}
	}

	return "", 0, nil
}

// extractFromService returns the external IP and first port of a LoadBalancer service.
func extractFromService(svc *corev1.Service) (host string, port int) {
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if ing.IP != "" {
			host = ing.IP
		} else if ing.Hostname != "" {
			host = ing.Hostname
		}
		if host != "" {
			break
		}
	}
	if host == "" {
		return "", 0
	}
	for _, p := range svc.Spec.Ports {
		if p.Port != 0 {
			return host, int(p.Port)
		}
	}
	return host, 0
}

// extractFromIngress returns the external IP and port (default 443) from an Ingress.
func extractFromIngress(ing *networkingv1.Ingress) (host string, port int) {
	for _, lb := range ing.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			return lb.IP, 443
		}
		if lb.Hostname != "" {
			return lb.Hostname, 443
		}
	}
	return "", 0
}

// managedRecordsEqual returns true if two ManagedRecord slices have the same content.
func managedRecordsEqual(a, b []proxyv1alpha1.ManagedRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// descriptionFor returns the DSM record description for a rule,
// falling back to the resource name.
func (r *SynologyProxyRuleReconciler) descriptionFor(rule *proxyv1alpha1.SynologyProxyRule) string {
	if rule.Spec.Description != "" {
		return rule.Spec.Description
	}
	return rule.Name
}

// deriveSourceHost returns the source hostname for a rule.
// When spec.sourceHost is set it is returned as-is.
// Otherwise the hostname is derived in priority order:
//  1. synology.proxy/source-host annotation on the referenced Service or Ingress
//  2. <name>.<defaultDomain> where name is the Service/Ingress/rule name
//  3. Error if defaultDomain is also empty
func (r *SynologyProxyRuleReconciler) deriveSourceHost(ctx context.Context, rule *proxyv1alpha1.SynologyProxyRule) (string, error) {
	if rule.Spec.SourceHost != "" {
		return rule.Spec.SourceHost, nil
	}

	ns := rule.Namespace

	if ref := rule.Spec.ServiceRef; ref != nil {
		svcNS := ref.Namespace
		if svcNS == "" {
			svcNS = ns
		}
		svc := &corev1.Service{}
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: svcNS}, svc); err == nil {
			if h := svc.Annotations[annotationSourceHost]; h != "" {
				return h, nil
			}
			if r.DefaultDomain != "" {
				return fmt.Sprintf("%s.%s", svc.Name, r.DefaultDomain), nil
			}
		}
	}

	if ref := rule.Spec.IngressRef; ref != nil {
		ingNS := ref.Namespace
		if ingNS == "" {
			ingNS = ns
		}
		ing := &networkingv1.Ingress{}
		if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ingNS}, ing); err == nil {
			if h := ing.Annotations[annotationSourceHost]; h != "" {
				return h, nil
			}
			if r.DefaultDomain != "" {
				return fmt.Sprintf("%s.%s", ing.Name, r.DefaultDomain), nil
			}
		}
	}

	// Fall back to rule name + default domain.
	if r.DefaultDomain != "" {
		return fmt.Sprintf("%s.%s", rule.Name, r.DefaultDomain), nil
	}

	return "", fmt.Errorf("spec.sourceHost is empty and no defaultDomain is configured; " +
		"set spec.sourceHost, add a synology.proxy/source-host annotation on the Service/Ingress, " +
		"or configure DEFAULT_DOMAIN on the operator")
}

// setCondition updates or appends a condition on the rule's status.
func (r *SynologyProxyRuleReconciler) setCondition(rule *proxyv1alpha1.SynologyProxyRule,
	condType string, status metav1.ConditionStatus, reason, message string) {

	meta.SetStatusCondition(&rule.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}

// SetupWithManager registers the controller with the Manager.
func (r *SynologyProxyRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&proxyv1alpha1.SynologyProxyRule{}).
		Complete(r)
}
