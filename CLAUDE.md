# CLAUDE.md — theo-packs

This file provides guidance to Claude Code when working with code in this repository.

---

## What This Project Is

theo-packs is a **zero-configuration application builder** that detects your project's language/framework and generates an optimized build plan for containerization. It is the language detection and Dockerfile generation engine for the [Theo](https://usetheo.dev) Kubernetes PaaS.

The repository is a **single Go module**: `github.com/usetheo/theopacks` (Go 1.25+). It produces:

| Component | Path | Purpose |
|-----------|------|---------|
| Library | `core/` | Language detection, build plan generation, Dockerfile generation. Minimal deps. |
| CLI | `cmd/theopacks-generate/` | Single binary that analyzes a source tree and writes a Dockerfile. Designed to run inside an Argo Workflow step in the Theo build cluster. |
| Internal helpers | `internal/utils/` | Merge/utility functions used across `core/`. |
| E2E tests | `e2e/` | Build-tagged tests that build real Docker images from `examples/`. |
| Examples | `examples/` | ~30 reference projects (Go, Node, Python, shell, static) used by unit and E2E tests. |

There is **no separate `railpack/` module** and no BuildKit/LLB integration in this repo — Dockerfile generation is the only build artifact produced. The downstream system (Argo Workflow + Kaniko/BuildKit running externally) consumes the generated Dockerfile.

**Key flow:**

```
Source code → Provider.Detect() → Provider.Plan() → BuildPlan → dockerfile.Generate() → Dockerfile
```

The CLI (`theopacks-generate`) wraps this flow with workspace detection, user-Dockerfile precedence, and Argo-friendly logging.

---

## Development Commands

All tasks are defined in `mise.toml` at the repo root. Run them from the root.

```bash
mise run test                   # go test ./core/... ./cmd/...
mise run test-e2e               # go test -tags e2e ./e2e/ -timeout 1500s
mise run test-update-snapshots  # UPDATE_GOLDEN=true go test ./core/dockerfile/...
mise run check                  # go vet + go fmt + golangci-lint
mise run tidy                   # go mod tidy
```

Direct Go invocations are also available for ad-hoc use:

```bash
# E2E tests (slow — builds real Docker images, requires Docker running)
go test -tags e2e ./e2e/ -timeout 1500s

# Update Dockerfile golden files in core/dockerfile/testdata/
UPDATE_GOLDEN=true go test ./core/dockerfile/...

# Run the CLI manually against an example
go run ./cmd/theopacks-generate \
  --source examples/node-npm \
  --app-path . \
  --app-name demo \
  --output /tmp/Dockerfile.demo
```

