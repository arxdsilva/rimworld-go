# Refuelling gap analysis (REF-001..)

Scope: how colonists feed fuel to the buildings that burn it (campfire, torch lamps, wood-fired and chemfuel generators, pod launcher, fueled smithy and stove, passive cooler), and what the player can do about it. Core vanilla RimWorld 1.5 only. Load only for REF-* tasks. The fuel table and burning rules are PLAY-021's (`handoffs/` PLAY-021 files) and AX-4/AX-9's; this file covers what is still different. Handoff: `.agents/handoffs/BASE.md#ref-1-prioritize-refuel-auto-toggle`.

## Decision record

| Item | Decision |
|---|---|
| Goal | A burner that runs dry must be fixable by the player: order a colonist to refuel it, and switch off haulers' automatic refuels where the player wants to ration fuel. |
| Numbers | REF-1 adds none. The capacities and burn rates already in `industry/fuel.go` are wiki numbers from PLAY-021, AX-4 and AX-9. A row that would need new numbers (REF-003) is marked "own design": no page publishes them. |
| Forced refuel | Offered on any burner that isn't full, whatever its auto setting, and fills it to full. The half-tank rule is for haulers only. |
| Auto refuel off | Haulers leave the burner alone; an ordered refuel still works. A refuel already under way finishes. |
| Fish tank | Out of both controls. Its feed is our own design (FISH-*), started by choosing a stock in the tank panel. |
| Hostile buildings | No order, no switch, and haulers skip them (REF-013). |
| Look | The switch uses the burner's fuel icon (`wood`, `chemfuel`); no new art. |

## How we can use it

- **A dark generator is fixable.** Today the "out of wood" alert has no answer but waiting. The order sends a named colonist now, past busier haul work.
- **Fuel can be rationed.** Wood is also building material and cooking fuel. With auto refuel off, torches and spare campfires stop eating it, and the player feeds them by hand when it matters.
- **Chemfuel can be held for the launcher.** Switching off the generator's auto refuel keeps chemfuel in storage for the pod launcher.

## Sources

Wiki pages read on 2026-11-06. All loaded; several carry "needs fact checking" or "unverified" tags, so treat every line as the wiki's best published figure.

