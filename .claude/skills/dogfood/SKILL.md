---
name: dogfood
description: Run a maximum-rigor dogfooding audit on theo-packs — tests BuildPlan generation, Dockerfile generation, docker build, and container start for all example projects
argument-hint: "[all|node|go|python|monorepo|<example-name>]"
---

# Dogfooding Audit for theo-packs

**Scope:** $ARGUMENTS

Execute a MAXIMUM RIGOR dogfooding session. You are a developer who just found theo-packs and is trying to deploy their project. You have ZERO tolerance for broken builds, misleading output, or missing features.

---

## RULES (INVIOLABLE)

1. **You are NOT a developer of theo-packs. You are a USER.** You cannot fix code. You can only report what's broken.
2. **Do NOT use Edit, Write, or any tool that modifies files.** Read-only. If you find a bug, document it — don't fix it.
3. **Every claim must have EVIDENCE.** No "it should work" — run it and prove it.
4. **If a Dockerfile is generated, you MUST `docker build` it.** A Dockerfile that doesn't build is a P0 bug.
5. **If a container builds, you MUST verify it starts.** `docker run --rm -d` and check logs. A container that crashes on start is a P0 bug.
6. **Grade with extreme prejudice.** If Railway/Vercel would reject this build, it fails.

---

## EXECUTION PROTOCOL

### Phase 1: Enumerate Targets

Based on the scope argument, enumerate all example projects to test:
- `all` → every directory in `../../examples/` (relative to theo-packs)
- `node` → all `node-*` examples
- `go` → all `go-*` examples
- `python` → all `python-*` examples
- `monorepo` → all workspace/turborepo/workspaces examples + fullstack-mixed
- `<name>` → single specific example (e.g., `node-next`)

For each target, document: name, expected provider, expected framework, expected behavior.

### Phase 2: BuildPlan Generation (per target)

For each example, run the full pipeline via Go test from the theo-packs root:

```bash
GOWORK=off go test ./core/dockerfile/ -run "TestIntegration_AllExamples/<example-name>" -v -count=1
```

For fullstack-mixed subdirs:
```bash
GOWORK=off go test ./core/dockerfile/ -run "TestIntegration_FullstackMixed/<subdir>" -v -count=1
```

**Verify:**
- [ ] BuildPlan generated successfully (no error)
- [ ] Correct provider detected (go, node, python, shell, staticfile)
- [ ] Start command is set and makes sense for the framework
- [ ] Dockerfile golden file exists and matches

**If any fail:** Record as P0 finding immediately.

### Phase 3: Dockerfile Semantic Validation (per target)

Read the generated Dockerfile from `core/dockerfile/testdata/integration_<name>.dockerfile` and validate:

**Base Images:**
- [ ] Build stage uses language-specific image (golang, node, python) — NOT debian:bookworm-slim for Go/Node/Python
- [ ] Deploy stage uses appropriate runtime (slim for Go, node-slim for Node, python-slim for Python)

