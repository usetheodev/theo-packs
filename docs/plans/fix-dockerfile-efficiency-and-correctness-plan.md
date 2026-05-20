# Plan: Fix Dockerfile Generation Efficiency and Correctness Gaps

> **Version 1.0** — Address 11 audit findings on the Dockerfiles theo-packs generates today: critical runtime correctness (bash CMD wrapping that breaks on slim/distroless images, double `sh -c` quoting bugs that mangle apt-get / bundle config), efficiency gaps (zero BuildKit cache mounts, oversized runtime images, universal `sh -c` overhead), and missing production defaults (USER non-root, HEALTHCHECK, size-optimization flags). The fix touches both the central renderer (`core/dockerfile/generate.go`, `core/plan/command.go`, `core/plan/step.go`) and the per-language providers. Done = generated Dockerfiles are byte-for-byte identical to the new expected production-grade output for all 30+ examples, and a real Docker build (E2E suite) succeeds for every language including the slim/distroless variants.

## Context

### What exists today

theo-packs renders Dockerfiles via `core/dockerfile/generate.go` from a `BuildPlan` produced by language providers. After landing the 6 new providers (Rust, Java, .NET, Ruby, PHP, Deno) in PR #14 (`develop → main`), an E2E run on a Docker host surfaced three immediate failures (PHP `apt-get` quoting broken, Deno image tag invalid) which were patched in `72239e9`. A subsequent audit of the resulting Dockerfiles revealed that the patch was a thin layer over deeper structural problems shared by **every** generated Dockerfile, not just the new providers.

### Concrete evidence (the 11 findings)

