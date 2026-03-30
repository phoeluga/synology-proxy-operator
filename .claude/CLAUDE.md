# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build & generate
make build                   # Compile binary to bin/manager
make generate                # Regenerate deepcopy methods (after editing api/v1alpha1/ types)
make manifests               # Regenerate RBAC + CRD manifests

# Test & lint
make test                    # Run tests with envtest (outputs coverage.out)
make fmt && make vet         # Format and vet
make lint                    # golangci-lint

# Local development
make minikube-start          # Start local cluster (4 CPUs, 6GB RAM, ingress addon)
make dev-setup               # Install CRDs + ArgoCD + test fixtures
make dev-run                 # Run operator locally (reads .env.local)
make minikube-stop           # Tear down local cluster

# Docker
make image-build             # Build container image
make image-buildx            # Multi-arch build (amd64 + arm64)
```

**After editing `api/v1alpha1/` types**, always run `make generate && make manifests`.

Run a single test: `go test ./internal/controller/... -run TestFoo -v`

## Architecture

Kubernetes operator that syncs reverse proxy rules to Synology DSM via its WebAPI.

**Three reconcilers, one sync point:**

1. **`ServiceIngressReconciler`** (`internal/controller/serviceingress_controller.go`) — watches Services/Ingresses annotated with `synology.proxy/enabled: "true"`, creates/deletes `SynologyProxyRule` CRs only. No DSM calls.
2. **`ArgoApplicationReconciler`** (`internal/controller/argoapplication_controller.go`) — watches ArgoCD `Application` resources (optional), creates/deletes `SynologyProxyRule` CRs only. No DSM calls.
3. **`SynologyProxyRuleReconciler`** (`internal/controller/synologyproxyrule_controller.go`) — the **only** reconciler that calls DSM. Handles upsert and deletion (via finalizer).

**Annotation constants** — all `synology.proxy/*` keys live in `internal/controller/annotations.go`. Never redefine them in individual controllers.

**Synology client** (`internal/synology/`):
- `client.go` — HTTP client, cookie jar, SynoToken session management
- `proxy.go` — CRUD for DSM proxy records; idempotency via `description` field
- `certificate.go` — CN/SAN matching (wildcard support), assigns cert to record
- `acl.go` — resolves ACL profile names to UUIDs

**CRD** (`api/v1alpha1/synologyproxyrule_types.go`): `SynologyProxyRule` (short: `spr`, namespaced). `status.managedRecords` tracks DSM UUIDs for cleanup on deletion. `serviceRef`/`ingressRef` trigger backend auto-discovery.

**Backend auto-discovery** (priority order in `reconcileUpsert`):
1. `spec.destinationHost` (explicit)
2. `spec.serviceRef` → LoadBalancer ExternalIP
3. `spec.ingressRef` → Ingress status IP
4. Namespace auto-scan for first LoadBalancer Service

**ArgoCD integration**: minimal local types in `internal/argo/types.go` (avoids full ArgoCD dependency). Watcher disabled gracefully at startup if ArgoCD CRDs are absent — no restart needed when they appear.

## Key conventions

- `description` field on DSM proxy records is the **idempotency key** — records are looked up by description, not UUID. Default is `<namespace>/<name>` to prevent cross-namespace collisions.
- Finalizer `proxy.synology.io/finalizer` on every `SynologyProxyRule` drives DSM cleanup on deletion. Finalizer is removed **last**, only after all DSM deletes succeed.
- `spec.additionalSourceHosts` creates one DSM record per hostname; all tracked in `status.managedRecords`.
- Stale record delete failures keep the record in `status.managedRecords` and requeue — never drop a failed delete from status.
- `ServiceIngressReconciler` and `ArgoApplicationReconciler` must diff the spec with `reflect.DeepEqual` before calling `Update` — skip the write if unchanged to avoid spurious DSM syncs.
- ACL profile UUID is resolved from DSM once and cached with a 5-minute TTL in the reconciler struct.
- CI pushes `:edge` to GHCR on every merge to `main`. `:latest` is only published on semver releases.
- `WATCH_NAMESPACE` accepts a glob pattern (e.g. `app-*`). When set, all Services, Ingresses and ArgoCD Applications in matching namespaces are auto-enabled — no `synology.proxy/enabled` annotation needed. Implemented via `namespaceMatches()` in `internal/controller/namespace.go` using `path.Match`.
- DSM API quirks (see `docs/local-testing.md`): create/update can take ~2 min; method is `update` not `set`; SynoToken required in both header and form body; always pass `old_id: ""` for certificate assignment.

## Linting

`.golangci.yml` defines the active linter set. Pin the version — do not use `latest`. Current pinned version in CI: `v1.61.0`.

## Agents & Skills

- `@code-reviewer` — review Go changes for K8s operator correctness (finalizers, RBAC, idempotency)
- `/release` — guided release workflow: bump Chart.yaml version, tag, push
- `/fix-issue [number]` — fix a GitHub issue end-to-end

## Documentation

- `docs/architecture.md` — internal design, reconciler flow, project structure
- `docs/development.md` — build, test, CRD generation, local dev
- `docs/local-testing.md` — full minikube walkthrough
- `docs/release.md` — release steps, CI pipeline, image tag strategy
