package controller_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	proxyv1alpha1 "github.com/phoeluga/synology-proxy-operator/api/v1alpha1"
	"github.com/phoeluga/synology-proxy-operator/internal/controller"
	"github.com/phoeluga/synology-proxy-operator/internal/synology"
)

// ── Fake DSM HTTP server ──────────────────────────────────────────────────────

// fakeDSM is an in-memory Synology DSM API stand-in for testing.
// It handles login, proxy record CRUD, certificate list, and ACL profile list.
type fakeDSM struct {
	mu       sync.Mutex
	records  map[string]*synology.ProxyEntry // keyed by description
	nextUUID int
	// Counters for assertions
	Creates int
	Updates int
	Deletes int
	// Optional hook: if non-nil, ListProxyRecords returns this error.
	ListErr bool
}

func newFakeDSM() *fakeDSM {
	return &fakeDSM{records: make(map[string]*synology.ProxyEntry)}
}

func (f *fakeDSM) successJSON(data any) []byte {
	raw, _ := json.Marshal(data)
	out, _ := json.Marshal(map[string]any{"success": true, "data": json.RawMessage(raw)})
	return out
}

func (f *fakeDSM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Auth endpoint (GET)
	if r.URL.Path == "/webapi/auth.cgi" {
		w.Write(f.successJSON(map[string]any{ //nolint:errcheck
			"sid":       "test-sid",
			"synotoken": "test-token",
		}))
		return
	}

	_ = r.ParseForm()
	api := r.FormValue("api")
	method := r.FormValue("method")

	switch {
	case api == "SYNO.Core.AppPortal.ReverseProxy":
		f.handleProxy(w, method, r.Form)
	case api == "SYNO.Core.Certificate.CRT" && method == "list":
		f.handleCertList(w)
	case api == "SYNO.Core.Certificate.Service" && method == "set":
		w.Write(f.successJSON(map[string]any{})) //nolint:errcheck
	case api == "SYNO.Core.AppPortal.AccessControl" && method == "list":
		f.handleACLList(w)
	default:
		http.Error(w, "unknown api/method", http.StatusBadRequest)
	}
}

func (f *fakeDSM) handleProxy(w http.ResponseWriter, method string, form url.Values) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch method {
	case "list":
		if f.ListErr {
			json.NewEncoder(w).Encode(map[string]any{"success": false, "error": map[string]any{"code": 100}}) //nolint:errcheck
			return
		}
		entries := make([]synology.ProxyEntry, 0, len(f.records))
		for _, e := range f.records {
			entries = append(entries, *e)
		}
		w.Write(f.successJSON(map[string]any{"entries": entries})) //nolint:errcheck

	case "create":
		var entry synology.ProxyEntry
		if err := json.Unmarshal([]byte(form.Get("entry")), &entry); err != nil {
			http.Error(w, "bad entry json", http.StatusBadRequest)
			return
		}
		f.nextUUID++
		entry.UUID = fmt.Sprintf("uuid-%d", f.nextUUID)
		entry.Key = entry.UUID
		f.records[entry.Description] = &entry
		f.Creates++
		w.Write(f.successJSON(map[string]any{"UUID": entry.UUID})) //nolint:errcheck

	case "update":
		var entry synology.ProxyEntry
		if err := json.Unmarshal([]byte(form.Get("entry")), &entry); err != nil {
			http.Error(w, "bad entry json", http.StatusBadRequest)
			return
		}
		f.records[entry.Description] = &entry
		f.Updates++
		w.Write(f.successJSON(map[string]any{})) //nolint:errcheck

	case "delete":
		var uuids []string
		if err := json.Unmarshal([]byte(form.Get("uuids")), &uuids); err != nil {
			http.Error(w, "bad uuids json", http.StatusBadRequest)
			return
		}
		for desc, e := range f.records {
			for _, u := range uuids {
				if e.UUID == u {
					delete(f.records, desc)
					f.Deletes++
				}
			}
		}
		w.Write(f.successJSON(map[string]any{})) //nolint:errcheck
	}
}

func (f *fakeDSM) handleCertList(w http.ResponseWriter) {
	// Return a single wildcard certificate as the DSM default.
	certs := []synology.Certificate{
		{
			ID:        "cert-1",
			Desc:      "wildcard",
			IsDefault: true,
			Subject: synology.CertSubject{
				CommonName: "*.example.com",
				SubAltName: []string{"*.example.com"},
			},
		},
	}
	w.Write(f.successJSON(map[string]any{"certificates": certs})) //nolint:errcheck
}

