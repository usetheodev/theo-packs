# Plan: Monorepo Build Contract Validation & Hardening

> **Version 1.0** — Address the in-repo subset of the dogfood findings (F1-F7) by (1) proving F5 is false with a regression test, (2) acknowledging that F3 has a theo-packs implication beyond the theo-stacks template, (3) documenting the build-context contract explicitly, (4) emitting a defensive header comment in every generated Dockerfile, and (5) adding an E2E test that builds real theo-stacks templates with the correct context. Out of scope: F1/F2 (theo-stacks template fixes — different repo) and F6/F7 (theo product schema/silent-success — different repo). This plan does NOT pretend to solve the systemic monorepo-context problem; that's F6 and lives in the product. This plan makes theo-packs's part of the contract explicit and verifiable, so when the product fix lands, theo-packs is already aligned.

## Context

A dogfood session against `theo-stacks/templates/monorepo-turbo` (a Turbo monorepo with `apps/api`, `apps/web`, `packages/shared`) surfaced 6 findings (F1-F7) across three repositories:

| Finding | Repo | What |
|---|---|---|
| F1 | theo-stacks | `theo.yaml` missing `build: dockerfile` |
| F2 | theo-stacks | Per-app Dockerfile copies `apps/api/node_modules` that doesn't exist (npm hoisting) |
| F3 | theo-stacks **+ theo-packs** | Dockerfile assumes context=workspace-root; theo product sets context=app-path |
| F5 | claimed theo-packs | Same `node_modules` bug supposedly emitted by theo-packs generator |
| F6 | theo product | `theo.yaml` schema lacks `build_context:` separate from `path:` |
| F7 | theo product | `static_delivery.go` returns nil when CF KV is unconfigured (silent success) |

### Evidence — F5 is false in current theo-packs

Manual reproduction against `theo-stacks/templates/monorepo-turbo` with the current generator (`develop` branch + v2 + cli-bridge fixes from this repo):

```
$ /tmp/theopacks-generate --source /tmp/mt-test --app-path apps/api --app-name api --output /tmp/mt.Dockerfile
[theopacks] Node workspace detected at /tmp/mt-test (type=1, hasTurbo=true, members=5) — analyzing root for app "api" at "apps/api"
[theopacks] Detected: [node]
```

The generated Dockerfile is:

```dockerfile
# syntax=docker/dockerfile:1

FROM node:20-bookworm AS install
WORKDIR /app
COPY package.json ./
COPY turbo.json ./
COPY apps/api/package.json apps/api/
COPY apps/web/package.json apps/web/
COPY packages/eslint-config/package.json packages/eslint-config/
COPY packages/shared/package.json packages/shared/
COPY packages/typescript-config/package.json packages/typescript-config/
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    sh -c 'npm install'

FROM install AS build
WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    sh -c 'npx turbo run build --filter=api...'
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    sh -c 'npm prune --omit=dev'

FROM node:20-bookworm-slim
RUN useradd -r -u 10001 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
CMD ["/bin/sh", "-c", "cd apps/api && npm start"]
```

There is NO `COPY --from=deps /app/apps/api/node_modules ./apps/api/node_modules`. The deploy stage does `COPY --from=build /app /app` (entire workspace) which preserves the hoisted root `node_modules`. F5's claim against theo-packs does not match observable output.

The buggy Dockerfile that the dogfood saw came from **`theo-stacks/templates/monorepo-turbo/apps/api/Dockerfile`** — a user-provided file in the template that takes precedence per `cmd/theopacks-generate/main.go:60-73` ("User-provided Dockerfile takes precedence"). theo-packs **passed it through unchanged**, which is the documented contract. The bug is in theo-stacks (F2), not in theo-packs.

### Evidence — F3 has a theo-packs implication

The generated Dockerfile above has `COPY apps/api/package.json apps/api/`, `COPY packages/shared/package.json packages/shared/`, etc. These paths are **relative to the build context**. theo-packs in workspace mode assumes the build context is the workspace root (= `--source` flag), not the app subdir.

