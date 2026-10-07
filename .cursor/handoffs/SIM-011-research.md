# SIM-011 Research

## What changed
- `domain/research`: projects (FoodPreservation, Herbalism, Tailoring, Traps,
  Gunsmithing <- Traps) with cost, prerequisites and unlock keys
  ("build:<kind>", "crop:<plant>", "recipe:<name>"); `Progress` (Done, Available,
  Locks, Add, Complete).
- New: `construction.ResearchBench` (2x1, rotatable, worker spot), `recipe.MakeRifle`
  (workbench, 12 wood), `job.Research`, `command.SetResearch` (Build = project),
  `command.WorkResearch`; Research is on by default and last in `work.Order`.
- `internal/application/game/research.go`: `assignResearch`/`doResearch` (a point per
  working tick, two-hour sittings), finish message, gates on DesignateBuild,
  DesignateGrowZone and AddBill plus `CanPlaceAt` ("needs research: X").
  `New` is a sandbox with everything researched; `NewColony` starts with nothing.
  Dev hooks: `CompleteResearch`, `AddResearch`, `PickResearch`, `ResetResearch`.
  `BuildNow` bypasses gates (scenarios/fixtures).
- WorldView `Research` and `Locked`; PawnView `WorkResearch`; saved progress.
- Adapter: Research tab (`research_ui.go`) lists projects with progress bars and
  states; locked architect tools, crops and bills are greyed with the reason on
  hover (`button.quiet`); Rsh work column live; research bench art + tool.
  `-scenario research` / `-capture-ui research`.

## Tests
`domain/research`, `research_test.go` (research at a bench unlocks spike traps;
gunsmithing waits on traps; views/locks), `research_ui_test.go`.
