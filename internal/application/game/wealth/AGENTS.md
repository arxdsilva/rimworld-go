# internal/application/game/wealth

## Purpose
The colony's wealth for the storyteller (EAI-0): `Tracker` per map with `Value`, `History`, `Step(now)`, `Export`/`Import`.

## Contracts
- `Counter` (one method, `Wealth() int`) is the full count; the game passes itself. The package never imports `game`.
- The game keeps one `*Tracker` per map (`mapState.wealth`, built in `initMapHosts`) and saves `Export()` per map (`saveState.Wealth`, nil while empty).
- EAI-0 skeleton: `Step` recounts and samples nothing, `History` stays empty, nothing in the tick calls `Step`. The v<17 migration seeds a loaded map with one `Step` (derived, not saved or hashed).

## Invariants
- Deterministic: time only from the caller's tick; no randomness.
- The last count is derived: never saved, so a save's state hash does not depend on when it was counted.
- No global scans on the tick: a full pass (`Counter.Wealth`) is for `Step` on its schedule and for read models, never every tick.
