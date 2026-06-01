# Discovery Plan: theokit-support

> **Version 1.1** — Investigação focada em fechar as lacunas concretas que impedem o theo-packs de gerar Dockerfile funcional para `theokit/examples/*`. **v1.1 incorpora os 8 edge cases identificados em `.claude/knowledge-base/reviews/theokit-support-edge-cases-2026-06-01.md` (3 MUST FIX, 4 SHOULD TEST, 1 DOCUMENT).** O alvo é um *blueprint técnico* que prescreva mudanças mínimas no Node provider do theo-packs (resolução de `package.json#name` real, start command em workspace, falha-rápida para siblings cross-repo) e que sirva de base para o PR `feat/theokit-support`. Reference investigada: `theokit` (repo sibling). Output esperado: `.claude/knowledge-base/discoveries/blueprints/theokit-support-blueprint.md`.

**Slug:** `theokit-support`
**Owner:** paulohenriquevn
**Created:** 2026-06-01
**Time budget:** 4h (theokit: 3h, varredura de prior-art em turbo/nx/moon: 1h — ver D1)

---

## Context

Diagnóstico rodado em 2026-06-01 nesta sessão (`/tmp/theokit-diag/Dockerfile.*`) gerou Dockerfile para os 4 examples do theokit (`deploy-vercel`, `devtools-demo`, `full-stack-agent`, `openrouter-demo`) e identificou **3 falhas bloqueantes**:

- **B1** — `core/providers/node/node.go:156,165,174` constrói `pnpm --filter <appName>...` usando o **nome do diretório** (`full-stack-agent`), mas o `package.json` declara `@usetheo/example-full-stack-agent`. Build falha.
- **B2** — `core/providers/node/node.go:115-116` emite `CMD ["cd <appPath> && npm start"]`; em workspace pnpm, isso não garante resolução do binário `theokit` (declarado como `workspace:*`). Container não inicia.
- **B3** — `core/providers/node/workspace.go:resolvePnpmWorkspaceMembers` aceita silenciosamente entries com prefixo `../` no `pnpm-workspace.yaml`; o theokit tem 3 desses (`../theokit-sdk/packages/{sdk,gateway,gateway-telegram}`) que estão fora do Docker build context. Hoje, falha silenciosa.

A investigação serve para validar três decisões de design **antes** de codar os fixes:

1. Qual o mecanismo correto para resolver dir→package-name em monorepos polimórficos (turbo, pnpm puro, npm workspaces) — buscando prior-art em `turbo` e `nx` para não reinventar.
2. Qual o comportamento real do `theokit start` (lê `scripts.start`? exige binário no PATH? aceita `pnpm exec`?).
3. Qual a política de fail-fast adequada para siblings cross-repo (abortar com mensagem orientativa vs warning + skip), e se há precedente no contrato CLI (`docs/contracts/theo-packs-cli-contract.md`).

Este discovery cita `architecture.md` (DIP layers) como rule de referência — embora o arquivo esteja em estado preliminar (TODOs), o invariante "providers não vazam para CLI" se aplica e será respeitado nos fixes propostos.

---

## Objective

Produzir um blueprint que permita decidir, **com evidência**, qual é a forma mínima e correta de cada um dos três fixes (B1, B2, B3) no theo-packs antes de abrir o PR `feat/theokit-support` na branch `develop`.

Critérios mensuráveis:

- [ ] Todas as 5 research questions abaixo respondidas com citações a paths reais (sibling `theokit/` ou repo local `theo-packs/`)
- [ ] Tabela comparativa: como turbo, nx, pnpm puro resolvem dir→package-name (Q5)
- [ ] Recomendações concretas (uma por bug B1/B2/B3) com diff conceitual + arquivo:linha alvo no theo-packs
- [ ] Lista de testes (unit + e2e) que devem ser adicionados para regredir cada fix
- [ ] `/discover-confidence` verdict ≥ SHIPPABLE_WITH_CAVEATS

---

## In-Scope / Out-of-Scope

### In-Scope

