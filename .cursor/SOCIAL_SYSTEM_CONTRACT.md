# Social System Contract (ENG-003)

This is the shared contract for the full social and relationship feature set: opinions, romance, weddings, fights, grief,
prisoners, world pawns, caravans, areas, animal bonds, and the host systems they need (body, surgery, drugs).
- Plan: `~/.claude/plans/pasted-content-id-a186-rimworld-snuggly-tiger.md`.
- Everything here is **additive and append-only**. Only the lead changes this file.

> **Balance override (user decision, 2026-10-23):** this feature set uses RimWorld's published wiki numbers for social, mood,
> fight, grief, prisoner and bond values. That is an explicit, feature-scoped exception to the AGENTS.md rule against copying
> proprietary balancing data and to MASTER_SPEC L10/§86. Names, labels, descriptions, log lines and topics stay **original**.
> Every tuning file that uses these values starts with the comment:
> `// Values follow the published RimWorld wiki by explicit user decision (ENG-003 balance override).`

## C0 Global rules
- **Integer fixed point.** Opinion is in centi-points (`int32`, ×100). Factors and probabilities are permille (`int32`). No floats
  in sim or domain code.
- **RNG streams.** Each is `randx.New(seed ^ salt)`, saved, and never `g.rng`.
  - Salts: `socialSalt = 0x50C1A1`, `bodySalt = 0xB0D1E5`, `drugSalt = 0xD2065`, `worldSalt = 0x30E1D`,
    `prisonSalt = 0x9215`, `bondSalt = 0xB04D`.
  - They live in `game/rng_streams.go`.
- **No Go map iteration in sim code.** Iterate sorted slices or `pawnIDs`.
- **Cadences.** Every periodic check is staggered: `(now + id*k) % period`.
- **Hysteresis.** Every threshold behaviour has hysteresis or a cooldown, and a no-flap test.
- **Saves.**
  - `SaveVersion` stays 2. New save fields are pointers or slices; nil means fresh initialisation.
  - Old `testdata` saves must load.

## C1 Shared enum additions (appended at the end of each list, before the count sentinel)

### `mood.Kind` (`internal/domain/pawn/mood/mood.go`)
- **Labels.** Labels are original.
- **Values.** Values in brackets are the def; durations are in days.
- **Stacking.** `StackLimit` and `StackFactor` (‰) are new `Def` fields. Defaults: limit 1, factor 1000 (no stacking).
  - Where marked "stack", use limit 5 and factor 900.
- **`ColonistDied`.** Change it to -3 for 6 days.

| Kind | Label | Value / duration | Notes |
|---|---|---|---|
| `Insulted` | "insulted" | -5, 2 days | stack |
| `KindWords` | "kind words" | +5, 2 days | stack |
| `RebuffedMood` | "rebuffed" | -5, 3 days | |
| `HadToRebuff` | "had to turn someone down" | -3, 3 days | |
| `CheatedOn` | "cheated on" | -20, 25 days | |
| `ProposalRejected` | "proposal turned down" | -18, 25 days | |
| `GotMarried` | "got married" | +40, 30 days | |
| `AttendedWedding` | "attended a wedding" | +20, 10 days | |
| `AttendedParty` | "attended a party" | +8, 10 days | |
| `BrokenUpWith` | "was dumped" | -15, 25 days | |
| `Divorced` | "divorced" | -20, 30 days | |
| `Lovin` | "intimacy" | +8, 3 days | stack |
| `PartnerOpinion` | "feelings for my partner" | 0, state thought | the value is computed |
| `SleptAlone` | "sleeping alone" | -4, state thought | |
| `Hungover` | "hungover" | -5, 1/2 day | |
| `Drunk` | "tipsy" | +6, state thought | |
| `Withdrawal` | "withdrawal" | -15, state thought | |
| `HighOnDrug` | "high" | +10, state thought | |
| `BondedMaster` | "my bonded animal" | +5, state thought | |
| `BondedNotMaster` | "kept from my bonded animal" | -3, state thought | |
| `BondedDied` | "bonded animal died" | -8, 20 days | stack |
| `BondedLost` | "bonded animal lost" | -8, 20 days | stack |
| `BondedGivenAway` | "gave my bonded animal away" | -10, 60 days | stack |
| `BondedReleased` | "released my bonded animal" | -5, 15 days | stack |
| `Imprisoned` | "imprisoned" | -10, state thought | |
| `ColonistLost` | "a colonist went missing" | -3, 6 days | |
| `DisfiguredSelf` | "disfigured" | 0, state thought | placeholder, value 0 |
| `MissingPart` | "missing a body part" | -5, state thought | |
| `Surgery` | "had surgery" | -3, 1 day | |
| `Grief` | "grief" | 0, 0 | relational grief marker; the value comes from social `Grief` tables via `Memory.Value` |

