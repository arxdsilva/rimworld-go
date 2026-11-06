# CI-004: the ebiten root's `-race` test time

Goal: summed top-level test time of `internal/adapters/inbound/ebiten` under `-race` at or under 450 s (it was 813 s on a quiet machine, 961 s in the lead's data). Reached: -29% under equal load, about 580 s scaled to the quiet baseline. The 450 s target is **not** met; what is left is mostly in production code (see Gaps).

## What was wrong
- Journeys waited on the simulation (a kill, a meal, a sleeper, a filled barrel) by running thousands of `App.Update`s at Very fast. Under `-race` one colony Update lays the HUD out for 3–7 ms, and six ticks cost a fraction of that, so the HUD dominated.
- A map opened after the first one (the settlement after an attack) has no view cache: `game.Game` keeps the flag per map and the renderer turns it on only for the map it starts on. With a building selected each Update rebuilt `World()` (about 0.8 s an Update under `-race`).
- The Architect box test re-picked every tool at every screen size.

## What changed (tests only)
- `journey_ff_test.go`: `ff`, `ffWatch`, `ffUntil`. The real loop is kept: a speed key starts the colony, every Update goes through `App.Update`, Space stops it. After each Update that advanced the colony, the simulation steps on to 60 ticks. An Update that ran no tick (a threat pause) steps nothing. Each loop keeps the tick budget of the one it replaces.
- `ff` (checked every 60 ticks) is used where the awaited state lasts: CombatRecords, ForbidBed, SittingRaisesComfort, HaulerFillsBarrel, BillsIngredientToggle, AutoUndraft, RoomCardStats, HunterHaulsKill, AX5RocketBattery, AX9PodsLaunch (landing), TradeCaravanAccept (trader ready), RschBuildHiTechBenchAndAnalyzer.
- `ffWatch` (Normal speed, checked after every tick) is used where a journey samples something passing:
  - HoldPosition.
  - BillsStoreModeAndRadius.
  - MechOpensDoor.
  - AX9 pod loading. It now stops on the tick the colonist steps in. With a 60-tick stride, someone else stood on the pod's tile and took the click.
- BurstFire stops once it sees two shots in one burst; the bound is unchanged.
- `wizardToColony`: `flowFF` fast-forwards the wait for the intro letter. This speeds up every wizard or world journey.
- `attackedBase` calls `SetViewCache(true)` after `ViewMap`, as the renderer intends.
- The Architect box test picks each tool once, then checks every size.

## Measurements
The HEAD binary (via `-overlay`) and the new binary ran concurrently, under the same load. Times are summed top-level `pass` Elapsed in seconds.

| test | before | after |
|---|---|---|
| CombatRecords | 59.8 | 20.1 |
| ForbidBed | 55.4 | 10.9 |
| SittingRaisesComfort | 46.2 | 15.3 |
| ClaimAfterTheDefeat | 39.2 | 6.5 |
| ArchitectBoxFitsEveryTool | 36.7 | 20.9 |
| HaulerFillsBarrel | 36.4 | 6.2 |
| BillsIngredientToggle | 31.4 | 4.0 |
| BurstFire | 28.0 | 0.5 |
| HoldPosition | 22.9 | 5.3 |
| MechOpensDoor | 22.9 | 5.4 |
| AX9PodsLaunch | 22.4 | 9.8 |
| EnemyDoorRefusesClaim | 20.7 | 4.2 |
| AutoUndraft | 20.1 | 3.4 |
| RoomCardStats | 18.7 | 0.7 |
| HunterHaulsKill | 18.6 | 1.8 |
| package, pass-sum | **1279.7** | **913.1** |

## Gaps
- **Not at 450 s.** The rest is spread over about 1,750 tests. The CPU sits in `App.Update` (the HUD), in genuine simulation, and in tsan. Further test-side cuts would remove coverage.
- **Production fixes.** These are outside the test files and would buy the remaining margin:
  - The game package (`views` in `mapState`) should enable the view cache for every map, not only the first one. The `attackedBase` workaround can go once that is fixed.
  - `construction.Parse` ranges over its defs by value, copying a large `Def` per iteration (reached from `architect.TooltipLines`).
  - `archToolCommands` and `Cards` are rebuilt on every `hud()` call through `CommandCount`.
  - `hudRightClick` recomputes the HUD inputs on every call. This is most of `TestHUDRightClickNeverStealsADraftedMove` (22 s).
- **Left as is:**
  - Wanderers' 8-hour `Step`: about 8 s under `-race` each. It is the countdown being asserted, and the cost is animal AI.
  - The wizard colony's world generation.
  - Drawing tests: CropUnits, PanelsStay, ColonyHUDDrawsEveryState.
- **Failing tests owned by other in-progress work.** They fail on both binaries:
  - `TestJourneyDropPodRaidDefended`: a raider is stuck on `kidnap goto`, costing 66–130 s per run until it is fixed.
  - `TestGrassSways`, `TestCropSwayPivotsAtRoot`, `TestUITokensMatchPaletteYAML`.
- No HUD change: this is test-only work.
