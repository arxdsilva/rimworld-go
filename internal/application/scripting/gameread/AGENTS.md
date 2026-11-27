# internal/application/scripting/gameread

## Purpose
Answers the `scripting.Queries` readers (wild animals, map terrain, temperature and weather, factions, sites, building counts) from a game's exported reads, for the script world queries `ctx.animals()`, `ctx.buildings()`, `ctx.map`, `ctx.world`.

## Owns
- `New(g *game.Game) scripting.Queries`: one stateless `reader` over the game.

## Depends on
`application/game` (a sub-package may import its parent; allowed by `internal/arch`), `application/scripting`.

## Contracts
- Every list is capped at `scripting.MaxQueryRows` (256) and sorted (species, kind), so the same game gives the same rows.
- A reader holds no cache and is valid only while the game has swapped to its home map during a scripted call; reads are the home map's even while the player views another (`TestExtCallsHomeMapWhileViewingAnother` in `application/game`).
- Built only by the composition root (`internal/compose/game`).

## Tests
`gameread_test.go`; end to end `TestExtE2E/AC3`.
