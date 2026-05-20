# Plan: Deep Review Hardening — Security, Code Smells & Artifact Correctness

> **Version 1.0** — Endereça todos os achados do deep review de 2026-05-20 contra o estado atual de `main` (commit 35f2f63). Cobre 4 vulnerabilidades CRITICAL (path traversal, command injection, symlink follow, secret pollution residual), 4 HIGH (recover() engolindo erros, runs-as-root, config-file allowlist, pseudo-version de dependência), 10 MEDIUM (comentário fantasma, SRP do main, errors sem %w, etc.) e 6 LOW. A entrega elimina vetores de exploit cross-tenant em ambiente Argo Workflow, normaliza tratamento de erro em conformidade com Rule 3/Rule 8 do CLAUDE global, e estabelece o CLAUDE.md do projeto como fonte da verdade da arquitetura atual (módulo único, sem `railpack/`).

## Context

Auditoria deep review executada em 2026-05-20 cobrindo segurança, code smells e geração de artefatos. Métodos: leitura completa de `core/`, `cmd/`, `internal/`; execução de `mise run test` (15/15 pacotes verde); compilação do binário; geração de Dockerfile para 7 exemplos (`node-express`, `python-fastapi`, `go-simple`, `staticfile`, `shell-script`, `node-turborepo`, `node-pnpm-workspaces`); build Docker real do `go-simple` (container respondeu HTTP) e tentativa de build dos workspaces Node.

### Achados verificados no estado atual

| ID | Severidade | Local | Evidência |
|---|---|---|---|
| C1 | CRITICAL (residual) | `core/core.go:215` | `config.Secrets = append(config.Secrets, slices.Sorted(maps.Keys(env.Variables))...)` — defaults de `step.Secrets` já foram migrados para `nil` (`core/plan/step.go:29`, `core/generate/command_step_builder.go:36`), mas a *promoção* de keys de env para plan-level secrets persiste. Reaparece se qualquer provider opt-in com `Secrets = []string{"*"}`. |
| C2 | CRITICAL | `core/providers/node/node.go:116`, `:156` | `fmt.Sprintf("cd %s && %s", appPath, ...)` e `--filter=%s...` sem sanitização. `appPath`/`appName` vêm da flag `--app-path`/`--app-name` controlada pelo Theo API (que recebe do `theo.yaml` do usuário). |
| C3 | CRITICAL | `cmd/theopacks-generate/main.go:80`, `:186` | `os.Stat(userDockerfile)` e `os.Stat(dockerignore)` seguem symlinks. TOCTOU vs `/etc/*`, secrets do Argo, cross-tenant. Crítico mesmo no novo contrato hard-fail: `os.Stat` em symlink retorna stat do target, então um symlink para `/etc/passwd` faria o binário rejeitar o build com mensagem "user-supplied Dockerfile found" — leak de existência. |
| C4 | CRITICAL | `cmd/theopacks-generate/main.go:63`, `core/core.go:158` | `appDir := filepath.Join(*source, *appPath)` sem clamp. Reproduzido: `--app-path ../../../etc` resolve para `/home/etc`. |
| H1 | HIGH | `internal/utils/merge.go:11-16` | `recover() { err = fmt.Errorf("config merge failed: %v", r) }` — engole panic do reflect e usa `%v` em vez de `%w`. Viola Rule 3 (error wrapping) e Rule 8 (NUNCA engolir exceções). |
| H2 | HIGH | `core/core.go:154` | `THEOPACKS_CONFIG_FILE` aceita qualquer path relativo ao `app.Source`. Combinado com C4, dá leitura arbitrária. |
| H3 | HIGH | `Dockerfile.generate:18-24` | Sem `USER` instruction — binário roda como root no Argo. |
| H4 | HIGH | `go.mod` | `tailscale/hujson v0.0.0-20260302212456-ecc657c15afd` (pseudo-version). Viola seção 9 do CLAUDE global. |
| M1 | MEDIUM | `core/validate.go:3` | `//Force 1` — marcador de force-push CI residual. Viola Rule 4. |
| M2 | MEDIUM | `cmd/theopacks-generate/main.go` | `main()` com 138 linhas, mistura: flag parse, user-Dockerfile contract, workspace detection, env bridging, generation, dual logging, dockerignore write. SRP. |
| M3 | MEDIUM (parcial) | `cmd/theopacks-generate/main.go:98-104` | Bridging CLI↔provider via env vars sintéticas (`THEOPACKS_APP_NAME`, `THEOPACKS_APP_PATH`). Já condicionalizado (`appPath != "."` skip), mas o acoplamento por string persiste. Solução: campo dedicado em `GenerateBuildPlanOptions`. |
| M4 | MEDIUM | `cmd/theopacks-generate/main.go:59,126,158,163,166` | `log.Fatal*` em 5 pontos + `os.Exit(1)` em `:138` e `os.Exit(2)` em `:88`. Padronizar. |
| M5 | MEDIUM | `core/providers/golang/golang.go:148`, `core/providers/node/workspace.go:38` | `log ...*logger.Logger` (variadic 0-1 logger) para "evitar nil check". DIP: passar `*logger.Logger` explícito. |
| M7 | MEDIUM | `core/dockerfile/generate.go` (próx à `writeFileCommand`) | `shellEscape` custom em vez de `strconv.Quote`. |
| M8 | MEDIUM | múltiplos | `internal/utils/merge.go:14` usa `%v`; 8 bare `return err` sem contexto em `core/app/app.go:153,158,174,187`, `core/providers/python/python.go:64`, `core/core.go:47,52`, `core/plan/step.go:43,51`, `core/config/config.go:81`. |
| M9 | MEDIUM | `core/providers/node/node.go:116` | `cd X && Y` start cmd frágil. Substituir por `WORKDIR` explícito + array-form CMD quando possível. |
| M10 | MEDIUM | `CHANGELOG.md:8` | Seção `[Unreleased]` vazia apesar de mudanças significativas (secrets default migration, hard-fail user Dockerfile, dockerignore generation, condicional env bridging). Viola Rule 6. |
| L1 | LOW | (corrigido) | `appPath == "."` agora skipa `THEOPACKS_APP_PATH`. |
| L2 | LOW | `core/dockerfile/generate.go` | `/app` hardcoded em `COPY --from=%s /app /app`. Convenção implícita. |
| L3 | LOW | `core/app/app.go:33` | `filepath.Abs(filepath.Join(currentDir, path))` redundante — `filepath.Abs(path)` resolve. |
| L4 | LOW | `core/generate/context.go:161` | Sanitização existe para apt packages mas não para `appName`/`appPath` (cobre C2). |
| L5 | LOW | `core/logger/logger.go` | `Logs` é `append`-only sem cap. Plano gigante = response inflada. |
| L6 | LOW | `examples/node-express/package-lock.json` | Lockfile dessincronizado — `npm ci` falha no build E2E. |
| L7 | LOW | `CLAUDE.md` projeto | Documenta `railpack/` que não existe e tasks Mise (`setup`, `cli`, `test-integration`) que não existem em `mise.toml`. |

### Mudanças já aplicadas pelo merge externo (NÃO repetir)

- ✅ `Step.Secrets` default = `nil` (`core/plan/step.go:29`)
- ✅ `CommandStepBuilder.Secrets` default = `nil` (`core/generate/command_step_builder.go:36`)
- ✅ CMD usa `/bin/sh` + exec form quando possível (resolve M6 do review original)
- ✅ Syntax directive `# syntax=docker/dockerfile:1` emitido (`core/dockerfile/generate.go:46`)
- ✅ Header comment com contexto de build (`core/dockerfile/generate.go:47`)
- ✅ Non-root USER (`appuser`) emitido no deploy stage
- ✅ `chown` + `--chown` no COPY do deploy
- ✅ Cache mount `--mount=type=cache,target=/root/.npm`
- ✅ `npm prune --omit=dev` na build stage
- ✅ Default `.dockerignore` per-language
- ✅ `appPath == "."` skipa env var bridge (L1)
- ✅ User-Dockerfile contract MUDOU para hard-fail (exit 2) — contradiz CHANGELOG 0.4.0; alinhar via M10

## Objective

**Done** = `mise run check && mise run test` verde, binário roda como non-root, todos os 4 vetores CRITICAL fechados com teste de regressão correspondente, comentário `//Force 1` removido, CLAUDE.md do projeto reflete estrutura real, CHANGELOG documenta todas as mudanças desde 0.4.0.

Metas mensuráveis:

1. Zero ocorrências de `os.Stat` (segue symlink) em paths controlados externamente — substituir por `os.Lstat` com check explícito.
2. Toda flag de string da CLI (`--app-path`, `--app-name`, `THEOPACKS_CONFIG_FILE`) passa por regex `^[A-Za-z0-9._/@-]+$` na entrada, falha com exit 2 se inválida.
3. Toda concatenação shell envolvendo input externo usa array-form ou shell escaping (`strconv.Quote` / `%q` semanticamente para shell, não Go).
4. `internal/utils/merge.go` não usa `recover()` para tratar erros de programação (DIP: retornar erro tipado).
5. `Dockerfile.generate` cria usuário `theopacks` (UID 10001) e termina com `USER theopacks`.
6. CLAUDE.md do projeto descreve a estrutura atual (sem `railpack/`).
7. `CHANGELOG.md` tem entry `[Unreleased]` listando todas as mudanças desde `[0.4.0]`.
8. Test coverage: cada finding endereçado tem ao menos 1 teste de regressão verde.

## ADRs

### D1 — Sanitização defensiva na fronteira da CLI

**Decisão:** validar `--app-path`, `--app-name`, `--source`, `--output`, e `THEOPACKS_CONFIG_FILE` contra regex restritiva `^[A-Za-z0-9._/@-]+$` (path) e `^[A-Za-z0-9._@/-]+$` (name). Reject com exit code 2 + mensagem identificando o campo.

**Rationale:** Rule 8 do CLAUDE global ("Falhe alto, falhe cedo, falhe claro" — valide nas fronteiras). Alternativas consideradas: (a) sanitização no provider — viola SRP (cada provider re-implementaria), (b) shell escaping no render — possível, mas torna a auditoria de injeção dispersa. Centralizar na CLI alinha com a regra "depois da fronteira, dados são confiáveis".

**Consequências:** Habilita confiar em `appPath`/`appName` no `core/`. Restringe nomes legítimos (sem espaços, sem unicode) — aceitável para identifiers de monorepo. Caracteres `@/-_.` cobrem todos os formatos npm-scoped (`@scope/pkg`) e paths Unix.

### D2 — Reject symlinks em paths controlados externamente

**Decisão:** substituir `os.Stat` por `os.Lstat` seguido de check explícito `info.Mode()&os.ModeSymlink == 0` antes de qualquer operação em `userDockerfile` e `dockerignore` em `cmd/theopacks-generate/main.go`. Se for symlink, emitir erro e abortar.

**Rationale:** Defense in depth contra TOCTOU em ambiente multi-tenant (Argo Workflow). Alternativas: (a) `O_NOFOLLOW` em open — específico de Linux/syscall, perde portabilidade do `os` package, (b) seguir symlinks mas validar target estiver dentro de `--source` — funcional mas frágil (race conditions). Hard-reject é o mais simples e seguro (KISS).

**Consequências:** Usuários não podem usar `Dockerfile` como symlink (raríssimo, e podem usar arquivo real). Reduz superfície de ataque a zero.

### D3 — Path clamp em todo `filepath.Join` derivado de input externo

**Decisão:** após `filepath.Join(*source, *appPath)`, validar via `filepath.Abs` + `strings.HasPrefix(abs+sep, sourceAbs+sep)`. Mesma validação para `THEOPACKS_CONFIG_FILE`.

**Rationale:** `filepath.Join` em Go **não** previne traversal. Documentação oficial é explícita. Validar com `Abs` é a forma canônica (idiomática Go). Alternativa rejeitada: confiar no Theo API para sanitizar — viola defense in depth.

**Consequências:** Path traversal fica fechado mesmo se o caller upstream falhar.

### D4 — Substituir env-var bridge por campo explícito em `GenerateBuildPlanOptions`

**Decisão:** adicionar `WorkspaceTarget *WorkspaceTarget` em `GenerateBuildPlanOptions` (com fields `AppName`, `AppPath`). Providers que precisam consultam via `ctx.Options.WorkspaceTarget`. Manter `THEOPACKS_APP_*` lendo do env por backward compat (warning de deprecação).

**Rationale:** Acoplamento por string é frágil (causa M3 e contribui para C1). Tipo explícito é checado pelo compilador e impossível de confundir com user secret. Alternativas: (a) novo objeto separado fora de Options — quebra a API existente, (b) manter env vars + filtrar em `core.go:215` por prefixo — funciona como mitigação tática, MAS não resolve o acoplamento de design.

**Consequências:** Migration interna; sem quebra de API externa (env vars continuam funcionando). Filtragem em `core.go:215` ainda é necessária como defesa em profundidade (cobre o caso de usuário passar env vars com mesmo nome).

### D5 — Remover `recover()` engolidor em `MergeStructs`

**Decisão:** `MergeStructs` valida antes (`reflect.TypeOf(dst) == reflect.TypeOf(src)` para cada par) e retorna erro tipado em vez de capturar panic. Mantém defesa pontual via `recover` apenas para panics genuinamente inesperados, mas nesse caso usa `%w` com `fmt.Errorf("config merge panic: %w", fmt.Errorf("%v", r))`.

**Rationale:** Rule 8 — exceções não são para controle de fluxo. Panic do reflect é bug de programação, não erro de config. Esconder atrás de mensagem genérica viola Rule 3 e impede diagnóstico.

**Consequências:** Quando merge falha, log identifica campo exato. Aumenta cobertura de testes (precisa validar paths de erro).

### D6 — Binário roda como non-root no `Dockerfile.generate`

**Decisão:** `Dockerfile.generate` cria usuário `theopacks` UID 10001 e termina com `USER theopacks`. Mantém root apenas durante `apk add`.

**Rationale:** Defense in depth contra C3/C4. Argo executa o contêiner — se ele for comprometido (zero day em algum parser, supply chain via `tailscale/hujson`), reduz blast radius.

**Consequências:** Workspace montado pelo Argo precisa ser legível pelo UID 10001. Theo controla esse mount, então é coordenável. Documentar requisito no CLI contract.

### D7 — `CHANGELOG.md` como contrato com upstream Theo

**Decisão:** popular `[Unreleased]` com TODAS as mudanças desde 0.4.0 — secret default migration, hard-fail user Dockerfile, dockerignore default, condicional env bridging, esta plataforma de fixes — em formato Keep a Changelog. Ao final do plano, recortar para uma versão `[0.5.0]`.

**Rationale:** Rule 6 — "se não está no changelog, ela não aconteceu". Theo product depende do contrato do theo-packs; mudanças não documentadas geram surpresa.

**Consequências:** Disciplina de commit message (a partir do plano: toda mudança visível atualiza CHANGELOG no mesmo PR).

### D8 — CLAUDE.md do projeto deve refletir estrutura real

**Decisão:** reescrever as seções "What This Project Is" e "Repository Layout" do `CLAUDE.md` na raiz, removendo referências a `railpack/`, alinhando lista de mise tasks com `mise.toml` atual, e adicionando referência ao novo `docs/contracts/theo-packs-cli-contract.md`.

**Rationale:** Documentação errada engana onboarding humano e LLMs. CLAUDE.md tem precedência sobre comportamento default (system reminder). Manter desatualizado é dívida invisível.

**Consequências:** Próximas iterações de agentes começam com mapa correto.

## Dependency Graph

```
Phase 0: Hygiene & docs (no code changes blocking)
  │
  ├──▶ Phase 1: Security CRITICAL (C2, C3, C4 + C1 residual)
  │       │
  │       └──▶ Phase 2: Security HIGH (H1, H2, H3, H4)
  │              │
  │              └──▶ Phase 3: Code quality (M2, M3, M5, M7, M8, L2-L5)
  │
  └──▶ Phase 4: Polish (M4, L6, final CHANGELOG cut)
```

- Phase 0 e Phase 1 podem rodar em paralelo (Phase 0 só toca docs).
- Phase 1 bloqueia Phase 2 (D4 do Phase 1 muda assinatura de Options usada em Phase 2 H2).
- Phase 3 e Phase 4 são paralelas após Phase 2.

---

## Phase 0: Hygiene & docs

**Objective:** zerar débito documental e ruído de código antes das mudanças críticas, para que o diff das próximas fases fique limpo.

### T0.1 — Remover comentário `//Force 1`

#### Objective
Eliminar marcador residual de force-push em `core/validate.go:3`.

#### Evidence
`grep -n "Force" core/validate.go` retorna `3://Force 1`. Commits `f6b6c77`, `2b8a2e1` no histórico mencionam "specify force version". Comentário não explica WHY (viola Rule 4 do CLAUDE).

#### Files to edit
```
core/validate.go — remover linha 3
```

