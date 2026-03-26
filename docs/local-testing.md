# Local testing guide

This guide walks through running and testing the operator locally using
[kind](https://kind.sigs.k8s.io/) (Kubernetes in Docker).

---

## Prerequisites

- Docker
- `kubectl`
- `kind` — `brew install kind` or see https://kind.sigs.k8s.io/docs/user/quick-start/
- Go 1.22+
- Access to a real Synology NAS **or** a mock HTTP server (see the mock section below)

---

## 1. Create a local kind cluster

```bash
kind create cluster --name synology-test
kubectl cluster-info --context kind-synology-test
```

---

## 2. Build the operator image and load it into kind

```bash
# From the synology-proxy-operator/ directory
make docker-build IMG=synology-proxy-operator:dev

# Load the image directly into the kind node (no registry needed)
kind load docker-image synology-proxy-operator:dev --name synology-test
```

---

## 3. Install the CRD

```bash
kubectl apply -f config/crd/synologyreverseproxy.yaml

# Verify
kubectl get crd synologyreverseproxies.proxy.hnet.io
```

---

## 4. Apply RBAC

```bash
kubectl create namespace synology-proxy-operator
kubectl apply -f config/rbac/role.yaml
kubectl apply -f config/rbac/rolebinding.yaml
```

---

## 5. Create the credentials Secret

Replace the values with your actual Synology NAS details.
Set `skipTLSVerify: "true"` if your NAS uses a self-signed certificate.

```bash
kubectl create secret generic synology-credentials \
  --namespace synology-proxy-operator \
  --from-literal=url=https://nas.hnet.io:5001 \
  --from-literal=username=admin \
  --from-literal=password=supersecret \
  --from-literal=skipTLSVerify=true
```

---

## 6. Deploy the operator

```bash
# Substitute the image tag
sed 's|\${IMG}|synology-proxy-operator:dev|g' config/manager/deployment.yaml \
  | kubectl apply -f -

# Watch it come up
kubectl -n synology-proxy-operator get pods -w
```

---

## 7. Run the operator locally (without Docker)

For rapid iteration you can run the operator binary directly against the kind cluster:

```bash
# Ensure your kubeconfig points at the kind cluster
kubectl config use-context kind-synology-test

# Export the namespace so the operator finds the credentials secret
export POD_NAMESPACE=synology-proxy-operator

# Run
make run
```

The operator will log to stdout. Press Ctrl-C to stop.

---

## 8. Apply a sample SynologyReverseProxy CR

```yaml
# sample-srp.yaml
apiVersion: proxy.hnet.io/v1alpha1
kind: SynologyReverseProxy
metadata:
  name: test-app
  namespace: default
spec:
  description: "test-app"
  sourceHostname: "test.hnet.io"
  sourcePort: 443
  sourceProtocol: https
  destHostname: "10.1.4.200"
  destPort: 8080
  destProtocol: http
  assignCertificate: true
```

```bash
kubectl apply -f sample-srp.yaml
```

---

## 9. Verify operator logs and Synology API calls

```bash
# Operator logs (if running in-cluster)
kubectl -n synology-proxy-operator logs -l app=synology-proxy-operator -f

# Check the CR status
kubectl get srp test-app -o yaml
```

Expected status after a successful reconcile:

```yaml
status:
  uuid: "some-uuid-from-synology"
  certId: "cert-id-if-assigned"
  conditions:
    - type: Ready
      status: "True"
      reason: Reconciled
    - type: Synced
      status: "True"
      reason: Synced
```

---

## 10. Test the update flow

Edit the CR to change the destination port:

```bash
kubectl patch srp test-app --type=merge -p '{"spec":{"destPort":9090}}'
```

The operator will detect the change and call the Synology `set` method to update the record.
Check the logs for `"reverse proxy record upserted"` and verify the UUID remains the same.

---

## 11. Test the delete flow

```bash
kubectl delete srp test-app
```

The operator will:
1. Detect the `DeletionTimestamp` on the CR
2. Call the Synology API to delete the reverse proxy record
3. Remove the `proxy.hnet.io/finalizer` finalizer
4. Allow Kubernetes to garbage-collect the CR

Watch the logs for `"reverse proxy record deleted from Synology"`.

---

## 12. Using a mock Synology API server

If you don't have a Synology NAS available, you can use a simple mock server.
Save the following as `mock-server.py` and run it with `python3 mock-server.py`:

```python
#!/usr/bin/env python3
"""Minimal mock of the Synology WebAPI for local operator testing."""
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
import json, uuid

records = {}

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length).decode()
        params = parse_qs(body)
        api = params.get("api", [""])[0]
        method = params.get("method", [""])[0]

        if api == "SYNO.API.Auth":
            resp = {"success": True, "data": {"sid": "mock-sid", "synotoken": "mock-token"}}
        elif api == "SYNO.Core.ReverseProxy.Rule":
            if method == "list":
                resp = {"success": True, "data": {"list": list(records.values())}}
            elif method == "create":
                rid = str(uuid.uuid4())
                records[rid] = {"id": rid, "description": params["description"][0]}
                resp = {"success": True, "data": {"id": rid}}
            elif method == "set":
                rid = params["id"][0]
                if rid in records:
                    records[rid].update({"description": params.get("description", [records[rid]["description"]])[0]})
                resp = {"success": True, "data": {}}
            elif method == "delete":
                records.pop(params.get("id", [""])[0], None)
                resp = {"success": True, "data": {}}
            else:
                resp = {"success": False, "error": {"code": 102}}
        elif api == "SYNO.Core.Certificate":
            resp = {"success": True, "data": {"certificates": []}}
        elif api == "SYNO.Core.ReverseProxy.ACL":
            resp = {"success": True, "data": {"list": []}}
        else:
            resp = {"success": False, "error": {"code": 102}}

        body = json.dumps(resp).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        print(f"[mock] {fmt % args}")

HTTPServer(("0.0.0.0", 5001), Handler).serve_forever()
```

Point the credentials secret at `http://host.docker.internal:5001` (or your host IP from
inside kind) and set `skipTLSVerify: "true"`.

---

## Cleanup

```bash
kubectl delete -f sample-srp.yaml
kind delete cluster --name synology-test
```
