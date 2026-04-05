<img src="https://raw.githubusercontent.com/phoeluga/synology-proxy-operator/main/docs/images/synoProxyOperator_4.png" alt="" style="float: right; border-radius: 11px;"  width="86"/>

# Architecture

This document describes the internal design of the Synology Proxy Operator.

---

## Overview

The operator follows the standard Kubernetes controller pattern: it watches resources, computes desired state, and reconciles the difference. There are three controllers and one sync point.

```
Service/Ingress  ──annotation──▶  ServiceIngressReconciler  ──▶┐
ArgoCD App       ──annotation──▶  ArgoApplicationReconciler ──▶├──▶ SynologyProxyRule CRD
Manual resource  ──────────────────────────────────────────────┘
                                                                        │
                                                                        ▼
                                                         SynologyProxyRuleReconciler
                                                                        │
                                          ┌─────────────────────────────┼──────────────────────────┐
                                          ▼                             ▼                          ▼
                                  Discover backend              Upsert DSM proxy           Assign certificate
                                  (Service/Ingress IP)          record via WebAPI           (CN/SAN or default)
```

### Design principles

- **Single DSM call point.** Only `SynologyProxyRuleReconciler` talks to DSM. The other two controllers are purely Kubernetes-side.
- **Idempotency via `description`.** DSM proxy records have no stable external ID across sessions. The operator uses the `description` field as the idempotency key — records are looked up, compared, and updated by description, not UUID.
- **Finalizer-driven cleanup.** Every `SynologyProxyRule` gets a finalizer (`proxy.synology.io/finalizer`) before any DSM write. Deletion of the CR triggers DSM cleanup before the finalizer is removed.
- **Minimal dependencies.** ArgoCD support uses a local type definition (`internal/argo/types.go`) rather than importing the full ArgoCD module. The watcher is disabled gracefully at startup if ArgoCD CRDs are not present.

---

## Controllers

### ServiceIngressReconciler

**File:** `internal/controller/serviceingress_controller.go`

Watches Services and Ingresses with the annotation `synology.proxy/enabled: "true"`. Uses an adapter pattern internally — `serviceReconcileAdapter` and `ingressReconcileAdapter` share a common `reconcileObject()` function.

**Behaviour:**
- Creates a `SynologyProxyRule` in the source object's namespace (or `RULE_NAMESPACE` if set) when an annotated object appears
- Updates the rule spec only when it has changed (equality check before write)
- Deletes the rule when the annotation is removed or the object is deleted

**Rule naming:** `<namespace>--<name>` — the double-dash is an intentional separator to prevent collisions when different namespaces have services with the same name (e.g. namespace `app-headlamp` + service `headlamp` → `app-headlamp--headlamp`).

**Reads annotations:** `source-host`, `acl-profile`, `destination-protocol`, `assign-certificate`

**Enable/disable decision order** (`isResourceEnabled`):
1. `synology.proxy/enabled: "false"` on the resource → skip (explicit opt-out always wins, even against glob)
2. `synology.proxy/enabled: "true"` on the resource → manage (explicit opt-in always wins)
3. Namespace matches `WATCH_NAMESPACE` glob **and** the Namespace object does not carry `synology.proxy/auto-discovery: "false"` → manage
4. None of the above → skip

The controller fetches the Namespace object on every reconcile to read `synology.proxy/auto-discovery`. This allows a namespace to be annotated at any time without restarting the operator.

---

### ArgoApplicationReconciler

**File:** `internal/controller/argoapplication_controller.go`

Watches ArgoCD `Application` objects (GVK: `argoproj.io/v1alpha1/Application`). Disabled gracefully at startup if ArgoCD CRDs are absent — no restart needed when they appear.

**Behaviour:**
- Creates a `SynologyProxyRule` in the Application's destination namespace (or `RULE_NAMESPACE` if set) when an annotated Application appears
- Sets `spec.managedByApp` to the Application name for ownership tracking
- Reads `service-ref` and `ingress-ref` annotations to build explicit backend references
- Auto-scans the Application's destination namespace when no refs are provided

**Rule namespace resolution** (`ruleNamespaceFor`): explicit `RULE_NAMESPACE` → `app.Spec.Destination.Namespace` → `app.Namespace`. Cross-namespace owner references are forbidden in Kubernetes, so ownership is tracked via labels (`proxy.synology.io/managed-by-argo-app`) when the rule and Application are in different namespaces.

**Enable/disable decision order** mirrors `ServiceIngressReconciler`: explicit `enabled: "false"` → skip; explicit `enabled: "true"` → manage; glob match without `auto-discovery: "false"` on the Namespace → manage; otherwise skip.

**Namespace filtering:** `WATCH_NAMESPACE` restricts which namespaces are observed.

---

### SynologyProxyRuleReconciler

**File:** `internal/controller/synologyproxyrule_controller.go`

