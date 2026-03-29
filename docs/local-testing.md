# Local Testing Guide

This guide covers running the operator locally against a **minikube** cluster (with Podman or Docker as the container runtime), including VSCode debugging.

---

## Prerequisites

| Tool | Install | Notes |
|---|---|---|
| Go ≥ 1.22 | `brew install go` | |
| minikube | `brew install minikube` | |
| Docker | Docker Desktop | Required as minikube driver — see note below |
| kubectl | `brew install kubectl` | |
| Helm | `brew install helm` | |

> **minikube driver on macOS:** Use the `docker` driver (default in the Makefile).
> The `podman` driver runs inside a Podman Machine VM that lacks `cpuset` cgroups,
> which kubeadm requires. Docker Desktop's VM exposes full cgroup support.

---

## 1. Create the dev cluster

```bash
# Uses podman by default. Switch to docker with MINIKUBE_DRIVER=docker.
make minikube-start

# Switch your kubectl context to the dev cluster
make minikube-context

# Verify
kubectl get nodes
```

Default profile name is `synology-dev`. Override with `MINIKUBE_PROFILE=<name>`.

---

## 2. Install CRDs and RBAC

```bash
make dev-install
```

This applies `config/crd/bases/` and `config/rbac/` and creates the `synology-proxy-operator` namespace.

---

## 3. Install ArgoCD (optional)

Only needed if you want to test the ArgoCD Application watcher.

```bash
make dev-argocd
```

This installs ArgoCD, waits for it to be ready, and prints the initial admin password.

---

## 4. Deploy test resources

```bash
make dev-test-app
```

Applies everything in `hack/dev/`:
- `nginx` Deployment + LoadBalancer Service in namespace `nginx-test`
- A manual `SynologyProxyRule` pointing at nginx
- An ArgoCD `Application` with proxy annotations (if ArgoCD is installed)

---

## 5. Expose LoadBalancer IPs

minikube's LoadBalancer support requires a tunnel. Run this in a **separate terminal** and leave it open:

```bash
make minikube-tunnel
```

After a few seconds the nginx Service gets an ExternalIP:

```bash
kubectl get svc -n nginx-test nginx
# NAME    TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)        AGE
# nginx   LoadBalancer   10.96.x.x      127.0.0.x     80:32xxx/TCP   1m
```

---

## 6. Configure credentials

```bash
cp .env.local.example .env.local
```

Edit `.env.local`:

```bash
SYNOLOGY_URL=https://192.168.1.x:5001
SYNOLOGY_USER=admin
SYNOLOGY_PASSWORD=changeme
SYNOLOGY_SKIP_TLS_VERIFY=true

DEFAULT_DOMAIN=
DEFAULT_ACL_PROFILE=
RULE_NAMESPACE=synology-proxy-operator
ENABLE_ARGO_WATCHER=true
```

`.env.local` is git-ignored — never commit it.

---

## 7. Run the operator

### Option A — VSCode debugger (recommended)

1. Open the **Run and Debug** panel (`⇧⌘D`)
2. Select **"Run operator (minikube)"**
3. Press **F5**

The operator runs on your Mac with the minikube kubeconfig, with full breakpoint and variable inspection support.

