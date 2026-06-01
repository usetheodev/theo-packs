# Plan: Rename theo-packs → theokit-packs

> **Version 1.0** — Cutover único, sem backward-compatibility. Renomeia o repo, módulo Go, CLI binary, env vars, config file, docs e branding de `theo-packs` para `theokit-packs`, refletindo o re-escopamento do produto: TheoCloud passa a aceitar apenas apps gerados pelo theokit, e este pacote é a engine de detecção + geração de Dockerfile que serve esse fluxo. Outcome: 0 strings `theopacks` em código vivo, 0 imports `github.com/usetheo/theopacks`, build verde, E2E verde, consumers (TheoCloud + theokit) com plano de cutover documentado.

## Context

**Estado hoje:** `theo-packs` é um fork do Railpack (Apache 2.0, atribuição em `NOTICE`) que serve TheoCloud como engine de Dockerfile generation. Módulo Go: `github.com/usetheo/theopacks`. CLI: `theopacks-generate`. Env vars: `THEOPACKS_*`. Config: `theopacks.json`. PR #21 (mergeado 2026-06-01) consolidou o produto como "single source of truth" para Dockerfile generation.

**Mudança em andamento (confirmada pelo usuário 2026-06-01):**
1. **TheoCloud está se restringindo a aceitar apenas apps gerados pelo theokit.** Decisão tomada, theokit-packs (este pacote, renomeado) é o **bloqueador de ativação**.
2. **theokit é poliglota no backend** (Wave 2 do `theo-stacks → create-theokit` absorve Python; outros backends seguem). Logo, os 11 providers atuais permanecem relevantes, **alinhados às formas que o theokit scaffolda**, não a "qualquer projeto" arbitrário.
3. **Mesmo time, Go continua a linguagem.** Sem migração de stack.
4. **Decisão de naming confirmada:** `theokit-packs` (com hífen no nome legível; sem hífen no módulo Go conforme convenção).
5. **Sem backward-compat:** cutover seco. Sem aliases de env vars, sem binary legado, sem detection condicional de config file antigo.

**Evidência do scan (executado 2026-06-01 pré-plano):**
- 71 arquivos Go com import `github.com/usetheo/theopacks`
- 56 arquivos com literal `THEOPACKS_*`
- 21 arquivos não-código mencionam "theopacks"
- 1 example com `theopacks.json` (`examples/node-npm-with-config/`)
- 10+ golden Dockerfiles com `theopacks` no defensive header comment
- 3 workflows GitHub Actions referenciam (build-runner, ci, e2e)
- NOTICE linha 1 + .claude/project.yaml linhas 11+13

**Trabalho adjacente já planejado:**
- Blueprint `theokit-support-blueprint.md` v1.0 (verdict SHIPPABLE 96.8/100) recomenda fixes B1/B3 + regression guard. Este plano de rename é **pré-requisito**: B1/B3 devem ser aplicados sobre o código já renomeado para evitar churn de PRs.

## Objective

**"Done" =** `git grep -E 'theopacks|THEOPACKS_'` retorna 0 matches em código vivo (Go, env vars, config keys, CLI, defensive header), com build verde, todos os tests passando, E2E verde, e documentação coerente refletindo o novo nome + escopo.

Goals mensuráveis:
- [ ] `go build ./...` verde após rename
- [ ] `mise run test` (unit + cmd) verde
- [ ] `mise run test-e2e` verde
- [ ] `mise run check` (vet + fmt + golangci-lint) zero warnings
- [ ] `git grep "theopacks" -- '*.go' '*.md' '*.yml' '*.yaml' '*.toml' '*.json' '*.dockerfile'` retorna 0 (excluindo `docs/plans/PR_DESCRIPTION_*.md` históricos e `NOTICE` § Railpack attribution)
- [ ] `git grep "THEOPACKS_"` retorna 0
- [ ] Repo GitHub renomeado (último step)
- [ ] TheoCloud + theokit consumers atualizados

## ADRs

### D1 — Cutover seco, sem backward-compatibility

**Decisão:** Sem aliases de env vars (THEOPACKS_* não continuam funcionando), sem binary legado `theopacks-generate` (substituído por `theokit-packs-generate` direto), sem fallback de leitura de `theopacks.json` quando `theokit-packs.json` ausente. Cutover único.

**Rationale:** Usuário confirmou explicitamente ("não precisamos manter compatibilidade"). Compat introduce dead-code paths que precisam de testes próprios, expira aleatoriamente, e força mantenedor a lembrar de removê-la depois. KISS + YAGNI. Os consumers reais (TheoCloud, theokit) são internos e podem ser coordenados.

**Consequências:** Coordenação cross-repo obrigatória — TheoCloud + theokit precisam de PRs próprios para repointar antes/após cutover. Documentado em Phase 6.

### D2 — Module path `github.com/usetheo/theokitpacks` (sem hífen)

**Decisão:** Caminho Go module **sem hífen** entre "theokit" e "packs". Repo, CLI binary e docs **com hífen**.

**Rationale:** Convenção Go strongly discoraja hífens em paths de módulo (alguns tools tratam mal). O módulo atual segue essa convenção: `github.com/usetheo/theopacks`. Manter o precedente. Nome legível (`theokit-packs`) usa hífen onde Go não impõe restrição.

**Consequências:** Mismatch nominal pequeno (módulo `theokitpacks`, nome `theokit-packs`). Documentado em README + CHANGELOG. Não causa confusão na prática — usuário interage com o binário/docs (hífen), não com import path.

### D3 — Env vars `THEOKIT_PACKS_*` (com underscore duplo)

**Decisão:** Prefix `THEOKIT_PACKS_` (e.g., `THEOKIT_PACKS_START_CMD`, `THEOKIT_PACKS_APP_NAME`), **não** `THEOKIT_*` ou `TKPACKS_*`.

**Rationale:** O theokit (CLI sibling) pode usar `THEOKIT_*` para suas próprias env vars. Conflito de prefix introduziria race entre "este é setting do CLI theokit ou do builder?". `THEOKIT_PACKS_*` é unambiguo. `TKPACKS_*` é compacto mas alien — usuário não sabe o que significa. Verbosidade ganha clareza aqui.

**Consequências:** Strings longas em testes e configs. Aceitável.

### D4 — Manter NOTICE + atribuição Railpack intacta

**Decisão:** A seção "This product is derived from Railpack" do `NOTICE` é **inalterada**. Apenas a primeira linha (`theo-packs`) vira `theokit-packs`.

