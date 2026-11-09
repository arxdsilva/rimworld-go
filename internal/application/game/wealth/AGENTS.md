# internal/application/game/wealth

## Purpose
The colony's wealth for the storyteller (EAI-014): `Tracker` per map with `Value`, `History`, `Step(now)`, `Export`/`Import`.

## Contracts
- `Source` (one method, `WealthOf(from, to) (sum, parts)`) is the map's wealth in parts; the game passes itself (`Game.WealthOf`, story.go). The package never imports `game`.
- The game keeps one `*Tracker` per map (`mapState.wealth`, built in `initMapHosts`) and saves `Export()` per map (`saveState.Wealth`, nil while empty). `Game.Wealth()` is `Value()`.
- The tick calls `Step(now)` every `Stride` ticks on each map's pass (phases.go). Calling it twice at one tick is harmless; a missed slice is caught up.

## Invariants
- No global scan on the tick: a Step sums at most its slice's share of the parts (about parts/100); a full count (`WealthOf(0, max)`) happens only in `Value` before the first pass has finished.
- Deterministic: time only from the caller's tick; no randomness. Everything, the pass in progress included, is saved.
- A pass starts on an hour's first tick and ends in its last slice (a Step at any other tick, as the v<17 load's, starts nothing); the day's first finished pass is its sample; at most `HistoryDays` samples.
