# UI handoffs

Families: UI, CTL, TIME, UIS. Each section condenses one task's handoff; the full original of `<stem>` is `git show 76d605d04:.agents/handoffs/<stem>.md` (a plan: `git show 76d605d04:.agents/handoffs/_plans/<stem>.md`). New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## CTL-001-mouse-controls
Mouse controls (CTL-001..020) · 2026-11-04 · c72269fee
- Changed (user request 2026-11-04: double click on furniture or an item takes the same type on screen and any action applies to all; plus everything else missing from the mouse; rows in `.agents/gaps/MOUSE_CONTROLS_GAP_ANALYSIS.md`): `selection.Group{Members []Cand}`, `Kind()` or `Mixed`; `Alike`, `Boxed`, `PickID`. A double click with nothing alike stays a click (tile stacks still cycle). Pane titles with count ("Various (n)"); commands merge by key, menus to shared options; `Button.Single`/`groupSingle` drop Move, Copy, Settings. F flips stacks and doors together.
- Changed: `windows/paint.go` shared brush (Work: right click, Shift on header, drag; Assign: drag on Drugs/Medical/Food). `uikit.Stepper` multiplies by `uikit.QtyStep` (trade 1/10, comms gift). Right click dismisses a message; right drag on the bar reorders, right click opens the world. Forbid/Allow/Hunt/Tame have a whole-map option (`architect.Card.Options`). Middle tap rotates the held tool/moved building. Escape clears a lone animal, item or plant.
- Sim/save: `command.SetColonistOrder`, `game/roster/bar.go`, `saveState.ColonistOrder`. `g.pawnIDs` never moves, so AI/work cannot see the order. Nil until used: no `SaveVersion` bump, pinned hashes hold; used, it enters `StateHash`.
- Decisions: the bar now sends the target's `BarOrder` (sim ranks across all maps; `TestRightDragInALaterGroupSendsTheRank`). Fixed on integration: tool cards took no clicks with a group selected; Shift on bare ground cleared selection; Architect order cards lost options (`archToolCommands`). `TestJourneyAX8EscapeShip` now waits out the double-click window (assertions unchanged); `TestGroupInspectorAtScale1_5` ignores options arrows.
- Tests: 30 journeys (`journey_multiselect_test.go`, `journey_mouse_test.go`); slowest (caravan counter) ~2.7 s under `-race`.
- Limits: `App` fell to 243 methods (`toggleWorkCell` -> `windows.Work.Click`); `passionPct`/`totalXP` moved to `game/roster`.
- Open: CTL-101..103; a colonist away during the first ever move holds no slot and returns at the bar's end. Sources are wiki/tips/reports, not checked against the game.

## TIME-001-day-periods-lighting
Day periods and outdoor light · 2026-09-08 · f84114d2e
- Changed: calendar names morning/afternoon/evening/night; `clock.Date` gained `Minute`, `Date.String()` = `Year Y, Quadrum Q, Day D, period HH:MM`; `clock.Period`, `PeriodAt`, `HourOfDay`, `NewAt`, `StartingMorning`, `StartHour`; `lighting.Outdoor(tick) Level`. `game.New` starts at 08:00. Presentation-only sky grade under the HUD (`daylight.go`); HUD shows period and HH:MM.
- Decisions: period and light derive from tick (not stored state); no colours in domain. Starting at 08:00 changes the initial tick of all new games (clock tests still use `clock.New()`). Future saves persist tick only.
- Open: indoor/roof darkness and lamps in `world/lighting`; dedicated date-weather HUD chrome (UI checklist 2.3); light does not yet affect plants/work/mood/accuracy; overlay is a full-screen grade, not per-tile.

## TIME-002-soft-daylight-overlay
Softer daylight overlay · 2026-09-08 · f84114d2e
- Changed: muted hues and a night alpha cap of 48 so day/night reads as a veil; adapter-only, `lighting.Outdoor` unchanged. No contract, determinism or save impact. Limits as TIME-001.

## TIME-003-one-x-tick-calendar
One-x tick calendar · 2026-09-08 · 7941e3322
- Changed: 60 ticks per real second at 1x, `clock.TicksPerHour` 2500 (was 1000), 60,000 per day, 60-day year; added `TicksPerSecond`, `DaysPerYear`, `RealSecondsPerHour`. Speeds 1x/3x/6x (renderer steps 1/3/6 ticks per frame); `cmd/game` sets TPS 60. `game.TicksPerCell` (15) gates random-walk cell changes; sprites lerp across that window (interpolation uses sim ticks).
- Decisions: speed only changes ticks per real second, never clock math. Calendar numbers are our content conversion of public 1x hour/day length. A day is ~16.7 real minutes at 1x.
- Determinism: new games start at `8*2500`; movement RNG consulted only on cadence ticks, so positions after N ticks differ from the old walker for a seed. Saves persisting raw ticks would read a different hour (no save format existed).
- Open: `TicksPerCell` is a stand-in for a move-speed stat; per-pawn speed and path following; schedule blocks driven by the clock.

## TIME-004-full-dual-time-surface
Full dual-time surface · 2026-09-08 · 7941e3322
- Changed: clock-face HH:MM:SS (2500-tick hour shown as 60 minutes), 1-based year/quadrum/day in `Date.String()` with seconds (`Date.Second`; `SimSeconds`, `MinutesPerHour`, `SecondsPerMinute`, `ClockSecondsPerHour`). Speed names normal/fast/superfast with 1x/3x/6x. New `pawn/schedule` package (`Activity`, `Grid`, `DefaultColony`); `PawnView.Activity`, `Game.Activity()`; renderer `Simulation` includes `Activity()`. HUD shows full clock, speed and current schedule block.
- Decisions: sim seconds (`tick/60`) differ from clock-face seconds. Schedule is derived policy; needs/jobs don't consume it yet. All pawns share `DefaultColony` (original blocks: night sleep, dawn anything, day work, evening recreation). Future saves persist tick + per-pawn grid, not formatted strings.
- Open: paint UI, copy, per-pawn grids and Schedule screen (Wave UI-2 / checklist 8.1; later done in UI-008); schedule-aware AI intent.

