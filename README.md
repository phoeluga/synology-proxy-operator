# Synology Proxy Operator

A Kubernetes operator that automates Synology reverse proxy management from Ingress resources.

## Overview

The Synology Proxy Operator watches Kubernetes Ingress resources and automatically creates, updates, and deletes reverse proxy records on your Synology NAS. It also handles SSL certificate assignment with wildcard matching support.

### Key Features

- 🔄 **Automatic Proxy Management**: Create proxy records from Ingress annotations
- 🔒 **Certificate Assignment**: Automatic wildcard certificate matching and assignment
- 🎯 **Namespace Filtering**: Watch specific namespaces with wildcard patterns
- ♻️ **Deletion Policies**: Configurable retain/delete behavior
- 🔑 **Credential Management**: Hot-reload credentials from Kubernetes Secrets
- 🛡️ **Error Handling**: Comprehensive retry logic, circuit breaker, rate limiting
- 📊 **Observability**: Prometheus metrics, structured logging, Kubernetes Events
- 🔐 **Security**: RBAC, sensitive data filtering, TLS verification

## Quick Start

### Prerequisites

- Kubernetes cluster (1.25+)
- Synology NAS with Reverse Proxy package installed
- kubectl configured
- Docker (for building images)

### Installation

1. **Create namespace**:
```bash
kubectl create namespace synology-proxy-operator
```

2. **Create credentials secret**:
```bash
kubectl create secret generic synology-credentials \
  --from-literal=username=<your-synology-username> \
  --from-literal=password=<your-synology-password> \
  -n synology-proxy-operator
```

3. **Deploy operator**:
```bash
# Apply RBAC
kubectl apply -f config/rbac/

# Apply operator deployment
kubectl apply -f config/manager/
```

4. **Configure operator** (edit `config/manager/manager.yaml`):
```yaml
env:
  - name: SYNOLOGY_URL
    value: "https://your-nas.example.com:5001"
  - name: WATCH_NAMESPACES
    value: "*"  # or "production,staging"
```

5. **Verify deployment**:
```bash
kubectl get pods -n synology-proxy-operator
kubectl logs -n synology-proxy-operator -l app=synology-proxy-operator
```

## Usage

### Basic Ingress Example

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  namespace: production
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
              number: 8080
```

### With ACL Override

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: internal-app
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

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `SYNOLOGY_URL` | Synology NAS URL (HTTPS) | - | Yes |
| `WATCH_NAMESPACES` | Namespace patterns to watch | `*` | No |
| `DEFAULT_ACL_PROFILE` | Default ACL profile name | `default` | No |
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` | No |
| `LOG_FORMAT` | Log format (json/text) | `json` | No |
| `TLS_VERIFY` | Verify TLS certificates | `true` | No |
| `MAX_RETRIES` | Max API retry attempts | `10` | No |

### Annotations

| Annotation | Description | Default |
|------------|-------------|---------|
| `synology.io/enabled` | Enable operator for this Ingress | `false` |
| `synology.io/acl-profile` | Override ACL profile | Operator default |
| `synology.io/deletion-policy` | Deletion policy (delete/retain) | `delete` |
| `synology.io/backend-protocol` | Backend protocol (http/https) | `http` |

## Development

### Building

```bash
# Build binary
make build

# Build Docker image
make docker-build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Run linter
make lint
```

### Running Locally

```bash
# Run operator locally (requires kubeconfig)
make run
```

### Testing

```bash
# Run all tests
go test -v ./...

# Run specific package tests
go test -v ./pkg/synology/...

# Run with race detection
go test -v -race ./...

# Generate coverage report
make test-coverage
```

## Architecture

### Components

- **ConfigManager**: Configuration loading and validation
- **Logger**: Structured logging with sensitive data filtering
- **SynologyClient**: Complete Synology API integration
- **IngressReconciler**: Kubernetes controller for Ingress resources
- **CertificateMatcher**: Certificate matching service (exact + wildcard)
- **MetricsRegistry**: Prometheus metrics collection

### Technology Stack

- **Framework**: Kubebuilder 3.x
- **Language**: Go 1.21+
- **Configuration**: Cobra + Viper
- **Logging**: Zap (structured JSON)
- **HTTP Client**: net/http (standard library)
- **Rate Limiting**: golang.org/x/time/rate
- **Kubernetes**: controller-runtime, client-go

## Monitoring

### Metrics

The operator exposes Prometheus metrics at `:8080/metrics`:

- `synology_operator_reconcile_total` - Total reconciliations
- `synology_operator_reconcile_duration_seconds` - Reconciliation duration
- `synology_api_requests_total` - API requests
- `synology_api_errors_total` - API errors
- `synology_certificate_cache_hits_total` - Cache hits
- `synology_certificate_matches_total` - Certificate matches

### Health Checks

- **Liveness**: `http://localhost:8081/healthz`
- **Readiness**: `http://localhost:8081/readyz`

### Logs

```bash
# View logs
kubectl logs -f -n synology-proxy-operator -l app=synology-proxy-operator

# View logs with JSON parsing
kubectl logs -n synology-proxy-operator -l app=synology-proxy-operator | jq .
```

## Troubleshooting

### Operator Not Starting

```bash
# Check pod status
kubectl get pods -n synology-proxy-operator

# Check pod events
kubectl describe pod -n synology-proxy-operator <pod-name>

# Check logs
kubectl logs -n synology-proxy-operator <pod-name>
```

### Ingress Not Reconciling

1. Check annotation is present: `synology.io/enabled: "true"`
2. Check namespace is in watch filter
3. Check operator logs for errors
4. Verify Synology API connectivity

### Certificate Not Assigned

1. Check certificate exists in Synology
2. Verify hostname matches certificate CN or SAN
3. Check operator logs for matching details
4. Verify certificate cache is working

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make verify`
6. Submit a pull request

## License

[Your License Here]

## Support

For issues and questions:
- GitHub Issues: [Your Repo URL]
- Documentation: `docs/` directory

## Acknowledgments

Built using the AI-Driven Development Life Cycle (AI-DLC) workflow.