#### Deep file dependency analysis
- **`core/validate.go`** — arquivo de validação de `BuildPlan`. Linha 3 é comentário órfão entre `package core` e o `import`. Remoção não afeta semântica nem build.
- Nenhum downstream depende dessa linha.

#### Deep Dives
N/A — remoção textual pura.

#### Tasks
1. Editar `core/validate.go` removendo linha `//Force 1`.

#### TDD
```
RED:     N/A (mudança de comentário não tem comportamento testável)
GREEN:   Edit textual.
REFACTOR: None expected.
VERIFY:  go vet ./... && go test ./core/...
```

#### Acceptance Criteria
- [ ] `grep -n "Force" core/validate.go` retorna vazio
- [ ] `go vet ./...` passa
- [ ] `go test ./core/...` passa

#### DoD
- [ ] Linha removida
- [ ] `mise run check` verde
- [ ] `mise run test` verde

---

### T0.2 — Atualizar CLAUDE.md do projeto para refletir estrutura real

#### Objective
Reescrever seções desatualizadas do `CLAUDE.md` removendo referências a `railpack/` e alinhando com `mise.toml`.

#### Evidence
- `find . -type d -name railpack` retorna vazio
- `mise.toml` define apenas tasks `test`, `check`, `tidy` (não `setup`, `cli`, `test-integration`, `test-update-snapshots`)
- CLAUDE.md atual descreve `railpack/cmd/cli`, `railpack/buildkit/`, `railpack/examples/` que não existem

#### Files to edit
```
CLAUDE.md — reescrever seções "What This Project Is" e "Repository Layout", atualizar lista de mise tasks
```

#### Deep file dependency analysis
- **`CLAUDE.md`** — carregado em todo invocation Claude Code via system reminder. Atualmente engana sobre estrutura de diretórios.
- Downstream: agentes Claude, novos engenheiros, scripts que parsem o doc.

#### Deep Dives
- Manter seções que já refletem realidade (App abstraction, Provider detection order, env vars).
- Adicionar pointer para `docs/contracts/theo-packs-cli-contract.md`.
- Não inventar novos princípios; só corrigir o que está errado.

#### Tasks
1. Reescrever "What This Project Is" descrevendo módulo único `github.com/usetheo/theopacks`, componentes `core/`, `cmd/theopacks-generate/`, `internal/utils/`, `e2e/`, `examples/`.
2. Substituir "Repository Layout" pelo tree atual.
3. Atualizar "Development Commands" para apenas as tasks que existem em `mise.toml`.
4. Substituir referências a `railpack/` em "Adding a New Provider" pela estrutura `core/providers/<lang>/`.

#### TDD
```
RED:     N/A (doc-only)
GREEN:   Edit do CLAUDE.md.
REFACTOR: None expected.
VERIFY:  grep -c "railpack" CLAUDE.md retorna 1 (apenas em Acknowledgements/derivação histórica)
```

#### Acceptance Criteria
- [ ] `grep -c "railpack" CLAUDE.md` ≤ 1 (apenas seção Acknowledgements)
- [ ] Lista de mise tasks em CLAUDE.md == tasks reais em `mise.toml`
- [ ] Tree de "Repository Layout" reflete `find . -maxdepth 3 -type d`

#### DoD
- [ ] Reescrita feita
- [ ] Diff revisado manualmente para precisão

---

### T0.3 — Popular CHANGELOG `[Unreleased]` com mudanças desde 0.4.0

#### Objective
Documentar todas as mudanças visíveis ao consumidor que ocorreram desde `[0.4.0]`.

#### Evidence
- `CHANGELOG.md:8` tem `## [Unreleased]` vazia
- Múltiplas mudanças significativas detectadas no merge externo: secrets default migration, hard-fail user Dockerfile contract (contradiz 0.4.0!), dockerignore default, syntax directive, non-root user, etc.

#### Files to edit
```
CHANGELOG.md — popular [Unreleased]
```

#### Deep file dependency analysis
- **`CHANGELOG.md`** — contrato com upstream (Theo product). Sem entry, mudanças "não aconteceram" do ponto de vista de consumidores.

#### Deep Dives
Categorias a usar (Keep a Changelog, ordem fixa):
- **Added**: dockerignore default, syntax directive emit, non-root USER, cache mounts, healthcheck support, prune step
- **Changed**: User-Dockerfile contract reverteu para hard-fail (exit 2) — **breaking change vs 0.4.0**; default de `Step.Secrets` migrou de `["*"]` para `nil`
- **Fixed**: appPath="." não bridgeia mais THEOPACKS_APP_PATH
- **Security**: (nesta fase vazio; popular ao final do plano)

Cada entrada deve referenciar issue/PR. Como não há números disponíveis no momento, usar `(#TBD)` e atualizar antes do release.

#### Tasks
1. Listar todas as mudanças visíveis comparando git log com `[0.4.0]`.
2. Categorizar em Added / Changed / Fixed.
3. Escrever cada uma em linguagem de consumidor (não dev).
4. Marcar a mudança no user-Dockerfile contract como **BREAKING** com explicação.

#### TDD
```
RED:     N/A (doc)
GREEN:   Edit do CHANGELOG.md.
REFACTOR: None expected.
VERIFY:  visual review
```

#### Acceptance Criteria
- [ ] `[Unreleased]` tem ≥ 1 entry em Added, Changed, Fixed
- [ ] Breaking change do user-Dockerfile está marcado com **BREAKING** prefix
- [ ] Toda entry tem `(#TBD)` ou referência real

#### DoD
- [ ] CHANGELOG.md atualizado
- [ ] Visual review feito

---

## Phase 1: Security CRITICAL

**Objective:** fechar os 4 vetores de exploit identificados (C1 residual, C2, C3, C4) com teste de regressão por finding.

### T1.1 — Sanitização defensiva das flags CLI (cobre C2, C4, L4)

#### Objective
Validar `--source`, `--app-path`, `--app-name`, `--output`, `THEOPACKS_CONFIG_FILE` na entrada com regex restritiva. Falha rápida com exit code 2.

#### Evidence
Reproduzido em sessão: `theopacks-generate --app-path ../../../etc` resolve para `/home/etc`. `core/providers/node/node.go:116` injeta `appPath` em `cd %s && %s` sem escape.

#### Files to edit
```
cmd/theopacks-generate/main.go — adicionar validateInput(*source, *appPath, *appName, *output) antes de qualquer uso
cmd/theopacks-generate/validate.go (NEW) — exportar regexes e função validateCLIInput
cmd/theopacks-generate/validate_test.go (NEW) — testes da regex
```

#### Deep file dependency analysis
- **`cmd/theopacks-generate/main.go`** — entry point. Hoje confia nas flags sem validar. Mudança: chamar `validateCLIInput` logo após `flag.Parse()`.
- **`cmd/theopacks-generate/validate.go` (NEW)** — isolado para testabilidade. Sem deps de `core/`.
- Downstream: `core.GenerateBuildPlan` recebe app já validado; providers podem confiar.

#### Deep Dives
- **Regex para paths**: `^[A-Za-z0-9._/@-]+$` (não permite espaço, unicode, shell metacharacters, `..` é PERMITIDO textualmente — bloqueio de traversal é T1.2). Hifen no final do char class para não virar range.
- **Regex para names**: `^[@A-Za-z0-9_][A-Za-z0-9._@/-]*$` permite npm scoped (`@theo/api`).
- **Reject empty `--output`**: já existe (`main.go:58`), padronizar exit code = 2.
- **Edge cases**: 
  - `--app-path ""` → reject (não pode ser vazio, mas `.` é válido)
  - `--app-path .` → válido
  - `--app-path apps/api` → válido
  - `--app-path apps/api with space` → reject
  - `--app-name @theo/api` → válido
  - `--app-name api;rm -rf /` → reject

#### Tasks
1. Criar `cmd/theopacks-generate/validate.go` com:
   - `var pathRegex = regexp.MustCompile("^[A-Za-z0-9._/@-]+$")`
   - `var nameRegex = regexp.MustCompile("^[@A-Za-z0-9_][A-Za-z0-9._@/-]*$")`
   - `func validateCLIInput(source, appPath, appName, output string) error` — retorna erro tipado por campo.
2. Em `main.go`, chamar logo após `flag.Parse()`. Em erro: `fmt.Fprintln(os.Stderr, err); os.Exit(2)`.
3. Validar `THEOPACKS_CONFIG_FILE` quando lido em `core/core.go:147` — adicionar check de regex de path lá também (defense em profundidade).
4. Atualizar test do binário (`cmd/theopacks-generate/main_test.go`) para casos negativos.