## TIME-005-clear-day-overlay
Clear day overlay · 2026-09-16 · 0fbe6665e
- Changed: no sky grade in Morning/Afternoon (alpha 0, so untinted 06:00-16:59); evening and night keep outdoor light x `skyOverlayMaxAlpha`. Main-menu colonist PNG cropped (20 empty rows removed) so the bust meets the bottom; splash layout constants unchanged, source PNG only.
- Limits: presentation-only; no domain, determinism or save impact. Open: roof/indoor and artificial lights.

## UI-000-agent-pipeline-integration
Wire the screens spec into the agent pipeline · 2026-09-08 · a9ccd60f8
- Changed: docs and skills only. `MAIN_UI_SCREENS_SPECIFICATION.md` is recognised always-on from `AGENTS.md` and loaded task-gated via skill `rimworld-go-ui-screens` (section named plus §0/§58/§59); the UI/UX checklist stays the ordered backlog. Added the skill and a thin pointer rule.
- Decisions: screens consume read models and emit typed commands; Wave UI-0 framework must exist before later-wave screens.
- Open: implement Wave UI-0 (primitives, focus, window/modal, snapshot boundary, command gateway); presentation-package `AGENTS.md` when the UI adapter outgrows the shell.

## UI-001-first-session-screens
First-session screens · 2026-09-13 · 45bf7692b
- Changed: the client boots into menus; `App` is the `ebiten.Game`, the colony `Renderer` exists only after Start (`SimFactory`; `cmd/game` no longer builds a `Game` at start). Exported `App`, `AppConfig`, `SimFactory`, `NewApp`, `DefaultWindowWidth/Height` (1280x720 resizable). Graph per spec §57: Boot, Main Menu, five new-game stubs, Colony, Pause, Options, Load, Credits. Escape closes pause then pops the stack; disabled actions stay visible with reasons.
- Decisions: menus run with `renderer == nil`; pause skips `Renderer.advance` and map input; session intents (`StartGame/Quit/ReturnToMenu`) live in the adapter; chrome is programmatic panels + debug text.
- Open: wizard selections don't affect the spawned map (Start used `cmd/game` flags); no virtual lists, text fields, rich tooltips, shortcut manager; Options tabs labels only; load stub, Continue disabled; §59 DoD not met.

## UI-002-warm-ui-palette
Warm UI palette · 2026-09-13 · 37c9fae3d
- Changed: menu/boot/pause/options/load/credits chrome uses an original warm palette (umber panels, leather buttons, cream-gold selection, procedural rust-on-navy gradient). Tokens in `assets/palettes/ui.yaml` and matching Go constants; tests fail on drift. Layout unchanged.
- Decisions: clean-room (original hexes and layout, generated gradient; the RimWorld screenshot was not used as an asset).
- Open: typed text colours (DebugPrint white only); health/mood/power/hostile semantic colours; `base.yaml`/`materials.yaml`/`factions.yaml`; readable body text once a font is chosen.

## UI-003-menu-text-layout
Menu text layout · 2026-09-15 · df4352581
- Changed: `bodyOrigin` places wizard body copy right of non-footer buttons (`max(x+w)+16`); footer buttons (`y` at `sh-64`) don't shift it; Boot uses `bodyDefaultX`. Button IDs unchanged. Layout only.
- Open: three-column scenario chrome when fonts exist; DebugPrint doesn't wrap; enable Generate when worldgen exists.

## UI-004-colony-hud-shells
Colony HUD shells · 2026-09-15 · 21aaef476
- Changed: colony HUD chrome replaces debug text: resource/time strip, colonist bar, bottom tabs (Architect, Work, Schedule, Assign, Animals, Research, World, History, Factions, Menu), selection inspector window (Character tab live), Work overlay. `Renderer` gained presentation-only `Select`, `ClearSelect`, `HasSelect`, `Selected`.
- Decisions: HUD/inspector/Work consume pointer hits before map selection; Escape closes Work, then inspector selection, then pause; Work cells are labels, never mutate sim; disabled tabs show "not implemented". Resource counts are placeholders.
- UI: spec §9, §10 (Character shell), §19 shells; Wave UI-1/UI-2 items `[-]`.
- Open: bind Work cells to priorities (done UI-008); inspector Needs/Health/Social/Gear tabs; resource strip from economy read model; virtualize the Work table; no double-click camera centre, status icons or content names; §59 DoD unmet.

## UI-005-soil-hover-panel
Soil hover panel · 2026-10-06 · 6b27621fa
- Changed: hovering a tile shows a bottom-right panel (above the colonist bar): soil label, fertility %, farmability ("Not farmable", "Impassable, mine to clear", "Crops grow X% faster or slower"), plus room/plant/stack lines. Read model `game.Soils() []SoilView{Name, Label, Fertility, Farmable, Walkable}` indexed by `WorldView.Terrain`.
- Decisions: fertility Rich soil 140 %, Soil 100 %, Gravel 70 % (all farmable); Rough stone 0 %, Rock wall 0 % (impassable). Capture `-capture-ui hover`; test `game/soil_test.go`.

