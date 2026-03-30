---
name: release
description: Guided release workflow for synology-proxy-operator — bump Chart.yaml version, tag, and push to trigger CI release pipeline.
disable-model-invocation: true
argument-hint: "[version e.g. v0.2.0]"
---

Release synology-proxy-operator as version $ARGUMENTS.

Follow these steps exactly, pausing to confirm before any destructive or publishing action:

## Step 1 — Verify clean state
Run:
```bash
git status
git log --oneline -5
```
If there are uncommitted changes, stop and ask the user whether to stash or commit them first.

## Step 2 — Determine version
If $ARGUMENTS is empty, ask the user what version to release (e.g. `v0.2.0`).
Strip the leading `v` where needed for Chart.yaml (e.g. `v0.2.0` → `0.2.0`).

## Step 3 — Update Helm chart version
Edit `helm/synology-proxy-operator/Chart.yaml`:
- Set `version:` to the chart version (semver without `v`, e.g. `0.2.0`)
- Set `appVersion:` to the same value (quoted, e.g. `"0.2.0"`)

Show the diff after editing and ask for confirmation before continuing.

## Step 4 — Run checks
```bash
make fmt && make vet
make test
make helm-lint
```
Stop and report any failures. Do not proceed if tests or lint fail.

## Step 5 — Commit the version bump
```bash
git add helm/synology-proxy-operator/Chart.yaml
git commit -m "chore: release $ARGUMENTS"
```

## Step 6 — Create and push the tag
Show the user exactly what will be run, then ask for confirmation:
```bash
git tag $ARGUMENTS
git push origin main
git push origin $ARGUMENTS
```

## Step 7 — Confirm CI triggered
```bash
gh run list --workflow=release.yaml --limit=3
```
Report the run URL so the user can monitor progress.

The release pipeline will automatically:
- Build multi-arch Docker image (amd64 + arm64) → GHCR
- Package Helm chart and attach to GitHub Release
- Generate release notes