#### TDD
```
RED:     TestValidateCLI_RejectsAppPathTraversal — appPath="../../etc" → err não nil
RED:     TestValidateCLI_RejectsAppNameInjection — appName="api;rm -rf /" → err não nil
RED:     TestValidateCLI_RejectsSourceEmpty — source="" → err não nil
RED:     TestValidateCLI_AllowsScopedNpmName — appName="@theo/api" → err nil
RED:     TestValidateCLI_AllowsDotAppPath — appPath="." → err nil
RED:     TestValidateCLI_AllowsNestedAppPath — appPath="apps/api" → err nil
RED:     TestMain_RejectsBadAppPath_ExitCode2 — subprocess com appPath inválido → exit 2 + stderr menciona campo
GREEN:   Implementar validate.go + chamada em main.go.
REFACTOR: Mover regexes para constantes nomeadas.
VERIFY:  go test ./cmd/... -run TestValidateCLI && go test ./cmd/... -run TestMain_RejectsBadAppPath
```

#### Acceptance Criteria
- [ ] Todos os testes acima passam
- [ ] `--app-path ../../etc` falha com exit 2 e stderr menciona "appPath"
- [ ] `--app-name api;rm` falha com exit 2 e stderr menciona "appName"
- [ ] Inputs legítimos (`.`, `apps/api`, `@scope/pkg`) continuam aceitos
- [ ] `go test ./cmd/... -count=1` verde

#### DoD
- [ ] T1.1.{1..4} completos
- [ ] Todos os testes acima verdes
- [ ] `go vet ./cmd/...` zero warnings

---

### T1.2 — Path clamp em `--app-path` e `THEOPACKS_CONFIG_FILE` (cobre C4)

#### Objective
Garantir que `appDir = filepath.Join(source, appPath)` resolve para dentro de `source` mesmo após `..`. Validar via `filepath.Abs` + `HasPrefix`.

#### Evidence
T1.1 fecha shell injection, MAS regex permite `..` textualmente (compatibilidade com paths relativos legítimos). Path traversal precisa de clamp pós-join.

#### Files to edit
```
cmd/theopacks-generate/main.go — chamar clampPath após filepath.Join
cmd/theopacks-generate/path.go (NEW) — função clampPath(source, sub) (string, error)
cmd/theopacks-generate/path_test.go (NEW)
core/core.go — clampPath em config file lookup (linha 158)
```

#### Deep file dependency analysis
- **`cmd/theopacks-generate/path.go` (NEW)** — pura, sem deps. Recebe source e sub, retorna abs path ou erro.
- **`main.go`** — chama clampPath após `filepath.Join(*source, *appPath)`. Idem `filepath.Join(appDir, "Dockerfile")` (defesa adicional embora a `Dockerfile` seja literal).
- **`core/core.go:158`** — `filepath.Join(app.Source, configFileName)` precisa do mesmo clamp.

#### Deep Dives
```go
func clampPath(root, sub string) (string, error) {
    rootAbs, err := filepath.Abs(root)
    if err != nil { return "", fmt.Errorf("source path %q: %w", root, err) }
    joined := filepath.Join(rootAbs, sub)
    if !strings.HasPrefix(joined+string(filepath.Separator), rootAbs+string(filepath.Separator)) && joined != rootAbs {
        return "", fmt.Errorf("path %q escapes source root %q", sub, rootAbs)
    }
    return joined, nil
}
```

Invariante: para todo `clampPath(root, sub)` que retorna `(p, nil)`, vale `strings.HasPrefix(p+sep, root+sep) || p == root`.

#### Tasks
1. Criar `cmd/theopacks-generate/path.go` com `clampPath`.
2. Em `main.go:63`, substituir `filepath.Join(*source, *appPath)` por `clampPath(*source, *appPath)` com erro → exit 2.
3. Em `core/core.go:158`, aplicar mesmo clamp para `configFileName`.
4. Documentar em CLI contract.

#### TDD
```
RED:     TestClampPath_RejectsTraversal — clampPath("/src", "../etc") → err
RED:     TestClampPath_RejectsAbsolute — clampPath("/src", "/etc/passwd") → err
RED:     TestClampPath_AllowsDot — clampPath("/src", ".") → "/src", nil
RED:     TestClampPath_AllowsNested — clampPath("/src", "apps/api") → "/src/apps/api", nil
RED:     TestClampPath_AllowsDotPrefix — clampPath("/src", "./apps") → "/src/apps", nil
RED:     TestMain_RejectsAppPathTraversal_ExitCode2 — subprocess --app-path ../../etc → exit 2
GREEN:   Implementar clampPath e integrar em main.go + core.go.
REFACTOR: Extrair erro tipado PathEscapesRootError.
VERIFY:  go test ./cmd/... ./core/...
```

#### Acceptance Criteria
- [ ] `clampPath` reject 100% dos paths que escapam root
- [ ] `THEOPACKS_CONFIG_FILE=/etc/passwd` falha em `core.GenerateBuildPlan` com erro claro
- [ ] Path legítimos não afetados

#### DoD
- [ ] T1.2.{1..4} completos
- [ ] Testes verdes
- [ ] Documento de contrato atualizado

---

### T1.3 — Rejeitar symlinks em `userDockerfile` e `dockerignore` (cobre C3)

#### Objective
Substituir `os.Stat` por `os.Lstat` + check `ModeSymlink == 0` em `cmd/theopacks-generate/main.go:80` e `:186`.

#### Evidence
Reproduzido: criar `Dockerfile -> /etc/hostname` no source faz `os.Stat` retornar info do target. No contrato hard-fail atual, isso causa leak de existência. Em contrato lenient anterior (CHANGELOG 0.4.0) causaria leak de conteúdo.

#### Files to edit
```
cmd/theopacks-generate/main.go — substituir os.Stat por os.Lstat + check em :80 e :186
cmd/theopacks-generate/main_test.go — adicionar testes de regressão com symlink
```

#### Deep file dependency analysis
- **`main.go:80`** — gate do user-Dockerfile contract. Symlink no path == hard-fail (consistente com policy "no user Dockerfile in app dir").
- **`main.go:186`** — gate do dockerignore default. Symlink == skipa generation (não sobrescreve user file).
- Tests: subprocess-based para garantir behavior real do binário.

#### Deep Dives
```go
info, err := os.Lstat(userDockerfile)
if err == nil {
    if info.Mode()&os.ModeSymlink != 0 {
        fmt.Fprintf(os.Stderr,
            "[theopacks] ERROR: %s is a symlink — refusing to follow.\n", userDockerfile)
        os.Exit(2)
    }
    if info.Mode().IsRegular() {
        // existing hard-fail path
    }
}
```

Edge cases:
- `Dockerfile` é arquivo regular → comportamento atual (hard-fail).
- `Dockerfile` é symlink → reject explícito.
- `Dockerfile` é diretório → ignorar (não é file).
- `Dockerfile` não existe → continuar (path normal).

#### Tasks
1. Em `main.go:80`, substituir `os.Stat` por `os.Lstat`, adicionar check `ModeSymlink`.
2. Em `main.go:186`, mesmo padrão.
3. Adicionar `TestUserDockerfile_RejectsSymlink` em `main_test.go`.
4. Adicionar `TestDockerignore_SkipsSymlink` em `main_test.go`.

#### TDD
```
RED:     TestUserDockerfile_RejectsSymlink — criar Dockerfile symlink, run binary → exit 2 + stderr "symlink"
RED:     TestUserDockerfile_HonorsRegularFile_HardFails — Dockerfile arquivo regular → exit 2 + stderr "single source of truth"
RED:     TestDockerignore_SkipsSymlink — .dockerignore symlink → binary continua, NÃO sobrescreve
GREEN:   Substituir os.Stat por os.Lstat + check.
REFACTOR: Extrair helper isRegularNonSymlink(path).
VERIFY:  go test ./cmd/... -run TestUserDockerfile && go test ./cmd/... -run TestDockerignore
```

#### Acceptance Criteria
- [ ] Symlink em `userDockerfile` produz exit 2 + mensagem identificando symlink
- [ ] Symlink em `.dockerignore` resulta em "skipping default generation" sem sobrescrever
- [ ] Arquivo regular continua sendo tratado como antes

#### DoD
- [ ] T1.3.{1..4} completos
- [ ] Testes verdes
- [ ] `go test ./cmd/... -count=1` zero falhas

---

### T1.4 — Defense in depth: filtrar `THEOPACKS_*` internos antes de virar plan-level secrets (cobre C1 residual)

