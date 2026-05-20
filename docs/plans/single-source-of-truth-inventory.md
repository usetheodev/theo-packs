# Single Source of Truth — File Inventory

> Frozen file list for `docs/plans/single-source-of-truth-plan.md`. Use this to verify completeness; if a file is touched outside this list, the plan is being violated.

## theo-packs files modified (PR-A)

| File | Phase | Change |
|---|---|---|
| `cmd/theopacks-generate/main.go` | TA1.1 | Replace user-Dockerfile precedence block (lines 60-73) with hard-fail block (`os.Exit(2)`) |
| `cmd/theopacks-generate/main_test.go` | TA3.1 | Delete `TestUserProvidedDockerfileTakesPrecedence` (lines 143-174); add `TestUserProvidedDockerfileIsRejected` and `TestUserProvidedDockerfileAtWorkspaceRoot_IsNotRejected` |
| `docs/contracts/theo-packs-cli-contract.md` | TA2.1 | Delete "User-Dockerfile precedence" section; add "Single source of truth" preamble; add failure-modes row for exit code 2; update product-must-pass and versioning sections |
| `CHANGELOG.md` | TA3.2 | Add `### Changed (BREAKING)` and `### Removed` entries under `[Unreleased]` |
| `docs/plans/single-source-of-truth-plan.md` | T0.0 | Plan itself (already on this branch) |
| `docs/plans/single-source-of-truth-inventory.md` | T0.1 | This file |
| `docs/plans/PR_DESCRIPTION_SINGLE_SOURCE_THEOPACKS.md` | TA4.1 | PR-A body |

## theo-stacks files deleted (PR-B)

26 files, all paths relative to the theo-stacks repo root.

| Template | Path |
|---|---|
| fullstack-nextjs | `templates/fullstack-nextjs/Dockerfile` |
| go-api | `templates/go-api/Dockerfile` |
| java-spring | `templates/java-spring/Dockerfile` |
| monorepo-go | `templates/monorepo-go/apps/api/Dockerfile` |
| monorepo-go | `templates/monorepo-go/apps/worker/Dockerfile` |
| monorepo-java | `templates/monorepo-java/apps/api/Dockerfile` |
| monorepo-java | `templates/monorepo-java/apps/worker/Dockerfile` |
| monorepo-php | `templates/monorepo-php/apps/api/Dockerfile` |
| monorepo-php | `templates/monorepo-php/apps/worker/Dockerfile` |
| monorepo-python | `templates/monorepo-python/apps/api/Dockerfile` |
| monorepo-python | `templates/monorepo-python/apps/worker/Dockerfile` |
| monorepo-ruby | `templates/monorepo-ruby/apps/api/Dockerfile` |
| monorepo-ruby | `templates/monorepo-ruby/apps/worker/Dockerfile` |
| monorepo-rust | `templates/monorepo-rust/apps/api/Dockerfile` |
| monorepo-rust | `templates/monorepo-rust/apps/worker/Dockerfile` |
| monorepo-turbo | `templates/monorepo-turbo/apps/api/Dockerfile` |
| monorepo-turbo | `templates/monorepo-turbo/apps/web/Dockerfile` |
| node-express | `templates/node-express/Dockerfile` |
| node-fastify | `templates/node-fastify/Dockerfile` |
| node-nestjs | `templates/node-nestjs/Dockerfile` |
| node-nextjs | `templates/node-nextjs/Dockerfile` |
| node-worker | `templates/node-worker/Dockerfile` |
| php-slim | `templates/php-slim/Dockerfile` |
| python-fastapi | `templates/python-fastapi/Dockerfile` |
| ruby-sinatra | `templates/ruby-sinatra/Dockerfile` |
| rust-axum | `templates/rust-axum/Dockerfile` |

**Total: 26.** Verify post-deletion: `find templates -name Dockerfile -type f | wc -l` returns `0`.

## theo-stacks READMEs updated (PR-B)

19 files (18 templates + 1 root). Each gets the "Build" / "Build artifacts" section per ADR D4.

| Path | Phase |
|---|---|
| `templates/fullstack-nextjs/README.md` | TB2.1 |
| `templates/go-api/README.md` | TB2.1 |
| `templates/java-spring/README.md` | TB2.1 |
| `templates/monorepo-go/README.md` | TB2.1 |
| `templates/monorepo-java/README.md` | TB2.1 |
| `templates/monorepo-php/README.md` | TB2.1 |
| `templates/monorepo-python/README.md` | TB2.1 |
| `templates/monorepo-ruby/README.md` | TB2.1 |
| `templates/monorepo-rust/README.md` | TB2.1 |
| `templates/monorepo-turbo/README.md` | TB2.1 |
| `templates/node-express/README.md` | TB2.1 |
| `templates/node-fastify/README.md` | TB2.1 |
| `templates/node-nestjs/README.md` | TB2.1 |
| `templates/node-nextjs/README.md` | TB2.1 |
| `templates/node-worker/README.md` | TB2.1 |
| `templates/php-slim/README.md` | TB2.1 |
| `templates/python-fastapi/README.md` | TB2.1 |
| `templates/ruby-sinatra/README.md` | TB2.1 |
| `templates/rust-axum/README.md` | TB2.1 |
| `README.md` (root) | TB2.1 |

## theo-stacks new files (PR-B)

| Path | Phase |
|---|---|
| `tests/no_dockerfiles_test.sh` | TB3.1 |
| `.github/workflows/contract.yml` (or step in existing workflow) | TB3.1 |
| `PR_DESCRIPTION.md` (or equivalent) | TB4.1 |
