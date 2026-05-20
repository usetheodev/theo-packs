Dogfooding audit — test theo-packs as a real user building real projects. Argument: $ARGUMENTS (scope: all, node, go, python, monorepo, or a specific example name like node-next)

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
- `all` → every directory in `../examples/`
- `node` → all `node-*` examples
- `go` → all `go-*` examples
- `python` → all `python-*` examples
- `monorepo` → all workspace/turborepo/workspaces examples
- `<name>` → single specific example

For each target, document: name, expected provider, expected framework, expected behavior.

### Phase 2: BuildPlan Generation (per target)

For each example, run the full pipeline programmatically via Go test:

```bash
GOWORK=off go test ./core/dockerfile/ -run "TestIntegration_AllExamples/<example-name>" -v -count=1
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
- [ ] CMD instruction present
- [ ] Start command matches what the framework expects (next start, node server.js, gunicorn, etc.)
- [ ] Package manager prefix matches (npm start for npm projects, pnpm start for pnpm, etc.)

### Phase 4: Docker Build (per target)

For each example, attempt a real `docker build`:

```bash
cd ../examples/<name>
docker build -f <path-to-golden-dockerfile> -t theopacks-dogfood-<name> --no-cache . 2>&1
```

**Grade:**
- BUILD SUCCESS → PASS
- BUILD FAIL → P0 (blocker: user can't deploy)

Record: exit code, build duration, error message if failed.

After build, inspect image:
```bash
docker inspect theopacks-dogfood-<name> --format '{{.Config.Cmd}} | Size: {{.Size}}'
```

### Phase 5: Container Start Verification (per target that built)

For each successfully built image:

```bash
docker run --rm -d --name dogfood-test-<name> -p 0:3000 theopacks-dogfood-<name>
sleep 3
docker logs dogfood-test-<name> 2>&1 | head -20
docker stop dogfood-test-<name> 2>/dev/null
```

**Grade:**
- Container starts and shows framework output → PASS
- Container crashes immediately → P1 (builds but can't run)
- Container starts but with errors → P2 (functional but degraded)

### Phase 6: Cleanup

```bash
docker rmi $(docker images -q 'theopacks-dogfood-*') 2>/dev/null
```

---

## SEVERITY LEVELS

| Level | Name | Definition | Example |
|-------|------|-----------|---------|
| **P0** | Blocker | User cannot deploy. Build fails or no Dockerfile generated. | `go build .` fails, missing `npm run build`, wrong base image |
| **P1** | Critical | Builds but doesn't run. Container crashes on start. | Wrong start command, missing runtime deps |
| **P2** | Major | Runs but incorrectly. Wrong behavior, security issue, huge image. | No layer caching, wrong package manager, dev deps in prod |
| **P3** | Minor | Works but suboptimal. Missing optimization, verbose Dockerfile. | No .dockerignore, redundant COPY, unnecessary layers |
| **P4** | Cosmetic | Nitpick. Style, naming, documentation. | Inconsistent stage names, verbose comments |

---

## REPORT FORMAT

After all phases complete, produce this report:

```markdown
# theo-packs Dogfood Report

**Scope:** <scope>
**Date:** <date>
**Examples tested:** <count>

## Executive Summary

<1-3 sentences: overall verdict>

## Results Matrix

| Example | Provider | BuildPlan | Dockerfile Valid | Docker Build | Container Start | Grade |
|---------|----------|-----------|-----------------|--------------|----------------|-------|
| node-next | node | ✅ | ✅/❌ + reason | ✅/❌ | ✅/❌ | PASS/FAIL |

## Findings

### P0 — Blockers
<numbered list with evidence>

### P1 — Critical
<numbered list with evidence>

### P2 — Major
<numbered list with evidence>

### P3 — Minor
<numbered list>

## Recommendations

<prioritized action items>

## Verdict

**PASS** / **FAIL** — with conditions
```

---

## CRITICAL REMINDERS

- You MUST actually run docker build. Reading the Dockerfile and saying "looks correct" is NOT dogfooding.
- If docker is not available, the audit CANNOT pass. Report it as a blocker.
- Every finding needs the exact command run, the exact output, and the exact expected vs actual behavior.
- Do NOT skip examples that seem similar. Build them ALL. Each framework has its own quirks.
- After ALL testing, clean up ALL docker images you created.
- If the scope is `all`, every single example must be tested. No exceptions.
