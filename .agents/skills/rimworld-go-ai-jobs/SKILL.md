---
name: rimworld-go-ai-jobs
description: Use when implementing or reviewing rimworld-go pawn intent, work discovery, job state machines, reservations, hauling, animal AI or large-colony AI scheduling.
---

# Pawn AI, Work, Jobs & Reservations

## Load

- `MASTER_SPEC` §23 Jobs and Work Architecture, §24 Job Structure and State Machines, §25 Work Discovery at Scale, §26 Reservation System, §34 Animals
- `MASTER_SPEC` §55 Concurrency Model and §78 Architectural Anti-Patterns (no goroutine per pawn, no global scans)

## Intent hierarchy

Use explicit priority layers:

```text
forced
danger/medical
critical survival
schedule
work
recreation/social
idle
```

Do not bury priority in arbitrary callback order.

## Work discovery

Never implement:

```text
for every pawn:
    scan every possible task
```

Use indexed candidates by:

- work type;
- region/chunk;
- reservation availability;
- storage category;
- urgency.

Then:

```text
filter
-> cheap score
-> small finalist set
-> exact path
-> reserve
```

## Jobs

Jobs are explicit state machines.

Each state must define:

- entry condition;
- action;
- completion;
- failure;
- reservation lifecycle.

Gameplay failure reasons should be typed domain outcomes, not generic Go errors.

## Reservations

Support explicit reservation keys such as:

- entity;
- tile;
- item quantity;
- bed;
- patient;
- workstation.

Reservations must release on:

- successful completion;
- cancellation;
- pawn death/downing where appropriate;
- target destruction;
- replacement job;
- invalidation.

## Hauling

Item quantity moves must be atomic with respect to inventory/location invariants.

An item stack exists in exactly one location model at a time.

## Animals

- Animals make full decisions every 60 ticks through the bucket index (`internal/application/game/animal_index.go`, `animal_needs.go`); never scan all creatures per tick.
- Biome data lives in `internal/domain/world/biome`; nature-event definitions in `internal/domain/incident`.
- New creatures also follow skill `rimworld-go-creatures`.

## Frequency

Expensive intent/job searches should be bucketed/invalidation-driven where possible.

Critical survival/emergency checks may run more often.

Every need-, threat- or threshold-driven behaviour uses different start and stop thresholds, or a minimum duration or retry cooldown, and ships a no-flap test (root `AGENTS.md`; model: `internal/application/game/sleep_test.go`).

## Tests

Cover:

- priority ordering;
- conflicting reservations;
- cancellation cleanup;
- target deletion cleanup;
- no duplicate item quantities;
- no two exclusive workers;
- unreachable candidate fallback;
- deterministic candidate tie-break;
- large-colony job search benchmark.
