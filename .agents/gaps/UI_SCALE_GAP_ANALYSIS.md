# UI size and scale gap analysis (UIS-*)

Scope: the logical sizes of the in-game HUD, windows, dialogs, fonts and map labels; the UI scale steps; and the map zoom range, all compared with the reference game's measured screen.
- Load only for UIS-* tasks.
- Handoffs are sections of `.agents/handoffs/UI.md`.
- Layout rules: `MAIN_UI_SCREENS_SPECIFICATION.md` §0. The HUD regions are placed by `hudlayout/layout.go`.
- Paths in the rows are under `internal/adapters/inbound/ebiten/` unless they start with `internal/`.

## Decision record

| Item | Decision |
|---|---|
| Source of numbers | Measured from screenshots of the installed reference game, version 1.6.4871, Core only: no DLC and no mods (user, 2026-11-06). Decompiled or wiki constants are not used. |
| Clean room | `CLEAN_ROOM.md` row UIS-*. **May use:** on-screen sizes, proportions, scale steps and zoom range. **Stays original:** art, icons, colours, fonts, text and code. Screenshots never enter the repo. |
| What "match" means | Same logical sizes: our 100 % is the reference 1x, every element has the reference logical size at 1x, and our scale steps are the reference steps (user, 2026-11-06). |
| Auto | Kept as our extra. The rule is new (UIS-005). |
| Kept on purpose | HiDPI and the render cap (they change sharpness, not size; UIS-002); the docked colonist card; zone labels; far-zoom markers (default off); the "+N" overflow slots; scrims on modal dialogs. |
| Style vs size | Only sizes and cap heights must match. Our SemiBold label weight, colours and art stay ours. |
| Not applicable | DLC tabs and gizmos, mod UIs, the world map screen. |

### Decided (user, 2026-11-07: the recommended option of each)

- Default scale: Auto with the UIS-005 rule.
- Step unit: physical window px per logical px.
- Map rebase: the default zoom shows 48 cells of window height.

The options as they were put:

1. **Default scale** (UIS-006):
   - **Recommended:** Auto with the new rule. It gives 1x at 1080p, the same as the reference first launch, and 3x on the user's 5120×2762 display, which is the user's own pick there.
   - **Alternative:** a 1x default, exact at 1080p but tiny on 4K and 5K.
2. **Step unit** (UIS-002):
   - **Recommended:** steps are physical window px per logical px, as in the reference. Then "3x" on a Retina display means the same thing in both games.
   - **Alternative:** keep steps relative to points. Then our 1x on Retina is the reference 2x.
3. **Map rebase** (UIS-080, UIS-086):
   - The default zoom becomes 48 cells of window height, about 22.5 px per cell at 1080 (64 today), so the default view shows about 4,100 cells instead of 510.
   - This needs the level-of-detail tiers retuned, a downscale check on the art, and the race benchmarks.

## Sources and method

- **Driving the game:**
  - The lead drove the reference game through Steam with a separate save-data folder.
  - The user's own config files were hashed before and after and are unchanged.
  - Dev mode was on in every shot, which adds a small icon row at the top centre.
- **Unit:** logical px = captured px ÷ (k × UI scale).
  - The engine draws 1:1 into the window, so k = 1 in every shot, including the 5K Retina display.
- **Shots:** 71 shots, listed by id in the rows, kept only in the session scratchpad (`shots/manifest.tsv`):
  - 1920×1080 at 1x, and at 1.25x (the `S_*` shots);
  - a 1280×720 window at 1x;
  - the user's 5120×2762 window at 3x (the `*_u` shots);
  - 1600×1200 and 2400×960 windows, for the minimum-size checks.
  - 2x and 1.75x are refused at 1920×1080, so 1.25x and 3x are the second scales.
- **Measuring:**
  - Five read-only agents measured the shots, each value twice (two shots or two scales), and read our code at file:line.
  - The lead re-measured 10 values against the shots: the zone cell, colonist slot and pitch, tab bar, inspect pane, Options window, Work window, tooltip, Needs panel, and option control pitch. All were within ±2.
- **Confidence in tables:**
  - `exact ±1` is a direct pixel run.
  - `±2` is a framed or bevelled edge.
  - `estimate` is inferred and is named as such.

## Scale model

- **Unit:**
  - A reference step is physical window px per logical px.
  - Every HUD element, font and row has a fixed logical size that never depends on the window. For example, the inspect pane is 432 wide at 1280×720 and at 1920×1080.
  - Sizes grow only with the step.
- **Steps:** 1, 1.25, 1.5, 1.75, 2, 2.5, 3, 3.5, 4. None is ever greyed out.
- **Minimum logical size:**
  - A step `s` is accepted when `s = 1`, or when W/s ≥ Wmin and H/s ≥ Hmin, with W and H the physical window size.
  - Refused heights: 720, 690, 617 and 576. Accepted heights: 768, 789, 864 and 960. So Hmin lies in (720, 768].
  - A width of 1280 is accepted.
  - We take 1280 × 768.
  - A refused step keeps the old one and shows a one-line warning.
  - An accepted step applies at once and asks to keep or revert, with a countdown.
- **First launch:** 1x at a 1920×1080 window.
- **Map:**
  - UI scale never changes px per cell.
  - Default zoom shows 48 cells of window height: 22.5 px per cell at 1080, 15 at 720, 57.5 at 2762.
  - Zoom runs from 0.40× to 2.18× of the default in about 65 wheel notches, pivoting at the screen centre.
  - Map text (pawn names, stack counts) follows the UI scale, never zoom.
- **Text sizes:** three.

  | Size | Cap | x-height | Line pitch | Noto Sans equivalent |
  |---|---|---|---|---|
  | Tiny | 8 | 6 | 15 | 11 |
  | Small | 10 | 7 | 16 when wrapped | 14 |
  | Medium | 15 | 11 | about 24 | 21 |

  - Text is re-rasterised at each step.
  - 1-px lines land on whole device px.

**Auto rule (ours):** the largest reference step with H/s ≥ 900 and W/s ≥ 1280 in physical px, else 1x.

| Window (physical px) | Reference: largest accepted | Proposed Auto (logical) | Our Auto today (logical) |
|---|---|---|---|
| 1280×720 | 1x | 1x (1280×720) | 1.0 (1280×720) |
| 1920×1080 | 1.25x | 1x (1920×1080), as the reference first launch | 1.5 (1280×720) |
| 2560×1440 | 1.75x | 1.5x (1707×960) | 2.0 at dev 1 (1280×720) |
| 3840×2160 | 2.5x | 2x (1920×1080) | 2.0 at dev 1 (1920×1080) |
| 5120×2762 (user, Retina) | 3.5x | 3x (1707×921), the user's own pick | 3.8 (1347×727) |

**Migration (UIS-009):** a stored fixed percent `p` becomes the step nearest `p/100 × devRaw`, with ties going to the lower step, then the minimum-size clamp. Auto stays Auto.

| Stored | 75 | 90 | 100 | 115 | 130 | 150 | 175 | 200 |
|---|---|---|---|---|---|---|---|---|
| dev 1 | 1x | 1x | 1x | 1.25x | 1.25x | 1.5x | 1.75x | 2x |
| dev 2 | 1.5x | 1.75x | 2x | 2.5x | 2.5x | 3x | 3.5x | 4x |

## Already matching

- Text is laid out in logical px and re-rasterised at the scaled size. Our Small 11 equals the tiny size (`gfx/text.go:44`, `:89-95`).
- HUD sizes are logical constants independent of the window, and UI scale leaves the map's px per tile alone (`hudlayout/layout.go:32-36`, `ui_scale.go:23-29`, `camera/camera.go:121-125`).
- **Table rows:** Assign and Animals rows are 30 (`windows/assign.go:31`, `windows/animals.go:20`).
- **Main-tab windows:** they sit bottom-left at x 0 on the tab bar, sized to content (`hudlayout/layout.go:249-256`, `windows/window.go:62-84`).
- **Inspect pane:** at x 0 (`hudlayout/layout.go:271`), text inset 12, 4 status bars per row, tab gap 2 (`inspector/hud_inspect.go:87`, `:95`, `:98`), header button 20 (`inspector/pane_header.go:45`).
- **Lower-right controls:** play-settings icons 24 at pitch 28, speed button height 24 (`hud/global.go:39-43`).
- **Colonist bar:** top-centre, shrinks with the count, drawn through `gfx` (`hudlayout/layout.go:456-479`, `hud/colonistbar.go:564-591`).
- **Dialogs and world text:** dialogs centre on the whole screen (`uikit/modal.go:56-58`). World text follows the UI scale on a world anchor (`render/world/labels.go:55-67`, `render/creature/traces.go:186-196`). Selection marks follow zoom (`render/world/draw_art.go:338`).
- **OPT-017 and OPT-009:** the setting, Options row, consumer and tests exist (`TestLargerSmallText`, `TestTooltipDelayFollowsSetting`, `TestTooltipDelayAndClamp`). Their screens-spec rows are ticked in this change.

## Rows

States:
- `open` is owed.
- `kept` differs on purpose, with the reason.
- `n/a` is merged or a statement.
- `done` is met; the phase that met it is named.
- `partial` is met in part; what is missing is named.

Each area lists its measurements, then its rows.

### Scale, type and generic rows (UIS-001 to UIS-019)

Measured:

| Element | Measure | Value | Confidence | Shots |
|---|---|---|---|---|
| UI scale steps | list, order | 1, 1.25, 1.5, 1.75, 2, 2.5, 3, 3.5, 4 (9 entries, none greyed) | exact | R01, R02 |
| Refusals | logical H refused / accepted | refused ≤ 720 (720, 690, 617, 576, 540); accepted ≥ 768 (768, 789, 864, 960); 1x always accepted (1280×720) | exact (bounds); Hmin ∈ (720, 768] | R01d–f, R02b, R01i_u, R01h_u, R01g, R01j, R01k |
| Minimum logical width | bound | ≤ 1280 (1280×960 accepted); no refusal attributable to width | estimate (bound only) | R01j |
| First-launch default | step at 1920×1080 | 1x | exact | R00, R00b |
| Apply flow | after an accepted step | applies at once; keep/revert dialog with auto-revert countdown (≥ 8 s left when captured) | countdown: estimate | R01g |
| HUD independent of window | inspect pane width | 432 at 1280×720 and 1920×1080 | exact ±1 | R04, R04_720 |
| HUD independent of window | main tab bar height | 34 (720 and 1080 at 1x); 33.6 at 1.25x; 33.3–34.7 at 3x | ±1 | R03, R03_720, S_R03, R03u |
| HUD follows step | inspect pane width | 540 device px at 1.25x (= 432); ≈1294 at 3x (= 431) | exact ±1 | S_R04, R04u |
| Map vs UI scale | px per cell, default zoom | 22.5 (1080, 1x), 22.6 (1080, 1.25x), 15.0 (720, 1x), 57.75 (2762, 3x) → about window height / 48 | ±0.5 device px | R03, S_R03, R03_720, R03u |
| Map text vs UI scale | pawn label cap | 8 (1x), 9 device = 7.2 (1.25x), 24 device = 8 (3x) | ±1 | R03, S_R03, R03u |
| Tiny font | cap / x-height | 8 / 6 at 1x; 9 / 7 device = 7.2 / 5.6 at 1.25x; 23 / 17 device = 7.7 / 5.7 at 3x | ±1 | R03, S_R03, R03u (colonist bar names); R04 (pane bar text) |
| Tiny font | line pitch (two-line label) | 15; 18 device = 14.4 at 1.25x | ±1 | R07, S_R07 |
| Small font | cap / x-height | 10 / 7 at 1x; 12–13 / 9 device = 10 / 7.2 at 1.25x; 30 / 22 device = 10.0 / 7.3 at 3x | exact ±1 | R03, R04 (body), S_R03, R03u, R04u |
| Small font | wrapped line pitch | 16; 20 device at 1.25x = 16; 48 device at 3x = 16 | exact | R19 (5 lines), R14, R04, R21c, S_R04, R04u |
| Small font | stacked single-line readout rows | 19 (1x); 23.5 device = 18.8 (1.25x) | ±1 | R03, S_R03 |
| Medium font | cap / x-height | 15 / 11 at 1x; 19 / 14 device = 15.2 / 11.2 at 1.25x; 45 / 34 device = 15.0 / 11.3 at 3x | exact ±1 | R04b, R14 (window heading), S_R04, R04u |
| Larger heading | in-game windows | none distinct; window headings (research) are medium | exact | R14, R16, R14q, R20a |
| Text at 1.25x | rasterisation | re-rasterised, sharp, same stem/edge quality as 1x; not an upscaled bitmap | exact (visual) | R03 vs S_R03 crop of the same readout word |
| Lines at 1.25x | 1-logical-px border/separator | 2 device px, flat colour, no blend (pane top and right border, 9 tab separators) | exact | S_R04, S_R03 vs R04, R03 |
| Tooltip (generic) | outer width | 268 when wrapping (two tooltips both 268; a third ≈266 at half-scale view); text wraps between 258 and 266 → text width 260 ±3 | exact ±1 / wrap ±3 | R19, R09, R14 |
| Tooltip (generic) | border, inset, pitch | 1 px border; text ink starts 5 px inside the outer edge; height = 16·lines + 14 (94 for 5 lines, 78 for title + blank + 2 lines) | exact ±1 | R19, R09 |
| Tooltip (generic) | title vs body | title in the small size (colour differs, same size and weight); one blank 16-px line between title and body | exact | R09, R14 |
| Options label row | row pitch / control height | 35 / 30 (1x, at 720 and 1080); 105 / 90 device at 3x | exact | R01c, R02, R01c_u |
| Dropdown / menu list row | row pitch | 29 (28 + 1 separator), list about 70 wide | exact ±1 | R01, R02 |
| Table row (main-tab table) | row pitch | 30 (29 + 1 line); 37.5 device at 1.25x = 30 | exact ±1 | R09, S_R09 |

Rows:

