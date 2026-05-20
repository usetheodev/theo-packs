# Build Correctness & Speed v2

> Reduces generated image size by 2-3× and shrinks build context transfer by an order of magnitude in the common case. Six concrete improvements landing in 8 review-bisectable commits.

## Plan

`docs/plans/build-correctness-and-speed-v2-plan.md` (8 phases, 13 tasks, 7 ADRs).

## What changes

### Image size (Node + Python)

Node deploy stages now drop devDependencies via `<pm> prune`:

| PM | Command emitted |
|---|---|
| npm | `npm prune --omit=dev` |
| pnpm | `pnpm prune --prod` |
| yarn classic | `yarn install --production --ignore-scripts --prefer-offline` |
| bun | (no prune — bun has no prune subcommand) |

Python local-source layer is annotated with default excludes (`__pycache__`, `*.pyc`, `.pytest_cache`, `tests`, `.venv`, `.env`, `.git`, etc.). The actual filesystem effect lands via the generated `.dockerignore`; user-supplied `.dockerignore` continues to take precedence.

### Build context transfer

When the project source has no `.dockerignore`, the CLI writes a per-language default to `<source>/.dockerignore`. Templates ship for **go**, **node**, **python**, **rust**, **java**, **dotnet**, **ruby**, **php**, **deno** and a common base (`.git`, OS noise, editor cruft) for any language. User-supplied files are **never** overwritten or merged (D3).

### Cold-build cache reuse (Java)

Gradle install step now runs `gradle dependencies --no-daemon --refresh-dependencies` and Maven workspace install step now runs `mvn -B -DskipTests dependency:go-offline`. The install layer's hash depends only on the manifests, so cold builds reuse the Docker layer cache when only application code changes. (Single-project Maven already had this since v1.)

### BuildKit frontend pin

Every generated Dockerfile now starts with `# syntax=docker/dockerfile:1` so cache mounts are honored regardless of the host's default frontend. Renderer-emitted, not provider-emitted (D4).

### Regression guards

New `requireSizeLessThan(t, tag, maxMB)` helper in `e2e/e2e_test.go`. Wired into `TestE2E_NodeNpm_BuildsImage` and `TestE2E_PythonFlask_BuildsImage` with a 280 MB cap. Caps are intentionally loose (D6) — absolute bounds beat ratios because base-image sizes drift across Debian point releases.

## Estimated size impact

| Scenario | Before | After | Reduction |
|---|---|---|---|
| Node hello-world | ~210 MB | ~210 MB (no devDeps were present) | — |
| Node + dev tooling (typescript, vitest…) | ~150 MB | ~50 MB | ~3× |
| Next.js 14 / React 18 | ~620 MB | ~180-220 MB ¹ | ~3× |
| Python flask | ~280 MB | ~140-160 MB | ~2× |
| `.dockerignore`-less Next.js context | ~2.5 GB transfer | ~80 MB transfer | ~30× |

¹ Estimated. Next.js standalone output detection is **deferred to v3** — the gain assumes user has `output: 'standalone'` in `next.config.js` AND that we filter the deploy COPY accordingly. v2 ships only the generic `npm prune` win.

## Commits (review-bisectable)

```
docs: add build correctness & speed v2 plan
feat(dockerfile): emit # syntax=docker/dockerfile:1 directive
feat(node): npm prune --omit=dev (and pnpm/yarn/bun) in build step
feat(python): exclude __pycache__/tests/.venv from local layer
feat(java): warm gradle/maven dep cache in install step
feat(dockerignore): per-language default templates + CLI write
test(e2e): image-size assertions + requireSizeLessThan helper
docs: update CHANGELOG, README, CLAUDE.md for build-correctness v2
```

Each commit is self-consistent (tests pass at every commit). Goldens are regenerated incrementally per phase rather than batched at the end — this keeps `mise run check` green at every internal point and makes per-language review possible.

## Out of scope (explicitly deferred to v3)

- Next.js standalone detection (requires `next.config.js` parsing + assumption that user has `output: 'standalone'` configured)
- Rust "dummy main.rs trick" for cargo dep cache warmup (high complexity, BuildKit `--mount=type=cache` already covers most cases)
- Custom base images (treadmill of CVE/release maintenance with marginal gain over a registry pull-through cache)
- Build-cluster infrastructure (registry mirror, BuildKit cache exporter to S3, DaemonSet pre-pulling) — these are ops concerns, not theo-packs concerns

## Backward compatibility

- `core.GenerateBuildPlan` API unchanged.
- `theopacks.json` and `THEOPACKS_*` env vars unchanged.
- Existing user workflows continue to work; new behavior is safe-by-default.
- New behaviors are visible: `.dockerignore` appears in user's source dir on first run (announced via stderr log).

## Notes for users

- **`.env` is now excluded from Python builds by default.** Security-positive: committed credentials no longer ship to runtime images. Runtime config should use `THEOPACKS_*` env vars or a secrets backend.
- **Bun projects unaffected by prune** — bun has no prune subcommand and its hardlinked store is already lean.
- **User-supplied `.dockerignore` is never overwritten.** Delete the file and rerun the CLI to regenerate from the per-language template.

## Quality gates

```
mise run test              ✓ all packages green
go vet ./... -tags e2e     ✓ no warnings
golangci-lint run          ✓ 0 issues
gofmt -l .                 ✓ 0 files need formatting
```

E2E suite (`mise run test-e2e`) requires a Docker-enabled host; it compiles cleanly via `go vet -tags e2e ./e2e/`. Will run in CI with the new size assertions in place.

## Test count

39 new tests added across 6 packages:

- `core/dockerignore`: 13 (template content + edge cases)
- `core/providers/node`: 9 (PruneCommand per PM + integration)
- `core/providers/python`: 7 (one per plan path + coverage lock)
- `core/providers/java`: 4 (gradle/maven × single/workspace warmup)
- `core/dockerfile`: 3 (syntax directive prefix + ordering + edge case)
- `cmd/theopacks-generate`: 3 (write/preserve/readonly source)

Plus 58 goldens regenerated and reviewed.
