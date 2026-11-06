# Research gap analysis (RSCH-0..RSCH-3)

Scope: research speed and cost, tech levels, the project table, research benches and the multi-analyzer, project selection, alerts and letters, the Research tab, and starting research per scenario, vanilla core only (no DLC). Load only for RSCH-* tasks. Phase handoffs are sections of `.agents/handoffs/BASE.md`. The pieces exist (a 45-project DAG, one current project, a bench job, a list UI), but projects have flat hand-tuned costs, no tech level or bench tier, and the screen is a list rather than a tech tree. Sister documents: `TEMPERATURE_GAP_ANALYSIS.md`, `FOOD_GAP_ANALYSIS.md`, `MEDICAL_GAP_ANALYSIS.md`, `BEDS_GAP_ANALYSIS.md`.

## Status (2026-11-01)

RSCH-0 (this analysis, ENG-003 extension, stale checklist row) and RSCH-1 (simulation, `.agents/handoffs/BASE.md#rsch-1-research-simulation`) are done; see the RSCH-2 and RSCH-3 rows.

| Phase | Delivers | State |
|---|---|---|
| RSCH-0 | This analysis; ENG-003 extended; inert contract (tech level and bench tier on `Def`, command and view fields, hi-tech bench and multi-analyzer names); stale checklist row corrected | done (`.agents/handoffs/BASE.md#rsch-0-gap-analysis-and-contract`) |
| RSCH-1 | Simulation: wiki cost, tech-level cost multiplier, research speed stat, bench factor, difficulty factor, project table, bench gating, hi-tech bench and multi-analyzer facility, starting research, save migration | done (`.agents/handoffs/BASE.md#rsch-1-research-simulation`; `game/research`) |
| RSCH-2 | Research tab UI (tech tree graph, info panel, search), "Need research project" alert, research-complete letter, bench inspector progress, tab tooltip | done (UI side; `.agents/handoffs/BASE.md#rsch-2-research-tab`): the alert and letter content come from RSCH-1 |
| RSCH-3 | Art: hi-tech bench, simple bench resize, multi-analyzer, project and tree chrome | done: simple bench 3x2 and hi-tech bench 5x2 in four facings, multi-analyzer 2x2 with its glow (on) layer, Architect icons and the renderer key (`.agents/handoffs/BASE.md#rsch-3-research-bench-art`); RSCH-1 footprints match; tree chrome is the RSCH-2 tab's procedural drawing (vanilla nodes are text boxes, no project icons) |

## 1. Decision record