func (f *fakeDSM) handleACLList(w http.ResponseWriter) {
	profiles := []synology.ACLProfile{
		{UUID: "acl-uuid-1", Name: "internal"},
	}
	w.Write(f.successJSON(map[string]any{"entries": profiles})) //nolint:errcheck
}

// recordCount returns the number of proxy records stored in the fake DSM.
func (f *fakeDSM) recordCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records)
}

// hasRecord returns true if the fake DSM has a record with the given description.
func (f *fakeDSM) hasRecord(desc string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.records[desc]
	return ok
}

// ── Test helpers ─────────────────────────────────────────────────────────────

func newSynologyClient(t *testing.T, serverURL string) *synology.Client {
	t.Helper()
	c, err := synology.New(synology.Config{
		URL:           serverURL,
		Username:      "admin",
		Password:      "test",
		SkipTLSVerify: true,
	}, logr.Discard())
	if err != nil {
		t.Fatalf("creating synology client: %v", err)
	}
	return c
}

func setupSPRController(mgr ctrl.Manager, sc *synology.Client, defaultDomain string) error {
	r := &controller.SynologyProxyRuleReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Log:            logr.Discard(),
		SynologyClient: sc,
		DefaultDomain:  defaultDomain,
	}
	return r.SetupWithManager(mgr)
}

func makeSPR(name, ns, sourceHost, destHost string, destPort int) *proxyv1alpha1.SynologyProxyRule {
	return &proxyv1alpha1.SynologyProxyRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: proxyv1alpha1.SynologyProxyRuleSpec{
			SourceHost:          sourceHost,
			SourcePort:          443,
			DestinationHost:     destHost,
			DestinationPort:     destPort,
			DestinationProtocol: "http",
		},
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestSPR_CreatesPushesToDSM verifies that creating a SynologyProxyRule causes
// the controller to create a record in DSM and update status.ManagedRecords.
func TestSPR_CreatesPushesToDSM(t *testing.T) {
	dsm := newFakeDSM()
	srv := httptest.NewServer(dsm)
	t.Cleanup(srv.Close)

	sc := newSynologyClient(t, srv.URL)

	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupSPRController(mgr, sc, "")
	})

	ctx := context.Background()
	ns := "spr-create"
	createNamespace(t, k8s, ns)

	spr := makeSPR("myapp", ns, "myapp.example.com", "10.0.0.1", 8080)
	if err := k8s.Create(ctx, spr); err != nil {
		t.Fatalf("creating SPR: %v", err)
	}

	key := types.NamespacedName{Name: "myapp", Namespace: ns}

	// Status should reflect the synced record.
	eventually(t, func() error {
		if err := k8s.Get(ctx, key, spr); err != nil {
			return err
		}
		if !spr.Status.Synced {
			return fmt.Errorf("not yet synced; conditions=%v", spr.Status.Conditions)
		}
		if len(spr.Status.ManagedRecords) == 0 {
			return fmt.Errorf("managed records empty")
		}
		return nil
	})

	// DSM should have exactly one record.
	if dsm.recordCount() != 1 {
		t.Errorf("expected 1 DSM record, got %d", dsm.recordCount())
	}
	if dsm.Creates != 1 {
		t.Errorf("expected 1 DSM create call, got %d", dsm.Creates)
	}
	// Description defaults to namespace/name.
	if !dsm.hasRecord(ns + "/myapp") {
		t.Errorf("expected DSM record with description %q", ns+"/myapp")
	}
}

// TestSPR_StatusTracksDestination verifies that after reconciliation the status
// fields ResolvedDestinationHost and ResolvedDestinationPort are populated.
func TestSPR_StatusTracksDestination(t *testing.T) {
	dsm := newFakeDSM()
	srv := httptest.NewServer(dsm)
	t.Cleanup(srv.Close)

	sc := newSynologyClient(t, srv.URL)

	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupSPRController(mgr, sc, "")
	})

	ctx := context.Background()
	ns := "spr-status"
	createNamespace(t, k8s, ns)

	spr := makeSPR("backend", ns, "backend.example.com", "192.168.1.10", 3000)
	if err := k8s.Create(ctx, spr); err != nil {
		t.Fatalf("creating SPR: %v", err)
	}

	key := types.NamespacedName{Name: "backend", Namespace: ns}

	eventually(t, func() error {
		if err := k8s.Get(ctx, key, spr); err != nil {
			return err
		}
		if spr.Status.ResolvedDestinationHost != "192.168.1.10" {
			return fmt.Errorf("ResolvedDestinationHost=%q", spr.Status.ResolvedDestinationHost)
		}
		if spr.Status.ResolvedDestinationPort != 3000 {
			return fmt.Errorf("ResolvedDestinationPort=%d", spr.Status.ResolvedDestinationPort)
		}
		if spr.Status.ManagedRecordCount != 1 {
			return fmt.Errorf("ManagedRecordCount=%d", spr.Status.ManagedRecordCount)
		}
		return nil
	})
}