`Grief`, `PartnerOpinion` and similar computed thoughts carry their magnitude in the new `Memory.Value` field (int, 0 means use the def).

New `mood.Memory` fields: `Subject uint64` (the other pawn or animal; 0 means none) and `Value int` (override, 0 means `Def.Value`).

### `trait.ID` (`internal/domain/pawn/trait/trait.go`)
Original names. The groups are new (`GroupBeauty`, `GroupOrientation`, `GroupChem`, `GroupAugment`, `GroupMisanthropy`).

| ID | Key | Group | Notes |
|---|---|---|---|
| `Striking` | `striking` | Beauty | beauty +2 |
| `Fair` | `fair` | Beauty | +1 |
| `Homely` | `homely` | Beauty | -1 |
| `Grotesque` | `grotesque` | Beauty | -2 |
| `Gracious` | `gracious` | | kind |
| `ColdHearted` | `cold_hearted` | | psychopath |
| `Bloodthirsty` | `bloodthirsty` | | bloodlust; `Needs` Violence |
| `Austere` | `austere` | | ascetic |
| `Asexual` | `asexual` | Orientation | |
| `Gay` | `gay` | Orientation | |
| `Bisexual` | `bisexual` | Orientation | |
| `GratingVoice` | `grating_voice` | | |
| `RaspingBreath` | `rasping_breath` | | |
| `DistrustsMen` | `distrusts_men` | Misanthropy | |
| `DistrustsWomen` | `distrusts_women` | Misanthropy | |
| `Idle` | `idle` | Pace | slothful: WorkSpeed ×600 |
| `Abstainer` | `abstainer` | Chem | |
| `Indulgent` | `indulgent` | Chem | |
| `Hedonist` | `hedonist` | Chem | |
| `AugmentFan` | `augment_fan` | Augment | |
| `NaturalPurist` | `natural_purist` | Augment | |

- **New `trait.Def` field:** `Rarity uint8`.
  - 0 means normal weight. When N>0 the trait is picked at 1/N weight in generation.
  - Orientation traits are rare: `Gay` 10, `Bisexual` 10, `Asexual` 20.
- **Generation must not reshuffle existing seeds unnecessarily.** `pawngen` draws traits from `trait.Generated()`. That function returns the
  original 19 when `Legacy` rules are set, and the full list otherwise.
- **Descriptions.** Each new trait gets a one-sentence original description using `{name}` and `{them}`.

### `stat.ID` (`internal/domain/pawn/stat/stat.go`)
All four are permille; each base is 1000 in `Base()`.

| ID | Label |
|---|---|
| `SocialImpact` | "Social impact" |
| `NegotiationAbility` | "Negotiation ability" |
| `TradePriceImprovement` | "Trade price improvement" |
| `ArrestSuccess` | "Arrest success chance" |

### `mentalbreak.Kind`
- `SocialFight`: label "social fight", message "%s is in a fight.", duration 420 ticks (the minimum).
- It is **not** in `Kinds()`, because a mood roll never picks it.

