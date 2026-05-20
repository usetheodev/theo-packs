# PR Description — feat/add-six-language-providers

> **Ready-to-paste body** for the pull request from `feat/add-six-language-providers` into `main`.
>
> Open the PR via:
> - `gh pr create --base main --head feat/add-six-language-providers --title "feat: add Rust, Java, .NET, Ruby, PHP, and Deno language providers" --body-file docs/plans/PR_DESCRIPTION.md`
> - or paste this content into the GitHub web UI at:
>   `https://github.com/usetheodev/theo-packs/pull/new/feat/add-six-language-providers`

---

## Summary

Adds six new language providers to theo-packs (**Rust, Java, .NET, Ruby, PHP, Deno**) plus monorepo support and framework auto-detection. Closes the gap between theo-packs and the languages shipped in `theo-stacks` templates.

Provider count: **5 → 11** (Go, Rust, Java, .NET, Ruby, PHP, Python, Deno, Node, Static, Shell — first-match-wins, **Deno before Node** by ADR D3).

Plan: [docs/plans/add-six-language-providers-plan.md](../../docs/plans/add-six-language-providers-plan.md)

## What's in here

Per-language commits land in this order (bisectable):

| Commit | Phase | Scope |
|--------|-------|-------|
| `ed94387` | Phase 0 | 12 base-image helpers + 6 default-version constants + 6 stub providers + registration order (D3) |
| `674206c` | Phase 1 | Rust: `Cargo.toml` + Cargo workspaces (with glob expansion), single-bin / library detection, version from `rust-toolchain.toml`/`rust-toolchain`/Cargo.toml, static-binary debian-slim runtime |
| `091c61e` | Phase 2 | Java: Gradle Kotlin DSL + Groovy + Maven; Spring Boot detection; Gradle subprojects + Maven multi-module; fat-JAR copy from `build/libs/*.jar` or `target/*.jar` to `/app/app.jar` on `eclipse-temurin:<v>-jre` |
| `8149711` | Phase 3 | .NET: `*.csproj`/`*.fsproj`/`*.vbproj` plus `*.sln`; ASP.NET vs console runtime routing; Windows backslash normalization in solution paths; project selection via `THEOPACKS_APP_NAME` |
| `6c19922` | Phase 4 | Ruby: Bundler install; Rails / Sinatra / Rack auto-detection; `apps/+packages/` monorepo with Procfile; default Ruby 3.3 |
| `60609de` | Phase 5 | PHP: Composer install; Laravel / Slim / Symfony auto-detection; `apps/+packages/` monorepo; default PHP 8.3 |
| `2be1613` | Phase 6 | Deno: `deno.json`/`deno.jsonc` + Deno 2 `workspace` arrays; Fresh / Hono auto-detection; deno cache warmup; default Deno 2 |
| `299dd26` | Phase 7 | CHANGELOG.md, README.md, CLAUDE.md, NOTICE updated. Per-provider Railpack derivation table added to NOTICE. |
| `0c3b574` | Phase 8-9 | 12 new E2E tests + gofmt + golden review |
| `f047308` | Cleanup | staticcheck ST1005 — lowercase first word of error messages |
| `f2232e5` | Cleanup | Split java_test.go (441→347) and dotnet_test.go (378→291) to satisfy ≤350-line DoD |
| `41c0e1b` | Cleanup | Build tag `e2e` on `e2e_test.go` + Mise wrappers (`test-e2e`, `test-update-snapshots`) |

## Stats

- **+5,500 net lines** of new code (providers + tests + examples + goldens + docs)
- **36 Go files** across 6 new provider packages
- **18 example projects** (3 per new language: simple + framework + workspace where applicable)
- **18 new golden Dockerfiles** in `core/dockerfile/testdata/`
- **12 new E2E test functions** in `e2e/e2e_test.go`
- **0 file in new providers exceeds 350 lines**
- **0 lint warnings** (`golangci-lint v2.11.4`)
- **0 vet warnings** (`go vet ./...`)
- **All ~135 unit tests pass** across the 6 new packages
- **All ~47 integration tests pass** (29 pre-existing unchanged + 18 new)
- **All 5 pre-existing providers unchanged** (no regression)
- **Statement coverage** on the 6 new providers (`go test -cover`):
  - rust: **92.0%**, java: **91.7%**, deno: **91.2%**, dotnet: **90.1%**, ruby: **89.6%**, php: **87.3%** — average ~90%
