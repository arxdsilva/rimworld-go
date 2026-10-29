# Beds, sleep and rooms gap analysis (BED-0..BED-3)

Scope: bring beds, rest, room quality and the comfort, beauty and outdoors needs to RimWorld-wiki fidelity (vanilla core only, no DLC). Load only for BED-* tasks. Phase handoffs go in `.cursor/handoffs/BED-N-*.md`. Sister documents: `PRODUCTION_GAP_ANALYSIS.md` (quality rolls feed bed stats) and `DEFENCE_GAP_ANALYSIS.md`.

## Status (2026-10-29)

Wave C done for BED-2 (room stats, roles and thoughts; handoff `handoffs/BED-2-room-stats-roles-thoughts.md`). Wave B done (PRD-1/BED-1/DEF-1). Wave A done earlier: the `game/rest` extraction, the domain packages and the wiki constants (BED-0). BED-3 has not started. Every row in section 3 is `open` unless it says `done` or `partial`. Rows are ticked per phase by the lead, in the same commit that ships the phase.

## 1. Decision record

| Item | Decision |
|---|---|
| Goal | The user asked for the beds, production and defence systems "so we can implement what is missing", at RimWorld fidelity. This document covers beds, sleep, rooms and the needs that depend on them. |
| Scope | All of vanilla core, no DLC. Royal and Ideology furniture, cribs, slab beds and the like are in section 7. |
| Numbers | ENG-003 in `AGENTS.md` was extended on 2026-10-29 to beds, production and defence. Published RimWorld wiki numbers are used as tuning data. Names and prose stay original. |
| Clean room | No wiki text, code, art or sprite data is copied. Each constant is a named constant with a `// wiki: <page>` comment, or `// original tuning` where the wiki gives none. Section 4 lists them. |
| Art and sound | Art ships with each phase that adds a building or item (skill `rimworld-go-art-assets`). Sound generation for this effort is pre-approved by the user; the lead states the count before each run. BED-1..3 add no creature, so the four-sound creature rule does not apply here. |
| New package | `internal/application/game` is at its SOLID ceiling. New sleeping, bed-stat and room-thought code lives in the concise sub-package `internal/application/game/rest` (`.cursor/SOLID_GO.md` section 10): a consumer-owned `deps.go` with roles of at most 8 methods, adapters in game, one Game field, saved state keeping its gob names. Bed rest and medical-bed holding stay in `game/care`. |
| Budget | BED-0 moves more than its adapters, so package game shrinks first. No later phase may leave package game larger (lines, files, `Game` methods) than the previous commit. |
| Hysteresis | Every new need- or threshold-driven behaviour (sitting down for comfort, going outside for the outdoors need, animals choosing beds) starts and stops at different thresholds or has a cooldown, and ships a no-flap test modelled on `sleep_test.go`. |
| Determinism | No new global RNG and no wall clock. Quality-dependent numbers are pure functions of the item's stored quality (from the `quality` domain package, PRD-0). |

## 2. Reading the tables

Legend for the `State` column: `open`, `done`, `out of scope (needs X)`. `Today` evidence gives a path and a function name; line numbers drift while extractions run, so find the function. Paths are under `internal/application/game` unless a full path is given. Every reference was checked against the code on 2026-10-29.

## 3. Gap tables

### 3.1 Bed kinds and ownership

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Sleeping spot, bedroll, double bedroll, bed, double bed, hospital bed, animal sleeping spot, animal bed. | `construction.Kind` adds `Bedroll`, `DoubleBedroll`, `AnimalSleepingSpot`, `AnimalBed`, `EndTable`, `Dresser`; `Kind.AnimalSleeps` is the predicate. | Bedrolls are not yet picked up as minified items. | BED-1 | partial |
| A bed has an owner; couples share a double bed; an owner can be reassigned. | `Building.Claimant`/`Claimant2`; `partnerBed`, `sleepInPartnerBed`, `cmdAssignBedOwner` (`beds.go`). | None. | - | done |
| Beds can be marked for prisoners or kept as medical beds. | `Building.ForPrisoners`; `Building.Medical` with `care.IsMedicalBed` (`care/beds.go`). Medical is a per-building flag, so any bed can be medical, not only the hospital bed. | None. | - | done |
| A bed can be forbidden so nobody claims or uses it. | `construction.Building.Forbidden` is checked in the sleep choice (`rest/beds.go` `SleepIn`, `rest/animals.go` `Bed`) and shown in `BuildingView.Forbidden`. Care, benches and doors do not check it. | Forbidden check in care, benches and doors. | BED-1 | partial |
| Lovin' in a shared double bed. | `lovinCheck`, `lovinBed`, `doLovin` (`beds.go`). | None. | - | done |
| Colonists may be told which bed to use, and a fresh bed is claimed on first sleep. | `sleepInBed` picks partner bed, own bed, else nearest free claimable bed inside the allowed area (`areaAllows`). | None. | - | done |
| A pawn without a bed sleeps on the ground in a spot of its allowed area. | `assignSleepInPlace`, `sleepFloorSpot` (`area_dest.go`). | None. | - | done |

