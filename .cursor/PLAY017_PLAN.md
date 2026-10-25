# PLAY-017: sowing, hauling, outfits, animals, plants and light, chase warnings, UI-scale fixes and visual tests

## Context
Twelve requests from playtesting. Each one comes with its cause, found by exploring the code.

| # | Request | Cause today |
|---|---|---|
| 1 | Sowing should clear whatever is in the cell | `sowable` (`game/food.go:52`) refuses any cell with a plant, so wild plants, moss, bushes, trees and other crops block a zone forever |
| 2 | Zone names float | `drawZoneLabel` (`ebiten/zones_draw.go:347`) passes physical `worldToScreen` coords to `drawText`, which multiplies by the UI scale again (`font.go:119`) |
| 3 | Zone names should scale with the zone | Fixed 11 px, hidden below a 40 px tile |
| 4 | Option to run in the background | Nothing configured; on macOS a hidden window stalls the vsync loop |
| 5 | Haul only the colony's things | `haulToStockpile` (`tick.go:798`) takes every loose stack (map chunks, ruin loot, predator kills) |
| 6 | Colonist bar art floats mid-screen | `drawPawnPortraitArt` (`draw_art.go:833-854`) draws logical coords with a raw `screen.DrawImage` (:851) and no UI scale; the colonist inspector portrait (`inspector.go:339`) has the same bug. The audit also found: tame animal names (`draw_combat.go:102-105`), the chat icon (`labels.go:109`), the fight marker (`labels.go:124`) and the drafted icon (`pawn_anim.go:145`) all double-scale the same way |
| 7 | Change straight into the best clothes they're comfortable in | `assignWear` only runs when a colonist has no work, changes one piece per check, ignores gains under 3 °C, gets stuck on an out-of-area garment, applies `comfortShift` unevenly, and undoes the player's "Wear" order |
| 8 | Tame animals eat the nearest food; predators hunt | Tame animals can't eat loose items or stockpile food and never hunt (`animal_needs.go:202, :320, :398, :406, :415`); food is chosen by category, not distance |
| 9 | Double-click a colonist in the bar to focus the camera | A bar click only selects (`app.go:505-515`); the camera never moves |
| 10 | Plants grow only with sun or strong light; ripe zone crops marked for harvest; growth visible on click | Growth is a 06:00–20:00 clock window (`plant.GrowSpan`) that ignores roofs and lamps; ripe zone crops are harvested but never marked, so no overlay shows; the inspector shows growth % but not why it isn't growing |
| 11 | Warn and pause when an animal chases a colonist | The auto-pause exists (`watchThreats`, `PauseOnThreat` on by default), but hungry-predator, charge, revenge and fight-back chases post no danger message |
| 12 | Enough tests that these visual bugs can't come back | The canvas guard exempts whole files (it hid bug 6) and never checks physical coords fed to UI helpers (bug 2); traces skip `drawText` and `uiDrawImage`; captures force scale 1 |

**User decisions so far:**
- **Hauling:** outputs of work the player ordered are colony items. Only Haul work moves items, following each colonist's priorities. Things the colony never touched wait for a Haul mark.

## 1. Growers clear a zone cell before sowing (game)
- `assignZoneClear(id)` in `food.go`, called in `assignSow` after `assignZoneChop`. It scans the zone cells nearest first, up to `workTries`.
- **Target:** a plant that is wrong for the zone (not zone-sown, or a crop of another species), on a cell that is otherwise sowable. `sowable` splits into `sowableIgnoringPlant` + the plant check.
- **Skips:** ripe wrong crops (they get harvested, see §10), plants reserved by others, and plants postponed for this pawn.
- **Job:** the existing `job.Cut`; the next `assignSow` sows the cell. Drops are colony items (§5).
- **Side fix:** forced `OrderSow` (`orders.go:190-201`) uses the species' own sow work.
- **Tests:**
  - moss, a bush, a wild tree and an unripe other crop are cut, then sown;
  - a ripe wrong crop is harvested, not cut;
  - the right crop is never cut;
  - nothing is cleared out of season.

