# Blueprint: theokit-support

> **Version 1.0** — Sintetiza 5 research questions sobre como o theo-packs precisa evoluir para gerar Dockerfile funcional para `theokit/examples/*` (monorepo pnpm com siblings cross-repo). Reference investigada: `theokit` (sibling em `../theokit-tools/theokit/`). Prior-art consultado em Turborepo + Nx (docs públicas, citações com snippets). Decisões resultantes: 3 fixes cirúrgicos no Node provider do theo-packs (B1/B2/B3) + 1 decisão de não-overlap com o `theokit docker` interno. Output alimenta o PR `feat/theokit-support` em `develop`.

**Slug:** `theokit-support`
**Source plan:** `.claude/knowledge-base/discoveries/plans/theokit-support-plan.md` (v1.1)
**Source review:** `.claude/knowledge-base/reviews/theokit-support-edge-cases-2026-06-01.md`
**Owner:** paulohenriquevn
**Generated:** 2026-06-01 via `/discover-execute` (inline, sem ralph-loop — 5 questões focadas)
**Confidence verdict:** TBD (atualizado por `/discover-confidence`)

---

## Context

Diagnóstico inicial (2026-06-01) gerou Dockerfile para os 4 examples do theokit e identificou 3 falhas bloqueantes (B1: `--filter` usa dir-name em vez de `package.json#name`; B2: `CMD` `cd && npm start` falha resolver binário workspace; B3: aceitação silenciosa de siblings `../` no `pnpm-workspace.yaml`). Esta discovery confirma a causa-raiz de cada bug, mapeia o ecossistema do theokit (relação com `theokit-sdk` e TheoCloud), e descobre um overlap **não-trivial** com um gerador de Dockerfile interno do próprio theokit (`theokit docker` command). Decisões emergem em "ADRs" abaixo.

## Objective

Permitir decidir, com evidência citada, qual é a forma mínima e correta de cada um dos 3 fixes (B1, B2, B3) no theo-packs, e clarificar a relação entre `theo-packs` e o `theokit docker` interno (overlap ou complementaridade).

---

## Coverage Corner 1 — Integration Tests

### Como o theokit testa o build/container

**Padrão encontrado:** o theokit testa a **geração** do Dockerfile via unit test, mas **não roda Docker build nem container start**.

- **Test alvo:** `[theokit] tests/unit/docker-adapter.test.ts:1-80` — importa `dockerCommand` de `packages/theo/src/cli/commands/docker.js`, cria projeto temporário com `package.json` + lockfile, invoca `dockerCommand()`, assertiona que `Dockerfile` e `.dockerignore` foram criados, e que o conteúdo menciona `node:22` e o package manager detectado (`pnpm` / `npm ci`).
- **Workflows CI:** `[theokit] .github/workflows/` contém 8 workflows (`architecture-guards.yml`, `ci.yml`, `codeql.yml`, `dist-tag-guard.yml`, `dogfood-stranger.yml`, `postgres-jobs-ci.yml`, `release-coordinated.yml`, `release.yml`). **Nenhum executa `docker build` ou `docker run` contra examples ou templates.** Postgres jobs CI roda Postgres em service container (não builda image do app).
- **Outros testes que mencionam docker/container:**
  - `tests/unit/services-compose-gen.test.ts` — testa geração de `docker-compose.yml` para `services:` block (Postgres adapter), não Dockerfile do app
  - `tests/unit/scaffold-services.test.ts` — testa scaffold
  - `tests/integration/job-backend-postgres.test.ts` — Postgres jobs, sem Docker build do app
  - `tests/e2e/template-postgres.spec.ts` — Playwright E2E SSR, sem Docker