| Page | Used for |
|---|---|
| [Wood-fired generator](https://rimworldwiki.com/wiki/Wood-fired_generator) | 75 wood, 22 a day, switched off burns nothing, refuel toggle (1.1.0) |
| [Chemfuel powered generator](https://rimworldwiki.com/wiki/Chemfuel_powered_generator) | 30 chemfuel, 4.5 a day, 70 % explosion when damaged, radius by fuel |
| [Campfire](https://rimworldwiki.com/wiki/Campfire) | 20 wood, 10 a day, 46 in rain, "Automatic refuel toggle on/off", no wood back on removal |
| [Torch lamp](https://rimworldwiki.com/wiki/Torch_lamp) | 20 wood, 2 a day, 38 in rain, no wood back on removal |
| [Fueled stove](https://rimworldwiki.com/wiki/Fueled_stove) | 50 wood, burns only while cooking, "eject fuel", can't be fuelled while cooking |
| [Passive cooler](https://rimworldwiki.com/wiki/Passive_cooler) | 50 wood, 10 a day, waits to be restocked when empty |
| [Hauling](https://rimworldwiki.com/wiki/Hauling) | Refuel is Haul work, second after "Rearm turrets" |
| [Work](https://rimworldwiki.com/wiki/Work) | Incapable of Hauling blocks refuelling; a forced task needs its work type assigned |
| [Orders](https://rimworldwiki.com/wiki/Orders), [Menus](https://rimworldwiki.com/wiki/Menus) | No refuel order is listed (checked, nothing found) |

**Not on any page read:** a target fuel level or its setting, the fuel share at which haulers start, the right-click order's wording, any refusal text, an out-of-fuel map icon, what most buildings give back on removal, how long a refuel takes.

## Rows

State: `done` shipped; `open` owed to a later change; `kept` differs on purpose. Phase in brackets.

| Id | Gap | Wiki | Today | State |
|---|---|---|---|---|
| REF-001 | Order a colonist to refuel | Work: a colonist "can be directed to immediately perform a task", only of an assigned work type | `OrderRefuel`: "Prioritize refueling <building>" in the right-click menu; reasons "already full", "no free wood"/"no free chemfuel", "cannot reach", "in use by …", Hauling disabled or incapable (`game/orders.go`, `industry.Plan`) | done [REF-1] |
| REF-002 | Auto refuel switch | "1.1.0 - Refueling is now toggleable"; fuel delivered "as long as refueling is enabled" | `SetAutoRefuel`, `Building.NoAutoRefuel`; "Auto refuel" toggle in the building pane; the fuel line says "auto refuel off" | done [REF-1] |
| REF-003 | Target fuel level | Not published | Always fills to full | open [REF-2], own design |
| REF-004 | When haulers start | Not published | Below half a tank (`industry/fuel.go`, `Below`); pod launcher 90 %; fish tank a quarter | kept |
| REF-005 | Eject fuel | Fueled stove only | None | open [REF-2] |
| REF-006 | No fuelling while cooking | Fueled stove: "cannot be fueled while it is being cooked" | Refuel and a bill may meet at a stove; not checked | open, check the bench claim first |
| REF-007 | Out-of-fuel mark on the map | Not published | HUD alert (`hud/alerts_power.go`), inspector line and Fuel bar; nothing on the map | open [REF-2], original look through the art pipeline |
| REF-008 | Burning in rain | Campfire 46 a day, torch 38 a day in rain | Rain and snow douse a fire in the open: it stops burning (`game.burning`) | open, weather scope |
| REF-009 | Chemfuel generator explosion | "70% chance to explode" when damaged, radius by fuel | Not modelled (`ARCHITECT_GAP_ANALYSIS.md`, chemfuel generator row) | open, DEF scope |
| REF-010 | Fuel on removal | Campfire and torch: no wood back; others not published | Fuel is dropped with the building (`placement.go`, `delete(g.fuel, id)`) | matches where published, kept elsewhere |
| REF-011 | Capacities and burn rates | 75 and 22, 30 and 4.5, 20 and 10, 20 and 2, 50 and 10; stove 50 while cooking | Same (`industry/fuel.go`); smithy and stove scaled / 10 with their work | matches, scale kept |
| REF-012 | Who refuels and when | Haul work, second after rearming; incapable of Hauling can't | Haul chain: urgent hauls, then refuel (`priorities.go`, WRK-007); the order needs Hauling (`orderWork`) | matches |
| REF-013 | Hostile burners | Not ours to feed | Haulers used to refuel a settlement's burners; they now skip `Building.Hostile` | done [REF-1] |
| REF-014 | Fish tank feed | No vanilla counterpart | Fed by the refuel work with hay, kibble or plant food once a stock is chosen | kept, outside both controls |
| REF-015 | Switch on a group | Not published | One building at a time | open [REF-2] |

## Proposed phases

- **REF-1, order and switch (shipped).** Right-click order, auto refuel switch, hostile skip. Refuel planning moved to `game/industry` (`industry.Plan`) so package game stayed inside its budget.
- **REF-2, player controls.** Target fuel level (needs our own slider steps and start share), eject fuel on the stove and smithy, an out-of-fuel mark on the map (art pipeline), the switch on a group.
- **Elsewhere.** REF-006 with the next craft bench change; REF-008 with weather; REF-009 with defence.

## Already matching

Haul work refuels, right after urgent hauls. A generator that is switched off burns nothing. Stoves and smithies burn only while worked. A passive cooler waits empty until restocked. Campfire and torch give nothing back when removed. Every capacity and burn rate in Sources.

## Checklist cross-reference

Vanilla: Fuel consumption `[x]` (PLAY-021) gains REF-1's order and switch, and open rows for target level and eject. UI: the right-click order sits with the other contextual commands; the switch with the building inspector's toggles.