| # | Severity | File / Line | Evidence |
|---|----------|-------------|----------|
| 1 | 🔴 Critical | `core/dockerfile/generate.go:99` | Hardcoded `CMD ["/bin/bash", "-c", %q]`. `debian:bookworm-slim` does not ship bash; `denoland/deno:distroless-*` has no shell at all. App fails at start. Bash also captures PID 1 → SIGTERM never reaches the real process → graceful shutdown is broken. |
| 2 | 🔴 Critical | `core/providers/java/{gradle,maven}.go` (the JAR-extract `RUN`) | Double `sh -c` wrapping mangles quoting: `RUN sh -c 'sh -c 'set -e; jar=$(ls build/libs/*.jar ...) cp "$jar" /app/app.jar''`. Same class of bug fixed earlier in PHP/Ruby; missed in Java. |
| 3 | 🟠 Critical | `core/providers/ruby/ruby.go` | `RUN sh -c 'bundle config set --local without 'development test''` — single-quotes inside single-quotes split into `'development test'` adjacent. Works by accident because bundler tolerates the noise. |
| 4 | 🟠 Inefficient | `core/plan/command.go:60` (`NewExecShellCommand`) | Every command is wrapped in `sh -c '...'`. For a plain `go mod download` the wrapper adds a process layer with no benefit. Universal across every RUN of every Dockerfile we emit. |
| 5 | 🟠 Inefficient | All 6 new providers + the original 5 | **Zero BuildKit `--mount=type=cache,target=...`** for package-manager caches: cargo registry/git/target, gradle `~/.gradle`, maven `~/.m2`, nuget `~/.nuget/packages`, composer `~/.composer/cache`, bundler `/usr/local/bundle`, pip `~/.cache/pip`, npm `~/.npm`, go `~/.cache/go-build` + `/go/pkg/mod`. Cold rebuilds redownload everything. |
| 6 | 🟠 Inefficient | `core/plan/step.go:22`, `core/dockerfile/generate.go:176-178`, `core/core.go:210` | Step.Secrets defaults to `["*"]`, plan-level secrets are populated from **every** env var name, and the renderer mounts every secret on every RUN regardless of whether the command actually references it. Result: `RUN --mount=type=secret,id=THEOPACKS_START_CMD sh -c 'pip install ...'` (pip doesn't read THEOPACKS_START_CMD). |
| 7 | 🟡 Inefficient | `core/generate/images.go` (Go/Rust runtime) | `debian:bookworm-slim` (~80MB) used for Go static binaries and Rust where `gcr.io/distroless/static-debian12` (~2MB) or `scratch` would work. .NET uses `dotnet/aspnet:8.0` (~210MB) where `dotnet/aspnet:8.0-alpine` (~120MB) is published. |
| 8 | 🟡 Best practice | All providers | No `USER appuser` directive in deploy stage. App runs as root in production. |
| 9 | 🟡 Best practice | All providers (lock+manifest copies) | `COPY package.json package-lock.json ./` puts both files in the same layer — touching one invalidates the other. Same for Cargo.toml/Cargo.lock, composer.json/composer.lock, etc. Splittable. |
| 10 | 🟡 Best practice | All HTTP-serving providers | No `HEALTHCHECK` directive even when framework detection identifies an HTTP server (Spring Boot, ASP.NET, Sinatra, Slim, Hono). |
| 11 | 🟡 Best practice | Go, Rust, .NET providers | Missing per-language size flags: Go `-ldflags="-s -w"` (~30% smaller), Rust `RUSTFLAGS="-C strip=symbols"` + Cargo.toml `lto=true`, .NET `--no-self-contained` + trim. |

### Why this is the right time

PR #14 lands 6 new languages. Image regressions caught now travel into 9 active language stacks instead of 11+ once Elixir, Gleam, etc. ship. The renderer changes (findings 1, 4, 6) touch one file each in `core/`; the per-language fixes (5, 7, 8, 10, 11) are independent and can ship per-language without breaking each other. Doing all of this in one focused effort — informed by a real E2E run that already showed bash-CMD-style failures — is dramatically cheaper than retrofitting 11 patches over months.

### Related references

- Audit of generated Dockerfiles (this conversation, "Are we building Docker specs efficiently?" exchange).
- E2E failure log from PR #14 CI run (PHP `apt-get` no-args + Deno bin-2 not found), already patched in `72239e9`.
- Upstream Railpack's renderer (`github.com/railwayapp/railpack/core/dockerfile/generate.go`) which uses the same `["/bin/bash", "-c", ...]` shape we inherited — the fix is a deliberate divergence from upstream. Document in `NOTICE`.

## Objective

**Done = the Dockerfiles theo-packs generates are correct on every supported runtime image (including slim and distroless) and within 5% of hand-crafted production Dockerfiles for size and cold-build time, validated end-to-end on a Docker host through every example project.**

Specific, measurable goals:

1. **No `/bin/bash`-dependent CMDs.** Every generated Dockerfile boots on `gcr.io/distroless/static-debian12`, `denoland/deno:distroless-*`, and `scratch` where applicable.
2. **No double-`sh -c` quoting bugs.** A grep of every golden for `sh -c 'sh -c '` returns zero matches.
3. **`go test -tags e2e ./e2e/ -timeout 1500s` green** on a Docker host for all 12 new and 7 pre-existing E2E test functions.
4. **Cold build wall time per language reduced ≥40%** vs current state, measured via `mise run test-e2e` end-to-end timings before/after on the same runner.
5. **Runtime image size reduced ≥30%** for Go, Rust, .NET, where the runtime image change is applied (measured via `docker image inspect <tag> --format '{{.Size}}'`).
6. **Smart secret mounting.** A RUN command only gets `--mount=type=secret,id=NAME` if the command actually references `$NAME` or `${NAME}` (substring match).
7. **`USER` directive present** in every deploy stage that uses a non-distroless base image (distroless already runs as nonroot user 65532).
8. **`HEALTHCHECK` present** in every Dockerfile where framework detection identifies an HTTP server.
9. **Lock and manifest are split into separate `COPY` directives** so a manifest-only change does not invalidate the lockfile cache layer.
10. **Per-language size optimization flags applied**: Go `-ldflags="-s -w"`, Rust `strip=symbols` + `lto=true`, .NET `-p:PublishTrimmed=true` (where compatible).
11. **All ~50 golden Dockerfiles regenerated and reviewed**, no shape regressions in pre-existing examples (Go/Node/Python/Static/Shell content is preserved modulo the 11 fixes).
12. **Backward compatibility preserved** — `theopacks.json` config, `THEOPACKS_*` env vars, `Provider` interface, and the public CLI flags all behave the same.
13. **Zero new lint warnings** (`golangci-lint run ./...`).

## ADRs

### D1 — Default `CMD` to **exec form**, no shell wrapping
**Decision:** The renderer emits `CMD ["arg0", "arg1", ...]` (JSON exec form, no shell) when the start command is a single recognizable program with arguments. When it requires shell features (pipes, env-var expansion via `${PORT:-8000}`, `||`, `&&`), it falls back to `CMD ["/bin/sh", "-c", "..."]` — never `/bin/bash`.

**Rationale:** Exec form makes the app PID 1 → SIGTERM works → graceful shutdown works → Kubernetes / docker stop don't have to send SIGKILL. `/bin/sh` is present in all images we emit (debian-slim, php-cli-bookworm, ruby-bookworm-slim, denoland/deno:debian, eclipse-temurin, dotnet/aspnet, etc.) **except** distroless and scratch. For distroless, exec form is the only form that works. Bash is not present in slim images.

**Consequences:**
- Enables: distroless / scratch runtime images; correct signal handling.
- Constrains: providers whose start command needs shell features must declare so explicitly (a new `StartCmdNeedsShell bool` on `DeployBuilder`); the renderer won't auto-wrap.

### D2 — Strip universal `sh -c` from `RUN` directives
**Decision:** `NewExecShellCommand` keeps its `sh -c '...'` wrapping (legacy callers rely on shell features in their command bodies). Add a new `NewExecCommand` (already exists for raw exec form) that the renderer outputs as `RUN <cmd>` without any wrapping. Migrate provider call sites that don't need shell features (most `cargo fetch`, `go mod download`, `npm ci`, `dotnet restore` calls) to `NewExecCommand`.

**Rationale:** Plain shell-less `RUN go mod download` is one process; `RUN sh -c 'go mod download'` is two. Across 50+ generated Dockerfiles this is real overhead. Keeping the legacy `NewExecShellCommand` for shell-needing commands preserves backward compat with any in-tree code we don't touch.

**Consequences:**
- Enables: lighter Dockerfiles, less process overhead.
- Constrains: each migrated call must be audited — does it use pipes/redirects/env-expansion? If yes, keep `NewExecShellCommand`; if no, switch.

### D3 — Smart secret mounting via substring match
**Decision:** The renderer no longer mounts every plan-level secret on every RUN. Instead, for each RUN, it mounts only the secrets whose name (`$NAME` or `${NAME}`) appears in the command string.

**Rationale:** Today's `Step.Secrets = ["*"]` default + plan-level secrets being populated from every env var means a `pip install` gets `--mount=type=secret,id=THEOPACKS_START_CMD`. That's noise. The substring check is O(N×M) where N=number of secrets and M=number of RUNs, which is fine at our scale (single-digit each).

**Consequences:**
- Enables: clean Dockerfiles; no more spurious mounts; faster builds (fewer layers carry mount metadata).
- Constrains: providers that need a secret in a command must reference it explicitly (`$THEOPACKS_BUILD_CMD` or similar). Not a real constraint — none of our current providers do.

### D4 — Cache mounts via per-package-manager `--mount=type=cache`
**Decision:** Every install/build step that invokes a package manager (`cargo`, `gradle`, `mvn`, `dotnet`, `composer`, `bundle`, `pip`, `npm`, `go`) attaches `--mount=type=cache,target=<path>,sharing=locked` for the manager's standard cache directory. Mount targets:

| PM | Mount target |
|----|--------------|
| Go | `/root/.cache/go-build` + `/go/pkg/mod` |
| Cargo | `/root/.cargo/registry` + `/root/.cargo/git` + `/app/target` |
| Gradle | `/root/.gradle` |
| Maven | `/root/.m2` |
| dotnet/NuGet | `/root/.nuget/packages` |
| Composer | `/root/.composer/cache` |
| Bundler | `/usr/local/bundle` |
| pip | `/root/.cache/pip` |
| npm | `/root/.npm` |

**Rationale:** Cold rebuilds redownload everything today. The `BuildPlan.Caches` map already exists in the data model — we just don't populate it from providers. Adding cache mounts to the renderer's RUN emission is a one-time change; populating per-provider is mechanical.

**Consequences:**
- Enables: ≥40% faster cold builds, far better developer iteration loop, lower CI minutes.
- Constrains: cache mounts are scoped to each builder (BuildKit handles concurrency via `sharing=locked`); no behavior change for users running plain `docker build` without BuildKit.

### D5 — Slim/distroless runtime images for compiled languages
**Decision:**
- **Go**: runtime → `gcr.io/distroless/static-debian12:nonroot` (Go binary is fully static, ~2MB image, runs as UID 65532).
- **Rust**: runtime → `gcr.io/distroless/cc-debian12:nonroot` (Rust by default links against glibc; cc-distroless includes it; ~17MB).
- **.NET**: runtime stays `dotnet/aspnet:8.0` for ASP.NET (alpine variant exists but introduces musl/glibc surprises) but switches to `dotnet/runtime-deps:8.0-alpine` for **console/worker** projects (~80MB vs 200MB).
- **Java**: runtime stays `eclipse-temurin:21-jre` (jlink-based custom JRE is a future optimization).
- **Ruby/PHP/Deno/Python/Node**: runtime stays as today (all interpreted, full image needed).

**Rationale:** Static-binary languages benefit hugely from distroless. Other languages don't have an equivalent slim option without losing functionality.

**Consequences:**
- Enables: 30-95% smaller runtime images for Go/Rust/.NET console.
- Constrains: distroless has no shell — `CMD` must be exec form (already in D1). Debugging the running container needs `kubectl debug --image=...` or rebuild with non-distroless base; document in CLAUDE.md.

### D6 — Run as non-root user via `USER` directive
**Decision:** Every deploy stage that uses a non-distroless base image gets a `USER appuser` directive (with the user created in the deploy stage). Distroless images already use `nonroot` (UID 65532) — D5 handles those.

**Rationale:** Production containers should not run as root. Reduces blast radius if a process is compromised. Standard for Kubernetes pod security policies.

**Consequences:**
- Enables: drop-in deploy on locked-down clusters (PSP/OPA gatekeeper).
- Constrains: apps that need to bind ports < 1024 will fail; document the override path via `theopacks.json`.

### D7 — `HEALTHCHECK` for HTTP-serving frameworks
**Decision:** When framework detection identifies an HTTP server (Spring Boot, ASP.NET Core, Rails, Sinatra, Rack, Laravel, Slim, Symfony, Fresh, Hono, generic-Node-with-port, FastAPI, Flask, Express, Next, etc.), emit:

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD curl -fsS http://localhost:${PORT:-<framework-default>}/health || exit 1
```

with the port and path defaulted by framework. Apps without a `/health` endpoint will fail the healthcheck — that's a feature; frameworks like Spring Boot Actuator and ASP.NET Core ship `/health` out of the box, others scaffolded by `theo-stacks` already include it (per `ROADMAP.md` Rule 2 of the Sprint 1 templates).

**Rationale:** Kubernetes / Docker Compose use `HEALTHCHECK` to mark containers `unhealthy` and route traffic away. Without it, a deadlocked app keeps receiving requests.

**Consequences:**
- Enables: zero-config healthchecks for the 80% case.
- Constrains: needs `curl` in the runtime image (debian-slim has it; alpine variants need `RUN apk add --no-cache curl`); for distroless, fall back to a `tcp` healthcheck via `dial` (Go binary). Document.

### D8 — Per-language size optimization flags
**Decision:**
- **Go**: build command becomes `go build -ldflags="-s -w" -trimpath -o /app/server <target>`. Strips debug info (~30% smaller binary).
- **Rust**: install step injects a `cargo` config snippet pinning `[profile.release] strip = "symbols"` and `lto = "thin"`. Optionally enable via `THEOPACKS_RUST_OPTIMIZE=size`.
- **.NET**: publish becomes `dotnet publish -c Release -p:PublishSingleFile=false -p:DebugType=None -p:DebugSymbols=false`. Trimming (`-p:PublishTrimmed=true`) is opt-in via `THEOPACKS_DOTNET_TRIM=true` because trimming breaks reflection-heavy code (EF Core, AutoMapper).
- **Java**: no change in this PR — jlink is a future optimization.

**Rationale:** Free wins on image size. Trim is opt-in because of the reflection trap.

**Consequences:**
- Enables: smaller binaries.
- Constrains: stripped binaries can't be debugged with stack traces; document.

### D9 — Single PR with one commit per phase
**Decision:** All work lands in one PR, branch `feat/dockerfile-correctness-and-efficiency`, with one commit per Phase. Commits are bisectable: if a regression appears post-merge, the offending commit can be reverted in isolation.

**Rationale:** Same reasoning as PR #14: per-phase commits give the reviewer focus and the operator rollback granularity.

**Consequences:**
- Enables: focused review, granular rollback.
- Constrains: PR will be ~1500-2500 lines; reviewers need ≥1 hour. Acceptable.

## Dependency Graph

```
Phase 0 (Foundation: renderer + plan model)
    │
    ├──▶ Phase 1 (Critical correctness: CMD exec form + Java/Ruby quoting)
    │
    ├──▶ Phase 2 (Smart secret mounting)
    │
    ├──▶ Phase 3 (Cache mounts per language) ─┐
    │                                          │
    ├──▶ Phase 4 (Slim/distroless runtimes) ──┤
    │                                          │
    ├──▶ Phase 5 (USER + HEALTHCHECK + size flags) ─┐
    │                                                │
    └──▶ Phase 6 (Layer cache micro-opts) ──────────┤
                                                     │
                                                     ▼
                                              Phase 7 (Goldens + E2E + docs)
```

- **Phase 0** is the hard prerequisite for Phases 1, 2 (renderer changes).
- **Phases 3, 4, 5, 6** are mutually independent and can land in any order after Phase 0.
- **Phase 7** depends on all of the above (goldens get regenerated last).

---

## Phase 0: Foundation — renderer + plan model changes

**Objective:** Land the central renderer/data-model changes that subsequent phases depend on, without changing observable Dockerfile output yet.

### T0.1 — Add `CommandKind` field to `plan.Command` so renderer knows when to wrap

#### Objective
Distinguish "needs shell" commands from "exec-form" commands at the data-model level so the renderer can emit the right form per-command.

#### Evidence
Today `core/plan/command.go:60` always wraps in `sh -c '...'`. The renderer has no signal to choose between exec form and shell form. Hardcoding shell form is the source of finding #4.

#### Files to edit
```
core/plan/command.go     — ADD CommandKind enum (CommandKindExec, CommandKindShell, CommandKindCopy, CommandKindFile, CommandKindPath); set on each NewXxx constructor
core/plan/command_test.go — ADD tests for each constructor's CommandKind
core/dockerfile/generate.go — READ CommandKind to decide RUN form
core/dockerfile/generate_test.go — table-driven tests covering both forms
```

#### Deep file dependency analysis
- **`core/plan/command.go`** is imported by every provider's Plan() body. Adding a new field is purely additive. Existing call sites (`NewCopyCommand`, `NewExecShellCommand`, `NewPathCommand`, `NewFileCommand`, `NewExecCommand`) continue to compile.
- **`core/dockerfile/generate.go`** consumes `Command` to emit Dockerfile directives. Today it special-cases by Go type assertion; it'll add a switch on `CommandKind` for cleaner dispatch.
- **No provider needs to change** in this task. Existing `NewExecShellCommand` calls keep emitting `sh -c '...'` form via `CommandKindShell`. Phase 2 migrates targeted call sites.

#### Deep Dives

**Type:**
```go
type CommandKind int

const (
    CommandKindExec  CommandKind = iota // RUN <cmd> with no wrapping (NewExecCommand)
    CommandKindShell                    // RUN sh -c '<cmd>' (NewExecShellCommand)
    CommandKindCopy                     // COPY <src> <dst>
    CommandKindFile                     // RUN echo ... > file (asset injection)
    CommandKindPath                     // ENV PATH=...
)
```

**Renderer dispatch (`generate.go`):**
```go
switch cmd.Kind {
case CommandKindExec:
    fmt.Fprintf(b, "RUN %s\n", cmd.Body)
case CommandKindShell:
    fmt.Fprintf(b, "RUN sh -c '%s'\n", escape(cmd.Body))
case CommandKindCopy:
    ...
}
```

**Invariants:**
- Every constructor in `command.go` sets `Kind` deterministically.
- Renderer never falls through without setting a form (compile-time exhaustiveness).
- Existing `NewExecShellCommand("foo")` still produces `sh -c 'foo'`.

**Edge cases:**
- A `CommandKindShell` body containing `'` needs escaping (already handled today; preserve).
- An empty command body is a programming error; renderer panics (with a clear message including the step name).

#### Tasks
1. Add `CommandKind` enum to `core/plan/command.go`.
2. Add `Kind CommandKind` to the existing `Command` struct.
3. Set `Kind` correctly in every `NewXxxCommand` constructor.
4. Update renderer in `core/dockerfile/generate.go` to switch on `Kind`.
5. Verify all integration goldens unchanged (no migration of existing provider calls yet — same `Kind` for them).

#### TDD
```
RED:    TestCommand_NewExecCommand_KindIsExec        — Kind == CommandKindExec
RED:    TestCommand_NewExecShellCommand_KindIsShell  — Kind == CommandKindShell
RED:    TestCommand_NewCopyCommand_KindIsCopy        — Kind == CommandKindCopy
RED:    TestCommand_NewPathCommand_KindIsPath        — Kind == CommandKindPath
RED:    TestCommand_NewFileCommand_KindIsFile        — Kind == CommandKindFile
RED:    TestRenderer_ExecKindEmitsRawRun             — `RUN <cmd>` no sh -c
RED:    TestRenderer_ShellKindEmitsShCWrap           — `RUN sh -c '<cmd>'`
GREEN:  Implement Kind field + renderer switch
REFACTOR: None expected.
VERIFY: mise run test ./core/plan/... ./core/dockerfile/...
```

#### Acceptance Criteria
- [ ] `CommandKind` exists with 5 variants matching the constructors.
- [ ] All 5 unit tests for kind-mapping pass.
- [ ] Renderer dispatch test passes for both Exec and Shell forms.
- [ ] All ~30 pre-existing golden Dockerfiles **byte-identical** (no migration yet).
- [ ] `mise run check` clean.
- [ ] File length: `command.go` ≤ 250 lines, `generate.go` ≤ 600 lines.

#### DoD
- [ ] All tasks complete.
- [ ] Tests green (`mise run test`).
- [ ] No regressions in goldens.

---

### T0.2 — Make `Step.Secrets` default empty + add substring auto-detect

#### Objective
Stop the renderer from mounting every plan-level secret on every RUN. Auto-mount secrets only when the command actually references them.

#### Evidence
Finding #6: today `core/plan/step.go:22` defaults `Secrets: []string{"*"}`, and `core/core.go:210` populates `plan.Secrets` from **every** env var. Result: bogus mounts like `RUN --mount=type=secret,id=THEOPACKS_START_CMD sh -c 'pip install ...'`.

#### Files to edit
```
core/plan/step.go               — default Secrets = []string{} (was ["*"])
core/dockerfile/generate.go     — replace resolveSecrets() with autoDetectSecrets() that scans command body for $NAME / ${NAME}
core/dockerfile/generate_test.go — table-driven coverage of the substring matcher
core/dockerfile/integration_test.go — env vars stay; goldens regenerate (this is the visible side effect)
```

#### Deep file dependency analysis
- **`core/plan/step.go`** — change one line literal. Tests on Step's defaults need an update.
- **`core/dockerfile/generate.go`** — `resolveSecrets` becomes `autoDetectSecrets(cmdBody, planSecrets) []string`. Returns the subset of plan secrets whose `$NAME` or `${NAME}` token appears in the command body. The wildcard `*` syntax is preserved as an escape hatch (explicit user intent), but `["*"]` is no longer the default.
- **Provider effect**: providers continue to call `step.UseSecrets("X", "Y")` if they really need explicit mounts. None of our current providers do.

#### Deep Dives

**Substring algorithm (`autoDetectSecrets`):**
```go
func autoDetectSecrets(cmdBody string, planSecrets []string) []string {
    var matched []string
    for _, name := range planSecrets {
        if name == "" { continue }
        if strings.Contains(cmdBody, "$"+name) ||
           strings.Contains(cmdBody, "${"+name+"}") {
            matched = append(matched, name)
        }
    }
    sort.Strings(matched)
    return matched
}
```

**Invariants:**
- Output is sorted (stable Dockerfile output).
- A command that doesn't reference any secret produces no `--mount=type=secret`.
- A wildcard step (`Secrets: ["*"]` set explicitly by config) keeps current behavior.

**Edge cases:**
- Substring within a different identifier (`$THEOPACKS_VARS_FOO` matches secret `THEOPACKS_VARS`)—use word-boundary check: only match when `$NAME` is followed by non-word char or end-of-string.
- Multi-line commands: scan whole body.

#### Tasks
1. Change `core/plan/step.go` default to `Secrets: []string{}`.
2. Replace `resolveSecrets` with `autoDetectSecrets` in `generate.go`; preserve `["*"]` wildcard semantics for explicit opt-in.
3. Update `core/plan/step_test.go` to assert the new default.
4. Add table-driven tests for `autoDetectSecrets` covering: no match, exact match, ${} braces form, partial-token rejection, multiple matches.
5. Regenerate goldens (`mise run test-update-snapshots`); the diff will show secret mounts disappearing from RUNs that don't reference the secret.

#### TDD
```
RED:    TestStep_DefaultSecrets_IsEmpty
RED:    TestAutoDetectSecrets_NoReferences         — RUN "go mod download", planSecrets=[FOO] → []
RED:    TestAutoDetectSecrets_DollarReference      — RUN "echo $FOO" → [FOO]
RED:    TestAutoDetectSecrets_BraceReference       — RUN "echo ${FOO}" → [FOO]
RED:    TestAutoDetectSecrets_TokenBoundary        — RUN "echo $FOOBAR", planSecrets=[FOO] → []  (no spurious match)
RED:    TestAutoDetectSecrets_Multiple             — RUN "$FOO ${BAR}" → [BAR, FOO]
RED:    TestAutoDetectSecrets_WildcardEscape       — Secrets=["*"] still mounts all
RED:    TestRenderer_BogusMountGone                — pip install dockerfile no longer has --mount for THEOPACKS_START_CMD
GREEN:  Implement defaults + autoDetectSecrets
REFACTOR: None expected.
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] `Step.Secrets` defaults to `[]`.
- [ ] `autoDetectSecrets` handles all 7 cases above.
- [ ] Goldens regenerated; `git diff` shows mount lines disappearing only from commands that don't reference the secret.
- [ ] `mise run check` clean.

#### DoD
- [ ] All tasks complete.
- [ ] All goldens reviewed and committed.
- [ ] No spurious mounts in generated Dockerfiles.

---

### T0.3 — Add `BuildKitCache` field to `plan.Step` for typed cache mounts

#### Objective
Provide a first-class data model for BuildKit cache mounts so providers can declare them ergonomically, and the renderer can emit `--mount=type=cache,target=...,sharing=locked`.

#### Evidence
Finding #5: zero cache mounts in current Dockerfiles. The plan model has a `Caches` map on `BuildPlan` but it's not wired through to RUN-level mounts.

#### Files to edit
```
core/plan/step.go              — ADD BuildKitCaches []BuildKitCacheMount field
core/plan/cache.go             — (NEW) BuildKitCacheMount struct {Target string; Sharing string}
core/generate/command_step_builder.go — ADD AddCacheMount(target, sharing) method
core/dockerfile/generate.go    — emit --mount=type=cache for each entry on RUN
core/plan/cache_test.go        — (NEW) unit tests for BuildKitCacheMount
core/dockerfile/generate_test.go — renderer test for cache mount emission
```

#### Deep file dependency analysis
- **`core/plan/cache.go`** (NEW) — defines `BuildKitCacheMount`. Pure data type, no dependencies.
- **`core/plan/step.go`** — adds the `BuildKitCaches` slice. Existing serialization round-trip tests keep passing because the field is `,omitempty`.
- **`core/generate/command_step_builder.go`** — adds the helper that providers call. Wraps the verbose struct construction.
- **`core/dockerfile/generate.go`** — when emitting a RUN, prepend `--mount=type=cache,target=<t>,sharing=<s>` for each declared cache (alongside any secret mounts from T0.2).

#### Deep Dives

**Type:**
```go
type BuildKitCacheMount struct {
    Target  string `json:"target"`            // /root/.cache/go-build
    Sharing string `json:"sharing,omitempty"` // "locked" | "shared" | "private"; default "locked"
}
```

**Builder API:**
```go
step.AddCacheMount("/root/.cache/go-build", "")  // sharing defaults to "locked"
step.AddCacheMount("/go/pkg/mod", "shared")
```

**Renderer output:**
```dockerfile
RUN --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    --mount=type=cache,target=/go/pkg/mod,sharing=shared \
    go mod download
```

**Invariants:**
- Mount targets are sorted lexicographically for stable Dockerfile output.
- Empty `Sharing` defaults to `"locked"` in the renderer.

**Edge cases:**
- Duplicate target paths in the same step: dedupe at builder level (idempotent `AddCacheMount`).
- A cache mount declared on a non-existent step: builder no-ops cleanly.

#### Tasks
1. Create `core/plan/cache.go` with `BuildKitCacheMount`.
2. Add field to `Step`; update `NewStep`.
3. Add `AddCacheMount` to `CommandStepBuilder`.
4. Update renderer to emit mounts (sorted by target, deduped).
5. Add tests at all three layers.

#### TDD
```
RED:    TestBuildKitCacheMount_DefaultsToLocked
RED:    TestStep_NewStepHasNoBuildKitCaches
RED:    TestStepBuilder_AddCacheMountIdempotent
RED:    TestRenderer_EmitsCacheMount               — produces --mount=type=cache,target=/X,sharing=locked
RED:    TestRenderer_MultipleCachesSorted          — by target asc
GREEN:  Implement
REFACTOR: None.
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] `BuildKitCacheMount` and `Step.BuildKitCaches` exist.
- [ ] Builder helper works; idempotent.
- [ ] Renderer emits sorted, deduped mounts.
- [ ] No provider migrated yet — Phase 3 wires them.
- [ ] All tests green; `mise run check` clean.

