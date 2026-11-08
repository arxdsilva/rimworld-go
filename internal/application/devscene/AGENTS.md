# internal/application/devscene

## Purpose
The developer and capture scenes of `cmd/game -scenario`, moved out of `cmd/game` (2026-11, GAR): set pieces placed on a fresh colony through the game's dev hooks, the benchmark's grown colony (ENG-001) and the social colony. README clips (MED-01..06, PLAY-024), linked walls (ART-015), power (PLAY-021, PWR-1), pens (PLAY-015) and the zoo (PLAY-022) start here.

## Owns
`Build(Config, seed, biome)`, `Bare`, `Known`, and the `scenes` table (`scenes.go`); the larger set pieces live in `walls.go`, `colony.go` (late game, burial, roofs), `animals.go` and `power.go`.

## Must not depend on
Adapters or `os`: the composition root passes `FixtureDir` (`RIMWORLD_FIXTURE_DIR`).

## Contracts
- Every table scene builds (`TestEverySceneBuilds`); every scenario the `-scenario` help names is `Known` (`internal/adapters/inbound/cli/game` `TestHelpedScenariosExist`).
- An unknown or empty scenario is a plain colony; `-hour` then jumps, and the rooms are roofed (SIM-018).

## Notes
A new scene is one entry in `scenes` (no `init()`), named in the `-scenario` help when it is meant for media.
