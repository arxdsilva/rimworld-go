---
name: rimworld-go-router
description: Route any rimworld-go engineering task to the smallest relevant set of project skills and canonical documents. Use first for ambiguous, cross-cutting, or newly assigned tasks.
---

# rimworld-go Task Router

Use this skill before implementation when the correct subsystem or context set is not already obvious.

## Goal

Select the smallest sufficient context and skill set for the task.

Never load every project document or every skill by default.

## Canonical project documents

Read in this order only as needed:

1. `/AGENTS.md`
2. nearest package `AGENTS.md`
3. relevant source interfaces/types
4. relevant tests
5. relevant `docs/systems/*.md`
6. relevant `docs/adr/*.md`
7. `.agent/HEXAGONAL_ARCHITECTURE.md`
8. `.agent/GO_BEST_PRACTICES.md`
9. `.agent/MASTER_SPEC.md` only when broader product intent is required

Token/context policy is defined by `.agent/AI_TOKEN_OPTIMIZATION.md`.

## Routing table

| Task | Add skill |
|---|---|
| New feature or domain behavior | `rimworld-go-feature` |
| Package/dependency/interface design | `rimworld-go-hexagonal` |
| Tick order, RNG, authoritative state, replay determinism | `rimworld-go-determinism` |
| Hot loops, scale, allocations, data layout | `rimworld-go-performance` |
| Paths, regions, movement, congestion | `rimworld-go-pathing` |
| Intents, work discovery, jobs, reservations | `rimworld-go-ai-jobs` |
| Save/load, schema, snapshots, replay | `rimworld-go-persistence-replay` |
| Definitions, YAML/JSON content, mods | `rimworld-go-content-modding` |
| Tests, fuzz, benchmarks, long simulations | `rimworld-go-testing` |
| Regression/bug/replay investigation | `rimworld-go-debugging` |
| PR review, merge wave, architecture compliance | `rimworld-go-review-integration` |
| ADR/system/package docs | `rimworld-go-documentation` |

A normal task should use **1–3 skills**, not the full catalog.

## Classification questions

Determine:

- What behavior changes?
- Which package owns that behavior?
- Is authoritative state affected?
- Can order/RNG/save state change?
- Is this a hot path?
- Does a public interface change?
- Does persistence/mod compatibility change?
- Does the task span more than one architectural layer?

## Output before coding

Internally establish a task packet:

```text
Task ID:
Objective:
Owner package:
Allowed packages:
Forbidden packages:
Skills:
Canonical docs:
Tests:
Benchmarks:
Determinism impact:
Save/mod impact:
```

Keep it concise.

## Stop conditions

Do not implement yet if:

- two packages both appear to own the same concept;
- a task requires violating dependency direction;
- a canonical primitive appears to need duplication;
- an existing ADR explicitly conflicts with the requested implementation.

In those cases, invoke the architecture/documentation workflow and resolve the contract first.
