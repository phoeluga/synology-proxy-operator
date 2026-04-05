package controller

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	proxyv1alpha1 "github.com/phoeluga/synology-proxy-operator/api/v1alpha1"
)

// hasManualSPRInNamespace returns true when at least one SynologyProxyRule in
// the given namespace was NOT created by the operator's auto-discovery logic
// (i.e. it lacks the "app.kubernetes.io/managed-by: synology-proxy-operator"
// label that the operator stamps on every rule it creates automatically).
//
// The namespace to check is always the SOURCE namespace (where the Service,
// Ingress, or ArgoCD Application destination resources live) — NOT the rule
// namespace. This ensures the per-namespace suppression is scoped correctly:
// a manual SPR in app-pihole suppresses only app-pihole, not app-homeassistant,
// even when both auto-created rules live in a centralised ruleNamespace.
func hasManualSPRInNamespace(ctx context.Context, c client.Client, namespace string) bool {
	list := &proxyv1alpha1.SynologyProxyRuleList{}
	if err := c.List(ctx, list, client.InNamespace(namespace)); err != nil {
		return false
	}
	for i := range list.Items {
		if list.Items[i].Labels["app.kubernetes.io/managed-by"] != "synology-proxy-operator" {
			return true
		}
	}
	return false
}
