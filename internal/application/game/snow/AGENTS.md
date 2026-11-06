# internal/application/game/snow

## Purpose
The per-tile snow layer and the snow clearing area with its shovelling job (PLAY-015 §4). A consumer sub-package of `internal/application/game` (`.agents/specs/SOLID_GO.md` §10), moved out to pay for game lines.

## Owns
- `State`: depth, clearing area, derived counts and view caches; the game keeps it as `snowState = snow.State` in its map state.
- `Layer` (`snow.New`): every operation, a value built per call over the current map's `State`, so it is never cached across a map swap. `Seed`, `SeedDue`, `Chunks`, `ChunksAt` run from the game's weather phase; `AssignClear`, `DoClear`, `PaintClear` from its job and command paths; `View`, `Export`, `Import` from the read model and saves.

## Depends on
- `pkg/randx` (`SplitMix64`).

## Must not depend on
- `internal/application/game`; the simulation comes in through the roles in `deps.go`, implemented by `snowHost` (`game/weather.go`).

## Invariants
- No state outside `State`; `dither` is a pure hash of step and chunk.
- `snowHost` holds only the game pointer, so `snow.New` does not allocate: `At` runs on the movement path.
- Clearing starts at `ClearStart` and ends at bare ground (hysteresis, no-flap test `TestClearSnowNoFlap` in the game package).
