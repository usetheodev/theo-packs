# Plan: Robust Test Suite — Industry-Grade Quality Gates

> **Version 1.0** — Eleva a suíte de testes do theo-packs ao patamar industrial-grade descrito na pesquisa técnica de 2026-05-20. Hoje a suíte cobre 678 testes, fuzz em 3 funções críticas, 48 examples E2E, benchmarks com baseline, race detector e behavior matchers — mas ainda faltam: lint/security gates no artefato (hadolint, trivy, dive), validação estrutural da imagem (container-structure-test), differential testing entre BuildKit e Kaniko (consumidor downstream em produção via Argo), reprodutibilidade bit-exata (`SOURCE_DATE_EPOCH`), property-based testing no resolver/merge, mutation testing como quality gate, e supply chain SLSA L3 + Cosign + SBOM. O plano fecha esses gaps em 5 fases não-bloqueantes (cada uma é entrega autônoma), com métricas-alvo concretas (cobertura ≥85% em providers, mutation ≥70% em pacotes críticos, flakiness <1%, zero CVEs HIGH/CRITICAL).

## Context

Estado atual (commit `d93f37a`):

- **Stack de teste:** `testify` + golden files custom (`UPDATE_GOLDEN=true`), `-race` em todos os runs, 19 pacotes verdes, golangci-lint zero issues.
- **E2E:** `e2e/e2e_table_test.go::TestE2E_All` cobre 48 examples via `docker build` real; 9 verifiers nomeados; `t.Cleanup` em todos os sites.
- **Regression tests:** `TestRegression_*_DeterministicOutput` (3 ex × 10 iter), `TestRegression_Turborepo_*_NoSpuriousSecrets` (4 paths).
- **Fuzz:** `FuzzShellEscape` (26880 execs), `FuzzValidateCLIInput` (119071), `FuzzClampPath` (82352) — corpus persistido.
- **Benchmarks:** 3 baselines em `docs/benchmarks/baseline.txt`.
- **CI:** `.github/workflows/ci.yml` roda `mise run check && mise run test`. `e2e.yml` separado (Docker required).

Gaps confirmados via análise da pesquisa técnica:

| ID | Gap | Severidade | Evidência |
|---|---|---|---|
| G1 | **Sem hadolint** sobre Dockerfiles gerados. Warnings DL3xxx/SC2xxx silenciosamente toleradas. | Alta | `find . -name "*.hadolint*"` retorna vazio; CI não chama hadolint. |
| G2 | **Sem container-structure-test**. Não validamos "imagem tem /app/server", "USER appuser", "ENV PORT", etc. de forma declarativa. | Alta | Verifiers atuais são imperativos em Go; cobertura é frágil e duplicada por example. |
| G3 | **Sem Trivy/grype** scan da imagem final. Imagem-base pode ter HIGH/CRITICAL CVE sem ninguém perceber. | Crítica | CI workflow não invoca scanner. CIS docker-cis-1.6.0 não validado. |
| G4 | **Sem dive efficiency check**. Layer bloat (devDependencies vazando) só pega via `requireSizeLessThan` ad-hoc. | Média | `e2e_table_test.go::maxMB` é proxy fraco; dive efficiency catch layer ordering issues. |
| G5 | **Sem differential test BuildKit vs Kaniko**. Argo Workflow do Theo usa Kaniko; `--mount=type=cache` tem semântica diferente. Regressão silenciosa. | Crítica | Todos os E2E rodam BuildKit; Kaniko nunca exercitado. |
| G6 | **Sem teste de reprodutibilidade** (`SOURCE_DATE_EPOCH`). Dois builds back-to-back produzem digests diferentes. | Média | Determinism cobre o texto do Dockerfile, mas não a imagem OCI. |
| G7 | **Sem property-based testing**. `config.Merge` (associatividade?), `Resolver` (prioridade?), `detect` (estabilidade contra arquivos ruído?) não têm propriedades verificadas. | Alta | Pesquisa cita `pgregory.net/rapid`. Hoje só fuzz. |
| G8 | **Sem mutation testing**. Cobertura de linha alta mas mutation kill rate desconhecido. | Média | `go-gremlins` ausente. Recomendado para `core/providers`, `core/resolver`, `core/config`. |
| G9 | **Sem SBOM** nas imagens. Auditoria de supply chain impossível. | Alta | syft não invocado em CI nem em release. |
| G10 | **Sem Cosign / SLSA L3 provenance**. Releases sem assinatura keyless. | Alta | `cosign` ausente do build pipeline. |
| G11 | **Sem `test.json` per-example com `httpCheck`**. Pesquisa cita padrão Railpack; nosso `e2eCase` tem verify hardcoded em Go. | Média | Migração para test.json + httpCheck reduz boilerplate, padroniza healthchecks. |
| G12 | **Sem sharding L3 no CI**. Full E2E suite roda serial (48 builds Docker) — caminho crítico longo. | Média | `e2e.yml` não tem matrix; 48 builds = ~25min sequencial. |
| G13 | **Sem flakiness tracking**. Testes que falham < 100% mas > 0% não são detectados sistematicamente. | Baixa | Sem run repeat + dashboard. |
| G14 | **Sem testcontainers-go** para healthchecks com dependências (Postgres, Redis). | Média | Examples como `python-django` (que normalmente precisa DB) testam só `import django`. |
| G15 | **Sem `oras` / `crane validate`** do manifest OCI. | Baixa | Imagem pode ser tecnicamente inválida sem ninguém perceber até deploy. |

Motivação: theo-packs roda em produção via Argo Workflow + Kaniko. **Cada regressão silenciosa entre BuildKit (dev) e Kaniko (prod) é hoje um incidente potencial**. Cada CVE HIGH/CRITICAL na base image é vetor de comprometimento de tenant. A pesquisa aponta os 4 gaps mais críticos como **G1 (hadolint), G3 (Trivy), G5 (Kaniko differential) e G10 (supply chain)** — endereçá-los reduz 80% das classes de bug típicas.

## Objective

**Done** = CI roda 5 camadas (L1 Fast / L2 PR integration / L3 Full matrix / L4 Quality gates / L5 Release), todos os 15 gaps fechados com teste correspondente, e métricas-alvo da pesquisa atingidas:

1. **Cobertura linhas** `core/providers/*` ≥ 85%; `core/plan`, `core/resolver`, `core/config` ≥ 90%.
2. **Mutation score** (gremlins) ≥ 70% em `core/providers`, `core/resolver`, `core/config`.
3. **Hadolint** zero warnings ≥ warning severity em todos os goldens + Dockerfiles E2E.
4. **Trivy** zero HIGH/CRITICAL CVEs nas imagens E2E críticas.
5. **Dive efficiency** ≥ 90% (target 95%) para cada example E2E.
6. **Differential BuildKit vs Kaniko** verde para ≥ 10 examples críticos.
7. **Reprodutibilidade** digest idêntico em 2 builds back-to-back para ≥ 3 examples.
8. **container-structure-test** YAML per-example com ao menos 1 file existence + 1 command + 1 metadata assertion.
9. **SBOM** SPDX + CycloneDX gerados em release, anexados via `cosign attest`.
10. **SLSA L3 provenance** assinada com cosign keyless via GitHub OIDC.
11. **Property tests** com `rapid` em resolver (idempotência + prioridade) e config (associatividade).
12. **Flakiness rate** < 1% em 100 runs do integration suite.
13. **L1 < 3min**, **L2 < 10min**, **L3 < 60min** com sharding.

