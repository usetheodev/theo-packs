# Plan: Test Suite Hardening — E2E Coverage & Regression Quality

> **Version 1.0** — Endereça os 13 gaps identificados na análise de qualidade dos testes E2E e de regressão de 2026-05-20 (continuação imediata do `deep-review-hardening-plan.md`). Foca em quatro frentes: (a) fechar a cobertura E2E que hoje deixa 28 dos 48 examples sem teste de build Docker real; (b) blindar a suíte contra regressões críticas já vistas (secret-mount pollution C1, contrato F3, determinismo do gerador); (c) hardening da infraestrutura de teste (race detector, paralelização, build cache, t.Cleanup); (d) defesa adversarial (fuzz no shellEscape e na CLI). Saída esperada: cobertura E2E ≥ 95% dos examples, suíte rodando 30-40% mais rápido em paralelo, fuzz corpus mantido em CI, e nenhum dos quatro vetores CRITICAL do review anterior pode regressar sem o teste correspondente quebrar.

## Context

A análise de qualidade dos testes em 2026-05-20 (registrada na conversa que originou este plano e referenciada por `docs/plans/deep-review-hardening-plan.md`) mapeou:

- **678 testes** em 56 arquivos, distribuídos em 4 camadas (unitários, in-process E2E, golden corpus, CLI subprocess, E2E Docker).
- **20 testes E2E reais** (`e2e/e2e_test.go`, build tag `e2e`) cobrindo apenas **20 de 48 examples** (~42%).
- **`t.Parallel()` em apenas 1 arquivo** (`core/generate/images_test.go`) entre 56.
- **`mise.toml::test` sem `-race`** — data races em `App.globCache` / `Resolver` passariam despercebidos.
- **`TestE2E_MonorepoTurboFromStacks`** skipa silenciosamente em CI quando o checkout sibling `theo-stacks/` não existe — o contrato F3 (build context = workspace root) **não está garantido em CI**.
- **`core/integration_test.go::expectedProvider`** usa `t.Logf("skipping unknown example %q ...")` quando um provider novo não tem mapeamento. Examples `dotnet-*`, `java-*`, `php-*`, `ruby-*`, `rust-*`, `deno-*` são silenciosamente skipados.
- **`monorepo_test.go:102`** e companhia testam literais como `result.Plan.Deploy.StartCmd == "npm start"`. Refactor para `node index.js` (potencialmente uma melhoria) quebraria 30+ testes — o anti-pattern explícito da Rule 7 do CLAUDE global ("testes que testam implementação em vez de comportamento").
- **CLI subprocess tests recompilam o binário em cada teste** via `buildBinary(t)`. ~20 testes × ~0.5s = ~10s gastos só com `go build`.
- **Cleanup E2E via `defer`** em 7 testes antigos: defer não roda quando o test é Skipped, e em panic do setup pode vazar imagens.
- **Sem fuzz** no `shellEscape` ou em `validateCLIInput`/`clampPath` — escapes adversariais e bypasses de regex precisariam ser descobertos em produção.
- **Sem benchmark** de `core.GenerateBuildPlan` ou `dockerfile.Generate` — regressões de performance (FindFiles N², readJSON repetido) ficariam invisíveis.
- **Sem teste de regressão E2E** que afirma "Dockerfile do turborepo NÃO contém `--mount=type=secret,id=THEOPACKS_*`" — o C1 do deep-review-hardening está coberto só por unit tests, sem guarda end-to-end.
- **`TestE2E_ShellScript_BuildsImage`** injeta `THEOPACKS_START_CMD=bash start.sh` — não testa a auto-detecção do shell provider, apenas que a imagem builda.

Estes gaps existem hoje, no commit `9e0eafd` (head de `main`).

## Objective

**Done** = `mise run check && mise run test && mise run test-e2e` verde com (1) ≥ 95% dos examples cobertos por E2E Docker real, (2) `-race` ativo em todos os runs, (3) o teste de contrato turbo não skipando em CI, (4) cada CRITICAL/HIGH do `deep-review-hardening-plan.md` blindado por E2E regression test, (5) fuzz corpus persistido para `shellEscape` e CLI inputs.

Metas mensuráveis:

1. **Cobertura E2E ≥ 45/48 examples** (excluir só os que dependem de credenciais ou são fixtures de teste).
2. **`go test -race ./...` verde** — `mise.toml::test` atualizado.
3. **`TestE2E_MonorepoTurboFromStacks` não-skipável** — fixture interna substitui dependência de checkout externo.
4. **`TestRegression_NoSpuriousSecretMounts_Turborepo`** existe e falha quando `--mount=type=secret,id=THEOPACKS_*` reaparece.
5. **`TestRegression_PlanGeneration_IsDeterministic`** existe — gera 2× o mesmo source e compara byte-a-byte.
6. **`FuzzShellEscape_RoundTripsThroughSh`** + **`FuzzValidateCLIInput_NeverPanics`** + **`FuzzClampPath_NeverEscapes`** existem e rodam ≥ 30s no CI.
7. **CLI subprocess suite ≤ 50% do tempo atual** via `TestMain` que compila o binário uma vez.
8. **`t.Parallel()` em ≥ 10 arquivos** entre os 56 (paralelização onde seguro).
9. **`expectedProvider` exaustivo** — `t.Logf` substituído por `require.NotEmpty` + mapeamento para os 11 providers.
10. **`monorepo_test.go` migra de "exato literal" para "matcher de comportamento"** em ≥ 30 sites.

## ADRs

### D1 — E2E smoke skeleton via tabela de fixtures

**Decisão:** introduzir uma estrutura tabular `[]e2eCase{exampleName, tag, env, verify}` em `e2e/e2e_test.go` e drivers separados (`runE2EBuild` já existe) para que adicionar cobertura a um novo example seja uma entry na tabela, não uma nova função `TestE2E_*_BuildsImage`. Cada case opcionalmente declara assertions específicas via `verify func(*testing.T, tag string)`.

**Rationale:** Hoje cada example tem uma função dedicada (`TestE2E_GoSimple_BuildsImage`, `TestE2E_NodeNpm_BuildsImage`...). Adicionar 28 examples nessa forma duplica boilerplate. Tabela + dispatcher minimiza fricção. Alternativa rejeitada: gerar testes via `go generate` — adiciona complexidade de toolchain por economia marginal.

**Consequências:** Diff de cobertura nova fica pequeno (adicionar uma linha). Trade-off: subtest names ficam na forma `TestE2E_All/<example>`, menos buscáveis que função top-level. Aceitável — `go test -run "TestE2E_All/node-vite-react"` ainda funciona.

### D2 — Embedar fixture turbo via `//go:embed`

**Decisão:** copiar o template `monorepo-turbo` de `theo-stacks/` para `e2e/testdata/monorepo-turbo/` e usar `//go:embed all:testdata/monorepo-turbo/*` para materializá-lo em runtime. Remove a dependência de checkout sibling.

**Rationale:** O contrato F3 (build context = workspace root) é crítico e não pode skipar em CI. Alternativas: (a) shallow clone via `git clone` em `TestMain` — adiciona dep externa + flakiness, (b) submodule — complica releases do theo-packs. Embed é hermético, rastreável via git, e o template é pequeno (~50 arquivos).

**Consequências:** Quando o template upstream evoluir, atualizar a cópia interna é uma operação manual (script `tools/sync-theo-stacks-fixture.sh` opcional). Mas a regressão do contrato fica blindada continuamente.

### D3 — `-race` por padrão no `mise run test`

**Decisão:** `mise.toml::test` muda para `go test -race ./core/... ./cmd/...`. Tests E2E (`test-e2e`) também ganham `-race`.

