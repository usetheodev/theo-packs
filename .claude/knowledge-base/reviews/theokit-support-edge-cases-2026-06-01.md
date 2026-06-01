# Discover Edge Case Review — theokit-support

Data: 2026-06-01
Discovery plan analisado: `.claude/knowledge-base/discoveries/plans/theokit-support-plan.md`
Research questions analisadas: 5 (Q1–Q5)
Edge cases encontrados: **8** (MUST FIX: **3**, SHOULD TEST: **4**, DOCUMENT: **1**)

---

## MUST FIX

### EC-1: Q1 Fase A grep `"start\|cwd\|process.argv"` retorna ~72 matches em `packages/theo/src/cli/`
- **Question afetada:** Q1
- **Família:** Method
- **Cenário:** Sanity-check executado: `grep -rn "start" theokit/packages/theo/src/cli/ | wc -l` → 72. O alternation adiciona `cwd` e `process.argv` (mais ruído). Métodos `.start()` de qualquer servidor, comentários, strings literais — tudo casa. A Fase A vira impossível de triagiar dentro do budget.
- **Impacto:** Q1 estoura budget na Fase A e cai em BLOCKED por exaustão sem que o investigador tenha tido chance real de chegar no handler.
- **Fix sugerido:** Substituir Fase A por: `ls theokit/packages/theo/src/cli/commands/ | grep -i start` (lista handlers) + `grep -n "name: 'start'\|.command('start'\|\"start\"" theokit/packages/theo/src/cli/index.ts`. Reduz para <10 hits úteis.

### EC-2: Q3 Fase A grep `"../"` casa todo import relativo
- **Question afetada:** Q3
- **Família:** Method
- **Cenário:** O pattern `"theokit-sdk\|sibling\|../"` faz `../` casar qualquer import relativo (`from '../foo'`), qualquer path em tsconfig, qualquer doc com `../docs/...`. O sinal real (entries `'../theokit-sdk/...` no `pnpm-workspace.yaml`) afoga em centenas de matches.
- **Impacto:** Q3 vira ruído. Investigador perde tempo filtrando ou desiste e cai em BLOCKED.
- **Fix sugerido:** Trocar o token `../` por `^\s*-\s*'\.\./` (entries YAML iniciando com `- '../`) e restringir scope com `--include="*.yaml" --include="*.md"`. Comando final: `grep -rEn "theokit-sdk|sibling|^\s*-\s*'\.\./" --include="*.yaml" --include="*.md" theokit/`.

### EC-3: Plano não valida que o sibling theokit existe antes de iniciar
- **Question afetada:** todas (Q1–Q4)
- **Família:** Reference path
- **Cenário:** D0 declara que `theokit` vive em `../theokit-tools/theokit/`. Em ambiente sem esse sibling checked out (CI, máquina nova, contributor sem `theokit-tools`), todas as Reads de Q1–Q4 falham silenciosamente ou retornam vazio. Já existe nota similar em `pnpm-workspace.yaml` ("pnpm tolerates missing sibling for contributors without the checkout") — o discovery não tem o mesmo cuidado.
- **Impacto:** `/discover-execute` quebra na primeira Read; sem mensagem clara, parece bug do skill.
- **Fix sugerido:** Adicionar checkpoint inicial no halt-loop: `test -d /home/paulo/Projetos/usetheo/theokit-tools/theokit/package.json` → se falhar, abortar com mensagem orientativa "clone theokit-tools como sibling ou ajuste THEOKIT_PATH". Linha única na seção Halt-loop Checkpoints.

---

## SHOULD TEST

### EC-4: Q1 scope creep para `dev` e `build`
- **Question afetada:** Q1
- **Halt-loop checkpoint sugerido:** Antes de aprofundar em qualquer handler que não seja `start`, marcar como out-of-scope e seguir. Concretamente: durante Fase B, se o investigador encontrar `dev.ts` ou `build.ts`, registrar 1 linha ("dev/build não investigados — fora do escopo Q1") e não abrir o arquivo.

