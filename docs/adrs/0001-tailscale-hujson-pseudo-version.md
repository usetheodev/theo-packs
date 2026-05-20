# ADR-0001: Accept `tailscale/hujson` as a pseudo-version dependency

**Status:** Accepted
**Date:** 2026-05-20
**Tracks:** T2.3 in `docs/plans/deep-review-hardening-plan.md` (H4 finding)
**Decided by:** theo-packs maintainers

## Context

`github.com/tailscale/hujson` parses [HuJSON / JWCC](https://github.com/tailscale/hujson) — JSON with comments and trailing commas — which is the on-disk format of `theopacks.json`. The library is referenced by `internal/utils/utils.go::StandardizeJSON`, which strips comments before standard `encoding/json` parsing.

The CLAUDE.md global guidance (Part II, §9 "Não Reinvente a Roda") prefers tagged releases with active maintenance. `tailscale/hujson` is currently consumed at:

```
github.com/tailscale/hujson v0.0.0-20260302212456-ecc657c15afd
```

This is a Go-module **pseudo-version**: there is no Semver tag in the upstream repository — every commit is referenced by date + commit hash. The upstream maintainers (Tailscale Inc.) follow this convention deliberately for this utility library.

## Decision

Accept the pseudo-version as the canonical pin. Do **not**:

- Vendor the library into this repo (would add ~600 LoC to the codebase surface and break upstream-merge workflows).
- Replace HuJSON parsing with a hand-rolled stripper (the auditable parser is the whole point of using a library — Rule §9).
- Fork to add Semver tags (operational burden + diverges from upstream Tailscale, undermining the reason to use it).

## Rationale

Three pieces of evidence make the pseudo-version acceptable here:

1. **Provenance.** Tailscale Inc. is a well-known, established commercial Go shop with public security practices. The repo is owned by `github.com/tailscale`, the same org that ships the core Tailscale product. Pseudo-version != unaudited code.
2. **Sum-locked.** `go.sum` records `h1:Rf9uhF1+VJ7ZHqxrG8pJ6YacmHvVCmByDmGbAWCc/gA=` — any future bump must re-verify the module hash, which catches commit-rewrite attacks at `go mod tidy` time.
3. **Narrow surface.** The library exposes one function we use (`hujson.Standardize`). The blast radius of a hypothetical compromise is "user can craft a theopacks.json that crashes the parser or produces unintended JSON" — which is bounded by the build sandbox, not arbitrary code execution.

## Consequences

- **Updates require human review.** When a Dependabot PR proposes a new pseudo-version, the reviewer must compare commit ranges in upstream `tailscale/hujson` rather than reading a release changelog. Maintainers will check the commit list at GitHub before merging.
- **CLAUDE.md global rule §9 is partially violated.** The "Último release < 6 meses" criterion does not apply cleanly to libraries that don't release. This ADR is the project-level justification recorded in code.
- **Re-evaluation criteria.** If any of the following becomes true, revisit this decision:
  1. The library accumulates open security advisories that go unanswered for > 90 days.
  2. A maintained alternative ships with Semver tags AND covers HuJSON spec (currently none known).
  3. The Tailscale repo is archived or transferred.

## References

- Upstream repository: <https://github.com/tailscale/hujson>
- Consumer: `internal/utils/utils.go::StandardizeJSON`
- Plan: `docs/plans/deep-review-hardening-plan.md` (T2.3)
- CLAUDE global rule referenced: Part II §9 "Não Reinvente a Roda"
