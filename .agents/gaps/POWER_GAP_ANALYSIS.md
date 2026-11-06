# Power wires gap analysis (PWR-1..PWR-5)

Scope: how a powered device joins a grid, the wire that shows it, and the Reconnect button, compared with core vanilla RimWorld 1.5 (no DLC). Load only for PWR-* tasks. The earlier power system is PLAY-021 (`handoffs/` PLAY-021 files); this file covers what PWR-* changed on top of it. Handoff: `.agents/handoffs/BASE.md#pwr-power-wires`.

Wiki pages read on 2026-11-03: Power, Power conduit, Wall lamp, Cooler, Heater, Battery. The Power page calls its mechanics a stub. The Power conduit, Wall lamp, Cooler, Heater and Battery pages carry "Unverified articles" tags. Treat every number below as the wiki's best published figure, not as verified game data.

## Decisions

| Item | Decision |
|---|---|
| Numbers | The 6-tile range and the list of what carries power are wiki numbers (ENG-003). The user approved both for PWR-* by approving the plan on 2026-11-03, and added power to the `AGENTS.md` ENG-003 exception list the same day. |
| Disconnect | No Disconnect button, as in vanilla. The power switch and the Power toggle already cut a device. |
| Look | The wire is our own dark cord colour (`ui.power_cord`). The overlay and the placement ghost keep a colour per grid. Vanilla's black cord, magenta overlay and golden blueprint are not copied. |
| Reconnect order | By grid, not by conduit tile (see "Original"). |
| Existing saves | No migration. Links are derived on load. The new range and conducting devices change what an old save connects (see "Behaviour changes"). |

## What vanilla does (wiki, read 2026-11-03)

- **Connecting is automatic.** An appliance "will connect automatically if placed up to six squares away from a power transporter". How the distance is measured is not published.
- **What carries power.** "power conduits, waterproof conduits, power switches, batteries, and all generators and generator equivalents". Transporters must touch to pass power on. Appliances "consume power but don't transmit it".
- **The wire.** A blueprint shows a "golden yellow connection". Once built it becomes a black "power cord" that "is graphical only and cannot be interacted with or damaged". The power grid view shows it in magenta.
- **Reconnect.** The button changes "which power source an appliance is connected to if there is more than one in range". The Battery page says the order works "instantly".
- **Disconnect.** No page names a Disconnect button. The ways to cut power are a power switch, which cuts the grid, and the toggle-power order, where a colonist flicks a device off. No page says what happens when a conduit is destroyed.
- **Walls.** "Conduits can be placed under walls and doors." Coolers "take the place of a wall tile". No page says a wall blocks a wire.

## Original (unpublished, so ours)

These rest on no wiki page. They are design choices, recorded so nobody mistakes them for vanilla.

- **Chebyshev measure.** `power.ConnectRange = 6` is measured in Chebyshev tiles from the footprint (`internal/domain/building/power/power.go`). The wiki gives the number and not the metric.
- **Which conductor among equals.** Of conductors at the same distance the wire takes the shortest straight line, then the smaller y, then the smaller x (`power/graph.go`, `footDist`, `nearest`). The ghost uses the same order (`GhostLink`).
- **Reconnect cycles by grid.** `Link.Next` is the nearest conductor of the next grid in range, by ascending net id, wrapping (`power/graph.go`, `next`). The wiki does not describe the order. Cycling by conduit tile would take many clicks on a long conduit. With one grid in range the button stays off, so it cannot move a wire to another tile of the same grid.
- **One wire per touching device-only set.** A set made only of generators and batteries that touch each other wires out once. The owner is the smallest-id member whose pin is usable, else the smallest-id member with a conductor outside the set in range (`chooseOwners`). The other members have no wire.
- **The cord look and colours.** A thin quad, `max(1, tile*0.04)` wide, from the edge of the device's cell to the centre of the wiring's cell (with a stub from the cell's centre drawn under the objects, so the wire reaches a sprite of any size; `DrawCordStubs`, built wires only), in `ui.power_cord` (`assets/palettes/ui.yaml`, `uikit.PowerCord`), a small plug square at the wiring end, a faint pale rim (`ui.power_cord_rim`, 2 px wider, 40 % of the wire's alpha) because a dark line alone vanished on a slate floor in a capture, alpha 0.6 for blueprints. The overlay colours the same wire by grid.
- **Reconnect applies at once, with no colonist.** This follows the Battery page's "instantly". Which devices show the button is ours.

## Items

Columns: id, vanilla (wiki page), ours before PWR (evidence), fix (what shipped), status.