**Rationale:** Apache 2.0 § 4(c)(i) exige atribuição preservada. Rename do produto **não** apaga origem. Honestidade.

**Consequências:** Nenhuma. NOTICE continua válido e auditable.

### D5 — Planos antigos em `docs/plans/PR_DESCRIPTION_*.md` permanecem como histórico

**Decisão:** Não renomear nem reescrever `PR_DESCRIPTION_*.md`, `single-source-of-truth-*.md`, `monorepo-contract-validation-plan.md`, etc. São snapshots históricos. Adicionar nota no início de `README.md` ou `CHANGELOG.md` explicando o rename para reader futuro.

**Rationale:** Editar planos passados reescreve a história. Eles são audit trail. KISS — deixar quietos.

**Consequências:** `git grep theopacks` em docs/plans/ continua matching, mas é histórico esperado. O check do "Done" exclui esse dir.

### D6 — Atualização do golden header comment é regeneração via UPDATE_GOLDEN

**Decisão:** Os ~10 golden Dockerfiles em `core/dockerfile/testdata/` que mencionam `theo-packs:` (defensive header) são regenerados via `UPDATE_GOLDEN=true go test ./core/dockerfile/...`, não editados à mão. Rule 7 do CLAUDE.md do projeto.

**Rationale:** Goldens são output de código; alteração manual quebra o invariante. O código que gera o header (em `core/dockerfile/generate.go` ou `core/dockerfile/HeaderComment`) é atualizado primeiro, golden é regenerado depois.

**Consequências:** Sequência obrigatória: atualizar geradora → run UPDATE_GOLDEN → verificar diff exato → commit.

### D7 — Repo GitHub é renomeado por último

**Decisão:** Phase 6 (repo rename + remote update) acontece **após** todas as outras fases mergeadas em `develop` e validadas. Não primeiro.

**Rationale:** GitHub redireciona old URLs por algum tempo, mas mover repo durante PR cycle confunde reviewers. Cutover atomic: develop sincronizado, depois rename, depois consumers atualizam refs.

**Consequências:** Phases 1-5 vivem no repo com nome antigo (`theo-packs`) temporariamente, com código já renomeado dentro. Esse limbo dura poucos dias.

## Dependency Graph

```
Phase 0 (audit + cutover window)
   │
   ▼
Phase 1 (Go module rename) ◀── foundational
   │
   ├──▶ Phase 2 (CLI rename + Dockerfile.generate + workflows)
   │
   ├──▶ Phase 3 (env vars rename) ◀── pode paralelizar com 2
   │
   └──▶ Phase 4 (config file rename) ◀── pode paralelizar com 2,3
              │
              ▼
        Phase 5 (docs + branding + goldens) ◀── depende de 2,3,4
              │
              ▼
        Phase 6 (repo rename + consumer cutover)
```

**Paralelizáveis:** Phases 2, 3, 4 após Phase 1. **Sequential blockers:** Phase 1 (foundational), Phase 5 (depende de tudo), Phase 6 (último).

---

## Phase 0: Audit + cutover window

**Objective:** Pré-requisitos não-codáveis: confirmar lista exata de strings e janela de coordenação com TheoCloud.

### T0.1 — Confirmar audit de strings via scan automatizado

#### Objective
Produzir contagem ground-truth de `theopacks` / `THEOPACKS_` por categoria de arquivo, salvar como referência para Phase 5 checks de "Done".

#### Evidence
Scan executado pré-plano contou 71 imports + 56 env vars + 21 docs. Esses números são baseline; após Phase 5 devem zerar (exceto históricos em `docs/plans/`).

#### Files to edit
```
docs/plans/rename-to-theokit-packs-audit.md (NEW) — relatório de contagem inicial + esperado final
```

#### Deep file dependency analysis
- **Arquivo novo, isolado.** Não impacta build. Serve como appendix do plano + checklist.

#### Tasks
1. Rodar `git grep -c "theopacks" -- '*.go'` e salvar contagem
2. Rodar `git grep -c "THEOPACKS_"` e salvar contagem por extensão
3. Rodar `git grep "github.com/usetheo/theopacks" --files-with-matches | wc -l`
4. Listar paths uniquees em `docs/plans/rename-to-theokit-packs-audit.md`
5. Marcar com `[expected after rename: 0]` cada categoria

#### TDD
```
RED:     N/A — relatório, não código
GREEN:   Escrever o relatório
REFACTOR: None expected
VERIFY:  cat docs/plans/rename-to-theokit-packs-audit.md
```

#### Acceptance Criteria
- [ ] Arquivo `docs/plans/rename-to-theokit-packs-audit.md` existe com 4 seções: imports Go, env vars, docs/configs, goldens
- [ ] Cada seção tem contagem inicial + "expected: 0"
- [ ] Lista paths uniquees em cada categoria

#### DoD
- [ ] Relatório commitado em `develop`
- [ ] Linkado no header deste plano

---

### T0.2 — Coordenar janela de cutover com TheoCloud e theokit

#### Objective
Confirmar com mantenedor do TheoCloud e do theokit quando o cutover acontece (cutover window) e quem atualiza qual consumer. Não é code task, é coordenação.

#### Evidence
TheoCloud usa `theopacks-generate` binário em Argo Workflow steps. theokit pode ter referência em scripts ou docs. Sem coordenação, rename do binário quebra a pipeline.

#### Files to edit
```
docs/plans/rename-to-theokit-packs-cutover.md (NEW) — lista de consumers + owner + status
```

#### Deep file dependency analysis
- **Arquivo de planejamento.** Não impacta build. Documenta dependências externas.

#### Tasks
1. Listar consumers conhecidos: TheoCloud (pipeline Argo), theokit (docs/scripts?), outros internos
2. Para cada consumer: nome, repo, owner contact, status (NOT_NOTIFIED / NOTIFIED / READY / DONE)
3. Definir cutover window (data + hora) — input externo
4. Definir ordem de operações: (a) PRs Phases 1-5 mergeados em theo-packs develop; (b) consumers preparam PRs apontando para novo binary/módulo; (c) Phase 6 executa rename; (d) consumers mergeiam

#### TDD
```
RED:     N/A — documento
GREEN:   Escrever cutover plan
REFACTOR: None expected
VERIFY:  cat docs/plans/rename-to-theokit-packs-cutover.md
```

#### Acceptance Criteria
- [ ] Documento lista pelo menos 2 consumers (TheoCloud, theokit) com owner
- [ ] Window definida (ou marcada TBD com responsável)
- [ ] Ordem de operações registrada

