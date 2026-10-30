# HAUL-002: haulers gather nearby piles before delivering

User request (2026-10-30): a hauler facing many 12-wood piles took one pile per trip; it should collect nearby piles until it can carry no more, then deliver.

## What changed
- `item.SameLoad` (internal/domain/economy/item/item.go): two stacks can form one carried load (same kind, rot, dried, origin, animal, tame, quality, stuff, wear); `CanMerge` now builds on it.
- `job.Record` gains append-only `Pickups []EntityID` (stack, count pairs) and `PickupI`.
- New sub-package `internal/application/game/haul`:
  - `gather.go`: `Plan` (greedy nearest-first chain, each next pile within `Radius` = 8 tiles of the previous pickup, original tuning; ties to the lower id) and `ClaimAll`;
  - `step.go`: `Gather.Next` and `Arrive`, the pickup loop behind the consumer-owned `Host` role;
  - `load.go`: `Toppable`, `Cell.Room`, `Capacity` and `SplitLoad`.
- `haulToStockpile` (tick.go) collects candidates during its existing single pass over the items (no extra scan) and plans the pickups. The player's Haul order is planned in `prioritize` (`forcedJob` stays read-only for the orders menu).
- Capacity is the lowest of the carry limit (100), the kind's stack limit, and the room at the chosen storage cell, so a load never spills outside the stockpile. The last pile may be taken in part.
- Construction deliveries (HAUL-001), bill product storing and trough feeding are unchanged.

## Deviations from the plan
- `rec.Count` holds the load cap, so the first pickup is capped by room too.
- The room scan runs only for kinds with a stack limit under 1000; plain kinds always have a full load's room (`Toppable`).
- A pile already in storage joins a load only when it is bound for a higher-priority zone.
- `occupancy.go`: a gatherer only needs to reach the pile, not stand on its exact tile.

## UI
No new field. The carried pile shows through `PawnView.Carrying` and the carried sprite, now with the larger count.

## Tests and numbers
- `game/haul` tests at 100 % coverage; `item.SameLoad` tests.
- `game/haul_gather_test.go`: 12 piles of 12 wood take 2 trips with loads [100, 44] and all 144 land in the stockpile; plus the radius cut, mixed quality and stuff, forbidden and reserved piles, two haulers, the meal limit (10), a destination cell holding 60 of 75 taking 15, an interrupt mid-gather, and a save mid-gather.
- Journey `TestJourneyHaulerGathersNearbyPiles`.
- Package game is 4 lines smaller; budget lowered to 54,470. HAUL-001's slot, supply, share and route helpers moved into `game/haul` to pay for the new lines.
- Frame p99 is within noise of HEAD: grown 1.51/1.39 ms, raid 1.81/1.87 ms.
- TestGrownGolden re-pinned to `0x65452a93524e7998` with the reason, stable over -count=2.

## Gaps
- Loads of different kinds are not mixed: one carried stack per trip, as in vanilla.
- The 8-tile radius is original tuning.
