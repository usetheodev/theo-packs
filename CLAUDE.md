# CLAUDE.md — theo-packs

This file provides guidance to Claude Code when working with code in this repository.

---

## What This Project Is

theo-packs is a **zero-configuration application builder** that detects your project's language/framework and generates an optimized build plan for containerization. It is the language detection and Dockerfile generation engine for the [Theo](https://usetheo.dev) Kubernetes PaaS.

The project has two Go modules:

| Module | Path | Purpose |
|--------|------|---------|
| `github.com/usetheo/theopacks` | `core/` | Library: language detection, build plan generation, Dockerfile generation. Minimal dependencies. |
| `github.com/railwayapp/railpack` | `railpack/` | CLI + BuildKit integration. Converts build plans to LLB and builds container images. |

**Key flow:**

```
Source code → Provider.Detect() → Provider.Plan() → BuildPlan → dockerfile.Generate() → Dockerfile
                                                              → BuildKit LLB → Container image
```

---

## Development Commands

```bash
# All commands run from railpack/ directory using Mise
cd railpack

mise run setup                # Bootstrap: install tools, start BuildKit, tidy modules
mise run check                # Lint + format (go vet, go fmt, golangci-lint)
mise run test                 # Unit tests (all)
mise run tidy                 # go mod tidy

# CLI development
mise run cli -- plan examples/node-bun              # Generate build plan JSON
mise run cli -- build examples/node-bun --show-plan # Build image via BuildKit
mise run cli -- info examples/node-bun              # Show detected config

# Integration tests (slow — runs Docker builds)
mise run test-integration -- -run "TestExamplesIntegration/node-vite"  # Single example
cd examples/node-bun && mise run test-integration-cwd                  # From example dir

# Update test snapshots
mise run test-update-snapshots

# Core module tests (from repo root)
cd core && go test ./...
```

**Prerequisites:** Go 1.25+, Docker, [Mise](https://mise.jdx.dev/)

---

## The Rules

### Rule 1: Use `mise run`, not `go` directly
Never run `go test`, `go vet`, `go fmt`, or `go build` directly. Always use `mise run <task>`. Check `railpack/mise.toml` for available tasks.

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

This enables caching, testing, and future remote file system support.

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

---

## Architecture

### Core Library (`core/`)

```
core/
├── core.go             # Entry point: GenerateBuildPlan()
├── validate.go         # Plan validation (start command, steps, inputs)
├── app/                # File system abstraction (App, Environment)
├── config/             # Config model (theopacks.json) + merging
├── dockerfile/         # BuildPlan → Dockerfile string conversion
├── generate/           # GenerateContext: step builders, deploy builder, caches
├── logger/             # Logger with Info/Warn/Error levels
├── plan/               # Data structures: BuildPlan, Step, Layer, Command, Deploy
├── providers/          # Provider interface + language-specific implementations
│   ├── provider.go     # Interface definition + registry
│   ├── golang/         # Go provider (go.mod, go.work)
│   ├── node/           # Node.js (npm, yarn, pnpm, bun, workspaces)
│   ├── python/         # Python (pip, poetry, pipenv, uv)
│   ├── staticfile/     # Static HTML files
│   └── shell/          # Shell scripts (fallback)
└── resolver/           # Package version resolution
```

### Provider Detection Order

Providers are checked in this order (first match wins):

1. **Go** — `go.mod` or `go.work`
2. **Python** — `requirements.txt`, `pyproject.toml`, `Pipfile`, `setup.py`
3. **Node.js** — `package.json`
4. **Static files** — `index.html`
5. **Shell** — `*.sh` files

This order matters. Override via `theopacks.json`:
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

1. **File** — `theopacks.json` at project root (JSONC format, comments allowed)
2. **Environment** — `THEOPACKS_*` variables
3. **Options** — programmatic `GenerateBuildPlanOptions`

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

### Layer Special Values

When parsing layer references in JSON:
- `"."` → local files (project source)
- `"..."` → spread operator (merge/append semantics)
- `"$stepname"` → reference to a previous build step
- Any other string → Docker image reference

---

## Testing

### Test Framework
- **testify** (`require` package) for assertions — not vanilla `testing`
- Table-driven tests with `t.Run()` for subtests
- `t.Parallel()` where safe
- Arrange-Act-Assert pattern

### Golden Files
- Located in `testdata/` directories
- Update with `UPDATE_GOLDEN=true` or `UPDATE_SNAPS=true`
- Never manually edit golden files — regenerate them

### Integration Tests (`railpack/examples/`)

Each example project has a `test.json` (JSONC format):

```jsonc
[
  // Basic output check
  { "expectedOutput": "hello world" },

  // HTTP endpoint check
  {
    "httpCheck": {
      "path": "/",
      "expected": 200,
      "internalPort": 3000
    }
  },

  // Build-only (no runtime check)
  { "justBuild": true },

  // With environment variables
  {
    "expectedOutput": "connected",
    "envs": { "DATABASE_URL": "postgresql://..." }
  },

  // Expected failure
  { "shouldFail": true }
]
```

Optional `docker-compose.yml` for service dependencies (postgres, redis, etc.).

### Running Tests

```bash
# Unit tests only
mise run test

# Single integration test
mise run test-integration -- -run "TestExamplesIntegration/python-uv"

# From example directory
cd examples/node-bun && mise run test-integration-cwd
```

---

## Adding a New Provider

1. Create `core/providers/<language>/` directory
2. Implement the `Provider` interface:

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

3. Register in `core/providers/provider.go` → `GetLanguageProviders()` (order matters)
4. Add example projects in `railpack/examples/<language>-*/` with `test.json`
5. Write unit tests in `core/providers/<language>/*_test.go`

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
| Running `go test` directly | Use `mise run test` |
| Editing lockfiles manually | Use the package manager (`npm install`, `yarn`, etc.) |
| `fmt.Errorf("error: %s", err)` | Use `%w` to preserve error chain |
| Starting comment with function name | Describe behavior, not the function signature |
| Deep `if` nesting | Early return on error conditions |
| Assuming provider order doesn't matter | First match wins — order in `GetLanguageProviders()` is intentional |
| Manually editing golden/snapshot files | Run `mise run test-update-snapshots` |

---

## Key References

| File | What It Does |
|------|-------------|
| `core/core.go` | `GenerateBuildPlan()` — main library entry point |
| `core/validate.go` | Plan validation + helpful error messages |
| `core/dockerfile/generate.go` | `BuildPlan` → Dockerfile string |
| `core/plan/plan.go` | `BuildPlan`, `Step`, `Deploy` data structures |
| `core/plan/command.go` | `Command` types (Exec, Copy, File, Path) |
| `core/plan/layer.go` | `Layer` type + special value handling |
| `core/generate/context.go` | `GenerateContext` — step/deploy builders |
| `core/app/app.go` | `App` — file system abstraction |
| `core/app/environment.go` | `Environment` — `THEOPACKS_*` variable access |
| `core/config/config.go` | Config structure + merging logic |
| `core/providers/provider.go` | `Provider` interface + registry |
| `railpack/mise.toml` | All development tasks |
| `railpack/integration_tests/run_test.go` | Integration test runner |
