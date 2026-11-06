# PLAY-020: bubbles, zone fill, lamp-lit rooms, sowing, bed rest, medical beds, zone management

## Context
Seven issues and requests from play:

1. **Chat bubbles have a white background.** `drawChatBubbles` (`labels.go:78`) puts a near-white box (`{250,248,240,235}`), with a border and a white tail, behind the round dark social badge (`ui_icon/social_*`). A plain chat (`"chat do"`) draws the box with three dots.
2. **Growing zones look painted over the crops.** The zone sprite is a 14% green wash with 44% stripes every quarter tile (`drawZones`, `zones_draw.go:183`).
   - Close zoom: plants are drawn after the zones (pixel-checked), but the wash and stripes cover the furrows and every gap between the thin plants.
   - Far zoom (below 16 px per tile): plants are baked into the map image under the zones, so the fill really covers them.
3. **Rooms with lamps are sun-lit.**
   - Menu scene (`scenes/menu.py`): one sun (`stage.py:264`), no roof, no point lights. Wall shadows fall inside the room and the bulb casts a sun shadow.
   - In-game: roofs drop the sun already (`light.go:421`, `shadow.go:320`/`:664`). But a lamp is a flat 1 over its room (`light.go:427`), with no falloff or direction (`lit.kage:94-98`), and `light_radius: 6` is unread. Lamp shadows vanish by day indoors (`shadow_cast.go:217`), and lamps outside rooms light nothing.
4. **Two colonists sow the same tile.** `assignSow` (`food.go:153-198`) checks `TileOwner` but never claims the cell, and `startJob` (`tick.go:577`) reserves nothing for Sow. The loser fails with `TargetGone`, and the same race follows every wrong-plant cut. A forced sow (`orders.go:190`) neither checks nor claims.
5. **Injured colonists don't rest.** `patientRest` (`infection.go:69-78`) needs an untended wound plus another doctor, and ends at the tend. It has no retry cooldown. A hungry patient may be put back to bed every tick before the eat check (`tick.go:470` vs `:528`).
6. **No per-bed medical use.** "For prisoners" is already a single-bed flag (`prisoners.go:70-82`). Only the `HospitalBed` kind is medical-ish, and healthy colonists even sleep in it (`assignSleep`, `tick.go:742`).
7. **Zones can't be managed from the map.**
   - There is no zone selection kind (`selection.go:15-23`), pane, per-zone flag or zone-edit command.
   - Painting next to a same-kind zone merges into it (`zone.Map.Paint`, `map.go:223`).
   - Cancel shrinks only with a rectangle that also cuts other zones (`Map.Clear`, `map.go:370`).

**User decisions (2026-10-25):**
- Lighting: "All, plus outdoor lamps".
- Injured or in-pain colonists stay in bed by default.
- Medical is a per-bed setting that never spreads to other beds.
- Zone management is done "the way RimWorld does it". With Allow sowing, Expand and Shrink, a selected zone also gets Plant to grow, Delete zone, Allow cutting, and Hide and rename.

**How RimWorld does it** (from rimworldwiki.com Zone, Growing zone and Stockpile zone; behaviour only, our own text and art):
- A selected zone shows its settings and controls in the inspect area.
- A growing zone's settings: plant to grow, rename, resize (expand and shrink), hide, allow sowing, allow cutting.
- Shrink removes squares from a zone with the same drag as drawing one (a rectangle, not a brush).
- Forbidding sowing stops only sowing; with cutting allowed, colonists still harvest and chop there. With cutting off, they don't cut.
- Changing the crop with both allowed makes growers cut the old plants and re-sow.
- The pane shows the growing period, the plants' count and age, and the settings.
- Zones are always contiguous and never overlap.
- Double-clicking an empty zone tile selects all similar zones in view: left for later.

## Phase 0: lead contract and the sowing fix
All additive, built, committed before any agent launches.
- **Commands** (`command.go`), appended after `DismissLetter` (gob order):
  - `SetBedMedical`, `SetPawnBedRest` (Target, Enabled);
  - `SetZoneSow`, `SetZoneCut`, `SetZoneHidden` (Target = zone id, Enabled);
  - `RenameZone` (Target, Name);
  - `SetZoneCrop` (Target, Build = species id, as `DesignateGrowZone`);
  - `ExpandZone`, `RemoveZoneCells` (Target, X..Y2 rect);
  - `DeleteZone` (Target).
