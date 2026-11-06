# internal/application/game/crops

## Purpose
The sowing catalogue the UI reads (crop picker, hover). A consumer sub-package of `internal/application/game` (`.agents/specs/SOLID_GO.md` §10), moved out by RES-1 to pay for game lines.

## Owns
- `View`, `All`, `ZoneSpecies` (crops.go): every sowable crop (fastest first) and the tree species a growing zone also sows; the game keeps `CropView = crops.View`, `Crops()` and `ZoneSpecies()` as thin wrappers.
- `ID`, `ZoneID`: a crop or zone species name to its `Command.Build` value (`game.CropID`, `game.ZoneSpeciesID`).

## Depends on
- `world/plant`, `world/terrain`, `sim/clock`.

## Must not depend on
- `internal/application/game`.

## Invariants
- Pure functions: no state.
