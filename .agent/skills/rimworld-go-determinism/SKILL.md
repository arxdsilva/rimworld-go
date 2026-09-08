---
name: rimworld-go-determinism
description: Implement or review deterministic authoritative simulation in rimworld-go, including tick ordering, RNG, worker commits, state hashing, and replay stability.
---

# Deterministic Simulation

## Authoritative rules

Authoritative gameplay must not depend on:

- wall clock;
- machine speed;
- goroutine completion order;
- map iteration order;
- filesystem order;
- global random state;
- renderer frame rate;
- CPU count.

## Time

Use integer `Tick`.

Fast-forward processes more ticks; it does not change rules.

## RNG

Never use global `math/rand` APIs in authoritative code.

Random identity should derive from stable inputs conceptually equivalent to:

```text
world seed
RNG stream
tick
entity ID
operation kind
local sequence
```

Adding an unrelated random roll should not perturb every future outcome.

## Order

If multiple actors/results can conflict, define a deterministic tie-break.

Typical keys:

```text
priority
sequence
EntityID
JobID
```

Do not rely on storage iteration order unless explicitly guaranteed.

## Async worker pattern

Workers may compute from immutable inputs:

```text
snapshot
-> request(sequence, revision)
-> parallel compute
-> collect
-> deterministic order
-> validate revision
-> authoritative commit
```

Workers never mutate live simulation stores.

## Simulation phases

Changes to phase order are architecture-level changes.

Keep phase transitions explicit rather than relying on registration order.

## State hashes

Stable hash traversal must use:

- stable entity/component order;
- fixed numeric encoding;
- sorted map keys if maps are unavoidable;
- no pointers;
- no wall-clock fields;
- no presentation-only state.

## Testing

Any determinism-sensitive change should include one or more:

- repeat-run equality;
- save/load hash equality;
- replay hash equality;
- worker-count equality;
- golden scenario state hash.

Run the same scenario under varying `GOMAXPROCS` where concurrency can affect computation.

## Review red flags

- `time.Now()` in domain;
- global RNG;
- random draw count coupled to iteration count;
- decision based on Go map range;
- channel receive order determining outcomes;
- worker directly mutating components;
- renderer-fed float coordinates becoming authoritative;
- hash of raw structs with padding/pointers.
