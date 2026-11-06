# Task Handoff

Task ID: HAUL-001
Status: complete

## Summary

Haulers carry full loads. The carry limit is now 100, up from 75. For construction, a hauler picks up as much as it can carry, feeds a route of up to 8 frames of the same material, each within 12 tiles of the previous one, and leaves the rest in the nearest stockpile tile that accepts it. If no stockpile has room, it carries exactly what the route needs. Refuelling works the same way, and a Supply order starts its route at the clicked frame.

Material on its way to each frame is booked, so two haulers never over-deliver one frame. Example: six 2-wood walls next to a stockpile holding 300 wood take one trip carrying 100. Every wall reaches 2/2 and 88 return to storage, with nothing left loose.

## Files Changed

- `internal/domain/economy/item/item.go`: `CarryLimit` 75 → 100.
- `internal/domain/ai/job/job.go`: `Route`, `RouteI`, `SpareX`/`SpareY`, `HasSpare`.
- `internal/application/game/haul_route.go`:
  - `planDelivery`, `routeFrom`: the route planner;
  - `startDelivery`, `releaseDelivery`: in-flight shares;
  - `routeWalk`, `routeArrive`, `storeLeftover`: walking the route;
  - export/import, plus `haul_route_test.go` (11 tests).
- `tick.go` (`assignHaul` falls back to stockpile hauling when planning fails; `doHaul` handles routes), `heat.go` (refuel), `orders.go` (Supply), `save.go` (`Incoming`, `Deliveries`), `game.go` (`haul` state).

## Invariants / Decisions

- `releaseDelivery` runs on every job end:
  - interrupt, which covers drafting, death, being downed, mental breaks, move and prioritize orders, and needs;
  - job replacement in `startJob`;
  - complete or fail in `phaseAI`;
  - the end of a route.
  - A safety net frees the shares of a hauler whose job was swapped without an interrupt.
- Frames that are gone or already filled are skipped on arrival. If the spare tile gets taken, the hauler looks for another; with none, it drops the load where it stands.
- Decisions iterate slices in id order. The maps are only looked up, so behaviour is deterministic.
- Jobs without a route (from old saves) still use the old single-frame path.

## Verification

- Race tests pass at 87.5% package coverage, and determinism passes with `-count=3`.
- Benchmarks: ColonyHour 21.9–23.9 ms and ColonyRaid 25.3–26.2 ms, against a baseline of about 21–24 and 25 ms.
- No existing test expectation changed. `TestHaulerRefuelsCampfire`'s scenario has no stockpile, so the exact-amount fallback applies.

## Known Limitations

- General goods are not batched; a pawn carries one stack per trip.
- There is no UI listing a route's remaining stops. The carried load, the walk goal and the drop marker (UI-013) show the trip.
