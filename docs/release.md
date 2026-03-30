# Release process

This document describes how releases work, what CI does, and how to cut a new version.

---

## Release workflow

Releases are triggered by pushing a semver tag. The full process:

```
1. Bump Chart.yaml version (manual)
          │
          ▼
2. git tag v0.x.0 && git push origin v0.x.0
          │
          ▼
3. release.yaml workflow fires automatically
   ├── Run full test suite + lint
   ├── Build multi-arch Docker image (linux/amd64, linux/arm64)
   ├── Push to GHCR:
   │     ghcr.io/phoeluga/synology-proxy-operator:v0.x.0
   │     ghcr.io/phoeluga/synology-proxy-operator:0.x
   │     ghcr.io/phoeluga/synology-proxy-operator:latest
   ├── Package Helm chart → synology-proxy-operator-0.x.0.tgz
   └── Create GitHub Release with auto-generated notes + chart attachment
```

The `/release` skill in `.claude/skills/release/SKILL.md` guides you through these steps interactively.

---

## Step-by-step

### 1. Update Chart.yaml

Edit `helm/synology-proxy-operator/Chart.yaml`:

```yaml
version: 0.x.0      # chart version (semver, no "v" prefix)
appVersion: "0.x.0" # operator version (quoted)
```

Both fields should match the tag you intend to push.

### 2. Run checks

```bash
make fmt && make vet
make test
make lint
make helm-lint
```

Do not proceed if any step fails.

### 3. Commit and tag

```bash
git add helm/synology-proxy-operator/Chart.yaml
git commit -m "chore: release v0.x.0"
git tag v0.x.0
git push origin main
git push origin v0.x.0
```

### 4. Monitor CI

```bash
gh run list --workflow=release.yaml --limit=3
```

The release workflow takes ~5–10 minutes (multi-arch build is the slow step).

---

## CI pipeline (on every PR and push to main)

**File:** `.github/workflows/ci.yaml`

| Job | Trigger | What it does |
|---|---|---|
| `test` | all PRs + main pushes | `go test ./...` with coverage |
| `lint` | all PRs + main pushes | `golangci-lint` (version pinned in `.golangci.yml`) |
| `helm-lint` | all PRs + main pushes | `helm lint` |
| `docker-build` | all PRs + main pushes | builds image, no push |
| `push-main` | main pushes only | pushes `:main` tag to GHCR (multi-arch) |

The `:main` tag represents the latest merged state of the `main` branch. It is not suitable for production use.

In-progress CI runs on the same branch are automatically cancelled when a new push arrives.

---

## Image tags

| Tag | When published | Suitable for |
|---|---|---|
| `:latest` | on every semver release | production |
| `:v0.x.0` | on every semver release | pinned production |
| `:0.x` | on every semver release | minor-pinned production |
| `:main` | on every push to `main` | testing unreleased changes |
| `:ci` | during PR builds (not pushed) | — |

---

## Branch protection

The `main` branch should have the following protection rules configured in GitHub:

- Require status checks: `test`, `lint`, `helm-lint`, `docker-build`
- Require branches to be up to date before merging
- No direct pushes to `main` — all changes via PR

This ensures `:latest` is always a product of passing CI.

---

## Versioning

The project follows [Semantic Versioning](https://semver.org/):

- **Patch** (`0.x.Y`) — bug fixes, no API or behaviour changes
- **Minor** (`0.X.0`) — new features, backwards-compatible
- **Major** (`X.0.0`) — breaking changes to the CRD API or operator behaviour

The CRD is currently `v1alpha1`. Graduating to `v1beta1` or `v1` constitutes a minor or major version bump depending on whether the migration is automatic.