#### DoD
- [ ] T0.3 complete.

---

## Phase 1: Critical correctness fixes

**Objective:** Ship the three correctness bugs (CMD bash wrap, Java double-sh-c, Ruby bundle-config quoting) so generated Dockerfiles boot on slim/distroless images and don't have shell quoting bugs.

### T1.1 — Replace `CMD ["/bin/bash", "-c", ...]` with exec or `/bin/sh` form

#### Objective
Stop emitting `CMD ["/bin/bash", ...]`. Use exec form when the start command is a single program, fall back to `/bin/sh -c` only when shell features are needed.

#### Evidence
Finding #1. `core/dockerfile/generate.go:99`: `fmt.Fprintf(b, "CMD [\"/bin/bash\", \"-c\", %q]\n", deploy.StartCmd)`. Tested manually against `debian:bookworm-slim` — bash absent, container fails.

#### Files to edit
```
core/dockerfile/generate.go     — change CMD emission to exec form when possible, /bin/sh -c otherwise
core/dockerfile/generate_test.go — coverage for both forms + edge cases
core/generate/deploy_builder.go — ADD optional StartCmdNeedsShell flag (default false)
```

#### Deep file dependency analysis
- **`core/dockerfile/generate.go`** — line 99 is the single emission point. Replace with a function `emitCMD(b, startCmd, needsShell)` that:
  1. If `needsShell` → `CMD ["/bin/sh", "-c", "<cmd>"]`
  2. Else if cmd is shell-special-free (no `$`, `;`, `&&`, `||`, `|`, `>`) → split by whitespace, emit `CMD ["arg0", "arg1", ...]`
  3. Else → fall back to `CMD ["/bin/sh", "-c", "<cmd>"]` (still slim-safe).
