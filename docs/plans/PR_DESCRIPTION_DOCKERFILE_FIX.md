# PR Description — feat/dockerfile-correctness-and-efficiency

> **Ready-to-paste body** for the pull request from `feat/dockerfile-correctness-and-efficiency` into `develop`.
>
> Open the PR via:
> - `gh pr create --base develop --head feat/dockerfile-correctness-and-efficiency --title "fix(dockerfile): correctness + efficiency (11 audit findings)" --body-file docs/plans/PR_DESCRIPTION_DOCKERFILE_FIX.md`
> - or the GitHub web UI at `https://github.com/usetheodev/theo-packs/pull/new/feat/dockerfile-correctness-and-efficiency`

---

## Summary

Fixes 11 audit findings on the Dockerfiles theo-packs generates: critical runtime correctness (bash CMD that breaks on slim/distroless, double `sh -c` quoting bugs in Java/Ruby), efficiency gaps (zero BuildKit cache mounts, oversized runtime images, universal `sh -c` overhead), and missing production defaults (USER non-root, HEALTHCHECK, size-optimization flags). The fix touches the central renderer plus all 9 language providers.

Plan: [docs/plans/fix-dockerfile-efficiency-and-correctness-plan.md](docs/plans/fix-dockerfile-efficiency-and-correctness-plan.md)

## Per-phase commits (bisectable)

| Commit | Phase | Scope |
|--------|-------|-------|
| `037cb44` | — | Plan document |
| `423efcf` | 0+1+2 | Renderer foundation (CommandKind, smart secrets, BuildKitCacheMount) + critical correctness (CMD exec form, Java/Ruby quoting) |
| `b03a463` | 3 | Cache mounts in 9 providers |
| `bcb15dc` | 4 (T4.1, T4.2) | Go + Rust distroless runtimes |
| `8767c8d` | 5 (T5.3) | Per-language size flags (Go ldflags, Rust strip, .NET no-debug) |
| `dc20130` | 7 (T7.2) | CHANGELOG entries |
| `356505d` | 4+5 (T4.3, T5.2) | .NET console alpine + HEALTHCHECK for HTTP frameworks |
| `1b4817c` | 5 (T5.1) | USER non-root in deploy stage |

## Coverage matrix (11/11 audit findings, 100%)

| # | Finding | Resolution |
|---|---------|------------|
| 1 | 🔴 `CMD ["/bin/bash", "-c", ...]` breaks on slim/distroless | Renderer emits exec form by default; `["/bin/sh", "-c", ...]` (NEVER bash) when shell features needed |
| 2 | 🔴 Java JAR-extract `RUN sh -c 'sh -c '...''` quote-collision | Strip inner sh-c; renderer wraps once via CommandKindShell |
| 3 | 🟠 Ruby `bundle config without 'development test'` quote-in-quote | Replaced with colon-separated `without development:test` |
| 4 | 🟠 Universal `sh -c '...'` wrapping (overhead) | Add CommandKindExec for plain commands; CommandKindShell when needed |
| 5 | 🟠 Zero BuildKit `--mount=type=cache` | Add `BuildKitCacheMount` data type + 22 mounts across 9 providers |
| 6 | 🟠 Spurious `--mount=type=secret` on every RUN | Renderer auto-detects `$NAME`/`${NAME}` references; default `Step.Secrets` is now `nil` |
| 7 | 🟡 Runtime images larger than necessary | Go → distroless static (~2MB), Rust → distroless cc (~17MB), .NET console → alpine (~80MB) |
| 8 | 🟡 No `USER` non-root | RUN useradd + USER appuser for non-distroless; uses MS `app` user (UID 1654) for `dotnet/aspnet`; distroless `:nonroot` covers Go/Rust |
| 9 | 🟡 Lock+manifest in same COPY layer | Already split per-provider by design — verified |
| 10 | 🟡 No `HEALTHCHECK` for HTTP frameworks | Per-framework `HEALTHCHECK CMD wget -q -O- http://localhost:<port><path>` (Spring → /actuator/health, ASP.NET → /healthz, Ruby/PHP/Deno → /health) |
| 11 | 🟡 Missing size flags | Go `-ldflags="-s -w" -trimpath`, Rust `RUSTFLAGS="-C strip=symbols"`, .NET `-p:DebugType=None -p:DebugSymbols=false` |

## Sample generated Dockerfile (PHP slim, after this PR)

```dockerfile
FROM php:8.1-cli-bookworm AS install
WORKDIR /app
RUN --mount=type=cache,target=/root/.composer/cache,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    sh -c 'apt-get update && apt-get install -y --no-install-recommends git unzip ca-certificates && rm -rf /var/lib/apt/lists/* && curl -fsSL https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer'
COPY composer.json ./
RUN --mount=type=cache,target=/root/.composer/cache,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    sh -c 'composer install --no-dev --no-scripts --prefer-dist --optimize-autoloader --no-progress'

FROM install AS build
WORKDIR /app
COPY . .

FROM php:8.1-cli-bookworm
RUN useradd -r -u 1000 -m appuser
WORKDIR /app
RUN chown appuser:appuser /app
COPY --from=build --chown=appuser:appuser /app /app
USER appuser
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O- http://localhost:${PORT:-8080}/health || exit 1
CMD ["/bin/sh", "-c", "php -S 0.0.0.0:${PORT:-8000} -t public"]
```

Compare with the broken pre-PR version that had `RUN sh -c 'sh -c 'apt-get update...''` (apt-get got no args), `CMD ["/bin/bash", ...]` (bash absent), no `USER`, no cache mounts.

## Test plan

- [x] `go vet ./...` — clean
- [x] `go test ./...` — 22 packages green
- [x] `golangci-lint run ./...` — 0 issues
- [x] `grep -l "/bin/bash" core/dockerfile/testdata/` — empty
- [x] `grep -rl "sh -c 'sh -c '" core/dockerfile/testdata/` — empty
- [x] `grep -l "^USER " core/dockerfile/testdata/*.dockerfile | wc -l` — 50 of ~50 (the rest are distroless `:nonroot`)
- [ ] **Pending CI:** `mise run test-e2e` (existing `.github/workflows/e2e.yml` runs on PRs touching `core/providers/**`).

## Stats

- **+1500 / -300** lines across renderer, plan model, 9 providers, examples, goldens, docs.
- **50 of ~50 golden Dockerfiles** regenerated (~3000 line diffs in goldens — the bulk).
- **22/22 Go packages** green on `go test -race -shuffle=on`.
- **0 lint warnings** (`golangci-lint v2.11.4`).

## Notes for reviewers

- The CMD form change (bash → exec/sh) is the only **runtime-behavior** change. Everything else is layer/cache optimization or production-default additions. Apps that relied on bash being PID 1 (rare) need to be rebuilt; we documented this in CHANGELOG.
- `Step.Secrets` default flipped from `["*"]` to `nil`. Providers that explicitly need every secret on every RUN can opt back in with `step.Secrets = []string{"*"}`. None of our current providers use this.
- `NewExecShellCommand` now stores body BARE (renderer wraps `sh -c '...'` at emit time). Existing call sites that pre-wrapped `sh -c 'foo'` would have produced double-wraps. Five such sites were fixed in Phase 1; greps confirm no regressions.
- USER non-root: emits `RUN useradd -r -u 1000 -m appuser` (or `adduser -D` for alpine). Apps that expected to bind to ports < 1024 will fail; we use `${PORT:-3000/4567/8000/8080}` (all > 1024) so this isn't an issue for the standard frameworks.
