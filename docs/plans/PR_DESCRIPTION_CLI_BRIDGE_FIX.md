# Fix: bridge `--app-name` to `THEOPACKS_APP_NAME` for ALL workspaces

> Compatibility fix surfaced while validating theo-packs against the 19 templates in `theo-stacks`. Three monorepo templates (`monorepo-php`, `monorepo-ruby`, `monorepo-rust`) errored with "set THEOPACKS_APP_NAME to one of: ..." even when the CLI was invoked with `--app-name=api`. Root cause: the env-var bridge from `--app-name` was Node-only.

## What broke

Validation matrix before this fix (19 theo-stacks templates):

| Template family | Status |
|---|---|
| Single-app (go-api, rust-axum, java-spring, ruby-sinatra, php-slim, python-fastapi, node-{express,fastify,nestjs,nextjs,worker}) | 13/13 OK |
| Node monorepos (fullstack-nextjs, monorepo-turbo) | 2/2 OK (Node bridge fires) |
| Other monorepos (monorepo-go via go.work, monorepo-java, monorepo-python via uv) | 3/3 OK (whole-workspace builds) |
| **Other monorepos requiring app selection (monorepo-php, monorepo-ruby, monorepo-rust)** | **0/3 FAIL** |

After this fix: 19/19 OK.

## Root cause

`cmd/theopacks-generate/main.go` only bridged `--app-name` → `THEOPACKS_APP_NAME` when the source root looked like a Node workspace (`turbo.json` / `pnpm-workspace.yaml` / `package.json#workspaces`). For Cargo workspaces, Ruby/PHP `apps/+packages/`, Gradle subprojects, .NET solutions, and Deno workspaces, providers received an empty `APP_NAME` and surfaced their usual multi-app error.

A second issue: `core/app/environment.go` doesn't consult `os.Getenv`. The `Environment.Variables` map is only what the CLI explicitly populates. So even when a user exported `THEOPACKS_APP_NAME=api` in their shell, providers couldn't see it.

## Fix

Two changes in `cmd/theopacks-generate/main.go`:

1. **Bridge `--app-name` and `--app-path` unconditionally** when non-empty. The Node-only conditional (CHG-002b) is now scoped to `analyzeDir` redirection only — the env-var population is universal.

2. **Default `--app-name` changed from `"app"` to `""`**. The previous default caused workspace flows to look for a literal app named `"app"` when the user didn't specify one. Empty string is the correct "unspecified" sentinel — providers handle it via the documented "set THEOPACKS_APP_NAME to one of: ..." error path.

## Tests

Four new CLI integration tests in `cmd/theopacks-generate/main_test.go`:

- `TestCLI_BridgesAppNameToCargoWorkspace` — Cargo workspace + `--app-name=api` produces `cargo build --release -p api`
- `TestCLI_BridgesAppNameToRubyMonorepo` — `apps/+packages/` Ruby monorepo accepts `--app-name=api`
- `TestCLI_BridgesAppNameToPhpMonorepo` — same for PHP
- `TestCLI_NoBridgeWhenAppNameEmpty` — single-app projects without `--app-name` flag work as before (regression guard for the empty-default change)

## Validation against theo-stacks

Manual run against all 19 templates with the fixed binary:

```
OK   fullstack-nextjs        OK   monorepo-rust         OK   php-slim
OK   go-api                  OK   monorepo-turbo        OK   python-fastapi
OK   java-spring             OK   node-express          OK   ruby-sinatra
OK   monorepo-go             OK   node-fastify          OK   rust-axum
OK   monorepo-java           OK   node-nestjs
OK   monorepo-php            OK   node-nextjs
OK   monorepo-python         OK   node-worker
OK   monorepo-ruby
```

For monorepos with multiple apps (php, ruby, rust, fullstack-nextjs, turbo), `--app-name=api` was passed (matches the Theo CLI invocation pattern in production).

## Backward compatibility

- `--app-name` flag still accepted; only the default changed (`"app"` → `""`).
- Single-app projects that previously passed `--app-name=app` (the default) by accident now skip the bridge — this is the desired behavior, since they had no workspace to select into.
- Node workspace flow unchanged: the Node provider already accepted both empty and non-empty app names, and continues to.

## Quality gates

```
mise run test     ✓ all packages green
go vet -tags e2e  ✓ no warnings
golangci-lint     ✓ 0 issues
gofmt -l .        ✓ 0 files
```

## Stacking note

This PR is independent of #16 (`feat/build-correctness-speed-v2`). Both target `develop`; merge order doesn't matter. If #16 lands first, this PR rebases cleanly. If this PR lands first, #16 picks up the new tests on rebase.
