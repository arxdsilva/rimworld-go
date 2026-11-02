# STK-AR: production bench art at vanilla size, planned and dump icons

Agent AR, 2026-11-01. Part of the STK programme (lead brief: 3x1 production benches). BN owns the simulation sizes in `construction.go`. All art is clean-room and drawn from code in Blender, with nothing traced or referenced.

## What changed
- **Benches redrawn at the wiki sizes, in all four facings, glow layers kept:**
  - `furniture.py`: workbench_basic, stonecutter_table, stove_basic, butcher_table, brewing_bench, drug_table, tailoring_bench, electric_stove, electric_tailoring_bench.
  - `workshop.py`: fueled_smithy, electric_smithy, machining_table, art_bench, electric_smelter.
  - These 14 are all 3x1.
  - Lead additions after BN's wiki check:
    - fabrication_bench 5x2 (`workshop.py`);
    - biofuel_refinery 3x2 (`workshop.py`);
    - radio_console 3x2 (`power.py`; `RADIO_MAST` and `RADIO_PANEL` moved).
- **Layout rule (added to ART §14):** the main work item sits on the middle cell of the worker's side: vise, burners, hearth, induction coil, lathe, turntable, mash tub, mortar, sewing machine, hopper, or the radio set and microphone. Tools and stock go on the side cells.
  - `_bench_legs` is a new helper in `furniture.py`.
  - The `worker`/`cook` attachments move to the middle: [1.5, 1.15]; fabrication [2.5, 2.15]; refinery [1.5, 2.15]. No Go code reads them.
- **Keys and anchor:**
  - Keys are unchanged, so `build_keys.go` needs no edit.
  - `footprint` metadata is [3,1]/[1,3], [5,2]/[2,5] and [3,2]/[2,3].
  - `origin_px` uses the same formula (anchor = footprint top-left). The renderer reads footprint and origin from metadata, and no Go file hardcodes bench sizes.
  - There are no 2x1 leftovers.
- **Icons:**
  - Architect icons re-cut from the new south sprites (17).
  - New badge icons in `overlay.py`:
    - `ui_icon/planned`: a grey hourglass. AG uses it for greyed placeholder cards (`architect/planned.go`).
    - `ui_icon/zone_dump`: a crate with a drop arrow, requested by ST for the dumping stockpile tool (`catalog.go`).
- **Shadows:** no `shadow:` blocks are written, because SH's pipeline change had landed before the final render.
- **Render scope:** only these sprites were rendered, with the scratch driver `render_benches.py`, which calls `furniture/workshop.build(only=…)`, `power.render`, `O._icon` and `O._badge_icon`. Full families were not re-rendered, which avoids GPU-noise churn. A full `make art` reproduces the same output from source.

## Checks
- `validate_assets.py` reports no errors for any of these keys. The remaining errors are SH's pending `shadow:` yaml strip on other families.
- `TestNoOrphanSprites` and `TestPlantRecipesHaveNoOutline` (-race) pass.
- `normalgen` was run.
- In-game: `game -capture -capture-ui bills` shows the stove and workbench at 3x1, side by side, with the selection covering all three cells.

## Open
- `scenes/menu.py` places `_workbench` at (9,5). The diorama is not part of `make art` and was not re-rendered. On the next `make art-menu` it becomes 3 wide (x 9-11).
- `capture.go:99-100` puts the stove at cx+1 and the workbench at cx+4. At 3x1 they touch but don't overlap.

## Checklists
- ART §32 "production/research benches" row: STK-AR note added.
- ART §14: bench layout rule added.
