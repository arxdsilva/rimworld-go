# PLAY-021: electricity, RimWorld-style

## Context
The user wants electricity "just like RimWorld does".

**Today:**
- No power exists. `internal/domain/building/power/doc.go` is an empty stub, and nothing imports it.
- Lamps are always on (`game/light.go:101`, rooms `Lit` at `rooms_index.go:236`).
- There are no heaters, coolers or electric benches, and no Electricity research.
- The docs already ask for it:
  - vanilla checklist Phase 15 (`:1272-1306`), solar flare (`:1164`), short circuit (`:2012`), Electricity/Battery research (`:1701-1702`);
  - the UI checklist's power overlay (`:925-930`) and "no power reason" (`:867`);
  - `MASTER_SPEC.md` §39 (`:1167-1190`): a power graph with no global discovery and dirty invalidation (`:410`, `:424`);
  - the art spec (`:518-521`): subtle conduits; powered state shown by emission, not duplicate sprites.

**How RimWorld does it** (rimworldwiki.com Power, Power conduit, Battery, the generator pages, Power switch, Events; behaviour only):
- Conduits link adjacent power buildings into a grid; devices connect to a transmitter within a short range, shown by a link line.
- When generation falls short, batteries cover the gap; past that, devices lose power.
- Fuelled generators burn fuel that colonists haul in.
- Solar follows daylight and drops under roofs; wind follows wind speed and drops with obstructions in its zone; geothermal sits on a geyser.
- Batteries charge with a loss and self-discharge.
- Switches and per-device toggles are flicked by a colonist.
- A solar flare stops electrical devices for a while; a short circuit blows a conduit and drains the batteries (fire).
- Research gates it all: Electricity first, then the rest.
- The inspector shows grid excess and stored energy.

**User decisions (2026-10-25):**
- **Generators:** wood-fired, solar, wind, geothermal.
- **Lamps need power,** plus a new wood-burning torch lamp before electricity.
- **Extras:** switches and power toggles; solar flare and short circuit; heater and cooler; electric benches.
- **Research:** a RimWorld-like tree.
- **Numbers are our own** (AGENTS.md forbids copying balancing data).

**Sequencing:** PLAY-020 is in flight. It touches the renderer light grid (LIGHT), saves, tick.go and beds (MEDSIM), and zones. PLAY-021 starts after PLAY-020 is verified and committed.
- On resuming LIGHT now, the lead adds one requirement: light sources come from one helper (`lightSources(w)`, lamps only for now), so PLAY-021 gates it on the view's `Glow`/`GlowRadius` in one place, not via `Kind == "lamp"` checks (`shadow_cast.go:451`, `light.go:312`).

## Model (our numbers; integer W; stored energy in W·ticks, shown as Wh)
| Thing | Rule |
|---|---|
| Conduit | Transmitter on a new appended `LayerConduit`. Goes under walls, doors and benches; not on rock, water or natural features. 4-neighbour links. |
| Hidden conduit | Like a conduit but buried: drawn only in the power overlay, never chosen by a short circuit. Costs more. |
| Power switch | A transmitter when closed. Open, it splits the grid. A colonist flicks it. |
| Device connection | Joins the nearest transmitter within 3 tiles (Chebyshev from its footprint; ties by distance, y, x). Shown as a link line. |
| Wood-fired generator | 800 W while on and fuelled. Burns logs that haulers bring (shared fuel table). |
| Solar panel | Up to 900 W, following daylight (`lighting.Outdoor`). Scaled by the share of its footprint not roofed; 0 at night. |
| Wind turbine | 250 W per unit of wind speed, capped at 1100 W. Scaled by the clear share of a band beside it (blocked by walls, rock, roofs, trees); the share is cached per structure, roof and game hour. Blades spin with the wind. |
| Geothermal | 1800 W constant, built exactly over a steam vent (2x2; the vent moves to `LayerFoundation`). |
| Battery | 1200 Wh, 75% charge efficiency, 8 W self-discharge. Equal split between batteries. Can't be toggled. |
| Lamp | 50 W, light radius from its metadata. Unlit without power. |
| Torch lamp | No power. Burns logs (fuel table: 8 logs). A smaller warm light; rain can put it out like a campfire. Free from the start. |
| Heater | 180 W while heating, 20 W idle. Thermostat: target 21 °C, heats below target − 1 °C, idles above target + 0.5 °C. |
| Cooler | Sits in a wall gap like a door, facing its cold side. 220 W cooling, 20 W idle. Cools the cold-side room and warms the hot side (outdoors: nothing). Same thermostat hysteresis. |
| Electric stove, tailoring bench, research bench | 250 / 150 / 200 W working, 10% idle. Work 40% faster than the fuelled or hand versions. Without power: no work ("No power"). Recipes come from the base kind (`RecipeBench`). |