## ADRs

### D1 — Snapshots `.golden` files são canônicos; `go-snaps` opcional para BuildPlan structs

**Decisão:** manter o helper golden file atual (`UPDATE_GOLDEN=true`) para Dockerfile-text (já estabelecido). Adicionar `go-snaps` (`gkampitakis/go-snaps`) **apenas** para asserts de `BuildPlan` struct (JSON-serializável complexa) quando o diff fica difícil de ler em mensagens de teste. **Não migrar** os 58 goldens existentes — go-snaps é aditivo.

**Rationale:** Pesquisa propõe go-snaps para tudo (como Railpack), mas migrar 58 goldens é alto custo e baixo benefício — nosso helper já tem `UPDATE_GOLDEN`. go-snaps brilha em diff de structs aninhadas; aproveitar pontualmente. Alternativa rejeitada: migrar 100% para go-snaps — quebra git blame da história dos goldens.

**Consequências:** Goldens de Dockerfile permanecem em `testdata/*.dockerfile`. Snapshots de struct futuros vão para `__snapshots__/*.snap` adjacentes ao `_test.go`. Ambos são revisão obrigatória em PR.

### D2 — `hadolint` é gate hard em PR; warnings de info viram opt-in

**Decisão:** introduzir `.hadolint.yaml` versionado com `failure-threshold: warning`; rodar em CI L1 contra todos os 58 goldens + Dockerfiles emitidos pelos E2E. Severities `info` e `style` ficam visíveis (warning no CI) mas não bloqueiam merge.

**Rationale:** zero-warning-policy desde dia 1 evita drift. Severities `info`/`style` muitas vezes geram falsos positivos com BuildKit-specific syntax — não bloqueamos por elas. Alternativa rejeitada: rodar hadolint só em release — drift acumula e o débito vira inviável.

**Consequências:** Cada novo Dockerfile gerado precisa passar hadolint. Provider que introduza `RUN apt-get update` sem `&& apt-get install` (DL3009) tem feedback imediato.

### D3 — `container-structure-test` declarativo per-example

**Decisão:** cada example crítico ganha um `structure-tests.yaml` ao lado do projeto (não dentro). O integration runner descobre via convenção. Verifiers em Go ficam como fallback para casos que YAML não cobre (logic decision-tree).

**Rationale:** YAML declarativo é manutenível e legível. Reduz duplicação dos verifiers em Go. Trade-off conhecido: container-structure-test está em maintenance mode (caveat da pesquisa) — aceitável; v1.19.3 é estável e cobre 100% dos casos atuais.

**Consequências:** Ferramenta nova como dep de teste; entry no `mise.toml`. Provider novo pode optar por escrever só structure-tests.yaml sem código Go.

### D4 — Trivy é gate hard em release; warning em PR

**Decisão:** L1 (PR) roda Trivy em modo `--exit-code 0` (apenas reporta). L5 (release) roda com `--severity HIGH,CRITICAL --exit-code 1` (bloqueia release se base image tem CVE). Compliance `--compliance docker-cis-1.6.0` roda no L4 (nightly).

**Rationale:** bloquear PR por CVE em base image é fricção alta para o autor (não pode corrigir sem bump em base image, que é decisão fora da PR). Bloquear release é o ponto correto — operacionalmente alguém precisa decidir patch ou ignore. Alternativa rejeitada: bloquear PR — força workarounds.

**Consequências:** Release pipeline depende de Trivy verde. Política de "patch dentro de N dias" precisa ser documentada para CVEs novos detectados pós-merge.

### D5 — Kaniko differential test é critical path antes de v1.0

**Decisão:** introduzir `e2e_kaniko_test.go` (build tag `e2e_kaniko`) que builda 10 examples críticos com Kaniko + verifica `Image-runtime` parity contra BuildKit. Roda em L3 (nightly) inicialmente; promove a L2 quando estável.

**Rationale:** Argo Workflow do Theo usa Kaniko. `--mount=type=cache` comporta diferente; secrets também. Differential detecta regressão silenciosa quando provider muda. Alternativa rejeitada: trust BuildKit ⇒ Kaniko — pesquisa documenta divergência real desde Kaniko v1.9.

**Consequências:** Build runtime de L3 sobe (~ +15min). Mitigado por sharding (D8).

### D6 — Property-based testing em `rapid`, escopo cirúrgico

**Decisão:** introduzir `pgregory.net/rapid` apenas para:
- `core/resolver/Resolver` (idempotência: `resolve(resolve(x)) == resolve(x)`; prioridade dos 4 sources).
- `core/config/Merge` (associatividade: `merge(merge(a,b),c) == merge(a,merge(b,c))`; identidade: `merge(a, empty) == a`).
- `core/providers/*::Detect` (estabilidade: adicionar arquivo ruído não muda detection).

**Rationale:** Pesquisa adverte que property testing tem curva. Limitar a 3 áreas concretas mantém ROI alto. `rapid` foi preferido a `gopter` pela API + shrinking automático.

**Consequências:** dep nova `pgregory.net/rapid`. 3-5 props no início, não 30. Resista à tentação de transformar tudo em rapid.

### D7 — Mutation testing roda no L4 (nightly), `core/providers` + `core/resolver` + `core/config`

**Decisão:** `go-gremlins` como L4 nightly job; quality gate em PR bloqueia se mutation score < 65% (warmup) → 70% (steady). Roda apenas nesses 3 diretórios; codebase inteiro é proibitivo segundo o próprio README do gremlins.

**Rationale:** mutation kill rate é o complemento da cobertura. Linhas cobertas com asserts fracos têm score baixo. Limitar a diretórios-críticos mantém tempo viável.

**Consequências:** L4 ~20min adicionais. PR check separado que pode falhar tardiamente — documentar como advisory por 4 semanas, então hard gate.

### D8 — CI em 5 camadas com sharding em L3

**Decisão:** estrutura piramidal:
- L1 (fast, PR): vet/lint/test -short, hadolint, ≤3min
- L2 (PR integration): ≤10min, sharded matrix 3-5 examples por provider
- L3 (main + nightly): full 48 examples × {BuildKit, Kaniko} × {amd64, arm64}, sharded 8-way
- L4 (nightly): gremlins, dive, trivy compliance, reproducibility
- L5 (tag): SLSA L3, cosign, SBOM, attestations

Implementar `-shard=N/M` no runner via `fnv32(name) % M == N`.

**Rationale:** L1/L2 dão feedback rápido; L3 cobre tudo sem bloquear PRs. Sharding torna L3 viável (60min target). Pesquisa cita os tempos.

**Consequências:** workflow files novos (`e2e-l2.yml`, `e2e-l3.yml`, `nightly.yml`, `release.yml`). CI bill sobe (~3x); aceitável dado o valor.

### D9 — `test.json` per-example com `httpCheck` substitui verifiers Go onde possível

