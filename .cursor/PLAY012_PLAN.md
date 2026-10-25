# PLAY-012: Plan tool, and crisp UI scaling (Auto + presets, HiDPI)

## Context
You asked for two things:
1. **A Plan tool:** purely visual placeholder marks you paint on tiles to sketch a layout. Nobody works on them and they block nothing. A mark disappears by itself once a building is finished on its tile. Your choice: **any building** (wall, door, fence, furniture…) clears it; floors don't.
2. **UI that scales with the screen.** Your choices:
   - **Crisp native scaling:** text and chrome are redrawn at the real size, not stretched.
   - **Real HiDPI:** Retina and other dense screens render at their true resolution.
   - Options offers **Auto + presets**.

**What exists today** (from exploration):
- **Home area** (`internal/application/game/home.go`) is the closest template for a per-tile paint layer: `homeState{cells []uint8; ver uint32}`, `paintHome`, `fillHomeView` with `shareLayer` (copy-on-write), `exportHome`/`importHome`, command `DesignateHome` with `Enabled` for paint/clear, the `home`/`unhome` Architect tool pair (`archtools.go:73-76`), a cached overlay (`home_draw.go`), the overlay button and hold-to-show (`hud.go:575-625`), and its tests (`home_test.go`, `home_carpet_ui_test.go`, `home_draw_test.go`).
- **Buildings complete in `completeFrame`** (`tick.go:~1260`). It already clears grass under the footprint, and `autoHome` is a per-building side effect. Material swaps finish in `completeSwap` and moves in `completeMove`; both run outside that path.
- **UI sizing:** every HUD and menu size is a fixed pixel count, about 230 layout constants and 250 `rect{}`/105 `button{}` literals in ~50 files. There are 114 direct `vector.FillRect`/`StrokeRect` calls and 38 `drawPanel` calls.
  - Text goes through `font.go` (`drawText`, `measure`, `lineH`, `wrapPx`, `ellipsize`). A global `uiScale` (0.9–1.3) only resizes fonts and is never applied: `setUIScale` is called only from a test.
  - `config.Settings.Accessibility.UIScale` exists (90–130, default 100), but there is no Options row; the Accessibility note says "planned".
- **Layout** returns the window size 1:1 (`app.go:923-933`), with no HiDPI anywhere (no `LayoutF` or `DeviceScaleFactor`).
- **Input:** the cursor is read once (`input_source.go:65`) into `rawInput.mx/my`. Widgets hit-test in the coordinates they were drawn in (`layoutHits`, `rect.hit`). The map uses `screenToWorld` on raw coordinates.
- **Captures** (`-capture`, `-capture-ui`) run with settings hooks off and must stay deterministic. Several tests draw setup screens at 1280×720 and 1920×1080, and `TestSetupMinWindow` checks the minimum window.

---

## Part A: Plan tool

### A.1 Simulation (`internal/application/game`, new `plan.go`, mirrors `home.go`)
- **State:** `planState{cells []uint8; ver uint32}` on the Game (field `planMarks`).
  - `cells` is allocated on the first paint (nil until then) and set to 1 per marked tile.
  - `ver` increases on every real change.
- **Command:** `command.DesignatePlan` is appended to the command enum after the last kind (append-only; the kind is saved in `Pending`). It carries `X, Y, X2, Y2, Enabled` (true = paint, false = erase), like `DesignateHome`. Its dispatch case goes beside `DesignateHome` in `applyOne` (`tick.go:~156`).
  - `paintPlan(x, y, x2, y2, on)` normalises with `zone.Normalize`, clips to the map, and bumps `ver` only when a cell changes.
- **No gameplay effect:** no job giver, reservation, pathing cost or room effect reads plan marks.
- **Auto-clear:**
  - In `completeFrame`'s footprint loop (next to `clearGrass`), clear the plan mark on every footprint cell when the completed kind is **not** on the floor layer (`construction.DefOf(k).Layer != LayerFloor`).
  - `completeMove` clears marks under the moved building's new footprint.
  - `completeSwap` (in-place material change) already stands on its tile, so it changes nothing.
  - Instant markings that complete through `completeFrame` (graves, carpets?) follow the same floor-layer rule.