## 2–3. Zone labels pinned to the zone and scaled with it (renderer)
- **Anchor:** through the new `worldText` helper (see §12), so labels stay in logical px.
- **Placement:** centred on the zone cell nearest the bounding-box centre, so an L-shaped zone keeps its label inside; on a soft dark plate.
- **Size:**
  - `clamp(0.45 × min(zone w, zone h on screen), 11, 40)` logical px, in half steps;
  - shrinks to fit 90% of the zone's width, ellipsizes only at 11 px, hidden only when 11 px can't fit.
- **Tests:**
  - at UI scale 1.5 and 2, the label stays inside the zone across pan and zoom;
  - the size grows with zone size and zoom, and fits the width;
  - an L-shaped zone gets a label inside it.

## 4. Option: run in the background
- **Setting:** `Gameplay.RunInBackground`, default off (settings version bump + `Normalize`).
- **Options row:** "Run in background", tip: "Keep the colony running while the window is in the background or minimised. Off: the game pauses until you come back."
- **Off:** unfocused means no ticks, and the top strip says "Paused (window in background)". Refocus keeps any earlier manual pause and the speed.
- **On:** `ebiten.SetRunnableOnUnfocused(true)`, plus vsync off with a ~20 fps draw cap while unfocused so ticks keep flowing on macOS; restored on focus.
- **Tests:**
  - the setting persists and migrates;
  - injected focus: off pauses and keeps a manual pause; on keeps ticking;
  - the vsync switch goes through a small consumer-owned interface.

## 5. Haulers carry only the colony's things (game + UI + art)
**Contract:**
- `item.Stack.Unclaimed` and `HaulMarked`. The zero value is colony, so old saves and the 176 test spawns are unchanged.
- `CanMerge` requires both to be equal.
- New commands `DesignateHaul` (rect) and `SetHaulMark` (one stack).
- `ItemView` gains both flags.
- `SaveVersion` 3.

**Rule:** only `haulToStockpile` skips `Unclaimed && !HaulMarked`. Every other job still uses unclaimed items; forbidden still blocks everything.

