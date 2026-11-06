# DOS-4: the card in the game

The colonist card now runs in the colony: docked in the pane or floating over the map. `HUD.Enabled` stays false by default.

## Where things are
- `ebiten/dossier_compat.go`: builds `dossier.Panel` in `startSession`, drops it in `dropSession`. `dossierHost` is `Reader`, `Actions`, `Prefs`, `Scene`, plus `followMap` (Reset on a map switch), `declareFloating`, `drawFloating`, `capture`. `inspectorHost` gains the `inspector.CardHost` methods.
- `inspector/pane_body.go`: `CardHost`, asserted on the Shell as `bedPrisonReader` is on Sim, so `deps.go` is untouched. `CardPane` (`Pane.Card()`): `dress` (fills the new `Spec` fields), `Size`, `FloatRect`, `Target`, the merged log.
- `inspector/pane_header.go`: header widgets `pane.head.*` (name, emblem tip, next, info, rename, threat, care, self-tend), rename dialog, `selfTendState` shared with the Health tab, `ThreatResponses`.
- `hudlayout/layout.go`: `paneSize`, `placeCard`, `placeExpanded`, `CardRect`, `CardOffsetBounds`, `CardMinH`.
- `hud/global.go`: `hud.dossier` toggle, listed only while `HUD.Enabled`.
- Captures: `-capture-ui dossier | dossier-float | dossier-log | dossier-animal`.

## Payments
`shield_feed.go` merged into `draw_combat.go`. `lastTick, lastDraw` folded into `App.stamp`. `App` stays at 57 fields and 244 methods; `Kit` not grown. `solid_budget.txt` untouched.

## Screen and UI impact
With the card on: the pane is `TabW * max(MinTabs, tabs)` wide, tabs fixed and right-aligned, the header row has its buttons, and the card fills a pane of `Pane.Height`. With it off nothing changes (`TestJourneyDossierOffIsIdentical`). Groups, buildings, plants, items, zones and cell info keep the plain pane in every mode.

## Deviations
- RHUD-011: `selectNext` lives in `pane_header.go` over `selection.Model.Selectables`; `selection` was not ours, and no new method was needed.
- RHUD-502: the floating card draws after `lrn.Draw`, not right after `DrawPane`, so the grid and the right column do not paint over it. Menus still paint above.
- RHUD-003: `Pane.Height` applies while the card is the pane's body; a floating card's plain pane keeps 188.
- Journeys set `settings.HUD` by assignment: `setSettings` is denied in journey files and the Options rows come with DOS-5, which should move `dossierOn` onto them.
- The stale-id test for an id nothing ever had needs `Select`, so it sits in `dossier_compat_test.go` (`TestDossierStaleSelection`); the journey covers an animal that dies while selected.
- Two fixes in `dossier/` (DOS-3): `build.go` and `data.go` set `.A` on an opaque `color.RGBA`, which is premultiplied, so skill bars and the hover row drew near white; both use `uikit.Fade`.

## Gaps and notes
- The threat button uses the "!" text fallback: `order_attack` has no art (the grid's button falls back too).
- A widget under the floating card can still show hover, because Hot is the last one added; the press stays with the card.
- `SetOnSetup` is unset until DOS-5; the gear does nothing.
- `inspector/AGENTS.md` (not ours) should learn `CardHost` and the `pane.head.*` ids.
- `cmd/game/main.go:53`: add `dossier | dossier-float | dossier-log | dossier-animal` to the help string. The flag has no validation, so the captures already work.

## Tests
`TestJourneyDossier*` (Docked, Floating, HeaderCare, SelfTendRefused, LogWheel, TameAnimal, HeaderAnimal, HourPick, Rename, Toggle, NineAnchors, StaleID, OffIsIdentical, Golden1113, UniqueIDs, Resets), `TestDossierStaleSelection`, `inspector/pane_body_test.go`.

Open failure, not from this work: `TestPawnTabBodiesDrawInsideExpandedPanel/gear/x1.5`, the Unload row in the gear tab (`inspector/gear_rows.go`, another agent's).
