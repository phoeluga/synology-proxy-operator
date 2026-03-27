# Synology Proxy Operator - Deployment Guide

## Overview

This guide covers deploying the Synology Proxy Operator to your Kubernetes cluster.

## Prerequisites

- Kubernetes cluster (1.19+)
- kubectl configured
- Helm 3.x (for Helm installation)
- Synology NAS with DSM 7.x
- Admin credentials for Synology NAS

## Installation Methods

### Method 1: Helm Chart (Recommended)

#### Step 1: Create Namespace

```bash
kubectl create namespace synology-operator
```

#### Step 2: Create Credentials Secret

```bash
kubectl create secret generic synology-credentials \
  --namespace synology-operator \
  --from-literal=username=admin \
  --from-literal=password=your-synology-password
```

#### Step 3: Install with Helm

```bash
helm install synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --set synology.url=https://nas.example.com:5001 \
  --set operator.watchNamespaces="production,staging"
```

#### Step 4: Verify Installation

```bash
# Check operator pod
kubectl get pods -n synology-operator

# Check operator logs
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator -f
```

### Method 2: kubectl (Manual)

#### Step 1: Apply RBAC

```bash
kubectl apply -f config/rbac/service_account.yaml
kubectl apply -f config/rbac/role.yaml
kubectl apply -f config/rbac/role_binding.yaml
```

#### Step 2: Create Secret

```bash
kubectl create secret generic synology-credentials \
  --namespace synology-operator \
  --from-literal=username=admin \
  --from-literal=password=your-password
```

#### Step 3: Apply Deployment

```bash
kubectl apply -f config/manager/manager.yaml
kubectl apply -f config/manager/service.yaml
```

## Configuration

### Helm Values

Key configuration options in `values.yaml`:

```yaml
synology:
  url: "https://nas.example.com:5001"  # Required
  secretName: synology-credentials
  tlsVerify: true
  defaultACLProfile: ""

operator:
  watchNamespaces: "*"  # or "prod,staging"
  logLevel: info

resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "500m"
```

### Environment Variables

Alternatively, configure via environment variables:

- `SYNOLOGY_URL` - Synology NAS URL
- `SECRET_NAME` - Credentials secret name
- `SECRET_NAMESPACE` - Credentials secret namespace
- `WATCH_NAMESPACES` - Namespaces to watch
- `LOG_LEVEL` - Logging level

## Usage Examples

### Basic Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: grafana
  namespace: production
  annotations:
    synology.io/enabled: "true"
spec:
  rules:
  - host: grafana.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: grafana
            port:
              number: 3000
```

### With ACL Override

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: internal-app
  namespace: production
  annotations:
    synology.io/enabled: "true"
    synology.io/acl-profile: "InternalOnly"
spec:
  rules:
  - host: internal.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: internal-app
            port:
              number: 8080
```

### With Deletion Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: legacy-app
  namespace: production
  annotations:
    synology.io/enabled: "true"
    synology.io/deletion-policy: "retain"
spec:
  rules:
  - host: legacy.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: legacy-app
            port:
              number: 80
```

## Local Testing

### Using Minikube

```bash
# Start Minikube
minikube start

# Build and load image
docker build -t synology-proxy-operator:dev .
minikube image load synology-proxy-operator:dev

# Install operator
helm install synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --create-namespace \
  --set image.repository=synology-proxy-operator \
  --set image.tag=dev \
  --set image.pullPolicy=Never \
  --set synology.url=https://your-nas:5001

# Test with example Ingress
kubectl apply -f examples/basic-ingress.yaml
```

### Using Kind

```bash
# Create Kind cluster
kind create cluster --name synology-test

# Build and load image
docker build -t synology-proxy-operator:dev .
kind load docker-image synology-proxy-operator:dev --name synology-test

# Install operator
helm install synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --create-namespace \
  --set image.repository=synology-proxy-operator \
  --set image.tag=dev \
  --set image.pullPolicy=Never \
  --set synology.url=https://your-nas:5001
```

## Monitoring

### Prometheus Metrics

Metrics are exposed at `:8080/metrics`:

```bash
# Port forward to access metrics
kubectl port-forward -n synology-operator svc/synology-operator-metrics 8080:8080

# View metrics
curl http://localhost:8080/metrics
```

### Key Metrics

- `synology_operator_reconcile_total` - Total reconciliations
- `synology_operator_reconcile_duration_seconds` - Reconciliation latency
- `synology_api_requests_total` - API request count
- `synology_certificate_cache_hits_total` - Cache hit rate

### Grafana Dashboard

Import the example dashboard from `docs/grafana-dashboard.json`

## Troubleshooting

### Operator Not Starting

```bash
# Check pod status
kubectl get pods -n synology-operator

# Check events
kubectl get events -n synology-operator --sort-by='.lastTimestamp'

# Check logs
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator
```

### Ingress Not Reconciling

1. Check annotation is present: `synology.io/enabled: "true"`
2. Check namespace is being watched
3. Check operator logs for errors
4. Verify RBAC permissions

### Synology API Errors

1. Verify credentials in Secret
2. Check Synology URL is accessible
3. Check TLS verification settings
4. Review operator logs for API errors

### Certificate Not Assigned

1. Verify certificate exists in Synology
2. Check hostname matches certificate CN or SAN
3. Check certificate cache (may take up to 5 minutes)
4. Review operator logs for matching errors

## Upgrading

### Helm Upgrade

```bash
helm upgrade synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --reuse-values
```

### kubectl Upgrade

```bash
kubectl apply -f config/manager/manager.yaml
kubectl rollout restart deployment/synology-proxy-operator -n synology-operator
```

## Uninstalling

### Helm Uninstall

```bash
helm uninstall synology-operator --namespace synology-operator
kubectl delete namespace synology-operator
```

### kubectl Uninstall

```bash
kubectl delete -f config/manager/
kubectl delete -f config/rbac/
kubectl delete namespace synology-operator
```

## Security Considerations

1. **Credentials**: Store Synology credentials in Kubernetes Secrets
2. **TLS**: Always use HTTPS for Synology URL
3. **RBAC**: Operator uses least-privilege permissions
4. **Network**: Ensure operator can reach Synology NAS
5. **Secrets**: Consider using sealed-secrets or external-secrets

## Production Checklist

- [ ] Synology credentials stored securely
- [ ] TLS verification enabled
- [ ] Resource limits configured
- [ ] Namespace filtering configured
- [ ] Monitoring and alerting set up
- [ ] Backup and disaster recovery plan
- [ ] Documentation updated
- [ ] Team trained on operator usage

## Support

- GitHub Issues: https://github.com/phoeluga/synology-proxy-operator/issues
- Documentation: https://github.com/phoeluga/synology-proxy-operator/docs
- Examples: https://github.com/phoeluga/synology-proxy-operator/examples
