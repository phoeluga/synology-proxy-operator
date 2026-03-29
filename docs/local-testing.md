# Local Testing Guide

This guide walks through running the Synology Proxy Operator locally against a Kind cluster, from cluster creation to verifying that proxy records are created in (or simulated against) your Synology DSM.

---

## 1. Prerequisites

Install the following tools:

```bash
# Kind — local Kubernetes clusters in Docker
brew install kind          # macOS
# or: curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.23.0/kind-linux-amd64 && chmod +x kind && mv kind /usr/local/bin/

# kubectl
brew install kubectl

# Helm
brew install helm

# Go 1.22+
brew install go

# golangci-lint (optional, for linting)
brew install golangci-lint
```

---

## 2. Create a Kind cluster

```bash
# Create a cluster with one control plane node
kind create cluster --name synology-test

# Verify
kubectl cluster-info --context kind-synology-test
kubectl get nodes
```

---

## 3. Install ArgoCD (optional — only needed for the ArgoCD watcher)

```bash
kubectl create namespace argocd
kubectl apply -n argocd \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Wait for ArgoCD to be ready
kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=120s
```

---

## 4. Build the operator image and load it into Kind

```bash
# From the project root
make docker-build IMG=synology-proxy-operator:dev

# Load into Kind (no registry push needed for local testing)
kind load docker-image synology-proxy-operator:dev --name synology-test
```

---

## 5. Install the CRD

```bash
kubectl apply -f config/crd/bases/proxy.synology.io_synologyproxyrules.yaml

# Verify
kubectl get crd synologyproxyrules.proxy.synology.io
```

---

## 6. Option A — Run the operator outside the cluster (easiest for development)

This is the fastest feedback loop: you run the binary on your laptop and it
talks to both the Kind cluster and your real (or mock) Synology DSM.

```bash
export SYNOLOGY_URL="https://your-diskstation:5001"
export SYNOLOGY_USER="admin"
export SYNOLOGY_PASSWORD="secret"
export SYNOLOGY_SKIP_TLS_VERIFY="true"    # if DSM uses self-signed cert
export DEFAULT_DOMAIN="home.example.com"

make run
```

The operator will use your current `~/.kube/config` context
(`kind-synology-test`).

---

## 6. Option B — Deploy the operator inside Kind via Helm

```bash
# Install RBAC + operator
helm upgrade --install synology-proxy-operator \
  helm/synology-proxy-operator \
  --namespace synology-proxy-operator \
  --create-namespace \
  --set image.repository=synology-proxy-operator \
  --set image.tag=dev \
  --set image.pullPolicy=Never \
  --set synology.url="https://your-diskstation:5001" \
  --set synology.username="admin" \
  --set synology.password="secret" \
  --set synology.skipTLSVerify=true \
  --set operator.defaultDomain="home.example.com"

# Watch operator logs
kubectl logs -n synology-proxy-operator -l app.kubernetes.io/name=synology-proxy-operator -f
```

---

## 7. Create a test LoadBalancer service

Kind does not provide an external IP by default.  Install MetalLB or use a
simple workaround: patch the Service manually to simulate an external IP.

### Option A — MetalLB (realistic)

```bash
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.14.5/config/manifests/metallb-native.yaml
kubectl wait --for=condition=available deployment/controller -n metallb-system --timeout=90s

# Get the Docker bridge subnet Kind uses
SUBNET=$(docker network inspect kind | jq -r '.[0].IPAM.Config[0].Subnet')
# Typically 172.18.0.0/16 — pick a small range from within it
cat <<EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kind-pool
  namespace: metallb-system
spec:
  addresses:
    - 172.18.255.200-172.18.255.250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: kind-l2
  namespace: metallb-system
EOF
```

### Option B — Manual patch (quick hack)

```bash
kubectl patch svc <service-name> -n <namespace> \
  --type='json' \
  -p='[{"op":"add","path":"/status/loadBalancer/ingress","value":[{"ip":"1.2.3.4"}]}]'
```

---

## 8. Test scenario A — SynologyProxyRule directly

```bash
cat <<EOF | kubectl apply -f -
apiVersion: proxy.synology.io/v1alpha1
kind: SynologyProxyRule
metadata:
  name: test-app
  namespace: synology-proxy-operator
spec:
  sourceHost: test-app.home.example.com
  destinationHost: 192.168.1.100   # use your real server or the MetalLB IP
  destinationPort: 8080
  aclProfile: ""
  assignCertificate: true
EOF

# Watch status
kubectl get spr -n synology-proxy-operator -w

# Describe for conditions / events
kubectl describe spr test-app -n synology-proxy-operator
```

Expected: `SYNCED = true` and `UUID` populated once the DSM API call succeeds.

---

## 9. Test scenario B — ArgoCD Application → auto-create SynologyProxyRule

```bash
# Deploy a sample app
kubectl create namespace demo
kubectl create deployment demo --image=nginx --namespace=demo
kubectl expose deployment demo --type=LoadBalancer --port=80 -n demo

# Simulate ArgoCD Application (minimal)
cat <<EOF | kubectl apply -f -
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: demo
  namespace: argocd
  annotations:
    synology.proxy/enabled: "true"
    synology.proxy/source-host: "demo.home.example.com"
    synology.proxy/service-ref: "demo/demo"
spec:
  project: default
  destination:
    namespace: demo
    server: https://kubernetes.default.svc
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps
    targetRevision: HEAD
    path: guestbook
EOF

# The ArgoCD watcher creates a SynologyProxyRule automatically
kubectl get spr -n synology-proxy-operator

# Check it synced with DSM
kubectl describe spr demo -n synology-proxy-operator
```

---

## 10. Test scenario C — Simulate deletion

```bash
# Delete the ArgoCD Application
kubectl delete application demo -n argocd
# → The owned SynologyProxyRule is garbage-collected
# → The finalizer removes the DSM record before deletion completes

kubectl get spr -n synology-proxy-operator   # should be gone within seconds
```

---

## 11. Verify DSM records

Log in to your Synology DSM and navigate to:

**Control Panel → Application Portal → Reverse Proxy**

You should see entries matching the `sourceHost` values, with the backend IP
and certificate assigned as configured.

Alternatively, query the DSM API directly:

```bash
# Get all proxy records
curl -sk "https://your-diskstation:5001/webapi/entry.cgi/SYNO.Core.AppPortal.ReverseProxy" \
  -d "api=SYNO.Core.AppPortal.ReverseProxy&method=list&version=1&_sid=YOUR_SID" | jq .
```

---

## 12. Debugging tips

```bash
# Operator logs (verbose)
kubectl logs -n synology-proxy-operator deploy/synology-proxy-operator --follow

# Events on a rule
kubectl describe spr <name> -n synology-proxy-operator

# Force re-reconcile (touch the resource)
kubectl annotate spr <name> -n synology-proxy-operator force-sync="$(date +%s)" --overwrite

# Check RBAC (run-as)
kubectl auth can-i list applications --as=system:serviceaccount:synology-proxy-operator:synology-proxy-operator -n argocd
kubectl auth can-i list services --as=system:serviceaccount:synology-proxy-operator:synology-proxy-operator --all-namespaces
```

---

## 13. Tear down

```bash
# Remove resources
kubectl delete spr --all -n synology-proxy-operator
helm uninstall synology-proxy-operator -n synology-proxy-operator
kubectl delete -f config/crd/bases/

# Delete the cluster
kind delete cluster --name synology-test
```
