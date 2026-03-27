# Local Testing Guide

This guide explains how to test the Synology Proxy Operator locally using Minikube or Kind.

## Prerequisites

- Docker installed and running (or Podman as an alternative)
- kubectl installed
- One of the following:
  - Minikube (recommended for beginners)
  - Kind (Kubernetes in Docker)
- Helm 3.x installed
- Access to a Synology NAS (or mock server)

## Option 1: Testing with Minikube

### 1. Start Minikube

```bash
# Start Minikube with sufficient resources
minikube start --cpus=2 --memory=4096

# Enable metrics-server (optional, for monitoring)
minikube addons enable metrics-server

# Verify cluster is running
kubectl cluster-info
```

### 2. Build and Load Image

```bash
# Build the operator image (works with Docker or Podman)
make docker-build IMG=synology-proxy-operator:local

# Load image into Minikube
minikube image load synology-proxy-operator:local

# Verify image is loaded
minikube image ls | grep synology
```

**Note**: The Makefile automatically detects whether you have Podman or Docker installed and uses the appropriate tool.

### 3. Deploy the Operator

```bash
# Create namespace
kubectl create namespace synology-operator

# Create credentials secret
kubectl create secret generic synology-credentials \
  --namespace synology-operator \
  --from-literal=username=admin \
  --from-literal=password=your-password

# Install with Helm (using local image)
helm install synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --set image.repository=synology-proxy-operator \
  --set image.tag=local \
  --set image.pullPolicy=Never \
  --set synology.url=https://nas.example.com:5001

# Verify deployment
kubectl get pods -n synology-operator
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator
```

### 4. Test with Sample Ingress

```bash
# Create a test namespace
kubectl create namespace test-app

# Deploy a simple test service
kubectl apply -n test-app -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  selector:
    app: test
  ports:
  - port: 80
    targetPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
EOF

# Create an Ingress
kubectl apply -n test-app -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
  annotations:
    synology.io/enabled: "true"
spec:
  rules:
  - host: test.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: test-service
            port:
              number: 80
EOF

# Check operator logs
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator -f
```

### 5. Verify Metrics

```bash
# Port-forward to metrics endpoint
kubectl port-forward -n synology-operator svc/synology-proxy-operator-metrics 8080:8080

# In another terminal, query metrics
curl http://localhost:8080/metrics | grep synology_
```

### 6. Cleanup

```bash
# Delete test resources
kubectl delete namespace test-app

# Uninstall operator
helm uninstall synology-operator -n synology-operator

# Delete namespace
kubectl delete namespace synology-operator

# Stop Minikube
minikube stop
```

## Option 2: Testing with Kind

### 1. Create Kind Cluster

```bash
# Create a cluster configuration
cat > kind-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 8080
    protocol: TCP
EOF

# Create cluster
kind create cluster --name synology-test --config kind-config.yaml

# Verify cluster
kubectl cluster-info --context kind-synology-test
```

### 2. Build and Load Image

```bash
# Build the operator image
make docker-build IMG=synology-proxy-operator:local

# Load image into Kind
kind load docker-image synology-proxy-operator:local --name synology-test

# Verify image is loaded
docker exec -it synology-test-control-plane crictl images | grep synology
```

### 3. Deploy and Test

Follow the same deployment and testing steps as Minikube (steps 3-5 above).

### 4. Cleanup

```bash
# Delete the cluster
kind delete cluster --name synology-test
```

## Option 3: Testing with Mock Synology Server

If you don't have access to a real Synology NAS, you can use a mock server for testing.

### 1. Create Mock Server

```bash
# Create mock server deployment
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: mock-synology-responses
  namespace: synology-operator
data:
  responses.json: |
    {
      "auth_login": {"success": true, "data": {"sid": "mock-session-id"}},
      "auth_logout": {"success": true},
      "proxy_list": {"success": true, "data": {"records": []}},
      "proxy_create": {"success": true},
      "proxy_update": {"success": true},
      "proxy_delete": {"success": true},
      "cert_list": {"success": true, "data": {"certificates": [
        {"id": "cert1", "desc": "*.example.com", "is_default": false}
      ]}},
      "cert_set": {"success": true}
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mock-synology
  namespace: synology-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mock-synology
  template:
    metadata:
      labels:
        app: mock-synology
    spec:
      containers:
      - name: mock-server
        image: mockserver/mockserver:latest
        ports:
        - containerPort: 1080
        env:
        - name: MOCKSERVER_INITIALIZATION_JSON_PATH
          value: /config/responses.json
        volumeMounts:
        - name: config
          mountPath: /config
      volumes:
      - name: config
        configMap:
          name: mock-synology-responses
---
apiVersion: v1
kind: Service
metadata:
  name: mock-synology
  namespace: synology-operator
spec:
  selector:
    app: mock-synology
  ports:
  - port: 5001
    targetPort: 1080
EOF

# Update operator to use mock server
helm upgrade synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --reuse-values \
  --set synology.url=http://mock-synology:5001
```

