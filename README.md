<p align="center">
  <a href="https://usetheo.dev">
    <img src="https://usetheo.dev/logo.png" alt="Theo" height="80" />
  </a>
</p>

# theo-packs

Zero-configuration application builder that detects your project's language and framework, generates an optimized build plan, and emits a multi-stage `Dockerfile` ready to be built by any standard image builder. Part of the [Theo](https://usetheo.dev) platform.

## How It Works

theo-packs analyzes your source code, detects the language/framework, and generates a `BuildPlan` — a structured representation of every step needed to build and run your application as a container. The build plan is then rendered to a `Dockerfile`. No hand-written Dockerfile required.

```
Source code → Provider detection → BuildPlan → Dockerfile
```

The Dockerfile is consumed by the downstream Theo build pipeline (Argo Workflow + BuildKit/Kaniko). theo-packs itself does not build images.

## Supported Languages

| Language | Detection | Build / Frameworks | Version Sources |
|----------|-----------|--------------------|-----------------|
| **Go** | `go.mod`, `go.work` | Go modules, workspaces | `go.mod`, `THEOPACKS_GO_VERSION` |
| **Rust** | `Cargo.toml` | cargo, Cargo workspaces (`-p <pkg>`), static-binary runtime | `rust-toolchain.toml`, `Cargo.toml` `rust-version`, `THEOPACKS_RUST_VERSION` |
| **Java** | `build.gradle.kts`, `build.gradle`, `pom.xml` | Gradle (Kotlin DSL + Groovy), Maven, multi-module/subprojects, Spring Boot auto-detect, JRE runtime | `.java-version`, `gradle.properties`, build script toolchain, `pom.xml`, `THEOPACKS_JAVA_VERSION` |
| **.NET** | `*.csproj`, `*.fsproj`, `*.vbproj`, `*.sln` | dotnet CLI, solutions, ASP.NET vs console runtime routing | `global.json`, `<TargetFramework>`, `THEOPACKS_DOTNET_VERSION` |
| **Ruby** | `Gemfile` | Bundler, Rails / Sinatra / Rack auto-detect, `apps/+packages/` monorepo | `.ruby-version`, Gemfile `ruby` directive, `THEOPACKS_RUBY_VERSION` |
| **PHP** | `composer.json` | Composer, Laravel / Slim / Symfony auto-detect, `apps/+packages/` monorepo | `.php-version`, `composer.json` `require.php`, `THEOPACKS_PHP_VERSION` |
| **Python** | `requirements.txt`, `pyproject.toml`, `Pipfile`, `setup.py` | pip, poetry, pipenv, uv | `.python-version`, `runtime.txt`, `THEOPACKS_PYTHON_VERSION` |
| **Deno** | `deno.json`, `deno.jsonc` | Deno 2 runtime, `workspace` arrays, Fresh / Hono auto-detect | `deno.json`, `THEOPACKS_DENO_VERSION` |
| **Node.js** | `package.json` | npm, yarn, pnpm, bun, npm/pnpm/yarn workspaces, Turbo | `engines.node`, `.nvmrc`, `.node-version`, `THEOPACKS_NODE_VERSION` |
| **Static files** | `index.html` | -- | -- |
| **Shell** | `*.sh` | -- | -- |

Detection order is fixed (first match wins): **Go → Rust → Java → .NET → Ruby → PHP → Python → Deno → Node → Static → Shell**. Deno is intentionally placed before Node so projects shipping both `deno.json` and a npm-compat `package.json` route to Deno. Override with `theopacks.json` → `{ "provider": "node" }`.

## Project Structure

```
theo-packs/
├── core/                       # Library: detection + build plan + Dockerfile rendering
│   ├── app/                    # File system abstraction for project analysis
│   ├── config/                 # Configuration model (theopacks.json) and merging
│   ├── dockerfile/             # BuildPlan → Dockerfile string (with golden tests)
│   ├── generate/               # GenerateContext (step/deploy builders, caches)
│   ├── plan/                   # BuildPlan, Step, Layer, Command data structures
│   ├── providers/              # Language-specific detection and planning
│   └── resolver/               # Package version resolution
├── cmd/theopacks-generate/     # CLI binary used by the Theo build cluster
├── internal/utils/             # Shared internal helpers
├── e2e/                        # End-to-end tests (build tag `e2e`, real Docker)
├── examples/                   # 50+ reference projects (Go, Node, Python, Rust, Java, .NET, Ruby, PHP, Deno, shell, static)
├── mise.toml                   # Tasks: test, check, tidy
└── Dockerfile.generate         # Container image that ships the CLI
```

