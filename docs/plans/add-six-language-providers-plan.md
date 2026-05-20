# Plan: Add Rust, Java, Ruby, PHP, .NET, and Deno Language Providers

> **Version 1.0** — Add six new language providers (Rust, Java, Ruby, PHP, .NET, Deno) to theo-packs so it can build container images for every language `theo-stacks` ships templates for. Detection logic, version resolution, and example projects are ported from Railpack upstream; the `Plan()` function is written fresh against theo-packs' base-image strategy. Single PR organized into ordered commits, including monorepo support, framework auto-detection (Spring Boot, Rails, Laravel, ASP.NET, Fresh, etc.), full unit/integration/E2E tests, and documentation/attribution updates.

> **File destination after approval:** move from `/home/paulo/.claude/plans/wild-bouncing-perlis.md` to `docs/plans/add-six-language-providers-plan.md` (per `to-plan` skill convention). The plan-mode harness only allows writing to the temp path during planning.

---

## Context

### What exists today

theo-packs is the language-detection and Dockerfile-generation engine for the Theo PaaS. As of `085ef84` (last commit before the doc cleanup branch), it ships **5 providers** registered in `core/providers/provider.go:22-30`:

```go
return []Provider{
    &golang.GoProvider{},
    &python.PythonProvider{},
    &node.NodeProvider{},
    &staticfile.StaticfileProvider{},
    &shell.ShellProvider{},
}
```

That covers 4 real programming languages: **Go, Node.js, Python, Shell**. Static is HTML serving. The repo has 30 example projects exercising those providers (`examples/go-*`, `examples/node-*`, `examples/python-*`, etc.), 30+ golden Dockerfiles in `core/dockerfile/testdata/`, and an E2E suite in `e2e/e2e_test.go` gated behind the `e2e` build tag.

### What's missing

The sister project `theo-stacks` (at `/home/paulo/Projetos/usetheo/theo-stacks`) ships scaffolding templates for **7 languages**: Node.js, Go, Python, **Rust, Java, Ruby, PHP**. Its README banner reads "Production-ready project scaffolding for Node.js, Go, Python, Rust, Java, Ruby, and PHP" but theo-packs cannot build 4 of them. Concretely, the following templates have a Dockerfile that the theo-packs generator cannot reproduce:

- `templates/rust-axum/` (Rust + Axum)
- `templates/java-spring/` (Java 21 + Spring Boot, Gradle Kotlin DSL)
- `templates/ruby-sinatra/` (Ruby 3.2 + Sinatra + Puma)
- `templates/php-slim/` (PHP 8.2 + Slim + Composer)
- Plus monorepo variants: `monorepo-rust`, `monorepo-java`, `monorepo-ruby`, `monorepo-php`.

In addition, the user has scoped the work to also include **.NET (ASP.NET Core)** and **Deno**, which are not yet in `theo-stacks` but are roadmap-aligned and standard in the Theo PaaS positioning.

### Evidence

- `theo-stacks/README.md` lists the 7 supported languages explicitly.
- `theo-stacks/templates/{rust-axum,java-spring,ruby-sinatra,php-slim}/Dockerfile` show the exact base images and start commands the theo-stacks team converged on.
- `theo-stacks/ROADMAP.md` Sprint 3 is titled "Advanced Templates → monorepos + fullstack + additional languages".
- `core/providers/provider.go` — only 5 providers registered; no Rust/Java/Ruby/PHP/.NET/Deno.
- Railpack upstream (`github.com/railwayapp/railpack/core/providers`) — confirmed to have providers for `cpp deno dotnet elixir gleam golang java node php procfile python ruby rust shell staticfile`. Six of those (`rust java ruby php dotnet deno`) are exactly what we need; the rest are out of scope.

### What this plan unlocks

- `npm create theo@latest` flows in `theo-stacks` produce a project that, once pushed to a Theo cluster, gets built end-to-end without a hand-written Dockerfile.
- The Theo CLI's `theo deploy` works for Rails, Laravel, Spring Boot, ASP.NET Core, Axum, and Deno applications.
- theo-packs continues to align with Railpack upstream where it makes sense (detection, version resolution) without inheriting Railpack's `mise`-based runtime model — see ADR D2 for why.

---

## Objective

**Done = `mise run test` is green, `go test -tags e2e ./e2e/` is green for all six new languages, and `theo-stacks` templates `rust-axum`, `java-spring`, `ruby-sinatra`, `php-slim` (plus a representative `dotnet-aspnet` and `deno-fresh`) all generate a buildable Dockerfile via `theopacks-generate` without hand-edits.**

Specific, measurable goals:

1. `core/providers/provider.go` registers 11 providers (5 existing + 6 new) in a deterministic, conflict-free order.
2. Six new provider packages (`core/providers/{rust,java,ruby,php,dotnet,deno}/`) each with: a `Provider` implementation, version detection, framework hints where applicable, monorepo workspace detection where applicable, and ≥80% line coverage in unit tests.
3. ≥15 new example projects under `examples/` covering simple + monorepo + framework variants per language.
4. ≥15 new golden Dockerfiles in `core/dockerfile/testdata/`.
5. ≥6 new entries in `e2e/e2e_test.go` (one per language minimum) producing buildable Docker images on a host with Docker available.
6. `CHANGELOG.md`, `README.md`, `CLAUDE.md`, and `NOTICE` updated to reflect the expanded language matrix and the specific Railpack provider derivations.
7. `mise run check` passes (zero `go vet`, `go fmt`, `golangci-lint` warnings).
8. Single PR with one internal commit per logical step (foundation → Rust → Java → … → docs → E2E) so the reviewer can bisect by language if needed.

---

## ADRs

### D1 — Keep language-specific base images; do not migrate to mise
**Decision:** Each new provider uses official language-specific Docker base images (`rust:1-bookworm`, `eclipse-temurin:21-jdk`, etc.) for build and runtime stages. We do NOT introduce `mise` as a runtime install mechanism the way Railpack upstream does.

**Rationale:** Migrating to mise would require rewriting the existing 5 providers and regenerating ~30 golden Dockerfiles, exploding scope. theo-stacks templates (the user-visible reference) already use language-specific base images — keeping the same convention preserves dev/build parity. Cold-build time stays low (no `mise install` round-trip). Surface for CVE tracking stays inherited from official images. User confirmed this approach in the planning conversation.

**Consequences:**
- Enables: incremental, per-language commits; isolated risk per provider; alignment with theo-stacks Dockerfiles.
- Constrains: poly-language projects (e.g., Rails + Node asset pipeline) need a custom path or a future hybrid; we maintain a small per-language image registry in `core/generate/images.go`.

### D2 — Port detection and version resolution from Railpack; rewrite `Plan()` fresh
**Decision:** Inputs to the Plan (manifest names, version-detection priorities, framework hints, fixture data) are ported from `github.com/railwayapp/railpack/core/providers/<lang>` under the existing Apache 2.0 attribution recorded in `NOTICE`. The body of `Plan()` is written from scratch using theo-packs' `ctx.NewCommandStep()` / `ctx.Deploy.AddInputs()` API and base-image helpers.

**Rationale:** Detection logic (which file means "this is a Rust project", which env var holds the version override) is the hard, error-prone part — porting it preserves correctness. The `Plan()` body diverges fundamentally because Railpack uses mise-driven LLB primitives while theo-packs renders Dockerfiles via base images.

**Consequences:**
- Enables: high confidence in detection edge cases; faster delivery; future merge of upstream fixes for detection.
- Constrains: every ported file must carry an SPDX header noting the derivation; `NOTICE` gains a per-provider attribution table.

### D3 — Provider registration order: Deno BEFORE Node, others independent
**Decision:** New registration order in `GetLanguageProviders()`:

```
Go → Rust → Java → .NET → Ruby → PHP → Python → Deno → Node → Static → Shell
```

**Rationale:** Most managed/compiled runtimes have unique manifest signatures (`Cargo.toml`, `*.csproj`, `Gemfile`, `composer.json`, `go.mod`, `pom.xml`, `build.gradle*`) and can be ordered any way. The single real conflict is **Deno vs Node**: a Deno project can ship a `package.json` for npm-compat but always has `deno.json` or `deno.jsonc`. Detecting Deno first guarantees correct routing. Static/Shell stay last as fallbacks.

**Consequences:**
- Enables: deterministic detection in mixed projects.
- Constrains: a future runtime that wants to claim `package.json` will need to be inserted before Node; the order is intentional and tested in `provider_test.go`.

### D4 — Default versions: LTS-aligned where possible, theo-stacks parity otherwise
**Decision:** Default versions when no project-level signal is found:

| Language | Default | Reason |
|----------|---------|--------|
| Rust | `1` (latest stable) | rust:1-bookworm tracks current stable |
| Java | `21` (LTS) | Matches theo-stacks; LTS until 2031 |
| Ruby | `3.3` | Current stable line; bump from theo-stacks 3.2 with note in CHANGELOG |
| PHP | `8.3` | Current stable; bump from theo-stacks 8.2 with note in CHANGELOG |
| .NET | `8.0` | LTS until Nov 2026 |
| Deno | `2` (latest major) | Tracks Deno 2 release line |

**Rationale:** Use LTS where the language has the concept; bump theo-stacks templates that lag, and document the bump.