**Rationale:** App.globCache, Resolver, Logger não usam mutex. CLI single-thread não dispara, mas providers podem paralelizar no futuro e a defesa precisa estar no lugar antes. Custo: `-race` adiciona ~2× tempo de teste; aceitável para 678 testes que rodam em < 60s.

**Consequências:** Se algum race vier à tona durante a transição, é bug genuíno que precisa fix antes de mergear este plano.

### D4 — Behavior matchers via tipo `StartCmdMatcher`

**Decisão:** Introduzir helper `assertReasonableStart(t, plan, provider)` que valida `Deploy.StartCmd` contra um conjunto sensato por linguagem (`npm start | node index.js | node server.js` para Node, `/app/server` para Go, etc.) em vez de string literal.

**Rationale:** Rule 7 anti-pattern: testes acoplados a implementação quebram em refactor legítimo. Alternativa rejeitada: regex genérica — perde precisão. Matcher por provider é o ponto de equilíbrio.

**Consequências:** Refactor do start command de um provider continua possível sem quebrar 30+ tests, desde que o novo valor seja "razoável".

### D5 — TestMain compartilhado para CLI subprocess tests

**Decisão:** `cmd/theopacks-generate/testmain_test.go` (NEW) com `TestMain(m *testing.M)` que compila o binário uma vez para `os.TempDir()/theopacks-generate-test` e o expõe via global `var sharedBinary string`. `buildBinary(t)` passa a retornar `sharedBinary`.

**Rationale:** ~20 testes × ~500ms `go build` = 10s desperdiçados em cada `go test ./cmd/...`. Compartilhar é seguro porque os testes não modificam o binário.

**Consequências:** Mudanças no código fonte exigem `go test` rebuilds (ok — Go já faz isso). Trade-off: paralelização de subtests fica trivial após esse refactor.

### D6 — Fuzz corpus mantido sob `testdata/fuzz/`

**Decisão:** seguir convenção padrão Go: `FuzzShellEscape_RoundTripsThroughSh` em `core/dockerfile/shell_escape_test.go`, corpus em `core/dockerfile/testdata/fuzz/FuzzShellEscape_RoundTripsThroughSh/`. Idem para fuzz da CLI. Seedar com casos conhecidos do review (`'`, `''`, `\n`, `$VAR`, etc.).

**Rationale:** Convenção idiomática Go (`go test -fuzz=Fuzz...`). Corpus comitado garante que crashes encontrados em runs anteriores são re-validados em todo CI.

**Consequências:** CI roda fuzz em modo "regression" (apenas corpus) por padrão — rápido. Fuzz exploratório (`-fuzz=...`) só roda quando explicitamente solicitado.

## Dependency Graph

```
Phase 0: Quick wins (paralelo)
  │      T0.1 expectedProvider exaustivo
  │      T0.2 Determinism regression test
  │      T0.3 Secret-mount E2E regression
  │      T0.4 Enable -race
  ▼
Phase 1: E2E coverage  (sequential — após Phase 0 confirmar baseline)
  │      T1.1 E2E table-driven refactor (D1)
  │      T1.2 Node examples coverage
  │      T1.3 Python examples coverage
  │      T1.4 Remaining (dotnet/deno/ruby/php/rust) coverage
  │      T1.5 Turbo fixture embed (D2)
  │      T1.6 Shell provider real E2E (remove hack)
  ▼
Phase 2: Test infrastructure (paralelo)
  │      T2.1 TestMain shared binary (D5)
  │      T2.2 defer → t.Cleanup migration
  │      T2.3 t.Parallel onde seguro
  ▼
Phase 3: Adversarial defense (paralelo)
  │      T3.1 Fuzz shellEscape
  │      T3.2 Fuzz validateCLIInput
  │      T3.3 Fuzz clampPath
  │      T3.4 Benchmark generation
  ▼
Phase 4: Behavior refactor (sequential — maior risco de regressão)
        T4.1 Behavior matchers em monorepo_test.go
        T4.2 Behavior matchers em dogfood_test.go
```

Phase 0 paraleliza tudo internamente. Phase 1 é sequencial porque T1.1 cria infra que as demais consomem. Phase 2/3 paralelizam internamente. Phase 4 é última porque toca testes que já passam — qualquer falha indica regressão genuína, não defeito do plano.

---

## Phase 0: Quick wins

**Objective:** fechar gaps onde o custo é XS e o risco de regressão é zero. Estabelece baseline antes das mudanças maiores.

### T0.1 — `expectedProvider` exaustivo + fail loud

#### Objective
Forçar `core/integration_test.go::expectedProvider` a mapear todos os 11 providers, e trocar `t.Logf("skipping ...")` por `require.NotEmpty` para que examples sem mapeamento causem falha visível, não silêncio.

#### Evidence
`core/integration_test.go:39-42`:
```go
default:
    return ""
```
e `:91-95`:
```go
if prov == "" {
    t.Logf("skipping unknown example %q (no expected provider mapping)", dirName)
    continue
}
```
Examples `rust-*`, `java-*`, `dotnet-*`, `ruby-*`, `php-*`, `deno-*` são silenciosamente skipados. `core/providers/provider.go::GetLanguageProviders` registra 11 providers, mas o test só conhece 5.

#### Files to edit
```
core/integration_test.go — expandir expectedProvider() para 11 providers; trocar t.Logf por require
```

#### Deep file dependency analysis
- **`core/integration_test.go`** — único arquivo onde o mapeamento existe. Mudança local; sem deps downstream.
- Examples diretório (`examples/`) é a fonte da verdade para nomes — sem mudança.
- Providers (`core/providers/provider.go::GetLanguageProviders`) é a fonte da verdade para registro — sem mudança.

#### Deep Dives
Mapeamento completo:
- Go (`go-*`), Node (`node-*`), Python (`python-*`), Rust (`rust-*`), Java (`java-*`), .NET (`dotnet-*`), Ruby (`ruby-*`), PHP (`php-*`), Deno (`deno-*`), Static (`staticfile`), Shell (`shell-script`).
- `fullstack-mixed` continua tratado especialmente (já existe lógica).

Invariante: para cada `entry` em `examples/`, `expectedProvider(entry.Name())` deve retornar nome não-vazio (exceto `fullstack-mixed`).

#### Tasks
1. Adicionar 6 cases ao switch em `expectedProvider` (rust, java, dotnet, ruby, php, deno).
2. Substituir `t.Logf` por `require.NotEmpty(t, prov, "example %q has no provider mapping — add it to expectedProvider()", dirName)`.
3. Rodar `go test ./core/ -run TestIntegrationExamples` e verificar zero skips.

#### TDD
```
RED:     TestIntegrationExamples — atualmente skipa examples não-mapeados; refactor deve fazê-los EXECUTAR (e passar)
GREEN:   Expansão do switch.
REFACTOR: None expected.
VERIFY:  go test ./core/ -run TestIntegrationExamples -v | grep -c "PASS:" → ≥ 48
```

#### Acceptance Criteria
- [ ] Switch em `expectedProvider` cobre 11 providers
- [ ] `require.NotEmpty` substitui `t.Logf` no skip
- [ ] `go test ./core/ -run TestIntegrationExamples -v` mostra PASS para todos os ~48 subtests (não Skip)
- [ ] `grep -c "no provider mapping" core/integration_test.go` retorna 1 (apenas na msg de erro)

#### DoD
- [ ] Mudanças aplicadas
- [ ] Subtests executam para todos os providers
- [ ] `go vet ./core/...` zero

---

### T0.2 — `TestRegression_PlanGeneration_IsDeterministic`

#### Objective
Garantir que `core.GenerateBuildPlan` produz output byte-idêntico em rodadas consecutivas contra o mesmo source. Captura regressões de map-iteration-order.