| Item | Decision |
|---|---|
| Goal | Make research work as close to RimWorld as possible, carried through until complete ("find another major feature we're missing like the research UI and overhaul it to work as close as possible to rimworld", no questions). |
| Numbers | ENG-003 in `AGENTS.md` is extended to research (2026-11-01): published wiki numbers are tuning data; names and prose stay original. Each constant gets `// wiki: <page>`, or `// original tuning` where the wiki is silent. Items the notes mark [MEM] (unconfirmed) are original tuning until verified. |
| Project table | Follows wiki costs, tech levels, prerequisites (hidden ones appear in the description, not drawn as lines) and bench tiers, only for projects whose content exists in this game or that gate content we have. Our own projects (Bridges, TrailRations, ashleaf and silverdust drugs, and so on) map onto the nearest vanilla row's cost and tech level. |
| Content not in game | Vanilla projects with no content here (TVs, moisture pump, deep drilling, scanners, transport pods, cryptosleep, starflight chain, shields if absent) are listed in section 4 as "content not in game". No content is invented for them. |
| Cost | The tech-level cost multiplier per faction tech level replaces the flat scenario `TechCostPct`. |
| Speed | Points per tick = 0.00825 x research speed x bench factor x difficulty factor. Research speed: Intellectual 8 % + 11.5 % per level, manipulation and sight at 50 % importance capped at 110 %, global work speed. Bench factor: simple 0.75, hi-tech 1.0, +0.10 with a linked multi-analyzer, not a Laboratory x0.8, cleanliness, outdoors, temperature outside 10-35 C x0.7. |
| Benches | Simple bench becomes 3x2 (75 stuff + 25 steel). The existing `ElectricResearchBench` becomes the hi-tech bench: 5x2, 250 W, needs Microelectronics. New multi-analyzer facility, 2x2, 200 W; link range is original tuning (reuse the existing facility link radius if the game has one). Costs follow the building-cost scaling convention in `construction.go`. Old saves: a resized bench that no longer fits is refunded with a message, otherwise it keeps its place (RSCH-1 picks the safe rule and tests it). |
| Bench gating | H and H+MA projects are never worked at a simple bench, nor H+MA at a hi-tech bench without a linked analyzer. |
| No queue | Not vanilla. Research stops when a project finishes. A "Need research project" alert and a research-complete letter carrying the description replace the queue. |
| Starting research | Per scenario, following the wiki (section 8). |
| Research tab | Tech tree graph in tech-level columns with prerequisite lines; node colours done green, available dark, unavailable grey, current highlighted; pan, scroll and search; left info panel with name, description, X / Y, prerequisites, bench and facility (red if missing), unlocks, Start research and a dev finish button; selecting a node highlights its prerequisites. |
| Out of scope | The endgame ship chain (starflight, reactor, drive), techprints and DLC tabs. Not a gap. |
| Clean room | No wiki text, code, art or sound is copied. New benches and the analyzer get original art through `rimworld-go-art-assets`. No new sound events. |
| Budget | `internal/application/game` is at its SOLID ceiling (53,712 lines, 178 files; `Game` 1904 methods, 146 fields). New behaviour goes in a new `game/research` sub-package behind a consumer-owned host (at most 8 methods per interface); added game lines are paid for by moving code out. Never raise a `solid_budget.txt` line. |
| Determinism and hysteresis | No global RNG or wall clock, no per-tick global scans (cache the bench-factor per bench and invalidate on room, power and facility change). Sittings keep their 2 h minimum; bench-fit and temperature thresholds use separate enter and leave values or a cooldown, each with a no-flap test. |
| Saves | Saves keep gob names. Costs change, so progress is migrated as a fraction of cost (`save.go` migrateSave). New saved state gets Export/Import and a round-trip test. Goldens are re-pinned only in behaviour phases, and the handoff says so. |
| Player visibility | Each phase ships read-model fields and HUD/UI representation (RSCH-2 collects the screens) or states in the handoff why none fits. |

## 2. Reading the tables

State legend: `done`, `partial`, `missing`. `Today` evidence is `file:line` from the 2026-11-01 inventory; line numbers drift, so find the function. Paths are under `internal/`; `G` is `application/game/`, `D` is `domain/research/research.go`, `UI` is `adapters/inbound/ebiten/`. Wiki pages: Research, Research speed, Hi-tech research bench, Simple research bench, Multi-analyzer, Scenario system, Menus. [MEM] marks wiki points the notes could not confirm.

## 3. Gap tables