**Consequences:**
- Enables: stable defaults that age gracefully.
- Constrains: `theo-stacks` templates need a follow-up alignment commit (out of this PR's scope) for ruby-sinatra and php-slim if the team wants exact parity. Coordinate via CHANGELOG entry.

### D5 — Framework auto-detection coverage
**Decision:** Each provider explicitly detects the most dev-friendly frameworks and falls back to a generic command otherwise:

| Language | Explicit | Generic fallback |
|----------|----------|------------------|
| Rust | Axum, actix-web, Rocket | `./target/release/<bin>` from Cargo.toml `[[bin]]` |
| Java | Spring Boot (`org.springframework.boot` plugin in build.gradle*; spring-boot-starter in pom.xml) | `java -jar build/libs/*.jar` (Gradle) or `java -jar target/*.jar` (Maven) |
| Ruby | Rails (`config/application.rb` + `gem "rails"`), Sinatra (`config.ru` + `gem "sinatra"`), Rack (`config.ru` only) | Procfile `web:` line, then error |
| PHP | Laravel (`artisan` file + `laravel/framework`), Slim (`slim/slim` in composer require) | Procfile `web:` line, then `php -S 0.0.0.0:$PORT -t public` |
| .NET | ASP.NET Core (`Microsoft.AspNetCore.*` package refs), generic console (`OutputType=Exe`) | `dotnet <Assembly>.dll` |
| Deno | Fresh (`fresh` import map), Hono, generic deno serve | `deno run -A main.ts` |

**Rationale:** Q4 from the planning conversation — "o que for mais Dev-Friendly". Framework hints turn detection from "you must set THEOPACKS_START_CMD" into "it just works".

**Consequences:**
- Enables: zero-config builds for the 80% case.
- Constrains: each new framework signal needs a unit test; false positives (e.g., a Sinatra-shaped non-Sinatra app) need a clear override path via `theopacks.json` `deploy.startCommand`.

### D6 — Single PR with one commit per logical step
**Decision:** All work lands in one PR. Internal commits split as:
1. `feat(images): add base image helpers for rust/java/ruby/php/dotnet/deno`
2. `feat(rust): add Rust provider with Cargo workspace support`
3. `feat(java): add Java provider with Gradle/Maven + Spring Boot detection`
4. `feat(dotnet): add .NET provider with solution + ASP.NET detection`
5. `feat(ruby): add Ruby provider with Rails/Sinatra detection`
6. `feat(php): add PHP provider with Laravel/Slim detection`
7. `feat(deno): add Deno provider with workspace support`
8. `test(integration): add golden Dockerfiles for new providers`
9. `test(e2e): add E2E builds for new providers`
10. `docs: update README, CLAUDE.md, CHANGELOG, NOTICE for new providers`

**Rationale:** Q6 — single PR. Internal commits give the reviewer bisectability and let CI fail fast on a single language without aborting the rest.

**Consequences:**
- Enables: focused review per language; rollback per commit if a provider regresses post-merge.
- Constrains: PR is large (estimated ~6000-8000 net lines); needs a thorough description and a per-language reviewer checklist.

### D7 — Monorepo target selection via existing `THEOPACKS_APP_NAME` / `THEOPACKS_APP_PATH`
**Decision:** Reuse the existing env var pair (already used by Node) for selecting the build target inside multi-app workspaces. Each language interprets them in context:
- **Java/Gradle:** `gradle :apps:${THEOPACKS_APP_NAME}:bootJar`
- **Java/Maven:** `mvn -pl apps/${THEOPACKS_APP_NAME} -am package`
- **Rust/Cargo:** `cargo build --release -p ${THEOPACKS_APP_NAME}`
- **.NET solution:** `dotnet publish ${THEOPACKS_APP_PATH}/${THEOPACKS_APP_NAME}.csproj`
- **Ruby/PHP (apps/+packages/ pattern):** `cd ${THEOPACKS_APP_PATH} && bundle install && exec ...` / equivalent

**Rationale:** Single namespace = simpler CLI (`cmd/theopacks-generate/main.go` already plumbs these for Node). Don't multiply env vars.

**Consequences:**
- Enables: uniform CLI surface; one set of monorepo conventions.
- Constrains: language-specific workspace concepts (Cargo `members`, Gradle subprojects, .NET solutions) all collapse onto the same env var pair — providers must validate that the named target exists in the workspace.

### D8 — Adapt `to-plan` template's TDD verification commands to Go toolchain
**Decision:** Where the to-plan template references `cargo test` / `pytest`, this plan uses theo-packs' actual quality gate: `mise run test` (unit), `mise run check` (vet + fmt + golangci-lint), `UPDATE_GOLDEN=true go test ./core/dockerfile/...` (golden refresh), `go test -tags e2e ./e2e/ -timeout 1200s` (E2E). The "code-audit" pseudo-checks in the template map to `mise run check` + golangci-lint defaults.

**Rationale:** Project conventions in `CLAUDE.md` Rule 1 require `mise run` over direct `go` invocations.

**Consequences:**
- Enables: tasks runnable as-is on this repo.
- Constrains: contributors must have Mise installed (already a stated prerequisite).

---

## Dependency Graph

```
Phase 0 (Foundation: image helpers + provider order)
    │
    ├──▶ Phase 1 (Rust)         ─┐
    ├──▶ Phase 2 (Java)         ─┤
    ├──▶ Phase 3 (.NET)         ─┼──▶ Phase 7 (Docs/Attribution)
    ├──▶ Phase 4 (Ruby)         ─┤              │
    ├──▶ Phase 5 (PHP)          ─┤              │
    └──▶ Phase 6 (Deno)         ─┘              ▼
                  │                       Phase 8 (Integration tests + golden files)
                  │                              │
                  └──────────────────────────────┴──▶ Phase 9 (E2E + final QA)
```

- **Phase 0** is a hard prerequisite for everything (image helpers are imported by all providers).
- **Phases 1-6** are mutually independent and can land in parallel commits/branches; serial commit order is chosen for review readability (D6).
- **Phase 7** can run partly in parallel with Phases 1-6 (CHANGELOG drafting can start immediately) but final doc updates wait for the language list to stabilize.
- **Phase 8** depends on all language providers being implemented.
- **Phase 9** depends on Phase 8 (integration tests must be green before spending time on E2E).

---

## Phase 0: Foundation — Image Helpers and Registration Order

**Objective:** Land the shared infrastructure (base image helper functions, default version constants, provider registry order) that every subsequent phase depends on.

### T0.1 — Add base image helper functions for the six new languages

#### Objective
Extend `core/generate/images.go` with `<Lang>BuildImageForVersion()` and `<Lang>RuntimeImageForVersion()` helpers (plus default version constants) so providers can request "the build image for Rust 1" without hardcoding strings.

#### Evidence
- The existing 3 language providers (golang, node, python) all call helpers from `core/generate/images.go` — see `golang.go:48` (`generate.GoBuildImageForVersion(version)`), `node.go` (`NodeBuildImageForVersion`/`NodeRuntimeImageForVersion`), `python.go` (analogous). Adding 6 more languages without helpers means string-literal sprawl.
- `core/generate/images.go` already exposes `NormalizeToMajor()` and `NormalizeToMajorMinor()` — reuse these for version normalization.

#### Files to edit
```
core/generate/images.go           — ADD helpers + DefaultXVersion constants for 6 new languages
core/generate/images_test.go      — ADD unit tests for normalization + image string assembly
```

#### Deep file dependency analysis
- **`core/generate/images.go`** is imported by every existing language provider's `Plan()`. Today it exports `GoBuildImageForVersion`, `NodeBuildImageForVersion`/`NodeRuntimeImageForVersion`, `PythonBuildImageForVersion`/`PythonRuntimeImageForVersion`, `GoRuntimeImage`, `NodeRuntimeImage`, `PythonRuntimeImage`, `StaticfileRuntimeImage`, `DefaultRuntimeImage`, `DefaultGoVersion`, `DefaultNodeVersion`, `DefaultPythonVersion`. Downstream consumers: `core/providers/golang/golang.go`, `core/providers/node/node.go`, `core/providers/python/python.go`. Adding new exports is purely additive — no consumer breaks.
- **`core/generate/images_test.go`** — table-driven tests that validate `NormalizeToMajor` / `NormalizeToMajorMinor` and image string assembly. Pure functions, no mocking needed.

#### Deep Dives

**Function signatures to add:**

```go
// Rust — static binary; build image is rust:<ver>-bookworm, runtime is debian:bookworm-slim
const DefaultRustVersion = "1"
const RustRuntimeImage = "debian:bookworm-slim"
func RustBuildImageForVersion(version string) string  // → "rust:1-bookworm"

// Java — distinct build (with JDK) vs runtime (JRE only). Use Eclipse Temurin.
const DefaultJavaVersion = "21"
func JavaJdkImageForVersion(version string) string    // → "eclipse-temurin:21-jdk"
func JavaJreImageForVersion(version string) string    // → "eclipse-temurin:21-jre"
func GradleImageForJavaVersion(version string) string // → "gradle:8-jdk21" (build w/ Gradle)
func MavenImageForJavaVersion(version string) string  // → "maven:3-eclipse-temurin-21"

// Ruby — single image for build + runtime (interpreted)
const DefaultRubyVersion = "3.3"
func RubyImageForVersion(version string) string       // → "ruby:3.3-bookworm-slim"

// PHP — single CLI image for build + runtime
const DefaultPhpVersion = "8.3"
func PhpImageForVersion(version string) string        // → "php:8.3-cli-bookworm"
const ComposerImage = "composer:2"                    // multi-stage helper

// .NET — distinct build SDK vs runtime
const DefaultDotnetVersion = "8.0"
func DotnetSdkImageForVersion(version string) string      // → "mcr.microsoft.com/dotnet/sdk:8.0"
func DotnetAspnetImageForVersion(version string) string   // → "mcr.microsoft.com/dotnet/aspnet:8.0"
func DotnetRuntimeImageForVersion(version string) string  // → "mcr.microsoft.com/dotnet/runtime:8.0"

// Deno — single image, official; distroless variant for runtime
const DefaultDenoVersion = "2"
func DenoImageForVersion(version string) string           // → "denoland/deno:bin-2"
func DenoRuntimeImageForVersion(version string) string    // → "denoland/deno:distroless-2"
```

**Invariants:**
- Every helper accepts a non-normalized version string and applies `NormalizeToMajor` (Rust, Java, .NET, Deno) or `NormalizeToMajorMinor` (Ruby, PHP) before assembling the tag.
- Empty string → returns the default-version image.
- Version strings already in canonical form (`"3.3"`) pass through.

**Edge cases:**
- `NormalizeToMajor("^21")` → `"21"` (Java caret range).
- `NormalizeToMajorMinor("3.3.5")` → `"3.3"` (Ruby patch level).
- `NormalizeToMajor("v8.0")` → `"8.0"` for .NET (which uses major.minor as "major").
- `.NET` uses `8.0` not `8` because Microsoft's tag scheme — handle this with a dedicated `NormalizeToDotnetMajor` if needed.

#### Tasks
1. Add `DefaultRustVersion`, `DefaultJavaVersion`, `DefaultRubyVersion`, `DefaultPhpVersion`, `DefaultDotnetVersion`, `DefaultDenoVersion` constants and `RustRuntimeImage`, `ComposerImage` constants.
2. Implement the 12 helper functions listed above with the documented behavior.
3. If `.NET`'s tag scheme proves it, add `NormalizeToDotnetMajor` (otherwise reuse `NormalizeToMajor`).
4. Update package doc comment in `images.go` to summarize the per-language image strategy.
5. Run `go vet ./core/generate/...` to catch any issues early.

#### TDD
```
RED:    TestRustBuildImageForVersion_Default       — empty/"1" → "rust:1-bookworm"
RED:    TestRustBuildImageForVersion_Specific      — "1.83" → "rust:1.83-bookworm"  (we keep precision when given)
RED:    TestJavaJdkImageForVersion                 — "21" → "eclipse-temurin:21-jdk"
RED:    TestJavaJreImageForVersion                 — "21" → "eclipse-temurin:21-jre"
RED:    TestGradleImageForJavaVersion              — "21" → "gradle:8-jdk21"
RED:    TestMavenImageForJavaVersion               — "21" → "maven:3-eclipse-temurin-21"
RED:    TestRubyImageForVersion                    — "3.3" → "ruby:3.3-bookworm-slim"
RED:    TestPhpImageForVersion                     — "8.3" → "php:8.3-cli-bookworm"
RED:    TestDotnetSdkImageForVersion               — "8.0" → "mcr.microsoft.com/dotnet/sdk:8.0"
RED:    TestDotnetAspnetImageForVersion            — "8.0" → "mcr.microsoft.com/dotnet/aspnet:8.0"
RED:    TestDenoImageForVersion                    — "2" → "denoland/deno:bin-2"
RED:    TestNormalizationOnAllNewLangs             — table-driven: verifies each helper applies normalization
GREEN:  Implement helpers and constants in core/generate/images.go
REFACTOR: Extract shared `imageForLang(repo, version, suffix)` helper if 3+ helpers diverge only in repo string. Otherwise: None expected.
VERIFY: mise run test (covers core/generate/images_test.go)
```

#### Acceptance Criteria
- [ ] All 12 helper functions exist with the signatures above.
- [ ] All 6 default-version constants exist with the values from D4.
- [ ] `RustRuntimeImage` constant exists for static-binary deploy.
- [ ] `ComposerImage` constant exists for PHP multi-stage build.
- [ ] All new tests added in `core/generate/images_test.go` pass.
- [ ] No regressions in existing `core/generate/images_test.go` tests.
- [ ] `mise run check` passes (zero `go vet` / `golangci-lint` warnings).
- [ ] File length: `images.go` stays under 500 lines.

#### DoD
- [ ] All tasks completed.
- [ ] `mise run test` green.
- [ ] `mise run check` green.
- [ ] Diff reviewed: no consumer of existing helpers broke.

---

### T0.2 — Register six new providers in correct order

#### Objective
Update `core/providers/provider.go` to register `RustProvider`, `JavaProvider`, `DotnetProvider`, `RubyProvider`, `PhpProvider`, `DenoProvider` in the order specified by ADR D3, and update `provider_test.go` to lock that order.

#### Evidence
- `core/providers/provider.go:22-30` is the single registration point; `core.GenerateBuildPlan` walks this list in order via `getProviders()` in `core/core.go:237-285`. Detection is first-match-wins.
- Deno conflict with Node is the only ordering-sensitive case (D3).

#### Files to edit
```
core/providers/provider.go        — REGISTER 6 new providers in the order from D3
core/providers/provider_test.go   — ADD test asserting full registration order
```

#### Deep file dependency analysis
- **`core/providers/provider.go`** — adds 6 import lines and 6 entries to the slice returned by `GetLanguageProviders()`. Downstream `core/core.go:237` iterates this slice; correctness depends on the order being stable across builds (Go slice literal is deterministic).
- **`core/providers/provider_test.go`** — already verifies that all registered providers implement `Provider`. We add a test that asserts the exact ordering, so future reorderings need explicit consent.

#### Deep Dives

**The new slice:**
```go
return []Provider{
    &golang.GoProvider{},
    &rust.RustProvider{},
    &java.JavaProvider{},
    &dotnet.DotnetProvider{},
    &ruby.RubyProvider{},
    &php.PhpProvider{},
    &python.PythonProvider{},
    &deno.DenoProvider{},
    &node.NodeProvider{},
    &staticfile.StaticfileProvider{},
    &shell.ShellProvider{},
}
```

**Invariants:**
- Length = 11.
- Index of `deno` < index of `node` (Deno-first guard).
- All language providers come before `staticfile` and `shell`.

**Edge cases:**
- This task lands BEFORE the actual provider implementations exist; it must be the LAST commit in Phase 0 OR import stub providers that are filled in by their respective phases. Practical solution: scaffold each provider as a minimal `package <lang>` with `type <Lang>Provider struct{}` and stub `Detect()` returning `false, nil` in this task, then fill in real logic in Phases 1-6.
- That keeps the registry change isolated and lets each language phase be self-contained.

#### Tasks
1. Create empty provider package for each new language with a struct literal that satisfies the `Provider` interface (all methods return zero values, `Detect` returns false).
2. Add imports and slice entries to `core/providers/provider.go` in the order from D3.
3. Add `TestRegistrationOrder` to `provider_test.go` asserting exact `Name()` sequence.
4. Run `mise run test` to confirm no panic and registration is wired.

#### TDD
```
RED:    TestRegistrationOrder         — asserts Name() sequence: ["go","rust","java","dotnet","ruby","php","python","deno","node","staticfile","shell"]
RED:    TestRegistrationCount         — asserts len(GetLanguageProviders()) == 11
RED:    TestDenoBeforeNode            — asserts indexOf("deno") < indexOf("node")
GREEN:  Add stub providers + register them
REFACTOR: None expected.
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] `GetLanguageProviders()` returns 11 entries.
- [ ] All 11 entries return distinct `Name()` strings.
- [ ] Order matches D3 exactly.
- [ ] `mise run test` green; `mise run check` green.

#### DoD
- [ ] All tasks completed.
- [ ] Registration order tests in place.
- [ ] Six stub provider packages exist (to be filled in subsequent phases).

---

## Phase 1: Rust Provider

**Objective:** Detect, plan, build, and deploy Rust applications via Cargo (binary crates), with workspace support and framework-aware start command detection.

### T1.1 — Implement single-crate Rust detection and plan

#### Objective
Wire `RustProvider.Detect()` to `Cargo.toml`, implement `Plan()` for a single-binary crate following the install/build/deploy pattern, and resolve the Rust version with the proper precedence.

#### Evidence
- Railpack `core/providers/rust/rust.go` exists and provides reference detection logic (Cargo.toml + edition + version files).
- `theo-stacks/templates/rust-axum/Dockerfile` shows the canonical pattern: `FROM rust:1-slim AS builder` → `cargo build --release` → `FROM debian:bookworm-slim` → copy binary. We will reproduce this shape.
- `core/providers/golang/golang.go:43-67` is the closest existing reference (compiled language, static-binary runtime).

#### Files to edit
```
core/providers/rust/rust.go         (NEW) — RustProvider implementation, Plan()
core/providers/rust/version.go      (NEW) — version detection helper
core/providers/rust/cargo.go        (NEW) — minimal Cargo.toml parser (we already depend on BurntSushi/toml)
core/providers/rust/rust_test.go    (NEW) — unit tests
```

#### Deep file dependency analysis
- **`core/providers/rust/rust.go`** — main entry point. Imports `core/generate`, `core/plan`, `core/app`. Exports `RustProvider` struct (already stubbed in T0.2). No downstream changes needed beyond filling the stub in.
- **`core/providers/rust/version.go`** — pure function `detectRustVersion(ctx) (version, source string)`. Reads `THEOPACKS_RUST_VERSION`, `rust-toolchain.toml` (channel field), `rust-toolchain` (single-line channel), and `Cargo.toml` `[package].rust-version` field.
- **`core/providers/rust/cargo.go`** — TOML decoder for the subset we need: `[package]`, `[[bin]]`, `[workspace]`. Reuses `BurntSushi/toml` already in `go.mod`.
- **`core/providers/rust/rust_test.go`** — unit tests covering Detect, Plan, version detection, single-bin vs multi-bin Cargo.toml.

#### Deep Dives

**Cargo.toml shapes we must handle:**

```toml
[package]
name = "myapp"
version = "0.1.0"
edition = "2021"
rust-version = "1.75"

[[bin]]
name = "server"
path = "src/main.rs"

[dependencies]
axum = "0.7"
```

Without `[[bin]]`, the binary name = `package.name`, path = `src/main.rs`. With `[[bin]]`, take the first entry (or one matching `THEOPACKS_RUST_PACKAGE` if set).

**Plan steps:**
1. **install step**: `FROM rust:<v>-bookworm`, `COPY Cargo.toml Cargo.lock ./`, dummy main.rs trick, `cargo build --release` (dependency cache layer).
2. **build step**: input from install, copy local source via `ctx.NewLocalLayer()`, `cargo build --release` (real build).
3. **deploy**: `FROM debian:bookworm-slim`, `apt-get install -y --no-install-recommends ca-certificates`, copy `/app/target/release/<binary>` to `/usr/local/bin/server`, set `StartCmd = "/usr/local/bin/server"`.

**Cache mounts:**
- `~/.cargo/registry` — cargo registry cache.
- `~/.cargo/git` — git deps cache.
- `target/` — build artifacts (use `--mount=type=cache` semantics via theo-packs' Cache type).

**Invariants:**
- After Plan() returns, `ctx.Deploy.StartCmd != ""`.
- `ctx.Deploy.Base` resolves to `RustRuntimeImage` (debian-slim).
- Deploy inputs include exactly one binary path filtered from the build step.

**Edge cases:**
- Library-only crate (no `[[bin]]`, no `src/main.rs`) → log warning, return error from Plan() with helpful message ("theo-packs cannot deploy a library crate; add a `[[bin]]` target").
- Workspace crate (root has `[workspace]` only) → defer to T1.2.
- `rust-toolchain.toml` with non-stable channel (`nightly`) → emit warning but continue with the channel name.

#### Tasks
1. Implement `cargo.go` with a `parseCargoToml(path) (*CargoToml, error)` function returning a struct with the subset of fields we need.
2. Implement `version.go` with `detectRustVersion(ctx)` per the precedence rule.
3. Implement `rust.go` with `RustProvider.Detect()`, `RustProvider.Plan()`, and the helpers above.
4. Wire `StartCommandHelp()` to a string explaining `[[bin]]` requirement.
5. Replace the T0.2 stub with the real implementation.

#### TDD
```
RED:    TestRustProvider_Name                       — Name() == "rust"
RED:    TestRustProvider_DetectCargoToml            — Cargo.toml present → true
RED:    TestRustProvider_DetectMissing              — no manifests → false
RED:    TestDetectRustVersion_Default               — no signal → ("1", "default")
RED:    TestDetectRustVersion_EnvOverride           — THEOPACKS_RUST_VERSION="1.75" → ("1.75", "THEOPACKS_RUST_VERSION")
RED:    TestDetectRustVersion_FromToolchainToml     — rust-toolchain.toml channel="1.74.0" → ("1.74.0", "rust-toolchain.toml")
RED:    TestDetectRustVersion_FromToolchainSingle   — rust-toolchain (single line) → matched
RED:    TestDetectRustVersion_FromCargoTomlField    — Cargo.toml rust-version="1.70" → ("1.70", "Cargo.toml")
RED:    TestParseCargoToml_SingleBin                — name + bin name + path
RED:    TestParseCargoToml_NoBin                    — defaults to package name + src/main.rs
RED:    TestParseCargoToml_LibraryOnly              — returns flag indicating no bin → error path
RED:    TestRustProvider_Plan_SimpleBin             — Plan produces install + build + deploy with /app/target/release/<bin>
RED:    TestRustProvider_Plan_DeployStartCmd        — StartCmd == "/usr/local/bin/server"
RED:    TestRustProvider_Plan_LibraryReturnsError   — clear error message
GREEN:  Implement cargo.go, version.go, rust.go
REFACTOR: Extract Plan helpers if file > 250 lines.
VERIFY: mise run test ./core/providers/rust/...
```

#### Acceptance Criteria
- [ ] `RustProvider.Detect()` returns true iff `Cargo.toml` exists.
- [ ] Version detection priority works: env > rust-toolchain.toml > rust-toolchain > Cargo.toml rust-version > default.
- [ ] `Plan()` for a single-bin crate produces a valid 2-step + deploy structure.
- [ ] `Plan()` for a library-only crate returns an error with a helpful message.
- [ ] All tests pass.
- [ ] `mise run check` passes.
- [ ] `core/providers/rust/rust.go` ≤ 300 lines.

#### DoD
- [ ] T1.1 tasks complete.
- [ ] `mise run test` green for `./core/providers/rust/...`.
- [ ] No new vet/lint warnings.

---

### T1.2 — Cargo workspace support

#### Objective
Detect Cargo workspaces (root `Cargo.toml` with `[workspace]`), parse `members` (with glob expansion), and adapt `Plan()` to build a single workspace member selected via `THEOPACKS_APP_NAME` (interpreted as crate package name).

#### Evidence
- `theo-stacks/templates/monorepo-rust/Cargo.toml` is a workspace; the user explicitly asked for monorepo support (Q5).
- `core/providers/node/workspace.go` is the proven pattern (`MemberPaths`, `DetectWorkspace` returning `nil` if not a workspace).
- ADR D7 — single env var pair for monorepo target selection.

#### Files to edit
```
core/providers/rust/workspace.go        (NEW) — DetectWorkspace, member resolution
core/providers/rust/workspace_test.go   (NEW) — unit tests
core/providers/rust/rust.go             — branch on workspace detection
```

#### Deep file dependency analysis
- **`core/providers/rust/workspace.go`** — exposes `DetectRustWorkspace(a *app.App, log *logger.Logger) *RustWorkspaceInfo`. Returns nil when no workspace. Internal-only to the rust package.
- **`core/providers/rust/rust.go`** — `Plan()` gains a branch: if `DetectRustWorkspace()` returns non-nil, route to `planWorkspace()` which uses `cargo build --release -p <pkg>` and reads `THEOPACKS_APP_NAME` to pick the package.

#### Deep Dives

**`RustWorkspaceInfo` struct:**
```go
type RustWorkspaceInfo struct {
    MemberPaths []string  // ["apps/api", "apps/worker", "packages/shared"]
    Members     []string  // crate names parsed from each member's Cargo.toml
}
```

**Detection algorithm:**
1. Read root `Cargo.toml`. If no `[workspace]` section → return nil.
2. Read `workspace.members` array. Each entry can be a literal path or a glob (`apps/*`).
3. Expand globs via `app.FindFiles` analog. For each resolved path, read `<path>/Cargo.toml` and extract `package.name`.
4. If no package resolves, log warning and return a workspace with empty Members.

**`Plan()` workspace branch:**
1. Resolve `THEOPACKS_APP_NAME` → must match one of `Members`.
2. If unset and there's exactly 1 member → use it.
3. If unset and >1 member → return error: "Cargo workspace has multiple members; set THEOPACKS_APP_NAME to one of: ...".
4. Build command: `cargo build --release -p <name>`.
5. Binary path: `/app/target/release/<name>` (use the package name; we don't currently support custom `[[bin]]` names in workspaces — defer).

**Invariants:**
- `len(MemberPaths) == len(Members)` after successful detection.
- Membership in workspace is symmetric across `THEOPACKS_APP_NAME` and parsed names.

**Edge cases:**
- Workspace with virtual root (no `[package]` at root) — common case, must be supported.
- Glob `apps/*` matching a non-crate directory — silently skip.
- Member path with trailing slash — normalize.

#### Tasks
1. Implement `DetectRustWorkspace` with glob expansion via `app.FindFiles("apps/*/Cargo.toml")` etc. (or our own walker).
2. Implement `planWorkspace` branch in `rust.go`.
3. Wire `StartCommandHelp` to mention `THEOPACKS_APP_NAME` for workspaces.

#### TDD
```
RED:    TestDetectRustWorkspace_NoWorkspace        — no [workspace] → nil
RED:    TestDetectRustWorkspace_LiteralMembers     — explicit paths → resolves package names
RED:    TestDetectRustWorkspace_GlobMembers        — apps/* → expands
RED:    TestDetectRustWorkspace_VirtualRoot        — root has only [workspace], no [package] → ok
RED:    TestPlanWorkspace_TargetSelected           — THEOPACKS_APP_NAME=api builds with -p api
RED:    TestPlanWorkspace_TargetMissing            — unknown name → error mentioning available names
RED:    TestPlanWorkspace_SingleMemberAutoSelect   — 1 member, no env → builds it without error
RED:    TestPlanWorkspace_MultipleMembersNoEnv     — 2+ members, no env → error
GREEN:  Implement workspace.go + plan branch
REFACTOR: None expected.
VERIFY: mise run test ./core/providers/rust/...
```

#### Acceptance Criteria
- [ ] Workspace detection handles literal + glob members + virtual root.
- [ ] `Plan()` correctly routes to workspace branch when applicable.
- [ ] Error messages list available members.
- [ ] All tests pass.
- [ ] `core/providers/rust/workspace.go` ≤ 250 lines.

#### DoD
- [ ] T1.2 tasks complete.
- [ ] `mise run test` green.
- [ ] `mise run check` clean.

---

### T1.3 — Example projects + golden Dockerfiles for Rust

#### Objective
Add three example projects for Rust (`rust-axum`, `rust-cli`, `rust-workspace`) under `examples/`, register them in `core/dockerfile/integration_test.go`, and generate the corresponding golden Dockerfiles.

#### Evidence
- Existing pattern: each language has 2-4 examples in `examples/<lang>-*/`. Each example is exercised by `core/dockerfile/integration_test.go` against a golden Dockerfile in `core/dockerfile/testdata/integration_<lang>_<example>.dockerfile`.
- `theo-stacks/templates/rust-axum/` provides a ready-made Axum project we can adapt (pre-tested, idiomatic).
- `theo-stacks/templates/monorepo-rust/` gives the workspace shape.

#### Files to edit
```
examples/rust-axum/                       (NEW) — minimal Axum HTTP service
examples/rust-axum/Cargo.toml             (NEW)
examples/rust-axum/src/main.rs            (NEW)
examples/rust-cli/                        (NEW) — pure CLI to exercise non-web start cmd
examples/rust-cli/Cargo.toml              (NEW)
examples/rust-cli/src/main.rs             (NEW)
examples/rust-workspace/                  (NEW) — workspace with apps/api + apps/worker + packages/shared
examples/rust-workspace/Cargo.toml        (NEW) — workspace root
examples/rust-workspace/apps/api/...      (NEW)
examples/rust-workspace/apps/worker/...   (NEW)
examples/rust-workspace/packages/shared/...(NEW)
core/dockerfile/integration_test.go       — ADD 3 new test cases
core/dockerfile/testdata/integration_rust_axum.dockerfile     (NEW, generated)
core/dockerfile/testdata/integration_rust_cli.dockerfile      (NEW, generated)
core/dockerfile/testdata/integration_rust_workspace.dockerfile(NEW, generated)
```

#### Deep file dependency analysis
- **`examples/rust-*`** — example projects are static fixtures. They must be compilable Rust (the E2E suite will actually `cargo build` them inside Docker), so dependencies pinned and minimal.
- **`core/dockerfile/integration_test.go`** — table-driven; adding cases is additive. Existing structure (per the explore agent's report at lines 25-98 of that file) takes `{example string, env map[string]string}` rows.
- **Golden files** are generated, not hand-written. Workflow: implement the provider → run `UPDATE_GOLDEN=true go test ./core/dockerfile/...` → review the diff → commit.

#### Deep Dives

**`rust-workspace` shape (mirroring theo-stacks):**
```
rust-workspace/
├── Cargo.toml              # [workspace] members = ["apps/*", "packages/*"]
├── apps/
│   ├── api/{Cargo.toml,src/main.rs}
│   └── worker/{Cargo.toml,src/main.rs}
└── packages/
    └── shared/{Cargo.toml,src/lib.rs}
```

For the workspace integration test, env will be `THEOPACKS_APP_NAME=api`.

**Invariants for examples:**
- Each example must be self-contained (no external git deps).
- Dependencies pinned (`= "0.7.5"` not `"^0.7"`) to keep golden files stable across `cargo update`.
- Source code minimal: `axum::Router::new().route("/", get(|| async { "ok" }))` is plenty.

**Edge cases:**
- `theo-stacks` Dockerfile has a "dummy main.rs trick" for caching. Verify our generated Dockerfile uses `--mount=type=cache` instead (theo-packs' cache primitive). This is a stylistic divergence — document in a comment in `rust.go` if it ships differently.

#### Tasks
1. Create the three example directories with minimal source.
2. Add three test cases in `integration_test.go` (with env for the workspace case).
3. Run `UPDATE_GOLDEN=true go test ./core/dockerfile/... -run TestIntegration_AllExamples/rust` to generate golden files.
4. Manual review: open each golden Dockerfile, confirm structure matches expectations (build → deploy, correct base images, correct copy filters).
5. Commit examples and golden files together.

#### TDD
```
RED:    TestIntegration_RustAxum         — golden file exists and matches
RED:    TestIntegration_RustCli          — golden file exists and matches
RED:    TestIntegration_RustWorkspace    — golden file exists and matches with env THEOPACKS_APP_NAME=api
GREEN:  Add examples; run UPDATE_GOLDEN=true to generate
REFACTOR: None.
VERIFY: mise run test ./core/dockerfile/...
```

#### Acceptance Criteria
- [ ] Three example directories exist with minimal compilable Rust.
- [ ] Three new test cases in `integration_test.go`.
- [ ] Three golden Dockerfiles exist in `core/dockerfile/testdata/`.
- [ ] All three integration tests pass without `UPDATE_GOLDEN`.
- [ ] Generated Dockerfiles use the `rust:1-bookworm` build image and `debian:bookworm-slim` runtime.
- [ ] Workspace golden file uses `cargo build --release -p api`.

#### DoD
- [ ] T1.3 tasks complete.
- [ ] Golden files committed and reviewed.
- [ ] `mise run test ./core/dockerfile/...` green.

---

## Phase 2: Java Provider

**Objective:** Detect Java projects (Gradle and Maven), resolve Java version, plan a multi-stage build that produces a fat JAR, and detect Spring Boot vs generic Java for start command.

### T2.1 — Implement Java detection (Gradle + Maven)

#### Objective
`JavaProvider.Detect()` returns true if any of `build.gradle.kts`, `build.gradle`, `pom.xml`, or a multi-module variant exists. `JavaProvider.Plan()` routes between `planGradle()` and `planMaven()`.

#### Evidence
- `theo-stacks/templates/java-spring/build.gradle.kts` and `theo-stacks/templates/java-spring/Dockerfile` show the canonical Gradle + Spring Boot pattern (FROM gradle → bootJar → FROM eclipse-temurin:21-jre).
- Railpack `core/providers/java/` exists and provides reference detection (Gradle wrapper, Maven wrapper handling).
- `core/providers/python/python.go:39-75` shows the dispatch pattern between multiple build tools.

#### Files to edit
```
core/providers/java/java.go             (NEW) — JavaProvider, dispatch
core/providers/java/version.go          (NEW) — version detection
core/providers/java/gradle.go           (NEW) — Gradle plan generation + Spring Boot detection
core/providers/java/maven.go            (NEW) — Maven plan generation + Spring Boot detection
core/providers/java/java_test.go        (NEW) — unit tests
core/providers/java/gradle_test.go      (NEW)
core/providers/java/maven_test.go       (NEW)
```

#### Deep file dependency analysis
- **`core/providers/java/java.go`** — entry point. Imports `core/generate`, `core/plan`, `core/app`. Detect logic + dispatch.
- **`core/providers/java/version.go`** — pure function `detectJavaVersion(ctx) (string, string)`. Reads `THEOPACKS_JAVA_VERSION`, `.java-version`, `gradle.properties` (`javaVersion=21`), `pom.xml` (`<maven.compiler.target>` or `<java.version>`), `build.gradle*` toolchain blocks (regex extraction; we're not implementing a full Groovy parser).
- **`core/providers/java/gradle.go`** — `planGradle(ctx, version, isWorkspace bool)`. Detects Spring Boot via the `id("org.springframework.boot")` plugin reference in `build.gradle.kts` or `apply plugin: 'org.springframework.boot'` in `build.gradle`. Build command: `gradle bootJar --no-daemon` (if Spring Boot) or `gradle build --no-daemon`. Output: `build/libs/*.jar`.
- **`core/providers/java/maven.go`** — `planMaven(ctx, version, isWorkspace bool)`. Detects Spring Boot via `<artifactId>spring-boot-starter-*</artifactId>` in pom.xml. Build command: `mvn -B -DskipTests package`. Output: `target/*.jar`.

#### Deep Dives

**Gradle plan structure (Spring Boot single-module):**
1. **install step**: `FROM gradle:8-jdk21`, `COPY build.gradle.kts settings.gradle.kts gradle/ ./`, `COPY gradlew* ./`, optional warmup `gradle dependencies --no-daemon`.
2. **build step**: input from install, copy local source, `gradle bootJar --no-daemon`. Cache mounts: `/root/.gradle`, `/app/.gradle`.
3. **deploy**: `FROM eclipse-temurin:21-jre`, copy `build/libs/*.jar` to `/app/app.jar`, `StartCmd = "java -jar /app/app.jar"`.

**Maven plan structure (Spring Boot single-module):**
1. **install step**: `FROM maven:3-eclipse-temurin-21`, `COPY pom.xml ./`, `mvn -B dependency:go-offline` (if cache miss).
2. **build step**: input from install, copy source, `mvn -B -DskipTests package`. Cache mount: `/root/.m2`.
3. **deploy**: same as Gradle.

**Spring Boot detection rules:**
- **Gradle Kotlin DSL:** regex `id\("org\.springframework\.boot"\)` in `build.gradle.kts`.
- **Gradle Groovy:** `apply plugin: 'org.springframework.boot'` OR `id 'org.springframework.boot'` in `build.gradle`.
- **Maven:** any `<artifactId>` starting with `spring-boot-starter` in `pom.xml` (we look for the substring; cheap and good enough for detection).

**Invariants:**
- If both Gradle and Maven manifests exist, Gradle wins (more common for new projects). Log a warning.
- If Spring Boot detected, use `bootJar` (Gradle) or rely on `spring-boot-maven-plugin` (Maven, standard).
- Default `StartCmd` always works as long as a single-fat-JAR is produced.

**Edge cases:**
- Multiple JARs in `build/libs/` (Spring Boot produces both `*-plain.jar` and the bootable JAR) — explicitly select the non-`-plain` one in the COPY filter.
- Maven `pom.xml` without `spring-boot-maven-plugin` but with starter deps → still Spring Boot, but the JAR isn't bootable. Document this in `StartCommandHelp` and require `theopacks.json` override.
- Project uses Java 8 / 11 / 17 instead of 21 — version detection picks it up; image helpers handle it.

#### Tasks
1. Implement `version.go` with the precedence rule.
2. Implement `gradle.go` with `planGradle` + Spring Boot detection.
3. Implement `maven.go` with `planMaven` + Spring Boot detection.
4. Implement `java.go` with Detect + dispatch.
5. Replace T0.2 stub.

#### TDD
```
RED:    TestJavaProvider_DetectGradleKts        — build.gradle.kts → true
RED:    TestJavaProvider_DetectGradleGroovy     — build.gradle → true
RED:    TestJavaProvider_DetectMaven            — pom.xml → true
RED:    TestJavaProvider_DetectNothing          — empty dir → false
RED:    TestJavaProvider_GradleWinsOverMaven    — both exist → log warning, plan Gradle
RED:    TestDetectJavaVersion_Default           — no signal → ("21", "default")
RED:    TestDetectJavaVersion_EnvOverride       — THEOPACKS_JAVA_VERSION="17" → ("17", "THEOPACKS_JAVA_VERSION")
RED:    TestDetectJavaVersion_DotJavaVersion    — .java-version="17" → ("17", ".java-version")
RED:    TestDetectJavaVersion_GradleProperties  — javaVersion=11 → ("11", "gradle.properties")
RED:    TestDetectJavaVersion_PomXml            — <java.version>17</java.version> → ("17", "pom.xml")
RED:    TestPlanGradle_SpringBoot               — Spring Boot detected → bootJar build, java -jar start
RED:    TestPlanGradle_GenericJava              — no Spring → gradle build, find first JAR
RED:    TestPlanMaven_SpringBoot                — Spring Boot starter deps → spring-boot-maven-plugin assumed
RED:    TestPlanMaven_GenericJava               — no Spring → mvn package, find target/*.jar
RED:    TestSpringBootDetection_GradleKts       — id("org.springframework.boot") detected
RED:    TestSpringBootDetection_GradleGroovy    — apply plugin or id 'org...' detected
RED:    TestSpringBootDetection_Maven           — spring-boot-starter substring in pom.xml
GREEN:  Implement provider files
REFACTOR: Extract Spring Boot detection into shared helper if Gradle/Maven duplicate logic.
VERIFY: mise run test ./core/providers/java/...
```

#### Acceptance Criteria
- [ ] All three build tool variants (Gradle KTS, Gradle Groovy, Maven) detected.
- [ ] Version detection works across `.java-version`, `gradle.properties`, `pom.xml`, env var.
- [ ] Spring Boot detection correct for both build tools.
- [ ] Generic Java path works without Spring Boot.
- [ ] All tests pass; `mise run check` clean.
- [ ] Each file ≤ 350 lines.

#### DoD
- [ ] T2.1 tasks complete.
- [ ] `mise run test ./core/providers/java/...` green.

---

### T2.2 — Java workspace support (Gradle subprojects + Maven multi-module)

#### Objective
Detect Gradle multi-project builds (`settings.gradle.kts` with `include(...)`) and Maven multi-module builds (`<modules>` in parent pom.xml), then route Plan to a workspace-aware build that targets a single app via `THEOPACKS_APP_NAME`.

#### Evidence
- `theo-stacks/templates/monorepo-java/settings.gradle.kts` shows the Gradle subprojects pattern (`include(":apps:api", ":apps:worker")`).
- ADR D7.

#### Files to edit
```
core/providers/java/workspace.go         (NEW) — workspace detection
core/providers/java/workspace_test.go    (NEW)
core/providers/java/gradle.go            — branch on workspace
core/providers/java/maven.go             — branch on workspace
```

#### Deep file dependency analysis
- **`core/providers/java/workspace.go`** — `DetectJavaWorkspace(a, log) *JavaWorkspaceInfo` parses both Gradle settings and Maven pom.xml `<modules>`. Returns nil if neither.
- **`core/providers/java/gradle.go`** / **`core/providers/java/maven.go`** — gain a `planWorkspace` helper that uses `THEOPACKS_APP_NAME` to produce `gradle :apps:<name>:bootJar` or `mvn -pl apps/<name> -am package`.

#### Deep Dives

**`JavaWorkspaceInfo`:**
```go
type JavaWorkspaceInfo struct {
    Tool        BuildTool          // Gradle or Maven
    AppPaths    map[string]string  // "api" → "apps/api"
}
```

**Gradle subprojects parsing:**
- Read `settings.gradle.kts` (or `.gradle`).
- Extract `include(...)` argument list (regex; full Groovy/KTS parser is overkill).
- For each `:apps:api` style path, map to filesystem `apps/api`.
- Validate by checking `apps/api/build.gradle*` exists.

**Maven multi-module parsing:**
- Read parent `pom.xml`.
- Extract `<modules><module>apps/api</module>...</modules>`.
- For each module, validate `<module>/pom.xml` exists.

**Build commands:**
- Gradle: `gradle :apps:<name>:bootJar --no-daemon` (or `:build` if no Spring Boot).
- Maven: `mvn -B -pl apps/<name> -am package -DskipTests`. The `-am` flag builds dependencies too.

**Output paths:**
- Gradle: `apps/<name>/build/libs/*.jar`.
- Maven: `apps/<name>/target/*.jar`.

**Invariants:**
- `THEOPACKS_APP_NAME` MUST resolve to a key in `AppPaths` when workspace is detected; otherwise error with available names.
- Single-module fallback: if `<modules>` present but only 1 entry, use it without requiring env var.

**Edge cases:**
- Gradle `includeBuild` (composite builds) — out of scope; document in `StartCommandHelp`.
- Maven module directory contains its own multi-module pom — recursion not supported; flat layout only (matches theo-stacks).

#### Tasks
1. Implement `DetectJavaWorkspace` for both build tools.
2. Add workspace branch in `planGradle` and `planMaven`.
3. Update `StartCommandHelp` to mention `THEOPACKS_APP_NAME` for monorepos.

#### TDD
```
RED:    TestDetectJavaWorkspace_GradleSubprojects   — settings.gradle.kts with include() → resolves
RED:    TestDetectJavaWorkspace_MavenMultiModule    — pom.xml <modules> → resolves
RED:    TestDetectJavaWorkspace_None                — no workspace markers → nil
RED:    TestPlanGradle_Workspace_Selected           — THEOPACKS_APP_NAME=api → :apps:api:bootJar
RED:    TestPlanMaven_Workspace_Selected            — THEOPACKS_APP_NAME=api → mvn -pl apps/api -am
RED:    TestPlanGradle_Workspace_Missing            — bad name → error listing options
RED:    TestPlanMaven_Workspace_Missing             — bad name → error listing options
GREEN:  Implement workspace.go + plan branches
REFACTOR: None.
VERIFY: mise run test ./core/providers/java/...
```

#### Acceptance Criteria
- [ ] Gradle and Maven workspaces both detected.
- [ ] Workspace plan correctly targets the selected app.
- [ ] Error messages include available app names.
- [ ] All tests pass; lint clean.

#### DoD
- [ ] T2.2 tasks complete.
- [ ] `mise run test` green.

---

### T2.3 — Java example projects + golden Dockerfiles

#### Objective
Add `examples/java-spring-gradle/`, `examples/java-spring-maven/`, `examples/java-gradle-workspace/`, register integration tests, generate golden Dockerfiles.

#### Evidence
- Same pattern as T1.3.
- `theo-stacks/templates/java-spring/` is ready-made; we adapt to a minimal example.

#### Files to edit
```
examples/java-spring-gradle/         (NEW) — Spring Boot Gradle KTS minimal
examples/java-spring-maven/          (NEW) — Spring Boot Maven minimal
examples/java-gradle-workspace/      (NEW) — Gradle subprojects with apps/api + apps/worker
core/dockerfile/integration_test.go  — ADD 3 cases
core/dockerfile/testdata/integration_java_spring_gradle.dockerfile     (NEW)
core/dockerfile/testdata/integration_java_spring_maven.dockerfile      (NEW)
core/dockerfile/testdata/integration_java_gradle_workspace.dockerfile  (NEW)
```

#### Deep file dependency analysis
- Same as T1.3.

#### Deep Dives
- Examples use `org.springframework.boot:3.3.0` (Java 21) for parity with theo-stacks template.
- Generic Java example skipped for now to limit scope; can add later if a generic-Java template lands in theo-stacks.

#### Tasks
1. Create three example directories.
2. Add three integration test cases.
3. Generate golden Dockerfiles via `UPDATE_GOLDEN=true`.
4. Manual review.

#### TDD
```
RED:    TestIntegration_JavaSpringGradle    — golden matches
RED:    TestIntegration_JavaSpringMaven     — golden matches
RED:    TestIntegration_JavaGradleWorkspace — golden matches with THEOPACKS_APP_NAME=api
GREEN:  Generate goldens
VERIFY: mise run test ./core/dockerfile/...
```

#### Acceptance Criteria
- [ ] Three Java examples present and minimal.
- [ ] Three test cases pass without UPDATE_GOLDEN.
- [ ] Generated Dockerfiles use `gradle:8-jdk21` / `maven:3-eclipse-temurin-21` build images and `eclipse-temurin:21-jre` runtime.

#### DoD
- [ ] T2.3 tasks complete.

---

## Phase 3: .NET Provider

**Objective:** Detect single-project (`.csproj`/`.fsproj`/`.vbproj`), solution-file (`.sln`), and multi-target framework (TFM) .NET projects; produce a publish-style Dockerfile that copies the published output to a runtime stage.

### T3.1 — Implement .NET single-project detection and plan

#### Objective
`DotnetProvider.Detect()` finds project files via glob; `Plan()` runs `dotnet publish` and copies the output to an ASP.NET or generic runtime stage based on TFM analysis.

#### Evidence
- Railpack `core/providers/dotnet/` provides reference parsing of `<TargetFramework>` and `<OutputType>`.
- ASP.NET vs generic-console runtime distinction is critical because `mcr.microsoft.com/dotnet/aspnet:8.0` includes web server bits while `dotnet/runtime:8.0` is smaller.

#### Files to edit
```
core/providers/dotnet/dotnet.go         (NEW) — DotnetProvider, Plan dispatch
core/providers/dotnet/version.go        (NEW) — version detection
core/providers/dotnet/project.go        (NEW) — .csproj parsing (subset XML)
core/providers/dotnet/dotnet_test.go    (NEW)
core/providers/dotnet/project_test.go   (NEW)
```

#### Deep file dependency analysis
- **`core/providers/dotnet/dotnet.go`** — entry; uses `app.FindFiles("*.csproj")` etc. to discover project files.
- **`core/providers/dotnet/project.go`** — minimal XML decoder for `<TargetFramework>`, `<TargetFrameworks>`, `<OutputType>`, `<PackageReference Include="Microsoft.AspNetCore.*" />`. We use stdlib `encoding/xml`.
- **`core/providers/dotnet/version.go`** — reads `global.json` (`sdk.version`), `THEOPACKS_DOTNET_VERSION`, then `<TargetFramework>` (e.g., `net8.0` → `8.0`).

#### Deep Dives

**Detection algorithm:**
1. Glob for `*.csproj`, `*.fsproj`, `*.vbproj` in the project root.
2. If multiple project files found → check for `*.sln` (defer to T3.2 solution flow).
3. If single project file → use it as the build target.

**`<TargetFramework>` interpretation:**
- `net8.0` → `8.0`
- `net6.0` → `6.0`
- `netcoreapp3.1` → `3.1` (legacy, allow but warn).
- `<TargetFrameworks>` (multiple) → first one for build target.

**ASP.NET detection:**
- Any `<PackageReference Include="Microsoft.AspNetCore.*">` or `<Project Sdk="Microsoft.NET.Sdk.Web">` → use `aspnet:8.0` runtime.
- Otherwise (`<Project Sdk="Microsoft.NET.Sdk">` + `<OutputType>Exe</OutputType>`) → use `runtime:8.0`.

**Plan structure:**
1. **install step**: `FROM mcr.microsoft.com/dotnet/sdk:8.0`, `COPY *.csproj ./`, `dotnet restore`. Cache mount: `/root/.nuget/packages`.
2. **build step**: input from install, copy source, `dotnet publish -c Release -o /app/publish --no-restore`. 
3. **deploy**: `FROM mcr.microsoft.com/dotnet/aspnet:8.0` (or `runtime:8.0`), `COPY --from=build /app/publish .`, `StartCmd = "dotnet <Assembly>.dll"`.

**Assembly name:** project filename without extension (e.g., `MyApi.csproj` → `MyApi.dll`).

**Invariants:**
- For ASP.NET, `aspnet:8.0` runtime mandatory.
- For console, `runtime:8.0` is sufficient.
- `dotnet publish` always produces a self-describing folder.

**Edge cases:**
- Solution file present (`.sln`) → defer to T3.2.
- Multiple .csproj without solution → ambiguous; require `THEOPACKS_DOTNET_PROJECT` env var.
- Multi-target frameworks (`<TargetFrameworks>net6.0;net8.0</TargetFrameworks>`) → pick highest, log notice.
- `Microsoft.NET.Sdk.Worker` SDK → use generic runtime, treat as console.

#### Tasks
1. Implement `project.go` XML parser.
2. Implement `version.go`.
3. Implement `dotnet.go` Detect + Plan.

#### TDD
```
RED:    TestDotnetProvider_DetectSingleCsproj    — *.csproj → true
RED:    TestDotnetProvider_DetectFsproj          — *.fsproj → true
RED:    TestDotnetProvider_DetectMissing         — empty → false
RED:    TestParseProject_TargetFramework         — <TargetFramework>net8.0</> → "8.0"
RED:    TestParseProject_AspNetSdk               — Sdk="Microsoft.NET.Sdk.Web" → IsAspNet=true
RED:    TestParseProject_AspNetPackageRef        — Microsoft.AspNetCore.* package → IsAspNet=true
RED:    TestParseProject_ConsoleSdk              — Sdk="Microsoft.NET.Sdk", OutputType=Exe → IsAspNet=false
RED:    TestDetectDotnetVersion_GlobalJson       — sdk.version="8.0.100" → ("8.0", "global.json")
RED:    TestDetectDotnetVersion_FromTargetFramework — net6.0 → ("6.0", ".csproj")
RED:    TestDotnetProvider_Plan_AspNetSingleProject  — full plan: sdk:8.0 → publish → aspnet:8.0
RED:    TestDotnetProvider_Plan_ConsoleSingleProject — full plan: sdk:8.0 → publish → runtime:8.0
GREEN:  Implement provider
REFACTOR: None expected.
VERIFY: mise run test ./core/providers/dotnet/...
```

#### Acceptance Criteria
- [ ] Single-project detection works for `.csproj`, `.fsproj`, `.vbproj`.
- [ ] ASP.NET vs console runtime routing correct.
- [ ] All tests pass; lint clean.

#### DoD
- [ ] T3.1 tasks complete.
- [ ] `mise run test` green.

---

### T3.2 — .NET solution-file (`.sln`) and multi-project support

#### Objective
Detect `.sln` solutions, parse them to extract project paths, and use `THEOPACKS_APP_NAME` (or default to a single-host project) to select the publish target.

#### Evidence
- `theo-stacks/templates/dotnet-aspnet` (when added) typically uses `.sln` for organization.
- ADR D7.

#### Files to edit
```
core/providers/dotnet/solution.go        (NEW) — .sln parser
core/providers/dotnet/solution_test.go   (NEW)
core/providers/dotnet/dotnet.go          — branch on .sln presence
```

#### Deep file dependency analysis
- **`core/providers/dotnet/solution.go`** — parses `.sln` text format (line-based, `Project("...") = "Name", "path/Name.csproj", "{GUID}"`). Stdlib + regex.

#### Deep Dives

**`.sln` line format:**
```
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "MyApi", "src\MyApi\MyApi.csproj", "{...}"
```

Extract project name and path. Note the path uses Windows separators in some files; normalize.

**Selection rules:**
1. If `THEOPACKS_APP_NAME` matches a project name → publish that project.
2. If only one project has ASP.NET host signals → pick it.
3. If multiple ambiguous → error with the list.

**Build command:**
- `dotnet publish src/MyApi/MyApi.csproj -c Release -o /app/publish`.

#### Tasks
1. Implement `solution.go` with line-based parser (regex for `Project(...) = ...` lines).
2. Update `dotnet.go` to dispatch based on `.sln` presence.

#### TDD
```
RED:    TestParseSolution_SingleProject       — one Project line → 1 entry
RED:    TestParseSolution_MultipleProjects    — multiple → list with names + paths
RED:    TestParseSolution_WindowsPaths        — backslash → normalized to forward slash
RED:    TestDotnetProvider_Plan_Solution_AppNameSet  — THEOPACKS_APP_NAME=MyApi → publishes that csproj
RED:    TestDotnetProvider_Plan_Solution_SingleAspNet — auto-selects the only ASP.NET project
RED:    TestDotnetProvider_Plan_Solution_AmbiguousNoEnv — error listing projects
GREEN:  Implement solution.go + dispatch
REFACTOR: None expected.
VERIFY: mise run test ./core/providers/dotnet/...
```

#### Acceptance Criteria
- [ ] `.sln` parsing extracts project name + path correctly.
- [ ] Selection rules work for the three cases.
- [ ] All tests pass.

#### DoD
- [ ] T3.2 tasks complete.

---

### T3.3 — .NET examples + golden Dockerfiles

#### Objective
Add `examples/dotnet-aspnet/` (single ASP.NET project), `examples/dotnet-console/` (console worker), `examples/dotnet-solution/` (solution with two projects), register integration tests, generate goldens.

#### Evidence
- Same pattern as T1.3 / T2.3.

#### Files to edit
```
examples/dotnet-aspnet/                  (NEW)
examples/dotnet-console/                 (NEW)
examples/dotnet-solution/                (NEW)
core/dockerfile/integration_test.go      — ADD 3 cases
core/dockerfile/testdata/integration_dotnet_aspnet.dockerfile     (NEW)
core/dockerfile/testdata/integration_dotnet_console.dockerfile    (NEW)
core/dockerfile/testdata/integration_dotnet_solution.dockerfile   (NEW)
```

#### Deep file dependency analysis
- Same pattern as T1.3.

#### Deep Dives
- ASP.NET example uses `Microsoft.NET.Sdk.Web` SDK and a minimal `Program.cs` with `WebApplication.CreateBuilder`.
- Console example uses `Microsoft.NET.Sdk` with `<OutputType>Exe</OutputType>`.
- Solution example: two projects, one ASP.NET, one console; solution test uses `THEOPACKS_APP_NAME=Api`.

#### Tasks
1. Create three .NET example directories with minimal source.
2. Add integration test cases.
3. Generate goldens.

#### TDD
```
RED:    TestIntegration_DotnetAspnet     — golden matches with aspnet:8.0 runtime
RED:    TestIntegration_DotnetConsole    — golden matches with runtime:8.0
RED:    TestIntegration_DotnetSolution   — golden matches with THEOPACKS_APP_NAME=Api
GREEN:  Generate goldens
VERIFY: mise run test ./core/dockerfile/...
```

#### Acceptance Criteria
- [ ] Three .NET examples present.
- [ ] Three integration tests pass.
- [ ] Generated Dockerfiles use correct base images per scenario.

#### DoD
- [ ] T3.3 tasks complete.

---

## Phase 4: Ruby Provider

**Objective:** Detect Ruby projects via `Gemfile`, distinguish Rails / Sinatra / generic Rack, build with bundler, and run with Puma where appropriate.

### T4.1 — Implement Ruby detection + framework routing

#### Objective
`RubyProvider.Detect()` returns true if `Gemfile` exists. `Plan()` resolves Ruby version, runs `bundle install`, and selects a start command based on framework detection.

#### Evidence
- `theo-stacks/templates/ruby-sinatra/Gemfile` and `Dockerfile` show the canonical pattern.
- Railpack `core/providers/ruby/` provides reference Gemfile parsing.
- Rails has explicit signals (`config/application.rb` plus `gem "rails"`).

#### Files to edit
```
core/providers/ruby/ruby.go              (NEW) — RubyProvider
core/providers/ruby/version.go           (NEW)
core/providers/ruby/framework.go         (NEW) — Rails / Sinatra detection
core/providers/ruby/gemfile.go           (NEW) — minimal Gemfile parser (regex-based for gem lines)
core/providers/ruby/ruby_test.go         (NEW)
core/providers/ruby/framework_test.go    (NEW)
```

#### Deep file dependency analysis
- **`core/providers/ruby/ruby.go`** — entry; Detect + Plan.
- **`core/providers/ruby/version.go`** — reads `THEOPACKS_RUBY_VERSION`, `.ruby-version`, `Gemfile` (`ruby '3.3'`).
- **`core/providers/ruby/framework.go`** — exposes `detectFramework(ctx) Framework` returning `FrameworkRails`, `FrameworkSinatra`, `FrameworkRack`, or `FrameworkUnknown`.
- **`core/providers/ruby/gemfile.go`** — regex-based parser extracting gem names + Ruby version directive. We're not implementing a full Ruby parser; the format is line-based and tractable.

#### Deep Dives

**Framework detection rules:**
- **Rails:** `gem "rails"` in Gemfile + `config/application.rb` exists.
- **Sinatra:** `gem "sinatra"` in Gemfile + `config.ru` exists (or `app.rb` with `class App < Sinatra::Base`).
- **Rack:** `config.ru` only (no Rails/Sinatra signal).
- **Unknown:** no clear signal — require `theopacks.json` `deploy.startCommand`.

**Plan steps:**
1. **install step**: `FROM ruby:3.3-bookworm-slim`, `apt-get install -y --no-install-recommends build-essential` (gems with native ext), `COPY Gemfile Gemfile.lock ./`, `bundle config set --local without 'development test'`, `bundle install --jobs 4`. Cache: `/usr/local/bundle`.
2. **build step**: input from install, copy source, run `bundle exec rake assets:precompile` if Rails detected (and `app/assets` exists).
3. **deploy**: `FROM ruby:3.3-bookworm-slim`, copy bundle + source, `apt-get install -y --no-install-recommends ca-certificates`, `StartCmd` per framework.

**Start commands:**
- **Rails:** `bundle exec rails server -b 0.0.0.0 -p ${PORT:-3000} -e production`.
- **Sinatra:** `bundle exec rackup -p ${PORT:-4567} -o 0.0.0.0` (matches theo-stacks).
- **Rack:** same as Sinatra.
- **Procfile fallback:** if `Procfile` has a `web:` line, use it verbatim.

**Invariants:**
- Bundler env: `BUNDLE_PATH=/usr/local/bundle`, `BUNDLE_DEPLOYMENT=true`, `BUNDLE_WITHOUT=development:test`.
- Production secret key handled via `RAILS_MASTER_KEY` secret env var (NOT baked into image; theo-packs `Secrets` field).

**Edge cases:**
- Rails without `Gemfile.lock` → error (lockfile mandatory for reproducible builds).
- Asset compilation requires Node — for full Rails apps, document that the user must add Node via `theopacks.json` `buildAptPackages`. Skip asset precompile for non-Rails apps.
- ImageMagick / native gems requiring `libpq-dev`, etc. — provide a hint in `StartCommandHelp` to use `theopacks.json` `buildAptPackages`.

#### Tasks
1. Implement `gemfile.go` regex-based extractor for gem names + Ruby version.
2. Implement `version.go`.
3. Implement `framework.go`.
4. Implement `ruby.go` Detect + Plan with framework dispatch.

#### TDD
```
RED:    TestRubyProvider_DetectGemfile          — Gemfile present → true
RED:    TestRubyProvider_DetectMissing          — no Gemfile → false
RED:    TestParseGemfile_RubyVersion            — ruby '3.3' → "3.3"
RED:    TestParseGemfile_GemNames               — gem "rails", gem "sinatra" → list
RED:    TestParseGemfile_DoubleQuotesAndSingle  — gem 'foo' and gem "foo" both work
RED:    TestDetectRubyVersion_Default           — no signal → ("3.3", "default")
RED:    TestDetectRubyVersion_DotRubyVersion    — .ruby-version="3.2" → ("3.2", ".ruby-version")
RED:    TestDetectRubyVersion_GemfileDirective  — Gemfile ruby '3.4' → ("3.4", "Gemfile")
RED:    TestDetectFramework_Rails               — gem "rails" + config/application.rb → FrameworkRails
RED:    TestDetectFramework_Sinatra             — gem "sinatra" + config.ru → FrameworkSinatra
RED:    TestDetectFramework_Rack                — config.ru only → FrameworkRack
RED:    TestDetectFramework_Unknown             — Gemfile only → FrameworkUnknown
RED:    TestRubyProvider_Plan_Rails             — Plan → rails server start cmd
RED:    TestRubyProvider_Plan_Sinatra           — Plan → rackup start cmd
RED:    TestRubyProvider_Plan_Procfile          — Procfile web: line wins
GREEN:  Implement provider
REFACTOR: Extract parseGemfile into shared helper if needed.
VERIFY: mise run test ./core/providers/ruby/...
```

#### Acceptance Criteria
- [ ] All four framework states detected correctly.
- [ ] Start commands match the expected pattern per framework.
- [ ] Procfile takes precedence when present.
- [ ] All tests pass; lint clean.

#### DoD
- [ ] T4.1 tasks complete.

---

### T4.2 — Ruby monorepo via apps/+packages/ + Procfile

#### Objective
Detect the apps/+packages/ pattern with a top-level Procfile (matches `theo-stacks/templates/monorepo-ruby` shape) and use `THEOPACKS_APP_NAME` to scope `bundle install` and the start command to a single app.

#### Evidence
- `theo-stacks/templates/monorepo-ruby/{apps,packages,Gemfile,Procfile}` — explicit shape.
- ADR D7.

#### Files to edit
```
core/providers/ruby/workspace.go           (NEW) — apps/ pattern detector
core/providers/ruby/workspace_test.go      (NEW)
core/providers/ruby/ruby.go                — branch on workspace
```

#### Deep file dependency analysis
- **`core/providers/ruby/workspace.go`** — `DetectRubyWorkspace(a) *RubyWorkspaceInfo`. Workspace is positively detected when both `apps/<name>` AND a top-level `Procfile` (or each app has its own Gemfile) exist.

#### Deep Dives

**Workspace shape (`monorepo-ruby`):**
```
monorepo-ruby/
├── Gemfile          # shared deps
├── Procfile         # api: cd apps/api && ruby app.rb
│                    # worker: cd apps/worker && ruby worker.rb
├── apps/
│   ├── api/{app.rb,...}
│   └── worker/{worker.rb,...}
└── packages/
    └── shared/...
```

**Detection logic:**
1. `apps/` directory exists.
2. Either: each `apps/<name>` has a `Gemfile` OR a top-level `Procfile` lists per-app processes.
3. `THEOPACKS_APP_NAME` selects which app to deploy.

**Plan for monorepo Ruby:**
- Install at root (single Gemfile.lock for all apps).
- Build (no-op for Ruby).
- Deploy: `cd apps/<name>` and run the start command (from per-app Procfile line or convention).

**Edge cases:**
- Each app has its own Gemfile (true monorepo): we'd need per-app bundle installs. Defer this complexity; document that the simple shared-Gemfile pattern is supported.

#### Tasks
1. Implement `DetectRubyWorkspace`.
2. Update `ruby.go` Plan with workspace branch.

#### TDD
```
RED:    TestDetectRubyWorkspace_AppsDir            — apps/api, apps/worker, root Gemfile → workspace
RED:    TestDetectRubyWorkspace_None               — no apps/ → nil
RED:    TestRubyProvider_Plan_Workspace_Selected   — THEOPACKS_APP_NAME=api → cd apps/api && ruby app.rb
RED:    TestRubyProvider_Plan_Workspace_Procfile   — uses Procfile line for app
RED:    TestRubyProvider_Plan_Workspace_BadName    — error listing options
GREEN:  Implement workspace + branch
REFACTOR: None.
VERIFY: mise run test ./core/providers/ruby/...
```

#### Acceptance Criteria
- [ ] Workspace correctly detected.
- [ ] Selected app builds via correct path.
- [ ] All tests pass.

#### DoD
- [ ] T4.2 tasks complete.

---

### T4.3 — Ruby example projects + goldens

#### Objective
Add `examples/ruby-rails/`, `examples/ruby-sinatra/`, `examples/ruby-monorepo/`, register integration tests, generate goldens.

#### Files to edit
```
examples/ruby-rails/                 (NEW) — minimal Rails app (rails new --minimal scope)
examples/ruby-sinatra/               (NEW) — Sinatra + config.ru
examples/ruby-monorepo/              (NEW) — apps/+packages/ + Procfile
core/dockerfile/integration_test.go  — ADD 3 cases
core/dockerfile/testdata/integration_ruby_rails.dockerfile     (NEW)
core/dockerfile/testdata/integration_ruby_sinatra.dockerfile   (NEW)
core/dockerfile/testdata/integration_ruby_monorepo.dockerfile  (NEW)
```

#### Tasks
1. Create three example directories.
2. Add integration test cases.
3. Generate goldens.

#### TDD
```
RED:    TestIntegration_RubyRails        — golden matches
RED:    TestIntegration_RubySinatra      — golden matches
RED:    TestIntegration_RubyMonorepo     — golden matches with THEOPACKS_APP_NAME=api
GREEN:  Generate goldens
VERIFY: mise run test ./core/dockerfile/...
```

#### Acceptance Criteria
- [ ] Three Ruby examples present.
- [ ] Three integration tests pass.

#### DoD
- [ ] T4.3 tasks complete.

---

## Phase 5: PHP Provider

**Objective:** Detect PHP projects via `composer.json`, distinguish Laravel / Slim / generic, install dependencies via Composer, and serve via PHP's built-in server (or Apache if needed for Laravel).

### T5.1 — Implement PHP detection + framework routing

#### Objective
Mirror the Ruby flow: detect via `composer.json`, parse to find framework signals, generate a minimal Composer-based plan.

#### Files to edit
```
core/providers/php/php.go               (NEW)
core/providers/php/version.go           (NEW)
core/providers/php/framework.go         (NEW) — Laravel / Slim / generic
core/providers/php/composer.go          (NEW) — composer.json parser (JSON, easy)
core/providers/php/php_test.go          (NEW)
core/providers/php/framework_test.go    (NEW)
```

#### Deep Dives

**Composer.json shape:**
```json
{
  "require": {
    "php": ">=8.1",
    "laravel/framework": "^11.0"
  }
}
```

**Framework detection:**
- **Laravel:** `laravel/framework` in `require` AND `artisan` file at root.
- **Slim:** `slim/slim` in `require`.
- **Symfony:** `symfony/framework-bundle` in `require`.
- **Generic:** none of the above.

**Plan steps:**
1. **install step**: `FROM php:8.3-cli-bookworm`, multi-stage `COPY --from=composer:2 /usr/bin/composer /usr/bin/composer`, `COPY composer.json composer.lock ./`, `composer install --no-dev --no-scripts --optimize-autoloader`.
2. **build step**: input from install, copy source, run `php artisan optimize` if Laravel.
3. **deploy**: `FROM php:8.3-cli-bookworm`, copy from build, `StartCmd` per framework.

**Start commands:**
- **Laravel:** `php artisan serve --host=0.0.0.0 --port=${PORT:-8000}`.
- **Slim:** `php -S 0.0.0.0:${PORT:-8000} -t public`.
- **Generic:** Procfile `web:` or `php -S 0.0.0.0:${PORT:-8000}`.

**Edge cases:**
- Laravel without `php artisan serve` exposed (production typically uses PHP-FPM + Apache) — keep dev-friendly default; users can override via `theopacks.json`.
- PHP extensions required by composer packages (`ext-mbstring`, etc.) — for now require them in the base image; if missing, document the override.

#### TDD
```
RED:    TestPhpProvider_DetectComposerJson   — composer.json → true
RED:    TestParseComposerJson_PhpRequire     — extracts php version constraint
RED:    TestDetectFramework_Laravel          — laravel/framework + artisan → FrameworkLaravel
RED:    TestDetectFramework_Slim             — slim/slim → FrameworkSlim
RED:    TestDetectFramework_Generic          — neither → FrameworkGeneric
RED:    TestPhpProvider_Plan_Laravel         — full plan with artisan serve
RED:    TestPhpProvider_Plan_Slim            — full plan with php -S
RED:    TestPhpProvider_Plan_Procfile        — Procfile wins
RED:    TestDetectPhpVersion_FromComposerReq — "php": "^8.2" → "8.2"
GREEN:  Implement
REFACTOR: None.
VERIFY: mise run test ./core/providers/php/...
```

#### Acceptance Criteria
- [ ] Three frameworks detected correctly.
- [ ] Plan generates correct multi-stage Dockerfile.
- [ ] All tests pass.

#### DoD
- [ ] T5.1 complete.

---

### T5.2 — PHP monorepo (apps/+packages/) + goldens

#### Objective
Mirror T4.2 + T4.3 for PHP. Add `examples/php-laravel/`, `examples/php-slim/`, `examples/php-monorepo/`, register integration tests, generate goldens.

#### Files to edit
```
core/providers/php/workspace.go             (NEW)
core/providers/php/workspace_test.go        (NEW)
core/providers/php/php.go                   — branch
examples/php-laravel/                       (NEW)
examples/php-slim/                          (NEW)
examples/php-monorepo/                      (NEW)
core/dockerfile/integration_test.go         — ADD 3 cases
core/dockerfile/testdata/integration_php_laravel.dockerfile     (NEW)
core/dockerfile/testdata/integration_php_slim.dockerfile        (NEW)
core/dockerfile/testdata/integration_php_monorepo.dockerfile    (NEW)
```

#### Deep Dives
- Same shape as Ruby monorepo (apps/+packages/ + root Procfile).

#### TDD
```
RED:    TestDetectPhpWorkspace            — apps/+packages/ → workspace
RED:    TestPhpProvider_Plan_Workspace    — THEOPACKS_APP_NAME selects app
RED:    TestIntegration_PhpLaravel        — golden matches
RED:    TestIntegration_PhpSlim           — golden matches
RED:    TestIntegration_PhpMonorepo       — golden matches
GREEN:  Implement + generate goldens
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] Workspace + 3 examples + 3 goldens.
- [ ] All tests pass.

#### DoD
- [ ] T5.2 complete.

---

## Phase 6: Deno Provider

**Objective:** Detect Deno projects via `deno.json`/`deno.jsonc`, resolve Deno version, plan a permission-aware build, and detect Fresh/Hono frameworks.

### T6.1 — Implement Deno detection + plan

#### Objective
`DenoProvider.Detect()` returns true if `deno.json` or `deno.jsonc` exists. `Plan()` runs `deno cache` (warmup), and the deploy stage uses `deno run`.

#### Evidence
- Railpack `core/providers/deno/` provides reference detection.
- ADR D3: Deno comes BEFORE Node in registration.

#### Files to edit
```
core/providers/deno/deno.go             (NEW)
core/providers/deno/version.go          (NEW)
core/providers/deno/config.go           (NEW) — deno.json parser
core/providers/deno/framework.go        (NEW) — Fresh / Hono detection
core/providers/deno/deno_test.go        (NEW)
```

#### Deep Dives

**deno.json shape:**
```jsonc
{
  "imports": {
    "fresh/": "https://deno.land/x/fresh@1.6.5/",
    "hono": "jsr:@hono/hono@4"
  },
  "tasks": {
    "start": "deno run -A main.ts",
    "build": "deno task build:client"
  },
  "workspace": ["./apps/api", "./apps/worker"]
}
```

**Framework detection:**
- **Fresh:** `imports` keys/values contain `fresh`.
- **Hono:** `imports` keys/values contain `hono`.
- **Generic:** none.

**Start command resolution:**
1. `tasks.start` → `deno task start`.
2. Procfile `web:` line.
3. Framework default:
   - Fresh: `deno task start` (Fresh projects always have it).
   - Hono: `deno run -A main.ts`.
   - Generic: `deno run -A main.ts`.

**Plan steps:**
1. **install step**: `FROM denoland/deno:bin-2`, `COPY deno.json deno.lock ./`, `deno cache --lock=deno.lock main.ts` (warmup).
2. **build step**: input from install, copy source, run `deno task build` if defined.
3. **deploy**: `FROM denoland/deno:distroless-2`, copy app, `StartCmd` per resolution.

**Edge cases:**
- `deno.lock` missing → still build (Deno tolerates).
- `deno.jsonc` (with comments) → use the existing JSONC parser (we depend on `tailscale/hujson`).
- Workspace (Deno 2 feature) → defer to T6.2.

#### TDD
```
RED:    TestDenoProvider_DetectDenoJson      — deno.json → true
RED:    TestDenoProvider_DetectDenoJsonc     — deno.jsonc → true
RED:    TestDenoProvider_DetectMissing       — empty → false
RED:    TestDenoProvider_DetectsBeforeNode   — package.json + deno.json → DenoProvider wins
RED:    TestParseDenoConfig_ImportsAndTasks  — imports + tasks parsed
RED:    TestDetectFramework_Fresh            — fresh/ in imports → FrameworkFresh
RED:    TestDetectFramework_Hono             — hono in imports → FrameworkHono
RED:    TestDenoProvider_Plan_Fresh          — full plan with deno task start
RED:    TestDenoProvider_Plan_Generic        — full plan with deno run -A main.ts
RED:    TestDenoProvider_Plan_Procfile       — Procfile wins
GREEN:  Implement
VERIFY: mise run test ./core/providers/deno/...
```

#### Acceptance Criteria
- [ ] Deno wins over Node when both manifests present.
- [ ] Fresh/Hono detected.
- [ ] All tests pass.

#### DoD
- [ ] T6.1 complete.

---

### T6.2 — Deno workspace support

#### Objective
Detect Deno 2 workspaces (`deno.json` `workspace` array), select target app via `THEOPACKS_APP_NAME`.

#### Files to edit
```
core/providers/deno/workspace.go       (NEW)
core/providers/deno/workspace_test.go  (NEW)
core/providers/deno/deno.go            — branch
```

#### Deep Dives
- `deno.json` `workspace` is a string array of relative paths.
- For each member, read `<path>/deno.json` to find its `name` (Deno 2 namespacing).
- Build command: `deno task -c <path>/deno.json start`.

#### TDD
```
RED:    TestDetectDenoWorkspace                 — workspace key → resolves
RED:    TestDenoProvider_Plan_Workspace         — THEOPACKS_APP_NAME selects member
RED:    TestDenoProvider_Plan_Workspace_BadName — error listing
GREEN:  Implement
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] Workspace detected; target selected.
- [ ] All tests pass.

#### DoD
- [ ] T6.2 complete.

---

### T6.3 — Deno examples + goldens

#### Objective
Add `examples/deno-fresh/`, `examples/deno-hono/`, `examples/deno-workspace/`, register integration tests, generate goldens.

#### Files to edit
```
examples/deno-fresh/                  (NEW)
examples/deno-hono/                   (NEW)
examples/deno-workspace/              (NEW)
core/dockerfile/integration_test.go   — ADD 3 cases
core/dockerfile/testdata/integration_deno_fresh.dockerfile      (NEW)
core/dockerfile/testdata/integration_deno_hono.dockerfile       (NEW)
core/dockerfile/testdata/integration_deno_workspace.dockerfile  (NEW)
```

#### TDD
```
RED:    TestIntegration_DenoFresh        — golden matches
RED:    TestIntegration_DenoHono         — golden matches
RED:    TestIntegration_DenoWorkspace    — golden matches with THEOPACKS_APP_NAME=api
GREEN:  Generate goldens
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] Three Deno examples present.
- [ ] Three integration tests pass.

#### DoD
- [ ] T6.3 complete.

---

## Phase 7: Documentation, Attribution, CHANGELOG

**Objective:** Update all user-facing documentation to reflect the expanded language matrix; record the Railpack derivation per provider in NOTICE; add the CHANGELOG entry mandated by the global engineering rules.

### T7.1 — CHANGELOG.md entry under [Unreleased]

#### Objective
Add a comprehensive Unreleased section to `CHANGELOG.md` documenting the six new providers, their default versions, supported frameworks, and any breaking changes.

#### Evidence
- The global `CLAUDE.md` Section 6 mandates that every change have a CHANGELOG entry. This is non-negotiable.
- `CHANGELOG.md` exists at the repo root.

#### Files to edit
```
CHANGELOG.md  — ADD or extend [Unreleased] section
```

#### Deep file dependency analysis
- `CHANGELOG.md` is purely documentation; no code depends on it. The format follows Keep a Changelog spec (Section 6 of global CLAUDE.md).

#### Deep Dives

**Entry shape (under `[Unreleased]`):**
```markdown
### Added
- Rust provider with Cargo + Cargo workspace support; default Rust 1, Axum/actix-web detection (#NNN)
- Java provider with Gradle (KTS + Groovy) and Maven; default Java 21, Spring Boot detection, Gradle subprojects + Maven multi-module (#NNN)
- .NET provider with .csproj/.fsproj/.vbproj single-project and .sln solution support; default .NET 8.0, ASP.NET vs console runtime routing (#NNN)
- Ruby provider with Bundler; default Ruby 3.3, Rails/Sinatra/Rack detection, apps/+packages/ monorepo with Procfile (#NNN)
- PHP provider with Composer; default PHP 8.3, Laravel/Slim detection, apps/+packages/ monorepo with Procfile (#NNN)
- Deno provider with deno.json/deno.jsonc and Deno 2 workspaces; default Deno 2, Fresh/Hono detection (#NNN)
- New environment variables: THEOPACKS_RUST_VERSION, THEOPACKS_JAVA_VERSION, THEOPACKS_RUBY_VERSION, THEOPACKS_PHP_VERSION, THEOPACKS_DOTNET_VERSION, THEOPACKS_DENO_VERSION (#NNN)

### Changed
- Provider registration order updated: Deno detection runs BEFORE Node so projects with both `deno.json` and `package.json` route correctly to Deno (#NNN)
- README.md "Supported Languages" table expanded; CLAUDE.md provider detection order documented (#NNN)

### Notes
- Default Ruby version bumped to 3.3 (theo-stacks templates currently use 3.2; users can override via THEOPACKS_RUBY_VERSION).
- Default PHP version bumped to 8.3 (theo-stacks templates currently use 8.2; users can override via THEOPACKS_PHP_VERSION).
```

**Invariants:**
- One bullet per logical capability.
- Reference ticket numbers (`#NNN`) — replaceable placeholder, real number filled in at PR creation.
- "Notes" subsection mentions defaults that diverge from theo-stacks for downstream awareness.

**Edge cases:**
- If theo-stacks team rejects the version bumps, this CHANGELOG entry must be revised to keep parity (3.2/8.2). Decision: stay on the bump, document in Notes.

#### Tasks
1. Open `CHANGELOG.md`.
2. Ensure `[Unreleased]` section exists; if not, create.
3. Add Added/Changed/Notes subsections per the shape above.

#### TDD
- N/A (documentation; no executable test). Acceptance is reviewer agreement.

#### Acceptance Criteria
- [ ] `[Unreleased]` section exists.
- [ ] Six new providers each have a bullet.
- [ ] Provider order change documented.
- [ ] Version-bump notes captured.

#### DoD
- [ ] T7.1 complete.

---

### T7.2 — Update README.md Supported Languages table + project structure

#### Objective
Extend `README.md` "Supported Languages" table with the six new entries and update the architecture section if needed.

#### Files to edit
```
README.md  — UPDATE Supported Languages table; UPDATE Environment Variables table
```

#### Deep file dependency analysis
- `README.md` is the user-facing entry. The "Supported Languages" table at the top (lines ~21-27 currently) needs six new rows.

#### Deep Dives

**New rows for the table:**
```markdown
| **Rust** | `Cargo.toml` | cargo, Cargo workspaces | `Cargo.toml` rust-version, `rust-toolchain.toml`, `THEOPACKS_RUST_VERSION` |
| **Java** | `build.gradle.kts`, `build.gradle`, `pom.xml` | Gradle, Maven, multi-module | `.java-version`, `gradle.properties`, `pom.xml`, `THEOPACKS_JAVA_VERSION` |
| **.NET** | `*.csproj`, `*.fsproj`, `*.sln` | dotnet CLI, solutions | `global.json`, `<TargetFramework>`, `THEOPACKS_DOTNET_VERSION` |
| **Ruby** | `Gemfile` | Bundler, Rails, Sinatra | `.ruby-version`, `Gemfile`, `THEOPACKS_RUBY_VERSION` |
| **PHP** | `composer.json` | Composer, Laravel, Slim | `composer.json` `require.php`, `THEOPACKS_PHP_VERSION` |
| **Deno** | `deno.json`, `deno.jsonc` | Deno 2, deno.json workspaces | `deno.json`, `THEOPACKS_DENO_VERSION` |
```

**New rows for Environment Variables table:**
- `THEOPACKS_RUST_VERSION`, `THEOPACKS_JAVA_VERSION`, `THEOPACKS_RUBY_VERSION`, `THEOPACKS_PHP_VERSION`, `THEOPACKS_DOTNET_VERSION`, `THEOPACKS_DENO_VERSION`.

**Detection order section:**
> "Detection order is fixed (first match wins): **Go → Rust → Java → .NET → Ruby → PHP → Python → Deno → Node → Static → Shell**."

#### Tasks
1. Update Supported Languages table.
2. Update Environment Variables table.
3. Update detection-order line.

#### Acceptance Criteria
- [ ] Six new languages in the table.
- [ ] Six new env vars documented.
- [ ] Detection order accurate.

#### DoD
- [ ] T7.2 complete.

---

### T7.3 — Update CLAUDE.md provider detection order + env vars

#### Objective
Mirror the README updates in `CLAUDE.md` (Provider Detection Order section, Environment Variables table, Common Mistakes table if applicable).

#### Files to edit
```
CLAUDE.md  — UPDATE Provider Detection Order, Environment Variables, Adding Provider step
```

#### Deep Dives
- The "Adding a New Provider" section in CLAUDE.md should mention that step 4 includes regenerating golden Dockerfiles for the new examples.

#### Acceptance Criteria
- [ ] Detection order matches D3.
- [ ] Env vars consistent with README.

#### DoD
- [ ] T7.3 complete.

---

### T7.4 — Update NOTICE with per-provider Railpack attribution

#### Objective
Extend `NOTICE` with a table of which Railpack providers (rust, java, ruby, php, dotnet, deno) were the source for which theo-packs files. The general acknowledgement is already there; this adds traceability.

#### Files to edit
```
NOTICE  — APPEND per-provider attribution table
```

#### Deep Dives

**New section:**
```
Per-provider derivations:

  * core/providers/rust/        — derived from railpack/core/providers/rust
  * core/providers/java/        — derived from railpack/core/providers/java
  * core/providers/ruby/        — derived from railpack/core/providers/ruby
  * core/providers/php/         — derived from railpack/core/providers/php
  * core/providers/dotnet/      — derived from railpack/core/providers/dotnet
  * core/providers/deno/        — derived from railpack/core/providers/deno

The detection logic (manifest names, version-resolution priorities, framework
hints) is ported essentially as-is. The Plan() implementation in each provider
diverges from upstream because theo-packs uses language-specific Docker base
images while Railpack uses mise-driven LLB primitives. Test fixtures, where
ported, retain attribution in their headers.
```

#### Tasks
1. Append the section to NOTICE.
2. Ensure SPDX-License-Identifier and original Railpack copyright are linked.

#### Acceptance Criteria
- [ ] Per-provider attribution explicit.
- [ ] License compatibility (Apache 2.0) confirmed.

#### DoD
- [ ] T7.4 complete.

---

### T7.5 — Add SPDX headers to ported files

#### Objective
Every new `.go` file derived from Railpack must carry an SPDX header noting the derivation. This is the file-level companion to NOTICE.

#### Files to edit
```
core/providers/rust/*.go      — ADD header
core/providers/java/*.go      — ADD header
core/providers/ruby/*.go      — ADD header
core/providers/php/*.go       — ADD header
core/providers/dotnet/*.go    — ADD header
core/providers/deno/*.go      — ADD header
```

#### Deep Dives

**Header template:**
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).
```

For files that are NOT ported (e.g., images.go additions), keep the existing header style without the "derived from" line.

#### Acceptance Criteria
- [ ] All ported files have the derivation note.
- [ ] Files written from scratch DO NOT have the derivation note (avoid over-claiming).

#### DoD
- [ ] T7.5 complete.

---

## Phase 8: Integration Tests + Golden Files (Aggregation)

**Objective:** Confirm that integration tests cover all new examples consistently, golden files are reviewed, and no Dockerfile regression has happened in the existing examples.

### T8.1 — Final integration test pass + golden review

#### Objective
Run `mise run test` end-to-end, then `UPDATE_GOLDEN=true go test ./core/dockerfile/...`, manually review every NEW golden file, and run regular `go test` to confirm everything is stable.

#### Evidence
- Phases 1-6 each generate 3 goldens (~18 total). Phase 8 ensures they pass collectively without flakes from cross-language interactions.

#### Files to edit
- None directly. Verification + commit of golden files.

#### Deep file dependency analysis
- All goldens already exist after Phases 1-6. This task is a checkpoint.

#### Deep Dives

**Manual review checklist per golden:**
1. Build image matches D4 default (or env-overridden version).
2. Runtime image matches the language's runtime choice (slim, distroless, JRE).
3. `WORKDIR /app` (consistent across providers).
4. `EXPOSE` directive exists for HTTP-serving frameworks.
5. `CMD` matches the framework's expected start command.
6. No leaked secrets or env vars.

**Regression check:** existing goldens (go-simple, node-npm, python-flask, etc.) MUST be byte-identical after this PR. If any change, investigate before accepting.

#### Tasks
1. `mise run test` — must be green.
2. `UPDATE_GOLDEN=true go test ./core/dockerfile/...` — regenerate goldens.
3. `git diff core/dockerfile/testdata/` — review, approve only the new files.
4. If any pre-existing golden changes, investigate root cause.
5. Commit golden files in a dedicated commit (`test(integration): add golden Dockerfiles for new providers`).

#### TDD
- N/A; this is a verification phase, not implementation.

#### Acceptance Criteria
- [ ] All ~18 new golden files exist and are reviewed.
- [ ] All pre-existing golden files are unchanged.
- [ ] `mise run test` passes.

#### DoD
- [ ] T8.1 complete.

---

## Phase 9: E2E + Final QA

**Objective:** Run the E2E suite for each new language at least once; ensure CI configuration handles the longer test runtime; finalize PR.

### T9.1 — Add E2E test entries

#### Objective
Add one `TestE2E_<Lang>_*` test per language to `e2e/e2e_test.go` exercising at least the simple example + workspace example with real Docker.

#### Evidence
- `e2e/e2e_test.go` is the existing E2E harness, gated by build tag `e2e`.

#### Files to edit
```
e2e/e2e_test.go  — ADD test funcs for the 6 new languages
```

#### Deep Dives

**Test func template (per language):**
```go
func TestE2E_Rust_AxumBuildsImage(t *testing.T) {
    if !dockerAvailable() {
        t.Skip("Docker not available")
    }

    dir := filepath.Join(examplesDir(t), "rust-axum")
    df := generateDockerfile(t, dir, nil)

    tag := "theopacks-test:rust-axum"
    defer removeImage(tag)

    buildImage(t, dir, df, tag)
    require.True(t, imageExists(tag))
}
```

Add ~12 test funcs (2 per language minimum: simple + workspace).

**Time budget:**
- Rust: ~120s per build (cargo cold build).
- Java: ~90s per build (Gradle download).
- .NET: ~60s per build (dotnet restore).
- Ruby: ~30s per build (small gems).
- PHP: ~20s per build (composer fast).
- Deno: ~30s per build (deno cache).

Total: ~12 builds × 60s avg = ~12 min added to E2E. Use `-timeout 1500s` (25 min).

#### Tasks
1. Add `TestE2E_Rust_*`, `TestE2E_Java_*`, `TestE2E_Dotnet_*`, `TestE2E_Ruby_*`, `TestE2E_Php_*`, `TestE2E_Deno_*` (12+ funcs).
2. Run `go test -tags e2e ./e2e/ -timeout 1500s` locally.
3. Confirm all pass on a Docker-enabled host.

#### TDD
- These ARE the tests. Run with the e2e tag and require green status.

#### Acceptance Criteria
- [ ] ≥12 new E2E test functions.
- [ ] All pass on a host with Docker available.
- [ ] CI workflow updated to allow longer timeout if needed (separate ticket if CI runner can't accommodate).

#### DoD
- [ ] T9.1 complete.

---

### T9.2 — Final repo-wide check

#### Objective
Run the full quality gate one more time against the integrated branch, then prepare the PR description.

#### Files to edit
- None.

#### Tasks
1. `mise run check` — zero warnings.
2. `mise run test` — all unit + integration tests green.
3. `go test -tags e2e ./e2e/ -timeout 1500s` — green on host with Docker.
4. Review CHANGELOG.md, README.md, CLAUDE.md, NOTICE for accuracy.
5. Author PR description with: scope summary, per-language reviewer checklist, deferred-work list (e.g., Rails asset pipeline), default-version bump notice.
6. Push branch, open PR, link to this plan.

#### Acceptance Criteria
- [ ] All quality checks pass.
- [ ] PR description references this plan.
- [ ] Per-language reviewer checklist included.

#### DoD
- [ ] T9.2 complete.
- [ ] PR opened.

---

## Coverage Matrix

| # | Gap / Requirement | Task(s) | Resolution |
|---|---|---|---|
| 1 | Rust language not supported (theo-stacks `rust-axum`, `monorepo-rust`) | T1.1, T1.2, T1.3 | Provider with Cargo + workspaces + Axum detection + 3 examples + goldens |
| 2 | Java language not supported (theo-stacks `java-spring`, `monorepo-java`) | T2.1, T2.2, T2.3 | Provider with Gradle + Maven + Spring Boot + multi-module + 3 examples + goldens |
| 3 | Ruby language not supported (theo-stacks `ruby-sinatra`, `monorepo-ruby`) | T4.1, T4.2, T4.3 | Provider with Bundler + Rails + Sinatra + apps/+packages/ monorepo + 3 examples + goldens |
| 4 | PHP language not supported (theo-stacks `php-slim`, `monorepo-php`) | T5.1, T5.2 | Provider with Composer + Laravel + Slim + apps/+packages/ monorepo + 3 examples + goldens |
| 5 | .NET language not supported (user-requested addition) | T3.1, T3.2, T3.3 | Provider with .csproj + .sln + ASP.NET routing + 3 examples + goldens |
| 6 | Deno language not supported (user-requested addition) | T6.1, T6.2, T6.3 | Provider with deno.json + workspaces + Fresh/Hono detection + 3 examples + goldens |
| 7 | Monorepo target selection mechanism | D7, T1.2, T2.2, T3.2, T4.2, T5.2, T6.2 | Reuse THEOPACKS_APP_NAME / THEOPACKS_APP_PATH; per-language plan branch |
| 8 | Provider order conflict (Deno vs Node manifest) | D3, T0.2 | Deno registered before Node; tested in provider_test.go |
| 9 | Base image strategy (mise vs language-specific) | D1 | Decision documented; new providers use language-specific images consistently with existing 5 |
| 10 | Framework auto-detection per language | D5, T1.1, T2.1, T3.1, T4.1, T5.1, T6.1 | Spring Boot, Rails, Sinatra, Laravel, Slim, ASP.NET, Fresh, Hono detected; generic fallbacks |
| 11 | Default versions selection | D4, T0.1 | LTS-aligned defaults documented in helper constants and CHANGELOG |
| 12 | Apache 2.0 attribution for ported code | D2, T7.4, T7.5 | Per-provider table in NOTICE + SPDX headers in each ported file |
| 13 | CHANGELOG entry (mandated by global rules) | T7.1 | Unreleased section with Added/Changed/Notes |
| 14 | README.md Supported Languages table | T7.2 | Six new rows + env var rows + detection order |
| 15 | CLAUDE.md provider detection order | T7.3 | Updated detection order + env var list |
| 16 | E2E coverage for new providers | T9.1 | ≥12 new E2E functions covering simple + workspace per language |
| 17 | Single-PR delivery | D6, T9.2 | One PR with 10 logical commits + comprehensive description |

**Coverage: 17/17 gaps covered (100%).**

---

## Global Definition of Done

- [ ] All 9 phases completed.
- [ ] `mise run test` green (unit + integration).
- [ ] `mise run check` green (zero `go vet`, `go fmt`, `golangci-lint` warnings).
- [ ] `go test -tags e2e ./e2e/ -timeout 1500s` green on a Docker-enabled host.
- [ ] All ~18 new golden Dockerfiles reviewed and committed.
- [ ] All pre-existing golden Dockerfiles unchanged (no regressions in Go/Node/Python/Static/Shell).
- [ ] Six new providers register in the order specified by D3.
- [ ] CHANGELOG.md `[Unreleased]` section captures all changes per Keep a Changelog format.
- [ ] README.md Supported Languages table covers 11 languages (5 existing + 6 new).
- [ ] CLAUDE.md provider detection order accurate.
- [ ] NOTICE includes per-provider Railpack attribution + SPDX headers in all ported files.
- [ ] Single PR opened with 10 logical commits and reviewer checklist.
- [ ] No file in the new providers exceeds 350 lines.
- [ ] Backward compatibility: existing `theopacks.json` configs and `THEOPACKS_*` env vars continue to work.
- [ ] Dev-friendly: `npm create theo@latest` flows that target `rust-axum`, `java-spring`, `ruby-sinatra`, `php-slim` produce deployable builds without hand-editing the generated Dockerfile.
