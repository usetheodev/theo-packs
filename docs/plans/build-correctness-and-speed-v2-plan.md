# Plan: Build Correctness & Speed v2

> **Version 1.0** — Reduce generated image size by 2-3× and shrink build context transfer by an order of magnitude in the common case. Six concrete improvements to the Dockerfile generation: (1) Node deploy stages prune devDependencies, (2) Python local layer excludes `__pycache__`/tests/.venv, (3) a sensible default `.dockerignore` is written when the user has none, (4) every Dockerfile starts with `# syntax=docker/dockerfile:1`, (5) Java install step actually warms the dep cache via `gradle dependencies`/`mvn dependency:go-offline`, (6) E2E suite gains image-size assertions to lock the gains. Single PR with internal commits split per concern. Out of scope: Next.js standalone, Rust dummy-main trick, custom base images, build-cluster infrastructure (deferred to v3).

## Context

### What exists today

After PR #15 merged (`feat/dockerfile-correctness-and-efficiency`), theo-packs ships 11 providers covering Go, Rust, Java, .NET, Ruby, PHP, Python, Deno, Node, static, shell. Every provider produces a multi-stage Dockerfile with cache mounts for the package manager, slim/distroless deploy images, USER non-root, and HEALTHCHECK where applicable. Quality gate is green: `mise run test`, `mise run check`, `mise run test-e2e` all passing.

A focused review of the 50 golden Dockerfiles in `core/dockerfile/testdata/` against the user's PaaS-velocity requirement surfaced six concrete gaps with measurable impact:

1. **Deploy copies `/app` whole.** `core/dockerfile/testdata/integration_node_next.dockerfile:18` does `COPY --from=build --chown=appuser:appuser /app /app`. For a Next.js app the `/app` directory contains: `node_modules` (~300-500MB, includes devDependencies), source files, the `.next/cache` directory, and any test fixtures the user keeps in-tree. The runtime image carries all of it.

2. **`core/dockerfile/testdata/integration_python_flask.dockerfile:15-17`** copies `/app` whole + `/usr/local/lib/python3.12/site-packages` + `/usr/local/bin`. The `/app` copy includes `__pycache__/`, `*.pyc`, `.pytest_cache/`, any `.venv/` the user kept locally, and `tests/`.

3. **No `.dockerignore` generation.** `core/plan/dockerignore.go:11-38` shows we *consume* a user-supplied `.dockerignore` (parses patterns, applies them as `Exclude` on the local layer), but nothing generates one when the user has none. Build context transfer ships `.git/`, `node_modules/` (if user has a local install), `target/`, `build/`, `.next/`, etc. to the build daemon. On a real-world Next.js repo this is multiple GB on the wire.

4. **No `# syntax=docker/dockerfile:1` directive.** `head -3` on every golden confirms zero have it. Modern Docker Engine + BuildKit handle cache mounts without it, but legacy builders silently drop them, and the directive is the canonical way to pin the dockerfile frontend version.

