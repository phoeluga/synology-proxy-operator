# Architecture

## Overview

The Synology Proxy Operator is a Kubernetes operator that automates the management of Synology reverse proxy records based on Ingress resources.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                        │
│                                                              │
│  ┌──────────────┐         ┌─────────────────────────────┐  │
│  │   Ingress    │────────▶│  Synology Proxy Operator    │  │
│  │  Resources   │         │                             │  │
│  └──────────────┘         │  ┌──────────────────────┐   │  │
│                           │  │ Ingress Controller   │   │  │
│  ┌──────────────┐         │  │  - Watch Ingresses   │   │  │
│  │  Services    │         │  │  - Reconcile         │   │  │
│  └──────────────┘         │  │  - Update Status     │   │  │
│                           │  └──────────────────────┘   │  │
│                           │                             │  │
│                           │  ┌──────────────────────┐   │  │
│                           │  │ Synology API Client  │   │  │
│                           │  │  - Auth              │   │  │
│                           │  │  - Proxy CRUD        │   │  │
│                           │  │  - Certificate Ops   │   │  │
│                           │  └──────────────────────┘   │  │
│                           └─────────────────────────────┘  │
│                                       │                     │
└───────────────────────────────────────┼─────────────────────┘
                                        │ HTTPS
                                        ▼
                            ┌───────────────────────┐
                            │   Synology NAS        │
                            │  ┌─────────────────┐  │
                            │  │ Reverse Proxy   │  │
                            │  │ Manager         │  │
                            │  └─────────────────┘  │
                            └───────────────────────┘
```

## Components

### 1. Ingress Controller
- **Purpose**: Main reconciliation loop
- **Responsibilities**:
  - Watch Ingress resources in filtered namespaces
  - Trigger reconciliation on changes
  - Manage finalizers for cleanup
  - Update Ingress status
  - Create Kubernetes events

### 2. Synology API Client
- **Purpose**: Interface with Synology NAS API
- **Responsibilities**:
  - Authentication and session management
  - Proxy record CRUD operations
  - Certificate operations
  - ACL profile management
  - Retry logic and error handling

### 3. Certificate Matcher
- **Purpose**: Match hostnames to SSL certificates
- **Responsibilities**:
  - Exact match (CN and SANs)
  - Wildcard match (*.example.com)
  - Certificate caching

### 4. Backend Discovery
- **Purpose**: Extract backend service information
- **Responsibilities**:
  - Parse Ingress spec
  - Construct Kubernetes service FQDN
  - Determine backend protocol

### 5. Metrics Registry
- **Purpose**: Prometheus metrics collection
- **Responsibilities**:
  - Track reconciliation metrics
  - Track API metrics
  - Track certificate cache metrics
  - Expose metrics endpoint

## Data Flow

### Reconciliation Flow

```
1. Ingress Created/Updated
   │
   ▼
2. Controller Detects Change
   │
   ▼
3. Check Annotation (synology.io/enabled)
   │
   ├─ No ──▶ Ignore
   │
   ├─ Yes
   │  │
   │  ▼
4. Extract Frontend Hostname
   │
   ▼
5. Discover Backend Service
   │
   ▼
6. Match Certificate
   │
   ▼
7. Query Existing Proxy Record
   │
   ├─ Not Found ──▶ Create New Record
   │                      │
   ├─ Found ──▶ Compare ──┤
   │              │       │
   │              ├─ Different ──▶ Update Record
   │              │                     │
   │              └─ Same ──▶ No-Op    │
   │                                    │
   ▼                                    ▼
8. Assign Certificate (if matched)
   │
   ▼
9. Update Ingress Status
   │
   ▼
10. Create Kubernetes Event
```

### Deletion Flow

```
1. Ingress Deleted
   │
   ▼
2. Controller Detects Deletion
   │
   ▼
3. Check Finalizer
   │
   ├─ No Finalizer ──▶ Allow Deletion
   │
   ├─ Has Finalizer
   │  │
   │  ▼