### 3.1 Mechanics and speed

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| A pawn with Research work works at a bench toward the one active project; any number of researchers and benches stack with no penalty; switching projects keeps progress (wiki: Research). | One current project (`researchCur`/`researchSet`, `G game.go:806-807`); `setResearch` accepts Available only (`G research.go:109`). Progress is a per-project array (`D :276-314`) so switching keeps progress. Several pawns can use different benches. | None beyond the sitting rules below. | none | done |
| Points per tick = 0.00825 x pawn research speed x building factor x difficulty factor; skill 8 gives 20.625 points per hour of 2,500 ticks (wiki: Research speed). | `doResearch` multiplies WorkSpeed x skill SpeedPermil x trait factor x bench % (`G research.go:170-207`, `skills.go:294-327`); `researchPoints` (`skills.go:654`) then x difficulty. Not the wiki constant. | Replace with the wiki formula in a research sub-package. | RSCH-1 | done (RSCH-1) |
| Research speed stat: Intellectual 8 % + 11.5 % per level (0 gives 8 %, 8 gives 100 %, 20 gives 238 %); manipulation and sight at 50 % importance, capped at 110 %; global work speed; minimum 10 % (wiki: Research speed). | `skill.SpeedPermil` is 1000 + 60 x (level - 7) (`skill.go:123-128`); WorkSpeed is manipulation only (`body.go:99`). | Intellectual curve, sight factor at 50 % importance, caps, minimum. | RSCH-1 | done (RSCH-1) |
| Difficulty factor: Peaceful and Community Builder x1.2; Adventure Story and Strive to Survive x1.0; Blood and Dust x0.95; Losing is Fun x0.9 (wiki: Research). | `difficulty.ResearchSpeed` 25-300 % (`difficulty.go:75,106-126`). | Map our difficulty presets to the wiki factors; keep any slider as an extra multiplier only if the preset system has one. | RSCH-1 | done (RSCH-1) |
| Building factor: simple x0.75, hi-tech x1.0, +0.10 with a linked multi-analyzer (wiki: Research, benches). | Electric bench is +40 % over the plain one (`craft/craft.go:46,57`). No analyzer. | Bench factor per bench kind; analyzer link term. | RSCH-1 | done (RSCH-1) |
| Not in a Laboratory room x0.8; cleanliness very dirty (<= -5) x0.75, clean x1, sterile (0.6) x1.09; outdoors counts as x0.75 and x0.9 [MEM]; temperature below 10 C or above 35 C x0.7 (wiki: Research). | None: no light, cleanliness, room or temperature term (`facilities.Bonus` is bills-only, `craft/station.go:121`). Room cleanliness and temperature exist (TEMP). | Room-role "Laboratory" test, cleanliness read, outdoors flag, temperature band with hysteresis. The outdoors x0.9 is original tuning (marked [MEM]). | RSCH-1 | done (RSCH-1) |
| A sitting at the bench: not specified beyond the work job (wiki silent). | `assignResearch`/`researchTry` pick the nearest free bench and reserve the spot; a sitting lasts 2 h (`G research.go:119-168`, `researchSession :23`; tick `tick.go:1080`). Unpowered electric bench skipped. | Keep; bench choice must skip benches that cannot work the current project (section 3.4). | RSCH-1 | done (RSCH-1) |
| Cleanliness research effects (vanilla checklist L1185 notes them missing). | Missing. | Covered by the building-factor row. | RSCH-1 | done (RSCH-1) |

### 3.2 Tech levels and cost

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Every project has a tech level (Neolithic, Medieval, Industrial, Spacer; Ultra has none in core) (wiki: Research). | `Def` has no tech level (`D :103-111`). | Add `TechLevel` to `Def`; the tree columns and cost multiplier read it. | RSCH-0 contract, RSCH-1 values | done (RSCH-1) |
| Cost multiplier by faction tech level: tribal pays Medieval x1.5 and Industrial or higher x2; an Industrial faction pays base. [MEM] formula: 1 + 0.5 x (min(projectTech, Industrial) - factionTech) when the project is above the faction's level, else 1 (wiki: Research). | Flat scenario `TechCostPct`: Neolithic 150 %, Industrial 100 %, Spacer 80 % (`scenario.go:204-212`, `setup.go:348`). | Replace by the per-project multiplier. The formula is [MEM], so it is original tuning that reproduces the two published tribal values. | RSCH-1 | done (RSCH-1) |
| Base costs follow the wiki table (200 - 6,000). | Flat hand-tuned costs 1,500-9,000, total 179,000 (`D :113-252`). | Rewrite costs from the project table (section 4); migrate saved progress as a fraction of cost. | RSCH-1 | done (RSCH-1) |
| Hidden prerequisites are listed in the description but not drawn as tree lines (wiki: Research). | Prerequisites are one `Requires` list, all drawn (`D :103-111`). | A hidden-prerequisite list on `Def`, enforced for availability, named in the description, skipped by the tree lines. | RSCH-0 contract, RSCH-1 | done (RSCH-1) |