5. **Java install stage is a no-op.** `core/providers/java/gradle.go:61-76` copies the build manifests but never invokes `gradle dependencies` or any other warmup. The build step then does `COPY . .` (which preserves the install-stage manifests because the local copy doesn't replace them) and runs `gradle bootJar` from a cold dep cache. The BuildKit `--mount=type=cache,target=/root/.gradle` mitigates this on subsequent rebuilds, but cold builds in CI eat the full download time.

6. **Node workspaces example without lockfile** generates `npm install` instead of `npm ci`. This was misdiagnosed in earlier review — `core/providers/node/workspace.go:113-118` correctly chooses `npm ci` when `package-lock.json` exists. The example just lacks one. **Not in scope as a code fix; documented here for completeness.**

### Evidence — measured impact (sample sizes)

| Scenario | Current image | After deploy filter | Reduction |
|---|---|---|---|
| Next.js 14 / React 18 hello-world | ~620 MB | ~180-220 MB | ~3× |
| Express + dev deps (typescript, eslint, vitest) | ~150 MB | ~50 MB | ~3× |
| Flask + gunicorn | ~280 MB | ~140-160 MB | ~2× |
| `.dockerignore`-less Next.js context (with `.next/cache` + `node_modules`) | ~2.5 GB transfer | ~80 MB transfer | ~30× |

These numbers are estimates from comparable open builders (Heroku Node buildpack, Railway Nixpacks, Cloud Native Buildpacks). We will measure exact numbers in `e2e/e2e_test.go` and lock the targets.

### Evidence — code references

- `core/plan/plan.go:11-27` — Deploy struct already has `Inputs []Layer` with embedded `Filter`. Filter `Include`/`Exclude` are propagated to renderer in `core/dockerfile/generate.go:290-309`.
- `core/dockerfile/generate.go:404-415` — `resolveDeployPaths` handles include path semantics; needs no changes for include-only filters but does not currently emit excludes.
- `core/plan/dockerignore.go:51-74` — `DockerignoreContext` is the structural model already used to track excludes/includes; v2 reuses this exact shape.
- `cmd/theopacks-generate/main.go:118-130` — CLI is the single writer of artifacts to disk; the natural place to also write the generated `.dockerignore`.

---

## Objective

**Done = (1) `mise run test`, `mise run check`, `mise run test-e2e` are all green; (2) the existing `examples/` produce images at least 1.8× smaller than today (measured via `docker image inspect ... --format '{{.Size}}'` in new E2E assertions); (3) running `theopacks-generate` against a project without a `.dockerignore` writes a language-aware default to `<source>/.dockerignore`; (4) every generated Dockerfile starts with `# syntax=docker/dockerfile:1`; (5) Java install step calls `gradle dependencies`/`mvn dependency:go-offline`; (6) all changes ship in a single PR with one internal commit per logical concern.**

Specific, measurable goals:

1. Image-size reductions (verified by E2E assertions):
   - Node: simple hello-world + Express ≥ 2× smaller after `npm prune --omit=dev`.
   - Python: flask example ≥ 1.5× smaller after `__pycache__`/tests filter.
2. `.dockerignore` written by CLI when source has none, tailored per detected language(s); idempotent (existing user file is never overwritten).
3. `# syntax=docker/dockerfile:1` present at line 1 of every generated Dockerfile (asserted in `goldens_audit_test.go`).
4. Java install step runs `gradle dependencies --no-daemon --refresh-dependencies` (Gradle) or `mvn -B dependency:go-offline` (Maven) under the dep cache mount.
5. Backward compatibility: no breaking change to `theopacks.json`, `THEOPACKS_*` env vars, or the public `core.GenerateBuildPlan` API. Behavior changes are safe-by-default (matches Heroku/Railway conventions).
6. ≥18 affected goldens regenerated via `UPDATE_GOLDEN=true` and reviewed.

---

## ADRs

### D1 — Deploy filter via build-stage prune command, not a separate Layer field
**Decision:** Implement `npm prune --omit=dev` (and pnpm/yarn/bun analogues) as a **command appended to the existing build step**, executed AFTER the user's `npm run build`. The deploy stage continues to use the existing `Layer.Filter.Include = ["."]` semantics. We do NOT add a new "prune" stage or a new field to `Layer`/`Deploy`.

**Rationale considered:**
- *Add a new "prune" intermediate stage* — would require a new step type and a new entry in the deploy chain. Doubles the number of changes to the renderer and the test data. Not justified by the actual semantic need.
- *Add `Layer.PostBuildCommands`* — pollutes the data model with a use-case-specific field. Easy to confuse with `Step.Commands`.
- *Pure include filter* — works for Python (where we exclude file patterns from the local layer) but does NOT work for Node, where the ENTIRE `node_modules` directory is one path. We can't surgically exclude only devDependencies via Filter; we need the package manager to mutate the directory in-place via `prune`.

**Consequences:**
- Enables: minimal data-model change; reuses existing renderer; commit-bisectable.
- Constrains: the prune command runs in the build stage, so its `node_modules` mutation is reflected in the COPY-out. If a future provider needs a TRULY separate prune step (e.g., to drop the build toolchain entirely), we revisit the data model in v3.

### D2 — `.dockerignore` is a CLI-side artifact, not part of `BuildPlan`
**Decision:** `core.GenerateBuildPlan` does NOT mutate or write `.dockerignore`. Instead, `cmd/theopacks-generate/main.go` reads the detected provider name from `BuildResult.DetectedProviders[0]`, looks up the corresponding language template, and writes the file to `<source>/.dockerignore` only when none exists. The library API stays read-only against the source tree (matches the project invariant: `core/` reads, `cmd/` writes).

**Rationale considered:**
- *Add `Plan.DockerIgnore` field* — pulls a side-effect-y artifact into a serializable plan that's supposed to describe ONLY the Dockerfile. Confuses semantics.
- *Generate `.dockerignore` per-provider as a `FileCommand` in the install step* — wrong layer; the file needs to exist BEFORE Docker reads the build context, not as a step output.
- *Embed templates in the providers package* — couples each provider to a writer concern. Templates live as a flat lookup table in `core/dockerignore/templates.go` to keep providers focused on plan generation.

**Consequences:**
- Enables: CLI is the single writer; library remains pure (testable without a temp dir).
- Constrains: programs that use the library directly (without the CLI) and want a default `.dockerignore` need to call the new public function `dockerignore.DefaultFor(providerName)` themselves. A short doc comment + example will cover this.

### D3 — User-supplied `.dockerignore` is never overwritten or merged
**Decision:** When `<source>/.dockerignore` exists, we leave it untouched. We do NOT merge our defaults with user patterns. We do NOT warn or error.

**Rationale:** Merging is hairy: the `!`-prefix include semantics interact non-obviously across files, and "I customized my .dockerignore" is the user signaling explicit ownership. Silent overrides erode trust. The cost of NOT generating one is paid only by users who never had one — those users had no opinion to override.

**Consequences:**
- Enables: zero risk of accidentally breaking a user's careful exclude configuration.
- Constrains: a user with an outdated `.dockerignore` that's missing recent best practices won't get our suggestions. Mitigation: a one-time `theopacks-generate --print-dockerignore` flag in v3 lets the user diff and adopt.

### D4 — `# syntax=docker/dockerfile:1` directive is renderer-emitted, not provider-emitted
**Decision:** `core/dockerfile/generate.go` writes `# syntax=docker/dockerfile:1` as the first line of every produced Dockerfile string, before any `FROM`. Providers contribute nothing to this — it's a rendering concern.

**Rationale:** The directive is universal across all providers and has nothing to do with language semantics. Putting it in the renderer keeps providers decoupled.

**Consequences:**
- Enables: every Dockerfile gets the directive; one-line change in the renderer; the only test surface is the renderer's existing tests.
- Constrains: if a future use case requires a Dockerfile WITHOUT the directive (e.g., for a legacy registry that doesn't support BuildKit frontend negotiation), we'd add a `Generate(plan, opts)` overload. Not foreseen.

### D5 — Java install warmup uses `--no-daemon --refresh-dependencies` for Gradle, `dependency:go-offline` for Maven
**Decision:** Add `gradle dependencies --no-daemon --refresh-dependencies` to the Gradle install step and `mvn -B dependency:go-offline` to the Maven install step. Both run under the existing `--mount=type=cache,target=/root/.gradle` (or `/root/.m2`) mount.

**Rationale:**
- `gradle dependencies` walks the configurations and downloads every artifact into the cache. `--refresh-dependencies` invalidates cached metadata for SNAPSHOTs (necessary for some workflows). `--no-daemon` matches the existing build step's flag.
- `mvn dependency:go-offline` is the canonical Maven recipe for the same goal: download every needed artifact into the local repository so subsequent commands need zero network.
- Spring Boot needs nothing different — `bootJar` reuses the same cache.

**Consequences:**
- Enables: cold builds reuse the Docker layer cache for deps even when the source code changes (the install step's hash depends only on the manifests).
- Constrains: install step runtime grows by the dep download time. Fine because that time is paid once per dep change, not per build.

### D6 — E2E size assertions use multiplicative factors, not absolute byte counts
**Decision:** New E2E helpers assert image size relative to a baseline measured at the start of the test suite, e.g. `require.Less(t, sizeAfter, sizeBefore/2)`. We do NOT hardcode "image must be ≤ 200MB" because base image sizes drift over time (Debian point releases, Node minor versions).

**Rationale considered:**
- *Hardcoded byte caps* — break on every Debian/Node minor version that ships a new manifest.
- *Snapshot-based (record current size, fail if grows by N%)* — needs a stored baseline, which means another file in the repo to maintain.
- *Multiplicative comparison* — robust: we measure twice (with current providers + with v2 providers), assert the ratio. Concrete and meaningful.

**Consequences:**
- Enables: low-maintenance assertions; future improvements automatically tighten the bound.
- Constrains: needs a way to run "old code" and "new code" in the same test invocation, OR (simpler) we measure the v2 image and assert against a known absolute upper bound that's loose enough to not flake on Debian updates (e.g., `require.Less(t, size, int64(300*MB))` for Node hello-world). We adopt the loose absolute bound — simpler, sufficient.

### D7 — Single PR, one commit per logical concern
**Decision:** All changes land in a single PR (`feat/build-correctness-speed-v2`). Internal commit split:
1. `feat(plan,dockerfile): emit # syntax=docker/dockerfile:1 directive`
2. `feat(node): npm prune --omit=dev (and pnpm/yarn/bun) in build step`
3. `feat(python): exclude __pycache__/tests/.venv from local layer`
4. `feat(java): warm gradle/maven dep cache in install step`
5. `feat(dockerignore): per-language default templates`
6. `feat(cli): write default .dockerignore when source has none`
7. `test(e2e): image-size assertions + new requireSizeLessThan helper`
8. `test(integration): regenerate goldens for all affected providers`
9. `docs: CHANGELOG, README, CLAUDE.md updates`

**Rationale:** matches the v1 PR pattern. Each commit can be reverted independently if a regression surfaces post-merge.

**Consequences:**
- Enables: bisect by concern; selective revert.
- Constrains: PR is large (~2000-3000 net lines including goldens). Description must be thorough.

---

## Dependency Graph

```
Phase 0 (Foundation: # syntax directive in renderer)
    │
    ├──▶ Phase 1 (Node prune)         ─┐
    ├──▶ Phase 2 (Python excludes)    ─┤
    ├──▶ Phase 3 (Java warmup)        ─┼──▶ Phase 5 (Goldens regen)
    │                                   │           │
    └──▶ Phase 4 (.dockerignore)      ─┘           │
                  │                                 ▼
                  └────────────────▶ Phase 6 (E2E size assertions)
                                                    │
                                                    ▼
                                          Phase 7 (Docs + final QA)
```

- **Phase 0** is a hard prerequisite for ALL goldens (they all change because of the `# syntax` line).
- **Phases 1-4** are mutually independent and can be implemented/committed in any order; serial commit order chosen for review readability (D7).
- **Phase 5** depends on all of 0-4 (regenerate goldens once, in one commit).
- **Phase 6** depends on Phase 5 (asserting against final goldens).
- **Phase 7** is pure docs and runs last.

---

## Phase 0: Foundation — `# syntax=docker/dockerfile:1` directive

**Objective:** Land the cheapest improvement first — one line in the renderer, but it touches every golden, so it must come before the language-specific phases to avoid noisy diffs in those commits.

### T0.1 — Emit `# syntax=docker/dockerfile:1` at the top of every generated Dockerfile

#### Objective
Modify `core/dockerfile/generate.go` so the very first thing written to the output is `# syntax=docker/dockerfile:1\n` followed by an empty line, before any `FROM`.

#### Evidence
- `core/dockerfile/testdata/integration_*.dockerfile` — confirmed via `head -3` that none have it.
- Dockerfile reference (https://docs.docker.com/build/buildkit/frontend/) recommends always setting the directive.

#### Files to edit
```
core/dockerfile/generate.go        — INSERT directive write at top of Generate()
core/dockerfile/generate_test.go   — UPDATE existing tests that assert raw output
core/dockerfile/testdata/*.dockerfile — REGENERATED in Phase 5 (do NOT touch in this commit)
```

#### Deep file dependency analysis
- **`core/dockerfile/generate.go`** — `Generate(p *plan.BuildPlan)` is the single entry point. Adding a write at the top of the body is a 2-line addition. No new public API.
- **`core/dockerfile/generate_test.go`** — non-golden tests (`TestGenerate_GoSimple`, etc., lines 1-50) assert specific output. They'll need to either prefix expectations with the directive OR be made directive-agnostic via `strings.Contains`. We choose `strings.HasPrefix(out, "# syntax=docker/dockerfile:1\n\n")` for one new test and update the rest minimally.
- **Goldens** — every `integration_*.dockerfile` and `*.dockerfile` golden gains a 2-line prefix. Defer regeneration to Phase 5 so the diff is reviewed against ALL changes at once.

#### Deep Dives

**Exact directive form:**
```dockerfile
# syntax=docker/dockerfile:1

FROM ...
```
- The version `1` resolves to the latest stable dockerfile frontend at build time. Pinning to `1.7` (the version with `--mount=type=cache` stabilized) is overly conservative — `1` is the recommended form per Docker docs.
- Empty line after the directive is required by some BuildKit parsers when the next line is a comment-prefixed line. We add it unconditionally.

**Invariants:**
- After T0.1, every Dockerfile produced by `dockerfile.Generate` starts with the exact bytes `"# syntax=docker/dockerfile:1\n\n"`.
- The `Generate` function never returns this prefix without a non-empty plan body (it errors if `len(p.Steps) == 0`, unchanged).

**Edge cases:**
- `TestGenerate_NilPlan` — already errors before reaching the prefix write. Unchanged.
- `TestGenerate_EmptyPlan` — same.

#### Tasks
1. Edit `core/dockerfile/generate.go:15-35` to write `"# syntax=docker/dockerfile:1\n\n"` to `b` immediately after the `if p == nil` check.
2. Add a unit test `TestGenerate_SyntaxDirective` that asserts the prefix.
3. Audit existing non-golden generate tests; update those that compare full strings.
4. Run `mise run test ./core/dockerfile/...` — golden tests will fail. Mark `Phase 5` as the regeneration commit. Do NOT regenerate now.

#### TDD
```
RED:    TestGenerate_SyntaxDirective                     — assert HasPrefix("# syntax=docker/dockerfile:1\n\n")
RED:    TestGenerate_SyntaxDirectiveBeforeFROM           — assert directive line index < first FROM line index
RED:    TestGenerate_SyntaxDirective_OnNonEmptyPlanOnly  — empty plan still errors, doesn't emit directive alone
GREEN:  Add the 2-line write at the top of Generate()
REFACTOR: None expected.
VERIFY: mise run test ./core/dockerfile/... -run TestGenerate_Syntax
```

#### Acceptance Criteria
- [ ] `TestGenerate_SyntaxDirective` passes.
- [ ] All non-golden tests in `generate_test.go` pass.
- [ ] Golden tests fail (expected — they regenerate in Phase 5).
- [ ] No new `go vet` warnings.
- [ ] File length: `generate.go` stays under 600 lines.

#### DoD
- [ ] T0.1 implemented and the 3 new unit tests pass.
- [ ] Commit message: `feat(dockerfile): emit # syntax=docker/dockerfile:1 directive`.
- [ ] Goldens deliberately not regenerated (deferred to Phase 5).

---

## Phase 1: Node Deploy Prune

**Objective:** Drop devDependencies from Node deploy stages by appending a package-manager-specific prune command to the build step.

### T1.1 — Append `npm prune --omit=dev` (and PM analogues) after build script runs

#### Objective
Modify `core/providers/node/node.go` so that after the build step's `<pm> run build` (or skipped when no build script), an additional shell command runs the appropriate prune. The prune mutates `node_modules/` in place; the existing deploy filter (`Include: ["."]`) then carries the slimmed `node_modules` to the runtime stage.

#### Evidence
- `core/providers/node/node.go:87-100` — current `buildStep` ends after `workspaceBuildCommand`. No prune.
- `integration_node_next.dockerfile:18` — entire `/app` (with full `node_modules`) is COPYed to the slim runtime image. Image size is dominated by node_modules with all devDependencies.
- npm docs: `npm prune --omit=dev` is the official idempotent way to drop devDependencies from an existing install. Equivalent: pnpm `prune --prod`, yarn classic `--production` (deprecated), bun has no built-in prune (skip — see edge cases).

#### Files to edit
```
core/providers/node/node.go              — add prune after build cmd
core/providers/node/workspace.go         — add PruneCommand(pm) → string
core/providers/node/node_test.go         — unit tests for the new path
core/providers/node/workspace_test.go    — unit test PruneCommand for each PM
core/dockerfile/testdata/integration_node_*.dockerfile — REGEN in Phase 5
```

#### Deep file dependency analysis
- **`core/providers/node/node.go`** — single function `Plan` builds the install/build/deploy chain. Adding 3-5 lines after the `workspaceBuildCommand` block.
- **`core/providers/node/workspace.go`** — already has `InstallCommand(pm, hasLockfile)`. Add a parallel `PruneCommand(pm) string`.
- **`workspace_test.go:138-146`** — table-driven tests for `InstallCommand`. Add a parallel block for `PruneCommand`.
- **Goldens** — every `integration_node_*.dockerfile` gains a new `RUN sh -c '<prune>'` line in the build stage.

#### Deep Dives

**`PruneCommand` outputs:**
```go
PackageManagerNpm  → "npm prune --omit=dev"
PackageManagerPnpm → "pnpm prune --prod"
PackageManagerYarn → "yarn install --production --ignore-scripts --prefer-offline"
PackageManagerBun  → ""  // bun has no prune; skipped (see edge cases)
```

For yarn classic, `--production` reinstalls without dev deps (yarn doesn't have a true prune). The `--ignore-scripts --prefer-offline` flags keep this fast and safe (no network) given the cache is warm from the install step.

**Where the prune lands in the build step:**
```
COPY . .
RUN --mount=type=cache,... <pm> run build       (existing — only when hasBuildScript)
RUN --mount=type=cache,... <pm> prune --omit=dev (NEW — always when not bun)
```

The prune runs unconditionally (if a prune command exists for the PM), even when the user has no build script. This is correct: dev-only deps shouldn't ship even if the project has no build step.

**Cache mounts on the prune RUN:**
- npm prune: needs `/root/.npm` (cache mount carries through).
- pnpm prune: needs `/root/.local/share/pnpm/store` (carries through).
- yarn `install --production`: needs `/usr/local/share/.cache/yarn` (carries through).
- We reuse `addNodeCacheMounts(buildStep, pm)` which is already called at line 90.

**Invariants:**
- After T1.1, generated Node Dockerfiles have a prune line in the build stage UNLESS the package manager is bun.
- The line uses `plan.NewExecShellCommand` (CommandKindShell) so the renderer wraps it in `sh -c`.
- Workspaces: prune at the workspace root removes dev deps from the entire `node_modules` tree. No per-app concerns.

**Edge cases:**
- **Bun:** no built-in prune. We skip emitting any prune line. Bun installs are already lean (links from a global store, not duplicated per package). Document in `node.go` comment.
- **`npm install` (no lockfile):** the `npm install` install step still installs everything (incl. devDeps). The prune line still runs and drops them. Correct.
- **Workspaces with `peerDependencies`:** prune respects the dependency graph; peer deps shared between dev-only and prod packages are kept correctly by npm/pnpm. Yarn classic `--production` is more conservative; documented in the help text.
- **Project that intentionally needs devDependencies at runtime** (e.g., a build-server use case) — escape hatch via `theopacks.json` `deploy.startCommand` or by setting `THEOPACKS_NODE_SKIP_PRUNE=1`. Adding the env var is OUT OF SCOPE for v2; the workaround is to set a custom `theopacks.json` that overrides the build step.

#### Tasks
1. Add `PruneCommand(pm PackageManager) string` to `workspace.go`. Returns "" for bun.
2. In `node.go:Plan`, after the `workspaceBuildCommand` block, if `PruneCommand(pm) != ""`, append a new `plan.NewExecShellCommand(pruneCmd)` to `buildStep`.
3. Add table-driven test `TestPruneCommand_PerPM` in `workspace_test.go`.
4. Add `TestNodeProvider_BuildStepHasPrune` in `node_test.go` (verify the build step's last command is the prune for npm/pnpm/yarn).
5. Add `TestNodeProvider_BunBuildStepHasNoPrune` (sanity check the bun branch).
6. Defer golden regen to Phase 5.

#### TDD
```
RED:    TestPruneCommand_Npm                  — npm → "npm prune --omit=dev"
RED:    TestPruneCommand_Pnpm                 — pnpm → "pnpm prune --prod"
RED:    TestPruneCommand_Yarn                 — yarn → "yarn install --production --ignore-scripts --prefer-offline"
RED:    TestPruneCommand_Bun                  — bun → ""
RED:    TestNodeProvider_BuildStepHasPrune    — build step's last cmd ≡ PruneCommand(pm)
RED:    TestNodeProvider_BunBuildStepHasNoPrune — bun build step has no prune line
RED:    TestNodeProvider_PruneRunsEvenWithoutBuildScript — projects without scripts.build still prune
GREEN:  Implement PruneCommand + Plan() append
REFACTOR: None expected.
VERIFY: mise run test ./core/providers/node/...
```

#### Acceptance Criteria
- [ ] All 7 new tests pass.
- [ ] No regression in existing `node_test.go` / `workspace_test.go`.
- [ ] `mise run check` clean.
- [ ] `core/providers/node/node.go` ≤ 350 lines.

#### DoD
- [ ] T1.1 implemented.
- [ ] Commit message: `feat(node): npm prune --omit=dev (and pnpm/yarn/bun) in build step`.

---

## Phase 2: Python Local Layer Excludes

**Objective:** Prevent Python deploy stages from carrying `__pycache__/`, `*.pyc`, `.pytest_cache/`, `tests/`, `.venv/` from the build context.

### T2.1 — Apply default excludes to Python local layer

#### Objective
Modify `core/providers/python/python.go` so that `ctx.NewLocalLayer()` (used in every plan path: `planRequirements`, `planPoetry`, `planPipfile`, `planPyproject`, `planSetupPy`, `planUvWorkspace`) has `Filter.Exclude` populated with the Python-specific patterns BEFORE being added to the build step.

#### Evidence
- `core/providers/python/python.go:87-88` — `buildStep.AddInput(ctx.NewLocalLayer())` adds the local layer with no excludes today.
- `core/dockerfile/testdata/integration_python_flask.dockerfile:15` — `COPY --from=build --chown=appuser:appuser /app /app` carries everything build copied.
- The renderer `core/dockerfile/generate.go:264-285` reads `Layer.Include` but currently ignores `Exclude` for non-deploy layers (this needs verification — see deep analysis).

#### Files to edit
```
core/providers/python/python.go            — wrap NewLocalLayer with default excludes
core/providers/python/python_test.go       — assert excludes are applied
core/dockerfile/generate.go                — emit COPY with --exclude flag (BuildKit) IF needed
core/dockerfile/generate_test.go           — test exclude rendering
core/dockerfile/testdata/integration_python_*.dockerfile — REGEN Phase 5
```

#### Deep file dependency analysis
- **`core/providers/python/python.go`** — every `planXxx` calls `ctx.NewLocalLayer()`. We add a small helper `pythonLocalLayer()` that returns `NewLocalLayer()` with `Filter.Exclude = pythonDefaultExcludes()`.
- **`core/dockerfile/generate.go`** — the renderer's handling of `Layer.Exclude` for the LOCAL `COPY . .` line is the unknown. Today line 266 emits a bare `COPY . .` — we need to extend it to honor excludes via either:
  - (a) `.dockerignore`-style preprocessing (out of scope for this phase; that's Phase 4)
  - (b) BuildKit's `COPY --exclude=<pattern>` (available since dockerfile syntax 1.7+, requires `# syntax=docker/dockerfile:1.7-labs` directive)
  - (c) A `.theopacks-ignore` file written next to the Dockerfile (custom, no good)
  
  **Decision:** use (a) — append the Python excludes to the generated `.dockerignore` (Phase 4 dependency). This keeps the renderer simple and uses Docker's native filtering. Phase 2 sets `Filter.Exclude` on the local layer; Phase 4 reads those excludes per-language and merges into the generated `.dockerignore`.

  Wait — that creates a Phase 2→Phase 4 dependency. Let me reconsider.

  **Revised decision:** Phase 2 ONLY annotates the layer with the exclude patterns (correct in the data model). The actual EFFECT happens in Phase 4 via the generated `.dockerignore`. Phase 2 alone changes nothing in the rendered Dockerfile output (no golden diff). Phase 4 is what makes it work end-to-end.

  This means Phase 2's golden tests SHOULD NOT show a difference. The unit tests still verify `Filter.Exclude` is populated.

- **`core/providers/python/python_test.go`** — new unit test assertions on the BuildPlan's local layer Exclude field.

#### Deep Dives

**`pythonDefaultExcludes` patterns:**
```go
[]string{
    "__pycache__",       // Python bytecode caches (recursive — gitignore-style match)
    "*.pyc",             // individual .pyc files
    "*.pyo",             // optimized
    ".pytest_cache",     // pytest run state
    ".mypy_cache",       // mypy run state
    ".ruff_cache",       // ruff run state
    "tests",             // common test dir name
    "test",              // alternate
    ".venv",             // user's local virtualenv
    "venv",              // alternate name
    ".tox",              // tox cache
    ".coverage",         // coverage report data
    ".env",              // local env files (also a security win)
    ".git",              // version control
}
```

These are all "user has it locally but doesn't need it in the image" patterns. For projects that legitimately ship a `tests/` directory at runtime (rare — the user is supposed to put runtime tests in a different place), the user adds an explicit `.dockerignore` to override (D3).

**Why these patterns are safe defaults:**
- `__pycache__/`, `*.pyc`, `*.pyo`: regenerated at runtime by the Python interpreter, so dropping them costs nothing.
- `.pytest_cache/`, `.mypy_cache/`, `.ruff_cache/`, `.tox/`: tooling artifacts.
- `tests/`, `test/`: production code shouldn't import from these. If it does, the import will fail at runtime — better than silently bloating the image.
- `.venv/`, `venv/`: runtime should NEVER use a build-time venv (we install into `/usr/local/lib/pythonX.Y/site-packages` instead). If user committed `.venv` to disk, dropping it is a big win.
- `.env`: security — accidentally committed credentials don't ship to the runtime image.
- `.git`: never relevant to the runtime; can be 100s of MB.

**Invariants:**
- After T2.1, every Python plan's `buildStep.Inputs` has a local layer with `Filter.Exclude = pythonDefaultExcludes()`.
- Order of patterns matches `pythonDefaultExcludes()` exactly (deterministic).
- The pure data-model change has zero golden impact (verified by no diff in Phase 5 from this commit alone).

**Edge cases:**
- User has a runtime-needed file at `tests/data.json`: dropped. Workaround: user writes `!tests/data.json` in `.dockerignore`. Document in CHANGELOG.
- User has `.env` file they need at runtime: dropped. SECURITY-positive default. Document in CHANGELOG that runtime config goes through `THEOPACKS_*` env vars.

#### Tasks
1. Add `pythonDefaultExcludes() []string` returning the canonical pattern list.
2. Add `pythonLocalLayer() plan.Layer` returning `plan.NewLocalLayer()` with `.Filter.Exclude = pythonDefaultExcludes()`.
3. Replace every `ctx.NewLocalLayer()` call in `python.go` with `pythonLocalLayer()`.
4. Add `TestPythonProvider_LocalLayerHasExcludes` and `TestPythonProvider_PoetryLocalLayerHasExcludes`, etc. (one per plan path — 6 tests).
5. Add `TestPythonDefaultExcludes_Coverage` enumerating the patterns to lock the list.

#### TDD
```
RED:    TestPythonDefaultExcludes_Coverage             — list contains 14 patterns, in stable order
RED:    TestPythonProvider_RequirementsLocalLayerHasExcludes
RED:    TestPythonProvider_PoetryLocalLayerHasExcludes
RED:    TestPythonProvider_PipfileLocalLayerHasExcludes
RED:    TestPythonProvider_PyprojectLocalLayerHasExcludes
RED:    TestPythonProvider_SetupPyLocalLayerHasExcludes
RED:    TestPythonProvider_UvWorkspaceLocalLayerHasExcludes
GREEN:  Implement helpers + replace NewLocalLayer calls
REFACTOR: None expected.
VERIFY: mise run test ./core/providers/python/...
```

#### Acceptance Criteria
- [ ] All 7 new unit tests pass.
- [ ] No regression in existing python tests.
- [ ] `mise run check` clean.

#### DoD
- [ ] T2.1 implemented.
- [ ] Commit message: `feat(python): exclude __pycache__/tests/.venv from local layer`.

---

## Phase 3: Java Install Step Warmup

**Objective:** Make the Java install step actually warm the dep cache via `gradle dependencies` (Gradle) or `mvn dependency:go-offline` (Maven), so cold builds can reuse the Docker layer cache after manifest changes.

### T3.1 — Add `gradle dependencies` to Gradle install step

#### Objective
Append a `gradle dependencies --no-daemon --refresh-dependencies` call to the install step in `core/providers/java/gradle.go:planGradle` and `planGradleWorkspace`.

#### Evidence
- `core/providers/java/gradle.go:61-76` — install step copies manifests but never invokes Gradle. The cache mount is attached but unused at this stage.

#### Files to edit
```
core/providers/java/gradle.go        — add dependencies call after manifest COPYs
core/providers/java/gradle_test.go   — assert install step has the call
core/dockerfile/testdata/integration_java_*.dockerfile — REGEN Phase 5
```

#### Deep file dependency analysis
- **`gradle.go:planGradle`** — install step ends after manifest COPYs (line 76). Add `installStep.AddCommand(plan.NewExecShellCommand("gradle dependencies --no-daemon --refresh-dependencies"))`.
- **`gradle.go:planGradleWorkspace`** — same pattern (line 168 region).

#### Deep Dives

**Why `--refresh-dependencies`:** SNAPSHOT dependencies have an internal cache TTL (default 24h). The flag forces re-resolution, which is what we want at the dep-warming stage. Release dependencies are unaffected.

**Why `--no-daemon`:** matches the existing build step. The Gradle daemon isn't useful in a one-shot Docker build.

**Failure mode:** if the `gradle dependencies` call fails (e.g., a bad manifest), the install step fails — better than the current behavior where the failure surfaces later in the build step. Net win for error visibility.

**Invariants:**
- After T3.1, the Gradle install step's last command is the dependencies-warming call.
- The build step is unchanged (still runs `gradle bootJar` or `gradle build`).
- Both single-project and workspace flows get the warming call.

**Edge cases:**
- A `build.gradle.kts` that requires source files to evaluate (rare — typically only when a custom plugin reads the source tree at configuration time): `gradle dependencies` may fail with a confusing error. Workaround: user disables the dep warmup via `theopacks.json deploy.skipDependencyWarmup` (out of scope; documented in the `StartCommandHelp` text).

#### Tasks
1. In `gradle.go:planGradle` after the wrapper COPYs, append the dependencies command.
2. In `gradle.go:planGradleWorkspace` after the manifest COPYs, append the same.
3. Add `TestPlanGradle_InstallWarmsDeps` and `TestPlanGradleWorkspace_InstallWarmsDeps`.

#### TDD
```
RED:    TestPlanGradle_InstallWarmsDeps           — install step's last command is "gradle dependencies --no-daemon --refresh-dependencies"
RED:    TestPlanGradleWorkspace_InstallWarmsDeps  — same for workspace flow
GREEN:  Add the AddCommand calls
REFACTOR: None expected.
VERIFY: mise run test ./core/providers/java/...
```

#### Acceptance Criteria
- [ ] Both new tests pass.
- [ ] Existing java tests pass.

#### DoD
- [ ] T3.1 implemented.

---

### T3.2 — Add `mvn dependency:go-offline` to Maven install step

#### Objective
Symmetric change to T3.1 for `core/providers/java/maven.go`.

#### Evidence
- `core/providers/java/maven.go` — single-project + multi-module (workspace) flows both have an install stage that copies manifests without warming.

#### Files to edit
```
core/providers/java/maven.go         — add go-offline call
core/providers/java/maven_test.go    — assertions
```

#### Deep Dives

**Command:** `mvn -B dependency:go-offline`. The `-B` (batch mode) suppresses ANSI codes which is conventional for CI invocations.

**Why `dependency:go-offline` and not just `mvn install`:** `dependency:go-offline` resolves all artifacts but doesn't compile, doesn't run plugins, doesn't deploy. It's the canonical "warm the local repo" recipe.

#### Tasks
1. Append `mvn -B dependency:go-offline` to install step in single-project and multi-module flows.
2. Add `TestPlanMaven_InstallWarmsDeps` and `TestPlanMavenWorkspace_InstallWarmsDeps`.

#### TDD
```
RED:    TestPlanMaven_InstallWarmsDeps           — install step's last cmd is "mvn -B dependency:go-offline"
RED:    TestPlanMavenWorkspace_InstallWarmsDeps  — same for workspace flow
GREEN:  Add AddCommand calls
REFACTOR: None.
VERIFY: mise run test ./core/providers/java/...
```

#### Acceptance Criteria
- [ ] Both tests pass.
- [ ] No regressions.

#### DoD
- [ ] T3.2 implemented.
- [ ] Combined commit (T3.1 + T3.2) message: `feat(java): warm gradle/maven dep cache in install step`.

---

## Phase 4: `.dockerignore` Default Generation

**Objective:** Generate a sensible per-language `.dockerignore` when the user has none, written to `<source>/.dockerignore` by the CLI.

### T4.1 — Build the language template registry

#### Objective
Create `core/dockerignore/templates.go` with per-language `.dockerignore` template strings and a public `DefaultFor(providerName string) string` function.

#### Evidence
- `core/plan/dockerignore.go:11-38` — we already PARSE `.dockerignore`. Need a sibling concept for "what would a sane default look like".
- Existing analogous projects (Heroku Node buildpack, Railway Nixpacks, Cloud Native Buildpacks) all ship per-language defaults.

#### Files to edit
```
core/dockerignore/                       (NEW PACKAGE)
core/dockerignore/templates.go           (NEW) — DefaultFor(name) string
core/dockerignore/templates_test.go      (NEW)
```

#### Deep file dependency analysis
- **`core/dockerignore/templates.go`** — pure-data file: a `map[string]string` keyed by provider name. Public function `DefaultFor(name string) string` returns the template or empty string. No external deps; testable trivially.
- The new package lives parallel to `core/plan` rather than inside it because it's a writer concern, not a plan concern.

#### Deep Dives

**Templates per language (each gets its own const string):**

```go
const baseCommon = `# Common — version control, OS noise, editor cruft.
.git/
.gitignore
.svn/
.hg/
.DS_Store
Thumbs.db
*.swp
*~
.idea/
.vscode/

# theo-packs internals
theopacks.json
.dockerignore
`

const goAdditions = `
# Go
*.test
*.out
vendor/
`

const nodeAdditions = `
# Node.js
node_modules/
npm-debug.log*
yarn-debug.log*
yarn-error.log*
.next/cache/
.turbo/
dist/
build/
coverage/
.nyc_output/
`

const pythonAdditions = `
# Python
__pycache__/
*.pyc
*.pyo
*.pyd
*.egg-info/
.pytest_cache/
.mypy_cache/
.ruff_cache/
.tox/
.coverage
htmlcov/
.venv/
venv/
ENV/
env/
.env
.env.*
!.env.example
build/
dist/
`

const rustAdditions = `
# Rust
target/
Cargo.lock.bak
`

const javaAdditions = `
# Java
target/
build/
.gradle/
.classpath
.project
.settings/
*.class
*.jar
*.war
hs_err_pid*
`

const dotnetAdditions = `
# .NET
bin/
obj/
*.user
*.suo
.vs/
publish/
`

const rubyAdditions = `
# Ruby
.bundle/
vendor/bundle/
log/
tmp/
*.gem
.byebug_history
`

const phpAdditions = `
# PHP
vendor/
.phpunit.result.cache
.phpcs-cache
`

const denoAdditions = `
# Deno
.deno/
deno.lock.bak
`
```

`DefaultFor(name)` concatenates `baseCommon` + the per-language additions.

**Invariants:**
- `DefaultFor("")` returns `baseCommon` (still useful — covers `.git`, OS noise).
- `DefaultFor("unknown-provider")` returns `baseCommon`.
- `DefaultFor("node")` returns `baseCommon + nodeAdditions`.
- All template strings end with a single trailing newline (no double-blank).

**Edge cases:**
- `node-with-static-frontend`: handled because the base + node already covers `.next/cache`, `dist`, `build`.
- `python-with-jupyter-checkpoints`: not covered explicitly (`*.ipynb_checkpoints/`). Considered but cut for v2 scope; users add it manually.

#### Tasks
1. Create `core/dockerignore/templates.go` with the constants + `DefaultFor`.
2. Create `core/dockerignore/templates_test.go` with table-driven tests for every provider name.

#### TDD
```
RED:    TestDefaultFor_Common      — DefaultFor("") returns the base (contains ".git/" line)
RED:    TestDefaultFor_Node        — contains both base AND "node_modules/"
RED:    TestDefaultFor_Python      — contains "__pycache__/"
RED:    TestDefaultFor_Java        — contains "target/" and ".gradle/"
RED:    TestDefaultFor_Rust        — contains "target/" (Rust)
RED:    TestDefaultFor_Dotnet      — contains "bin/" and "obj/"
RED:    TestDefaultFor_Ruby        — contains ".bundle/"
RED:    TestDefaultFor_Php         — contains "vendor/"
RED:    TestDefaultFor_Deno        — contains ".deno/"
RED:    TestDefaultFor_Unknown     — unknown name returns base only
RED:    TestDefaultFor_TrailingNewline — output ends with exactly one "\n"
GREEN:  Implement constants + DefaultFor
REFACTOR: None.
VERIFY: mise run test ./core/dockerignore/...
```

#### Acceptance Criteria
- [ ] All 11 tests pass.
- [ ] `core/dockerignore/templates.go` ≤ 200 lines (templates dominate).

#### DoD
- [ ] T4.1 implemented.

---

### T4.2 — CLI writes `.dockerignore` when source has none

#### Objective
Modify `cmd/theopacks-generate/main.go` to write `<source>/.dockerignore` (or `<analyzeDir>/.dockerignore` when workspace mode is active) using `dockerignore.DefaultFor(detectedProvider)`, ONLY when no such file exists.

#### Evidence
- `cmd/theopacks-generate/main.go:60-73` — already does the analogous "user-supplied takes precedence" check for Dockerfile. Same pattern applies to `.dockerignore`.

#### Files to edit
```
cmd/theopacks-generate/main.go          — write default .dockerignore
cmd/theopacks-generate/main_test.go     (NEW or EXTEND existing) — integration test
```

#### Deep file dependency analysis
- **`main.go`** — after `core.GenerateBuildPlan` returns, before writing the Dockerfile, check for `<analyzeDir>/.dockerignore`. If absent, write the default.
- Returns the source dir (analyzeDir) — the place Docker reads from when running `docker build <ctx>`.

#### Deep Dives

**Write semantics:**
- Path: `filepath.Join(analyzeDir, ".dockerignore")`.
- Mode: 0644.
- Content: `dockerignore.DefaultFor(result.DetectedProviders[0])`.
- Skip when `os.Stat(path)` returns no error (file already exists).
- On stat error other than `os.IsNotExist`, abort with logged warning (don't overwrite a file we can't read).

**Logging:**
- Skipped: stderr `"[theopacks] User-provided .dockerignore found at <path> — skipping default generation"`.
- Written: stderr `"[theopacks] Wrote default .dockerignore for provider <name> to <path>"`.

**Invariants:**
- The write happens BEFORE the Dockerfile is written (so a build that picks up both has the ignore in effect).
- Idempotent: a second invocation with the file already there is a no-op.
- Detected provider must be non-empty to write — if `result.DetectedProviders` is empty (shouldn't happen when `result.Success`), skip.

**Edge cases:**
- Workspace mode: `analyzeDir` is the workspace root. We write to the root, which is what Docker expects.
- Source is read-only (CI ephemeral mount): write fails. Log and continue (don't abort the whole CLI invocation — Dockerfile writing still happens).

#### Tasks
1. Add the write-default-dockerignore block in `main.go` between provider detection and Dockerfile generation.
2. Add a CLI integration test (or unit test for the helper) that uses a `t.TempDir()` and asserts file presence.

#### TDD
```
RED:    TestCLI_WritesDockerignore_WhenAbsent  — temp dir without .dockerignore → CLI writes a Node template
RED:    TestCLI_PreservesUserDockerignore       — temp dir WITH .dockerignore (custom content) → CLI does NOT overwrite
RED:    TestCLI_HandlesReadonlySourceGracefully — temp dir made read-only → CLI logs but doesn't abort
GREEN:  Add the block in main.go
REFACTOR: Extract `writeDefaultDockerignore(dir, providerName) error` helper for testability.
VERIFY: go test -tags e2e ./cmd/... or whatever wraps the CLI test
```

#### Acceptance Criteria
- [ ] All 3 tests pass.
- [ ] No regression in existing cmd tests.

#### DoD
- [ ] T4.2 implemented.
- [ ] Combined commit (T4.1 + T4.2) message: `feat(dockerignore): per-language default templates + CLI write`.

---

## Phase 5: Goldens Regeneration

**Objective:** With Phases 0-4 implemented, regenerate every affected golden via `UPDATE_GOLDEN=true` and review the diff.

### T5.1 — Bulk regenerate goldens and audit the diff

#### Objective
Run `UPDATE_GOLDEN=true mise run test ./core/dockerfile/...`, inspect every changed file, and commit the bulk diff in a single review-friendly commit.

#### Evidence
- Phases 0-4 each ship code changes that affect the rendered output.
- Goldens in `core/dockerfile/testdata/` count: 50 today.

#### Files to edit
```
core/dockerfile/testdata/integration_*.dockerfile  — REGENERATED
core/dockerfile/testdata/*.dockerfile              — REGENERATED (the smaller fixture set too)
```

#### Deep file dependency analysis
Affected goldens by phase (estimated from inspection):
- **Phase 0 (# syntax):** ALL 50 goldens — 2-line prefix added.
- **Phase 1 (Node prune):** all `integration_node_*.dockerfile` (~14 files) — new RUN line in build stage.
- **Phase 2 (Python excludes):** zero goldens (data-only change; effect happens via Phase 4's `.dockerignore`).
- **Phase 3 (Java warmup):** all `integration_java_*.dockerfile` (~3 files) — new install step command.
- **Phase 4 (`.dockerignore`):** zero goldens (CLI-side artifact, not in the Dockerfile output).

#### Deep Dives

**Review checklist per regenerated golden:**
1. First two lines: `# syntax=docker/dockerfile:1\n\n`.
2. Node goldens: build stage ends with `RUN ... <pm> prune ...`.
3. Java goldens: install step ends with `RUN ... gradle dependencies ...` or `RUN ... mvn -B dependency:go-offline`.
4. No unexpected diff in unaffected goldens (Go, Rust, Ruby, PHP, .NET, Deno, static, shell).

**Audit script (one-shot):**
```bash
git diff core/dockerfile/testdata/ | grep "^+" | grep -v "^+++" | sort | uniq -c | sort -rn | head -20
```
Top changes should be: `+# syntax=docker/dockerfile:1` (50×), `+RUN ... prune ...` (~14×), `+RUN ... dependencies ...` (~3×).

#### Tasks
1. Run `UPDATE_GOLDEN=true mise run test ./core/dockerfile/...`.
2. `git diff core/dockerfile/testdata/` — visual review.
3. Run the audit one-liner.
4. Commit goldens.

#### TDD
- N/A. This is a verification phase.

#### Acceptance Criteria
- [ ] `mise run test ./core/dockerfile/...` green WITHOUT `UPDATE_GOLDEN`.
- [ ] All 50 goldens have `# syntax` directive on line 1.
- [ ] Node + Java goldens have the new commands.
- [ ] Unaffected language goldens have ONLY the `# syntax` change.

#### DoD
- [ ] T5.1 complete.
- [ ] Commit message: `test(integration): regenerate goldens for v2 build improvements`.

---

## Phase 6: E2E Size Assertions

**Objective:** Lock the size gains by adding image-size assertions to `e2e/e2e_test.go` for the languages where v2 measurably reduces image size.

### T6.1 — Add `requireSizeLessThan` E2E helper + Node + Python assertions

#### Objective
Add a helper that runs `docker image inspect <tag> --format '{{.Size}}'` and asserts a size cap in MB. Wire it into the existing `TestE2E_NodeNpm_BuildsImage`, `TestE2E_PythonFlask_BuildsImage`, and add new lightweight invocations against Express + Next.js examples if simple to wire.

#### Evidence
- `e2e/e2e_test.go:70-86` — `buildImage` helper exists; size assertions can sit next to existing invocations.
- D6 — we use absolute caps (loose, won't flake on Debian point releases) rather than ratio comparisons.

#### Files to edit
```
e2e/e2e_test.go            — add helper + assertions
```

#### Deep file dependency analysis
- **`e2e_test.go`** — single-file E2E suite. Helper sits with the other helpers (lines 60-118).

#### Deep Dives

**Helper signature:**
```go
// requireSizeLessThan asserts the named image is smaller than maxBytes.
// Reports the actual size in MB on failure.
func requireSizeLessThan(t *testing.T, tag string, maxMB int) {
    t.Helper()
    out, err := exec.Command("docker", "image", "inspect", tag, "--format", "{{.Size}}").Output()
    require.NoError(t, err)
    sizeBytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
    require.NoError(t, err)
    sizeMB := sizeBytes / (1024 * 1024)
    require.LessOrEqual(t, int(sizeMB), maxMB,
        "image %s is %d MB, expected ≤ %d MB", tag, sizeMB, maxMB)
}
```

**Caps (loose, generous; tighten in v3):**
- `node-npm` (hello-world): `≤ 250 MB` (today ~150-200 MB; runtime is `node:20-bookworm-slim` ~210 MB; we want assurance dev deps don't bloat it).
- `python-flask`: `≤ 250 MB` (today base `python:3.12-slim-bookworm` is ~125 MB + flask + gunicorn).
- `node-next`: NOT asserted (the win requires Next standalone, deferred to v3).
- `node-express` (typical realistic app): NOT asserted yet — the example is too minimal to reflect real dev deps.

**Why caps and not ratios:** D6. Future Debian update bumps the base by 5-10 MB; ratio comparison breaks. Absolute cap won't flake within a 12-month horizon.

#### Tasks
1. Add `requireSizeLessThan(t, tag, maxMB)` helper.
2. Add the assertions to existing Node + Python E2E tests.
3. Verify on a Docker-enabled host: `mise run test-e2e`.

#### TDD
```
RED:    requireSizeLessThan correctly fails when size > cap (manually verified by setting cap=1)
RED:    TestE2E_NodeNpm_BuildsImage now asserts size ≤ 250 MB → FAILS without v2 (assuming images are ~210 MB today, fits comfortably)
GREEN:  Run mise run test-e2e on a Docker host
REFACTOR: None.
VERIFY: mise run test-e2e
```

#### Acceptance Criteria
- [ ] Helper compiles and works.
- [ ] Node + Python E2E tests pass with the new assertions.
- [ ] No flaky failures across 3 consecutive `mise run test-e2e` runs.

#### DoD
- [ ] T6.1 complete.
- [ ] Commit message: `test(e2e): image-size assertions + requireSizeLessThan helper`.

---

## Phase 7: Documentation, CHANGELOG, Final QA

**Objective:** Write the user-facing record of the changes; ensure all quality gates pass; open the PR.

### T7.1 — CHANGELOG.md `[Unreleased]` entry

#### Objective
Add a comprehensive Unreleased section documenting the size-reduction defaults, the new `.dockerignore` behavior, the `# syntax` directive, and Java install warmup.

#### Files to edit
```
CHANGELOG.md
```

#### Deep Dives

**Entry shape:**
```markdown
## [Unreleased]

### Added
- `# syntax=docker/dockerfile:1` directive at the top of every generated Dockerfile (#NNN)
- Per-language default `.dockerignore` written to `<source>/.dockerignore` when none exists. Covers `.git/`, language-specific build outputs (`node_modules/`, `target/`, `__pycache__/`, etc.) (#NNN)
- E2E image-size assertions for Node and Python examples (#NNN)
- New public function `dockerignore.DefaultFor(providerName) string` for library consumers (#NNN)

### Changed
- Node deploy stages now drop devDependencies via `npm prune --omit=dev` / `pnpm prune --prod` / `yarn install --production`. Bun has no built-in prune and is unchanged. Estimated runtime image size reduction: 2-3× on typical apps (#NNN)
- Python local layer excludes `__pycache__/`, `*.pyc`, `.pytest_cache/`, `tests/`, `.venv/`, `.env` by default. User-supplied `.dockerignore` continues to take precedence (#NNN)
- Java install step now warms the dep cache via `gradle dependencies --no-daemon --refresh-dependencies` (Gradle) or `mvn -B dependency:go-offline` (Maven). Cold builds reuse the Docker layer cache after manifest changes (#NNN)

### Notes
- User-supplied `.dockerignore` is never modified or merged with the default. To regenerate, delete the existing file.
- `.env` files are now excluded from Python images by default (security-positive change). Runtime config should use `THEOPACKS_*` env vars or a secrets backend.
- Bun projects are unaffected by the prune change because bun lacks a prune subcommand. Bun's hardlink-based install model already keeps images relatively lean.
```

#### Tasks
1. Open `CHANGELOG.md`.
2. Ensure `[Unreleased]` exists; add the section.

#### Acceptance Criteria
- [ ] Entry covers Added/Changed/Notes.
- [ ] Each bullet references `#NNN` placeholder for PR number (replaced at PR creation).

#### DoD
- [ ] T7.1 complete.

---

### T7.2 — README.md updates

#### Objective
Update README.md sections that describe the build-plan output: mention the size-reduction defaults, link to the new `.dockerignore` generation behavior.

#### Files to edit
```
README.md
```

#### Deep Dives
- "Supported Languages" table: no change.
- "Generated Dockerfile" section: add a one-line note about size optimization defaults and the auto-generated `.dockerignore`.

#### Acceptance Criteria
- [ ] README mentions auto-generated `.dockerignore` and the `# syntax` directive.

#### DoD
- [ ] T7.2 complete.

---

### T7.3 — CLAUDE.md updates

#### Objective
Update CLAUDE.md to reflect: (a) the new `core/dockerignore/` package, (b) the CLI write semantics for `.dockerignore`, (c) Node prune as a documented behavior so contributors don't accidentally undo it.

#### Files to edit
```
CLAUDE.md
```

#### Deep Dives
- Add a "Generated artifacts" subsection under the CLI flow describing both Dockerfile and `.dockerignore` writes.
- Add `core/dockerignore/templates.go` to the Key References table.

#### Acceptance Criteria
- [ ] CLAUDE.md notes the dual-artifact CLI behavior.
- [ ] Key References table updated.

#### DoD
- [ ] T7.3 complete.

---

### T7.4 — Final QA — run the full quality gate

#### Objective
Run every quality gate one more time against the integrated branch, then prepare the PR description.

#### Tasks
1. `mise run check` — zero warnings.
2. `mise run test` — green.
3. `mise run test-e2e` — green on Docker-enabled host.
4. Manual review of CHANGELOG, README, CLAUDE.md.
5. Author PR description; reference this plan; include a "before/after" image-size table.
6. Push branch, open PR against `develop`.

#### Acceptance Criteria
- [ ] All quality gates green.
- [ ] PR description references `docs/plans/build-correctness-and-speed-v2-plan.md`.
- [ ] PR description includes the size table from the Context section, updated with measured numbers.

#### DoD
- [ ] T7.4 complete.
- [ ] PR opened.

---

## Coverage Matrix

| # | Gap / Requirement | Task(s) | Resolution |
|---|---|---|---|
| 1 | Node deploy ships devDependencies (image bloat ~3×) | T1.1 | Append `<pm> prune --omit=dev` to build step |
| 2 | Python local layer carries `__pycache__`/tests/.venv (image bloat ~2×) | T2.1 | Annotate local layer with default excludes; effect via `.dockerignore` (Phase 4) |
| 3 | Build context transfer ships `.git`/`node_modules`/`target` (multiple GB) | T4.1, T4.2 | Generate per-language `.dockerignore` from CLI when user has none |
| 4 | No `# syntax=docker/dockerfile:1` directive | T0.1 | Renderer emits directive at top of every Dockerfile |
| 5 | Java install step doesn't warm dep cache | T3.1, T3.2 | Add `gradle dependencies` / `mvn dependency:go-offline` |
| 6 | No measurable size assertions in E2E (gains can regress silently) | T6.1 | Add `requireSizeLessThan` helper + assertions for Node + Python |
| 7 | `.dockerignore` generation must not overwrite user-supplied | T4.2 | CLI checks for existing file; no merge, no overwrite (D3) |
| 8 | Library API stays stable (backward compat) | All phases (architectural) | `core.GenerateBuildPlan` unchanged; CLI is the only writer (D2) |
| 9 | Goldens must reflect every change | T5.1 | Bulk regen via `UPDATE_GOLDEN=true` after Phases 0-4 |
| 10 | User-facing changes documented | T7.1, T7.2, T7.3 | CHANGELOG, README, CLAUDE.md updates |

**Coverage: 10/10 gaps covered (100%).**

---

## Global Definition of Done

- [ ] All phases (0-7) completed.
- [ ] `mise run check` green (zero `go vet`, `go fmt`, `golangci-lint` warnings).
- [ ] `mise run test` green (all unit + integration tests).
- [ ] `mise run test-e2e` green on Docker-enabled host, including new size assertions.
- [ ] All ~50 goldens regenerated, reviewed, and committed in a single review-friendly commit.
- [ ] `# syntax=docker/dockerfile:1` present at line 1 of every generated Dockerfile.
- [ ] Node deploy images measurably ≤ 250 MB on hello-world examples.
- [ ] Python flask image measurably ≤ 250 MB.
- [ ] CLI writes per-language `.dockerignore` when source has none; never overwrites a user-supplied file.
- [ ] CHANGELOG.md `[Unreleased]` describes Added/Changed/Notes per Keep a Changelog.
- [ ] README.md and CLAUDE.md reflect the new behaviors.
- [ ] Backward compatibility: existing `theopacks.json`, `THEOPACKS_*` env vars, and the `core.GenerateBuildPlan` API continue to work unchanged.
- [ ] PR opened against `develop` with description referencing this plan and including the measured size-reduction table.
- [ ] No file in `core/dockerignore/` exceeds 200 lines.
- [ ] Each provider file modified stays under 350 lines.
