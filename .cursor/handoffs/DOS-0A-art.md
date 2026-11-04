# DOS-0A: colonist card colours and icons

**Asset family:** UI palette tokens and `ui_icon` badges (ART §17, §33). RHUD-036, 039, 399, 403, 406-409, 411, 414.

## Done
- 13 new `ui.dossier_*` tokens in `assets/palettes/ui.yaml`, mirrored in `uikit/widgets.go`: off, excellent, bar_tick, skill_like/love/full/busy, side_ally/wild, tree_stack/block/line/mine. Original values.
- `uikit/dossier_colors.go`: `DossierColor(key string) (color.RGBA, bool)` and `DossierColorKeys() []string` (31 keys, editor order, `tree_*` last and not player-editable). No `Kit` method or field. Keys with a chrome token keep it, as G2 says.
- New family `assets/source/blender/artlib/families/dossier.py`: `dossier_gear` (solid cog) and `dossier_toggle` (a person's card lifted out of its tray), both `_badge_icon`. Hue set to sit with the row's other badges.

## Contrast on the panel (white map / black map)
text 7.83/12.27, off 3.15/4.94, critical 5.07/7.95, warning 4.91/7.70, neutral 4.80/7.53, good 5.31/8.32, top 5.83/9.13. The skill, side and tree inks are all at least 4.5 over white. Bars: main on back 4.26, low on back 8.03, tick on back 13.72, tick on main 3.22.

## Decisions and deviations
- `text_off` uses the new `ui.dossier_off` (#93918A), not `TextDisabled` as RHUD-394 says: `TextDisabled` is 1.84:1 on the panel over a white map, too faint for a line that states a fact.
- RHUD-417/418: `tree_widget` is `Text`, `tree_core` is `TextDim`, `tree_bad` is `Danger`; the rest are new. The doc's "selected" stays `BtnSelected`, read directly.
- The tick is only 1.71:1 on `bar_low` (one colour can't read on a dark track and a light fill). A tick rarely sits inside a low fill; DOS-3 can add a dark 1 px side if the mood bar shows it.

## Lead to do
- Add `"dossier"` to `FAMILIES` in `assets/source/blender/build.py`, or `make art` won't rebuild these icons (lead-only file; `-- dossier` already renders them).
- Tick the checklists: the two icon rows and tokens are ready to use; nothing draws them until DOS-3, DOS-4 and DOS-5.
- `uikit/AGENTS.md` doesn't mention the theme table or `dossier_colors*.go` (outside my files).

## Tests
- `go test -race ./internal/adapters/inbound/ebiten/uikit/`: new `dossier_colors_test.go` (keys complete, contrast, bars, colours read apart, tokens in the YAML and used).
- Root package: `TestUITokensMatchPaletteYAML` (the doc's `TestUIPaletteMatchesYAML`), `TestUIContrast`, `TestUIPanelsAreNeutralTranslucent`, `TestUITextContrast`, `TestUITokensAreNotCopies`, `TestNoOrphanSprites` all pass. `internal/arch` passes.
- `validate_assets.py`: no dossier errors; 55 errors for other agents' furniture still missing normals.

Not inspected in the running game (nothing draws them yet): contact sheet at 96, 40, 28 and 20 px on button, selected and panel fills beside `plan`, `learn`, `overlay_rooms`, `colonist`, `power_toggle`. In-game check falls to DOS-3/DOS-4.

**Visual impact:** none in play until those packages wire it. **Save/sim impact:** none.