#### Objective
Em `core/core.go:215`, excluir keys com prefixo `THEOPACKS_` que sejam configuração interna (`APP_NAME`, `APP_PATH`) do `config.Secrets`. Manter promoção para outras env vars (pode ser desejado em alguns providers).

#### Evidence
`core.go:215` faz `config.Secrets = append(config.Secrets, slices.Sorted(maps.Keys(env.Variables))...)`. Defaults novos de `step.Secrets = nil` evitam o vazamento HOJE, mas qualquer provider opt-in com `Secrets = ["*"]` faz reaparecer. Defesa em profundidade.

#### Files to edit
```
core/core.go — filtrar THEOPACKS_APP_NAME, THEOPACKS_APP_PATH, e outros internos antes de promover
core/core_test.go — TestGenerateConfigFromEnvironment_FiltersInternalKeys
```

#### Deep file dependency analysis
- **`core/core.go:213-218`** — `GenerateConfigFromEnvironment`. Mudança: definir conjunto `internalKeys = {"THEOPACKS_APP_NAME", "THEOPACKS_APP_PATH"}` e filtrar.
- Downstream: `config.Secrets` é lido em `core/generate/context.go:188` (`c.Config.Secrets`). Tudo que NÃO está em internalKeys continua promovido — backward compat.

#### Deep Dives
Lista de keys internos (devem ser tratados como configuração, não secret):
- `THEOPACKS_APP_NAME`
- `THEOPACKS_APP_PATH`
- `THEOPACKS_CONFIG_FILE`
- `THEOPACKS_PROVIDER` (override)
- `THEOPACKS_GO_MODULE` (workspace target)
- `THEOPACKS_*_VERSION` (todos os language version overrides)

Heurística: `THEOPACKS_*_VERSION`, `THEOPACKS_APP_*`, `THEOPACKS_CONFIG_*` são configuração. `THEOPACKS_BUILD_APT_PACKAGES`, `THEOPACKS_DEPLOY_APT_PACKAGES`, `THEOPACKS_PACKAGES`, `THEOPACKS_INSTALL_CMD`, `THEOPACKS_BUILD_CMD`, `THEOPACKS_START_CMD` também são configuração.

Implementação:
```go
var internalConfigKeys = map[string]bool{
    "THEOPACKS_APP_NAME": true, "THEOPACKS_APP_PATH": true,
    "THEOPACKS_CONFIG_FILE": true, "THEOPACKS_PROVIDER": true,
    "THEOPACKS_GO_MODULE": true, "THEOPACKS_PACKAGES": true,
    "THEOPACKS_INSTALL_CMD": true, "THEOPACKS_BUILD_CMD": true,
    "THEOPACKS_START_CMD": true,
    "THEOPACKS_BUILD_APT_PACKAGES": true, "THEOPACKS_DEPLOY_APT_PACKAGES": true,
    // language versions: padrão THEOPACKS_*_VERSION
}

func isInternalConfigKey(k string) bool {
    if internalConfigKeys[k] { return true }
    return strings.HasPrefix(k, "THEOPACKS_") && strings.HasSuffix(k, "_VERSION")
}
```

#### Tasks
1. Adicionar `internalConfigKeys` e `isInternalConfigKey` em `core/core.go`.
2. Filtrar em `:215`: `for _, k := range slices.Sorted(...) { if !isInternalConfigKey(k) { ... } }`.
3. Testes cobrindo: key interno NÃO promovido, key externo promovido.

#### TDD
```
RED:     TestGenerateConfigFromEnvironment_FiltersAppName — env {"THEOPACKS_APP_NAME": "x"} → config.Secrets não contém "THEOPACKS_APP_NAME"
RED:     TestGenerateConfigFromEnvironment_FiltersGoVersion — env {"THEOPACKS_GO_VERSION": "1.22"} → não promovido
RED:     TestGenerateConfigFromEnvironment_PromotesExternalSecret — env {"DATABASE_URL": "x"} → config.Secrets contém "DATABASE_URL"
GREEN:   Adicionar filtro.
REFACTOR: Extrair `isInternalConfigKey` para arquivo dedicado se ficar grande.
VERIFY:  go test ./core/ -run TestGenerateConfigFromEnvironment
```

#### Acceptance Criteria
- [ ] Nenhuma key `THEOPACKS_APP_*` aparece em `plan.Secrets` em testes E2E
- [ ] Keys externas (DATABASE_URL, etc.) continuam promovidas
- [ ] Dockerfile gerado para `node-turborepo` com `--app-path apps/api` NÃO contém `--mount=type=secret,id=THEOPACKS_*`

#### DoD
- [ ] T1.4.{1..3} completos
- [ ] Regerar Dockerfile do node-turborepo e confirmar ausência de mount=secret indevido
- [ ] `mise run test` verde

---

## Phase 2: Security HIGH

**Objective:** endereçar findings HIGH de defesa em profundidade.

### T2.1 — Remover `recover()` engolidor em `MergeStructs` (cobre H1)

#### Objective
Substituir `recover() { err = fmt.Errorf("config merge failed: %v", r) }` por validação prévia + erro tipado com `%w`.

#### Evidence
`internal/utils/merge.go:11-16` engole panic do reflect com `%v`. Viola Rule 3 e Rule 8. Esconde bugs como type mismatch entre structs.

#### Files to edit
```
internal/utils/merge.go — substituir recover por pre-validation
internal/utils/merge_test.go — adicionar testes de type mismatch
```

#### Deep file dependency analysis
- **`internal/utils/merge.go`** — utilitário usado por `core/config/config.go:68` (`Merge`). Mudança preserva assinatura.
- **`core/config/config.go`** — `Merge` propaga erro de `MergeStructs`. Sem mudança.

#### Deep Dives
Estratégia: validar `reflect.TypeOf(dst).Elem() == reflect.TypeOf(src)` (ou `src.Elem()` se ptr) ANTES de chamar `mergeStruct`. Se mismatch → erro tipado. Manter `recover` apenas como guarda última, mas com `%w`:

```go
type TypeMismatchError struct{ Want, Got reflect.Type }
func (e TypeMismatchError) Error() string { return fmt.Sprintf("type mismatch: want %v, got %v", e.Want, e.Got) }

func MergeStructs(dst interface{}, srcs ...interface{}) (err error) {
    defer func() {
        if r := recover(); r != nil {
            if e, ok := r.(error); ok {
                err = fmt.Errorf("merge panic: %w", e)
            } else {
                err = fmt.Errorf("merge panic: %v", r)
            }
        }
    }()
    dstType := reflect.TypeOf(dst).Elem()
    for _, src := range srcs {
        if src == nil { continue }
        srcType := reflect.TypeOf(src)
        if srcType.Kind() == reflect.Ptr { srcType = srcType.Elem() }
        if dstType != srcType {
            return TypeMismatchError{Want: dstType, Got: srcType}
        }
        mergeStruct(dst, src)
    }
    return nil
}
```

#### Tasks
1. Adicionar tipo `TypeMismatchError` em `internal/utils/merge.go`.
2. Adicionar pre-validation no início de `MergeStructs`.
3. Manter `recover` como guarda mas tornar `%w`-friendly.
4. Adicionar testes.

#### TDD
```
RED:     TestMergeStructs_TypeMismatch_ReturnsTypedError — merge de structs diferentes → erro do tipo TypeMismatchError
RED:     TestMergeStructs_NilSrc_NoOp — merge com nil → ok
RED:     TestMergeStructs_HappyPath_PreservesBehavior — config.Config merge existente continua funcionando
GREEN:   Implementar pre-validation.
REFACTOR: Extrair erro para arquivo.
VERIFY:  go test ./internal/utils/
```

#### Acceptance Criteria
- [ ] Erro de type mismatch retorna `TypeMismatchError` (não string genérico)
- [ ] `errors.Is`/`errors.As` funcionam contra `TypeMismatchError`
- [ ] Comportamento existente de merge correto preservado

#### DoD
- [ ] T2.1.{1..4} completos
- [ ] Testes verdes
- [ ] Nenhum `%v` em path de erro normal de `merge.go`

---

### T2.2 — Binário `theopacks-generate` roda como non-root (cobre H3)

#### Objective
`Dockerfile.generate` cria usuário `theopacks` UID 10001 e termina com `USER theopacks`.

#### Evidence
`Dockerfile.generate:18-24` sem `USER`. Argo executa o contêiner como root. C3/C4 ganham blast radius.

#### Files to edit
```
Dockerfile.generate — adicionar useradd + USER theopacks
```

