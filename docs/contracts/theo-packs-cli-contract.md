# theo-packs CLI contract

> **Status:** Stable across minor versions. Breaking changes ship in major releases with a `Changed` entry in `CHANGELOG.md`.
>
> **Audience:** Anyone integrating with `theopacks-generate` — primarily the theo product's build orchestrator (Argo Workflow), but also CI pipelines and humans invoking the CLI directly.
>
> **What this document is:** the explicit contract between `theopacks-generate` and its caller. Every flag, env var, file the CLI reads, file the CLI writes, and assumption about the calling environment is enumerated here. If a behavior is not documented here, it is not part of the contract — relying on it is unsafe.

---

## Single source of truth

theo-packs is the **only** producer of Dockerfiles consumed by the Theo build pipeline. A user-supplied Dockerfile inside the analyzed app directory (`<source>/<app-path>/Dockerfile`) is a contract violation: the CLI rejects the build with **exit code 2** and an error message naming the offending path. There is no override flag, no warning mode, no env var. The contract is unambiguous — one path, one source of truth.

If a user wants to ship a hand-tuned Dockerfile, they should not invoke theo-packs at all and use a different deployment pipeline. theo-packs does not co-exist with user Dockerfiles by design.

**A Dockerfile at the workspace root (`<source>/Dockerfile`) is NOT checked** — that file may legitimately exist for local development outside Theo (e.g., `docker compose up` reading a top-level Dockerfile unrelated to the app being deployed). theo-packs only rejects within the app path it analyzes.

The exact stderr message the CLI emits on rejection:

```
[theopacks] ERROR: user-supplied Dockerfile found at <path>.

theo-packs is the single source of truth for Dockerfile generation.
Remove the file and rerun. To opt out of generation entirely, do not
invoke theo-packs — declare your build via a different mechanism in
your deployment pipeline.
```

---

## Scope

This contract covers `cmd/theopacks-generate/main.go` and the public library entry point `core.GenerateBuildPlan`. Provider-specific details (which manifest files trigger which provider, version-detection priority, framework auto-detection) live in `CLAUDE.md`. Deploy-stage size optimizations and `.dockerignore` template content live in `README.md` and `core/dockerignore/templates.go`.

This document does **not** cover:

- The internal `BuildPlan` data model (subject to refactor across minor versions; consumers should use the produced Dockerfile, not the JSON plan).
- The renderer's exact output shape beyond the syntax directive and header comment (existing per-provider goldens lock the rest).

---

## CLI flags

| Flag | Default | Required | Meaning |
|---|---|---|---|
| `--source` | `/workspace` | Yes (in practice) | Root of the cloned source tree. **For workspaces, this MUST be the workspace root**, not the per-app subdir. |
| `--app-path` | `.` | No | Relative path from `--source` to the app being built. Used to compute `<source>/<app-path>` for user-Dockerfile lookup. For workspaces, also bridged to `THEOPACKS_APP_PATH`. |
| `--app-name` | `""` (unset) | Yes for multi-app workspaces | Logical name of the app. Bridged to `THEOPACKS_APP_NAME`. Required when the workspace has multiple selectable members (Cargo workspaces, Ruby/PHP `apps/`, Gradle subprojects, .NET solutions, Deno workspaces). |
| `--output` | (none) | Yes | Path where the generated Dockerfile is written. Parent directory is created if absent. |

---

## Env-var bridge (for workspace target selection)

The CLI bridges `--app-name` and `--app-path` to env vars that providers consult via `app.Environment.GetConfigVariable(...)`:

| Flag | Env var |
|---|---|
| `--app-name=<name>` (when non-empty) | `THEOPACKS_APP_NAME=<name>` |
| `--app-path=<path>` (when non-empty and not `.`) | `THEOPACKS_APP_PATH=<path>` |

The bridge is **universal** — it fires for all workspaces (Cargo, Gradle, Maven, Ruby/PHP `apps/`, .NET solutions, Deno, Node), not just Node. Providers select the workspace target from these env vars.

`Environment` does **not** read `os.Getenv`. Variables exported in the calling shell are NOT seen by providers unless the CLI bridges them. This is intentional — keeps the generation step deterministic and reproducible from a known input set.

---

## Single-app mode

