# Synology Proxy Operator

A Kubernetes operator that automatically manages **Synology DSM reverse proxy records** based on ArgoCD Applications and `SynologyProxyRule` custom resources.

## How it works

```
ArgoCD Application  ──annotation──▶  ArgoApplicationReconciler
                                             │
                                             ▼
                                    SynologyProxyRule CRD
                                             │
                                             ▼
                              SynologyProxyRuleReconciler
                                             │
                          ┌──────────────────┼──────────────────┐
                          ▼                  ▼                  ▼
                  Discover backend    Upsert DSM proxy    Assign cert
                  (Service/Ingress)   record via API      (wildcard/SAN)
```

1. **ArgoCD watcher** — watches ArgoCD `Application` objects annotated with `synology.proxy/enabled: "true"`.  For each such app it creates or updates a `SynologyProxyRule` CRD in the configured namespace, auto-populating source hostname, backend reference, and ACL profile from annotations and defaults.

2. **Proxy rule reconciler** — watches `SynologyProxyRule` CRDs.  For each rule it:
   - Auto-discovers the backend IP / port from a `LoadBalancer` Service or `Ingress` (or uses explicit values).
   - Resolves the ACL profile UUID from DSM.
   - Creates or updates the reverse proxy entry in Synology DSM via the WebAPI.
   - Assigns the best matching wildcard/SAN certificate.
   - Updates `.status` with the DSM UUID and sync result.

You can also create `SynologyProxyRule` objects **directly**, without ArgoCD.

---

## Prerequisites

| Tool | Version |
|------|---------|
| Go | ≥ 1.22 |
| Docker | ≥ 24 |
| kubectl | ≥ 1.28 |
| Helm | ≥ 3.14 |
| ArgoCD (optional) | ≥ 2.8 |

---

## Quick start (Helm)

```bash
helm upgrade --install synology-proxy-operator \
  oci://ghcr.io/synology-proxy-operator/charts/synology-proxy-operator \
  --namespace synology-proxy-operator \
  --create-namespace \
  --set synology.url="https://diskstation.local:5001" \
  --set synology.username="admin" \
  --set synology.password="secret" \
  --set synology.skipTLSVerify=true \
  --set operator.defaultDomain="home.example.com"
```

After installation, annotate an ArgoCD Application:

```yaml
metadata:
  annotations:
    synology.proxy/enabled: "true"
    # Optional overrides:
    synology.proxy/source-host: "myapp.home.example.com"
    synology.proxy/acl-profile: "LAN Only"
    synology.proxy/service-ref: "myapp/myapp-svc"
```

Or create a rule directly:

```yaml
apiVersion: proxy.synology.io/v1alpha1
kind: SynologyProxyRule
metadata:
  name: myapp
  namespace: synology-proxy-operator
spec:
  sourceHost: myapp.home.example.com
  serviceRef:
    name: myapp-svc
    namespace: myapp
  aclProfile: "LAN Only"
```

---

## ArgoCD Application annotations

| Annotation | Description | Default |
|---|---|---|
| `synology.proxy/enabled` | `"true"` to enable proxy management | (required) |
| `synology.proxy/source-host` | Public FQDN for the frontend | `<app-name>.<defaultDomain>` |
| `synology.proxy/acl-profile` | Synology ACL profile name | operator default |
| `synology.proxy/destination-protocol` | `http` or `https` | `http` |
| `synology.proxy/destination-host` | Backend IP/hostname override | auto-discovered |
| `synology.proxy/destination-port` | Backend port override | auto-discovered |
| `synology.proxy/assign-certificate` | `"false"` to skip cert assignment | `"true"` |
| `synology.proxy/service-ref` | `<namespace>/<service>` for discovery | auto-scan |
| `synology.proxy/ingress-ref` | `<namespace>/<ingress>` for discovery | auto-scan |

---

## SynologyProxyRule CRD reference