### `job.Kind` (`internal/domain/ai/job/job.go`)
- Kinds: `Surgery`, `Ingest`, `Lovin`, `Gather`, `Brawl`, `Capture`, `Warden`, `DeliverFood`, `SocialRelax`, `Train`, `Arrest`, `Kidnap`,
  `ExitMap`, `TendAnimal`, `Visit`.
- Each gets a `String()` case. `SocialRelax` and `Gather` count as `IsIdle()`.

### `command.Kind` (`internal/domain/sim/command/command.go`)

| Kind | Field use |
|---|---|
| `AddOperation` | `Pawn`=patient, `Index`=body.OpKind, `Build`=part, `Stuff`=item kind |
| `RemoveOperation` | `Pawn`, `Index`=op slot |
| `SetDrugPolicy` | `Pawn`, `Mode`=drug.Preset |
| `AssignBedOwner` | `Target`=bed, `Pawn`=owner (0 clears), `Index`=slot |
| `SetAnimalMaster` | `Target`=animal, `Pawn`=master (0 clears) |
| `SetTraining` | `Target`=animal, `Index`=training.Skill, `Enabled` |
| `GiftAnimal` | `Target`=animal, `Pawn`=visitor |
| `Capture` | `Pawn`=captor, `Target`=downed pawn |
| `Arrest` | `Pawn`=arrester, `Target` |
| `SetPrisonerMode` | `Target`=prisoner, `Mode`=prisoner.Mode |
| `SetBedForPrisoners` | `Target`=bed, `Enabled` |
| `SetBedMedical` (PLAY-020) | `Target`=bed, `Enabled`: that bed only |
| `SetPawnBedRest` (PLAY-020) | `Target`=colonist, `Enabled`: rest when hurt |
| `FormCaravan` | `Name`=comma-separated member ids, `Count`=destination tile |
| `CaravanReturn` | `Target`=caravan id |
| `PaintArea` | `Index`=area, X..X2/Y..Y2, `Enabled` (paint or clear) |
| `RenameArea` | `Index`, `Name` |
| `AssignArea` | `Pawn`, `Index` (-1 = unrestricted) |
| `ReleasePrisoner` | `Target` |

Also add `command.WorkWarden` to `WorkType`.

### `work.Type` (`internal/domain/ai/work/work.go`)
- Append `Warden` (Social skill) before `NumTypes`.
- It is off by default in `DefaultColonist()`, so the lead must not change existing defaults.
- Old saves: the existing legacy-priorities migration pattern in `save.go` (`legacyPriorities`/`migrateHandling`) must be extended so
  saves written with 12 types still load and get Warden at 0.
- Wire up: `skill.ForWork(work.Warden)` returns `skill.Social`, and `priorities.go` `workTypeOf(command.WorkWarden)` returns
  `work.Warden`. In `assignWorkType`, Warden calls `g.assignWarden(id)` (a stub).

### `item.Kind` (`internal/domain/economy/item/item.go`)
- Kinds and names: `Beer`, `AshleafTea`, `SparkTablet`, `Brewgrain`, `AshleafLeaf`, `PegLeg`, `ProstheticLeg`, `ProstheticArm`, `WoodenHand`,
  `Denture`, `HearingTrumpet`.
  - kindNames: "beer", "ashleaf tea", "spark tablet", "brewgrain", "ashleaf", "peg leg", "prosthetic leg", "prosthetic arm",
    "wooden hand", "denture", "hearing trumpet".
- `Brewgrain` is a raw food with nutrition 1200, preference 1. `AshleafLeaf` is not food.
- Add `IsDrug()` (Beer, AshleafTea, SparkTablet) and `IsImplant()` (the six prosthetics).

### `recipe.ID` (`internal/domain/economy/recipe/recipe.go`)