### EC-5: Q2 critério "Build-time / Runtime / Ambos" é subjetivo
- **Question afetada:** Q2
- **Halt-loop checkpoint sugerido:** Antes de classificar deps, registrar no blueprint o critério usado em 1 frase. Sugestão: "Runtime = dep importada por código alcançado a partir do script `start`; Build = dep usada apenas em `build`/`devDependencies`; Ambos = qualquer dep React/Vite usada em SSR/CSR." Sem o critério escrito, qualquer reviewer questiona a tabela.

### EC-6: Q4 pode descobrir que theokit não testa container build
- **Question afetada:** Q4
- **Halt-loop checkpoint sugerido:** Se a varredura `grep -l "docker\|build" theokit/tests/` retornar zero matches, marcar Q4 como ANSWERED-NEGATIVE (não BLOCKED) com a finding "theokit não exercita Docker build em seu próprio CI — lacuna conhecida, theo-packs E2E deve compensar". Essa é uma resposta legítima e útil ao blueprint, não uma falha de investigação.

### EC-7: Q5 pode descobrir que nem turbo nem nx têm "dir-name fallback"
- **Question afetada:** Q5
- **Halt-loop checkpoint sugerido:** Caso ambas as ferramentas exijam o `package.json#name` real (zero fallback), registrar como NEGATIVE finding na tabela comparativa: "Nenhum prior-art para dir-name fallback — fortalece o argumento de fazer o theo-packs ler `package.json#name` no path do app antes de chamar `--filter`." Esse é o desfecho esperado, não falha.

---

## DOCUMENT

### EC-8: Q5 link rot em docs públicas
- **Risco aceito:** O plano permite citações via URL pública (Q5). Páginas de docs do turborepo/nx mudam de URL ocasionalmente. Implementar fallback robusto (snapshot via Wayback, mirror em `references/`) custa mais que o budget de 1h alocado à Q5. Aceitar que algumas URLs podem ficar dead 6-12 meses depois é razoável: o blueprint não é um contrato de longo prazo, é uma decisão técnica para o PR `feat/theokit-support` agora.

---

## Resumo

| Question | Edges encontrados | MUST FIX | SHOULD TEST | DOCUMENT |
|----------|-------------------|----------|-------------|----------|
| Q1 | 2 | 1 (EC-1) | 1 (EC-4) | 0 |
| Q2 | 1 | 0 | 1 (EC-5) | 0 |
| Q3 | 1 | 1 (EC-2) | 0 | 0 |
| Q4 | 1 | 0 | 1 (EC-6) | 0 |
| Q5 | 2 | 0 | 1 (EC-7) | 1 (EC-8) |
| Cross-cutting | 1 | 1 (EC-3) | 0 | 0 |
| **Total** | **8** | **3** | **4** | **1** |

**Veredicto: DISCOVERY PLAN PRECISA DE AJUSTE** (bump v1.0 → v1.1)

Os 3 MUST FIX são incorporáveis com mudanças cirúrgicas:

1. **EC-1** → reescrever a coluna Fase A de Q1 (1 célula da tabela)
2. **EC-2** → reescrever a coluna Fase A de Q3 (1 célula da tabela)
3. **EC-3** → adicionar 1 linha na seção "Halt-loop Checkpoints" (path-exists check antes de qualquer Read)

Os 4 SHOULD TEST são acréscimos pequenos na mesma seção "Halt-loop Checkpoints" + 1 nota de critério em Q2.

EC-8 entra como nota ou ADR menor (D4) — aceitar link rot conscientemente.

Total de mudanças estimadas no plan v1.1: **~15 linhas alteradas**. Nenhuma research question precisa ser reformulada, nenhum corner re-mapeado, coverage 4/4 mantida.

---

## Notas honestas

- Apliquei o checklist real — não passei cego por cada item. Os checks que não se aplicam (ex: "versão do clone" para um sibling local) foram ignorados conforme guia do skill.
- Sanity-check físico executado para EC-1 (grep contagem 72) e EC-3 (workflows existem, mas Q4 não precisa deles — só precisaria se a Fase A as listasse explicitamente, o que não acontece).
- Não inventei edge cases: rejeitei "e se o pnpm muda o YAML spec", "e se Node 22 não roda no debian:bookworm" e similares como teóricos demais.
- O EC-3 é o único cross-cutting — vale o esforço porque protege todas as outras 4 questions com um único checkpoint.
