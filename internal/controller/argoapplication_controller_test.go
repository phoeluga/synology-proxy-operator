package controller_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	proxyv1alpha1 "github.com/phoeluga/synology-proxy-operator/api/v1alpha1"
	"github.com/phoeluga/synology-proxy-operator/internal/argo"
	"github.com/phoeluga/synology-proxy-operator/internal/controller"
)

func setupArgoController(mgr ctrl.Manager, watchNS, ruleNS, defaultDomain string) error {
	r := &controller.ArgoApplicationReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Log:            logr.Discard(),
		WatchNamespace: watchNS,
		RuleNamespace:  ruleNS,
		DefaultDomain:  defaultDomain,
	}
	return r.SetupWithManager(mgr)
}

func makeArgoApp(name, destNS string, annotations map[string]string) *argo.Application {
	return &argo.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "argocd",
			Annotations: annotations,
		},
		Spec: argo.ApplicationSpec{
			Destination: argo.ApplicationDestination{
				Namespace: destNS,
			},
		},
	}
}

// TestArgoAnnotation_CreatesSPR verifies that an ArgoCD Application annotated
// with synology.proxy/enabled=true causes an SPR to be created.
func TestArgoAnnotation_CreatesSPR(t *testing.T) {
	k8s, _ := startManagerWithArgo(t, func(mgr ctrl.Manager) error {
		return setupArgoController(mgr, "", "", "example.com")
	})

	ctx := context.Background()
	ns := "argocd"
	createNamespace(t, k8s, ns)

	app := makeArgoApp("myapp", "app-myapp", map[string]string{
		"synology.proxy/enabled": "true",
	})
	if err := k8s.Create(ctx, app); err != nil {
		t.Fatalf("creating application: %v", err)
	}

	// SPR should be in destination namespace (app-myapp) with name = app name
	sprKey := types.NamespacedName{Name: "myapp", Namespace: "app-myapp"}
	createNamespace(t, k8s, "app-myapp")

	eventually(t, func() error {
		spr := &proxyv1alpha1.SynologyProxyRule{}
		if err := k8s.Get(ctx, sprKey, spr); err != nil {
			return fmt.Errorf("SPR not found yet: %w", err)
		}
		return nil
	})
}

// TestArgoAnnotation_SourceHostFromDefaultDomain verifies that when no source-host
// annotation is set, the SPR sourceHost is derived from the app name + default domain.
func TestArgoAnnotation_SourceHostFromDefaultDomain(t *testing.T) {
	k8s, _ := startManagerWithArgo(t, func(mgr ctrl.Manager) error {
		return setupArgoController(mgr, "", "", "internal.example.com")
	})

	ctx := context.Background()
	ns := "argocd"
	destNS := "headlamp-ns"
	createNamespace(t, k8s, ns)
	createNamespace(t, k8s, destNS)

	app := makeArgoApp("headlamp", destNS, map[string]string{
		"synology.proxy/enabled": "true",
	})
	if err := k8s.Create(ctx, app); err != nil {
		t.Fatalf("creating application: %v", err)
	}

	sprKey := types.NamespacedName{Name: "headlamp", Namespace: destNS}

	eventually(t, func() error {
		spr := &proxyv1alpha1.SynologyProxyRule{}
		if err := k8s.Get(ctx, sprKey, spr); err != nil {
			return fmt.Errorf("SPR not found yet: %w", err)
		}
		if spr.Spec.SourceHost != "headlamp.internal.example.com" {
			return fmt.Errorf("unexpected sourceHost: %q", spr.Spec.SourceHost)
		}
		return nil
	})
}

