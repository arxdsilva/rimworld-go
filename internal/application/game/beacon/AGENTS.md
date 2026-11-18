# internal/application/game/beacon

## Purpose
The cells an orbital trade beacon trades from (PLAY-024, PLG-026 Part B). A pure helper sub-package of `internal/application/game` (`.agents/specs/SOLID_GO.md` §10): the game asks it for one beacon's reach and caches the union.

## Owns
- `Diameter` (15): the reach across, in cells. The placement ring (`overlay.BeaconRadiusTiles`) is `Diameter / 2`.
- `Reach(w, h, bx, by, blocked, add)`: the cells of the disc 15 across (`dx²+dy² ≤ 56`) that a 4-neighbour flood from the beacon reaches without entering a `blocked` cell.

## Depends on
- Nothing but the standard library.

## Must not depend on
- `internal/application/game` or any adapter.

## Invariants
- Wiki: Orbital_trade_beacon, "15 tiles in diameter"; "Solid walls will break the area of influence of a trade beacon, but columns and other passible objects don't block it."
- The game's `blocked` is rock (`terrain.IsRockWall`) and built object-layer walls (`construction.Wall`); doors, columns, fences, frames and blueprints don't block (`game/trade_orbital.go` `beaconCells`).
- The flood never steps diagonally, so a wall ring with a missing corner still closes.
- The beacon's own cell is always in and never asked about; an off-map beacon reaches nothing; no cell leaves the map or wraps a row.
- Cells come in ascending index order, the same every call. `Reach` allocates nothing (fixed arrays over the 15 x 15 box).
- The game caches the union of all beacons as a `tileSet` under cell-set key bit 254 (no building kind) and rebuilds it when `structVer` moves; the index self-check rebuilds and compares, so a missed bump panics.
- Not modelled: beacon power (the wiki brief has none).