#### Evidence
Os providers e o renderer usam `sort.Strings`/`sort.Slice` em vários lugares (ex: `dockerfile/generate.go:222` para env vars). Mas não há teste que assegure que `dockerfile.Generate(plan1) == dockerfile.Generate(plan2)` após `plan1, _ := core.GenerateBuildPlan(...)` e `plan2, _ := core.GenerateBuildPlan(...)`. Uma future PR que esqueça um `sort` introduz flakiness silenciosa.

#### Files to edit
```
core/dockerfile/determinism_test.go (NEW) — gera 10× o mesmo plano e compara
```

#### Deep file dependency analysis
- **`determinism_test.go` (NEW)** — depende de `core`, `app`, `dockerfile`. Sem mudança em outros arquivos.
- Roda contra examples reais (`examplesDir(t)` já existe em `core/dockerfile/integration_test.go`).

#### Deep Dives
Estratégia: executar 10 vezes contra `node-turborepo` (suficientemente complexo: workspace, múltiplos manifestos, caches, secrets). Comparar `dockerfile.Generate(result.Plan)` byte-a-byte. Falha implica não-determinismo.

Edge case: se o resolver de packages atinge rede (tem internet calls?), o test pode ficar lento. Sanity: `Resolver.Default` é local; sem rede.

#### Tasks
1. Criar `core/dockerfile/determinism_test.go`.
2. Helper `generateNTimes(t, exampleName, n)` retorna `[]string` com os N outputs.
3. Assert: todos iguais ao primeiro.
4. Cobrir 3 examples: `node-turborepo`, `python-flask`, `go-workspaces` (variedade de provider).

#### TDD
```
RED:     TestRegression_NodeTurborepo_DeterministicOutput — gera 10× → todas iguais
RED:     TestRegression_PythonFlask_DeterministicOutput
RED:     TestRegression_GoWorkspaces_DeterministicOutput
GREEN:   Hoje passa (com sorts atuais). Test é guard rail.
REFACTOR: None expected.
VERIFY:  go test ./core/dockerfile/ -run TestRegression_.*Deterministic
```

#### Acceptance Criteria
- [ ] 3 testes existem e passam
- [ ] Cada teste roda ≥ 10 iterações
- [ ] Falha do test deixa diff visível (use `cmp.Diff` ou similar)

#### DoD
- [ ] Testes verdes em main
- [ ] Diff legível em caso de falha

---

### T0.3 — `TestRegression_NoSpuriousSecretMounts_Turborepo`

#### Objective
Blindar o fix do C1 (deep-review-hardening T1.4): garantir que o Dockerfile gerado para `node-turborepo --app-path apps/api` NÃO contém `--mount=type=secret,id=THEOPACKS_*`.

#### Evidence
`core/core_test.go::TestGenerateConfigFromEnvironment_FiltersInternalKeys` cobre o filtro na função, mas não há teste end-to-end pelo binário. Uma futura mudança no `step.Secrets = ["*"]` default ou no `applyConfig` que re-promova env keys passaria nos unit tests mas reintroduziria a regressão observada em 2026-05-20.

#### Files to edit
```
core/dockerfile/regression_test.go (NEW) — gera turborepo via lib e faz grep
```

#### Deep file dependency analysis
- **`regression_test.go` (NEW)** — usa `core.GenerateBuildPlan` + `dockerfile.Generate`. Sem mudança em outros arquivos.
- Roda contra `examples/node-turborepo` real.

#### Deep Dives
Pattern a detectar: `--mount=type=secret,id=THEOPACKS_`
Implementação:
```go
df := generateForTurborepo(t, "apps/api", "api")
require.NotContains(t, df, "--mount=type=secret,id=THEOPACKS_",
    "spurious THEOPACKS_* secret mount — see C1 in deep-review-hardening-plan")
```

Cobrir também:
- `WorkspaceTarget` via Options (T3.1 path)
- env-var bridge (legacy path)
- Ambos juntos

Cada um deve produzir zero mount=secret,id=THEOPACKS_*.

#### Tasks
1. Criar `core/dockerfile/regression_test.go`.
2. Test 1: via Options.WorkspaceTarget
3. Test 2: via env vars (THEOPACKS_APP_NAME/PATH)
4. Test 3: via ambos
5. Assert NotContains em todos os 3.

#### TDD
```
RED:     TestRegression_Turborepo_OptionsBridge_NoSpuriousSecrets
RED:     TestRegression_Turborepo_EnvBridge_NoSpuriousSecrets
RED:     TestRegression_Turborepo_BothBridges_NoSpuriousSecrets
GREEN:   Já passam pós T1.4. Test é guard rail.
REFACTOR: None expected.
VERIFY:  go test ./core/dockerfile/ -run TestRegression_Turborepo
```

#### Acceptance Criteria
- [ ] 3 testes existem
- [ ] Cada um gera Dockerfile e busca por `--mount=type=secret,id=THEOPACKS_`
- [ ] Todos PASS no estado atual; um deles falha se C1 regredir

#### DoD
- [ ] Testes verdes
- [ ] Mensagem de falha referencia "C1 in deep-review-hardening-plan"

---

### T0.4 — Habilitar `-race` em `mise run test`

#### Objective
Adicionar `-race` flag aos comandos `mise run test` e `mise run test-e2e` para que data races em código novo (ou existente) sejam detectados em CI.

#### Evidence
`mise.toml`:
```toml
[tasks.test]
run = "go test ./core/... ./cmd/..."

[tasks.test-e2e]
run = "go test -tags e2e ./e2e/ -timeout 1500s"
```
Nenhum tem `-race`. `App.globCache` é mapa mutado sem mutex; `Resolver` provavelmente também. Em uso atual (single-thread CLI) é seguro, mas escala mal.

#### Files to edit
```
mise.toml — adicionar -race em test e test-e2e
```

#### Deep file dependency analysis
- **`mise.toml`** — config de tarefas. Sem código.
- Downstream: CI (que usa `mise run test`). Mudança transparente — só fica mais rigoroso.

#### Deep Dives
Risco: se algum race vier à tona, vira blocker. Mitigação: rodar `go test -race ./...` localmente primeiro e corrigir antes de commitar a mudança no mise.toml.

#### Tasks
1. Rodar `go test -race ./...` localmente. Registrar races encontrados.
2. Para cada race genuíno: adicionar mutex/atomic ou refatorar.
3. Atualizar `mise.toml::test` e `test-e2e` com `-race`.
4. Confirmar `mise run test` verde com a flag.

#### TDD
```
RED:     go test -race ./... → falha em race genuíno (se houver)
GREEN:   Corrigir cada race com mutex/atomic.
REFACTOR: None expected.
VERIFY:  go test -race ./core/... ./cmd/... → 0 warnings
```

#### Acceptance Criteria
- [ ] `mise.toml::test` contém `-race`
- [ ] `mise run test` verde
- [ ] Se algum race foi corrigido, commit separado registra o fix

#### DoD
- [ ] mise.toml atualizado
- [ ] Race detector limpo
- [ ] CI runtime aceitável (< 2× atual)

---

## Phase 1: E2E coverage

**Objective:** chegar a ≥ 45/48 examples cobertos por E2E Docker real, garantindo que cada provider/framework tem pelo menos um teste que faz `docker build` real e valida runtime básico.

### T1.1 — Refactor table-driven em `e2e_test.go`

#### Objective
Introduzir estrutura `e2eCase` e função dispatcher `runE2ETable(t, cases)` para que adicionar cobertura a um novo example seja uma entry, não uma função top-level.

#### Evidence
20 funções `TestE2E_*_BuildsImage` repetem o mesmo shape. Adicionar 28 examples assim duplica boilerplate. `runE2EBuild` já existe (linha 314) como base — falta a tabela.