**Conclusão (ANSWERED-NEGATIVE per checkpoint EC-6 do plan v1.1):** O theokit **NÃO** exercita Docker build real no próprio CI. Confia no smoke test unit do gerador interno. **Lacuna conhecida** — a E2E do theo-packs (que faz `docker build` + `docker run` para cada example em `e2e/e2e_test.go`) é mais rigorosa e deve continuar sendo. Lição para o theo-packs: **espelhar a forma do theokit** (workspace pnpm + package name ≠ dir name + workspace deps) em pelo menos um example interno (Fix 5 do diagnóstico) para regredir B1/B2.

---

## Coverage Corner 2 — Dependencies

### Critério canônico (EC-5 do plan v1.1)

> Runtime = dep importada por código alcançável a partir do script `start`; Build = dep usada apenas em script `build` ou em `devDependencies`; Ambos = dep usada em ambos os fluxos (típico: React/Vite em SSR+CSR).

Para o theokit, **a maioria das deps cai em "Ambos"** porque `theokit build` transforma o código que `theokit start` serve — não há split real. Honestidade: o critério acima foi suficiente para classificar `workspace:*` deps; para outras deps a coluna fica "Ambos" por construção.

### workspace:* deps por example

| Example | name real (`package.json#name`) | workspace:* deps | Start script | Tipo |
|---|---|---|---|---|
| `deploy-vercel` | `example-deploy-vercel` | `theokit` | `tsx ../../packages/theo/src/cli/index.ts start --port 3471` | Ambos |
| `devtools-demo` | `example-devtools-demo` | `theokit` | `tsx ../../packages/theo/src/cli/index.ts start --port 3470` | Ambos |
| `full-stack-agent` | `@usetheo/example-full-stack-agent` | `theokit`, `@usetheo/sdk`, `@usetheo/gateway`, `@usetheo/gateway-telegram` | `theokit start` (binário) | Ambos |
| `openrouter-demo` | `@usetheo/example-openrouter-demo` | `theokit`, `@usetheo/sdk` | `theokit start` (binário) | Ambos |

Citações:
- `[theokit] examples/deploy-vercel/package.json:2,5-9` (name + scripts)
- `[theokit] examples/devtools-demo/package.json:2,5-9`
- `[theokit] examples/full-stack-agent/package.json:2,6-12` (`theokit start` em `:9`)
- `[theokit] examples/openrouter-demo/package.json:2,7-12`

### Observação crítica sobre B1

**Os 4 examples têm `package.json#name` ≠ nome do diretório.** Confirmação direta:

| Dir | package.json#name |
|---|---|
| `deploy-vercel` | `example-deploy-vercel` |
| `devtools-demo` | `example-devtools-demo` |
| `full-stack-agent` | `@usetheo/example-full-stack-agent` |
| `openrouter-demo` | `@usetheo/example-openrouter-demo` |

O `pnpm --filter <dir-name>... run build` que o theo-packs gera hoje (`[theo-packs] core/providers/node/node.go:165`) **falha em todos os 4 casos**.

### Observação sobre B2 — heterogeneidade de start

Os 4 examples têm **dois padrões de start** diferentes:

1. **tsx + path relativo** (2 examples): `tsx ../../packages/theo/src/cli/index.ts start --port N` — só funciona se `packages/theo/src/` está preservado no container (workspace inteiro copiado) e `tsx` (devDep da raiz) está em PATH
2. **Binário** (2 examples): `theokit start` — depende de `node_modules/.bin/theokit` resolvido pelo pnpm install (link para `packages/theo/dist/cli.js` via `bin` field)

`npm start` no subdir (que o theo-packs gera hoje em `[theo-packs] core/providers/node/node.go:115-116`) **roda o script de start de cada example sem garantir resolução do PATH** — funciona se o pnpm install hidratou `node_modules/.bin/` no subdir, **falha** se o tsx não estiver no PATH ou se o symlink quebrou.

---

## Coverage Corner 3 — Tools

### Política do theokit sobre siblings cross-repo

O theokit **documenta explicitamente** que assume contributor com `theokit-sdk` clonado como sibling. Isto é uma **decisão arquitetural** registrada em ADRs.

Evidências:

