package controller_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	proxyv1alpha1 "github.com/phoeluga/synology-proxy-operator/api/v1alpha1"
	"github.com/phoeluga/synology-proxy-operator/internal/controller"
)

func setupServiceIngressController(mgr ctrl.Manager, watchNS, ruleNS string) error {
	r := &controller.ServiceIngressReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Log:            logr.Discard(),
		WatchNamespace: watchNS,
		RuleNamespace:  ruleNS,
	}
	return r.SetupWithManager(mgr)
}

// createNamespace creates a namespace if it doesn't exist.
func createNamespace(t *testing.T, k8s client.Client, name string) {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := k8s.Create(context.Background(), ns); err != nil {
		// Ignore already-exists
		if client.IgnoreNotFound(err) == nil {
			return
		}
		t.Logf("namespace %q may already exist: %v", name, err)
	}
}

// TestServiceAnnotation_CreatesSPR verifies that annotating a Service
// with synology.proxy/enabled=true causes a SynologyProxyRule to be created
// in the same namespace when ruleNamespace is empty.
func TestServiceAnnotation_CreatesSPR(t *testing.T) {
	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupServiceIngressController(mgr, "", "")
	})

	ctx := context.Background()
	ns := "test-svc-create"
	createNamespace(t, k8s, ns)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp",
			Namespace: ns,
			Annotations: map[string]string{
				"synology.proxy/enabled": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "myapp"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
	if err := k8s.Create(ctx, svc); err != nil {
		t.Fatalf("creating service: %v", err)
	}

	sprKey := types.NamespacedName{
		Name:      fmt.Sprintf("%s--myapp", ns),
		Namespace: ns,
	}

	eventually(t, func() error {
		spr := &proxyv1alpha1.SynologyProxyRule{}
		if err := k8s.Get(ctx, sprKey, spr); err != nil {
			return fmt.Errorf("SPR not found yet: %w", err)
		}
		return nil
	})
}

// TestServiceAnnotation_DeletesSPROnAnnotationRemoval verifies that removing
// the annotation causes the owned SynologyProxyRule to be deleted.
func TestServiceAnnotation_DeletesSPROnAnnotationRemoval(t *testing.T) {
	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupServiceIngressController(mgr, "", "")
	})

	ctx := context.Background()
	ns := "test-svc-delete"
	createNamespace(t, k8s, ns)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp",
			Namespace: ns,
			Annotations: map[string]string{
				"synology.proxy/enabled": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "myapp"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
	if err := k8s.Create(ctx, svc); err != nil {
		t.Fatalf("creating service: %v", err)
	}

	sprKey := types.NamespacedName{
		Name:      fmt.Sprintf("%s--myapp", ns),
		Namespace: ns,
	}

	// Wait for SPR to be created
	eventually(t, func() error {
		return k8s.Get(ctx, sprKey, &proxyv1alpha1.SynologyProxyRule{})
	})

	// Remove the annotation
	patch := client.MergeFrom(svc.DeepCopy())
	delete(svc.Annotations, "synology.proxy/enabled")
	if err := k8s.Patch(ctx, svc, patch); err != nil {
		t.Fatalf("patching service: %v", err)
	}

	// SPR should be gone
	eventually(t, func() error {
		err := k8s.Get(ctx, sprKey, &proxyv1alpha1.SynologyProxyRule{})
		if err == nil {
			return fmt.Errorf("SPR still exists")
		}
		if client.IgnoreNotFound(err) == nil {
			return nil // deleted
		}
		return err
	})
}

// TestWatchNamespaceGlob_AutoEnablesWithoutAnnotation verifies that services in
// namespaces matching the WatchNamespace glob are managed without an annotation.
func TestWatchNamespaceGlob_AutoEnablesWithoutAnnotation(t *testing.T) {
	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupServiceIngressController(mgr, "app-*", "")
	})

	ctx := context.Background()
	ns := "app-headlamp"
	createNamespace(t, k8s, ns)

	// No synology.proxy/enabled annotation
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "headlamp",
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "headlamp"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
	if err := k8s.Create(ctx, svc); err != nil {
		t.Fatalf("creating service: %v", err)
	}

	sprKey := types.NamespacedName{
		Name:      "app-headlamp--headlamp",
		Namespace: ns,
	}

	eventually(t, func() error {
		return k8s.Get(ctx, sprKey, &proxyv1alpha1.SynologyProxyRule{})
	})
}

// TestWatchNamespaceGlob_DoesNotAutoEnableNonMatchingNamespace verifies that
// services in namespaces that do not match the glob are not auto-managed.
func TestWatchNamespaceGlob_DoesNotAutoEnableNonMatchingNamespace(t *testing.T) {
	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupServiceIngressController(mgr, "app-*", "")
	})

	ctx := context.Background()
	ns := "monitoring"
	createNamespace(t, k8s, ns)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus",
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "prometheus"},
			Ports:    []corev1.ServicePort{{Port: 9090}},
		},
	}
	if err := k8s.Create(ctx, svc); err != nil {
		t.Fatalf("creating service: %v", err)
	}

	// Wait a moment and confirm no SPR was created
	sprKey := types.NamespacedName{
		Name:      "monitoring--prometheus",
		Namespace: ns,
	}

	// Give controller time to process (it shouldn't create anything)
	waitSeconds(t, 2)
	err := k8s.Get(ctx, sprKey, &proxyv1alpha1.SynologyProxyRule{})
	if client.IgnoreNotFound(err) != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err == nil {
		t.Error("SPR was created for non-matching namespace, should not have been")
	}
}

// TestCentralRuleNamespace_PlacesSPRInConfiguredNamespace verifies that when
// ruleNamespace is set, the SPR is created there instead of the source namespace.
func TestCentralRuleNamespace_PlacesSPRInConfiguredNamespace(t *testing.T) {
	ruleNS := "synology-proxy-operator"

	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupServiceIngressController(mgr, "", ruleNS)
	})

	ctx := context.Background()
	appNS := "test-central-rule"
	createNamespace(t, k8s, appNS)
	createNamespace(t, k8s, ruleNS)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myapp",
			Namespace: appNS,
			Annotations: map[string]string{
				"synology.proxy/enabled": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "myapp"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
	if err := k8s.Create(ctx, svc); err != nil {
		t.Fatalf("creating service: %v", err)
	}

	// SPR should be in ruleNS, not appNS
	sprKey := types.NamespacedName{
		Name:      fmt.Sprintf("%s--myapp", appNS),
		Namespace: ruleNS,
	}

	eventually(t, func() error {
		return k8s.Get(ctx, sprKey, &proxyv1alpha1.SynologyProxyRule{})
	})

	// Confirm it does NOT exist in the app namespace
	wrongKey := types.NamespacedName{
		Name:      fmt.Sprintf("%s--myapp", appNS),
		Namespace: appNS,
	}
	err := k8s.Get(ctx, wrongKey, &proxyv1alpha1.SynologyProxyRule{})
	if err == nil {
		t.Error("SPR was created in app namespace, should be in ruleNamespace")
	}
}

// waitSeconds pauses the test for N seconds.
// Used only to verify absence of a resource (controller had time to act but shouldn't).
func waitSeconds(t *testing.T, seconds int) {
	t.Helper()
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
}