#### Files to edit
```
e2e/e2e_test.go — extrair e2eCase struct + runE2ETable dispatcher
```

#### Deep file dependency analysis
- **`e2e/e2e_test.go`** — único arquivo de testes E2E. Refactor preserva os testes existentes; novos cases entram via tabela.
- Sem mudança em `core/`, `cmd/`, etc.

#### Deep Dives
```go
type e2eCase struct {
    example string                       // examples/<name>
    tag     string                       // docker image tag
    env     map[string]string            // optional env vars
    verify  func(t *testing.T, tag string) // optional post-build check
    maxMB   int                          // optional size budget; 0 = no check
}

var e2eCases = []e2eCase{
    {example: "go-simple", tag: "te2e-go-simple", verify: requireBinaryAtServer},
    {example: "node-npm", tag: "te2e-node-npm", verify: requireNodeOK, maxMB: 280},
    // ... 40+ more
}

func TestE2E_All(t *testing.T) {
    if !dockerAvailable() { t.Skip(...) }
    for _, c := range e2eCases {
        t.Run(c.example, func(t *testing.T) {
            t.Parallel() // safe: docker daemon serializes builds anyway
            runE2ECase(t, c)
        })
    }
}
```

Helpers já existentes (`requireBinaryAt`, `requireSizeLessThan`) são reaproveitados como verifiers.

Migração: as 20 funções top-level continuam por compat (chamam o mesmo `runE2ECase`); novos examples vão direto na tabela. Eventualmente, remover as duplicatas top-level (out-of-scope deste plano se quebrar test runners externos).

#### Tasks
1. Adicionar `type e2eCase struct {...}` em `e2e/e2e_test.go`.
2. Adicionar verifiers nomeados (`requireBinaryAtServer`, `requireNodeOK`, etc.).
3. Adicionar slice `e2eCases` com os 20 examples já cobertos.
4. Adicionar `TestE2E_All(t)` que itera.
5. Manter as 20 funções top-level inicialmente como aliases (chamam `runE2ECase`) — remover em PR separado.

#### TDD
```
RED:     TestE2E_All — sem o dispatcher, função não existe (test não compila)
GREEN:   Implementar struct + dispatcher.
REFACTOR: Após verificar paridade, simplificar top-level functions ou removê-las.
VERIFY:  go test -tags e2e -run TestE2E_All ./e2e/ -timeout 1500s
```

#### Acceptance Criteria
- [ ] `e2eCase` struct existe
- [ ] `TestE2E_All` cobre os 20 examples já cobertos
- [ ] Top-level functions OU continuam OU foram removidas (decisão registrada)
- [ ] `go test -tags e2e -count=1 ./e2e/` verde

#### DoD
- [ ] Refactor aplicado
- [ ] Tempo total não regrediu (paralelização compensa overhead)
- [ ] CI runtime aceitável

---

### T1.2 — Cobertura E2E dos examples Node faltantes

#### Objective
Adicionar entries para 10 examples Node ao `e2eCases`: `node-express`, `node-next`, `node-astro`, `node-remix`, `node-nuxt`, `node-vite-react`, `node-vite-svelte`, `node-vite-vue`, `node-pnpm-workspaces`, `node-yarn-workspaces`, `node-npm-workspaces`, `node-turborepo`, `node-npm-with-config`, `node-npm-with-dockerignore`.

#### Evidence
Examples existem; goldens existem (`integration_node_*.dockerfile` em `testdata/`); mas E2E Docker real só cobre `node-npm`. Bugs como "Next standalone output não copia" passariam.

#### Files to edit
```
e2e/e2e_test.go — adicionar 14 entries em e2eCases
```

#### Deep file dependency analysis
- Mesmo arquivo do T1.1. Depende do dispatcher pronto.
- Examples ficam intactos.

#### Deep Dives
- Frameworks com build (`next`, `nuxt`, `astro`, `remix`, `vite-*`): verify checa runtime `node -e "console.log('ok')"` + size budget 280 MB.
- Workspaces (`*-workspaces`, `turborepo`): verify checa que `apps/api/dist` ou similar foi copiado (via `requireBinaryAt` adaptado).

Alguns examples podem precisar de `env` específico (ex: turborepo precisa de `THEOPACKS_APP_NAME`).

#### Tasks
1. Para cada um dos 14 examples, adicionar entry em `e2eCases`.
2. Definir `verify` específico onde aplicável.
3. Set `maxMB` quando relevante.

#### TDD
```
RED:     TestE2E_All/<each-new-example> — não existe ainda
GREEN:   Adicionar entries.
REFACTOR: None expected.
VERIFY:  go test -tags e2e -run TestE2E_All/node ./e2e/ -timeout 1500s
```

#### Acceptance Criteria
- [ ] 14 examples Node cobertos
- [ ] Cada um faz `docker build` real
- [ ] Cada um valida runtime básico
- [ ] Workspaces validam que app target foi copiado

#### DoD
- [ ] Tabela atualizada
- [ ] Todos passam (Docker requerido)

---

### T1.3 — Cobertura E2E dos examples Python faltantes

#### Objective
Adicionar `python-django`, `python-fastapi`, `python-gradio`, `python-streamlit`, `python-poetry`, `python-pipfile`, `python-setuppy`, `python-uv-workspace`. Total 8.

#### Evidence
Goldens existem; só `python-flask` tem E2E real.

#### Files to edit
```
e2e/e2e_test.go — 8 entries
```

#### Deep file dependency analysis
- Igual ao T1.2.

#### Deep Dives
- `poetry`: verify checa `poetry --version` no contêiner ou que `pyproject.toml` está em `/app`.
- `pipfile`: verify checa `pipenv --version`.
- `setuppy`: verify checa `pip show <pkg>`.
- `uv-workspace`: verify checa que workspace member foi instalado.
- `gradio`/`streamlit`: importar a lib é suficiente (sem subir servidor — risco de hang).

#### Tasks
1-8. Para cada example, adicionar entry com `verify` apropriado.

#### TDD
```
RED:     TestE2E_All/python-django — não existe
RED:     TestE2E_All/python-fastapi
... (8 testes)
GREEN:   Adicionar entries.
REFACTOR: None expected.
VERIFY:  go test -tags e2e -run TestE2E_All/python ./e2e/ -timeout 1500s
```

#### Acceptance Criteria
- [ ] 8 examples Python cobertos
- [ ] Cada um valida import da framework principal
- [ ] Sem hangs (gradio/streamlit não inicializam servidor no test)

#### DoD
- [ ] Tabela atualizada
- [ ] Todos passam

---

### T1.4 — Cobertura E2E dos examples restantes

#### Objective
Cobrir `go-cmd-dirs`, `dotnet-console`, `deno-fresh`, `ruby-rails`, `php-laravel`, `rust-cli` — 6 examples.

#### Evidence
Cada provider já tem 1-2 examples cobertos; estes são as variações que ficam de fora.

#### Files to edit
```
e2e/e2e_test.go — 6 entries
```

#### Deep file dependency analysis
- Igual ao T1.2/T1.3.

#### Deep Dives
- `rust-cli`: binário em `/app/server` ou similar.
- `dotnet-console`: `ls /app/publish/*.dll`.
- `deno-fresh`: deno --version + check `apps/<name>/` se workspace.
- `ruby-rails`: `bundle info rails`.
- `php-laravel`: `php -r "echo PHP_VERSION;"`.
- `go-cmd-dirs`: binary check em `/app/server`.

#### Tasks
1-6. Adicionar entries.

#### TDD
```
RED:     TestE2E_All/<each> — não existem
GREEN:   Adicionar.
REFACTOR: None expected.
VERIFY:  go test -tags e2e -run TestE2E_All ./e2e/
```

#### Acceptance Criteria
- [ ] 6 examples cobertos
- [ ] Cada um valida funcionalidade específica da framework