**Decisão:** introduzir formato `test.json` em `examples/<name>/`. Schema:
```jsonc
{
  "mode": "httpCheck" | "justBuild" | "expectedOutput",
  "httpCheck": { "path": "/health", "port": 8080, "expectedStatus": 200 },
  "structureTest": "structure-tests.yaml",
  "env": { "THEOPACKS_APP_NAME": "..." },
  "skip": false,
  "skipReason": "..."
}
```
Manter `e2eCase` Go-defined como fallback. Migrar examples gradualmente.

**Rationale:** padroniza shape de teste, reduz boilerplate Go por example, alinha com Railpack/Nixpacks. Migration incremental sem flag-day.

**Consequências:** novo parser de `test.json`. Examples ganham um arquivo a mais. Pesquisa adverte: `justBuild` é code-smell para tudo exceto trivial; usar `httpCheck` por padrão.

### D10 — Reprodutibilidade via `SOURCE_DATE_EPOCH` + BuildKit ≥ 0.13

**Decisão:** novo test `TestE2E_Reproducible_*` builda 3 examples (go-simple, node-npm, python-flask) duas vezes com `SOURCE_DATE_EPOCH=1700000000` + `--build-arg SOURCE_DATE_EPOCH` + `--output type=image,rewrite-timestamp=true`. Assert digest idêntico.

**Rationale:** reprodutibilidade é invariante de supply chain. BuildKit 0.13+ suporta nativamente.

**Consequências:** dep em buildx ≥ 0.13 documentada no CI. Imagens base via `--platform` fixed.

## Dependency Graph

```
Phase 0 — Foundation (paralelo entre si)
  T0.1 hadolint                ─┐
  T0.2 .hadolint.yaml          ─┤
  T0.3 trivy advisory          ─┤
  T0.4 dive .dive-ci           ─┘
            │
            ▼
Phase 1 — Artifact quality (sequencial em L2/L3 deps)
  T1.1 container-structure-test infra
  T1.2 structure-tests.yaml para 10 examples
  T1.3 SBOM via syft
  T1.4 test.json schema + parser
  T1.5 Migrar 20 examples para test.json
            │
            ▼
Phase 2 — Property + mutation (paralelo entre si)
  T2.1 rapid + resolver props      ─┐
  T2.2 rapid + config Merge props  ─┤
  T2.3 rapid + Detect stability    ─┤
  T2.4 gremlins infra + baseline   ─┘
            │
            ▼
Phase 3 — Differential + reproducibility (paralelo)
  T3.1 Kaniko differential            ─┐
  T3.2 Reproducibility (SOURCE_DATE)  ─┤
  T3.3 diffoci diagnose helper        ─┘
            │
            ▼
Phase 4 — Supply chain (sequencial)
  T4.1 SLSA L3 provenance
  T4.2 Cosign keyless signing
  T4.3 SBOM attestations
  T4.4 Release verify gate
            │
            ▼
Phase 5 — CI orchestration (sequencial após features prontas)
  T5.1 L1/L2/L3 split em workflows
  T5.2 Sharding helpers + matrix
  T5.3 L4 nightly job
  T5.4 L5 release job
  T5.5 Flakiness tracking
```

Phases 0/1 são pré-requisito de tudo. Phase 2 e 3 podem rodar em paralelo. Phase 4 e 5 fecham o ciclo.

---

## Phase 0: Foundation — hadolint + trivy advisory + dive

**Objective:** habilitar lint e scan sobre o artefato gerado sem bloquear PRs ainda. Cria infraestrutura para gates futuros.

### T0.1 — Hadolint over goldens

#### Objective
Adicionar step no CI L1 que roda `hadolint` em todos os `core/dockerfile/testdata/*.dockerfile` e falha o build se severity ≥ warning.

#### Evidence
G1. Hoje nenhum Dockerfile gerado é validado contra best practices DL3xxx/SC2xxx. Provider pode introduzir `RUN apt-get update` órfão sem feedback.

#### Files to edit
```
.hadolint.yaml (NEW) — config versionada com failure-threshold, ignores justificados
.github/workflows/ci.yml — adicionar step "Hadolint goldens"
docs/contributing.md (NEW) — documentar como rodar hadolint local
```

#### Deep file dependency analysis
- **`.hadolint.yaml` (NEW)** — config raiz; lido por `hadolint <files>` sem flags. Define ignores DL3008 (apt versão pinned não pratico) etc.
- **`ci.yml`** — adiciona step ANTES de `mise run test` para feedback rápido.
- **`testdata/*.dockerfile`** — leitura apenas; valida-os.

#### Deep Dives
- Hadolint via Docker action `hadolint/hadolint-action@<sha>` para versionamento. Alternativa: `apt install hadolint` no runner (mais simples mas atrelado ao Ubuntu).
- Ignores típicos justificáveis em theo-packs:
  - **DL3018** (apk add without version): aceitável quando provider depende de versão da distro
  - **DL3008** (apt without version): idem
  - **SC1091** (sh source warning): cache mount paths nem sempre exist na hora do lint
- `failure-threshold: warning` é a config-chave.

#### Tasks
1. Criar `.hadolint.yaml` com defaults + 3-5 ignores documentados.
2. Adicionar step em `ci.yml` rodando `hadolint core/dockerfile/testdata/*.dockerfile`.
3. Rodar localmente; corrigir warnings emergentes nos goldens (ou adicionar ignore explícito + comentário).
4. Documentar em `CONTRIBUTING.md` (ou apêndice ao `CLAUDE.md`) como rodar hadolint local.

#### TDD
```
RED:     CI passes hoje sem hadolint; novo step deve FAILER ao menos uma vez (warning genuíno num golden) → corrigir → green.
GREEN:   Step passa; corpus de goldens livre de warnings ≥ warning severity.
REFACTOR: None expected.
VERIFY:  hadolint core/dockerfile/testdata/*.dockerfile (local)
         + CI green
```

#### Acceptance Criteria
- [ ] `.hadolint.yaml` existe com `failure-threshold: warning`
- [ ] CI step "Hadolint goldens" passa
- [ ] Ignores ≤ 5 e cada um tem comentário justificando
- [ ] CONTRIBUTING.md / docs documenta o gate

#### DoD
- [ ] Step verde em `main`
- [ ] PR teste introduz uma violação genuína e o CI falha → confirma gate funcional

---

### T0.2 — Hadolint over E2E generated Dockerfiles

#### Objective
Estender o gate para os Dockerfiles que `e2e_table_test.go` gera em runtime. Captura provider que escreve Dockerfile inválido só em runtime.

#### Evidence
T0.1 cobre estática (goldens commitados). E2E pode emitir Dockerfile diferente em runtime (cache mount, env-driven). Sem T0.2, esse path fica oculto.

#### Files to edit
```
e2e/e2e_table_test.go — após generateDockerfile(), rodar hadolint se disponível
e2e/hadolint.go (NEW) — wrapper que invoca hadolint via exec
```

#### Deep file dependency analysis
- **`e2e/hadolint.go`** — função `runHadolint(t, dockerfile)` que escreve para tempfile + invoca binário + parse stdout.
- **`e2e_table_test.go::runE2ECase`** — chama `runHadolint` entre `generateDockerfile` e `buildImage`.

