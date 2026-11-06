# NST-D: drawing stored stacks in the Neat Storage pieces

**Files:** `render/world/{neat_keys.go,neat_test.go,build_keys.go,objects.go,labels.go}`, `art/catalog.go`. With the lead's OK, also `render/world/deps.go` (one field, `base.neat`) and `draw_art.go` (the item queue's row goes through `itemRow`).

**Keys:** `NeatKey(kind, facing)` gives `furniture/<slug>_<facing>`, or `furniture/<slug>` for a fixed piece. Slugs and rotation come from `construction.DefOf`; all keys are built once, so a lookup allocates nothing.

**Look table:** `neatLooks` holds slots per cell and `neatShow` for each of the 27:
- `showFlat`: every stack level at its slot.
- `showNone`: Safe, Drum and the four crates draw no stack, count or forbidden mark.
- `showHeap`: the dump spot draws its first 4 stacks and one count for the tile, the sum of `Qty`.
- `showOnEnd`: the vertical weapon rack turns each stack 90 degrees about its slot, right side up (`DrawSpritePose`). A probe gave 3 allocations per stack for both racks without `-race` and 677 B, 4 allocs/op in a benchmark for both; under `-race` both jitter 3-4 (cause not traced; both racks vary alike, so it isn't the rotation), so `TestOnEndDrawsAndAllocatesNothing` measures only the pose and its transform.

**Metadata:** `art.Meta` gains `SlotScale` and `Slots`, read in `ParseMeta` through `flatyaml`. Slots above `MaxSlots` (16) stay in `Attach` only. `slot_scale` absent means 1.

**Placement contract:** a slot is the point the stack's **centre** sits on, as NST-E1, E2 and E3 authored. The canvas px are for the first cell of the faced footprint, and the renderer adds the item's cell offset. A stack past the last slot (NST-B: a spill chain can overfill a cell) shares that slot. A missing attachment, sprite or building puts the stack on the centre of its tile.

**Finding the piece:** the renderer had no building-at-tile lookup, so `neatIndex` maps tile to piece, facing and footprint. It builds when a stored stack first asks and rebuilds when StructVer changes. It also rebuilds on another map's building list, because `Game.ViewMap` moves no StructVer. The key adds the list length and the first and last building ids.

**Draw order:** a piece sorts by its bottom row, so an upper cell's stack would be painted over at its own tile row. A stored stack's row is the piece's bottom row plus a small step per cell and slot. Loose stacks and plain shelves keep exactly `float64(it.Y)`.

**Labels:** counts in pieces of 6+ slots show only in the Live tier. Tags sit under each stack's slot, so a cell's tags don't lie on each other. `drawStackCounts` still reads `w.Items` as before.

**Plants:** not touched.

**Not done / for the lead:**
- `TestNeatArtCoverage` (doc's test table) is not written; it needs the real art, which this package's fake catalog doesn't load.
- A corpse in a heap has no flies: `CorpseFlies` is tied to the tile and allocates. The loose path keeps them.
- `render/world/AGENTS.md` should list `domain/building/construction` under allowed dependencies and the new index. The file isn't mine.
- Not seen in the real renderer: tests use a fake catalog. `TestGeneratedArtCoversRenderer` and the root journeys are the lead's run.

**Visual impact:** only pieces holding stacks change. No sprite, palette or asset was touched.

**Player-facing:** this is the on-screen representation of stored stacks; the Architect cards and inspector are WP-C. No checklist box changed (the lead owns those for NST).
