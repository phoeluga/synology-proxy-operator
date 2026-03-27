# Troubleshooting Guide

## Common Issues

### Operator Not Starting

**Symptoms**:
- Pod in CrashLoopBackOff
- Pod not reaching Ready state

**Possible Causes**:
1. Missing or invalid credentials Secret
2. Invalid Synology URL
3. Network connectivity issues
4. RBAC permissions missing

**Solutions**:

```bash
# Check pod status
kubectl get pods -n synology-operator

# Check pod logs
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator

# Check events
kubectl get events -n synology-operator --sort-by='.lastTimestamp'

# Verify Secret exists
kubectl get secret synology-credentials -n synology-operator

# Verify Secret has correct keys
kubectl get secret synology-credentials -n synology-operator -o jsonpath='{.data}' | jq

# Test Synology URL connectivity
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -k https://your-synology-url:5001
```

---

### Ingress Not Being Reconciled

**Symptoms**:
- Ingress created but no proxy record in Synology
- No events on Ingress
- No status updates

**Possible Causes**:
1. Missing annotation `synology.io/enabled: "true"`
2. Namespace not being watched
3. Operator not running
4. RBAC permissions issue

**Solutions**:

```bash
# Check Ingress has annotation
kubectl get ingress <name> -n <namespace> -o yaml | grep synology.io/enabled

# Add annotation if missing
kubectl annotate ingress <name> -n <namespace> synology.io/enabled=true

# Check operator is watching namespace
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator | grep "namespace"

# Check operator logs for errors
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator | grep <ingress-name>

# Check Ingress events
kubectl describe ingress <name> -n <namespace>
```

---

### Synology API Authentication Failures

**Symptoms**:
- Logs show "401 Unauthorized"
- Logs show "authentication failed"
- Reconciliation fails with auth errors

**Possible Causes**:
1. Incorrect username/password
2. Account locked
3. Account doesn't have admin privileges
4. Session expired

**Solutions**:

```bash
# Verify credentials
kubectl get secret synology-credentials -n synology-operator -o jsonpath='{.data.username}' | base64 -d
kubectl get secret synology-credentials -n synology-operator -o jsonpath='{.data.password}' | base64 -d

# Update credentials
kubectl create secret generic synology-credentials \
  --namespace synology-operator \
  --from-literal=username=admin \
  --from-literal=password=new-password \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart operator to reload credentials
kubectl rollout restart deployment -n synology-operator

# Check Synology DSM
# - Verify account exists
# - Verify account is not locked
# - Verify account has admin privileges
```

---

### Certificate Not Assigned

**Symptoms**:
- Proxy record created but no certificate
- Logs show "no certificate matched"
- Status shows no certificate ID

**Possible Causes**:
1. Certificate doesn't exist in Synology
2. Hostname doesn't match certificate CN or SAN
3. Certificate cache stale
4. Wildcard certificate not matching correctly

**Solutions**:

```bash
# Check operator logs for certificate matching
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator | grep certificate

# Check Ingress hostname
kubectl get ingress <name> -n <namespace> -o jsonpath='{.spec.rules[0].host}'

# Verify certificate in Synology DSM
# - Go to Control Panel > Security > Certificate
# - Check certificate CN and SANs
# - Verify certificate is not expired

# Force cache refresh (wait 5 minutes or restart operator)
kubectl rollout restart deployment -n synology-operator

# Check certificate matching logic
# Exact match: hostname == CN or hostname in SANs
# Wildcard match: *.example.com matches subdomain.example.com (one level only)
```

---

### Proxy Record Not Updating

**Symptoms**:
- Ingress updated but proxy record unchanged
- Old backend still in use
- Status shows old information

**Possible Causes**:
1. Reconciliation not triggered
2. Update failed silently
3. Synology API error
4. Operator not detecting changes

**Solutions**:

```bash
# Force reconciliation by adding annotation
kubectl annotate ingress <name> -n <namespace> synology.io/force-sync="$(date +%s)" --overwrite

# Check operator logs
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator --tail=100

# Check Ingress status
kubectl get ingress <name> -n <namespace> -o yaml | grep -A 10 annotations

# Verify in Synology DSM
# - Go to Control Panel > Application Portal > Reverse Proxy
# - Check record details
# - Verify backend matches Ingress spec
```

---

### Deletion Not Working

**Symptoms**:
- Ingress stuck in Terminating state
- Proxy record not deleted
- Finalizer not removed

**Possible Causes**:
1. Operator not running
2. Synology API error during deletion
3. Deletion policy set to "retain"
4. Finalizer removal failed