| Recipe | Bench | Input → output | Work | Skill / learn | Other |
|---|---|---|---|---|---|
| `BrewBeer` | `BrewingBench` | Brewgrain×6 → Beer×3 | 600 | Cook / Cooking | |
| `BrewTea` | `DrugTable` | AshleafLeaf×4 → AshleafTea×2 | 500 | Craft / Crafting | |
| `PressTablet` | `DrugTable` | AshleafLeaf×6 + nothing else → SparkTablet×1 | 900 | Craft / Crafting | MinSkill 4 |
| `MakePegLeg` | `Workbench` | Wood×10 → PegLeg | 600 | Craft | |
| `MakeProstheticLeg` | `Workbench` | Steel×20 → ProstheticLeg | 1400 | Craft | MinSkill 5 |
| `MakeProstheticArm` | `Workbench` | Steel×20 → ProstheticArm | 1400 | Craft | MinSkill 5 |
| `MakeWoodenHand` | `Workbench` | Wood×8 → WoodenHand | 500 | Craft | |
| `MakeDenture` | `Workbench` | Wood×4 → Denture | 400 | Craft | |
| `MakeHearingTrumpet` | `Workbench` | Steel×6 → HearingTrumpet | 500 | Craft | |

- **Doctor bill fix (bug found in exploration).** `assignBill` must also serve `work.Doctor` bills (`RefineMedicine`) from
  `assignWorkType(work.Doctor)`, after tending.

### `construction.Kind`
- **`BrewingBench`.** 2x1, wood 8, work 150, bench, rotatable, spot `{0,1}`.
- **`DrugTable`.** 2x1, wood 8, work 180, bench, rotatable, spot `{0,1}`.
- **`MarriageSpot`.** 2x1, free, placed instantly like `SleepingSpot`, passable, rotatable, layer object.
- **`PartySpot`.** 1x1, free, passable, layer object.
- **New `Building` field:** `ForPrisoners bool`.

### `plant.Species`
- **`Brewgrain`.** Crop harvested for `item.Brewgrain`, growth like Wheat.
- **`Ashleaf`.** Crop harvested for `item.AshleafLeaf`, growth like Healroot.
- Neither has an outline, ever.
- Research gates in `research.go`:
  - "Brewing" unlocks `crop:brewgrain` and `build:brewing bench`.
  - "Herbal remedies" unlocks `crop:ashleaf`, `build:drug table`, `recipe:press_tablet`.
  - "Prosthetics" unlocks `recipe:make_prosthetic_leg` and `recipe:make_prosthetic_arm`.

### `incident.ID`
- Append `VisitorGroup`, `WandererJoins`, `Party` (Nature category).
- The weights, cooldowns and minimum days are our own (not wiki).

### `animal.State` and `animal.Mode`
- **New `animal.State` fields:**
  - `Master uint64`, `Bond uint64`
  - `Trained [training.NumSkills]uint8` (steps), `TrainDecay uint64`, `NextBondCheck uint64`
  - `ConfusedUntil uint64`
  - `Mother uint64`
- **New `animal.Mode`:** `Confused`, appended.

### `health.Record`
- No field changes. `Injury` becomes a cache derived from the body for humanlikes (M1).
- `health.Cause` gains `CauseExecution` and `CauseSurgery`.

## C2 New domain packages (stubs now; agents fill them in)
- **Ownership.** Each package below is owned by one agent in Wave A.
- **Tests.** Each must reach ≥70% coverage.
- **Dependencies.** They may depend only on domain packages: no `game` imports and no RNG globals. Randomness comes in as a
  `*randx.Rand` or `func(n int) int` parameter.

| Package | Scope |
|---|---|
| `internal/domain/pawn/social` | `tuning.go`, `curve.go`, `relation.go`, `family.go`, `compat.go`, `memory.go`, `opinion.go`, `interaction.go`, `romance.go`, `gathering.go`, `fight.go`, `grief.go`, `generate.go` |
| `internal/domain/pawn/body` | `anatomy.go`, `wound.go`, `capacity.go`, `scar.go`, `implant.go`, `surgery.go` |
| `internal/domain/pawn/drug` | `drug.go` (defs keyed by `item.Kind`), `severity.go`, `addiction.go`, `policy.go` |
| `internal/domain/pawn/prisoner` | `prisoner.go` (State, Mode, ResistanceFactor, recruit chance) |
| `internal/domain/animal/training` | `training.go` (Skill: Obedience, Guard, Release; step chances; decay; bond chances) |
| `internal/domain/world/area` | `area.go` (bitset areas, ≤10, `Paint`, `Contains`, `Export`/`Import`) |
| `internal/domain/world/caravan` | `caravan.go` (Caravan{ID, Members, Animals, Tile, Path, Progress, Dest, State}, speed, `Step`) |