// TestArgoWatchNamespace_AutoEnablesApp verifies that apps in namespaces matching
// WatchNamespace glob are auto-enabled without an annotation.
func TestArgoWatchNamespace_AutoEnablesApp(t *testing.T) {
	k8s, _ := startManagerWithArgo(t, func(mgr ctrl.Manager) error {
		return setupArgoController(mgr, "argocd", "", "example.com")
	})

	ctx := context.Background()
	ns := "argocd"
	destNS := "my-app"
	createNamespace(t, k8s, ns)
	createNamespace(t, k8s, destNS)

	// No synology.proxy/enabled annotation
	app := makeArgoApp("grafana", destNS, nil)
	if err := k8s.Create(ctx, app); err != nil {
		t.Fatalf("creating application: %v", err)
	}

	sprKey := types.NamespacedName{Name: "grafana", Namespace: destNS}

	eventually(t, func() error {
		return k8s.Get(ctx, sprKey, &proxyv1alpha1.SynologyProxyRule{})
	})
}

// TestArgoCentralRuleNamespace_PlacesSPRInConfiguredNamespace verifies that when
// RuleNamespace is set, the SPR is created there regardless of destination namespace.
func TestArgoCentralRuleNamespace_PlacesSPRInConfiguredNamespace(t *testing.T) {
	ruleNS := "proxy-rules"

	k8s, _ := startManagerWithArgo(t, func(mgr ctrl.Manager) error {
		return setupArgoController(mgr, "argocd", ruleNS, "example.com")
	})

	ctx := context.Background()
	appNS := "argocd"
	createNamespace(t, k8s, appNS)
	createNamespace(t, k8s, ruleNS)

	app := makeArgoApp("prometheus", "monitoring", map[string]string{
		"synology.proxy/enabled": "true",
	})
	if err := k8s.Create(ctx, app); err != nil {
		t.Fatalf("creating application: %v", err)
	}

	// SPR should be in ruleNS, not "monitoring"
	sprKey := types.NamespacedName{Name: "prometheus", Namespace: ruleNS}

	eventually(t, func() error {
		return k8s.Get(ctx, sprKey, &proxyv1alpha1.SynologyProxyRule{})
	})

	// Confirm it is NOT in "monitoring"
	wrongKey := types.NamespacedName{Name: "prometheus", Namespace: "monitoring"}
	err := k8s.Get(ctx, wrongKey, &proxyv1alpha1.SynologyProxyRule{})
	if err == nil {
		t.Error("SPR was created in destination namespace, should be in ruleNamespace")
	}
}

// TestArgoApp_AnnotationRemovalDeletesSPR verifies that removing the enabled
// annotation from an Application causes its SPR to be deleted. This tests the
// reconciler's deleteRuleIfExists path, which runs while the Application object
// still exists (annotation-removal trigger vs. object deletion).
//
// Note: Kubernetes GC-based SPR deletion (via owner reference when app is
// deleted in the same namespace) requires the kube-controller-manager to be
// running, which envtest does not provide. That path is exercised by e2e tests.
func TestArgoApp_AnnotationRemovalDeletesSPR(t *testing.T) {
	k8s, _ := startManagerWithArgo(t, func(mgr ctrl.Manager) error {
		// WatchNamespace does NOT match app namespace → annotation is sole gate.
		return setupArgoController(mgr, "other-ns", "", "example.com")
	})

	ctx := context.Background()
	ns := "argocd"
	destNS := "loki-app"
	createNamespace(t, k8s, ns)
	createNamespace(t, k8s, destNS)

	app := makeArgoApp("loki", destNS, map[string]string{
		"synology.proxy/enabled": "true",
	})
	if err := k8s.Create(ctx, app); err != nil {
		t.Fatalf("creating application: %v", err)
	}

	sprKey := types.NamespacedName{Name: "loki", Namespace: destNS}

	// Wait for SPR to exist.
	eventually(t, func() error {
		return k8s.Get(ctx, sprKey, &proxyv1alpha1.SynologyProxyRule{})
	})

	// Remove annotation — reconciler should delete the SPR.
	patch := client.MergeFrom(app.DeepCopy())
	delete(app.Annotations, "synology.proxy/enabled")
	if err := k8s.Patch(ctx, app, patch); err != nil {
		t.Fatalf("patching application: %v", err)
	}

	eventually(t, func() error {
		err := k8s.Get(ctx, sprKey, &proxyv1alpha1.SynologyProxyRule{})
		if err == nil {
			return fmt.Errorf("SPR still exists after annotation removal")
		}
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return err
	})
}