| ID | Vanilla (wiki page) | Ours before PWR (evidence) | Fix | Status |
|---|---|---|---|---|
| PWR-1 | Thin cord always visible once built (Power) | The link line drew only while the power overlay was on (`render/overlay/power_overlay.go`, `DrawOverlay`) | `Wiring.DrawCords` in `render/overlay/power_cords.go`, called from `render/world/draw_art.go` after the object pass and before weather and fog. One batched draw, none when no wire, not on the Map zoom tier or in a bake. `Bakes.CordChunks` (`render/world/lod.go`) widens the chunk lists by one chunk so a device just off-screen still draws its wire. Every device with `HasLink` and a distinct from and to cell gets one | done |
| PWR-2 | 6 squares (Power) | `ConnectRange = 3` | `ConnectRange = 6`, Chebyshev. Catalog cards say 6 tiles (`architect/catalog.go`) | done |
| PWR-3 | Batteries and generators carry power (Power) | Only conduits and closed switches conducted (`power/graph.go`, `conducts`) | `conducts` also returns true for `Producer` and `Storage`. A device that touches another conductor is on that grid with no wire. Otherwise it wires to the nearest conductor outside its own set. A lone generator or battery with nothing touching or wired to it has net 0, so "Not connected" stays accurate | done |
| PWR-4 | Reconnect button (Power, Battery) | None | `command.ReconnectPower` (Target: the device). `cmdReconnectPower` (`application/game/power_flick.go`) sets the pin to `Link.Next`, bumps `topo` and balances at once. It does nothing when the target is not a device or `Next` is 0. `BuildingView.PowerRelink` feeds the inspector button `cmd.power.reconnect` (`inspector/cmds_power.go`), icon `power_reconnect`. Disabled reasons: "No grid in range.", "Touching the grid: no wire to move.", "No other grid in range." The pin is saved (`powerSave.Pins`, additive) and pruned when the owner stops being a device | done |
| PWR-5 | A blueprint shows its connection in golden yellow (Power) | Only the ghost under the cursor showed a link (`architect/marks.go`) | Placed frames of `PowerDeviceKinds` draw a dimmer wire in the target grid's colour (`power_cords.go`, `planOf`, via `PowerOverlay.GhostWire`). The cursor ghost uses the same `GhostWire`. A ghost that joins by touching shows no "Not connected" tag | done, with one known gap (below) |
| none | No Disconnect button | Switch and Power toggle exist | No change | no change |
| none | Conduits under walls | Already supported (`TestConduitUnderWallSurvivesSaveAndLoad`) | The wire is drawn over objects, so a wall hides no wire to a conduit under it or a cooler set in it | no change (PWR-1 covers the look) |

## Behaviour changes in existing saves

- A device 4 to 6 tiles from a conduit now connects on load. This can raise a grid's demand.
- Generators and batteries that touch each other now join into one grid.
- Links are derived, not saved, so no migration is needed. A save with no pins loads as before (`TestPowerSaveWithNoPins`).

## Known behaviours

Each is what the code does today. None is a bug being tracked unless it says so.

- **Open switch beside a generator.** An open switch cuts its cells, so nothing conducts there. A generator whose only neighbour is an open switch has no neighbour to join. It wires around the switch to a conduit in range on the far side, as lamps already do (`TestBuildConductors`: "an open switch cuts the cell a generator shares").
- **Two device-only sets in range.** Each set wires out once, so two wires may overlap (`TestBuildConductors`: "two sets in range each wire once, and merge"). Reconnecting one set's wire merges it with the target set. A pin on a set that is itself wired merges only that set (`TestBuildPin`). A set wired to the one that moves goes along with it, so one Reconnect can move more than one generator or battery off a grid (`TestBuildPinCarriesWiredSets`).
- **A pin whose target is gone.** The pin stays stored (`TestReconnectPinPruned`) and the wire falls back to the nearest conductor, because `pinned` finds no target. Pins are dropped only when the device itself stops being a device. A rebuilt conduit is a new building with a new id, so the wire does not jump back to it.
- **A pin cut by a switch.** The wire falls back while the switch is open and returns to the pin when it closes (`TestReconnectThroughASwitch`). The pin stays through both.
- **Ghost, known gap.** A new generator or battery that touches a wireless grid of only generators and batteries (net not 0), and has a conductor of its own in range, gets a wire from `Build` (it becomes the set's owner). The ghost cannot tell that grid from one that already has a wire without reading every member, so it reports "joined" and draws no wire. A lone battery with a consumer wired to it is the same gap. Not fixed. `TestGhostWireAgreesWithBuild` (`render/overlay/power_cords_test.go`) skips exactly this case and counts it, and checks the two agree over seeded layouts everywhere else.
- **No Disconnect button.** The wiki names none. The power switch and the Power toggle cover it.

## Counts

| | Count |
|---|---|
| Rows PWR-1 to PWR-5 | 5 |
| Missing | 0 |
| Partial | 0 |
| Done | 5 |
| No-change rows | 2 |