**Spawned unclaimed** (`spawnUnclaimed`, new `haul_claim.go`):
- mapgen ruin loot and chunks;
- vault loot and casket weapons;
- raider and visitor gear;
- animal corpses not killed by the colony (a predator, raider, fire, old age, or a wild animal's starvation);
- eggs from wild mothers.

**Everything else is colony:** start kit, pods, cargo, bills, drying, harvests, cut wood, mining, hunting, tame animal products, refunds, colonists' gear.

**Claiming:**
- Anything a colonist picks up becomes colony (`splitLoad`, the `doHaul` pickup, bill and refuel carries), so leftovers get stored.
- Storing clears both flags.
- `splitLoad`, `dropStackOf` and `clearItemsFrom` keep the flags; `cancelRect` clears marks.

**Old saves:** chunks and animal corpses lying outside stockpiles become unclaimed.

**UI:**
- Orders tool **"Haul things"**, description "Mark things found on the map for hauling. Colonists haul the colony's own goods by themselves; anything else waits for this mark."
- `overlay/designation_haul` on marked stacks.
- Item inspector line "Found here. Colonists won't haul it until you mark it." and a "Mark for hauling"/"Unmark" button.
- A Multiselect button.

**Art:** `ui_icon/order_haul` and `overlay/designation_haul`, made via `assets/source/blender/build.py`; `make art-check` must be clean.

**Tests:**
- spawn origins;
- the hauler ignores unclaimed items but hauls colony output;
- marking, storing and unmarking;
- leftovers are claimed;
- bills still use unclaimed items;
- merge rules;
- no follow-up carry (a chopper with Haul off never carries its own wood);
- save round-trip and the v2 migration;
- UI tool, inspector, overlay and multiselect.

## 6. Portraits and world labels drawn in place (renderer)
- **Portrait:** `drawPawnPortraitArt` moves into `portrait.go` and draws through `uiDrawImage`. That fixes the colonist bar (`hud.go:261`) and the inspector portrait.
- **World labels:** the other double-scaled ones switch to the new `worldText`/`worldIcon` helpers (§12):
  - tame animal names, which also get a plate sized in the right units;
  - the chat-bubble icon;
  - the fight marker;
  - the drafted icon.
- **Tests:** at 1, 1.5 and 2, every portrait image sits inside its bar slot and the inspector panel. The other labels are covered by §12's pan test.

## 7. Colonists dress at once in the best outfit they're comfortable in (game + UI)
**Comfortable:** the Gear tab's "Comfortable X to Y" range (`bandsOf` + `comfortShift`, now applied to candidates too) should cover the day's expected span (the daily mean ±10 °C; not hourly, which would swap clothes at dawn).

**Best outfit:**
- The best Body×Head combination: current piece, reachable candidate, or none in each slot.
- Score: degrees of discomfort; ties go to less surplus insulation, then a shorter walk.
- Unwanted pieces come off at once, then the colonist walks to each new piece in turn.

**Instantly:**
- The dress check runs before work at every job pick, in all schedule blocks, after draft, flee, warmth and critical needs.
- In the 60-tick colonist bucket, a colonist on an interruptible work job drops it (through drafting's abandon path) when a better outfit exists.
- Needs, forced orders and drafted colonists are never interrupted.

**Cheap:**
- an `apparelIDs` index updated on spawn, merge and delete;
- an `apparelGen` counter, so busy colonists are re-scored only when the stock, the day or their comfort changes.

**Hysteresis:**
- a gain of at least 1 °C is needed;
- a 2-hour dress cooldown after any change, skipped while Freezing or Sweltering;
- a no-flap test.

**Availability:**
- not forbidden or reserved;
- reachable and in the allowed area;
- a failed Equip is postponed, so the next best is tried;
- unclaimed garments are fine.

**Player choice:** "Wear X" marks that slot forced until the player picks another garment for it (an additive save field).

**UI:**
- Gear tab shows "(you chose this)" and "Changing into: parka".
- Quiet log lines "Ada put on a parka." / "Ada took off the duster."

**Tests:**
- a mid-job colonist in the cold dresses within one bucket;
- the full outfit goes on in one go;
- an out-of-area garment falls back to the next best;
- no comfort-shift bias;
- a forced piece is kept;
- no-flap;
- needs and drafts are never interrupted.

## 8. Tame animals eat the nearest food; tame predators hunt (game + UI)
Applies to tame animals only. Wild behaviour, the thresholds (4000/9500), the 60-tick bucketed decisions and the food bucket index are unchanged.

**Nearest across every source the diet allows** (ties go to the higher food value):
- **Grass and browse, including berry bushes.** Plants in growing zones or marked for harvest are skipped.
- **Loose and stockpiled raw food:** berries and raw vegetables, straw, raw meat. Never meals, medicine, milk, eggs or forbidden items. The stack is reserved while walking, re-checked on arrival, and the animal only takes what fills it.
- **Carcasses** for meat eaters, now from stockpiles too.
- **Hunting** for tame predators (`Predator`/`PackPredator`).

**Pens and areas still bound the search:**
- penned animals use their pen and its troughs;
- roaming animals stay inside their allowed area and fence reach;
- `FeedRadius` is still 30.

**Hunting:**
- only the nearest huntable wild animal within `HuntRadius` and the area;
- never colony, tame or penned animals, colonists, visitors, prisoners, predators, hive insects or legendaries;
- uses the wild chase path with give-up and leash;
- `feedOnKill` lets the hunter eat; leftovers are colony items;
- pets don't hunt.

**Hysteresis:** the unchanged dead band, `NoFoodRetry`/`HuntRetry`, and two no-flap tests (equal stacks; prey vs stockpile meat).

**UI:**
- state text "eating berries" / "eating from the stockpile" / "hunting a hare";
- the Diet line says "Hunts wild animals when hungry";
- a quiet log line "Rex hunted a hare."

**Tests:**
- a deer prefers a bush 2 tiles away over grass 20 away;
- a dog eats stockpiled meat;
- a wolf hunts a wild hare and never a penned sheep, a colonist or a tame animal;
- pens and areas are respected;
- zone crops and marked bushes are untouched;
- no-flap;
- index and pacing tests stay green;
- no `BenchmarkWildlife*` regression.

## 9. Double-click a colonist in the bar: focus the camera (UI)
- **Detection:** the bar keeps `{id, at a.updates}`. A second click on the same id within `doubleClickFrames` (18, the map's double-click window, `multiselect.go:38`) calls `centreOnPawn` (`hotkeys.go:223`): select, then centre on the drawn position via `centerOnMap`.
- **Smooth reset:** the jump stops any `camSmooth` velocity and zoom ease, so the camera doesn't drift off afterwards.
- **Bar click clearing:** a bar click also clears other selections (animal, item, plant), like `selectCand`.
- **Tests:**
  - at UI scale 1.5, two `fakeInput` clicks on the bar slot centre the camera on the colonist (`TestJumpToCentresOnPhysicalViewport` pattern);
  - clicks further apart don't;
  - a single click only selects.

## 10. Plants grow with sun or strong light; ripe zone crops marked; growth shown on click (game + UI)
**Light rule.** In each 250-tick chunk window, a plant grows for `max(sunTicks, lampTicks)`:
- `sunTicks` is today's `GrowSpan` (the 06:00–20:00 sun window, which matches the outdoor light curve), only when the cell is not roofed;
- `lampTicks` is the whole window when the cell's artificial light is at least `plant.StrongGrowLight` (0.5, i.e. a lamp within 2 tiles or next to a burning campfire), so lamps grow plants at any hour.

Unroofed outdoor crops therefore grow exactly as today: no balance change, and the crop-picker days are unchanged. A roofed crop with no lamp stops growing. Cave species (duskmoss) and moss keep their own light rules.

**Grass:**
- grass follows the same rule per tile: roofed and unlit tiles don't regrow;
- outdoors it grows in the sun window at 24/14 of today's rate, so a day's regrowth is unchanged.

**Light fix:** campfires light only while burning, and the sim light grid is rebuilt when fuel goes between 0 and above 0.

**Auto mark for harvest:**
- When a harvestable plant on a growing-zone cell ripens (growth reaches `GrowTicks` in `growPlant`), it is marked `Designated` and indexed (`notePlantMark`).
- Drawing a growing zone, or changing its crop, marks plants in it that are already ripe.
- `wantsHarvest` becomes `Harvestable && Designated`, so "Cancel harvest" really cancels until the next ripening.
- The harvest overlay now shows on ripe zone crops.
- Old saves: ripe zone plants get marked on load.

**On click (plant inspector):**
- the growth bar caption gives the state: "growing", "resting (night)", "too dark: needs sun or a lamp nearby", "too cold" or "ripe";
- a line "Ripe in about N days", estimated from the current conditions;
- `PlantView` gains `GrowState` and `RipeIn`, recomputed on every build like `Wilted`.

**Tests:**
- a roofed crop doesn't grow;
- with a lamp 1 tile away it grows round the clock; 4 tiles away it doesn't;
- outdoor growth is unchanged (`TestStaggeredGrowthMatchesSpan`, `TestCropsGrowLikeTheRealThing`);
- duskmoss is unchanged;
- grass under a roof stops, and the outdoor daily total is unchanged;
- an unfueled campfire gives no light;
- ripening marks for harvest, and the overlay draws;
- a cancel sticks;
- a zone drawn over ripe plants marks them;
- the old-save migration;
- the inspector state and ripe-in at UI scale 1.5.

## 11. Warn and pause when an animal chases a colonist (game + UI)
**Sim:**
- `noteChase(aid, target)` in a new `chase.go`, called where a chase starts without a message today:
  - a hungry predator (`animal_needs.go:204`);
  - revenge (`animals.go:346`);
  - a charge (`animals.go:413`);
  - fighting back (`animal_flee.go:54`);
  - a vault sleeper woken by a hit (`combat.go:215`).
- It posts `MessageView{Danger: true, Focus: aid}`, e.g. "A timber wolf is chasing Ada!" or "A boar charges Ada!".
- Enraged and manhunter paths already message, so they are skipped.

**Hysteresis:**
- per animal, `ChaseWarnedAt` and `ChaseWarnedTarget` (saved);
- a re-warn needs a new target, or the chase to have ended plus 1 in-game hour;
- so charges rolling every 600 ticks don't re-pause the game (no-flap test).

**Pause:** the existing `watchThreats` pauses on the new danger message when "Pause on threat" is on (the default). Its tip now mentions animals chasing colonists. No new command: the sim posts and the UI pauses itself.

**Visible:**
- danger messages are drawn red (`climate_ui.go:173`);
- the "hostile" alert uses the label and names the colonist, "A timber wolf is chasing Ada!" (fixes the species-key bug at `alerts.go:76-88`);
- the red hostile marker shows on every hostile animal, not only manhunters (`draw_combat.go:92`);
- a red chase line runs from the animal to its target (`AnimalView` gains `TargetID`);
- on a chase auto-pause, if the chase is off-screen, the camera centres between the animal and the colonist.

**Tests:**
- each path posts one danger message with `Focus`;
- repeated charges don't re-warn, but a new target does;
- manhunters aren't messaged twice;
- the UI pauses, shows the red message and the alert text, draws the chase line, and centres the camera only when the chase is off-screen.

## 12. Visual regression tests, so these bugs can't come back (renderer tests)
1. **Helpers (contract):**
   - `worldText(screen, s, physX, physY, style)` and `worldIcon(...)` in `ui_canvas.go` take world-pass physical coords and convert them once.
   - Every world-anchored label uses them.
2. **Trace everything:**
   - `drawText` (with its text), `uiDrawImage`, lines, circles, polys and triangles all emit traces;
   - a trace records the kind, text, physical rect and pass (world or HUD).
3. **AST guard per function:**
   - `TestUIDrawsThroughCanvas` moves from per-file regex to `go/ast`, with exemptions keyed `file:func` and a reason each;
   - new rule: a function that calls `worldToScreen*` may not call `drawText`/`drawIcon`/`ui*` directly, only `worldText`/`worldIcon`;
   - HUD and inspector functions may not call raw draws or helpers from exempt functions;
   - stale exemptions fail.
4. **Frame invariance harness** (`visual_invariance_test.go`). Setup: a real game (32×32, `liveColony`) with `testArt`, a growing zone, a stockpile, a pen, a tame named animal, items, a drafted colonist, a chat bubble and a fight. It runs at UI scale 1, 1.5 and 2, at device scale 1 and 2, and zoomed in and out. Checks:
   - **Pan:** draw, pan by Δ tiles, draw again. Every HUD trace is identical, and every world trace moves by exactly −Δ physical. This catches bug 2 and the label bugs in §6.
   - **Scale:** HUD traces at scale s equal the scale-1 traces × s, matched by kind, order and text. This catches unscaled UI.
   - **Anchors:** zone labels sit inside their zone, animal names under their animal, portraits inside their bar slot (catches bug 6), and stack counts on their stack.
   - **Panels:** with each inspector open (colonist, animal, item, plant, building), each main tab, Options, a letter and the alerts, every trace stays inside its panel and the screen.
5. **Budget:** under about 20 s with `-race`, sharing the lazy art index, within CI's 10-minute budget. No pixel or window tests (tests must not open a window).

## 13. Execution
**Contract first (lead):**
- item fields and `CanMerge`; command kinds;
- `ItemView`, `PlantView` (`GrowState`, `RipeIn`) and `AnimalView` (`TargetID`) fields and fills;
- `SaveVersion` 3 with load-hook stubs (`migrateClaims`, `migrateHarvestMarks`);
- the dress-check call site in `tick.go` with a stub in `apparel.go`;
- the `noteChase` stub in `chase.go` and its `combat.go` call;
- the `onThreatPause` hook in `watchThreats` with a stub in `chase_ui.go`;
- `worldText`/`worldIcon` in `ui_canvas.go`;
- the portrait move to `portrait.go` (TDD).

Then 11 agents run in parallel, with disjoint files:

| Agent | Model | Owns |
|---|---|---|
| **GROW** | sonnet | `food.go` (§1, `wantsHarvest`), `orders.go`, `zones.go` (zone-draw harvest marks); tests |
| **PLANTS-LIGHT** | sonnet (opus if stuck) | `plant_growth.go`, `plant_light.go`, `light.go`, `grass.go`, `domain/world/plant/{light,grass}.go`, `plant_view.go`, `migrateHarvestMarks`, `ebiten/plant_inspector.go`; tests |
| **VISUAL-FIX** | sonnet | `zones_draw.go` (§2–3), `labels.go`, `pawn_anim.go`; tests |
| **APP-SHELL** | sonnet | `platform/config/settings.go`, `ebiten/settings.go`, `screen_options.go`, `app.go`, `input_source.go`, `hud.go` (bar double-click), `threat_ui.go` (background chip); tests |
| **HAUL-SIM** | sonnet (opus if stuck) | `tick.go` haul paths, `haul_claim.go`, `setup_mapgen.go`, `vault.go`, `casket.go`, `raids.go`, `defense.go`, `drying.go`, `save.go`, command dispatch; tests |
| **HAUL-UI** | sonnet | `archtools.go`, `item_inspector.go`, `multiselect.go`, `draw_art.go` overlay; tests |
| **HAUL-ART** | sonnet | Blender sources + generated `order_haul`, `designation_haul` |
| **APPAREL** | sonnet (opus if stuck) | `apparel.go`, `postpone.go`, `combat.go` (`doEquip`), apparel index hooks, `ebiten/inspector.go` Gear tab; tests |
| **ANIMALS** | opus (hunting safety across many paths) | `animal_needs.go`, `animals.go`, `animal_life.go`, `animal_flee.go`, `wildlife.go`, `handling.go`, `chase.go`, `ebiten/animal_inspector.go`, `animals_ui.go`; corpse and egg claims (§5), feeding (§8), chase sim (§11); tests |
| **CHASE-UI** | sonnet | `climate_ui.go`, `alerts.go`, `draw_combat.go` (marker, chase line, tame name fix), `chase_ui.go`; tests |
| **VISUAL-TESTS** | sonnet (opus if stuck) | `ui_canvas_guard_test.go` (AST), `ui_canvas.go`/`font.go` tracing, `visual_invariance_test.go`. It is written in parallel and expected to go green once VISUAL-FIX, APP-SHELL and CHASE-UI merge; it reports what still fails |

**Agents never re-pin goldens.** They report any moves; the lead re-pins once, field by field. Expected moves:
- §5: wild corpses are no longer hauled;
- §10: grass timing and roofed plants;
- §7: earlier dressing;
- §8: should be neutral (tame animals only).

**Lead verification:**
1. Build, vet, gofmt.
2. `go test -race ./...`, plus the application tests without `-race`.
3. `make art-check`.
4. Fps probe.
5. Real-app checks at UI scale 1.5 and 2:
   - colonist bar, double-click focus;
   - zone names zoomed in and out;
   - unhauled map chunks and the haul mark;
   - a lamp-lit indoor crop, and ripe crops showing the harvest mark;
   - a hungry wolf chasing a colonist: pause, red message, chase line.
6. Manual background check (minimise 10 s).
7. Docs:
   - handoffs;
   - the UI checklist (labels, options, haul tool, colonist bar, double-click, plant panel, chase warning);
   - the vanilla checklist (sowing, hauling, apparel, animal feeding, plant light, harvest marks);
   - the README (Options, Hauling, Animals, Farming).
8. Commits on `main` by area; no push.

## Critical files
- `internal/domain/{economy/item/item.go, sim/command/command.go, world/plant/light.go, world/plant/grass.go}`
- `internal/application/game/{food.go, orders.go, zones.go, tick.go, haul_claim.go, save.go, game.go, apparel.go, animal_needs.go, animals.go, chase.go, plant_growth.go, light.go, grass.go}`
- `internal/adapters/inbound/ebiten/{zones_draw.go, portrait.go, ui_canvas.go, font.go, hud.go, app.go, screen_options.go, threat_ui.go, archtools.go, item_inspector.go, inspector.go, plant_inspector.go, animal_inspector.go, climate_ui.go, alerts.go, draw_combat.go, labels.go, pawn_anim.go, ui_canvas_guard_test.go, visual_invariance_test.go}`
- `internal/platform/config/settings.go`
- `assets/source/blender/` (`order_haul`, `designation_haul`)