#### DoD
- [ ] Documento commitado
- [ ] Owner de cada consumer notificado (ou registrado quem notifica)

---

## Phase 1: Go module rename

**Objective:** Renomear `github.com/usetheo/theopacks` → `github.com/usetheo/theokitpacks` em `go.mod` e todos os 71 imports. Build verde sem qualquer mudança funcional.

### T1.1 — Atualizar `go.mod` module declaration

#### Objective
Trocar a primeira linha de `go.mod` para o novo module path.

#### Evidence
`head -1 go.mod` retorna `module github.com/usetheo/theopacks`. Esse é o singular source-of-truth do path.

#### Files to edit
```
go.mod — line 1: module github.com/usetheo/theopacks → github.com/usetheo/theokitpacks
```

#### Deep file dependency analysis
- **`go.mod`** declara o módulo. Toda a árvore de imports nos `.go` arquivos abaixo se resolve relativa a esse path. Mudança aqui sem mudança nos imports = `go build` quebra com "package not found".

#### Tasks
1. Editar `go.mod` line 1
2. Rodar `go mod tidy` (vai falhar se imports não tiverem sido atualizados — esperado, fica para T1.2)

#### TDD
```
RED:     go build ./... falha com "imports github.com/usetheo/theopacks/... no required module provides package"
GREEN:   T1.2 atualiza imports → build passa
REFACTOR: None expected
VERIFY:  head -1 go.mod | grep -q "theokitpacks"
```

#### Acceptance Criteria
- [ ] `go.mod` line 1 = `module github.com/usetheo/theokitpacks`
- [ ] (Build quebrado intencionalmente; T1.2 conserta)

#### DoD
- [ ] Commit isolado: "chore(module): rename module path to theokitpacks (build broken until T1.2)"

---

### T1.2 — Replace imports em todos os 71 arquivos Go

#### Objective
Substituir `"github.com/usetheo/theopacks"` por `"github.com/usetheo/theokitpacks"` em todos os arquivos `.go`. Build volta a verde.

#### Evidence
Scan: 71 arquivos com o import. Distribuídos em `core/`, `cmd/`, `e2e/`, `internal/`.

#### Files to edit
```
core/**/*.go — 65 files (estimativa)
cmd/**/*.go — 3 files
e2e/*.go — 1 file
internal/**/*.go — 2 files
```

Lista exata: produzida por `git grep -l "github.com/usetheo/theopacks" -- '*.go'`.

#### Deep file dependency analysis
- **Cada arquivo Go** tem um ou mais blocos `import ( ... )` com strings literais do module path.
- **Substituição mecânica via `find + sed`** ou `gofmt -r` é segura porque o path é único e não-ambíguo.
- **Imports em testes (`*_test.go`)** seguem mesma regra.
- **Downstream:** nada — esses imports são fonte de verdade para Go resolver, não há rebroadcast.

#### Tasks
1. Executar substituição mecânica:
   ```bash
   git grep -l "github.com/usetheo/theopacks" -- '*.go' | xargs sed -i 's|github.com/usetheo/theopacks|github.com/usetheo/theokitpacks|g'
   ```
2. Rodar `go mod tidy`
3. Rodar `go build ./...` — deve passar
4. Rodar `mise run check` (vet + fmt + golangci-lint)

#### TDD
```
RED:     git grep "github.com/usetheo/theopacks" -- '*.go' returns 0 hits  (asserts complete sweep)
GREEN:   Aplicar sed em batch
REFACTOR: go fmt ./... para garantir formatting consistente
VERIFY:  go build ./... && mise run check
```

#### Acceptance Criteria
- [ ] `git grep "github.com/usetheo/theopacks" -- '*.go'` retorna 0 matches
- [ ] `git grep "github.com/usetheo/theokitpacks" -- '*.go'` retorna ≥71 matches
- [ ] `go build ./...` zero erros
- [ ] `mise run check` zero warnings
- [ ] `mise run test` (unit) verde

#### DoD
- [ ] Build verde
- [ ] Tests verde
- [ ] golangci-lint zero warnings
- [ ] Commit: "refactor(module): bulk rename imports theopacks → theokitpacks"

---

## Phase 2: CLI rename + Dockerfile.generate + workflows

**Objective:** Renomear o binário `theopacks-generate` → `theokit-packs-generate` (diretório, build target, refs em Dockerfile.generate e workflows). Pode rodar em paralelo com Phase 3 e 4.

### T2.1 — Renomear diretório `cmd/theopacks-generate/` → `cmd/theokit-packs-generate/`

#### Objective
Mover o pacote do CLI para o novo path. Atualizar imports internos e go.mod build target.

#### Evidence
`ls cmd/` mostra `theopacks-generate/`. Build target em mise.toml + Dockerfile.generate apontam para esse path.

#### Files to edit
```
cmd/theopacks-generate/ → cmd/theokit-packs-generate/ (directory rename)
cmd/theokit-packs-generate/main.go — log prefixes "[theopacks]" → "[theokit-packs]"
cmd/theokit-packs-generate/main_test.go — same
```

#### Deep file dependency analysis
- **Diretório `cmd/theopacks-generate/`** é o entry point Go (`package main`). Renomeá-lo afeta:
  - `go build ./cmd/theopacks-generate` deixa de funcionar → vira `./cmd/theokit-packs-generate`
  - Refs em `Dockerfile.generate` (linha "go build -o /theopacks-generate ./cmd/theopacks-generate")
  - Refs em mise.toml (atualmente não mencionado diretamente, mas check ainda)
- **`main.go`** loga prefixo `[theopacks]` em ~6-8 lugares (Fprintf em stderr/stdout). User-facing — deve refletir nome novo.
- **`main_test.go`** assert prefixos de log e error messages — atualizar.

#### Tasks
1. `git mv cmd/theopacks-generate cmd/theokit-packs-generate`
2. Atualizar `cmd/theokit-packs-generate/main.go`: replace `"[theopacks]"` → `"[theokit-packs]"` (todas as ~6-8 ocorrências)
3. Atualizar `cmd/theokit-packs-generate/main_test.go`: assertions de log prefix
4. `go build ./cmd/theokit-packs-generate` — passa
5. `go test ./cmd/theokit-packs-generate/...` — passa

