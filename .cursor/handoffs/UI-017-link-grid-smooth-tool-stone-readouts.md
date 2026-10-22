# UI-017 Link grid, Smooth tool, stone materials in the UI

## What changed
- **`linkgrid.go`:** presentation-only connectivity, rebuilt when `StructVer` changes.
  - Per-cell flags and accepts: rock is `Rock|MapEdge`; walls and doors are `Wall|Rock`; fences are `Fence` and accept `Fence|Wall`.
  - Exposes `mask`, `corners` and `linkedKey`. Walls now join natural rock, and rock links past the map edge.
- **Draw path:**
  - Rock and terrain draw only the visible tiles, replacing the full-map scans.
  - The per-frame wall and fence sets are gone.
  - `drawLinked` picks the baked corner variant, falling back to `_NN`.
  - Stone is tinted by cell stone or by material, through a colour scale, or a mask for ore.
- **UI:**
  - Smooth tool and a Stonecutter table card.
  - `designation_smooth` overlay and a smoothing animation.
  - HUD shows Stone and Steel, plus Silver and Gold when above 0.
  - Architect info line shows the selected material's stock.
  - Material buttons ("Granite blocks", "Steel"); inspector titles ("Granite wall").
  - Hover uses `game.SoilLabel` plus a "Smoothable" hint.
- **Sound:** mine and smooth cues play only when the terrain actually changed. Cancelling a designation no longer plays `rock_break`.

## Verification
- `go test -race ./internal/adapters/inbound/ebiten/...` is green. The linkgrid tests cover:
  - all 16 masks, wall↔rock, fences linking one way;
  - diagonal-only walls;
  - the 2×2, no-diagonal and diagonal-only corner cases;
  - map edges;
  - mining the centre of a 3×3;
  - a rebuild with zero allocations when nothing changed.
- `BenchmarkDrawArt` went from 3.5 ms/op with 13,049 allocs to 1.7–1.8 ms/op with 6,154 allocs. `BenchmarkDrawArtMountain` measures 1.3–1.45 ms/op.
- Capture: `-scenario walls`.

## Known limitations
- The hover still says "Smoothable" on rough stone that has a floor built on it; smoothing such a tile is rejected.
- Stone-block builds show stock on the info line instead of a low-stock warning.
