# Configuration Guide

This document describes all configuration options for the Synology Proxy Operator.

## Table of Contents

- [Configuration Methods](#configuration-methods)
- [Synology Configuration](#synology-configuration)
- [Controller Configuration](#controller-configuration)
- [Logging Configuration](#logging-configuration)
- [Secret Format](#secret-format)
- [Examples](#examples)

---

## Configuration Methods

The operator supports three configuration methods with the following precedence:

1. **CLI Flags** (highest priority)
2. **Environment Variables**
3. **Default Values** (lowest priority)

### CLI Flags

Pass configuration via command-line flags:

```bash
./manager \
  --synology-url=https://synology.example.com:5001 \
  --secret-name=synology-credentials \
  --secret-namespace=default \
  --watch-namespaces="app-*,*-prod" \
  --log-level=info
```

### Environment Variables

Set configuration via environment variables:

```bash
export SYNOLOGY_URL=https://synology.example.com:5001
export SECRET_NAME=synology-credentials
export SECRET_NAMESPACE=default
export WATCH_NAMESPACES="app-*,*-prod"
export LOG_LEVEL=info
./manager
```

### Configuration Precedence Example

```bash
# Environment variable sets URL
export SYNOLOGY_URL=https://env.example.com:5001

# CLI flag overrides environment variable
./manager --synology-url=https://flag.example.com:5001

# Result: Uses https://flag.example.com:5001
```

---

## Synology Configuration

### `--synology-url` / `SYNOLOGY_URL`

**Required**: Yes  
**Type**: String  
**Default**: None  
**Description**: Base URL of the Synology DSM instance.

**Requirements**:
- Must use HTTPS protocol
- Must be a valid URL
- Should include port if non-standard (e.g., `:5001`)

**Examples**:
```bash
--synology-url=https://synology.example.com:5001
--synology-url=https://192.168.1.100:5001
```

### `--secret-name` / `SECRET_NAME`

**Required**: Yes  
**Type**: String  
**Default**: None  
**Description**: Name of the Kubernetes Secret containing Synology credentials.

**Example**:
```bash
--secret-name=synology-credentials
```

### `--secret-namespace` / `SECRET_NAMESPACE`

**Required**: Yes  
**Type**: String  
**Default**: None  
**Description**: Namespace where the credentials Secret is located.

**Example**:
```bash
--secret-namespace=default
```

### `--max-retries` / `MAX_RETRIES`

**Required**: No  
**Type**: Integer  
**Default**: `3`  
**Range**: 0-10  
**Description**: Maximum number of retry attempts for failed API calls.

**Example**:
```bash
--max-retries=5
```

### `--retry-delay` / `RETRY_DELAY`

**Required**: No  
**Type**: Duration  
**Default**: `1s`  
**Description**: Initial delay between retry attempts (uses exponential backoff).

**Examples**:
```bash
--retry-delay=1s
--retry-delay=500ms
--retry-delay=2s
```

### `--timeout` / `TIMEOUT`

**Required**: No  
**Type**: Duration  
**Default**: `30s`  
**Description**: Timeout for Synology API requests.

**Examples**:
```bash
--timeout=30s
--timeout=1m
```

---

## Controller Configuration

### `--watch-namespaces` / `WATCH_NAMESPACES`

**Required**: No  
**Type**: String (comma-separated patterns)  
**Default**: `*` (all namespaces)  
**Description**: Namespace patterns to watch for Ingress resources.

**Pattern Syntax**:
- `*` - Match all namespaces
- `exact-name` - Match exact namespace name
- `prefix-*` - Match namespaces starting with prefix
- `*-suffix` - Match namespaces ending with suffix
- `prefix-*-suffix` - Match namespaces with prefix and suffix

**Examples**:
```bash
# Watch all namespaces
--watch-namespaces="*"

# Watch specific namespaces
--watch-namespaces="default,kube-system"

# Watch namespaces with patterns
--watch-namespaces="app-*,*-prod"

# Combine exact and patterns
--watch-namespaces="default,app-*,*-prod"
```

### `--metrics-addr` / `METRICS_ADDR`

**Required**: No  
**Type**: String  
**Default**: `:8080`  
**Description**: Address for the metrics HTTP server.

**Examples**:
```bash
--metrics-addr=:8080
--metrics-addr=0.0.0.0:8080
```

### `--health-probe-addr` / `HEALTH_PROBE_ADDR`

**Required**: No  
**Type**: String  
**Default**: `:8081`  
**Description**: Address for the health check HTTP server.

**Examples**:
```bash
--health-probe-addr=:8081
--health-probe-addr=0.0.0.0:8081
```

### `--leader-election-id` / `LEADER_ELECTION_ID`

**Required**: No  
**Type**: String  
**Default**: `synology-proxy-operator`  
**Description**: Identifier for leader election (for future multi-replica support).

**Example**:
```bash
--leader-election-id=synology-proxy-operator
```

---

## Logging Configuration

### `--log-level` / `LOG_LEVEL`

**Required**: No  
**Type**: String  
**Default**: `info`  
**Valid Values**: `debug`, `info`, `warn`, `error`  
**Description**: Logging level for the operator.

**Examples**:
```bash
--log-level=debug  # Verbose logging
--log-level=info   # Standard logging
--log-level=warn   # Warnings and errors only
--log-level=error  # Errors only
```

### `--log-format` / `LOG_FORMAT`

**Required**: No  
**Type**: String  
**Default**: `json`  
**Valid Values**: `json`, `console`  
**Description**: Log output format.

**Examples**:
```bash
--log-format=json     # Structured JSON logs (production)
--log-format=console  # Human-readable logs (development)
```

---

## Secret Format

The operator expects a Kubernetes Secret with the following structure:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: synology-credentials
  namespace: default
type: Opaque
stringData:
  username: admin
  password: your-secure-password
```

**Required Fields**:
- `username` - Synology DSM admin username
- `password` - Synology DSM admin password

**Security Notes**:
- Use a dedicated admin account for the operator
- Store the Secret securely (consider using sealed-secrets or external secrets)
- The operator watches the Secret and automatically reloads credentials on changes
- Credentials are filtered from logs (never logged in plain text)

---

## Examples

### Minimal Configuration

```bash
./manager \
  --synology-url=https://synology.example.com:5001 \
  --secret-name=synology-credentials \
  --secret-namespace=default
```

### Production Configuration

```bash
./manager \
  --synology-url=https://synology.example.com:5001 \
  --secret-name=synology-credentials \
  --secret-namespace=synology-operator \
  --watch-namespaces="app-*,*-prod" \
  --max-retries=5 \
  --retry-delay=2s \
  --timeout=60s \
  --log-level=info \
  --log-format=json \
  --metrics-addr=:8080 \
  --health-probe-addr=:8081
```

### Development Configuration

```bash
./manager \
  --synology-url=https://192.168.1.100:5001 \
  --secret-name=synology-credentials \
  --secret-namespace=default \
  --watch-namespaces="*" \
  --log-level=debug \
  --log-format=console
```

### Environment Variables Configuration

```bash
# Set environment variables
export SYNOLOGY_URL=https://synology.example.com:5001
export SECRET_NAME=synology-credentials
export SECRET_NAMESPACE=default
export WATCH_NAMESPACES="app-*,*-prod"
export LOG_LEVEL=info
export LOG_FORMAT=json

# Run operator
./manager
```

### Kubernetes Deployment Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: synology-proxy-operator
  namespace: synology-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: synology-proxy-operator
  template:
    metadata:
      labels:
        app: synology-proxy-operator
    spec:
      serviceAccountName: synology-proxy-operator
      containers:
      - name: manager
        image: synology-proxy-operator:latest
        command:
        - /manager
        args:
        - --synology-url=https://synology.example.com:5001
        - --secret-name=synology-credentials
        - --secret-namespace=synology-operator
        - --watch-namespaces=app-*,*-prod
        - --log-level=info
        - --log-format=json
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        ports:
        - containerPort: 8080
          name: metrics
          protocol: TCP
        - containerPort: 8081
          name: health
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: health
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: health
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 500m
            memory: 256Mi
          requests:
            cpu: 100m
            memory: 128Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          runAsNonRoot: true
          runAsUser: 65532
```

---

## Validation

The operator validates all configuration at startup. If validation fails, the operator will exit with an error message.

**Common Validation Errors**:

- `synology URL must use HTTPS` - URL must start with `https://`
- `synology URL is required` - URL cannot be empty
- `invalid synology URL` - URL format is invalid
- `secret name is required` - Secret name cannot be empty
- `secret namespace is required` - Secret namespace cannot be empty
- `max retries must be >= 0` - Max retries cannot be negative
- `max retries must be <= 10` - Max retries cannot exceed 10
- `invalid log level` - Log level must be debug, info, warn, or error
- `invalid retry delay` - Retry delay must be a valid duration

---

## Health Checks

The operator exposes two health check endpoints:

### Liveness Probe

**Endpoint**: `/healthz`  
**Port**: Health probe port (default: 8081)  
**Description**: Returns 200 if the operator process is running.

```bash
curl http://localhost:8081/healthz
# Response: OK
```

### Readiness Probe

**Endpoint**: `/readyz`  
**Port**: Health probe port (default: 8081)  
**Description**: Returns 200 if the operator is ready to handle requests.

```bash
curl http://localhost:8081/readyz
# Response: JSON with detailed health status
```

**Example Response**:
```json
{
  "ready": true,
  "live": true,
  "message": "All checks passed",
  "checks": {
    "config": {
      "passed": true,
      "message": "Configuration is valid"
    },
    "synology": {
      "passed": true,
      "message": "Synology API is reachable"
    },
    "kubernetes": {
      "passed": true,
      "message": "Kubernetes API is accessible"
    }
  },
  "timestamp": "2026-03-09T10:30:00Z"
}
```

---

## Troubleshooting

### Operator Won't Start

1. Check configuration validation errors in logs
2. Verify Synology URL is accessible from the operator pod
3. Verify Secret exists in the specified namespace
4. Check RBAC permissions

### Credentials Not Loading

1. Verify Secret exists: `kubectl get secret <secret-name> -n <namespace>`
2. Verify Secret has `username` and `password` fields
3. Check operator logs for Secret watcher errors
4. Verify RBAC allows reading the Secret

### Namespace Filtering Not Working

1. Check `watch-namespaces` configuration
2. Verify pattern syntax (use `*` for wildcards)
3. Check operator logs for namespace filter initialization
4. Test pattern matching with debug logs enabled

---

## Best Practices

1. **Use HTTPS**: Always use HTTPS for Synology URL
2. **Secure Secrets**: Use sealed-secrets or external secrets for production
3. **Namespace Filtering**: Limit watched namespaces to reduce API load
4. **Resource Limits**: Set appropriate CPU/memory limits in production
5. **Health Checks**: Configure liveness and readiness probes
6. **Logging**: Use JSON format in production, console in development
7. **Monitoring**: Expose metrics endpoint for Prometheus scraping
8. **Retry Configuration**: Tune retry settings based on network reliability
9. **Dedicated Account**: Use a dedicated Synology admin account
10. **Regular Updates**: Keep credentials Secret updated via automation

---

## See Also

- [README.md](../README.md) - Project overview and quick start
- [Deployment Guide](deployment.md) - Kubernetes deployment instructions
- [API Reference](api-reference.md) - Synology API documentation
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