#### DoD
- [ ] Cobertura E2E total ≥ 45/48 examples (apenas `staticfile`, `shell-script` e `fullstack-mixed` ficam como exceções aceitáveis — staticfile e shell já cobertos; fullstack já tem teste dedicado)

---

### T1.5 — Embedar fixture `monorepo-turbo` (D2)

#### Objective
Substituir a dependência de checkout sibling `theo-stacks/templates/monorepo-turbo` por fixture interna via `//go:embed` ou cópia em `e2e/testdata/`. Resultado: `TestE2E_MonorepoTurboFromStacks` nunca skipa.

#### Evidence
`e2e/e2e_test.go:434-447`:
```go
if _, err := os.Stat(abs); os.IsNotExist(err) {
    t.Skipf("theo-stacks not checked out next to theo-packs ...")
}
```
Em CI sem o segundo clone, o contrato F3 não é testado.

#### Files to edit
```
e2e/testdata/monorepo-turbo/ (NEW) — cópia minimizada do template upstream
e2e/e2e_test.go — substituir theoStacksDir() por loadEmbeddedFixture()
e2e/embed.go (NEW) — //go:embed all:testdata/monorepo-turbo/* 
tools/sync-theo-stacks-fixture.sh (NEW, opcional) — script para refresh
```

#### Deep file dependency analysis
- **`e2e/testdata/monorepo-turbo/`** — diretório novo com cópia atual do template upstream.
- **`e2e/embed.go`** — declara `//go:embed` da fixture.
- **`e2e/e2e_test.go`** — função `theoStacksDir` substituída por `loadTurboFixture(t)` que materializa o embed em `t.TempDir()`.
- Downstream: ninguém depende; é puramente test infra.

#### Deep Dives
Estratégia:
1. Identificar os ~30-50 arquivos mínimos do template (package.json roots, apps/*/package.json, apps/*/src/*.ts, packages/*/...).
2. Copiar para `e2e/testdata/monorepo-turbo/`.
3. `//go:embed all:testdata/monorepo-turbo/*` em `embed.go`.
4. `loadTurboFixture(t)`: itera entries do embed.FS e escreve em `t.TempDir()`.
5. `TestE2E_MonorepoTurboFromStacks` deixa de skipar.

Risco: drift entre fixture interna e template upstream. Mitigação: script `tools/sync-theo-stacks-fixture.sh` (opcional) que copia de um path conhecido — chamável manualmente quando upstream evoluir.

#### Tasks
1. Copiar template de uma referência atual do `theo-stacks/templates/monorepo-turbo/` para `e2e/testdata/monorepo-turbo/`.
2. Criar `e2e/embed.go` com `//go:embed all:testdata/monorepo-turbo`.
3. Implementar `loadTurboFixture(t)` em `e2e/e2e_test.go`.
4. Modificar `TestE2E_MonorepoTurboFromStacks` para usar `loadTurboFixture`.
5. Validar `go test -tags e2e -run TestE2E_MonorepoTurboFromStacks` SEM o sibling — não deve skipar.
6. (Opcional) `tools/sync-theo-stacks-fixture.sh`.

#### TDD
```
RED:     TestE2E_MonorepoTurboFromStacks com sibling AUSENTE → atualmente Skip; deve PASS
GREEN:   Implementar embed + loader.
REFACTOR: None expected.
VERIFY:  rm -rf ../theo-stacks; go test -tags e2e -run TestE2E_MonorepoTurboFromStacks
```

#### Acceptance Criteria
- [ ] Fixture interna existe em `e2e/testdata/monorepo-turbo/`
- [ ] `//go:embed` carrega-a
- [ ] Test não skipa quando sibling ausente
- [ ] Build Docker real continua passando

#### DoD
- [ ] Embed funcionando
- [ ] Test verde sem checkout externo
- [ ] Script de sync documentado em README/CONTRIBUTING

---

### T1.6 — Shell provider real E2E (sem `THEOPACKS_START_CMD` hack)

#### Objective
`TestE2E_ShellScript_BuildsImage` (linha 224) hoje injeta `THEOPACKS_START_CMD=bash start.sh` — não testa a auto-detecção do shell provider. Refactor para validar o comportamento default.

#### Evidence
`e2e/e2e_test.go:230-232`:
```go
df := generateDockerfile(t, dir, map[string]string{
    "THEOPACKS_START_CMD": "bash start.sh",
})
```
Se o shell provider quebrar a detecção, esse test continua passando.

#### Files to edit
```
e2e/e2e_test.go — refactor TestE2E_ShellScript_BuildsImage (ou TestE2E_All/shell-script entry)
core/providers/shell/shell.go — se necessário, ajustar auto-detect (provavelmente já funciona)
examples/shell-script/ — verificar conformidade
```

#### Deep file dependency analysis
- **`shell-script` example** — provavelmente tem `start.sh`. Verificar que o provider o detecta.
- **`core/providers/shell/shell.go`** — pode precisar de ajuste se detect heurística não pegar.

#### Deep Dives
Comportamento esperado: shell provider detecta `*.sh` files e usa o primeiro (ou `start.sh` por convenção) como entry. Sem env var override, `result.Plan.Deploy.StartCmd` deveria ser algo como `bash start.sh` (mas vindo do provider, não do hack).

Investigar primeiro: rodar shell provider sem env override e ver o que vem. Se vier vazio, escrever ADR explicando porque o provider precisa de hint do user.

#### Tasks
1. Investigar comportamento atual: `theopacks-generate --source examples/shell-script --app-path . --output /tmp/Df` SEM `THEOPACKS_START_CMD`.
2. Se gera start cmd válido: remover o hack do test e validar.
3. Se NÃO gera: melhorar provider OU adicionar ADR justificando o requisito de env hint.
4. Atualizar test conforme decisão.

#### TDD
```
RED:     TestE2E_ShellScript_NoEnvHint_BuildsAndRuns — sem env, contêiner sobe e executa start.sh
GREEN:   Provider ajustado OR test mantém env hint mas com comentário explicando o porquê.
REFACTOR: None expected.
VERIFY:  go test -tags e2e -run TestE2E_ShellScript ./e2e/
```

#### Acceptance Criteria
- [ ] Test não usa hack OR hack é justificado por ADR
- [ ] Provider behavior é validado, não fabricado

#### DoD
- [ ] Decisão documentada
- [ ] Test verde

---

## Phase 2: Test infrastructure

**Objective:** acelerar a suíte e tornar cleanup à prova de panic.

### T2.1 — `TestMain` compartilhado para CLI subprocess tests (D5)

#### Objective
Mover compilação do binário `theopacks-generate` para `TestMain` em `cmd/theopacks-generate/`. Atualmente cada um dos ~25 testes recompila o binário (~500ms cada).

#### Evidence
`cmd/theopacks-generate/main_test.go:14-29`:
```go
func buildBinary(t *testing.T) string {
    dir := t.TempDir()
    binPath := filepath.Join(dir, "theopacks-generate")
    cmd := exec.Command("go", "build", "-o", binPath, ".")
    // ...
}
```
Chamado em cada test. Total: ~12s desperdiçados na suíte CLI.

#### Files to edit
```
cmd/theopacks-generate/testmain_test.go (NEW) — TestMain + sharedBinary var
cmd/theopacks-generate/main_test.go — buildBinary() retorna sharedBinary
```

#### Deep file dependency analysis
- **`testmain_test.go` (NEW)** — define `TestMain(m *testing.M)` que compila em `os.TempDir()`, expõe via package var, e remove no exit.
- **`main_test.go`** — `buildBinary(t)` simplificado para retornar a var global.

