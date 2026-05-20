# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/)
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- **Generic monorepo workspace redirect for non-Node languages**: Ruby,
  PHP, Python, Java, Rust monorepos that follow the `theo-stacks`
  convention (root-level manifest + `apps/<name>/` subdirs) now have
  `theopacks-generate` analyze the source root instead of the per-app
  subdir. Mirrors the Node CHG-002b redirect for the other language
  families. Detection via the presence of `Gemfile`, `composer.json`,
  `pyproject.toml`, `Cargo.toml`, `build.gradle*`, `settings.gradle*`,
  or `pom.xml` at root and absence of the same in the app subdir (#NNN)
- **`TestE2E_TheoStacksTemplates`** end-to-end gate against every one of
  the 19 upstream `theo-stacks/templates/` directories. Verifies that
  each template can be rendered, fed to `theopacks-generate`, and
  produces a Dockerfile claimed by the expected provider. Closes the
  drift gap where an upstream template change would silently break
  scaffolded projects (#NNN)
- `examples/node-fastify`, `examples/node-nestjs`, `examples/node-worker`
  mirroring the upstream theo-stacks templates of the same names (#NNN)
- `docs/theo-stacks-compatibility.md` — canonical matrix mapping every
  theo-stacks template to its theo-packs provider + in-repo example
  fixture, with the gating story (#NNN)
- `mise.toml::coverage` task running `go test -coverprofile`; CI L1 now
  uploads coverage to Codecov (#NNN)
- Mutation testing baseline recorded in `docs/benchmarks/mutation-baseline.txt`:
  `core/providers` 81.6% efficacy, `core/config` 100% efficacy (#NNN)

### Changed
- `.gremlins.yaml::timeout-coefficient` raised from 3 to 10 so property
  tests (rapid, 100 iterations per property) complete before the
  per-mutant timeout (#NNN)

## [0.5.0] - 2026-05-20

> Security & hardening release. Closes every CRITICAL / HIGH / MEDIUM / LOW
> finding from the 2026-05-20 deep review. See
> `docs/plans/deep-review-hardening-plan.md` for the full audit trail.

### Security
- **CVE-class — Path traversal via `--app-path`** (T1.2). The CLI now passes
  every flag through a restrictive allowlist (`^[A-Za-z0-9._/@-]+$` for
  paths, `^[@A-Za-z0-9_][A-Za-z0-9._@/-]*$` for names) and clamps the
  resolved app dir to the source root via `filepath.Abs` + prefix check.
  Reproduced pre-fix: `--app-path ../../etc` resolved to `/home/etc`.
- **CVE-class — Symlink follow in user-Dockerfile / .dockerignore checks**
  (T1.3). `os.Stat` replaced by `os.Lstat` + `ModeSymlink` check. A
  symlinked Dockerfile in the app dir now hard-fails with exit code 2
  instead of leaking the target's existence (or content, in earlier lenient
  versions of the contract).
- **CVE-class — Shell injection via `--app-path` / `--app-name`** (T1.1).
  Same input allowlist rejects shell metacharacters before any
  `fmt.Sprintf("cd %s && %s", appPath, ...)` interpolation can occur.
- **Secret-mount pollution** (T1.4). `core.GenerateConfigFromEnvironment`
  no longer promotes `THEOPACKS_APP_NAME`, `THEOPACKS_APP_PATH`,
  `THEOPACKS_*_VERSION`, and other internal configuration keys to
  `plan.Secrets`. Defense-in-depth against future providers opting back
  into `Step.Secrets = ["*"]` and emitting spurious
  `--mount=type=secret,id=THEOPACKS_*` lines in generated Dockerfiles.
- **`THEOPACKS_CONFIG_FILE` allowlist** (T2.4). Now rejects absolute
  paths, paths that escape the source root, and any extension other than
  `.json` / `.jsonc`.
- **Non-root runner** (T2.2). `Dockerfile.generate` creates user
  `theopacks` (UID 10001) and ends with `USER theopacks`. Reduces the
  blast radius of any path-traversal or parser bug that escapes upstream
  sanitization in the multi-tenant Argo build cluster.

### Added
- Typed `core.WorkspaceTarget` field on `GenerateBuildPlanOptions` (T3.1).
  Providers consume the monorepo target via `ctx.ResolveAppName()` /
  `ctx.ResolveAppPath()`; the legacy `THEOPACKS_APP_NAME` /
  `THEOPACKS_APP_PATH` env-var bridge remains as a backward-compat
  fallback during the deprecation window.
- `core/logger.Nop()` discards messages; replaces the historical
  `log ...*Logger` variadic hack.
- `logger.MaxLogs = 1000` cap on `Logger.Logs` — pathological plans can
  no longer inflate `BuildResult.Logs` without bound; a single warning
  replaces overflow.
- `dockerfile.DefaultWorkdir = "/app"` centralizes the in-container
  WORKDIR used by every generated stage.
- ADR-0001 (`docs/adrs/0001-tailscale-hujson-pseudo-version.md`)
  documenting the explicit acceptance of `tailscale/hujson` at its
  current pseudo-version.
- Typed errors `utils.TypeMismatchError` and `utils.ErrMergeNilDestination`
  surfaced by `MergeStructs`.

### Changed
- `internal/utils.MergeStructs` no longer swallows reflect panics via
  `recover() { %v }` (T2.1). Pre-validates destination + source types
  and returns a typed `*TypeMismatchError` / `ErrMergeNilDestination`;
  the remaining `recover` preserves the error chain via `%w`.
- `core/providers/node.DetectWorkspace(*App, *Logger)` and
  `core/providers/golang.parseGoWork(*App, *Logger)` now take an
  explicit non-nil logger (T3.3). Variadic `log ...*Logger` API removed.
  Pass `logger.Nop()` when output is not desired.
- `core/app.NewApp` collapses `Getwd + Join + Abs` into a single
  `filepath.Abs(path)` call (T3.5 L3).
- CLI exit codes documented and enforced via a single
  `fatal(code, format, args)` helper (T4.1): `0` success, `1` generic
  failure, `2` input invariant violated.

### Fixed
- Workspace env-var bridge no longer sets `THEOPACKS_APP_PATH=.` when
  the CLI is invoked with `--app-path .` (or omitted). Previously the
  value was bridged literally and could be misinterpreted by providers
  that branch on a non-empty `APP_PATH`. (#NNN)
- Multiple bare `return err` sites in `core/app/app.go`,
  `core/providers/python/python.go`, `core/core.go`,
  `core/plan/step.go`, `core/config/config.go`, and
  `core/providers/dotnet/dotnet.go` now wrap errors with `%w` and
  identifying context (T3.2). Restores `errors.Is` / `errors.As`
  traceability in operator-facing failures.
- Regenerated stale `examples/node-express/package-lock.json` so the
  E2E build no longer fails with `npm error code EUSAGE` (T4.2 L6).

### Removed
- Comment `//Force 1` from `core/validate.go:3` — residual CI
  force-push marker that violated Rule 4 (T0.1).

## [Pre-0.5.0 — Unreleased aggregate from develop]

### Changed (BREAKING)
- **theo-packs is now the single source of truth for Dockerfile generation.** The CLI rejects user-supplied Dockerfiles at `<source>/<app-path>/Dockerfile` with exit code 2 and an error message naming the offending path. The previous "user-Dockerfile takes precedence" behavior has been removed entirely. There is no override flag, no warning mode, no env var. A Dockerfile at the workspace root (`<source>/Dockerfile`, outside the analyzed app path) is NOT checked — it may legitimately exist for local development outside Theo. See `docs/contracts/theo-packs-cli-contract.md`, "Single source of truth" preamble, for the full rationale and edge cases (#NNN)

### Removed
- User-Dockerfile precedence path from `cmd/theopacks-generate/main.go`. The block that copied a user-supplied Dockerfile to `--output` has been replaced with a hard-fail (#NNN)
- "User-Dockerfile precedence" section from `docs/contracts/theo-packs-cli-contract.md`. Replaced by "Single source of truth" preamble + a "User-Dockerfile rejection" section + a new failure-modes table row (#NNN)
- `TestUserProvidedDockerfileTakesPrecedence` from `cmd/theopacks-generate/main_test.go`. Replaced by `TestUserProvidedDockerfileIsRejected` and `TestUserProvidedDockerfileAtWorkspaceRoot_IsNotRejected` (#NNN)

### Added
- Defensive header comment in every generated Dockerfile naming the producing provider and the expected `docker build` context. Turns cryptic `"/<path>": not found` build failures into self-explanatory output (#NNN)
- New public function `dockerfile.HeaderComment(providerName) string` returning the metadata block; renderer-emitted, not provider-emitted (#NNN)
- `BuildPlan.ProviderName` field carrying the detected provider into the renderer (#NNN)
- New canonical reference `docs/contracts/theo-packs-cli-contract.md` documenting CLI flags, env-var bridge, build-context invariant, user-Dockerfile precedence, `.dockerignore` rules, and what the calling pipeline must guarantee. Cross-linked from CLAUDE.md and README.md (#NNN)
- E2E test `TestE2E_MonorepoTurboFromStacks` building the real upstream `theo-stacks/templates/monorepo-turbo` template with `docker build` context = workspace root, locking the contract end-to-end. Skips cleanly without Docker or without theo-stacks checked out as a sibling (#NNN)
- Audit tests `TestGoldens_NoPerAppNodeModulesCopy` (and supporting positive/negative regex tests) locking the assertion that no Node workspace golden contains a per-app `node_modules` COPY pattern (F5 regression guard) (#NNN)
- Audit test `TestGoldens_HasProviderHeader` asserting every golden carries the defensive header (#NNN)
- `# syntax=docker/dockerfile:1` directive at the top of every generated Dockerfile so cache mounts are honored on every BuildKit-capable host regardless of the default frontend version (#NNN)
- Per-language default `.dockerignore` written to `<source>/.dockerignore` when none exists. Covers `.git/`, language-specific build outputs (`node_modules/`, `target/`, `__pycache__/`, etc.), tooling caches, and OS noise. User-supplied files are never overwritten or merged (#NNN)
- New public package `core/dockerignore/` with `DefaultFor(providerName) string` for library consumers (#NNN)
- E2E image-size assertions: `requireSizeLessThan(t, tag, maxMB)` helper, wired into the Node and Python E2E tests with a 280 MB cap to lock the v2 size gains (#NNN)

### Changed
- **Node deploy stages now drop devDependencies** via `npm prune --omit=dev` (and `pnpm prune --prod` / `yarn install --production --ignore-scripts --prefer-offline`). Bun has no built-in prune subcommand and is unchanged (its hardlinked store is already lean). Estimated runtime image size reduction: 2-3× on typical apps (#NNN)
- **Java install step now warms the dep cache** via `gradle dependencies --no-daemon --refresh-dependencies` (Gradle) or `mvn -B -DskipTests dependency:go-offline` (Maven). Cold builds reuse the Docker layer cache when only application code changes between rebuilds (#NNN)
- Python local layer annotated with default excludes (`__pycache__`, `*.pyc`, `.pytest_cache`, `tests`, `.venv`, `.env`, `.git`, etc.). Effective via the generated `.dockerignore`; user-supplied `.dockerignore` continues to take precedence (#NNN)

### Notes
- User-supplied `.dockerignore` is never modified or merged with the default. To regenerate, delete the existing file and rerun the CLI.
- `.env` files are excluded from Python builds by default — security-positive change. Runtime config should use `THEOPACKS_*` env vars or a secrets backend.
- Bun projects are unaffected by the prune change because bun lacks a prune subcommand.
- Dogfood report findings F1, F2 (theo-stacks template bugs) and F6, F7 (theo product schema gap and silent-success in static delivery) are out of scope for this repo and tracked in their respective repos. F5 (claim that theo-packs generates per-app `node_modules` COPY) was disproven against the current generator — the bug exists in `theo-stacks/templates/monorepo-turbo/apps/api/Dockerfile`, which theo-packs passes through unchanged per the user-Dockerfile-precedence contract. F3 (build-context mismatch) is acknowledged in the new contract document; the systemic fix lives in the theo product (F6 — `build_context:` field separate from `path:` in `theo.yaml`).

### Fixed
- **CMD form**: Generated Dockerfiles no longer hardcode `CMD ["/bin/bash", "-c", ...]`. The renderer now emits exec form (`CMD ["/app/server"]`) when the start command is shell-feature-free, falling back to `["/bin/sh", "-c", ...]` (NEVER bash) when shell features are present. Bash is absent from `debian:bookworm-slim` and distroless images — the previous form would have failed to start. Exec form also makes the app PID 1 so SIGTERM propagates correctly. (#15)
- **Java JAR-extract quoting**: Fixed `RUN sh -c 'sh -c '...''` double-wrap that broke quoting in Gradle and Maven JAR-copy commands. Renderer now wraps in `sh -c '...'` exactly once. (#15)
- **Ruby `bundle config` quoting**: Replaced `--local without 'development test'` with the colon-separated form `without development:test`. The single-quote-in-single-quote form survived only because bundler tolerated the noise. (#15)
- **Spurious secret mounts**: `--mount=type=secret` is no longer attached to RUNs that don't reference the secret. The renderer auto-detects `$NAME` / `${NAME}` via substring match with token-boundary checking. (#15)

### Changed
- **Go runtime image**: `debian:bookworm-slim` (~80MB) → `gcr.io/distroless/static-debian12:nonroot` (~2MB). Nonroot user built in. (#15)
- **Rust runtime image**: `debian:bookworm-slim` → `gcr.io/distroless/cc-debian12:nonroot` (~17MB). cc-debian12 ships glibc + ca-certificates; Rust deploy no longer apt-installs ca-certificates. (#15)
- **BuildKit cache mounts** added to all 9 provider install/build steps. Cold rebuilds reuse cached downloads instead of re-fetching. (#15)
- **Per-language size flags**: Go `-ldflags="-s -w" -trimpath`, Rust `RUSTFLAGS="-C strip=symbols"`, .NET `-p:DebugType=None -p:DebugSymbols=false` (~25-30% smaller artifacts). (#15)
- **`plan.Command` gains a `Kind` field** (Shell vs Exec). `NewExecShellCommand` stores the body bare; the renderer wraps in `sh -c '...'` once at emit time. `NewExecCommand` emits `RUN <cmd>` raw. (#15)
- **`Step.Secrets` default** is now `nil` (was `["*"]`). Renderer auto-detects per-RUN. (#15)

### Added
- **Rust** language provider with single-crate and Cargo workspace support; default Rust 1, Axum-friendly multi-stage build emitting `/app/server` static binary on `debian:bookworm-slim` runtime; `THEOPACKS_RUST_VERSION`, `rust-toolchain.toml`, and `Cargo.toml#package.rust-version` honored for version pinning (#3)
- **Java** language provider supporting Gradle (Kotlin DSL + Groovy) and Maven, with single-module and multi-module/subproject layouts; auto-detects Spring Boot via plugin/starter signatures and ships a fat-JAR runtime on `eclipse-temurin:<v>-jre`; `THEOPACKS_JAVA_VERSION`, `.java-version`, `gradle.properties`, build script toolchain, and `pom.xml` properties honored for version pinning (#3)
- **.NET** language provider supporting `*.csproj`/`*.fsproj`/`*.vbproj` and `*.sln` solution files; routes ASP.NET projects to `dotnet/aspnet:<v>` and console/worker projects to the smaller `dotnet/runtime:<v>` image; `THEOPACKS_DOTNET_VERSION`, `global.json`, and `<TargetFramework>` honored for version pinning (#3)
- **Ruby** language provider with Bundler install, Rails / Sinatra / Rack auto-detection, and `apps/+packages/` monorepo support driven by Procfile; default Ruby 3.3 on `ruby:<v>-bookworm-slim`; `THEOPACKS_RUBY_VERSION`, `.ruby-version`, and `Gemfile` `ruby` directive honored for version pinning (#3)
- **PHP** language provider with Composer install, Laravel / Slim / Symfony auto-detection, and `apps/+packages/` monorepo support; default PHP 8.3 on `php:<v>-cli-bookworm`; `THEOPACKS_PHP_VERSION`, `.php-version`, and `composer.json#require.php` honored for version pinning (#3)
- **Deno** language provider supporting `deno.json` / `deno.jsonc` and Deno 2 `workspace` arrays; Fresh and Hono auto-detected for the start command; default Deno 2 on `denoland/deno:bin-<v>`; `THEOPACKS_DENO_VERSION` honored for version pinning (#3)
- New environment variables for the six providers above: `THEOPACKS_RUST_VERSION`, `THEOPACKS_JAVA_VERSION`, `THEOPACKS_DOTNET_VERSION`, `THEOPACKS_RUBY_VERSION`, `THEOPACKS_PHP_VERSION`, `THEOPACKS_DENO_VERSION` (#3)
- Twelve new base-image helper functions in `core/generate/images.go` covering build/runtime variants for the six new languages (#3)
- 18 example projects under `examples/` (3 per new language: simple + framework + workspace where applicable) plus 18 corresponding golden Dockerfiles in `core/dockerfile/testdata/` (#3)
- CI workflow for lint and test on push/PR (`ci.yml`) (#2)
- CI workflow to build and push `theo-packs-runner` image to DO Registry (`build-runner.yml`) (#2)
- Unit tests for `theopacks-generate` binary: Dockerfile generation, user-provided precedence, error messages, stdout output (#2)
- Scan de vulnerabilidades (Trivy) na imagem antes do push no `build-runner.yml`
- Smoke test na imagem (`--help`) antes do push no `build-runner.yml`
- Cache de Go modules e build artifacts nos workflows de CI
- golangci-lint no workflow de CI para análise estática além do `go vet`
- Dependabot para atualização automática de dependências Go e GitHub Actions
- Proteção de branch `main`: exige CI verde e 1 review antes de merge
- Tag com data (`YYYYMMDD`) nas imagens para facilitar rollback
- `mise.toml` na raiz do projeto com tasks `check` e `test`

### Changed
- Provider detection order now includes the six new languages and places **Deno before Node** so projects shipping both `deno.json` and an npm-compat `package.json` route correctly to Deno (#3)
- `theopacks-generate` and the library now register **11 language providers** (was 5): Go → Rust → Java → .NET → Ruby → PHP → Python → Deno → Node → Static → Shell (first-match-wins) (#3)
- Actions pinadas por commit SHA em vez de tags mutáveis para proteção contra supply chain attacks
- `build-runner.yml` agora roda testes antes de buildar e pushar a imagem
- `build-runner.yml` usa `docker/build-push-action` com cache GHA para builds incrementais
- CI ignora mudanças em docs e markdown para evitar execuções desnecessárias
- Workflows usam `mise run` em vez de comandos `go` diretos, consistente com Rule 1 do CLAUDE.md
