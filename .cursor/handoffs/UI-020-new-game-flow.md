# UI-020 The new-game flow, widget kit, real font and in-colony additions

Part of NG-1 (`.cursor/NG1_NEW_GAME_PLAN.md`). Commits: `9ea84352` (widget kit), `c2f24849` (scenario, editor, storyteller, difficulty), `d62f06ed` (globe, backdrop), `d7b8c84d` (world, landing, factions, characters, loading), `a9ca527c` (commitment UI, minimap, colony UI), `6cf941f2` (setup sounds), `e531b633` (font sweep).

## What changed

### Widget kit (`ebiten/ui_*.go`)
Immediate mode, on the real font. A screen declares its widgets once per `Update` from a hook registered with `registerScreenUI` (`ui_state.go`); the hook runs right after input is polled and before the click path. `App.Draw` then calls `uiPaint` for the queued widgets, the modal stack and the tooltip. Widget ids carry a per-screen prefix (`scen.`, `sced.`, `story.`, `diff.`, `world.`, `land.`, `pawns.`, `fac.`, `modal.`). State lives on `App` as `a.ui` (`uiState`).

| Widget | File |
|---|---|
| Button with tip, style, disabled reason | `widgets.go`, `ngButton` |
| Checkbox, radio group | `ui_check.go` |
| Slider (drag, stops, keys, Shift x5) | `ui_slider.go` |
| Text field, single and multi-line (caret, selection, key repeat, Esc restores, click-away commits) | `ui_textfield.go` |
| Virtualised scroll view with wheel routing away from the camera | `ui_scroll.go` |
| Tooltip (rests 24 frames, wrapped, clamped to the screen) | `ui_tooltip.go` |
| Modal stack (`CtxModal`, Esc closes the top one) | `ui_modal.go` |
| Sortable table | `ui_table.go` |
| Tabs | `ui_tabs.go` |
| Progress bar, sparkline | `ui_progress.go`, `ui_spark.go` |
| Layout helpers (`rect`, dialog frame, "window too small" notice) | `ui_layout.go` |

Tab and Shift+Tab move focus. Esc order: text edit, top modal, Back. Widgets under an open dialog stay inert (fixed in `c2f24849`).

### Real font (D4)
- Noto Sans (OFL 1.1), Regular and SemiBold, embedded from `ebiten/fontdata/` with the licence; Ebiten `text/v2`.
- API in `font.go`: `textStyle`, `drawText`, `measure`, `lineH`, `wrapPx`, `ellipsize`, `setUIScale`. Type scale `tsSmall` 11, `tsBody` 13, `tsLabel` 13 bold, `tsLarge` 16 bold, `tsTitle` 22 bold, `tsHuge` 32 bold.
- `drawLabel` and `debugPrintAt` route to `drawText`. The sweep (`e531b633`) replaced every 6-7 px glyph and 14-16 px line assumption with measure/wrap/ellipsize: architect categories on two rows, a wider inspector with proportional label columns, a hover hint card above the colonist bar, proportional load-list columns, vector damage floaters. UI scale (90/100/115/130 %) multiplies all sizes.

### Wizard screens
All draw over `setup_backdrop.go`: the dimmed menu diorama, a starfield and a distant planet (the generated world, or a showcase world generated a few steps per frame); Reduced Motion freezes the drift. Draft state is `newgame_state.go`; it survives Back and Next and is turned into a `game.Setup` only when Start is pressed.

- **Choose Scenario** (`screen_scenario.go`): searchable list of the four built-ins (Broken Hull, Ember Clan, Gilded Expedition, Bare Hands) plus custom files, hardness pips, a full details panel; new, edit-a-copy, load, delete.
- **Scenario Editor** (`screen_scenario_editor.go`): name, summary, description, the mandatory setup, an ordered list of optional parts with an add-part picker (14 part kinds), a live preview and a validation panel that blocks Done on errors only. Custom scenarios are JSON files written atomically.
- **Choose Storyteller** (`screen_storyteller.go`): portrait cards (`ui_portrait/<key>`), bios, an example pacing sparkline, the six difficulty presets plus Custom, and the save mode (Reload anytime or Commitment).
- **Custom Difficulty** (`screen_difficulty.go`, modal): grouped sliders and toggles with dependent fields, Copy from a preset, Reset, Cancel, Accept.
- **Create World** (`screen_worldgen.go`): free-text seed with original two-word phrases (for example "amber-hollow"), coverage (30/50/100 %; 5 % in dev mode), rainfall, temperature, population, a faction table (counts, add, reset, no-raider warning) and generation behind a progress modal that steps `planet.Builder` a fixed budget per frame. Unchanged settings reuse the world; changed ones mark it stale.
- **Select Landing Site** (`screen_landing.go`, `globe.go`): the rotating hex globe (one triangle batch; biome colours with hill shading, ocean depth, lakes, rivers by size, dashed roads, faction-tinted settlement emblems, hover and selected outlines; views for biome, temperature, rainfall, elevation; drag with inertia, zoom, keyboard rotation). Tile panel with terrain, coordinates, elevation, temperatures, rainfall, growing period, stones, water, caves, roads, nearest settlement, movement difficulty, forage, wildlife and warnings. Random site, landing rules with the reason in Next's tooltip, **Advanced** (starting season, map size) and the **Factions** dialog (`screen_factions.go`: goodwill bars, a row click centres the globe on the capital). Globe cost 0.3-0.5 ms per frame at full coverage.
- **Create Characters** (`screen_pawns.go`): selected and left-behind rosters with drag, swap and reorder, editable names with validation, portraits by build, backstories, traits, eleven skills with passions, incapabilities, health conditions, team skills with gap warnings, Randomize all, per-candidate reroll, undo. Backed by `game.CandidatePool` (see SIM-032).
- **Start and loading** (`screen_loading.go`): Start builds the `game.Setup` and runs `game.ColonyBuilder` in steps behind a progress screen; the old seed and biome placeholders are gone.