#### Deep Dives
```go
// testmain_test.go
var sharedBinary string

func TestMain(m *testing.M) {
    dir, err := os.MkdirTemp("", "theopacks-binary-")
    if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
    sharedBinary = filepath.Join(dir, "theopacks-generate")

    cmd := exec.Command("go", "build", "-o", sharedBinary, ".")
    cmd.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")
    if out, err := cmd.CombinedOutput(); err != nil {
        fmt.Fprintln(os.Stderr, "failed to build:", string(out))
        os.Exit(1)
    }

    code := m.Run()
    os.RemoveAll(dir)
    os.Exit(code)
}
```

`buildBinary(t)` vira `return sharedBinary`.

Edge case: se o source mudou desde a última compilação, Go test recompila o package mas não o subprocess binário. Isso pode mascarar bugs em rare cases. Mitigação: documentar e aceitar.

#### Tasks
1. Criar `testmain_test.go`.
2. Simplificar `buildBinary(t)` em `main_test.go`.
3. Confirmar todos os ~25 testes passam.
4. Medir antes/depois.

#### TDD
```
RED:     TestMain_BuildsBinaryOnce — assert binary compiled once (verificar via mtime/atime tricks ou simples timing)
GREEN:   Implementar.
REFACTOR: None expected.
VERIFY:  go test ./cmd/theopacks-generate/ -count=1 -v | grep -c PASS
         Medir tempo total — deve cair pelo menos 30%.
```

#### Acceptance Criteria
- [ ] `TestMain` compila o binário uma vez
- [ ] Todos os testes existentes passam
- [ ] Tempo total da suíte CLI cai ≥ 30%

#### DoD
- [ ] Refactor aplicado
- [ ] Suíte verde
- [ ] Benchmark de tempo registrado no commit

---

### T2.2 — Migrar `defer` → `t.Cleanup` em E2E antigos

#### Objective
Os 7 testes E2E originais (`TestE2E_GoSimple_BuildsImage`, `TestE2E_NodeNpm_*`, etc.) usam `defer removeImage(tag)`. `defer` não roda em `t.Skip()` e pode vazar imagens em panic de setup. `t.Cleanup` é o idiomatic seguro.

#### Evidence
```go
defer removeImage(tag)
defer func() { _ = os.Remove(dfPath) }()
```
`runE2EBuild` já usa `t.Cleanup`; basta migrar os antigos.

#### Files to edit
```
e2e/e2e_test.go — 7 funções (linhas ~136-294)
```

#### Deep file dependency analysis
- Mesma localização. Mudança mecânica.

#### Deep Dives
Padrão a aplicar:
```go
// antes
defer removeImage(tag)

// depois
t.Cleanup(func() { removeImage(tag) })
```

#### Tasks
1. Identificar todos os `defer removeImage` e `defer os.Remove(dfPath)` em E2E.
2. Substituir por `t.Cleanup(...)`.
3. Garantir que cleanup ocorre mesmo se `t.Skip` for chamado antes.

#### TDD
```
RED:     TestE2E_CleanupRunsOnSkip — usa t.Run com Skip e verifica que cleanup rodou (via flag local)
GREEN:   Migração de defer → t.Cleanup.
REFACTOR: None expected.
VERIFY:  docker images | grep theopacks-e2e → vazio após go test
```

#### Acceptance Criteria
- [ ] Zero `defer removeImage` em e2e_test.go
- [ ] Imagens são limpas mesmo em Skip/panic

#### DoD
- [ ] Migração aplicada
- [ ] Suite verde

---

### T2.3 — `t.Parallel()` onde seguro

#### Objective
Habilitar paralelização em pacotes onde os testes são puramente in-process e não compartilham estado mutável. Alvos primários: `core/`, `core/dockerfile/`, `core/providers/*/`, `cmd/theopacks-generate/` (após T2.1).

#### Evidence
`grep -c "t.Parallel()" $(find . -name "*_test.go")` → 30 ocorrências, todas em `core/generate/images_test.go`. Os outros 55 arquivos rodam serial.

#### Files to edit
```
core/dockerfile/integration_test.go — t.Parallel em subtests
core/monorepo_test.go — t.Parallel em subtests de TestRealExample_*
core/integration_test.go — t.Parallel no t.Run interno
core/providers/*/*_test.go — t.Parallel onde TempDir é usado
cmd/theopacks-generate/main_test.go — t.Parallel após TestMain shared
```

#### Deep file dependency analysis
- Arquivos identificados acima usam `t.TempDir` (isolamento garantido por subdir único) ou são puramente in-memory.
- Risco: testes que compartilham fixture (golden corpus) podem ter race se o corpus mudar — mas o corpus é read-only em runtime, então safe.

#### Deep Dives
Regra geral: subtests com `t.TempDir` ou que só leem fixtures podem `t.Parallel()`. Subtests que mutam env vars globais (`os.Setenv`) ou cwd (`os.Chdir`) NÃO.

Inspecionar cada arquivo antes de adicionar. Não é refactor massivo; é uma onda controlada.

#### Tasks
1. Listar candidatos: arquivos que usam `t.TempDir` exclusivamente.
2. Adicionar `t.Parallel()` em `t.Run(name, func(t *testing.T) {...})` blocks.
3. Rodar `go test -race` para detectar problemas.
4. Medir antes/depois.

#### TDD
```
RED:     N/A — paralelização é otimização; não há novo teste.
GREEN:   Adicionar t.Parallel onde seguro.
REFACTOR: None expected.
VERIFY:  go test -race ./... ainda passa; tempo total cai.
```

#### Acceptance Criteria
- [ ] ≥ 10 arquivos usando `t.Parallel`
- [ ] `go test -race ./...` verde
- [ ] Tempo total cai ≥ 20%

#### DoD
- [ ] Paralelização aplicada
- [ ] Suite verde com -race
- [ ] Benchmark registrado

---

## Phase 3: Adversarial defense

**Objective:** captar bugs que tabela manual nunca cobriria — fuzz + benchmark.

### T3.1 — Fuzz para `shellEscape`

#### Objective
`FuzzShellEscape_RoundTripsThroughSh`: para input aleatório `s`, `shellEscape(s)` interpolado em `sh -c "printf '%s' <out>"` deve produzir `s` de volta.

#### Evidence
T3.4 do plano anterior adicionou tabela manual + round-trip de 8 inputs. Fuzz exploraria milhões de strings, cobrindo unicode patológico, control chars, etc.

#### Files to edit
```
core/dockerfile/shell_escape_test.go — adicionar FuzzShellEscape_*
core/dockerfile/testdata/fuzz/FuzzShellEscape_RoundTripsThroughSh/ (NEW) — corpus seed
```

#### Deep file dependency analysis
- **`shell_escape_test.go`** — já existe; adiciona função `Fuzz...`.
- **`testdata/fuzz/...`** — Go convention para corpus persistido.

#### Deep Dives
```go
func FuzzShellEscape_RoundTripsThroughSh(f *testing.F) {
    seeds := []string{"", "'", "''", "$VAR", "\n", "`whoami`", "a'b", string([]byte{0})}
    for _, s := range seeds { f.Add(s) }

    f.Fuzz(func(t *testing.T, s string) {
        if !utf8.ValidString(s) { t.Skip() } // sh requires valid UTF-8 in most locales
        if strings.Contains(s, "\x00") { t.Skip() } // sh strings can't carry NUL
        escaped := shellEscape(s)
        out, err := exec.Command("sh", "-c", "printf '%s' "+escaped).Output()
        if err != nil { t.Fatalf("sh rejected %q: %v", escaped, err) }
        if string(out) != s { t.Fatalf("round-trip mismatch: %q → %q → %q", s, escaped, out) }
    })
}
```

Edge cases ignorados via `t.Skip`: NUL byte, invalid UTF-8 — não são alvo do shellEscape.