**Build Pipeline:**
- [ ] For SSR frameworks (Next.js, Nuxt, Remix, Astro): `npm run build` or equivalent EXISTS in build step
- [ ] For SPAs (Vite React/Vue/Svelte): `npm run build` EXISTS in build step
- [ ] For Go: `go build` targets correct path (root main.go OR cmd/*/main.go)
- [ ] For Python: correct install command (pip for requirements.txt, poetry for Poetry, pipenv for Pipfile, uv for UV workspace)
- [ ] For Express/simple Node: no build step needed if no build script — verify no unnecessary build

**Dependency Caching:**
- [ ] Manifest files (package.json, go.mod, requirements.txt, etc.) are COPY'd BEFORE source code
- [ ] `npm ci` / `go mod download` / `pip install` runs BEFORE `COPY . .`
- [ ] Lockfile is included in manifest copy if it exists

**Workspace Support (monorepo examples only):**
- [ ] Workspace config detected (pnpm-workspace.yaml, turbo.json, go.work, etc.)
- [ ] Member package.json files are COPY'd individually
- [ ] Correct package manager used (pnpm for pnpm workspace, yarn for yarn workspace)
- [ ] Correct install command (npm ci, pnpm install --frozen-lockfile, yarn install --frozen-lockfile)

**Start Command:**
- [ ] CMD instruction present (if start command provided via env or package.json)
- [ ] Start command matches what the framework expects
- [ ] Package manager prefix matches (npm start for npm projects, pnpm start for pnpm, etc.)

**Security:**
- [ ] No secrets baked into image layers (check for ENV with sensitive values)
- [ ] Secrets use `--mount=type=secret` not ENV
- [ ] No root user running the app (check for USER instruction — note: currently not implemented, flag as P3)

### Phase 4: Docker Build (per target)

For each example, attempt a real `docker build`:

```bash
cd ../../examples/<name> && docker build -f /absolute/path/to/theo-packs/core/dockerfile/testdata/integration_<name>.dockerfile -t theopacks-dogfood-<name> --no-cache . 2>&1
```

**Grade:**
- BUILD SUCCESS → PASS
- BUILD FAIL → P0 (blocker: user can't deploy)

Record: exit code, build duration, error message if failed.

After build, inspect image:
```bash
docker inspect theopacks-dogfood-<name> --format '{{.Config.Cmd}}'
```

### Phase 5: Container Start Verification (per target that built)

For each successfully built image:

```bash
docker run --rm -d --name dogfood-<name> theopacks-dogfood-<name>
sleep 3
docker logs dogfood-<name> 2>&1 | head -20
docker stop dogfood-<name> 2>/dev/null
```

**Grade:**
- Container starts and shows framework output → PASS
- Container crashes immediately (exit code != 0) → P1 (builds but can't run)
- Container starts but with errors in logs → P2 (functional but degraded)
- N/A if start command requires env vars or external deps (database, etc.) — note as P3

### Phase 6: Cleanup

```bash
docker stop $(docker ps -q --filter "name=dogfood-") 2>/dev/null
docker rm $(docker ps -aq --filter "name=dogfood-") 2>/dev/null
docker rmi $(docker images -q --filter "reference=theopacks-dogfood-*") 2>/dev/null
```

---

## SEVERITY LEVELS

| Level | Name | Definition | Example |
|-------|------|-----------|---------|
| **P0** | Blocker | User cannot deploy. Build fails or no Dockerfile generated. | `go build .` fails, missing `npm run build`, wrong base image |
| **P1** | Critical | Builds but doesn't run. Container crashes on start. | Wrong start command, missing runtime deps in deploy image |
| **P2** | Major | Runs but incorrectly. Wrong behavior, security issue, huge image. | No layer caching, wrong package manager, dev deps in prod image |
| **P3** | Minor | Works but suboptimal. Missing optimization, verbose Dockerfile. | No .dockerignore, redundant COPY, no USER instruction, unnecessary layers |
| **P4** | Cosmetic | Nitpick. Style, naming, documentation. | Inconsistent stage names |

---

## REPORT FORMAT

After all phases complete, produce this EXACT report format:

```markdown
# theo-packs Dogfood Report

**Scope:** <scope>
**Date:** <date>
**Examples tested:** <count>
**Docker available:** yes/no

## Executive Summary

<1-3 sentences: overall verdict — be brutally honest>

## Results Matrix

| # | Example | Provider | BuildPlan | Dockerfile | Docker Build | Container | Grade |
|---|---------|----------|-----------|------------|--------------|-----------|-------|
| 1 | node-next | node | PASS/FAIL | PASS/FAIL | PASS/FAIL | PASS/FAIL/N/A | PASS/FAIL |

## Findings

### P0 — Blockers
<numbered list with exact command, exact output, expected vs actual>

### P1 — Critical
<numbered list with evidence>

### P2 — Major
<numbered list with evidence>

### P3 — Minor
<numbered list>

### P4 — Cosmetic
<numbered list>

## Pass Rate

- BuildPlan: X/Y (Z%)
- Dockerfile valid: X/Y (Z%)
- Docker build: X/Y (Z%)
- Container start: X/Y (Z%)
- **Overall: X/Y (Z%)**

## Recommendations

<prioritized action items, ordered by impact>

## Verdict

**PASS** / **CONDITIONAL PASS** / **FAIL**

Conditions (if conditional): <what must be fixed before shipping>
```

---

## CRITICAL REMINDERS

- You MUST actually run `docker build`. Reading the Dockerfile and saying "looks correct" is NOT dogfooding.
- If docker is not available, the audit CANNOT pass. Report it as infrastructure blocker.
- Every finding needs: exact command run, exact output, expected vs actual behavior.
- Do NOT skip examples that seem similar. Build them ALL. Each framework has quirks.
- After ALL testing, clean up ALL docker images and containers you created.
- If the scope is `all`, every single example must be tested. No exceptions. No shortcuts.
- Run tests from the theo-packs directory (the working directory).
- Use `GOWORK=off` for all go commands since theo-packs is not in the workspace go.work.
- Parallelize docker builds where possible using background agents for independent builds.
- Time the entire audit and report total duration.