#### Deep Dives
- Skip gracioso se hadolint binary não disponível (não-crítico no CI L1 local).
- Em CI L2/L3 o hadolint está instalado — gate hard.
- Reutiliza `.hadolint.yaml` do repo via `--config` flag.

#### Tasks
1. Criar `e2e/hadolint.go::runHadolint`.
2. Chamar em `runE2ECase` antes de `buildImage`.
3. Validar localmente.

#### TDD
```
RED:     N/A (testa-se via execução normal do e2e)
GREEN:   Helper roda; suite atual continua verde.
REFACTOR: None expected.
VERIFY:  go test -tags e2e -run TestE2E_All/go-simple ./e2e/
```

#### Acceptance Criteria
- [ ] `runHadolint` invocado em cada case da tabela
- [ ] Skip gracioso quando binário ausente
- [ ] Failure mode emite output legível com linha do Dockerfile

#### DoD
- [ ] T0.2 verde
- [ ] Documentado em ADR-0003 (se decidirmos formalizar)

---

### T0.3 — Trivy scan em modo advisory

#### Objective
Adicionar Trivy ao L1 em modo `--exit-code 0` para REPORTAR HIGH/CRITICAL sem bloquear. Output em PR comment via action.

#### Evidence
G3. Imagem-base pode ter CVE e ninguém vê. Trivy adv mode é zero-fricção primeiro passo.

#### Files to edit
```
.github/workflows/ci.yml — novo step "Trivy advisory"
```

#### Deep file dependency analysis
- **`ci.yml`** — step usa `aquasecurity/trivy-action@<sha>` ou docker run direto.
- O scan precisa de uma imagem; usaremos `theo-packs-runner` (imagem do próprio binário) inicialmente.

#### Deep Dives
- Trivy DB cache: usar `actions/cache` em `~/.cache/trivy`.
- Output: SARIF para upload em GitHub Security tab (`github/codeql-action/upload-sarif`).
- Hoje só escaneia o runner; em Phase 1 estende para imagens E2E.

#### Tasks
1. Adicionar step Trivy no `ci.yml`.
2. SARIF upload.
3. Confirmar zero crashes.

#### TDD
```
RED:     N/A (advisory; não bloqueia)
GREEN:   Step roda e produz SARIF; GitHub Security tab populado.
REFACTOR: None expected.
VERIFY:  CI run completo + Security tab verificado.
```

#### Acceptance Criteria
- [ ] Step Trivy roda em < 60s (com cache)
- [ ] SARIF upload sucesso
- [ ] Zero falsos crashes do scanner

#### DoD
- [ ] Verde em main
- [ ] Issue aberto para cada HIGH/CRITICAL encontrado (review manual)

---

### T0.4 — Dive `.dive-ci` thresholds

#### Objective
Adicionar `dive` em CI L2 para detectar layer ordering issues, devDependencies vazando, etc.

#### Evidence
G4. `requireSizeLessThan(t, tag, 280)` é proxy fraco. Dive efficiency catch layer reordering.

#### Files to edit
```
.dive-ci (NEW) — thresholds versionados
.github/workflows/e2e.yml — step "Dive layers" após cada build E2E
```

#### Deep file dependency analysis
- **`.dive-ci`** — YAML com `lowestEfficiency: 0.9`, `highestWastedBytes: 50MB` inicial (relaxado), tighten depois.
- **`e2e.yml`** — invoca `CI=true dive <tag>` por imagem.

#### Deep Dives
- Dive efficiency = (useful_bytes) / (total_bytes). 95% é target stretch; 90% pragmático.
- `highestWastedBytes` flagrante: devDependencies em runtime.

#### Tasks
1. Criar `.dive-ci`.
2. Adicionar step em `e2e.yml` invocando dive por imagem.
3. Iterar thresholds.

#### TDD
```
RED:     N/A (gate threshold-based)
GREEN:   Dive passa para os 48 examples; ou falha em provider com bloat (P1 issue).
REFACTOR: None expected.
VERIFY:  e2e job verde com dive step.
```

#### Acceptance Criteria
- [ ] `.dive-ci` versionado
- [ ] `lowestEfficiency` ≥ 0.9 para todos examples
- [ ] Step falha o build em violação

#### DoD
- [ ] Step verde
- [ ] Pelo menos 1 finding genuíno corrigido (ou aceito com bump de threshold + comentário)

---

## Phase 1: Artifact quality — structure-test + SBOM + test.json

**Objective:** validar a imagem produzida estruturalmente (não só runtime ad-hoc), gerar SBOM, padronizar configuração de E2E via `test.json`.

### T1.1 — `container-structure-test` infrastructure

#### Objective
Adicionar dep ao toolchain, wrapper Go que invoca, integrar ao `e2e_table_test.go`.

#### Evidence
G2.

#### Files to edit
```
mise.toml — adicionar container-structure-test como tool
e2e/structure_test.go (NEW) — wrapper Go
e2e/e2e_table_test.go — chamar wrapper quando structure-tests.yaml existe
```

#### Deep file dependency analysis
- **`structure_test.go`** — função `runStructureTest(t, tag, yamlPath)` invoca binário.
- **`e2e_table_test.go`** — opção nova em `e2eCase`: `structureTest string` (path relativo ao example dir).

#### Deep Dives
- Modo `--driver docker` (usa daemon) vs `--driver tar` (offline). Em CI L2 usar docker; offline para dev local.
- container-structure-test em maintenance mode (caveat) — aceitável; alternativa futura `goss`.

#### Tasks
1. mise.toml: adicionar tool.
2. Wrapper Go.
3. Integrar opcionalmente no runE2ECase.

#### TDD
```
RED:     TestE2E_StructureTest_RejectsMissingFile — yaml afirma file existe, image sem → falha
RED:     TestE2E_StructureTest_AcceptsExistingFile
GREEN:   Wrapper implementado.
REFACTOR: None expected.
VERIFY:  go test -tags e2e ./e2e/
```

#### Acceptance Criteria
- [ ] container-structure-test instalável via `mise install`
- [ ] Wrapper retorna erro com output legível em falha
- [ ] Opcional por example (skip se yaml ausente)

#### DoD
- [ ] Wrapper implementado e testado
- [ ] Documentado em docs/contributing

---

### T1.2 — Structure tests YAML para 10 examples críticos

#### Objective
Criar `structure-tests.yaml` para os 10 examples mais comuns (go-simple, node-npm, python-flask, java-spring-gradle, rust-axum, dotnet-aspnet, ruby-rails, php-laravel, deno-hono, node-turborepo).

#### Evidence
T1.1 cria infra; T1.2 popula.

#### Files to edit
```
examples/go-simple/structure-tests.yaml (NEW)
examples/node-npm/structure-tests.yaml (NEW)
... (10 total)
e2e/e2e_table_test.go — populate structureTest field para esses 10 cases
```

#### Deep file dependency analysis
- Cada YAML afirma:
  - **file existence**: `/app/server` (Go), `/app/index.js` (Node)
  - **metadata**: `User: appuser` (não-distroless), `User: 65532:65532` (distroless)
  - **command**: e.g., `node -e "console.log('ok')"` (Node), `php --version | grep PHP` (PHP)