- **Read model:** `WorldView.Plan []uint8` and `PlanVer uint32`, filled by `fillPlanView` through `shareLayer(&g.viewPlan, …)`. The new layer cache sits beside `viewHome`. When nothing is marked, the view carries an empty slice.
- **Save:** `saveState.Plan []int32` holds the **sparse sorted list of marked tile indexes** (gob, additive): nil when no tile is marked, so the existing golden pins don't move. Import rebuilds the cells. Old saves have no marks.
- **State hash:** the plan is player state and is hashed as saved. An empty list is a zero value, so goldens hold.
- **Tests (`plan_test.go`, red first):**
  - painting and erasing a rectangle, and an oval via row runs;
  - clamping at the map edges;
  - `ver` changes only on a real change;
  - the view aliases with no copy when unchanged;
  - a finished wall clears its tile, and a 2×2 bed clears all four tiles;
  - a finished floor does **not** clear a mark;
  - a material swap leaves marks unchanged;
  - a move clears marks under the new spot;
  - marks never block placement, pathing or zones;
  - a save round trip, and an old save loads with no marks;
  - goldens unchanged.

### A.2 UI (`internal/adapters/inbound/ebiten`)
- **Architect tools:** `plan` ("Plan", `enable: true`) and `unplan` ("Remove plan") under **Orders**, `mode: placeArea` with the filled-rectangle and filled-oval draw styles, `cmd: command.DesignatePlan`. `submitArchitect` already sends one command per rectangle or row run.
- **Icons:** new `ui_icon/plan` and `ui_icon/plan_clear`, made by the overlay family. Original art: a dotted square with a pencil, and the same crossed out. Re-render overlay icons; the orphan check must stay clean.
- **Overlay (`plan_draw.go`, copy of `home_draw.go`):**
  - One triangle batch of soft blue-white fills plus dashed outline edges around each marked region. Tokens `plan_fill` #B8D4F0 at alpha 0x30 and `plan_edge` #D8E8FF at alpha 0x90 are added to `assets/palettes/ui.yaml` and the Go mirror in `widgets.go` (`TestUIPaletteMatchesYAML`).
  - It is cached by `PlanVer` and drawn in `drawArt` right after zones, under blueprints and objects, so walls built later cover the faded mark until it clears.
  - It is **always visible** (marks are only useful when seen). A HUD toggle "Plans" beside Home hides them; the Plan tools force them visible while held, like Home.
- **Hover readout:** "Planned" on a marked tile (`hud.go` cell readout, like `homeLabelAt`).
- **Tests:**
  - tools emit `DesignatePlan` with the right `Enabled`;
  - every tool is reachable from a category (`hud_test.go:228`);
  - every tool's icon exists (`art_test.go:276`);
  - the overlay is drawn only for marks, cached by version, and forced while a tool is held;
  - the toggle hides it;
  - the hover label.

---

## Part B: crisp UI scaling

### B.1 Model: two scales, one rule
- **`dev`:** the device pixel density, from `ebiten.Monitor().DeviceScaleFactor()` (1 on normal screens, 2 on Retina). It is used so the game renders at the screen's true resolution.
- **`ui`:** how big the interface looks. The total UI scale is `s = dev × ui`.
  - **Auto (default):** `ui = clamp(windowHeightInPoints / 720, 1.0, 2.0)`, rounded to the nearest 0.05. A 720p-tall window gives 100% and a 1080p window 150%. The UI keeps about the same proportion of the screen as the baseline layout.
  - **Presets:** 75, 90, 100, 115, 130, 150, 175 and 200 %.
- **Logical UI size:** `lw = physicalW / s`, `lh = physicalH / s`. The UI lays itself out in **logical pixels** exactly as today (every existing constant and test stays valid at s = 1) and is drawn at physical resolution through a scaling canvas.
- **The map** renders at physical resolution and multiplies its tile size by `dev` only. The world keeps its apparent size on Retina but gets sharper, and the UI scale never zooms the map.
- **Captures** (`cfg.Capture != nil`) force `dev = 1` and `ui = 1`, so captures stay byte-identical.

### B.2 Core (lead, Phase 0, new `ui_scale.go` + `font.go` + `app.go`)
- **`Layout` becomes `LayoutF`:**
  - `func (a *App) LayoutF(outsideW, outsideH float64) (float64, float64)` returns `outside × dev` (physical pixels) and records:
    - `a.pw, a.ph`: the physical size;
    - `a.s`: the UI scale;
    - `a.sw, a.sh`: the **logical** size, `ceil(pw/s)`, `ceil(ph/s)`.
  - Everything that reads `a.sw/a.sh` (layout, hit lists, tests) keeps working in logical pixels.
  - Ebiten's `Layout` stays implemented for older call paths and returns the same rounded values.
