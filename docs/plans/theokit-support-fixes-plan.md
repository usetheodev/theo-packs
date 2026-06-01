# Plan: theokit-support fixes (B1 + B3 + regression guard)

> **Version 1.0** — Implementa Recomendações 1, 2, 4 do blueprint
> `.claude/knowledge-base/discoveries/blueprints/theokit-support-blueprint.md`
> (SHIPPABLE 96.8/100). Defers B2 conforme ADR D2 do blueprint. Output:
> theokit-packs gera Dockerfile correto para monorepos pnpm com
> `package.json#name ≠ dir-name`, e aborta com erro orientativo quando o
> pnpm-workspace.yaml lista paths siblings `../`. Validado E2E contra
> os 4 examples reais do theokit.

## Context

Sessão anterior renomeou `theo-packs → theokit-packs` (6 commits em
`develop`, branch ahead of main +6, working tree clean). Diagnóstico
inicial (mesma sessão, pre-rename) gerou Dockerfile para os 4 examples
do theokit e identificou 3 falhas:

- **B1** — `pnpm --filter` usa dir-name do `--app-name` em vez do
  `package.json#name` real. Falha 4/4 examples do theokit.
- **B3** — `pnpm-workspace.yaml` com entries `../theokit-sdk/...` é
  aceito silenciosamente; `pnpm install --frozen-lockfile` quebra
  dentro do container sem mensagem orientativa.
- **B2** — `CMD cd <app> && npm start` pode não resolver binário
  `theokit` do workspace. Q1 do discovery sugere que é sintoma de
  B1+B3, não bug independente — diferido até validar pós-B1+B3
  (ADR D2 do blueprint).

## Objective

Fixar B1 + B3, plantar regression guard (example interno + golden +
E2E), validar contra os examples reais do theokit. "Done" = (a)
`theokit-packs-generate` contra `theokit/examples/{deploy-vercel,
devtools-demo}` produz Dockerfile com filter usando nome real do
package, e (b) contra `theokit/examples/{full-stack-agent,
openrouter-demo}` (que dependem de sibling `theokit-sdk`) emite
**erro claro** identificando os entries `../` ofensores e dois
caminhos de remediação.

## ADRs

### D1 — Fix B1 lê `<appPath>/package.json#name` via `app.App` no Node provider

**Decisão:** Adicionar helper `readAppPackageName(a *app.App, appPath
string)` em `core/providers/node/node.go`. Quando `ws != nil && appName
!= "" && appPath != ""`, tentar ler o name real. Em sucesso, usar.
Em falha (file ausente, JSON inválido, name vazio), logar warn e
usar o `appName` literal vindo do env.

**Rationale:** Q5 do blueprint (NEGATIVE finding) confirmou que Turbo
e Nx exigem `package.json#name` real, sem dir-name fallback. Resolver
internamente respeita SRP (provider já lê outros campos de package.json
em `readPackageJSON`). Mantém o contrato `--app-name = dir-name`
externo (não quebra TheoCloud).

**Alternativas:** (a) Quebrar contrato CLI exigindo `--package-name` —
rejeitado, força caller a descobrir; (b) tentar dir-name primeiro e
fallback — rejeitado, fail-silent; (c) deletar `appName` argument e
forçar caller a passar resolved name — rejeitado, blueprint D1 explicitly
prefers internal resolution.

**Consequências:** +1 `app.ReadJSON()` no path do Plan() em workspace
mode. Custo ≈µs. Não afeta non-workspace path.

### D2 — Fix B3 fail-fast em `Plan()` quando workspace tem entries `../`

**Decisão:** Adicionar `validateWorkspaceEntries(patterns []string)
error` em `core/providers/node/workspace.go`. Chamada no início do
`Plan()` do Node provider, depois de `DetectWorkspace`. Retorna erro
estruturado se algum entry tem prefixo `../` (ou path absoluto).
Mensagem nomeia os entries + dois caminhos de remediação.

**Rationale:** Q3 do blueprint confirmou que esse caso é
**arquiteturalmente intencional** no theokit (ADR 0001 em
theokit-sdk). theokit-packs não conserta — declara incompatibilidade
com Docker build context e direciona a uma das duas estratégias
coerentes (publish-as-npm OU multi-context build).

**Alternativas:** (a) warning + skip — rejeitado, fail-silent;
(b) skipar entries `../` automaticamente — rejeitado, lockfile não-coerente;
(c) detectar via `DetectWorkspace` retornando erro — rejeitado, força
refactor de 2 callsites adicionais (KISS).

**Consequências:** examples theokit `full-stack-agent` + `openrouter-demo`
**não buildam** até o theokit publicar `@usetheo/sdk` como npm package
OU mudar source root. Esperado e desejável — é a única honest path.

### D3 — Regression guard interno: `examples/node-pnpm-workspaces-scoped/`

**Decisão:** Criar example interno minimal espelhando a **forma** do
theokit (pnpm-workspace.yaml + apps com `package.json#name` ≠ dir-name),
**sem** siblings `../`. Cobrir com golden + E2E.

**Rationale:** Blueprint Recomendação 4 (HIGH). Sem regression guard,
B1 pode regredir ao primeiro refactor.

**Consequências:** +1 example dir, +1 golden file, +1 case no e2e_test.go.

### D4 — B2 (Defer)

Não tocar agora. Após Recomendações 1+2 aplicadas, rodar
`theokit-packs-generate` contra os 2 examples theokit que NÃO dependem
de sibling (`deploy-vercel`, `devtools-demo`). Se Dockerfile build
local for green (ou ao menos não quebrar em `npm start`), B2 fica
fechado como "não-bug". Se quebrar, PR dedicado posterior.