| Projeto | Subdirs in-scope | Razão |
|---|---|---|
| `theokit` (sibling em `/home/paulo/Projetos/usetheo/theokit-tools/theokit/`) | `package.json`, `pnpm-workspace.yaml`, `examples/*/package.json`, `packages/theo/src/cli/`, `scripts/`, `.github/workflows/` (se existirem) | Reference de comportamento esperado e prova viva dos bugs |
| `theo-packs` (este repo) | `core/providers/node/{node.go,workspace.go,workspace_test.go}`, `cmd/theopacks-generate/main.go`, `docs/contracts/theo-packs-cli-contract.md`, `core/dockerfile/testdata/integration_node_pnpm_workspaces.dockerfile` | Onde os fixes vão acontecer; baseline existente |
| `turborepo` (lookup público read-only) | Documentação oficial sobre `--filter` + package-name resolution | Prior-art para Q5 |
| `nx` (lookup público read-only) | Documentação oficial sobre project-name detection em monorepos | Prior-art para Q5 |

### Out-of-Scope (explícito)

| Path / Projeto | Por que excluído |
|---|---|
| `theokit/node_modules/`, `theokit/dist/`, `theokit/coverage/`, `theokit/test-results/` | Build artefacts e ambient state |
| `theokit/fixtures/*` | Dev-only; não são deploy targets, não influenciam a decisão dos fixes |
| `theokit/packages/create-theo/` | Scaffolder; não é deploy target |
| `theokit-sdk` (sibling) | B3 é sobre **detectar** a presença de paths `../`, não sobre buildar o SDK em si. O comportamento esperado é o theo-packs abortar antes de chegar nele. |
| `theo` (produto Theo PaaS, repo separado) | F6/F7 do CHANGELOG são tracked em outro repo |
| Qualquer mudança no theokit | Decisão registrada na conversa: arrumar o theo-packs, não o theokit |

---

## ADRs

### D0 — Reference está fora de `.claude/knowledge-base/references/`

**Decisão:** O `theokit` é tratado como reference legítima mesmo morando em `../theokit-tools/theokit/` (sibling), sem cópia/symlink para `.claude/knowledge-base/references/theokit/`.

**Rationale:** (a) o `theokit` é repo sibling sob a mesma organização — não é um terceiro a ser auditado, é o consumidor primário do theo-packs; (b) clonar dentro de `.claude/knowledge-base/` introduz drift entre duas cópias do mesmo working tree; (c) o invariante do skill ("Cross-Project Rule: never claim a project feature without reading its source") é mantido — todas as citações vão para paths reais lidos durante a execução.

**Alternativas consideradas:** (i) symlink `references/theokit -> ../../../../theokit-tools/theokit` — rejeitada porque `boundary-check.sh` (hook) pode tratar o link como write-zone; (ii) clone shallow — rejeitada porque introduz drift e fica desatualizada.

**Consequências:** Citações no blueprint usam paths absolutos `theokit/...` (relativos a `/home/paulo/Projetos/usetheo/theokit-tools/`) com prefix textual `[theokit] path:linha`. `/discover-confidence` precisa aceitar esse formato — flag manual de revisão se cap por fabricated-citation disparar.

---

### D1 — Time budget + stop conditions

**Decisão:**
- theokit (CLI + workspace + examples): **3h**
- Prior-art turbo/nx/moon (varredura web e docs): **1h**
- Total: **4h**

**Rationale:** o repo theokit é pequeno (~30 fixtures + examples + 2 packages); o esforço-mor é entender o `packages/theo/src/cli/` para responder Q1. Prior-art é tempo-boxado a 1h porque já temos forte intuição de que turbo resolve via `package.json#name` real (Q5 confirma).

**Stop condition — por question:** se Fase A retornar 0 hotspots após 3 retries com variantes de busca (grep alternativo, regex, path adjacente), marcar a question como BLOCKED com razão "Fase A esgotada" e seguir. Não preencher Fase B com chutes.

**Stop condition — por projeto:** se o budget da reference esgotar com questions pendentes, marcar pendentes como BLOCKED com razão "budget esgotado" e produzir blueprint parcial honesto. Esses BLOCKEDs viram seed para próximo discovery.

**Anti-pattern:** NUNCA fabricar resposta. BLOCKED honesto > resposta inventada (Inquebrável §3).

---

### D2 — Profundidade da investigação

**Decisão:** **Read parcial + Grep**. Leituras integrais só nos arquivos críticos (`packages/theo/src/cli/index.ts` e os 4 `examples/*/package.json`). Resto via grep com snippet (`grep -n "padrão"` + `Read` localizado ±15 linhas).

**Rationale:** O `theokit` tem >100 arquivos TS; ler tudo estoura budget. O foco é comportamento observável (qual binário, qual PATH, qual cwd) — não auditoria de qualidade.

