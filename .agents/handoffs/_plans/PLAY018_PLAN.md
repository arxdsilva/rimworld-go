# PLAY-018: colony HUD re-layout (familiar arrangement), command buttons, charcoal palette, letters, Learning Helper

## Context
The user wants the colony screen to feel immediately familiar to RimWorld players. A returning player should know by instinct where to find resources, colonists, alerts, building tools, selected-object info, commands, time controls and the management tabs.

Today's layout came from our own spec diagram (MAIN_UI §9, UI-004); no user decision lies behind it:
- a top strip holding resources, date, weather and the overlay toggles;
- the colonist bar as a full-width band above the tabs;
- every inspector and the Architect in one top-left slot (8,40), with all actions drawn inside the panel;
- no clickable speed buttons;
- management windows that are either full-width under the strip or centred modals.

**User decisions (2026-10-25):**
1. **Arrangement only.** Copy RimWorld's spatial arrangement and interaction behaviour, but size everything on our own grid with our own fonts and icons. No RimWorld decompiled numbers or colours; project rules (AGENTS.md, MASTER_SPEC L10, ART §17/§33) stay intact.
2. **Palette.** A new original cool charcoal palette for every screen. The Learning Helper gets an original warm brown/gold.
3. **Scope.** All of: the HUD arrangement, the inspect pane plus command-button grid, the Architect re-layout, windows, letters and alerts.
4. **Learning Helper.** Add one.

The MAIN_UI §9 diagram is redrawn and this decision is recorded there.

**Bugs found on the way (fixed here):**
- `hud.plan` and `hud.rooms` share one rect (`hud.go:705` vs `room_stats.go:32`), so clicking the visible Rooms icon toggles Plans.
- `colonyChromeHit` misses the animal, item and plant panels, the chips and the hover panel, so clicks on them fall through to the map.
- Gizmo hotkeys follow screen position, so they change whenever a panel moves.

## Target layout (logical px, our sizes, all multiples of 4; margin 8, gap 4)
The map fills the whole screen (`colonyViewport` = full screen) and the HUD floats over it. `computeHUDLayout(sw, sh, hudState) hudLayout` is a pure function, recomputed once per Update (`a.hud()`).

**Bottom of the screen**

| Region | Rule |
|---|---|
| Tabs | Full width, 28 tall at `sh-28`. 11 equal tabs: Architect, Work, Schedule, Assign, Animals, Wildlife (new; was F5), Research (shows a progress fill), World, History, Factions, Menu. |
| Pane (inspect) | `{0, Tabs.Y-188, 400, 188}` |
| PaneTabs | A 24 px strip above the Pane; tab widths are measured. |
| Expanded | Opens upward from PaneTabs. Width `clamp(want, 400, 560)`; height capped below the colonist bar. |
| Command grid | 64×64 cells on a stride of 68. Starts at `Pane.right+8` (or `ArchCats.right+8`) and ends at `Global.X-8` (9 columns at 1280). Fills the bottom row left to right, then wraps upward, up to 4 rows. Only occupied cells block the map. |

**Right column** (x = `sw-240`, 232 wide)

| Region | Rule |
|---|---|
| Global controls | Built bottom-up from `Tabs.Y-8`: overlay toggle icon rows (24 px, 8 per row); 4 speed buttons (52×24: pause, normal, fast, superfast); date, then time + season; weather + wind; "Indoors/Outdoors N °C" for the hovered cell; one line per active condition (replaces the status chips). |
| Minimap | 180 square, just above Global, only on big maps. |
| Letters | 28 px rows stacked upward from the minimap or Global; at most 8, then "+N older". |
| Learning Helper | Top-right: `{X, 8, 232, 28}` collapsed, up to 200 tall expanded. |
| Alerts | 24 px rows from below the Learning Helper down to the letters, ordered by severity, then "+N more". |

**Top of the screen**