#### Deep file dependency analysis
- **`Dockerfile.generate`** — imagem `theo-packs-runner` consumida pela Argo Workflow do Theo.
- Downstream: Argo monta `/workspace` no contêiner. Permissão precisa ser legível por UID 10001.

#### Deep Dives
Alpine usa `adduser` (não `useradd`):
```dockerfile
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10001 theopacks
COPY --from=build /theopacks-generate /usr/local/bin/theopacks-generate
USER theopacks
ENTRYPOINT ["theopacks-generate"]
```

Permissões do binário: `COPY` herda owner root. UID 10001 só precisa de execute, que é default 0755.

Workspace mount: Theo precisa garantir `--read-only` ou `--user 10001` no Argo step (coordenação fora deste plano, mas documentar).

#### Tasks
1. Editar `Dockerfile.generate` adicionando `adduser` e `USER theopacks`.
2. Validar smoke test `docker run theo-packs-runner --help` continua funcionando.
3. Documentar em CLI contract: workspace mount precisa ser legível por UID 10001.

#### TDD
```
RED:     TestDockerfileGenerate_HasUserDirective — grep "^USER " Dockerfile.generate → match
RED:     TestDockerfileGenerate_RunsNonRoot (CI integration) — docker run ... -u 0 não, default user → uid != 0
GREEN:   Editar Dockerfile.
REFACTOR: None expected.
VERIFY:  docker build -f Dockerfile.generate . && docker run --rm <image> theopacks-generate --help (sai 0 ou 2 sem error de FS)
```

#### Acceptance Criteria
- [ ] `grep -c "^USER " Dockerfile.generate` ≥ 1
- [ ] `docker run --entrypoint id <image>` reporta `uid=10001`
- [ ] `--help` funciona

#### DoD
- [ ] Dockerfile editado
- [ ] Build manual verde
- [ ] CLI contract doc atualizado

---

### T2.3 — Tagged version de `tailscale/hujson` (cobre H4)

#### Objective
Substituir pseudo-version `v0.0.0-20260302212456-ecc657c15afd` por tag estável; ou — se não houver tag — documentar com ADR a aceitação consciente.

#### Evidence
`go.mod` linha 8: `github.com/tailscale/hujson v0.0.0-20260302212456-ecc657c15afd`. Pseudo-version. Viola critério "Último release < 6 meses" da seção 9 do CLAUDE global.

#### Files to edit
```
go.mod — atualizar versão se tag existir
go.sum — regenerar via go mod tidy
docs/contracts/theo-packs-cli-contract.md (ou ADR novo) — se manter pseudo-version, documentar razão
```

#### Deep file dependency analysis
- **`go.mod`** — direct dep do `core/`. Usado em `internal/utils/utils.go` (`StandardizeJSON`).
- Downstream: parser JSONC para `theopacks.json`.

#### Deep Dives
- Verificar `https://github.com/tailscale/hujson/tags` para tag estável recente.
- Se não houver: documentar em ADR (`docs/adrs/use-tailscale-hujson-pseudo-version.md`) com:
  - Por que essa dependência (JSONC com comentários — sem alternativa madura)
  - Por que pseudo-version aceitável (commit auditado, repo Tailscale com track record)
  - Quando reavaliar

#### Tasks
1. `go list -m -versions github.com/tailscale/hujson` para listar versões disponíveis.
2. Se houver tag SemVer: `go get github.com/tailscale/hujson@<tag>` e `mise run tidy`.
3. Se não: escrever ADR justificando.

#### TDD
```
RED:     N/A (mudança de dependência não tem teste comportamental, depende de output do go list)
GREEN:   Atualizar go.mod OU escrever ADR.
REFACTOR: None expected.
VERIFY:  mise run test && mise run check
```

#### Acceptance Criteria
- [ ] `go.mod` tem versão tagged OR ADR existe justificando pseudo-version
- [ ] `mise run test` verde após mudança

#### DoD
- [ ] Decisão documentada
- [ ] Testes verdes

---

### T2.4 — Allowlist para `THEOPACKS_CONFIG_FILE` (cobre H2)

#### Objective
Restringir `THEOPACKS_CONFIG_FILE` a arquivos com extensão `.json`/`.jsonc` dentro de `app.Source` (após clamp T1.2). Reject paths absolutos.

#### Evidence
`core/core.go:154` lê `configFileName` de env sem validar. Combinado com C4, permitia ler qualquer arquivo dentro do source. Agora protegido por clamp (T1.2), mas adicionar allowlist é defesa adicional.

#### Files to edit
```
core/core.go — validar extensão de configFileName antes de stat/read
core/core_test.go — TestGenerateConfigFromFile_RejectsBadExtension
```

#### Deep file dependency analysis
- **`core/core.go:147-167`** — `GenerateConfigFromFile`. Mudança: após resolver `configFileName`, validar `filepath.Ext(configFileName) in {".json", ".jsonc"}`.

#### Deep Dives
```go
ext := strings.ToLower(filepath.Ext(configFileName))
if ext != ".json" && ext != ".jsonc" {
    return nil, fmt.Errorf("config file %q has unsupported extension %q (expected .json or .jsonc)", configFileName, ext)
}
```

Edge: `filepath.IsAbs(configFileName)` → reject (não permitir absolute path via env).

#### Tasks
1. Adicionar validação de extensão em `core/core.go:147`.
2. Reject `filepath.IsAbs(configFileName)`.
3. Adicionar testes.

#### TDD
```
RED:     TestGenerateConfigFromFile_RejectsBadExtension — configFileName="evil.sh" → erro
RED:     TestGenerateConfigFromFile_RejectsAbsolutePath — configFileName="/etc/passwd" → erro
RED:     TestGenerateConfigFromFile_AcceptsJsonc — configFileName="custom.jsonc" → ok (se arquivo existir)
GREEN:   Adicionar validações.
REFACTOR: None expected.
VERIFY:  go test ./core/ -run TestGenerateConfigFromFile
```

#### Acceptance Criteria
- [ ] Path absoluto rejeitado
- [ ] Extensão fora de `.json`/`.jsonc` rejeitada
- [ ] `theopacks.json` default continua funcionando

#### DoD
- [ ] T2.4.{1..3} completos
- [ ] Testes verdes

---

## Phase 3: Code Quality

**Objective:** corrigir code smells e violações de regra do CLAUDE.

### T3.1 — Substituir env-var bridge por `WorkspaceTarget` em options (cobre M3)

#### Objective
Adicionar campo `WorkspaceTarget` em `GenerateBuildPlanOptions`. CLI passa via campo, não via env vars sintéticas.

#### Evidence
`cmd/theopacks-generate/main.go:98-104` fabrica env vars para comunicar com provider. Causa risco residual de C1 (env vars internas confundidas com secrets) e é acoplamento por string.

#### Files to edit
```
core/core.go — adicionar tipo WorkspaceTarget e campo em GenerateBuildPlanOptions
core/providers/node/node.go — ler de ctx.Options.WorkspaceTarget (fallback para env)
cmd/theopacks-generate/main.go — passar via Options em vez de env vars
core/core_test.go — testes
core/providers/node/node_test.go — testes
```

#### Deep file dependency analysis
- **`core/core.go`** — define `GenerateBuildPlanOptions`. Adicionar campo é backward compat.
- **`core/providers/node/node.go`** — lê env hoje (`ctx.Env.GetConfigVariable("APP_NAME")`). Refactor: ler de options se presente, fallback env.
- **`core/generate/context.go`** — precisa expor `Options` no `GenerateContext` (hoje não expõe).
- **`cmd/theopacks-generate/main.go`** — passa via options.

#### Deep Dives
```go
// core/core.go
type WorkspaceTarget struct {
    AppName string
    AppPath string
}

type GenerateBuildPlanOptions struct {
    // ...existing fields
    WorkspaceTarget *WorkspaceTarget
}
```

```go
// core/generate/context.go
type GenerateContext struct {
    // ...existing
    WorkspaceTarget *WorkspaceTarget // populated from options
}
```

```go
// core/providers/node/node.go — lookup helper
func resolveWorkspaceTarget(ctx *generate.GenerateContext) (name, path string) {
    if ctx.WorkspaceTarget != nil {
        return ctx.WorkspaceTarget.AppName, ctx.WorkspaceTarget.AppPath
    }
    name, _ = ctx.Env.GetConfigVariable("APP_NAME")
    path, _ = ctx.Env.GetConfigVariable("APP_PATH")
    return
}
```