### Exported shapes that game code relies on (agents must keep them)

**social**
```go
type ID = uint64
type Kind uint8 // relation kinds, append-only: Parent..ExSpouse per plan §M3
type Term struct{ Label string; Value int32 } // centi
type Inputs struct { Observer, Target Person; Relations []Kind; Memories []OpinionMemory; Now uint64 }
type Person struct { ID ID; Male bool; AgeYears int; Traits []trait.ID; Beauty int8; Disfigured, Blind, Deaf bool; ArtificialParts int; Addicted bool; Dead bool }
func Opinion(in Inputs) (centi int32, terms []Term)
type MemKind uint8 // opinion memory kinds (chitchat … botched surgery)
type OpinionMemory struct { Kind MemKind; Subject ID; Expires uint64 }
func AddMemory(ms []OpinionMemory, k MemKind, subject ID, now uint64) []OpinionMemory
func PruneMemories(ms []OpinionMemory, now uint64) []OpinionMemory
func Compatibility(seed uint64, a, b ID, ageA, ageB int) int32 // permille
type Interaction uint8 // Chitchat, DeepTalk, Slight, Insult, KindWords, RomanceAttempt, Breakup, Proposal
type InteractCtx struct { Initiator, Recipient Person; OpinionIR, OpinionRI int32; Compat int32; Pair PairKind; Partnered bool; Attraction int32 }
func Choose(roll func(int) int, c InteractCtx) Interaction
type PairKind uint8 // None, Lover, Fiance, Spouse, ExLover, ExSpouse
type Pair struct { Kind PairKind; Since, LastChange, NextRomanceAt, NextProposalAt uint64 }
func Attraction(a, b Person, kin bool, opinion int32) int32 // permille; 0 = ineligible
type Graph interface { Parents(ID) [2]ID; Spouses(ID) []ID }
func KinOf(a, b ID, g Graph) []Kind
func Label(k Kind, otherMale bool) string
func Bonus(k Kind) int32 // centi
func FightChance(trigger Interaction, mults ...int32) int32 // permille
type GriefDef struct { Mood int; Days int }
func GriefFor(k Kind, friend, rival, lost bool) (GriefDef, bool)
func KillerOpinion(k Kind) int32 // centi, 0 none
type Gathering struct { Kind GatherKind; SpotX, SpotY int; Hosts [2]ID; Phase GatherPhase; PhaseAt, BothArrivedAt uint64 }
func (g *Gathering) Advance(now uint64, hostsPresent, abort bool) (changed bool)
```

**body**
```go
type Part uint8 // Brain, EyeL, EyeR, EarL, EarR, Nose, Jaw, Torso, Heart, LungL, LungR, Stomach, ArmL, ArmR, HandL, HandR, LegL, LegR, FootL, FootR, NumParts
type Capacity uint8 // Consciousness, Moving, Manipulation, Sight, Hearing, Talking, Breathing, NumCaps
type Body struct { Wounds []Wound; Missing uint32; Implants [NumParts]item.Kind }
func (b *Body) Hurt(roll func(int) int, dmg int, kind WoundKind, now uint64) Part
func (b *Body) Heal(roll func(int) int, amount int) (scarred bool)
func (b *Body) Damage() int // total, feeds health.Record.Injury
func (b *Body) Capacity(c Capacity) int32 // permille
func (b *Body) Pain() int32 // permille
func (b *Body) Disfigured() bool
func (b *Body) ArtificialParts() int
type OpKind uint8 // InstallImplant, RemoveImplant, Amputate
type Operation struct { Kind OpKind; Part Part; Item item.Kind }
func SurgeryChance(medicineSkill int, hospitalBed bool, medicine item.Kind, manip, sight int32) int32 // permille
```