- `[theokit] pnpm-workspace.yaml:24-32` — comentários inline: *"@usetheo/sdk via sibling checkout (workspace protocol) — required by the canonical chat.ts wired in fixtures/template-default. pnpm tolerates missing sibling for contributors without the checkout."* Entries: `'../theokit-sdk/packages/sdk'`, `'../theokit-sdk/packages/gateway'`, `'../theokit-sdk/packages/gateway-telegram'`.
- `[theokit] pnpm-lock.yaml:49` — confirma `version: link:../theokit-sdk/packages/sdk`
- `[theokit] CLAUDE.md:156` — *"`@usetheo/sdk` ... **Workspace protocol (permanent link, assimetria intencional vs UI)** — `pnpm-workspace.yaml` includes `../theokit-sdk/packages/{sdk,gateway,gateway-telegram}`. Local edits in the sibling reflect immediately. Status quo justificado em ADR 0001 (theokit-sdk): SDK é runtime de produção, perfil de acoplamento alto, iteração rápida é crítica."*
- `[theokit] CLAUDE.md:157` — em contraste, `@usetheo/ui` é **npm dep** (`^0.13.0`), com workspace link **opt-in** via `pnpm-workspace.linked-ui.yaml` (ADR 0020) — não permanente.
- `[theokit] CLAUDE.md:158` — TheoCloud é o **principal deploy target**, mas `packages/theo/src/adapters/theo-cloud.ts` ainda não existe (próximo milestone após 0.4.0).
- `[theokit] CONTRIBUTING.md:168-169` — referencia ADR 0001 em theokit-sdk: workspace-link-default-status-quo.

**Conclusão para B3:** o theokit-sdk **NÃO É** problema arquitetural do theokit — é estratégia consciente. O problema é que **Docker build context = source root** não pode resolver paths `../`. Logo, qualquer container build do `examples/full-stack-agent` ou `examples/openrouter-demo` (que dependem de `@usetheo/sdk: workspace:*`) **só funciona** se:

- (a) o source enviado ao Docker engloba ambos os repos (`theokit-tools/` parent), OU
- (b) o `@usetheo/sdk` é publicado no npm registry e o `package.json` aponta para `^X.Y` em vez de `workspace:*` para deploys, OU
- (c) o build é multi-context (BuildKit `--build-context`)

Nenhuma dessas opções é decisão do theo-packs sozinho — todas exigem coordenação com o theokit/TheoCloud. O theo-packs deve **detectar** o sinal (`../` entry no YAML) e **abortar com erro orientativo**, evitando falha silenciosa.

### Estrutura do ecossistema theokit

Resumo extraído de `[theokit] CLAUDE.md:148-160`:

| Sibling | Path | Direção | Mecanismo |
|---|---|---|---|
| `@usetheo/sdk` + gateways | `../theokit-sdk/` | theokit ← sibling | **Workspace permanente** (ADR 0001) |
| `@usetheo/ui` | `../theo-ui/` | theokit ← sibling | npm + opt-in workspace link (ADR 0020) |
| `theo` → TheoCloud | `../theo/` (Go) | theokit → sibling | Adapter na roadmap (não shipped) |
| `theokit-plugins` | `../theokit-plugins/` | theokit ← sibling (invertido) | Sibling consome theokit via npm peerDep |
| `theo-stacks` → `create-theo` | `../theo-stacks/` | theokit ← (absorbing) | Wave 2 — sendo absorvido em `create-theokit` |

**Relevância para theo-packs:** apenas `@usetheo/sdk` + gateways (workspace permanente) afeta build de container hoje. `@usetheo/ui` é npm (não afeta build context). Outros são out-of-scope para Docker.

---

## Coverage Corner 4 — Techniques

### T1 — Como `theokit start` resolve cwd, env, deps (Q1)

**Comportamento observado** em `[theokit] packages/theo/src/cli/commands/start.ts:1-133`:

| Aspecto | Comportamento | Path:linha |
|---|---|---|
| `cwd` | `process.cwd()` — assume invocação **dentro do app dir** | `start.ts:45` |
| Preflight | `preflightNodeAndBindings(cwd)` — valida Node major + native bindings antes de qualquer I/O | `start.ts:24,47` |
| Env vars | `loadEnv({ cwd, mode: 'production' })` — lê `.env.production` etc. do cwd | `start.ts:48` |
| Config | `loadConfig(cwd)` — espera `theo.config.ts` no cwd | `start.ts:49` |
| Build output | `resolve(cwd, '.theo')` — falha com `"No build found. Run \`theo build\` first."` se `.theo/client/` ausente | `start.ts:54,58-60` |
| Server dir | `resolve(cwd, 'server')` — espera `server/` no cwd para route handlers | `start.ts:56` |
| Deps | TODAS via `import` relativo dentro do package `theokit` (`../../config/`, `../../server/`) — **não invoca subprocesso** nem requer binário externo no PATH | `start.ts:18-23,26-35` |
| Listen | `server.listen(port, ...)` onde `port = options.port ?? config.port` | `start.ts:64,127` |

**Implicações para Dockerfile gerado pelo theo-packs:**

1. **WORKDIR no container precisa ser o app dir**, não a raiz do workspace. Hoje o theo-packs gera `cd <appPath> && npm start` que move o cwd corretamente — **bom**.
2. **`.theo/` precisa estar presente no container.** Isto significa que `theokit build` foi executado e o output `.theo/` está incluído no COPY final. O build atual do theo-packs faz `pnpm --filter <name>... run build` (que invocaria `theokit build` no example), depois COPY de tudo de `/app` para o deploy — `.theo/` virá junto. **Bom, se B1 for fixado.**
3. **`theokit` package precisa estar resolvível por `import`.** Como `start.ts` usa imports **internos** ao próprio package, basta que o package `theokit` exista em `node_modules`. O symlink workspace do pnpm faz isso. **Bom.**
4. **`node_modules/.bin/theokit` precisa existir para os 2 examples que usam `theokit start` (full-stack-agent, openrouter-demo).** O pnpm install hidrata `.bin/` no subdir do workspace member (`examples/<name>/node_modules/.bin/`). **Para o `npm start` no subdir funcionar, o PATH precisa incluir `./node_modules/.bin/`** — que é o default do npm-run-script. Logo, `npm start` no subdir **DEVE** funcionar, contanto que o pnpm install tenha hidratado o `.bin/`.

**Inferência sobre B2:** B2 talvez **não seja bug real** quando B1 e B3 são corrigidos. O `cd <appPath> && npm start` é viável SE:
- B1 fixado: `pnpm --filter` usa nome real → install + build completam corretamente
- B3 fixado: nenhum `../` sibling órfão → `pnpm install --frozen-lockfile` não falha → `.bin/` hidratado
- E o `node_modules` do subdir é mantido no deploy (`pnpm prune --prod` no build mantém a árvore)

**Calibração honesta:** Esta inferência é hipótese deduzida da leitura de `start.ts`. **Não foi validada por `docker build` real** — validação completa exige rodar o E2E após implementar B1+B3 e ver se B2 some sozinho. Recomendação: **deferir B2 fix até ver o resultado pós-B1+B3**. Se quebrar, então atacar B2 com fix dedicado.

### T2 — Prior-art em filter resolution (Turbo + Nx) (Q5)