// TestSPR_DeleteRemovesDSMRecord verifies that deleting an SPR causes its DSM
// record to be removed (via the finalizer cleanup path).
func TestSPR_DeleteRemovesDSMRecord(t *testing.T) {
	dsm := newFakeDSM()
	srv := httptest.NewServer(dsm)
	t.Cleanup(srv.Close)

	sc := newSynologyClient(t, srv.URL)

	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupSPRController(mgr, sc, "")
	})

	ctx := context.Background()
	ns := "spr-delete"
	createNamespace(t, k8s, ns)

	spr := makeSPR("todelete", ns, "todelete.example.com", "10.0.0.2", 9090)
	if err := k8s.Create(ctx, spr); err != nil {
		t.Fatalf("creating SPR: %v", err)
	}

	key := types.NamespacedName{Name: "todelete", Namespace: ns}

	// Wait until DSM record exists and SPR is synced.
	eventually(t, func() error {
		if err := k8s.Get(ctx, key, spr); err != nil {
			return err
		}
		if !spr.Status.Synced {
			return fmt.Errorf("not synced yet")
		}
		return nil
	})

	if dsm.recordCount() != 1 {
		t.Fatalf("expected 1 DSM record before delete, got %d", dsm.recordCount())
	}

	// Delete the SPR — the finalizer will call DSM delete.
	if err := k8s.Delete(ctx, spr); err != nil {
		t.Fatalf("deleting SPR: %v", err)
	}

	// SPR should disappear from Kubernetes (finalizer removed after DSM delete).
	eventually(t, func() error {
		err := k8s.Get(ctx, key, &proxyv1alpha1.SynologyProxyRule{})
		if client.IgnoreNotFound(err) == nil && err != nil {
			return nil // deleted
		}
		if err == nil {
			return fmt.Errorf("SPR still exists")
		}
		return err
	})

	// DSM record should be gone.
	if dsm.recordCount() != 0 {
		t.Errorf("expected 0 DSM records after SPR deletion, got %d", dsm.recordCount())
	}
	if dsm.Deletes != 1 {
		t.Errorf("expected 1 DSM delete call, got %d", dsm.Deletes)
	}
}

// TestSPR_BackendDiscoveryFromService verifies that when spec.serviceRef is set,
// the controller discovers the backend from the referenced LoadBalancer service.
func TestSPR_BackendDiscoveryFromService(t *testing.T) {
	dsm := newFakeDSM()
	srv := httptest.NewServer(dsm)
	t.Cleanup(srv.Close)

	sc := newSynologyClient(t, srv.URL)

	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupSPRController(mgr, sc, "example.com")
	})

	ctx := context.Background()
	ns := "spr-discovery"
	createNamespace(t, k8s, ns)

	// Create the service first (spec only — Create strips status).
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: ns},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 80}},
		},
	}
	if err := k8s.Create(ctx, svc); err != nil {
		t.Fatalf("creating service: %v", err)
	}
	// Patch the LoadBalancer status via the status subresource.
	svc.Status = corev1.ServiceStatus{
		LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{{IP: "10.10.10.10"}},
		},
	}
	if err := k8s.Status().Update(ctx, svc); err != nil {
		t.Fatalf("updating service status: %v", err)
	}

	// Wait for the status to be visible in the manager's cache before creating
	// the SPR. This prevents a race where the first reconcile sees no IP and
	// requeues after 60 s — longer than the 10 s test timeout.
	svcKey := types.NamespacedName{Name: "myapp", Namespace: ns}
	eventually(t, func() error {
		cached := &corev1.Service{}
		if err := k8s.Get(ctx, svcKey, cached); err != nil {
			return err
		}
		if len(cached.Status.LoadBalancer.Ingress) == 0 ||
			cached.Status.LoadBalancer.Ingress[0].IP == "" {
			return fmt.Errorf("service status not yet in cache")
		}
		return nil
	})

	// SPR with serviceRef and no explicit destination.
	spr := &proxyv1alpha1.SynologyProxyRule{
		ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: ns},
		Spec: proxyv1alpha1.SynologyProxyRuleSpec{
			SourceHost: "myapp.example.com",
			SourcePort: 443,
			ServiceRef: &proxyv1alpha1.ObjectRef{Name: "myapp", Namespace: ns},
		},
	}
	if err := k8s.Create(ctx, spr); err != nil {
		t.Fatalf("creating SPR: %v", err)
	}

	key := types.NamespacedName{Name: "myapp", Namespace: ns}

	eventually(t, func() error {
		if err := k8s.Get(ctx, key, spr); err != nil {
			return err
		}
		if !spr.Status.Synced {
			return fmt.Errorf("not synced; conditions=%v", spr.Status.Conditions)
		}
		if spr.Status.ResolvedDestinationHost != "10.10.10.10" {
			return fmt.Errorf("unexpected destination host: %q", spr.Status.ResolvedDestinationHost)
		}
		return nil
	})
}