- **Scale state:** the per-App value `uiScaleState{dev, ui, s float64}` is mirrored into one package value `curUIScale` (read by the draw helpers). It is set once per frame on the main goroutine before drawing. Tests that need a scale use a helper that sets and restores it (not parallel). The existing global `uiScale` in `font.go` is removed and replaced by `curUIScale.s`.
- **Scaled drawing helpers** (`ui_canvas.go`). Each takes logical coordinates and draws at physical resolution:
  - `uiFillRect(dst, x, y, w, h, clr)`, `uiStrokeRect(dst, x, y, w, h, width, clr)`, `uiLine(dst, x0, y0, x1, y1, width, clr)`, `uiFillCircle` / `uiStrokeCircle` if used;
  - `uiImage(dst, img, x, y, w, h, opts)`, which draws an image into a logical rectangle and fixes the GeoM scale;
  - `uiImageOp(op *ebiten.DrawImageOptions)`, which appends `GeoM.Scale(s, s)` for code that builds its own GeoM;
  - `uiTriangles(dst, vs, is, img, opts)`, which scales vertex positions (sparklines, progress bars, the overlay batches drawn in UI space).
- **Text:**
  - `drawText`, `measure`, `lineH`, `wrapPx` and `ellipsize` keep logical inputs and outputs.
  - The face is created at `size × s` (cached per (size, bold, s)). It is drawn at `(x×s, y×s)` and its metrics are divided by `s`, so wrap and ellipsis decisions are the same at every scale and text is always rasterised at the real pixel size.
  - `debugPrintAt` and `drawLabel` already forward to `drawText`.
- **Panels and buttons:** `drawPanel`, `drawButton`, `drawLabel` and the widget-kit primitives (`ui_*.go`) call the helpers internally, so most screens become scaled through them.
- **Cursor:**
  - `pollInput` keeps `raw.mx/my` in physical pixels for the map (`screenToWorld`, globe dragging and picking, map selection, minimap clicks become UI space, see B.4).
  - It also sets `ui.mx/my = raw / s`, which is what the widget kit, `layoutHits`, `hitButton`, `currentCursor` and every HUD/menu hit test read.
  - `a.cursor` (captures/tests) is given in logical pixels.
