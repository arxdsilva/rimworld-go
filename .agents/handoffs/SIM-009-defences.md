# SIM-009 Defences: doors, sandbags, spike traps

## What changed
- `construction`: buildings have hit points (`MaxHP`, doubled for stone) and a
  saved `Damage`; `CoverPct` per kind. New kinds `sandbags` (walkable, 55% cover,
  stone OK) and `spike trap`.
- `internal/application/game/defense.go`:
  - `hostileMap`: the walkability raiders and wild animals path with; the colony's
    doors are shut to them (`pathQuery` picks it for raiders; animals always use it).
  - `startBreach`/`doBreach`: a raider that cannot reach its target plans a route
    as if walls and doors were gone and bashes the first one on it (club damage,
    7 with a rifle butt, every 60 ticks). `damageBuilding` removes a building at 0 HP.
  - `coverFor`: ranged hit chance is cut by the best cover on the cells beside the
    target, facing the shooter.
  - Spike traps: raiders and predators that step on one spring it 75% of the time
    (32 injury); the trap is re-laid as a blueprint. Colonists never spring them.
  - `assignRepair`/`doRepair` (job.Repair) under Construction work, only once no
    raiders are standing. `DamageAt` dev hook.
- Raiders try to attack from where they stand when already in reach; otherwise a
  failed path (`unreachable`) starts a breach.
- Adapter: Security architect category (sandbags line tool, spike trap); health bar
  under damaged buildings. New art `furniture/sandbags`, `furniture/spike_trap`, new
  palette material `burlap`. `-scenario defense` / `-capture-ui defense`.

## Tests
`defense_test.go`: doors shut to hostiles; a raider smashes into a closed hut;
sandbag cover; trap springs and is re-laid; builders repair; walls break at 0 HP.