Good breakpoints to set for the 4151 / DSM API errors:
- [`synologyproxyrule_controller.go:98`](../internal/controller/synologyproxyrule_controller.go#L98) — `reconcileUpsert` entry
- [`proxy.go:100`](../internal/synology/proxy.go#L100) — `UpsertProxyRule`, just before the DSM call
- [`client.go:250`](../internal/synology/client.go#L250) — `post`, to inspect raw HTTP responses

### Option B — Terminal

```bash
make dev-run
```

Reads `.env.local` automatically and runs with `--zap-devel=true` for pretty-printed logs.

---

## 8. Test scenarios

### Scenario A — Manual SynologyProxyRule

`hack/dev/02-proxy-rule.yaml` is already applied by `make dev-test-app`. Watch it reconcile:

```bash
kubectl get spr -n synology-proxy-operator -w
```

Expected: `SYNCED = true` and `UUID` populated once the DSM call succeeds.

```bash
# Describe for conditions and events
kubectl describe spr nginx-test -n synology-proxy-operator

# Force re-reconcile
kubectl annotate spr nginx-test -n synology-proxy-operator force-sync="$(date +%s)" --overwrite
```

### Scenario B — ArgoCD Application → auto-create SynologyProxyRule

`hack/dev/03-argo-app.yaml` creates an ArgoCD Application annotated with `synology.proxy/enabled: "true"`. The ArgoCD watcher creates a `SynologyProxyRule` named `nginx-argo` automatically:

```bash
kubectl get spr -n synology-proxy-operator
kubectl describe spr nginx-argo -n synology-proxy-operator
```

### Scenario C — Deletion

```bash
# Delete the ArgoCD Application — the owned SynologyProxyRule is garbage-collected
# and the finalizer removes the DSM record before deletion completes.
kubectl delete application nginx-argo -n argocd

kubectl get spr -n synology-proxy-operator   # should be gone within seconds
```

### Scenario D — Manually-managed rule is not deleted

The ArgoCD watcher must NOT delete `SynologyProxyRule` objects it did not create
(i.e. those without an owner reference). Verify:

```bash
# The nginx-test rule has no owner reference
kubectl get spr nginx-test -n synology-proxy-operator -o jsonpath='{.metadata.ownerReferences}'
# → should be empty []

# Disabling the annotation on the argo app should not touch nginx-test
kubectl annotate application nginx-argo -n argocd synology.proxy/enabled=false --overwrite
kubectl get spr nginx-test -n synology-proxy-operator   # must still exist
```

---

## 9. Known DSM API behaviours

| Behaviour | Details |
|---|---|
| **Write timeout** | Create/update calls can take up to 2 minutes. The client timeout is set to 3 minutes. |
| **`SynoToken` required in body** | DSM 7.x CSRF protection requires the token in both the `X-SYNO-TOKEN` header and the form body for all mutating requests. Read-only calls (`list`) work without it. |
| **Update method** | The correct method for updating a record is `update` (not `set`). The `_key` field returned by `list` must be included alongside `UUID`. |
| **Certificate `list` not supported** | `SYNO.Core.Certificate.Service` does not support a `list` method. The operator always passes `old_id: ""` when assigning certificates. |
| **Certificate fallback** | When no certificate CN/SAN matches the source hostname, the operator assigns the DSM default certificate (`is_default: true`). |
| **Idempotency** | The operator compares the existing DSM record against the desired state before writing. If nothing changed, the DSM API is not called. |
| **Cross-namespace owner refs** | ArgoCD Applications live in the `argocd` namespace while rules are created in `synology-proxy-operator`. Kubernetes disallows cross-namespace owner references; ownership is tracked via labels instead (`proxy.synology.io/managed-by-argo-app`). |

---

## 10. Inspect DSM proxy records

Log in to your Synology DSM:
**Control Panel → Application Portal → Reverse Proxy**

Or query the API directly:

```bash
# Authenticate first
SID=$(curl -sk "https://$SYNOLOGY_URL/webapi/auth.cgi" \
  -d "api=SYNO.API.Auth&version=3&method=login&account=$SYNOLOGY_USER&passwd=$SYNOLOGY_PASSWORD&session=test&format=sid" \
  | jq -r '.data.sid')

# List all proxy records
curl -sk "https://$SYNOLOGY_URL/webapi/entry.cgi/SYNO.Core.AppPortal.ReverseProxy" \
  -d "api=SYNO.Core.AppPortal.ReverseProxy&method=list&version=1&_sid=$SID" | jq '.data.entries[] | {description,frontend:.frontend.fqdn}'
```

---

## 10. Debugging tips

```bash
# Operator logs (pretty-printed in dev mode)
# (when running via make dev-run or VSCode)

# Events on a rule
kubectl describe spr <name> -n synology-proxy-operator

# Check RBAC
kubectl auth can-i list applications \
  --as=system:serviceaccount:synology-proxy-operator:synology-proxy-operator -n argocd
kubectl auth can-i list services \
  --as=system:serviceaccount:synology-proxy-operator:synology-proxy-operator --all-namespaces

# Inspect what the operator would send to DSM (set breakpoint in proxy.go:UpsertProxyRule)
# or add temporary logging around the entryJSON marshal in proxy.go
```

---

## 11. Tear down

```bash
# Remove test resources only
make dev-clean

# Stop the cluster (preserves state)
make minikube-stop

# Delete the cluster entirely
make minikube-delete
```
