<p align="center">
    <img src="https://raw.githubusercontent.com/phoeluga/synology-proxy-operator/main/docs/images/synoProxyOperator_1.png" alt="" style="border-radius: 13px;" width="80%" >
</p>


[![CI](https://github.com/phoeluga/synology-proxy-operator/actions/workflows/ci-build-and-test.yaml/badge.svg)](https://github.com/phoeluga/synology-proxy-operator/actions/workflows/ci-build-and-test.yaml)
[![Release](https://img.shields.io/github/v/release/phoeluga/synology-proxy-operator?label=latest%20release)](https://github.com/phoeluga/synology-proxy-operator/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/phoeluga/synology-proxy-operator)](https://goreportcard.com/report/github.com/phoeluga/synology-proxy-operator)
[![Go Version](https://img.shields.io/github/go-mod/go-version/phoeluga/synology-proxy-operator)](go.mod)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/synology-proxy-operator)](https://artifacthub.io/packages/helm/synology-proxy-operator/synology-proxy-operator)

[![Donate](https://img.shields.io/static/v1?label=Treat%20a%20coffee&message=donate%20a%20tip&color=2a9cde&logo=data:image/svg+xml;base64,PHN2ZyB2aWV3Qm94PSIwIDAgMjQgMjQiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+PHBhdGggZD0iTTcgMjJoMTBhMSAxIDAgMCAwIC45OS0uODU4TDE5Ljg2NyA4SDIxVjZoLTEuMzgybC0xLjcyNC0zLjQ0N0EuOTk4Ljk5OCAwIDAgMCAxNyAySDdjLS4zNzkgMC0uNzI1LjIxNC0uODk1LjU1M0w0LjM4MiA2SDN2MmgxLjEzM0w2LjAxIDIxLjE0MkExIDEgMCAwIDAgNyAyMnptMTAuNDE4LTExSDYuNTgybC0uNDI5LTNoMTEuNjkzbC0uNDI4IDN6bS05LjU1MSA5LS40MjktM2g5LjEyM2wtLjQyOSAzSDcuODY3ek03LjYxOCA0aDguNzY0bDEgMkg2LjYxOGwxLTJ6IiBmaWxsPSIjZWRmMmZhIiBjbGFzcz0iZmlsbC0wMDAwMDAiPjwvcGF0aD48L3N2Zz4=)](https://www.paypal.com/donate/?hosted_button_id=9MLB29CKX5674)



> **Deploy an app. Get a working HTTPS reverse proxy entry. Automatically.**
> No manual DSM configuration. No certificate assignment. No cleanup to remember.


---

You have a Synology NAS acting as your home lab gateway. Every time you deploy a new service to Kubernetes, you open DSM, navigate to Application Portal → Reverse Proxy, fill in the hostname, the backend IP, the port, and assign a certificate. When a LoadBalancer IP changes, you update it manually. When you remove an app, you remember (or forget) to clean up the rule.

**Synology Proxy Operator eliminates all of that.** It watches your Kubernetes cluster and keeps your Synology DSM reverse proxy configuration in sync — automatically. Deploy an app, get a reverse proxy entry. Delete it, the rule is gone. Change the backend IP, the rule is updated. All without touching DSM.

On top of that, the operator:
- **Assigns TLS certificates automatically** — picks the best matching wildcard or SAN certificate from DSM, falls back to the default certificate
- **Enforces access control** — apply a Synology ACL profile (e.g. "LAN Only") globally or per-rule to restrict which clients can reach a service
- **Supports additional hostnames** — one app, multiple public FQDNs, each with its own DSM record and certificate
- **Cleans up reliably** — a Kubernetes finalizer ensures DSM records are removed before the rule object is deleted, even if the operator restarts mid-deletion

---

## How it works

<p align="center">
    <img src="https://raw.githubusercontent.com/phoeluga/synology-proxy-operator/main/docs/images/chart_howItWorks.png" alt="" width="95%" >
</p>

The operator runs three controllers:

| Controller | Watches | Action |
|---|---|---|
| `ServiceIngressReconciler` | Services + Ingresses with `synology.proxy/enabled: "true"` | Creates / deletes `SynologyProxyRule` objects |
| `ArgoApplicationReconciler` | ArgoCD `Application` objects with `synology.proxy/enabled: "true"` | Creates / deletes `SynologyProxyRule` objects |
| `SynologyProxyRuleReconciler` | All `SynologyProxyRule` objects | Syncs to DSM — the **only** controller that calls the DSM API |

The first two controllers are purely Kubernetes-side. All DSM interaction flows through the third.

---

## Zero-touch reverse proxy management

With `operator.defaultDomain` configured, the full lifecycle is hands-free:

1. You deploy an app to Kubernetes
2. The operator detects the new Service or ArgoCD Application
3. A DSM reverse proxy rule is created for `myapp.home.example.com → <backend IP:port>`
4. The best matching TLS certificate is automatically assigned
5. If an ACL profile is configured, access restrictions are applied immediately
6. When you delete the app, the DSM rule is removed automatically

### Optional — automatic DNS:
If you run the Synology DNS Server package, it can automatically create A records for hostnames the reverse proxy manages. Combined with forwarding your internal DNS queries to the NAS, even DNS is zero-touch.

If you already use some DNS server like e.g. Pi-Hole you can set the upstream DNS to your NAS IP. This will provide an end-to-end workflow of provisioning new services to become available.

---

## Prerequisites

| Requirement | Version |
|---|---|
| Synology DSM | ≥ 7.0 (WebAPI access required) |
| Kubernetes | ≥ 1.28 |
| ArgoCD | ≥ 2.8 — optional, only for the ArgoCD watcher |

---

## Installation

### Option A — Helm from GHCR (recommended for stable releases)

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

#### Using an existing Secret

If you manage credentials externally (SealedSecret, External Secrets Operator, etc.):

```bash
helm upgrade --install synology-proxy-operator \
  oci://ghcr.io/phoeluga/charts/synology-proxy-operator \
  --namespace synology-proxy-operator \
  --create-namespace \
  --set synology.existingSecret=synology-credentials \
  --set operator.defaultDomain="home.example.com"
```

The Secret must have keys: `SYNOLOGY_URL`, `SYNOLOGY_USER`, `SYNOLOGY_PASSWORD`, `SYNOLOGY_SKIP_TLS_VERIFY`.

---

### Option B — Helm from Git (tracks `main` branch, includes latest unreleased changes)

Use this when you want the latest fixes and features before a release, or when you need the CRD and chart to stay in sync with the operator binary automatically.

```bash
helm upgrade --install synology-proxy-operator \
  ./helm/synology-proxy-operator \
  --namespace synology-proxy-operator \
  --create-namespace \
  --set synology.existingSecret=synology-credentials \
  --set operator.defaultDomain="home.example.com" \
  --set image.tag=main \
  --set image.pullPolicy=Always
```

Or clone the repo and point Helm at the local chart directory.

---

### Option C — GitOps with ArgoCD

The operator references credentials via `synology.existingSecret`. Create the Secret separately before ArgoCD syncs.

**Step 1 — create the credentials Secret** (once, outside ArgoCD):

```bash
kubectl create secret generic synology-credentials \
  --namespace synology-proxy-operator \
  --from-literal=SYNOLOGY_URL="https://192.168.1.x:5001" \
  --from-literal=SYNOLOGY_USER="admin" \
  --from-literal=SYNOLOGY_PASSWORD="secret" \
  --from-literal=SYNOLOGY_SKIP_TLS_VERIFY="false"
```

**Step 2 — deploy with ArgoCD (stable GHCR release):**

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: synology-proxy-operator
  namespace: argocd
spec:
  source:
    repoURL: ghcr.io/phoeluga/charts
    chart: synology-proxy-operator
    targetRevision: ">=0.0.0"
    helm:
      valuesObject:
        operator:
          defaultDomain: "home.example.com"
        synology:
          existingSecret: synology-credentials
  destination:
    server: https://kubernetes.default.svc
    namespace: synology-proxy-operator
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
```

**Alternative — track `main` branch directly (chart + CRD + binary always in sync):**

```yaml
  source:
    repoURL: 'https://github.com/phoeluga/synology-proxy-operator.git'
    targetRevision: main
    path: helm/synology-proxy-operator
    helm:
      valuesObject:
        operator:
          defaultDomain: "home.example.com"
        synology:
          existingSecret: synology-credentials
        image:
          tag: main
          pullPolicy: Always
```

> `ServerSideApply=true` is required for ArgoCD to upgrade CRDs on sync. Without it, CRD schema changes (e.g. new fields) are not applied on `helm upgrade` or ArgoCD sync.

> For a fully GitOps credential workflow, encrypt the Secret with [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) or manage it via [External Secrets Operator](https://external-secrets.io) and commit only the encrypted/reference form to your cluster repo.

---

## Configuration

All settings are read from environment variables. Helm sets these automatically from `values.yaml`.

| Variable | Description | Default |
|---|---|---|
| `SYNOLOGY_URL` | DSM base URL — e.g. `https://192.168.1.x:5001` | required |
| `SYNOLOGY_USER` | DSM username | required |
| `SYNOLOGY_PASSWORD` | DSM password | required |
| `SYNOLOGY_SKIP_TLS_VERIFY` | Skip TLS verification (self-signed certs) | `false` |
| `DEFAULT_DOMAIN` | Domain appended to auto-derived hostnames, e.g. `home.example.com` | `""` |
| `DEFAULT_ACL_PROFILE` | Synology ACL profile name applied to all rules that do not specify one | `""` |
| `RULE_NAMESPACE` | Namespace where auto-created `SynologyProxyRule` objects are placed. Empty = source app namespace | `""` |
| `ENABLE_ARGO_WATCHER` | Enable the ArgoCD Application watcher | `true` |
| `WATCH_NAMESPACE` | Namespace glob pattern (e.g. `app-*`) — Services, Ingresses and ArgoCD Applications in matching namespaces are auto-managed without needing the `synology.proxy/enabled` annotation. Empty = annotation-only mode. | `""` |

---

## Usage

There are three ways to use the operator. Pick the one that fits your workflow.

> **Tip — skip annotations entirely:** set `WATCH_NAMESPACE` to a glob pattern (e.g. `app-*`) and every Service, Ingress, and ArgoCD Application in matching namespaces is managed automatically — no annotation required.

> **Fine-grained control within a glob-managed namespace:**
> - Set `synology.proxy/enabled: "false"` on a **resource** to exclude it from auto-management, even when its namespace matches the glob.
> - Set `synology.proxy/auto-discovery: "false"` on a **Namespace** to disable glob-based auto-management for the whole namespace. Individual resources in that namespace can still opt in with `synology.proxy/enabled: "true"`.

### Mode 1 — Annotate a Service or Ingress

The simplest approach: add one annotation to any existing Service or Ingress. The operator handles the rest.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: myapp
  namespace: myapp
  annotations:
    synology.proxy/enabled: "true"
    # Optional — omit if DEFAULT_DOMAIN is set; hostname becomes "myapp.home.example.com"
    synology.proxy/source-host: "myapp.home.example.com"
spec:
  type: LoadBalancer
  ports:
    - port: 80
```

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

Removing the `synology.proxy/enabled` annotation — or deleting the object — removes the DSM record automatically.

---

### Mode 2 — Annotate an ArgoCD Application

Annotate the `Application` object. The operator discovers the backend from the application's destination namespace automatically.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: myapp
  namespace: argocd
  annotations:
    synology.proxy/enabled: "true"
    # Optional — defaults to "myapp.home.example.com" when DEFAULT_DOMAIN is set
    synology.proxy/source-host: "myapp.home.example.com"
    # Optional — pin to a specific Service; otherwise auto-scans the namespace
    synology.proxy/service-ref: "myapp/myapp-svc"
    # Optional — restrict access using a Synology ACL profile
    synology.proxy/acl-profile: "LAN Only"
spec:
  destination:
    namespace: myapp
    server: https://kubernetes.default.svc
```

---

### Mode 3 — Manual SynologyProxyRule

Create a `SynologyProxyRule` directly for full control. Useful for services outside Kubernetes (VMs, NAS services, IoT devices).

**Minimal — with `DEFAULT_DOMAIN` configured:**

```yaml
apiVersion: proxy.synology.io/v1alpha1
kind: SynologyProxyRule
metadata:
  name: myapp
  namespace: myapp
spec:
  serviceRef:
    name: myapp
    namespace: myapp
```

This creates a DSM record for `myapp.home.example.com` pointing at the LoadBalancer IP of the `myapp` Service.

**Explicit — no auto-discovery:**

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
  aclProfile: "LAN Only"
```

**Multiple public hostnames for the same backend:**

```yaml
spec:
  sourceHost: myapp.home.example.com
  additionalSourceHosts:
    - myapp.example.org
  serviceRef:
    name: myapp
    namespace: myapp
```

Each hostname gets its own DSM record and certificate assignment.

---

## Controlling auto-discovery per namespace

When `WATCH_NAMESPACE` is set to a glob pattern, every resource in matching namespaces is managed automatically. Two annotations give you fine-grained control when you need it.

### Exclude one resource from a glob-managed namespace

Set `synology.proxy/enabled: "false"` on the resource. This overrides the glob — the operator will skip it and clean up any existing DSM rule:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: internal-debug
  namespace: app-myapp     # matches WATCH_NAMESPACE=app-*
  annotations:
    synology.proxy/enabled: "false"   # excluded — no DSM rule created
```

### Disable auto-discovery for an entire namespace

Annotate the **Namespace** itself. This stops the glob from matching any resource inside it:

```bash
kubectl annotate namespace app-dns synology.proxy/auto-discovery=false
```

Resources in that namespace can still be managed individually with an explicit opt-in:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: dns-web
  namespace: app-dns
  annotations:
    synology.proxy/enabled: "true"   # opted in explicitly — rule is created
    synology.proxy/source-host: "dns.home.example.com"
```

### Decision order

For any resource the operator evaluates in this order:

1. `synology.proxy/enabled: "false"` on the resource → **skip** (always wins)
2. `synology.proxy/enabled: "true"` on the resource → **manage** (always wins)
3. Namespace matches `WATCH_NAMESPACE` glob **and** the namespace does not have `synology.proxy/auto-discovery: "false"` → **manage**
4. None of the above → **skip**

---

## Hostname derivation

When `spec.sourceHost` is empty the operator derives it automatically:

<p align="center">
    <img src="https://raw.githubusercontent.com/phoeluga/synology-proxy-operator/main/docs/images/chart_hostnameDerivation.png" alt="" width="70%" >
</p>

| Mode | Name used for derivation |
|---|---|
| Service / Ingress annotation | Service or Ingress name |
| ArgoCD Application | Application name |
| Manual `SynologyProxyRule` | Rule name, or `serviceRef`/`ingressRef` name |

---

## Certificate assignment

When `spec.assignCertificate: true` (the default), the operator assigns a TLS certificate to each DSM record after creation or update.

**Selection order:**
1. Find a DSM certificate whose CN or SAN matches the source hostname — wildcard patterns like `*.home.example.com` are supported
2. If no match is found, assign the DSM **default certificate** (`is_default: true`)

Certificate assignment is only called when the proxy record was just created or updated, not on every reconcile loop.

---

## Access control (ACL profiles)

Synology DSM supports Access Control Profiles that restrict which source IPs or networks can reach a reverse proxy rule. The operator integrates this at two levels:

- **Global default** — set `operator.defaultACLProfile` (Helm) or `DEFAULT_ACL_PROFILE` (env) to apply a profile to every rule that does not specify one
- **Per-rule** — set `synology.proxy/acl-profile` annotation on a Service, Ingress, or ArgoCD Application, or set `spec.aclProfile` on a `SynologyProxyRule` directly

Profile names are resolved to DSM UUIDs automatically and cached for 5 minutes to reduce API calls.

---

## Backend discovery

When `destinationHost` / `destinationPort` are not set:

<p align="center">
    <img src="https://raw.githubusercontent.com/phoeluga/synology-proxy-operator/main/docs/images/chart_backendDiscovery.png" alt="" width="70%" >
</p>

---

## Annotation reference

| Annotation | Applies to | Description | Default |
|---|---|---|---|
| `synology.proxy/enabled` | Service, Ingress, ArgoCD App | `"true"` opts in; `"false"` explicitly opts out (overrides `WATCH_NAMESPACE` glob) | — |
| `synology.proxy/auto-discovery` | **Namespace** | `"false"` disables glob-based auto-management for all resources in this namespace; explicit `synology.proxy/enabled: "true"` on individual resources still works | `"true"` |
| `synology.proxy/source-host` | Service, Ingress, ArgoCD App | Public FQDN override | derived from name + domain |
| `synology.proxy/acl-profile` | Service, Ingress, ArgoCD App | Synology ACL profile name | `DEFAULT_ACL_PROFILE` |
| `synology.proxy/destination-protocol` | Service, Ingress, ArgoCD App | Backend protocol: `http` or `https` | `http` |
| `synology.proxy/assign-certificate` | Service, Ingress, ArgoCD App | Set `"false"` to skip TLS cert assignment | `"true"` |
| `synology.proxy/service-ref` | ArgoCD App | `<namespace>/<name>` — Service for backend discovery | auto-scan |
| `synology.proxy/ingress-ref` | ArgoCD App | `<namespace>/<name>` — Ingress for backend discovery | auto-scan |
| `synology.proxy/destination-host` | ArgoCD App | Backend IP/hostname override | auto-discovered |
| `synology.proxy/destination-port` | ArgoCD App | Backend port override | auto-discovered |

---

## SynologyProxyRule CRD reference

```yaml
apiVersion: proxy.synology.io/v1alpha1
kind: SynologyProxyRule
metadata:
  name: myapp
  namespace: myapp
spec:
  # ── Frontend ───────────────────────────────────────────────────────────────
  sourceHost: myapp.home.example.com     # optional when DEFAULT_DOMAIN is set
  additionalSourceHosts:                 # each gets its own DSM record
    - myapp.example.org
  sourcePort: 443                        # default: 443

  # ── Backend ────────────────────────────────────────────────────────────────
  destinationHost: ""                    # auto-discovered when empty
  destinationPort: 0                     # auto-discovered when 0
  destinationProtocol: http              # http (default) | https

  # ── Backend auto-discovery ─────────────────────────────────────────────────
  serviceRef:
    name: myapp
    namespace: myapp                     # defaults to rule namespace when omitted
  ingressRef:
    name: myapp-ingress
    namespace: myapp

  # ── DSM settings ───────────────────────────────────────────────────────────
  aclProfile: "LAN Only"                 # DSM Access Control profile name
  assignCertificate: true                # auto-assign matching TLS certificate

  customHeaders:                         # defaults to WebSocket upgrade headers
    - name: Upgrade
      value: $http_upgrade
    - name: Connection
      value: $connection_upgrade

  timeouts:
    connect: 60                          # seconds
    read: 60
    send: 60

  # ── Internal (set automatically) ───────────────────────────────────────────
  description: ""                        # DSM record label — defaults to namespace/name
  managedByApp: ""                       # set by ArgoCD watcher, do not set manually
```

---

## Status and observability

```bash
kubectl get spr -A
```

```
NAMESPACE   NAME                     SOURCE HOST                DESTINATION     SYNCED   RECORDS   AGE
myapp       myapp--myapp             myapp.home.example.com     192.168.1.55    true     1         12m
nas         nas-photos               photos.home.example.com    192.168.1.100   true     1         3d
```

```bash
kubectl describe spr myapp -n myapp
```

| Status field | Description |
|---|---|
| `status.synced` | `true` when the last DSM sync succeeded |
| `status.managedRecords` | All DSM records owned by this rule (one per source hostname) |
| `status.managedRecords[].uuid` | DSM record UUID |
| `status.managedRecords[].sourceHost` | Frontend hostname for this record |
| `status.resolvedDestinationHost` | Backend IP/hostname that was discovered |
| `status.resolvedDestinationPort` | Backend port that was discovered |
| `status.lastSyncTime` | Timestamp of last successful sync |
| `status.conditions[Synced]` | Standard Kubernetes condition |
| `status.conditions[Ready]` | `true` when backend is discovered and rule is active |

**Force re-sync:**

```bash
kubectl annotate spr myapp -n myapp force-sync="$(date +%s)" --overwrite
```

---

## Helm values reference

| Value | Description | Default |
|---|---|---|
| `synology.url` | DSM base URL | required |
| `synology.username` | DSM username | required |
| `synology.password` | DSM password | required |
| `synology.skipTLSVerify` | Skip TLS certificate check | `false` |
| `synology.existingSecret` | Name of an existing Secret with DSM credentials | `""` |
| `operator.defaultDomain` | Domain suffix for auto-derived hostnames | `""` |
| `operator.defaultACLProfile` | ACL profile applied when none is specified per rule | `""` |
| `operator.enableArgoWatcher` | Enable ArgoCD Application watcher | `true` |
| `operator.watchNamespace` | Namespace glob (e.g. `app-*`) for annotation-free auto-management | `""` |
| `operator.ruleNamespace` | Namespace for auto-created `SynologyProxyRule` objects. Empty = source app namespace | `""` |
| `installCRDs` | Install CRDs via Helm | `true` |
| `leaderElection` | Enable leader election for HA deployments | `false` |

---

## Further reading

| Document | Description |
|---|---|
| [Architecture](docs/architecture.md) | Internal design, reconciler flow, project structure |

---

## License

Apache 2.0 — see [LICENSE](LICENSE).