## Dependency Graph

```
Phase 1 (Fix B1) ─┐
                  ├──▶ Phase 3 (Regression guard) ──▶ Phase 4 (Validate)
Phase 2 (Fix B3) ─┘
```

Phase 1 e Phase 2 podem ser commits independentes mas vou commitar
junto se test setup compartilhar fixtures (provavelmente sim).

---

## Phase 1: Fix B1 — package name resolution

**T1.1** — Add helper + integrate

- `core/providers/node/node.go`:
  - New function `readAppPackageName(a *app.App, appPath string) string` —
    reads `<appPath>/package.json`, returns `.name` ("" on failure).
  - Modify `Plan()`: when `ws != nil && appName != "" && appPath != ""`,
    call helper; on non-empty result, override `appName`. Log info.

- `core/providers/node/workspace_build_test.go` (or new test file):
  - Test using `app.NewMemoryApp` with fixture: root + `apps/api/package.json`
    where `name = "@scope/api"`. Set `THEOKIT_PACKS_APP_NAME=api`,
    `THEOKIT_PACKS_APP_PATH=apps/api`. Assert generated build command
    contains `--filter @scope/api...`, not `--filter api...`.

**Acceptance:** Test verde + existing tests verde (`go test ./core/...`).

## Phase 2: Fix B3 — fail-fast siblings `../`

**T2.1** — Add validator + integrate

- `core/providers/node/workspace.go`:
  - New function `validateWorkspaceEntries(patterns []string) error` —
    returns structured error if any pattern starts with `../` or is
    absolute. Error message names the offending entries and the two
    remediation paths.
  - `DetectWorkspace` continues to return `*WorkspaceInfo` (no
    signature change). New raw-entries reader returns patterns
    **before** glob expansion so we can validate.

- `core/providers/node/node.go`:
  - `Plan()` early-out: after `DetectWorkspace`, if `ws != nil`, call
    validator on raw entries. If err, return err from `Plan()`.

- New test: fixture pnpm-workspace.yaml com sibling entry, asserta
  que `core.GenerateBuildPlan` retorna result com `Success=false` e
  log error mencionando o entry e os 2 caminhos.

**Acceptance:** Test verde + existing tests verde.

## Phase 3: Regression guard example

**T3.1** — Create example + golden + e2e case

- `examples/node-pnpm-workspaces-scoped/` (NEW):
  - `package.json` (root) — `name: "scoped-monorepo-root"`, `private:
    true`, `type: module`, no workspaces field (uses pnpm-workspace.yaml)
  - `pnpm-workspace.yaml` — `packages: [apps/*]`
  - `pnpm-lock.yaml` — minimal valid (empty importers map)
  - `apps/api/package.json` — `name: "@scoped/api"`, `private: true`,
    `type: module`, `scripts: { build: "node build.js", start: "node
    server.js" }`, `dependencies: { ... minimal ... }`
  - `apps/api/build.js`, `apps/api/server.js` — minimal stubs
  - `apps/web/package.json` — symmetric, `name: "@scoped/web"`

- `core/dockerfile/integration_test.go` (or `goldens_audit_test.go`):
  - Add case `node_pnpm_workspaces_scoped` calling `GenerateBuildPlan`
    with `THEOKIT_PACKS_APP_NAME=api THEOKIT_PACKS_APP_PATH=apps/api`,
    asserts the generated Dockerfile contains
    `pnpm --filter @scoped/api... run build` and not
    `pnpm --filter api... run build`.

- Run `UPDATE_GOLDEN=true go test ./core/dockerfile/...` to create
  the golden file.

**Acceptance:** Golden created, all existing tests green.

## Phase 4: Validate against real theokit examples

**T4.1** — Build CLI + run against real theokit

- `go build -o /tmp/theokit-packs-generate ./cmd/theokit-packs-generate`
- For each theokit example, run the CLI with `--source` = theokit
  root, `--app-path` = `examples/<name>`, `--app-name` = `<dir-name>`,
  `--output` = `/tmp/df.<name>`.
- Capture stdout/stderr + Dockerfile.
- Expected results:
  - `deploy-vercel`: filter uses `example-deploy-vercel` (real name)
  - `devtools-demo`: filter uses `example-devtools-demo`
  - `full-stack-agent`: CLI fails with structured error message about
    `../theokit-sdk/...` entries (B3 trigger)
  - `openrouter-demo`: same as full-stack-agent

**Acceptance:** 2/4 produce correct Dockerfile, 2/4 fail with
structured error (the expected behavior given theokit's intentional
cross-repo workspace).

## Coverage Matrix

| # | Rec from blueprint | Phase / Task |
|---|---|---|
| 1 | B1 fix | T1.1 |
| 2 | B3 fix | T2.1 |
| 4 | Regression guard | T3.1 |
| 3 (deferred) | B2 | ADR D4 — not implemented this PR |
| 5 (contract doc) | — | Deferred to follow-up PR |
| 6 (Node 22 override check) | — | Deferred — TheoCloud can set `THEOKIT_PACKS_PACKAGES` |
| 7 (theokit alignment) | T4 | Validation phase covers it |

## Global Definition of Done

- [ ] T1.1 implemented + test green (B1)
- [ ] T2.1 implemented + test green (B3)
- [ ] T3.1 example + golden created, all tests green
- [ ] T4.1 validation: 2 OK + 2 expected-fail with structured error
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `go test ./core/... ./cmd/...` all green
- [ ] `CHANGELOG.md [Unreleased]` ganha entry documentando os fixes
- [ ] 1-3 commits atômicos em `develop` (B1, B3+guard, doc/validation)