## UI-006-camera-bounds
Camera bounds · 2026-10-06 · 233f6db20
- Changed: viewport = screen minus top bar and colonist bar/tab strip (`colonyViewport`). Max zoom-out `minZoomFor`: whole map with an 8 % margin, but the map covers at least 60 % of the viewport's wider side (`minCover`). Pan limit `clampCamera`/`clampAxis`: each edge may be pulled 18 % of the viewport inside (`edgeSlack`); an axis smaller than the viewport is centred; first frame centres. Zoom multiplicative (10 % per step), wheel around the cursor, +/- around centre. Alert jumps are clamped. Captures skip camera input. New captures `zoomout`, `zoomedge`.

## UI-007-order-queue
Per-colonist order queue · 2026-10-06 · 5d98470c9
- Changed: Shift on an order-menu entry (any order) queues it behind the current one; drafted Shift+right-click queues a move; no Shift runs at once and clears the queue. Footer "Shift: queue (max 3)". `command.Command.Queue`, `command.ClearOrders`; `g.orderQueue[id]` holds up to `MaxQueuedOrders` (3, with menu labels); full queue drops with "Ada already has 3 orders queued". Read model `PawnView.QueuedOrders{Label,X,Y}`, `HasOrder/OrderX/OrderY`.
- Decisions: `phaseAI` starts the next queued order before drafting and autonomy; queued orders don't start in sleep hours unless drafted. `execOrder` re-validates via `forcedJob`/reachability and skips with `Ada skipped "Equip rifle": no longer there`. `cmdMove` now marks moves Forced. New `ReasonGone` ("no longer there") replaces "unknown building".
- UI: "Queued orders n/3" panel with Clear; dashed route on the map through numbered orders; temperament next to the name. Capture `-capture-ui queue`.

## UI-008-work-priorities-and-schedules
Work priorities and schedules · 2026-10-06 · 73096ba7f
- Changed: `work.Priorities` per work type (0 off, 1 first .. 4 last; `DefaultPriorities()` 3; `Mask()` derives the old enable set); `work.Order` breaks ties (Doctor, Haul, Construct, Cook, Craft, Hunt, Grow, Mine = old fixed order, default behaviour unchanged). `g.prio[id]` saved, `g.labor` kept in sync; `assignWorkByPriority` replaces the fixed chain in `phaseAI` via `assignWorkType`. Commands `SetWorkPriority{Pawn, Work, Count}`; `SetWorkEnabled` on = 3, off = 0. `PawnView.Priorities`. Work tab cells cycle off,1..4 with shading.
- Changed: per-colonist 24-hour grid `g.scheds[id]` (copy of colony default `g.sched`, saved); `activityOf(id)` replaces `Activity()` in `phaseAI` and `doSleep` (top bar still shows the default). Command `SetSchedule{Pawn, Index: hour, Mode}`; Pawn 0 paints everyone and the default for newcomers. `PawnView.Schedule/Activity`, `WorldView.Schedule`. Schedule tab (`schedule_ui.go`, `a.panelMode`="schedule"): four brushes, Everyone row, click or drag paint, current hour outlined. Captures `work`, `schedule`.

## UI-009-threat-pause-and-damage-numbers
Pause on threat, damage numbers · 2026-10-06 · 94efe8901
- Changed: `MessageView.Danger` marks threats (`g.threat(...)`: raids, wolf packs); `projectile.Trace/TraceView` carry `Damage` and `Building`. `watchThreats` (`threat_ui.go`) pauses on a danger message newer than the last handled one; messages present at session start or after load never pause; HUD reads "paused (threat)". Options toggle "Pause on threat" (`App.noThreatPause`). Floating "-N"/"miss" numbers rise and fade over 50 frames, scale with zoom; building hits amber with wood chips. Capture `hits`.

## UI-010-roof-overlay-and-tools
Roof overlay and roof tools · 2026-10-20 · 9d64cde88
- Changed: Architect Structure tools "Build roof" (`DesignateRoof`, icon `roof_build`; "A roof needs a wall within 6 tiles; closed rooms are roofed automatically") and "Remove roof" (`DesignateRemoveRoof`); holding either forces the Roof overlay (`roofOverlayForced`); Build-roof ghost marks unsupported tiles red with "x". Roof overlay on R (moved to Shift+R in UI-016) or the HUD "Roof" button (`hud.roof`, icon `overlay_roof`): Built blue with lighter edge facing unroofed, Natural slate stippled, PlanBuild amber dashes, PlanRemove red hatch; legend tab under the top bar.
- Changed: hover roof line by priority: "Roof removal planned", "Roof planned", "Natural roof (rock)", "Roofed", "Unroofed (room too large to auto-roof)", "Unroofed"; walls none; room line adds ", roofed N%". Spec: MAIN_UI overlays paragraph, ART §16.1.
- Open: R toggled inline in `renderer.go` rather than `toggleRoofOverlay`; no other overlays share the button strip.

