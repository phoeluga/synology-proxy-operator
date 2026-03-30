---
name: fix-issue
description: Fix a GitHub issue in synology-proxy-operator end-to-end — read the issue, implement the fix, test, and open a PR.
disable-model-invocation: true
argument-hint: "[issue-number]"
---

Fix GitHub issue #$ARGUMENTS in synology-proxy-operator.

## Step 1 — Understand the issue
```bash
gh issue view $ARGUMENTS
```
Read the full issue including comments. Summarise: what is broken or missing, what the expected behaviour is, and which part of the codebase is likely involved.

## Step 2 — Explore affected code
Based on the issue summary, read the relevant files. Focus on:
- `internal/controller/` — reconciler logic
- `internal/synology/` — DSM client (proxy, cert, acl)
- `api/v1alpha1/` — CRD types

Do not read files that are clearly unrelated.

## Step 3 — Implement the fix
Make the minimal change that addresses the issue. Follow the project conventions:
- `description` is the idempotency key for DSM records
- Finalizer must be removed last after DSM cleanup
- Status updates use `Status().Update()` separately from spec updates
- RBAC markers must match any new resource accesses
- If `api/v1alpha1/` types changed: run `make generate && make manifests`

## Step 4 — Test
```bash
make fmt && make vet
make test
make lint
```
Fix any failures before continuing.

## Step 5 — Create a branch and commit
```bash
git checkout -b fix/issue-$ARGUMENTS
git add <relevant files>
git commit -m "fix: <short description of fix>

Fixes #$ARGUMENTS"
```

## Step 6 — Open a PR
```bash
gh pr create \
  --title "fix: <short description>" \
  --body "$(cat <<'EOF'
## Summary
- <what was broken>
- <what the fix does>

Fixes #$ARGUMENTS

## Test plan
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Manually verified with `make dev-run` (if applicable)
EOF
)"
```

Return the PR URL.
