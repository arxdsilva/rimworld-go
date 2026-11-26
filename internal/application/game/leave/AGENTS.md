# internal/application/game/leave

## Purpose
Shared edge departure selection for fleeing visitors and raiders (DEF-4, MAP-21). The caller owns actor state, jobs, movement, faction and departure result.

## Owns
Try attempts walkable exits in stable nearest-side order and then structural breach jobs. BreachRoute is a read-only reachability query. Host is the consumer-owned map/job seam.

## Contracts
A failed unreachable Move is skipped during the next attempt; another side or a legal breach may recover it. Do not despawn an actor here or create a second scheduler. The original failed goal is retried only through the caller's bounded cadence.

## Tests
TestRetrySkipsUnreachableEdgeAndChoosesAnother and TestEdgeRockFallsBackToOtherSides cover route selection; root raid/visit tests exercise departure.