## UI-011-inspectors-move-grass-trees
Inspectors, paused actions, move/copy, grass, tree zones · 2026-10-20 · 4df64ab35
- Changed: paused actions apply at once (`Game.ApplyPending`, via consumer-owned `pendingApplier`; state equals the next `Step`, `TestApplyPendingMatchesStep`). `Undesignate` clears one mark (before, cancelling used `DesignateCancel`, which also deleted pens/zones/stockpiles on the tile). Selection precedence pawn, animal, ground item, building, plant, floor; same tile cycles. New plant, item and pawn-action panels (per-colonist chips from `OrdersAt` with reasons, Equip nearby).
- Changed: building Move (`MoveBuilding` places a reinstall frame `Frame.Reinstall`: no materials, `max(60, work/2)`; the same entity moves, keeping bills, fuel, contents, owners, HP, ID; saved with the frame) and Copy. Not movable: walls, doors, floors, graves (fences/gates later, UI-022). Ghost validated by `CanMoveTo`, Q/E rotate, right click/Esc cancels.
- Changed: grass is simulated (`plant.GrassAt`, `game/grass.go`), seeded from terrain so maps look the same; removed by building/floor/sowing/terrain change; cut-plants marks grass (work 25); saved, older saves reseed bare under buildings. Tree zones: Tree is `Sowable`; growers plant saplings, chop grown zone trees, replant; wild trees still need Cut.
- Perf: ColonyHour 17.5 ms, ColonyRaid 20.5 ms (unchanged); coverage game 86.8 %, plant 83 %.
- Open: `CanMoveTo` repeats `placement.go` rules (can drift); Cancel move cancels the destination origin tile (could remove a zone); grass never regrows; trees take a day to grow; plant panel shows at most 3 colonist chips per row; Social tab label overflow; no forbid, set-owner or multi-selection (multi-selection came in UI-016, forbid later).

## UI-012-options-and-settings
Options and settings · 2026-10-21 · 5f0d1f6df
- Changed: credits open with "Created by arxdsilva"; Options has Audio, Graphics, Gameplay, Accessibility tabs, every setting applied immediately and saved to `<user config dir>/rimworld-go/settings.json` (new `internal/platform/config` `Settings`, `Normalize`, `Decode/Encode`; `outbound/settingsfile` atomic store, corrupt file kept as `.bad`). `AppConfig.Settings` (zero = defaults, nothing saved), `AppConfig.Window(fullscreen, vsync)` (nil leaves the window). Settings affect effects detail, edge scrolling, pan speed, reduced flashing, autosave interval, menu diorama. Captures `credits|options|options-graphics|options-gameplay|options-access`.
- Decisions: settings are presentation-only; captures ignore saved settings; stepper is -/bar/+ buttons (debug font, hand layouts). Deferred with reasons in MAIN_UI §42: UI scale and text size (done UI-024), resolution presets, frame cap (Ebitengine has only VSync), colour-vision support, tooltip delay, far-zoom detail, Controls (done UI-016). No determinism or save impact.

## UI-013-paths-draft-logs
Paths, draft orders, logs · 2026-10-21 · 01407f161
- Changed: every colonist's remaining walk is drawn cell by cell with a destination ring and a drop-point square for hauls; drafted routes red, thick, solid; the selected one white on top; zoomed far out only rings. `PawnView.Path`, `HasGoal/GoalX/GoalY`, `HasDrop/DropX/DropY` (derived from saved job state). A drafted colonist right-clicked onto a raider or animal opens the Shoot/Melee menu; ground still moves at once. Colonist Log tab (wrapped, scrollable); animal inspector taller with 4 Recent entries and a Log/Less toggle; disabled inspector tabs no longer print "(disabled)" (reason on hover). Captures `zones|paths|log` (ZONE-001 `zones_draw.go`). Presentation only.

## UI-014-biome-picker-wildlife-tab
Biome picker, Wildlife tab, animal inspector · 2026-10-22 · 04d46d084
- Changed: new-game wizard gets a seed field (digits, Dice) and biome picker cards (icon, temperatures, growing days, ground, plants, top wildlife); Generate enabled; top bar shows the biome. Animals tab Colony/Wild toggle with Hunt all, Tame one, Cancel (species, count, diet, predator, tame %, meat); Work tab Handle column. Animal inspector (state, food/rest bars, sex, stage, age, pregnancy %, product bar, diet, tame chance); trough panel (stock, feed filter). Alerts: hungry animals, wild animals eating crops, timbergnaws eating trees, pen full. Juvenile scaling, rest pose with snore Zs, biome grass tint. Read models only.
- Open: no world-map tile picker (spec §6/§7 partial); wild list lacks sort/filter, hunt-risk, body size; colony table lacks health, master, area, training; no pen capacity readout beyond the alert.

## UI-015-frame-inspector-carcass-art
Frame inspector, carcass art, materials alert · 2026-10-22 · fc49cab16
- Changed: carcass sprite bug fixed (`itemSpriteSpecies` maps every species; `PawnView.CarryingAnimal`; regression test through real `World()`). Frame inspector: status (reinstall, no stock with craft hint, materials on the way, waiting for hauler/builder, built by X), materials delivered/needed/in stock, work done/total with minutes, haulers, builder, footprint; `CancelFrame` refunds delivered materials (Cancel area tool too). Carpet card shows stock and where to make one. New alert "Blueprints waiting for materials (N)".
- Open: the materials alert fires immediately, not after an in-game hour as planned.