### 3.3 Bench and facility requirements

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Simple research bench: 75 stuff + 25 steel, work 2,800, 3x2, factor 0.75, cannot link an analyzer, handles only simple-tier projects (wiki: Simple research bench). | `ResearchBench` 2x1, 8 wood (`G construction.go:324`). | Resize to 3x2, wiki cost per the construction cost-scaling convention; old-save rule (refund when it no longer fits). | RSCH-1 | done (RSCH-1) |
| Hi-tech research bench: needs Microelectronics, 150 metal + 100 steel + 10 components, work 5,000, Construction 6, 5x2, 250 W, factor 1.0 (wiki: Hi-tech research bench). | `ElectricResearchBench`: 65 metal, 3 components, 200 W, unlocked by Electricity (`construction.go:393`, `power.go:117`). Vanilla checklist L1699 "advanced bench" is unchecked. | The electric bench becomes the hi-tech bench (rename on the label only, gob and kind name kept); new size, cost, power, unlock. | RSCH-1 | done (RSCH-1) |
| Multi-analyzer: 40 steel + 50 plasteel + 20 gold + 8 components, work 10,000, Construction 8, 2x2, 200 W, +0.10 to the linked bench; one analyzer serves many benches, each bench links one; link range unconfirmed (4 or 8 tiles) (wiki: Multi-analyzer). | None. | New facility kind using the existing facility link mechanism; link radius original tuning (reuse the existing radius). | RSCH-1 | done (RSCH-1) |
| A project needing a hi-tech bench is never worked at a simple bench; H+MA needs a hi-tech bench with a linked analyzer (wiki: Research). | `RecipeBench == ResearchBench` is the only filter (`G research.go:119-168`). `Def` has no bench tier. | Bench tier on `Def` (S, H, H+MA); `researchTry` skips benches that cannot work the current project; the current-project card says what is missing. | RSCH-0 contract, RSCH-1 | done (RSCH-1) |

### 3.4 Selection, alerts and letters

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| No queue (not vanilla); research stops when a project finishes (wiki: Research). | Same: completion sets `researchSet=false` (`G research.go:197`). | None. | none | done |
| A research-complete letter with the project description [MEM]. | A plain `g.message` "Research finished: X. Unlocked: ..." (`G research.go:184-196`); threats already use `Letter:true` (`G climate.go:75-80`). Vanilla checklist L2743 unchecked. | Emit a letter carrying the description and unlock list. | RSCH-1 logic, RSCH-2 view | done (RSCH-1) |
| A "Need research project" alert when nothing is selected while the colony has a bench and a researcher [MEM]. | None (UI/UX checklist L1282, vanilla L2040 unchecked). The "No researcher" warning exists (`G candidates.go:236,288`). | Alert with a start and a stop threshold (a one-minute minimum) so it does not flap; no-flap test. | RSCH-2 | done (RSCH-2 UI: `hud/alerts.go` key `research` from `WorldView.ResearchAlert`, click opens the tab; RSCH-1 raises it from discrete inputs only (a pick, a finish, a bench built or lost, Research work toggled), so it cannot flicker and needs no threshold; `game/research/view.go` Alert) |
| Project choice by command. | `command.SetResearch` uses the `Build` field as the project id (`command.go:46-47`, `tick.go:128`). | Keep the command; add a typed id field only if the contract needs it. | RSCH-0 | done |
| Unlocks gate building, crop and recipe lazily. | `lockedBy`/`buildLocked`/`cropLocked`/`recipeLocked` (`G research.go:77-106`); unlock strings `build:`, `crop:`, `recipe:` (`D`). | Unlock lists re-pointed at the new rows; hi-tech bench unlock moves from Electricity to Microelectronics. | RSCH-1 | done (RSCH-1) |