#### Deep Dives
- Para distroless (Go, Rust): metadata check do User não-root; sem command (sem shell).
- Para non-distroless: command checks (process executável). File existence sempre vale.

#### Tasks
1-10. Para cada example, escrever 1 YAML com ≥ 3 asserts.

#### TDD
```
RED:     TestE2E_All/<example> — antes do yaml, passa só com verify Go atual
GREEN:   Adicionar yaml; runE2ECase invoca structure-test; ambos verdes.
REFACTOR: Gradualmente, remover verify Go quando yaml cobre o mesmo.
VERIFY:  go test -tags e2e -run TestE2E_All/<example> ./e2e/
```

#### Acceptance Criteria
- [ ] 10 yaml files criados
- [ ] Cada um com ≥ 3 asserts (file + command + metadata)
- [ ] e2eCase entries atualizadas

#### DoD
- [ ] 10 examples cobertos
- [ ] Suite verde

---

### T1.3 — SBOM via syft

#### Objective
Gerar SPDX + CycloneDX SBOM para cada imagem E2E. Output em `e2e/sboms/` (gitignored exceto para release).

#### Evidence
G9.

#### Files to edit
```
e2e/sbom.go (NEW) — wrapper syft
e2e/e2e_table_test.go — chamar wrapper após buildImage
.gitignore — adicionar e2e/sboms/
mise.toml — adicionar syft
```

#### Deep file dependency analysis
- **`sbom.go`** — função `generateSBOM(t, tag) (spdx, cyclonedx []byte)`.
- Em Phase 4 esses SBOMs viram attestations via cosign.

#### Deep Dives
- syft `<image> -o spdx-json` + `<image> -o cyclonedx-json`.
- Skip gracioso se syft ausente.
- L3 nightly arquiva SBOMs como artifacts (90d retention).

#### Tasks
1. Wrapper.
2. Integrar runE2ECase.
3. Documentar.

#### TDD
```
RED:     N/A (advisory; gera artefato)
GREEN:   Wrapper produz SBOM válido (gov via crane validate ou similar).
REFACTOR: None.
VERIFY:  e2e run + ls e2e/sboms/
```

#### Acceptance Criteria
- [ ] SBOM gerado para cada tag E2E
- [ ] Formato SPDX e CycloneDX
- [ ] Tamanho razoável (< 1MB por SBOM)

#### DoD
- [ ] Wrapper integrado
- [ ] L3 nightly arquiva SBOMs

---

### T1.4 — `test.json` schema + parser

#### Objective
Definir schema, escrever parser Go, integrar ao `runE2ECase`.

#### Evidence
G11.

#### Files to edit
```
e2e/testjson/testjson.go (NEW) — struct + Parse()
e2e/testjson/testjson_test.go (NEW)
e2e/e2e_table_test.go — opcionalmente carregar test.json se existir
```

#### Deep file dependency analysis
- **`testjson.go`** — struct com:
  ```go
  type Config struct {
      Mode          string             // "httpCheck" | "justBuild" | "expectedOutput"
      HTTPCheck     *HTTPCheckConfig   // path, port, expectedStatus
      StructureTest string             // path relativo
      Env           map[string]string
      Skip          bool
      SkipReason    string
  }
  ```
- Parser usa `tailscale/hujson` (já dep) para JSONC.

#### Deep Dives
- Reutiliza pacote `hujson` já no projeto.
- Schema documentado em comentário do struct + JSON schema separado (opcional).

#### Tasks
1. Struct + Parse() + tests.
2. Integrar opcionalmente em `runE2ECase` (fallback para e2eCase Go).

#### TDD
```
RED:     TestParseTestJSON_HTTPCheck — JSONC válido com httpCheck → struct correto
RED:     TestParseTestJSON_RejectsInvalidMode
RED:     TestParseTestJSON_SkipsCommented
GREEN:   Implementar Parse.
REFACTOR: None.
VERIFY:  go test ./e2e/testjson/
```

#### Acceptance Criteria
- [ ] Schema documentado
- [ ] Parser cobre 4 modos + skip
- [ ] Errors descritivos para JSON malformado

#### DoD
- [ ] Pacote testjson implementado
- [ ] Integração com e2e_table_test.go pronta para uso

---

### T1.5 — Migrar 20 examples para `test.json`

#### Objective
Para cada um dos 20 examples mais comuns, criar `test.json` substituindo a entry equivalente em `e2eCases`.

#### Evidence
G11. Reduz boilerplate Go; padrão Railpack.

#### Files to edit
```
examples/<name>/test.json — 20 novos arquivos
e2e/e2e_table_test.go — limpar e2eCases das entries migradas
```

#### Deep file dependency analysis
- Cada `test.json` declara `httpCheck` quando o example tem um servidor; `expectedOutput` para CLI; `justBuild` para nada.

#### Deep Dives
- Examples com framework HTTP (express, flask, fastapi, sinatra, rails, laravel, hono, etc.): `httpCheck`.
- Examples CLI puros (go-simple sem main loop): `justBuild` justificado.
- Pesquisa adverte: < 30% `justBuild` ideal.

#### Tasks
1-20. Para cada example: criar test.json + remover entry equivalente em e2eCases.

#### TDD
```
RED:     E2E roda para example antes da migração; deve continuar passando depois.
GREEN:   Migração transparente.
REFACTOR: Reduzir e2eCases para fallback-only.
VERIFY:  go test -tags e2e -run TestE2E_All/<each-migrated> ./e2e/
```

#### Acceptance Criteria
- [ ] 20 test.json criados
- [ ] e2eCases reduzido em 20 entries
- [ ] 95% dos examples migrados usam `httpCheck` (não justBuild)

#### DoD
- [ ] Migração feita
- [ ] Suite verde

---

## Phase 2: Property-based + mutation

**Objective:** elevar qualidade dos asserts via propriedades formais e mutation kill rate.

### T2.1 — `rapid` properties no `core/resolver`

#### Objective
3 propriedades:
1. **Prioridade**: theopacks > env > file > default.
2. **Idempotência**: `Resolve(Resolve(x)) == Resolve(x)`.
3. **Monotonicidade**: adicionar uma source de menor prioridade não muda resultado se já há de maior.

#### Evidence
G7.

#### Files to edit
```
go.mod — adicionar pgregory.net/rapid
core/resolver/resolver_property_test.go (NEW)
```

#### Deep file dependency analysis
- **`resolver_property_test.go`** — usa `rapid.Check`. Gera versions via `rapid.SampledFrom(semverPool)`.

#### Deep Dives
- Gerator de semver simples: `rapid.SampledFrom(["1.0.0", "2.3.4", "0.1.0", "16.18.0", ...])`. Não fuzz unicode; foco em domínio.
- Para shrinking, rapid corta valores automaticamente.

#### Tasks
1. Adicionar dep `pgregory.net/rapid`.
2. Gerator semver.
3. 3 propriedades + Run.

#### TDD
```
RED:     TestResolver_PriorityProperty — antes da impl, FAILER ou COMPILE-FAIL
GREEN:   Implementar.
REFACTOR: None.
VERIFY:  go test ./core/resolver/ -run TestResolver_.*Property
```