#### Tasks
1. Definir `WorkspaceTarget` em `core/core.go`.
2. Expor em `GenerateContext` (`core/generate/context.go`).
3. Refactor `core/providers/node/node.go` para usar helper.
4. CLI passa via options.
5. Marcar env vars `THEOPACKS_APP_NAME`/`PATH` como deprecated em CHANGELOG mas continuar lendo.

#### TDD
```
RED:     TestNodeProvider_UsesWorkspaceTargetFromOptions — options.WorkspaceTarget setado → comando usa esses valores
RED:     TestNodeProvider_FallsBackToEnv — sem options, com env → continua funcionando
GREEN:   Implementar.
REFACTOR: None expected.
VERIFY:  go test ./core/... ./cmd/...
```

#### Acceptance Criteria
- [ ] CLI passa via options, não env vars
- [ ] Backward compat preservado (env vars ainda lidas)
- [ ] Testes verdes

#### DoD
- [ ] T3.1.{1..5} completos
- [ ] Testes verdes
- [ ] CHANGELOG atualiza Deprecated

---

### T3.2 — Padronizar error wrapping com `%w` (cobre M8)

#### Objective
Substituir `%v` por `%w` e adicionar contexto em todos os bare `return err`.

#### Evidence
- `internal/utils/merge.go:14`: `%v` (tratado em T2.1)
- `core/app/app.go:153,158,174,187`: bare `return err`
- `core/providers/python/python.go:64`: bare `return err`
- `core/core.go:47,52`: bare `return err`
- `core/plan/step.go:43,51`: bare `return err`
- `core/config/config.go:81`: bare `return err`

#### Files to edit
```
core/app/app.go — wrap erros de ReadJSON/ReadYAML/ReadTOML
core/providers/python/python.go — wrap erro do planner principal
core/core.go — wrap erro de readConfigJSON
core/plan/step.go — wrap erro de UnmarshalJSON
core/config/config.go — wrap erro de UnmarshalJSON
```

#### Deep file dependency analysis
- Cada file tem `return err` em pontos onde context (file path, op) é conhecido.
- Mudança puramente local; não muda assinatura de função.

#### Deep Dives
Padrão a aplicar:
```go
// antes:
if err := json.Unmarshal(...); err != nil {
    return err
}
// depois:
if err := json.Unmarshal(...); err != nil {
    return fmt.Errorf("unmarshal step: %w", err)
}
```

#### Tasks
1. Sweeper: rodar `grep -rn "return err$" --include="*.go" core/ cmd/ internal/ | grep -v _test`
2. Para cada hit, adicionar contexto local.
3. Confirmar `errors.Is`/`errors.As` continuam funcionando.

#### TDD
```
RED:     TestErrorContext_PreservesChain — errors.Is(err, io.EOF) ainda funciona depois do wrap
RED:     TestErrorContext_HasDescription — err.Error() contém nome da função/arquivo
GREEN:   Aplicar wrapping.
REFACTOR: None expected.
VERIFY:  go test ./...
```

#### Acceptance Criteria
- [ ] `grep -rn "return err$" --include="*.go" core/ cmd/ internal/ | grep -v _test` retorna vazio
- [ ] `grep -rn "fmt.Errorf.*%v" --include="*.go" core/ cmd/ internal/ | grep -v _test` retorna vazio em paths de erro normais

#### DoD
- [ ] Todos os hits resolvidos
- [ ] Testes verdes

---

### T3.3 — Eliminar variadic `log ...*logger.Logger` (cobre M5)

#### Objective
Substituir parâmetro variadic por `*logger.Logger` explícito não-nil. Chamadores que não querem logger passam `logger.Nop()` (novo helper).

#### Evidence
`core/providers/golang/golang.go:148`, `core/providers/node/workspace.go:38` usam variadic 0-1 logger. Smell de "evitar nil check". Viola DIP (Rule 13.5) e ISP.

#### Files to edit
```
core/logger/logger.go — adicionar Nop() helper
core/providers/golang/golang.go — assinatura sem variadic
core/providers/node/workspace.go — assinatura sem variadic
core/providers/golang/golang_test.go — atualizar
core/providers/node/workspace_test.go — atualizar
```

#### Deep file dependency analysis
- **`logger/logger.go`** — adicionar `func Nop() *Logger { return NewLogger() }` ou similar.
- Tests existentes precisam atualizar chamadas.

#### Deep Dives
```go
// logger.go
// Nop returns a logger that discards messages — used in tests and contexts
// where logging would be noise but the API requires a non-nil logger.
func Nop() *Logger { return &Logger{Logs: nil} }
```

#### Tasks
1. Adicionar `Nop()` em `core/logger/logger.go`.
2. Trocar `log ...*logger.Logger` por `log *logger.Logger` em `parseGoWork` e `DetectWorkspace`.
3. Atualizar callers.

#### TDD
```
RED:     TestParseGoWork_AcceptsNopLogger — passar logger.Nop() → função funciona
RED:     TestDetectWorkspace_AcceptsNopLogger — idem
GREEN:   Refactor + atualizar callers.
REFACTOR: None expected.
VERIFY:  go test ./core/...
```

#### Acceptance Criteria
- [ ] Nenhum `log ...*logger.Logger` em production code
- [ ] `logger.Nop()` documentado

#### DoD
- [ ] T3.3.{1..3} completos
- [ ] Testes verdes

---

### T3.4 — Substituir `shellEscape` custom por `strconv.Quote` analog (cobre M7)

#### Objective
Avaliar substituir `shellEscape` em `core/dockerfile/generate.go` por implementação baseada em primitivas stdlib (`strconv.Quote` para Go literal, ou função explícita comentada para shell).

#### Evidence
`shellEscape` reescreve quoting que existe em libs. Cheiro: "não reinvente a roda" (seção 9 CLAUDE global).

#### Files to edit
```
core/dockerfile/generate.go — avaliar swap
```

#### Deep file dependency analysis
- **`generate.go`** — `shellEscape` é usado em `writeFileCommand` (`RUN printf '%%s' %s > %s`). Precisa de sh-safe quoting (single quotes).
- `strconv.Quote` produz Go literal (double quotes, Unicode escapes) — não sh-safe direto.

#### Deep Dives
Conclusão: `shellEscape` está OK porque sh-safe quoting com single quotes é simples e estável. Mas adicionar TESTE comprovando comportamento (hoje pode não ter cobertura).

#### Tasks
1. Adicionar teste `TestShellEscape_HandlesEmbeddedQuotes` cobrindo `'a'`, `a'b`, `''`, multi-line.
2. Documentar via comment que NÃO usar `strconv.Quote` (não é sh-compatível) — registrar a decisão.

