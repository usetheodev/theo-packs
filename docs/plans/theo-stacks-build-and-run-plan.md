# Plan: theo-stacks Templates — Build AND Run Validation

> **Version 1.0** — Eleva o gate de "Dockerfile generated + image built" para "container starts and serves traffic" para todos os 19 templates upstream do `theo-stacks`. Hoje `TestE2E_TheoStacksTemplates` e `build-all-templates.sh` provam **build**; nada prova **run**. Esse gap esconde regressões críticas: um Dockerfile que builda mas cuja CMD entra em crash-loop, escuta na porta errada, ou nunca atinge readiness por dependência ausente — todos passam pelo gate atual e quebram só em produção via Argo. O plano introduz um runtime gate que faz `docker run`, espera o processo subir, executa healthcheck (HTTP probe ou process-alive check conforme o tipo) e mata o contêiner. Para cada falha encontrada, o root cause é diagnosticado e corrigido **sem workarounds** — no `theo-packs` se o bug é geração do Dockerfile, ou no `theo-stacks` (via PR) se o bug é no source do template. O loop continua iterativamente até **19/19 build AND run PASS**.

## Context

Estado atual (commit `93a131b`):

- `TestE2E_TheoStacksTemplates` (`e2e/theo_stacks_test.go`) — **passa**: gera Dockerfile para cada um dos 19 templates, valida header do provider esperado, roda hadolint. Mas **não builda nem roda**.
- `scripts/build-all-templates.sh` (script manual) — **19/19 PASS** após o ciclo de fixes (issue #38 fechado via PR #39). Mas **só faz `docker build`, nunca `docker run`**.
- O usuário questionou explicitamente: "Voce executou o docker run e eles estao 100% funcinais?" — resposta honesta: **não**.

Gap concreto: o gate atual aceita:
- Container que builda mas cuja CMD imediatamente exit 1 (e.g., variável de ambiente faltante)
- Servidor que escuta em `127.0.0.1` em vez de `0.0.0.0` (não responde fora do contêiner)
- Healthcheck path declarado mas não implementado (404 silencioso)
- Worker que crasha por falta de DB/Redis sem nenhum aviso
- Imagens que dependem de arquivos `.env` que o `.dockerignore` excluiu

Cada uma dessas categorias é regressão real que passa hoje pelo gate. O Argo workflow do Theo dispara `kubectl run` que falha minutos depois — só descoberta tardia.

Evidência de risco real:
- O issue `#38` (já fechado) foi um TS error que `tsc` exibia em logs mas o `docker build` **não falhava** (provider gerou Dockerfile que tenta `npm run build` que falha — foi pego só porque o build-all-templates.sh roda `docker build` completo).
- Sem rodar `docker run`, **nenhum bug runtime-only é detectável** via CI atual.

## Objective

**Done** = todos os 19 templates do `theo-stacks/templates/` (a) buildam via `docker build` (atual), (b) `docker run -d` inicia, (c) o processo permanece vivo por ≥ 5s, (d) para templates HTTP: `curl -fsS http://localhost:<port>/<healthpath>` retorna `2xx` em < 30s, (e) para workers/CLIs: o processo executa sem crashloop, com `exit 0` aceitável se o template é por design não-daemon. Cada falha encontrada é **corrigida na raiz** — bug no `theo-packs` vira commit no repo atual; bug no `theo-stacks` vira PR upstream + aguarda merge antes de prosseguir.

Metas mensuráveis:

1. **19/19 templates passam o runtime gate** end-to-end (build + run + health).
2. **Zero workarounds**: nenhum `t.Skip`, nenhum `if template == "X" { ... }` bypass, nenhum healthcheck stub. Failing template = fix.
3. **Cada bug descoberto tem rastro escrito**: commit (theo-packs) OU PR (theo-stacks) referenciando o teste que falhava.
4. **Gate vira CI permanente**: `TestE2E_TheoStacksTemplates_Runtime` rodando em L3 nightly + opt-in em PR (timeboxed).
5. **Tempo total do gate ≤ 30 min** com paralelização (cada template é independente, max 4 paralelos para não exaurir resources do runner).
6. **Doc `theo-stacks-compatibility.md`** atualizado com seção "Runtime verified" com timestamp por template.

## ADRs

### D1 — Runtime gate como teste Go separado, não shell script

**Decisão:** implementar `TestE2E_TheoStacksTemplates_Runtime` em `e2e/theo_stacks_runtime_test.go` (build tag `e2e`), seguindo o padrão do `TestE2E_TheoStacksTemplates` existente. Não estender `build-all-templates.sh` para fazer runtime check.

**Rationale:** o gate precisa rodar em CI (L3 nightly), produzir output diagnóstico estruturado em caso de falha (logs do contêiner, stack trace do healthcheck), e integrar com `t.Parallel` para tempo viável. Shell scripts são quebradiços (sem retry estruturado, sem cleanup garantido em panic). Go test tem `t.Cleanup`, `context.WithTimeout`, e o framework de subtests permite `-run "Runtime/<name>"` para depuração isolada.

Alternativa rejeitada: estender o script bash — funciona para uma execução manual mas não integra com o CI nem com o resto do framework de E2E.

**Consequências:** dep nova é `nettest` ou similar para HTTP probe robusto; reutilizamos `docker run -d -p`. Output cresce um pouco mais (logs por contêiner em falha) mas é exatamente o que se quer em CI.

### D2 — Health probe baseado em `theo.yaml` do template

**Decisão:** ler `theo.yaml` de cada template (já presente em todos os 19), extrair `apps.<name>.port` e `apps.<name>.type`. Para `type: server` → HTTP probe na porta declarada com `/health` (convenção theo); para `type: worker` → process-alive check (PID 1 vivo por ≥ 5s); para `type: frontend` → HTTP probe em `/`. Sem `theo.yaml`: skip com `t.Fatal` (não é template válido).

**Rationale:** `theo.yaml` é a fonte da verdade da intenção do template; usar valores codificados em tabela Go (que poderiam derivar) seria fonte de drift. O contrato com `theo-stacks` é "se você adicionar um template, declare port + type no theo.yaml" — já é o caso. Alternativa rejeitada: hardcoded por template name — duplica info já presente upstream.

**Consequências:** o gate falha quando o `theo.yaml` é inválido OU quando a porta declarada não corresponde à implementação. Ambos são bugs legítimos do template.

### D3 — Healthcheck path = `/health` por convenção, fallback `/`

**Decisão:** tentar `GET /health` primeiro (Theo doc dita essa convenção, e a maioria dos templates já tem); se 404, tentar `GET /`. Sucesso = qualquer 2xx em qualquer dos dois. 3xx redirect → seguir uma vez.

**Rationale:** `/health` é o padrão da indústria (k8s readiness, Argo healthcheck). Mas alguns templates simples (`go-api`, `node-fastify`) podem ter só `/`. Fallback aceita ambos sem forçar mudança em todos.

**Consequências:** templates novos devem implementar `/health` para máxima resiliência; o fallback evita PR upstream desnecessário para templates trivials.

### D4 — Sem workarounds: bug encontrado vira commit/PR

**Decisão:** para cada falha de runtime, o triage decide:
- **Bug no Dockerfile gerado** (porta errada, USER missing, CMD malformada): commit no `theo-packs` corrigindo o provider.
- **Bug no source do template** (servidor escuta em 127.0.0.1, healthcheck path não definido, dependência runtime missing): PR no `theo-stacks` upstream + aguarda merge antes de prosseguir.
- **Bug genuíno na expectativa do teste** (ex: template é por design um worker que crashloops sem DB): ajusta a expectativa (ex: skip probe para esse caso usando `theo.yaml::type` corretamente — NÃO via skip per-name hardcoded). Essa categoria não é workaround porque o teste passa a refletir a intenção declarada.

**Rationale:** workarounds (hardcode skip, ignorar template) acumulam débito. A goal é "100% functional"; cada bug é root-caused.

**Consequências:** o plano pode bloquear se um PR upstream demorar. Mitigação: o `theo-packs` segue verde via `t.Skip` temporário com TODO + issue link, e o teste só re-habilita quando o PR é merged. Esse skip TEM um link explícito no commit message e no test body — é diferente de um workaround silencioso.

### D5 — Templates que precisam de dependências runtime (DB, Redis) usam testcontainers-go

**Decisão:** para templates `monorepo-*/worker` ou similares que talvez requeiram Postgres/Redis, instanciar via `testcontainers-go` (já no plano de testes anterior) e setar env vars (`DATABASE_URL`, `REDIS_URL`) antes do `docker run` do template. NÃO mockar.

**Rationale:** se um template precisa de DB pra rodar, o teste precisa fornecer DB real — caso contrário, "funciona em produção" não é o que estamos validando. testcontainers é o standard. Alternativa rejeitada: mock — máscara bugs reais de conexão/SQL.

**Consequências:** runtime do test cresce (Postgres boot ~5s); orquestração via `testcontainers.GenericContainer`. Aceitável dado o valor.

### D6 — `docker run` com `--rm`, network host opcional, timeout absoluto

**Decisão:** todo `docker run` usa `--rm` (cleanup automático), `-d` (detached para o teste continuar), `-p <hostPort>:<containerPort>`, timeout absoluto de 60s por template via `context.WithTimeout`. Logs do contêiner são capturados antes do `docker rm -f` em caso de falha.

**Rationale:** evita vazar contêineres em falha (já comum em testes mal-feitos). Captura de logs é crítica para diagnóstico — sem isso, `t.Fatalf("container died")` é inútil.

**Consequências:** infraestrutura wrap exige cuidado (porta aleatória para evitar conflito em parallel). Helper `pickFreePort()` usa `net.Listen(":0")`.

### D7 — Loop de fix iterativo, não single-shot

**Decisão:** o plano não tenta enumerar todos os bugs a priori. Phase 2 é um **loop**: rodar gate → se passar 19/19, done; se algum falhar, abrir branch fix, corrigir, validar, rebuild upstream se necessário, repetir. Cada iteração é um sub-task gerado on-demand (`T2.1`, `T2.2`, ... `T2.N`).

**Rationale:** prever 19 bugs específicos é especulação. Iterar via evidência é honest engineering — exatamente como o ciclo recente que descobriu 5 bugs do theo-packs + 1 do theo-stacks no `build-all-templates.sh`.

**Consequências:** o plano não tem um número fixo de tasks na Phase 2; cresce conforme bugs aparecem. Aceitável; melhor que tasks especulativas.

## Dependency Graph

```
Phase 0: Runtime gate infra (must complete first)
   T0.1 theo.yaml parser
   T0.2 docker run helper + health probe
   T0.3 testcontainers integration for DB-deps templates
            │
            ▼
Phase 1: First-pass discovery (sequential — needs Phase 0)
   T1.1 Run gate against all 19 templates, collect failures
   T1.2 Triage each failure into category (theo-packs / theo-stacks / test-expectation)
            │
            ▼
Phase 2: Fix loop (iterative — repeats until 0 failures)
   T2.1 Fix first failure category (theo-packs bug OR theo-stacks PR)
   T2.2 ... etc, one task per bug found
   T2.N Last fix
            │
            ▼
Phase 3: CI integration (parallel with late Phase 2)
   T3.1 Add runtime gate to L3 nightly workflow
   T3.2 Documentation update
            │
            ▼
Phase 4: Final certification
   T4.1 Full run, capture verified-at timestamp
   T4.2 Update compatibility matrix with runtime status
```

Phase 0 bloqueia Phase 1. Phase 1 produz a lista que alimenta Phase 2 (loop). Phase 3 pode rodar em paralelo com Phase 2 quando ≥ 80% dos bugs estiverem fixados. Phase 4 só após Phase 2 zerar.

---

## Phase 0: Runtime gate infrastructure

**Objective:** ter o framework Go capaz de rodar `docker run`, esperar healthcheck, capturar logs e falhar com diagnóstico actionable.

### T0.1 — `theo.yaml` parser

#### Objective
Parsear `theo.yaml` de cada template e extrair `{ apps: { <name>: { port, type, path } } }`. Fonte da verdade para health-probe target.

#### Evidence
Todos os 19 templates têm `theo.yaml` no root (verificado em `/tmp/theo-stacks/templates/*/theo.yaml`). Exemplos típicos já lidos no contexto: `port: 3000`, `type: server`. Sem parser, o teste teria que hardcode 19 entradas — drift garantido.

#### Files to edit
```
e2e/theoyaml/theoyaml.go (NEW) — struct + ParseFile()
e2e/theoyaml/theoyaml_test.go (NEW) — table-driven parser tests
```

#### Deep file dependency analysis
- **`theoyaml.go`** — pacote standalone; depende só de `gopkg.in/yaml.v2` (já no go.sum).
- **`theoyaml_test.go`** — testes unitários do parser; usa testify.
- Downstream: `theo_stacks_runtime_test.go` (criado em T0.2) consome o `Config` struct.

#### Deep Dives
Struct exato:
```go
type Config struct {
    Version int                    `yaml:"version"`
    Project string                 `yaml:"project"`
    Apps    map[string]AppConfig   `yaml:"apps"`
}

type AppConfig struct {
    Path      string `yaml:"path"`
    Framework string `yaml:"framework"`
    Type      string `yaml:"type"` // "server", "worker", "frontend"
    Port      int    `yaml:"port"`
}
```

Edge cases: `port: 0` → invalid; `type` ausente → default "server"; multi-app monorepo → caller escolhe pelo `appName`.

#### Tasks
1. Criar `e2e/theoyaml/theoyaml.go` com `Config`, `AppConfig`, `ParseFile(path string) (*Config, error)`.
2. Criar `theoyaml_test.go` com 4 casos: single-app server, multi-app monorepo, frontend type, malformed yaml.
3. Validate por substituição em fixture de um template real.

#### TDD
```
RED:     TestParse_SingleAppServer — node-express theo.yaml → port=3000, type=server
RED:     TestParse_MonorepoTurbo — monorepo-turbo → 2 apps
RED:     TestParse_RejectsMalformed — invalid YAML returns error
RED:     TestParse_DefaultsTypeToServer — yaml sem type explicit → "server"
GREEN:   Implementar ParseFile.
REFACTOR: None expected.
VERIFY:  go test ./e2e/theoyaml/ -v
```

#### Acceptance Criteria
- [ ] Parser cobre os 19 templates upstream sem erro
- [ ] 4 testes verdes
- [ ] Edge cases (port=0, type missing) tratados com defaults documentados

#### DoD
- [ ] Pacote compila
- [ ] Testes verdes
- [ ] Doc inline sobre `theo.yaml` schema reference

---

### T0.2 — `docker run` + health probe helper

#### Objective
Função `runAndHealthcheck(t, image, appConfig)` que: faz `docker run -d -p`, espera o contêiner subir, probe HTTP (server/frontend) ou process-alive (worker), captura logs em falha, dá `docker rm -f` no cleanup.

#### Evidence
Não existe wrapper para esse fluxo. Reimplementar inline em cada test = 19× duplicação.

#### Files to edit
```
e2e/runtime_probe.go (NEW) — runAndHealthcheck helper
e2e/runtime_probe_test.go (NEW) — unit-level tests com imagem mock
```

#### Deep file dependency analysis
- **`runtime_probe.go`** — build tag `e2e`, depende de `os/exec` (`docker run/logs/rm`), `net` (free port), `net/http` (probe).
- Consumido por `theo_stacks_runtime_test.go` (Phase 1).

#### Deep Dives
Algoritmo:
1. `pickFreePort()` → `net.Listen(":0")`, fechar, retornar porta.
2. `docker run --rm -d -p <free>:<containerPort> --name=<unique> <image>`.
3. `t.Cleanup(func() { docker logs <name>; docker rm -f <name> })`.
4. Para `type: server` / `frontend`:
   - Loop até 30s: `http.Get("http://localhost:<free>/health")` → 2xx? OK. 404? tentar `/`. timeout? continuar polling.
   - 0 sucessos em 30s → `t.Fatalf` com logs.
5. Para `type: worker`:
   - Esperar 5s.
   - `docker inspect --format '{{.State.Running}}'` → "true"? OK. "false"? capturar exit code + logs.

Invariantes:
- Cada call usa `--name` único (UUID prefix) para parallel safety.
- Cleanup roda mesmo se `t.Fatal` no meio (via `t.Cleanup`).
- Porta livre escolhida ANTES do run, não retentada.

Edge cases:
- Imagem que demora 30s para subir (gradle warming): timeout configurável via `probeTimeout` arg.
- Workers que `exit 0` voluntariamente após N segundos: aceitável; verificar exit code = 0.

#### Tasks
1. Criar `runtime_probe.go` com `runAndHealthcheck(t, image, AppConfig, probeTimeout) error`.
2. Helper `pickFreePort()`.
3. Helper `captureLogsAndCleanup(t, containerName)`.
4. Testes unitários usando imagem `nginx:alpine` como server e `busybox sleep 10` como worker.

#### TDD
```
RED:     TestRunAndProbe_ServerHealthy — image nginx:alpine port 80 → /  → 200
RED:     TestRunAndProbe_ServerCrashes — image alpine sh -c "exit 1" → fail com logs
RED:     TestRunAndProbe_WorkerStaysAlive — busybox sleep 10 → success (alive at 5s)
RED:     TestRunAndProbe_WorkerExits0 — alpine echo done → success (exit 0)
GREEN:   Implementar runAndHealthcheck.
REFACTOR: Extrair captureLogs como func separada se ficar grande.
VERIFY:  go test -tags e2e ./e2e/ -run TestRunAndProbe
```

#### Acceptance Criteria
- [ ] Helper compila + 4 testes verdes
- [ ] Cleanup garante zero contêineres residuais após `go test ./e2e/ -run TestRunAndProbe`
- [ ] Logs capturados em falha são incluídos na `t.Fatalf` message
- [ ] Free port picking não-flaky em 100 runs consecutivos

#### DoD
- [ ] Helper integrado
- [ ] Testes verdes
- [ ] Documentação inline do contract

---

### T0.3 — testcontainers para templates com DB-deps

#### Objective
Helper que, para templates que declaram dependência DB/Redis (via convenção theo.yaml ou env var `DATABASE_URL`), instancia o container correspondente antes do `docker run` do template.

#### Evidence
Templates como `monorepo-go/worker`, `monorepo-python/worker` provavelmente requerem Postgres/Redis. Sem isso, o worker entra em crash-loop por connection refused. D5.

#### Files to edit
```
e2e/runtime_deps.go (NEW) — depDispatcher
go.mod — adicionar github.com/testcontainers/testcontainers-go (já recomendado em plan anterior)
```

#### Deep file dependency analysis
- **`runtime_deps.go`** — depende de testcontainers-go. Função `provisionDeps(t, AppConfig) (envVars map[string]string, cleanup func())`.
- Detecta deps por convenção: se o `theo.yaml::apps.<name>.framework` é "rails" ou contém keyword DB → provision Postgres. Configurable via lista interna.

#### Deep Dives
Heurística inicial (ajustável conforme falhas reais):
- `framework: rails` → Postgres
- `framework: django` → Postgres
- `type: worker` + import detectado de redis lib → Redis
- Outros → no-op

Para Phase 1, podemos rodar SEM deps primeiro e ver quais falham por connection refused; então adicionar deps reativamente.

#### Tasks
1. Adicionar `testcontainers-go` ao `go.mod`.
2. Criar `runtime_deps.go` com `provisionDeps(t, cfg) (env, cleanup)`.
3. Implementar provisioners para Postgres e Redis (2 mais comuns).
4. Test que instancia Postgres + retorna URL válida.

#### TDD
```
RED:     TestProvision_Postgres — provisionDeps quando framework=rails → DATABASE_URL set + reachable
RED:     TestProvision_Noop — framework=express → empty env, no-op cleanup
GREEN:   Implementar.
REFACTOR: None.
VERIFY:  go test -tags e2e ./e2e/ -run TestProvision
```

#### Acceptance Criteria
- [ ] Postgres provisioner funciona
- [ ] Redis provisioner funciona
- [ ] Cleanup remove os containers de dep

#### DoD
- [ ] Helper integrado
- [ ] Testes verdes

---

## Phase 1: First-pass discovery

**Objective:** rodar o gate runtime contra os 19 templates pela primeira vez, coletar quais falham e por quê.

### T1.1 — `TestE2E_TheoStacksTemplates_Runtime`

#### Objective
Test que, para cada um dos 19 templates: render → theopacks-generate → docker build → docker run via T0.2 → assert health.

#### Evidence
Conjunto de tests prontos em `e2e/` cobre build. Falta o tier seguinte.

#### Files to edit
```
e2e/theo_stacks_runtime_test.go (NEW) — TestE2E_TheoStacksTemplates_Runtime
```

#### Deep file dependency analysis
- **`theo_stacks_runtime_test.go`** — reusa renderTemplate, runTheopacksOnTemplate do existing `theo_stacks_test.go`. Adiciona docker build + runAndHealthcheck (T0.2).
- Skip gracioso quando Docker indisponível.

#### Deep Dives
Estrutura:
```go
for _, exp := range templateExpectations {
    t.Run(exp.template, func(t *testing.T) {
        t.Parallel()
        rendered := renderTemplate(t, srcDir, t.TempDir())
        df := runTheopacksOnTemplate(t, rendered, exp.appName, exp.appPath)
        tag := buildImage(t, df, rendered)
        cfg := loadTheoYAML(t, rendered, exp.appName)
        runAndHealthcheck(t, tag, cfg, 60*time.Second)
    })
}
```

`templateExpectations` (já existe em `theo_stacks_test.go`) é reaproveitado.

Paralelização: max 4 contêineres simultâneos via semaphore (resources do runner). Implementação: `chan struct{}` buffered = 4.

#### Tasks
1. Implementar test loop.
2. Adicionar semaphore de 4-parallel.
3. Captura de logs estruturada no t.Cleanup.
4. Rodar localmente; coletar resultados.

#### TDD
```
RED:     TestE2E_TheoStacksTemplates_Runtime/<each> — antes do implement, COMPILE-FAIL.
GREEN:   Implementação completa.
REFACTOR: None.
VERIFY:  go test -tags e2e -run TestE2E_TheoStacksTemplates_Runtime ./e2e/ -timeout 30m
```

#### Acceptance Criteria
- [ ] Test compila
- [ ] Roda em < 30 min total
- [ ] Falhas produzem output com logs do contêiner

#### DoD
- [ ] Test integrado
- [ ] Lista de PASS/FAIL gravada em `/tmp/runtime-results.txt`

---

### T1.2 — Triage das falhas

#### Objective
Para cada template que falhou em T1.1, classificar em `{theo-packs-bug, theo-stacks-bug, test-expectation-bug}` e criar sub-task em Phase 2.

#### Evidence
Saída de T1.1.

#### Files to edit
```
docs/plans/theo-stacks-build-and-run-plan.md — adicionar sub-tasks T2.N para cada finding
```

#### Deep file dependency analysis
- Triage docs/decisões. Sem código.

#### Deep Dives
Categorização:
- **theo-packs bug**: o Dockerfile gerado tem CMD errada / porta hardcoded / USER errado → fix no provider.
- **theo-stacks bug**: source do template usa `127.0.0.1` em vez de `0.0.0.0`, `/health` route não definida, etc. → PR upstream.
- **test-expectation bug**: o `theo.yaml` declara `port: 3000` mas o template real serve em 8080 → triage manual: corrigir theo.yaml (upstream) OU o teste (se a convenção for ambigua).

#### Tasks
1. Para cada falha em `/tmp/runtime-results.txt`, abrir log do contêiner.
2. Classificar via inspect do source + Dockerfile gerado.
3. Adicionar sub-task em `Phase 2` neste documento.

#### TDD
```
RED:     N/A (triage manual)
GREEN:   N/A
REFACTOR: None.
VERIFY:  Cada falha em T1.1 tem sub-task correspondente em Phase 2.
```

#### Acceptance Criteria
- [ ] 100% das falhas catalogadas
- [ ] Cada categoria tem owner (theo-packs OR theo-stacks PR)

#### DoD
- [ ] Phase 2 expandida com sub-tasks numeradas

---

## Phase 2: Fix loop (iterative)

**Objective:** corrigir cada bug descoberto até gate passar 19/19. Sub-tasks criadas dinamicamente conforme falhas surgem.

### T2.{N} — Bug template `template-name`

Template repetível. Para cada bug:

#### Objective
Corrigir 1 modo de falha específico identificado em T1.1 + T1.2.

#### Evidence
Stack trace / logs do contêiner em `/tmp/runtime-results.txt` ou seção específica em T1.2.

#### Files to edit
```
(varies — depends on diagnosis)
- Bug no provider: core/providers/<lang>/<lang>.go
- Bug no source upstream: PR no theo-stacks
- Bug no test: e2e/theo_stacks_runtime_test.go
```

#### Deep file dependency analysis
Caso por caso.

#### Deep Dives
Para cada bug, documentar:
- Root cause (1-2 sentences)
- Fix strategy (no workaround — actual fix)
- Lateral risk (que outros templates podem ser afetados)

#### Tasks
1. Reproduzir o bug isoladamente
2. Aplicar fix
3. Re-rodar `TestE2E_TheoStacksTemplates_Runtime/<template>` localmente
4. Se passar, marcar task completed e ir para próximo bug
5. Se theo-stacks PR: abrir PR, aguardar merge, re-clone, re-test

#### TDD
```
RED:     TestE2E_TheoStacksTemplates_Runtime/<template> — currently failing
GREEN:   Apply fix; re-run test.
REFACTOR: None expected.
VERIFY:  go test -tags e2e -run "TestE2E_TheoStacksTemplates_Runtime/<template>" ./e2e/
```

#### Acceptance Criteria
- [ ] Bug específico reproduzido e diagnosed
- [ ] Fix landed (commit local OR PR upstream merged)
- [ ] Teste isolado verde
- [ ] Não regrediu outros 18 templates

#### DoD
- [ ] Task fechada quando teste verde
- [ ] Loop continua até zero falhas

---

(Phase 2 will expand to T2.1, T2.2, ... T2.N as bugs are discovered.
Each instance follows the template above.)

---

## Phase 3: CI integration

**Objective:** runtime gate vira parte do CI permanente.

### T3.1 — L3 nightly + opt-in PR

#### Objective
Workflow GitHub Actions roda `TestE2E_TheoStacksTemplates_Runtime` em L3 nightly. PRs podem opt-in via label `runtime-gate`.

#### Evidence
T1.1 prova viabilidade local. CI integration garante non-regression.

#### Files to edit
```
.github/workflows/e2e-l3.yml — adicionar step "Runtime gate"
.github/workflows/nightly.yml — idem
```

#### Deep file dependency analysis
- Workflow existente já clona theo-stacks (T1.5 do plano anterior). Adicionar step para rodar o test runtime.

#### Deep Dives
Cuidados específicos do CI:
- Docker daemon disponível? Sim (ubuntu-latest tem)
- Memory: nginx + go + java contêineres simultâneos demandam ~6GB. Sharding (T5.2 do plano anterior) ajuda.
- testcontainers networking no GHA: requer `--network=host` ou bridge — testar.

#### Tasks
1. Adicionar step ao L3 nightly.
2. Adicionar label-gated step ao PR workflow.
3. Validar via push de branch teste.

#### TDD
```
RED:     N/A (workflow integration)
GREEN:   Workflow verde no main após merge.
REFACTOR: None.
VERIFY:  Trigger manual via workflow_dispatch.
```

#### Acceptance Criteria
- [ ] Workflow runs em < 45 min
- [ ] Logs estruturados no GitHub Actions

#### DoD
- [ ] CI integrado
- [ ] Doc atualizada

---

### T3.2 — Compatibility matrix update

#### Objective
`docs/theo-stacks-compatibility.md` ganha coluna "Runtime status" e data de last-verified.

#### Evidence
Doc atual só cobre "build status".

#### Files to edit
```
docs/theo-stacks-compatibility.md — coluna nova + seção "Runtime verified"
```

#### Tasks
1. Adicionar coluna na tabela
2. Seção "Runtime verified end-to-end" com timestamp

#### TDD
N/A (doc-only).

#### Acceptance Criteria
- [ ] Coluna runtime populated 19/19
- [ ] Timestamp matching o último run verde

#### DoD
- [ ] Doc commit + push

---

## Phase 4: Final certification

**Objective:** confirmar 19/19 build + run sustentado.

### T4.1 — Run consecutive 3x

#### Objective
Garantir flakiness rate < 5% no gate runtime.

#### Evidence
Sistemas baseados em time-window (healthcheck polling) podem ser flaky.

#### Files to edit
```
docs/benchmarks/runtime-gate-stability.txt (NEW)
```

#### Tasks
1. Rodar `TestE2E_TheoStacksTemplates_Runtime` 3x consecutivamente.
2. Gravar resultado em doc.
3. Se algum flaky, investigar.

#### TDD
```
RED:     N/A
GREEN:   3/3 runs com 19/19 PASS.
VERIFY:  go test -count=3 -tags e2e -run TestE2E_TheoStacksTemplates_Runtime ./e2e/
```

#### Acceptance Criteria
- [ ] 3/3 runs verdes
- [ ] Tempo total documentado

#### DoD
- [ ] Resultado gravado
- [ ] CHANGELOG atualizado

---

### T4.2 — Commit + push final + close goal

#### Objective
Final sync.

#### Files to edit
```
CHANGELOG.md
docs/theo-stacks-compatibility.md
```

#### Tasks
1. Atualizar CHANGELOG com "19/19 templates build AND run verified end-to-end".
2. Commit final + push.

#### Acceptance Criteria
- [ ] Push verde
- [ ] Compatibility matrix mostra 19/19 runtime

#### DoD
- [ ] Done.

---

## Coverage Matrix

| # | Requirement / Gap | Task(s) | Resolution |
|---|---|---|---|
| 1 | `docker run` nunca executado contra os 19 templates | T0.2, T1.1 | Helper runAndHealthcheck + gate test |
| 2 | theo.yaml não consumido para drive expectations | T0.1 | Parser dedicated |
| 3 | Sem testcontainers para DB-deps | T0.3 | Provisioner |
| 4 | Falhas runtime invisíveis em CI | T3.1 | Nightly L3 + label PR |
| 5 | Bugs theo-packs sem-fix workarounds | T2.{N}, D4 | Iterative fix loop, no skip |
| 6 | Bugs theo-stacks sem-fix workarounds | T2.{N}, D4 | PR upstream, aguarda merge |
| 7 | Sem flakiness validation | T4.1 | 3x consecutive runs |
| 8 | Sem doc do runtime status | T3.2 | Compat matrix coluna nova |

**Coverage: 8/8 (100%)**

## Global Definition of Done

- [ ] Phase 0 completa (infra Go pronta)
- [ ] Phase 1 completa (primeira execução + triage feita)
- [ ] Phase 2 completa (zero bugs pendentes; cada um teve commit OR PR upstream merged)
- [ ] Phase 3 completa (CI L3 nightly verde)
- [ ] Phase 4 completa (3x consecutive runs 19/19)
- [ ] **19/19 templates passam `docker build` AND `docker run` + healthcheck**
- [ ] Zero `t.Skip` permanente no gate runtime
- [ ] Compatibility matrix mostra "Runtime ✅" para todos os 19
- [ ] CHANGELOG documenta o ciclo + cada PR/commit em rastro