The repository is a **single Go module**: `github.com/usetheo/theopacks`.

## Library Usage

```go
import (
    "github.com/usetheo/theopacks/core"
    "github.com/usetheo/theopacks/core/app"
    "github.com/usetheo/theopacks/core/dockerfile"
)

a, _ := app.NewApp("/path/to/project")
env := app.NewEnvironment(&map[string]string{"NODE_ENV": "production"})

result := core.GenerateBuildPlan(a, env, &core.GenerateBuildPlanOptions{})
if !result.Success {
    // result.Logs contains structured diagnostics
    return
}

df, err := dockerfile.Generate(result.Plan)
// `df` is a complete multi-stage Dockerfile string ready to be written to disk
```

## CLI Usage

The single binary `theopacks-generate` is designed to run inside the Theo build cluster (one Argo Workflow step per app). It writes a Dockerfile to disk; image construction happens in a later step.

```bash
go run ./cmd/theopacks-generate \
  --source /workspace \
  --app-path apps/api \
  --app-name api \
  --output /workspace/Dockerfile.api
```

Behavior:

1. If `<source>/<app-path>/Dockerfile` exists, it is copied to `--output` (user-provided wins).
2. If the source root looks like a Node monorepo workspace (`turbo.json`, `pnpm-workspace.yaml`, `package.json#workspaces`), the **workspace root** is analyzed instead of the per-app subdirectory, and `THEOPACKS_APP_NAME` / `THEOPACKS_APP_PATH` are set so the Node provider scopes the build (e.g. `turbo run build --filter=<app>...`).
3. Otherwise the app directory is analyzed standalone.
4. The generated Dockerfile is written to `--output` and echoed to stdout for log capture.

## Configuration

Projects can be customized via `theopacks.json` at the analyzed app's root (JSONC — comments allowed):

```jsonc
{
  // Override auto-detected provider
  "provider": "node",

  // Extra system packages for the build step
  "buildAptPackages": ["git"],

  // Customize build steps
  "steps": {
    "build": {
      "commands": [
        { "exec": ["npm", "run", "build:prod"] }
      ]
    }
  },

  // Runtime configuration
  "deploy": {
    "startCommand": "node dist/server.js",
    "aptPackages": ["curl"],
    "variables": { "NODE_ENV": "production" }
  },

  // Pin package versions
  "packages": { "nodejs": "20" }
}
```

### Version Detection

theo-packs picks the language version (and matching Docker base image) from your project files. If nothing is specified, sensible defaults are used (Node 20, Python 3.12, Go 1.23).

**Priority order (highest wins):**

1. `theopacks.json` `packages` field, or `THEOPACKS_PACKAGES` env var
2. Language-specific env var (`THEOPACKS_NODE_VERSION`, `THEOPACKS_PYTHON_VERSION`, `THEOPACKS_GO_VERSION`)
3. Project version files (`.nvmrc`, `.python-version`, `go.mod`, `engines.node`, `runtime.txt`)
4. Default version

**Examples:**

