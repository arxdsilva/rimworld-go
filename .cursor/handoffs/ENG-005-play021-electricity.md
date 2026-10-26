# Task Handoff

Task ID: ENG-005 (PLAY-021; plan `.cursor/PLAY021_PLAN.md`, with sim-core design notes in its appendix)
Status: complete

## Summary
The user asked for electricity "just like RimWorld does" (2026-10-25). The behaviour follows RimWorld; the numbers and art are our own.

**User decisions:**
- generators: wood-fired, solar, wind, geothermal;
- lamps need power, and a wood torch lamp lights colonies before electricity;
- switches and power toggles;
- solar flare and short circuit;
- heater and cooler;
- electric benches;
- a RimWorld-like research tree.

**Domain** (`internal/domain/building/power`): pure and fully tested.
- Nets are union-find over conduits and closed switches (stable ids; an open switch cuts).
- A device links to the nearest transmitter within 3 tiles.
- Each balance:
  - charges batteries at 75% efficiency;
  - self-discharges 8 W;
  - drains batteries on a deficit;
  - sheds consumers by class (lamps, benches, coolers, heaters, newest first) with a 2500-tick hold and 10% headroom (no-flap tested).

**Sim** (`game/power*.go`, `fuel.go`, `power_gen.go`, `power_heat.go`, `power_events.go`):
- `phasePower` runs every 50 ticks over a cached net graph; colonies without devices keep no state.
- **Generators:** wood-fired 800 W while fuelled and on; solar up to 900 W by daylight and its unroofed share; wind up to 1100 W by wind speed and a clear band; geothermal 1800 W, exactly on a steam vent (vents moved to the foundation layer and still occupy their cells).
- **One fuel table:** campfire, torch lamp (8 logs), wood-fired generator (15 logs).
- **Consumers:**
  - lamps (50 W) light only while powered;
  - heater (180 W) and cooler (220 W, in a wall gap, cold side and hot side) hold a 21 °C target with hysteresis;
  - electric stove, tailoring and research benches work 40% faster and stop without power.
- **Flicks:** a colonist flicks switches and toggles (`SetPowerWanted`, `job.Flick`, an order-menu entry).
- **Events:**
  - Solar flare: 3–8 h, every net down.
  - Short circuit: drains a charged net, blows a visible conduit and starts a fire.
  - Neither touches the RNG in a colony without power.
- **Research:** Electricity (conduits, switch, wood-fired generator, lamp, heater, cooler, electric benches), then Batteries, Solar, Wind, Geothermal.
- **Saves:** version 5 turns old lamps into fuelled torch lamps; the power state is saved only when present.

**UI:**
- **Architect:** a Power category; the torch lamp in Furniture, the heater and cooler in Misc, the electric benches in Production. Cards name draw, output and lock. The placement ghost links to the nearest conduit or says "Not connected".
- **Shift+P power grid overlay:**
  - a colour per net, hidden conduits included;
  - link lines;
  - producer and consumer marks, battery bars, no-power signs;
  - a legend.
  - It is forced on while a power tool is in hand.
- **Pane:**
  - uses or produces, "Up to N W" for sun and wind;
  - the grid balance and stored energy;
  - a switch's state; a thermostat's target and whether it heats, cools or idles;
  - flick pending; "No power: <reason>" in the warning colour.
- **Controls:** the V power toggle and thermostat ±.
- **Alerts and notices:**
  - alerts: brownout, not connected, out of fuel (widened);
  - a solar flare chip and letters;
  - "no power" on bills.
- **Research window:** now scrolls (16 projects).
- **Selection:** a hidden conduit is selectable only while the overlay shows.

**Rendering:**
- conduit autotile;
- glow layers only while running;
- turbine blades with their own phase;
- torch flame; generator smoke and steam;
- battery charge bar;
- a soft no-power badge;
- light sources gated on the view's `Glow`.

**Art (opus):** the power family, the electric benches, the torch lamp, tool icons and power badges; review sheet `assets/generated/power_buildings.png`.

**Sound (code-made):** generator, geothermal, turbine, heater and cooler loops; switch click, short-circuit zap, flare alarm.

## Notes
- **Goldens:**
  - The grown fixture's pin moved twice: it completes the five new research projects, and its lamps now save a power state.
  - Scale and biome goldens are unchanged.
- **Measurements** (grown 500, fair probe):

  | Build | fps avg | 1% low | Update avg | Draw avg |
  |---|---|---|---|---|
  | Before PLAY-020 | 118.8 | 76.1 | 2.20 ms | 2.01 ms |
  | Now | 120.0 | 80.0 | 2.55 ms | 1.82 ms |

  - The probe now holds its speed and reports the ticks run. An apparent 4x Update "regression" was a baseline paused by a threat.
  - Power balance: 8.3 µs per run on 368 devices.
- **Dev:** `-scenario power` (a small base north of the start), and the capture states `power` and `power-overlay`.
- **RAM budget:** `cmd/rambudget` has a `capture` kind (2.2 GiB).

## Left open
- No communications or defensive powered devices.
- Solar ignores weather and eclipses.
- Lamp light steps per tile.
- The standing lamp sprite still has an always-lit bulb (needs a glow layer like the new devices).
- The overlay doesn't print a net's production vs consumption (the pane does).
- Buildings can't form groups for the merged power toggle.
- The audio budget is full (17 KB left), so the loops are short (0.3–0.5 s). Dropping the third thunder variant would free 384 KB; that's the user's call.
- `refundFrame` drops a cancelled conduit frame's materials on the wall cell.
