<p align="center">
  <a href="https://usetheo.dev">
    <img src="https://usetheo.dev/logo.png" alt="Theo" height="80" />
  </a>
</p>

# theo-packs

Zero-configuration application builder that automatically detects your project's language and framework, generates an optimized build plan, and produces container images via BuildKit. Part of the [Theo](https://usetheo.dev) platform.

## How It Works

theo-packs analyzes your source code, detects the language/framework, and generates a `BuildPlan` — a structured representation of all steps needed to build and deploy your application as a container. No Dockerfile required.

```
Source code → Provider detection → Build plan → BuildKit LLB → Container image
```

## Supported Languages

| Language | Detection | Package Managers | Version Sources |
|----------|-----------|------------------|-----------------|
| **Go** | `go.mod`, `go.work` | Go modules, workspaces | `go.mod`, `THEOPACKS_GO_VERSION` |
| **Node.js** | `package.json` | npm, yarn, pnpm, bun | `engines.node`, `.nvmrc`, `.node-version`, `THEOPACKS_NODE_VERSION` |
| **Python** | `requirements.txt`, `pyproject.toml`, `Pipfile`, `setup.py` | pip, poetry, pipenv, uv | `.python-version`, `runtime.txt`, `THEOPACKS_PYTHON_VERSION` |
| **Static files** | `index.html` | -- | -- |
| **Shell** | `*.sh` | -- | -- |

## Project Structure

```
theo-packs/
├── core/           # Library: language detection + build plan generation
│   ├── app/        # File system abstraction for project analysis
│   ├── config/     # Configuration (theopacks.json)
│   ├── generate/   # Build plan generation context
│   ├── plan/       # BuildPlan, Step, Layer, Command data structures
│   ├── providers/  # Language-specific detection and planning
│   └── resolver/   # Package version resolution
├── railpack/       # CLI + BuildKit integration
│   ├── cmd/cli/    # CLI entry point
│   ├── buildkit/   # Plan → BuildKit LLB conversion
│   ├── examples/   # 110+ example projects with integration tests
│   └── docs/       # Documentation site (Astro)
└── internal/       # Shared utilities
```

## Modules

### `core` (`github.com/usetheo/theopacks`)

Lightweight library with minimal dependencies. Analyzes source code and produces a JSON build plan.

```go
import (
    "github.com/usetheo/theopacks/core"
    "github.com/usetheo/theopacks/core/app"
)

a, _ := app.NewApp("/path/to/project")
env := app.NewEnvironment(map[string]string{"NODE_ENV": "production"})

result := core.GenerateBuildPlan(a, env, &core.GenerateBuildPlanOptions{
    StartCommand: "npm start",
})

if result.Success {
    // result.Plan contains the full BuildPlan
    // result.DetectedProviders shows which language was detected
}
```

### `railpack` (`github.com/railwayapp/railpack`)

Full CLI application that uses `core` for detection and BuildKit for image construction.

```bash
# Generate build plan (JSON)
railpack plan ./my-app

# Build container image
railpack build ./my-app --name my-image:latest

# Show detected configuration
railpack info ./my-app
```

## Configuration

Projects can be customized via `theopacks.json` at the project root:

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

theo-packs automatically detects the language version from your project files and selects the appropriate Docker base image. If no version is specified, sensible defaults are used (Node 20, Python 3.12, Go 1.23).

**Priority order** (highest wins):

1. `theopacks.json` `packages` field or `THEOPACKS_PACKAGES` env var
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
| `THEOPACKS_BUILD_APT_PACKAGES` | Extra apt packages for build |
| `THEOPACKS_DEPLOY_APT_PACKAGES` | Extra apt packages for runtime |
| `THEOPACKS_CONFIG_FILE` | Custom config file path |

## Development

### Prerequisites

- Go 1.25+
- Docker
- [Mise](https://mise.jdx.dev/) (recommended)

### Setup

```bash
cd railpack
mise run setup    # Installs tools, starts BuildKit container, tidies modules
```

### Common Tasks

```bash
# Lint and format
mise run check

# Unit tests
mise run test

# Run CLI in development
mise run cli -- build examples/node-vite-react

# Integration test for a specific example
mise run test-integration -- -run "TestExamplesIntegration/python-uv"

# Integration test from within an example directory
cd examples/node-bun && mise run test-integration-cwd

# Update test snapshots
mise run test-update-snapshots
```

### Adding a New Provider

1. Create a directory under `core/providers/<language>/`
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

3. Register the provider in `core/providers/provider.go` (`GetLanguageProviders`)
4. Add example projects under `railpack/examples/` with `test.json` for integration tests

## License

Apache License 2.0 -- see [LICENSE](LICENSE) for details.