// TestSPR_CustomDescriptionUsedAsKey verifies that spec.description is used as
// the DSM idempotency key when set, overriding the default namespace/name.
func TestSPR_CustomDescriptionUsedAsKey(t *testing.T) {
	dsm := newFakeDSM()
	srv := httptest.NewServer(dsm)
	t.Cleanup(srv.Close)

	sc := newSynologyClient(t, srv.URL)

	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupSPRController(mgr, sc, "")
	})

	ctx := context.Background()
	ns := "spr-desc"
	createNamespace(t, k8s, ns)

	spr := &proxyv1alpha1.SynologyProxyRule{
		ObjectMeta: metav1.ObjectMeta{Name: "myapp", Namespace: ns},
		Spec: proxyv1alpha1.SynologyProxyRuleSpec{
			SourceHost:          "myapp.example.com",
			SourcePort:          443,
			DestinationHost:     "10.0.0.3",
			DestinationPort:     8080,
			DestinationProtocol: "http",
			Description:         "custom/description",
		},
	}
	if err := k8s.Create(ctx, spr); err != nil {
		t.Fatalf("creating SPR: %v", err)
	}

	key := types.NamespacedName{Name: "myapp", Namespace: ns}

	eventually(t, func() error {
		if err := k8s.Get(ctx, key, spr); err != nil {
			return err
		}
		if !spr.Status.Synced {
			return fmt.Errorf("not synced")
		}
		return nil
	})

	if !dsm.hasRecord("custom/description") {
		t.Errorf("expected DSM record with description %q", "custom/description")
	}
	if dsm.hasRecord(ns + "/myapp") {
		t.Errorf("unexpected DSM record with default description %q", ns+"/myapp")
	}
}

// TestSPR_DSMFailureSetsCondition verifies that when the DSM API returns an error,
// the controller sets a SyncFailed condition rather than crashing.
func TestSPR_DSMFailureSetsCondition(t *testing.T) {
	dsm := newFakeDSM()
	dsm.ListErr = true // make every list call fail
	srv := httptest.NewServer(dsm)
	t.Cleanup(srv.Close)

	sc := newSynologyClient(t, srv.URL)

	k8s, _ := startManager(t, func(mgr ctrl.Manager) error {
		return setupSPRController(mgr, sc, "")
	})

	ctx := context.Background()
	ns := "spr-failure"
	createNamespace(t, k8s, ns)

	spr := makeSPR("failing", ns, "failing.example.com", "10.0.0.9", 8080)
	if err := k8s.Create(ctx, spr); err != nil {
		t.Fatalf("creating SPR: %v", err)
	}

	key := types.NamespacedName{Name: "failing", Namespace: ns}

	eventually(t, func() error {
		if err := k8s.Get(ctx, key, spr); err != nil {
			return err
		}
		// Look for a SyncFailed or Synced=False condition.
		for _, c := range spr.Status.Conditions {
			if c.Status == metav1.ConditionFalse {
				return nil // condition present
			}
		}
		if spr.Status.Synced {
			return fmt.Errorf("SPR unexpectedly reports Synced=true despite DSM failure")
		}
		return fmt.Errorf("no failure condition set yet; conditions=%v", spr.Status.Conditions)
	})
}