If the theo product runs `docker build --context=apps/api` (per F3's claim that `path: apps/api` produces context=app subdir), THIS dockerfile **also fails** — `apps/api/package.json` is not at `apps/api/apps/api/package.json`.

This is not theo-packs' bug to fix at the data level (F6 is the systemic fix in the product), but theo-packs **must document the contract explicitly** so the product team knows what theo-packs requires from its caller.

### Evidence — current state of the contract documentation

`CLAUDE.md` lines 243-261 describe the CLI behavior including "Workspace detection (CHG-002b)" and that the env vars `THEOPACKS_APP_NAME` / `THEOPACKS_APP_PATH` get set when the source root is a Node workspace. **It does not state the build-context invariant.** A reader assumes (correctly) that theo-packs analyzes the workspace root, but no line ties that to "the docker build context MUST also be the workspace root".

`README.md` similarly omits the contract.

The Dockerfile itself emits no comment indicating expected context. A debugger looking at a failed `docker build` has no signal.

---

## Objective

**Done = (1) a regression test asserts that theo-packs Node workspace generation does NOT emit per-app `COPY node_modules` patterns; (2) `docs/contracts/theo-packs-cli-contract.md` exists and explicitly documents the workspace-root-context invariant; (3) every generated Dockerfile starts with a comment block stating the expected build context; (4) a new e2e test (`TestE2E_MonorepoTurboFromStacks`) builds the actual theo-stacks `monorepo-turbo` template with `--source=workspace-root --app-path=apps/api`, asserts the docker build succeeds when run with context=workspace-root, and is wired into `mise run test-e2e`.**

Specific, measurable goals:

1. **F5 false-claim regression test** — `core/dockerfile/goldens_audit_test.go` gains a test that fails if any future Node workspace golden contains `COPY ... apps/.*/node_modules` or similar per-app `node_modules` reference.
2. **Contract document** — `docs/contracts/theo-packs-cli-contract.md` (≥ 100 lines) covering: env-var bridge, build-context invariant, single-app vs workspace mode, user-Dockerfile precedence, and the explicit "what the theo product MUST pass" section.
3. **Defensive Dockerfile header** — every generated Dockerfile starts with `# theo-packs: built from <provider>; expected docker build context = <source-or-workspace-root>`. Renderer-emitted, not provider-emitted.
4. **E2E test against theo-stacks** — `TestE2E_MonorepoTurboFromStacks` in `e2e/e2e_test.go` (build tag `e2e`) clones (or symlinks) the upstream template, removes the buggy user-Dockerfile, generates a Dockerfile via the CLI, runs `docker build` from the workspace root, asserts the image exists. Requires Docker.
5. **No regression** — all existing `mise run test`, `mise run check`, and `mise run test-e2e` continue to pass.
6. **Backward compat** — no `theopacks.json`, `THEOPACKS_*`, or `core.GenerateBuildPlan` API change.

---

## ADRs

### D1 — Defensive Dockerfile header is renderer-emitted, not provider-emitted
**Decision:** `core/dockerfile/generate.go` writes the header comment block right after `# syntax=docker/dockerfile:1`, before any `FROM`. Providers contribute the `<provider>` token via a new field on the BuildPlan, but the SHAPE of the header is renderer-owned. Only Dockerfile-format concerns live in the renderer.

**Rationale considered:**
- *Provider-emitted via `FileCommand`* — would mean every provider has to remember to emit the header, easy to miss in a new provider, no enforcement.
- *Renderer-emitted using a new BuildPlan field* — single source of truth; the field is optional (empty → header still emitted with "unknown" provider name); future audit checks can verify presence universally.

**Consequences:**
- Enables: a single audit test asserts the header is present on every golden.
- Constrains: BuildPlan gains one optional field (`ProviderName string`) to carry the provider's name into the renderer. The field is set by `core.GenerateBuildPlan` at the end of plan generation.

### D2 — Contract document lives in `docs/contracts/`, not `docs/plans/`
**Decision:** Long-lived contract documents go under `docs/contracts/`. `docs/plans/` is for in-flight implementation plans that get archived (or stay as historical record) once the work merges.

**Rationale:** Mixing live contracts with completed plans makes contracts hard to find and easy to forget to update. Separating directories establishes the convention.

**Consequences:**
- Enables: future contracts (e.g., "theopacks.json schema", "BuildPlan stability guarantees") have an obvious home.
- Constrains: a one-time directory addition; no other code change.

### D3 — F5 regression test asserts a NEGATIVE pattern, not a positive one
**Decision:** The regression test asserts that no Node workspace golden contains the substring `apps/.*/node_modules`. It does NOT positively assert the deploy stage's exact COPY shape — that's already covered by per-golden tests.

**Rationale considered:**
- *Positive shape assertion* — would couple the test to a specific deploy strategy. If we ever change to `COPY --from=build /app/dist /app/dist` or some other selective filter, the positive test would break unnecessarily.
- *Negative pattern assertion* — robust: passes on the current strategy AND on any future strategy that doesn't reintroduce per-app `node_modules` copies.

**Consequences:**
- Enables: the test is stable across future deploy-filter optimizations (e.g., the v3 Next.js standalone work).
- Constrains: a new Node deploy strategy that legitimately needs per-app `node_modules` (extremely unlikely given hoisting semantics) would have to relax the test.

### D4 — E2E test against theo-stacks uses a vendored copy, not a network fetch
**Decision:** `TestE2E_MonorepoTurboFromStacks` reads the `monorepo-turbo` directory from a sibling checkout of theo-stacks at `../theo-stacks/templates/monorepo-turbo`. If the sibling doesn't exist, the test skips with a clear message. We do NOT fetch from GitHub during tests.

**Rationale considered:**
- *git clone during the test* — flaky (depends on network), slow (full clone), and creates noisy test output. Network failures would mask real bugs.
- *Vendored copy in `examples/`* — would duplicate a maintained upstream artifact, drift over time, and create a confusing "is this the real template or theo-packs's mirror?" question.
- *Sibling-checkout convention* — matches how local development works (the user has both repos cloned next to each other). When the sibling is absent (CI without theo-stacks checked out), skip cleanly.

**Consequences:**
- Enables: the E2E test reflects the real upstream template; no drift.
- Constrains: CI must check out theo-stacks alongside theo-packs to run the test. Local dev workflow already has both. A small CI update is needed (out of scope for this plan; document in CHANGELOG).

### D5 — F3 is acknowledged but NOT fixed in theo-packs
**Decision:** This plan does NOT add a `--build-context-relative` flag, a path-rewriting mode, or any other workaround for F3. The systemic fix lives in the theo product (F6). theo-packs documents its contract clearly and validates it via tests; the product team makes the product respect the contract.

**Rationale:** Adding workarounds in theo-packs would create two source-of-truth places for the build-context decision (theo.yaml vs theo-packs CLI flag) and confuse the contract. Better to be explicit and let the product align.

**Consequences:**
- Enables: clean separation of responsibilities; one canonical place where build_context is decided (theo.yaml when F6 lands).
- Constrains: until F6 ships, monorepos with cross-app shared packages don't deploy via the theo product. theo-packs is correct; the product is the blocker. CHANGELOG calls this out so users know.

---

## Dependency Graph

```
Phase 0 (Verify F5 + add regression test)
    │
    ├──▶ Phase 1 (Defensive Dockerfile header) ──┐
    │                                             │
    ├──▶ Phase 2 (Contract document)             ─┤
    │                                             │
    └──▶ Phase 3 (E2E against theo-stacks)       ─┴──▶ Phase 4 (Docs/CHANGELOG/PR)
```

- **Phase 0** is the prerequisite — verifies the F5 dispute is grounded and locks it with a test.
- **Phases 1, 2, 3** are independent; can be implemented in any order. Serial commits chosen for review readability.
- **Phase 4** depends on all of 0-3.

---

## Phase 0: F5 False-Claim Regression Test

**Objective:** Lock the assertion that theo-packs Node workspace flow does not emit per-app `node_modules` COPY patterns. If a future change reintroduces the bug, this test fails before merge.

### T0.1 — Add `TestGoldens_NoPerAppNodeModulesCopy` to the audit suite

#### Objective
Extend `core/dockerfile/goldens_audit_test.go` with a corpus-level regression test that scans every Node workspace golden and fails if it contains `apps/<name>/node_modules` or `packages/<name>/node_modules` literal patterns.

#### Evidence
- The dogfood report claimed (F5) that theo-packs generates `COPY --from=deps /app/apps/api/node_modules ./apps/api/node_modules`. Manual reproduction with the current binary against `theo-stacks/templates/monorepo-turbo` shows this is not the case — theo-packs emits `COPY --from=build /app /app`. F5's claim against theo-packs is false.
- Without a test locking this, a future "optimization" that tries to selectively COPY only the per-app `node_modules` (e.g., to slim the deploy image) would silently reintroduce the broken pattern. npm/pnpm/yarn workspaces hoist deps to root by default; per-app COPY would fail with `not found` exactly as F2 documents.

#### Files to edit
```
core/dockerfile/goldens_audit_test.go    — ADD audit test
```

#### Deep file dependency analysis
- **`core/dockerfile/goldens_audit_test.go`** — already houses corpus-level tests (e.g., `TestGoldens_NoBashCMD`, `TestGoldens_PackageManagerStepsHaveCacheMounts`, `TestGoldens_NoDoubleShC`). New test follows the same shape: glob all `integration_node_*.dockerfile` files, regex-scan each for the forbidden pattern, fail with the file name + matching line.
- **No downstream impact.** The audit suite runs as part of `mise run test`.

#### Deep Dives

**Forbidden pattern (regex):**
```
COPY .* (apps|packages)/[^/]+/node_modules
```

This matches:
- `COPY --from=deps /app/apps/api/node_modules ./apps/api/node_modules` (the F5 claim)
- `COPY apps/web/node_modules ...`
- `COPY packages/shared/node_modules ...`

It does NOT match:
- `COPY apps/api/package.json apps/api/` (current — only manifests, not modules)
- `COPY --from=build /app /app` (current — whole workspace, hoisting preserved)
- `COPY --from=build /app/node_modules ...` (theoretical — root node_modules only; OK)

**Invariants:**
- After T0.1, no Node workspace golden may contain a per-app `node_modules` COPY.
- Test runs against EVERY `core/dockerfile/testdata/integration_node_*.dockerfile`, not just a sample.
- Test passes today (verified empirically); the test exists to GUARD against future regressions.

**Edge cases:**
- A user-defined `theopacks.json` that overrides the deploy stage to copy only specific subpaths could hypothetically write a per-app `node_modules` COPY via custom commands. That path is not covered by goldens (goldens are deterministic generator output), so the test is unaffected.
- Future Bun/Deno provider that needs per-package COPY (unlikely but possible if they don't hoist) would relax the regex to skip those files. Not foreseen.

#### Tasks
1. Add `TestGoldens_NoPerAppNodeModulesCopy(t *testing.T)` after the existing tests in `goldens_audit_test.go`.
2. The test reads `core/dockerfile/testdata/`, filters to `integration_node_*.dockerfile`, and asserts none contain the forbidden regex.
3. Run `mise run test ./core/dockerfile/...` to confirm it passes today.

#### TDD
```
RED:    TestGoldens_NoPerAppNodeModulesCopy_DetectsBadPattern
        — synthetic input string containing `COPY apps/api/node_modules` → regex matches
RED:    TestGoldens_NoPerAppNodeModulesCopy_AcceptsCleanOutput
        — synthetic input from current Node workspace golden → no match
RED:    TestGoldens_NoPerAppNodeModulesCopy
        — runs against real corpus → all goldens pass
GREEN:  Implement the test (no code change needed in producers — they're already clean)
REFACTOR: None expected.
VERIFY: mise run test ./core/dockerfile/...
```

#### Acceptance Criteria
- [ ] `TestGoldens_NoPerAppNodeModulesCopy` passes against the current corpus.
- [ ] Forbidden-pattern detection has at least one positive (synthetic bad input) and one negative (real golden) sub-assertion.
- [ ] No regression in existing audit tests.
- [ ] `mise run check` clean.

#### DoD
- [ ] T0.1 implemented.
- [ ] Commit message: `test(audit): regression guard against per-app node_modules COPY (F5 dispute)`.

---

## Phase 1: Defensive Dockerfile Header

**Objective:** Emit a comment block at the top of every generated Dockerfile that states the expected build context. A debugger looking at a failed `docker build` immediately sees what context the file was generated for.

### T1.1 — Add `ProviderName` to BuildPlan + render header in dockerfile/generate.go

#### Objective
- BuildPlan gains an optional `ProviderName string` field set by `core.GenerateBuildPlan` after detection.
- `dockerfile.Generate` emits a fixed header comment after the syntax directive:
  ```
  # syntax=docker/dockerfile:1

  # theo-packs: generated for provider "<name>".
  # Build context: the directory passed as `theopacks-generate --source` (workspace
  # root for monorepos, app dir otherwise). When invoking `docker build`, set
  # `--file <this-dockerfile>` and the context to that same directory.
  ```

#### Evidence
- F3 in the dogfood report shows that misalignment between the assumed context and the actual `docker build` context produces cryptic errors (`"/packages/shared": not found`). A header that explicitly states "the context I expect is X" turns 30 minutes of debugging into 30 seconds.
- No existing way to identify, from a Dockerfile alone, which theo-packs version produced it or what context it expects.

#### Files to edit
```
core/plan/plan.go                         — ADD ProviderName field to BuildPlan
core/core.go                              — SET BuildPlan.ProviderName from detection
core/dockerfile/generate.go               — EMIT header after syntax directive
core/dockerfile/generate_test.go          — ADD unit tests
core/dockerfile/goldens_audit_test.go     — ADD audit test asserting header presence
core/dockerfile/testdata/*.dockerfile     — REGENERATE goldens (header added)
```

#### Deep file dependency analysis
- **`core/plan/plan.go`** — adds one optional `ProviderName string` field to `BuildPlan`. Backward compatible: omitted → empty string → renderer falls back to "unknown".
- **`core/core.go`** — `GenerateBuildPlan` already records the detected provider name (`detectedProviderName`). Adds one line: `buildPlan.ProviderName = detectedProviderName`.
- **`core/dockerfile/generate.go`** — `Generate(p)` writes `SyntaxDirective` then the new `headerComment(p.ProviderName)`.
- **`core/dockerfile/goldens_audit_test.go`** — adds a corpus-level test that EVERY golden has the header.

#### Deep Dives

**Header function signature:**
```go
// HeaderComment returns the metadata block emitted at the top of every
// generated Dockerfile, after the syntax directive. The block clarifies the
// expected docker build context — a frequent source of confusion when a
// monorepo Dockerfile is invoked with the wrong context.
func HeaderComment(providerName string) string {
    if providerName == "" {
        providerName = "unknown"
    }
    return fmt.Sprintf(`# theo-packs: generated for provider %q.
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.

`, providerName)
}
```

**Emit order in `Generate`:**
```go
b.WriteString(SyntaxDirective)            // unchanged
b.WriteString(HeaderComment(p.ProviderName))  // NEW
// existing step rendering...
```

**Invariants:**
- After T1.1, every Dockerfile produced by `dockerfile.Generate` contains the header comment.
- Header position: line 4-9 (after `# syntax=...\n\n`).
- ProviderName is set for ALL providers because `core.GenerateBuildPlan` records the detected name unconditionally.

**Edge cases:**
- `BuildPlan` constructed by hand (no provider) → empty ProviderName → header falls back to "unknown". Still emitted.
- Tests that build `BuildPlan` manually for renderer testing → header includes "unknown" but tests don't assert on it specifically.

#### Tasks
1. Add `ProviderName string` to `BuildPlan` struct in `core/plan/plan.go`.
2. In `core/core.go:GenerateBuildPlan`, set `buildPlan.ProviderName = detectedProviderName` after `ctx.Generate()`.
3. Add `HeaderComment(providerName string) string` to `core/dockerfile/generate.go` and call it from `Generate`.
4. Update `TestGenerate_SyntaxDirective` to assert the header follows.
5. Add `TestGoldens_HasProviderHeader` to `goldens_audit_test.go`.
6. Run `UPDATE_GOLDEN=true mise run test ./core/dockerfile/...` to regenerate.
7. Visual review of one regenerated golden per language to confirm format.

#### TDD
```
RED:    TestHeaderComment_Default               — HeaderComment("") contains "unknown"
RED:    TestHeaderComment_WithProvider          — HeaderComment("node") contains `provider "node"`
RED:    TestGenerate_HeaderAfterSyntax          — header line index > syntax line index
RED:    TestGoldens_HasProviderHeader           — every golden has `# theo-packs: generated for provider`
GREEN:  Implement HeaderComment + wire into Generate + set ProviderName in core
REFACTOR: None expected.
VERIFY: mise run test ./core/dockerfile/... ./core/...
```

#### Acceptance Criteria
- [ ] `BuildPlan.ProviderName` field added; `core.GenerateBuildPlan` sets it.
- [ ] `HeaderComment` function exists with documented behavior.
- [ ] All existing tests pass after golden regen.
- [ ] `TestGoldens_HasProviderHeader` passes against the regenerated corpus.
- [ ] `mise run check` clean.

#### DoD
- [ ] T1.1 implemented.
- [ ] All ~58 goldens regenerated with the new header.
- [ ] Commit message: `feat(dockerfile): emit defensive header comment with expected build context`.

---

## Phase 2: Contract Document

**Objective:** A long-lived document under `docs/contracts/` that explicitly states what theo-packs requires from its caller (the theo product or a human invocation).

### T2.1 — Write `docs/contracts/theo-packs-cli-contract.md`

#### Objective
Create a comprehensive contract document covering: CLI flags and their semantics, env-var bridge for workspace target selection, build-context invariant, single-app vs workspace mode, user-Dockerfile precedence, .dockerignore generation, and explicit "the theo product MUST..." section.

#### Evidence
- F3 surfaced because the build-context invariant was implicit. theo-packs's CLAUDE.md describes the workspace flow but does not state "the docker build context MUST be the same directory passed as `--source`".
- The dogfood reporter had to reverse-engineer the invariant from the error message. A canonical document prevents this for future product iterations.

#### Files to edit
```
docs/contracts/                              (NEW DIR)
docs/contracts/theo-packs-cli-contract.md    (NEW) — the contract document
```

#### Deep file dependency analysis
- **`docs/contracts/theo-packs-cli-contract.md`** — pure documentation. References (but does not duplicate) sections of CLAUDE.md and README.md. Cross-linked from CLAUDE.md and README.md.

#### Deep Dives

**Document structure:**

1. **What this contract covers** — scope statement.
2. **CLI flags** — `--source`, `--app-path`, `--app-name`, `--output` with current semantics.
3. **Env-var bridge** — `THEOPACKS_APP_NAME`, `THEOPACKS_APP_PATH` are populated from the corresponding flags (both for Node workspaces AND for non-Node workspaces — covers the v2 + cli-bridge fixes).
4. **Single-app mode** — when `--source` points to a single-app project: `--app-path=.`, no env-var bridging needed.
5. **Workspace mode** — when `--source` points to a workspace root: theo-packs analyzes the root, generates a Dockerfile whose paths are relative to the workspace root. **Build context MUST be the workspace root**.
6. **User-Dockerfile precedence** — if `<source>/<app-path>/Dockerfile` exists, theo-packs copies it verbatim. theo-packs makes NO claim about the user's Dockerfile correctness.
7. **`.dockerignore` generation** — written to `<source>/.dockerignore` only when absent.
8. **Required of the caller (the theo product)**:
   - For workspaces: invoke theo-packs with `--source=<workspace-root> --app-path=<app-subdir> --app-name=<app>`, and run `docker build --context=<workspace-root>`.
   - For single apps: invoke theo-packs with `--source=<app-dir> --app-path=. --app-name=<app>`, and run `docker build --context=<app-dir>`.
   - The `path:` field in `theo.yaml` is currently overloaded with both "app dir" and "build context"; F6 in the theo product introduces `build_context:` to disambiguate. Until F6 ships, theo-packs's workspace-mode Dockerfiles cannot be deployed via the theo product unless the product passes the workspace root as context.
9. **Failure modes** — when the contract is violated:
   - Wrong context → `docker build` fails with `"<some-path>": not found`.
   - Empty `--app-name` for a multi-app workspace → theo-packs errors with "set THEOPACKS_APP_NAME to one of: ...".
   - Missing lockfile → provider-specific behavior; documented per-provider in CLAUDE.md.
10. **Versioning** — this contract is stable across minor versions of theo-packs. Breaking changes ship in major versions with a CHANGELOG entry under `Changed`.

**Invariants:**
- The doc MUST explicitly state "build context = workspace root for workspace mode, app dir for single-app mode" near the top.
- The doc MUST link to the theo product's `build_context:` schema work (F6) so readers know about the upcoming change.
- The doc MUST NOT duplicate provider-specific details that live in CLAUDE.md (DRY).

#### Tasks
1. Create `docs/contracts/` directory.
2. Write `docs/contracts/theo-packs-cli-contract.md` per the structure above.
3. Cross-link from `CLAUDE.md` (CLI section) and `README.md` (Generated Dockerfile defaults section).

#### TDD
- N/A. This is documentation. Acceptance is reviewer agreement.

#### Acceptance Criteria
- [ ] `docs/contracts/theo-packs-cli-contract.md` exists and covers all 10 sections above.
- [ ] CLAUDE.md links to the contract.
- [ ] README.md links to the contract.
- [ ] Document is at least 100 lines (substantive coverage, not stubby).

#### DoD
- [ ] T2.1 implemented.
- [ ] Commit message: `docs(contracts): theo-packs CLI contract — workspace-root context invariant`.

---

## Phase 3: E2E Against Real theo-stacks Templates

**Objective:** Wire a new E2E test that exercises the end-to-end flow with a real theo-stacks template, asserting that the docker build succeeds when invoked with the correct context.

### T3.1 — Add `TestE2E_MonorepoTurboFromStacks`

#### Objective
The test reads `../theo-stacks/templates/monorepo-turbo` (sibling-checkout convention per D4), removes the buggy user-provided Dockerfile (the F2 bug), generates a Dockerfile via `theopacks-generate`, runs `docker build` from the workspace root, asserts the resulting image exists. Skips with a clear message if the sibling checkout is absent.

#### Evidence
- Existing E2E tests cover only `examples/` projects. None exercise the actual upstream theo-stacks template, so contract drift between the two repos is invisible until production.
- F3 + F5 + the validation runs in this conversation have shown that `monorepo-turbo` is the most demanding template for theo-packs's monorepo flow — covers turbo, npm workspaces, hoisted deps, cross-app shared packages.

#### Files to edit
```
e2e/e2e_test.go    — ADD TestE2E_MonorepoTurboFromStacks + helper
```

#### Deep file dependency analysis
- **`e2e/e2e_test.go`** — single-file E2E suite. New test fits with existing helpers (`buildImage`, `imageExists`, `requireBinaryAt`).

#### Deep Dives

**Test structure:**
```go
func TestE2E_MonorepoTurboFromStacks(t *testing.T) {
    if !dockerAvailable() {
        t.Skip("Docker not available")
    }

    upstream := filepath.Join(repoRoot(t), "..", "theo-stacks", "templates", "monorepo-turbo")
    if _, err := os.Stat(upstream); os.IsNotExist(err) {
        t.Skip("theo-stacks not checked out next to theo-packs; skipping (see docs/contracts/...)")
    }

    // Copy to temp so we don't mutate the upstream working tree
    workspace := t.TempDir()
    require.NoError(t, copyDir(upstream, workspace))

    // Remove the buggy user-Dockerfile (F2/F3 in theo-stacks). We're testing
    // the theo-packs-generated Dockerfile, not the user's.
    _ = os.Remove(filepath.Join(workspace, "apps", "api", "Dockerfile"))

    df := generateDockerfileForApp(t, workspace, "apps/api", "api")

    tag := "theopacks-e2e-monorepo-turbo:test"
    defer removeImage(tag)

    // CRITICAL: build context = workspace root (NOT apps/api). This validates
    // the contract documented in docs/contracts/theo-packs-cli-contract.md.
    buildImage(t, workspace, df, tag)
    require.True(t, imageExists(tag))
}
```

`generateDockerfileForApp` is a small helper (extracted in T3.1) that wraps the existing `generateDockerfile` to support the workspace flow:
```go
func generateDockerfileForApp(t *testing.T, source, appPath, appName string) string {
    t.Helper()
    // Same as generateDockerfile but plumbs --app-path / --app-name through
    // the theo-packs-generate binary rather than calling core.GenerateBuildPlan
    // directly — exercises the CLI's workspace detection and env bridging.
}
```

**Invariants:**
- The test runs the same code path as the theo product (theopacks-generate binary, not the in-process library).
- `docker build` context is the workspace root, asserted explicitly via `buildImage(t, workspace, ...)`.
- If the upstream template ever introduces a build-breaking change, this test catches it.

**Edge cases:**
- theo-stacks not checked out → `t.Skip` with a useful message pointing to the contract doc.
- Upstream template structure changes (e.g., `apps/api` renamed) → test fails with "directory not found"; we update the test to match the new structure (or change the template to match this test).
- Build takes long → adopt the existing `e2e_test.go` timeout (1500s suite-wide).

#### Tasks
1. Add `repoRoot(t *testing.T) string` helper if not already present (returns directory of `go.mod`).
2. Add `copyDir(src, dst string) error` helper or use `filepath.Walk` inline.
3. Add `generateDockerfileForApp` helper that invokes the CLI binary with `--source --app-path --app-name --output`.
4. Add `TestE2E_MonorepoTurboFromStacks` per the structure above.
5. Manual run: `go test -tags e2e ./e2e/ -run TestE2E_MonorepoTurboFromStacks -v` on a Docker-enabled host with theo-stacks sibling-checked-out.

#### TDD
```
RED:    TestE2E_MonorepoTurboFromStacks_SkipsWithoutSibling
        — temporarily rename the sibling dir → test skips with the documented message
RED:    TestE2E_MonorepoTurboFromStacks
        — sibling present + Docker available → docker build succeeds, image exists
GREEN:  Implement helpers + test
REFACTOR: None expected.
VERIFY: go test -tags e2e ./e2e/ -run TestE2E_MonorepoTurboFromStacks -v
```

#### Acceptance Criteria
- [ ] Test compiles cleanly under `go vet -tags e2e ./e2e/`.
- [ ] Test passes on a Docker-enabled host with theo-stacks checked out.
- [ ] Test skips cleanly when theo-stacks is absent (CI must explicitly choose to check out the sibling to run it).
- [ ] No regression in existing E2E tests.

#### DoD
- [ ] T3.1 implemented.
- [ ] Commit message: `test(e2e): build monorepo-turbo from theo-stacks (workspace-root context contract)`.

---

## Phase 4: Documentation, CHANGELOG, and PR

**Objective:** Tie everything together with user-facing docs and ship the PR.

### T4.1 — CHANGELOG.md `[Unreleased]` entry

#### Objective
Add an Unreleased section documenting the additions: F5 regression test, defensive header, contract document, theo-stacks E2E.

#### Files to edit
```
CHANGELOG.md
```

#### Deep Dives

**Entry shape (Added/Notes only — no Changed/Fixed because no behavior change for existing users):**

```markdown
### Added
- Defensive header comment in every generated Dockerfile stating the expected docker build context (#NNN)
- New public function `dockerfile.HeaderComment(providerName)` returning the metadata block (#NNN)
- `BuildPlan.ProviderName` field carrying the detected provider's name into the renderer (#NNN)
- `docs/contracts/theo-packs-cli-contract.md` — long-form contract describing the CLI's expectations of its caller (#NNN)
- E2E test `TestE2E_MonorepoTurboFromStacks` exercising the contract against the real upstream theo-stacks template (#NNN)
- Audit test `TestGoldens_NoPerAppNodeModulesCopy` locking the F5 regression (#NNN)

### Notes
- F5 (the dogfood claim that theo-packs generates buggy per-app `node_modules` COPY) was disproven against the current generator. The bug exists in `theo-stacks/templates/monorepo-turbo/apps/api/Dockerfile` (F2), which theo-packs passes through unchanged per the user-Dockerfile-precedence contract. Fixing F2 is theo-stacks's responsibility.
- F3 (build context mismatch) is acknowledged as a contract gap that the theo product must close via F6 (`build_context:` field in `theo.yaml` schema). theo-packs has no in-repo workaround; the contract document spells this out for the product team.
- This PR does NOT change the runtime behavior of any existing build flow. It adds documentation, a header comment (cosmetic), and tests.
```

#### Tasks
1. Open `CHANGELOG.md`.
2. Add the Unreleased entries above.

#### Acceptance Criteria
- [ ] CHANGELOG entry references issues/PRs (`#NNN` placeholder).
- [ ] Notes section explicitly addresses F3 and F5.

#### DoD
- [ ] T4.1 complete.

---

### T4.2 — README.md cross-link

#### Objective
Add a one-line link to the contract document in the "Generated Dockerfile defaults" section.

#### Files to edit
```
README.md
```

#### Acceptance Criteria
- [ ] README mentions the contract by name and links to it.

#### DoD
- [ ] T4.2 complete.

---

### T4.3 — CLAUDE.md cross-link + Key References update

#### Objective
Cross-link the contract from the CLI section. Add `docs/contracts/theo-packs-cli-contract.md` to the Key References table.

#### Files to edit
```
CLAUDE.md
```

#### Acceptance Criteria
- [ ] CLAUDE.md "CLI" section links to the contract.
- [ ] Key References table includes the contract file.

#### DoD
- [ ] T4.3 complete.

---

### T4.4 — Final QA + open PR

#### Objective
Run all quality gates one last time, push the branch, open the PR with a clear description that explicitly notes "this PR is in-repo response to the dogfood findings; F1/F2 → theo-stacks; F6/F7 → theo product".

#### Tasks
1. `mise run test` green.
2. `mise run check` green.
3. `go vet -tags e2e ./...` clean.
4. `gofmt -l .` returns nothing.
5. Push branch `fix/monorepo-contract-validation`.
6. Open PR against `develop`. Description references this plan + the dogfood report.

#### Acceptance Criteria
- [ ] All quality gates green.
- [ ] PR opened.
- [ ] PR description names F1/F2/F3/F5/F6/F7 explicitly and states which are in-scope for this PR (F5 verification, F3 documentation; nothing else).

#### DoD
- [ ] T4.4 complete.
- [ ] PR opened.

---

## Coverage Matrix

| # | Gap / Requirement | Task(s) | Resolution |
|---|---|---|---|
| 1 | F5: dispute the claim that theo-packs generates per-app `node_modules` COPY | T0.1 | Audit test asserts no Node workspace golden contains the forbidden pattern |
| 2 | F3 (theo-packs implication): generated Dockerfile assumes workspace-root context | T1.1, T2.1 | Defensive header states the assumption explicitly + contract document codifies it |
| 3 | No long-lived contract document explaining theo-packs's caller expectations | T2.1 | `docs/contracts/theo-packs-cli-contract.md` |
| 4 | No regression test exercising real theo-stacks templates | T3.1 | `TestE2E_MonorepoTurboFromStacks` builds the actual upstream template |
| 5 | User-facing CHANGELOG entry for the additions | T4.1 | Unreleased section with Added/Notes |
| 6 | Cross-links so the contract is discoverable | T4.2, T4.3 | README + CLAUDE.md link to the contract |
| 7 | F1, F2 (theo-stacks template fixes) | — | Out of scope (different repo); CHANGELOG Notes documents this |
| 8 | F6 (theo product schema gap) | — | Out of scope (different repo); contract document describes the upcoming product change |
| 9 | F7 (theo product silent-success) | — | Out of scope (different repo) |
| 10 | Backward compatibility | All phases (architectural) | No behavior change for existing flows; only adds header/docs/tests |

**Coverage of in-scope gaps: 6/6 in-scope (100%). Out-of-scope items (F1/F2/F6/F7) explicitly named with redirection.**

## Global Definition of Done

- [ ] All phases (0-4) completed.
- [ ] `mise run test` green (existing + new tests).
- [ ] `mise run check` green (zero `go vet`, `go fmt`, `golangci-lint` warnings).
- [ ] `mise run test-e2e` green on a Docker-enabled host with theo-stacks sibling-checked-out; passes (skips cleanly) without it.
- [ ] All ~58 goldens regenerated with the new header.
- [ ] `# theo-packs: generated for provider "<name>"` present on every golden.
- [ ] Audit test `TestGoldens_NoPerAppNodeModulesCopy` passing.
- [ ] `docs/contracts/theo-packs-cli-contract.md` exists and is cross-linked from CLAUDE.md and README.md.
- [ ] CHANGELOG.md `[Unreleased]` Added/Notes describe the changes and explicitly out-of-scope F1/F2/F6/F7.
- [ ] PR opened against `develop` with a description referencing this plan and the dogfood report.
- [ ] Backward compatibility: existing `theopacks.json`, `THEOPACKS_*` env vars, `core.GenerateBuildPlan` API unchanged.
- [ ] No file in `core/dockerfile/` exceeds 600 lines.
- [ ] No file in `docs/contracts/` exceeds 400 lines.