#### TDD
```
RED:     go test ./cmd/theokit-packs-generate/... falha com "[theopacks]" não casa (espera "[theokit-packs]")
         OR: ls cmd/theopacks-generate retorna "not found"
GREEN:   Mv + sed em main.go + main_test.go
REFACTOR: None expected
VERIFY:  go build ./cmd/theokit-packs-generate && go test ./cmd/theokit-packs-generate/...
```

#### Acceptance Criteria
- [ ] `ls cmd/theopacks-generate` retorna "not found"
- [ ] `ls cmd/theokit-packs-generate` retorna conteúdo
- [ ] `git grep "\[theopacks\]" -- 'cmd/**/*.go'` retorna 0
- [ ] `git grep "\[theokit-packs\]" -- 'cmd/**/*.go'` retorna ≥6 matches
- [ ] Build do binário com novo path verde
- [ ] Tests CLI verdes

#### DoD
- [ ] `mise run test` verde
- [ ] `mise run check` zero warnings
- [ ] Commit: "refactor(cli): rename cmd/theopacks-generate → cmd/theokit-packs-generate"

---

### T2.2 — Atualizar `Dockerfile.generate`

#### Objective
`Dockerfile.generate` constrói o container do runner. Atualizar todas as 3 refs ao binary e o comment header.

#### Evidence
`cat Dockerfile.generate`:
- linha 1: `# theo-packs-runner — Dockerfile generator...`
- linha 6: `# Build: docker build -f Dockerfile.generate -t theo-packs-runner .`
- linha 7: `# Run:   theopacks-generate --source ...`
- linha 14: `RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /theopacks-generate ./cmd/theopacks-generate`
- linha 22: `COPY --from=build /theopacks-generate /usr/local/bin/theopacks-generate`
- linha 24: `ENTRYPOINT ["theopacks-generate"]`

#### Files to edit
```
Dockerfile.generate — 6 string occurrences updated
```

#### Deep file dependency analysis
- **Dockerfile.generate** é consumido pela pipeline TheoCloud (build do runner image). Tag esperada: `theo-packs-runner`. Após rename: `theokit-packs-runner`. TheoCloud precisa repointar (cutover doc — T0.2).

#### Tasks
1. Sed em Dockerfile.generate:
   - `s/theo-packs-runner/theokit-packs-runner/g`
   - `s|theopacks-generate|theokit-packs-generate|g`
   - `s|/theopacks-generate|/theokit-packs-generate|g`
   - `s|./cmd/theopacks-generate|./cmd/theokit-packs-generate|g`
2. Verificar: `docker build -f Dockerfile.generate -t theokit-packs-runner .` (opcional — local dev)

#### TDD
```
RED:     grep -c "theopacks" Dockerfile.generate retorna ≥6
GREEN:   Aplicar sed
REFACTOR: None expected
VERIFY:  grep -c "theopacks" Dockerfile.generate retorna 0
```

#### Acceptance Criteria
- [ ] `grep "theopacks" Dockerfile.generate` zero matches
- [ ] `grep "theokit-packs" Dockerfile.generate` ≥4 matches

#### DoD
- [ ] Commit: "chore(docker): rename Dockerfile.generate refs to theokit-packs"

---

### T2.3 — Atualizar GitHub Actions workflows

#### Objective
3 workflows (`build-runner.yml`, `ci.yml`, `e2e.yml`) mencionam `theopacks` (case-insensitive scan). Atualizar.

#### Evidence
`grep -iln "theo.pack\|theopack" .github/workflows/*.yml` retornou: `e2e.yml`, `build-runner.yml`, `ci.yml`.

#### Files to edit
```
.github/workflows/build-runner.yml
.github/workflows/ci.yml
.github/workflows/e2e.yml
```

#### Deep file dependency analysis
- **`build-runner.yml`** provavelmente builda o `Dockerfile.generate`. Image tag refletida.
- **`ci.yml`** roda tests Go — pode referenciar paths de cmd/ ou binary name em script steps.
- **`e2e.yml`** roda `mise run test-e2e` — pode referenciar binary path.

Cada workflow precisa de Read + identificação manual dos pontos a alterar (não simples sed por causa de syntax YAML).

#### Tasks
1. Read cada workflow (`.github/workflows/{build-runner,ci,e2e}.yml`)
2. Identificar refs: image tags, paths, binary names, env vars
3. Aplicar substituições contextuais (não regex cego — YAML pode quebrar)
4. Validar com `yamllint` ou `actionlint` se disponível

#### TDD
```
RED:     grep -i "theo.pack" .github/workflows/*.yml retorna ≥3 hits
GREEN:   Edit contextual em cada workflow
REFACTOR: None expected
VERIFY:  grep -i "theo.pack" .github/workflows/*.yml retorna 0  (excluindo comentários de history se houver)
```

#### Acceptance Criteria
- [ ] 3 workflows sem refs a `theopacks`
- [ ] YAML válido (parsing limpo)

#### DoD
- [ ] Commit: "chore(ci): rename workflow refs theo-packs → theokit-packs"

---

## Phase 3: Env vars rename (THEOPACKS_* → THEOKIT_PACKS_*)

**Objective:** Substituir todas as 56 ocorrências de `THEOPACKS_*` em código, tests e docs por `THEOKIT_PACKS_*`. Pode rodar em paralelo com Phase 2.

### T3.1 — Replace env var strings em código Go (produção)

#### Objective
Renomear env vars lidas em `core/app/environment.go`, providers, validators e configs.

#### Evidence
Top arquivos Go (não-test) com `THEOPACKS_`:
- `core/providers/dotnet/dotnet.go: 7`
- `core/dockerignore/templates.go` (provável)
- `core/app/environment.go` (provável — leitor de env)
- Outros providers

#### Files to edit
```
core/app/environment.go — env reader, prefix constant
core/providers/dotnet/dotnet.go
core/providers/golang/golang.go (se referência existe)
core/providers/python/python.go (se ref existe)
core/providers/node/node.go (se ref existe)
core/providers/ruby/ruby.go
core/providers/php/php.go
core/providers/rust/rust.go
core/providers/java/java.go
core/providers/deno/deno.go
core/config/config.go (CONFIG_FILE env var)
core/dockerignore/templates.go
... (qualquer .go não-test com THEOPACKS_)
```

Lista exata via: `git grep -l "THEOPACKS_" -- '*.go' ':!*_test.go'`.

