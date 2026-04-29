# BREAKING: theo-packs is single source of truth for Dockerfile generation

> **Coordinated with theo-stacks PR-B** (deletes 26 Dockerfiles from templates). Both PRs MUST merge on the same day per ADR D2 in `docs/plans/single-source-of-truth-plan.md`.

## What changes

Pre-release window with no external users. theo-packs is now the **only** producer of Dockerfiles consumed by the Theo build pipeline. A user-supplied Dockerfile at `<source>/<app-path>/Dockerfile` causes the CLI to **exit with code 2** and an actionable error message:

```
[theopacks] ERROR: user-supplied Dockerfile found at /workspace/apps/api/Dockerfile.

theo-packs is the single source of truth for Dockerfile generation.
Remove the file and rerun. To opt out of generation entirely, do not
invoke theo-packs — declare your build via a different mechanism in
your deployment pipeline.
```

There is no override flag. No warn-and-ignore. No env var. **One path.**

## Edge case (deliberate)

A `Dockerfile` at the **workspace root** (`<source>/Dockerfile`, OUTSIDE the analyzed app path) is NOT checked. It may legitimately exist for local development outside Theo (e.g., `docker compose up` reading a top-level Dockerfile unrelated to the app being deployed). The rejection scope is the analyzed app path only.

Verified by `TestUserProvidedDockerfileAtWorkspaceRoot_IsNotRejected`.

## Why now

Pre-release; no migration cost. The previous precedence path was the source of every "theo-packs is broken" misdiagnosis — the recent dogfood against `monorepo-turbo` (F2/F5 in the dogfood report) found a buggy template Dockerfile and blamed theo-packs for emitting it. Removing the path eliminates the entire class of confusion.

FAANG-level releases have one path through the contract. Two paths means double the test surface, double the docs, double the failure modes, double the misdiagnosis.

## Commits

- `docs: add single-source-of-truth plan + inventory` (Phase 0)
- `feat(cli)!: hard-fail on user Dockerfile (single source of truth)` (Phase A)

The single feature commit covers TA1.1 (CLI hard-fail), TA2.1 (contract rewrite), TA3.1 (test inversion), TA3.2 (CHANGELOG). Per ADR D7 of the v2 plan, larger feats split internally; this one is a single semantic transition and reads cleanly together.

## Quality gates

```
mise run test         ✓ all packages green (existing + 2 new rejection tests)
go vet -tags e2e      ✓ no warnings
golangci-lint         ✓ 0 issues
gofmt -l .            ✓ 0 files
```

## Test coverage

`TestUserProvidedDockerfileIsRejected` asserts:
1. CLI exits with `*exec.ExitError` having `ExitCode() == 2`.
2. stderr contains `user-supplied Dockerfile found at <expected-path>`.
3. stderr contains the phrase `single source of truth`.
4. **No Dockerfile is written to `--output`** (failure must be silent on the output path).

`TestUserProvidedDockerfileAtWorkspaceRoot_IsNotRejected` asserts:
1. Synthetic monorepo with `Dockerfile` at the workspace root + `apps/api/package.json` succeeds.
2. The CLI generates a Dockerfile at `--output` despite the unrelated workspace-root file.

## Coordination with theo-stacks PR-B

PR-A alone (this PR) means theo-packs rejects every theo-stacks template that still ships a Dockerfile → every deploy fails until templates are cleaned. PR-B alone means theo-stacks templates have no Dockerfile but theo-packs's old code path tries to copy a non-existent file → cosmetic but the contract is unenforced.

The two changes are a single semantic transition. Recommended sequence:

1. Approve and merge **this PR (PR-A)** to theo-packs `develop`.
2. Immediately approve and merge **PR-B** to theo-stacks `develop`.
3. theo product picks up both via vendoring or fresh clone.
4. Run the existing `TestE2E_MonorepoTurboFromStacks` against the post-merge state to verify the contract holds end-to-end.

## Backward compatibility

**Explicitly NOT preserved.** Pre-release, no external users, user has waived backward compatibility ("fix in FAANG level"). The next theo-packs release is a major version bump (`v2.0.0`) per ADR D6 of the plan.

## See also

- `docs/plans/single-source-of-truth-plan.md` — the plan this PR implements (ADRs D1-D6, full rationale).
- `docs/plans/single-source-of-truth-inventory.md` — frozen file list.
- `docs/contracts/theo-packs-cli-contract.md` — the canonical contract, now reflecting single-source-of-truth.
