# theo-stacks Compatibility Matrix

theo-packs is the Dockerfile generator for every template that
`create-theo` (theo-stacks) scaffolds. This document maps each upstream
template to (a) the theo-packs provider that handles it, (b) the
in-repo example used as the equivalent test fixture, and (c) the
current end-to-end test status.

Upstream: https://github.com/usetheodev/theo-stacks/tree/main/templates

Last sync: 2026-05-21

| Template | Provider | Build | Runtime |
|---|---|---|---|
| `fullstack-nextjs` | `node` (next) | ✅ | ✅ HTTP 200 / |
| `go-api` | `go` (distroless static) | ✅ | ✅ HTTP 200 /health |
| `java-spring` | `java` Gradle (Spring Boot) | ✅ | ✅ HTTP 200 /actuator/health |
| `monorepo-go` | `go` (go.work) | ✅ | ✅ HTTP 200 /health |
| `monorepo-java` | `java` Gradle multi-module | ✅ | ✅ HTTP 200 /actuator/health |
| `monorepo-php` | `php` (composer + apps/+packages) | ✅ | ✅ HTTP 200 /health |
| `monorepo-python` | `python` (uv workspace) | ✅ | ✅ HTTP 200 /health |
| `monorepo-ruby` | `ruby` (Bundler apps/+packages) | ✅ | ✅ HTTP 200 /health |
| `monorepo-rust` | `rust` (Cargo workspace) | ✅ | ✅ HTTP 200 /health |
| `monorepo-turbo` | `node` (turbo + npm workspaces) | ✅ | ✅ HTTP 200 /health |
| `node-express` | `node` | ✅ | ✅ HTTP 200 /health |
| `node-fastify` | `node` | ✅ | ✅ HTTP 200 /health |
| `node-nestjs` | `node` | ✅ | ✅ HTTP 200 /health |
| `node-nextjs` | `node` (next) | ✅ | ✅ HTTP 200 / |
| `node-worker` | `node` (no HTTP) | ✅ | ✅ process alive |
| `php-slim` | `php` (Slim) | ✅ | ✅ HTTP 200 /health |
| `python-fastapi` | `python` (FastAPI) | ✅ | ✅ HTTP 200 /health |
| `ruby-sinatra` | `ruby` (Sinatra) | ✅ | ✅ HTTP 200 /health |
| `rust-axum` | `rust` (Axum, distroless static) | ✅ | ✅ HTTP 200 /health |

**Verified: 19/19 build + run end-to-end on 2026-05-21.**

## Gates

Two layered gates run in CI:

1. **`TestE2E_TheoStacksTemplates`** — provider gate. Renders each
   template, runs `theopacks-generate`, asserts the generated
   Dockerfile carries the expected provider header and passes
   `hadolint`. Fast (~3s); runs in L1/L2.

2. **`TestE2E_TheoStacksTemplates_Runtime`** — runtime gate
   (`docs/plans/theo-stacks-build-and-run-plan.md`). Adds
   `docker build` + `docker run` + HTTP healthcheck (server/frontend)
   or process-alive check (worker). Reads `theo.yaml::apps.<name>.port`
   and `type` as the source of truth. Slow (~4 min);
   runs in L3 nightly + can be opt-in per-PR.

## Gating

`TestE2E_TheoStacksTemplates` (`e2e/theo_stacks_test.go`) ingests the
upstream templates directly (via local clone at `/tmp/theo-stacks`,
falls back to `t.Skip` when absent) and asserts theo-packs can produce
a Dockerfile for each. This gate runs in CI's L3 nightly + on tag pushes
and catches drift when:

- An upstream template adds a new file pattern that no provider detects.
- A provider regression silently fails to detect a known shape.
- A new template is added upstream without being mapped here.

## Process for new templates

When `theo-stacks/templates/<new-name>` is added upstream:

1. Identify the closest existing provider; if none fits, add a new one
   in `core/providers/<lang>/`.
2. Create an in-repo example at `examples/<new-name>` mirroring the
   template's signal files (manifest, lockfile, entry point).
3. Add a row to the matrix above.
4. Add an entry to `e2eCases` in `e2e/e2e_table_test.go` so the E2E
   suite exercises it.
5. Optionally add `structure-tests.yaml` for declarative invariants.

## Verified end-to-end (real `docker build`)

The matrix above is gated by `TestE2E_TheoStacksTemplates` which runs
`theopacks-generate` against each template and inspects the produced
Dockerfile. A separate manual verification cycle on 2026-05-20 built
each template with real Docker and exercised the resulting image:

- **19/19 templates build successfully end-to-end.** ✅

Issues fixed in this cycle to reach 18/19:
- Generic monorepo workspace redirect now covers Go (`go.work`), Java
  Gradle (`settings.gradle*`), Rust (`Cargo.toml [workspace]`), Python
  uv (`[tool.uv.workspace]`) in addition to the previous Node-only
  redirect.
- Gradle subproject regex now accepts both `include(":a:b")` (leading
  colon) and `include("a:b")` (no leading colon — theo-stacks's
  monorepo-java form).
- PHP workspace install step copies `packages/` before `composer
  install` so path repositories resolve.
- Node workspace mode resolves the user-supplied app alias (e.g.
  `api`) to the canonical `package.json#name` (e.g. `@demo/api`) so
  `turbo --filter` and `pnpm --filter` succeed.

## Known gaps

- `monorepo-python` uses a generic `apps/+packages/` layout the
  upstream uses; our `python-uv-workspace` fixture uses uv-specific
  layout. The gating test in `e2e/theo_stacks_test.go` ensures the
  upstream shape still builds — even though the fixture differs in
  detail. If a future drift breaks this, add `python-monorepo` as an
  in-repo example mirroring the upstream apps/+packages shape.