#### Deep file dependency analysis
- **`core/app/environment.go`** provavelmente declara const `EnvPrefix = "THEOPACKS_"` ou similar. Mudança aqui propaga para todos os getters via `prefix + name`.
- **Providers (`core/providers/*/`)** chamam `ctx.Env.GetConfigVariable("APP_NAME")` que internamente prefixa. Se providers usam literais hardcoded `"THEOPACKS_X"` (em vez do prefix const), precisam mudar individualmente.
- **Downstream:** tests dependentes (`t.Setenv("THEOPACKS_X", ...)`) — T3.2.

#### Tasks
1. Listar arquivos exatos: `git grep -l "THEOPACKS_" -- '*.go' ':!*_test.go'`
2. Para cada arquivo, Read → identificar se usa const central ou literais
3. Replace mecânico: `sed -i 's/THEOPACKS_/THEOKIT_PACKS_/g'` (literais)
4. Atualizar const central (e.g., `EnvPrefix = "THEOKIT_PACKS_"`)
5. `go build ./...` verde

#### TDD
```
RED:     git grep "THEOPACKS_" -- '*.go' ':!*_test.go' retorna 0
GREEN:   Aplicar sed + atualizar const
REFACTOR: None expected (mecânico)
VERIFY:  go build ./... && mise run check
```

#### Acceptance Criteria
- [ ] `git grep "THEOPACKS_" -- '*.go' ':!*_test.go'` retorna 0
- [ ] `git grep "THEOKIT_PACKS_" -- '*.go' ':!*_test.go'` ≥ baseline anterior
- [ ] Build verde
- [ ] golangci-lint zero warnings

#### DoD
- [ ] `mise run check` zero warnings
- [ ] Commit: "refactor(env): rename THEOPACKS_* → THEOKIT_PACKS_* (production code)"

---

### T3.2 — Replace env var strings em tests Go

#### Objective
Atualizar `t.Setenv("THEOPACKS_...", ...)` em ~30+ test files. Sem isso, tests falham porque providers leem `THEOKIT_PACKS_*` mas tests setam o antigo.

#### Evidence
Top test files com `THEOPACKS_`:
- `core/monorepo_test.go: 40`
- `core/dockerfile/integration_test.go: 16`
- `core/core_test.go: 10`
- `e2e/e2e_test.go: 8`
- `core/providers/java/java_test.go: 8`
- `core/dogfood_test.go: 8`
- `core/app/environment_test.go: 8`
- ... (todos os `*_test.go` com THEOPACKS_)

#### Files to edit
```
core/**/*_test.go com THEOPACKS_ literal
cmd/**/*_test.go com THEOPACKS_ literal
e2e/*_test.go com THEOPACKS_ literal
```

Lista exata: `git grep -l "THEOPACKS_" -- '*_test.go'`.

#### Deep file dependency analysis
- **Tests usam `t.Setenv("THEOPACKS_X", "value")`** ou similar. Substituição mecânica.
- **Test helpers/fixtures** podem ter constantes (raro). Verificar.
- **Downstream:** nenhum — tests são folha.

#### Tasks
1. `git grep -l "THEOPACKS_" -- '*_test.go' | xargs sed -i 's/THEOPACKS_/THEOKIT_PACKS_/g'`
2. `mise run test` — verde
3. `mise run test-e2e` — verde (Docker required)

#### TDD
```
RED:     mise run test falha em N arquivos porque setenv usa nome antigo e providers leem novo
         (após T3.1 mas antes de T3.2, tests caem)
GREEN:   Aplicar sed nos tests
REFACTOR: None expected
VERIFY:  mise run test && mise run test-e2e
```

#### Acceptance Criteria
- [ ] `git grep "THEOPACKS_" -- '*_test.go'` retorna 0
- [ ] `mise run test` verde
- [ ] `mise run test-e2e` verde

#### DoD
- [ ] Tests verdes
- [ ] Commit: "test: update env var refs in tests THEOPACKS_* → THEOKIT_PACKS_*"

---

## Phase 4: Config file rename (theopacks.json → theokit-packs.json)

**Objective:** Renomear o config file path constant e o example que usa esse path. Pode rodar em paralelo com Phases 2 e 3.

### T4.1 — Atualizar constante de config file path em Go

#### Objective
Wherever `theopacks.json` é hardcoded como path default (config reader), trocar para `theokit-packs.json`.

#### Evidence
`grep -l "theopacks.json" --include="*.go"` (executar). Provavelmente em `core/config/` ou `core/app/`.

#### Files to edit
```
core/config/config.go (ou similar — wherever "theopacks.json" literal aparece)
```

Lista exata via `git grep -l "theopacks.json" -- '*.go'`.

#### Deep file dependency analysis
- **Config reader** define o path default para o JSONC file lido pelo `app.App`. THEOPACKS_CONFIG_FILE env var pode override. Trocar literal apenas.
- **Downstream:** tests que mockam `app.HasFile("theopacks.json")` precisam atualizar (T4.2).

#### Tasks
1. `git grep -l "theopacks.json" -- '*.go' | xargs sed -i 's|theopacks.json|theokit-packs.json|g'`
2. Build verde

#### TDD
```
RED:     git grep "theopacks.json" -- '*.go' retorna 0
GREEN:   Aplicar sed
REFACTOR: None expected
VERIFY:  go build ./...
```

#### Acceptance Criteria
- [ ] `git grep "theopacks.json" -- '*.go'` retorna 0
- [ ] Build verde

#### DoD
- [ ] Commit: "refactor(config): rename config file path theopacks.json → theokit-packs.json"

---

### T4.2 — Renomear example `examples/node-npm-with-config/theopacks.json`

#### Objective
O único example com config file precisa do file renomeado para que os tests que loadam esse example continuem funcionando após T4.1.

#### Evidence
`find examples -name "theopacks.json"` retornou: `examples/node-npm-with-config/theopacks.json`.

#### Files to edit
```
examples/node-npm-with-config/theopacks.json → examples/node-npm-with-config/theokit-packs.json
```

#### Deep file dependency analysis
- **Single file rename.** Tests que loadam esse example via `app.NewApp("examples/node-npm-with-config")` agora vão procurar `theokit-packs.json` (per T4.1).

#### Tasks
1. `git mv examples/node-npm-with-config/theopacks.json examples/node-npm-with-config/theokit-packs.json`
2. Rodar test relacionado: `go test ./core/... -run "WithConfig"` (ou nome similar)

#### TDD
```
RED:     test que usa node-npm-with-config falha porque file não existe com nome antigo (após T4.1 + antes T4.2)
GREEN:   git mv
REFACTOR: None expected
VERIFY:  ls examples/node-npm-with-config/theokit-packs.json && go test ./core/...
```

