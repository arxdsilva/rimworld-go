---
name: rimworld-go-debugging
description: Investigate rimworld-go simulation regressions using deterministic seeds, state hashes, replay fixtures, entity/tick-scoped diagnostics, and minimal reproductions.
---

# Debugging & Regression Reproduction

## Goal

Turn vague simulation bugs into a minimal deterministic artifact.

## Gather

Prefer this compact bug identity:

```text
seed
scenario
tick or tick range
entity IDs
command/replay ID
expected state hash
actual state hash
system suspected
```

Avoid megabytes of logs.

## Workflow

1. reproduce under fixed seed;
2. identify first divergent tick if determinism-related;
3. identify first divergent subsystem/entity;
4. reduce commands/world state where practical;
5. create replay or simtest regression;
6. inspect owning package and relevant invariant;
7. fix root cause;
8. verify repeated identical result;
9. run nearby tests/benchmarks.

## State narrowing

Use state hashing hierarchically when possible:

```text
whole simulation
-> system
-> entity/chunk
-> component
```

## Logs

Filter by:

- entity;
- tick;
- system;
- job/path ID.

Logging must remain observational.

## Common classes

Check early for:

- stale revision committed;
- reservation not released;
- deleted entity reference;
- implicit map ordering;
- RNG stream coupling;
- component swap-remove order affecting logic;
- command ordering;
- cached state not invalidated;
- save field omitted;
- worker completion order leak.

## Deliverable

The fix should leave:

- regression test/replay;
- concise root-cause note;
- any invariant/document update required.