- **Fields:**
  - `construction.Building.Medical`;
  - `zone.Zone`: `NoSow`, `NoCut`, `Hidden` (negative or zero-default, so old saves and `StateHash` are unchanged) and `Name string` ("" = default);
  - `game.ZoneView`: `NoSow`, `NoCut`, `Hidden`, `Name`;
  - `game.BuildingView.Medical`;
  - `game.PawnView`: `BedRest`, `RestWhenHurt`;
  - `Renderer.zonePlants map[[2]int]bool`.
- **Routing:** `applyHostCommand` (`tick.go:57`) and `host_commands.go` dispatch to stub methods in new agent-owned files:
  - `med_cmds.go`: `cmdSetBedMedical`, `cmdSetPawnBedRest`;
  - `zone_cmds.go`: one method per zone command.
- **UI hooks** (`app.go`, `hud.go`, `hud_inspect.go`, `hud_commands.go` are lead-frozen):
  - App fields `hasZone`, `selZone uint16`;
  - calls into stubs in a new ZONEUI-owned `zone_select.go`: `handleZoneTool() bool` (map-mouse gate beside `handleMoveMode`, `app.go:355`), `applyZoneClick(id) bool` (the `applyClick` prefix switch), `escapeZone() bool` (the Esc chain), `drawZoneSelection(screen)` (in `drawSelectionMarks`), and the zone cases in `paneFor` and `commandsFor`.
- **Sowing fix (TDD, lead):**
  - `startJob` claims the tile for `job.Sow` (`g.reserve.TryTile`), beside the Move and Haul claims, and fails when another pawn holds it. This covers `assignSow`, whose `tried` list moves on, and forced sows.
  - `forcedJob` `OrderSow` refuses a held tile with `ReasonBusyTarget + g.name(owner)` (as mine does, `orders.go:224`).
  - Tests:
    - two adjacent growers get different cells;
    - a forced sow on a claimed cell is refused;
    - one pawn per freed cell after a cut;
    - a two-grower 12-cell field checked every tick never has two Sow jobs on one cell and ends fully sown with no Sow `TargetGone`;
    - save/load mid-sow keeps the claim.

## Phase 1+: agents (RAM-checked)
Run `make agent-budget` before each wave. Expected: 5.7 GiB, art slot 1, light 5. Real-art UI tests can reach about 2 GiB.
- **Wave 1:** ART (art slot, Blender, short), LIGHT and MEDSIM (light).
- **Wave 2:** ZONESIM, ZONEUI and MEDUI, once ART's renders finish and the budget allows.
- LIGHT takes the art slot for game captures only after ART; the lead relays "art slot free".
- Agents run `GOFLAGS=-p=2 go test -race -count=1 -run '<patterns>'`, never whole big packages.
- Forbidden for every agent: other owners' files, Phase-0 contract files, `go.mod`, `Makefile`, `.cursor` docs, README, git.
- Reports are ≤ 250 words, including any golden or hash change with its reason.
- An agent that fails its gate twice is relaunched one tier up.

