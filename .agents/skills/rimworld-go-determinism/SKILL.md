---
name: rimworld-go-determinism
description: Implement or review deterministic simulation, RNG, tick order, worker commits, state hashing, saves, schema migrations and replay in rimworld-go.
---

# Deterministic simulation, saves and replay

## Load

- `.agents/specs/MASTER_SPEC.md` §6 Authoritative Simulation Clock, §7 Determinism Contract (the rules and the RNG identity), §8 Simulation Phases, §55 Concurrency Model.
- `.agents/specs/GO_BEST_PRACTICES.md` §14, §27, §28 for the Go how-to.

## Procedure

1. Integer `Tick` only. Fast-forward runs more ticks; it never changes rules.
2. Authoritative code never calls global `math/rand`. Derive randomness from stable inputs (world seed, stream, tick, entity ID, operation kind, local sequence), so an unrelated roll does not shift later outcomes. Never couple the number of draws to an iteration count.
3. Where actors or results can conflict, define a tie-break (priority, sequence, `EntityID`, `JobID`). Never rely on storage iteration order unless it is guaranteed.
4. Workers compute from immutable inputs and never mutate live stores:

```text
snapshot -> request(sequence, revision) -> parallel compute -> collect
-> deterministic order -> validate revision -> authoritative commit
```

5. A change to phase order is an architecture-level change. Keep transitions explicit, not by registration order.
6. State hashes traverse in stable entity/component order, with fixed numeric encoding, sorted map keys if maps are unavoidable, no pointers, no wall-clock fields, no presentation-only state.
7. Tests: any determinism-sensitive change adds one or more of repeat-run equality, save/load hash equality, replay hash equality, worker-count equality, a golden scenario hash. Run the scenario under varying `GOMAXPROCS` where concurrency can affect computation.

## Review red flags

`time.Now()` in domain; global RNG; draw count coupled to iteration count; decisions from Go map range; channel receive order deciding outcomes; a worker mutating components; renderer float coordinates becoming authoritative; visual RNG feeding the simulation; hashing raw structs with padding or pointers.

## Saves and replay

Load MASTER §44 Commands and Replay-Ready Input, §45 Save Architecture, §46 Snapshot and Autosave, §47 Replay and State Hashing, §83 Save and Replay Regression Corpus; HEX §19 Persistence Boundary.

- **Boundary.** Domain runtime structures are not public persistence schemas. Translate through an adapter; generated protobuf types never leak into domain APIs.
- **A save records** enough canonical state to continue exactly: schema version, game/content version, seed, tick, authoritative world state, content/mod identity, and the deterministic counters and streams. Presentation caches are reconstructed.
- **Autosave flow:** capture a consistent snapshot -> resume the simulation -> encode/compress asynchronously -> atomic temp write -> fsync where policy requires -> rename. Encoding workers never mutate the simulation.
- **Versioning and migrations.** Migrations are explicit per schema version and tested with historical fixtures. Never "best effort" a corrupt authoritative state.
- **Replay input:** initial seed or scenario, the ordered `CommandEnvelope` stream, optional checkpoints, expected state hashes. A replay fixture is the preferred regression artifact for a hard simulation bug.
- **Verification:** save -> load -> identical canonical hash; replay -> expected hash; repeated replay equality; migration fixtures; corrupt and truncated input; count and decompression limits.
- **Untrusted saves.** Bound collection sizes, nesting, compressed expansion, string lengths, referenced IDs and schema versions.
