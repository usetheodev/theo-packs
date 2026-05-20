# Monorepo Build Contract Validation & Hardening

> In-repo response to the dogfood findings (F1-F7) against `theo-stacks/templates/monorepo-turbo`. Disproves F5 against current theo-packs, documents the workspace-root build-context contract, makes Dockerfile output self-explanatory via a defensive header, and locks the contract end-to-end with a new E2E test against the real upstream template.

## What this PR fixes (in scope)

| Finding | Status |
|---|---|
| **F5** — claim that theo-packs generates per-app `node_modules` COPY | **Disproven**. Manual reproduction against upstream `monorepo-turbo` shows the current generator emits `COPY --from=build /app /app` (whole workspace). The buggy Dockerfile that the dogfood saw came from `theo-stacks/templates/monorepo-turbo/apps/api/Dockerfile` — a user-provided file the CLI passes through unchanged per the documented user-Dockerfile-precedence contract. Locked with regression test `TestGoldens_NoPerAppNodeModulesCopy`. |
| **F3** — build context mismatch — theo-packs implication | **Documented**. theo-packs's workspace-mode Dockerfiles use paths relative to the workspace root. The contract is now explicit in `docs/contracts/theo-packs-cli-contract.md`, in the defensive header on every generated Dockerfile, and enforced end-to-end by `TestE2E_MonorepoTurboFromStacks`. |

## What this PR does NOT fix (explicitly out of scope)

| Finding | Why out | Where it lives |
|---|---|---|
| F1 — `theo.yaml` missing `build: dockerfile` | theo-stacks repo | PR against `usetheodev/theo-stacks` |
| F2 — buggy per-app `node_modules` COPY in user Dockerfile | theo-stacks repo | PR against `usetheodev/theo-stacks` |
| F6 — `theo.yaml` schema lacks `build_context:` separate from `path:` | theo product repo; systemic | Issue/plan in the theo product repo |
| F7 — `static_delivery.go` silent success when CF KV is unconfigured | theo product repo | Issue/plan in the theo product repo |

The contract document explains explicitly that theo-packs has **no in-repo workaround** for F3 — adding one would create two source-of-truth places for build-context resolution and confuse the contract. The fix lives in the product (F6).

## Changes

### Renderer

Every generated Dockerfile now carries a defensive header between the syntax directive and the first FROM:

```
# syntax=docker/dockerfile:1

# theo-packs: generated for provider "node".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

FROM ...
```

Implementation:
- `BuildPlan` gains optional `ProviderName string` field.
- `core.GenerateBuildPlan` stamps the detected provider name into the plan.
- `dockerfile.HeaderComment(name) string` produces the block; renderer-emitted (D1).
- All 58 goldens regenerated.
- `TestGoldens_HasProviderHeader` locks the invariant.

### Audit suite

- `TestGoldens_NoPerAppNodeModulesCopy` — corpus-level negative regex (no `(apps|packages)/<name>/node_modules` literals). D3 chose negative pattern over positive shape assertion: robust to future deploy-filter optimizations.
- `TestGoldens_NoPerAppNodeModulesCopy_DetectsBadPattern` — verifies the regex actually matches the F5-shape failure (synthetic input).
- `TestGoldens_NoPerAppNodeModulesCopy_AcceptsCleanOutput` — verifies the regex doesn't false-positive on what the current generator emits.
- `TestGoldens_HasProviderHeader` — every golden has the defensive header.

### Contract document

`docs/contracts/theo-packs-cli-contract.md` (~200 lines) covering:

- All four CLI flags with exact semantics.
- Env-var bridge (`--app-name` → `THEOPACKS_APP_NAME`, etc.) — universal across all workspace shapes after the v2 + cli-bridge fixes.
- Single-app vs workspace mode.
- User-Dockerfile precedence and what theo-packs guarantees about it.
- `.dockerignore` generation rules.
- Generated Dockerfile invariants.
- Explicit "what the theo product MUST pass" section with `docker build` invocations.
- F3/F6 acknowledgement and explanation of why theo-packs has no workaround.
- Failure modes table.
- Versioning policy.

Cross-linked from CLAUDE.md and README.md.

### E2E test

`TestE2E_MonorepoTurboFromStacks` in `e2e/e2e_test.go`:
- Reads `../theo-stacks/templates/monorepo-turbo` (sibling-checkout convention; D4).
- Skips cleanly when Docker is unavailable OR when theo-stacks is not checked out.
- Copies the template to a temp dir (never mutates upstream).
- Removes the buggy `apps/api/Dockerfile` so we test theo-packs's output, not the template's.
- Invokes the `theopacks-generate` BINARY (not the library) — same code path as the theo product.
- Asserts the defensive header is present.
- Runs `docker build` with **context = workspace root** — this is the contract assertion.
- Asserts the resulting image exists.

## Backward compatibility

- `core.GenerateBuildPlan` API unchanged (`BuildPlan.ProviderName` is additive).
- `theopacks.json` and `THEOPACKS_*` env vars unchanged.
- Existing generated Dockerfiles get a header comment but no functional change. `docker build` continues to work exactly as before.

## Quality gates

```
mise run test         ✓ all packages green (existing + 4 new tests)
go vet -tags e2e      ✓ no warnings
golangci-lint         ✓ 0 issues
gofmt -l .            ✓ 0 files
```

E2E suite (`mise run test-e2e`) requires Docker; it compiles cleanly via `go vet -tags e2e ./e2e/`.

## Stacking note

This PR is independent of #16 (`feat/build-correctness-speed-v2`) and #17 (`fix/cli-bridge-app-name-all-workspaces`). All three target `develop`; merge order doesn't matter.