| Region | Rule |
|---|---|
| Colonist bar | Top-centre at y 8. Width cap `sw − 2·248`, so it never reaches the resources list or the right column. 52×64 slots. Scale steps 1 / 0.75 / 0.5 allow 1 / 2 / 3 rows, then a "+N" slot; names are hidden below scale 1 (tooltip instead). Six colonists fit one row; 30 colonists wrap to 18+12 at 0.75. |
| Resources | Top-left `{8, 8, 168, …}`, 20 px rows of icon + count. Counts stored goods only (stockpile cells). Categorised mode is collapsible. The list scrolls above the highest lower-left element and hides when less than 40 px is left. |
| Messages | Right-aligned to the right column, under the bar; at most 5 rows of 22 px, each at most 480 wide. |
| Hint and toasts | Just above the grid or pane (replaces `drawBottomHint` and the toast position). |

**Architect**
- ArchCats `{0, Tabs.Y-140, 212, 140}`: 2 columns of 100×28.
- ArchDesc above it, 320 wide: tool name, cost, description, stock, crop picker or material choice.
- The tools are command buttons in the grid.

**Windows**
- Work, Schedule, Research, Animals, Wildlife, Assign, History, Factions and World sit bottom-left on top of the tabs: `{0, Tabs.Y-h, min(wantW, Right.X-8), min(wantH, Tabs.Y-Bar.bottom-16)}`.
- They never cover the bar, alerts or global controls.
- Menu stays the pause overlay.

**Nothing selected:** the Pane shows the hovered cell's info (terrain, fertility, roof, temperature, light, room, objects), replacing the bottom-right hover panel.

## Contract (lead, alone, first)
1. **`hud_layout.go`.** `hudLayout` holds every rect above plus `BarSlots`/`GridCells []rect`, `BarScale` and `GridCols`. `hudState` holds the counts and flags the layout needs. `colonyChromeHit` becomes "the point is inside a region the HUD drew" (the minimap keeps its own click path). `colonyViewport` returns the full screen. `inspectorRect`, `hudTopH` and `hudBarY` stay as deprecated aliases until the last caller is gone, then get deleted.
2. **Split `hud.go` into owned files**, moving code verbatim and stubbing the rest: `hud_resources.go`, `hud_colonistbar.go`, `hud_right.go`, `hud_global.go`, `hud_cellinfo.go`, `hud_inspect.go`, `hud_commands.go`, `hud_tabs.go`, `hud_windows.go`, `learn_ui.go`, `cmds_pawn.go`, `cmds_thing.go`, `cmds_build.go`, `cmds_arch.go`. `hud.go` keeps only the frozen dispatchers: `drawColonyHUD`, `colonyButtons`, `colonyChromeHit`. Lead-frozen files: `hud.go`, `app.go`, `colony_ui.go`, `renderer.go`, `app_test.go`, `command.go`, config.
3. **Command buttons (`hud_commands.go`).**
   - Types:
     - `cmdButton{id, key, label, icon, desc, reason; hotkey; disabled, toggle, on, mixed; run func(*App); cmds []command.Command; options func(*App) []cmdOption}`
     - `cmdOption{label, reason, disabled, cmd, run}`
   - **Ids keep their legacy names** (`pawn.draft`, `item.forbid`, `bldg.move`, …) and go into `colonyButtons`. Every click-by-id test keeps working. On click: `run`, else enqueue `cmds`, else `applyClick(id)`.
   - **Producers:** `selectionCommands()` dispatches to per-kind producers: `pawnCommands`, `animalCommands`, `itemCommands`, `plantCommands`, `buildingCommands`, `frameCommands`, `archToolCommands`.
   - **Multi-select merge** by `key`, in the order keys were first declared:
     - disabled only if all members are, with the reason "(k of n)";
     - `on` means all members are on; `mixed` means some are;
     - a click on a mixed toggle switches everyone on.
   - **Hotkeys** follow declaration order (`input.Gizmos[i]`, up to 12); dedicated keys stay (R draft, F forbid). `gizmo.go` is deleted at the end.
   - **Options:** left-click runs the first enabled option; right-click or the corner arrow opens the float menu (`orders.go`, plus `pick func(i)`). Options cover:
     - the per-colonist "now" chips;
     - Equip/Wear/Eat/Haul;
     - Equip nearby;
     - bed owner and animal master;
     - build material.
   - **Tooltip** (kit `uiTip`): label, description, key, and the disabled reason in danger colour.
