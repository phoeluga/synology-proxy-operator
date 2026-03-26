# synology-proxy-operator

A Kubernetes operator that manages reverse proxy records on a Synology NAS via its WebAPI.
Define a `SynologyReverseProxy` custom resource and the operator will create, update, or delete
the corresponding entry in Synology's reverse proxy configuration — including optional wildcard
certificate assignment and ACL profile binding.

---

## Overview

The operator watches `SynologyReverseProxy` resources (CRD group `proxy.hnet.io/v1alpha1`) and
reconciles them against the Synology DSM WebAPI. It supports:

- **Login** — acquires a session SID + SynoToken via `SYNO.API.Auth`
- **List / find** — queries `SYNO.Core.ReverseProxy.Rule` to detect existing records
- **Upsert** — creates a new record or updates an existing one matched by `description`
- **Delete** — removes the record when the CR is deleted (via a Kubernetes finalizer)
- **Certificate assignment** — finds a wildcard cert matching the source hostname via `SYNO.Core.Certificate` and binds it to the proxy rule
- **ACL profile** — resolves an ACL profile name to its ID via `SYNO.Core.ReverseProxy.ACL`

---

## Prerequisites

- Kubernetes 1.26+
- `kubectl` configured against your cluster
- A Synology NAS running DSM 7.x with the reverse proxy feature enabled
- A Synology user account with permission to manage reverse proxy rules and certificates
- Docker (for building the image)

---

## Quick deploy to cluster

### 1. Build and push the image

```bash
make docker-build IMG=ghcr.io/yourorg/synology-proxy-operator:latest
make docker-push  IMG=ghcr.io/yourorg/synology-proxy-operator:latest
```

### 2. Create the credentials Secret

```bash
kubectl create namespace synology-proxy-operator

kubectl create secret generic synology-credentials \
  --namespace synology-proxy-operator \
  --from-literal=url=https://nas.hnet.io:5001 \
  --from-literal=username=admin \
  --from-literal=password=supersecret \
  --from-literal=skipTLSVerify=false
```

### 3. Deploy the operator

```bash
make deploy IMG=ghcr.io/yourorg/synology-proxy-operator:latest
```

This applies the CRD, RBAC resources, and the manager Deployment in one step.

### 4. Verify the operator is running

```bash
kubectl -n synology-proxy-operator get pods
kubectl -n synology-proxy-operator logs -l app=synology-proxy-operator -f
```

---

## Configuration — credentials Secret

The operator reads a Secret named `synology-credentials` from its own namespace.

| Key             | Required | Description                                              |
|-----------------|----------|----------------------------------------------------------|
| `url`           | yes      | Base URL of the Synology DSM, e.g. `https://nas:5001`   |
| `username`      | yes      | DSM username                                             |
| `password`      | yes      | DSM password                                             |
| `skipTLSVerify` | no       | Set to `"true"` to skip TLS certificate verification     |

Example manifest (use Sealed Secrets or an external secrets operator in production):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: synology-credentials
  namespace: synology-proxy-operator
type: Opaque
stringData:
  url: "https://nas.hnet.io:5001"
  username: "admin"
  password: "supersecret"
  skipTLSVerify: "false"
```

---

## CRD usage examples

### Basic HTTP → HTTPS proxy

```yaml
apiVersion: proxy.hnet.io/v1alpha1
kind: SynologyReverseProxy
metadata:
  name: my-app
  namespace: default
spec:
  description: "my-app"
  sourceHostname: "myapp.hnet.io"
  sourcePort: 443
  sourceProtocol: https
  destHostname: "10.1.4.200"
  destPort: 8080
  destProtocol: http
  assignCertificate: true
```

### With ACL profile

```yaml
apiVersion: proxy.hnet.io/v1alpha1
kind: SynologyReverseProxy
metadata:
  name: internal-app
  namespace: default
spec:
  description: "internal-app"
  sourceHostname: "internal.hnet.io"
  sourcePort: 443
  sourceProtocol: https
  destHostname: "10.1.4.201"
  destPort: 9090
  destProtocol: http
  aclProfile: "LAN-only"
  assignCertificate: true
```

### Check status

```bash
kubectl get srp
kubectl describe srp my-app
```

The `.status` fields show:

- `uuid` — the Synology record UUID
- `certId` — the assigned certificate ID
- `conditions` — `Ready` and `Synced` conditions

---

## Argo CD ApplicationSet integration

The operator fits naturally into a GitOps workflow. Add it as an application in your
ApplicationSet so it is deployed alongside your other infrastructure:

```yaml
# clusters/hnet-k8s-prod/apps/prod/synology-proxy-operator/application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: synology-proxy-operator
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    repoURL: 'git@github.com:phoeluga/hnet-k8s-cluster.git'
    targetRevision: main
    path: synology-proxy-operator/config
    directory:
      recurse: true
  destination:
    server: https://kubernetes.default.svc
    namespace: synology-proxy-operator
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

Individual `SynologyReverseProxy` CRs can then live in any application directory and be
managed by the existing `prod-appset.yaml` ApplicationSet — the operator will reconcile them
automatically.

---

## Uninstall

```bash
make undeploy
```

This removes the Deployment, RBAC resources, CRD, and the operator namespace.
Existing `SynologyReverseProxy` resources will be garbage-collected; the operator will call
the Synology API to delete the corresponding proxy records before removing the finalizer.