## UI-016-input-system
Input system · 2026-10-22 · 28fbc8aea
- Changed: one input layer. `internal/platform/input` is a pure action registry (context, kind, default keys, conflict checks; action ids are the stored keys, never rename). Each `App.Update` polls once (`input_source.go` holds the only Ebitengine input reads); frame resolved through a context stack text > modal > tool > panel > map > global > dev. Options > Controls rebinds, saved in `settings.json` (`Settings.Controls{Bindings}` overrides only, `Gameplay.DevMode` toggle in Gameplay; old files decode over defaults; `Settings` no longer `==`-comparable).
- Changed: Shift+click/box/double-click multi-selection with group panel; R/F/right-click act on the selection; formation moves (groups take nearest free walkable cells by Chebyshev rings; right-drag spreads along a line; greedy nearest-first, ties by id); `,` `.` cycle colonists; Numpad * cycles a cell; F1-F6 tabs (F3 Assign, F7-F9 reserved ones toast "not implemented yet"); Home/End speed; PgUp/PgDn zoom; middle-drag pan; Shift pans faster; Z search; F10 screenshot; F11 hide UI; G room stats; Alt cell detail; Architect draw styles (Q/E shape, Ctrl square); bill quantity Ctrl x10, Shift x100, Ctrl+Shift x1000; gizmo keys B H Y N J U M K I L O P; dev keys. Ovals send one rectangle command per row run.
- Decisions: R drafts; roof overlay moved to Shift+R. Esc while editing the seed commits it. Modifier actions (fast pan, add to selection, queue, constrain, detail, quantity) are fixed, not rebindable, and left out of conflicts. Text context swallows every key; a modal switches map keys off. Debug actions call `game.Game` hooks via the consumer-owned `devTools` interface only in developer mode (the one exception to "commands only"); dev step = `stepTicks` x `Step(1)` while paused; ultrafast = 15 ticks per frame (key 4).
- Fixed: R both drafted and toggled roof; right-click leaving a tool also ordered the colonist (`TestRightClickLeavingToolDoesNotOrder`); Esc in Move mode also deselected (`TestEscInMoveModeHandledOnce`).
- Open: Heal in the debug menu (no hook; "Fill needs" instead); control groups; beauty overlay, power toggle, Assign/Quests/World/History tabs reserved; shared overlay registry; debug inspector lacks components/path/reservations; item search skips storage contents. No determinism impact.

## UI-017-link-grid-smooth-tool-stone-readouts
Link grid, Smooth tool, stone readouts · 2026-10-22 · f7bce7011
- Changed: `linkgrid.go` presentation-only connectivity rebuilt on `StructVer` (rock = `Rock|MapEdge`, walls/doors `Wall|Rock`, fences `Fence` accepting `Fence|Wall`); walls now join natural rock and rock links past the map edge. Rock/terrain draw only visible tiles; stone tinted by cell stone or material. Smooth tool and Stonecutter card, `designation_smooth` overlay and animation; HUD shows Stone, Steel (Silver, Gold when above 0); material buttons ("Granite blocks"), titles ("Granite wall"); hover uses `game.SoilLabel` plus "Smoothable". Mine/smooth sounds only when terrain changed; cancelling no longer plays `rock_break`.
- Perf: `BenchmarkDrawArt` 3.5 ms / 13,049 allocs -> 1.7-1.8 ms / 6,154; `DrawArtMountain` 1.3-1.45 ms. Capture `-scenario walls`.
- Open: hover says "Smoothable" on rough stone with a floor on it (rejected); stone-block builds show stock on the info line, not a low-stock warning.

## UI-020-new-game-flow
New-game flow, widget kit, real font, in-colony additions (NG-1) · 2026-10-22 · f33df2d86
- Changed: plan `.agents/handoffs/NG.md#ng1_new_game_plan`. Immediate-mode widget kit (`ebiten/ui_*.go`: button, checkbox/radio, slider, text field, virtualised scroll, tooltip rests 24 frames, modal stack `CtxModal`, sortable table, tabs, progress/sparkline, layout helpers); screens declare via `registerScreenUI`, state in `a.ui`; Tab/Shift+Tab focus; Esc order text edit, top modal, Back.
- Changed: real font Noto Sans (OFL 1.1) Regular/SemiBold via `text/v2` (`font.go`; scale 11/13/13b/16b/22b/32b); layouts measure/wrap/ellipsize; UI scale 90/100/115/130 % (later UI-024). Wizard over `setup_backdrop.go`; draft state `newgame_state.go` becomes `game.Setup` only on Start. Screens: Scenario (four built-ins Broken Hull, Ember Clan, Gilded Expedition, Bare Hands + custom JSON files), Scenario Editor (14 part kinds, validation blocks Done on errors), Storyteller (six difficulty presets + Custom, save mode Reload anytime or Commitment), Custom Difficulty modal, Create World (original two-word seed phrases, coverage 30/50/100 %, 5 % dev, faction table, progress modal stepping `planet.Builder`), Landing (rotating hex globe, tile panel, Advanced, Factions dialog; globe 0.3-0.5 ms/frame, `BenchmarkGlobeFrame`), Characters (backed by `game.CandidatePool`, SIM-032), loading (`game.ColonyBuilder`).
- Decisions: map sizes from `game.MapSizes()` gated by `game.MaxEnabledSide` = 500 after the ENG-001 gate; 250 default; 350+ "needs a capable computer". Commitment mode (chosen at storyteller, fixed): one slot tagged "Commitment", saves at day rollover/menu/quit/window close, pause offers Save & Quit to Menu, Load disabled ("Commitment: one timeline"), developer mode locked. Big maps: `lod.go`/`chunk_cache.go` tiers (live >=48 px, 16 px baked chunks, 4 px whole-map bake), ENG-001.
- Changed (colony): Factions window (goodwill sparkline), Bio tab, Work tab incapable cells, Options > Story (`SetStoryteller`, `SetDifficulty`), Letters (`letters.go`), condition chips, minimap for maps over 150 tiles (Shift+M), Hospital bed/Tool cabinet cards, scenario-forbidden buildings disabled with reason, setup sounds. Many `-capture-ui` states (scenario ... chips).
- Open: no `pause-commitment`/`minimap` capture; landing has no feature search; no relationships in character details; Custom Difficulty some sliders lack tooltips and Disease frequency says "No diseases yet"; text-field copy/paste depends on Ebiten; no world map in colony; README gallery for the new flow; `BenchmarkHUDText` not added.

