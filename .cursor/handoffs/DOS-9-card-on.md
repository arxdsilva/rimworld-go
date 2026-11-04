# DOS-9: the colonist card on by default, and the audit

## What changed

- `config.HUD.Enabled` is true. Settings files from v0.1.23 have no `hud` key and decode over the defaults, so existing players get the card too. Options > Accessibility has the switch.
- RHUD-042: the developer inspector lists each widget's prepare, declare and draw time (`dossier/timing.go`, `devtools.Host.CardTiming`). It measures only while the inspector asks.
- New capture state `arch-storage`.

## Bugs the flip exposed

- Focusable pane head buttons let `Kit.TakeTab` steal Tab, so Tab no longer opened the Architect with a colonist selected. `uikit.BtnOpts.NoFocus` fixes it (`inspector/pane_header.go`, `gear_rows.go`). `TestJourneyDossierLeavesTabToTheArchitect` covers it.
- The expanded panel beside the tall pane overlapped Global at small windows. `hudlayout.placeExpanded` now narrows it.
- Values right-aligned in a card column touched the next column's text ("97%Wander"). `dossier.cardGutter` adds 8 px between side-by-side children.
- Bodies on a corpse dump spot sat off the tile, because `LyingPose`'s offsets are for full size. `world.heapBody` scales them with the body.

## Audit

Every row of the three gap documents was re-read against the code. Result: RHUD 429 done, 75 excluded. NST 27 done. CSE 45 done, 10 excluded. The audit found RHUD-092 and 100 missing and 057 and 284 partial; all four were built afterwards. The audit read code and did not run it; the journeys and package tests ran it. The stated differences from each mod are listed under Counts in each gap document.

## Tests changed by the default

Journeys now reach the plain pane through `dossierOff`, the way a player does. Tests about the plain pane set `HUD.Enabled = false` with a comment saying so. No assertion was weakened.

## Known gaps

The map's extreme lower-left corner (about 4x5 tiles at the camera's limit) stays behind the docked card; float it or turn it off.

## UI

The card itself, its setup modal and Options rows are the representation. Spec §10 and §42 are updated.
