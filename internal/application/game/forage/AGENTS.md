# forage package

## Package Purpose
A tame animal's food search rules (FEED-001). A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created when the game package sat at its budget.

## Owned Concepts
- `Reach` (reach.go): the cells an animal can walk to (or eat from beside) on its region layer; a label compare per cell, never a search. Wild animals use it only while fences stand (`game.fenceReachable`); tame animals always, on `region.Tame` when the colony's doors and gates open for them (`game.gatesOpen`), else `region.Wild`.
- `Pick` and `Best` (pick.go): the search result in two tiers, animal feed before meals (user decision 2026-10-30); each tier the nearest, then the bigger meal, then the lower rank (`Rank*`) and id.
- `Memory` (memory.go): saved with the wildlife state (`Wildlife.Forage`). `Skip`: spots an animal gave up on (path failed or the walk limit passed), skipped for `ForgetTicks` = 2500 (original tuning), at most four per animal. `Far`: starving animals searching their whole area or pen (`animal.FarSearch`: below 15 % until they eat or pass 25 %).

## Not Here
- The candidate scan and the walk: `game/animal_needs.go` (`findTameFeed`, `feedStep`). The diet table and tiers: `domain/animal` (`FeedValue`, `FoodTier`, `Prepared`).

## Allowed Dependencies
- Domain value types only (`animal`, `sim/entity`, `world/region`). Nothing from `internal/application/game`.

## Invariants
- No randomness, no clock; maps are looked up, and ranged only to delete (order-free).
- Every threshold has hysteresis or a cooldown and a no-flap test (`TestMemorySearchingNoFlap`; game: `TestStarvingTameSearchesFar`).

## Hot Paths
- `Reach`'s filter and `Skips` run per food candidate of a tame animal's decision (every 60 ticks through the bucket index).