#### Acceptance Criteria
- [ ] `ls examples/node-npm-with-config/theopacks.json` retorna "not found"
- [ ] `ls examples/node-npm-with-config/theokit-packs.json` retorna conteúdo
- [ ] Tests relevantes verdes

#### DoD
- [ ] Commit: "chore(examples): rename theopacks.json → theokit-packs.json"

---

### T4.3 — Regenerar golden Dockerfile do example renomeado

#### Objective
O golden `core/dockerfile/testdata/integration_node_npm_with_config.dockerfile` (ou similar) pode incluir refs ao filename. Regenerar via UPDATE_GOLDEN.

#### Evidence
Após T4.1+T4.2, o pipeline lê do novo file. Golden snapshot pode mencionar o filename em comentário ou COPY directive.

#### Files to edit
```
core/dockerfile/testdata/integration_node_npm_with_config.dockerfile — regenerated
```

#### Deep file dependency analysis
- Golden é output de teste. Regeneração via UPDATE_GOLDEN=true sobrescreve com novo conteúdo.

#### Tasks
1. `UPDATE_GOLDEN=true go test ./core/dockerfile/... -run "NpmWithConfig"`
2. `git diff core/dockerfile/testdata/integration_node_npm_with_config.dockerfile` — revisar
3. Commitar se ok

#### TDD
```
RED:     go test ./core/dockerfile/... -run "NpmWithConfig" falha (golden divergente)
GREEN:   UPDATE_GOLDEN=true regenera
REFACTOR: None expected
VERIFY:  go test ./core/dockerfile/... -run "NpmWithConfig"
```

#### Acceptance Criteria
- [ ] Golden file regenerado
- [ ] Diff revisado e legítimo
- [ ] Test verde

#### DoD
- [ ] Commit: "test(golden): regenerate node-npm-with-config golden after config file rename"

---

## Phase 5: Docs + branding + goldens

**Objective:** Atualizar todos os arquivos não-código (README, CHANGELOG, CLAUDE.md, contracts, NOTICE) + regenerar goldens com defensive header novo. Depende de Phases 2, 3, 4.

### T5.1 — Atualizar README.md, CHANGELOG.md, NOTICE

#### Objective
Front-page docs refletem novo nome + escopo TheoCloud-only.

#### Evidence
- `head -20 NOTICE` linha 1: `theo-packs`
- `head README.md` (provavelmente menciona theo-packs como título)
- `CHANGELOG.md` — adicionar entrada `[Unreleased]` `### Changed (BREAKING)` com o rename

#### Files to edit
```
README.md — title, badges, install instructions, nome do binary, exemplos
CHANGELOG.md — adicionar [Unreleased] entry "Changed (BREAKING): renamed theo-packs → theokit-packs, ..."
NOTICE — line 1: theo-packs → theokit-packs (Railpack attribution PRESERVED per D4)
```

#### Deep file dependency analysis
- **README.md** é a primeira impressão. Precisa nome, descrição (alinhada com TheoCloud-only), binary, exemplo de invocação.
- **CHANGELOG.md** segue Keep a Changelog. Entry [Unreleased] com seção BREAKING.
- **NOTICE** mantém o copyright header mas troca produto-nome. Railpack attribution intacta (D4).

#### Tasks
1. Read README.md atual
2. Atualizar: titulo, descrição (mencionar TheoCloud-only), install/build commands, exemplos
3. Atualizar CHANGELOG.md adicionando entry [Unreleased] Changed (BREAKING) com lista do que mudou (módulo path, binary, env vars, config file)
4. Atualizar NOTICE line 1 (apenas)
5. `git grep "theo-packs" README.md CHANGELOG.md NOTICE` deve retornar 0 (exceto NOTICE § Railpack se aplicável)

#### TDD
```
RED:     N/A (docs)
GREEN:   Editar 3 arquivos
REFACTOR: None expected
VERIFY:  grep -c "theo-packs" README.md CHANGELOG.md NOTICE  (esperado: 0 em README/CHANGELOG; NOTICE só na seção Railpack se houver, mantida)
```

#### Acceptance Criteria
- [ ] README.md title = "theokit-packs"
- [ ] CHANGELOG.md tem entry [Unreleased] BREAKING com lista
- [ ] NOTICE line 1 = "theokit-packs"
- [ ] NOTICE § Railpack inalterada (D4)

#### DoD
- [ ] Commit: "docs: rename theo-packs → theokit-packs in README, CHANGELOG, NOTICE"

---

### T5.2 — Atualizar `docs/contracts/theo-packs-cli-contract.md`

#### Objective
Rename o file e atualizar conteúdo.

#### Evidence
File existe e é authoritative reference para CLI contract (linkado de CLAUDE.md).

#### Files to edit
```
docs/contracts/theo-packs-cli-contract.md → docs/contracts/theokit-packs-cli-contract.md
content — todas refs ao binary, env vars, config file, módulo
```

#### Deep file dependency analysis
- File renomeado afeta links em CLAUDE.md, README.md, comentários em main.go ("See docs/contracts/...").
- Atualizar refs em outros files que linkam.

#### Tasks
1. `git mv docs/contracts/theo-packs-cli-contract.md docs/contracts/theokit-packs-cli-contract.md`
2. Sed no conteúdo do file: `theopacks-generate` → `theokit-packs-generate`, `THEOPACKS_` → `THEOKIT_PACKS_`, `theopacks.json` → `theokit-packs.json`, `theo-packs` → `theokit-packs`
3. Atualizar links inbound:
   - `CLAUDE.md` ref a `docs/contracts/theo-packs-cli-contract.md`
   - `README.md` similar
   - `cmd/theokit-packs-generate/main.go` comments

#### TDD
```
RED:     ls docs/contracts/theo-packs-cli-contract.md retorna "not found"
GREEN:   git mv + sed + atualizar refs inbound
REFACTOR: None expected
VERIFY:  ls docs/contracts/theokit-packs-cli-contract.md && grep -rL "theo-packs-cli-contract" CLAUDE.md README.md cmd/
```

#### Acceptance Criteria
- [ ] File renamed
- [ ] Conteúdo limpo de `theopacks` / `theo-packs`
- [ ] Refs inbound atualizadas

#### DoD
- [ ] Commit: "docs(contract): rename CLI contract doc + sweep refs"

---

### T5.3 — Atualizar `CLAUDE.md` (projeto)

#### Objective
CLAUDE.md descreve a arquitetura para Claude Code. Precisa refletir novo nome + escopo TheoCloud-only.