| Id | RimWorld | Ours today | State | Change | Files | Phase |
|---|---|---|---|---|---|---|
| UIS-001 | 9 steps: 1, 1.25, 1.5, 1.75, 2, 2.5, 3, 3.5, 4 | 75/90/100/115/130/150/175/200 % (`internal/platform/config/settings.go:188-193`); nearest-preset snap (`:195-204`, `:273-276`) | done (UIS-2e) | `UIScalePresets = {100,125,150,175,200,250,300,350,400}`, `UIScaleMin 100`, `UIScaleMax 400`; snap to the nearest step (ties go to the lower step). Show the value as the step (our own wording), e.g. "1.25×". | internal/platform/config/settings.go, options/screen_options.go (label) | UIS-1, UIS-2e (label) |
| UIS-002 | The step is physical window px per logical px; the 5K Retina window at 3x is logical 1707×921 | `S = dev × ui` with ui relative to points (`ebiten/ui_scale.go:97-104`), so a fixed 100 % on Retina equals RimWorld 2x | done (UIS-1) | Make the step physical-relative: logical = `points×devRaw/step`, `S = step×devEff/devRaw`, `UI = step/devRaw` (devEff = dev after HiDPI off or `capDev`). Rewrite `TestComputeUIScale`/`TestLayoutFHiDPI` for it. If the lead keeps point-relative steps instead, record it as kept with that reason. | internal/adapters/inbound/ebiten/ui_scale.go, ui_scale_test.go | UIS-1 |
| UIS-003 | Step accepted iff s = 1 or (W/s ≥ Wmin, H/s ≥ Hmin), Hmin ∈ (720, 768], Wmin ≤ 1280; refusal keeps the old step and shows a one-line top-left warning | No minimum outside the setup screens; `TooSmall` 1280×720 is used only by `wizard/*` (`uikit/dialog.go:25-26`, `:53`); the Options row accepts any preset (`options/screen_options.go:314-327`) | done (UIS-2e) | Add `uiStepAllowed(physW, physH, step)` with Wmin 1280, Hmin 768 (constants beside `MinSetupW/H`). Options refuses a disallowed step: keep the old value and toast our own one-line warning. Test the five measured windows (accept/refuse table above). | internal/adapters/inbound/ebiten/ui_scale.go, uikit/dialog.go, options/screen_options.go | UIS-1 (rule), UIS-2e (row) |
| UIS-004 | Not captured: what happens when the window later shrinks below the current step's minimum | n/a | done (UIS-2e) | Our choice: the effective step is the largest allowed step ≤ the stored one; the stored value stays, and the Options label shows both (e.g. "2× (using 1.5×)"). Keeps a fixed step from producing a logical screen RimWorld would refuse. | internal/adapters/inbound/ebiten/ui_scale.go, options/screen_options.go | UIS-1 |
| UIS-005 | No Auto (our extra) | Auto = `min(w/1280, h/720)` over points, clamped 1–2, floored to 0.05 (`ui_scale.go:32-52`): 1.5 at 1080p, 3.8 total at 5K | done (UIS-1) | Auto = largest RimWorld step with `H/s ≥ 900` and `W/s ≥ 1280` in physical px, else 1x (table above: 1x, 1x, 1.5x, 2x, 3x). Drop `autoUIStep`/`autoUIMax`. Rewrite `TestAutoUIScale` with the five windows. | internal/adapters/inbound/ebiten/ui_scale.go, ui_scale_test.go | UIS-1 |
| UIS-006 | First launch 1x at 1920×1080 | Default Auto (`settings.go:214`), 1.5 at 1080p, so the HUD is 1.5× RimWorld's | done (UIS-1) | Keep Auto as the default with the UIS-005 rule (it gives 1x at 1080p, the same as RimWorld). Leave the alternative "default 1x" to the user: it is RimWorld-exact at 1080p but tiny on 4K/5K. | internal/platform/config/settings.go | UIS-1 |
| UIS-007 | An accepted step applies at once, then a keep/revert dialog auto-reverts after a countdown (≥ 8 s; total not captured) | Applies at once, no confirmation (`options/screen_options.go:314-327`) | done (UIS-2e) | After a fixed-step change, open a small keep/revert modal that restores the previous value after 10 s. 500 x 300, two 210 x 32 buttons with a 42 gap, inset 18 (merged from UIS-074); the text is ours. Auto-driven changes on window resize do not ask. | options/screen_options.go | UIS-2e |
| UIS-008 | A dropdown lists every step (row pitch 29, about 70 wide) | Click cycles Auto → presets → Auto (`options/screen_options.go:302-328`); the label shows "Auto (150%)" (`:308-311`) | partial (UIS-2e: the list and its rows are in; its cells are 150 x 26, not the reference width) | Replace the cycling row with a dropdown: Auto first, then the 9 steps, with the live value "Auto (1.5×)" kept. Rows use the menu-row token from UIS-017. Update `TestOptionsUIScaleCyclesAutoAndPresets` and `TestOptionsUIScaleAutoLabelShowsPercent`. | internal/adapters/inbound/ebiten/options/screen_options.go, options_scale_test.go | UIS-2e |
| UIS-009 | n/a (migration) | `ui_scale` is a point-relative percent (`settings.go:171-175`); `SettingsVersion` 10 (`:33`) | done (UIS-1) | Version 11, new key `ui_step` (hundredths); Auto users stay Auto. A legacy fixed value p becomes the step nearest `p/100 × devRaw` (ties go lower), then the UIS-003 clamp. `devRaw` is known only in the adapter, so `Normalize` keeps p in a legacy field and the first `LayoutF` converts and saves it once. Mappings at dev 1 and dev 2: see the table under Rows. | internal/platform/config/settings.go, settings_test.go, internal/adapters/inbound/ebiten/ui_scale.go | UIS-1 |
| UIS-010 | Text re-rasterised at size × step, rounded to whole px (19 for 18.75; tiny 9 for 10) | Rasterised at `floor(size×s)` (`gfx/text.go:89-95`): at most 1 device px smaller | kept | Floor guarantees drawn text never exceeds its measured box; the difference is ≤ 1 device px of cap at 1.25x and 1.75x. Re-check after the UIS-012 sizes in UIS-3 crops. | internal/adapters/inbound/ebiten/gfx/text.go | UIS-3 |
| UIS-011 | 1-logical-px lines and borders land on whole device px in flat colour (2 device px at 1.25x everywhere checked) | `FillRect`/`StrokeRect`/`Line` draw at fractional physical coordinates (`gfx/canvas.go:50-92`): with `aa=false` a 1-px line is 1 or 2 device px depending on position; with `aa=true` it is blended | done (UIS-1) | At non-integer S, for `aa=false` rects and axis-aligned lines: round the start edge to the nearest device px and size `max(1, ceil(w×S−0.01))`. Equal elements then get equal widths, never blended. Add a canvas-trace test at S = 1.25 and 1.75. | internal/adapters/inbound/ebiten/gfx/canvas.go, ui_scale_test.go | UIS-1 |
| UIS-012 | Three sizes: tiny cap 8 / x 6, small cap 10 / x 7, medium cap 15 / x 11 | Small 11 (cap 7.85, matches tiny); Body 13 (cap 9.3) and Label 13 SemiBold; Large 16 (cap 11.4, no RimWorld size); Title 22 (cap 15.7); Huge 32 (`gfx/text.go:44`, `:52-57`) | done (gameplay UI; the setup screens keep `gfx.Large`, outside UIS) | Body 13 → 14 and Label 13 → 14 (small, cap 10.0; SemiBold weight kept as our style). Title 22 → 21 (medium, cap 15.0). Large has no RimWorld size: each consumer moves to Title 21 (pane/window titles) or Body 14 (emphasis), decided by each UIS-2 worker for its own files. Huge stays (main menu and splash only, outside the in-game UI). Update `ART_DIRECTION_AND_ASSET_PIPELINE.md:709`. | internal/adapters/inbound/ebiten/gfx/text.go (+ consumers of gfx.Large in inspector/, uikit/modal.go, uikit/dialog.go, windows/, options/story_tab.go) | UIS-1 (token), UIS-2a–2d (Large consumers) |
| UIS-013 | Line pitch: tiny 15, small 16 (wrapped), medium ≈ 24 (estimate) | `LineH` = ascent + descent + 2 = 1.362 em + 2 (`gfx/text.go:189-205`): Small 17, Body 19.7 (21 at 14), Title 32 | done (UIS-1) | Give each size a pitch token and return it from `LineH`/`LineStep`: 11 → 15, 14 → 16, 21 → 24. Noto glyph boxes overlap at these pitches but the glyphs do not (cap + descender of 14 px Noto = 13 < 16). Every multi-line layout reflows through `LineH`, so UIS-2x re-checks its heights. | internal/adapters/inbound/ebiten/gfx/text.go | UIS-1 |
| UIS-014 | The Interface option that turns off the tiny font (off by default) draws tiny text in the small size | "Larger small text" raises Small 11 → 13 (`gfx/text.go:44-45`, `:100-103`; `config/prefs.go:46`) | done (UIS-1) | `LargerSmallSize` = the new Body size (14) with the small pitch (16); fix the 13-px comments (`prefs.go:46`, `text_larger_test.go:19`). `TestLargerSmallText` already compares against Body, so it still holds. | internal/adapters/inbound/ebiten/gfx/text.go, gfx/text_larger_test.go, internal/platform/config/prefs.go | UIS-1 |
| UIS-015 | Generic tooltip details below | `TipMaxW` 360 (`uikit/frame.go:13`); `tipPad` 8, `tipGap` 2 (`uikit/tooltip.go:16-17`); title in `Label` 13 SemiBold, body in `Small` 11, i.e. tiny (`tooltip.go:35-38`, `:89-97`) | done (UIS-2c) | `TipMaxW` 268; inner text width 260; pad 4 including a 1-px border; title and body both Body 14, title coloured; one blank 16-px line between title and body; pitch from UIS-013. Height then comes out as 16·lines + 14 (see the RimWorld list below). | internal/adapters/inbound/ebiten/uikit/tooltip.go, uikit/frame.go | UIS-2c |
| UIS-016 | Options label row pitch 35, controls 30 tall, the same at every window | `optRowStep` 44, `OptBtnH` 32 (`options/screen_options.go:100-102`) | done (UIS-2e) | Tokens: option row 35, option control 30 (a 28 face plus bevel; merged from UIS-076). The Options screen uses them. Window size: UIS-075. | internal/adapters/inbound/ebiten/hudlayout (metrics table), options/screen_options.go | UIS-1 (token), UIS-2e |
| UIS-017 | Dropdown/float list row pitch 29 (28 + 1 separator); main-tab table row pitch 30 (29 + 1) | `ListRowH` 26 (`uikit/scroll.go:19`); Work `workRowH` 22 (`windows/work.go:74`); Assign/Animals 30 already | done (UIS-1 token, UIS-2c) | Tokens: menu row 29 (28 + 1 line), table row 30. Work goes from 22 to 30, and lists using `ListRowH` move to 29 where they are menus. Float-menu specifics: UIS-070. | internal/adapters/inbound/ebiten/hudlayout (metrics table), uikit/scroll.go, windows/work.go | UIS-1 (token), UIS-2c |
| UIS-018 | HUD in fixed logical px (independent of window), scaled only by the step. Map: default zoom shows about 48 cells of window height at every window and step (px/cell = H/48), and UI scale never changes it. Map text labels follow the UI step | HUD and labels match the model (Already matching). Map tile = 64 px × zoom × dev, default zoom 1: about 16.9 cells tall at 1080, a fixed px size, not cells per window height | n/a | Model statement; carried out by UIS-080 and UIS-084. | none | n/a |
| UIS-019 | n/a (hygiene) | Comments still name `curUIScale`, now `gfx.SetScale`/`gfx` `cur.scale` (`gfx/state.go:16-40`): `ui_scale.go:187`; `ui_scale_test.go:23`, `:25`; `zones_draw_test.go:163`; `bar_doubleclick_test.go:16`; `world_labels_test.go:21`; `map_hidpi_fixup_test.go:11`; `map_hidpi_test.go:21`, `:23`; `weather_label_test.go:15` | done (UIS-1) | Rename every mention to "the gfx scale (`gfx.SetScale`)". Comments only. | internal/adapters/inbound/ebiten/ui_scale.go and the 8 *_test.go files listed | UIS-1 |

### Top HUD: colonist bar, messages, alerts, letters (UIS-020 to UIS-039)

Measured:

| Element | Measure | Value | Confidence | Shots |
|---|---|---|---|---|
| Colonist slot (portrait box) | width x height | 48 x 48 (square) | exact ±1 | R03, R22a, R22_16, S_R03 (60 px = 48), R03u (143 px = 47.7) |
| Slot spacing | gap / pitch at full size | 24 gap, 72 pitch (gap = half the slot) | exact ±1 | R03, R22a, R22_16, S_R03 (pitch 89.3 px = 71.5), R03u (216 px = 72) |
| Slot distance from top edge | top of portrait box | 21 | ±2 (dev mode on in every shot) | R03, R22_24, R22_40, R04, S_R03 (26 px = 20.8), R03u (62 px = 20.7) |
| Horizontal centring | rule | row width = n x pitch (the trailing gap counts), centred on screen width / 2, so the visible slots sit half a gap (12 at full size) left of centre. Checked: 3 slots at 1x, 1.25x (660 logical), 3x (745.7 logical); 8, 16, 24 and 34 slots at 1x. Each row of a wrapped bar is centred on its own | exact ±1 | R03, R22a, R22_16, R22_24, R22_40, R22_60, S_R03, R03u |
| Name label | text cap height / position | cap height 8 at every slot size (does not shrink with the slot); label top at the slot's bottom edge, about 1 to 2 px overlapping it; centred under the slot, on a small dark translucent plate about 11 high that hugs the text; overlong names end in an ellipsis (only at the 27 px slots) | ±1 (cap), estimate (plate height) | R03, R22_24, R22_40, S_R03 (9 px = 7.2) |
| Name label size as slots shrink | cap height | 8 at slot 48, 37 and 27 | exact ±1 | R03, R22_24, R22_40 |
| Slot size by count: 3 / 8 / 16 | slot, pitch | 48, 72 (no shrink up to 16; 16 x 72 = 1152 wide) | exact ±1 | R03, R22a, R22_16 |
| Slot size at 24 | slot, gap, pitch | 37, 19, 56 (1 row, 24 x 56 = 1344 wide) | exact ±1 | R22_24 |
| Slot size at 40 and 60 | slot, gap, pitch | 27, 14, 41; two rows. Row 1 holds 34 (34 x 41 = 1394 wide), row 2 the rest (6 at 40, 26 at 60). Row pitch 46. The first row starts at the same place at 40 and at 60 | exact ±1 | R22_40, R22_60 |
| Shrink steps | slot / pitch ratio to full | 1.00, 0.78 (56/72), 0.57 (41/72); the intermediate counts (17 to 23, 25 to 39) were not captured, so whether the shrink is continuous or in three steps is not pinned | estimate | R22a to R22_60 |
| Bar width cap | widest single row seen | 1394 (34 slots at pitch 41, 72.6 % of 1920). The cap lies in [1394, 1435): 35 x 41 did not fit. How it depends on screen width is not measured (only 3 colonists at 1280x720) | estimate | R22_40, R22_60 |
| Row wrap | when | at 40 the 24 x 56 single row no longer fits at the smallest step; two rows at the smallest size, with 60 colonists still two rows (no "+N" overflow) | ±2 | R22_24, R22_40, R22_60 |
| Selected slot | indicator | four white corner brackets sitting about 2 px outside the portrait box, arms about 5 to 12 px; the box itself is unchanged | ±2 | R04 |
| Drafted colonist | indicator | the name is underlined; the brackets stay | ±2 | R06 |
| Mood, status icons | indicator | none visible on any calm colonist at rest; not measurable | n/a | R03 to R22_60 |
| Dev palette row (dev mode only) | size | 8 icons about 20 wide, pitch 24, top y 3 to 25, centred on the screen; it overlaps the top 4 px of the slots. Not part of the stock HUD | estimate | R03 |
| Resources readout | all | top-left stays empty in every shot (no row, no panel) | n/a | R03, R00, R21 |
| Message text | cap height / left edge / first row | cap 10; text starts at x 141 (ink), cap top at y 22, baseline y 31; no panel or edge strip drawn; one message seen at a time, width 270 (text) | ±1 | R21d |
| Message over the bar | overlap | the message text overlaps the bar's top-left slots; the slots under its rectangle fade to a faint ghost (portrait nearly invisible, name dimmed) | ±2 | R21d |
| Alert row | pitch / text | 22 pitch at 1x (4 rows: 441, 463, 485, 507) and at 3x (65 px = 21.7); cap height 10 | exact ±1 | R21, R03, R03u, R03_720 (pitch 22) |
| Alert row at 1.25x | pitch | 37 px = 29.6 logical, unexplained (identical in S_R03, S_R04, S_R07, S_R09, S_R18, S_R24b); the readout rows below keep their 1x pitch (32 px = 25.6). Trust the 22 at 1x and 3x | estimate | S_R03 |
| Alert text alignment | right edge | right-aligned, ink ends 7 from the screen edge (1x), 6.4 (1.25x), 5.3 (3x); no row background | ±2 | R21, S_R03, R03u |
| Alert stack direction | order | anchored at the bottom: the last alert sits just above the letters (or above the date/weather readout when there are no letters) and the stack grows upward. Gap from the bottom alert row to the readout about 16; bottom alert centre to first letter icon top 24 | ±2 | R03, R21, R00 |
| Letter icon | size / colour | 38 x 30 (a framed envelope), blue for normal letters and grey for the visitor letter | exact ±1 | R21 |
| Letter stack | pitch / inset | pitch 42 (gap 12), icon right edge 12 from the screen edge, stack bottom 22 above the readout text (icon bottom 729, clock text centre 751); grows upward | exact ±1 | R21, R21c |
| Letter label | position | right-aligned text of cap 10 centred on the icon's height, its right end 3 px from the screen edge, so it runs over the icon and out to the left; a dark translucent strip hugs the text (visible behind the lower four) | ±2 | R21, R21c |
| Letter hover | result | a tooltip box 330 x 105 opens to the left of the icon (right edge 10 from the icon) holding the letter's text, wrapped at about 300; no other visible change to the icon or label. Letter opened: a window (UIS-073) | ±2 | R21c, R21b |
| Letter stack cap | count | all 5 letters shown, no "+N older" row seen | n/a | R21 |
| Alerts+letters column | width | no fixed column: each row is only as wide as its text; the longest label seen (letter) is 285 px | ±2 | R21 |
| UI scale behaviour | bar | slot 48 at 1x, 60 px at 1.25x, 143 px at 3x: the whole bar scales with UI scale, same logical layout and same centring | exact ±1 | R03, S_R03, R03u |

