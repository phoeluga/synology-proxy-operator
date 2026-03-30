# Synology Proxy Operator

A Kubernetes operator that automatically manages **Synology DSM reverse proxy records**.
It watches ArgoCD Applications, annotated Services/Ingresses, and manual `SynologyProxyRule`
custom resources — and keeps the corresponding DSM configuration in sync.

---

## Table of contents

- [How it works](#how-it-works)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage modes](#usage-modes)
  - [Mode 1 — Annotate a Service or Ingress](#mode-1--annotate-a-service-or-ingress)
  - [Mode 2 — Annotate an ArgoCD Application](#mode-2--annotate-an-argocd-application)
  - [Mode 3 — Manual SynologyProxyRule](#mode-3--manual-synologyproxyrule)
- [Hostname derivation](#hostname-derivation)
- [Certificate assignment](#certificate-assignment)
- [Annotation reference](#annotation-reference)
- [SynologyProxyRule CRD reference](#synologyproxyrule-crd-reference)
- [Status and observability](#status-and-observability)
- [Helm values reference](#helm-values-reference)
- [Development](#development)
- [Release](#release)
- [Project structure](#project-structure)

---

## How it works

```
Service/Ingress  ──annotation──▶  ServiceIngressReconciler ──▶┐
ArgoCD App       ──annotation──▶  ArgoApplicationReconciler ──▶┤
Manual resource                                                 │
                                                                ▼
                                                   SynologyProxyRule CRD
                                                                │
                                                                ▼
                                              SynologyProxyRuleReconciler
                                                                │
                                    ┌───────────────────────────┼───────────────────────────┐
                                    ▼                           ▼                           ▼
                            Discover backend           Upsert DSM proxy            Assign certificate
                            (Service/Ingress IP)       record via WebAPI           (wildcard/SAN match
                                                                                   or DSM default)
```

The operator runs three controllers:

| Controller | Watches | Creates |
|---|---|---|
| `ServiceIngressReconciler` | Services + Ingresses with `synology.proxy/enabled: "true"` | `SynologyProxyRule` objects |
| `ArgoApplicationReconciler` | ArgoCD `Application` objects with `synology.proxy/enabled: "true"` | `SynologyProxyRule` objects |
| `SynologyProxyRuleReconciler` | All `SynologyProxyRule` objects | DSM reverse proxy records |

The `SynologyProxyRuleReconciler` is the single point that talks to DSM — the other two
controllers just produce and maintain `SynologyProxyRule` resources.

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Synology DSM ≥ 7.0 | WebAPI access required |
| Kubernetes ≥ 1.28 | |
| ArgoCD ≥ 2.8 | Optional — only needed for the ArgoCD watcher |

---

## Installation

### Helm (recommended)

```bash
helm upgrade --install synology-proxy-operator \
  oci://ghcr.io/phoeluga/charts/synology-proxy-operator \
  --namespace synology-proxy-operator \
  --create-namespace \
  --set synology.url="https://192.168.1.x:5001" \
  --set synology.username="admin" \
  --set synology.password="secret" \
  --set synology.skipTLSVerify=true \
  --set operator.defaultDomain="home.example.com"
```

### GitOps (ArgoCD multi-source)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: synology-proxy-operator
  namespace: argocd
spec:
  sources:
    - repoURL: https://github.com/phoeluga/synology-proxy-operator
      path: config
      targetRevision: main
    - repoURL: https://github.com/your-org/your-cluster-repo
      path: clusters/prod/infrastructure/synology-proxy-operator/credentials
      targetRevision: HEAD
  destination:
    server: https://kubernetes.default.svc
    namespace: synology-proxy-operator
```

The `credentials/` directory should contain a Secret (or SealedSecret) with keys:
`url`, `username`, `password`, `skipTLSVerify`.

---

## Configuration

All configuration is read from environment variables. When deployed via Helm these are
set automatically from `values.yaml`. When running locally, put them in `.env.local`.

| Variable | Description | Default |
|---|---|---|
| `SYNOLOGY_URL` | DSM base URL, e.g. `https://192.168.1.x:5001` | required |
| `SYNOLOGY_USER` | DSM username | required |
| `SYNOLOGY_PASSWORD` | DSM password | required |
| `SYNOLOGY_SKIP_TLS_VERIFY` | Skip TLS verification (self-signed certs) | `false` |
| `DEFAULT_DOMAIN` | Domain appended to auto-derived hostnames, e.g. `home.example.com` | `""` |
| `DEFAULT_ACL_PROFILE` | ACL profile name applied when none is specified on a rule | `""` |
| `RULE_NAMESPACE` | Namespace where auto-created `SynologyProxyRule` objects are placed | `synology-proxy-operator` |
| `ENABLE_ARGO_WATCHER` | Enable the ArgoCD Application watcher | `true` |
| `WATCH_NAMESPACE` | Restrict ArgoCD watcher to a single namespace (empty = all) | `""` |

---

## Usage modes

### Mode 1 — Annotate a Service or Ingress

The simplest approach: add `synology.proxy/enabled: "true"` to any existing Service
or Ingress. The operator automatically creates a `SynologyProxyRule` and a DSM record.

**Example — LoadBalancer Service:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: myapp
  annotations:
    synology.proxy/enabled: "true"
    # Optional — if omitted, hostname is derived as "<service-name>.<DEFAULT_DOMAIN>"
    synology.proxy/source-host: "nginx.home.example.com"
spec:
  type: LoadBalancer
  ports:
    - port: 80
```

With `DEFAULT_DOMAIN=home.example.com` configured, the annotation `synology.proxy/source-host`
is not needed — the operator derives `nginx.home.example.com` automatically.

**Example — Ingress:**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myapp
  namespace: myapp
  annotations:
    synology.proxy/enabled: "true"
    synology.proxy/destination-protocol: "https"
    synology.proxy/acl-profile: "LAN Only"
```

Removing the `synology.proxy/enabled` annotation (or deleting the object) removes
the DSM record automatically.

---

### Mode 2 — Annotate an ArgoCD Application

When using ArgoCD, annotate the `Application` object instead of individual Services.
The operator creates a `SynologyProxyRule` that auto-discovers the backend from the
application's destination namespace.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: myapp
  namespace: argocd
  annotations:
    synology.proxy/enabled: "true"
    # Optional — defaults to "<app-name>.<DEFAULT_DOMAIN>"
    synology.proxy/source-host: "myapp.home.example.com"
    synology.proxy/service-ref: "myapp/myapp-svc"
    synology.proxy/acl-profile: "LAN Only"
spec:
  destination:
    namespace: myapp
    server: https://kubernetes.default.svc
```

---

### Mode 3 — Manual SynologyProxyRule

Create a `SynologyProxyRule` directly for full control. Useful for services not managed
by ArgoCD and not running in Kubernetes (e.g. a VM or NAS service).

**Minimum (with `DEFAULT_DOMAIN` configured):**

```yaml
apiVersion: proxy.synology.io/v1alpha1
kind: SynologyProxyRule
metadata:
  name: myapp
  namespace: synology-proxy-operator
spec:
  serviceRef:
    name: myapp
    namespace: myapp
```

This creates a DSM record for `myapp.home.example.com` pointing at the LoadBalancer IP
of the `myapp` service.

**Explicit (no auto-discovery):**

```yaml
apiVersion: proxy.synology.io/v1alpha1
kind: SynologyProxyRule
metadata:
  name: nas-photos
  namespace: synology-proxy-operator
spec:
  sourceHost: photos.home.example.com
  destinationHost: 192.168.1.100
  destinationPort: 8080
  destinationProtocol: http
  assignCertificate: true
```

**Multiple frontend hostnames (one backend, multiple DSM records):**

```yaml
spec:
  sourceHost: myapp.home.example.com
  additionalSourceHosts:
    - myapp.example.org
  serviceRef:
    name: myapp
    namespace: myapp
```

---

## Hostname derivation

When `spec.sourceHost` is empty (or not set), the operator derives the hostname:

1. **`synology.proxy/source-host` annotation** on the referenced Service or Ingress — used as-is
2. **`<objectName>.<DEFAULT_DOMAIN>`** — e.g. `nginx.home.example.com`
3. **`<ruleName>.<DEFAULT_DOMAIN>`** — fallback when no ServiceRef/IngressRef is set
4. **Error** if `DEFAULT_DOMAIN` is also not configured

This applies to all three usage modes:

| Mode | Name used |
|---|---|
| Service/Ingress annotation | service or ingress name |
| ArgoCD Application | application name |
| Manual SynologyProxyRule | rule name or serviceRef/ingressRef name |

---

## Certificate assignment

When `spec.assignCertificate: true` (the default), the operator assigns a certificate
to each DSM proxy record after creation or update.

**Selection logic:**

1. Find a certificate in DSM whose CN or SAN matches the source hostname (wildcard patterns like `*.example.com` are supported)
2. If no match is found, assign the DSM **default certificate** (`is_default: true`)

The assignment only calls the DSM API when the proxy record was just created or updated —
not on every reconcile.

---

## Annotation reference

These annotations are supported on **Services**, **Ingresses**, and **ArgoCD Applications**:

| Annotation | Description | Default |
|---|---|---|
| `synology.proxy/enabled` | `"true"` to enable proxy management | — (required) |
| `synology.proxy/source-host` | Public FQDN override | derived from name + domain |
| `synology.proxy/acl-profile` | Synology ACL profile name | `DEFAULT_ACL_PROFILE` |
| `synology.proxy/destination-protocol` | Backend protocol: `http` or `https` | `http` |
| `synology.proxy/destination-host` | Backend IP/hostname override (ArgoCD only) | auto-discovered |
| `synology.proxy/destination-port` | Backend port override (ArgoCD only) | auto-discovered |
| `synology.proxy/assign-certificate` | `"false"` to skip certificate assignment | `"true"` |
| `synology.proxy/service-ref` | `<namespace>/<name>` — Service to use for discovery (ArgoCD only) | auto-scan |
| `synology.proxy/ingress-ref` | `<namespace>/<name>` — Ingress to use for discovery (ArgoCD only) | auto-scan |

---

## SynologyProxyRule CRD reference

```yaml
apiVersion: proxy.synology.io/v1alpha1
kind: SynologyProxyRule
metadata:
  name: myapp
  namespace: synology-proxy-operator
spec:
  # ── Frontend hostnames ────────────────────────────────────────────────────
  # sourceHost is optional when DEFAULT_DOMAIN is configured.
  sourceHost: myapp.home.example.com

  # Additional hostnames — each gets its own DSM record and certificate.
  additionalSourceHosts:
    - myapp.example.org

  sourcePort: 443               # Default: 443

  # ── Backend ───────────────────────────────────────────────────────────────
  # Set explicitly or let the operator discover from serviceRef / ingressRef.
  destinationHost: ""           # Auto-discovered when empty
  destinationPort: 0            # Auto-discovered when 0
  destinationProtocol: http     # http (default) | https

  # ── Backend auto-discovery ────────────────────────────────────────────────
  # The operator reads the LoadBalancer ExternalIP from the referenced Service,
  # or the status IP from the referenced Ingress.
  serviceRef:
    name: myapp
    namespace: myapp            # Defaults to rule namespace when omitted

  ingressRef:
    name: myapp-ingress
    namespace: myapp

  # ── Synology DSM settings ─────────────────────────────────────────────────
  aclProfile: "LAN Only"        # DSM Access Control profile name
  assignCertificate: true       # Auto-assign matching certificate

  # Custom HTTP headers forwarded to the backend.
  # Defaults to WebSocket upgrade headers when omitted.
  customHeaders:
    - name: Upgrade
      value: $http_upgrade
    - name: Connection
      value: $connection_upgrade

  timeouts:
    connect: 60                 # seconds
    read:    60
    send:    60

  # ── Internal fields (set automatically) ───────────────────────────────────
  description: myapp            # DSM record label. Defaults to .metadata.name
  managedByApp: myapp           # Set by ArgoCD watcher — do not set manually
```

### Backend discovery priority

When `destinationHost`/`destinationPort` are not set:

1. `serviceRef` — uses the `ExternalIP` of the referenced LoadBalancer Service
2. `ingressRef` — uses the status IP of the referenced Ingress
3. Auto-scan — finds any LoadBalancer Service in the rule's namespace with an ExternalIP

---

## Status and observability

```bash
kubectl get spr -n synology-proxy-operator
```

```
NAME     SOURCE HOST              DESTINATION     SYNCED   RECORDS   AGE
myapp    myapp.home.example.com   192.168.1.100   true     1         5m
```

```bash
kubectl describe spr myapp -n synology-proxy-operator
```

Key status fields:

| Field | Description |
|---|---|
| `status.synced` | `true` when last DSM sync succeeded |
| `status.managedRecords` | List of all DSM records managed by this rule (one per source host) |
| `status.managedRecords[].uuid` | DSM UUID of the record |
| `status.managedRecords[].sourceHost` | Frontend hostname of this record |
| `status.resolvedDestinationHost` | Backend IP/hostname discovered |
| `status.resolvedDestinationPort` | Backend port discovered |
| `status.lastSyncTime` | Timestamp of last successful sync |
| `status.conditions[Synced]` | Standard Kubernetes condition |
| `status.conditions[Ready]` | True when backend is discovered and rule is active |

**Force re-reconcile:**

```bash
kubectl annotate spr myapp -n synology-proxy-operator \
  force-sync="$(date +%s)" --overwrite
```

---

## Helm values reference

| Value | Description | Default |
|---|---|---|
| `synology.url` | DSM base URL | `""` |
| `synology.username` | DSM username | `""` |
| `synology.password` | DSM password | `""` |
| `synology.skipTLSVerify` | Skip TLS certificate check | `false` |
| `synology.existingSecret` | Name of an existing Secret with DSM credentials | `""` |
| `operator.defaultDomain` | Domain suffix for auto-derived hostnames | `""` |
| `operator.defaultACLProfile` | ACL profile applied when none is specified | `""` |
| `operator.enableArgoWatcher` | Enable ArgoCD Application watcher | `true` |
| `operator.watchNamespace` | Restrict ArgoCD watcher to one namespace | `""` (all) |
| `operator.ruleNamespace` | Namespace for auto-created `SynologyProxyRule` objects | `synology-proxy-operator` |
| `installCRDs` | Install CRDs via Helm | `true` |
| `leaderElection` | Enable leader election for HA deployments | `false` |

---

## Development

### Prerequisites

| Tool | Install |
|---|---|
| Go ≥ 1.22 | `brew install go` |
| Docker | Docker Desktop |
| minikube | `brew install minikube` |
| kubectl | `brew install kubectl` |
| Helm | `brew install helm` |

```bash
make controller-gen   # install controller-gen locally
go mod tidy
```

### Build and test

```bash
make build    # compile binary → bin/manager
make lint     # golangci-lint
make test     # unit tests with envtest
```

### Container image

```bash
# Build with Docker (default) or Podman
make image-build CONTAINER_TOOL=podman IMG=synology-proxy-operator:dev

# Multi-arch push
make image-buildx IMG=ghcr.io/phoeluga/synology-proxy-operator:latest
```

### Regenerate CRD manifests

After changing `api/v1alpha1/synologyproxyrule_types.go`:

```bash
make manifests   # regenerates config/crd/bases/*.yaml + config/rbac/
make generate    # regenerates zz_generated.deepcopy.go
cp config/crd/bases/*.yaml helm/synology-proxy-operator/crds/
```

### Local dev environment

See [docs/local-testing.md](docs/local-testing.md) for a full walkthrough using minikube and the VSCode debugger.

Quick start:

```bash
make minikube-start       # start dev cluster (docker driver)
make dev-setup            # install CRDs + ArgoCD + test fixtures
# edit .env.local with Synology credentials
make dev-run              # run operator locally
# or press F5 in VSCode with "Run operator (minikube)" selected
```

---

## Release

1. Bump `version` and `appVersion` in `helm/synology-proxy-operator/Chart.yaml`
2. Push a semver tag:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The [release workflow](.github/workflows/release.yaml) automatically:
- Builds and pushes a multi-arch image (`linux/amd64`, `linux/arm64`) to GHCR
- Packages and attaches the Helm chart to the GitHub Release

The [CI workflow](.github/workflows/ci.yaml) pushes `:latest` to GHCR on every merge
to `main`. In-progress runs are cancelled when a new push arrives.

---

## Project structure

```
synology-proxy-operator/
├── api/v1alpha1/
│   ├── synologyproxyrule_types.go   # CRD type definitions
│   └── zz_generated.deepcopy.go     # Generated deepcopy methods
├── cmd/
│   └── main.go                      # Operator entry point, flag/env wiring
├── internal/
│   ├── argo/
│   │   └── types.go                 # Minimal ArgoCD types (no full dependency)
│   ├── controller/
│   │   ├── argoapplication_controller.go    # ArgoCD App → SynologyProxyRule
│   │   ├── serviceingress_controller.go     # Service/Ingress → SynologyProxyRule
│   │   └── synologyproxyrule_controller.go  # SynologyProxyRule → DSM API
│   └── synology/
│       ├── client.go           # HTTP client, session, SynoToken, wire types
│       ├── proxy.go            # Proxy record CRUD + idempotency check
│       ├── certificate.go      # Certificate matching + assignment
│       └── acl.go              # ACL profile UUID resolution
├── config/
│   ├── crd/bases/              # Generated CRD manifests
│   ├── rbac/                   # ClusterRole, ClusterRoleBinding, ServiceAccount
│   └── manager/                # Deployment + ConfigMap
├── hack/
│   └── dev/                    # Local dev fixtures
├── helm/
│   └── synology-proxy-operator/
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── crds/
│       └── templates/
├── docs/
│   └── local-testing.md
├── .github/workflows/
│   ├── ci.yaml
│   └── release.yaml
├── .vscode/
│   ├── launch.json
│   └── tasks.json
├── .env.local.example
├── Dockerfile
├── Makefile
└── README.md
```

---

## License

Apache 2.0
