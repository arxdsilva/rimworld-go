# dossier package

## Package Purpose
The colonist card (RHUD, `.cursor/RIMHUD_GAP_ANALYSIS.md`): a player-edited tree of value widgets for one pawn or animal, docked in the inspect pane or floating over the map, and the setup modal that edits it. Read-only over the game: it reads read models through `Reader` and sends typed commands through `Actions`. On by default; `config.HUD.Enabled` off leaves the plain pane.

## Owned Concepts
- `Panel`: the card. `Select`, `Declare`/`Draw` in the pane, `DeclareFloating`/`DrawFloating` over the map, `Invalidate`, `Reset`, `OpenStats`, `TimingLines`.
- `Setup`: the modal, with the tabs Look (`setup_look.go`), Sections (`setup_sections.go`), Presets (`setup_presets.go`) and Colours (`setup_colours.go`). It holds no copy of the settings: every control reads `Prefs` and every change goes through `Setup.Edit`.
- `cardTimer` (`timing.go`): per-widget timings, measured only while the developer inspector asks.
- Roles in `deps.go`: `Reader`, `Actions`, `Prefs`, `Scene`, `PresetStore`, `FolderOpener`. The root's `dossierHost` (`dossier_compat.go`) fills them.
- `sheet/`: the pure part (layout trees, targets, widget catalog, built-in presets, text and tone of each line, colour maths). It imports no ebiten.

## Allowed Dependencies
- Leaves: `art`, `gfx`, `ids`, `ports`, `uikit`. The inspector is not one of them: it asks the root for the card through `inspector.CardHost`.
- `application/game` read models, `application/game/dossierview`, domain value types, `platform/config`.

## Forbidden Dependencies
- The ebiten root package, `cmd/`, and other feature packages.
- Anything that writes game state except `Actions.Enqueue`.

## Public Contracts
- The area menu is the game's own picker (`Actions.AreaPicker`), so hovering a row previews that area on the map.
- A stored layout that fails `sheet.Validate`, or that the size limits threw away (`config.HUDLayouts.Dropped`), warns once through `Actions.Hint`.
- Built rows wait for `HUD.RefreshTenths`; a selection change rebuilds at once. `followMap` in the root resets the caches when the viewed map changes.

## Test Requirements
- `panel_test.go`, `setup_test.go`, `setup_editor_test.go` over the fakes in `shell_test.go`; `bench_test.go` for the card's frame cost. Journeys in the root: `journey_dossier*_test.go`.
- `sheet/rows_test.go` holds `TestEveryRHUDRowIsClassified`: the gap document's rows, counts and widget key list must agree with the code.

## Docked size (DOS-10)

The docked card never scrolls. `config.HUDPane.Height` (120..192), `TabW` and the text widths are measures on a 720-tall logical screen; `sheet.DockedFor(hud, sh)` fits them to the real one (`CardHUD()` in the root's `dossier_compat.go` and the panel's `hud()`), so pane and tabs take at most 30 % of the screen's height (168 + 24 at 720, 252 + 24 at 1080) and the text grows with it; under 720 the text shrinks to 9 pt before the pane passes 30 %. The setup window reads the raw values. The built-in docked tree (`sheet.DefaultDocked`) is two lines, a rule and a four-column strip (bars; selectors and state; two skill columns; training for animals): 8 rows at the default text. A saved layout taller than the card warns once (`layout.go`) and hides its last rows; the floating card keeps the player's own size.
