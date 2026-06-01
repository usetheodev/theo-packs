# Handoff: theokit ↔ theokit-packs integration status

> **Generated:** 2026-06-01
> **Branch:** develop (7 commits ahead of main, pushed to origin/develop)
> **Sister repo:** `theokit-tools/theokit` (sibling)
> **Status:** theokit-packs side is COMPLETE. End-to-end depends on a theokit-side decision.

---

## What theokit-packs delivers (this repo, this cycle)

| Capability | Status | Evidence |
|---|---|---|
| Renamed `theo-packs` → `theokit-packs` | ✅ Done | Commits efbbbe9, 291764d, ce61053, c753f0f |
| `pnpm/turbo/npm --filter` resolves real `package.json#name` (FIX B1) | ✅ Done | Commit fa06e9a, tests `TestNodeProvider_B1_*` |
| Fail-fast on cross-repo `../` workspace entries (FIX B3) | ✅ Done | Commit fa06e9a, tests `TestNodeProvider_B3_*` |
| Regression guard: `examples/node-pnpm-workspaces-scoped/` | ✅ Done | Golden + integration test |
| 22 Go test packages green | ✅ Done | `go test ./core/... ./cmd/...` |
| Defer B2 (start cmd) | ✅ As planned | Per blueprint ADR D2 — to be re-evaluated after first real build |

## End-to-end POC: validated 2026-06-01

Simulated `theokit/examples/openrouter-demo` with:
- `@usetheo/sdk: workspace:*` → `^1.3.0` (the version published on npm)
- `pnpm-workspace.yaml` with the 3 `../theokit-sdk/...` entries removed

theokit-packs-generate then produced a valid Dockerfile:
- Provider detected: node (Node 22 from engines)
- Filter line: `pnpm --filter @usetheo/example-openrouter-demo... run build` ← **B1 resolving the real scoped name**
- CMD: `cd examples/openrouter-demo && npm start`
- Defensive header, syntax directive, prune all present

**Conclusion:** the moment theokit removes the cross-repo workspace entries (for the deploy flow), theokit-packs produces a working Dockerfile end-to-end. No further theokit-packs change required.

## What theokit must decide (out of this repo's scope)

The 4 examples in `theokit/examples/` declare `@usetheo/sdk`, `@usetheo/gateway`, `@usetheo/gateway-telegram` as `workspace:*` and the workspace lists `../theokit-sdk/packages/{sdk,gateway,gateway-telegram}`. This is **intentional** (theokit ADR 0001) — cross-repo workspace link enables hot-reload during local dev.

For containerized deployment, that path can never resolve inside the Docker build context. The theokit team needs to choose ONE of the following:

### Option A — Two workspace files (recommended; minimal churn)

1. Keep `pnpm-workspace.yaml` (current — siblings present, DX preserved).
2. Add `pnpm-workspace.deploy.yaml` (sibling entries removed).
3. Add `package.deploy.json` overlay or use the `pnpm overrides` mechanism to pin `@usetheo/sdk` and the gateways to their published npm versions during deploy.
4. theokit-packs invocation in TheoCloud Argo: copy `pnpm-workspace.deploy.yaml` over `pnpm-workspace.yaml` before invoking `theokit-packs-generate`.

**Pros:** zero impact on local dev. Single PR in theokit.
**Cons:** TheoCloud pipeline gets one extra step.

### Option B — Move workspaces to a deploy-only branch

Maintain a `deploy` branch in theokit whose `pnpm-workspace.yaml` already has the deploy form. Cherry-pick releases. **Rejected as fragile.**

### Option C — theokit-packs feature: workspace rewrite

Add `--rewrite-workspace-siblings` flag to theokit-packs that on detect-and-rewrite removes `../` entries and substitutes `workspace:*` with published versions resolved from `pnpm-lock.yaml`. **Possible but** violates KISS — adds knowledge of pnpm internals to theokit-packs, expands its scope from "Dockerfile generator" to "deploy-prep tool".

**Recommendation:** Option A.

## What's deferred at the theokit-packs side

- **B2 (`CMD cd <app> && npm start`)** — once a theokit example produces a Dockerfile via Option A and a real `docker build && docker run` is attempted, observe whether the container starts successfully. If yes, close B2 as "not a bug" per blueprint ADR D2. If no, open a focused PR with the minimum fix (most likely add `node_modules/.bin` to PATH explicitly).
- **Phase 6 of the rename plan** — GitHub repo rename `theo-packs → theokit-packs`. Requires human action (settings UI + remote update on dev machines + TheoCloud + theokit CI configs). Cutover window TBD.
- **CHANGELOG `#NNN` placeholders** — replace with actual PR/issue numbers when each feature gets a tracking issue.

## How to verify (quick recipe)

```bash
# In theo-packs (this repo)
git switch develop
git pull
go build -o /tmp/theokit-packs-generate ./cmd/theokit-packs-generate
go test ./core/... ./cmd/...  # all green

# Simulate theokit-deploy (no commit, just verify the path works)
cp -r /home/paulo/Projetos/usetheo/theokit-tools/theokit /tmp/poc
grep -v "'\.\./" /tmp/poc/pnpm-workspace.yaml > /tmp/ws.yaml
cat /tmp/ws.yaml > /tmp/poc/pnpm-workspace.yaml
sed -i 's|"@usetheo/sdk": "workspace:\*"|"@usetheo/sdk": "^1.3.0"|' /tmp/poc/examples/openrouter-demo/package.json

/tmp/theokit-packs-generate \
  --source /tmp/poc \
  --app-path examples/openrouter-demo \
  --app-name openrouter-demo \
  --output /tmp/df.openrouter

grep --color filter /tmp/df.openrouter
# Expected: pnpm --filter @usetheo/example-openrouter-demo... run build
```

## Audit trail

- Discovery plan: `.claude/knowledge-base/discoveries/plans/theokit-support-plan.md` (v1.1)
- Edge-case review: `.claude/knowledge-base/reviews/theokit-support-edge-cases-2026-06-01.md`
- Blueprint: `.claude/knowledge-base/discoveries/blueprints/theokit-support-blueprint.md` (SHIPPABLE 96.8/100)
- Rename plan: `docs/plans/rename-to-theokit-packs-plan.md`
- Audit baseline: `docs/plans/rename-to-theokit-packs-audit.md`
- Fixes plan: `docs/plans/theokit-support-fixes-plan.md`
- This handoff: `docs/plans/theokit-integration-handoff.md`