**Balance:**
- `phasePower` runs before `phaseClimate` every 50 ticks, or at once when the grid is dirty. It integrates the exact elapsed time and does nothing (writes no state) in colonies without power devices.
- A net's generation covers its load; any surplus charges the batteries (with the loss), and any deficit draws on them.
- **Brownout:** if the load still exceeds generation plus what the batteries can give, consumers are shed by class (lamps, benches, coolers, heaters), the newest within a class first. A shed device stays off for at least 1 h and comes back only with 10% headroom.
- Nets are rebuilt only when the structure or a switch changes (cache key: structVer, topology version, store identity; the `light.go:86` template).

**Events** (Nature pool; `incidentFits` returns false with no power devices, so other colonies' RNG and hashes are unchanged; their own `streamPower` stream):
- **Solar flare:** 3–8 h, 12-day cooldown, not before day 12. Every net goes down: consumers off, generators idle and burning nothing, batteries frozen. Torches and campfires keep burning. Letters at the start and end, and a status chip while it lasts.
- **Short circuit:** 8-day cooldown, not before day 10. Needs a net with ≥ 100 Wh stored and a visible conduit; the conduit is picked deterministically from a sorted list.
  - That net's batteries drain to 0, the conduit is removed, and a fire starts there (`startFire`, sized by the energy).
  - A threat letter.

**Research** (appended): Electricity first, then Batteries, Solar power, Wind power and Geothermal power, each requiring Electricity.
- **Electricity unlocks:** conduit, hidden conduit, switch, wood-fired generator, lamp, heater, cooler and the electric benches.
- **The follow-ups unlock:** the battery, solar panel, wind turbine and geothermal generator.
- **Costs** are our own, in line with the existing projects.

**Saves:**
- A `Power *powerSave` (nil when empty, so `StateHash` skips it) holds:
  - battery charge;
  - wanted and actual on/off;
  - shed timers;
  - thermostat targets and active flags;
  - the flare end;
  - the last balance tick.
- Nets and outputs are derived. Fuel stays in the existing `Fuel` map; cooldowns stay in `incidents.Last`.
- `SaveVersion` bumps (after PLAY-020's 4, so 5). Loading an older save converts existing lamps to torch lamps, so old colonies keep their light.

## UI, art and sound
- **Architect:**
  - a Power category: conduit (line tool), hidden conduit, switch, the four generators, battery;
  - the lamp and torch lamp in Furniture;
  - the heater and cooler in a Temperature category with the campfire (or Misc if a category is too many; `architect_hud_test.go:88`);
  - the electric benches in Production.
  - Research locks via `toolLock`.
  - While placing a powered building, the ghost draws a link to the nearest transmitter in range, or "Not connected".
- **Power overlay:**
  - A global toggle `Power grid` (`hud_global.go`; a second row when there are more than 8 toggles).
  - Auto-shown while placing a power tool (`syncPowerForce` beside `syncRoofForce`, `hud_global.go:518`).
  - It shows:
    - each net's conduits in their own colour, hidden conduits included;
    - the link lines;
    - producers and consumers marked;
    - an unpowered marker on devices without power;
    - battery charge bars;
    - disconnected devices.
- **Inspector** (`buildingPane`, `cmds_build.go:100`):
  - power lines: "Uses 50 W" / "Produces 800 W", "Grid: +350 W, stored 900 of 1200 Wh", "No power: <reason>" (not connected / brownout / switched off / solar flare);
  - bars: battery charge, generator fuel (the existing Fuel bar), the wind or solar share.
- **Commands:**
  - "Power" toggle (`SetPowerWanted`; shows "flick pending" until a colonist flicks it; hotkey `input.Power`, V, enabled);
  - the switch's toggle;
  - thermostat − / + (`SetThermostat`).
  - The bills of an unpowered bench show "No power".
- **Alerts:** "Brownout" (danger, names the net); "Not connected" (caution, devices with no grid); "Out of fuel", widened from the campfire to generators and torches.
- **Letters:** solar flare (start and end), short circuit (threat). A status chip during the flare.
- **Rendering** (presentation only; read models):
  - powered devices show their emissive bits;
  - the lamp lights only when powered and the torch only while burning (`lightSources` from `Glow`/`GlowRadius`);
  - the torch flame reuses `drawFlame`;
  - wind turbine blades turn with the wind, and generators show a working glow;
  - conduits are a linked 16-mask family (the fence recipe, `families/fence.py:88-96`, `linkgrid.go`), subtle on the map;
  - hidden conduits draw only in the overlay.
- **Art** (opus agents, skill `rimworld-go-art-assets`, our own designs, no outlines on plants):
  - Blender sprites with normals, shadow blocks and metadata (`emissive`, `light_radius`): conduit (linked), switch (on and off), wood-fired generator, solar panel, wind turbine (tower plus blade frames), geothermal generator, battery, torch lamp, heater, cooler (in-wall, facing), electric stove, electric tailoring bench, electric research bench;
  - `ui_icon` tool icons;
  - badge icons `overlay_power`, `power_toggle`, `thermostat_up`, `thermostat_down`, `no_power`;
  - the review sheet `power-buildings` (art spec `:1105`).
- **Sound** (code-made through `cmd/audiogen`; no paid generation):
  - loops: generator chug, geothermal hiss, wind whoosh, heater and cooler hum;
  - one-shots: switch click, short-circuit zap and crackle, flare buzz.
  - Loops are keyed per building in `colonyAmbience` (`sound_director.go:302`); one-shots via `r.cue`.

## Execution (`.cursor/PARALLEL_WORK.md`; after PLAY-020)
**Lead contract first** (additive, built, committed):
- Building kinds and defs: conduit, hidden conduit, switch, the four generators, battery, torch lamp, heater, cooler, electric stove, electric tailoring and research benches. Appended to `construction.go` with `LayerConduit`, `NumLayers` and `RecipeBench`.
- The `power` domain API with stub bodies.
- `job.Flick`.
- Commands `SetPowerWanted`, `SetThermostat`, and `OrderFlick` (in the order menu).
- Incident IDs `SolarFlare`, `ShortCircuit`.
- The research projects.
- `Game.pw` and `powerSave`.
- View fields:
  - `BuildingView`: PowerRole, PowerNet, Powered, PowerOn, PowerWantOn, PowerUse, PowerOut, StoredWh, CapWh, NoPowerReason, LinkX/LinkY/HasLink, Glow, GlowRadius, ThermoTarget.
  - `WorldView`: PowerNets{ID, Gen, Load, StoredWh, CapWh, Brownout}, SolarFlare, PowerVer.
- Hook lines: `Step`/`phases.go`, `heat.go:142`, `priorities.go:113`, job dispatch (`tick.go:1016`), `host_commands.go`, view filling (`game.go:1178`).
- UI hooks: the Power and Temperature categories, the overlay toggle id, `syncPowerForce`.

**Agents.** RAM: `make agent-budget` before every wave; the art slot is one at a time; targeted `GOFLAGS=-p=2` tests; reports ≤ 250 words.

| Agent | Model | Owns |
|---|---|---|
| POWERDOM | opus (graph, balance, shedding) | `internal/domain/building/power/*`: `Builder.Build` (union-find, connections, switches), `Balance` (generation, batteries, shed order, minimum off time), `SpecOf`; pure tests, including the no-flap test |
| POWERSIM | opus (phase, caches, flick) | `game/power.go`, `power_flick.go` (designation index, `job.Flick` under Construct before `assignConstruct`), `power_view.go`; the phase, net cache and views |
| FUELGEN | sonnet | `game/fuel.go` (the shared fuel table, moved from `heat.go:182-338` with the same order so goldens hold), `power_gen.go` (solar, wind, geothermal and wood-fired outputs), `heat.go`, `vents.go`, `setup_mapgen.go` (vent on `LayerFoundation`) |
| CONSUMERS | opus (cross-cutting, thermostat hysteresis) | `game/light.go` (lamps only when powered, torch light), `rooms.go` and `rooms_index.go` (`Lit` from the light-source list, nothing in `roomStats`), `power_heat.go` (heater, cooler, thermostat), `production.go` and `research.go` (electric benches) |
| EVENTS | sonnet | `game/power_events.go`, `incidents.go` (flare, short circuit, letters, chip state) |
| INFRA | sonnet | `save.go` (powerSave, version, lamp→torch migration), `placement.go` (conduit and cooler-in-wall rules), `orders.go`, `fire.go`, `bridge_remove.go`, `indexes.go`, scenario fixtures, goldens (re-pinned with reasons) |
| POWERUI | opus (overlay, placement links) | ebiten: the architect power tools and categories, `power_overlay.go`, the placement link ghost, `syncPowerForce`, hotkey |
| POWERPANE | sonnet | ebiten: the building inspector power lines, bars and commands (toggle, thermostat, switch), alerts, letter and chip UI, bill "No power" |
| RENDER | opus (light grid integration) | ebiten: sprite mapping (`buildSprite`), the conduit link flag (`linkgrid.go`), turbine blades, torch flame, emissive and powered glow, `lightSources` gating in `light.go`/`shadow_cast.go` |
| ART | opus, art rule (one at a time, the art slot) | `assets/source/blender/artlib/families/power.py` (new) plus furniture and production additions, icons, metadata, the review sheet |
| SOUND | sonnet | `cmd/audiogen/recipes.go`, `audio.json`, `sound_director.go`/`sound_signals.go` cues |

**Waves** (each after a budget check):
1. POWERDOM, FUELGEN, INFRA, ART (art slot).
2. POWERSIM, CONSUMERS, EVENTS.
3. POWERUI, POWERPANE, RENDER, SOUND.

UI agents develop against the contract and stand-ins while the sim lands.

## Tests (TDD everywhere; never weaken)
- **Domain:**
  - nets merge, split at an open switch, and keep stable ids;
  - connection range and ties;
  - balance: surplus charges with the loss, deficit drains, self-discharge;
  - shed order and its minimum off time (no-flap: ±5% ripple for two days, every off span ≥ the minimum).
- **Sim:**
  - each generator's output (solar day and night, roofed panel cells, wind speed and an obstructed zone, geothermal only on a vent, wood-fired fuel burn and refuel);
  - lamps dark without power and lit with it;
  - the torch burns logs and lights its room;
  - heater and cooler thermostats (no-flap);
  - electric benches faster, and blocked without power;
  - flicks done by a colonist;
  - the flare stops every net and ends;
  - a short circuit drains, removes the conduit and starts a fire;
  - research locks;
  - save/load mid-balance matches an unbroken run;
  - old saves' lamps become torches;
  - `StateHash` unchanged for colonies without power, except the grown fixture, which has lamps: re-pinned with a note.
- **UI:**
  - architect tools and locks;
  - the overlay draws per net (draw trace);
  - the placement link;
  - inspector lines and commands;
  - alerts and letters;
  - layout at UI scales 1, 1.5 and 2.
- **Render:** lights only from powered sources; turbine frames follow the wind; conduits autotile; hidden conduits draw only in the overlay.
- **Art:** `make art-check`, `TestGeneratedArtCoversRenderer`, orphan checks.

## Verification (lead)
1. Builds, gofmt, vet, `make lint`, the Windows cross-build.
2. The full `GOFLAGS=-p=2 go test -race ./...` run alone.
3. `make art-check`, `make audio-check`.
4. Determinism and goldens: only the stated re-pins.
5. fps probe before/after (grown 500 with a power grid added to the fixture, or a dev scenario).
6. Captures, one at a time:
   - a powered base by day and night;
   - the power overlay;
   - a brownout;
   - a solar flare;
   - a short circuit;
   - the placement link;
   - the inspector for each device type.
7. Docs:
   - handoff;
   - the vanilla checklist Phase 15, research, events, heat sources and workstations;
   - the UI checklist power overlay, no-power reason, Power category and semantic colours;
   - MAIN_UI_SCREENS (inspector Power, Power category, the overlay);
   - ART spec (power family);
   - MASTER_SPEC §39 notes;
   - README features and controls (V = power toggle);
   - ebiten `AGENTS.md`.
8. Commits by area on `main`, no push.

## Critical files
- **Lead:** `internal/domain/building/construction/construction.go`, `internal/domain/building/power/`, `internal/domain/ai/job/job.go`, `internal/domain/sim/command/command.go`, `internal/domain/incident/incident.go`, `internal/domain/research/research.go`, `internal/application/game/{game.go,phases.go,host_commands.go,tick.go,priorities.go}`, `internal/adapters/inbound/ebiten/{archtools.go,hud_global.go}`
- **Agents:** as in the table.
- **Reuse:** the campfire fuel and refuel code (`heat.go:182-338`, `setFuel` at `light.go:75`); `heatRooms` (`heat.go:118`); `lighting.Outdoor`; `underSky` (`roof.go:102`); `weather.State.WindSpeed`; the timed-event template (`weather_ng1.go`); `strikeTick` (`lightning.go`); `g.stream` (`rng_streams.go`); `startFire`; door-in-wall placement for the cooler; the fence linked-art recipe; `syncRoofForce` (`hud_global.go:518`); `toolLock` (`arch_locks.go:46`); the `buildingPane`/`buildingBars` patterns; `colonyAmbience`.

## Appendix: sim-core design notes (design review, 2026-10-25)
- **Units:** integer W. Stored energy is int64 W·ticks (2500 W·ticks = 1 Wh, shown as Wh).
- **`phasePower`** runs before `phaseClimate` (`game.go:939`, `phases.go:25`) when `now%50==0` or `pw.dirty`, integrating `dt = now − pw.last`. With no power devices it returns without writing state.
- **Domain `power`** (imports `construction` only):
  - `Role` (None, Producer, Consumer, Storage, Transmitter, Switch);
  - `Class` (shed order: Lamp, Bench, Cooler, Heater);
  - `Spec{Role, Class, Draw, Idle, Peak, CapWh int; Toggle, Hidden bool}` and `SpecOf(construction.Kind)`;
  - `Node{ID uint64; Role; Cells [][2]int; Closed bool}`;
  - `Builder.Build(w, h, nodes) Graph` (Net and Link per ID);
  - `Balance(devs, dt, now) NetOut`.
  - Transmitters are conduits and closed switches; an open switch cuts.
  - Nets are 4-neighbour union-find. NetID is the smallest transmitter ID.
  - Devices join the nearest transmitter within `ConnectRange = 3` (Chebyshev from the footprint; ties by distance, then y, then x); NetID 0 when none.
- **`LayerConduit`** is appended after `LayerFoundation`; `NumLayers` is exported.
  - Places that enumerate layers: `indexes.go:110`, `fire.go:274`, `bridge_remove.go:21`, `orders.go:482`, `placement.go:248` (new branch: under walls and benches, not on rock, water or natural features), the view `Conduit` flag at `game.go:1151` and `:1168`, `placement.go:70`, `Movable` (`construction.go:390`).
  - Checked and unchanged: `food.go:76`, `grass.go:379`, `smooth.go:36`, `home.go:82`, `plan.go:57`, `filth.go:289`.
  - `SteamVent` moves to `LayerFoundation`; `naturalAt` (`setup_mapgen.go:222`) reads that layer; geothermal is placed exactly over a vent (`placement.go:217`).
- **Numbers (ours):**
  - wood-fired 800 W;
  - solar `900 × (Outdoor−0.12)/0.88 ×` unroofed share;
  - wind `250 W` per unit of wind speed, capped at 1100, `×` clear share. The band is 5 cells along both long sides; it is blocked by walls, rock, roofs and trees, and cached per (structVer, roofVer, game hour). `roofVer` is a new counter bumped at `roof.go:87`.
  - geothermal 1800 W;
  - battery 1200 Wh, 75% charge efficiency, 8 W self-discharge, equal split (remainder to the lowest ids);
  - lamp 50 W;
  - heater 180 W and cooler 220 W, each 20 W idle;
  - electric stove 250 W, tailoring 150 W, research 200 W at full while reserved, 10% otherwise.
  - Floats are turned into permille once per balance.
- **Brownout:** if load > gen + stored/dt, shed by class, newest id first within a class. `ShedUntil = now+2500`; back only with 10% headroom.
  - No-flap test, domain: ±5% ripple, empty batteries, two days; every off span ≥ the minimum.
  - No-flap test, game: a `TestNoMicroNaps`-style check on `BuildingView.Powered`.
- **Switches:**
  - `WantOff` and `IsOff` maps; a pending flick is any building where they differ.
  - Commands: `SetPowerWanted{Target, Enabled}`, `SetThermostat{Target, Count}` (tenths of a degree); `OrderFlick` for Prioritize.
  - The index `pw.flicks` is a sorted `[]EntityID`, rebuilt on load.
  - `job.Flick` (30 work) runs under Construct, before `assignConstruct` (`priorities.go:113`).
  - A flick bumps `pw.topoVer`; the net cache key is (structVer, topoVer, store identity).
- **Events** (Nature pool; `incidentFits` is false with no power devices; their own `streamPower` stream):
  - **Solar flare:** 3–8 h, cooldown 12 d, not before day 12. Every net is down: generators idle with no fuel burn, batteries frozen. `FlareUntil` is saved; letters at the start and end.
  - **Short circuit:** cooldown 8 d, not before day 10. Needs ≥ 100 Wh stored and a visible conduit, `sortedCandidates[stream.IntN]`. Drains that net, removes the conduit, then `startFire(x, y, 1 + Wh/500)`; a threat letter.
- **Consumers:**
  - `light.go:101` stamps a lamp only if `g.powered(id)`; `setPowered` bumps `light.ver`.
  - `roomStats.lit` is deleted; `roomViews` sets Lit from one cached light-source list (powered lamps, burning torches and campfires).
  - Thermostat: `thermoRoomHeat` next to `heat.go:142`, ±220 per interval. Target 21 °C; active below target −1, idle above +0.5; the active flag is saved.
  - Electric benches: `construction.RecipeBench(k)` maps to the base kind (`production.go:122`, `:391`, `frame_view.go:170`). Unpowered benches are skipped (`production.go:203`); `doBill` fails Blocked with `BillNoPower`; speed `+40`.
  - Research bench: `research.go:141` accepts both; `doResearch` uses `workProgressPct(…, 140)`.
- **Fuel table** (`fuel.go`): `fuelSpec(k){CapTicks, Below, BurnPermil}`. Campfire is unchanged; torch 8 logs at 600‰; wood-fired 15 logs at 1600‰. `heat.go:182-338` moves there, iterating a structVer-cached fuel-user list in the same order (goldens hold). Torches burn in `phaseClimate`, and rain puts them out like campfires.
- **Saves:**
  - `Power *powerSave` (nil when empty) holds Charge, WantOff, IsOff, Unpowered, Shed, Thermo, FlareUntil and Last.
  - Derived and not saved: nets, outputs, the flick index.
  - Migration `v < 5` in `migrateSave(v)`: existing lamps become torch lamps.
- **Views:**
  - `BuildingView`: PowerRole, PowerNet, Powered, PowerOn, PowerWantOn, PowerUse, PowerOut, StoredWh, CapWh, NoPowerReason, LinkX/LinkY/HasLink, Glow, GlowRadius, ThermoTarget.
  - `WorldView`: PowerNets{ID, Gen, Load, StoredWh, CapWh, Brownout}, SolarFlare, PowerVer.
- **Research costs:** Electricity 3000; Batteries 2500, Solar 4000, Wind 3500, Geothermal 5000, each requiring Electricity. The torch lamp is free.
- **Risks:**
  - The grown golden (`grown.go:219` builds lamps) needs a re-pin with a note.
  - Conduits add buildings to full-list scans (`assignBill`, `researchTry`, the WorldView loop, `removeBuildingAs` O(n)): use per-kind lists cached per structVer.
  - Delivering materials to conduit frames under walls (`workSpot`, `orders.go:359`) needs a test.