## UI-021-door-over-wall
Door over wall (PLAY-010) · 2026-10-23 · cf43235d7
- Changed: a door or fence gate may be placed over a wall, fence, door or gate of a different kind: the old piece is marked for deconstruction and the new frame waits ("Waiting for the wall to be removed"; forced orders refused "The wall must come down first"; haulers may still deliver). Same kind over same kind, walls over doors, floors/carpets stay refused. Cancelling the frame clears the mark and vice versa. Ghost reads "replaces wall" (`ReplacesAt` on the renderer's `Simulation`). Rooms merge while the cell is open, re-split when the door is built.
- Save: `construction.Frame` gains `Replaces`, `ReplaceMarked` (gob, additive).

## UI-022-orientation-and-material-swap
Door orientation and material swap (PLAY-011) · 2026-10-24 · b6227357b
- Changed: doors and gates face along their walls/fences (`autoFacing` counts solid neighbours incl. planned and natural rock; tie keeps the player's Q/E or the current facing; reruns on neighbour change via O(1) `reorientAround`). Renderer draws the stored facing (old wall-mask guess gone); ghost previews per-cell facing (`AutoFacingAt`). Save `DoorsOriented` migrates older saves.
- Changed: same piece on the same tile in a different material swaps in place (walls, doors, fences, floors, carpets, sandbags, beds, tables, chairs): frame with `Swaps`, old piece keeps standing (rooms stay sealed), on completion same building gets the new material at full HP and the old material is refunded; same material is `ReasonSame` (grey ghost, nothing queued); pending swap blocks move (`ReasonSwapPending`). Fences and gates can no longer be moved. Hover "Changing to X (N%)".
- Decisions: removing one wall of a door's run can flip it (the rule, not flapping). Goldens unchanged; format flag not hashed.
- Open: `TestAtlasCoversKeys` failed in the working tree from ENG-003 face-scar pawn sprites re-rendered without re-packing the pawn atlas (unrelated).

## UI-022-social-tab-history
Social tab, health, assign, history UI (ENG-003 M4) · 2026-10-24 · 141eccc86
- Changed: read-only surfaces over SIM-039/040/041: Social tab (relations with perspective labels, A->B opinion with (B->A) beside it, 5-band colour, status tags, hover breakdown, social log with icons), interaction bubbles and fight marker, History tab (F9, click-to-select), Bio relationship summary, Health tab (capacities, pain, parts, operations queue add/remove), Assign tab (F3) drug policy selector.
- Open: no other editing controls; no ex/lover rows until M5; no history filters; wire the six unwired order icons and the face-scar overlay (ART-019).

## UI-023-plan-tool
Plan tool (PLAY-012) · 2026-10-24 · 556c3d38c
- Changed: Architect > Orders "Plan"/"Remove plan" (rectangle or oval) paint visual layout marks: nobody works on them, they block nothing and change no room, path or zone. A mark vanishes when any building is finished or moved onto the tile (floors and in-place swaps leave it). Overlay (soft blue fill, dashed edges) on by default; HUD "Plans" hides it, holding a plan tool shows it; hover reads "Planned". Command `DesignatePlan`; `WorldView.Plan/PlanVer`; sparse save `Plan []int32` (nil without marks, goldens unchanged); icons `plan`, `plan_clear`; palette `ui.plan_fill/plan_edge`.

## UI-023-social-finish
Social finish: animal layout, dev hooks, scenario · 2026-10-24 · 5791b03b8
- Changed: animal inspector log rect subtracts the bond block (`animalExtraH + animalBondH`); training buttons 134 px, two per row; Log button 48 px (`TestAnimalInspectorNoOverlap`). Social relation label column 92 px; prisoner "Reduce resistance" reads "Reduce" (tooltip keeps full text).
- Changed: dev hooks `game/dev_social.go` (bump viewGen, tested, unused by goldens): `DevSetPair`, `DevAddOpinionMemory`, `DevBond`, `DevScar`. `-scenario social` (Maren/Tobias married, Eli/Fen engaged, Hale/Iris rivals, Biscuit bonded, Cyrus jaw scar; `TestSocialScenario`). Capture states social, health, prisoner, history, world, assign, inspector (`capture_social.go`).
- Open: social scenario has no world map ("no world map" in World window); Assign Area column disabled until areas exist (done UI-025).

## UI-024-ui-scaling
UI scaling (PLAY-012) · 2026-10-24 · 556c3d38c
- Changed: UI lays out in logical pixels and draws through a scaling canvas (rects, lines, images, triangles multiplied; text shaped at real size). `LayoutF` renders at true resolution on HiDPI; the map keeps its apparent size (tile = TileSize x zoom x device scale). Scale = device scale x UI scale; Auto = `min(w/1280, h/720)` clamped 1-2, rounded down to 5 %; presets 75-200 %. Options > Accessibility "UI scale", Graphics "Sharp on high-density screens"; saved (settings v2; old files load Auto + HiDPI on). Captures force scale 1. Input: `currentCursor()` logical, `mapCursor()` physical. `cmd/game -hidpi` dev flag.
- Guard: `TestUIDrawsThroughCanvas` fails if a file draws raw outside a reasoned exemption list.
- Perf: 500x500 grown colony, z0.25, Retina: HiDPI on (2560x1440) 120 fps avg, 1 % low ~111; off 120 / ~110 (vsync-capped).
- Open: spacing/row heights are per-file logical constants (central token set is a follow-up); `TestAtlasCoversKeys` fails only with stale git-ignored pawn PNGs.

## UI-025-areas-pens-fire-ui
Areas, pens, fire, snow UI (PLAY-015 UI) · 2026-10-24 · 0b1389229
- Changed: area manager modal (swatch, rename, Invert, Delete confirm, New off at 10, Close); Architect "Expand/Clear allowed area" submenus; tinted overlay for the armed/hovered area; Schedule tab "Allowed area" dropdown and pawn "Area:" line; Assign tab lost its Area column. Animals tab Master dropdown, Follow toggle, Area/Pen column. Pen marker inspector (Animals, Food, Auto-cut), outline when selected, open pens dashed red. Home "Auto" toggle; Zone tools renamed/moved.
- Changed: fire (flames by size at 8 fps, embers, smoke, orange light, one positioned `amb.fire` per cluster, flash + `sfx.lightning_strike` + thunder, Extinguish right-click entry, Work tab Firefight column, dev "Start fire here", ash decals); snow per-cell veil and cyan snow-clear overlay. Art ART-022; sounds `amb.fire`, `sfx.lightning_strike`.
- Limits: submenus and dropdowns are small centred modals (no popup primitive); sound budget now 19.7 of 20 MB.

## UI-026-play017-scale-fixes
Scale fixes, zone labels, hauling, chase UI (PLAY-017) · 2026-10-25 · 7b4dc6abd
- Changed: portraits draw through `uiDrawImage`; world-anchored labels via `worldText`/`worldIcon` (zone labels, tame names, chat/fight/draft icons; wind arrow `uiLine`). Zone labels (`zoneLabelLayout`) centred on a plate, size 0.45 x shorter side, 11 px up to `zoneLabelCap(tile)` (16 zoomed in, up to 40 far out). Hauling: Orders > Haul things, `overlay/designation_haul`, `ui_icon/order_haul`, item panel "Found here" + Mark for hauling. Chase: red danger messages, hostile alert names who is chased, red marker + dotted chase line, `onThreatPause` centres an off-screen chase. Options > Gameplay "Run in background" (`background.go`); double-click a bar colonist centres the camera (`centreOnPawn`). Plant growth caption "Ripe in about N days"; Gear tab "(you chose this)", "Changing into".
- Guards: `TestUIDrawsThroughCanvas` is go/ast with `file:func` exemptions (world-pixel helpers need `worldText`/`worldIcon` or `uiScale()`); `drawTrace` beside `canvasTrace`; `TestFrameInvariance`, `TestPanelsStayInTheirPanel` at UI scales 1-2 x device 1-2 (~14 s `-race`); `-capture-ui-scale s`.
- Open: up close a small stockpile's name overlaps its counts; raw draws invisible to the trace (AST guard covers them).

## UI-027-play018-hud-relayout
HUD relayout and learning helper (PLAY-018) · 2026-10-25 · f37ee3243
- Decisions (user, 2026-10-25): familiar colony-sim arrangement on our own grid, no other game's numbers or colours; a new original charcoal palette (slate with sea-glass accent; tests pin text contrast and a minimum distance from a reference set of another game's UI colours); a learning helper; art agents always on the best model (PARALLEL_WORK §6).
- Changed: `computeHUDLayout` (`hud_layout.go`) is a pure function placing every region in logical px; map fills the screen, HUD floats. Regions: Resources top-left (`WorldView.Stored`, collapsible), colonist bar top-centre (scales 1/0.75/0.5, 3 rows then "+N"), right column (learning helper, alerts, letter stack, minimap, lower-right global stack of 8 toggles + 4 speeds + date, weather, temperature), messages, lower-left inspect pane with tabs, 64x64 command buttons (hotkeys by order; groups merge by key), Architect, windows above 11 tabs (Wildlife new), cell info.
- Changed (game): `WorldView.Letters` saved, capped 40, `DismissLetter`; every major threat is a red letter; `WorldView.Stored`, `item.Category()`; `Simulation.TemperatureAt`. Command ids: legacy ids carry no Run and go through `applyClick`; new ones `cmd.<kind>.<name>`. Learning helper (`learn_ui.go`, `lessons.go`): 10 lessons done by the player's own commands or "Got it", folds when cramped, progress in settings (v4). 25 new icons.
- Fixed: Plans/Rooms toggles shared a rect; clicks fell through animal/item/plant panels; hidden Work cells under kit windows; `hud_windows.go` built on Windows only.
- Tests: `hud_layout_test.go` (no overlaps at 3 sizes x 3 UI scales, 6 or 30 colonists); AST draw guard covers `hud_*.go`/`cmds_*.go`. fps probe 119.8 avg / 99.9 1 % low (was 119.4 / 93.8); game package ~3 min under `-race`.
- Open: with nothing stored the resource list falls back to all loose stacks; messages age while hidden; no inspired/caravan mark on the bar; drag-to-reorder bar (done CTL-001).

## UI-028-play020-zones-beds-lamps
Zones, beds, lamps, bed rest (PLAY-020) · 2026-10-25 · 327fe63f2
- Changed (user, 2026-10-25): chat bubble is the round interaction icon with no box (plain chat = chit-chat icon; without art a dark disc with dots; guard exemption removed). Growing-zone wash/stripes skip cells holding the zone's own plants. Lamps (`light_sources.go`, `light_lamp.go`): point lights with falloff over metadata `light_radius` within the room or line of sight outdoors; `LampSky` fades glow by day; roofed rooms keep lamp shadows at noon; one helper lists light sources (PLAY-021 gates on power). Menu scene: roofed house, Blender point light at the bulb, additive `stage.py` helpers. A Sow job claims its tile (one grower per cell; forced sow on a held cell refused; "sowing is off in this zone").
- Changed: bed rest (`bedrest.go`) starts at healable injury >= 40, untended wound, sick, or awaiting surgery; stops at <= 15, nothing to tend, not sick, and >= 2 h; no free bed retries after 6 h (those waiting for a doctor lie on the floor); self-tend in bed when no other doctor; hungry patients eat and return (no per-tick re-bedding). "Rest when hurt" per colonist, default on. Medical beds (`SetBedMedical`): no owner, healthy colonists never use, patients/rescuers prefer; hospital beds start medical; save v4 `migrateSave(v)`; UI "Medical" toggle beside "Prisoners".
- Changed: zone management: zone is the last selection candidate on its tile (label selects it); pane (size, crop, growing now/too cold/too hot, sown, ripe, growth); Plant, Allow sowing/cutting, Expand/Shrink (drag-rect for that zone until right-click/Esc), Delete, Hide, Rename. Sim `zone_cmds.go`, `zone.Map.AddCells/RemoveCells/Remove/Update`; settings never affect merging; zero values keep state hashes.
- Decisions: goldens (48x40 seeds 1 and 42, 120x120 seed 7, grown) re-pinned because wounded colonists now rest until tended. `cmd/rambudget` gained a `capture` kind (2.2 GiB). Captures `zone`, `zone-shrink`. Fixed: floating cell info covered a selected zone's pane.
- Open: double-click select all similar zones in view; crop selector columns (skill, growth time, yield); lamp pools step per tile; overlay ash PNGs differ from source (pre-existing drift); electricity PLAY-021 next (torch lamp before it).

## UIS-gap-analysis
UI size and scale gap analysis · 2026-11-06 · bd8480c26
- Changed: `.agents/gaps/UI_SCALE_GAP_ANALYSIS.md`, 92 rows comparing the reference game's logical sizes at 1x with ours: scale steps and the minimum logical size, type sizes, colonist bar, messages, alerts, letters, tab bar, inspect pane, commands, Architect, lower-right stack, main-tab windows, float menu, tooltips, Options, zoom range and map labels. `CLEAN_ROOM.md` row UIS-* (sizes, steps and zoom range may be measured; art, icons, colours, fonts, text and code stay ours; screenshots never enter the repo).
- Method: the lead drove the reference game (1.6.4871, Core, dev mode) through Steam with `-savedatafolder` on a scratch folder; the user's config hashes were unchanged after. 71 shots at 1920×1080 (1x, 1.25x), 1280×720, 5120×2762 at 3x and two minimum-size windows; five read-only agents measured each value twice; the lead re-measured 10 values, all within ±2.
- Decisions (user, 2026-11-06): same logical sizes (our 100 % is the reference 1x, same nine steps 1–4); Auto stays as our extra with a new rule (largest step keeping 1280×900 physical px per step); decompiled constants banned.
- Findings: our Auto gives 1.5 at 1080p, so the whole HUD is 1.5× the reference; the reference default zoom is 48 cells of window height (22.5 px per cell at 1080, ours 64); commands past 4 rows are silently dropped today (UIS-052); no pawn name labels on the map (UIS-088).
- Not measured: 1.75x rendering, message stacks, 20+ commands, work bars, tame animals, many-colonist rows between 17 and 39; UIS-3 captures them.
- No HUD/UI representation: analysis only.
- Open (the user's): default scale (UIS-006), step unit physical vs points (UIS-002), map rebase to 48 cells (UIS-080, UIS-086). Implementation: UIS-1 contract by the lead, then UIS-2a–f in parallel, then UIS-3; the Quests tab and window (UIS-041 part, UIS-066) wait for a quests feature.

## UIS-implementation
Gameplay UI sizes and scaling to the reference · 2026-11-07 · 2687260c0 (UIS-1), 2dd27b5fa, 27578b9b4, 729d4f688, 1b9787ecd, 049a90108, 8a6be3f1d (UIS-2a–f), 891cdbfda (UIS-3)
- Changed: the nine reference steps 1×–4× counted in physical px per logical px; Auto (default) = the largest step leaving 1280×900, a step refused below 1280×768 logical; settings v11 migrates the old percentages. Type scale Small 11/15, Body 14/16, Title 21/24. Every HUD and window size is a `hudlayout/metrics.go` token pinned to its row. The map cell is the window's height / 48 (zoom 0.40–2.18), with colonist names on the map and count tags under their cell. Commands never drop: the last cell opens the rest. Captures now sync the cell too.
- HUD/UI: the Options UI scale row lists Auto and the steps, names the step in use ("Auto (1.5×)", "2× (using 1.5×)") and asks to keep a new step for 10 s.
- Decisions: command rows are `CmdRowPitch` 90 apart (ours: no reference shot has two rows), so a label hanging `CmdLabelBand` under its cell never lies on the next row or a side panel. A zoom key press is 4 wheel notches (ours). A message row lying over a bar slot is drawn on top and takes the click. Count tags, the work bar and the LOD tiers use map px, as the reference does, so a Retina window shows counts at the default zoom. Setup screens keep Large and Huge.
- Save impact: none in the simulation; the view sidecar gains `ViewVersion` 1 (the zoom rebases on load).
- Perf: `DrawArt500/z1` now draws a 48-cell-high view (≈3.5 ms, 1102 draw calls, 9969 allocs). At the old 64 px cell it is 0.98 ms, 409 draw calls, 3835 allocs (was 3745): the +90 are colonist names, allocated inside ebiten's `text.Draw`.
- Tests: `TestFrameInvariance` zooms in to 2 for its second zoom (the 32×32 map is held at zoom 1) and checks zone names from 30 logical px cells.
- Open: UIS-008 step-list cell width; UIS-075 Options close X; UIS-032 letter hover card; UIS-073 frameless letter (`RunModals` ignores `Frameless`); UIS-061/062/064/065/068 window details; UIS-041/066 Quests. Reference captures still owed (17–39 colonists, 20+ commands, mid-job pawn, tame animal, 1.75×, message stacks): a later UIS pass. `env.DecalMinTile` is still 32.
