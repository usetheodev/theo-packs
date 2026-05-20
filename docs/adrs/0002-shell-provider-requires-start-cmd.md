# ADR-0002: Shell provider requires explicit `THEOPACKS_START_CMD`

**Status:** Accepted
**Date:** 2026-05-20
**Tracks:** T1.6 in `docs/plans/test-suite-hardening-plan.md`
**Decided by:** theo-packs maintainers

## Context

The `shell` provider in `core/providers/shell/shell.go` is the fallback
detector — it matches when the source tree contains *any* `*.sh` file but
no manifest from a stronger provider (`package.json`, `go.mod`, etc.).
Its `Plan()` copies the source tree into the deploy image but
deliberately does NOT set `Deploy.StartCmd`. The contract is documented
in the provider's `StartCommandHelp()`:

> "Specify a start command with THEOPACKS_START_CMD environment variable."

The E2E test `TestE2E_All/shell-script` therefore injects
`THEOPACKS_START_CMD=bash start.sh`. Earlier review questioned whether
this was a test hack masking a provider bug.

## Decision

The injection is **not** a hack. The shell provider is genuinely
ambiguous about entry point: a project with `start.sh`, `build.sh`, and
`deploy.sh` has no portable signal that says which one is the runtime
entry. Forcing the user to declare the start command via env var (or
config) is the safe default. Auto-picking `start.sh` would silently
break projects whose entry is named differently and would create a
brittle convention.

## Rationale

1. **No portable heuristic.** Unlike Node (`package.json#scripts.start`),
   Go (`main.go`), or Python (`gunicorn app:app`), shell projects have
   no manifest that names the entry. Picking by filename convention
   embeds an opinion users can't easily override.
2. **The provider is a fallback.** Anyone reaching the shell provider
   has already declined to use a stronger provider — they're declaring
   "I'll wire this up myself". Forcing the explicit env var is
   consistent with that posture.
3. **The error path is helpful.** Without `THEOPACKS_START_CMD`, the
   plan validator surfaces `StartCommandHelp()` verbatim — the user
   sees exactly what to do.

Alternatives rejected:

- Auto-pick `start.sh` (or first lexically-sorted `.sh`): silently
  wrong for projects with non-conventional entries.
- Reject the provider entirely (force users to configure
  `theopacks.json`): regresses the zero-config promise for the simple
  case where a single `start.sh` exists.

## Consequences

- E2E tests for the shell provider MUST set `THEOPACKS_START_CMD`. This
  ADR is the canonical reference for why.
- The verifier (`verifyShellExecutes`) now actually runs the script
  inside the image and checks output — stronger guarantee than the
  previous "the file was copied" check.
- If/when we add convention-detect logic (e.g., honor `[start]` in a
  `Procfile`), revisit this ADR.

## References

- Provider source: `core/providers/shell/shell.go`
- Test: `e2e/e2e_table_test.go::verifyShellExecutes`
- Plan: `docs/plans/test-suite-hardening-plan.md` T1.6