**Prerequisites:** Go 1.25+, [Mise](https://mise.jdx.dev/), Docker (only for E2E tests).

---

## The Rules

### Rule 1: Use `mise run`, not `go` directly (when a task exists)
Use `mise run test`, `mise run test-e2e`, `mise run test-update-snapshots`, `mise run check`, `mise run tidy`. Tasks live in `mise.toml` at the repo root. For ad-hoc invocations (CLI demo, single-test runs) use `go` directly — do not invent new top-level scripts.

### Rule 2: Use the App abstraction for file operations
All file system operations on the analyzed project MUST go through the `app.App` abstraction:

```go
// DO:
app.HasFile("package.json")
app.ReadFile("go.mod")
app.ReadJSON("package.json", &pkg)
app.FindFiles("*.go")
app.HasMatch("**/*.py")

// DON'T:
os.ReadFile(filepath.Join(source, "package.json"))  // WRONG
```

This enables caching, testing, and future remote file system support. The CLI entry (`cmd/theopacks-generate/main.go`) is the only legitimate place to call `os.ReadFile` / `os.WriteFile`, and only against paths *outside* the analyzed app (the user-provided Dockerfile copy and the output path).

### Rule 3: Error wrapping with context
Always wrap errors with `fmt.Errorf` and `%w`. Include enough context to trace the problem:

```go
// DO:
return fmt.Errorf("failed to read package.json: %w", err)

// DON'T:
return err                          // No context
return fmt.Errorf("error: %s", err) // Loses error chain (%s vs %w)
```

### Rule 4: Comments explain WHY, not WHAT
Do not write comments that repeat the code. Do not start function comments with the function name. Focus on explaining non-obvious logic, assumptions, or domain-specific hooks.

```go
// DO:
// Poetry projects with a build-system section need pip for installation
// because poetry itself doesn't handle native extensions.

// DON'T:
// NewApp creates a new App.
// readConfigJSON reads config from JSON file.
```

### Rule 5: Never manually edit lockfiles
Never edit `yarn.lock`, `package-lock.json`, `pnpm-lock.yaml`, or any other lockfile by hand. Always use the respective package manager.

### Rule 6: Prefer early returns
Avoid deep `if` nesting. Check error conditions and return early.

### Rule 7: Never manually edit golden files
Golden Dockerfiles in `core/dockerfile/testdata/` are regenerated, not hand-edited. Run `UPDATE_GOLDEN=true go test ./core/dockerfile/...` and review the diff.

---

## Architecture

### Repository Layout

```
theo-packs/
├── core/                       # Library
│   ├── core.go                 # GenerateBuildPlan() — entry point
│   ├── validate.go             # Plan validation (start command, steps, inputs)
│   ├── core_test.go
│   ├── dogfood_test.go         # End-to-end plan tests on synthetic apps
│   ├── monorepo_test.go        # Multi-app workspace scenarios
│   ├── integration_test.go
│   ├── app/                    # File system abstraction (App, Environment)
│   ├── config/                 # Config model (theopacks.json) + merging
│   ├── dockerfile/             # BuildPlan → Dockerfile string conversion
│   │   └── testdata/           # Golden Dockerfiles (UPDATE_GOLDEN=true to refresh)
│   ├── generate/               # GenerateContext: step builders, deploy builder, caches
│   ├── logger/                 # Logger with Info/Warn/Error levels
│   ├── plan/                   # Data structures: BuildPlan, Step, Layer, Command, Deploy
│   ├── providers/              # Provider interface + language-specific implementations
│   │   ├── provider.go         # Interface definition + registry
│   │   ├── golang/             # Go provider (go.mod, go.work)
│   │   ├── node/               # Node.js (npm, yarn, pnpm, bun, workspaces)
│   │   │   ├── workspace.go    # Turbo / pnpm / npm workspace detection
│   │   │   └── ...
│   │   ├── python/             # Python (pip, poetry, pipenv, uv)
│   │   ├── staticfile/         # Static HTML files
│   │   └── shell/              # Shell scripts (fallback)
│   └── resolver/               # Package version resolution
├── cmd/theopacks-generate/     # CLI binary
│   └── main.go
├── internal/utils/             # Internal helpers (merge, etc.)
├── e2e/
│   └── e2e_test.go             # E2E tests (build tag `e2e`)
├── examples/                   # Reference projects (Go, Node, Python, shell, static)
├── mise.toml                   # Tasks: test, check, tidy
├── go.mod / go.sum
└── Dockerfile.generate         # Container image that runs theopacks-generate
```

### Provider Detection Order

Providers are checked in this order (first match wins) — defined in `core/providers/provider.go` → `GetLanguageProviders()`:

1. **Go** — `go.mod` or `go.work`
2. **Rust** — `Cargo.toml`
3. **Java** — `build.gradle.kts`, `build.gradle`, or `pom.xml`
4. **.NET** — `*.csproj`, `*.fsproj`, `*.vbproj`, or `*.sln` (root or any subtree)
5. **Ruby** — `Gemfile`
6. **PHP** — `composer.json`
7. **Python** — `requirements.txt`, `pyproject.toml`, `Pipfile`, `setup.py`
8. **Deno** — `deno.json` or `deno.jsonc` (intentionally before Node so projects with both deno.json and a npm-compat package.json route to Deno)
9. **Node.js** — `package.json`
10. **Static files** — `index.html`
11. **Shell** — `*.sh` files

Order matters. The Deno-before-Node invariant is locked by `TestRegistrationOrder` in `core/providers/provider_test.go`. Override via `theopacks.json`:
```json
{ "provider": "node" }
```

### Build Plan Data Model

```
BuildPlan
├── Steps[]           # Named build stages (install, build, ...)
│   ├── Inputs[]      # Layer references (image, previous step, local files)
│   ├── Commands[]    # Exec, Copy, File, Path commands
│   ├── Caches[]      # Named cache mounts
│   ├── Secrets[]     # Secret names to pass
│   └── Variables{}   # Environment variables
├── Deploy            # Final runtime stage
│   ├── Base          # Runtime base image
│   ├── Inputs[]      # Files to copy from build stages
│   ├── StartCmd      # Container entry point
│   ├── Variables{}   # Runtime environment variables
│   └── Paths[]       # Additional PATH entries
├── Caches{}          # Cache definitions (directory, type)
└── Secrets[]         # Secret names
```

### Configuration Merging

Three config sources merge in order (later overrides earlier):

1. **File** — `theopacks.json` at the analyzed app's root (JSONC format, comments allowed)
2. **Environment** — `THEOPACKS_*` variables, accessed via `app.Environment`
3. **Options** — programmatic `core.GenerateBuildPlanOptions`

Key environment variables:

| Variable | Purpose |
|----------|---------|
| `THEOPACKS_START_CMD` | Override start command |
| `THEOPACKS_BUILD_CMD` | Override build command |
| `THEOPACKS_INSTALL_CMD` | Override install command |
| `THEOPACKS_PACKAGES` | Space-separated package versions (`nodejs@20 npm@10`) |
| `THEOPACKS_BUILD_APT_PACKAGES` | Extra apt packages for build |
| `THEOPACKS_DEPLOY_APT_PACKAGES` | Extra apt packages for runtime |
| `THEOPACKS_CONFIG_FILE` | Custom config file path |
| `THEOPACKS_GO_MODULE` | Go workspace: which module to build |
| `THEOPACKS_RUST_VERSION` | Rust toolchain version |
| `THEOPACKS_JAVA_VERSION` | Java major version (drives JDK build image + JRE runtime) |
| `THEOPACKS_DOTNET_VERSION` | .NET SDK version (major.minor) |
| `THEOPACKS_RUBY_VERSION` | Ruby version (major.minor) |
| `THEOPACKS_PHP_VERSION` | PHP version (major.minor) |
| `THEOPACKS_DENO_VERSION` | Deno major version |
| `THEOPACKS_APP_NAME` | Workspace-aware build target — Cargo workspace member, Gradle subproject / Maven module leaf, .NET solution project, Ruby/PHP `apps/<name>`, Deno workspace member. Set automatically by the CLI on Node monorepo detection; manual for the others. |
| `THEOPACKS_APP_PATH` | Workspace-aware build path (Node monorepo only — auto-set by CLI) |

### Layer Special Values

When parsing layer references in JSON:
- `"."` → local files (project source)
- `"..."` → spread operator (merge/append semantics)
- `"$stepname"` → reference to a previous build step
- Any other string → Docker image reference

### CLI: `theopacks-generate`

Single binary used by Theo's build pipeline (Argo Workflow). Flags:

```
theopacks-generate \
  --source /workspace \
  --app-path apps/api \
  --app-name api \
  --output /workspace/Dockerfile.api
```

Behavior:
1. **User Dockerfile precedence** — if `<source>/<app-path>/Dockerfile` exists, copy it to `--output` and exit.
2. **Workspace detection (CHG-002b)** — if the source root is a Node workspace monorepo (`turbo.json`, `pnpm-workspace.yaml`, or `package.json#workspaces`), analyze the **workspace root** instead of the per-app subdir, and pass `THEOPACKS_APP_NAME` / `THEOPACKS_APP_PATH` so the Node provider scopes the build (e.g. `turbo run build --filter=<app>...`).
3. Otherwise analyze `--app-path` as a standalone app.
4. Run `core.GenerateBuildPlan` → `dockerfile.Generate` → write to `--output`.
5. Echo the Dockerfile to stdout for Loki/Promtail capture.

---

## Testing

### Test Framework
- **testify** (`require` package) for assertions — not vanilla `testing`
- Table-driven tests with `t.Run()` for subtests
- `t.Parallel()` where safe
- Arrange-Act-Assert pattern

### Golden Dockerfiles
- Located in `core/dockerfile/testdata/`
- Update with `UPDATE_GOLDEN=true go test ./core/dockerfile/...`
- Never hand-edit — regenerate them and review the diff

### Unit / Integration Tests (in-process)

`core/` contains unit tests per package plus three end-to-end-in-process suites:

| File | What it covers |
|------|----------------|
| `core/core_test.go` | `GenerateBuildPlan` happy paths, env-driven config, secrets propagation |
| `core/dogfood_test.go` | Realistic apps assembled via `app.NewMemoryApp` — exercises full pipeline without Docker |
| `core/monorepo_test.go` | Workspace scenarios: pnpm/turbo/npm workspaces, Python monorepos, multi-app |
| `core/integration_test.go` | Cross-package wiring + golden Dockerfile assertions |

Run all in-process tests with `mise run test`.

### E2E Tests (real Docker)

`e2e/e2e_test.go` is gated behind the `e2e` build tag. It:

- Runs `core.GenerateBuildPlan` against each `examples/<project>` directory.
- Calls `dockerfile.Generate` to produce a Dockerfile string.
- Invokes `docker build` and (for some examples) `docker run` to verify the image works.
- Skips automatically if Docker is not running.

```bash
# Run all E2E tests
go test -tags e2e ./e2e/ -timeout 600s

# Single example
go test -tags e2e ./e2e/ -run "TestE2E/node-npm" -timeout 600s
```

There is **no per-example `test.json`** in this repo — expectations live inline in `e2e_test.go`.

---

## Adding a New Provider

1. Create `core/providers/<language>/` directory.
2. Implement the `Provider` interface (defined in `core/providers/provider.go`):

```go
type Provider interface {
    Name() string                                           // e.g., "ruby"
    Detect(ctx *generate.GenerateContext) (bool, error)     // Check manifest files
    Initialize(ctx *generate.GenerateContext) error         // Parse/validate
    Plan(ctx *generate.GenerateContext) error               // Generate build steps
    CleansePlan(buildPlan *plan.BuildPlan)                  // Post-generation cleanup
    StartCommandHelp() string                               // Help when start cmd missing
}
```

3. Register in `core/providers/provider.go` → `GetLanguageProviders()`. **Order matters** — first match wins.
4. Add example projects in `examples/<language>-*/` so the E2E suite picks them up.
5. Write unit tests in `core/providers/<language>/*_test.go`.
6. Regenerate golden Dockerfiles for any new integration cases: `UPDATE_GOLDEN=true go test ./core/dockerfile/...`.

### Typical Provider Structure

```go
func (p *MyProvider) Plan(ctx *generate.GenerateContext) error {
    // Install step: copy manifests, install dependencies (cacheable)
    installStep := ctx.NewCommandStep("install")
    installStep.AddInput(plan.NewImageLayer("build-image:latest"))
    installStep.AddCommand(plan.NewCopyCommand("manifest.lock"))
    installStep.AddCommand(plan.NewExecCommand("install-deps"))

    // Build step: copy source, compile
    buildStep := ctx.NewCommandStep("build")
    buildStep.AddInput(plan.NewStepLayer("install"))
    buildStep.AddInput(ctx.NewLocalLayer())
    buildStep.AddCommand(plan.NewExecCommand("build-cmd"))

    // Deploy configuration
    ctx.Deploy.Base = plan.NewImageLayer("runtime-image:latest")
    ctx.Deploy.StartCmd = "start-cmd"
    ctx.Deploy.AddInput(plan.NewStepLayer("build"))

    return nil
}
```

---

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Using `os.ReadFile` on project files | Use `app.ReadFile()` or `app.ReadJSON()` |
| Skipping `mise run check` before pushing | `check` runs `go vet`, `go fmt`, and `golangci-lint` — wire it into pre-push |
| Editing lockfiles manually | Use the package manager (`npm install`, `yarn`, etc.) |
| `fmt.Errorf("error: %s", err)` | Use `%w` to preserve error chain |
| Starting comment with function name | Describe behavior, not the function signature |
| Deep `if` nesting | Early return on error conditions |
| Assuming provider order doesn't matter | First match wins — order in `GetLanguageProviders()` is intentional |
| Manually editing golden Dockerfiles | Run `UPDATE_GOLDEN=true go test ./core/dockerfile/...` |
| Running E2E tests without the build tag | Use `go test -tags e2e ./e2e/` — without the tag the file is excluded |
| Looking for a `railpack/` directory | It does not exist — the repo is a single module rooted at `core/` + `cmd/` |

---

## Key References

| File | What it does |
|------|-------------|
| `core/core.go` | `GenerateBuildPlan()` — main library entry point |
| `core/validate.go` | Plan validation + helpful error messages |
| `core/dockerfile/generate.go` | `BuildPlan` → Dockerfile string |
| `core/dockerfile/golden_test.go` | Golden file harness (`UPDATE_GOLDEN=true`) |
| `core/plan/plan.go` | `BuildPlan`, `Step`, `Deploy` data structures |
| `core/plan/command.go` | `Command` types (Exec, Copy, File, Path) |
| `core/plan/layer.go` | `Layer` type + special value handling |
| `core/generate/context.go` | `GenerateContext` — step/deploy builders |
| `core/app/app.go` | `App` — file system abstraction |
| `core/app/environment.go` | `Environment` — `THEOPACKS_*` variable access |
| `core/config/config.go` | Config structure + merging logic |
| `core/providers/provider.go` | `Provider` interface + registry |
| `core/providers/node/workspace.go` | Node monorepo detection (turbo/pnpm/npm workspaces) |
| `cmd/theopacks-generate/main.go` | CLI entry point used by Argo Workflow |
| `e2e/e2e_test.go` | E2E suite (build tag `e2e`) |
| `mise.toml` | Development tasks (root) |
| `Dockerfile.generate` | Container image that ships `theopacks-generate` |

---

## Acknowledgements

theo-packs is **derived from [Railpack](https://github.com/railwayapp/railpack)** by Railway Corporation and the Railpack contributors, released under the Apache License, Version 2.0.

The pieces inherited from Railpack include — but are not limited to — the `Provider` interface, the `BuildPlan` / `Step` / `Layer` / `Command` data model, the language-specific providers (Go, Node.js, Python, static, shell), the `App` file-system abstraction, the configuration-merging strategy, and large parts of the test fixtures used by the example projects.

What this fork did differently:

- Dropped the upstream CLI and direct BuildKit/LLB code path. theo-packs emits a Dockerfile and the Theo build cluster (Argo Workflow + external BuildKit/Kaniko) takes it from there.
- Collapsed the two-module layout (`core` + `railpack`) into a single module rooted at `github.com/usetheo/theopacks`, plus the `cmd/theopacks-generate` binary used by the build pipeline.
- Added workspace-aware Dockerfile generation for Node monorepos (turbo / pnpm / npm workspaces) driven by `THEOPACKS_APP_NAME` / `THEOPACKS_APP_PATH`.
- Renamed the configuration namespace from `RAILPACK_*` / `railpack.json` to `THEOPACKS_*` / `theopacks.json`.
- Trimmed the dependency surface to what the Theo build environment needs.

When touching code that mirrors upstream Railpack design, prefer staying close to the upstream conventions — it makes future merges easier and respects the work this project is built on. See `NOTICE` at the repo root for the formal attribution required by the license.