| Agent | Model | Owns |
|---|---|---|
| ART | opus (art rule, skill `rimworld-go-art-assets`) | `artlib/scenes/menu.py`; additive point-light support in `artlib/stage.py`/`contracts.py` (sun and world sprites untouched); `assets/generated/ui/menu/*`, `metadata/ui_menu/*`, `menu_diorama.go` and its test; new badge icons in `artlib/families/overlay.py` (`zone_expand`, `zone_shrink`, `zone_sow`, `zone_cut`, `zone_delete`, `zone_hide`, `zone_rename`, `bed_rest`), our own glyphs, with their PNGs and metadata |
| LIGHT | opus (shader, light grid) | `light.go`, `lit.kage`, `shadow_cast.go`, `shadow.go`; lit batch builders `crop_batch.go`, `grass_batch.go`, `lod.go`, `blood_draw.go`, `dirt_draw.go`, `water_draw.go`, `fire_world_draw.go`, `draw_art.go` (lit draw calls only); their tests |
| MEDSIM | opus (hysteresis, saves, job order) | `med_cmds.go`; `infection.go`, `medical.go`, `beds.go`, `prisoners.go`, `surgery.go`, `items_ng1.go`, `area_dest.go`, `jobindex.go`, `save.go`, `world_stash.go`, `game.go` (pawn and building views); `tick.go` except `startJob`; `orders.go` `OrderSleep`; tests |
| ZONESIM | sonnet (against the contract; split logic exists) | `zone_cmds.go`; `food.go` (`sowable`, `assignZoneClear`, `doSow`); `tree_zone.go`; `zones.go` (`zoneViews`, `zoneSave`); `internal/domain/world/zone/map.go` (`AddCells`, `RemoveCells`, `Remove`); tests |
| ZONEUI | sonnet (UI against the contract; skill `rimworld-go-ui-screens`) | `zone_select.go` (new), `selection.go`, `cmds_zone.go` and `zone_tools.go` (new), `zones_draw.go` (selection outline, hidden zones and custom names, after the lead's fill fix lands); tests |
| MEDUI | sonnet (UI against the contract) | `building_inspector.go` (`bedCommands`, `bedLines`, `applyBedClick`, `bedSlots`), `cmds_pawn.go` (rest toggle, status line); tests |
| Lead | session model | Phase 0, `labels.go`, `zones_draw.go` fill fix (first, then hands the file to ZONEUI), docs, verification, commits |

## 1. Chat bubbles (lead, `labels.go`)
- No white `FillRect`, no border, no tail.
- **Interaction with art:** only the icon via `worldIcon` at today's anchor and size; `TestChatIconSitsOnPawnAtScale` stays unchanged.
- **Plain chat:** `ui_icon/social_chitchat` the same way; the flicker stays.
- **No art:** a small `uiPanel` disc with three `uiTextDim` dots.
- **Tests:**
  - `TestChatBubbleHasNoWhiteBox` (real art): the old square's corners have alpha 0, no near-white opaque pixel, and `"chat do"` draws one `worldIcon`.
  - `TestChatBubbleFallbackIsNotWhite`.

## 2. Zone fill skips its own plants (lead, `zones_draw.go`)
- **`zonePlantCells(w)`** fills `r.zonePlants` (cleared with `resetCells`), only when a grow zone exists.
  - Predicate: `p.Zone != "" && (p.Species == p.Zone || p.Zone == "tree" && isTreePlant(p.Species))`.
  - Iterated like `drawFurrows` (`crop_batch.go:836`).
- **`drawZones`:** for grow zones, `zoneFill` and both fallbacks skip those cells. Outlines, labels, stockpiles and pens are unchanged, at every zoom.
- **Tests:**
  - `TestGrowZoneFillSkipsItsPlants`: a potato, wild grass and 2 empty cells give 3 quads, at a live and a far-zoom tile size; a stockpile keeps every quad.
  - `TestTreeZoneFillSkipsItsTrees`.

## 3. Lamp-lit rooms
### 3a. Menu scene (ART)
- **The room is roofed:** no sun shading or sun shadows inside (no shadow-catcher pass there). The indoor lamp is a Blender point light at the bulb: warm `lamp_warm`, radius about 6 tiles, soft falloff, clipped to the room.
- **Furniture:** beds, table and chairs are shaded toward the lamp, with soft lamp shadows pointing away from it.
- **The bulb** is emissive.
- **The outdoor lamp:** sun-lit at dusk, plus its own pool and light, blocked by walls.
- **Dusk grade:** `_dusk` does not darken the interior as if it were outside.
- **Glow layers** match the new light.
- **Constraints:** same composition, camera and palette; original art.
- **Verify:** `make art-menu`, `make art-check`, before/after crops.

### 3b. In-game point lamps (LIGHT)
- **Light grid** (`lightGrid.base`, `light.go:421`): per-tile lamp light is the strongest lamp falloff, radius from metadata `light_radius` (default 6), with a smooth curve such as `(1-(d/r)^2)^2`.
  - A room lamp lights only its room; walls keep the brightest-neighbour rule.
  - An outdoor lamp lights what it can see in its radius: not through walls, closed doors or rock, and not into enclosed rooms.
  - Always on. Rebuilt lazily on structure or lamp change, with no per-frame global scan. `Room.Lit` is unchanged.
- **Direction:** sprites shade toward the dominant lamp.
  - The encoding is LIGHT's design (the four custom vertex floats are taken).
  - Builders that only pass `lightTile` values keep working unchanged.
  - Flat directly under a lamp; half-Lambert or a kept ambient share.
  - `litFactor` and `flatLitScale` stay the shader's Go twins; Lighting off gives falloff without direction.
- **Shadows:**
  - Creature lamp shadows depend on the creature's tile (roofed or lamp-lit), not the global daylight (`shadow_cast.go:217`).
  - Their reach follows the grid.
  - Lit outdoor lamps join `pointLights` at night.
- **Performance:** fps probe before/after (250 map, grown 500); draw p99 within 1 ms.
- **Tests first:**
  - near is brighter than far in a room;
  - no light through a wall;
  - an outdoor lamp lights open ground but not behind a wall;
  - west-facing normals east of a lamp are brighter;
  - a roofed room at noon has no sun term and keeps lamp shadows;
  - an outdoor lamp casts creature shadows at night;
  - the Go twin matches the shader.
  - Re-express (never weaken) `TestIndoorAmbientWhenRoofed`, `TestLampAddsWarmth`, `TestLitRoomWallCornersGlow` and `TestPointLightsFromCampfiresAndLitLamps`.

## 4. Bed rest for injured colonists (MEDSIM; widen `patientRest`, `infection.go:69`)
- **Start** when any of these holds and `RestWhenHurt` is on (default):
  - in pain (`Injury >= painInjury` 40);
  - an untended bleeding or open wound;
  - sick (`Infection >= Sick`);
  - waiting for surgery.

  It no longer needs another doctor: self-tend happens in bed.
- **Stop** at Injury ≤ 15, no untended wound, and not sick. A 2 h minimum rest; with no free bed, a 6 h retry cooldown (the `surgeryGaveUp` pattern).
- **Interruptions:**
  - A hungry patient ends rest, eats and returns (this fixes the loop at `tick.go:470`).
  - Fire, flight, mental breaks, social duty, warmth, drafting and queued or forced orders keep their `phaseAI` precedence. Rest resumes after a forced job.
- **Bed choice:**
  - the nearest free medical bed the pawn may use (the prisoner flag must match), then its own bed, then the nearest free non-medical bed;
  - medical claims are temporary;
  - `freeBedFor` (rescue) ranks medical beds first, then the hospital kind, then distance.
- **Per pawn:** `SetPawnBedRest` turns "Rest when hurt" off or on (saved; default on).
- **State:** new maps are saved as additive `saveState` fields and cleaned in `world_stash.go`, with deterministic iteration. `PawnView.BedRest` and `RestWhenHurt` are filled, and `pv.Job` reads "Resting (injured)" (the `fillFleeView` pattern).
- **Tests:**
  - no-flap: at most 3 job-kind changes in 300 ticks around the thresholds, and the state survives save/load (`sleep_test.go` helpers);
  - goes to bed and stays until healed;
  - a hungry patient eats then returns;
  - no bed puts the cooldown in effect;
  - toggled off, the pawn keeps working;
  - drafting overrides;
  - prefers and then releases a medical bed.
  - Goldens move only where injured pawns now rest, re-pinned with a reason.

## 5. Medical beds (MEDSIM sim, MEDUI UI)
- **Sim:**
  - `SetBedMedical` sets `Medical` on that one sleeping bed only, never on its neighbours.
  - Turning it on releases its owners. It is independent of "For prisoners".
  - New `HospitalBed`s start medical, with a one-time migration for older saves.
  - `g.isMedicalBed(b)` replaces the "for patients" kind checks at `medical.go:77`, `jobindex.go:265` and `area_dest.go:41-45`. Quality bonuses stay kind-based (`items_ng1.go:86-93`, `surgery.go:375`/`:549`).
  - `assignSleep` (`tick.go:755`) and `partnerBed` (`beds.go:74`) skip medical beds for anyone not resting.
  - `cmdAssignBedOwner` and `OrderSleep` refuse them for healthy pawns, with a reason.
  - `BuildingView.Medical` is filled.
- **UI (MEDUI):**
  - a "Medical" toggle beside "Prisoners" on any sleeping bed: icon `ui_icon/hospital_bed`, desc "Only the sick and injured rest here.";
  - the bed line "Medical bed";
  - the Owner button disabled with "Medical beds have no owner";
  - the pawn toggle "Rest when hurt" (`cmd.pawn.bedrest`, icon `bed_rest`) and the status line "Resting (injured)".
- **Tests:**
  - sim: a second bed in the same room is unchanged, owners released, healthy colonists never use a medical bed, patients prefer it, save/load, migration;
  - UI: the `TestBedPaneOwnersAndCommands` and `cmdsOf(..., command.SetBedMedical)` patterns, and the pawn toggle.

## 6. Zone management, RimWorld-style
### Sim (ZONESIM)
- **`NoSow`** stops only sowing: `sowable` returns false (covering `assignSow`, `OrderSow` and the Sow menu item), `doSow` cancels in-flight sows, and tree zones stop replanting.
- **`NoCut`** stops growers cutting in the zone: no wrong-plant clearing (`assignZoneClear`) and no zone tree chopping (`assignZoneChop`).
  - Cells held by wrong plants then stay unsown.
  - Harvesting the zone's own ripe crop, and the ripe auto-mark, continue with either toggle off.
- **`SetZoneCrop`:** changes the crop (`zone.Map.Set`; a merge with a touching same-crop zone is allowed). With both toggles on, the existing PLAY-017 flow harvests or cuts the old plants and re-sows, as RimWorld does.
- **`ExpandZone`:** adds the rect's valid cells (`CanZoneAt` and the kind's terrain rules) to that zone, keeping its settings and name.
  - Cells that don't connect to it become a new zone with the same settings, so zones stay contiguous.
  - New `zone.Map.AddCells`.
