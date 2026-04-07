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

| Language | Detection | Package Managers |
|----------|-----------|------------------|
| **Go** | `go.mod`, `go.work` | Go modules, workspaces |
| **Node.js** | `package.json` | npm, yarn, pnpm, bun |
| **Python** | `requirements.txt`, `pyproject.toml`, `Pipfile`, `setup.py` | pip, poetry, pipenv, uv |
| **Static files** | `index.html` | -- |
| **Shell** | `*.sh` | -- |

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

### Environment Variables

| Variable | Description |
|----------|-------------|
| `THEOPACKS_START_CMD` | Override start command |
| `THEOPACKS_BUILD_CMD` | Override build command |
| `THEOPACKS_INSTALL_CMD` | Override install command |
| `THEOPACKS_PACKAGES` | Comma-separated package versions (e.g. `nodejs@20,npm@10`) |
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