- **Race-detector clean**: `go test -race -shuffle=on ./...` green; no concurrency issues
- **Order-independent**: `go test -count=3 -shuffle=on` green for providers + dockerfile (no state leakage between repeated runs)

## ADRs (selected — see plan for full list)

- **D1** — Keep language-specific base images (`rust:1-bookworm`, `eclipse-temurin:21-jre`, `ruby:3.3-bookworm-slim`, `php:8.3-cli-bookworm`, `mcr.microsoft.com/dotnet/{sdk,aspnet,runtime}:8.0`, `denoland/deno:bin-2`) instead of migrating to mise. Aligns with theo-stacks templates and avoids regenerating ~30 pre-existing golden Dockerfiles.
- **D2** — Port detection logic from Railpack upstream; rewrite `Plan()` bodies fresh against theo-packs' base-image strategy. Each ported file carries `SPDX-License-Identifier: Apache-2.0` plus a "Portions derived from github.com/railwayapp/railpack" comment. NOTICE has a per-provider attribution table.
- **D3** — Provider registration order: Go → Rust → Java → .NET → Ruby → PHP → Python → Deno → Node → Static → Shell. Deno before Node so projects shipping both `deno.json` and an npm-compat `package.json` route correctly to Deno. Locked by `TestRegistrationOrder`.
- **D5** — Framework auto-detection per language: Rust (Axum/actix-web/Rocket), Java (Spring Boot), .NET (ASP.NET Core), Ruby (Rails / Sinatra / Rack), PHP (Laravel / Slim / Symfony), Deno (Fresh / Hono). Procfile `web:` line takes precedence on every interpreted language.
- **D7** — Reuse the existing `THEOPACKS_APP_NAME` / `THEOPACKS_APP_PATH` environment variables for monorepo target selection across all six languages.

## Test plan

- [x] `mise run check` (= `go vet ./... && go fmt ./... && golangci-lint run`) — all green
- [x] `mise run test` (= `go test ./core/... ./cmd/...`) — 21 packages green
- [x] `golangci-lint run --max-issues-per-linter=0 ./...` — 0 issues
- [x] 18 new golden Dockerfiles generated via `mise run test-update-snapshots` and reviewed
- [x] All ~30 pre-existing golden Dockerfiles **byte-identical** (no regression)
- [x] `TestRegistrationOrder` locks the Deno-before-Node invariant
- [x] `TestRegistrationCount` asserts `len == 11`
- [x] `TestNamesAreUnique` asserts no duplicate provider names
- [ ] **Pending merge / Docker host:** `mise run test-e2e` (= `go test -tags e2e ./e2e/ -timeout 1500s`) — 12 new E2E test functions skip cleanly without Docker; would exercise real builds when Docker is available.
- [ ] **Pending merge:** integration with `theo-stacks` templates (`rust-axum`, `java-spring`, `ruby-sinatra`, `php-slim`, etc.) end-to-end on a Docker host.

## Notes for reviewers

- **Default versions** for Ruby (3.3) and PHP (8.3) are slightly newer than the corresponding `theo-stacks` templates (3.2 / 8.2). This bump is documented in the CHANGELOG `Notes` subsection. If parity is required, override via `THEOPACKS_RUBY_VERSION` / `THEOPACKS_PHP_VERSION`.
- **E2E tests** use `dockerAvailable()` skip pattern + `//go:build e2e` build tag — consistent with the existing pattern but stricter (now excluded from `go test ./...` entirely unless the tag is set).
- All ported logic carries SPDX-License-Identifier headers + a Railpack derivation note. Per-provider attribution is in `NOTICE`.
- Per-language commits are bisectable: if any one language regresses post-merge, that single commit can be reverted without affecting the others.

## Related

- ROADMAP context: theo-stacks Sprint 3 ("Advanced Templates → monorepos + fullstack + additional languages").
- Upstream: [Railpack](https://github.com/railwayapp/railpack) (source of detection logic for the 6 new providers).