### Map sizes
The size list comes from `game.MapSizes()`, gated by `game.MaxEnabledSide` (`ng1_types.go`), currently **120**. Sizes 200-500 are listed in Advanced as "Not available yet" and the default falls back to 120 ("120 x 120 (pre-release)"). They open when the lead raises `MaxEnabledSide` after the gates in ENG-001.

### Commitment-mode UI
Chosen on Choose Storyteller and fixed for the colony. In play (`saves.go`, `screen_pause.go`): one save slot, tagged "Commitment" in lists; the game saves at every day rollover, on return to the menu, on quit and on window close; pause offers **Save & Quit to Menu** instead of Save; Load is disabled with "Commitment: one timeline"; developer mode is locked ("Commitment mode"). Options › Story shows the mode read-only.

### In-colony additions
- **Factions window** (`factions_window.go`): the faction table over the live read model, a 60-day goodwill sparkline and the last five goodwill events; opened from the HUD bottom bar.
- **Bio tab** (`bio_ui.go`) in the pawn inspector: portrait, names, sex/age/build, backstories and traits with hover cards, what the colonist cannot do, eleven skills with level, passion, experience bar and "This week: +N", conditions.
- **Work tab**: hatched incapable cells with the reason, plus skill levels and passions shown in the grid.
- **Options › Story** (`story_tab.go`): change storyteller and difficulty during play with the same widgets (commands `SetStoryteller`, `SetDifficulty`).
- **Letters** (`letters.go`): a message with `Letter` set opens a window once with Continue (the scenario intro, a heat wave, a cold snap); the game keeps running.
- **Chips**: the top strip shows the permanent condition (for example Ashfall sky) and heat wave or cold snap.
- **Minimap** (`minimap.go`): maps over 150 tiles get a 180x180 overview bottom-right (Shift+M toggles; click centres the camera): ground, water, floors, buildings, plants, colonists as light dots, hostiles red, the camera rectangle.
- **Big-map rendering** (`lod.go`, `chunk_cache.go`): three tiers by on-screen tile size, live at 48 px or more, baked 16 px chunk images from 16 px, a 4 px whole-map bake below. Details in ENG-001.
- **New building cards**: Hospital bed and Tool cabinet with components in their tooltips; buildings a scenario forbids show disabled with the reason.
- **Sounds** (`6cf941f2`): slider ticks, wizard page turns, drop-pod fall/land/open, river and shore ambience (damped indoors).

### Captures
`-capture-ui <state>` (help text in `cmd/game/main.go`; scripts in `capture.go`, `capture_colony.go`):
- Setup: `scenario`, `scenario-editor`, `scenario-editor-addpart`, `storyteller`, `difficulty-custom`, `worldgen`, `worldgen-progress` (frozen at 40 %), `landing`, `landing-advanced`, `landing-factions`, `pawns`, `pawns-reroll`.
- Colony: `factions-colony`, `bio`, `work-incapable`, `arrival` (use `-capture-ticks 60`), `options-story`, `letter-intro` (`-capture-ticks 300`), `arch-hospital`, `arch-cabinet`, `arch-locked`, `chips`.
- The minimap shows on any map over 150 tiles, for example `-scenario grown -width 250 -height 250`.

## Verification
- Recorded at commit time; not re-run by the docs pass. The widget kit has `ui_kit_test.go` (slider drag and keys, text editing, scroll wheel and thumb, tooltip delay and clamp, modal Esc, table sort, tab focus order, wheel routing); each screen has its own tests; `widgets_test.go` still pins the UI palette to `ui.yaml` and `TestGlobePaletteMatchesYAML` pins the globe colours to `world.yaml`; `TestSetupMinWindow` covers the 1280x720 minimum.
- Font sweep: layouts measure, wrap and ellipsize instead of assuming glyph sizes. The plan's `BenchmarkHUDText` budget check (§1.1) was not added; text cost was not benchmarked separately.
- Globe: `BenchmarkGlobeFrame` (`globe_test.go`) measured 0.3-0.5 ms per frame at full coverage with no allocations (from the commit message).

## Known limitations
- Map sizes above 120 are not selectable yet (`MaxEnabledSide = 120`).
- No `pause-commitment` or `minimap` capture state was added (the plan listed them).
- The landing screen has no feature search, only Random site (out of NG-1 scope).
- Character details show no relationships (there is no relationship system).
- Custom Difficulty: the economy yield sliders, Infection chance and Pain tolerance have no tooltip, and Disease frequency says "No diseases yet" because disease events do not exist.
- Copy and paste in text fields depends on what Ebiten exposes; where it does not, they are skipped.
- No world map once the colony runs; the globe is a setup screen.
- README gallery strips for the new flow are not captured yet.
