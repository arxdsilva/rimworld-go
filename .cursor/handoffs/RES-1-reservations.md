# RES-1: reservations, RimWorld-style

Checklist §5.2 (Entity, Tile, Item quantity, Workstation, Cleanup `[x]`; Bed/patient stays `[-]`: no per-patient policy). UI checklist 14.3 "Take ingredients from".

## What changed
- `domain/ai/reservation/reservation.go`: `TryCount`, `Free`/`FreeFor`, `ReleaseTarget`, `ReleaseTile`; `Claim.Count` is saved with a sorted export. `EntityOwner` still reports a count-claimed stack as taken (lowest-id claimant) for unconverted callers. Coverage 97 %.
- Partial claims: eating (`food.go` `bestFood`, `doEat`, dining), haul loads, pickups and routed deliveries (`planDelivery`), bill ingredients (craft gather, shop, haul), medicine (one dose on tend, doses used in surgery). A pickup releases the rest; `infection.go` `pickUpItem` frees the dose.
- `hasEdible` (`tick.go`) counts a stack while `Free > 0`, so a second colonist seeks food while the first eats.
- Workstation: a bill or research job claims the bench's work spot.
- Cleanup: `ReleaseTarget` hooks in `deleteItem`, `removeBuilding`, `removePlant`, `removeAnimal` and `dieBy`.
- Beds: `SleepIn` skips a bed another pawn holds (`rest_host` `BedHeld`).
- Bill source: `SetStorageSource` sets `Bill.Source` (zone or group id, saved); `BillView.Source`/`SourceName`; `zone.Map.SameStore` for groups.
- `game/crops` sub-package pays lines (net about -10 game lines).

## Numbers
No wiki numbers; reservations have none. Nothing original beyond the count-claim design.

## UI
Bill config "take from" menu (see STO-3). No other player-visible change.

## Tests
Two eaters share a meal stack (also through `seekFood`), two crafters share wood, two haulers share piles, gone targets release, two benches sharing a spot seat one, a sleeper skips a carrier's bed, a bill takes from its source. No-flap tests for eaters and crafters. Updated `TestTwoHaulersSplitTheCluster`, a `production_test` comment and three care tests (the dose claim moved to job start).

## Gaps
Parallel claim systems remain: haul frames (`incoming`/`deliveries`), bed `Claimant`, `care.Rest.bed`, surgery `hosp`, `care.Carries`, animal `FeedClaim`. Bed/patient has no per-patient policy.