### 3.2 Bed quality: comfort, rest effectiveness, beauty

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Each bed kind has base comfort, rest effectiveness and beauty; stuff and craft quality scale them. | `construction.FurnitureOf(kind, stuff, quality)` (`construction/furniture.go`) gives comfort, rest effectiveness and beauty; `rest.Stats.Of`/`RestEffect` (`rest/stats.go`). Quality is `quality.Default` (Normal) until PRD-2. Leather is rejected in the stuff picker. | Quality input (PRD-2); leather in the stuff picker. | BED-0 (ids), BED-1 (tables) | partial |
| Rest gain while asleep scales with the bed's rest effectiveness and the pawn's rest rate. | `needs.SleepTick` scales the game's base gain by the bed's rest effectiveness (`domain/pawn/needs`); `Stats.RestEffect` supplies it. The base rate is kept and scaled, not the wiki's absolute 0.571 percent per 150 ticks. | None. | BED-1 | done |
| Bed facilities (end table, dresser) beside a bed raise its comfort or rest effectiveness. | `EndTable` and `Dresser` buildings; `FacilityComfort` adds 0.05 comfort each (`construction/furniture.go`); `Stats.endTable`/`dresser` link by position and `DresserReach` (`rest/stats.go`); `BedView.Links`. | None. | BED-1 | done |
| Medical beds heal faster and prevent infection by the bed's medical stats. | Unchanged: `body.HealRate` and `care/rest.go`; the hook is not yet used for per-bed stats. | Per-bed immunity and heal-speed stats. | BED-1 (hook only) | open |
| Sleeping in a cold, hot or noisy place gives a thought; waking up well rested gives none. | `afterSleep` (`rooms.go`) remembers outside, ground, bed, awful room, nice room (`internal/domain/pawn/mood/mood.go` kinds `SleptOutside`..`SleptInNiceRoom`). | Slept-cold, slept-hot, slept-in-barracks, shared-bed and shared-bedroom thoughts (BED-2/3). | BED-2, BED-3 | open |
| Colonists with no bed raise an alert. | No alert. | Alert data and text: "colonists need beds". | BED-3 | open |

### 3.3 Animals and beds

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Tamed animals sleep in animal beds or spots assigned by the player; medical animal beds heal them. | `rest.Animals.Bed` (`rest/animals.go`) picks a free, allowed animal bed or spot and claims it for the pet (`Claimant`); `Animals.SleepRate` scales rest by the bed. Medical animal beds and `AssignOwner` for animal beds are not done. | Medical animal beds; AssignOwner for animal beds. | BED-1 | partial |
| Animals rest on the 60-tick bucket decision, never scanning all creatures. | The bed choice runs inside the existing bucket decision (`animal_needs.go` calling `rest.Animals.Bed`); no new scan. | None. | BED-1 | done |

### 3.4 Room statistics and roles

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Room stats: beauty, space, wealth, cleanliness. Impressiveness is a function of those stats. | `room.Tally` (`domain/world/room/tally.go`) is built at room rebuild: cells, building beauty (furniture x stuff + quality offset), floor beauty and cleanliness, building and floor value; filth (-15 beauty, -5 clean per unit), item value and medical/prisoner bed flags are read live (`rooms.go`, `rooms_index.go`); impressiveness stages in `room/stats.go`. The old ad-hoc score is gone. | None. | BED-0 (domain), BED-2 | done |
| Room roles: bedroom, barracks, hospital, prison cell, prison barracks, dining room, recreation room, workshop, kitchen. | `room.Role` with the added hospital, prison cell, prison barracks, kitchen and recreation room, ruled by medical and prisoner flags and joy furniture (`room.go`, `rooms_index.go`). | None. | BED-2 | done |
| Room caching and invalidation. | `roomTouch`, `roomInputs` (`rooms_index.go`). | Keep as is; new stats must reuse the dirty-room path. | - | done |
| Room panel and overlay show beauty, space, wealth, impressiveness. | `RoomView` carries Space, Beauty, Wealth, CleanStat, Impressiveness and labels; `hud.RoomCardLines` shows role, impressiveness, space, beauty, wealth, cleanliness; `RoomTint` bands follow the stage thresholds (`hud/rooms.go`). | None. | BED-2 | done |