**Solutions**:

```bash
# Check Ingress status
kubectl get ingress <name> -n <namespace> -o yaml

# Check deletion policy
kubectl get ingress <name> -n <namespace> -o jsonpath='{.metadata.annotations.synology\.io/deletion-policy}'

# Check operator logs
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator | grep deletion

# Force finalizer removal (CAUTION: may leave orphaned proxy record)
kubectl patch ingress <name> -n <namespace> -p '{"metadata":{"finalizers":[]}}' --type=merge

# Manually delete proxy record in Synology DSM if needed
```

---

### High Memory Usage

**Symptoms**:
- Pod using more memory than expected
- OOMKilled events
- Performance degradation

**Possible Causes**:
1. Too many Ingress resources
2. Memory leak
3. Large certificate cache
4. Insufficient limits

**Solutions**:

```bash
# Check current memory usage
kubectl top pod -n synology-operator

# Check resource limits
kubectl get deployment -n synology-operator -o yaml | grep -A 5 resources

# Increase memory limits
helm upgrade synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --reuse-values \
  --set resources.limits.memory=512Mi

# Check for memory leaks
kubectl logs -n synology-operator -l app.kubernetes.io/name=synology-proxy-operator | grep -i "memory\|leak"

# Restart operator
kubectl rollout restart deployment -n synology-operator
```

---

### Slow Reconciliation

**Symptoms**:
- Ingress takes long time to reconcile
- Metrics show high latency
- Timeouts in logs

**Possible Causes**:
1. Slow Synology API
2. Network latency
3. Too many concurrent reconciliations
4. Certificate cache misses

**Solutions**:

```bash
# Check reconciliation metrics
kubectl port-forward -n synology-operator svc/synology-operator-metrics 8080:8080
curl http://localhost:8080/metrics | grep reconcile_duration

# Check API latency
curl http://localhost:8080/metrics | grep api_request_duration

# Check certificate cache hit rate
curl http://localhost:8080/metrics | grep certificate_cache

# Increase timeout
helm upgrade synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --reuse-values \
  --set synology.timeout=60s

# Reduce concurrent reconciliations (edit deployment)
# Set --max-concurrent-reconciles=5
```

---

### TLS Verification Failures

**Symptoms**:
- Logs show "x509: certificate signed by unknown authority"
- Logs show "TLS handshake failed"
- Cannot connect to Synology

**Possible Causes**:
1. Self-signed certificate
2. Custom CA certificate
3. Certificate expired
4. Hostname mismatch

**Solutions**:

```bash
# Disable TLS verification (NOT RECOMMENDED for production)
helm upgrade synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --reuse-values \
  --set synology.tlsVerify=false

# Or provide CA certificate
kubectl create configmap synology-ca \
  --namespace synology-operator \
  --from-file=ca.crt=/path/to/ca.crt

helm upgrade synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --reuse-values \
  --set synology.caCertPath=/etc/ssl/certs/ca.crt

# Verify certificate
openssl s_client -connect your-synology:5001 -showcerts
```

---

## Debugging Tips

### Enable Debug Logging

```bash
helm upgrade synology-operator ./charts/synology-proxy-operator \
  --namespace synology-operator \
  --reuse-values \
  --set operator.logLevel=debug
```

### Check Metrics

```bash
kubectl port-forward -n synology-operator svc/synology-operator-metrics 8080:8080
curl http://localhost:8080/metrics
```

### Check Health Endpoints

```bash
kubectl port-forward -n synology-operator pod/<pod-name> 8081:8081
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

### Exec into Pod

```bash
kubectl exec -it -n synology-operator <pod-name> -- sh
# Note: Distroless image has no shell, use debug container instead
kubectl debug -it -n synology-operator <pod-name> --image=busybox --target=operator
```

---

## Getting Help

If you're still experiencing issues:

1. **Check GitHub Issues**: https://github.com/phoeluga/synology-proxy-operator/issues
2. **Create New Issue**: Include:
   - Operator version
   - Kubernetes version
   - Synology DSM version
   - Operator logs
   - Ingress YAML
   - Error messages
3. **Community Support**: Join discussions on GitHub

---

## Known Issues

### Issue: Operator doesn't support multiple backends
**Status**: Planned for future release  
**Workaround**: Create separate Ingress for each backend

### Issue: Certificate matching doesn't support multi-level wildcards
**Status**: By design (Synology limitation)  
**Workaround**: Use exact match certificates or single-level wildcards

### Issue: No support for TCP/UDP proxy
**Status**: Planned for future release  
**Workaround**: Configure manually in Synology DSM
