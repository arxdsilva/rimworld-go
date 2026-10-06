# Room heat, campfires and exposure

## Room temperature (`application/game/heat.go`)
- Each room keeps its own temperature (`g.roomTemps`, indexed like `g.rooms`).
- On a room rebuild (`remapRoomTemps`), each new room starts at the average of its cells' old values: the old room's temperature, or outdoor air for cells that were outside. The very first build starts rooms at `Indoor()`.
- Every climate interval (250 ticks), `heatRooms`:
  - Moves each room toward outdoor + 3C by `roomLeak(cells)` permille. That's 120/sqrt(cells), clamped to 10-80, so small rooms swing faster.
  - Adds heat from burning campfires. Each fire gives `campfireHeat` / cells per interval: 160 tenths·cells, which holds a 20-cell room about 30C above outside.
- A campfire banks (no heat, no fuel use) once the air around it reaches 16C.
- `temperatureAt` uses the room temperature indoors and `Outdoor()` outside.
- `RoomView.Temp` exposes it. The adapter reads it for hover.
- A room with a fuelled campfire counts as lit (`roomViews`). A campfire doesn't count as a lamp for room impressiveness.

## Campfire
- Build: `construction.Campfire`, 1x1, 5 wood, blocks movement. It's under Architect > Furniture.
- Fuel: `g.fuel[bid]` in ticks. Capacity is 20 wood, and each log burns `TicksPerWood` (6000 ticks), so a full fire burns about 10 logs a day.
- Haulers (Haul work, checked before ordinary hauling) refuel any fire at half or below:
  - New job `job.Refuel` takes up to the missing wood from the nearest free stack.
  - The hauler stands beside the fire (`besideSpot`) and works for 30 ticks.
- `BuildingView` gains Fuel, FuelPct and Burning. The inspector shows burning, banked or out of fuel.
- Alert: "Campfire out of wood".
- Presentation (`fire.go`): a live flickering flame with a ground glow, plus ember (`partEmber`) and smoke particles.
- Art: `furniture/campfire` (stone ring, crossed logs, cinders) and `ui_icon/campfire`.

## Wood
- `woodLow()` is true when wood on the ground is under max(20, two days of burning per campfire). While it's true, colonists with Grow work cut designated trees before harvesting or sowing.

## Exposure (`domain/pawn/exposure`)
- Bands for a colonist in everyday clothes:
  - Comfortable from 8C to 28C.
  - Mood thoughts: cold (-4) below 8C, freezing (-10) below -5C, hot (-4) above 28C, sweltering (-10) above 34C.
- Hypothermia builds at or below 0C, faster the colder it is, and fades above 0C (quickly once comfortable). Heatstroke is the mirror above 38C.
- Severity thresholds (permille):

  | Level | Hypothermia / heatstroke |
  |---|---|
  | 200 (Mild) | the colonist looks for a bearable room |
  | 600 (Serious) | the colonist collapses |
  | 1000 (Fatal) | the colonist dies |
  | below 400 | a collapsed colonist stands up again |

- At -10C a colonist collapses in about 2.4 hours and dies in about 4.
- `expose()` runs each climate interval and posts messages: "Ada collapsed from hypothermia" and "Ada froze to death".
- New health causes: CauseHypothermia and CauseHeatstroke. Doctors rescue these to a bed but don't tend them (no wound to tend).
- AI: a colonist with mild exposure standing in uncomfortable air:
  - Drops what it's doing (except eating) and takes a `job.Warm` to the nearest reachable cell of a bearable room.
  - Waits there up to 2500 ticks, or until recovered.
  - If no bearable room is reachable, it retries every 1000 ticks (`g.noComfort`).
- `PawnView` gains Hypothermia, Heatstroke and Temp. The inspector shows the temperature and exposure %. Alerts: "Ada has hypothermia" and "Ada has heatstroke".

## Fix: unreachable plants
- `assignCut` and `assignHarvest` now go through `nearestPlant`. It picks the closest unreserved plant the pawn can actually reach (checking up to 16 candidates).
- Before this, a designated tree walled in by rock made every colonist retry it all day ("cut (unreachable)").

## Year probe (seed 7, after these changes)
- The hut held about 16C through winter while it was -8C outside. The fire burned about 11 logs a day.
- The cut designation (top band only) ran out of wood on day 49, and the fire died on day 51. Late winter was mild by then, so the hut stayed above freezing (3C).
- All 3 colonists survived the year.

## Dev
- `Game.SkipTo(day, hour)`, `Game.FuelCampfire(x, y)`, `Game.ClearPlants(rect)`.
- `-scenario winter -capture-ui winter`: a heated and an unheated hut on a winter evening.
- The year probe now builds a hut with 3 beds and a campfire, and designates tree cutting.

## Next
- Apparel (parkas widen the cold band).
- Heaters and coolers with power, then freezers.
- Doors leaking heat when open.
- Heat flowing between adjacent rooms.