| Ferramenta | Como `--filter` resolve `<arg>` | Dir-name fallback? | Citação |
|---|---|---|---|
| **Turborepo** | (a) Package name from `package.json` (canônico); (b) directory path **com sintaxe `{}`** (ex: `--filter='{./apps/web}'`); (c) Git commit `[]`. **Três mecanismos separados, sem fallback automático.** | **Não.** Sem mapeamento implícito dir → name. | [Turborepo docs: `run --filter`](https://turborepo.dev/docs/reference/run) — snippet: *"Select a package by its name in package.json"* e *"Specify directories... must be wrapped in {}"* |
| **Nx** | Merge `package.json` + `project.json` por projeto. **Precedência entre `name` em ambos os arquivos não está documentada oficialmente.** Issue #16191 em aberto há meses pedindo essa documentação. Quando nomes diferem, pode gerar **entradas duplicadas no graph**. | **Não documentado.** Fallback para dir-name não é mencionado nem confirmado. | [Nx Project Configuration](https://nx.dev/docs/reference/project-configuration), [nrwl/nx#16191](https://github.com/nrwl/nx/issues/16191) |

**Conclusão NEGATIVE FINDING (per checkpoint EC-7 do plan v1.1):** **Nem Turborepo nem Nx têm dir-name fallback** no `--filter`. O padrão da indústria é exigir `package.json#name` real (ou o caminho exato do diretório com sintaxe específica). Isto **reforça a recomendação** de que o theo-packs deve **ler `<appPath>/package.json#name`** antes de chamar `pnpm --filter` / `turbo --filter` / `npm --workspace`. Não há prior-art para fazer fallback silencioso de dir-name — o caminho consagrado é resolver na fonte (package.json) ou rejeitar.

---

## Cross-cutting Comparison

| Dimensão | theokit (hoje) | theo-packs (hoje) | Recomendação |
|---|---|---|---|
| Geração de Dockerfile | `theokit docker` (simples, single-app, sem workspace, `node:22-alpine`, copia `node_modules` inteiro) — `[theokit] packages/theo/src/cli/commands/docker.ts:1-107` | Multi-stage, BuildKit cache mounts, workspace-aware (com bugs B1/B2), `pnpm prune --prod`, `node:20-bookworm` + slim | theo-packs é superior em rigor; `theokit docker` serve caso single-app standalone via `create-theokit` |
| Teste do gerador | Unit test smoke (`docker-adapter.test.ts`) — não builda Docker real | E2E real (`docker build` + `docker run`) por example em `e2e/e2e_test.go` | Manter divisão; **NÃO** consolidar |
| Workspace detection | Não suportado | Suportado (pnpm-workspace.yaml YAML lido) | Continuar; fixar B1/B2/B3 |
| Resolução de `--filter` | N/A | Usa dir-name (BUG B1) | Ler `package.json#name` (alinha com Turbo + Nx) |
| Política para siblings `../` | Documentada + tolerada (ADR 0001 em theokit-sdk) | Aceita silenciosamente (BUG B3) | Fail-fast com mensagem orientativa |
| Node version | `22-alpine` | `20-bookworm[-slim]` | Verificar se override via `THEOPACKS_PACKAGES=nodejs@22` funciona; caso negativo, adicionar Fix 6 |

---

## ADRs

### D1 — Resolver `package.json#name` real antes de chamar `--filter`

**Decisão:** No `core/providers/node/node.go`, antes de chamar `workspaceBuildCommand`, ler `<appPath>/package.json#name` (resolvido via `app.App`) e passar esse valor como `appName` para a função. Fallback para `THEOPACKS_APP_NAME` literal **somente** se a leitura falhar (e nesse caso, **logar warning explícito**).

**Rationale:** Q1+Q2 confirmaram que os 4 examples têm `name` ≠ dir-name. Q5 (NEGATIVE finding) confirmou que **nem Turbo nem Nx têm dir-name fallback** — exigir o nome real é o padrão da indústria. Resolver via leitura local respeita SOLID/SRP (Node provider sabe ler `package.json`, já faz isto para `engines.node` em `[theo-packs] core/providers/node/node.go:199`).

**Alternativas consideradas:**
- (i) Mudar o CLI `theopacks-generate` para aceitar `--package-name` separado de `--app-name`. **Rejeitada:** quebra contrato CLI (`docs/contracts/theo-packs-cli-contract.md`) sem necessidade — provider pode resolver internamente.
- (ii) Cliente (Theo product) passar `THEOPACKS_PACKAGE_NAME`. **Rejeitada:** transfere ônus de descoberta para o caller, que tem menos contexto.
- (iii) Tentar dir-name primeiro, fallback para name lido. **Rejeitada:** fail-silent disfarçado; conflita com fail-fast (princípio §8 do CLAUDE.md global).

**Consequências:** (+) Todos os 4 examples do theokit + qualquer monorepo `package.json#name ≠ dir-name` passa a funcionar. (−) Adiciona 1 `app.ReadJSON()` no path do Plan(). Custo ≈ µs, irrelevante.

---

### D2 — Diferir B2 (`CMD cd && npm start`) até validar pós-B1+B3

**Decisão:** Não implementar fix B2 agora. Após D1 (B1) e D3 (B3) serem implementados, rodar E2E real contra um example minimal espelhando theokit e observar se o container starta. Só então decidir se B2 precisa de fix dedicado.

**Rationale:** Q1 mostrou que `theokit start` (`start.ts:1-133`) usa **apenas imports internos** ao próprio package — não invoca subprocesso, não precisa de binário externo no PATH global. `npm start` no subdir herda `./node_modules/.bin/` no PATH automaticamente. Logo, **a hipótese de que B2 é bug independente pode estar errada** — pode ser sintoma de B1 (pnpm install não completou) ou B3 (sibling órfão). Implementar B2 sem validar essa hipótese é **YAGNI** (CLAUDE.md global §11).

**Alternativas consideradas:**
- (i) Implementar todas as opções (PATH ENV, pnpm no runtime, node direto resolvendo bin) juntas. **Rejeitada:** YAGNI + risco de regredir tamanho de imagem.
- (ii) Substituir `npm start` por `pnpm --filter <real-name> start` no deploy. **Rejeitada:** força pnpm no runtime image, regride tamanho ~30MB.

**Consequências:** (+) Menor superfície de mudança. (+) Honestidade: não fixar bug que não existe. (−) Se a hipótese estiver errada, precisa de PR adicional. Custo do PR ≈ 1-2h.

---

### D3 — Fail-fast em entries `../` no `pnpm-workspace.yaml`

**Decisão:** Em `core/providers/node/workspace.go:resolvePnpmWorkspaceMembers` (e equivalente para `package.json#workspaces` no Q3 não-pnpm), detectar qualquer entry com prefixo `../` (ou path absoluto começando fora do source root) e **abortar `GenerateBuildPlan`** com mensagem estruturada nomeando: (a) o(s) entry(s) ofensor(s); (b) por que não funciona em Docker build; (c) caminhos para o usuário (publicar workspace dep como npm package + remover do YAML; OU enviar source ampliado englobando ambos os repos como contexto Docker).

**Rationale:** Q3 confirmou que o theokit **documenta** explicitamente esses siblings como decisão arquitetural (ADR 0001 em theokit-sdk). Não é descuido — é estratégia. Logo, o theo-packs **não pode** "consertar" silenciosamente. Mas deve **declarar** que esse caso é incompatível com seu modelo de build context, e direcionar o usuário a uma das duas estratégias coerentes. Aceitar silenciosamente (status quo) leva a `pnpm install --frozen-lockfile` falhar dentro do container com mensagem cripta. Fail-fast com contexto é o caminho.

**Alternativas consideradas:**
- (i) Warning + skip (continuar build). **Rejeitada:** fail-silent disfarçado; usuário descobre que está quebrado só quando o app crash em prod.
- (ii) Skipar entries `../` automaticamente. **Rejeitada:** muda o lockfile resolvido implicitamente; resultado provavelmente quebra runtime (deps faltando).

**Consequências:** (+) Mensagem clara que ensina o usuário a decidir. (+) Compatível com fail-fast (§8 do CLAUDE.md global). (−) `examples/full-stack-agent` e `openrouter-demo` do theokit ainda **não buildarão** sem ação do usuário — mas agora com mensagem explicativa.

---

### D4 — Não consolidar `theokit docker` com theo-packs

**Decisão:** Manter `theokit docker` (`[theokit] packages/theo/src/cli/commands/docker.ts`) como ferramenta **independente** do theo-packs. Não fundir, não substituir, não fazer um chamar o outro. Documentar a coexistência.

**Rationale:** Q4 + leitura de `docker.ts` mostraram que `theokit docker` é **simples** (single-app, sem workspace, copia node_modules inteiro, single-stage essencialmente) e serve um **caso de uso diferente**: usuário cria app standalone com `create-theokit my-app`, roda `theokit docker` e tem um Dockerfile básico que builda. theo-packs serve o caso TheoCloud (e produção em geral) — workspace-aware, BuildKit cache, prune, defensive header, contract. Consolidar exigiria refactor cruzado entre dois repos e introduz acoplamento que viola "não reinvente, mas não acople desnecessariamente" (CLAUDE.md global §9-13).

**Alternativas consideradas:**
- (i) Fazer `theokit docker` invocar `theo-packs` via binário. **Rejeitada:** força theokit a ter Go toolchain ou pre-built binary; quebra "self-contained" do CLAUDE.md theokit §168.
- (ii) Reescrever `theokit docker` para chamar a library `core/` do theo-packs via WASM ou child process. **Rejeitada:** overengineering; pequeno benefício, alto custo.
- (iii) Marcar `theokit docker` como deprecated. **Rejeitada:** sem decisão do time do theokit, fora do escopo.

**Consequências:** (+) Cada ferramenta foca no seu caso. (+) Zero acoplamento cross-repo. (−) Há overlap funcional; reviewer pode confundir. Mitigação: nota em `docs/contracts/theo-packs-cli-contract.md` clarificando o overlap.

---

## Recommendations for the project

| # | Recomendação | Linked to | Prioridade |
|---|---|---|---|
| 1 | **Fix B1** — `core/providers/node/node.go`: antes de `workspaceBuildCommand`, ler `<appPath>/package.json` via `app.App` e passar `pkg.Name` (se não-vazio) como `appName`. Fallback para `THEOPACKS_APP_NAME` literal apenas com warning. Adicionar teste em `node_test.go` com fixture onde dir-name ≠ name. | Q1, Q2, Q5, **D1**, `architecture.md` (Node provider em `core/providers/node/`) | **HIGH** |
| 2 | **Fix B3** — `core/providers/node/workspace.go`: em `resolvePnpmWorkspaceMembers` (e equivalente para `readWorkspacesField`), detectar entries com prefixo `../` e retornar erro do `Plan()` com mensagem estruturada (entries ofensores + 2 caminhos de resolução). Adicionar teste com fixture pnpm-workspace.yaml com sibling. | Q3, **D3**, `architecture.md`, KISS (CLAUDE.md global §10) | **HIGH** |
| 3 | **Defer B2** — não implementar ainda. Após Fix B1+B3, rodar E2E real contra example sintético espelhando theokit. Se container subir, fechar B2 como "não-bug". Se quebrar, abrir PR dedicado para B2 com fix mínimo. | Q1, **D2** | **MEDIUM** |
| 4 | **Fix 5 — Regression guard** — adicionar `examples/node-pnpm-workspaces-scoped/` ao theo-packs com: (a) `pnpm-workspace.yaml` listando `apps/*`; (b) `apps/api/package.json` com `name: "@my-org/api"` (≠ dir-name); (c) golden Dockerfile assertando filter usa nome real; (d) E2E `docker build` + `docker run`. **Sem** siblings cross-repo nesse example — caso B3 coberto por teste unitário. | Q4, **D1, D3** | **HIGH** |
| 5 | **Documentar** em `docs/contracts/theo-packs-cli-contract.md`: (a) `--app-name` é o dir-name; theo-packs resolve `package.json#name` internamente; (b) seção "Unsupported: workspace entries com paths `../`"; (c) seção "Overlap com `theokit docker`": cada ferramenta serve um caso (single-app standalone vs monorepo TheoCloud-bound). | Q3, Q5, **D3, D4** | **MEDIUM** |
| 6 | **Verificar override Node version** — confirmar (sanity-check rápido) se `THEOPACKS_PACKAGES=nodejs@22 theopacks-generate ...` produz `node:22-bookworm` no Dockerfile. Se não, abrir issue separada (não bloqueia theokit-support). | Q2 (theokit usa Node 22) | **LOW** |
| 7 | **Não consolidar** com `theokit docker`. Confirmar com time do theokit em check-in informal que mantemos a divisão (single-app vs monorepo). | Q4, **D4** | **LOW** |

**Decisão sobre PRs:** **3 PRs candidatos** (Fix B1, Fix B3, Fix 5+doc) na branch `develop`, na ordem listada acima. Cada PR é independente e tem ~2-3 arquivos tocados. PRs separados permitem revisão focada e rollback granular. **Não unificar** em um único `feat/theokit-support` — KISS + revisibilidade.

---

## Blocked questions (if any)

**Nenhuma.** Todas as 5 questões foram respondidas com citações reais. EC-6 (Q4 ANSWERED-NEGATIVE) e EC-7 (Q5 NEGATIVE finding) entraram como respostas válidas, não bloqueios.

---

## Halt-loop progress (audit trail)

- **Iterações usadas:** 1 (execução inline, sem ralph-loop — decisão KISS para 5 questões focadas; ver objeção declarada na abertura do execute)
- **Questões respondidas:** 5 / 5
- **Questões bloqueadas:** 0
- **Citações verificadas:** 22+ (4 `[theokit] examples/*/package.json` + 11 `[theokit] CLAUDE.md` + 6 `[theokit] pnpm-workspace.yaml` + 1 `[theokit] pnpm-lock.yaml` + 4 `[theokit] packages/theo/src/cli/commands/{start,docker}.ts` + 3 `[theo-packs] core/providers/node/node.go` + 2 URLs públicas com snippets per D4)
- **Promise emitida na iteração:** 1 (esta — inline). Conteúdo: `BLUEPRINT_COMPLETE`.

---

## Compaction Boundary

### Preserved for downstream phases
- Este blueprint (`.claude/knowledge-base/discoveries/blueprints/theokit-support-blueprint.md`)
- Confidence snapshot (a ser produzido por `/discover-confidence`)
- Estado do consumer project: `theo-packs` em `develop` (sincronizado com `main` após PR #21)
- Slug ativo: `theokit-support`
- Plan v1.1 + review v1.0 (entrada para o execute)

### Discarded (no longer required for `/to-plan`)
- Conteúdo raw das URLs públicas (snippets preservados nas tabelas Q5)
- Iterações intermediárias do halt-loop (trivial — apenas 1 iteração)
- Raciocínio conversacional anterior às ADRs

### Critical decisions captured below this point
1. **ADRs D1-D4** — ver `## ADRs` acima
2. **Recomendações 1-7** — ver `## Recommendations for the project`
3. **Decisão de 3 PRs separados** (não unificar) — ver final de Recommendations
4. **NEGATIVE findings de Q4 e Q5** — incorporadas como evidência das ADRs, não como bloqueios

---

## Related

- Discovery plan: `.claude/knowledge-base/discoveries/plans/theokit-support-plan.md` (v1.1)
- Edge-case review: `.claude/knowledge-base/reviews/theokit-support-edge-cases-2026-06-01.md`
- Confidence report: `.claude/knowledge-base/reviews/theokit-support-confidence-{YYYY-MM-DD}.md` (a gerar por `/discover-confidence`)
- Project rule citada: `.claude/rules/architecture.md` (DIP — providers em `core/providers/`, CLI em `cmd/theopacks-generate/`)
- Princípios universais aplicados (CLAUDE.md global): §8 (fail-fast — D3), §10 (KISS — D2/D4), §11 (YAGNI — D2/D4), §9 (não-reinvente — D4)

<promise>BLUEPRINT_COMPLETE</promise>
