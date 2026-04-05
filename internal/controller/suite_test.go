package controller_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	proxyv1alpha1 "github.com/phoeluga/synology-proxy-operator/api/v1alpha1"
	"github.com/phoeluga/synology-proxy-operator/internal/argo"
)

const (
	timeout  = 10 * time.Second
	interval = 100 * time.Millisecond
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)
	_ = proxyv1alpha1.AddToScheme(scheme)
	_ = argo.AddToScheme(scheme)
}

// startEnvtest boots a real API server + etcd for integration tests.
// Returns a configured client and a cancel func to shut down the environment.
func startEnvtest(t *testing.T) (client.Client, context.CancelFunc) {
	t.Helper()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		Scheme:                scheme,
	}

	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("starting envtest: %v", err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	t.Cleanup(func() {
		cancel()
		if err := env.Stop(); err != nil {
			t.Logf("stopping envtest: %v", err)
		}
	})

	_ = ctx
	return k8sClient, cancel
}

// startManager boots a controller-runtime manager with all controllers registered.
// Returns the manager's client and a cancel func.
func startManager(t *testing.T, setupFn func(ctrl.Manager) error) (client.Client, context.CancelFunc) {
	return startManagerWithCRDs(t, setupFn, []string{
		filepath.Join("..", "..", "config", "crd", "bases"),
	})
}

// startManagerWithArgo is like startManager but also installs the ArgoCD Application CRD.
func startManagerWithArgo(t *testing.T, setupFn func(ctrl.Manager) error) (client.Client, context.CancelFunc) {
	return startManagerWithCRDs(t, setupFn, []string{
		filepath.Join("..", "..", "config", "crd", "bases"),
		filepath.Join("testdata"),
	})
}

func startManagerWithCRDs(t *testing.T, setupFn func(ctrl.Manager) error, crdPaths []string) (client.Client, context.CancelFunc) {
	t.Helper()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	env := &envtest.Environment{
		CRDDirectoryPaths:     crdPaths,
		ErrorIfCRDPathMissing: true,
		Scheme:                scheme,
	}

	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("starting envtest: %v", err)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		// Tests register multiple managers in the same process; skip unique-name validation.
		Controller: ctrlconfig.Controller{SkipNameValidation: ptr.To(true)},
	})
	if err != nil {
		t.Fatalf("creating manager: %v", err)
	}

	if err := setupFn(mgr); err != nil {
		t.Fatalf("setting up controllers: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := mgr.Start(ctx); err != nil && ctx.Err() == nil {
			t.Logf("manager exited: %v", err)
		}
	}()

	t.Cleanup(func() {
		cancel()
		if err := env.Stop(); err != nil {
			t.Logf("stopping envtest: %v", err)
		}
	})

	return mgr.GetClient(), cancel
}

// eventually retries f until it returns nil or timeout is exceeded.
func eventually(t *testing.T, f func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if lastErr = f(); lastErr == nil {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("condition not met within %s: %v", timeout, lastErr)
}