The only controller that calls DSM. Reconciles every `SynologyProxyRule` object cluster-wide.

**Reconcile loop:**

```
Reconcile(rule)
  ├── if DeletionTimestamp set → reconcileDelete()
  │     ├── for each managed record: DeleteProxyRecord()
  │     │     └── on error: keep record in status, requeue
  │     └── RemoveFinalizer() — only after all DSM deletes succeed
  └── else → reconcileUpsert()
        ├── AddFinalizer() if missing
        ├── resolveDestination() — discovery chain
        ├── resolveACLProfile() — cached, 5-min TTL
        ├── for each hostname (primary + additionalSourceHosts):
        │     ├── UpsertProxyRecord() — create or update DSM record
        │     └── if written: AssignCertificate()
        ├── reconcile stale records (deleted from spec) → DeleteProxyRecord()
        │     └── on error: keep in status, requeue
        └── update status.ManagedRecords + status.ManagedRecordCount + conditions
```

**Requeue:** every 30 seconds (`requeueAfter`) to catch external DSM drift.

---

## Backend discovery chain

When `spec.destinationHost` is not set, the operator resolves the backend in this order:

1. `spec.serviceRef` → reads the `ExternalIP` of the referenced LoadBalancer Service
2. `spec.ingressRef` → reads the status IP of the referenced Ingress
3. Auto-scan → searches the rule's namespace for the first LoadBalancer Service with an ExternalIP

Discovery result is written to `status.resolvedDestinationHost` and `status.resolvedDestinationPort`.

---

## Synology client

**Package:** `internal/synology/`

| File | Responsibility |
|---|---|
| `client.go` | HTTP client, cookie jar, SynoToken session management, login/logout |
| `proxy.go` | CRUD for DSM reverse proxy records; `proxyRecordEqual` for idempotency |
| `certificate.go` | List DSM certs, match by CN/SAN (wildcard), assign to proxy record |
| `acl.go` | List ACL profiles, resolve name to UUID |

**Session management:** The DSM WebAPI uses a two-factor token scheme. The client maintains a `sid` (session ID) and `synoToken` (CSRF token) via a cookie jar. Both are required in each API request — `sid` in the form body and `synoToken` in both the `X-SYNO-TOKEN` header and form body. The client transparently re-authenticates when the session expires (error code 119).

**Wire types:** DSM JSON shapes are defined inline in each file (`ProxyEntry`, `ProxyFrontend`, `ProxyBackend`, `Certificate`, `ACLProfile`). Enums use DSM's integer protocol codes (frontend protocol 1 = HTTPS, backend protocol 0 = HTTP, 1 = HTTPS).

**Known DSM API quirks** (see also `docs/local-testing.md`):
- Create/update operations can take up to 2 minutes
- The update method name is `update`, not `set`
- Certificate assignment always requires `old_id: ""`

---

## CRD

**Package:** `api/v1alpha1/`

| File | Contents |
|---|---|
| `synologyproxyrule_types.go` | Spec, status, conditions, print columns, kubebuilder markers |
| `zz_generated.deepcopy.go` | Auto-generated — do not edit |
| `groupversion_info.go` | Schema registration |

**Key design choices:**
- `status.managedRecords` is the source of truth for which DSM records exist. Each entry holds the DSM UUID (for reference), the description (idempotency key), and the source hostname.
- `status.managedRecordCount` mirrors `len(status.managedRecords)` as a dedicated integer field, used by the `kubectl get spr` RECORDS print column (JSONPath cannot count arrays directly).
- `spec.description` defaults to `<namespace>/<name>` when empty — this prevents cross-namespace collisions when two rules have the same name.
- `spec.additionalSourceHosts` causes one DSM record per hostname. All records are tracked in `status.managedRecords`.
- Print columns: SOURCE HOST shows `status.managedRecords[0].sourceHost` (the resolved primary hostname, not `spec.sourceHost` which is intentionally left empty when auto-derived from defaultDomain).
- `api/v1alpha1/groupversion_info.go` carries `+kubebuilder:object:generate=true` — without it `make generate` only produces deepcopy for root types (`SynologyProxyRule`, `SynologyProxyRuleList`) and omits Spec/Status/sub-types, causing a build failure.

---

## Testing architecture

The test suite is split into two layers that reflect the operator's own design.

### Unit tests (no cluster)

`internal/controller/helpers_test.go` — pure Go tests for the helper functions that implement the core business rules:

| Function | What it decides |
|---|---|
| `namespaceMatches(ns, pattern)` | Whether a namespace matches a `WATCH_NAMESPACE` glob |
| `ruleNameForObject(name, ns)` | The `<namespace>--<name>` double-dash format for auto-created SPR names |
| `isEnabled(annotations)` | Whether the `synology.proxy/enabled` annotation is set to `true` |
| `isResourceEnabled(ns, annotations)` | Combined check: annotation OR namespace glob match |

