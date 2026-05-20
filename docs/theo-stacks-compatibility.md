# theo-stacks Compatibility Matrix

theo-packs is the Dockerfile generator for every template that
`create-theo` (theo-stacks) scaffolds. This document maps each upstream
template to (a) the theo-packs provider that handles it, (b) the
in-repo example used as the equivalent test fixture, and (c) the
current end-to-end test status.

Upstream: https://github.com/usetheodev/theo-stacks/tree/main/templates

Last sync: 2026-05-20

| Template | theo.yaml `framework` | theo-packs provider | Fixture (examples/) | E2E status |
|---|---|---|---|---|
| `fullstack-nextjs` | `nextjs` | `node` (next runtime) | `node-next` | ✅ Phase 1 build, structure-test |
| `go-api` | `custom` | `go` | `go-simple` | ✅ Builds, distroless runtime, healthchecked |
| `java-spring` | (spring boot) | `java` (Gradle) | `java-spring-gradle` | ✅ fat-JAR build + healthcheck |
| `monorepo-go` | `custom` (workspace) | `go` (go.work) | `go-workspaces` | ✅ Workspace build |
| `monorepo-java` | (gradle multi-module) | `java` | `java-gradle-workspace` | ✅ Workspace JAR |
| `monorepo-php` | (apps/+packages/) | `php` | `php-monorepo` | ✅ apps/api scoped |
| `monorepo-python` | (apps/+packages/) | `python` | `python-uv-workspace` (shape differs slightly) | ⚠️ `python-uv-workspace` uses uv layout; upstream uses generic `apps/`. Gating via `TestE2E_TheoStacksTemplates`. |
| `monorepo-ruby` | (apps/+packages/) | `ruby` | `ruby-monorepo` | ✅ apps/api scoped |
| `monorepo-rust` | (cargo workspace) | `rust` | `rust-workspace` | ✅ Workspace member build |
| `monorepo-turbo` | `nextjs` + `express` | `node` (turbo) | `node-turborepo` | ✅ `TestE2E_MonorepoTurboContract` end-to-end |
| `node-express` | `express` | `node` | `node-express` | ✅ Build + structure-test |
| `node-fastify` | (fastify) | `node` | `node-fastify` (NEW) | 🆕 Created in this cycle |
| `node-nestjs` | (nestjs) | `node` | `node-nestjs` (NEW) | 🆕 Created in this cycle |
| `node-nextjs` | `nextjs` | `node` | `node-next` | ✅ |
| `node-worker` | (worker — no HTTP) | `node` | `node-worker` (NEW) | 🆕 Created in this cycle, `justBuild` mode |
| `php-slim` | (slim) | `php` | `php-slim` | ✅ Build + structure-test |
| `python-fastapi` | `fastapi` | `python` | `python-fastapi` | ✅ Build + fastapi import check |
| `ruby-sinatra` | (sinatra) | `ruby` | `ruby-sinatra` | ✅ Build + bundle info |
| `rust-axum` | `axum` | `rust` | `rust-axum` | ✅ Distroless static binary |

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

## Known gaps

- `monorepo-python` uses a generic `apps/+packages/` layout the
  upstream uses; our `python-uv-workspace` fixture uses uv-specific
  layout. The gating test in `e2e/theo_stacks_test.go` ensures the
  upstream shape still builds — even though the fixture differs in
  detail. If a future drift breaks this, add `python-monorepo` as an
  in-repo example mirroring the upstream apps/+packages shape.
