# RSCH-0: research gap analysis and contract

User request (2026-11-01): find another major missing feature and overhaul research (the Research UI) to work as close to RimWorld as possible, no questions. ENG-003 is extended to research (AGENTS.md).

## What changed
- `.agents/gaps/RESEARCH_GAP_ANALYSIS.md`: phases RSCH-0..3, decision record, gap tables (mechanics and speed, tech levels and cost, benches and facility, selection/alerts/letters, UI, scenarios) and the project table (wiki cost, tech level, prerequisites, bench tier, mapped project or "content not in game").
- `AGENTS.md`: ENG-003 exception names research (RSCH-*) and 2026-11-01.
- Vanilla checklist row "Research project" no longer says "5 projects" (45 projects, flat costs); checkbox left `[x]` for now, RSCH-1 revisits it.

## Notes
- Docs only; no code, tests or goldens touched. Inventory line numbers were not re-grepped.
- Unconfirmed wiki items ([MEM]) are original tuning until verified.

## Contract names
CT6, inert: both goldens are unchanged.
- **domain/research (new tier.go):**
  - `TechLevel` (Neolithic, Medieval, Industrial, Spacer, Ultra, NumTechLevels; String) and `LevelOf(faction.Tech)`. faction.Tech has no Medieval and is saved by value, so it isn't reused.
  - `BenchTier`: SimpleBench (zero), HiTechBench, HiTechAnalyzer.
  - `Def` gains `TechLevel`, `Bench`, `Hidden []Project`, all zero until RSCH-1.
- **construction:**
  - `MultiAnalyzer` ("multi-analyzer"): 2x2, steel only, Wood 4, Work 1000, 8 components; no Architect row and no power spec yet. The existing facility reach is 6 (tool cabinet).
  - `ElectricResearchBench` keeps its kind and Name (the art, power and unlock keys); RSCH-1 labels it "hi-tech research bench".
- **views:**
  - ResearchView: `TechLevel`, `Bench`, `BenchMissing` (strings), `CostNow`, `ETAHours` (int), `Hidden []string`.
  - WorldView: `ResearchAlert bool`, `TechLevel string`.
  - BuildingView: `ResearchSpeed int` (permille), `Analyzer uint64` (both in the equality check).
- **command:** `command.DevFinishResearch` (ignored until RSCH-1 routes it in tick.go).
- **test:** game/rsch_contract_test.go. Game package 53711 lines (budget lowered).
