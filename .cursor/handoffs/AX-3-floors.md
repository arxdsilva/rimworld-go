# AX-3: every vanilla floor, made real

User request (2026-11-01, AX programme): make every Floors row of `.cursor/ARCHITECT_GAP_ANALYSIS.md` work end to end, with wiki numbers (ENG-003 extended to AX-*) and original names.

## What works
- **Floor kinds.** Straw matting (hay), concrete, paved tile, steel tile, silver tile, gold tile, sterile tile (steel, plus silver through A4's secondary-material delivery), stone tile and flagstone (any stone blocks), and the carpet floor (cloth).
  - The construction defs are AX-0's (materials /9, work /10).
  - `construction.FixedWork`: these floors keep the wiki work in every material, so `WorkFor` skips the stone/steel multiplier.
- **Wood floor is wood only.** Stone-block floors are stone tiles now.
  - Save v11 step: `construction.MigrateFloors` turns built and planned `Floor` of stone blocks or legacy stone into `StoneTile` (legacy stone becomes granite).
  - Map generation lays ruin and vault floors through `construction.FloorFor`.
- **Carpet floor colours.** `construction/carpet_colours.go` has 63 colours, following vanilla's set one for one under our own names. They are saved by index in `Building.Tint`/`Frame.Tint`.
  - DesignateBuild carries the colour in `Command.Index`; `TintFrame` in tick.go applies it.
  - Views: `BuildingView.AX.Colour` and `FrameView.AX.Colour`.
  - The RGB values are the `carpet` tints in `assets/palettes/base.yaml`.
- **The entrance carpet** (the 1-tile mat that wipes boots) stays as it is: no migration.
- **Room stats.** Room floors are counted by kind in `room.FloorOf(kind, stuff)` and `FloorStats`. Wiki values per cell:

  | Floor | Beauty | Cleanliness |
  |---|---|---|
  | Straw matting | -1 | -0.1 |
  | Concrete | -1 | 0 |
  | Paved tile | 0 | 0 |
  | Steel tile | 0 | +0.2 |
  | Silver tile | +4 | +0.2 |
  | Gold tile | +11 | +0.2 |
  | Sterile tile | -1 | +0.6 |
  | Stone tile | +1 | 0 |
  | Flagstone | 0 | 0 |
  | Carpet floor | +2 | 0 |

  Every floor kind counts as floored. The values feed room beauty and impressiveness, tile beauty, cleanliness, research speed (up to x1.09 in sterile tile) and food poisoning.
- **Infection.** In a clean room, `infection.RoomCleanFactor` slides the factor from x0.5 to x0.3 as floor cleanliness rises to +0.6. The two endpoints are wiki values; the line between them is original tuning.
- **Surgery.** `room.SurgeryCleanliness` adds the floors' own cleanliness to the surgery chance's room input.
- **Fire.** Straw matting burns at 1000 permille and the carpet floor at 800. These are original tuning on our scale, keeping the wiki's ordering.
- **UI.**
  - Architect cards in Floors carry AR1's icons. Stone tile and flagstone get the material row (stone picker).
  - The carpet floor gets a 63-swatch colour picker (`architect/colour.go`, click id `arch.colour.<name>`) and a "Colour: X" line.
  - Floor tooltips show "Beauty … Cleanliness …" (`game/floors.Line`).
  - Floor panes show the same line plus the carpet colour (`inspector/floor_title.go`). Titles read "Marble tile", "Slate flagstone", "Mint carpet floor".
- **Art.** AR1's sprites: `structure/floor_<kind>`, `floor_stone_tile_<stone>`, `floor_flagstone_<stone>` (stone tint), and `floor_carpet` (colour scale), in `render/world/floor_keys.go`. `structure/floor_stone` and its source were deleted, because nothing draws them now.

## Remove floor
A1 owns it (`orders.FloorRefund`): half of the stuff and the extras, nothing for flagstone; smoothed stone is never a target.

## Follow-up (lead's three items)
- **Walk speed.** wiki: Floors, Terrain: speed = 13/(13+path cost), soil 2, floors 0, straw matting 1.
  - A step on a floor takes 87 % of the plain-ground ticks, and on straw matting 93 % (`construction.WalkPct`).
  - It is applied in `weatherSlow` for colonists and animals: 15 ticks become 13 or 14, a diagonal 21 becomes 18 or 20. Rounding is to the nearest tick.
  - The path costs are unchanged: floors make a step faster, never costlier.
- **Sterile surgery.** `body.CleanFactor` now reaches its wiki 1.018 (about x1.02) at +0.6 cleanliness, sterile tile's own, instead of at +1.0, which no floor reaches.
- **Floor over floor (vanilla) and carpet repaint.**
  - Any floor laid over another floor is an in-place swap (`construction.Replaces`). When it is built, the old floor is refunded as a deconstruction would be.
  - A carpet floor laid again in another colour repaints it: the colour rides the swap blueprint, and `completeSwap` copies Kind, Stuff and Tint.
  - In the Architect, a carpet floor already in the picked colour shows as "same" and is skipped.
  - The pane shows "Changing to carpet floor" or "Changing to sterile tile".
- `StuffID` now names single-material floor stuffs (hay, silver, gold, cloth). Before, the Architect sent 0 for them and the sim fell back to the default.

## Not done
- The `BuildNow` test hook still can't lay hay, cloth or gold floors.

## Game package
- Lines added: tick.go +1, frame_view.go +1, save.go +1, weather.go +2.
- Lines removed: surgery.go -4 (rule moved to `room.SurgeryCleanliness`), infection.go -2 (`infection.RoomFactor`), placement.go -1.
- Net -2. No new methods.

## Tests
- construction (contract, stuff, work)
- room `floor_test.go`
- infection `sterile_test.go`
- fire flammability
- `game/floors`
- game `ax3_floors_test.go`: colour, materials, v11 migration, sterile room, beauty
- render keys and art coverage
- journey `journey_ax3_test.go`: stone picker, colour picker, a steel tile's pane
- inspector `floor_title_test.go`
- the Architect box-fit test now also checks the colour swatches

Save round-trip failures in the full game run come from A4's `WorldView.AX.DeepOreVer` (0 before save, 1 after load), not from AX-3. The determinism gate passed on a snapshot of the tree.
