# TEMP-1: room thermodynamics (agent R)

Wiki fidelity under ENG-003 (temperature). Gap rows: `.cursor/TEMPERATURE_GAP_ANALYSIS.md` §3.2, §3.3.

## What changed
- New `application/game/thermo` behind a host in `game/thermo_host.go` (`power_heat.go` moved there with `git mv`). The equalisation math is in `domain/world/temperature/equalise.go`.
- Rooms equalise every 120 ticks. Exposure counts (wall length, double walls, roof kind) are cached per room and recounted on rebuild, roof change or structure change. IndoorBonus is removed.
- Sources use the wiki heat/s rates, converted per room cell count.
- Power: heater 175 W (17 W idle), cooler 200 W (20 W idle). Costs are 6 and 10 steel after the wave-D divide-by-9 rule. The machine hum plays only above midway between idle and working watts (`render/cue/machines.go`).
- New buildings: passive cooler (50 wood, 10 a day) and wall-mounted vent (open/closed, `ToggleVent`). Exact edits: `combat/door.go` `IsOpen`; `power.go` prune keeps vent toggle state; `render/world/objects.go` and `render/overlay/power_draw.go` (vent open state, wall link); Architect rows for both.
- Views: `BuildingView.HeatRate/Open`, `RoomView.ExposedWalls/DoubleWalls/HeatRate/RoofKind`, `WorldView.TempVer`.
- Campfire banks at 28 C (4-hour marks only) and relights below 18 C; winter no-flap test.

## Wiki vs original numbers
- **Wiki:** wall constant 0.0042512 (within 1 % of the table; a double wall halves), thin roof 0.0060001, overhead mountain no exchange with passive cooling at 15 C or more, 25 % unroofed means outdoors; heater 21, cooler -21, passive cooler -11 (floor 17 C), campfire 21 (cap 28), torch 3.5 (cap 23), smelter 9, fueled stove 4, electric stove 3, wood generator 6, body heat 0.3 x BodySize x 4.1666665 (animals x0.6).
- **Original tuning:** doors 1.5 and vents 3 degree-cells per degree; cooler hot side 125 % with output falling with lift (zero at 50 C); fire 42 heat/s at size 100; steam vent 1 heat/s; heat-to-air 1.15 degree-cells per heat/s per 120 ticks (calibrated so the wiki proofs hold); unroofed cells below the 25 % rule exchange at 4x thin.

## Proof
- At -17 C outside a 9x9 room holds 20.8 C and an 11x11 holds 9.5 C (wiki: about 20 and 10). One cooler holds a 10x10 at 10.7 C with 30 C outside and 15.7 C with 35 C (wiki: 10-16 C).
- Thermo coverage 83.4 %. 250x250 bench (961 rooms): p99 16 us per step, 0 allocs, recount 0.5 ms.
- Game package: -164 lines, `Game` -6 methods, -1 field.
- Journeys: `journey_temp1_test.go` (J).

## UI still owed (TEMP-5)
Vent toggle and passive cooler panes, heat output lines in the building pane, room stats showing the exchange terms, the temperature overlay.

## Open
- Crematorium heat (no crematorium kind).
- OutdoorsForWork and psychologically-outdoors definitions are not modelled.
- The 36-region / 5184-tile room cap is not checked.
- Fire ignition (about 235 C) and burn thresholds are not modelled.
- Door and vent rates are original; the vent chain penalty is not modelled.