These run instantly with no external dependencies and are the first line of defence for logic regressions.

### Controller integration tests (envtest)

`internal/controller/*_test.go` — each test starts a real Kubernetes API server and etcd in-process via [envtest](https://book.kubebuilder.io/reference/envtest), registers the controller under test with a `ctrl.Manager`, and drives the full reconcile loop against real Kubernetes objects.

```
TestServiceAnnotation_CreatesSPR
  │
  ├─ envtest.Environment.Start()        → real kube-apiserver + etcd
  ├─ ctrl.NewManager(cfg, ...)          → informer caches, client
  ├─ ServiceIngressReconciler.Setup()   → watches Services + Ingresses
  │
  ├─ k8s.Create(Service{annotation=true})
  └─ eventually(k8s.Get(SynologyProxyRule))  → controller ran and created the CR
```

Each test function gets its own isolated environment (separate API server process). The `startManager` / `startManagerWithArgo` helpers in `suite_test.go` handle setup and register a `t.Cleanup` that stops the environment after the test.

**Controller name uniqueness** — controller-runtime rejects duplicate controller names within one process. Tests use `ctrlconfig.Controller{SkipNameValidation: ptr.To(true)}` in the manager options so multiple test managers can coexist in the same `go test` binary.

### DSM mock (SynologyProxyRuleReconciler tests)

`SynologyProxyRuleReconciler` is the only controller that calls DSM. Its tests use an `httptest.Server` (`fakeDSM`) that speaks the Synology DSM wire protocol and stores proxy records in memory:

```
TestSPR_CreatesPushesToDSM
  │
  ├─ fakeDSM.Start()                  → httptest.Server on random port
  ├─ synology.New(Config{URL: srv.URL})  → real client pointing at mock
  ├─ envtest + SynologyProxyRuleReconciler wired to the real client
  │
  ├─ k8s.Create(SynologyProxyRule{...})
  ├─ eventually(spr.Status.Synced == true)
  └─ assert(fakeDSM.Creates == 1)     → controller called DSM create exactly once
```

The `fakeDSM` handles: login (`/webapi/auth.cgi`), proxy CRUD (`list` / `create` / `update` / `delete`), certificate list, ACL profile list. A real `synology.Client` is pointed at its URL — no interface abstraction or production code changes are required.

### CI integration

The CI workflow (`.github/workflows/ci-build-and-test.yaml`) installs `setup-envtest` and sets `KUBEBUILDER_ASSETS` before running `go test ./...`:

```yaml
- name: Install setup-envtest
  run: go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

- name: Run tests
  run: |
    KUBEBUILDER_ASSETS="$(setup-envtest use 1.32.x --bin-dir /tmp/envtest -p path)"
    export KUBEBUILDER_ASSETS
    go test ./... -v -coverprofile=coverage.out
```

This means every pull request and every merge to `main` runs the full envtest suite — unit tests, all three controller integration test suites, and the fake-DSM tests — without needing a real cluster or Synology device.

---

## Project structure

```
synology-proxy-operator/
├── api/v1alpha1/                         # CRD type definitions
│   ├── synologyproxyrule_types.go
│   ├── zz_generated.deepcopy.go         # generated — do not edit
│   └── groupversion_info.go
├── cmd/
│   └── main.go                          # entry point — flag/env wiring, manager setup
├── internal/
│   ├── argo/
│   │   └── types.go                     # minimal ArgoCD types (no full dependency)
│   ├── controller/
│   │   ├── annotations.go               # shared annotation key constants
│   │   ├── argoapplication_controller.go
│   │   ├── serviceingress_controller.go
│   │   ├── synologyproxyrule_controller.go
│   │   ├── helpers_test.go              # unit tests (no cluster)
│   │   ├── suite_test.go                # envtest helpers (startManager, eventually)
│   │   ├── serviceingress_controller_test.go
│   │   ├── argoapplication_controller_test.go
│   │   ├── synologyproxyrule_controller_test.go  # includes fakeDSM
│   │   └── testdata/
│   │       └── argo-application-crd.yaml  # minimal CRD for envtest ArgoCD tests
│   └── synology/
│       ├── client.go
│       ├── proxy.go
│       ├── certificate.go
│       └── acl.go
├── config/
│   ├── default/
│   │   └── kustomization.yaml           # root overlay — kubectl apply -k config/default/
│   ├── crd/bases/                       # generated CRD manifests
│   ├── rbac/                            # ClusterRole, ClusterRoleBinding, ServiceAccount
│   └── manager/                         # Deployment + ConfigMap
├── helm/
│   └── synology-proxy-operator/
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── crds/                        # CRD copy for Helm packaging
│       └── templates/
├── hack/
│   └── dev/                             # local dev fixtures (namespace, nginx, proxy rule, ArgoCD app)
└── docs/
    └── architecture.md                  # this file

```