#### Evidence
CLAUDE.md tem ~310 linhas. Menciona theopacks-generate, THEOPACKS_*, theo-packs em vários lugares.

#### Files to edit
```
CLAUDE.md — sweep + atualizar seção "What This Project Is" para refletir escopo TheoCloud-only
```

#### Deep file dependency analysis
- CLAUDE.md é lido por Claude Code em toda sessão. Refletir realidade pós-rename é crítico para evitar drift.
- Seção "Provider Detection Order" — manter (todos os 11 providers continuam).
- Seção "What This Project Is" — atualizar para mencionar **theokit como único alvo de geração** (TheoCloud-only scope).

#### Tasks
1. Read CLAUDE.md inteiro
2. Sweep substitutions:
   - `theo-packs` → `theokit-packs`
   - `theopacks-generate` → `theokit-packs-generate`
   - `THEOPACKS_` → `THEOKIT_PACKS_`
   - `github.com/usetheo/theopacks` → `github.com/usetheo/theokitpacks`
   - `theopacks.json` → `theokit-packs.json`
3. Reescrever seção "What This Project Is" mencionando: "theokit-packs é a engine de detecção + Dockerfile generation para apps gerados pelo theokit, que TheoCloud usa como fluxo único"
4. Atualizar paths em "Key References" table

#### TDD
```
RED:     N/A
GREEN:   Edit CLAUDE.md
REFACTOR: None expected
VERIFY:  grep -c "theopacks\|theo-packs" CLAUDE.md retorna 0 (exceto contexto histórico se mantido)
```

#### Acceptance Criteria
- [ ] Zero refs a `theopacks` / `theo-packs` em CLAUDE.md
- [ ] Seção "What This Project Is" reflete TheoCloud-only scope
- [ ] Module path, binary, env vars, config file: tudo novo nome

#### DoD
- [ ] Commit: "docs(claude-md): rename + scope shift for TheoCloud-only theokit-packs"

---

### T5.4 — Atualizar `.claude/project.yaml`

#### Objective
project.yaml descreve metadata do projeto para cycles-engine. Atualizar name + module_prefix.

#### Evidence
`.claude/project.yaml` linha 11: `name: theo-packs`, linha 13: `module_prefix: github.com/usetheo/theopacks`.

#### Files to edit
```
.claude/project.yaml — name, module_prefix, description (se menciona theo-packs)
```

#### Deep file dependency analysis
- `cycles-engine` lê esse file. Mudança propaga para qualquer skill que consulte.
- `architecture.layers` block — manter (DIP layout não muda).

#### Tasks
1. Editar:
   - `name: theo-packs` → `name: theokit-packs`
   - `module_prefix: github.com/usetheo/theopacks` → `module_prefix: github.com/usetheo/theokitpacks`
   - `description:` (se mencionar theo-packs)
2. Verificar consistência com architecture rules

#### TDD
```
RED:     N/A
GREEN:   Edit YAML
REFACTOR: None expected
VERIFY:  grep -c "theopacks\|theo-packs" .claude/project.yaml retorna 0
```

#### Acceptance Criteria
- [ ] `name: theokit-packs`
- [ ] `module_prefix: github.com/usetheo/theokitpacks`
- [ ] Zero refs antigas

#### DoD
- [ ] Commit: "chore(project-yaml): rename to theokit-packs"

---

### T5.5 — Regenerar todos os golden Dockerfiles com defensive header novo

#### Objective
~10+ goldens em `core/dockerfile/testdata/` contêm `# theo-packs: generated for provider "X"` no defensive header (linha emitida por `core/dockerfile/HeaderComment` ou similar). Após T1.2 + T3.1, o código gerador foi renomeado mas os golden snapshots ainda têm "theo-packs". Regenerar.

#### Evidence
`grep -l "theopacks\|theo-packs" core/dockerfile/testdata/*.dockerfile` retornou 10+ files.

#### Files to edit
```
core/dockerfile/testdata/*.dockerfile — regenerated en bloc
```

Lista exata por `grep -l "theo-packs" core/dockerfile/testdata/`.

#### Deep file dependency analysis
- Goldens são output. `core/dockerfile/generate.go` (ou `HeaderComment` func) emite a string `theo-packs:`. Mudar a função → regenerate.
- Após T1.2 + T3.1 a função já foi renomeada? **VERIFICAR.** `HeaderComment` provavelmente tem string literal `"theo-packs:"` no source. Precisa mudar antes de regenerar — incluir em T1.2/T3.1 ou explicitar aqui.

#### Tasks
1. Confirmar local da string literal `theo-packs:` em `core/dockerfile/generate.go` ou `HeaderComment`. Se ainda lá:
   - `git grep "theo-packs:" -- core/dockerfile/` → identificar exato
   - Substituir literal: `"theo-packs:"` → `"theokit-packs:"`
2. Rodar `UPDATE_GOLDEN=true go test ./core/dockerfile/...`
3. `git diff core/dockerfile/testdata/` — revisar (esperado: cada golden tem `theo-packs:` → `theokit-packs:`)
4. Commitar

#### TDD
```
RED:     go test ./core/dockerfile/... falha em N goldens (theo-packs vs theokit-packs no header)
GREEN:   Substituir literal no gerador + UPDATE_GOLDEN
REFACTOR: None expected
VERIFY:  go test ./core/dockerfile/...
```

#### Acceptance Criteria
- [ ] `grep "theo-packs\|theopacks" core/dockerfile/testdata/*.dockerfile` retorna 0
- [ ] `go test ./core/dockerfile/...` verde
- [ ] Diff dos goldens revisado (apenas linha do header alterada)

#### DoD
- [ ] Commit: "test(golden): regenerate goldens with theokit-packs defensive header"

---

### T5.6 — `llm.txt` e outros files raiz

#### Objective
File `llm.txt` na raiz (visto no listing inicial) pode ter ref ao nome antigo. Auditar.

#### Evidence
`ls` raiz mostra `llm.txt`. Conteúdo desconhecido até Read.

#### Files to edit
```
llm.txt — sweep se necessário
```

#### Tasks
1. Read `llm.txt`
2. Identificar refs a `theo-packs` / `theopacks`
3. Substituir

#### TDD
```
RED:     N/A
GREEN:   Edit
REFACTOR: None expected
VERIFY:  grep -c "theo-packs\|theopacks" llm.txt retorna 0
```

#### Acceptance Criteria
- [ ] Zero refs antigas em llm.txt