#### Acceptance Criteria
- [ ] 3 props passam
- [ ] rapid corpus persistido em testdata (opcional)
- [ ] Em ≥ 100 iterações por prop sem violação

#### DoD
- [ ] T2.1 verde
- [ ] Documentado em CONTRIBUTING

---

### T2.2 — `rapid` properties no `core/config::Merge`

#### Objective
2 propriedades:
1. **Associatividade**: `Merge(Merge(a,b),c) == Merge(a,Merge(b,c))`.
2. **Identidade**: `Merge(a, EmptyConfig()) == a` (módulo zero values).

#### Evidence
G7.

#### Files to edit
```
core/config/config_property_test.go (NEW)
```

#### Deep file dependency analysis
- Gerador de `Config` parcial via rapid (random fields populated).

#### Deep Dives
- Comparação via `cmp.Diff` ou reflect.DeepEqual após normalização (sort de maps).
- Edge: nil maps vs empty maps — Merge tem que ser idempotente sobre isso.

#### Tasks
1. Gerador rapid.
2. 2 propriedades.

#### TDD
```
RED:     TestConfig_MergeAssociativity — propriedade.
RED:     TestConfig_MergeIdentity
GREEN:   Já passa hoje (esperamos); se não, é bug do Merge atual.
REFACTOR: Se falhar, normalizar Merge.
VERIFY:  go test ./core/config/ -run TestConfig_.*Property
```

#### Acceptance Criteria
- [ ] 2 propriedades passam
- [ ] Iterações ≥ 100

#### DoD
- [ ] Verde
- [ ] Documentado

---

### T2.3 — `rapid` property: `Detect` estabilidade

#### Objective
Propriedade: adicionar arquivo "ruído" (que nenhum provider detecta como signal) ao project NÃO muda o resultado de `Detect`.

#### Evidence
G7.

#### Files to edit
```
core/providers/detect_property_test.go (NEW)
```

#### Deep file dependency analysis
- Gerador: nome de arquivo random não em [package.json, go.mod, Cargo.toml, ...]. Conteúdo arbitrário.
- Compara `Detect(dir)` antes/depois.

#### Deep Dives
- Arquivos a evitar: lista positiva de detectors (extraida de cada provider).
- README.md, LICENSE, .gitignore — exemplo "noise".

#### Tasks
1. Lista de detector signals.
2. Gerador rapid.
3. Property.

#### TDD
```
RED:     TestProviders_DetectionStable — antes do gerador, COMPILE-FAIL
GREEN:   Implementar e validar.
REFACTOR: None.
VERIFY:  go test ./core/providers/ -run TestProviders_DetectionStable
```

#### Acceptance Criteria
- [ ] Propriedade roda contra todos 11 providers
- [ ] Zero violações em 100 iter

#### DoD
- [ ] Verde

---

### T2.4 — `go-gremlins` mutation testing

#### Objective
Adicionar gremlins ao toolchain, configurar para `core/providers`, `core/resolver`, `core/config`, definir baseline.

#### Evidence
G8.

#### Files to edit
```
.gremlins.yaml (NEW)
mise.toml — task `mutation` que roda gremlins
.github/workflows/nightly.yml (NEW) — L4 job
docs/benchmarks/mutation-baseline.txt (NEW)
```

#### Deep file dependency analysis
- **`.gremlins.yaml`** — config: include patterns, mutation operators, timeout.
- **`nightly.yml`** — job que roda gremlins e reporta.

#### Deep Dives
- Operadores gremlins: arithmetic, conditional, increment, etc. Comece com defaults.
- Baseline: aceitar score atual; iterar para 70%.

#### Tasks
1. Install gremlins via mise.
2. Config.
3. Run inicial; gravar baseline.
4. Nightly job.

#### TDD
```
RED:     N/A (mutation report, não test direto)
GREEN:   Run completa; score acima de 50% inicial.
REFACTOR: Tighten tests para subir score.
VERIFY:  gremlins unleash --output=mutation.json
```

#### Acceptance Criteria
- [ ] gremlins instalado
- [ ] Run completa em < 30min nos 3 dirs
- [ ] Baseline registrado

#### DoD
- [ ] Job nightly verde
- [ ] Issue P1 aberto para mutantes LIVED suspeitos

---

## Phase 3: Differential + reproducibility

**Objective:** garantir que o que CI valida = o que prod executa.

### T3.1 — Kaniko differential test

#### Objective
Test E2E `e2e_kaniko_test.go` (build tag `e2e_kaniko`) que builda 10 examples críticos com Kaniko + valida parity.

#### Evidence
G5. Critical path antes de v1.0.

#### Files to edit
```
e2e/e2e_kaniko_test.go (NEW)
e2e/kaniko.go (NEW) — wrapper que invoca gcr.io/kaniko-project/executor via docker
.github/workflows/e2e-kaniko.yml (NEW) — L3 nightly
```

#### Deep file dependency analysis
- **`kaniko.go::buildWithKaniko(t, dockerfile, contextDir, tag)`** — usa Docker para rodar executor de Kaniko (sem precisar daemon Kubernetes).
- **`e2e_kaniko_test.go`** — itera 10 examples, builda com Kaniko, compara structure-test result com BuildKit.

#### Deep Dives
- Kaniko não suporta cache mounts BuildKit-style nativamente; algumas regressões silenciosas potenciais. Hoje todos providers usam `--mount=type=cache,target=...`. Quando Kaniko ignora, build ainda funciona (sem cache); validar.
- Imagem `gcr.io/kaniko-project/executor:debug-v1.23.2` (latest tag estável).

#### Tasks
1. Wrapper Kaniko via docker run.
2. Lista de 10 examples críticos.
3. Test loop.
4. Workflow CI.

#### TDD
```
RED:     TestE2E_Kaniko_Builds_GoSimple — Kaniko falha hoje? Se sim, é bug do provider.
GREEN:   Implementar wrapper; rodar; corrigir provider se necessário.
REFACTOR: None.
VERIFY:  go test -tags e2e_kaniko ./e2e/
```

#### Acceptance Criteria
- [ ] 10 examples buildam com Kaniko
- [ ] Structure tests passam tanto para BuildKit quanto Kaniko
- [ ] Nightly job documentado

#### DoD
- [ ] Verde
- [ ] Differencas documentadas (cache mount comportamento)

---

### T3.2 — Reproducibility test

#### Objective
3 examples buildados 2x com `SOURCE_DATE_EPOCH=1700000000` produzem digests idênticos.

#### Evidence
G6.

#### Files to edit
```
e2e/e2e_reproducible_test.go (NEW)
e2e/reproducibility.go (NEW) — buildAndDigest helper
```

#### Deep file dependency analysis
- **`reproducibility.go::buildAndDigest(t, df, tag, epoch)`** — buildx build com `--build-arg SOURCE_DATE_EPOCH` + `--output type=image,rewrite-timestamp=true`, captura digest via `docker inspect`.

#### Deep Dives
- BuildKit ≥ 0.13 required (config no CI).
- 3 examples: go-simple (binário static), node-npm (com timestamps), python-flask (pip install reproducible).

#### Tasks
1. Helper.
2. 3 tests.
3. CI L4 job.

