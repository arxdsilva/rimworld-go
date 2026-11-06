# NG handoffs

Families: NG (the NG1 new-game plan). Each section condenses one task's handoff; the full original of `NG1_NEW_GAME_PLAN` is `git show 76d605d04:.agents/handoffs/NG.md#ng1_new_game_plan`. New sections go at the end, in the format of `.agents/templates/HANDOFF.md`.

## NG1_NEW_GAME_PLAN
NG-1: complete new-game flow, big maps at 60 fps, generated colonists, factions, water · 2026-10-22 · fbc76fd69
- Scope: plan of record for the full vanilla-style setup (Main Menu, Choose Scenario, Scenario Editor, Choose Storyteller, Custom Difficulty, Create World, Landing globe, Factions dialog, Create Characters, loading, arrival) with original names, text, art and numbers. Staged S1-S5 over disjoint agents; lead-only files (go.mod, Makefile, `cmd/game`, palettes, `app.go`, `nav.go`, `capture.go`, `game.go` fields).
- User decisions D1-D16 (binding): everything in one plan; rotating 3D hex globe; skills/passions/traits/backstories affect gameplay; real font everywhere (Noto Sans OFL, Ebiten `text/v2`); original Blender/painted storyteller busts; map sizes 200-300 plus up to 500x500 at 60 fps (3x speed, grown colony); real factions (types, names, leaders, goodwill, settlements); commitment save mode enforced; 2 sexes x thin/average/heavy = 6 builds, atlas-packed; four scenario archetypes plus custom; drop pods plus standing arrival; water terrain from landing-site features; components and industrial medicine added (silver = currency); threats from wealth + time, difficulty multiplier, adaptation, optional time-based mode, storytellers differ in pacing; wildlife cap linear to 120^2 then sqrt(area), cap 300; names below.
- Names (D16): storytellers Orla Vane "the Steady Hand", Bram Holloway "the Hearthkeeper", Kestrel Quay "the Gambler"; difficulties Wanderer, Gentle, Settled, Seasoned, Grim, Merciless + Custom; scenarios Broken Hull, Ember Clan, Gilded Expedition, Bare Hands. `cmd/contentlint` blocklists RimWorld proper nouns.
- Compatibility rules: `Config.Setup == nil` keeps the old path bit for bit (tests, `-scenario`, captures); `SpawnNamedPawn` makes a neutral pawn (all skills 7 = exactly 1000 permille, no traits, adult, average build, `look.Legacy`); `TestTemperateGenerationUnchanged`, `testdata/premotion_v1.sav` stay green; golden hashes pinned before engine work, any re-pin recorded with its reason (plant staggering, corridor paths, `workProgress`).
- UI framework: `text/v2` font API (`drawText/measure/wrapPx/lineH/ellipsize`, type scale 11/13/13sb/16sb/22sb/32sb, UIScale 90-130%); 19 new palette tokens plus `world.yaml`; layout dialog frame min(sw-80,1240) x min(sh-80,760), minimum window 1280x720; immediate-mode kit (checkbox, radio, slider, text field, scroll view with wheel routing, tooltip after 24 frames, modal stack, table, tabs, progress, sparkline); Esc order text edit -> top modal -> Back; wizard draft `newGameState` kept across Back, discarded on Main Menu.
- Scenario/storyteller/difficulty: scenario `Def` of tagged `Part`s with `Validate/Encode/Decode/Apply`, versioned JSON store `scenariofile` in `<UserConfigDir>/rimworld-go/scenarios/`; storyteller modes Cycle (8 on / 5 off), Calm (9-15 day gaps, LossEase +6 days at 70%), Dice (p=1/8 +1/40 per quiet day, points x0.6-1.5), hidden Legacy (today's constants, used for `Setup == nil` and v1); presets table (ThreatScale 5/35/65/100/145/200%); threat points `wealthCurve` + 25/colonist, x ThreatScale x adaptation; `maxScaledRaiders` 8 + area/40000 capped 24; Adapter hysteresis (dead band 3, 2 points/day, death -25 days).
- World/site: icosahedral dual 10f^2+2 tiles (f=48 -> 23,042), coverage cap 30/50/100%, phased deterministic `Builder.Step(units)` (shape, elevation 38%+-6% land, climate, biomes, hills/caves/stones, rivers, factions, roads), `Site` -> local map with new terrain ShallowWater/DeepWater/Mud/Track, coast, river (creek 3, river 9, large 15 wide, fords every 45+-10), lakes, safety flood-fill >= 70% soft ground; seasons local (south +30 days, `Climate.SeasonAt`).
- Pawns: 11 skills, `SpeedPermil(L)=1000+60(L-7)`, XP cost 20000+6000L, passion learn 35/100/150%, decay with no-flap band; 18 traits (+Sturdy) with conflict groups; ~24 childhood + ~30 adulthood backstories (shipped as 59), incapable bits -> work masks; `pawngen.Generate(seed, slot, roll, rules)`; stat sheet order base, body, age, condition, traits, difficulty; `workProgress` with permille carry replaces `workTick`; learning cap 4000 XP/day.
- Factions: settlers, clans, drifters, freebooters, swarm, player; hostile at <= -50 (stops at >= -10), friendly at >= +50 (stops at <= +30); daily drift 1/day toward baseline (Settlers +10, Clans 0, Drifters -30); repelled raid -5; Poisson-disc settlements, capitals; `PickRaider` hostile human only, else the raid converts to pack/manhunters/hive.
- Items/buildings: Components (value 28), IndustrialMedicine (24), Machinery ore, recipes Fabricate component and Refine medicine (research Fabrication, Pharmacology), ToolCabinet (+6% per cabinet, max 2), HospitalBed (heal +15%, infection x0.5), PodShell (6 steel); medicine tier chosen at job start (no flap); steel `MetalOK` for furniture.
- Arrival: one pod per colonist plus one per 300 kit value, within 6 tiles, staggered over 180 ticks, "dazed" -4 for a day; Ember Clan stands in a ring; intro letter once; Options "Pause on start" default on.
- Saves: `SaveVersion = 2` additive (Setup, factions, climate, story state, adapter, bios, skills, carry, conditions, scheduled, comp frames, arrival); world regenerated on load from params + tile; v1 still loads (Setup nil, `pawngen.Legacy` bios); `SaveInfo` gains Commitment/Storyteller/Difficulty/MapSize/Biome. Commitment mode: single `.rwsave`, no autosave copy, saves on daily hour/return to menu/quit/window close, atomic tmp+fsync+rename, Load disabled, dev mode disabled.
- Engine scale: budgets avg tick <= 1.5 ms at 500^2 (1.0 at 250^2), p99 <= 4 ms, read models <= 0.5 ms, draw <= 6 ms/6000 calls, <= 2 KB/tick; E0 measure first (grown-colony fixture, `cmd/benchmark`, `statehash`, `TestScaleGolden`), E1 bitset grid + per-cell indexes + versioned read models, E2 16x16 chunks/regions/incremental rooms, E3 region-label reachability and corridor A* (> 96 tiles), E4 staggered plants (`GrowSpan` per chunk every 250 ticks) and job-giver indexes, E5 baked chunk/whole-map LOD, E6 saves <= 4 MB / 300 ms, load <= 1.5 s, generation <= 2 s.
- Map-size gate (6.9): standard sizes enabled when budgets pass at 250^2; large (350-500) also need a 10-minute 3x manual run at >= 59 fps; `game.MaxEnabledSide` is lead-owned, 120 until the gate passes; 120x120 offered as "Medium (pre-release)".
- Art/audio: 6 builds from one parametrised pawn rig, `cmd/atlasgen` shelf packing (pawn family <= 120 MB), 5 new hair styles and 3 beards, painted storyteller busts (512x640 + 120x150), faction/scenario/skill/passion icons, water tiles + `water.kage` ripple (Reduced Motion freezes), pod effects, globe relief geometry from `world.yaml`, procedural audio ids `ui.slider_tick`, `ui.page`, `sfx.pod_*`, `amb.water_*`.
- UI representation: every screen above plus Factions window, Options "Story" tab, pawn Bio tab, Work-tab incapable cells and skill numbers, minimap (maps > 150^2, `M`), faction-named threat letters; 18 `-capture-ui` states.
- Limits (what stayed partial, see checklist rows): disease events (DiseaseFreq hook only), relationships on starting pawns, no Artistic skill, backstories in Go not data files, no equipment/environment stat layers, no quality or failed construction, factions have no effect on settlements/traders, no migration tool or warning dialog, other systems still tick per tick while plants are staggered, colonist-count benchmark scales 100-1,000 not covered.
- Out of scope (0.5): trade, caravans, world travel, quests, disease events, Workshop/mod scenarios, multiplayer, Learning helper, per-tile world weather, bridges over water, fishing.
- Handoffs the plan named: UI-020, ENG-001, SIM-030, SIM-031, SIM-032, SIM-033, SIM-034, ART-014.

### Open (verbatim)
Parts of `_plans/NG1_NEW_GAME_PLAN.md` that still own an open checklist row, copied unchanged, grouped by the plan's headings with the row each serves.


#### 0.5 Out of scope
_Rows: Difficulty profile / Custom difficulty (disease), Goodwill changes (trade, quests)_

### 0.5 Out of scope (recorded so nothing is half-implied)
Trade, caravans, world travel and quests are out of scope; the Factions dialog shows goodwill only. Also excluded:
- disease events (only the difficulty hook ships);
- Workshop / mod scenarios;
- multiplayer;
- a "Learning helper" overlay;
- per-tile world weather;
- bridges over water;
- fishing.


#### 2.5 Custom Difficulty (item 4 only)
_Rows: Custom difficulty (NG-1: Disease frequency slider has no effect)_

    4. **Health and mind**: Mood offset slider (−20 to +20), Infection chance (0–300%), Disease frequency (0–300%, with a note "No diseases yet — takes effect when they arrive"), Pain tolerance offset (−20 to +20%).


#### 2.9 Create Characters
_Rows: Character details (relationships not shown); Generic inspect panel (Bio tab)_

  - **Under the names:** "Female, age 34 (biological), arrived <n> years ago" (chronological age = biological; no cryosleep in our lore). Then the player faction badge (scenario faction type) and the build (Thin / Average / Heavy).
  - **Portrait:** 160×200 on the right of the name block. Rendered by a **standalone portrait renderer** (`ebiten/portrait.go`, `drawLookPortrait(dst, look, x, y, w, h)`), which uses the art catalog without a `Renderer`. Today's `drawPawnPortraitArt` needs a `*Renderer`, so the layer logic is extracted and shared by both.
  - **Backstories** (`tsLabel` headings): Childhood `<title>` and Adulthood `<title>`. Hovering a title shows a tooltip with the description and its skill bonuses and incapabilities.
  - **Traits:** chips. Hovering a chip shows a tooltip with the description and its effects.
  - **Incapable of:** a list (e.g. "Violence, Cooking") in `danger` colour, or "None" in `text_dim`.
  - **Skills table**, one row per skill (11, §4.6):
    - Skill name.
    - Passion glyph: none, minor (1 flame) or major (2 flames). Our own flame icon `ui_icon/passion_minor|major`.
    - Level 0–20 as a number and a bar.
    - Incapable skills show "—" and are greyed.
    - Tooltip: level description (Novice 0–3, Capable 4–7, Skilled 8–11, Expert 12–15, Master 16–20), work affected, speed %, and the passion effect.
  - **Health:** "No conditions", or a list like "Old leg fracture: moves 8% slower".


#### 2.11 Arrival / 2.12 In-colony additions
_Rows: Letter window (NG-1/PLAY-018); Generic inspect panel_

- **Intro letter:** if the scenario has intro text, a letter window shows it once after arrival ("Continue"). A built-in has a short original intro.

- **Pawn inspector:** new **Bio** tab (§2.9 details, read-only, plus XP progress per skill and "This week: +N" learning). The colonist bar tooltip shows age and adulthood title.

- **Letters for threats** name the faction, e.g. "Raid! 6 raiders of the Ashen Compact are coming from the east."


#### 3.3 Difficulty presets (header and DiseaseFreq row)
_Rows: Difficulty profile; Custom difficulty_

| Field | Wanderer | Gentle | Settled | Seasoned | Grim | Merciless | Custom range |
|---|---|---|---|---|---|---|---|

| DiseaseFreq % | 0 | 50 | 75 | 100 | 120 | 150 | 0–300 (hook) |


#### 3.4 Factions
_Rows: Faction types, Faction tech level, Goodwill changes_

### 3.4 Factions

| Type | Key | Tech | Relation range at start | Can be friendly | Settlements / 10k land tiles (normal pop) | Raid kit | Name grammar (original syllables) | Leader titles |
|---|---|---|---|---|---|---|---|---|
| Settlers | `settlers` | Industrial | −20…+60 | yes | 1.2 | Rifles + knives | "the {Adj} Compact" / "{Place} Commonwealth" | Warden, Steward |
| Clans | `clans` | Neolithic | −40…+40 | yes | 1.6 | Spears, clubs, knives | "{Noun} Clan" / "People of the {Place}" | Elder, Hearth-mother, Hearth-father |
| Drifters | `drifters` | Industrial | −60…+10 | yes (rarely) | 1.0 | Mixed | "the {Adj} Drift" / "{Place} Wanderers" | Boss, Caller |
| Freebooters | `freebooters` | Industrial | −100 (fixed) | no | 0.8 | Rifles heavy (+20% points) | "the {Noun} Reavers" / "{Adj} Knives" | Captain, Ringleader |
| Swarm | `swarm` | — | −100 (fixed) | no | 0 (hidden) | Hive insects | "The Swarm" | — |
| Player | `player` | per scenario | — | — | — | — | "{Name}'s Colony" (renameable later) | — |

- **Word lists:** ~60 adjectives, ~60 nouns and ~80 generated place names (two-syllable combinations from our own syllable table) in `domain/faction/names.go`. Example names: "the Ashen Compact", "Thornwick Commonwealth", "Kestrel-Eye Clan", "the Salt Drift", "the Rust Reavers".
- **Relations:**
  - Hostile at goodwill ≤ −50; it becomes non-hostile only at ≥ −10 (**hysteresis**).
  - Friendly at ≥ +50; it stops being friendly at ≤ +30.
- **Daily drift:** goodwill moves 1 point per day toward the type's baseline (Settlers +10, Clans 0, Drifters −30). Freebooters and Swarm are fixed.
- **Events:**
  - A raid repelled from faction F: F −5.
  - Killing a raider: no change.
  - Nothing else in this task (no trade or gifts).
- **Settlement placement:**
  - For each faction, count = round(land × density × popFactor / 10 000). popFactor: Sparse 0.5, Normal 1.0, Crowded 1.6.
  - Placement uses Poisson-disc on land tiles with minimum spacing 6 tiles (sparse 8, crowded 4), excluding Ice Sheet, Sea Ice, Extreme Desert and water.
  - Clans prefer forest/swamp/tundra; Settlers prefer temperate/arid; Freebooters prefer hills.
  - Each faction's first settlement is its **capital**.
  - Settlement names come from the place-name generator.
- **Raid attribution:** `PickRaider(factions, rng)` chooses a hostile human faction, weighted by settlement count. Kit by tech level. If no hostile human faction exists, the scheduler converts the raid to another threat category (pack / manhunters / hive).


#### 3.5 Skills (11)
_Rows: Core skill set (no Artistic)_

### 3.5 Skills (11)

| Skill | Key | Work types affected (existing `work.Type`) | Other effects |
|---|---|---|---|
| Shooting | `shooting` | Hunt (ranged) | Ranged hit chance |
| Melee | `melee` | Hunt (melee weapon) | Melee hit chance |
| Construction | `construction` | Construct (build, deconstruct, repair, roof, smooth) | — |
| Mining | `mining` | Mine | Ore/chunk yield |
| Cooking | `cooking` | Cook (stove/campfire bills, butchering) | Butcher yield |
| Plants | `plants` | Grow (sow, harvest, cut, cut grass) | Harvest yield, failure chance |
| Animals | `animals` | Handle (tame, milk, shear, collect) | Tame chance, product yield |
| Crafting | `crafting` | Craft (workbench, tailoring, stonecutting) | — |
| Medicine | `medicine` | Doctor (tend, rescue) | Heal amount, infection chance |
| Social | `social` | — | Chat mood bonus, tame chance ×(1+0.02·(L−7)) |
| Intellectual | `intellectual` | Research | — |

Haul, Clean and Firefight use only the WorkSpeed stat.


#### 3.6 Traits
_Rows: Trait-aware stat calculation; Human pawn definition_

### 3.6 Traits (18, original; conflict groups in brackets)

| Trait | Group | Effects |
|---|---|---|
| Tireless | Pace | WorkSpeed +20% |
| Unhurried | Pace | WorkSpeed −20%, recreation drain −15% |
| Sunny | Outlook | Mood +8 |
| Brooding | Outlook | Mood −8 |
| Steady | Nerve | Break threshold −6 |
| Brittle | Nerve | Break threshold +6 |
| Fleet-footed | Gait | MoveSpeed +15% |
| Plodding | Gait | MoveSpeed −12% |
| Hearty eater | Appetite | HungerRate +25%, AteMeal thought +2 |
| Light eater | Appetite | HungerRate −20% |
| Roughneck | Standards | SleptOutside / SleptOnGround / AteRaw thoughts ×0.25 |
| Fussy | Standards | Those thoughts ×2, SleptInNiceRoom +3 |
| Deliberate aim | Trigger | ShootAcc +12, AimCooldown +20% |
| Quick draw | Trigger | AimCooldown −25%, ShootAcc −10 |
| Green thumb | — | Plants work +25% |
| Bookish | — | Research +25%, Intellectual learning ×1.5 |
| Gentle | — | Incapable of Violence, Social +3 levels |
| Cold-hardy | — | ColdComfort −6 °C |

**Generation:**
- Trait count: 1 (30%), 2 (50%), 3 (20%).
- Never two traits from the same group, and never a trait that conflicts with the backstory's incapabilities (e.g. Gentle with a soldier backstory).
- Pain tolerance trait "Sturdy" (+15%) is the 19th, ungrouped.


#### 3.7 Backstories
_Rows: Childhood and adulthood background system (Go, not data files)_

### 3.7 Backstories
- **~24 childhood + ~30 adulthood**, original titles and descriptions, written by the content agent.
- Each entry: `{Title, Short, Desc template using {name} {they} {their}, Skills (+2…+6 or −2), Incapable bits, Weight, Tags (tech: neolithic/industrial/spacer; for scenario fitting)}`.
- **Fitting:** Ember Clan uses neolithic-tagged backstories. Others use industrial/spacer, with neolithic allowed at 10%.
- **Example titles:**
  - Childhood: *Lighthouse kid*, *Ration-line runner*, *Orchard foundling*, *Deck-scrubber*, *Hearth-tender's child*, *Quarry urchin*.
  - Adulthood: *Convoy mechanic*, *Field medic*, *Hydroponics keeper*, *Debt collector*, *Itinerant teacher*, *Salvage diver*, *Clan tracker*, *Retired duellist*, *Seed archivist*, *Signal operator*.
- **Lint** (`cmd/contentlint`): titles unique, descriptions non-empty, no blocklisted names, incapable bits valid, skill keys valid.


#### 4.6 Pawn domain
_Rows: Human pawn definition, Procedural pawn generation (no relationships), Trait-aware stat calculation, Core skill set, Backstories_

### 4.6 Pawn domain (`pawn/skill`, `pawn/stat`, `pawn/trait`, `pawn/backstory`, `pawn/look`, `pawn/pawngen`)
**`skill`:**
- `ID` (11, append-only).
- `Skill{Level uint8; XP int32; Passion uint8}` and `Set [NumSkills]Skill`.
- `ForWork(work.Type) (ID, bool)`. Hunt maps to Shooting or Melee, decided by the equipped weapon at the call site.
- `SpeedPermil(L) = 1000 + 60·(L−7)`, so 580‰ at 0 and 1780‰ at 20.
- `Cost(L) = 20000 + 6000·L` XP to reach L+1.
- `Learn(s *Skill, xp int)` with passion multipliers: none 35%, minor 100%, major 150%.
- **Decay:**
  - `DecayDay(s *Skill)` above level 10 removes `500·(L−9)` XP per day.
  - **No-flap band:** XP may go negative, and the level drops only when XP < −Cost(L−1)/2. After a drop, XP = 75% of Cost(L−1).
- **Levels:** `Label(L)` gives Novice/Capable/Skilled/Expert/Master. Max level is 20.

**`stat`:**
- `ID`: WorkSpeed, MoveSpeed, HungerRate, RestRate, MoodOffset, BreakThreshold, ColdComfort, HeatComfort, ShootAcc, MeleeAcc, AimCooldown, PainTolerance, LearnRate.
- `Sheet [N]int32`, in permille for rates and units for offsets.
- `Build(base, mods...)` applies mods in a fixed order: base → body → age → condition → traits → difficulty (PainTolerance, Mood).

**`trait`:** `Def{ID, Key, Label, Desc, Group, Mods []stat.Mod, ThoughtScale map[mood.Kind]int16, AddThought map[mood.Kind]int8, Incapable backstory.Bits, SkillBonus map[skill.ID]int8}`; `Conflicts(a, b)`; `All()`.

**`backstory`:**
- `Slot` (Childhood, Adulthood).
- `Bits` (Violence, Caring, DumbLabor, Intellectual, Animals, Cooking, Plants, Mining, Crafting, Construction).
- `Bits.WorkMask() work.Mask`:

  | Bit | Work types disabled |
  |---|---|
  | Violence | Hunt (and drafted attacks) |
  | Caring | Doctor |
  | DumbLabor | Haul, Clean |
  | Intellectual | Research |
  | Animals | Handle |
  | Cooking | Cook |
  | Plants | Grow |
  | Mining | Mine |
  | Crafting | Craft |
  | Construction | Construct |

- `Bits.Skills() []skill.ID` lists the skills shown as "—".
- `Def{ID, Slot, Title, Short, Desc, Skills map[skill.ID]int8, Incapable Bits, Weight, Tags}`.

**`look`:** `Look{Sex (F/M); Build (Thin/Average/Heavy); Skin, Hair, HairStyle, Beard, Shirt, Pants uint8; Vest bool; Grey bool}` holds palette indices only. `Legacy(id)` reproduces today's `lookFor` exactly: the hashing moves from `ebiten/art.go:536` into the domain, as **male average with today's hair styles**.

**`pawngen`:**
- `Pawn{First, Nick, Last string; Sex; Build; AgeDays int32; Childhood, Adulthood backstory.ID; Traits []trait.ID; Skills skill.Set; Look look.Look; Conditions []Condition; Temperament idle.Temperament}`.
- `Generate(seed uint64, slot, roll int, rules Rules) Pawn`, with `Rules{MinAge, MaxAge; Forced []ForcedTrait; ForcedCond; Tech tags; NoClothes}`.
- **Distributions:**
  - **Age:** 92% are 18–68, triangular with mode 30. 8% are 14–17: childhood backstory only, no adulthood ("—"), skills ×0.6.
  - **Sex:** 50/50, which drives names and art only.
  - **Build:** Thin 25%, Average 50%, Heavy 25%.

    | Build | HungerRate | ColdComfort | MoveSpeed | MeleeAcc |
    |---|---|---|---|---|
    | Thin | 900 | +2 °C | +3% | −5 |
    | Heavy | 1150 | −3 °C | −5% | +5 |

  - **Age effects:**

    | Age | LearnRate | WorkSpeed | MoveSpeed |
    |---|---|---|---|
    | under 18 | +25% | −15% | — |
    | 50–64 | — | −5% | −5% |
    | 65+ | — | −15% | −10% |

    Grey hair: 55% chance at 50+, 90% at 65+.
  - **Skills:** for each skill, `rand(0..3)` + backstory deltas + age points (`min(8, (age−18)/6)`, spread at random over non-incapable skills), clamped to 0..20. Incapable skills are fixed at 0, never get a passion and never learn.
  - **Passions:** 1 + rand(0..3) picks. Each pick is weighted by level+2 over capable skills; 30% of picks become major.
  - **Traits:** §3.6.
  - **Conditions:** 12% chance, §3.9.
  - **Temperament:** chosen from traits (Sunny → Social, Brooding → Homebody, otherwise a hash).
- `Legacy(id, name) Pawn`: all skills 7, no passions, no traits, no incapabilities, age 30, male average, `look.Legacy(id)`, temperament from today's name hash. This keeps old saves and `SpawnNamedPawn` identical.
- **Tests:**
  - `TestGenerateDeterministic`
  - `TestDistributions` (2,000 pawns: sex 50±3%, builds ±5%, ages)
  - `TestIncapableSkillsZeroNoPassion`
  - `TestTraitConflictsNever`
  - `TestBackstoryFitsTech`
  - `TestLegacyLookEqualsRenderer` (golden vs today's `lookFor` output)
  - `TestSkillXPAndDecayNoFlap`
  - `TestStatSheetOrder`


#### 4.8 Items, recipes, research, buildings (Construction)
_Rows: Resource delivery, Construction work_

- **Construction:**
  - `Def.Components int`; `Frame.Comp, CompNeed int`.
  - `Ready() = Wood ≥ Need && Comp ≥ CompNeed`.
  - Haulers deliver the material first, then the components (haul-route extension).


#### 5.3 Difficulty hooks (header and DiseaseFreq row)
_Rows: Difficulty profile; Custom difficulty_

| Field | Site | Change |
|---|---|---|

| DiseaseFreq | (hook only) | `g.diff.DiseaseFreq` stored; noted in the handoff |


#### 5.4 Skills, stats and traits in play (construction row)
_Rows: Construction work (skill)_

  | Function (file) | Skill | Extra |
  |---|---|---|
  | `doConstruct`, `doDeconstruct` (×1.5 speed), `doRepair` (`defense.go`), `doRoof` (`roof.go`), `doSmooth` | Construction | — |


#### 5.4 Read models
_Rows: Character details, Human pawn definition_

- **Read models:**
  - `PawnView` gains `Sex string`, `Build string`, `AgeYears int`, `Look look.Look`, `Incapable work.Mask`, `IncapableWhy map[work.Type]string` (only filled when the inspector asks), `SkillLv [11]uint8`, `Passion [11]uint8`, `Title string` (adulthood or childhood title).
  - `BioView(id) (BioView, bool)` is on demand: full name, backstories (title + description with pronouns filled), traits (label + desc + effects text), skills (level, label, XP%, passion, affected work, speed%), incapabilities with reasons, conditions, stats summary.
  - `CandidatePool` (pre-game, no `Game`):
    - `NewCandidatePool(setupSeed, scenario.Effects)`
    - `Views() []CandidateView` (same fields as BioView, plus `Selected`)
    - `Reroll(i, keepName bool)`, `RandomizeAll()`, `Rename(i, first, nick, last) error`, `Move(i, j)`, `Swap(i, j)`
    - `Team() TeamReport{Best [11]{Level, Who}; Warnings []string}`
    - `Selected() []pawngen.Pawn`


#### 5.5 Factions in play
_Rows: Faction types, Goodwill changes_

### 5.5 Factions in play (`game/factions.go`)
- **State:** `g.factions []faction.Faction` and `g.playerFaction`.
- **Daily:** `faction.DailyDrift` at the day rollover.
- **Raid outcome:** goodwill hooks.
- **Read model:** `Factions() []FactionView{ID, Name, Type, Leader, LeaderTitle, Goodwill, Relation, Settlements int, Tech, Emblem, Accent, History []int16 (last 60 days), Events []string (last 5)}`.
- `Story() StoryView{Storyteller, Title, Preset, Difficulty difficulty.Settings, Adaptation int, SaveMode, NextThreatHint string (dev only)}`.
- Both are added to the `Simulation` interface (`ebiten/sim.go` or wherever `Simulation` lives), with a compile-time assertion in `cmd/game`.


#### 5.6 Water and new items in play (Components)
_Rows: Resource delivery_

- **Components:**
  - Frames with `CompNeed` show "Needs 2 components" in the frame inspector.
  - Hauling delivers components after the material (`haul_route.go` gains a second material slot).
  - Deconstruction refunds 50% of components (round down).


#### 5.8 Saves (v1 to v2 migration)
_Rows: Schema migration_

- **v1 → v2 migration** (load accepts 1 and 2; new saves write 2):
  - `Setup = nil`, so the legacy storyteller and difficulty are used.
  - `Bios` from `pawngen.Legacy(id, name)`, so looks and temperament are unchanged.
  - Factions: none, so raids remain unattributed ("Raiders").
  - Climate re-derived as today.
- **Golden fixture:** `testdata/premotion_v1.sav` must still load. Add `testdata/ng1_v2.sav` as a v2 fixture after Stage S3.


#### 6.1 Budgets
_Rows: Deterministic benchmark runner_

### 6.1 Budgets
| Item | Budget |
|---|---|
| Frame | 16.67 ms at 60 fps |
| Sim at 3× (`speedFast`, 3 ticks/frame) | ≤ 6 ms/frame → **avg tick ≤ 1.5 ms at 500², ≤ 1.0 ms at 250²**, p99 ≤ 4 ms, any 3 consecutive ticks ≤ 7 ms |
| Read models (`World()`+`Snapshot()`) | ≤ 0.5 ms and ≤ 64 KB allocated per frame when nothing changed |
| Draw | ≤ 6 ms CPU, ≤ 6 000 draw calls at every zoom |
| Remainder | ~3.6 ms GC, audio, GPU submit |
| Allocation | ≤ 2 KB per tick in steady state |
| Legacy | `BenchmarkColonyHour` (48×40) within +10% of today's 17.5 ms/hour |


#### 6.2 E0: Measure first
_Rows: Deterministic benchmark runner_

### 6.2 E0: Measure first (lands before any optimisation)
- **Fixture** `internal/application/scenario.GrownColony(seed, w, h, opts)`. It builds through public dev hooks only (`BuildNow`, `SpawnNamedPawn`, `SpawnStack`, zone and designation commands, `Raid`).
  - 20 colonists.
  - 50 tame animals in a fenced pen, plus wildlife at the cap.
  - About 400 buildings in walled rooms with doors.
  - 1,000 stacks.
  - 600 grow cells, 200 mine designations, 50 blueprints.
  - Stains and dirt.
  - 2-day warm-up. Built once with `sync.Once`, saved to bytes, and each benchmark `Load`s it.
- **Benchmarks:**

  | Benchmark | Location | Sizes / variants |
  |---|---|---|
  | `BenchmarkTickGrown` | `game/scale_bench_test.go` | {120, 250, 500}; `ReportMetric` `ms/tick`, `p99-ms/tick`, `max-ms/3ticks` |
  | `BenchmarkTickRaid` | `game/scale_bench_test.go` | 500, with `Raid(12)` |
  | `BenchmarkReadModel` | `game/scale_bench_test.go` | 500 |
  | `BenchmarkDrawArt500` | `ebiten/draw_bench_test.go` | zoom z1, z0.5, z0.25, zmin; reports draw-call count from a counting wrapper |
  | `BenchmarkGenerateSite500`, `BenchmarkSave500`, `BenchmarkLoad500`, `BenchmarkReachable500`, `BenchmarkPath500Long`, `BenchmarkPathUnreachable500`, `BenchmarkRoomsAfterWall500`, `BenchmarkPlanetBuild100`, `BenchmarkGlobeFrame`, `BenchmarkHUDText` | — | — |

- **Runner:** `cmd/benchmark/main.go` runs fixture × size × speed and prints p50/p95/p99 per tick and per phase. It uses an optional `game.PhaseObserver` hook set only from `cmd/*`. Wall-clock time is used for observation only and never feeds gameplay.
- **Dev flags** in `cmd/game`: `-cpuprofile`, `-memprofile`, `-trace`.
- **pprof workflow:** `go test ./internal/application/game -run '^$' -bench 'TickGrown/500' -benchtime 3000x -cpuprofile cpu.out`, then `go tool pprof -top -cum cpu.out`. Use `benchstat` for before/after.
- **Golden state hashes:**
  - `domain/sim/statehash`: `Game.StateHash()` = FNV-64 of `Save` bytes.
  - `TestScaleGolden`: seeds 1/7/42 at 48×40 and 120², 20k ticks. Pinned before E1.


#### 6.6 E4 (Plants)
_Rows: Plants (grown per chunk; other systems still tick)_

### 6.6 E4: Staggered periodic work and job-giver indexes
- **Plants:** grown per chunk every 250 ticks, staggered by chunk index, via `plant.GrowSpan(f, t0, t1)`.
  - This re-pins the golden hashes once (with a reason) because `climate.Outdoor` changes every tick.
  - `frostCrops` folds into the same pass.
  - `TestGrowSpanTelescopes`.
- **Climate-interval work** (`heatRooms` in `heat.go`, `stainDecay` in `stains.go`, `checkRoofSupport` in `roof.go`) is bucketed across the 250-tick window. `checkRoofSupport` uses a dirty list.


#### 9.2 Benchmarks
_Rows: Deterministic benchmark runner_

### 9.2 Benchmarks (pass/fail against §6.1)
| Benchmark | Budget |
|---|---|
| `TickGrown/250` | avg ≤ 1.0 ms |
| `TickGrown/500` | avg ≤ 1.5 ms, p99 ≤ 4 ms, max-3 ≤ 7 ms, ≤ 2 KB/tick |
| `TickRaid/500` | p99 ≤ 5 ms |
| `ReadModel/500` | ≤ 0.5 ms, ≤ 64 KB |
| `DrawArt500/*` | ≤ 6 ms CPU, ≤ 6k draws |
| `Reachable500` | ≤ 1 µs |
| `PathUnreachable500` | ≤ 5 µs |
| `Path500Long` | ≤ 1.5 ms |
| `RoomsAfterWall500` | ≤ 0.2 ms |
| `GenerateSite500` | ≤ 2 s |
| `Save500` / `Load500` | ≤ 300 ms / ≤ 1.5 s |
| `PlanetBuild100` | ≤ 1.5 s |
| `GlobeFrame` | ≤ 2 ms |
| `HUDText` | ≤ 0.6 ms |
| `ColonyHour` (48×40) | ≤ +10% vs 17.5 ms |
| Existing draw, shadow, stain and dirt benches | within +10% |

