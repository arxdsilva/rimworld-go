# Work priorities and schedules

## Priorities
- `work.Priorities` holds one value per work type: 0 is off, 1 is done first, 4 is done last.
  - `DefaultPriorities()` puts the default colonist work at 3.
  - `Mask()` derives the old enable set.
- `work.Order` breaks ties within a priority: Doctor, Haul, Construct, Cook, Craft, Hunt, Grow, Mine. That's the old fixed order, so default behaviour is unchanged.
- `g.prio[id]` holds each colonist's priorities. It's set at spawn and saved; `g.labor` (the mask) is kept in sync.
- `assignWorkByPriority` replaces the fixed chain in `phaseAI`. It walks priorities 1-4 and, within each, work types in Order, via `assignWorkType`:
  - Haul: refuel, then haul.
  - Cook and Craft: bills filtered to that skill.
  - Grow: cut first when wood is low, then harvest, sow, cut.
- Commands:
  - `SetWorkPriority{Pawn, Work, Count}`.
  - `SetWorkEnabled` still works: on means 3, off means 0.
- Read model: `PawnView.Priorities []int`, indexed by work.Type.
- Work tab:
  - Each cell cycles off → 1 → 2 → 3 → 4 → off.
  - Cells are shaded brighter for more urgent work, with a header explaining the scale.

## Schedules
- `g.scheds[id]` holds each colonist's 24-hour grid. It starts as a copy of the colony default `g.sched` and is saved.
- `activityOf(id)` replaces the colony-wide `Activity()` in `phaseAI` and `doSleep`. The HUD's top bar still shows the colony default.
- Command `SetSchedule{Pawn, Index: hour, Mode: activity}`. Pawn 0 paints everyone, including the default for newcomers.
- Read model: `PawnView.Schedule`, `PawnView.Activity` (now per colonist) and `WorldView.Schedule` (the colony default).
- Schedule tab (`schedule_ui.go`), sharing the Work panel area with `a.panelMode` = "schedule":
  - Four brushes: anything, work, recreation, sleep.
  - An "Everyone" row plus one row per living colonist, 24 hour cells each.
  - Click, or drag with the mouse held (`handleScheduleDrag`), to paint. Unchanged cells aren't resent.
  - The current hour is outlined.

## Dev
- `-capture-ui work` and `-capture-ui schedule`. The schedule capture gives Bo a night-owl schedule and sets Ada's Grow priority to 1.

## Tests
- `priorities_test.go`:
  - At equal priority hauling beats harvesting.
  - Grow at priority 1 beats Haul at 3.
  - Priority 0 turns the work off.
  - A personal schedule applies, and an "Everyone" paint reaches all colonists.
- `hud_test.go`: schedule painting through the UI. The work cell cycle and disabled-tab tests were updated.