#### DoD
- [ ] Commit: "docs(llm-txt): rename refs"

---

## Phase 6: Repo rename + consumer cutover

**Objective:** Operação final: rename do repo no GitHub, atualizar remote local, notificar consumers para mergeiar PRs próprios. Acontece **após** Phases 1-5 mergeadas em `develop` e validadas.

### T6.1 — Rename do repo no GitHub

#### Objective
`usetheodev/theo-packs` → `usetheodev/theokit-packs` no GitHub. GitHub mantém redirecionamento por algum tempo, mas é melhor coordenar.

#### Evidence
Decisão de naming confirmada. Cutover window definida em T0.2.

#### Files to edit
N/A — operação GitHub UI / API.

#### Deep file dependency analysis
- Após rename: clone URLs mudam. Open PRs em vôo continuam funcionando via redirect.
- Webhooks (se houver) podem precisar reativar.

#### Tasks
1. No GitHub UI: Settings → Repository → Rename
2. Verificar webhooks
3. Atualizar remote local: `git remote set-url origin git@github.com:usetheodev/theokit-packs.git`

#### TDD
```
RED:     N/A (operação GitHub)
GREEN:   Rename via UI
REFACTOR: None expected
VERIFY:  git remote -v mostra theokit-packs
```

#### Acceptance Criteria
- [ ] Repo renomeado no GitHub
- [ ] Remote local apontando para novo URL
- [ ] PRs abertos continuam funcionando

#### DoD
- [ ] Rename completo
- [ ] Notificar consumers no canal #theo-cloud / #theokit

---

### T6.2 — Notificar TheoCloud + theokit para repointar refs

#### Objective
TheoCloud Argo Workflow + theokit (se referencia o binary) atualizam PRs próprios para usar `theokit-packs-generate` e o novo image tag.

#### Evidence
T0.2 listou consumers + owners.

#### Files to edit
N/A neste repo — mudanças vivem nos repos consumers.

#### Tasks
1. Para cada consumer no cutover doc: ping owner com link do PR final de Phase 5
2. Verificar PRs deles abrirem e mergeiarem
3. Validar end-to-end: TheoCloud builda app via pipeline com binary novo, sucesso

#### TDD
```
RED:     N/A
GREEN:   Coordenação
REFACTOR: None expected
VERIFY:  TheoCloud pipeline run verde com binary novo
```

#### Acceptance Criteria
- [ ] Todos os consumers no doc T0.2 status=DONE
- [ ] TheoCloud Argo pipeline run sample verde com `theokit-packs-generate`

#### DoD
- [ ] Cutover completo
- [ ] Documento T0.2 atualizado com timestamp de conclusão

---

## Coverage Matrix

| # | Gap / Requirement | Task(s) | Resolution |
|---|---|---|---|
| 1 | Módulo Go path renamed | T1.1, T1.2 | `go.mod` line 1 + 71 imports |
| 2 | CLI binary renamed | T2.1 | Diretório + log prefixes |
| 3 | Dockerfile.generate refs | T2.2 | 6 string occurrences |
| 4 | GitHub Actions workflows | T2.3 | 3 workflows |
| 5 | Env vars THEOPACKS_* → THEOKIT_PACKS_* (production) | T3.1 | ~25 files Go non-test |
| 6 | Env vars em tests | T3.2 | ~30 files _test.go |
| 7 | Config file path constant | T4.1 | core/config/ literais |
| 8 | Example config file | T4.2 | examples/node-npm-with-config/ |
| 9 | Golden de example com config | T4.3 | 1 golden regenerated |
| 10 | README, CHANGELOG, NOTICE | T5.1 | 3 files |
| 11 | CLI contract doc | T5.2 | rename + sweep |
| 12 | CLAUDE.md scope shift | T5.3 | sweep + scope-shift wording |
| 13 | .claude/project.yaml | T5.4 | name + module_prefix |
| 14 | Defensive header em goldens | T5.5 | 10+ goldens regenerated |
| 15 | llm.txt | T5.6 | sweep |
| 16 | Repo rename | T6.1 | GitHub UI |
| 17 | Consumer notification | T6.2 | TheoCloud + theokit |
| 18 | Cutover plan documentado | T0.2 | docs/plans/rename-to-theokit-packs-cutover.md |
| 19 | Audit baseline | T0.1 | docs/plans/rename-to-theokit-packs-audit.md |

**Coverage: 19/19 gaps covered (100%)**

## Global Definition of Done

- [ ] Todas as 6 phases completadas e mergeadas em `develop`
- [ ] `git grep "theopacks" -- '*.go' '*.yml' '*.yaml' '*.toml' '*.json' '*.dockerfile'` retorna 0
- [ ] `git grep "theo-packs" -- '*.md'` retorna 0 (exceto NOTICE § Railpack se aplicável e `docs/plans/PR_DESCRIPTION_*.md` históricos)
- [ ] `git grep "THEOPACKS_"` retorna 0
- [ ] `git grep "github.com/usetheo/theopacks"` retorna 0
- [ ] `mise run check` zero warnings
- [ ] `mise run test` verde
- [ ] `mise run test-e2e` verde
- [ ] Repo renomeado no GitHub
- [ ] TheoCloud + theokit consumers atualizados (status=DONE no cutover doc)
- [ ] CHANGELOG.md tem entry [Unreleased] documentando o BREAKING change
- [ ] NOTICE preserva atribuição Railpack (Apache 2.0 § 4(c)(i))
- [ ] Sem backward-compatibility introduzida (D1 respeitado)
- [ ] Próximo plano (`feat/theokit-support` aplicando blueprint fixes B1/B3) pode rodar no código renomeado

---

## Notas honestas

- **Plano é mecânico em sua maior parte.** ~80% das tasks são sed + git mv + go build. Risco real está em (a) workflows YAML edits, (b) goldens regeneration, (c) coordenação cross-repo (Phase 6).
- **TDD é pseudo-TDD em rename mecânico.** Os "RED" são verificações de presença/ausência de strings, não tests de comportamento. Isso é honesto — não há comportamento mudando.
- **Sem backward-compat (D1) é dívida deliberada.** Consumers fora dos 2 conhecidos (TheoCloud, theokit) que dependem do binary/env vars antigos **quebram silenciosamente**. Mitigação: T0.2 deve catalogar consumers tão exaustivamente quanto possível.
- **Phase 6 depende de input externo.** A janela de cutover não é decidida por este plano — vem de coordenação humana com TheoCloud team.