### 2. Test with Mock Server

Now you can create Ingress resources and the operator will interact with the mock server instead of a real NAS.

## Debugging Tips

### View Operator Logs

```bash
# Follow logs in real-time
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator -f

# View logs with timestamps
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator --timestamps

# View previous container logs (if crashed)
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator --previous
```

### Check Operator Status

```bash
# Check pod status
kubectl get pods -n synology-operator

# Describe pod for events
kubectl describe pod -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator

# Check resource usage
kubectl top pod -n synology-operator
```

### Inspect Ingress Status

```bash
# Check Ingress status
kubectl get ingress -A

# Describe Ingress for events
kubectl describe ingress <ingress-name> -n <namespace>

# Check Ingress annotations
kubectl get ingress <ingress-name> -n <namespace> -o yaml
```

### Test Health Endpoints

```bash
# Port-forward to health endpoint
kubectl port-forward -n synology-operator svc/synology-proxy-operator-metrics 8081:8081

# Check liveness
curl http://localhost:8081/healthz

# Check readiness
curl http://localhost:8081/readyz
```

### Enable Debug Logging

```bash
# Update operator with debug logging
helm upgrade synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --reuse-values \
  --set logging.level=debug

# Restart operator to apply changes
kubectl rollout restart deployment -n synology-operator synology-proxy-operator
```

### Test Credential Reload

```bash
# Update credentials secret
kubectl create secret generic synology-credentials \
  --namespace synology-operator \
  --from-literal=username=newuser \
  --from-literal=password=newpassword \
  --dry-run=client -o yaml | kubectl apply -f -

# Watch operator logs for reload message
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator -f | grep "credentials reloaded"
```

## Common Issues

### Image Pull Errors

If you see `ImagePullBackOff` errors:

```bash
# Verify image is loaded
minikube image ls | grep synology  # For Minikube
# OR
docker exec -it <kind-node> crictl images | grep synology  # For Kind

# Ensure pullPolicy is set correctly
helm upgrade synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --reuse-values \
  --set image.pullPolicy=Never
```

### Connection Refused Errors

If operator can't connect to Synology NAS:

```bash
# Test connectivity from operator pod
kubectl exec -n synology-operator -it <pod-name> -- wget -O- https://nas.example.com:5001

# Check if URL is correct
kubectl get deployment -n synology-operator synology-proxy-operator -o yaml | grep SYNOLOGY_URL
```

### RBAC Permission Errors

If you see permission denied errors:

```bash
# Verify ClusterRole is created
kubectl get clusterrole synology-proxy-operator

# Verify ClusterRoleBinding
kubectl get clusterrolebinding synology-proxy-operator

# Check ServiceAccount
kubectl get serviceaccount -n synology-operator synology-proxy-operator
```

### Operator Not Reconciling

If Ingress resources aren't being processed:

```bash
# Check if operator is watching correct namespaces
kubectl get deployment -n synology-operator synology-proxy-operator -o yaml | grep WATCH_NAMESPACES

# Verify Ingress has correct annotation
kubectl get ingress <name> -n <namespace> -o jsonpath='{.metadata.annotations.synology\.io/enabled}'

# Check operator logs for errors
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator | grep ERROR
```

## Performance Testing

### Load Testing with Multiple Ingresses

```bash
# Create multiple test Ingresses
for i in {1..10}; do
  kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress-$i
  namespace: test-app
  annotations:
    synology.io/enabled: "true"
spec:
  rules:
  - host: test$i.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: test-service
            port:
              number: 80
EOF
done

# Monitor reconciliation performance
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator -f | grep "reconciliation_duration"
```

### Monitor Resource Usage

```bash
# Watch resource usage
watch kubectl top pod -n synology-operator

# Get detailed metrics
kubectl get --raw /apis/metrics.k8s.io/v1beta1/namespaces/synology-operator/pods
```

## Next Steps

After successful local testing:

1. Review operator logs for any warnings or errors
2. Test all supported Ingress configurations (see examples/)
3. Verify metrics are being exported correctly
4. Test failure scenarios (NAS unreachable, invalid credentials, etc.)
5. Proceed to staging/production deployment (see DEPLOYMENT-GUIDE.md)

## Additional Resources

- [Minikube Documentation](https://minikube.sigs.k8s.io/docs/)
- [Kind Documentation](https://kind.sigs.k8s.io/)
- [Kubernetes Ingress Documentation](https://kubernetes.io/docs/concepts/services-networking/ingress/)
- [Helm Documentation](https://helm.sh/docs/)
- [Operator Troubleshooting Guide](./TROUBLESHOOTING.md)
