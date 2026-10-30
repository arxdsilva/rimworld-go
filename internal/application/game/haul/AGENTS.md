# haul package

## Package Purpose
Stockpile hauling's own rules. A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), created by HAUL-002 when the game package sat at its budget.

## Owned Concepts
- `Plan` (gather.go, HAUL-002): the load a stockpile haul gathers. A greedy chain from the first pile: each next pile is the nearest same-load candidate within `Radius` (8 tiles Chebyshev, original tuning) of the last one taken, ties to the lower id, up to the load's capacity; the last pile may be taken in part. `Pickups` encodes the picks into `job.Record.Pickups` (stack id, count pairs); `ClaimAll` claims each pile's planned count when the job starts (RES-1 count claims: another hauler may plan the rest of a pile; the game releases the first pile's claim once the load is split off it).
- `Gather` (step.go): the pickup walk. `Next` heads to the next pile left that still joins the load (skipping vanished, taken, forbidden, changed or unreachable piles); `Arrive` takes the pile into the carried stack (rot and poison averaged by quantity) and walks on. Interrupts need nothing here: the game drops the carried load and frees the hauler's claims.
- Storage room (load.go): `Toppable` (a stack takes a load), `Cell.Room` and `Capacity` (a load never exceeds what the carrier, the stack limit and the destination cell allow), `SplitLoad` (the load split off a pile).
- `Ground` (ground.go, STO-1): the per-cell index of loose stacks, in item-list order, verified not trusted (rebuilt when the game's `Moved` generation moves or the store is replaced; every lookup re-checks the store; `looseIndexed`/`looseScan` compared under the game's index self-check). `First`, `Others`, `Room` and `Shelf` answer the game's `itemAt`, `cellFull`, `haulRoom` and the storage search's shelf loads.
- HAUL-001 delivery keys and shares (slots.go, route.go): `SlotKey`/`SlotFrame`/`SlotOpen`, `Supply*`, `RouteReach`, `RouteFrames`, `Share`, `Shares`, `Route`. The delivery planner itself stays in `game/haul_route.go`.

## Not Here
- Choosing the first pile and its storage cell, and the candidate scan: `game/tick.go` (`haulToStockpile`, `planGather`, `haulCands`), `game/zones.go` (`storageCell`, `storeSearch` over `zone.Stores`) and `game/haul_priority.go`.
- Construction deliveries, bill product storing and trough feeding: they never gather.

## Allowed Dependencies
- Domain value types only (`sim/entity`, `ai/job`, `ai/pathing`, `economy/item`, `building/construction`).
- Nothing from `internal/application/game`: the roles in `deps.go` are the only way in.