- **Viewport:** `colonyViewport(sw, sh)` keeps returning the map area in **logical** pixels for UI layout. A new `colonyViewportPhysical()` (logical × s) feeds the renderer's culling, camera clamp and `screenToWorld`. The HUD bar heights then occupy `hudBarH × s` physical pixels, so the map's visible area matches the scaled chrome.
- **Settings** (`internal/platform/config/settings.go`):
  - `Accessibility.UIScale` keeps its meaning (percent) but widens to 75–200.
  - New `Accessibility.UIScaleAuto bool` (default true) and `Graphics.HiDPI bool` (default true: render at the screen's true resolution).
  - `Normalize`: a 0 scale means 100, and values clamp and snap to the presets.
  - Old settings files (`UIScale = 100`, no Auto field) load as Auto on, since the zero value means "unset" and is treated as true via a `*bool` or an explicit migration in `Normalize`.
  - `applySettings` (`settings.go`) recomputes `ui`/`dev` live, with no restart.
- **Minimum window:** `tooSmall` / `drawTooSmall` and `minSetupW/H` (1280×720) compare against the **logical** size. A large preset on a small window shows the "Enlarge the window or lower the UI scale" note instead of overlapping, and the text names both fixes.

### B.3 Options (Accessibility tab, `screen_options.go`)
Replace the "planned" note with:
- **UI scale:** a stepper `Auto · 75% · 90% · 100% · 115% · 130% · 150% · 175% · 200%`. Auto shows its current value, e.g. "Auto (150%)". Changes apply immediately. A changed scale keeps the Options screen open at the same tab and row (focus restored), so the stepper stays under the cursor's logical position.
- **Sharp on high-density screens** (on/off, Graphics tab): HiDPI rendering. The tooltip notes it costs GPU time on very large maps.
- **Text size stays tied to UI scale;** there is no separate slider (YAGNI).
- **Persistence:** `settings.json` via the existing `setSettings`. Both are presentation-only and never reach the sim or saves.

### B.4 The sweep: every UI draw goes through the canvas
The map stays raw. The rule: anything positioned by the **camera** (world space) keeps raw `vector`/`DrawImage` in physical pixels. Anything positioned by **screen layout** (panels, buttons, text, icons, HUD, menus, tooltips, the minimap frame) uses the `ui*` helpers with logical coordinates.

Files that mix both get the split explicitly:

| File | Split |
|---|---|
| `architect.go` | ghosts are world, the palette and info box are UI |
| `minimap.go` | the box is UI; the map image draws into the scaled box; clicks convert logical → map |
| `room_stats.go` | the hover panel is UI |
| `devmode.go` | the debug overlays: tile markers are world, panels are UI |
| `orders.go` | the context menu is UI |
| `move_mode.go` | the ghost is world, hints are UI |
| `labels.go` | name labels are world-anchored, but text should scale with the UI: positioned in world space and drawn with `drawText` at UI scale (anchor converted physical → logical) |
| `selection.go` | brackets are world |
| `globe.go` | the globe is drawn at physical size inside a logical panel rect converted with `× s` |
| `screen_landing.go` | panels are UI, the globe as above |
| `setup_backdrop.go`, `menu_diorama.go`, `splash.go` | full-screen backgrounds: physical size, unscaled |

- **Guard test:** `TestUIDrawsThroughCanvas` lists the files that may call `vector.*`, `ebitenutil.*` or `.DrawImage(` directly. It covers the world renderer, the backgrounds and `ui_canvas.go` itself, with a reason per file. It scans the package source, and the build fails if a new UI file draws raw.
- **Hit rectangles** need no change: widgets keep registering logical rects, and input is logical.

### B.5 Tests and checks (core + sweep)
- **`ui_scale_test.go`:**
  - the auto formula (720 → 1.0, 1080 → 1.5, 1440 → 2.0, 600 → 1.0);
  - presets snap;
  - captures force 1.0;
  - `LayoutF` with dev = 2 returns physical size and logical `sw/sh = physical / s`;
  - text measured at s = 1 and s = 2 gives the same logical width ±1 px;
  - a button drawn at s = 1.5 is hit at its logical centre through the scaled cursor;
  - the map viewport physical = logical × s;
  - `screenToWorld` uses raw coordinates.
- **Re-run the existing two-size draw tests** (`newgame_test.go:873`, `screen_pawns_test.go:419`, `screen_worldgen_test.go:333`, `app_test.go:381`, `screen_landing_test.go:409`, `editor_test.go:526`) at s = 1.0 and also at s = 1.5 and s = 2.0 (logical 1280×720 kept). Nothing overlaps or leaves the screen (a widget-rect inside-screen and non-overlap check added to the kit's debug hit list).
- **`TestSetupMinWindow` / `TestNewGameScreensNeedMinWindow`** compare in logical pixels.
- **Font race:** removing the `font_test.go` global mutation fixes the race.
- **Captures:** `-capture-ui menu`, `pawns` and `colony` give byte-identical output before and after the change at default settings (a hash test over one capture state).

---

## Execution (parallel-work rules)

### Phase 0: lead contract (one commit)
- **Plan:** the command kind `DesignatePlan`, `WorldView.Plan`/`PlanVer`, `saveState.Plan` and the plan palette tokens.
- **Scaling core (all of B.2):**
  - `ui_scale.go`, `ui_canvas.go` helpers, font scaling, `LayoutF`, logical/physical sizes, cursor split, viewport split, settings fields with `Normalize`;
  - `drawPanel`/`drawButton`/`drawLabel`/kit primitives converted;
  - the guard test with a **temporary allowlist** of every not-yet-converted file. Each agent removes its files from the list.
- Build and the full test suite green at s = 1.

### Phase 1: agents (8 at once)
| Agent | Model (why) | Owns |
|---|---|---|
| **PLAN-SIM** | sonnet (clear template: home.go) | `game/plan.go`, the `completeFrame`/`completeMove` clear hooks, `applyOne` case, view fill, save import/export, `plan_test.go` |
| **PLAN-UI** | sonnet | `archtools.go` plan tools, `plan_draw.go`, the hud.go plan toggle and hover line, overlay icons in `assets/source/blender/artlib/families/overlay.py` (+ render `-- overlay`), UI tests |
| **HIDPI-MAP** | opus (renderer, camera, input mapping, performance) | `renderer.go` (tile size × dev, physical viewport), `camera.go`, `camera_smooth.go`, `selection.go`, `minimap.go` click mapping, `globe.go` panel mapping, `fps_probe.go` (reports s), map tests; runs `-fpsprobe` at 500² with HiDPI on and off |
| **SWEEP-HUD** | sonnet (mechanical, against the rule) | `hud.go` (non-plan parts), `status_chips.go`, `alerts.go`, `toast.go`, `letters.go`, `log_ui.go`, `colony_ui.go`, `labels.go` |
| **SWEEP-PANELS** | sonnet | `inspector.go`, `building_inspector.go`, `animal_inspector.go`, `item_inspector.go`, `plant_inspector.go`, `bio_ui.go`, `room_stats.go`, `threat_ui.go`, `trough_ui.go`, `climate_ui.go`, `biome_ui.go`, `story_tab.go`, `factions_window.go` |
| **SWEEP-TOOLS** | sonnet | `architect.go` (UI parts), `orders.go`, `move_mode.go`, `work.go`, the remaining tab windows (schedule, research, animals, wildlife tabs), `search_ui.go`, `devmode.go`, `controls_ui.go` |
| **SWEEP-SCREENS** | sonnet | `screen_menu.go`, `screen_scenario*.go`, `screen_storyteller.go`, `screen_difficulty.go`, `screen_worldgen.go`, `screen_landing.go` (panels), `screen_factions.go`, `screen_pawns.go`, `portrait.go`, `screen_loading.go`, `screen_boot.go`, `screen_pause.go`, `saves.go`, credits |
| **OPTIONS** | sonnet | `screen_options.go` (UI scale stepper, HiDPI toggle, focus restore), `settings.go` apply path, `internal/platform/config` tests |

- **Every agent:**
  - TDD where behaviour changes;
  - converts its files and removes them from the guard allowlist;
  - runs `go test -race -count=1 ./internal/adapters/inbound/ebiten -run '<its tests>|Setup|Draw|Layout|Hit'` and checks its screens at s = 1, 1.5 and 2;
  - never touches another agent's files;
  - reports in 200 words or fewer.
- Shared files (`hud.go`, `architect.go`) are split by function as listed. The lead relays cross-file needs.

### Phase 2: lead verification
1. `go build ./...`, `gofmt -l`, `go vet ./...`, `staticcheck` (no new findings).
2. `go test -count=1 -timeout 40m ./...`; `-race` on the game and ebiten packages; determinism ×3; goldens unchanged.
3. The guard allowlist holds only the world, background and canvas files.
4. `make art-check`: plan icons present, no orphans.
5. `-fpsprobe 60` at 500×500 z0.25 and z1 with HiDPI on and off. If the 1% low falls under 60 fps with HiDPI on at 2×, HiDPI defaults to off on that resolution class; the result is recorded either way.
6. Captures byte-identical at default settings.
7. **Manual:**
   - resize the window from 1280×720 to fullscreen 1080p/1440p: Auto grows the UI, text stays sharp, nothing overlaps, every click lands;
   - each preset;
   - a Retina screen with HiDPI on and off;
   - paint plans, erase part, build walls over some: those marks vanish, floors leave them, and the toggle hides plans.
8. **Docs:**
   - handoffs `UI-023-plan-tool.md` and `UI-024-ui-scaling.md`;
   - the UI/UX checklist items for UI scale/accessibility, HiDPI, and planning designation;
   - vanilla checklist "Plan designation";
   - the screens spec Options › Accessibility rows;
   - the README lines (Plan tool; UI scale Auto/presets);
   - `ebiten/AGENTS.md` (canvas rule, logical vs physical coordinates).
9. Commits on `main` per area (contract, plan, scaling core and sweep, options/HiDPI, docs); no push.

## Critical files
- **Sim:** `internal/application/game/{plan.go,tick.go,move.go,game.go,save.go}`, `internal/domain/sim/command/command.go`.
- **UI core:** `internal/adapters/inbound/ebiten/{ui_scale.go,ui_canvas.go,font.go,app.go,input_frame.go,input_source.go,camera.go,renderer.go,widgets.go,ui_*.go}`.
- **Sweep:** about 45 UI files in `internal/adapters/inbound/ebiten` (groups above).
- **Settings:** `internal/platform/config/settings.go`, `ebiten/settings.go`, `ebiten/screen_options.go`.
- **Art:** `assets/source/blender/artlib/families/overlay.py` (plan icons), `assets/palettes/ui.yaml`.