4. **Inspect pane (`hud_inspect.go`).**
   - Types:
     - `paneSpec{title, subtitle; bars []paneBar; chips []paneChip (e.g. a clickable allowed-area chip); lines []string; tabs []paneTab; close}`
     - `paneTab{id, label, reason, disabled, wantW, wantH, draw(img, rect) /*legacy*/, declare(rect) /*kit*/}`
   - `selectionPane()` dispatches to per-kind producers.
   - `a.paneRect()` and `a.expandedRect()` replace `inspectorRect`, `itemRect`, `plantRect`, `frameRect`, `buildingRect`, `animalPanel` and `groupRect`.
   - Tab ids stay `inspector.<tab>`. Clicking the open tab again collapses it.
5. **Hooks.**
   - `colony_ui.go`/`app.go`: `resourcesUI`, `colonistBarUI`, `rightColumnUI`, `globalUI`, `paneUI`, `commandGridUI`, `learnUI` and `mainWindowUI`.
   - Click prefixes `letter.`, `speed.`, `learn.`, `res.`, `cmd.`, `win.`, `tab.wildlife`.
   - `hudRightClick(mx, my)` runs before map right-click and only consumes clicks over the HUD.
   - `Renderer.onEnqueue` (Learning Helper hook).
   - `ui_modal.go` gets `anchor`/`modeless`.
   - The AST guard's `hudFiles` gains `hud_*.go`, `cmds_*.go` and `learn_ui.go`.
6. **Read models (game, additive).**
   - **`WorldView.Stored []StoredResourceView{Kind, Label, Category, Qty}`.** Counts uncarried, unfogged stacks on stockpile cells, inside `World()`'s existing item loop (`zoneOf` lookup, O(1)). Categories come from a new `item.Category`. `w.Food/Wood/…` keep counting all loose stacks, so alerts and Architect stock lines are unchanged.
   - **`WorldView.Letters []LetterView{ID, Tick, Kind (threat/caution/info/good), Title, Text, X, Y, Focus, Intro}`.**
     - Saved additively and capped at 40. Old saves' letter messages are migrated on load.
     - `MessageView` gains `Kind`; post sites set it.
     - Dismissal goes through a new typed `command.DismissLetter{Target}` (letters live in the save); "read" is UI-only.
     - Pause-on-threat keeps reading Danger messages.
   - **`Simulation.TemperatureAt(x, y)`** joins the UI interface (and `stubSim`).
   - Alert explanations stay UI-side.
7. **Palette tokens.**
   - Same 32 names in `ui.yaml`/`widgets.go`, with new values from PALETTE (lead writes `ui.yaml`).
   - New tokens:
     - `ui.letter_threat|caution|info|good`
     - `ui.alert_danger_bg|alert_warn_bg`
     - `ui.tutor_panel|border|heading|text`
     - `ui.mood_good|mid|bad`
8. **Settings** (`config`).
   - `Gameplay.ShowLearningHelper` (default true), `LearnCollapsed`, `LearnedLessons []string` (deduped and capped in `Normalize`), `CategorizedResources`, `ShowColonistBar` (default true).
   - `SettingsVersion` 4; older files decode over the defaults.
9. **Spec.**
   - MAIN_UI §9/§10 rewritten: new diagram, commands, letters, Learning Helper, and the recorded decision.
   - ART §17/§33 palette note.

## Agents (14; contract first, then all in parallel; consumers code against the stubs)
**Art rule (user, 2026-10-25):** every agent that makes or changes art (palette, icons, sprites, Blender families, visual review of art) runs on the best model available, `opus`, and loads the art-assets skill (`.cursor/skills/rimworld-go-art-assets/SKILL.md`). At the start of execution the lead records this in `.cursor/PARALLEL_WORK.md` §6 and as a feedback memory. Art is never put on a lower tier to save cost.