#### TDD
```
RED:     TestShellEscape_SimpleString
RED:     TestShellEscape_WithSingleQuote — `a'b` → `'a'\''b'`
RED:     TestShellEscape_EmptyString
RED:     TestShellEscape_MultiLine
GREEN:   Implementação existente já passa (ou ajustar)
REFACTOR: Adicionar comentário documentando decisão
VERIFY:  go test ./core/dockerfile/
```

#### Acceptance Criteria
- [ ] Testes cobrem casos críticos de shellEscape
- [ ] Comentário documenta por que não usar stdlib

#### DoD
- [ ] T3.4.{1..2} completos
- [ ] Testes verdes

---

### T3.5 — Limpar redundâncias e nits LOW (L2, L3, L5)

#### Objective
Endereçar findings LOW restantes:
- L2: `/app` hardcoded em `COPY --from`
- L3: `filepath.Abs(filepath.Join(currentDir, path))` redundante
- L5: `logger.Logs` sem cap

#### Evidence
- `core/dockerfile/generate.go` — `/app /app` literal
- `core/app/app.go:33`
- `core/logger/logger.go` — sem limite

#### Files to edit
```
core/app/app.go — simplificar NewApp
core/logger/logger.go — adicionar cap soft (e.g. 1000 msgs, descartar oldest)
core/dockerfile/generate.go — extrair constant DefaultWorkdir = "/app"
```

#### Deep file dependency analysis
- L2: trocar literais por `const DefaultWorkdir = "/app"`. Sem mudança de comportamento.
- L3: `filepath.Abs(path)` resolve relative-to-cwd automaticamente.
- L5: log cap evita memory exhaustion em planos patológicos.

#### Tasks
1. L3: `core/app/app.go:33` substituir.
2. L5: adicionar `maxLogs = 1000` em logger; truncate quando excede com aviso.
3. L2: extrair constante.

#### TDD
```
RED:     TestNewApp_RelativePathResolves — TestNewApp("./foo") = TestNewApp("$PWD/foo")
RED:     TestLogger_CapsAtMaxLogs — adicionar 1500 msgs → len(Logs) == 1000, primeira msg avisa truncamento
GREEN:   Implementar
REFACTOR: None expected
VERIFY:  go test ./core/app/ ./core/logger/ ./core/dockerfile/
```

#### Acceptance Criteria
- [ ] Cada finding LOW endereçado com teste
- [ ] Constante `DefaultWorkdir` usada em todos os pontos de hardcoded `/app`

#### DoD
- [ ] T3.5.{1..3} completos
- [ ] Testes verdes

---

## Phase 4: Polish

**Objective:** finalização, fixture broken, padronização de exit codes.

### T4.1 — Padronizar exit codes da CLI (cobre M4)

#### Objective
Definir constantes em `cmd/theopacks-generate/main.go` para exit codes:
- 0 = success
- 1 = generic failure (provider detection failed, write failed)
- 2 = input invariant violated (bad flag, user Dockerfile, traversal)

#### Evidence
Atual: misturado `log.Fatal*` (exit 1) e `os.Exit(1)` e `os.Exit(2)` sem documentação.

#### Files to edit
```
cmd/theopacks-generate/main.go — usar constantes
cmd/theopacks-generate/main_test.go — verificar exit codes em casos de erro
```

#### Deep file dependency analysis
- Caller (Theo API) precisa diferenciar input invariant (não retentar) de generic failure (pode retentar).
- Documentar em CLI contract.

#### Deep Dives
```go
const (
    exitSuccess          = 0
    exitGenericFailure   = 1
    exitInputInvariant   = 2
)
```

Substituir `log.Fatal*` por `fmt.Fprintf(os.Stderr, ...); os.Exit(exitX)` para controle granular.

#### Tasks
1. Definir constantes.
2. Audit cada `log.Fatal*` / `os.Exit` e atribuir código apropriado.
3. Documentar em CLI contract.

#### TDD
```
RED:     TestExitCode_NoOutput_GenericFailure — sem --output → exit 1 (faltando arg)
RED:     TestExitCode_TraversalAppPath_InputInvariant — --app-path ../../etc → exit 2
RED:     TestExitCode_BadProviderDetection_GenericFailure → exit 1
GREEN:   Padronizar
REFACTOR: None expected
VERIFY:  go test ./cmd/... -run TestExitCode
```

#### Acceptance Criteria
- [ ] Exit codes documentados em CLI contract
- [ ] Cada path de erro tem exit code apropriado
- [ ] Testes cobrem todos os exit codes

#### DoD
- [ ] T4.1.{1..3} completos
- [ ] Testes verdes
- [ ] Doc atualizado

---

### T4.2 — Corrigir fixture `node-express` lockfile (cobre L6)

#### Objective
Regenerar `examples/node-express/package-lock.json` para sincronizar com `package.json`.

#### Evidence
Build E2E falha: "Missing: express@4.18.0 from lock file". Bloqueia teste e2e do exemplo.

#### Files to edit
```
examples/node-express/package-lock.json — regenerar via npm install
```

#### Deep file dependency analysis
- E2E test `e2e/e2e_test.go` (build tag `e2e`) deve passar para `node-express`.

#### Tasks
1. `cd examples/node-express && rm package-lock.json && npm install --package-lock-only`
2. Validar `mise run test-e2e` para o exemplo passa.

#### TDD
```
RED:     TestE2E_NodeExpress_Builds (tag e2e) — falha atual
GREEN:   Regenerar lockfile
REFACTOR: None expected
VERIFY:  go test -tags e2e ./e2e/ -run TestE2E/node-express
```

#### Acceptance Criteria
- [ ] E2E test para node-express passa
- [ ] `package.json` e `package-lock.json` sincronizados

#### DoD
- [ ] Lockfile regenerado
- [ ] E2E verde

---

### T4.3 — Cortar CHANGELOG `[Unreleased]` para `[0.5.0]`

#### Objective
Após todas as fases anteriores aplicadas, cortar `[Unreleased]` em release `[0.5.0]` com data e marcar como Security release.

#### Evidence
Plano todo trata de segurança + correção de comportamento. Padrão de versionamento SemVer indica MINOR bump (não-breaking? mas há breaking change em user-Dockerfile contract → MAJOR? Decisão: como 0.x.y, MINOR aceita breaking).

#### Files to edit
```
CHANGELOG.md — cortar release
```

#### Tasks
1. Mover entries de `[Unreleased]` para `[0.5.0] - YYYY-MM-DD`.
2. Adicionar nova seção `[Unreleased]` vazia no topo.
3. Adicionar seção `Security` com todos os CVE-class findings desta entrega.

#### TDD
N/A (doc).

#### Acceptance Criteria
- [ ] `[0.5.0]` existe com data
- [ ] `[Unreleased]` vazia preparada para próxima iteração
- [ ] Seção `Security` lista C2, C3, C4, C1-residual

#### DoD
- [ ] CHANGELOG cortado
- [ ] PR description menciona security release

---

## Coverage Matrix

| # | Finding (severidade) | Task(s) | Resolução |
|---|---|---|---|
| C1 (CRITICAL residual) | secret pollution residual | T1.4 | Filtra `THEOPACKS_*` internos antes de promover a plan-level secrets |
| C2 (CRITICAL) | shell injection appPath/appName | T1.1 | Regex restritiva na fronteira CLI rejeita chars maliciosos |
| C3 (CRITICAL) | symlink follow | T1.3 | `os.Lstat` + ModeSymlink check, reject explícito |
| C4 (CRITICAL) | path traversal | T1.2 | `clampPath` com `filepath.Abs` + `HasPrefix` |
| H1 (HIGH) | recover() engolidor | T2.1 | Pre-validation tipada + `%w` |
| H2 (HIGH) | config-file allowlist | T2.4 | Validar extensão + reject paths absolutos |
| H3 (HIGH) | runs as root | T2.2 | `USER theopacks` UID 10001 |
| H4 (HIGH) | pseudo-version hujson | T2.3 | Tag estável OR ADR justificando |
| M1 (MEDIUM) | //Force 1 comment | T0.1 | Remoção |
| M2 (MEDIUM) | main.go SRP | T1.1+T1.2+T1.3 (split em validate.go, path.go) | Extração de módulos torna main.go menor |
| M3 (MEDIUM parcial) | env var bridge coupling | T3.1 | `WorkspaceTarget` em options |
| M4 (MEDIUM) | exit code inconsistency | T4.1 | Constantes documentadas |
| M5 (MEDIUM) | variadic logger | T3.3 | `*logger.Logger` + `logger.Nop()` |
| M7 (MEDIUM) | shellEscape custom | T3.4 | Documenta decisão + adiciona testes |
| M8 (MEDIUM) | erros sem %w | T3.2 | Sweeper de wrapping |
| M9 (MEDIUM) | cd X && Y pattern | T3.1 (parcial via WorkspaceTarget) + manter como follow-up no provider | Refactor maior — fora deste plano se complexo |
| M10 (MEDIUM) | CHANGELOG vazia | T0.3 + T4.3 | Popular [Unreleased] e cortar [0.5.0] |
| L1 (LOW) | (já corrigido externamente) | — | n/a |
| L2 (LOW) | /app hardcoded | T3.5 | `const DefaultWorkdir` |
| L3 (LOW) | filepath.Abs redundância | T3.5 | Simplificação |
| L4 (LOW) | sanitização gap | T1.1 | Coberto pela regex |
| L5 (LOW) | logs unbounded | T3.5 | Cap soft de 1000 |
| L6 (LOW) | fixture quebrada | T4.2 | Regenerar lockfile |
| L7 (LOW) | CLAUDE.md outdated | T0.2 | Reescrita |

**Coverage: 23/23 findings cobertos (100%)** — M9 marcado parcial (refactor cd→WORKDIR maior fica como follow-up se T3.1 não resolver via opcoes).

## Global Definition of Done

- [ ] Todas as 4 fases completas
- [ ] `mise run test` verde (15 pacotes)
- [ ] `mise run check` zero warnings
- [ ] `go test -tags e2e ./e2e/` verde (com Docker rodando)
- [ ] Cada finding CRITICAL/HIGH tem ≥ 1 teste de regressão
- [ ] `Dockerfile.generate` build + smoke test (--help) passam como non-root
- [ ] `CHANGELOG.md` tem `[0.5.0]` cortado com seção Security
- [ ] `CLAUDE.md` projeto não referencia `railpack/` exceto em Acknowledgements
- [ ] Coverage matrix 100%
- [ ] PR description vincula a este plano e enumera mudanças breaking (user-Dockerfile contract)