### 3.5 UI

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Tech tree graph in tech-level columns with prerequisite lines; box colours green done, dark available, grey unavailable; scrollable (wiki: Research, Menus). | A scrolling list, 760 px wide, 58 px rows, auto-revealing the current project (`UI windows/research.go:30-111`). MAIN_UI section 24 describes a graph; UI/UX checklist L894 and L925 unchecked; Wave UI-4 `[ ] Research` (L2207). | Graph layout from the DAG by tech level, edges, pan and scroll. | RSCH-2 | done (RSCH-2: `windows/research_layout.go` layered DAG layout per tech level, barycentre crossing reduction; `research_tree.go` lines, colours, pan) |
| Left info panel: name, description, X / Y, prerequisites, bench and facility (red if missing), unlocks, Start research, dev finish [MEM] (wiki: Menus). | Rows carry a 220 px button, a 120 px bar, status text, description and unlocks (`windows/research.go:113-192`). `ResearchView` has ID, Name, Label, Desc, Cost, Progress, Done, Available, Current, Requires, Unlocks; no tech level, bench or ETA (`G research.go:26-38`). UI/UX L914-917 unchecked. | Panel plus view fields: tech level, bench tier, bench and facility present, hidden prerequisites, ETA. | RSCH-0 view fields, RSCH-2 | done (RSCH-2: `windows/research_panel.go`) |
| Selecting a node highlights its prerequisites; search box [MEM]. | Disabled button text "Needs X first" (`windows/research.go:113-133`); UI/UX L1983 search unchecked. | Highlight and search. | RSCH-2 | done (RSCH-2: chain highlight; search dims non-matches, Enter jumps) |
| The Research button doubles as the current project's progress bar; the bench inspect pane shows progress. | Tab tooltip with name and percent and an accent fill (`hud/tabs.go:64-68,80-118`); "Needs research: X" in `inspector/building_inspector.go:51`. | Add progress and the missing-bench reason to the bench inspector. | RSCH-2 | done (RSCH-2: tab fill uses CostNow; `inspector/research_lines.go` project, speed, analyzer link) |
| Header warning when no suitable bench exists. | "No bench" warning (`windows/research.go:150-152`, `HasBench :205`). | Make it tier-aware (simple versus hi-tech versus analyzer). | RSCH-2 | done (RSCH-2: header warning when no bench; tier reasons from `BenchMissing` in red in the panel) |
| Dev finish. | Dev hooks `CompleteResearch`, `AddResearch`, `PickResearch`, `ResetResearch` (`G research.go:209-235`). | Button in the info panel. | RSCH-2 | done (RSCH-2: "Debug: finish now" sends SetResearch then DevFinishResearch) |
| Lesson and hotkey. | Hotkey `hotkeys.go:84`; lesson `learn/lessons.go:134-139`. | Update lesson text for the tree. | RSCH-2 | done (RSCH-2: lesson text speaks of the tree and Start research) |

### 3.6 Scenarios

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Crashlanded and Naked Brutality (Industrial) start with: passive cooler, stonecutting, complex clothing, complex furniture, electricity, air conditioning, nutrient paste (wiki: Scenario system). | Starting research per scenario in `G setup.go:246` (our own sets). | Map to the nearest projects in our table; entries without content are skipped. | RSCH-1 | done (RSCH-1) |
| Lost Tribe (Neolithic): passive cooler, tree sowing, recurve bow, psychoid brewing, pemmican. | As above. | Same. | RSCH-1 | done (RSCH-1) |
| Rich Explorer: the Crashlanded set plus smithing, machining, gunsmithing, blowback operation, gun turrets (unconfirmed). | As above. | Same, marked [MEM]. | RSCH-1 | done (RSCH-1) |
| Tech cost per scenario. | `TechCostPct` (see 3.2). | Removed with 3.2. | RSCH-1 | done (RSCH-1) |

### 3.7 RSCH-1 notes (2026-11-01)

