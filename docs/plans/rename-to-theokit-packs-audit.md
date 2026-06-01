# Audit Baseline — rename-to-theokit-packs

Generated: 2026-06-01
Source plan: `docs/plans/rename-to-theokit-packs-plan.md` (T0.1)

This document is the **ground-truth measurement** of every string this rename touches, **before** any change. It exists so the Definition of Done in the plan ("zero matches") can be verified mechanically against the same probe.

## Baseline counts (HEAD = d620abc on develop, pre-rename)

| Category | Probe | Count (HEAD) | Expected after rename |
|---|---|---|---|
| Go imports of module path | `git grep -l "github.com/usetheo/theopacks" -- '*.go'` | **71 files** | **0** |
| `THEOPACKS_` literals in Go | `git grep -l "THEOPACKS_" -- '*.go'` | **42 files** | **0** |
| `THEOPACKS_` literals in docs/configs | `git grep -l "THEOPACKS_" -- '*.md' '*.yml' '*.yaml' '*.toml'` | **15 files** | **0** (except `docs/plans/PR_DESCRIPTION_*` historical) |
| `theopacks` lowercase refs (all) | `git grep -l "theopacks" -- '*.go' '*.md' '*.yml' '*.yaml' '*.toml' '*.dockerfile'` | **152 files** | **0** (except `NOTICE` § Railpack and historical plans) |
| `theo-packs` (hyphen) refs | `git grep -l "theo-packs" -- '*.go' '*.md' '*.yml' '*.yaml' '*.toml' '*.dockerfile' '*.json'` | **100 files** | **0** (except `NOTICE` § Railpack and historical plans) |
| Goldens with `theo-packs`/`theopacks` | `grep -l "theo-packs\|theopacks" core/dockerfile/testdata/*.dockerfile` | **58 goldens** | **0** (regenerated via `UPDATE_GOLDEN`) |
| Example config file | `examples/node-npm-with-config/theopacks.json` | **exists** | **renamed to `theokit-packs.json`** |

## Notes

- The 58-golden count exceeded the 10-golden estimate in the plan: the defensive header (`# theo-packs: generated for provider "X"`) is emitted on essentially every golden, not only on integration goldens. T5.5 regenerates all of them via `UPDATE_GOLDEN=true`.
- 152 total `theopacks` refs ≈ 71 (module imports) + ~30 (Go env-var literals reusing the prefix) + ~15 (docs) + ~36 (NOTICE / CHANGELOG / golden defensive headers). Sum matches with overlap.
- The historical `docs/plans/PR_DESCRIPTION_*.md` and `single-source-of-truth-*.md` files are **excluded from "expected 0"** per ADR D5 of the plan — they are audit trail, not active documentation.
- `NOTICE` § "This product is derived from Railpack" is **preserved verbatim** per ADR D4 (Apache 2.0 § 4(c)(i)).

## Final-state verification commands

After Phase 5 completes, run these to assert "Done":

```bash
# Module imports — must be 0
git grep -l "github.com/usetheo/theopacks" -- '*.go' | wc -l

# Env vars — must be 0
git grep -l "THEOPACKS_" -- '*.go' '*.md' '*.yml' '*.yaml' '*.toml' | wc -l

# Lowercase refs — must be 0 (excluding NOTICE § Railpack and docs/plans/PR_DESCRIPTION_*)
git grep -l "theopacks" -- '*.go' '*.md' '*.yml' '*.yaml' '*.toml' '*.dockerfile' \
  | grep -v "^docs/plans/PR_DESCRIPTION_" \
  | grep -v "^NOTICE$" \
  | wc -l

# Hyphen refs — must be 0 (same exclusions)
git grep -l "theo-packs" -- '*.go' '*.md' '*.yml' '*.yaml' '*.toml' '*.dockerfile' '*.json' \
  | grep -v "^docs/plans/PR_DESCRIPTION_" \
  | grep -v "^NOTICE$" \
  | wc -l

# Example config file — must exist with new name
test -f examples/node-npm-with-config/theokit-packs.json && echo OK

# Build + tests — must all pass
go build ./... && mise run check && mise run test
```
