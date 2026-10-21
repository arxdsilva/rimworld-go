# SIM-018 Roofs

## What changed
- **Domain** (`internal/domain/world/roof`):
  - `Cell` flags: Built, Natural, PlanBuild, PlanRemove, Supported (derived), NoAuto.
  - `SupportRange` 6 (Chebyshev), `AutoMaxCells` 150, `BuildWork` 30, `RemoveWork` 15.
  - `SupportField`: a multi-source BFS from every support.
- **Game** (`internal/application/game/roof.go`):
  - **Support:** Rock, or a completed Wall or Door. It's recomputed on every room rebuild (`afterRoomsRebuilt`).
  - **Auto-roof:** every unroofed, supported cell of a closed room of at most 150 cells gets PlanBuild, unless the player removed its roof (NoAuto).
  - **Commands:**
    - `DesignateRoof`: supported tiles get PlanBuild; clears NoAuto and PlanRemove.
    - `DesignateRemoveRoof`: sets NoAuto; built tiles get PlanRemove; natural roofs are untouched.
    - `DesignateCancel` clears the plans.
  - **Jobs:** `job.Roof` and `job.Unroof`, Construct work type, appended to `assignConstruct || assignRepair`.
    - The nearest planned tile is reserved; ties go to the lower row-major index.
    - The work spot is the tile, or a walkable neighbour. A builder tries the 3 nearest reachable tiles, then waits.
    - Action `build`; the builder faces the tile.
  - **Collapse:** in `phaseClimate`, unsupported built roofs fall. One message per batch, "A roof collapsed near (X, Y) — no support within 6 tiles.", and a `RoofCollapses` stamp kept for an hour.
  - **Natural roofs:** mining or sapper digging that turns Rock into Stone leaves a Natural roof. It never collapses.
  - **Index:** every write goes through `setRoof`, which keeps a sorted plan index and per-room roofed counts (`roomRoofed`, permille, O(1)).
- **Heat:** with roofed share f, a room drifts to `Outdoor + IndoorBonus·f` and leaks `roomLeak·(1 + 4(1−f))`. Fully roofed rooms are unchanged.
- **Sky:** `outdoorsAt` now means `underSky`: no roof (walls and doors count as covered). Used by soaked and snow-day thoughts, `weatherSlow`, `weatherTempDelta`, `doused`, `PawnView.Soaked` and `PlantView.Wilted`. "Slept outside" requires no room and open sky.
- **Fix:** `RoomView.Lit` now needs a burning campfire, so a fire doused by snow no longer lights its room.
- **Views:** `WorldView.RoofCells` (flags per tile), `RoofCollapses`, `Hour`, `DayOfYear`; `RoomView.Roofed` (0..1).
- **Saves:** `saveState.Roofs` (additive). Older saves auto-roof their closed rooms after loading. Collapse stamps aren't saved.
- **Dev:**
  - `RoofRooms()` completes every planned roof.
  - `HoldWeather()`.
  - `-scenario roofs`: winter snow; a roofed room with a fire, the same room with its roof removed, a porch roofed by designation, and a 13×13 yard too big to roof.
  - `-scenario sunny`, `-hour`, `-capture-timelapse`.
  - The scenarios call `RoofRooms`.

## Tests
- `roof_test.go` (domain and game): 25 tests covering support, auto-roof, size limit, NoAuto, tile-by-tile building, designate, remove and cancel, collapse, natural roofs, heat (unroofed, roofed unchanged, half roofed, campfire), rain, snow, door, sleep, views, saves, old saves, determinism, doused fires.
- `heat_test.go`: huts now get roofs (`hut` = `hutWalls` + `RoofRooms`). No assertions changed.
- Coverage: roof 100%, game 86.7%.
- Headless winter huts (`scratchpad/light/tempcheck.txt`):
  - roofed with fire: about 16 °C, as before roofs
  - roofed without fire: about 3 °C above outdoor
  - unroofed: tracks outdoor with about an hour's lag

## Benchmarks and determinism
- ColonyHour 17.5 → 17.5 ms, ColonyRaid 20.5 → 20.5 ms.
- Determinism tests ×3 green. Seeds diverge where rooms exist (builders roof them; unroofed rooms leak heat).

## UI
The Roof overlay, roof tools, hover lines and lighting are in UI-010 and ART-010.

## Known limitations
- No collapse injuries, and no thin versus overhead-mountain roofs.
- Collapse stamps aren't saved.
- Plants under roofs still grow (no light requirement yet).
