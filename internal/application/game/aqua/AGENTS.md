# aqua package

## Package Purpose
Fish tanks on one map (FISH-*, `.agents/handoffs/_plans/FISH_TANK_PLAN.md` §3): each tank's state, the hourly step, the handler's stock and net jobs, the player's stock choice and the read model. A consumer sub-package of `internal/application/game` (`.agents/specs/SOLID_GO.md` §10). The rules live in `internal/domain/building/aqua`.

## Owned Concepts
- `Tanks` (aqua.go): one per map (`mapState.tanks`, nil until a tank is used), `Entry` per tank with a stock chosen, sorted by id. `Phase`, `Assign`, `Do`, `Apply`, `Gone`, `Export`, `Import`.
- `View`, `Option` (view.go): the game aliases them as `TankView`, `TankOption`.
- Roles in `deps.go`; the game's adapter is `tankHost` in `game/fuel.go`.

## Allowed Dependencies
- Domain packages and `pkg/mathx` (`Abs`) only. Nothing from `internal/application/game` or adapters.

## Invariants
- No dice, no wall clock; tanks walk in id order. `Phase` walks its own list, never the buildings.
- Feed is the shared fuel pipeline (`game/fuel.go`, `industry.FuelSpec`): refuel asks at a quarter, fills to full, burns only while stocked and powered.
- No actor reacts to health, and `Do` never reads power: a blinking grid cannot start and stop jobs.
- A tank out of reach is put off for the handler (`PutOff`), not retried every tick.
- A removed, uninstalled or moved tank loses its state: a ripe stock drops its fish (fumbled, as at Animals 0), a growing one is lost.