### 3.5 Room thoughts

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Bedroom impressiveness ladder (many stages from awful to wonderful), also for barracks, dining and recreation rooms. | `rest/room_mood.go` with six new mood kinds: bedroom and prison cell -2, 0, +1..+8; barracks and prison barracks -7..+4; hospital -4, -2, none, none, +1..+6; dining and recreation +2..+8 from decent up. Each is read once per occasion (waking, meal, finished pastime), memory lasts a day, zero allocations. The old nice/awful pair is no longer awarded. | None. | BED-2 | done |
| Slept in barracks; shared bed; shared bedroom. | Barracks ladder as above; shared bed -5 + opinion/20 clamped -10..0 (`rest/waking.go`). | A distinct shared-bedroom thought is not separate from the barracks one. | BED-2 | partial |
| Traits that need their own bedroom. | Trait Fussy rekeyed to the new ladder. | Greedy and Jealous need the trait catalogue; Ascetic ladder not coded. | BED-2 | partial |
| Shelves and storage count toward room wealth. | No shelf building. | Items on shelves in the room tally (PRD-4). | PRD-4 | open |
| Colony wealth reuses room building value. | `Wealth()` is a separate sum. | Reuse `room.BuildingValue`. | BED-3 | open |

### 3.6 Needs: comfort, beauty, outdoors

| Vanilla behaviour | Today | Gap | Phase | State |
|---|---|---|---|---|
| Comfort need that rises on comfortable seats and beds and falls elsewhere. | None. "Comfort" in code means the temperature band (`stat.ColdComfort`, `stat.HeatComfort`). | Need record, fall and rise rates, mood stages. | BED-3 | open |
| Comfort furniture: stool, dining chair, armchair, couch. | `Chair` exists as room input only (`room.Contents.Chairs`). | Comfort stat per furniture, sitting jobs. | BED-3 | open |
| Beauty of tiles: buildings, floors, filth, plants and art; the beauty need and thoughts. | None (`checklist 9.3` all `[ ]`). | Per-tile beauty read, perceived beauty, beauty need. | BED-3 | open |
| Outdoors need and the cramped-indoors thoughts. | None. | Need, rates, and the roofed-time source. | BED-3 | open |
| Slept cold or hot thoughts. | The temperature band already exists for waking pawns (`tick.go` thought states `Freezing`, `Cold`, `Hot`, `Sweltering`). | Sleep-time check against the bed's room temperature. | BED-3 | open |
| Need bars for comfort, beauty and outdoors. | `Need bars`: food and rest only. | Bars and tooltips. | BED-3 | open |

### 3.7 Already at fidelity (no work)

Partner double beds and shared-bed lovin', owner assignment and the owner UI, prisoner and medical bed flags, sleep intent with allowed areas, collapse into sleep at zero rest, bed rest and lying-heal bonus, room detection and caching, rescue to the nearest safe bed (`care`). State: `done`.

## 4. Wiki constants

Filled in wave A from the wiki research. Each row becomes a named constant with `// wiki: <page>`. A row that the wiki does not publish records the decision.