- **Speed:** `game/research/speed.go`. Points are carried in billionths. A tick earns `PointsNano(Speed, Factor, Difficulty)`, divided by the project's `research.CostPct` and the scenario's cost parts. Progress stays at base cost; the view's `CostNow` is the price at the colony's tech level.
- **Original tuning:** the outdoors x0.9 ([MEM]); the laboratory rule (no bed, and research benches x30 at least bill benches x27); the temperature hysteresis (bad below 10 C or above 35 C, good again from 12 C up to 33 C, saved as `ResearchCold`); the analyzer reach, which is the tool cabinet's 6 tiles (`craft.FacilityReach`, first analyzer in building order); the tech-cost formula ([MEM]: 100 + 50 per level the project is above the faction, capped at Industrial).
- **Difficulty:** the presets' `ResearchSpeed` is the wiki factor: Wanderer and Gentle 120, Settled and Seasoned 100, Grim 95, Merciless 90. Old saves on a moved preset read the preset again.
- **Benches:** the simple bench is 3x2 (8 stuff, 280 work, wood, stone or steel). The hi-tech bench is 5x2 (17 steel, 10 components, 500 work, 250 W), unlocked by Microcircuits. The multi-analyzer draws 200 W and has an Architect row. Secondary materials (the 25 and 100 steel, the analyzer's plasteel and gold) are not modelled, following the wave D convention.
- **Save v9:** progress keeps its fraction of the cost (a project already at exactly its new cost counts as done). Research-bench blueprints are cancelled. A bench whose new footprint and spot still fit (in bounds, standable, no other object or blueprint) stays and blocks its new cells; any other is taken down, its old materials (8 stuff, or 65 steel and 3 components) are refunded in full, and a message is posted.
- **Final mapping (fixes section 4's tentative pairing):**
  - Herbalism maps to Psychoid brewing (Neolithic, 500): healroot is ungated in vanilla, and the Lost Tribe start grants it.
  - Herbal remedies maps to Drug production, since the drug table is the drug lab.
  - Food preservation maps to Pemmican's row (own project).
  - Microcircuits is Microelectronics: it gains the hi-tech bench and needs Electricity only.
  - Powered armor maps to Marine armor, whose Recon prerequisites (Fabrication, hidden Complex clothing) stand in.
  - Bowcraft is Recurve bow only.
- **New projects:**
  - Multi-analyzer (`multi_analyzer`).
  - Passive cooling, Stonecutting, Air conditioning (the cooler moves off Electricity).
  - Hospital beds and Vitals monitors (both off Fabrication; the hidden Sterile materials is not in the game).
  - Carpet making (carpets and their five recipes).
  - Greatbows (off Bowcraft).
  - Prophyl pressing (Penoxycyline production, off Pharmacology).
  - Complex furniture (Upholstery) gains the tool cabinet (off Fabrication), vent, dresser and end table (wiki).
- **Kept deliberately:**
  - Radio (the comms console and trade beacon; vanilla puts them under Microelectronics) stays our own cheap Advanced lights row, so early trade stays reachable.
  - The wind turbine stays on our Wind power project (vanilla: Electricity).
  - Beds and double beds stay free (vanilla: Complex furniture) so a tribe and every fixture keep a buildable bed.
  - The frag grenades and molotovs need only their bench.
- **Scenarios:** Broken Hull and Bare Hands start with the Crashlanded set; Ember Clan with the Lost Tribe set, without tree sowing (not in the game); Gilded Expedition with the Rich Explorer set ([MEM]). `scenario.TechCostPct` is gone and `ResearchCostPct` starts at 100.

## 4. Project table

Costs are wiki base costs; tribal factions pay Medieval x1.5, Industrial or higher x2. Bench: S simple OK, H hi-tech, H+MA hi-tech plus multi-analyzer. `h:` marks hidden prerequisites. "Maps" names our existing project (`D`) whose cost, tech level and prerequisites move to this row; the final pairing is fixed in RSCH-1 and recorded in its handoff. "Content not in game" rows are listed, not built.

### 4.1 Neolithic (all S)

| Wiki project | Cost | Prereq | Maps to / note |
|---|---|---|---|
| Psychoid brewing | 500 | none | Brewing (nearest; ashleaf brew) |
| Tree sowing | 1000 | none | content not in game unless tree planting exists |
| Beer brewing | 400 | none | Brewing |
| Passive cooler | 400 | none | FoodPreservation (nearest; also a scenario start row) |
| Cocoa | 1000 | Tree sowing | content not in game |
| Devilstrand | 800 | none | content not in game |
| Pemmican | 500 | none | TrailRations |
| Recurve bow | 400 | none | Bowcraft |

### 4.2 Medieval (all S)

| Wiki project | Cost | Prereq | Maps to / note |
|---|---|---|---|
| Complex clothing | 600 | none | Tailoring |
| Complex furniture | 300 | none | Upholstery |
| Smithing | 700 | none | Metalworking |
| Stonecutting | 300 | none | scenario start row; content only if stonecutting exists |
| Long blades | 400 | Smithing | Bladesmithing |
| Plate armor | 600 | Smithing, Complex clothing | Armoursmithing |
| Greatbow | 600 | Recurve bow | content not in game unless a greatbow exists |
| Carpet making | 800 | none | content not in game |

### 4.3 Industrial, simple bench

| Wiki project | Cost | Prereq | Maps to / note |
|---|---|---|---|
| Drug production | 500 | none | Herbalism |
| Psychite refining | 400 | Drug production | LeafRefining |
| Wake-up | 600 | Drug production | content not in game |
| Go-juice | 1000 | Drug production | TonicSynthesis |
| Penoxycyline | 500 | Drug production | content not in game |
| Electricity | 1600 | none | Electricity |
| Battery | 400 | Electricity | Batteries |
| Biofuel refining | 700 | Electricity | FuelRefining |
| Watermill | 700 | Electricity | WaterPower (AX-0; the watermill, A4) |
| Nutrient paste | 400 | Electricity | NutrientPaste |
| Solar panel | 600 | Electricity | SolarPower |
| Air conditioning | 500 | Electricity | content depends on the cooler; scenario start row |
| Autodoor | 600 | Electricity | PoweredDoors |
| Hydroponics | 700 | Electricity | Hydroponics |
| Firefoam | 600 | Electricity | FoamSuppression (AX-0; the foam popper, A5) |
| IEDs | 500 | Electricity | Charges |
| Geothermal | 3200 | Electricity | GeothermalPower |
| Sterile materials | 600 | Electricity | SterileSurfaces (AX-0; the sterile tile, A3) |
| Advanced lights | 300 | Electricity | FloodLighting (AX-0; the flood light, A2) |
| Packaged survival meal | 500 | Nutrient paste | SealedRations |
| Tube television | 1000 | Electricity, h: Complex furniture | Television (AX-0; A6) |
| Machining | 1000 | Electricity, Smithing | MachineTools |
| Smokepop | 300 | Machining, h: Complex clothing | content not in game |
| Prosthetics | 600 | Machining | Prosthetics |
| Gunsmithing | 500 | Machining | Gunsmithing |
| Flak armor | 1200 | Machining, h: Plate armor | FlakArmor |
| Mortars | 2000 | Gunsmithing | Artillery |
| Blowback operation | 500 | Gunsmithing | SelfLoading |
| Gas operation | 1000 | Blowback operation | GasOperated |
| Gun turrets | 500 | Blowback operation | Sentries |
| Foam turret | 800 | Gun turrets, h: Firefoam | FoamTurrets (AX-0; A5) |
| Microelectronics | 3000 | Electricity | Microcircuits (unlocks the hi-tech bench; also gates the analyzer chain) |
| Wind power, Radio relay, Traps, Herbal remedies, Pharmacology, Bridges, Servo replacements | n/a | n/a | Own projects with no wiki row: map onto the nearest row's cost and tech level (Wind power as Solar panel, Radio relay as Advanced lights, Traps as Smithing, Herbal remedies as Drug production, Pharmacology as Psychite refining, Bridges as Complex furniture, Servo replacements as Prosthetics) |

### 4.4 Industrial, hi-tech bench

| Wiki project | Cost | Prereq | Maps to / note |
|---|---|---|---|
| Flatscreen television | 2000 | Microelectronics, Tube television | FlatScreens (AX-0; A6) |
| Moisture pump | 1200 | Microelectronics, h: Machining | Drainage (AX-0; A4) |
| Hospital bed | 1200 | Microelectronics, h: Sterile materials, Complex furniture | content not in game unless a hospital bed is gated |
| Deep drilling | 4000 | Microelectronics | DeepDrilling (AX-0; A4) |
| Ground-penetrating scanner | 1000 | Deep drilling | GroundScanning (AX-0; A4) |
| Transport pod | 1000 | Microelectronics, h: Biofuel refining, Machining | TransportPods (AX-0; A9) |
| Medicine production | 1500 | Drug production, Microelectronics | Pharmacology (herbal-to-industrial medicine) |
| Long-range mineral scanner | 2000 | Microelectronics, h: Machining | MineralScanning (AX-0; A9) |
| Shields | 1000 | Microelectronics, h: Complex clothing | content not in game unless shields exist |
| Precision rifling | 1400 | Microelectronics, Gas operation | RifledBarrels |
| Autocannon turret | 1600 | Microelectronics, Gun turrets, Gas operation | HeavyTurrets |
| Multibarrel weapons | 2600 | Microelectronics, Gas operation | RotaryGuns |
| Multi-analyzer | 4000 | Microelectronics, h: Machining | new project; unlocks the analyzer facility |

### 4.5 Industrial, hi-tech bench with multi-analyzer

| Wiki project | Cost | Prereq | Maps to / note |
|---|---|---|---|
| Vitals monitor | 2500 | Multi-analyzer, Hospital bed | content not in game |
| Fabrication | 4000 | Multi-analyzer | Fabrication |
| Advanced fabrication | 4000 | Multi-analyzer | PrecisionFabrication |
| Uranium slug turret | 3000 | Multi-analyzer, h: Autocannon, Precision rifling | SlugTurrets |
| Rocketswarm launcher | 3000 | Multi-analyzer, h: Autocannon | RocketBatteries (AX-0; A5) |

### 4.6 Spacer (all H+MA)

| Wiki project | Cost | Prereq | Maps to / note |
|---|---|---|---|
| Cryptosleep casket | 2000 | Multi-analyzer | Stasis (AX-0; the stasis casket, A9) |
| Recon armor | 6000 | Fabrication, h: Complex clothing | PoweredArmor (nearest) |
| Marine armor | 6000 | Recon armor | content not in game |
| Pulse-charged munitions | 3000 | Fabrication, h: Precision rifling | content not in game |
| Bionic replacements | 2000 | Fabrication, h: Prosthetics | ServoReplacements |
| Starflight basics, starflight sensors, vacuum cryptosleep, starship reactor, Johnson-Tanaka drive | 4000, 4000, 2800, 6000, 6000 | Advanced fabrication, then the chain | ShipFrames, ShipSensors, ShipStasis, ShipReactors, ShipDrives (AX-0; the escape ship, A8; chain prerequisites past Starflight basics [MEM]) |
| Machine persuasion | 3000 | Multi-analyzer | ShipMinds (AX-0; the ship computer core, A8) |

Ultra: none in core. Evidence for the 45 current projects: `D :6-98` (enum), `:113-252` (defs, costs, `Requires`). Remaining own projects not placed above (Metalworking tiers, RotaryGuns variants, Upholstery duplicates and the like) take the nearest row's cost and tech level in RSCH-1.

## 5. Spot-check

Line numbers come from the lead's 2026-11-01 inventory and were not re-grepped in this docs-only phase; RSCH-1 and RSCH-2 should re-verify each cited function before editing. Wiki [MEM] items (tech-cost formula, outdoors x0.9, info panel details, search box, letter and alert text) remain original tuning until confirmed.