Rows:

| Id | RimWorld | Ours today | State | Change | Files | Phase |
|---|---|---|---|---|---|---|
| UIS-020 | Portrait box 48 x 48; the name sits below it (name height is outside the box) | Slot 52 x 64 with the name drawn inside it (`hudlayout/layout.go:44-45`, `hud/colonistbar.go:314-320`) | done (UIS-1) | Set the full-size slot to 48 x 48; draw the name plate below the slot (top edge at the slot bottom minus about 1) instead of inside it; the name plate adds about 11 to the bar's footprint | `hudlayout/layout.go`, `hud/colonistbar.go` | UIS-1 (tokens), UIS-2a |
| UIS-021 | Pitch 72 at full size: gap 24 (half the slot), gap scales with the slot (19 at 37, 14 at 27) | Gap 4 (`hudlayout/layout.go:46`) | done (UIS-1) | Make the gap half the slot size; the pitch is slot x 1.5 | `hudlayout/layout.go` | UIS-1 |
| UIS-022 | Slot sizes 48, 37, 27 (pitch 72, 56, 41) at 1x for 16, 24, 40 or more colonists; no overflow at 60 (two rows) | Steps 1, 0.75, 0.5 for 1, 2, 3 rows, then "+N" (`hudlayout/layout.go:70-74`, `:456-479`; groups `:492-`) | done (UIS-1) | Replace the steps with slot x pitch sizes 48/37/27 (ratios 1, 0.78, 0.57; 2 rows at the smallest; 3 rows are not needed to hold 60). Keep the row-cap rule: a row may be at most about 1394 to 1435 wide at 1920 logical. Capture 17 to 23 and 25 to 39 colonists later (UIS-3) to decide between three steps and a continuous shrink. Our "+N" slot can stay as a last resort past 2 rows x the smallest step | `hudlayout/layout.go` (`barScales`, `layoutBar`, `layoutBarGroups`) | UIS-1 |
| UIS-023 | Bar width cap about 1400 at 1920 logical (73 % of the width) | `capW = W - 2*(HudMargin + RightW + HudMargin)` = 1408 at W 1920, 1056 at 1536, 784 at 1280 (`hudlayout/layout.go:235`) | done (UIS-1) | Keep a width-relative cap but verify it against W: at 1920 it already matches (1408 vs the 1394 to 1435 measured). Do not tie it to the right column; use about W minus 520. Re-measure at 1280 and 1536 once a many-colonist shot exists at those widths | `hudlayout/layout.go` | UIS-1 |
| UIS-024 | Slot top at 21 | Slot top at 8 (`HudMargin`, `hudlayout/layout.go:474`) | done (UIS-1) | Add a bar-top token of 21 and use it instead of `HudMargin`; re-check it without the dev palette row | `hudlayout/layout.go` | UIS-1 |
| UIS-025 | Each row centred on its own; row start = (W - n x pitch) / 2 (trailing gap counted) | One x0 for the whole bar, later rows left-aligned to it, trailing gap not counted (`hudlayout/layout.go:469-474`) | done (UIS-1) | Centre every row on its own with row width n x pitch; for a short last row this puts it under the middle of the full row | `hudlayout/layout.go` (`layoutBar`, `fitGroups`) | UIS-1 |
| UIS-026 | Name cap height 8 at every slot size; names always drawn, with an ellipsis at the narrowest slot | Small 11 name only at bar scale >= 1 (`barNameMin = 1`, `hud/colonistbar.go:40`, `:317-319`); the name hides at 0.75 and 0.5 | done (UIS-1) | Always draw the name; use a font with cap about 8 at any slot size and let the text keep that size while the slot shrinks; ellipsize to the slot width (already done: `:585`). Cap mapping to our fonts: UIS-012 | `hud/colonistbar.go`, `gfx/text.go` | UIS-1, UIS-2a |
| UIS-027 | Selected = four white corner brackets about 2 px outside the portrait box; drafted = underlined name; no mood fill, no status marks seen for calm colonists | 2 px accent frame inside the card (`hud/colonistbar.go:587-590`); mood backing fill when the BarMood setting is on (`:568-571`); status marks 14 px down the right edge (`:596-613`) | kept (mood fill and status marks); done (selection brackets and draft underline, UIS-2a) | Draw the selection as corner brackets outside the box and mark a drafted colonist with an underlined name. Keep our mood fill and status marks: RimWorld's calm-state bar shows none, and our setting is optional | `hud/colonistbar.go` | UIS-2a |
| UIS-028 | Resources readout: top-left is empty in all shots; not measurable | 168 wide, rows 20, icon 16 at (8, 8) (`hudlayout/layout.go:55-56`, `:329-335`; `hud/resources.go:31`) | kept | Leave as is until a shot with a visible readout exists | none | n/a |
| UIS-029 | Message text starts at x 140 and y about 17 (cap top 22), above the bar's top area; plain text, no panel; cap 10; the bar slots it overlaps fade | Messages sit under the bar, right-aligned against the right column, 22 rows stride 24, up to 5, width at most 480, Body font (`hudlayout/layout.go:59-63`, `:312-327`; `hud/messages.go:275`) | done (UIS-1) | Anchor the messages block top-left at x 140, y 17; left-align; fade out the colonist slots under the block. Row stride for several messages is not measured: keep 22 until captured (see Not measurable) | `hudlayout/layout.go`, `hud/messages.go`, `hud/colonistbar.go` (slot fade) | UIS-1, UIS-2a |
| UIS-030 | Alert rows: pitch 22 (1x and 3x), cap 10, right-aligned text ending 7 from the edge, no row panel, anchored just above the letters or the readout, growing upward | Rows 24 + 4 gap, panel rows with a 4 px edge strip, text left-aligned at +12, in a 232 wide column; alerts sit at the top of the column, letters at the bottom (`hudlayout/layout.go:36`, `:223-230`; `hud/alerts.go:452`, `:504-513`) | done (UIS-1) | Set the alert row to 22, remove the gap, right-align the text 7 from the edge, drop the row panel for plain text with a shadow plate; stack alerts directly above the letters, upward, with a gap of about 24 from the last row to the first letter. Keep the severity colour as the text colour | `hudlayout/layout.go`, `hud/alerts.go` | UIS-1, UIS-2a |
| UIS-031 | Letter: 38 x 30 envelope icon, pitch 42, 12 from the right edge, label (cap 10) right-aligned over and left of the icon on a dark strip | 28 + 4 rows, a 232 wide panel with a 4 px colour bar, a 20 px kind icon and left-aligned label (`hudlayout/layout.go:34`, `:189-221`; `hud/letters.go:42`, `:361-382`) | done (UIS-1) | Draw letters as 38 x 30 icon tiles with the label right-aligned over them: row height 30, gap 12, right inset 12; the tile colour stays our own (blue for normal, grey for neutral) | `hudlayout/layout.go`, `hud/letters.go` | UIS-1, UIS-2a |
| UIS-032 | Letter hover opens a 330 x 105 tooltip to the left of the icon (10 gap); nothing else changes | Hover brightens the row (`hud/letters.go:361-365`); no preview box | partial (the hover card in `hud/right.go` keeps its old size and anchors on the full row) | Show the letter's text in a tooltip box to the left of the tile, wrapped at about 300, on hover through the shared tooltip | hud/letters.go | UIS-2a |
| UIS-033 | Letter stack shows 5 letters with no "+N older" row; the cap is unknown | `MaxLetters = 8` then a "+N older" row (`hudlayout/layout.go:35`) | kept | Keep 8 and "+N older": nothing in RimWorld contradicts it | none | n/a |
| UIS-034 | Alert and letter column width follows the text (up to about 285 for a letter label); no fixed 232 column | `RightW = 232` fixed (`hudlayout/layout.go:32`) | done (UIS-1) | Treat 232 as a hit-test and layout width only; size each row to its text so nothing is clipped at 232; the global controls and speed buttons are UIS-056 to UIS-058 | `hudlayout/layout.go` | UIS-1 |
| UIS-035 | Alert pitch at 1.25x measured 29.6 instead of 22 (not explained) | n/a | done (UIS-3: layouts are logical, `TestPanelsStayInTheirPanel` checks every HUD draw is the scale-1 draw x s) | At UIS-3 verify our alert pitch is 22 logical at 1.25x; do not copy the 29.6 | none | UIS-3 |
| UIS-036 | Bar scales with UI scale (slot 48 logical at 1x, 1.25x and 3x) | Slots are logical and drawn through `gfx` (`hudlayout/layout.go:474`, `hud/colonistbar.go:564-566`) | done (UIS-3: same check) | Verify at UIS-3 that the bar is 48 logical at every UI scale step once UIS-1 lands; no code change of its own | none | UIS-3 |

### Bottom HUD: tab bar, inspect pane, commands, Architect, lower right, cell readout (UIS-040 to UIS-059)

Measured:

| Element | Measure | Value | Confidence | Shots |
|---|---|---|---|---|
| Main tab bar | height (top highlight line to window bottom) | 34 | exact ±1 | R03 (34), R03_720 (34), S_R03 (34.4), R03u (34 from pane/bar offsets in R04u) |
| Main tab bar | span, anchor | full logical width, x 0..W, flush with the window bottom, no margin | exact | R03, R03_720, S_R03, R03u |
| Main tab bar | tab count | 12: 9 text tabs (our text tabs less History, Factions and Menu, plus a Quests tab between Research and World) then 3 icon tabs (book, building-with-flag, bullet-list) | exact | R03 |
| Main tab bar | text tab width rule | width = floor(W_logical / 10.5); pitch 182 at 1920, 121 at 1280, 146.3 at 1536 (1.25x), 162 at 1707 (3x). Separator 2 px between tabs | exact ±1 | R03, R03_720, S_R03, R03u |
| Main tab bar | icon tab width | half a text tab: 91 / 60 / ~73 / ~81. The last icon tab takes the rounding remainder (100 at 1920, 71 at 1280) | exact ±1 | R03, R03_720 |
| Main tab bar | label | cap height 10 (W), left-aligned at 14.8 % of the tab width from its left edge (27 at 1920, 18 at 1280, 21.6 at 1.25x), baseline 20 below the bar top (14 above the bottom); not centred, not ellipsized in any shot | exact ±1 | R03, R03_720, S_R03 |
| Main tab bar | icon tab glyph | ~24 wide, centred in its tab | estimate | R03 |
| Inspect pane | outer size | 432 x 165 (interior 430 x 163), same at 1x 1920, 1x 1280, 1.25x, 3x: fixed in logical px, never follows the window | exact ±1 | R04, R04b, R04_720, S_R04 (428.8 x 162.4), R04u (430 x 163) |
| Inspect pane | anchor | x = 0 (flush with the left edge), bottom flush on the tab bar top (no gap) | exact | R04, R04_720, R04u |
| Inspect pane | tab row (colonist) | 6 equal tabs filling the pane width: pitch 72 (70 + 2 gap), height 30 (28 face + frame); row sits 1 px above the pane body (y 850..879, pane from 881). Labels cap 9, centred | exact ±1 | R04, R05a-f |
| Inspect pane | tab row (zone/storage) | a single 70 x 28 tab right-aligned to the pane's right edge (x 361..430). Building selection: no tab row at all, same pane body height | exact ±1 | R20b, R04b |
| Inspect pane | title row | name cap 15 (S overshoot; x-height 11), left inset 12, baseline 28 below the pane top; surname/role part dimmer | exact ±1 | R04 |
| Inspect pane | title icon buttons | 3 glyph buttons, each ~18 x 20, right-aligned, last one ends ~12 from the pane's right edge (x 353..420); building pane has 2 | estimate (±2) | R04, R04b |
| Inspect pane | status mini-bars | 4 bars, each 93 x 16, gap 6, left inset 12, top 36 below the pane top; label cap 8 inside the bar, left inset 3; building pane shows one bar (hit points) | exact ±1 | R04, R04b |
| Inspect pane | body text | cap 10, line pitch 16, first baseline 68 below the pane top, left inset 12 | exact ±1 | R04 |
| Pane tab panel (open) | Log | 630 x 510 (interior 628 x 508) | exact ±2 | R05a |
| Pane tab panel (open) | Gear | 460 x 450 | exact ±2 | R05b |
| Pane tab panel (open) | Social | 540 x 510 (a divider at 334 from the top) | exact ±2 | R05c |
| Pane tab panel (open) | Bio | 514 x 489 | exact ±2 | R05d |
| Pane tab panel (open) | Needs | 580 x 520 | exact ±2 | R05e |
| Pane tab panel (open) | Health | 630 x 430 (left sub-panel 236 wide with its own two tabs) | exact ±2 | R05f |
| Pane tab panel (open) | placement | x = 0, bottom edge touches the tab row top (no gap), width and height chosen per tab, top corner has a close button (and a dev button in dev mode) | exact | R05a-f |
| Gizmo | cell | 75 x 75 outer (frame included); stride 80, so gap 5 | exact ±1 | R04b, R20b, R07b (designators identical) |
| Gizmo | position | first cell x = pane right edge + 14 (432 -> 446); same at 1x 1280 and 3x. Bottom row: cell bottom 16 above the tab bar top (cell y 956..1030, bar top 1046) | exact ±1 | R04b, R04_720, R04u |
| Gizmo | hotkey letter | top-left, inset 6 / 6, cap 8, no backing plate | exact ±1 | R04b |
| Gizmo | label | cap 8, centred, bottom-anchored: one line sits with its baseline 6 below the cell bottom (it overlaps the frame); two lines (e.g. a two-word label too wide) stack upward inside the cell with pitch 15-16. A single long word ellipsizes | exact ±1 | R04b, R20b, R07 |
| Gizmo | icon | art ~36-40 px centred in the cell | estimate | R04b |
| Gizmo | extra row/wrap/overflow | not observed: the largest selection shows 6 gizmos in one row (R20b/R23b) with room for 18+ at 1920 | n/a | R20b, R23b |
| Architect | category panel | 200 wide, flush left and bottom (bottom = bar top); 2 columns x 6 rows of 100 x 32 buttons, no gap, no pad, row-major (12 categories, one greyed) | exact ±1 | R07b, R08, S_R07 |
| Architect | search row | under the buttons, ~28 high, magnifier + text field spanning the panel; panel total about 220 high (y 826..1046) | exact ±2 | R07b |
| Architect | category label | cap 10, left inset 8 | exact ±1 | R07, S_R07 |
| Architect | description panel | fixed 200 x 270 (interior 198 x 268), flush on the category panel (no gap), always present while the Architect is open, empty without hover; same 200 x 268 at 1.25x | exact ±1 | R07b, R08, S_R07 |
| Architect | description text | body cap 10, line pitch 16, inset ~8; title line the same size as body, info glyph top-right | exact ±1 | S_R07 |
| Architect | designator grid | cells identical to gizmos: 75 outer, stride 80; first cell x = 210 (category panel right + 10); one row, same bottom as gizmos; 12 shown at 1920 and the row ran off screen at 11+ in R07 only because the window ended | exact ±1 | R07, R07b, R08, S_R07 |
| Architect | designator label | cap 8, same rules as a gizmo label; a two-word label wraps to 2 lines; long single words and three-word labels ellipsize | exact ±1 | R07, S_R07 |
| Bottom right | play-settings icon grid | icons 24 (outline to outline 23-24), horizontal pitch 28, vertical pitch ~28.5; 5 columns, rows right-aligned (3 + 5 + 5 = 13 icons); right edge 3 from the window edge; bottom row bottom 6 above the bar top | exact ±1 | R03, R03_720, R03u (84 px / 3 = 28), S_R03 (35 / 1.25 = 28) |
| Bottom right | speed buttons | 4 cells of 32 x 24, no gap, row right edge 6 from the window edge, top 28 above the first icon row top (cell y 933..957, first icon row at 961); selected cell has a light plate | exact ±1 | R03, R03u (95.7 / 3 = 32) |
| Bottom right | text rows | six rows, right-aligned, right ink edge ~8 from the window edge: time, outdoor temperature, weather, hours-left-of-weather, date, season; cap 10; pitch ~25.6 (baselines 793, 819, 845, ~870, ~895, 921); last baseline 12 above the speed cells top; alerts end above the time row (first text row centre 34 below the last alert centre in R07) | exact ±1 (pitch ±1) | R03, R07, R20b |
| Cell/room readout | text | hover-driven, 3 lines, no backing plate, cap 10, line pitch 19, left ink edge 16 (inset ~15), last baseline 17 above the bar top (descender ends 14 above it) | exact ±1 | R03, S_R03 (matches), R03_720 |

Rows:

| Id | RimWorld | Ours today | State | Change | Files | Phase |
|---|---|---|---|---|---|---|
| UIS-040 | Main tab bar 34 high, full width, flush bottom | 28 high (`hudlayout/layout.go:14` TabsH, `:151`); `hud/tabs.go:33` and `hud/tabs.go:45` use it | done (UIS-2b) | Set the bar-height token to 34 (every anchor that subtracts TabsH follows: pane, gizmos, Architect, right column stack, cell readout); the bar spans the full width as now | `hudlayout/layout.go`, `hud/tabs.go` | UIS-1 token, UIS-2b |
| UIS-041 | 12 tabs: 9 text tabs (ours plus Quests between Research and World) + 3 icon tabs (History, Factions, Menu) | 11 tabs, all text: Architect, Work, Schedule, Assign, Animals, Wildlife, Research, World, History, Factions, Menu (`hud/tabs.go:21-33`); no Quests tab | partial (UIS-2b: icon tabs and widths in; no Quests tab, UIS-066) | Add a Quests text tab between Research and World (wired to the Quests window; UIS-066), then reorder to match; render History, Factions and Menu as icon tabs (art in `assets`, drawn by us, label moves to tooltip). Keep Factions disabled-with-reason | hud/tabs.go, ids/ (tab ids), assets/ (3 tab icons, through skill `rimworld-go-art-assets`) | UIS-2b |
| UIS-042 | Text tab width = floor(W_logical / 10.5); icon tab = half of it; last tab absorbs the remainder; 2 px separators | `w := tr.W / float32(n)`, all tabs equal, so 1/11 of the width (`hud/tabs.go:50-52`) | done (UIS-2b) | Replace the equal split with the rule: unit = floor(W / (9 + 3 x 0.5)); text tabs = unit, icon tabs = unit / 2 (floor), last icon tab = W minus the rest. Button.X/W follow; keep tooltips and Selected state | `hud/tabs.go` | UIS-2b |
| UIS-043 | Tab label cap 10, left-aligned at ~14.8 % of tab width, never ellipsized at 1280 | label centred and ellipsized by `drawButtonLabel` (`hud/tabs.go:11-12` comment, `hud/deps.go:206`) at Body 13 | done (UIS-2b) | Draw the tab label left-aligned at 0.148 x tab width with the cap-10 label size (UIS-012 maps the size); baseline 20 below the bar top. Tab widths from UIS-042 leave room for the longest label (55 wide) in 121 at 1280 so ellipsis should not trigger | hud/tabs.go, hud/deps.go | UIS-2b (size token UIS-1) |
| UIS-044 | Pane outer 432 x 165, fixed in logical px; no margin | PaneW 400, PaneH 188 (`hudlayout/layout.go:16-17`); card mode: height 168, tab width 96, `config/hud.go:87`, min/max `config/hud.go:169-170`, `dossier/sheet/scale.go:20` DockMargin 34 | done (UIS-2b) | Set PaneW 432 and PaneH 165 (outer, frame included). The docked colonist card keeps its own size options as an extra only we have (kept), but its default height becomes 165 and its default width 432 so the first-run pane equals RimWorld's. DockMargin and the pane's inner row budget must be rechecked with the shorter pane | hudlayout/layout.go, internal/platform/config/hud.go (UIS-1); dossier/sheet/scale.go, inspector/hud_inspect.go (UIS-2b) | UIS-1 token, UIS-2b |
| UIS-045 | Pane tab row 30 high, 6 equal 72-pitch tabs fill the pane; storage case: one 70 x 28 tab right-aligned; buildings: no tab row | PaneTabsH 24 (`hudlayout/layout.go:18`); tab widths measured from text, `max(48, textW + 20)` (`inspector/hud_inspect.go:299-322`); fixed-width path exists via `TabW` (`:326-336`) | done (UIS-2b) | Set the strip height token to 30. For a pawn pane use `fixedTabRects` with 6 equal tabs that fill the pane width (width = pane width / 6, gap 2); for other kinds keep the single right-aligned fixed tab at 70 wide. Label size cap 9 | `hudlayout/layout.go`, `inspector/hud_inspect.go`, `inspector/pawn_state_ui.go` (Spec.TabW for pawns) | UIS-1 token, UIS-2b |
| UIS-046 | Title row: name cap 15, baseline 28 below the top, inset 12; 3 icon buttons right-aligned ending ~12 from the right edge | Name in `gfx.Large` 16 bold at `p.Y + paneTop(8)` (`inspector/pane_header.go:85-86`, `inspector/hud_inspect.go:88`); head buttons 20 + gap 2 from the close button (`pane_header.go:92-99`) | done (UIS-2b) | Size the name by cap height 15 (UIS-012), set its baseline 28 below the pane top; place buttons right-aligned 12 from the edge with RimWorld's glyph order (jump, info, expand/drop-down). No close button on the docked pane (RimWorld has none) | `inspector/pane_header.go`, `inspector/hud_inspect.go` | UIS-2b |
| UIS-047 | 4 status bars 93 x 16, gap 6, inset 12, top 36 below the pane top, label cap 8 inside, left inset 3 | `paneBarCols = 4`, `paneBarGap = 8`, fill 6 under its label (`inspector/hud_inspect.go:95-97`); cell width from the inner width (`:262-266`) | done (UIS-2b) | Make the bar a 16-high cell with the label inside; width 93 at pane 432 (= (432 - 24 - 3 x 6) / 4 + 3, keep inset 12 each side and gap 6), gap 6, label cap 8 | `inspector/hud_inspect.go`, `inspector/pane_body.go` | UIS-2b |
| UIS-048 | Pane body text cap 10, pitch 16 | `gfx.Body` 13 with `LineStep` (`gfx/text.go:267`); rows `PaneRowH` 20 (`inspector/hud_inspect.go:101`) | done (UIS-2b) | Set the pane body text to the cap-10 style and its line pitch to 16; `PaneRowH` follows 16 for plain lines (table-style rows keep their own pitch, see the Gear panel rows) | `inspector/hud_inspect.go`, `inspector/pane_body.go`, `gfx/text.go` | UIS-1 (size), UIS-2b |
| UIS-049 | Open pane-tab panels: Log 630x510, Gear 460x450, Social 540x510, Bio 514x489, Needs 580x520, Health 630x430; x 0, bottom touches the tab row (0 gap) | one width range `ExpandMinW 400..ExpandMaxW 560`, height from content (`hudlayout/layout.go:23-24`, `placeExpanded` `:370-386`), 8 px gap above the tab row (`:375` `room := ... - HudMargin`) | done (UIS-2b) | Add a per-tab size table (token: width, height per tab id) in `hudlayout` and use it as the wanted `ExpandedW/H`; remove the HudMargin gap so the panel sits directly on the tab row; each tab's body scrolls inside the fixed size | `hudlayout/layout.go`, `inspector/hud_inspect.go`, `inspector/gear/tab.go`, `inspector/log_ui.go`, `inspector/bio_ui.go`, `inspector/health_ui.go`, `inspector/social_ui.go` | UIS-1 token, UIS-2b |
| UIS-050 | Gizmo 75 x 75, stride 80, first x = pane right + 14, bottom row 16 above the bar top | CmdCell 64, CmdStride 68 (`hudlayout/layout.go:26-27`); first x = `Pane.Right() + HudMargin` (`:284`), bottom = `Tabs.Y - HudMargin` (`:294`) | done (UIS-2b) | Set CmdCell 75 and CmdStride 80; add a dedicated "command grid left gap" 14 (pane right to cell) and "bottom clearance" 16 (cell bottom to bar top) in place of HudMargin; keep `CmdReserveCols` honest at the new stride | `hudlayout/layout.go`, `inspector/hud_commands.go` | UIS-1 token, UIS-2b |
| UIS-051 | Gizmo hotkey top-left inset 6/6, cap 8, no plate; label cap 8 bottom-anchored with baseline 6 below the cell, two lines stack up, one long word ellipsizes; icon art ~36-40 | Hotkey on a filled plate at +2/+2 in `gfx.Small` (`inspector/hud_commands.go:298-302`); label inside the cell at `r.Bottom() - 2` (`:280`), min size 9 shrink before wrapping (`:634-654`); icon `cmdIconMax` 32 (`:83`) | kept (UIS-label-fit, user 2026-11-07: text never leaves its box. The label block is laid out from its measured height so its bottom is `CmdLabelPadB` 2 above the cell's bottom; a two-line label shrinks the icon (min `CmdIconMin` 24) and a label too tall for two lines ellipsizes to one. Rows are back to `CmdStride`; guards `TestCommandTextStaysInItsCell`, `TestButtonTextStaysInsideItsButton`) | Drop the hotkey plate, move the letter to inset 6/6; anchor the last label line's baseline at cell bottom + 6 (so a single line straddles the frame, two lines rise into the cell); remove the font-shrink step (`cmdLabelMinSize`) in favour of wrap-then-ellipsis at the cap-8 size; raise `cmdIconMax` to 40 | `inspector/hud_commands.go`, `inspector/cmds_pawn.go` | UIS-2b |
| UIS-052 | Gizmo overflow: not observed in RimWorld (max 6 in one row, 18 columns of room at 1920, ~10 at 1280); no row-drop seen | `CommandCells` truncates to `min(len(cmds), len(cells))`, so commands beyond `CmdMaxRows` 4 rows x columns are silently dropped (`inspector/hud_commands.go:149`); layout caps at `CmdMaxRows = 4` (`hudlayout/layout.go:28`, `:295`) | done (UIS-2b; past the room the last cell is "+N more") | Never drop a command silently: when the row is full continue in a further row upward (cells keep 75/80), lifting `CmdMaxRows` until the top reaches the pane/tab rows; if still more, page with a small "more" cell that reports the count. Needs one RimWorld capture with a selection of 20+ gizmos (a large multi-select or an Architect category) to confirm RimWorld's own wrap; until then this row is `open` with the wrap behaviour an estimate | `hudlayout/layout.go`, `inspector/hud_commands.go`, `inspector/multiselect.go` | UIS-1 (CmdMaxRows token), UIS-2b |
| UIS-053 | Architect category panel 200 wide: 2 x 6 buttons of 100 x 32, flush (no pad/gap), row-major; 12 categories; search row (field + magnifier) | `ArchCatsW` 212, `ArchCatsH` 216: 7 rows x 2 buttons of 96 x 26, pad 8, gap 3 (`hudlayout/layout.go:65-66`, `architect/panel.go:30-33`); button rects `architect/tool.go:154-158`; no search field | done (UIS-2b) | Set the category panel to 200 x ~220: 6 rows (our Storage category moves into the 12 or gets a 13th cell; decide with the Architect owner) of 100 x 32, zero pad/gap, order row-major; add the search field row (28 high) that filters designators by name | `hudlayout/layout.go`, `architect/panel.go`, `architect/tool.go`, `architect/catalog.go` | UIS-1 token, UIS-2b |
| UIS-054 | Architect description panel fixed 200 x 270, gap 0 above the category panel, always shown; body cap 10 pitch 16, inset 8 | `ArchDescW` 320, content-sized height (`architect/panel.go:105` `DescHeight`), 4 px gap above the category panel (`hudlayout/layout.go:264-265`); does not follow hover (tooltip shows the hovered tool) | done (UIS-2b) | Fix the box at 200 x 270 flush on the category panel; show the hovered designator's name, cost lines and description inside it (RimWorld does), with the text scrolling/clipping if long; keep the existing tooltip only when the box is hidden | `hudlayout/layout.go`, `architect/panel.go`, `architect/tool.go` | UIS-1 token, UIS-2b |
| UIS-055 | Designator grid starts at x = 210 (category panel right + 10), cells as gizmos (75/80), one row bottom-aligned with the gizmo row | grid x0 = `ArchCats.Right() + HudMargin` (`hudlayout/layout.go:268`), cells 64/68, up to 4 rows | done (UIS-2b) | Use the UIS-050 cell metrics and a 10 px gap from the category panel; same row rules as UIS-052 | `hudlayout/layout.go`, `architect/panel.go` | UIS-1 token, UIS-2b |
| UIS-056 | Play-settings grid: 24 icons, pitch 28 x ~28.5, 5 columns, rows 3 + 5 + 5, right edge 3 from the window edge, bottom 6 above the bar | 24 icons, gap 4, 8 per row, pad 4 inside the 232-wide Global rect (`hud/global.go:37-44`, `:240-253`); Global rect bottom = `Tabs.Y - HudMargin` (`hudlayout/layout.go:158`) | done (UIS-2b) | Set `GlobalPerRow` to 5; set right inset 3 and bottom clearance 6; keep 24 / gap 4. Vertical pitch 28 or 29 (use 28; RimWorld ~28.5) | `hud/global.go`, `hudlayout/layout.go` | UIS-1 token, UIS-2b |
| UIS-057 | Speed buttons 4 x (32 x 24), no gap, right edge 6 from the window edge, 28 above the first icon row top | 4 x (52 x 24), gap 4, `right - 4*52 - 3*4` (`hud/global.go:42-43`, `:254-259`) | done (UIS-2b) | `speedBtnW` 32, gap 0 between speed cells, right inset 6, vertical gap above the icon grid 4 (cell bottom 957 to icon top 961) | `hud/global.go` | UIS-2b |
| UIS-058 | Bottom-right text rows: six right-aligned rows, cap 10, pitch ~25.6, right ink edge ~8, last baseline 12 above the speed row | `TextRowH()` = Body `LineStep` (`hud/global.go:202`), rows built upward from the speed row by `LayoutGlobal` (`:261-300`), inner pad 4 | done (UIS-2b) | Set the row pitch token to 26 (RimWorld 25.6) and the right inset to 8; keep the order time / temperature / weather / hours / date / season (check that our ordering matches; ours has Reason/Hour/Date/Weather/Temp, `hud/global.go:228`) | `hud/global.go`, `hud/clock_text.go` | UIS-1 (pitch), UIS-2b |
| UIS-059 | Cell/room readout: 3 lines, no plate, cap 10, pitch 19, left 15, last baseline 17 above the bar | boxed text with backing, pad 6, at (8, `Tabs.Y - 8`), Body 13 with a Label heading line (`hud/cellinfo.go:103-126`) | done (UIS-2b) | Remove the backing plate, left = 15, line pitch 19, baseline of the last line 17 above the bar top; no heading style change (all three lines same colour weight). Keep the room stats overlay (rooms.go) separate | `hud/cellinfo.go` | UIS-2b |

### Windows, float menu and dialogs (UIS-060 to UIS-079)

Measured:

| Element | Measure | Value | Confidence | Shots |
|---|---|---|---|---|
| Tab bar (reference; UIS-040) | height; every main-tab window's bottom edge sits flush on its top | 34 (a 2 px lit line then 32) | exact +-1 | R09 to R17b, R13 |
| Main-tab window anchor | x of left edge; bottom; gap to the tab bar | x = 0 (flush, no margin); bottom = tab bar top; gap 0 | exact +-1 | R09, R10, R11, R13, R14q, R16, R16f |
| Menu panel anchor | right edge, bottom | flush right (x = screen width) and flush on the tab bar | exact +-1 | R17, R17b, R17b_720 |
| Main-tab windows: frame | title bar, close X, scrim | none of the three; a 1 px frame, flat panel fill, game keeps running, map not dimmed | exact | R09 to R16f |
| Main-tab windows: inner margin | left/top/right/bottom inset of content | 18 (Quests list at x 18, History tabs at x 18 and graph to 18 from right, Factions rows start 18) ; Work/Schedule/Assign tables start at x 8 (name column hugs the left) | +-2 | R14q, R16, R16f, R09, R10 |
| Work window | outer size with 3 colonists, 20 work columns | 1035 x 245 (interior 1033 x 243), top at y 800 | exact +-1 | R09 |
| Work window at 1.25x | same window | 1034 x 245 logical (1292 x 306 captured): uniform scaling, no re-fit | exact +-1 | S_R09 |
| Work table | row pitch; priority cell; cell stride; name column to first cell | 30; 25 x 25 box (row band 29); 39; 240 (cell 1 starts at x 240) | exact +-1 | R09, S_R09 |
| Work table header | title row to first colonist row; header label rows | first row at +97 from window top; two staggered label rows (pitch 20) plus a name-column header row at +84 | +-2 | R09 |
| Work/Schedule/Assign bottom padding | last row bottom to window bottom, with 3 colonists | 59 to 60 (same in all three) | exact +-1 | R09, R10, R11 |
| Schedule window | outer size with 3 colonists | 1010 x 221 (interior 1008 x 218), top at y 826 | exact +-1 | R10 |
| Schedule table | hour cell; gap; row pitch; cell height; first row below top | 19 wide, 2 gap (stride 21), 28 tall, row pitch 30, first row at +70 | exact +-1 | R10 |
| Schedule top buttons | brush buttons; offset from window top | 28 tall, 91 to 99 wide, 7 below the top, first at x 8 | +-1 | R10 |
| Schedule area column | 3 buttons per row | 88 wide each, 4 gap; column starts 7 right of the hours | +-2 | R10 |
| Assign window | outer size with 3 colonists | 1375 x 221, top at y 826 | exact +-1 | R11 |
| Assign table | policy button; stride; row pitch | 250 x 28 inside a 255 stride (4 columns), row pitch 30; first row at +70 | +-1 | R11 |
| Wildlife window | outer size; anchor | 515 wide, top y 0, bottom on the tab bar (about 1046 tall, full height); flush left | exact +-1 | R13 |
| Wildlife table | header; row pitch; scroll track | header band 32; row pitch 30 (46, 76, 106...); vertical track at the right edge about 10 to 12 wide (thumb about 6) | +-2 | R13 |
| Animals window with no tame animals | frame | nothing is drawn: no frame, no empty-state text | exact (observation) | R12 |
| Research window | outer size | 1920 x 731, top at y 315 (full screen width, bottom on the tab bar); the colonist bar and the map stay visible above | exact +-1 | R14 |
| Research layout | left info column; top row; tab; search field | info text from x 18 to about 425; tree frame starts x 443; tab 198 x 37 at y 335 (20 below top); search field 174 x 24 at the right, 18 from edge | +-2 | R14 |
| Research tree | project box; stride | 140 x 40; 190 across, 70 down | exact +-1 | R14 |
| Research scroll | both bars | vertical about 12 to 13 wide at the right inside edge, horizontal about 12 to 15 tall along the bottom; content is wider than the window so both show | estimate | R14 |
| Quests window | outer size; split | 1010 x 641, top y 405; left list 351 wide (x 18 to 369), right detail from x 386 to 992 (606), list top 428 (tab strip 428 to 460) | +-2 | R14q |
| Quests controls | tab; search field; bottom button | 3 tabs, about 32 tall (widths 120 / 107 / 120); field 325 x 22 with 18 px icon; button 350 x 38 at the bottom of the list | +-2 | R14q |
| History window | outer size; tabs; period buttons | 1010 x 641 (same as Quests); 3 tabs 196 / 180 / 196 wide x 31 tall at y +31; chart x 78 to 992, y 500 to 905; 4 period buttons 108 x 38 | +-2 | R16 |
| Factions window | outer size; row; | 1010 x 685, top y 362; faction row 80 tall, pitch 80, stripe from x 18 to 976 (a 16 px gutter right for a scroll bar); icon 32, info glyph 14 | +-2 | R16f |
| Menu panel (R17 and the same in R17b) | outer size; buttons | 450 x 391 (interior 448 x 388), flush right and on the tab bar; 5 buttons 168 x 44, pitch 52 (8 gap), first at 19 from the left and 19 from the top; a link list in the right 250 | exact +-1 | R17, R17b |
| Menu panel at 1280x720 | size and anchor | identical 450 x 391, same corner (bottom-right above the tab bar) | exact +-1 | R17b_720 |
| Float menu | row pitch; row interior | 29 (28 interior + 1 divider); 1 px frame, rows share one frame | +-1 | R18, S_R18 |
| Float menu width | rule | content-fitted: label width + 48 (212 + 48 = 260 at 1x; 327 captured = 261.6 at 1.25x); no minimum seen; rows without an icon share the width | +-2 | R18, S_R18 |
| Float menu row padding | left pad; icon; gap to text; right pad | 6; 19 x 19 icon at the left of an icon row; text starts 32 from the left on an icon row, 6 on a plain row; 13 | +-1 | R18 |
| Float menu position | anchor to click | top-left of the menu at about the click point (pawn name label floats 15 above it, outside the frame); no footer hint row | estimate (click not recorded) | R18, S_R18 |
| Float menu text | body cap height | 10 (same as every window body text) | exact +-1 | R18 |
| Rename dialog | outer size; anchor | 490 x 308 (interior 488 x 306), centred on the whole screen (centre 959.5, 539.5), no title bar, no X, no scrim, game pauses | exact +-1 | R20a |
| Rename dialog layout | margins; row pitch; field; buttons; title | margin 18 all round; rows 34 apart; field 234 x 30 (interior 28); action button 113 x 30 beside it; two bottom buttons 216 / 217 x 33 filling the width with a 20 gap, 18 from the bottom; title cap height 15 | exact +-1 | R20a |
| Storage settings panel (content) | top button; search; row pitch | panel about 300 x 478; top button 158 x 27, 9 below the panel top; list rows 22 apart; close X about 18 at the panel's top-right | +-2 | R20b |
| Letter window | outer size; anchor | 620 x 480 (interior 618 x 478), centred on the whole screen, no title bar, no X; text starts 17 from the left and 22 below the top; body text cap 10 | exact +-1 | R21b |
| Letter window actions | form; pitch | plain text rows (no bevelled button), pitch 29, indented 15 beyond the text margin, the last row's text ends 28 above the bottom | +-1 | R21b |
| Keep/revert dialog | outer size; anchor | 500 x 300 (interior 497 x 297 at 1.25x, 497.7 x 297.7 at 3.5x), centred on the whole screen, over the Options window | exact +-1 | R01g, R01h_u |
| Keep/revert buttons | size; gap; inset | 210 x 32 each, gap 42, 18 from the sides and bottom; message text 18 from the left, cap about 21 below the top | +-1 | R01g, R01h_u |
| Options window | outer size; anchor | 650 x 600 (interior 648 x 598), centred on the whole screen; same size at 1080 and 720 (R01a_720: window at y 60 to 659 of a 720 screen) and at 1.25x and 3.5x | exact +-1 | R01a, R01a_720, R01g, R01h_u |
| Options side tab list | tab; pitch; inset; selected state | 152 x 40, pitch 50 (10 gap), 22 from the left edge, first 21 below the top; selected tab gets a 3 px outline so it reads 158 x 46 | exact +-1 | R01a, R01c |
| Options content | label x; value button; row pitch | labels at 194 from the window's left edge; value buttons 173 x 28 from 457 to 630; check marks at 457; row pitch 35 | +-1 | R01a, R01c |
| Options OK button | size; position | 154 x 39 (bevel included), centred horizontally, 18 above the bottom | +-1 | R01a |
| Options close X | glyph; inset | 18 x 18 glyph, 4 from the top and 4 from the right of the outer frame | +-1 | R01a, R01a_720 |
| Window title text | cap height | 15 for a window title (rename title, research title); 10 for all body, label and button text | exact +-1 | R20a, R14 |
| Button heights in windows | three kinds | window action button 38 to 39; dialog action button 32 to 33; menu button 44; options row value button 28 | +-1 | R16, R01a, R20a, R17b, R01g |
| Small-window behaviour | all windows | no window is re-fitted or shrunk when the screen is 1280x720: Options and Menu keep 650 x 600 and 450 x 391 and the same anchors; windows scale only through the UI scale (S_R09) | exact (R17b_720, R01a_720), others not captured | R17b_720, R01a_720, S_R09 |

Rows:

| Id | RimWorld | Ours today | State | Change | Files | Phase |
|---|---|---|---|---|---|---|
| UIS-060 | Main-tab windows: no title bar, no X, no scrim; 1 px frame, 18 inner margin; the tab bar click or Esc closes. Window outer sizes are content-fitted (rows below) | Kit-based windows (Assign, History, Factions) are modals with a title bar 40, a footer 52, a close X 28 and pad 16 (`uikit/modal.go:21-25`, `modalParts` 66-72); Work/Schedule/Research/Animals are hand-drawn with an in-panel title at +6 (`windows/schedule.go:100`) | done (UIS-2c) | Give `RunWindow` (the main-tab frame) a frameless variant: no title bar, no X, pad 18, title drawn as a window title (cap 15) only where RimWorld shows one; keep the modal frame for true dialogs. Token `WindowPad = 18`, `WindowEdge = 1` | uikit/modal.go (frameless variant, UIS-1 contract), windows/window.go, hudlayout/layout.go (token) | UIS-1 (variant, token), UIS-2c |
| UIS-061 | Work window: 1035 x 245 for 3 colonists; row pitch 30; cell box 25, stride 39; name column 240; first row at +97; 59 to 60 bottom padding; grows with colonists | Row 22, cell stride 46 (box 42 x 20), name 110, first row at +48, bottom pad 20, 14 columns, `want()` = `12 + 110 + 14*46 + 8` = 774 wide (`windows/work.go:73-76, 143-154, 292-299`) | partial (UIS-2c: window and rows in; cells 35 wide) | Set `workRowH = 30`, `workCellW = 39` (box 25 x 25), `WorkNameW = 240` (portrait + name), first row offset 97, bottom pad 60, left pad 8; two staggered header label rows; column count stays ours (14 vs 20) and width follows | `windows/work.go` (consts and `want`), draw in the same file | UIS-2c |
| UIS-062 | Schedule window: 1010 x 221 for 3 colonists; cell 19 + 2 gap, 28 tall, row pitch 30; area column 3 x 88; first row at +70; bottom pad 60 | Cell 26, row 24, area 170 + gap 8, `SchedTop = 78`, bottom pad 20 (`windows/schedule.go:27-31, 89-95`) | partial (UIS-2c: the area column is 170, not 270) | `SchedCellW = 21` (19 box + 2), `SchedRowH = 30` (28 box), `SchedTop = 70`, area column 3 buttons of 88 with 4 gaps (270) and gap 7, bottom pad 60 | `windows/schedule.go` | UIS-2c |
| UIS-063 | Assign window: 1375 x 221 for 3 colonists, 4 policy columns of 250 x 28 at stride 255, row 30, first row at +70, bottom pad 60, top row of 4 manage buttons 250 x 28 | Modal 880 x 480 fixed with title and footer (`windows/assign.go:29-31, 84-85`) | done (UIS-2c) | Make Assign a content-fitted frameless window (width = name col + 4 x 255 + carry col, height = 70 + rows x 30 + 60) with the same anchor as the others; columns follow the policies we have | `windows/assign.go`, `windows/window.go` | UIS-2c |
| UIS-064 | Animals window empty: draws nothing at all; Wildlife: 515 wide, full height (0 to the tab bar), header 32, row 30, scroll track about 12 | Animals/Wildlife width `colonyFullW` / `wildFullW` by column sum, rows `animalsRowTop = 84` + 30 each, height `max(160, ...)` (`windows/animals.go:62, 80-81, 129-137`) | partial (UIS-2c: Wildlife narrower than 515, no scroll bar) | Wildlife: header band 32 (`animalsRowTop = 32`), width follows columns but target 515 for the species table; list height to fill the screen down to the tab bar when rows overflow; scroll thumb inside a 12 px track. Animals with no tame animals: keep our empty-state text (`kept`, cheaper than a missing window) | `windows/animals.go`, `uikit/scroll.go` | UIS-2c |
| UIS-065 | Research window is as wide as the screen (1920 at 1080) and 731 tall (bottom on the tab bar, top 315); a vertical and horizontal scroll over a tree wider than the window; project box 140 x 40, stride 190 x 70; tab 198 x 37; search 174 x 24 | Fixed 1600 x 1000 wanted, clamped by the layout to `min(WindowW, rx - 8)` and the room under the bar (`windows/research.go:36-49`, `hudlayout/layout.go:251-256`) | partial (UIS-2c: Research capped short of the screen width) | Want = (screen width, max(480, 0.68 x (screen height - tabs))) at 1x; verify the fraction with one more shot at another height before freezing (one data point: 731 of 1046 = 0.70). Project box 140 x 40, stride 190 x 70, tab 198 x 37 | `windows/research.go`, `windows/research_layout.go`, `windows/research_tree.go` | UIS-2c |
| UIS-066 | Quests window 1010 x 641: left list 351 with 3 tabs, right detail 606, search and a bottom button; reachable from the tab bar | No Quests tab or window (the tab strip has 11 equal tabs, no Quests: UIS-041); contract offers live elsewhere | open | Feature gap, size only: if a Quests window is added, use 1010 x 641, split 351 / 606, margin 18; do not add before the tab bar row (UIS-041) | `windows/` (new), `hud/tabs.go` | UIS-2c (after UIS-041 in UIS-2b) |
| UIS-067 | History window 1010 x 641 (shares Quests' size), tabs 31 tall at 18, chart to 18 from the right, period buttons 108 x 38 | Modal 640 x 520, tab 28, row 22 (`windows/history.go:22-23, 61-62`) | done (UIS-2c) | Frameless window 1010 x 641; tab height 31; button height 38 | `windows/history.go` | UIS-2c |
| UIS-068 | Factions window 1010 x 685 for 6 factions; row 80 tall; stripe 18 to 976 with a 16 gutter | Modal 1120 x 640 (`windows/factions.go:23-24`) | partial (UIS-2c: 1010 wide, height fitted; rows stay 34 high) | Frameless 1010 wide, content-fitted height (header 150 + rows x 80 + 55), row height 80 | `windows/factions.go` | UIS-2c |
| UIS-069 | Menu panel: 450 x 391, bottom-right above the tab bar, same for the menu icon and Esc; 5 buttons 168 x 44, pitch 52, 19 inset; game keeps its look (no scrim) | Pause screen: centred 360 x 360 over a scrim, 280 x 36 buttons, pitch 44, 7 entries (`screen_pause.go:13-21, 34`) | done (UIS-2c) | Anchor the in-game menu bottom-right above the tab strip, 450 x 391 (token), button 168 x 44, pitch 52, inset 19; keep our 7 entries by letting the panel grow in 52 steps; drop the scrim for this panel (pause itself is a separate rule) | screen_pause.go, hudlayout/layout.go (token) | UIS-1 (token), UIS-2c |
| UIS-070 | Float menu: row pitch 29, width = longest label + 48 (icon column included), no minimum, row padding left 6, icon 19, text offset 32 on icon rows, right pad 13 | Row 22, min width 200, width = text + 28, icon 16 at the right, text at +10 (`orders/float.go:24-27, 54-80, 118-126`) | done (UIS-2c) | `FloatRowH = 29`, drop `floatMinW`, width = TextW + 48, icon 19 at x +6, text at +32 (icon rows) or +6 (plain rows), right pad 13; frame 1 px not 2 | `orders/float.go`, `hudlayout/layout.go` (token) | UIS-1 (token), UIS-2c |
| UIS-071 | Float menu opens with its top-left at the click; no hint footer | Opens at click + (6, 4); footer row `FloatHintH = 18` with a Shift-queue hint (`orders/float.go:27, 106-108, 161`) | done (UIS-2c) | Open at the click point (0, 0 offset), keep the screen/tab-strip clamp; move the Shift hint into the row tooltip or the HUD so the menu is only rows | `orders/float.go` | UIS-2c |
| UIS-072 | Rename dialog: 490 x 308 centred, no title bar or X, margin 18, row 34, field 234 x 30, two bottom buttons 216 x 33 | Modal 380 x 180 with title bar 40, footer 52, pad 16 (`inspector/pane_header.go:291`, `uikit/modal.go:21-25`) | done (UIS-2b) | Dialog frame variant with no title bar (title as heading text inside, cap 15), pad 18, buttons 33 tall splitting the width with a 20 gap; size from content with 490 as the width token | inspector/pane_header.go (uses the UIS-1 frameless variant) | UIS-2b |
| UIS-073 | Letter window: 620 x 480 centred, no title bar or X, text margin 17 / top 22, actions as text rows pitch 29 | Modal 560 wide, height from the text, title 40, footer 52 (`hud/letters.go:33, 290-298`) | partial (UIS-2a: 620 wide, 480 high minimum; `RunModals` still draws the title and X) | Frameless dialog 620 x 480 (480 as a minimum, grows if the text needs it), pad 18, actions as plain rows at pitch 29 instead of footer buttons; Esc still closes | hud/letters.go (uses the UIS-1 frameless variant) | UIS-2a |
| UIS-074 | Keep/revert dialog after a UI scale change: 500 x 300 centred over Options, buttons 210 x 32 (gap 42), a visible countdown | Missing: no confirmation or revert exists for the UI scale preset (no `revert` code in `options/`) | n/a | Merged into UIS-007 (sizes there). | none | n/a |
| UIS-075 | Options is a 650 x 600 window centred on the screen, same at every window size and scale; tab list on the left, close X, OK at the bottom | Full-screen panel inset 40 (`options/screen_options.go:539`), horizontal tabs 112 x 32 at y 108, rows at 44 starting y 164, Back 140 x 36 (`screen_options.go:97-103, 449, 475`) | partial (UIS-2e: 650 x 600 centred; no 18 x 18 close X yet, the keep/revert buttons use the kit size) | Re-lay Options as a centred 650 x 600 window: vertical tab list 152 x 40 pitch 50 at (22, 21), content origin x 194, rows pitch 35, value buttons 173 x 28 at x 457, OK 154 x 39 centred 18 above the bottom, X 18 x 18 inset 4. Rows that do not fit scroll inside the window (RimWorld Interface tab has 13 rows and still fits, so most of ours will need a scroll) | `options/screen_options.go`, `options/rows_helpers.go` | UIS-2e |
| UIS-076 | Options value button 28 tall and row pitch 35; window action button 38 | Control height 32 (`OptBtnH`), step 44 | n/a | Merged into UIS-016 (the control is 30 outer: a 28 face plus bevel). | none | n/a |
| UIS-077 | Scrollbar track about 12 wide in lists and the Research tree | `ScrollBarW = 8` (`uikit/scroll.go:16`) | done (UIS-2c) | `ScrollBarW = 12`; re-measure with a zoomed crop in UIS-3 before freezing (estimate +-2) | `uikit/scroll.go` | UIS-2c |
| UIS-078 | At 1280 x 720 no window shrinks or moves: Menu and Options keep size and corner anchor; windows scale only by UI scale | Layout clamps a main-tab window to `min(want, room)` and a modal to the screen minus 40 (`hudlayout/layout.go:251-256`, `uikit/modal.go:57-58`); dialogs under 1280x720 draw "too small" only in the setup screens (`uikit/dialog.go:52-53`) | kept (clamps stay as a safety net, they only bite below RimWorld's minimum logical size which UIS-1 enforces) | With UIS-1 refusing scales that leave less than the RimWorld minimum logical size, keep the clamps but add a test that no standard window at 1x on 1280x720 needs them | `hudlayout/layout.go`, `uikit/modal.go` | UIS-3 |
| UIS-079 | Dialogs pause the game and draw no scrim; the main-tab windows neither pause nor dim | Modals draw a dim scrim over the map (`uikit/modal.go`, `Dim`) and pause rules are per dialog | kept (the scrim helps read an unframed dialog; pause is already per dialog) | Revisit only if a visual match with RimWorld is wanted: no scrim for the rename and letter dialogs | `uikit/modal.go` | n/a |

### Map: zoom, cell size, map labels (UIS-080 to UIS-099)

Measured:

| Element | Measure | Value | Confidence | Shots |
|---|---|---|---|---|
| Default zoom, 1920x1080 1x | px per cell | 22.5 (14-cell zone spans 315 px wide, 23 tall; crate pitch 22.75) | exact ±0.5 | R23a, R03, R00 |
| Default zoom, visible cells | 1920x1080 window | 85.3 x 48.0 cells (the window height, HUD included, is 48 cells) | exact ±1 cell | R23a, R03 |
| Default zoom at 1.25x | px per cell (captured / logical) | 22.5 / 18.0 (crate pitch 180 px per 8 cells) | exact ±0.5 | S_R03 |
| Default zoom at 1280x720, 1x | px per cell | 15.0 (crate pitch 15) = 48 cells tall again | ±1 | R03_720 |
| Default zoom at 5120x2762, 3x | px per cell (captured / logical) | 57.4 / 19.1 = 48.1 cells tall | ±0.5 | R03u |
| Default zoom first launch | px per cell | 22.5 (same as R03) | ±1 | R00 |
| Max zoom in, 1x | px per cell | 49 (crate outline pitch 49); 39.2 x 22.0 cells | exact ±1 | R23b |
| Max zoom in, 1.25x | px per cell | 49 captured (crate pitch 49); same as 1x. Open question settled: the range does not depend on UI scale; S_R24b only looks closer because it is another map with bigger plant art and bigger labels | exact ±1 | S_R24b, R23b |
| Max zoom out, 1x | px per cell | 9.0 (zone is 126 px for 14 cells); 213 x 120 cells | exact ±0.3 | R23c |
| Zoom ratios | in / default, default / out, in / out | 2.18, 2.50, 5.44 | ±0.05 | R23a-c |
| Zoom step per wheel notch | factor | about 1.026 (landmark distances from the unmoved zoom centre grow 2.7 % and 2.5 % per notch); range = about 65 notches, about 30 from default to max in, 35 to max out | estimate ±0.003 | R23c, R23d, R23e |
| Zoom anchor | pivot | stays at the screen centre (base landmark x=964 fixed over 3 notches), not the cursor | exact | R23c-e |
| Does default px/cell follow UI scale? | 1x vs 1.25x vs 3x at one window height | No (22.5 captured at 1x and 1.25x) | exact | R03, S_R03, R03u |
| Does default px/cell follow window size? | 1080 vs 720 vs 2762 tall | Yes: always height/48 (22.5, 15.0, 57.5) | exact ±1 | R03, R03_720, R03u |
| Extremes at other window heights | in/out px per cell | not captured (assume they scale like the default) | not measured | none |
| Colonist name label: text | cap height | 8 at 1x, same at default, max in, max out and at 720p; 10 at 1.25x; 23 at 3x (7.7 logical). Follows UI scale only, never zoom | exact ±1 | R03, R23b, R23c, R03_720/R04_720, S_R24b, R03u |
| Colonist name label: plate | size, offset | dark translucent plate hugging the text: about 11-12 high, text width + 2-4; centred under the pawn; top edge 1.5-3 px below the sprite's feet (the sprite's feet sit about 1.5 px below the cell's bottom) | ±2 | R04, R03, R03u, R04_720 |
| Name label visibility | zoom, clutter | drawn at every zoom including max out (9 px cells), never hidden, no overlap avoidance (two names collide in R23c) | exact | R23c, R23d, R23e |
| Stack count label | text, place, plate | digit height 8 at 1x, 9 at 1.25x (UI scale); one centred tag just below the cell's bottom edge; faint plate; same tag style for item quality words under non-stack items | ±2 | R23b, S_R24b |
| Stack count visibility | zoom | absent at 22.5 px cells (R03, R23a, R00), present at 49; threshold between not pinned | ±: between 22.5 and 49 | R23a, R23b |
| Forbidden mark on items | size | drawn at every zoom where the item is, scales with the cell (about 13 px at 49, 8 at 22.5) | estimate | R23b, R03 |
| Pawn selection brackets | size | four corner pieces on the pawn's cell: square 23 px (= 1 cell) at 22.5 and 15 px at 15; arms about a third of the cell (8 and 5 px), line 1.5-2 px. Follows zoom. UI-scale dependence not captured | ±2 | R04, R04_720 |
| Building selection brackets | size | corners of the footprint (2x2 stele: 44.6 px = 2 cells), arms about 8 px at 22.5 | ±2 | R04b |
| Selected zone | look | pale uniform overlay over the zone rectangle; no text label on the stockpile zone at any zoom | exact | R23a-c |
| Far-zoom pawn | marker | none: at 9 px cells the pawn is its normal sprite scaled with the cell (about 6 x 11 px), name still drawn | exact | R23c |
| Work progress bar | all | no shot shows a timed job | not measured | none |

Rows:

| Id | RimWorld | Ours today | State | Change | Files | Phase |
|---|---|---|---|---|---|---|
| UIS-080 | Default zoom = window height / 48 per cell (22.5 px at 1080, 15 at 720, 57.5 at 2762); not UI-scale dependent | Fixed 64 px x zoom x Dev; default zoom 1 gives 64 px/tile = 2.84x too big at 1080 and not height-relative (`camera/camera.go:121-131`, `scene.go:63`) | done (UIS-2d) | Make the apparent tile `T = windowPhysH / 48 x z`, with `z = 1` the default and the camera's `TileSize` constant replaced by a function of the full window height in physical px (Dev already folded in). Equivalent at 1080 and device 1: zoom 0.3516 of today's scale. Keep `Camera.Zoom` meaning "multiple of default" so options, saves and the min/max below read the same at any window | `camera/camera.go`, `scene.go`, `renderer.go`, `render/view.go` (tile users) | UIS-2d |
| UIS-081 | Range 0.40x to 2.18x of default (9 to 49 px at 1080); 5.44x total | 0.1 to 3.0 of a 64 px tile = 6.4 to 192 px at dev 1 | done (UIS-2d) | `MinZoom = 0.40`, `MaxZoom = 2.18` in the new unit (default 1). At 1920x1080 that shows 213 x 120 cells at the far end and 39 x 22 cells at the near end. Extremes at other window heights were not measured; scale them like the default | `camera/camera.go`, `renderer.go:443-450` (`clampZoom`) | UIS-2d |
| UIS-082 | About 1.026 per wheel notch, 65 notches end to end | x1.1 per notch (`zoomStep = 0.1`, `renderer.go:53,366`), so 18 notches end to end | done (UIS-2d: wheel 1.026 a notch; a zoom key press is 4 notches, ours) | Set the wheel and keyboard step factor to 1.026; keep the existing ease (`zoomTau`). Touchpad or high-resolution wheel deltas still pass through `math.Pow(1+step, wheel)` | `renderer.go` | UIS-2d |
| UIS-083 | Zoom pivots at the screen centre | Pivots at the cursor by default (`prefs.go:168`; `camera/fx.go:129`) | done (UIS-1) | Default `ZoomToMouse` to false for new profiles; keep the option (our extra). Needs a settings migration note: existing profiles keep their stored value | internal/platform/config/prefs.go (default; existing profiles keep their value) | UIS-1 |
| UIS-084 | Tile size independent of UI scale, dependent on window height | Independent of UI scale (matches); depends on `Dev` x map render scale, not on height (`camera/camera.go:38-45`) | done (UIS-2d) | After UIS-080 the map viewport height in physical px is the only input; `Dev` and `WorldK` stay inside it. UI-scale changes must leave the tile size unchanged (add a test: tile equal at UI 100 % and 125 % for one window). The `ColonyViewport` (HUD excluded) must not be used as the 48-cell base; use the whole window like RimWorld | camera/camera.go, camera/*_test.go | UIS-2d |
| UIS-085 | Zoom-out floor is fixed (9 px), the whole 250-wide map never needs to fit | Floor is map-fit with 8 % slack, min 0.1 (`camera/camera.go:322-336`); can allow a closer or farther floor than RimWorld by map size | done (UIS-2d) | Keep a map-fit guard only for maps that are smaller than the new fixed floor would show (so black never dominates); otherwise the floor is `MinZoom` from UIS-081. Update tests that assert `MinZoomFor` | `camera/camera.go`, `camera/*_test.go` | UIS-2d |
| UIS-086 | Same detail at 9 px as at 22.5 px: all objects drawn as sprites, scaled | Level-of-detail tiers by px per tile: live at 48+, baked 16 px chunks at 16-48, whole-map 4 px bake under 16 (`render/world/lod.go:18-30,53,63`). The new default 22.5 px lands in the chunk tier (baked ground upscaled 1.4x), max out 9 px in the map tier | done (UIS-2d) | Re-derive the tiers for the new range: live ground from about 20 px (just under the default), chunk bake 16 px/tile until about 12 px, map bake under that; objects stay live down to 16 px (`CreatureMinTile`). Re-run the large-map benchmark with -race, because the default now shows 4100 cells, not 510. Check the 64 px art downscale to 22.5 px for aliasing (art owner) | `render/world/lod.go`, `render/world/chunk_cache.go`, `render/creature/far.go` | UIS-2d |
| UIS-087 | n/a | A save remembers apparent zoom (`camera.View.Zoom`, `camera/camera.go:89`) in the old unit | done (UIS-2d) | Convert on load: `newZoom = oldZoom x 64 / (H/48)` at the saved window is not recoverable, so store the new unit with a `view version` field and map old saves to the default zoom (centre kept) | camera/camera.go (`View`), scene.go (`Resume`), internal/adapters/outbound/filesave/filesave.go (`ViewFile`, save version) | UIS-2d |
| UIS-088 | Colonist name under every colonist pawn at every zoom: cap 8 logical, plate 11-12 high hugging text (+2-4 wide), top 1.5-3 px below the feet, never hidden, no declutter | We draw no colonist name (animals only: `render/creature/traces.go:88,184-196`) | done (UIS-2d) | New `DrawPawnName` beside `DrawAnimalName`: text in the Small style with a cap height of 8 at 1x (our `Small` is 11 px, cap 7.85, UIS-012), centred at the pawn's cell centre x, plate top = cell bottom + 3 logical px, plate fill dark translucent, width text + 3, height 12. Draw for every colonist, always, after the pawns and before the HUD; the read model already holds the name | `render/creature/traces.go` (or new `render/creature/names.go`), `render/world/draw_art.go`, `render/world/labels.go` | UIS-2d |
| UIS-089 | Name text follows UI scale, not zoom (cap 8 at 1x, 10 at 1.25x, 23 at 3x) | Animal plate: `gfx.Small`, plate padding 6, height 14, text raised 2, scaled by `UIScale` (`traces.go:146-152,185-195`) | done (UIS-2d) | Same rule for colonists via `gfx.WorldText`. Align the animal plate to the colonist one (padding 3, height 12) so both read as one family; a tame animal's name need not hide below 36 px if RimWorld also shows names at far zoom (animal labels are not in our shots, so keep `NameMinTile` until measured) | `render/creature/traces.go` | UIS-2d |
| UIS-090 | Stack count: centred tag below the cell's bottom edge, digit height 8, shown at 49 px cells and hidden at 22.5 | Bottom-right inside the tile, `Small` plate `Shade(190)`, only from 36 px (`render/world/labels.go:17-67`, `StackCountMinTile = 36`) | done (UIS-2d) | Move the tag to centred under the cell (x = cell centre, y = cell bottom + 1 logical px) and keep 36 px as the threshold (between RimWorld's 22.5 and 49; it hides at the new default, shows at max zoom). Keep stored-stack placement rules in `storedCount` | `render/world/labels.go`, `render/world/neat_*.go` (stored counts) | UIS-2d |
| UIS-091 | No label on a stockpile zone at any zoom (selected or not) | Zone names drawn: 0.45 x short side, 11-40 px, grow with zoom-out (`render/overlay/zones_draw.go:171-180,541-542,599-616`) | kept (checked by `TestFrameInvariance` at zoom 2) | Keep our zone labels (read-only HUD value, PLAY-017). Only check they are not drawn on the stockpile zone at the new default; no sizing change | `render/overlay/zones_draw.go` | UIS-2d |
| UIS-092 | Selection brackets: pawn = 1 cell square, arms about a third of the cell, 1.5-2 px; building = whole footprint; scale with zoom | `overlay/selection` sprite drawn at tile size (`render/world/draw_art.go:338`, `multiselect_render.go:244-296`) | done (UIS-2d) | Replace the ring sprite by (or restyle it as) corner brackets at the footprint rectangle, arm = cell/3, thickness max(1.5, cell/14) in physical px, white; follows zoom only. Art is original (corner pieces drawn in code) | `render/world/draw_art.go`, `multiselect_render.go`, `assets/` (sprite removed if orphaned) | UIS-2d |
| UIS-093 | Pawn at far zoom is the plain sprite scaled by the cell (about 6 x 11 px at 9 px cells); no dot or silhouette marker | Optional far markers: 6 px dot or 7x8 silhouette, `farDot`, `farBodyW` (`render/creature/far.go:27-37`); creatures drawn from 16 px (`lod.go:63`) | kept | Keep the OPT-016 markers as our extra, default off to match RimWorld, and lower `CreatureMinTile` use to the new 9 px floor so sprites show down to it | render/creature/far.go, render/world/lod.go (UIS-2d); the default in internal/platform/config (UIS-1) | UIS-1 (default), UIS-2d |
| UIS-094 | Work progress bar: not measured | Bar 0.6 cell wide, 0.06 cell tall (min 3 px), at 0.82 down the cell, hidden under 24 px (`render/creature/progress.go:21-26,81`) | done (UIS-2d) | After the rebase the default (22.5 px) is under the 24 px threshold, so bars would vanish at the default zoom. Lower `WorkBarMinTile` to 16 and re-check visually; RimWorld bar size needs a new capture (a pawn mid-job at 1x, default and in) before sizing | render/creature/progress.go | UIS-2d |
| UIS-095 | Name, stack and quality labels all follow UI scale; they never follow zoom | Same already (stack tag in logical px, animal plate `UIScale`) | n/a | Add one test that the label cap height scales 1 : 1.25 : 3 with UI scale and not with zoom for pawn names and stack tags, in `UIS-3` | `render/world/*_test.go` | UIS-3 |

## Not measurable

Each item names what is missing. UIS-3 captures what it can; the rest stays an estimate in its row.

- Rendering at 1.75x: refused at every captured window ≤ 1080 high, and no 1.75x shot exists. Inferred from 1.25x and 3x: re-rasterised text, whole-px lines.
- Exact Hmin within (720, 768] and Wmin below 1280: no shot discriminates them. The proposal picks 768 and 1280.
- RimWorld's behaviour when the window shrinks below the current step (UIS-004), and its first-launch step on large or HiDPI windows: only 1920×1080 was captured.
- Full keep/revert countdown length: one frame showed 8 s remaining.
- Medium-font line pitch: there is no multi-line medium text in any shot. 24 is an estimate (1.6 × cap, the small font's ratio).
- Tooltip minimum width and fit-to-content: every captured generic tooltip wrapped at its maximum. Tooltip offset from the cursor: the cursor is not in the captures.
- Resources readout: empty in every shot (a fresh quicktest draws nothing top-left).
- Learning helper / tutor panel: no shot shows one (R00, R03 and R21 have no panel in the top-left or right column).
- Mood, status and need icons on a bar slot: all colonists were calm; no stress or low-mood slot was captured.
- Message stack: only one message appeared at a time (R21d); the row pitch, the stack direction, the maximum number of rows and the width rule are not measured. R21 shows no message although its description says so.
- Letters at 1.25x or 3x: R21, R21b and R21c are all 1x; the letter sizes are measured once, at 1x.
- Bar width cap versus screen width: only 1920 had many colonists.
- Message overlap fade: the exact opacity was not measured, only that the slots under a message nearly vanish.
- Gizmo overflow and wrap: no shot has more than 6 gizmos (R20b/R23b), the Architect never wraps (12 designators fit at 1920 and 1536). Needs a new capture with 20+ commands or a narrower window; see UIS-052.
- Whether the main-tab icon tabs are History, Factions and Menu in that order: the glyphs (book, building with flag, bullet list) were measured but their windows were not clicked in these shots; assumed from R16, R16f, R17 (UIS-067, UIS-068, UIS-069).
- Pane tab strip height at 1.25x and 3x is derived from the body box (same offsets), not measured separately for each tab.
- Gizmo icon art size (varies per icon, 36-40): estimate only.
- Frame/separator stroke width: 2 px at 1x and at 1.25x (1.6 logical) and 4 px at 3x (1.33 logical), so strokes are fixed-pixel textures, not scaled with the UI scale; not assigned a row because it belongs to the scale model.
- Window height with more or fewer colonists than 3, and the scroll start threshold for Work/Schedule/Assign: only 3-colonist captures (R09, R10, R11); the extra 59 to 60 bottom padding hints at a fixed bottom reserve but could not be separated from a minimum height.
- Animals window with tame animals (frame, columns, row height): none on the map (R12 draws nothing).
- Research window height rule (fraction vs fixed): one height only (1080); 731 of the 1046 above the tab bar is 0.70.
- Keep/revert countdown start value: R01g was taken after the click delay and shows 8.
- Float menu long-menu behaviour (flip, scroll, clamp at the screen edge): only 2-row menus captured.
- Float menu exact anchor to the cursor: the click position was not recorded.
- Window clamping at 1280x720 for the main-tab windows: no 720 shot of a main-tab window was taken (only Menu and Options).
- Quests detail layout with a quest selected: none available.
- Work progress bars (size, colour, offset): no shot has a pawn doing a timed job.
- Zoom range at heights other than 1080 (R03_720, R03u only show the default): assumed proportional.
- Animal name labels and their zoom threshold: no tame animal in the shots.
- Pawn selection bracket scale with UI scale: no 1.25x/3x map shot with a pawn selected and a known cell (S_R04 selected pawn not located; R04u not measured).
- Exact stack count hide threshold between 22.5 and 49 px: no shot between.
- Architect cursor readout (`architect/tool.go:680-701`, `cursorTip`): no matching shot (brief says not captured).

## Phases

The implementation is one later task in three waves. Every `open` row is in exactly one phase. A row marked "UIS-1 token, UIS-2x" has its size in UIS-1 and its drawing in UIS-2x.

### UIS-1: contract (the lead, alone)

The lead writes every shared value first, so the UIS-2 workers only consume it. Commit before wave 2 starts.

| Part | Rows | Files |
|---|---|---|
| Steps, refusal rule, Auto, default, migration | UIS-001 (presets), 002, 003 (rule), 004, 005, 006, 009 | `internal/platform/config/settings.go`, `settings_test.go`, `ui_scale.go`, `ui_scale_test.go` |
| Other defaults | UIS-083 (`ZoomToMouse`), UIS-093 (far markers off) | `internal/platform/config/prefs.go` |
| Type sizes and pitches | UIS-012 (sizes), 013, 014, 026 (name cap), 048 (pane size) | `gfx/text.go`, `gfx/text_larger_test.go` |
| Whole-px drawing at fractional steps | UIS-011 | `gfx/canvas.go`, `ui_scale_test.go` |
| Metrics table | Every token below | `hudlayout/` (one table, `TestMetricsMatchReference` pins each value to its row id) |
| Top-HUD layout | UIS-020 (slot), 021, 022, 023, 024, 025, 029, 030, 031, 034 | `hudlayout/layout.go` |
| Bottom-HUD layout | UIS-040, 044 (with `internal/platform/config/hud.go`), 045, 049, 050, 052 (`CmdMaxRows`), 053, 054, 055, 056, 058 (pitch) | `hudlayout/layout.go` |
| Window and menu tokens | UIS-016, 017, 060 (`WindowPad`, `WindowEdge`), 069, 070 | `hudlayout/` |
| Frameless window variant | UIS-060 | `uikit/modal.go` (a new option on the window frame; the modal frame stays for dialogs) |
| Minimum logical size | UIS-003 | `uikit/dialog.go` (constants beside `MinSetupW/H`) |
| Large consumers in lead files | UIS-012 | `uikit/modal.go`, `uikit/dialog.go` |
| Comment renames | UIS-019 | `ui_scale.go` and the 8 test files listed in the row |

**Gate:**
- `go test -race ./internal/platform/config/ ./internal/adapters/inbound/ebiten/gfx/ ./internal/adapters/inbound/ebiten/hudlayout/ ./internal/adapters/inbound/ebiten/uikit/`
- `go test -race -run 'TestComputeUIScale|TestAutoUIScale|TestLayoutFHiDPI|TestUIStepAllowed' ./internal/adapters/inbound/ebiten/` (`TestUIStepAllowed` is new, from UIS-003)
- `make arch`

The HUD may look wrong between UIS-1 and UIS-2: the tokens move before their consumers do. The invariance tests (`visual_invariance_test.go`, `hud_layout_test.go`, `screens_scale_test.go`) are updated in UIS-1 only where a token changes their expected value, and re-run in full in UIS-3.

### UIS-2: area work (parallel, disjoint)

| Worker | Rows |
|---|---|
| UIS-2a top HUD | UIS-012 (its consumers), 020, 026, 027, 029, 030, 031, 032, 073 |
| UIS-2b bottom HUD | UIS-012 (its consumers), 040, 041, 042, 043, 044, 045, 046, 047, 048, 049, 050, 051, 052, 053, 054, 055, 056, 057, 058, 059, 072 |
| UIS-2c windows | UIS-012 (its consumers), 015, 017, 060, 061, 062, 063, 064, 065, 067, 068, 069, 070, 071, 077 |
| UIS-2d map | UIS-080, 081, 082, 084, 085, 086, 087, 088, 089, 090, 091 (check only), 092, 093, 094 |
| UIS-2e options | UIS-001 (label), 003 (refusal row and warning), 007, 008, 016, 075 |
| UIS-2f art | the 3 icon tabs of UIS-041 |

**Deferred:** the Quests tab of UIS-041 and the Quests window of UIS-066. No quests feature or read model exists, so both wait for one. The rest of UIS-041 (order and icon tabs) lands in UIS-2b.

### UIS-3: verification (the lead)

- Re-capture the reference game (the UIS-* `CLEAN_ROOM.md` row, launched through Steam as in Sources) and ours (`cmd/game -capture -capture-ui <state> -capture-ui-scale 1`), and compare region rectangles within ±2 logical px.
- Captures still owed:
  - 17–23 and 25–39 colonists (UIS-022, UIS-023);
  - 20 or more commands (UIS-052);
  - a pawn mid-job (UIS-094);
  - a tame animal (UIS-064, and the animal name labels);
  - a 1.75x shot on a window at least 1344 high (UIS-010);
  - several messages at once (UIS-029).
- Rows owned here: UIS-010, 035, 036, 078, 095.
- Run `go test -race ./internal/adapters/inbound/ebiten/...` within CI's 10-minute `-race` budget, then `make arch`.
- Tick the checklist items in Cross-references.

## Parallel workers

**Areas:** config and scale, type, the HUD layout, top HUD, bottom HUD, windows, map, options, art.

**Contract (the lead, before any agent):** all of UIS-1 above, committed. Workers read the tokens and never add one. A missing token is a message to the lead, who adds it to the table and tells every worker.

**RAM:** run `make agent-budget` before wave 2. Wave 2 is six agents plus the lead.

**Waves:**
1. UIS-1: the lead alone.
2. UIS-2a–f: six agents at once.
3. UIS-3: the lead alone.

**Integration:**
- After each agent reports, the lead runs that agent's gate and `make arch`, then commits its files with `git commit -m … -- <paths>`.
- A failed gate goes back to the same agent through SendMessage. Never spawn a replacement.
- Stop each agent once its work is committed.
- Shared config (`hudlayout/`, `gfx/`, `internal/platform/config/`, `uikit/modal.go`, `uikit/dialog.go`, `assets/metadata/`) stays with the lead.

### Wave 2

`E` = `./internal/adapters/inbound/ebiten`.

| id | model (why) | job | owns | forbidden | tests |
|---|---|---|---|---|---|
| UIS-2a | sonnet (sizes from given tokens) | Top HUD drawing | `hud/colonistbar.go`, `hud/resources.go`, `hud/messages.go`, `hud/alerts.go`, `hud/letters.go`, `learn/` | `hudlayout/`, `gfx/`, every other `hud/` file | `go test -race $E/hud/ $E/learn/` |
| UIS-2b | opus (the largest area: pane tabs, command overflow, Architect) | Bottom HUD | `hud/tabs.go`, `hud/global.go`, `hud/cellinfo.go`, `hud/clock_text.go`, `hud/deps.go`, `inspector/`, `architect/`, `dossier/sheet/`, `ids/` | `hudlayout/`, `gfx/`, `assets/`, the UIS-2a `hud/` files | `go test -race $E/hud/ $E/inspector/... $E/architect/... $E/dossier/... $E/ids/` |
| UIS-2c | sonnet (sizes from given tokens) | Main-tab windows, float menu, tooltips, pause menu | `windows/`, `orders/float.go`, `uikit/` except `modal.go` and `dialog.go`, `screen_pause.go` | `uikit/modal.go`, `uikit/dialog.go`, `hudlayout/` | `go test -race $E/windows/... $E/orders/... $E/uikit/...` and `go test -race -run 'Pause' $E` |
| UIS-2d | opus (camera unit, LOD retune, save version, benchmarks) | Map zoom, cell size and map labels | `camera/`, `render/`, `renderer.go`, `scene.go`, `multiselect_render.go`, `internal/adapters/outbound/filesave/filesave.go`, the selection sprite in `assets/` if it becomes orphaned | `hudlayout/`, `gfx/`, `internal/platform/config/`, `assets/metadata/` | `go test -race $E/camera/... $E/render/... ./internal/adapters/outbound/filesave/`, `go test -race -run 'Zoom\|Camera\|Resume' $E`, and the large-map benchmark with `-race` |
| UIS-2e | sonnet (one screen) | Options rows for the new steps | `options/` | `internal/platform/config/`, `ui_scale.go` | `go test -race $E/options/...` |
| UIS-2f | opus (art, through skill `rimworld-go-art-assets`) | The 3 icon tabs | the 3 new icon sources and generated files under `assets/` | `assets/metadata/` (sends its lines to the lead), every Go file | `make art-check` |

#### UIS-2a top HUD
- **Goal:** the colonist bar, messages, alerts and letters draw at the reference sizes from the UIS-1 tokens.
- **Starts at:** `hud/colonistbar.go:564-591`, `hud/messages.go`, `hud/alerts.go`, `hud/letters.go`.
- **Behaviour:**
  1. The name plate is drawn below the slot, at the fixed cap-8 size, at every slot size (UIS-020, UIS-026).
  2. Messages sit top-left at the UIS-029 anchor, left-aligned; the bar slots under them fade.
  3. Alerts are plain right-aligned text rows of 22, stacked upward directly above the letters (UIS-030).
  4. Letters are 38 × 30 tiles with the label right-aligned over them, in our own colours (UIS-031, UIS-032).
  5. An opened letter uses the UIS-1 frameless variant (UIS-073).
  6. Large text in these files moves to Title 21 or Body 14 (UIS-012).
- **Done when:** the package tests pass, including a test that the name plate's cap stays 8 at the smallest slot.
- **Out:** slot sizes, rows and the bar top (UIS-1); the resources readout and the learning helper have no reference sizes (Not measurable), so they change only through the shared tokens.

#### UIS-2b bottom HUD
- **Goal:** the tab bar, inspect pane, commands, Architect and lower-right stack match the reference sizes.
- **Starts at:** `hud/tabs.go`, `inspector/hud_inspect.go:87-98`, `inspector/hud_commands.go:146-152`, `architect/panel.go`, `hud/global.go:39-43`, `hud/global.go:228`.
- **Behaviour:**
  1. The tabs are reordered and History, Factions and Menu become icon tabs, with the label moved to the tooltip (UIS-041, UIS-042, UIS-043). Factions stays disabled with its reason.
  2. The pane is 432 × 165. A pawn pane has 6 equal tabs across its width; other kinds keep one fixed tab (UIS-044, UIS-045).
  3. Each tab panel opens at its size from the per-tab table and scrolls inside it (UIS-049).
  4. A command that does not fit is never dropped. More rows grow upward until they reach the pane; past that, a "more" cell shows the count (UIS-052).
  5. The Architect has a 200-wide category panel with a search row and a fixed description box (UIS-053, UIS-054, UIS-055).
  6. The lower-right stack uses the UIS-056 and UIS-058 insets and pitch.
  7. Pane headers use the frameless variant (UIS-072). Large text moves to Title 21 or Body 14 (UIS-012).
  8. The docked colonist card stays, at the new default size (UIS-044).
- **Done when:** the package tests pass, including one with 30 commands that all stay reachable, and one for the 6-tab strip at 1280 and 1920 logical width.
- **Out:** the icon art (UIS-2f; until it lands, wire the ids with a text fallback in the test only); the Quests tab (deferred); tokens (UIS-1).

#### UIS-2c windows
- **Goal:** the main-tab windows, float menu, tooltips and pause menu use the reference sizes and the frameless variant.
- **Starts at:** `windows/window.go:62-84`, `windows/work.go`, `orders/float.go`, `uikit/tooltip.go`, `uikit/scroll.go`, `screen_pause.go`.
- **Behaviour:**
  1. Main-tab windows use the frameless variant, pad 18 (UIS-060).
  2. Work rows are 30; the menu lists that are menus use row 29 (UIS-017, UIS-061).
  3. Schedule, Assign, Animals, Research, History and Factions take their row sizes (UIS-062 to UIS-068).
  4. The float menu has row 29, width = text + 48 and no minimum width (UIS-070, UIS-071).
  5. The in-game menu is anchored bottom-right above the tab bar, has no scrim, and grows in 52-px steps to hold our 7 entries (UIS-069).
  6. The tooltip uses the UIS-015 width and padding. The scroll bar uses UIS-077.
- **Done when:** the package tests pass, including a new test that every main-tab window fits a 1280×768 logical screen.
- **Out:** the dialog frame and dialog sizes (`uikit/modal.go`, `uikit/dialog.go`: UIS-1); the Quests window (deferred).

#### UIS-2d map
- **Goal:** the default zoom shows 48 cells of window height, and map labels follow the UI scale only.
- **Starts at:** `camera/camera.go:121-145`, `camera/camera.go:321-349`, `renderer.go:443-450`, `render/world/lod.go`, `render/creature/far.go:27-37`, `render/creature/progress.go:21-26`.
- **Behaviour:**
  1. The tile is window physical height / 48 × zoom, with zoom from 0.40 to 2.18 and about 1.026 per wheel notch, pivoting at the screen centre (UIS-080, UIS-081, UIS-082, UIS-084, UIS-085).
  2. The LOD tiers are retuned for 9–49 px tiles (UIS-086). Run the large-map benchmark with `-race` before and after, and report both.
  3. Saves gain a view version. An old save loads at the default zoom with its centre kept (UIS-087). Add a load test with an old-version view.
  4. Pawn name labels are drawn on the map at the UI scale (UIS-088, UIS-089). Stack counts follow UIS-090.
  5. Selection brackets are drawn in code at the footprint and follow zoom (UIS-092). Remove the ring sprite and its loader if nothing else reads it.
  6. Work bars show from 16 px (UIS-094). Far markers keep working, default off (UIS-093).
- **Done when:** the package tests pass, the old-save load test passes, and the benchmark numbers are in the report.
- **Out:** the `ZoomToMouse` and far-marker defaults (UIS-1); zone labels stay as they are (UIS-091 is a check only).

#### UIS-2e options
- **Goal:** the Options screen offers the reference steps and the new sizes.
- **Starts at:** `options/screen_options.go:295-328`.
- **Behaviour:**
  1. The scale row lists Auto plus the nine steps, labelled as steps, e.g. "1.25×" (UIS-001, UIS-007).
  2. A refused step keeps the old value and shows our own one-line warning; an accepted one shows keep or revert with a countdown (UIS-003, UIS-008).
  3. A stored step larger than the window allows is labelled with both values (UIS-004).
  4. Rows and controls use the option tokens; the window takes the UIS-075 size.
- **Done when:** `options_scale_test.go` covers a refused and an accepted step at 1920×1080 and 2560×1440.
- **Out:** the step list, the rule and the migration (UIS-1).

#### UIS-2f art
- **Goal:** three original icon-tab glyphs for History, Factions and Menu, sized for the 34-px tab bar.
- **Starts at:** skill `rimworld-go-art-assets` and the existing tab art under `assets/`.
- **Behaviour:**
  1. The glyphs are our own design, never a copy of the reference icons.
  2. Sizes come from the UIS-041 row.
- **Done when:** `make art-check` passes and the lead has the metadata lines.
- **Out:** wiring (UIS-2b).

## Cross-references

- `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md` §1.2: "UI scale and density system" and "Typography hierarchy" point here. UIS-3 left both `[-]`: UIS-008 and UIS-075 are partial, and numeric column alignment is not checked.
- `MAIN_UI_SCREENS_SPECIFICATION.md`: OPT-017 and OPT-009 are ticked in this change (Already matching).
- `ART_DIRECTION_AND_ASSET_PIPELINE.md`, the UI scale line: corrected to the code's presets, and points here.
