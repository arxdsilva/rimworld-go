# AX-1: orders and zones

The Architect's remaining Orders and Zones rows, plus Remove floor, now work end to end: simulation, Architect cards, pane commands and lines, map marks, and journeys. ENG-003 applies (wiki numbers, original names).

## Behaviour
- **Uninstall:** marks movable, non-instant colony pieces (`Building.Uninstall`).
  - A builder works the deconstruct work and packs the building into a crated building (`item.MinifiedBuilding`, with `Stack.Building`, Stuff, Quality, and the damage carried as Wear). Nothing is refunded.
  - A deconstruct mark replaces an uninstall mark.
- **Reinstall:** `command.Reinstall` on a crate lays a blueprint (a frame whose `Reinstall` is the crate). A crate has one blueprint, so placing again replaces it.
  - Builders never build that frame bare (`crateFrame`, tick.go).
  - The new `job.Install` walks to the crate, carries it to the blueprint, works the move order's work, and installs. The building comes back with the crate's stuff, quality and wear.
  - A blueprint whose crate is gone is dropped.
- **Chop wood:** marks trunked plants (the saguaro included) from 40 % grown (wiki).
- **Slaughter:** marks tame animals only. It is the hunt mark, so the FOOD-3 job does the killing.
- **Claim:** clears `Ruin` and `Ancient` at once, with no work (wiki). That covers ruins and raider-built pieces. A claimed building joins the Home area, counts as wealth and refunds in full.
- **Strip:** marks downed people that wear or hold something. A hauler strips each one with the SIM-065 job, and the mark ends once there is nothing left to strip.
  - Corpses already shed their gear at death (PRD-2), so a corpse never needs the mark.
- **Open:**
  - A sealed sleeper casket goes through the existing casket open order.
  - A filled grave or sarcophagus is marked. A hauler works 250 ticks (original tuning) and lifts the body out beside it, unburied.
  - The stasis casket is A9's.
- **Delete zone:** removes every zone's cells (`zones.Clear`).
- **Clear build roof area:** clears the PlanBuild and AreaBuild roof bits.
- **Clear remove roof area:** clears the PlanRemove and NoAuto roof bits.
- **Remove floor:** marks floors only, through the deconstruct mark and job.
  - The refund rule, agreed with A3 and the lead, is `orders.FloorRefund`: half the material, rounded down, and none for flagstone. `orders.FloorExtraRefund` gives half of any secondary material.
  - Smoothed stone is ground, so the order never targets it. Deconstruct still takes floors too.

## Code
- **game/orders:** a new sub-package, reached only through the roles in deps.go (Buildings, Hands, Crates, Blueprints, Containers, each at most 8 methods).
  - Components: Uninstall, Install, Strip, Open, Claim.
  - The strip and open marks are saved as `saveState.Orders` (`orders.Save`; absent in older saves).
- **Adapter:** `ordersHost` and the routing live in game/move.go.
  - `ordersOf(g)` builds the component lazily, hung off `postState`, so there is no Game field.
  - Every hook is a plain function, so there are no new Game methods.
- **Contract additions** (approved by the lead): `job.Install`, `axview.Pawn.StripMarked`, `saveState.Orders`.
- **UI:**
  - Architect cards: uninstall, chop, slaughter, claim, strip, open, delete_zone, clear_roof_area, clear_noroof_area, remove_floor. Their placeholders are deleted.
  - Commands in inspector/cmds_orders.go: building Uninstall, grave Open, crate Reinstall (the `ports.ReinstallCrate` intent, which runs Move mode in crate mode, architect/reinstall.go), and downed-pawn Strip.
  - Pane lines for each mark. The frame status says "Reinstalling a crated …".
  - Map marks: overlay/designation_uninstall, designation_open and designation_strip (AR2).
- **Budget payer:** pure moves, hash-identical.
  - The scenario detail lines went to game/setupview.
  - The Health tab text went to domain body/health_text.go.
  - `IncapableReasons`, `Skill.XPPct`, and `Sex.Label`/`Build.Label` went to the pawn domain.
  - Net change to package game: about −19 lines and no new files.

## Tests
- game/ax1_orders_test.go and orders/orders_test.go.
- ebiten/journey_ax1_test.go, with seven journeys including a claim on a settled generated map.
- Two command-list expectations now include `cmd.bldg.uninstall`.
- The determinism gate passes with no golden change.
- No-flap test: not applicable. No need, threat or threshold behaviour was added; the 40 % chop limit only filters which trees a mark takes.

## Open
- A crate keeps no bills, fuel or owners.
- The Reinstall ghost uses CanPlaceAt, which shows research locks; the simulation's install ignores them.
- There is no forced "Prioritize uninstall/open" in the right-click menu (OrderUninstall and OrderOpen stay unrouted).
- The floor's deconstruct job is labelled "deconstruct"; `job.RemoveFloor` is unused.
- A4's ax4RiverColony asks for scenario "crashlanded", which does not exist (the keys are broken_hull, …).
