# RSCH-1: research simulation at wiki fidelity

Agent S, 2026-11-01. The request was to overhaul research to work as close to RimWorld as possible. ENG-003 is extended to RSCH-*. The spec is `.cursor/RESEARCH_GAP_ANALYSIS.md`; §3.7 has the final mapping and every original-tuning value.

## What changed
- **`domain/research`:**
  - The project table carries wiki costs, tech levels, bench tiers, and drawn plus hidden prerequisites.
  - 9 projects are appended (saved order): MultiAnalysis, PassiveCooling, Stonecutting, AirConditioning, HospitalBeds, VitalsMonitors, CarpetMaking, Greatbows, ProphylPressing.
  - `Available` checks hidden prerequisites too.
  - `CostPct(project, faction)`: the tech-level multiplier.
- **`domain/scenario`:** wiki starting research per built-in. `TechCostPct` is removed, so `ResearchCostPct` holds the parts only.
- **`domain/difficulty`:** the presets' research speed is now the wiki factor: 1.2 / 1.2 / 1.0 / 1.0 / 0.95 / 0.9.
- **`domain/building`:**
  - Simple bench: 3x2, spot (1,2).
  - Hi-tech bench: 5x2, spot (2,2), 17 steel, 10 components, 250 W.
  - Multi-analyzer: 200 W.
  - Architect rows: hi-tech label and the new multi-analyzer.
- **New `application/game/research` (Lab, behind consumer roles in `deps.go`):**
  - the bench job and its gating;
  - research speed and bench factor;
  - the analyzer link (`craft.FacilityReach`);
  - temperature hysteresis;
  - the completion letter (good, with the description and unlocks);
  - the alert;
  - views (memoised; `Prereqs` built once);
  - dev finish, grant and migration.
- **Game package:**
  - `game/research.go` is now the host adapter plus the v9 bench refit/refund.
  - `ResearchView = research.View`.
  - The fields `research`, `researchCur` and `researchSet`, the research memo, `researchPoints`, `researchCostPct` and `DiffResearch` are removed.
  - `DevFinishResearch` is routed in `tick.go`.
  - Save is v9 (`ResearchCold`). The carry stays in `SkillLive.Research`, now in billionths.

## Tests
- **`game/research/lab_test.go`:**
  - Speed: skill 8 makes 20.625 points/h at the hi-tech bench and 0.75 of that at the simple one; the curve; the capacities; the difficulty factors.
  - Bench factor, gating, ETA.
  - Tech cost (a tribe pays 2x and 1.5x).
  - Letter and no queue.
  - Temperature no-flap.
  - Alert and dev finish.
  - Migration.
- **`domain/research/rsch1_test.go`:** wiki rows, `CostPct`, hidden gates, acyclicity.
- **`game/rsch_contract_test.go`:** views, dev finish, in-game gating, v8 to v9 save migration (fit, refund, message, blueprint).
- Existing tests are updated to the wiki tree (same intent).

## Budget and goldens
- The game package is at 53652 lines (budget 53711, so 59 are free to lower) with 178 files.
- `TestGrownGolden` moves to `0x51487b30909d4220` (stable over `-count=2`). The reason: wiki costs, speed and starting research.

## Player-facing
Every new behaviour reaches the read model:
- `ResearchView`: TechLevel, Bench, BenchMissing, CostNow, ETAHours, Hidden, Prereqs.
- `WorldView`: ResearchAlert, TechLevel.
- `BuildingView`: ResearchSpeed, Analyzer.
- The letter.

The RSCH-2 tab draws them.