#### TDD
```
RED:     TestE2E_Reproducible_GoSimple — antes da impl, COMPILE-FAIL
GREEN:   Implementar; digests devem coincidir.
REFACTOR: None.
VERIFY:  go test -tags e2e -run TestE2E_Reproducible ./e2e/
```

#### Acceptance Criteria
- [ ] 3 tests
- [ ] Cada um: digest_1 == digest_2
- [ ] BuildKit ≥ 0.13 disponível no CI

#### DoD
- [ ] Verde
- [ ] Documentado como invariante de release

---

### T3.3 — `diffoci` diagnose helper

#### Objective
Quando T3.2 falhar, fornecer helper que invoca `diffoci` para mostrar diff bit-a-bit.

#### Evidence
Caveat da pesquisa: quando reprodutibilidade quebra, debugging manual é caro.

#### Files to edit
```
e2e/diffoci.go (NEW)
```

#### Deep file dependency analysis
- **`diffoci.go::diagnoseNonReproducible(t, tag1, tag2)`** — invoca `diffoci diff <tag1> <tag2>` e emite output em t.Logf.

#### Deep Dives
- `reproducible-containers/diffoci` instalado via mise; chamado só em falha de T3.2.

#### Tasks
1. Wrapper.
2. Integrar em TestE2E_Reproducible.

#### TDD
```
RED:     N/A (advisory; emite output)
GREEN:   Quando T3.2 falha, diagnose roda e produz output legível.
REFACTOR: None.
VERIFY:  forçar não-reprodutibilidade (mudar arquivo entre builds); diagnose mostra diff.
```

#### Acceptance Criteria
- [ ] Helper integrado
- [ ] Output legível em falha

#### DoD
- [ ] Verde
- [ ] Documentado em CONTRIBUTING

---

## Phase 4: Supply chain — SLSA L3 + Cosign + Attestations

**Objective:** releases assinadas, verificáveis, com provenance non-falsifiable.

### T4.1 — SLSA L3 provenance via GitHub Actions

#### Objective
Tag push → SLSA GitHub Generator produz `multiple.intoto.jsonl`.

#### Evidence
G10.

#### Files to edit
```
.github/workflows/release.yml (NEW)
```

#### Deep file dependency analysis
- Usa `slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml`.
- Hash do binário theopacks-generate input.

#### Deep Dives
- SLSA L3 requirements: build isolation (GHA runners isolados), non-falsifiable provenance (gerado pela infra do GH, não pelo dev).
- Output: `attestation.intoto.jsonl` anexado ao release.

#### Tasks
1. Workflow release.
2. Trigger em `v*` tag.
3. Validar local com `slsa-verifier`.

#### TDD
```
RED:     Sem workflow, release manual sem attestation.
GREEN:   Tag push gera attestation.
REFACTOR: None.
VERIFY:  slsa-verifier verify-artifact <bin> --provenance-path <intoto.jsonl>
```

#### Acceptance Criteria
- [ ] Workflow existe
- [ ] Provenance gerada em test release (tag pre-release)

#### DoD
- [ ] Verde
- [ ] Documentado em release process

---

### T4.2 — Cosign keyless signing

#### Objective
Imagens de release (theo-packs-runner) assinadas via OIDC do GitHub Actions.

#### Evidence
G10.

#### Files to edit
```
.github/workflows/release.yml — adicionar step cosign
```

#### Deep file dependency analysis
- `sigstore/cosign-installer@<sha>` action.
- `cosign sign --yes <image>@<digest>`.

#### Deep Dives
- Keyless usa OIDC token do GHA runner como identity. Sigstore Rekor log público.

#### Tasks
1. cosign install action.
2. Sign step.
3. Verify step (sanity).

