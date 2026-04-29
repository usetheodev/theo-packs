# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/)
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
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
