# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/)
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