**Consequências:** Blueprint pode ter pontos cegos em casos extremos (ex: fluxo `theokit dev` raramente exercitado em produção). Esses pontos cegos viram nota no blueprint, não fabricação.

---

### D3 — Regra do projeto citada

**Decisão:** O blueprint deve respeitar `architecture.md` mesmo em estado preliminar — concretamente: providers ficam em `core/providers/`, CLI em `cmd/theopacks-generate/`, e nenhuma das mudanças propostas viola essa separação. O princípio universal **KISS** (Parte II §10 do CLAUDE.md global) é a guia: prefere a solução mais simples que resolve B1/B2/B3, não a mais elegante.

**Consequências:** Qualquer proposta no blueprint que sugira refactor amplo (ex: novo abstraction layer para "monorepo resolver") será rejeitada e marcada como YAGNI. Os fixes ficam locais às funções com bug.

---

### D4 — Link rot em citações públicas de Q5 [v1.1 — EC-8]

**Decisão:** Citações via URL pública (apenas Q5: docs do turborepo/nx) são aceitas no blueprint sem mitigação de link rot.

**Rationale:** Implementar fallback robusto (snapshot via Wayback Machine, mirror em `references/`) custa mais que o budget de 1h alocado à Q5. O blueprint não é um contrato de longo prazo — é uma decisão técnica que alimenta o PR `feat/theokit-support` agora. Quando os fixes forem mergeados (semanas, não anos), as URLs ainda estarão vivas com alta probabilidade.

**Alternativas consideradas:** (i) Wayback snapshot de cada URL — custo de ~30min, rejeitado por exceder budget; (ii) Copy-paste de trechos relevantes para o blueprint — viável e parcialmente adotado (o investigador deve incluir o snippet citado entre aspas, não só o link).

**Consequências:** Reviewers do blueprint após 6-12 meses podem encontrar URLs dead. Mitigação parcial: incluir snippet textual junto à URL para que o ponto esteja preservado mesmo se o link morrer.

---

## Research Questions

| # | Question | Corner | Reference / Projeto | Fase A (mapa amplo) | Fase B (leitura profunda) | Formato esperado |
|---|---|---|---|---|---|---|
| **Q1** | O que `theokit start` faz por dentro? Espera o binário `theokit` no PATH, lê algum config (`theo.config.ts`), assume cwd = app dir? Em que ponto resolve workspace deps? | techniques | `theokit/packages/theo/src/cli/` | **[v1.1 — EC-1 fix]** `ls theokit/packages/theo/src/cli/commands/ \| grep -i start` (lista handler de `start`) + `grep -nE "name: 'start'\|\\.command\\('start'\|\"start\"" theokit/packages/theo/src/cli/index.ts` (localiza dispatch). Padrão original (`grep "start\|cwd\|process.argv"`) descartado: 72 matches em sanity-check, ruído inviável. | `Read` integral de `packages/theo/src/cli/index.ts` + handler de `start` identificado em Fase A. Capturar: cwd assumido, ENV vars lidos, deps resolvidos | Tabela: aspecto (cwd / ENV / deps) → comportamento → path:linha |
| **Q2** | Quais workspaces cada example de fato consome em runtime (vs só dev)? Qual delta entre dependencies e devDependencies, e quais `workspace:*` são *necessárias* para o container subir? | dependencies | `theokit/examples/{deploy-vercel,devtools-demo,full-stack-agent,openrouter-demo}/package.json` | SKIP Fase A (text-shape) — `Read` direto dos 4 package.json (já confirmados existir) | **[v1.1 — EC-5 fix]** Antes de classificar, registrar no blueprint o critério em 1 frase. Critério canônico: *"Runtime = dep importada por código alcançável a partir do script `start`; Build = dep usada apenas em script `build` ou em `devDependencies`; Ambos = dep usada em ambos os fluxos (típico: React/Vite em SSR+CSR)."* Depois: `Read` integral + tabular: para cada example, listar dependencies `workspace:*` e classificar Build / Runtime / Ambos | Tabela 4×N: example × dep × tipo (build/runtime) × workspace:* yes/no |
| **Q3** | Como o `theokit` documenta/exige a coexistência com `theokit-sdk` sibling? Há script de bootstrap? README com instruções de checkout adjacente? Skip behavior implícito? | tools | `theokit/README.md`, `theokit/CONTRIBUTING.md`, `theokit/scripts/`, `theokit/pnpm-workspace.yaml` (comentários) | **[v1.1 — EC-2 fix]** `grep -rEn "theokit-sdk\|sibling\|^\\s*-\\s*'\\.\\./" --include="*.yaml" --include="*.md" theokit/`. Padrão original (`../` solto) descartado: casava todo import relativo. O novo casa só entries YAML iniciando com `- '../` + menções textuais. | Ler trechos em que o tema aparece. Capturar se a expectativa é "developer clona ambos" ou "publicado no npm em produção" | Resumo (3-5 frases) + 2-3 citações `[theokit] path:linha` |
| **Q4** | Como o theokit testa o próprio build/scaffold? Há padrão de "build minimo container-ready" que possamos espelhar nos testes E2E do theo-packs? | tests | `theokit/playwright.config.ts`, `theokit/vitest.config.ts`, `theokit/scripts/check-bundle-budget.sh`, `theokit/.github/workflows/` (se existir) | `ls theokit/.github/workflows/` + `grep -l "docker\|build" theokit/tests/` + `Read` dos configs vitest/playwright | Ler scripts/configs identificados. Mapear: o que é testado, o que é assumido como pré-condição | Lista de testes/scripts relevantes + lições para o E2E do theo-packs (ex: "espelhar `examples/full-stack-agent` em `theo-packs/examples/node-pnpm-workspaces-scoped`") |
| **Q5** | Como Turborepo e Nx resolvem o argumento `--filter <name>` quando o user passa o nome do diretório vs o nome real do `package.json`? Há prior-art para "dir-name fallback"? | techniques | turborepo (docs públicas) + nx (docs públicas) + `theo-packs/core/providers/node/node.go` (estado atual) | Web/docs search: `"turbo filter package directory name"`, `"nx project name detection"` — limitar a 5 fetches | Ler docs identificados. Comparar lógica com `core/providers/node/node.go:144-176` | Tabela: ferramenta → como faz dir→name → como fallback → recomendação para theo-packs |