4. Check Deletion Policy
   │
   ├─ "retain" ──▶ Log Retention ──┐
   │                                │
   ├─ "delete" ──▶ Query Record ───┤
   │                 │              │
   │                 ▼              │
   │            Delete Record       │
   │                 │              │
   │                 ▼              │
   ▼                 │              │
5. Remove Finalizer ◀──────────────┘
   │
   ▼
6. Allow Kubernetes Deletion
```

## Design Decisions

### 1. Annotation-Based Opt-In
**Decision**: Require explicit annotation `synology.io/enabled: "true"`  
**Rationale**: 
- Prevents accidental management of all Ingresses
- Allows gradual adoption
- Clear intent from users

### 2. Finalizer-Based Cleanup
**Decision**: Use Kubernetes finalizers for cleanup  
**Rationale**:
- Ensures proxy records are cleaned up before Ingress deletion
- Prevents orphaned records
- Supports deletion policies (delete/retain)

### 3. Description-Based Record Identification
**Decision**: Store `k8s:namespace/name:uid` in description field  
**Rationale**:
- Synology API doesn't provide custom metadata
- Description field is searchable
- UID ensures uniqueness across recreations

### 4. Certificate Caching
**Decision**: Cache certificates with 5-minute TTL  
**Rationale**:
- Reduces API calls
- Certificates change infrequently
- TTL ensures eventual consistency

### 5. Namespace Filtering
**Decision**: Filter at watch level, not reconciliation level  
**Rationale**:
- More efficient (fewer events)
- Reduces API server load
- Supports wildcard patterns

### 6. Status in Annotations
**Decision**: Store status in Ingress annotations  
**Rationale**:
- Ingress doesn't have custom status fields
- Annotations are easily queryable
- Also update LoadBalancer status for visibility

### 7. Single Replica
**Decision**: Run single replica (no leader election in v0.1)  
**Rationale**:
- Simpler initial implementation
- Synology API is the bottleneck, not operator
- Can add leader election in future versions

### 8. Idempotent Reconciliation
**Decision**: All operations are idempotent  
**Rationale**:
- Safe to retry on failures
- Handles operator restarts gracefully
- Simplifies error handling

## Security Considerations

### 1. Credentials Storage
- Stored in Kubernetes Secrets
- Never logged in plain text
- Filtered from all log output
- Secret watcher for automatic reload

### 2. RBAC
- Least-privilege permissions
- ClusterRole for Ingress access
- Limited Secret access (specific name only)
- No write access to Secrets

### 3. TLS Verification
- HTTPS required for Synology URL
- TLS verification enabled by default
- Optional CA certificate support
- No HTTP fallback

### 4. Network Security
- Operator initiates all connections
- No inbound connections required
- Synology API over HTTPS only

## Scalability

### Current Limits
- Single replica
- ~500 Ingress resources
- ~10 concurrent reconciliations
- ~10 API requests/second

### Future Improvements
- Leader election for multi-replica
- Horizontal scaling with sharding
- Increased concurrency
- Batch API operations

## Observability

### Metrics
- 10 Prometheus metrics
- Reconciliation latency and count
- API request latency and count
- Certificate cache hit rate
- Error tracking by type

### Logging
- Structured JSON logging
- Sensitive data filtering
- Context propagation
- Configurable log levels

### Health Checks
- Liveness probe (process health)
- Readiness probe (dependency health)
- Configuration validation
- API connectivity check

## Dependencies

### External
- Synology NAS with DSM 7.x
- Kubernetes 1.19+
- Prometheus (optional, for metrics)

### Internal
- controller-runtime (Kubernetes controller framework)
- client-go (Kubernetes client)
- Zap (structured logging)
- Prometheus client (metrics)

## Future Enhancements

1. **Multi-Replica Support**: Leader election for high availability
2. **Custom Resource**: Dedicated CRD instead of Ingress annotations
3. **Webhook Validation**: Validate Ingress before admission
4. **Batch Operations**: Bulk create/update/delete for efficiency
5. **Advanced Certificate Matching**: Support for multiple certificates per hostname
6. **ACL Management**: Create/update ACL profiles from Kubernetes
7. **Metrics Dashboard**: Pre-built Grafana dashboard
8. **Alerting Rules**: Pre-configured Prometheus alerts