#### Tasks
1. Adicionar `FuzzShellEscape_RoundTripsThroughSh` em `shell_escape_test.go`.
2. Seedar com casos conhecidos.
3. Rodar `go test -fuzz=FuzzShellEscape -fuzztime=60s` localmente.
4. Comitar corpus gerado (`testdata/fuzz/...`).

#### TDD
```
RED:     FuzzShellEscape — função não existe; teste não compila.
GREEN:   Implementar fuzz.
REFACTOR: None expected.
VERIFY:  go test -fuzz=FuzzShellEscape -fuzztime=60s ./core/dockerfile/
         go test ./core/dockerfile/ -run FuzzShellEscape (regression mode)
```

#### Acceptance Criteria
- [ ] Função `FuzzShellEscape_RoundTripsThroughSh` existe
- [ ] Corpus persistido inclui ≥ 8 seeds
- [ ] `go test -fuzz=... -fuzztime=60s` não encontra crashes

#### DoD
- [ ] Fuzz adicionado
- [ ] Sem crashes em 60s de fuzzing local
- [ ] Documentar em CONTRIBUTING como rodar fuzz exploratório

---

### T3.2 — Fuzz para `validateCLIInput`

#### Objective
`FuzzValidateCLIInput_NeverPanics`: input aleatório nunca causa panic na função de sanitização da CLI. Captura bugs em regex backtracking ou edge cases.

#### Evidence
T1.1 do plano anterior adicionou validação com regex. Regex pathological inputs (longas, com `..` aninhado) podem causar performance regression ou panic.

#### Files to edit
```
cmd/theopacks-generate/validate_test.go — adicionar FuzzValidateCLIInput
cmd/theopacks-generate/testdata/fuzz/FuzzValidateCLIInput_NeverPanics/ (NEW)
```

#### Deep file dependency analysis
- **`validate_test.go`** — já existe; adiciona fuzz.

#### Deep Dives
```go
func FuzzValidateCLIInput_NeverPanics(f *testing.F) {
    f.Add("/workspace", ".", "", "/out/Dockerfile")
    f.Add("../../etc", ".", "", "/out")
    f.Add("/workspace", "../../../../../etc/passwd", "evil", "/out")
    f.Add("", "", "", "")

    f.Fuzz(func(t *testing.T, source, appPath, appName, output string) {
        defer func() {
            if r := recover(); r != nil {
                t.Fatalf("panic on input source=%q appPath=%q appName=%q output=%q: %v",
                    source, appPath, appName, output, r)
            }
        }()
        _ = validateCLIInput(source, appPath, appName, output)
    })
}
```

Objetivo NÃO é validar que reject é correto — é apenas que nunca panicka. Crash-free é o invariante.

#### Tasks
1. Adicionar fuzz em `validate_test.go`.
2. Seedar com casos do review (traversal, injection).
3. Rodar 60s + comitar corpus.

#### TDD
```
RED:     FuzzValidateCLIInput — função não existe
GREEN:   Implementar
REFACTOR: None expected
VERIFY:  go test -fuzz=FuzzValidateCLIInput -fuzztime=60s ./cmd/theopacks-generate/
```

#### Acceptance Criteria
- [ ] Fuzz existe e roda
- [ ] Zero crashes em 60s

#### DoD
- [ ] Adicionado
- [ ] Sem crashes

---

### T3.3 — Fuzz para `clampPath`

#### Objective
`FuzzClampPath_NeverEscapes`: para qualquer `(root, sub)` que retorne `(p, nil)`, vale `strings.HasPrefix(p+sep, root+sep) || p == root`.

#### Evidence
T1.2 do plano anterior adicionou `clampPath`. Invariante crítico que precisa ser garantido contra inputs adversariais.

#### Files to edit
```
cmd/theopacks-generate/path_test.go — adicionar FuzzClampPath_NeverEscapes
cmd/theopacks-generate/testdata/fuzz/FuzzClampPath_NeverEscapes/ (NEW)
```

#### Deep file dependency analysis
- **`path_test.go`** — já existe; adiciona fuzz.

#### Deep Dives
```go
func FuzzClampPath_NeverEscapes(f *testing.F) {
    f.Add("/workspace", "apps/api")
    f.Add("/workspace", "../etc")
    f.Add("/workspace", "./apps/../../../etc")
    f.Add("/", "")
    f.Add("/workspace", "/etc/passwd")

    f.Fuzz(func(t *testing.T, root, sub string) {
        defer func() {
            if r := recover(); r != nil {
                t.Fatalf("panic on (%q, %q): %v", root, sub, r)
            }
        }()
        p, err := clampPath(root, sub)
        if err != nil { return } // reject is fine

        rootAbs, _ := filepath.Abs(root)
        sep := string(filepath.Separator)
        if p != rootAbs && !strings.HasPrefix(p+sep, rootAbs+sep) {
            t.Fatalf("INVARIANT VIOLATED: clampPath(%q, %q) = %q escapes %q",
                root, sub, p, rootAbs)
        }
    })
}
```

#### Tasks
1. Adicionar fuzz.
2. Seedar.
3. Rodar 60s + commit corpus.

#### TDD
```
RED:     FuzzClampPath_NeverEscapes — não existe
GREEN:   Implementar
REFACTOR: None expected
VERIFY:  go test -fuzz=FuzzClampPath -fuzztime=60s ./cmd/theopacks-generate/
```

#### Acceptance Criteria
- [ ] Fuzz existe
- [ ] Zero violações de invariante em 60s

#### DoD
- [ ] Adicionado
- [ ] Sem crashes

---

### T3.4 — Benchmark de geração

#### Objective
`BenchmarkGenerateBuildPlan_NodeTurborepo` e similares — estabelecer baseline de performance para detectar regressões.

#### Evidence
Não há benchmark hoje. Mudanças que adicionam I/O repetido (FindFiles N², readJSON em loop) ficariam invisíveis.

#### Files to edit
```
core/benchmark_test.go (NEW) — BenchmarkGenerateBuildPlan_*
core/dockerfile/benchmark_test.go (NEW) — BenchmarkGenerate_*
```

#### Deep file dependency analysis
- Arquivos novos isolados; sem mudança em runtime.

#### Deep Dives
3 benchmarks alvo:
1. `BenchmarkGenerateBuildPlan_GoSimple` — caso trivial; estabelece floor.
2. `BenchmarkGenerateBuildPlan_NodeTurborepo` — caso realista; 4 apps, 2 packages.
3. `BenchmarkDockerfileGenerate_NodeTurborepo` — renderização isolada.

Output esperado: tempos consistentes em ordens de magnitude (ms, não s). Salvar baseline em `docs/benchmarks/baseline.txt`.

#### Tasks
1. Criar `core/benchmark_test.go` e `core/dockerfile/benchmark_test.go`.
2. Implementar 3 benchmarks.
3. Rodar e capturar baseline.
4. Comitar baseline em `docs/benchmarks/baseline.txt`.

#### TDD
```
RED:     BenchmarkGenerateBuildPlan_* — não existem
GREEN:   Implementar
REFACTOR: None expected
VERIFY:  go test -bench=. -run=^$ ./core/ ./core/dockerfile/
```

#### Acceptance Criteria
- [ ] 3 benchmarks existem
- [ ] Baseline registrado em `docs/benchmarks/baseline.txt`
- [ ] Tempos < 100ms para os casos triviais, < 500ms para turborepo

#### DoD
- [ ] Benchmarks rodando
- [ ] Baseline comitado
- [ ] CI workflow opcional para comparar PRs

---

## Phase 4: Behavior refactor

**Objective:** trocar testes acoplados a literais (`"npm start"`) por matchers de comportamento. Reduz fricção de futuros refactors do start command.

### T4.1 — Behavior matchers em `monorepo_test.go`