- **`RemoveZoneCells`:** removes only that zone's cells in the rect (`zone.Map.RemoveCells`). The zone splits into contiguous pieces, which keep its settings; the piece with the most cells keeps the id. An emptied zone is deleted.
- **`DeleteZone`:** `zone.Map.Remove(id)`.
- **`SetZoneHidden`, `RenameZone`:** flags only. Names are trimmed, at most 24 characters; empty means the default.
- **Saves:** `zoneSave` gets the additive fields; zero values mean today's behaviour, so `StateHash` is unchanged for existing zones. `ZoneView` is filled.
- **Tests:**
  - sowing off then on; an in-flight sow is cancelled; harvest continues;
  - cutting off leaves a wild plant uncut and its cell unsown; tree chop is stopped;
  - a crop change makes growers clear and re-sow;
  - Expand: adjacent cells join, a detached block becomes a new zone with the same settings, invalid cells are skipped;
  - Remove: a neighbouring zone in the rect is untouched, splits keep settings, emptied zones are deleted;
  - Delete; Rename; Hidden;
  - save/load of every field;
  - an unchanged `StateHash` for default zones.

### UI (ZONEUI; skill `rimworld-go-ui-screens`, `MAIN_UI_SCREENS_SPECIFICATION.md` §18 and the selection order `:518`)
- **Selecting:**
  - A zone is the last selection candidate on its cell (`selectablesAt`): a click on an empty zone cell selects it, and repeat clicks on a crop or item cell cycle to it.
  - Clicking the zone's label plate selects it directly.
  - The selected zone draws a brighter outline and fill (`drawZoneSelection`); a hidden zone shows its outline while selected.
  - A vanished id (split or delete) follows the piece holding the last clicked cell, or clears.