**drug**
```go
type State struct { Alcohol int32; Highs []High; Addictions []Addiction; Tolerance [numDrugs]int32; Hangover uint64 }
func (s *State) Ingest(k item.Kind, roll func(int) int, now uint64)
func (s *State) Tick(hours int, now uint64)
func (s *State) FightMultiplier() int32 // permille: alcohol band × withdrawal
func (s *State) Thoughts() []mood.Kind
type Preset uint8 // NoDrugs, SocialDrinking, Unrestricted
func (p Preset) Allows(k item.Kind, joyLow bool, s *State) bool
func (s *State) Craving() (item.Kind, bool) // with hysteresis
```

**prisoner**
```go
type Mode uint8 // None, Recruit, Reduce, Release, Execute
type State struct { Mode Mode; Resistance int32; NextVisit uint64; Captor uint64 }
func ResistanceFactor(opinionCenti int32, mood int) int32 // permille
func Reduction(negotiation int32, factor int32) int32
func RecruitChance(social int) int32
```

## C3 Application layer (`internal/application/game`)
- **New phases** in `phases.go` and `Step`:
  - `PhaseSocial` runs after `PhaseJobs` and calls `g.phaseSocial()` (stub).
  - `PhaseWorld` runs after `PhaseIncidents` and calls `g.phaseWorld()` (stub).
- **New state containers** on `Game`. Each is a pointer, lazily created, and saved as an additive field.
  - `soc *socialState` (`social_state.go`)
  - `people *peopleRegistry` (`people.go`)
  - `bodies map[entity.EntityID]*body.Body` (`body.go`)
  - `drugs map[entity.EntityID]*drug.State` (`drugs.go`)
  - `prisoners map[entity.EntityID]*prisoner.State` (`prisoners.go`)
  - `world *worldState` (`world_state.go`)
  - `areas *area.Set` (`areas.go`)
- **Hook stubs.** Each is an empty method in the file named in brackets, and is called from the existing code site.
  - `dieBy(id, cause, killer)` [`combat.go`]. `die(id, cause)` becomes `dieBy(id, cause, 0)`, and `hurt` passes `by` through.
  - `socialOnDeath(dead, killer)` [`social_grief.go`]: called from `dieBy` before `mourn`.
  - `onHarmed(victim, by)` [`social.go`]: called in `hurt` for pawn victims when `by != 0`.
  - `onRescued(rescuer, patient)` [`social.go`]: called where `doRescue` drops the patient in bed.
  - `onTamed(tamer, animal)` [`bonds.go`]: called on tame success.
  - `onAnimalDied(animal, by)` [`bonds.go`]: called from `killAnimal`.
  - `raiderPerson(id, faction)` [`people.go`]: called in `raidFrom` after spawn.
  - `seedFamilies()` [`people.go`]: called once when the colony is founded (`finishSetup` / `LoadStartKit`).
  - `socialDuty(id, j) bool` [`gathering.go`]: called in `phaseAI` right after `mentalBreakTick`. Returning true means the pawn is busy.
  - `partnerBed(id) (entity.EntityID, int, bool)` [`beds.go`]: consulted first in `assignSleep`.
  - `socialThoughts(id) []mood.Thought` [`social_mood.go`]: appended in `thoughts(id)`.
  - `smoothedMood(id, target) int` [`social_mood.go`]: `moodOf` returns it. The stub returns the target.
  - `assignWarden(id) bool` [`prisoners.go`]
  - `assignSurgery(id) bool` [`surgery.go`]
  - `assignTrain(id) bool` [`bonds.go`]
  - `assignTendAnimal(id) bool` [`bonds.go`]
  - `assignDrugUse(id) bool` [`drugs.go`]: called before idle recreation in `phaseAI`.
  - `inArea(id, x, y) bool` [`areas.go`]: the stub returns true.
  - `doSurgery`, `doIngest`, `doLovin`, `doGather`, `doBrawl`, `doCapture`, `doWarden`, `doDeliverFood`, `doSocialRelax`, `doTrain`,
    `doArrest`, `doKidnap`, `doExitMap`, `doTendAnimal`, `doVisit`: job dispatch cases in `phaseJobs`, each an empty stub in its owner
    file.
