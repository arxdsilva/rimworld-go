# internal/application/game/windfall

## Purpose
The helpful incidents of 25.2 (#13): escape pod crash, ore meteor, supply drop, sky song. A consumer sub-package of `internal/application/game` (`.agents/specs/SOLID_GO.md` §10). `Windfall` is stateless and built per call; the game implements `Host` in a value type (no new `Game` field or method).

## Owns
- `Windfall` (windfall.go): `New(Host, Dice)`, `Fits(id, now)` (colonists > 0; the song also needs it off; never rolls), `Fire(id, now)`.
- `Host` and its roles (deps.go, architect-owned).
- Tests with a recording fake host and a scripted dice (`windfall_test.go`).

## Depends on
- `domain/incident` (IDs, `CrashHurt`, `Meteor*`, `Supply*`, `SongTicks`, `SongSpan`), `domain/economy/item`, `domain/world/geology`, `domain/sim/clock`, `domain/sim/entity`.

## Must not depend on
- `internal/application/game`: the roles in `deps.go` are the only way in.

## Contracts
- `Host` embeds five value-only roles, each ≤ 8 methods: Ground (5), Goods (1), Strangers (3), Story (2), Letters (1); plus `Dice` (1).
- Letter titles and formats are string literals at the call (the lang key scanner reads them).

## Invariants
- Draw order is part of the contract. Crash: no dice. Supply: stack count, then per stack a pick from the shrinking kind list and a quantity. Meteor: ore (weighted), cell count, then per try a centre x and y. Song: one roll over `SongSpan()+1`.
- No state: nothing is kept between calls.
- Bounded search: a meteor makes at most `MeteorTries*(1+4*MeteorCellsMax)` `MeteorFree` calls; a failed meteor leaves no rock and no letter.
- A supply drop that finds no first tile drops nothing; a later missing tile ends it with what landed.

## Tests
- `go test -race ./internal/application/game/windfall`: `TestFitsNeedsColonistsAndNoSong`, `TestPodCrashLandsDownedStranger`, `TestSupplyDropDistinctForbiddenStacks`, `TestOreMeteorClusterOnFreeCells`, `TestSkySongStartsOnce`, `TestFireDeterministic`.
