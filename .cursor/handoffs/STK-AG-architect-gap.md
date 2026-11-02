# STK-AG: Architect gap analysis and placeholder cards

User request: "double check all default vanilla "architect" menu items and build a gap analysis so we can have all there at least as place holders" (even if greyed out). Programme brief: STK (lead, 2026-11-01).

## Delivered

- `.cursor/ARCHITECT_GAP_ANALYSIS.md`: 191 core vanilla 1.5 Architect items (wiki: Architect, Menus, Orders, Zone/Area, Structure, Floors, Ship, Campfire, Ornate door, Carpet), one row each with category, card, status and what blocks it. 118 rows have a working card (some partial); 73 are missing, shown on 66 placeholder cards. Families share one card (large tables, bookcases, flagstones; stone tiles map to the Wood floor card's stone-block materials).
- Category panel: all twelve vanilla categories in vanilla order (Orders, Zones, Structure, Production, Furniture, Power, Security, Misc, Floors, Recreation, Ship, Temperature), six rows of two. `hudlayout.ArchCatsH` went from 140 to 204 (2x8 pad + 6x28 + 5x4).
- Placeholders (`architect/planned.go`, `architect.Planned`): id `planned_<slug>`, icon `ui_icon/planned` (STK-AR draws it), no build kind or command. `Cards()` shows them disabled with reason "Not in this game yet" and the one-sentence description. The tooltip reads label, sentence, "Unavailable: Not in this game yet". `SelectTool` never arms one (it hints the reason), `Submit` and `Preset` skip them, so they can never emit a command.
- Card order: working tools first, then placeholders, both in rough vanilla order (`ToolsIn`). This keeps every working tool on the smallest supported grid (1113x626: 34 cells; Furniture holds 33 cards) and gives working tools the first command keys.
- Moved to their vanilla category: heater, cooler, passive cooler, vent and campfire to Temperature; wood floor, carpet and Smooth to Floors; horseshoe pin to Recreation; grave, sarcophagus, funeral pyre, vitals monitor, tool cabinet and multi-analyzer to Misc.
- New working cards for kinds that already existed with art and panes: Hydroponics basin (Production, icon `crop`) and Sun lamp (Furniture, icon `lamp`). They reuse those icons until dedicated ones exist.
- Bench sizes in tooltips (lead relay): the 14 production benches now read (3x1), the fabrication bench (5x2), the fuel refinery and radio console (3x2).
- ST's dump entry edit (icon `zone_dump`, new description) is kept as it is.

## Tests

- `architect/catalog_test.go`:
  - `TestEveryGapRowHasACard` parses the gap analysis. Every row's card exists in its category. Missing rows point at placeholders and done or partial rows at working cards. Every placeholder has a row, and the counts table matches.
  - `TestPlannedCardsAreGreyAndInert` covers the greyed card, the hint, no arming, no commands from a forced Submit, and working cards before placeholders.
  - `TestArchCategoriesHaveTools` pins the twelve categories in order.
  - `TestToolCatalogIsConsistent` checks the placeholder shape.
- `journey_arch_test.go`:
  - `TestJourneyArchitectVanillaCategoriesAndPlaceholders`: Tab, then each category's button, with every card on the grid. A hover past `uikit.TipDelay` shows the tooltip. A mouse click on the card arms nothing, and a click plus a drag on the map send no command and add no frame. Eleven categories are covered; Temperature has no placeholder.
  - `TestJourneyArchitectMovedTools`: the moved and new tools arm from their new categories.
- Exact edits to root tests and captures that pinned the old categories (no assertion dropped):
  - `architect_hud_test.go`: only placeholders may be disabled with nothing selected.
  - `burial_anim_test.go`: grave and pyre are in Misc.
  - `bridge_ui_test.go`: the bridge-after-floor ordering check now checks that the floor is in Floors.
  - `power_ui_test.go`: Temperature is listed and holds the heater and cooler.
  - `journey_architect_test.go`: floor and Smooth open from Floors.
  - `capture_colony.go` and `capture_colony_test.go`: the cabinet opens from Misc and the campfire from Temperature.
  - `capture.go`: the carpet opens from Floors.
  - `power_ui_test.go`: the heater and cooler cases open Temperature; the Power list ends with its three placeholders; the campfire is checked in Temperature.
  - `journey_rsch_test.go`: the multi-analyzer opens from Misc.
  - `journey_temp1_test.go`: `t1Place` opens Temperature.
  - `home_carpet_ui_test.go`, `frame_ui_test.go`: the carpet opens from Floors.
  - `stone_art_test.go`: Smooth opens from Floors.
  - `health_ui_test.go`: the vitals monitor is in Misc.
  - `colony_ui_test.go`, `architect_hud_test.go`: the scenario-locked furniture card is now the stool (the campfire left Furniture), the tool cabinet is checked in Misc, and the Orders area-tool loop allows placeholders.
- Ran with `-race`, `GOFLAGS=-p=2`: `./internal/adapters/inbound/ebiten/architect/`, `./internal/adapters/inbound/ebiten/hudlayout/` and `./internal/adapters/inbound/ebiten/`. No simulation change, so no golden values move.

## Checklists

- MAIN_UI §17 lists the twelve categories and the placeholder rule.
- UI/UX §5.1: Category navigation stays [x] with the STK-AG note. Build option cards stays [-]: grouped families still lack a material or colour picker.
- Vanilla checklist §34.1: per-category [-] with what is missing, and Temperature [x]. Power went from [x] to [-] because three vanilla items are missing.

## UI representation

Presentation only. Placeholders read the static catalog and emit nothing. No domain or application change.

## Open

- Each placeholder becomes real by replacing it with a `ToolDef` in `Tools` and flipping its gap row to done (the test enforces the pairing).
- Until AR's `ui_icon/planned` lands, placeholders fall back to the label-initial tile (`drawCmdFallback`).