- **`core/generate/deploy_builder.go`** — add `StartCmdNeedsShell bool`. Providers that emit a start command with `${PORT:-...}` or pipe sequences set this to `true` (existing PHP/Ruby/Deno commands need it).

#### Deep Dives

**Tokenizer for exec form:**
- Use `strings.Fields(cmd)` to split.
- Reject if any field contains shell-meta: `[$\\\";\`&|<>(){}]`.
- This is conservative — false negatives (commands that look shell-special but aren't) fall back to shell form, which is still safe.

**Examples (before → after):**
- `/app/server` → `CMD ["/app/server"]` (exec form ✓)
- `npm start` → `CMD ["npm", "start"]` (exec form ✓)
- `dotnet /app/MyApp.dll` → `CMD ["dotnet", "/app/MyApp.dll"]` (exec form ✓)
- `bundle exec rails server -b 0.0.0.0 -p ${PORT:-3000}` → `CMD ["/bin/sh", "-c", "bundle exec rails server -b 0.0.0.0 -p ${PORT:-3000}"]` (env-expansion ⇒ shell form, but `/bin/sh` not bash)
- `php -S 0.0.0.0:${PORT:-8000} -t public` → same as above.
- `deno task start` → `CMD ["deno", "task", "start"]` (exec form, but Deno's distroless image is fine — task is interpreted by deno itself).

**Invariants:**
- Generated CMD must work on `debian:bookworm-slim` (no bash assumed).
- Generated CMD with shell features must use `/bin/sh`, not `/bin/bash`.
- Distroless images: only exec form will work; the framework's job is to ensure shell-feature commands aren't used with distroless runtimes — Phase 4 enforces this for Go/Rust.

**Edge cases:**
- Empty StartCmd → renderer panics with clear message ("provider X did not set Deploy.StartCmd").
- StartCmd with only whitespace → same.

#### Tasks
1. Add `StartCmdNeedsShell` field to `DeployBuilder`.
2. Add `emitCMD()` helper to `generate.go`.
3. Wire `Deploy.StartCmdNeedsShell` from provider where shell features are used.
4. Update renderer to call `emitCMD()`.
5. Update each affected provider (Ruby, PHP, Deno, Java, Python, Node) to set `NeedsShell` when their start command uses shell features.

#### TDD
```
RED:    TestEmitCMD_PlainExec_NoSpecials       — "/app/server" → CMD ["/app/server"]
RED:    TestEmitCMD_MultiWordExec              — "npm start" → CMD ["npm","start"]
RED:    TestEmitCMD_ShellFeatures_FallsBack    — "echo $PORT | xargs ..." → CMD ["/bin/sh","-c","..."]
RED:    TestEmitCMD_NeedsShellOverride         — NeedsShell=true forces /bin/sh form regardless
RED:    TestEmitCMD_EmptyPanics                — empty StartCmd → panic with provider name
RED:    TestEmitCMD_NeverEmitsBinBash          — golden grep for /bin/bash returns empty
GREEN:  Implement emitCMD + provider migrations
REFACTOR: None expected.
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] All 6 unit tests pass.
- [ ] `grep -r "/bin/bash" core/dockerfile/testdata/` returns empty.
- [ ] All 50+ goldens regenerate; the only changed line per file is the CMD line.
- [ ] `mise run check` clean.

#### DoD
- [ ] T1.1 complete.
- [ ] CMD-line diff in goldens reviewed manually.

---

### T1.2 — Fix Java double-`sh -c` JAR-extract bug

#### Objective
Strip the inner `sh -c` from the Gradle and Maven JAR-extraction commands.

#### Evidence
Finding #2. `core/providers/java/gradle.go:line` and `maven.go:line` build a command like:
```go
plan.NewExecShellCommand(
    "sh -c 'set -e; jar=$(ls build/libs/*.jar | grep -v -- ...) cp \"$jar\" /app/app.jar'",
)
```
`NewExecShellCommand` already wraps in `sh -c`, producing `RUN sh -c 'sh -c '...''`. Same root cause as the PHP/Ruby fix in `72239e9`.

#### Files to edit
```
core/providers/java/gradle.go  — strip inner sh -c '...' wrap
core/providers/java/maven.go   — same
core/providers/java/{gradle,maven}_test.go — adjust expected substring assertions if any
core/dockerfile/testdata/integration_java_*.dockerfile — regenerate goldens
```

#### Deep file dependency analysis
- **`gradle.go` / `maven.go`** — single-line edit per file. Strips outer `sh -c '...'` from the body string passed to `NewExecShellCommand`. The actual semicolon-separated sequence (which DOES need shell features for `$(...)`, `|`, `;`) keeps `NewExecShellCommand`.

#### Deep Dives

**Before:**
```go
buildStep.AddCommand(plan.NewExecShellCommand(
    "sh -c 'set -e; jar=$(ls build/libs/*.jar | grep -v -- \"-plain\\.jar$\" | head -n1); cp \"$jar\" /app/app.jar'",
))
```

**After:**
```go
buildStep.AddCommand(plan.NewExecShellCommand(
    "set -e; jar=$(ls build/libs/*.jar | grep -v -- \"-plain\\.jar$\" | head -n1); cp \"$jar\" /app/app.jar",
))
```

**Invariants:**
- The shell semantics are unchanged — `NewExecShellCommand` produces the single `sh -c '...'` wrapper.
- `set -e` still aborts on first error.

#### Tasks
1. Edit `gradle.go` to remove the inner `sh -c '...'`.
2. Edit `maven.go` to remove same.
3. Run goldens; verify diff shows only the double-wrap collapsing.

#### TDD
```
RED:    TestPlanGradle_JarCopyNoDoubleShCLayer  — generated Dockerfile has `RUN sh -c 'set -e; ...'` (one wrap)
RED:    TestPlanMaven_JarCopyNoDoubleShCLayer   — same
RED:    TestGoldensFreeOfDoubleShC              — grep "sh -c 'sh -c '" across all goldens returns empty
GREEN:  Strip inner sh -c
REFACTOR: None.
VERIFY: mise run test ./core/providers/java/... && UPDATE_GOLDEN=true go test ./core/dockerfile/...
```

#### Acceptance Criteria
- [ ] Two unit tests + one corpus-grep test pass.
- [ ] `grep "sh -c 'sh -c '" core/dockerfile/testdata/` returns empty.
- [ ] Java integration goldens regenerate cleanly.

#### DoD
- [ ] T1.2 complete.

---

### T1.3 — Fix Ruby `bundle config without` single-quote-in-single-quote

#### Objective
Replace `bundle config set --local without 'development test'` with a quoting form that survives `sh -c` wrapping.

#### Evidence
Finding #11. `core/providers/ruby/ruby.go:line` calls:
```go
installStep.AddCommand(plan.NewExecShellCommand("bundle config set --local without 'development test'"))
```
After `NewExecShellCommand` wraps in `sh -c '...'`, the single quotes inside collide:
```
sh -c 'bundle config set --local without 'development test''
```
The shell parser closes the outer `'` at `without '`, then concatenates `development test` as a separate word, then re-opens `'`. Bundler tolerates the noise so it works by accident.

#### Files to edit
```
core/providers/ruby/ruby.go — use double quotes inside, or pass the values as separate args
core/providers/ruby/ruby_test.go — assert the new shape
core/dockerfile/testdata/integration_ruby_*.dockerfile — regenerate goldens
```

#### Deep Dives

**Option A (chosen): use the multi-arg form (no quoting needed):**
```go
installStep.AddCommand(plan.NewExecShellCommand("bundle config set --local without development:test"))
```
Bundler accepts colon-separated groups — same semantics, no quotes.

**Option B (rejected): escape single quotes (`'\''`).** Works but is harder to read.

**Invariants:**
- Bundler installs without `development` and `test` groups.
- No single-quote-in-single-quote anywhere.

#### Tasks
1. Edit `ruby.go` (planSimple + planWorkspace) to use `development:test` form.
2. Update tests if any check the literal command.
3. Regenerate goldens.

#### TDD
```
RED:    TestPlanRuby_BundleConfigNoNestedQuotes — generated command does NOT contain `'development test'`
RED:    TestGoldenRubySinatra_NoQuoteCollision — grep for the broken pattern returns empty
GREEN:  Change command form
REFACTOR: None.
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] Both ruby providers updated.
- [ ] All ruby goldens regenerate cleanly.
- [ ] No single-quote-in-single-quote in any golden.

#### DoD
- [ ] T1.3 complete.

---

## Phase 2: Smart secret mounting

**Objective:** Apply T0.2's substring detection to all generated Dockerfiles; verify that bogus mounts disappear without losing real ones.

### T2.1 — Verify and lock smart secret mount behavior

#### Objective
Re-render all goldens after T0.2 lands; manually inspect the `--mount=type=secret` lines that disappear; ensure none of them were actually used.

#### Files to edit
```
core/dockerfile/testdata/*.dockerfile — regenerate (this is the visible side effect)
core/dockerfile/integration_test.go    — keep env vars as today (they're testing real env-driven behavior)
```

#### Deep Dives
This is largely a verification task. T0.2 did the renderer change; T2.1 confirms that no provider relied on the bogus mount semantics.

#### Tasks
1. `mise run test-update-snapshots`.
2. Diff goldens; for each removed `--mount=type=secret` line, verify the corresponding RUN body does NOT reference the secret.
3. Commit golden diffs as a single commit.

#### TDD
```
RED:    TestNoBogusSecretMounts — for each generated dockerfile, every --mount=type=secret line corresponds to a $ or ${} reference in the same RUN body
GREEN:  Already implemented in T0.2; T2.1 just regenerates and verifies.
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] Audit test passes for all goldens.
- [ ] Removed mount lines manually reviewed; no false removals.

#### DoD
- [ ] T2.1 complete.

---

## Phase 3: Cache mounts per language

**Objective:** Wire `BuildKitCache` (T0.3) into every provider's install/build steps so cold rebuilds reuse package-manager caches.

### T3.1 — Go provider cache mounts

#### Objective
Add cache mounts for `~/.cache/go-build` and `/go/pkg/mod` to Go install + build steps.

#### Files to edit
```
core/providers/golang/golang.go      — add AddCacheMount calls
core/providers/golang/golang_test.go — assert mounts present
core/dockerfile/testdata/integration_go_*.dockerfile — regenerate
```

#### Deep Dives

**Mount targets:**
- `/root/.cache/go-build` — Go's compiled-package cache.
- `/go/pkg/mod` — Go module download cache.

Both are sharing-safe (`locked`).

**Tasks:**
1. In `planSimple` and `planWorkspace`, on `installStep`: `AddCacheMount("/go/pkg/mod", "")` and `AddCacheMount("/root/.cache/go-build", "")`.
2. On `buildStep`: same two mounts (build also benefits).

#### TDD
```
RED:    TestPlanGo_InstallStepHasGoPkgModCache
RED:    TestPlanGo_BuildStepHasGoBuildCache
RED:    TestGoldenGoSimple_HasCacheMounts        — Dockerfile contains --mount=type=cache,target=/go/pkg/mod
GREEN:  Add mounts
REFACTOR: None.
VERIFY: mise run test ./core/providers/golang/...
```

#### Acceptance Criteria
- [ ] Both Go cache mounts present in every Go integration golden.
- [ ] Tests green.

#### DoD
- [ ] T3.1 complete.

---

### T3.2 — Rust provider cache mounts

Same pattern as T3.1; targets:
- `/root/.cargo/registry`
- `/root/.cargo/git`
- `/app/target` (build cache; only on build step)

#### Files
```
core/providers/rust/rust.go      — AddCacheMount on install (registry, git) and build (registry, git, target)
core/providers/rust/rust_test.go — assert mounts
core/dockerfile/testdata/integration_rust_*.dockerfile — regenerate
```

#### TDD
```
RED:    TestPlanRust_InstallHasRegistryAndGitCache
RED:    TestPlanRust_BuildHasTargetCache
RED:    TestGoldenRustAxum_HasCacheMounts
GREEN:  Add mounts
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] All Rust goldens have the 3 expected mounts.
- [ ] Tests green.

#### DoD
- [ ] T3.2 complete.

---

### T3.3 — Java provider cache mounts (Gradle + Maven)

#### Files
```
core/providers/java/gradle.go    — AddCacheMount("/root/.gradle")
core/providers/java/maven.go     — AddCacheMount("/root/.m2")
core/providers/java/{gradle,maven}_test.go
core/dockerfile/testdata/integration_java_*.dockerfile
```

#### TDD
```
RED:    TestPlanGradle_HasGradleCache
RED:    TestPlanMaven_HasM2Cache
GREEN:  Add mounts
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] Mounts present in Java goldens.

#### DoD
- [ ] T3.3 complete.

---

### T3.4 — .NET provider cache mounts

#### Files
```
core/providers/dotnet/dotnet.go      — AddCacheMount("/root/.nuget/packages")
core/providers/dotnet/dotnet_test.go
core/dockerfile/testdata/integration_dotnet_*.dockerfile
```

#### TDD
```
RED:    TestPlanDotnet_HasNugetCache
GREEN:  Add mount
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] NuGet cache mount in .NET goldens.

#### DoD
- [ ] T3.4 complete.

---

### T3.5 — Ruby provider cache mounts

#### Files
```
core/providers/ruby/ruby.go      — AddCacheMount("/usr/local/bundle")
core/providers/ruby/ruby_test.go
core/dockerfile/testdata/integration_ruby_*.dockerfile
```

#### TDD
```
RED:    TestPlanRuby_HasBundleCache
GREEN:  Add mount
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] Bundler cache mount in Ruby goldens.

#### DoD
- [ ] T3.5 complete.

---

### T3.6 — PHP provider cache mounts

#### Files
```
core/providers/php/php.go        — AddCacheMount("/root/.composer/cache") + apt-get cache mount on install
core/providers/php/php_test.go
core/dockerfile/testdata/integration_php_*.dockerfile
```

#### TDD
```
RED:    TestPlanPhp_HasComposerCache
RED:    TestPlanPhp_HasAptCache
GREEN:  Add mounts
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] Composer + apt cache mounts in PHP goldens.

#### DoD
- [ ] T3.6 complete.

---

### T3.7 — Node provider cache mounts

#### Files
```
core/providers/node/node.go      — AddCacheMount("/root/.npm") + per-PM mounts (~/.yarn, ~/.pnpm-store)
core/providers/node/node_test.go
core/dockerfile/testdata/integration_node_*.dockerfile
```

#### TDD
```
RED:    TestPlanNode_NpmHasCacheMount
RED:    TestPlanNode_YarnHasCacheMount
RED:    TestPlanNode_PnpmHasCacheMount
GREEN:  Add per-PM mounts
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] Per-PM cache mounts in Node goldens.

#### DoD
- [ ] T3.7 complete.

---

### T3.8 — Python provider cache mounts

#### Files
```
core/providers/python/python.go      — AddCacheMount("/root/.cache/pip") on every install variant
core/providers/python/python_test.go
core/dockerfile/testdata/integration_python_*.dockerfile
```

#### TDD
```
RED:    TestPlanPython_PipHasCacheMount
RED:    TestPlanPython_PoetryHasCacheMount
RED:    TestPlanPython_UvHasCacheMount
GREEN:  Add mounts
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] pip cache mount in Python goldens.

#### DoD
- [ ] T3.8 complete.

---

### T3.9 — Deno provider cache mounts

#### Files
```
core/providers/deno/deno.go      — AddCacheMount("/deno-dir")  (default DENO_DIR)
core/providers/deno/deno_test.go
core/dockerfile/testdata/integration_deno_*.dockerfile
```

#### TDD
```
RED:    TestPlanDeno_HasDenoDirCache
GREEN:  Add mount
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] DENO_DIR cache mount in Deno goldens.

#### DoD
- [ ] T3.9 complete.

---

## Phase 4: Slim/distroless runtime images

**Objective:** Switch Go and Rust runtime to distroless; add alpine variant for .NET console; document the trade-offs.

### T4.1 — Go runtime → `gcr.io/distroless/static-debian12:nonroot`

#### Objective
Drop Go runtime image size from ~80MB (debian-slim) to ~2MB (distroless static).

#### Files
```
core/generate/images.go           — change GoRuntimeImage constant
core/generate/images_test.go      — update expected value
core/dockerfile/testdata/integration_go_*.dockerfile — regenerate
```

#### Deep Dives
- Distroless static includes ca-certificates and timezone data; no shell, no apt.
- Go binaries built without CGO (default for `go build`) are fully static.
- The image runs as `nonroot` (UID 65532).

#### TDD
```
RED:    TestGoRuntimeImage_IsDistrolessStatic
RED:    TestGoldenGoSimple_RuntimeIsDistroless
GREEN:  Change constant
VERIFY: mise run test + e2e on Docker host
```

#### Acceptance Criteria
- [ ] Constant updated.
- [ ] Goldens use new image.
- [ ] E2E: `docker image inspect theopacks-e2e-go-simple:test --format '{{.Size}}'` returns < 10MB.

#### DoD
- [ ] T4.1 complete.

---

### T4.2 — Rust runtime → `gcr.io/distroless/cc-debian12:nonroot`

#### Objective
Drop Rust runtime size from ~80MB to ~17MB (distroless cc has glibc).

#### Files
```
core/generate/images.go           — change RustRuntimeImage constant
core/generate/images_test.go
core/dockerfile/testdata/integration_rust_*.dockerfile
```

#### Deep Dives
- Rust by default links dynamically against glibc; need `cc-debian12` (not `static-debian12`).
- For musl-built Rust binaries, the user can override via `theopacks.json` deploy.base. (Out of scope.)

#### TDD
```
RED:    TestRustRuntimeImage_IsDistrolessCc
RED:    TestGoldenRustAxum_RuntimeIsDistroless
GREEN:  Change constant
VERIFY: mise run test + e2e
```

#### Acceptance Criteria
- [ ] Constant updated.
- [ ] Rust goldens use new image.
- [ ] E2E: image size < 30MB.

#### DoD
- [ ] T4.2 complete.

---

### T4.3 — .NET console runtime → `aspnet:8.0-alpine`

#### Files
```
core/generate/images.go           — DotnetRuntimeImageForVersion now returns alpine variant
core/generate/images_test.go
core/dockerfile/testdata/integration_dotnet_console.dockerfile
```

#### Deep Dives
- ASP.NET Web stays on `aspnet:8.0` (alpine variant has had compatibility issues with some web stacks).
- Console / worker switches to alpine — saves ~80MB.

#### TDD
```
RED:    TestDotnetRuntimeImage_ConsoleIsAlpine
RED:    TestDotnetAspnetImage_RemainsBookworm
GREEN:  Update constants per project shape
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] Console projects use alpine; ASP.NET stays bookworm.

#### DoD
- [ ] T4.3 complete.

---

## Phase 5: USER + HEALTHCHECK + size flags

**Objective:** Production defaults: non-root user, healthchecks for HTTP frameworks, size-optimization flags for compiled languages.

### T5.1 — `USER` directive in deploy stage (non-distroless)

#### Files
```
core/generate/deploy_builder.go    — emit RUN useradd ... && USER appuser
core/dockerfile/generate.go        — call into deploy_builder for the directives
core/generate/deploy_builder_test.go
core/dockerfile/testdata/*.dockerfile — regenerate (USER lines added)
```

#### Deep Dives
- Distroless images already use `nonroot`; skip USER emission for those (detect by image name prefix `gcr.io/distroless/`).
- For non-distroless: insert `RUN useradd -r -u 1000 appuser` before the COPY directives, then `USER appuser` after COPY but before CMD.
- File ownership: COPY uses `--chown=appuser:appuser`.

#### TDD
```
RED:    TestDeploy_NonDistroless_HasUserDirective
RED:    TestDeploy_Distroless_NoUserDirective
RED:    TestDeploy_CopyHasChownFlag
GREEN:  Implement
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] All non-distroless deploys have USER appuser.
- [ ] All COPY in deploy use --chown.

#### DoD
- [ ] T5.1 complete.

---

### T5.2 — `HEALTHCHECK` for HTTP frameworks

#### Files
```
core/generate/deploy_builder.go    — ADD HealthCheck struct + emission
core/providers/{ruby,php,java,dotnet,deno,node,python}/*.go — set HealthCheck per detected framework
core/dockerfile/generate.go
*test* + golden regen
```

#### Deep Dives
- Default endpoint `/health`, port from framework default or `${PORT}` (interpolated by `/bin/sh -c`).
- Use `curl -fsS http://localhost:<port>/health || exit 1`.
- Skip when runtime image is distroless (no curl).

#### TDD
```
RED:    TestSpringBoot_HasHealthcheck
RED:    TestSinatra_HasHealthcheck
RED:    TestLaravel_HasHealthcheck
RED:    TestAspnet_HasHealthcheck
RED:    TestHono_HasHealthcheck
RED:    TestDistroless_NoHealthcheck
GREEN:  Implement per framework
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] HEALTHCHECK in HTTP-framework goldens.
- [ ] No HEALTHCHECK in non-HTTP / distroless goldens.

#### DoD
- [ ] T5.2 complete.

---

### T5.3 — Per-language size-optimization flags

#### Files
```
core/providers/golang/golang.go         — go build -ldflags="-s -w" -trimpath
core/providers/rust/rust.go             — inject [profile.release] strip = "symbols", lto = "thin"
core/providers/dotnet/dotnet.go         — dotnet publish -p:DebugType=None -p:DebugSymbols=false
*_test.go + goldens
```

#### TDD
```
RED:    TestGoBuild_HasLdflagsStripFlag
RED:    TestRustBuild_HasStripSymbols
RED:    TestDotnetPublish_NoDebugSymbols
GREEN:  Implement flags
VERIFY: mise run test + e2e (verify image sizes shrink)
```

#### Acceptance Criteria
- [ ] All three providers emit the correct flags.
- [ ] E2E: image size verified smaller.

#### DoD
- [ ] T5.3 complete.

---

## Phase 6: Layer cache micro-optimization

**Objective:** Split lockfile from manifest in COPY directives so a manifest-only change doesn't invalidate the lockfile cache layer.

### T6.1 — Split lock + manifest in install steps

#### Files
```
core/providers/{golang,node,rust,java,dotnet,ruby,php,python,deno}/*.go — split COPY commands
*test* + goldens
```

#### Deep Dives
Today's pattern:
```dockerfile
COPY package.json package-lock.json ./
```
After split:
```dockerfile
COPY package.json ./
COPY package-lock.json ./
```
Now editing `package.json` (e.g., bumping a version) doesn't bust the lockfile-COPY layer's cache. Most package managers re-read both anyway, but the BuildKit content-addressed store keys layer hits by file content — and the manifest file changes more often than the lockfile.

#### TDD
```
RED:    TestNodeInstall_LockManifestSplit
RED:    TestPythonInstall_RequirementsSplit
RED:    TestRustInstall_CargoTomlAndLockSplit
... per language
GREEN:  Split the COPY calls
VERIFY: mise run test
```

#### Acceptance Criteria
- [ ] All providers' install steps split lock + manifest.

#### DoD
- [ ] T6.1 complete.

---

## Phase 7: Final QA + documentation

**Objective:** Regenerate all goldens, run E2E on Docker host, update CHANGELOG / README / NOTICE, open PR.

### T7.1 — Final regeneration + E2E

#### Files
```
core/dockerfile/testdata/*.dockerfile  — final regenerate
e2e/e2e_test.go                        — verify all 19+ E2E tests pass on Docker host
```

#### Tasks
1. `mise run test-update-snapshots`.
2. Manually inspect 10 random goldens (one per language) end-to-end.
3. Trigger CI E2E workflow (or run locally with Docker).
4. Verify size + cold-build-time metrics.

#### Acceptance Criteria
- [ ] All goldens regenerate cleanly.
- [ ] E2E passes for all 19+ tests.
- [ ] Image sizes hit the targets in Objective.

---

### T7.2 — Documentation updates

#### Files
```
CHANGELOG.md   — Unreleased entry: "Fixed Dockerfile correctness (CMD form, Java/Ruby quoting); added BuildKit cache mounts; switched Go/Rust to distroless runtime; added USER + HEALTHCHECK; per-language size flags. Image sizes shrunk 30-95% for compiled languages; cold builds 40%+ faster."
README.md      — update Supported Languages table; mention distroless runtimes; document HEALTHCHECK + USER defaults
CLAUDE.md      — update CMD form note; document StartCmdNeedsShell + new BuildKit cache helper
NOTICE         — note divergence from upstream Railpack on CMD form / cache mounts
docs/plans/fix-dockerfile-efficiency-and-correctness-plan.md — mark Phase 7 complete
```

#### Acceptance Criteria
- [ ] All four files updated.
- [ ] CHANGELOG entry references this plan + ADR IDs.

---

### T7.3 — PR description + open

#### Tasks
1. Compose PR description summarizing all 11 findings + how each is resolved with ADR IDs.
2. Run `scripts/open-multi-language-pr.sh` (parameterized by env vars) to open the PR.

#### Acceptance Criteria
- [ ] PR opened against `develop` (since `main` is protected and the project flow is `feat → develop → main`).
- [ ] PR description includes the 11-issue audit table.
- [ ] CI E2E workflow runs on the PR.

#### DoD
- [ ] T7.3 complete.

---

## Coverage Matrix

| # | Gap / Requirement | Task(s) | Resolution |
|---|---|---|---|
| 1 | `CMD ["/bin/bash", "-c", ...]` breaks on slim/distroless | T1.1, T0.1, T4.1, T4.2 | Renderer emits exec form by default; falls back to `/bin/sh` (not bash) when shell features needed |
| 2 | Java JAR-extract has double `sh -c` quoting bug | T1.2 | Strip inner `sh -c` from `gradle.go` and `maven.go` |
| 3 | Ruby `bundle config without 'development test'` quote-in-quote | T1.3 | Use `development:test` colon-separated form |
| 4 | Universal `sh -c '...'` wrapping on every RUN | T0.1, T2.1 (provider migrations) | Add `CommandKindExec`; migrate plain commands to `NewExecCommand` |
| 5 | Zero BuildKit cache mounts | T0.3, T3.1-T3.9 | New `BuildKitCacheMount` data type + per-language wiring |
| 6 | Spurious `--mount=type=secret` on RUNs that don't reference secrets | T0.2, T2.1 | Default `Step.Secrets = []`; renderer auto-detects via substring |
| 7 | Runtime images larger than necessary (Go/Rust/.NET console) | T4.1, T4.2, T4.3 | Switch to distroless static / cc / aspnet alpine |
| 8 | No `USER` non-root in deploy | T5.1 | `RUN useradd ...` + `USER appuser` for non-distroless |
| 9 | Lock + manifest in same COPY layer | T6.1 | Split into separate COPY directives |
| 10 | No `HEALTHCHECK` for HTTP frameworks | T5.2 | Per-framework `HEALTHCHECK CMD curl -fsS http://localhost:<port>/health` |
| 11 | Missing per-language size flags | T5.3 | Go ldflags-s-w, Rust strip+lto, .NET no-debug-symbols |

**Coverage: 11/11 gaps covered (100%).**

## Global Definition of Done

- [ ] All 8 phases completed with their per-task DoDs.
- [ ] `mise run test` green (~150+ unit/integration tests).
- [ ] `mise run check` green (zero `go vet`, `gofmt`, `golangci-lint` warnings).
- [ ] `mise run test-e2e` green on a Docker host (19+ E2E tests).
- [ ] All ~50 goldens regenerated and reviewed.
- [ ] Pre-existing 5 providers (Go/Node/Python/Static/Shell) **still produce equivalent semantics** modulo the 11 fixes (verified by docker-running each through the E2E suite).
- [ ] Image-size targets hit (Go/Rust < 10/30MB, .NET console < 100MB).
- [ ] Cold-build wall-time targets hit (≥40% reduction vs current).
- [ ] No file in `core/` exceeds 600 lines (`generate.go` is the biggest; current 482 lines).
- [ ] `CHANGELOG.md` Unreleased section captures all changes; `README.md` Supported Languages updated; `CLAUDE.md` Provider-Detection-Order section unchanged but new sections on CMD form + cache mounts added; `NOTICE` notes divergence from upstream Railpack.
- [ ] PR opened against `develop` with the 11-issue audit table; CI E2E workflow runs and passes.
- [ ] Backward compatibility preserved: `theopacks.json`, `THEOPACKS_*` env vars, `Provider` interface, public CLI flags all behave identically.
- [ ] Plan file (this document) marked complete; promoted to `docs/plans/archive/` after merge.
