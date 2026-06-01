---
paths:
  - "internal/**/*.go"
---

# Architecture Rules — theo-packs

> Project-specific architecture overlay. The 4+1 cycle rules + universal principles live in `cycles-engine/core/rules/`. This file declares what is unique to theo-packs.

## DIP layers

| Layer | Path | May import |
|---|---|---|
| **domain** | `internal/domain/**` | (nothing) |
| **application** | `internal/application/**` | domain |
| **adapters** | `internal/adapters/**` | domain, application |
| **infra** | `internal/infra/**` | domain, application, adapters |

Mirror this in `.claude/project.yaml.architecture.layers` (cycles-engine reads from there).

## Composition root

TODO — name the entry point that wires everything together (e.g., `cmd/theo-packs/main..go`, `src/main..go`, `src/server.ts`).

## Naming conventions

TODO — per-project naming (file casing, type/function/constant patterns).

## Module hygiene

TODO — package boundary rules, public vs internal exports.

## Public surface

If your project exposes a public API or CLI surface, declare its directory here and the rules engine will enforce that internal modules cannot leak into the public types.

TODO — fill in or remove.
