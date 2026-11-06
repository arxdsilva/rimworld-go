---
name: rimworld-go-ai-jobs
description: Implement or review rimworld-go pawn intent selection, work discovery, job state machines, reservations, hauling, and large-colony AI scheduling.
---

# Pawn AI, Work, Jobs & Reservations

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

## Frequency

Expensive intent/job searches should be bucketed/invalidation-driven where possible.

Critical survival/emergency checks may run more often.

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