**Budget de questions:** 5 perguntas (dentro do range 5-10). Distribuição: techniques=2, deps=1, tools=1, tests=1. Min 1 por corner ✅. Max 3 por corner ✅.

---

## Coverage Matrix

| Corner | Questions mapeadas | Status |
|---|---|---|
| Integration tests | Q4 | Covered |
| Dependencies | Q2 | Covered |
| Tools | Q3 | Covered |
| Techniques | Q1, Q5 | Covered |

**Coverage: 4/4 corners (100%)**

---

## Halt-loop Checkpoints

Para `/discover-execute`:

| Checkpoint | Assertion | Ação se falhar |
|---|---|---|
| **[v1.1 — EC-3] Pré-condição (antes de qualquer Q)** | `test -f /home/paulo/Projetos/usetheo/theokit-tools/theokit/package.json` retorna sucesso | Abortar `/discover-execute` com mensagem: "Sibling theokit não encontrado. Clone `theokit-tools` como sibling de `theo-packs` (mesmo parent dir) ou ajuste `THEOKIT_PATH`. Path esperado: `../theokit-tools/theokit/`." |
| Antes de responder Qx | Todos os paths citados em Fase A existem (validar com `ls`/`test -f`) | Marcar Qx BLOCKED com razão "path inválido", seguir |
| Fase A budget per question | Pelo menos 1 hotspot ou 3 retries de variantes esgotadas | Marcar Qx BLOCKED com razão "Fase A esgotada"; seguir |
| **[v1.1 — EC-4] Q1 scope guard** | Se Fase B encontrar `dev.ts`, `build.ts` ou outro handler ≠ `start`, NÃO abrir o arquivo | Registrar 1 linha no blueprint: "Handler `<nome>` identificado, fora de escopo Q1, não investigado"; seguir com `start` |
| **[v1.1 — EC-5] Q2 critério prévio** | Antes de classificar dep como Build/Runtime/Ambos, o critério está registrado no blueprint em 1 frase | Se ausente, escrever o critério ANTES de preencher a tabela |
| **[v1.1 — EC-6] Q4 fallback negativo** | Se varredura encontrar 0 testes de Docker/container build no theokit | Marcar Q4 como ANSWERED-NEGATIVE (não BLOCKED) com finding: "theokit não exercita Docker build no próprio CI — lacuna conhecida, E2E do theo-packs compensa" |
| **[v1.1 — EC-7] Q5 fallback negativo** | Se nem turbo nem nx tiverem dir-name fallback no `--filter` | Registrar como NEGATIVE finding na tabela: "Nenhum prior-art — fortalece argumento de fazer theo-packs ler `package.json#name` antes de chamar `--filter`" |
| Após responder Qx | Resposta tem ao menos 1 citação `[theokit] path:linha` OU `[theo-packs] path:linha` OU URL pública + snippet (apenas Q5, ver D4) | Re-iterar Qx (1 retry); senão BLOCKED |
| Por projeto | Budget de tempo respeitado (theokit 3h / prior-art 1h) | Quando esgotar, BLOCKED em remanescentes, advance |
| Antes de prometer completo | 4/4 corners têm seção populada no blueprint | Refusar promessa; continuar iterando |