```bash
# Via version files (just add the file to your project)
echo "18" > .nvmrc                    # → FROM node:18-bookworm
echo "3.11" > .python-version         # → FROM python:3.11-bookworm

# Via environment variable
THEOPACKS_NODE_VERSION=22             # → FROM node:22-bookworm
THEOPACKS_PYTHON_VERSION=3.9          # → FROM python:3.9-bookworm
THEOPACKS_GO_VERSION=1.21             # → FROM golang:1.21-bookworm

# Via theopacks.json
{ "packages": { "node": "22" } }      # → FROM node:22-bookworm

# Go version is auto-detected from go.mod
# go 1.22                             # → FROM golang:1.22-bookworm
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `THEOPACKS_START_CMD` | Override start command |
| `THEOPACKS_BUILD_CMD` | Override build command |
| `THEOPACKS_INSTALL_CMD` | Override install command |
| `THEOPACKS_PACKAGES` | Space-separated package versions (e.g. `node@20 python@3.11`) |
| `THEOPACKS_NODE_VERSION` | Override Node.js version for base image |
| `THEOPACKS_PYTHON_VERSION` | Override Python version for base image |
| `THEOPACKS_GO_VERSION` | Override Go version for base image |
| `THEOPACKS_RUST_VERSION` | Override Rust version for build image |
| `THEOPACKS_JAVA_VERSION` | Override Java major version (build + JRE runtime) |
| `THEOPACKS_DOTNET_VERSION` | Override .NET SDK major.minor version |
| `THEOPACKS_RUBY_VERSION` | Override Ruby major.minor version |
| `THEOPACKS_PHP_VERSION` | Override PHP major.minor version |
| `THEOPACKS_DENO_VERSION` | Override Deno major version |
| `THEOPACKS_BUILD_APT_PACKAGES` | Extra apt packages for build |
| `THEOPACKS_DEPLOY_APT_PACKAGES` | Extra apt packages for runtime |
| `THEOPACKS_CONFIG_FILE` | Custom config file path (relative to app root) |
| `THEOPACKS_GO_MODULE` | Go workspace: which module to build |
| `THEOPACKS_APP_NAME` | Workspace-aware build: app/member/subproject name (Rust Cargo workspaces, Java Gradle subprojects / Maven modules, .NET solutions, Ruby/PHP `apps/+packages/`, Deno workspaces) |
| `THEOPACKS_APP_PATH` | Workspace-aware build: app path inside workspace (set automatically by the CLI on Node monorepo detection; manually for other languages) |

## Development

### Prerequisites

- Go 1.25+
- [Mise](https://mise.jdx.dev/) (recommended)
- Docker (only required for E2E tests)

### Common Tasks

All Mise tasks are defined in `mise.toml` at the repo root and run from the root.

```bash
mise run test     # go test ./core/... ./cmd/...
mise run check    # go vet + go fmt + golangci-lint
mise run tidy     # go mod tidy
```

### End-to-End Tests

E2E tests are gated behind the `e2e` build tag and build real Docker images from `examples/`. They are skipped automatically if Docker is not running.

```bash
# All E2E tests
go test -tags e2e ./e2e/ -timeout 600s

# A single example
go test -tags e2e ./e2e/ -run "TestE2E/node-npm" -timeout 600s
```

### Golden Dockerfiles

Dockerfile golden files live in `core/dockerfile/testdata/`. Regenerate them after intentional changes — never edit by hand.

```bash
UPDATE_GOLDEN=true go test ./core/dockerfile/...
```

### Running the CLI Locally

```bash
go run ./cmd/theopacks-generate \
  --source examples/node-npm \
  --app-path . \
  --app-name demo \
  --output /tmp/Dockerfile.demo
```

### Adding a New Provider

1. Create a directory under `core/providers/<language>/`.
2. Implement the `Provider` interface:

   ```go
   type Provider interface {
       Name() string
       Detect(ctx *generate.GenerateContext) (bool, error)
       Initialize(ctx *generate.GenerateContext) error
       Plan(ctx *generate.GenerateContext) error
       CleansePlan(buildPlan *plan.BuildPlan)
       StartCommandHelp() string
   }
   ```

3. Register the provider in `core/providers/provider.go` → `GetLanguageProviders()`. Order matters — first match wins.
4. Add example projects under `examples/<language>-*/` so the E2E suite picks them up.
5. Write unit tests in `core/providers/<language>/*_test.go`.
6. Regenerate golden Dockerfiles for any new integration cases: `UPDATE_GOLDEN=true go test ./core/dockerfile/...`.

## Acknowledgements

theo-packs is derived from **[Railpack](https://github.com/railwayapp/railpack)** by [Railway](https://railway.com) and the Railpack contributors, released under the Apache License, Version 2.0.

The provider interface, build-plan model, language-specific providers (Go, Node.js, Python, static, shell), file-system abstraction, configuration merging, and many of the example projects originate from Railpack. We are grateful to the Railway team and the Railpack community for publishing their work under a permissive license — without it, this project would not exist.

Modifications made for the Theo platform are summarized in [`NOTICE`](NOTICE) at the repo root.

## License

Apache License 2.0 -- see [LICENSE](LICENSE) for details. Attribution for upstream work is in [`NOTICE`](NOTICE).
