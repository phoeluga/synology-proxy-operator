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
// This is used to implement the DisableAutoDiscoveryIfSPRExists option: when
// a user places an explicit SPR in a namespace, the glob-based auto-discovery
// backs off and stops generating additional rules there.
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