---

## Acceptance Criteria

- [ ] Q1-Q5 respondidas OU explicitamente BLOCKED com razão
- [ ] 4/4 coverage corners populados no blueprint
- [ ] Toda citação `[theokit] ...` aponta para path real (verificado com `ls`)
- [ ] Tabela comparativa turbo/nx (Q5) preenchida
- [ ] Seção "Recomendações" do blueprint contém: (a) diff conceitual + arquivo:linha alvo para B1, (b) idem para B2, (c) idem para B3, (d) lista de testes novos
- [ ] Time budget respeitado (≤4h total ou BLOCKED honesto com remanescente)
- [ ] `/discover-confidence` verdict ≥ SHIPPABLE_WITH_CAVEATS
- [ ] Blueprint salvo em `.claude/knowledge-base/discoveries/blueprints/theokit-support-blueprint.md`

---

## Global Definition of Done

- [ ] Phases concluídas: discover-plan (este doc) → discover-edge-cases → discover-execute → discover-confidence → (improve se necessário) → re-score
- [ ] Verdict de `/discover-confidence` registrado no header do blueprint
- [ ] Zero citações fabricadas (toda `path:linha` foi `Read`/`Grep`-confirmada)
- [ ] Coverage Matrix 100%
- [ ] ADRs citam `architecture.md` (D3) + princípios universais KISS/YAGNI/Não-reinvente (CLAUDE.md global §9-13)
- [ ] Blueprint produz **3 PRs candidatos** (B1, B2, B3) ou um PR unificado `feat/theokit-support`, decisão registrada como ADR no próprio blueprint

---

## Changelog deste plano

- **v1.1 (2026-06-01):** Incorpora os 8 edge cases de `.claude/knowledge-base/reviews/theokit-support-edge-cases-2026-06-01.md`. Refina Fase A de Q1 (EC-1) e Q3 (EC-2); adiciona critério Build/Runtime em Q2 (EC-5); adiciona 5 checkpoints novos no halt-loop (EC-3, EC-4, EC-5, EC-6, EC-7); adiciona ADR D4 (link rot — EC-8). Nenhuma research question reformulada, coverage 4/4 mantida.
- **v1.0 (2026-06-01):** Versão inicial.

---

## Notas de calibração honesta

- **O skill `/discover-plan` assume `cycle-discover.md` e `.claude/knowledge-base/references/` populado.** Nenhum dos dois existe neste repo. Este plano foi adaptado em D0 + nesta nota — se o cycle-discover.md for criado depois, este plan pode precisar de revisão de compliance.
- **Q5 depende de fetches públicos.** Se a rede estiver indisponível durante `/discover-execute`, Q5 vira BLOCKED com razão "rede indisponível" e o blueprint cai para SHIPPABLE_WITH_CAVEATS automaticamente — os fixes B1/B2/B3 ainda podem ser propostos com base no comportamento conhecido do theo-packs sem o benchmark externo.
- **B3 é apenas parcialmente endereçável pelo theo-packs.** O blueprint deve recomendar fail-fast claro, mas o desbloqueio completo do `examples/full-stack-agent` exige decisão arquitetural no theokit (publicar SDK como npm package ou unificar repos). Essa decisão fica fora do escopo deste discovery.
- **v1.1 não alterou coverage nem questions.** Edits foram cirúrgicos sobre métodos (Fase A de Q1/Q3), critério (Q2), 5 checkpoints novos e 1 ADR. Total: ~18 linhas alteradas, dentro da estimativa do review (~15 linhas).
