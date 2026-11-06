# REF-1: ordered refuels and the auto refuel switch

A burner that ran dry ("X out of wood") had no player answer. Now a colonist can be ordered to refuel it, and each burner has an Auto refuel switch. The gap analysis is `.agents/gaps/REFUEL_GAP_ANALYSIS.md` (REF-001..015). No new numbers, so no ENG-003 extension.

## What changed
- **Planning moved.** `game/industry/refuel.go` holds the trip plan haulers and the order share: `Plan(RefuelHost, pawn, burner)` (fuel choice: wood, then ember oil; chemfuel only for the chemfuel generator and pod launcher; hay, then tank feed), `Room`, `RefuelWork`. It also holds `Controllable` (a fuel user that is not a fish tank or hostile) and `Gate` (the order's pre-checks). `game/fuel.go` keeps the candidate loop and sort; its `refuelHost` adapts the game. The move alone was hash-identical: grown fixture, seed 9, 120 tiles, `0x78f1eca8840ed959` after 2000 ticks and `0x8c4d9b4e88416ef7` after 30000.
- **Order.** `command.OrderRefuel`. The right-click row "Prioritize refueling <kind>" (`orders.go`, `OrdersAt`) is offered on controllable burners. The order:
  - ignores the half-tank threshold and the switch, and fills to full;
  - needs Hauling (`orderWork`);
  - holds the burner once started.
  Reasons: "already full", "no free wood" (after ember oil), "no free chemfuel", "cannot reach", "in use by …", "hauling disabled", "Incapable: …".
- **Switch.**
  - `command.SetAutoRefuel` sets `Building.NoAutoRefuel` (`tick.go`); it applies to controllable burners only.
  - `assignRefuel` skips switched-off and hostile burners (REF-013). A delivery under way finishes.
  - `BuildingView.Refuelable` and `AutoRefuel` feed the inspector toggle `bldg.autorefuel` (`inspector/refuel.go`; icon `wood` or `chemfuel`). The fuel line adds "auto refuel off".
- **Save.** The field rides in the saved Building. A save without it loads with auto refuel on.

## Budget
Package game was exactly at budget. The move paid 16 lines. The order and the switch took 15, part of which was paid by compacting `setDraft` and `nearestFree`. The ceiling was lowered to 52,706.

`construction.Building` is now at the 25-field limit, so the next field needs a split.

## Tests
- `industry`: `TestRefuelPlan`, `TestRoomRounding`.
- `game`:
  - `refuel_order_test.go`: the forced fill above the threshold, the order working with the switch off, the reasons, unreachable, chemfuel and tank, the ember oil fallback, the Shift queue;
  - `refuel_toggle_test.go`: haulers skip a switched-off burner, the switch rejects non-burners, survives a save, hostile burners skipped, the view fields;
  - `TestOrdersAtReasons` has the row.
- `inspector`: `TestRefuelToggleShownForBurnersOnly`.
- Journeys: `journey_refuel_test.go`.
- `TestGrownGolden` is unchanged (`-count=2`).

## Notes
- The checklists' "stale" stove-fuel lines were checked and left alone: `FOOD_GAP_ANALYSIS.md:105` is the before column of a done row, and `PRODUCTION_GAP_ANALYSIS.md:134` is the PRD-3 phase plan.
- Order labels and reasons stay plain strings like their neighbours in `orders.go`. The inspector strings are in `en.json`.

## Open (REF-2 and elsewhere)
- REF-2: target fuel level (REF-003), eject fuel (REF-005), an out-of-fuel mark on the map (REF-007), the switch on a group (REF-015).
- REF-006: stove fuelled while cooking.
- REF-008: rain burn rates (weather).
- REF-009: chemfuel generator explosion (defence).
