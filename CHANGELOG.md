# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/)
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.4.0] - 2026-05-07

### Added
- **User-provided Dockerfile precedence is the canonical contract.** When a `Dockerfile` is present at `<app-dir>/Dockerfile`, `theopacks-generate` copies its content verbatim to `--output`, prints `[theopacks] User-provided Dockerfile found at <path> — skipping generation` to stderr, prints `--- User-provided Dockerfile ---` followed by the content to stdout, and exits with code 0. This behavior is the **defense-in-depth lower layer** for Theo's build pipeline: callers (Theo API) skip invoking theopacks-generate entirely when `app.Build == "dockerfile"`, but if it is invoked anyway (legacy path, bug, or direct usage), the user's Dockerfile is honored, never rejected. Supersedes the implicit-strict regression behavior observed in earlier image builds tagged `:latest` that errored with "user-supplied Dockerfile found ... Remove the file and rerun".
- Unit test `TestUserProvidedDockerfileTakesPrecedence` (`cmd/theopacks-generate/main_test.go`) pins the lenient behavior. Asserts: source dir with user Dockerfile + run binary → output equals user content verbatim + stdout contains "User-provided Dockerfile".
- Plan reference: `docs/plans/build-mode-dispatch-and-dockerfile-security-plan.md` (Theo repo) — D4/D5 codify this contract cross-repo.

## [Pre-0.4.0]

### Added
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
- Actions pinadas por commit SHA em vez de tags mutáveis para proteção contra supply chain attacks
- `build-runner.yml` agora roda testes antes de buildar e pushar a imagem
- `build-runner.yml` usa `docker/build-push-action` com cache GHA para builds incrementais
- CI ignora mudanças em docs e markdown para evitar execuções desnecessárias
- Workflows usam `mise run` em vez de comandos `go` diretos, consistente com Rule 1 do CLAUDE.md
