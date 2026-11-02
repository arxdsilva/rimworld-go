# STK-BN: production benches at the wiki footprints

Agent BN, 2026-11-01, STK programme (lead brief). The user asked for production tables sized as in RimWorld: 1x3 or 3x1 depending on the facing, with the work done at the middle section. ENG-003 covers the wiki numbers.

## Wiki check (fabrication bench, refinery, comms console)
The brief left these three unchanged unless the wiki disagreed. It did, and the lead said go (AR redrew them):

| kind | ours before | wiki infobox | now |
|---|---|---|---|
| Fabrication bench | 3x2, spot (1,2) | 5 × 2 (rimworldwiki.com/wiki/Fabrication_bench) | 5x2, spot (2,2) |
| Biofuel refinery (our "fuel refinery") | 2x2, spot (0,2) | 3 × 2 (/wiki/Biofuel_refinery) | 3x2, spot (1,2) |
| Comms console (our "radio console") | 2x1, spot (0,1) | 3 × 2 (/wiki/Comms_console) | 3x2, spot (1,2) |

The electric stove page (3 × 1) confirms the 3x1 row.

## What changed
- **`domain/building/construction`:**
  - These 14 kinds go from 2x1 with spot (0,1) to 3x1 with spot (1,1): Workbench, Stove, ButcherTable, TailoringBench, StonecutterTable, BrewingBench, DrugTable, ElectricStove, ElectricTailoringBench, FueledSmithy, ElectricSmithy, MachiningTable, ArtBench, ElectricSmelter.
  - The three kinds above get the wiki sizes. Every spot is the cell in front of the middle, and `Footprint` turns it with the facing.
  - The origin stays at the top-left of the rotated footprint, which matches AR's metadata anchoring.
- **New `application/game/resize`:** the save footprint migrations as data.
  - `V9` and `RefundV9`: RSCH-1's research benches, moved out of the game package.
  - `V10` and `RefundV10`: the 17 kinds above. Costs did not change, so the refund is the current cost.
- **Game package:**
  - `migrateResearchBenches` became the generic `migrateBenches(grew, refund)` in `research.go` (RSCH-1's helper). It is reused by both steps, and its message names the bench.
  - `SaveVersion` goes from 9 to 10. The v10 step keeps a bench whose new footprint and spot fit (bounds, standable ground, no object or blueprint). That bench keeps its origin and facing, and its new cells are blocked. Any other bench is taken down, refunded in full and announced with a message. Blueprints of these kinds are cancelled.
  - Line delta 0. Game method count is unchanged (one method replaced).
- **Fixtures:**
  - `scenario/grown.go`: the workshop's tailoring bench moves to (x+2, y+2), because two 3x1 benches do not fit in a 5-wide row.
  - `cmd/game` winter hut: the tailoring bench moves to cx+8 so it sits inside the hut.
  - The other placements were checked and do not overlap: the lategame compound, social, power, capture.go, and map-gen (ruins place walls and floors only; scenarios place no benches).
- **Consumers reviewed:**
  - These all go through `Footprint` and need no change: placement and ghost validity, frame and building views, power nodes, rooms, regions, fire, filth, the craft work spot and its reservation, the comms operator spot, ferment and art drop spots.
  - Facility reach (tool cabinet, multi-analyzer) and heat sources stay measured from the origin cell, as before.
  - The renderer reads footprint and origin from the sprite metadata (AR), so no pivot or key edits were needed.
  - AG fixed the catalog "(2x1)" strings.

## Tests
- `construction`:
  - `TestFootprintRotation`: exact cells and spot in all four facings.
  - `TestWikiBenchFootprints`: all 17 kinds, front-middle spot in every facing.
  - `TestStonecutterIsBench` updated.
- `game/resize`: `TestV10IsTheWikiBenches` and `TestRefunds`.
- `game/bench3_test.go`:
  - `TestStkBenchSaveMigration`: a rotated stove and the radio console fit and stay. A workbench meeting a wall and a machining table off the map edge are refunded (wood, steel, components) with two messages. The blueprint is cancelled, and the save is not migrated twice.
  - `TestPawnWorksWikiBenchFromMiddleFront`: 16 benches × 4 facings (64 runs). The crafter is always on the middle-front cell while working.
- Updated to the new size, never weakened: `TestMultiTilePlacementAndBlocking`, `TestActionRescueAndBenches` (benches spaced for 3x1), `TestWindBandIsBlocked…` (a 3x1 bench blocks 3 band cells: 425 W), `TestSupportedSaveVersions` (10 supported, 11 not), craft `TestBillStoreModes` and `TestIngredientSearchEdges` (spot 5,5).
- `ebiten/journey_bench3_test.go`:
  - `TestJourneyBench3StoveRotatedCoversThreeCells`: Tab, then the production stove, then E. A wall on the third cell or on the middle-front spot turns the ghost red. The click places a 1x3 blueprint, a colonist builds it, and the built stove is 1x3 with spot (11,9).
  - `TestBench3ArtFootprints`: every resized sprite's metadata footprint matches in all facings.

## Golden and gates
- Grown golden with BN's changes alone on HEAD: `0xd3dc0cc5d27f8703`, deterministic over `-count=2`. The shared tree with the other STK agents gave `0x753367ca3d4f612e`. The lead re-pins the final value.
- Every other `Determin|SameSeed|Hash|Golden` test passes.
- These pass with `-race`: `game/craft`, `thermo`, `research`, `resize`, `construction`, `arch`, and the whole `adapters/inbound/ebiten/...`.
- The failures left in `game` (burial, haul_claim unclaimed, area cap, cow butchering, gather cap) come from the other agents' changes. On a HEAD worktree with only BN's files, they pass.

## No new UI
The sizes show through the existing ghost, blueprint, sprite and inspector views. No new screen is needed.
