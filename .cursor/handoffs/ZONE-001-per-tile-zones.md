# Task Handoff

Task ID: ZONE-001
Status: complete

## Summary

Zones are now tile-based. A tile belongs to at most one zone, whether a stockpile, a grow zone (tree zones included) or a pen. A drag over taken tiles skips them. Touching zones of the same kind merge into one zone, which keeps the lowest id and its settings; grow zones merge only when they have the same crop. Grow zones skip infertile ground (rough stone, rock), while gravel is allowed. Each zone draws as a borderless fill per tile plus one outline along its outer edge.

## Files Changed

- `internal/domain/building/zone/map.go` (new) and `map_test.go`. These hold the `Map` with `Paint`, `Clear` (splits disconnected pieces), `At`, `Cells`, `Set`, `Export`/`Import`, and a sorted cell list per zone.
- `internal/application/game/zones.go`: `Game.zones`, `ZoneView`, `CanZoneAt`, save and migration.
- `food.go`, `tick.go` (Designate cases, `cancelRect`, stockpile rules, `stockpileCell(kind, nearX, nearY)`), `handling.go` (pens of any shape), `orders.go`, `game.go`, `save.go`. Tests are in `zones_test.go`.
- UI: `zones_draw.go` (fill, edge runs, pen fence, one label per zone) and the per-tile green/red drag ghost in `architect.go`.
- Art: `overlay.py` stockpile and grow-zone fills without the baked border, re-rendered.

## Public Contracts Changed

- `WorldView.Zones []ZoneView` (tiles plus settings) replaces `Stockpiles`, `GrowZones` and `Pens`. The old fields are removed once hauling lands.
- `Game.CanZoneAt(kind, x, y)` is added, along with `Simulation.CanZoneAt` on the UI's interface.
- Save gains the `ZoneMap` field. Older saves paint their old rectangles on load, in the order stockpiles, grow zones, pens; the first to claim a tile keeps it.
- Behaviour change: Cancel now clears only the tiles it covers. Before, it deleted every zone it touched. Two tests were updated for this.

## Verification

Race tests pass: the zone package at 95.3% coverage, game at 87.5%. Determinism (`-count=3`), save round-trip and migration pass. Captures were checked: one outline for merged stockpiles, the red/green ghost, and grow zones that leave rough stone bare.

## Determinism Impact

None. Ids are never reused, and iteration is always in id order or row-major order.