| Agent | Model | Owns |
|---|---|---|
| PALETTE | opus (art rule) | `widgets.go`, `widgets_test.go` (new contrast ≥ 4.5 test). Prototypes the original cool charcoal palette and the warm tutor set on a contact sheet of real HUD captures before choosing. Hands its hex table to the lead (who writes `ui.yaml`), plus a grep list of hard-coded warm literals per owner. |
| ICONS | opus (art rule) | `overlay.py` (cool badge disc; new glyphs: slaughter, release, move, copy, bill, equip, wear, eat, clear orders, 4 speeds, 4 letter kinds, 7 status icons (sleeping, mental, downed, idle, fighting, fleeing, burning), learn), `make art FAMILIES=overlay`, `make art-check`. Reviews every glyph at 1x and 2x and restores any noise-only re-renders. |
| GAME-RM | sonnet (opus if migration fails twice) | `game/letters.go`, `game/stored.go`, `World()` Stored lines, `save.go` Letters, post-site Kind arguments, `domain/economy/item/category.go`, `DismissLetter` apply |
| CORE-PANE | sonnet | `hud_inspect.go`, `hud_commands.go`, `gizmo.go`, `orders.go` (float-menu pick), `multiselect.go` (group pane and merge) |
| PAWN-INSPECT | sonnet | `inspector.go`, `prisoner_ui.go`, `bio_ui.go`, `social_*`, `health_ui.go`, `log_ui.go`, `order_queue_ui.go`, `inspect_actions.go`, `area_select.go`, `cmds_pawn.go` + tests |
| THING-INSPECT | sonnet | `animal_inspector.go`, `item_inspector.go`, `plant_inspector.go`, `cmds_thing.go` + tests (incl. `haul_ui_test`) |
| BUILD-INSPECT | sonnet | `building_inspector.go`, `frame_inspector.go`, `pen_ui.go`, `trough_ui.go`, `vault_draw.go` (casket UI), `cmds_build.go` + `s49`/`pen`/`frame` tests |
| ARCHITECT | sonnet | `architect.go`, `archtools.go`, `arch_locks.go`, `cmds_arch.go`, `tools_scale_test.go` |
| HUD-TOP | sonnet | `hud_resources.go`, `hud_colonistbar.go` (mood backing, status icons, selected outline, scale/wrap, double-click kept), `portrait.go`, `search_ui.go`, `stone_art.go` (`resourceReadouts`), `bar_doubleclick_test.go` |
| HUD-RIGHT | sonnet | `hud_right.go`, `alerts.go` (severity order, hover explanation card to the left, click/right-click cycles), `letters.go` (stack, open on click, right-click dismiss, intro still opens itself), `climate_ui.go` (messages), `toast.go`, `gathering_ui.go`, `threat_ui.go`, `chase_ui.go` + tests |
| GLOBAL | sonnet | `hud_global.go`, `hud_cellinfo.go`, `status_chips.go` (→ condition lines), `biome_ui.go`, `environment.go` (weather label), `room_stats.go`, `home_auto_ui.go` (plan/rooms overlap fix), `minimap.go`, `camera.go` (full-screen viewport, clamping, edge scroll), `devmode.go`, `mapfields_overlay.go` + roof/home/minimap tests |
| WINDOWS | sonnet | `hud_tabs.go`, `hud_windows.go`, `ui_modal.go`, `work.go`, `schedule_ui.go`, `research_ui.go`, `animals_ui.go` (+ Wildlife tab), `assign_ui.go`, `history_ui.go`, `factions_window.go`, `faction_rows.go`, `world_ui.go`, `caravan_ui.go`, `story_tab.go` |
| LEARN | sonnet | `learn_ui.go`, `lessons.go` (10 original lessons: camera, select, build, zones, work, schedule, draft and move, forbid and haul, research, speed; done on the action or "Got it"), `screen_options.go` row, `options_test.go` |
| VISUAL-TESTS | sonnet | Shared tests: `visual_invariance_test.go`, `ui_sweep_hud_test.go`, `ui_canvas_guard_test.go`, `hud_test.go`, `input_ui_test.go`, `colony_ui_test.go`, `map_hidpi_test.go`, `portrait_scale_test.go`, `panels_scale_test.go`, `capture_colony.go` (cursors from the layout, new capture states); new `hud_layout_test.go` |

A haiku agent runs the capture sweep and builds the contact sheets at the end, which is mechanical work. The visual review of those sheets is art work, so it goes to an opus agent or the lead. The lead keeps `ui.yaml`, `cmd/**`, specs, checklists, handoffs and README.