#### TDD
```
RED:     Imagens sem signature.
GREEN:   Pós-release, cosign verify passa.
REFACTOR: None.
VERIFY:  cosign verify <image> --certificate-identity-regexp '...' --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

#### Acceptance Criteria
- [ ] Imagem assinada
- [ ] verify passa

#### DoD
- [ ] Workflow completo
- [ ] Documentado

---

### T4.3 — SBOM attestations

#### Objective
SBOMs (SPDX + CycloneDX) gerados em T1.3 são anexados como attestations via `cosign attest --predicate <sbom> --type spdxjson`.

#### Evidence
G9 + G10.

#### Files to edit
```
.github/workflows/release.yml — step "Attest SBOM"
```

#### Tasks
1. Gerar SBOMs do runner.
2. cosign attest x2 (SPDX + CycloneDX).

#### TDD
```
RED:     Sem attestation.
GREEN:   cosign verify-attestation retorna SBOM válido.
REFACTOR: None.
VERIFY:  cosign verify-attestation --type spdxjson <image>
```

#### Acceptance Criteria
- [ ] SBOM SPDX anexado
- [ ] SBOM CycloneDX anexado
- [ ] Ambos verify

#### DoD
- [ ] Workflow integrado
- [ ] Documentado

---

### T4.4 — Release verify gate

#### Objective
Step final em release que: `slsa-verifier` + `cosign verify` + `cosign verify-attestation`. Falha o release se algum falhar.

#### Evidence
Critical path — gate hard.

#### Files to edit
```
.github/workflows/release.yml — step "Verify supply chain"
```

#### Tasks
1. Step que roda os 3 verifies.

#### TDD
```
RED:     N/A (gate-only)
GREEN:   Release verde.
REFACTOR: None.
VERIFY:  release pre-tag dry-run.
```

#### Acceptance Criteria
- [ ] Gate impede release sem provenance + signatures + SBOM

#### DoD
- [ ] Verde
- [ ] Documentado

---

## Phase 5: CI orchestration

**Objective:** estruturar CI em 5 camadas com sharding.

### T5.1 — Split workflows em L1/L2/L3

#### Objective
`ci.yml` permanece L1. Novos: `e2e-l2.yml` (PR integration, sharded), `e2e-l3.yml` (full matrix, main+nightly).

#### Evidence
G12 + estratégia pyramid da pesquisa.

#### Files to edit
```
.github/workflows/ci.yml — limpar para L1 (fast: vet, lint, test -short, hadolint)
.github/workflows/e2e-l2.yml (NEW) — sharded, BuildKit only, 3-5 examples por provider
.github/workflows/e2e-l3.yml (NEW) — full, BuildKit + Kaniko, amd64 + arm64
```

#### Tasks
1. Refactor ci.yml.
2. Criar e2e-l2.yml.
3. Criar e2e-l3.yml.

#### TDD
```
RED:     N/A (refactor workflow)
GREEN:   PR PR roda L1+L2 em tempo target.
REFACTOR: None.
VERIFY:  PR experimental.
```

#### Acceptance Criteria
- [ ] L1 < 3min
- [ ] L2 < 10min
- [ ] L3 < 60min

#### DoD
- [ ] Workflows estabilizados em main

---

### T5.2 — Sharding helpers + matrix

#### Objective
Implementar `-shard=N/M` no runner E2E via `fnv32(example) % M == N`.

#### Evidence
G12. Pesquisa cita matrix 8-way.

#### Files to edit
```
e2e/sharding.go (NEW) — função filterShard
e2e/e2e_table_test.go — aplicar filtro se TEST_SHARD env definido
.github/workflows/e2e-l3.yml — matrix com 8 shards
```

#### Tasks
1. Helper sharding.
2. Matrix CI.

#### TDD
```
RED:     TestSharding_ConsistentDistribution — fnv32 distribui ~uniforme.
GREEN:   Implementar.
REFACTOR: None.
VERIFY:  go test ./e2e/ -run TestSharding (não-tagged)
```

#### Acceptance Criteria
- [ ] Distribuição razoavelmente uniforme (max 1.5× min)
- [ ] L3 reduz tempo proporcionalmente

#### DoD
- [ ] Verde

---

### T5.3 — L4 nightly: gremlins + dive + trivy compliance + reproducibility

#### Objective
Workflow `nightly.yml` orquestra os jobs caros.

#### Evidence
G8, G6, G4, T3.2.

#### Files to edit
```
.github/workflows/nightly.yml (NEW)
```

#### Tasks
1. Schedule cron diário.
2. Jobs: gremlins, dive, trivy compliance, reproducibility.

#### TDD
```
RED:     N/A (orchestration)
GREEN:   Nightly verde.
REFACTOR: None.
VERIFY:  cron trigger manual.
```

#### Acceptance Criteria
- [ ] Roda em < 90min total
- [ ] Reporta findings via issue automation

#### DoD
- [ ] Verde

---

### T5.4 — L5 release: SLSA + Cosign + SBOM + verify

#### Objective
`release.yml` consolida T4.1-T4.4.

#### Evidence
G10.

#### Files to edit
```
.github/workflows/release.yml — consolidado
```

#### Tasks
1. Single workflow.
2. Tag-triggered.

#### TDD
```
RED:     Sem tag, sem release.
GREEN:   Tag push → release com all artifacts.
REFACTOR: None.
VERIFY:  pre-release tag.
```

#### Acceptance Criteria
- [ ] Release contém: binary, SBOM SPDX, SBOM CycloneDX, intoto attestation, signed image
- [ ] cosign verify-attestation passa

#### DoD
- [ ] Test release feito
- [ ] Documentado

---

### T5.5 — Flakiness tracking

#### Objective
Workflow `flakiness.yml` que roda integration suite 5x e reporta falhas inconsistentes.

#### Evidence
G13.

#### Files to edit
```
.github/workflows/flakiness.yml (NEW)
scripts/flakiness-report.sh (NEW) — agrega resultados
```

#### Tasks
1. Workflow weekly.
2. Aggregator.
3. Issue automation (cria issue se test flaky > 5%).

#### TDD
```
RED:     N/A
GREEN:   Workflow verde; report gerado.
REFACTOR: None.
VERIFY:  manual trigger.
```

#### Acceptance Criteria
- [ ] Reporta flakiness rate por test
- [ ] Issue auto para flaky > 5%

#### DoD
- [ ] Verde
- [ ] Dashboard inicial

---

## Coverage Matrix

| # | Gap (severity) | Task(s) | Resolution |
|---|---|---|---|
| G1 | Sem hadolint (Alta) | T0.1, T0.2 | Gate L1 sobre goldens + E2E |
| G2 | Sem container-structure-test (Alta) | T1.1, T1.2 | Infra + 10 yaml per-example |
| G3 | Sem Trivy (Crítica) | T0.3, T5.3, T5.4 | Advisory L1 + nightly compliance + release gate |
| G4 | Sem dive (Média) | T0.4 | `.dive-ci` thresholds |
| G5 | Sem Kaniko differential (Crítica) | T3.1 | Build tag `e2e_kaniko` + 10 examples |
| G6 | Sem reproducibility (Média) | T3.2, T3.3 | SOURCE_DATE_EPOCH × 3 ex + diffoci |
| G7 | Sem property-based (Alta) | T2.1, T2.2, T2.3 | rapid no resolver/config/detect |
| G8 | Sem mutation testing (Média) | T2.4 | gremlins nightly + baseline |
| G9 | Sem SBOM (Alta) | T1.3, T4.3 | syft em E2E + cosign attest no release |
| G10 | Sem Cosign/SLSA (Alta) | T4.1, T4.2, T4.3, T4.4 | SLSA L3 + keyless + attestations + verify |
| G11 | Sem test.json (Média) | T1.4, T1.5 | Schema + parser + 20 examples migrados |
| G12 | Sem sharding L3 (Média) | T5.1, T5.2 | Workflow split + matrix |
| G13 | Sem flakiness tracking (Baixa) | T5.5 | Weekly + issue automation |
| G14 | Sem testcontainers (Média) | T1.4 (test.json httpCheck cobre o uso comum) | Parcial — httpCheck do test.json roda runtime; testcontainers como follow-up se DB needed |
| G15 | Sem OCI validate (Baixa) | T4.4 (verify inclui crane validate) | Verify gate cobre |

**Coverage: 15/15 (100%)**

## Global Definition of Done

- [ ] Todas as 5 phases completas
- [ ] CI em 5 camadas operacional (L1 < 3min, L2 < 10min, L3 < 60min, L4 nightly, L5 tag)
- [ ] Métricas-alvo:
  - [ ] Cobertura linhas `core/providers/*` ≥ 85%
  - [ ] Mutation score em `core/providers`, `core/resolver`, `core/config` ≥ 70%
  - [ ] Hadolint zero warnings ≥ warning em goldens + E2E
  - [ ] Trivy zero HIGH/CRITICAL em release
  - [ ] Dive efficiency ≥ 90%
  - [ ] Reprodutibilidade: 3 examples × 2 builds idênticos
  - [ ] ≥ 10 examples cobertos por Kaniko differential
  - [ ] Property tests: 3+3+1 = 7 propriedades verificadas
  - [ ] Flakiness rate < 1% em 100 runs
- [ ] Supply chain:
  - [ ] SLSA L3 provenance em release
  - [ ] Cosign keyless signing
  - [ ] SBOM SPDX + CycloneDX anexados
- [ ] Documentação:
  - [ ] CONTRIBUTING atualizado com instruções de cada ferramenta
  - [ ] CLAUDE.md com referências às fases
  - [ ] CHANGELOG documenta cada fase no [Unreleased]

## Resumo executivo

**20 tasks em 5 phases.** Fases 0-1 (artefato + estrutura) entregam baseline em ~4 semanas. Fase 2 (property + mutation) eleva qualidade dos asserts em ~2 semanas paralelas. Fase 3 (Kaniko + reproducibility) fecha o gap crítico de divergência prod vs CI em ~2 semanas paralelas. Fase 4 (supply chain) habilita release verificável em ~2 semanas sequenciais. Fase 5 (CI orchestration) consolida tudo em ~1 semana.

**Total estimado:** 8-11 semanas com 1 engenheiro; 4-6 semanas com 2 engenheiros paralelizando Phase 2 + Phase 3.

**Risco principal:** Phase 3 (Kaniko differential) pode revelar bugs reais nos providers (cache mount semantics, secrets). Mitigação: rodar primeiro localmente; corrigir provider-by-provider.

**Sequência recomendada:** Phase 0 → Phase 1 → (Phase 2 ∥ Phase 3) → Phase 4 → Phase 5. Phase 0 sozinha (≤1 semana) já endereça G1, G3 advisory e G4 — ROI imediato.
