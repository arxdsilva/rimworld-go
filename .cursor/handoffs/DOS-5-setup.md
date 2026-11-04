# DOS-5: the card's setup dialog

## Where things are
- `ebiten/dossier/setup_window.go`: `Setup`, `NewSetup`, `Open`, `OpenAt(tab)`, `Reset`, `HUD()`, `Edit(f)`, `Changed()`, the tab strip, the "Changed" tag, "Back to defaults" with its confirm. The dialog is a `uikit.Modal` (`dossier.setup`), so Escape and the X close it through `RunModals`; the root needs no per-frame call. `Edit` copies the colour map, clears `HUD.Preset` unless `f` set it, stores through `Prefs`, then `Panel.Invalidate()`.
- `setup_look.go`: the Look tab. Four groups (card, pane, floating card, text), every number as typed entry plus slider, every switch with a reason line when greyed (also its tooltip).
- `setup_colours.go`: the Colours tab. Grouped list with swatches (changed ones in the heading colour), editor (sample, `#RRGGBB[AA]` entry, hue, saturation, lightness, opacity), reset one, reset all, and a preview of the four bar styles over the player's bar colours. Only differences are stored; the theme's own colour deletes the key. HSL is kept while the colour is what it made, so grey keeps its hue.
- `options/card_rows.go`: Options > Accessibility, second column: Colonist card (on/off), Card sits (pane or map), Pane height (step 20), Floating card at (nine places). Greyed with a reason by `optionLock` (`saves.go`), which now also stops `ApplyOption` for a greyed row.
- Root: capture states only (`capture.go`, `capture_colony.go`); DOS-6 wired `SetOnSetup` in `dossier_compat.go`. `App` is still 57 fields and 244 methods.
- `journey_dossier_test.go`: `dossierOn` now goes Escape, Pause > Options > Accessibility, clicks the rows, returns, and restores the selection the Escapes dropped.

## Rows
Finished: 001, 002, 003, 028-030, 032-033, 054-056, 383-391, 392-415, 417-431 (colour keys through `uikit.DossierColor`; 417-418 keys are not editable, by design), 476. RHUD-419 groups: card, skills, sides. Bar styles (428-431) are previewed here; each bar picks its style in DOS-6's tree editor.
Checklists are not ticked (not mine).

## Decisions
- "Back to defaults" keeps `Enabled` as it is, or the card would vanish under the player's hand; the "Changed" tag ignores `Enabled` for the same reason.
- Look controls are greyed, not hidden: floating group while docked; pane group without Restyle; pane height while floating; letter gap without compress or while docked (RHUD-476).
- Offset sliders use `hudlayout.CardOffsetBounds` at the live screen size; changing the anchor clamps the nudge.
- Typed entries commit on Enter or leaving the field (and clamp); sliders apply live. The buffer keeps the typed text the frame an edit ends, so the commit reads it.
- The colour editor and rows use only `uikit` tokens and `uikit.DossierColor`; no RimHUD name, text or colour.

## Tests
`dossier/setup_test.go` (reasons, copy-on-edit, anchors, letter gap, colour edit and resets, HSL grey, defaults, fit inside the dialog at 1113x626, 960x540 and 742x417), `journey_dossier_setup_test.go`: `TestJourneyDossierColourEdit`, `TestJourneyDossierBackToDefaults`, `TestJourneyDossierOptionsRows`, `TestCaptureDossierSetup`.

## Captures
`-capture-ui dossier-setup | dossier-setup-sections | dossier-setup-presets | dossier-setup-colours`. `cmd/game/main.go:55` help string: add them after `dossier-animal`.

## Gaps
- A click on a text field puts the caret where it lands (kit behaviour); there is no select-all on focus.
- The Options group sits in the second column like the Graphics extras; at 742 logical pixels wide (UI scale 1.5) that column is clipped, as the existing ones are.
