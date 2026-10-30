# crops package

## Package Purpose
The sowing catalogue the UI reads (crop picker, hover). A consumer sub-package of `internal/application/game` (`.cursor/SOLID_GO.md` §10), moved out by RES-1 to pay for game lines.

## Owned Concepts
- `View`, `All`, `ZoneSpecies` (crops.go): every sowable crop (fastest first) and the tree species a growing zone also sows; the game keeps `CropView = crops.View`, `Crops()` and `ZoneSpecies()` as thin wrappers.
- `ID`, `ZoneID`: a crop or zone species name to its `Command.Build` value (`game.CropID`, `game.ZoneSpeciesID`).

## Allowed Dependencies
- Domain tables only (`world/plant`, `world/terrain`, `sim/clock`). Nothing from `internal/application/game`.

## Invariants
- Pure functions: no state, no randomness, no clock reads.