- **Pane** (`plantPane` is the model):
  - Title: the zone's name, or a default "Growing zone N" / "Stockpile N" / "Pen N" from its id.
  - Lines:
    - size in tiles;
    - for grow zones: "Plant: Potato"; growing now or too cold or too hot, with the zone's temperature (`TemperatureAt`) against the crop's range; plants: sown of total, ripe count, average growth;
    - "Sowing off" or "Cutting off" when set;
    - stockpiles: their filters.
- **Commands** (`cmd.zone.*`, with `Run`, in RimWorld's order):
  1. **Plant: Potato:** grow zones; opens the crop list (`openPick`, the architect crop picker's entries and research locks) and enqueues `SetZoneCrop`.
  2. **Allow sowing:** grow zones; Toggle, `On = !NoSow`; `SetZoneSow`.
  3. **Allow cutting:** grow zones; Toggle; `SetZoneCut`.
  4. **Expand:** a drag-rectangle tool for this zone, with the zone-painting ghost (green or red per cell, `drawZoneGhost`, `CanZoneAt`); enqueues `ExpandZone`.
  5. **Shrink:** a drag-rectangle tool that tints in red only this zone's cells under the drag; a single click removes one cell; enqueues `RemoveZoneCells` per rect.
  6. **Delete:** immediate; `DeleteZone`.
  7. **Hide:** Toggle; `SetZoneHidden`.
  8. **Rename:** a text-entry modal with the existing field widgets; `RenameZone`.
- **The two tools:**
  - They act like architect designators: the tool's icon follows the cursor, and a hint reads "Expand <zone>: drag to add tiles" / "Shrink <zone>: drag to remove tiles · right-click or Esc: done".
  - They stay active after a drag, until right-click or Esc. They swallow the opening click and gate the map mouse via `handleZoneTool`.
  - Raw world drawing gets canvas-guard exemptions; UI text goes through the canvas.
- **Map drawing** (`zones_draw.go`): a hidden zone draws no fill, outline or label unless selected, and a custom name replaces the label text.
- **Icons** come from ART, falling back to existing icons (`zone_grow`, `order_cancel`, `plan_clear`) until they land.
- **Tests:**
  - click precedence and cycling; a label click;
  - pane lines for each kind;
  - each command: toggle state and command enqueued; the Plant picker; Delete; the Rename modal;
  - Expand and Shrink: the ghost cells, one command per drag, only the selected zone, right-click and Esc exit, the opening click swallowed;
  - hidden and renamed drawing;
  - selection after a split;
  - layout at UI scales 1, 1.5 and 2 (the existing pane and grid tests).

## 7. Lead verification and docs
1. Builds and checks:
   - `go build ./...` and `GOOS=windows CGO_ENABLED=0 go build ./...`;
   - gofmt and `go vet ./...`;
   - `make lint`: no new findings in touched files.
2. `make agent-budget`, then the full `GOFLAGS=-p=2 go test -race ./...` alone. The known local `TestAtlasCoversKeys` failure (stale pawn PNGs) is expected.
3. `make art-check`; goldens move only where agents re-pinned them with a reason.
4. Captures, one at a time:
   - the late farm at zoom 1 and at far zoom, plus the pixel check;
   - a chat bubble;
   - the main menu;
   - a lamp room at night and at noon, and an outdoor lamp at night;
   - an injured colonist in a medical bed;
   - a selected growing zone with its pane and commands;
   - the Expand and Shrink tools mid-drag;
   - a hidden and a renamed zone.

   Then the fps probe before/after.
5. Docs:
   - handoff `.cursor/handoffs/UI-028-play020-...md`;
   - `ART_DIRECTION_AND_ASSET_PIPELINE.md` (zone fill, menu lighting, lamp radius in-game, new icons);
   - the vanilla checklist: §11.5 medical beds, §5.3 bed rest/patient, lamps, sowing, growing-zone settings;
   - the UI/UX checklist: `:467-469` select, expand and shrink; `:474-488` the stockpile panel (partly); `:492` crop selector; `:499` allow sowing; `:2069-2070` zone settings; social bubbles; bed and pawn commands;
   - `MAIN_UI_SCREENS_SPECIFICATION.md`: §2 background, §18 growing settings, the selection order;
   - `.cursor/SOCIAL_SYSTEM_CONTRACT.md` commands (`:141`);
   - ebiten `AGENTS.md`: the lamp model, zone selection and tools.
6. Commits by area on `main`, no push:
   - contract;
   - sowing;
   - bubbles;
   - zone fill;
   - zone sim;
   - zone UI;
   - bed rest and medical sim;
   - medical UI;
   - lamps;
   - menu art and icons;
   - docs.

## Critical files
- **Lead:** `command.go`, `construction.go`, `zone/map.go` (fields only), `game.go`/`zones.go` (view fields), `tick.go` (`startJob`, routing), `host_commands.go`, `orders.go` (`OrderSow` check), `app.go`, `hud.go`, `hud_inspect.go`, `hud_commands.go` (hooks), `labels.go`, `zones_draw.go` (fill fix)
- **Agents:** as in the table.
- **Reuse:**
  - renderer: `worldIcon`, `interactionIconKey`, `resetCells`, `r.frameLists`, `isTreePlant`, `uiPanel` and `uiTextDim`, `pointLights`, `presetArchTool`, `move_mode.go`, `drawZoneGhost`, `zoneGhostCells`, `openPick`, `archDescButtons` (crop entries), `plantPane`;
  - sim: `g.reserve.TryTile` and `TileOwner`, `ReasonBusyTarget`, `cmdSetBedForPrisoners`, `surgeryGaveUp`, `fillFleeView`, `zone.Map.split`/`Set`/`Paint`, `CanZoneAt`, and the `sleep_test.go` no-flap helpers.
