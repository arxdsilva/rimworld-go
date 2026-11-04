# DOS-10: the docked colonist card in 30 % of the screen

User request (2026-11-04): the docked colonist/animal card took half the screen (pane 360 + tabs 24 on a 724-tall logical screen); it should be half that height with every function and no scrolling, and its numbers must scale with the screen but never pass 30 % of it.

## Size

- `config.HUDPane.Height`, `TabW`, `HUDText.BodySize`, `LabelW`, `ValueW` are measures on a 720-tall logical screen. Defaults: 168, 96 (6 tabs), 12 pt with the body line at 89 % (16 px), 100, 36. Limits 120..192 (`HUDPaneMinH`, `HUDPaneMaxH`), line pct floor 85.
- `sheet.DockedFor(hud, sh)` (`dossier/sheet/scale.go`) fits them to the real screen: at or above 720 the text grows with `sh/720` and the pane with it, never past `30 % − tabs`; below 720 the pane keeps its height while that fits in 30 %, else takes 30 %, and the text is the largest that leaves the layout room, down to 9 pt (only under 550 tall does the pane pass 30 %). Column and tab widths follow the text. Integer math, no allocation. Applied in `CardHUD()` (`dossier_compat.go`) and the panel's `hud()`; the setup window reads the raw values.
- Results: 550 → 9 pt / 141 px; 720 → 12 / 168 (26.7 %); 1080 → 18 / 252 (25.6 %); 1440 → 24 / 336 (25.0 %). At 1280x720 the pane is 576x168 with 6 command columns; 1024x768 436x179, 4 columns; 1920x1080 864x252.

## Content

`sheet.DefaultDocked`: line(standing, identity); line(body_state, mind_state); rule; strip(bars column; area, hour, diet, doing, next_up, in_hand, keeper, captive; skills shooting…intellectual; skills melee…social + work speed; training for animals). Eight rows and a rule: 8×16 + 4 = 132 px in the 134 the pane leaves (`PaneLayout` with a card: 0 under the title, 2 below; margin 34 in all). The old tall tree is gone (no permitted height holds it); a test pins its widget list and checks nothing is dropped for colonist, prisoner, tame and wild animals. The floating card keeps `skillBlock` and the player's own size. The gear sits in the card's upper-right corner, docked too (`TestGearCorners` allows it over the two text lines only).

## Settings

- `SettingsVersion` 9: a file still on the old defaults (Height 360, TabW 72, line 100 %, ValueW 60) takes the new ones; a custom height is clamped to 192; a custom docked tree keeps its values and gets 192. Sliders and `options/card_rows.go` show the height as a share of the screen. A saved layout taller than the card warns once (`layout.go` `warned`) instead of hiding rows silently.
- Built-in presets (`preset.go`): Standard 168 / 6×96 / 12 pt; Roomy 192 / 7×96 / 13 pt (its right column keeps the target filters to fit; over 30 % only under 604 tall); Snug 152 / 6×80 / 10 pt; Snug with traits 168 (over 30 % under 594). `testdata/standard.preset.json` regenerated.
- `hudlayout.paneSize`: card floor 120, `w = min(st.PaneW, max(PaneW, rx − HudMargin − 5·CmdStride), rx − HudMargin)` so the commands keep room.

## Also in this change

VSync is on by default (user decision, PERF-003): the third kernel panic of 2026-11-04 came with the client pacing the frames on an external display in a scaled mode. Presets set it, the version-9 migration turns it on unless the player chose it off, and the frame cap stays 120 for VSync off.

## Tests

`TestDockedForTable`, `TestEachBuiltInFitsEveryScreen`, `TestDockedTreeKeepsEveryWidget`, `TestDockedCardDropsNothing`, `TestGearCorners`, `TestTooTallLayoutWarnsOnce`, `TestCardBodyMargins`, `TestCardPaneLeavesCommandRoom`, `TestPaneHeightRow`, `TestCompactPaneMigration`, and `TestJourneyDossierCompact` (area, hour and diet pickers, bar → tab, gear, resize 1280x720 → 1920x1080 → 1024x768: pane + tabs ≤ 30 %, commands never over the pane); the geometry journeys re-pinned to the new numbers.

## Not done here

Captures (`-capture-ui dossier`, `dossier-animal`, `dossier-setup` at 1280x720, 1024x768, 1920x1080) wait for the Mac's display to be back on its native mode (PERF-003): a capture opens a window. Things to look at in them: the gear over the end of a long name while hovering; 9 pt text on small screens; the strip's spacing at 436 px wide.
