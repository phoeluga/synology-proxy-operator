# Synology Proxy Operator Helm Chart

Kubernetes operator that automates Synology reverse proxy management from Ingress resources.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.x
- Synology NAS with DSM 7.x
- Admin credentials for Synology NAS

## Installation

### Add Helm Repository (if published)

```bash
helm repo add synology-operator https://phoeluga.github.io/synology-proxy-operator
helm repo update
```

### Install from Local Chart

```bash
# Create namespace
kubectl create namespace synology-operator

# Create credentials secret
kubectl create secret generic synology-credentials \
  --namespace synology-operator \
  --from-literal=username=admin \
  --from-literal=password=your-password

# Install chart
helm install synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --set synology.url=https://nas.example.com:5001
```

## Configuration

### Required Values

| Parameter | Description | Example |
|-----------|-------------|---------|
| `synology.url` | Synology NAS URL | `https://nas.example.com:5001` |

### Common Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `synology.secretName` | Credentials secret name | `synology-credentials` |
| `synology.tlsVerify` | Enable TLS verification | `true` |
| `operator.watchNamespaces` | Namespaces to watch | `*` (all) |
| `operator.logLevel` | Log level | `info` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `resources.limits.memory` | Memory limit | `256Mi` |

### All Values

See [values.yaml](values.yaml) for complete configuration options.

## Usage

### Basic Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    synology.io/enabled: "true"
spec:
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: my-app
            port:
              number: 80
```

### With ACL Profile

```yaml
metadata:
  annotations:
    synology.io/enabled: "true"
    synology.io/acl-profile: "InternalOnly"
```

### With Deletion Policy

```yaml
metadata:
  annotations:
    synology.io/enabled: "true"
    synology.io/deletion-policy: "retain"
```

## Monitoring

Metrics are exposed at `:8080/metrics` and can be scraped by Prometheus:

```yaml
apiVersion: v1
kind: ServiceMonitor
metadata:
  name: synology-operator
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: synology-proxy-operator
  endpoints:
  - port: metrics
    interval: 30s
```

## Upgrading

```bash
helm upgrade synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --reuse-values
```

## Uninstalling

```bash
helm uninstall synology-operator --namespace synology-operator
```

## Troubleshooting

### Check Operator Status

```bash
kubectl get pods -n synology-operator
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator
```

### Check Ingress Status

```bash
kubectl get ingress -A
kubectl describe ingress <name> -n <namespace>
```

### Common Issues

1. **Operator not starting**: Check credentials secret exists
2. **Ingress not reconciling**: Verify annotation `synology.io/enabled: "true"`
3. **Certificate not assigned**: Check certificate exists in Synology

## Support

- GitHub: https://github.com/phoeluga/synology-proxy-operator
- Issues: https://github.com/phoeluga/synology-proxy-operator/issues
- Documentation: https://github.com/phoeluga/synology-proxy-operator/docs