#### Objective
Substituir `require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)` (≥ 30 ocorrências) por helper que aceita formas razoáveis para o provider.

#### Evidence
`monorepo_test.go:102,110,118` etc.:
```go
require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
```
Anti-pattern explícito da Rule 7 do CLAUDE.

#### Files to edit
```
core/test_helpers_test.go (NEW) — helpers assertReasonableStart, assertReasonableNodeStart, etc.
core/monorepo_test.go — substituir Equal por helpers
```

#### Deep file dependency analysis
- **`test_helpers_test.go` (NEW)** — pacote `core`; exporta funções para outros `*_test.go` no mesmo pacote.
- **`monorepo_test.go`** — refactor ~30 sites.

#### Deep Dives
```go
// assertReasonableNodeStart accepts: npm start | yarn start | pnpm start |
// bun start | node <file>. Rejects: empty, /bin/sh -c blob (smell).
func assertReasonableNodeStart(t *testing.T, cmd string) {
    t.Helper()
    require.NotEmpty(t, cmd, "Node start command should not be empty")
    valid := []string{"npm start", "yarn start", "pnpm start", "bun start"}
    for _, v := range valid {
        if strings.HasPrefix(cmd, v) { return }
    }
    if strings.HasPrefix(cmd, "node ") { return }
    if strings.HasPrefix(cmd, "cd ") && strings.Contains(cmd, "&&") {
        // workspace-mode start: "cd apps/api && npm start"
        return
    }
    t.Fatalf("unexpected Node start command: %q", cmd)
}
```

Aplicar análogo para Python (`gunicorn`, `python -m`, `uvicorn`), Go (`/app/server`), etc.

#### Tasks
1. Criar `test_helpers_test.go` com 5 helpers (node, python, go, rust, java, ...).
2. Sweeping `monorepo_test.go` substituindo `Equal(... StartCmd)` por helper apropriado.
3. Validar suite verde.

#### TDD
```
RED:     N/A — refactor de testes existentes.
GREEN:   Helpers + sweep.
REFACTOR: None expected.
VERIFY:  go test ./core/ -run TestRealExample -v
```

#### Acceptance Criteria
- [ ] Helpers existem para cada linguagem
- [ ] `grep "Equal.*StartCmd.*npm start" core/monorepo_test.go` retorna 0
- [ ] Suite verde

#### DoD
- [ ] Refactor aplicado
- [ ] Suite verde

---

### T4.2 — Behavior matchers em `dogfood_test.go`

#### Objective
Mesma transformação do T4.1 aplicada a `dogfood_test.go`. ~15 sites adicionais.

#### Evidence
`dogfood_test.go:38,49,68,76,95`:
```go
require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
require.Equal(t, "/app/server", result.Plan.Deploy.StartCmd)
require.Equal(t, "gunicorn app:app", result.Plan.Deploy.StartCmd)
```

#### Files to edit
```
core/dogfood_test.go — substituir Equal por helpers do T4.1
```

#### Deep file dependency analysis
- Mesma localização do T4.1; reusa helpers.

#### Deep Dives
Exceção: testes que validam env var override (ex: `TestDogfood_PythonProject_WithEnvConfig` afirma `gunicorn app:app` veio do env). Esse Equal é legítimo (testa o env-override behavior, não o provider default). Manter.

#### Tasks
1. Sweep `dogfood_test.go`.
2. Distinguir Equal legítimo (env override) de Equal acoplamento (default provider).

#### TDD
```
RED:     N/A.
GREEN:   Sweep.
REFACTOR: None expected.
VERIFY:  go test ./core/ -run TestDogfood -v
```

#### Acceptance Criteria
- [ ] Equal por acoplamento substituído
- [ ] Equal legítimo (env override) preservado
- [ ] Suite verde

#### DoD
- [ ] Sweep aplicado
- [ ] Verde

---

## Coverage Matrix

| # | Gap | Severity | Task(s) | Resolution |
|---|---|---|---|---|
| G1 | E2E cobre só 20/48 examples (~42%) | High | T1.1, T1.2, T1.3, T1.4 | Table-driven dispatcher + 28 entries cobrindo Node (14), Python (8), outros (6) → ≥ 45/48 |
| G2 | `TestE2E_MonorepoTurboFromStacks` skipa sem checkout externo | High | T1.5 | Embedar fixture via `//go:embed` (D2); contrato F3 testado em CI sem dep externa |
| G3 | Sem teste de determinismo do gerador | Medium | T0.2 | `TestRegression_*_DeterministicOutput` × 3 examples × 10 iterações |
| G4 | Testes literais (`"npm start"`) — testam implementação | Medium | T4.1, T4.2 | Behavior matchers (D4) substituem Equal estrito |
| G5 | Sem `-race` na suíte | Medium | T0.4 | `mise.toml::test` ganha `-race`; races genuínos corrigidos antes |
| G6 | `t.Parallel()` em 1 arquivo só | Low | T2.3 | Paralelização onde seguro; ≥ 10 arquivos |
| G7 | `expectedProvider` loga em vez de falhar | High | T0.1 | Switch exaustivo + `require.NotEmpty` |
| G8 | Cleanup E2E via `defer` (vaza em panic/skip) | Low | T2.2 | `t.Cleanup` em 7 testes antigos |
| G9 | CLI recompila binário a cada teste | Medium | T2.1 | `TestMain` compartilhado (D5); ~30% mais rápido |
| G10 | Sem fuzz no `shellEscape` | Medium | T3.1 | `FuzzShellEscape_RoundTripsThroughSh` + corpus comitado |
| G11 | Sem regression test E2E para secret-mount pollution | High | T0.3 | `TestRegression_NoSpuriousSecretMounts_Turborepo` × 3 (Options, env, ambos) |
| G12 | `TestE2E_ShellScript` injeta env hack | Low | T1.6 | Refactor sem hack OR ADR justificando |
| G13 | Sem benchmark de geração | Low | T3.4 | 3 benchmarks + baseline comitado |
| Extra | Sem fuzz CLI inputs | Medium | T3.2, T3.3 | `FuzzValidateCLIInput_NeverPanics` + `FuzzClampPath_NeverEscapes` |

**Coverage: 14/13 (gaps originais + 1 extra) = 100% +**

## Global Definition of Done

- [ ] Todas as fases completas
- [ ] `mise run test` verde com `-race`
- [ ] `mise run test-e2e` verde com cobertura ≥ 45/48 examples
- [ ] `go test -fuzz=. -fuzztime=60s ./...` sem crashes (rodado localmente; CI roda em modo regression)
- [ ] `TestE2E_MonorepoTurboFromStacks` NÃO skipa em CI limpo
- [ ] Benchmarks rodando com baseline registrado em `docs/benchmarks/baseline.txt`
- [ ] `monorepo_test.go` e `dogfood_test.go` usam matchers de comportamento
- [ ] CHANGELOG `[Unreleased]` documenta as melhorias (test suite hardening)
- [ ] Plano referenciado em PR description

## Resumo executivo

**14 tasks em 5 phases.** Phase 0 (paralelo, ~2h) fecha gaps críticos com XS. Phase 1 (sequencial, ~1 dia) expande cobertura E2E para ~95%. Phase 2 (paralelo, ~3h) acelera a suíte. Phase 3 (paralelo, ~3h) adiciona fuzz + benchmark. Phase 4 (sequencial, ~3h) remove acoplamento de implementação.

**Risco principal:** T0.4 (-race) pode revelar race genuíno e bloquear. Mitigação: rodar localmente primeiro; corrigir como pré-requisito desta phase.

**Valor incremental:** após Phase 0 sozinha, todos os CRITICAL/HIGH do `deep-review-hardening-plan.md` ficam blindados por regression test E2E.