- **Read models**, all in `views_social.go`:
  - `SocialView{Relations []RelationRowView; Log []SocialLogLineView}`
  - `RelationRowView{Other uint64; Name, Label, Status string; OpinionTo, OpinionFrom int; TermsTo, TermsFrom []OpinionTermView; RomanceNote string}`
  - `OpinionTermView{Label string; Value int}`
  - `SocialLogLineView{Tick uint64; Text string; Icon string; Weighty bool}`
  - `HealthView{Capacities []CapacityView; Parts []PartView; Pain int; Operations []OperationView; Drugs []DrugLineView}`
  - `PrisonerView{Mode uint8; Resistance int; Warden string; ForPrisonersBed bool}`
  - `WorldMapView{Tile int; Caravans []CaravanView}`
  - `CaravanView{ID uint64; Members []string; Tile, Dest int; Progress int; State string}`
  - `AreaView{Index int; Name string; Cells int}`
  - `HistoryEntryView{Tick uint64; Text string; Kind string; Pawns []uint64}`
  - `PawnView` gains `Interaction string`, `InteractTick uint64`, `MoodTarget int`, `Prisoner bool`, `Visitor bool`, `SocialFight bool`.
  - `AnimalView` gains `Master string`, `Bond string`, `Trained []bool`.
- **Methods on `*Game`.** These are added to the ebiten `Simulation` interface (`renderer.go`) and to the `stubSim` fake:
  - `Social(id uint64) (SocialView, bool)`
  - `Health(id uint64) (HealthView, bool)`
  - `Prisoner(id uint64) (PrisonerView, bool)`
  - `WorldMap() WorldMapView`
  - `Areas() []AreaView`
  - `History() []HistoryEntryView`
- **Save fields** on `saveState`, all pointer or nil-safe:
  - `Social *socialSave`, `People *peopleSave`, `Bodies []entry[body.Body]`, `Drugs []entry[drug.State]`,
    `Prisoners []entry[prisoner.State]`, `World *worldSave`, `Areas *area.Set`, `History []historyEntry`, `RNGStreams map[string][]byte`.
  - Each `*Save` struct starts empty and is filled by its owning milestone.

## C4 UI contract (`internal/adapters/inbound/ebiten`)
- **Tabs.** Enable the inspector tabs Health and Social. They render their views, or "No data" when `ok` is false.
- **Architect.** `archCategories` gains `{"misc","Misc"}`.
- **Icons.** Every icon key is under `ui_icon/` and named in ART_DIRECTION §Social. Until the art exists, renderers draw
  vector fallbacks, and `TestGeneratedArtCoversRenderer` must stay green. A sprite key is only added once the art is generated.

## C5 Ownership map

| Area | Owner |
|---|---|
| shared enums, `save.go`, `phases.go`, `game.go`, `command.go`, `renderer.go` interface, this file, checklists, handoffs | lead / contract agent |
| `domain/pawn/social/**` | A-social |
| `domain/pawn/body/**` | A-body |
| `domain/pawn/drug/**` | A-drug |
| `domain/pawn/prisoner/**`, `domain/animal/training/**`, `domain/world/area/**`, `domain/world/caravan/**` | A-misc-domain |
| `domain/pawn/mood/mood.go` logic (stacking, `AddFor`, `Approach`), `stat.go` | A-mood |
| game-layer files are owned per milestone (§M1…M10 of the plan) | per-milestone agent |