```yaml
apiVersion: proxy.synology.io/v1alpha1
kind: SynologyProxyRule
metadata:
  name: myapp
  namespace: synology-proxy-operator
spec:
  # ── Required ────────────────────────────────────────
  sourceHost: myapp.home.example.com   # Public FQDN (frontend)

  # ── Optional: source ────────────────────────────────
  sourcePort: 443                      # Default: 443

  # ── Optional: backend ────────────────────────────────
  destinationHost: ""                  # Auto-discovered when empty
  destinationPort: 0                   # Auto-discovered when 0
  destinationProtocol: http            # http (default) | https

  # ── Optional: backend auto-discovery ─────────────────
  serviceRef:
    name: myapp-svc
    namespace: myapp                   # Defaults to rule namespace
  ingressRef:
    name: myapp-ingress
    namespace: myapp

  # ── Optional: Synology config ─────────────────────────
  aclProfile: "LAN Only"
  assignCertificate: true

  customHeaders:
    - name: Upgrade
      value: $http_upgrade
    - name: Connection
      value: $connection_upgrade

  timeouts:
    connect: 60
    read:    60
    send:    60
```

### Status fields

```
kubectl get spr -A

NAMESPACE                  NAME     SOURCE HOST                  DESTINATION     SYNCED  UUID          AGE
synology-proxy-operator    myapp    myapp.home.example.com       192.168.1.100   true    abc-def-123   5m
```

---

## Configuration reference (Helm values)

| Value | Description | Default |
|---|---|---|
| `synology.url` | DSM base URL | `""` |
| `synology.username` | DSM username | `""` |
| `synology.password` | DSM password | `""` |
| `synology.skipTLSVerify` | Skip TLS check | `false` |
| `synology.existingSecret` | Use existing Secret for credentials | `""` |
| `operator.defaultDomain` | Domain suffix for auto-generated hostnames | `""` |
| `operator.defaultACLProfile` | Default ACL profile applied to all rules | `""` |
| `operator.enableArgoWatcher` | Enable ArgoCD Application watcher | `true` |
| `operator.watchNamespace` | Namespace to watch for ArgoCD Apps | `""` (all) |
| `operator.ruleNamespace` | Namespace for auto-created `SynologyProxyRule` objects | `synology-proxy-operator` |
| `installCRDs` | Install CRDs via Helm | `true` |
| `leaderElection` | Enable leader election (for HA) | `false` |

---

## Development

### Prerequisites

```bash
# Install Go toolchain (https://go.dev/dl/)
go version   # >= 1.22

# Install controller-gen
make controller-gen

# Download module dependencies
go mod tidy
```

### Build

```bash
# Compile binary
make build

# Run linter
make lint

# Run tests
make test
```

### Regenerate CRD manifests

After changing `api/v1alpha1/synologyproxyrule_types.go`:

```bash
make manifests   # regenerates config/crd/bases/*.yaml
make generate    # regenerates zz_generated.deepcopy.go
```

Copy the updated CRD into the Helm chart:

```bash
cp config/crd/bases/*.yaml helm/synology-proxy-operator/crds/
```

---

## Local testing

See [docs/local-testing.md](docs/local-testing.md) for a complete step-by-step guide using Kind.

---

## Release

1. Bump the version in `helm/synology-proxy-operator/Chart.yaml`.
2. Create and push a semver tag:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The [release workflow](.github/workflows/release.yaml) will:
- Run tests
- Build and push a multi-arch Docker image to GHCR
- Package and attach the Helm chart to the GitHub Release

---

## Project structure

```
synology-proxy-operator/
├── api/v1alpha1/               # CRD type definitions
│   ├── groupversion_info.go
│   ├── synologyproxyrule_types.go
│   └── zz_generated.deepcopy.go
├── cmd/
│   └── main.go                 # Operator entry point
├── internal/
│   ├── argo/                   # Minimal ArgoCD types (no full dependency)
│   │   └── types.go
│   ├── controller/
│   │   ├── argoapplication_controller.go   # Watches ArgoCD Apps → creates SynologyProxyRule
│   │   └── synologyproxyrule_controller.go # Reconciles rules with DSM
│   └── synology/               # Synology DSM API client
│       ├── client.go           # HTTP session, login, post helper
│       ├── proxy.go            # Proxy record CRUD
│       ├── certificate.go      # Certificate lookup & assignment
│       └── acl.go              # ACL profile resolution
├── config/
│   ├── crd/bases/              # CRD YAML manifests (generated)
│   ├── rbac/                   # ClusterRole, ClusterRoleBinding, ServiceAccount
│   └── manager/                # Deployment manifest
├── helm/
│   └── synology-proxy-operator/
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── crds/               # CRD (installed before templates)
│       └── templates/
├── docs/
│   └── local-testing.md
├── .github/workflows/
│   ├── ci.yaml
│   └── release.yaml
├── Dockerfile
├── Makefile
└── README.md
```

---

## License

Apache 2.0