| Constant | Value | Source | Used by |
|---|---|---|---|
| Comfort, rest effectiveness, beauty per bed kind (sleeping spot, bedroll, double bedroll, bed, double bed, hospital bed, animal spot, animal bed) | Comfort / rest effectiveness / beauty (Normal quality): sleeping spot 0.4 / 0.8 / n.p.; bedroll 0.68 / 0.95 / 0; double bedroll 0.68 / 0.95 / 0; bed 0.75 / 1.0 / 1; double bed 0.75 / 1.0 / 2; hospital bed 0.8 / 1.0 / 0 (scales with stuff); animal spot 0 / 0.8 / n.p.; animal bed 0.75 / 1.0 / 0. Work ticks: bedroll 600, double bedroll 1100, bed 800, double bed 1500, hospital 2800, animal bed 400. Immunity gain 1.0 / 1.05 / 1.05 / 1.07 / 1.07 / 1.11. Stat ids exist (in `internal/domain/pawn/stat`: Comfort, RestEffectiveness, Beauty); per-bed values not yet coded. Sleeping-spot/animal-spot beauty and mass: not published | https://rimworldwiki.com/wiki/Sleeping_spot, /Bedroll, /Bed, /Double_bed, /Hospital_bed, /Animal_sleeping_spot, /Animal_bed | BED-1 |
| Stuff and craft-quality multipliers on bed stats | Quality (Awful..Legendary) comfort 0.76/0.88/1/1.12/1.24/1.45/1.7; rest effectiveness 0.86/0.92/1/1.08/1.14/1.25/1.6; surgery success 0.9/0.95/1/1.05/1.1/1.15/1.3 (in `internal/domain/economy/quality`). Stuff multipliers on bed stats: not published (only stone double bed gives 90% of nonstone rest effectiveness; hospital bed beauty scales with stuff); decision pending (BED-1) | https://rimworldwiki.com/wiki/Quality, /Rest, /Double_bed | BED-1 |
| End table and dresser: bonus per facility, facility cap, reach | End table +0.05 comfort offset (quality curve 0.04/0.04/0.05/0.06/0.06/0.07/0.09), adjacent to bed head, no diagonal, no distance number; dresser +0.05 comfort (same curve), reach 6 tiles, one effective per bed; both together +0.10. No rest-effectiveness bonus published. End table: work 1000, cost 30, beauty 3; dresser: work 2000, cost 50, beauty 5. Facility cap: not published; decision pending (BED-1) | https://rimworldwiki.com/wiki/End_table, /Dresser | BED-1 |
| Rest gain per tick at 100 percent effectiveness; wake and tired thresholds | Gain 0.57143 % per 150 ticks x rest effectiveness x rest-rate multiplier (0 to 100% in 26250 ticks at effectiveness 1). Awake fall per 150 ticks: 0.237 % (rest >= 28%), 0.166 % (14 to <28%), 0.071 % (1 to <14%), 0.142 % (<= 1%); wiki odd, tired lower than drowsy. Thresholds drowsy 28 / tired 14 / exhausted 1 %, mood -6 / -12 / -18, immunity speed 0.96 / 0.92 / 0.80; collapse at 0 %. 60000 ticks per day. Wake threshold: not published; decision pending (BED-1) | https://rimworldwiki.com/wiki/Rest | BED-1 |
| Room space bands and the space-to-impressiveness curve | Space = 1.4 x cells - 0.9 per blocking furniture tile (chairs and stools do not count). Labels: <12.5 cramped, <29 rather tight, <55 average-sized, <70 somewhat spacious, <130 quite spacious, <349.5 very spacious, else extremely spacious (in `internal/domain/world/room/stats.go`). Beauty divisor: total / (cells if > 40 else 20 + cells/2); beauty labels <-3.5, <0, <2.4, <5, <15, <50, <100 (in `stats.go`). Wealth thresholds 500/700/2000/4000/10000/40000/100000/1000000; cleanliness thresholds -1.1/-0.4/-0.05/0.4 (in `stats.go`) | https://rimworldwiki.com/wiki/Room | BED-2 |
| Room impressiveness formula (beauty, wealth, space, cleanliness weights) | Bases: wealth/1500, beauty/3, space/125, 1 + cleanliness/2.5; each curved (b >= 1: 1 + ln b; b <= -1: -(1 + ln -b); else b); result = 65 x (sum of four)/4 + 35 x min (in `stats.go`, `Impressiveness4`). Wiki space cap (S' = 500 x space; if I > S': 0.25 I + 0.75 S') is flagged unverified by the wiki; domain differs: cap not applied (original tuning) | https://rimworldwiki.com/wiki/Room | BED-2 |
| Room impressiveness stage thresholds and thought values, per role | Stages <20 awful, <30 dull, <40 mediocre, <50 decent, <65 slightly, <85 somewhat, <120 very, <170 extremely, <240 unbelievably, else wondrously (in `stats.go`, `ImpressivenessLabel`). Bedroom mood: awful -2, dull 0, mediocre +1, decent +2, slightly +3, impressive +4, very +5, extremely +6, unbelievably +7, wondrously +8. Barracks: -7, -5, -4, -3, -2, -1, +1, +2, +3, +4. Dining room and rec room: decent +2 up to wondrously +8, below decent none. Hospital: awful -4, dull -2, slightly +1, somewhat +2, very +3, extremely +4, unbelievably +5, wondrously +6. All last 1 day. Conflict: Thoughts page bedroom ladder says "impressive", Room page says "somewhat impressive" and skips a rung; map by stage order, decision pending (BED-2). Role scores: bedroom 100000, barracks 100100 per bed, hospital 100000, dining 12 per table, rec 7 per item; highest wins, ties to earlier (dining vs rec rule in `stats.go`, `RoleOfFull`; prison and kitchen priority is original tuning) | https://rimworldwiki.com/wiki/Room, /Thoughts | BED-2 |
| Slept in barracks, shared bed, shared bedroom: mood values | Barracks stage thoughts as above (-7..+4). Sharing a bed: -5 at opinion 0, +1 per 20 opinion, range -10..0; wiki flags it as possibly other-version, decision pending (BED-2). Sleeping alone while a lover or spouse exists: -4. Slept outside / on the ground: -4 each, 1 day. Disturbed sleep: -1, stacks to 3, 1 day | https://rimworldwiki.com/wiki/Thoughts | BED-2 |
| Own-bedroom traits and their effect | Greedy: no bedroom / awful / dull -8, mediocre -6, decent -4. Jealous: -8 when someone has a better bedroom. Ascetic: awful +5, dull +4, mediocre +3, impressive -5 (trait page) | https://rimworldwiki.com/wiki/Thoughts, /Trait | BED-2 |
| Tile beauty values per building, floor, plant, filth | Floors: bare (rough stone or soil) -1; carpet +2; wood 0; concrete -1; stone tile +1; sterile -1; steel 0; dirty floor -5. Walls: smoothed +2, marble +1, jade +10. Filth (indoor): dirt -15, vomit -40, bugblood -40, gray flesh -8; outdoor about 30% of indoor. Furniture beauty: bed 1, double bed 2, end table 3, dresser 5, dining chair 8, armchair 4, couch 8, stool 0. Conflict: Beauty page says dirt -5, blood -30 vs Filth page dirt -15; use Filth page, decision pending (BED-3). Plant and tree beauty: not published (not retrieved) | https://rimworldwiki.com/wiki/Beauty, /Floors, /Filth | BED-3 |
| Beauty need rates and stage thresholds | Neutral band 35-65 % of need; perceived beauty = average tile beauty within radius 8 (241 tiles, line of sight). Thought ladder: unsightly -5 (opinion < 42), pretty +5 (> 58), beautiful +10 (> 70), gorgeous +15 (> 85); the two ugly rungs (opinion < 15 and < 30) are -15 and -10 per the Beauty page. Need fill/fall rates: not published; decision pending (BED-3) | https://rimworldwiki.com/wiki/Beauty, /Thoughts | BED-3 |
| Comfort need rates, stage thresholds, comfort per furniture | Gain 3.6 % per 150 ticks (60 %/h) only while using furniture below its comfort; fall 4 %/h (conflict: page also says about 5); cap = furniture comfort x 100 %. Thoughts: 0-9 % -3, 10-59 % none, 60-69 % +4, 70-79 % +6, 80-89 % +8, 90-100 % +10. Furniture comfort / beauty: stool 0.5 / 0, dining chair 0.7 / 8, armchair 0.8 / 4, couch 0.85 / 8. Standing or walking gives none | https://rimworldwiki.com/wiki/Comfort, /Stool, /Dining_chair, /Armchair, /Couch | BED-3 |
| Outdoors need rates and thresholds | Thresholds >80 % mood 0; >60 -1; >40 -3; >20 -5; >0.05 -7; <=0.05 -9. Change per 150 ticks: indoors unroofed +1.25, indoors roofed -0.08, indoors mountain -0.1125, outside unroofed +2.0, outside roofed +0.25, outside mountain -0.1; in bed the loss is x0.2 | https://rimworldwiki.com/wiki/Outdoors | BED-3 |
| Slept cold and slept hot: temperature limits and mood | Comfortable sleep range 16 to 26 C (cold below 16, heat above 26; wiki flags unverified); mood -4 each, 1 day. Scaling with how extreme the temperature is: not published; decision pending (BED-3) | https://rimworldwiki.com/wiki/Temperature, /Thoughts | BED-3 |

## 5. Phase plan

**BED-0, extraction and domain groundwork.** No behaviour change. `beds.go` and the sleep giver (`sleepInBed`, `partnerBed`, `sleepInPartnerBed`, `assignSleepInPlace`, `afterSleep`, the lovin' code) move into `internal/application/game/rest` behind consumer-owned roles. Bed rest stays in `care`. The pure domain work lands inert: `stat.Comfort`, `stat.RestEffectiveness`, `stat.Beauty` ids (append only), and the room stats and impressiveness formula with role rules in `internal/domain/world/room`. Golden hashes and old saves stay byte-identical (`bin/benchmark -hash`). Moves: no checklist item; it prepares 7.4, 9.1 and 9.2.

**BED-1, beds and rest effectiveness.** Bed comfort, rest effectiveness and beauty by kind, stuff and quality; `needs.SleepTick` scales by the bed. Bedroll and double bedroll (minifiable), animal sleeping spot and bed with a `SleepAnimalBed` job (animals choose inside the bucket decision), medical animal beds, end table and dresser as facilities with a link rule, and forbidding buildings. Art for each new building, read-only bed stats in the inspector. Moves: vanilla 7.3 Furniture, 9.1 Beds and Rest need.

**BED-2, rooms.** Room beauty, space and wealth feed the vanilla impressiveness through the BED-0 formula in place of the ad-hoc sum. New roles: hospital, prison cell and barracks, recreation room, kitchen. Thought ladders for bedroom, barracks, dining and recreation rooms, slept in barracks, shared bed and bedroom, own-bedroom traits. The room panel and G overlay show the new stats. Moves: vanilla 7.4 Room statistics and Room roles; UI 16.7.

**BED-3, needs.** Tile beauty and the beauty need, the comfort need with comfort furniture (stool, dining chair, armchair, couch) and sitting jobs, the outdoors need, slept-cold and slept-hot thoughts, the "colonists need beds" alert, and need bars with threshold markers. Every need has hysteresis and a no-flap test. Moves: vanilla 9.2, 9.3, 9.4; UI 9.1 Need bars.

## 6. Checklist cross-reference

Each phase ticks these items when its acceptance criteria are met. Titles are quoted exactly. The lead edits the checklists; this file does not.

### 6.1 `RIMWORLD_GO_VANILLA_FEATURE_IMPLEMENTATION_CHECKLIST.md`

| Section | Item | Phase |
|---|---|---|
| 7.3 Core structures | **Furniture** (`[-]`; comfort effects arrive with BED-1 and BED-3, light and joy stay elsewhere) | BED-1 |
| 7.4 Rooms | **Room statistics** (`[-]`) | BED-2 |
| 7.4 Rooms | **Room roles** (`[-]`) | BED-2 |
| 9.1 Rest | **Rest need** (`[-]`; rest effectiveness by bed) | BED-1 |
| 9.1 Rest | **Beds** (`[-]`; bed comfort and rest-effectiveness stats) | BED-1 |
| 9.2 Comfort | **Comfort need** (`[ ]`) | BED-3 |
| 9.3 Beauty | **Tile beauty**, **Perceived beauty**, **Beauty need** (`[ ]`) | BED-3 |
| 9.4 Outdoors | **Outdoors need** (`[ ]`) | BED-3 |

### 6.2 `RIMWORLD_GO_UI_UX_IMPLEMENTATION_CHECKLIST.md`

| Section | Item | Phase |
|---|---|---|
| 9.1 Needs tab | **Need bars** (`[-]`) | BED-3 |
| 16.6 Beauty overlay | **Beauty values**, **Selected pawn perceived beauty** (`[ ]`) | BED-3 |
| 16.7 Room overlay | **Space**, **beauty**, **wealth** (`[ ]`) and **Room boundary** (`[-]`) | BED-2 |
| 23.3 Alerts | **Examples** (`[-]`; add the beds alert) | BED-3 |

## 7. Out of scope

| Item | Missing prerequisite | State |
|---|---|---|
| Royal bed, throne room and royal bedroom requirements | DLC (Royalty) | out of scope (DLC) |
| Crib, baby and child sleeping rules | DLC (Biotech) | out of scope (DLC) |
| Slab bed, sleeping-only mood from ideology precepts | DLC (Ideology) | out of scope (DLC) |
| Ideology and Royalty room and furniture requirements beyond the above | DLC | out of scope (DLC) |
