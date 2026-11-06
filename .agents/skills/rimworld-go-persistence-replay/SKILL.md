---
name: rimworld-go-persistence-replay
description: Implement or review rimworld-go saves, versioned persistence schemas, snapshots, migrations, replay command streams, and deterministic state verification.
---

# Persistence, Save & Replay

## Boundary

Domain runtime structures are not public persistence schemas.

Translate through an adapter.

Generated protobuf types must not leak into domain APIs.

## Save requirements

A save records enough canonical state to continue exactly, including:

- schema version;
- game/content version metadata;
- seed;
- tick;
- authoritative world/domain state;
- content/mod identity;
- relevant deterministic counters/streams.

Presentation caches should normally be reconstructed.

## Snapshot flow

Preferred autosave:

```text
capture consistent simulation snapshot
-> resume simulation
-> encode/compress asynchronously
-> atomic temp write
-> fsync where policy requires
-> rename
```

Encoding workers never mutate simulation.

## Migrations

Migrations are explicit by schema version.

Test historical fixtures.

Never silently "best effort" corrupt authoritative state.

## Replay

Replay input should be conceptually:

```text
initial seed/scenario
ordered CommandEnvelope stream
optional checkpoints
expected state hashes
```

## Verification

Add tools/tests for:

- save -> load -> identical canonical hash;
- replay -> expected hash;
- repeated replay equality;
- migration fixtures;
- corrupt/truncated input;
- count/decompression limits.

## Security

Treat saves as untrusted.

Bound:

- collection sizes;
- nesting;
- compressed expansion;
- string lengths;
- referenced IDs;
- schema versions.

## Debug benefit

A deterministic replay fixture is the preferred regression artifact for hard simulation bugs.