## Tests
**Changed by design.** Pinned numbers become rects from `a.hud()`; no assertion is dropped:
- `hud_test:137`, `roof_ui_test:147`, `home_carpet_ui_test:161`, `home_auto_ui_test` → "inside Global";
- `colony_ui_test:569/605`, `gathering_ui_test:116`, `map_hidpi_test:128` (y0 = 0);
- `input_ui_test:113/730` → hotkeys by declaration;
- `checkPortraits` and `portrait_scale_test` → `BarSlots`;
- `TestPanelsStayInTheirPanel` → the union of Pane, PaneTabs, Expanded and the occupied GridCells, or Window;
- the letter visual test → a stack entry, plus the intro modal.

**Must stay green unchanged:**
- every click-by-id command test;
- Escape order (window → selection → pause);
- disabled reasons and bar double-click;
- `TestUIDrawsThroughCanvas`, `TestFrameInvariance` pan/scale, `TestColonyHUDDrawsAtScale`;
- `widgets_test` token parity.

**New tests:**
- **Layout.** No two regions overlap, checked:
  - at logical 1280×720, 1920×1080 and 1113×626;
  - at UI scales 1, 1.5 and 2;
  - with the minimap and Learning Helper on and off;
  - with 6 and 30 colonists;
  - with 13 and 27 commands (wrap upward, right edge ≤ `Global.X − 8`).
- **Commands:** merge (mixed and disabled), hotkey order, options menu.
- **Chrome:** each region's centre consumes a click; empty grid space still reaches the map; HUD right-click never steals a drafted move.
- **Letters:** save round-trip, `DismissLetter`, right-click dismiss.
- **Alerts:** order, back-cycling, the card stays left of the alert.
- **Learning Helper:** a lesson completes on the action and on "Got it"; progress persists.
- **Stored resources:** stockpile-only and fog-aware counts.

## Risks
- **Input precedence** (new right-click step, stale rects). Covered by the chrome tests.
- **Hotkey letters change** (declaration order; Architect tools get keys). Noted in the README.
- **Tooltips and hints at the bottom.** They must flip above the grid.
- **Full-screen viewport.** Affects camera clamping, `centerOnMap` targets under the HUD, the minimap frame and edge scrolling; also about 17% more map rows are drawn, so check fps.
- **Letters save growth.** Capped at 40, with deterministic ids.
- **Leftover warm colours.** PALETTE's grep list goes to each owner.
- **All captures and README media go stale.** Refreshed at the end.

## Verification
- **Gates after each phase:** `go build`, `make fmt vet lint`, `go test -race -count=1 ./...`, the determinism set (`-count=3`), `make art-check`.
- **Captures** at `-capture-ui-scale` 1, 1.5 and 2 for: colony, inspector, pawn, item, plant, building/bills, multiselect, crowd (30 colonists), architect, arch-hospital, work, schedule, assign, world, history, letters, alerts, learn, wildlife, cellinfo, options. The lead adds new capture states in `cmd/game`. Then a visual review against the target layout.
- **Performance:** `-fpsprobe` (lategame and the default colony) against today's 119 fps avg / 94 fps 1% low; `BenchmarkDrawArt500`; new `BenchmarkColonyHUD`.
- **Docs:** MAIN_UI §9/§10 (diagram plus the decision), the ART palette section, both checklists (HUD anchors, colonist bar, alerts/letters, inspector, command grid, Architect, windows, Learning Helper), handoffs, and README text and media.
- **Commits** by area on `main`; no push.

## Critical files
- `internal/adapters/inbound/ebiten/{hud.go, hud_layout.go (new), hud_commands.go (new), hud_inspect.go (new), app.go, colony_ui.go, gizmo.go, inspector.go, architect.go, alerts.go, letters.go, work.go, widgets.go, visual_invariance_test.go}`
- `internal/application/game/{game.go, climate.go, save.go, letters.go (new)}`
- `internal/domain/sim/command/command.go`
- `internal/domain/economy/item/category.go` (new)
- `internal/platform/config/settings.go`
- `assets/palettes/ui.yaml`
- `assets/source/blender/artlib/families/overlay.py`
- `.cursor/MAIN_UI_SCREENS_SPECIFICATION.md`
