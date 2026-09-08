---
name: rimworld-go-pathing
description: Implement or review rimworld-go pathfinding, regions, movement, congestion, navigation invalidation, and asynchronous path workers.
---

# Pathing & Movement

## Architecture

Separate:

```text
domain/ai/pathing          pure deterministic algorithms
adapters/outbound/pathworker  asynchronous worker execution
```

The path algorithm is gameplay logic; worker scheduling is infrastructure.

## Path hierarchy

Preferred progression:

```text
origin tile
-> origin region
-> region graph route
-> local detailed path segments
```

Avoid global tile A* for every long route at scale.

## Search rules

Before exact paths:

- filter by reachable region;
- use cheap distance;
- limit candidate set;
- exact-path only finalists.

## Navigation revisions

Path requests should carry a navigation revision.

On result commit:

- verify the result still applies;
- discard/recompute stale results deterministically.

## Dynamic entities

Do not rebuild world navigation for moving pawns.

Use local deterministic arbitration/avoidance for transient congestion.

## Tie-breaking

A* and movement conflict resolution need stable tie-break rules.

Do not depend on heap insertion accidents or map iteration.

## Cache

Potential cache keys:

```text
origin region
target region
movement profile
navigation revision
```

Invalidate on topology revision change.

## Tests

Cover:

- shortest valid route;
- impassable geometry;
- doors/special costs;
- stable tie-break;
- stale worker result;
- deterministic repeat;
- region disconnection;
- large-map benchmark;
- movement conflict resolution.

## Performance

Track:

```text
path requests/tick
nodes expanded
cache hit rate
regional candidates
path latency
allocs/path
repath frequency
```