Triggered when `--source` points to a directory containing exactly one app (no monorepo markers like `pnpm-workspace.yaml`, no `Cargo.toml [workspace]`, etc.).

- `--app-path=.` (default).
- `--app-name` may be empty — providers don't need it for single-app builds.
- The CLI analyzes `--source` directly.
- The generated Dockerfile uses paths relative to `--source`.
- **`docker build` MUST be invoked with the build context set to `--source`.**

---

## Workspace mode

Triggered when `--source` is the root of a multi-app workspace.

- `--app-path=<app-subdir>` (e.g., `apps/api`).
- `--app-name=<name>` is **required** when the workspace has multiple members (otherwise the provider errors with "set THEOPACKS_APP_NAME to one of: ...").
- For Node workspaces specifically, the CLI redirects `analyzeDir` to `--source` (workspace root) so the Node provider sees cross-package dependencies. This is `CHG-002b` and predates v2.
- For all other workspace shapes (Cargo, Gradle, Maven, Ruby/PHP, .NET, Deno), the providers handle workspace detection themselves from `--source/<app-path>` — the CLI does NOT redirect `analyzeDir`.
- Either way, the generated Dockerfile uses paths relative to the **workspace root**.
- **`docker build` MUST be invoked with the build context set to the workspace root** (the same directory passed as `--source`), regardless of the workspace shape.

The defensive header at the top of every generated Dockerfile states this invariant explicitly:

```
# theo-packs: generated for provider "node".
# Build context: the directory passed as theopacks-generate --source
# (workspace root for monorepos, app dir otherwise). When invoking
# docker build, set --file <this-file> and the context to that same
# directory. Misalignment is the most common cause of "not found" errors.
```

---

## User-Dockerfile rejection

The CLI checks for `<source>/<app-path>/Dockerfile` immediately after flag parsing. If the file exists, the CLI exits with **code 2** and the stderr message documented in the "Single source of truth" preamble. No further action — no plan generation, no `.dockerignore` write, no output file.

The check is scoped to the app path being analyzed. A `Dockerfile` at the workspace root (`<source>/Dockerfile`, when `--app-path` is a subdir) is NOT examined and does NOT trigger rejection. This permits local development workflows (e.g., `docker compose up` against a top-level Dockerfile unrelated to Theo deployment).

---

## `.dockerignore` generation

When `<source>/.dockerignore` does **not** exist, the CLI writes a per-language default to that path using `core/dockerignore.DefaultFor(<provider-name>)`. The detected provider determines the template (e.g., `node` template excludes `node_modules/`, `.next/cache/`, `dist/`).

Behavior:

- **User-supplied files are never overwritten or merged.** If `<source>/.dockerignore` exists, the CLI logs `User-provided .dockerignore found at <path>` and continues.
- Read-only sources fail gracefully with a logged warning; the Dockerfile write still proceeds.
- The write happens **before** the Dockerfile is written, so a `docker build` that picks up both has the ignore in effect from the first read.

To regenerate the default, delete `<source>/.dockerignore` and rerun the CLI.

---

## Generated Dockerfile invariants

Every Dockerfile produced by `dockerfile.Generate` (and therefore by the CLI) satisfies:

1. Line 1 is `# syntax=docker/dockerfile:1` followed by a blank line. BuildKit cache mounts (`--mount=type=cache`) require this directive on some builders.
2. After the directive, a header comment block names the provider and states the expected build context (see "Workspace mode").
3. Multi-stage build with named install/build/deploy stages (subject to per-provider variation; see goldens under `core/dockerfile/testdata/`).
4. Deploy stage uses a slim or distroless runtime image, runs as a non-root user where the base image supports it, and emits `CMD` in exec form when the start command is shell-feature-free.

---

## What the theo product MUST pass

This is the section the product team should treat as the contract surface.

For a single-app project at `/workspace`:

```
theopacks-generate \
  --source /workspace \
  --app-path . \
  --app-name <name> \           # optional for single-app, recommended for logging
  --output /workspace/Dockerfile.<name>

docker build \
  --file /workspace/Dockerfile.<name> \
  /workspace                    # <-- context = --source
```

For a workspace at `/workspace` with the app at `/workspace/apps/api`:

```
theopacks-generate \
  --source /workspace \
  --app-path apps/api \
  --app-name api \
  --output /workspace/Dockerfile.api

docker build \
  --file /workspace/Dockerfile.api \
  /workspace                    # <-- context = WORKSPACE ROOT, not apps/api
```

The product's `theo.yaml` field `path:` is currently overloaded — it means both "where the app lives" AND "what context to use for `docker build`". For monorepos with shared packages (e.g., `apps/api` references `packages/shared`), these two concepts are different, and `theo.yaml` cannot express the difference today.

The product team is tracking this as F6 in the dogfood report: a `build_context:` field separate from `path:` in the `theo.yaml` schema. Until that lands:

- Workspaces with **no** cross-app shared packages work fine — context = workspace root, the lone app builds, the rest is dead context that BuildKit ignores via `.dockerignore`.
- Workspaces **with** cross-app shared packages (the common Turbo/Nx/pnpm pattern) only work when the product passes the workspace root as context. If the product passes `apps/api` as context (current default per F3 reproduction), the build fails with `"/packages/shared": not found`.

theo-packs has **no in-repo workaround** for this gap. Adding a path-rewriting flag would create two source-of-truth places for the build-context decision and confuse the contract. The fix lives in the product.

**The product orchestrator MUST surface the exit-code-2 rejection to the user**, not retry without theo-packs or fall back to a different generator. The single-source-of-truth contract requires that a user Dockerfile is treated as a hard error, not a routing signal.

---

## Failure modes

| Symptom | Cause | Resolution |
|---|---|---|
| CLI exits with code 2 and the stderr line `user-supplied Dockerfile found at <path>` | Contract violation: a Dockerfile exists at `<source>/<app-path>/Dockerfile` | Delete the Dockerfile. theo-packs generates it; do not commit one. See "Single source of truth" preamble. |
| `docker build` fails with `"/<some-path>": not found` | Build context doesn't match what the generated Dockerfile expects | Set `docker build` context to the directory passed as `theopacks-generate --source`. Read the generated Dockerfile's header comment. |
| theo-packs errors with `set THEOPACKS_APP_NAME to one of: ...` | Multi-app workspace; `--app-name` was not passed | Pass `--app-name=<one-of-the-listed-apps>` to the CLI. |
| `bundle install` / `npm ci` fails because lockfile missing | Provider-specific contract — most providers require a lockfile for reproducible builds | Commit the lockfile. Per-provider details in `CLAUDE.md`. |
| Generated `.dockerignore` excludes a file the user needs at runtime | The default template is opinionated | Override by writing your own `.dockerignore`. The CLI never overwrites a user-supplied file. |

---

## Versioning

This contract is stable across minor versions. Specifically:

- New flags may be added with sensible defaults.
- New env vars may be bridged.
- The generated Dockerfile shape (slim/distroless runtime, BuildKit cache mounts, USER directive, HEALTHCHECK, etc.) may change in ways that improve image size or build correctness without breaking `docker build`.

Breaking changes that ship in a major version include: removing a flag, removing a documented env-var bridge, changing the meaning of `--source` / `--app-path` / `--app-name`, or changing what causes a build to be rejected.

When a breaking change ships, `CHANGELOG.md` carries a `### Changed` entry under the new major version explicitly naming what consumers must migrate.

### Breaking changes since v1

- **User-Dockerfile precedence removed.** Pre-v1 (and the v1 PR that introduced this contract document) treated a user-supplied Dockerfile at `<source>/<app-path>/Dockerfile` as taking precedence over generation: the CLI would copy the user file to `--output` and exit successfully. As of `[Unreleased]` (next major), the same condition causes exit code 2 with a rejection message. There is no override flag. Rationale: pre-release window with no external users; eliminates the entire class of "buggy template Dockerfile blamed on theo-packs" misdiagnosis (see `docs/plans/single-source-of-truth-plan.md`).

---

## See also

- `CLAUDE.md` — project conventions, provider detection order, `theopacks.json` schema.
- `README.md` — high-level overview, supported languages, generated Dockerfile defaults.
- `core/dockerignore/templates.go` — per-language `.dockerignore` template content.
- `core/dockerfile/testdata/integration_*.dockerfile` — golden Dockerfiles per language. The header comment is asserted on every golden by `TestGoldens_HasProviderHeader`.
- `docs/plans/monorepo-contract-validation-plan.md` — the plan that produced this contract document.
