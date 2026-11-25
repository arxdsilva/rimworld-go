// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package viewmodel owns detached UI and gameplay read-model data shapes.
// Game keeps compatibility aliases while the models have no simulation owners.
package viewmodel

import (
	"rimworld-go/internal/application/game/aqua"
	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/letterdata"
	"rimworld-go/pkg/msgfmt"
)

// MapID is a storage-neutral tile map identifier for detached read views.
type MapID = uint32

// ThoughtView is one line of a pawn's mood.
type ThoughtView struct {
	Label string
	Value int
}

// QueuedOrderView is one waiting order in the read model.
type QueuedOrderView struct {
	Label string
	X, Y  int
}

// PawnView is a read-only snapshot of a single pawn.
type PawnView struct {
	BarOrder int // place on the colonist bar (CTL-011)
	ID       entity.EntityID
	Name     string
	X, Y     int
	Map      MapID // the map X, Y are on (MAP-0)
	Activity schedule.Activity
	Job      string
	Hunger   int
	Rest     int
	// Pack is what the pawn carries on its back (CSE-036), with its weight
	// and limit in grams.
	Pack                      []PackItemView
	PackGrams, PackLimitGrams int
	Status                    string
	CanTalk                   bool // physical speech availability, not social or trade authorization
	Reason                    string
	WorkHaul                  bool
	WorkConstruct             bool
	WorkGrow                  bool
	WorkMine                  bool
	WorkCook                  bool
	WorkHunt                  bool
	WorkCraft                 bool
	WorkDoctor                bool
	WorkResearch              bool
	WorkClean                 bool
	WorkFirefight             bool // PLAY-015 (firefight.go)
	// NG-1 identity (PAWN-G fills): sex, build, age, look, incapable work,
	// skill levels and passions (skill.ID order), adulthood title.
	Sex       string
	Build     string
	AgeYears  int
	Look      look.Look
	Incapable work.Mask
	SkillLv   [skill.NumSkills]uint8
	Passion   [skill.NumSkills]uint8
	Title     string
	Ate       bool
	LastMeal  string // kind name of the last food eaten, "" if none
	// Dirt is the dirt on the pawn's boots, percent of filth.Full (SIM-021).
	Dirt       int
	SleptInBed bool
	HasBed     bool // holds a bed (prison and medical ones too) or a free one is left for it (BED-3)
	Drafted    bool
	// Wave E (DEF-4 fills): FireAtWill (on unless ordered off), HoldPosition, Grenades carried.
	FireAtWill, HoldPosition bool
	Grenades                 int
	Mood                     int
	MoodLabel                string
	Thoughts                 []ThoughtView
	Health                   int // hit points left, percent
	// RaidPlan is a raider\'s raid plan: "assault", "siege" or "sappers".
	RaidPlan string
	// Infection severity (0-100, 0: none); Wounded: an open, untended wound.
	Infection int
	Wounded   bool
	// InfectionLosing: an infection is on course to kill before immunity
	// wins at today's tend; UntendedHours is how long the oldest untended
	// open wound has waited (whole hours, 0: none) (MED-02).
	InfectionLosing bool
	UntendedHours   int
	// MED-00 (filled by MED-01/MED-03): Bleeding, the body is losing
	// blood; CareLevel, the pawn's medical care policy (medcare.Policy).
	// BleedDeathHours is the hours left before bleeding out (MED-01;
	// meaningful only while Bleeding).
	Bleeding        bool
	CareLevel       uint8
	BleedDeathHours int
	// Staggered: a hit's stopping power has it moving at a sixth of its
	// speed (CBT-1, body.go).
	Staggered bool
	// Disease names the pawn's diseases ("flu", "flu, gut worms"; "":
	// none) (MED-04).
	Disease string
	// OverdosePct is the pawn's overdose severity in percent, and
	// OverdoseStage its stage (1 major, 2 extreme; 0: none or below major)
	// (MED-07).
	OverdosePct, OverdoseStage int
	// Exposure: Hypothermia and Heatstroke severity in permille, and the
	// air temperature where the colonist is (tenths of a degree C).
	Hypothermia, Heatstroke int
	Temp                    int
	// Recreation and (BED-3, 0 until filled) Comfort, Beauty and Outdoors
	// are needs, percent; MentalBreak names a break ("" when none).
	Recreation, Comfort, Beauty, Outdoors int
	NoRecreation                          bool // this pawn has no recreation need
	MentalBreak                           string
	MentalTarget                          entity.EntityID // whom a targeted break is aimed at (0: none; SIM-066)
	// Apparel is what the colonist wears over everyday clothes; ComfortMin
	// and ComfortMax its comfortable range (tenths of a degree C).
	// ApparelForced is, per Apparel entry, whether the player chose it
	// ("Wear X": kept until another garment is ordered for the slot);
	// ChangingInto the garment it is walking to put on ("" when none).
	Apparel                []string
	ApparelForced          []bool
	ChangingInto           string
	ComfortMin, ComfortMax int
	// Worn is each Apparel entry's material, quality and condition, and
	// Inspiration the colonist's inspiration ("" when none) with
	// InspirationTicks left of it (PRD-2).
	Worn             []craft.WornView
	Inspiration      string
	InspirationTicks uint64
	// Priorities is the work priority per work.Type (0 off, 1 first .. 4);
	// Schedule the activity per hour.
	Priorities []int
	Schedule   []schedule.Activity
	// QueuedOrders are orders waiting behind the current one, in order.
	QueuedOrders []QueuedOrderView
	// HasOrder: following a player order whose target tile is OrderX, OrderY.
	HasOrder       bool
	OrderX, OrderY int
	// Hostile marks a raider (not a colonist); Fleeing a raider running off
	// or an unarmed colonist running from a threat (PLAY-010).
	Hostile, Fleeing bool
	// Advancing: a raider walking on the colony knowing of no foe yet (the
	// sheet's "Advancing on the colony"; raid/sight.go, EAI-010).
	Advancing bool
	// Temperament is the colonist's free-time leaning
	// (wanderer, social, homebody).
	Temperament string
	Weapon      string // wielded weapon name ("fists" when unarmed)
	// CBT-3: the weapon's quality (stored, quality.Level+1) and stuff ("": the def's own), combat records, the spare weapon ("": none),
	// shield belt charge (percent, 0 without one) and smokepop uses left.
	WeaponQuality                                         uint8
	WeaponStuff, Spare, ThreatResponse                    string // CBT-4: ThreatResponse is "default", "flee", "attack" or "ignore"
	Kills, DamageDealt, DamageTaken, ShieldPct, Smokepops int
	// WeaponStats are its numbers and ApparelArmor each Apparel entry's
	// armor by its stuff and quality, permille (DEF-2: the gear tab).
	WeaponStats  weapon.View
	ApparelArmor []armor.Set
	// Lying: downed or asleep. InBed: lying in its claimed bed.
	// CarriedBy: the rescuer carrying it (0 when not carried);
	// CarryingPawn: the pawn it carries (0 when none; filled from SIM-062).
	Lying        bool
	InBed        bool
	CarriedBy    entity.EntityID
	CarryingPawn entity.EntityID
	// OnMedicalBed: lying in a medical bed; GuestFaction: a guest's
	// faction ("" for a stranger or a non-guest) (SIM-063).
	OnMedicalBed bool
	GuestFaction string
	Tended       bool
	// PLAY-020 bed rest: BedRest while resting in bed to heal;
	// RestWhenHurt is the colonist's setting (on unless the player turned
	// it off).
	BedRest      bool
	RestWhenHurt bool
	// Acting names the hands-on work being done right now ("cut",
	// "mine", "construct", "work at bench", ...), "" otherwise; ActX/ActY
	// is the tile being worked (presentation: facing, swings, particles).
	Acting     string
	ActX, ActY int
	// Motion (presentation lerp, see ART §27): StepTick is the tick the
	// current cell was entered, StepTicks how long that step lasts;
	// NextX/NextY is the next planned cell when HasNext; BlockedBy is the
	// creature in the way of the next step (0 when not blocked).
	StepTick     uint64
	StepTicks    int
	NextX, NextY int
	HasNext      bool
	BlockedBy    entity.EntityID
	// Action (presentation, ART §27.2): Action is the canonical action
	// (actions.go, "" when idle or walking); ActionProgress the job's
	// progress 0..1 (-1 unknown); ActTarget the creature acted on (attack,
	// tend, tame, rescue); AttackCycle 0..1 through the current aim or
	// cooldown, 1 on the strike tick; Carrying the item kind held in the
	// arms ("pawn" while rescuing). HitTick/HitFromX,Y/HitDamage describe
	// the last hit taken (HitTick 0: never).
	Action         string
	ActionProgress float64
	ActTarget      entity.EntityID
	AttackCycle    float64
	Carrying       string
	// CarryingAnimal is the species key of a carried animal carcass
	// ("" otherwise), so the renderer can draw the right body.
	CarryingAnimal string
	// Postponed lists work this colonist put off because it was blocked
	// or out of reach ("build wall: blocked by Bo, retry in 12 s"; SIM-027).
	Postponed          []string
	HitTick            uint64
	HitFromX, HitFromY int
	HitDamage          int
	// Embodied: dead, and the body is now a human corpse item (SIM-017).
	// Soaked: out in the rain.
	Embodied bool
	Soaked   bool
	// Path (UI-013) is the rest of the current walk, cell by cell, ending
	// at GoalX/GoalY (HasGoal); a haul still going to its pickup also has
	// its drop point DropX/DropY (HasDrop).
	Path         [][2]int
	HasGoal      bool
	GoalX, GoalY int
	HasDrop      bool
	DropX, DropY int
	// Log (LOG-001) is what happened to the creature, newest first.
	Log []evlog.Line
	// ENG-003: the interaction shown in a bubble (Interaction, an icon key
	// suffix, since InteractTick), the mood the smoothed mood is heading
	// for, and whether the pawn is a prisoner, a visitor or in a social
	// fight. Zero until their milestones fill them.
	Interaction                    string
	InteractTick                   uint64
	MoodTarget                     int
	Prisoner, Visitor, SocialFight bool
	// Faction is a stranger's raid-group faction name, if any.
	Faction string
	// Ally: a visitor sent as military aid (PLAY-024), fighting raiders.
	Ally bool
	// Disfigured: a scar on the face (body.Disfigured); the renderer draws it.
	Disfigured bool
	// FOOD-0 contract ("" and 0 until FOOD-4 and FOOD-1): food policy name, malnutrition percent.
	FoodPolicy   string
	Malnutrition int
	ToxicBuildup int           // FOOD-6: toxic buildup percent (body/cond_toxic.go)
	Frostbite    int           // TEMP-3: parts with an open frostbite (body.Frostbite wounds)
	ExposureNote string        // AIL-004 contract (heat.go, "" until AIL-1): why heatstroke or hypothermia is not healing
	AX           axview.Pawn   // AX-0 contract (zero until its owners fill it)
	Gear         gearview.Pawn // GEAR-1/GEAR-3 contract (zero until its owners fill it)
}

// ItemView is a ground stack.
type ItemView struct {
	ID      entity.EntityID
	Kind    string
	Qty     int
	X, Y    int
	Carried bool
	// FreshDays is how long the stack keeps at room temperature before it
	// spoils; -1 means it never spoils. Rotten marks a spoiled corpse.
	FreshDays float64
	Rotten    bool
	// RotRate is the current rot speed in permille (0 = frozen).
	RotRate int
	// Dried food came off a drying rack and never spoils.
	Dried bool
	// Of/OfName: whose body a human corpse (or bones) is (SIM-017).
	Of     entity.EntityID
	OfName string
	// Store and Slot (NST): the storage furniture's construction kind this
	// stack lies in (0: loose) and its index among the cell's stacks.
	Store, Slot uint8
	Animal      string // species key of a CorpseAnimal ("deer"), SIM-019
	Meat        string // FOOD-3: the species key a meat stack was butchered from ("cow": "cow meat"; "" untagged)
	// Wreck (DEF-4): the CorpseAnimal is a mechanoid's wreck (Kind
	// "harrow wreck"): it keeps, nothing eats it, it is shredded for salvage.
	Wreck bool
	// Catch: a carpet item's share of tracked dirt wiped, percent (SIM-021).
	Catch int
	// Forbidden: colonists leave this stack alone (UI-016).
	Forbidden bool
	// Unclaimed: found on the map, so haulers leave it unless HaulMarked
	// (PLAY-017, haul_claim.go).
	Unclaimed  bool
	HaulMarked bool
	// PoisonPermille: a cooked meal's food-poisoning chance per meal
	// eaten (WS-M, food_poison.go); 0 for anything else.
	PoisonPermille int
	// PRD-2: a made thing's Quality and material ("" when none), its hit
	// points HP of MaxHP (MaxHP 0: no condition), and Tainted apparel
	// (worn by someone who died in it).
	Quality, Stuff string
	HP, MaxHP      int
	Tainted        bool
	Urgent         bool        // STO/CBT contract (false until STO-2): marked to haul urgently
	Ingredients    []string    // FOOD-0 contract (nil until FOOD-4): a meal's ingredient kind names
	AX             axview.Item // AX-0 contract (zero until its owners fill it)
}

// LetterView is one letter in the right-side stack (PLAY-018).
type LetterView struct {
	ID   uint64 // stable, for DismissLetter
	Tick uint64
	Kind string // "threat", "caution", "info" or "good"
	// Code and Fields are the event's stable identity and typed parameters.
	Code   string
	Fields []letterdata.Field
	Title  string
	Text   string
	Msg    msgfmt.Text     // Title and Text: Text as its catalog key and args (OPT-001; zero: show Text)
	X, Y   int             // where it happened (-1: nowhere)
	Map    MapID           // the map X, Y are on (MAP-0)
	Focus  entity.EntityID // the creature it is about (0: none)
	Intro  bool            // the scenario's opening letter (opens by itself)
	// JoinOffer: the letter asks whether a rescued stranger may join
	// (AcceptLetter accepts, DismissLetter declines; SIM-063).
	JoinOffer bool
	// Demand: a caravan's encounter asks Give or Refuse (EncounterAnswer,
	// Target this letter, Mode 0 give, 1 refuse; MAP-08).
	Demand bool
	Quest  uint32 // the quest it is about (0: none): it opens the Quests window there (QST-131)
	Until  uint64 // the offer lapses then (0: never; a wanderer's, QST-170)
	// Choices: the answers it offers (AnswerLetter, Target this letter,
	// Mode the index; EAI-0: none yet). Shared with the stack: read only.
	Choices []letters.Choice
}

// FrameView is a construction blueprint.
type FrameView struct {
	ID                   entity.EntityID
	Kind                 string
	X, Y                 int
	Wood, Need, WorkLeft int
	Facing               string
	Floor                bool
	Stuff                string
	W, H                 int
	// Moving: a reinstall frame for building MovingFrom (UI-011).
	Moving     bool
	MovingFrom entity.EntityID
	// Frame inspector (UI-015): total work, materials in transit to it,
	// free and reserved stock of its material on the map, who builds and
	// who hauls ("Ada (4 wood)"), and a one-line status.
	WorkTotal         int
	Incoming          int
	InStock, Reserved int
	Builder           string
	Haulers           []string
	Status            string
	// Components delivered, needed and on the way (NG-1 §4.8; 0 for a
	// kind without any).
	Comp, CompNeed, CompIncoming int
	AX                           axview.Frame // AX-0 contract (zero until its owners fill it)
}

// BuildingView is a completed wall or bed.
type BuildingView struct {
	ID          entity.EntityID
	Kind        string
	X, Y        int
	Claimant    entity.EntityID
	Claimant2   entity.EntityID // second sleeper of a double bed
	Facing      string
	Floor       bool
	Deconstruct bool
	Stuff       string
	Quality     string // PRD-2: furniture and beds ("" when it has none)
	// SwapTo is the material a pending in-place swap turns it into, with
	// SwapPct of the work done (PLAY-011); "" when none.
	SwapTo  string
	SwapPct int
	// Ruin: an abandoned piece from map generation (MAPGEN-2).
	Ruin    bool
	Hostile bool // MAP-13: a settlement's turret, firing on us
	// OffGrid: a claimed settlement turret on no grid; Sparking: a turret's
	// fuse burns (MAP-21).
	OffGrid, Sparking bool
	// Medical: a bed kept for the sick and injured (PLAY-020).
	Medical        bool
	W, H           int // footprint in tiles (after facing)
	HasSpot, Bench bool
	SpotX, SpotY   int
	Bills          []craft.BillView
	// Drying rack contents: DryQty items of DryKind, DryPct percent dry.
	DryKind        string
	DryQty, DryPct int
	// Burners: FuelPct left, Burning when lit; REF-1 Refuelable/AutoRefuel.
	Fuel, Burning, Refuelable, AutoRefuel bool
	FuelPct                               int
	FuelLogs, FuelCapLogs                 int // wood burners: logs held and the tank, in logs (0 for other burners)
	// HPPct is the building's health (percent); below 100 it is damaged.
	HPPct int
	// DeconWork is the labor left to deconstruct the building, or its full cost.
	DeconWork int
	// Catch: an installed carpet's share of tracked dirt wiped, percent (SIM-021).
	Catch int
	// Grave (SIM-017): Occupant is who lies there ("" while empty),
	// Epitaph the headstone line, Headstone the headstone style 0..3.
	// A pyre reports Burning while it cremates.
	Occupant  string
	Epitaph   string
	Headstone int
	// MovingTo is where a planned move reinstalls it (nil: not moving).
	MovingTo *[2]int
	// Feeding trough (SIM-024): TroughFill items of TroughCap, and the
	// filter ("plants", "meat", "any").
	TroughFill, TroughCap int
	TroughFilter          string
	// Sleeper casket (MAPGEN-3, casket.go): Sealed while someone (or
	// something) still sleeps inside; OpenReason why the Open order would
	// be refused ("" when a colonist can reach it). A sealed casket marked
	// Deconstruct is ordered opened.
	Sealed     bool
	OpenReason string
	// Electricity (PLAY-021, power_view.go): PowerRole is power.Role's
	// name ("producer", "consumer", "storage", "transmitter", "switch";
	// "" when not electrical) and PowerNet its net (PowerNetView.ID, 0:
	// none). Powered: a consumer running on power now. PowerOn is the
	// actual on/off state, PowerWantOn the player's (a flick is pending
	// while they differ). PowerUse and PowerOut are the W drawn and made
	// now; StoredWh of CapWh a battery's charge. NoPowerReason says why a
	// consumer is unpowered (NoPower*; "" when powered). LinkX, LinkY is
	// the link line's end (HasLink); PowerRelink: another grid is in range.
	PowerRole            string
	PowerNet             int
	Powered              bool
	PowerOn, PowerWantOn bool
	PowerUse, PowerOut   int
	StoredWh, CapWh      int
	NoPowerReason        string
	LinkX, LinkY         int
	LinkFromX, LinkFromY int // the device's cell the link line starts from
	HasLink, PowerRelink bool
	// Glow: the building gives off light now (a lamp, a burning campfire;
	// light.go glowOf), reaching GlowRadius tiles in the simulation's
	// light. Presentation lights only from Glow; it may draw its own reach.
	Glow       bool
	GlowRadius int
	// ThermoTarget is a heater's or cooler's target temperature (tenths of
	// a degree C).
	ThermoTarget int
	ThermoActive bool // a heater or cooler working now (not idling)
	PowerMax     int  // a producer's peak output, W
	// Sentry turret (PLAY-024, turret.go): AimDir is the gun's baked aim
	// frame 0..15 (sentry_turret_gun_00..15, like the turbine blades:
	// frame 0 points south, counting clockwise on screen); ShotAge the
	// ticks since its last shot, -1 before the first (the muzzle flash).
	AimDir  int8
	ShotAge int
	// Beds and bed furniture (BED-1, rest): Comfort and RestEffect are a
	// sleeping place's comfort and rest effectiveness, permille, animal
	// beds included (0 for anything else); Beauty is the piece's beauty
	// in thousandths of a point; Facilities the linked bed facilities as a
	// bitset (FacilityEndTable, FacilityDresser), kept a value so the view
	// stays comparable. Forbidden: nobody uses it.
	Comfort, RestEffect int
	Beauty              int
	Facilities          uint8
	Forbidden           bool
	// Wave D (0 until filled): HoldOpen, a door held open; a turret's or
	// mortar's Barrel life left (permille, 0 none), Shells loaded, the
	// mortar's forced TargetX, TargetY (HasTarget), HoldFire; a bench's
	// Linked facilities and WorkSpeed (permille, 0 unknown).
	HoldOpen, HasTarget, HoldFire               bool
	Barrel, Shells, TargetX, TargetY, WorkSpeed int
	Linked                                      []string
	// STO/CBT contract (0 until STO-2): storage Group id, Stored stacks of Capacity.
	CleanOff         bool // CSE-019: the player turned tidying before bills off here
	Group            uint64
	Stored, Capacity int
	Open             bool            // TEMP-0 contract (false until TEMP-1): a vent letting air through (command.ToggleVent)
	HeatRate         int             // TEMP-0 contract (0 until TEMP-1): heat into its room, tenths of heat/s (signed: cooling < 0)
	ResearchSpeed    int             // RSCH-1: a research bench's factor now, permille (game/research Factor)
	Analyzer         uint64          // RSCH-1: the multi-analyzer a hi-tech research bench links (building id, 0 none)
	AX               axview.Building // AX-0 contract (zero until its owners fill it; in the equality check)
	Tank             *aqua.View      // FISH-*: a fish tank's stock, growth, health and feed (nil for any other building)
}

// Bed facility bits on BuildingView.Facilities (BED-1).
const (
	FacilityEndTable uint8 = 1 << iota
	FacilityDresser
)

// PlantView is a wild plant.
type PlantView struct {
	ID          entity.EntityID
	X, Y        int
	Designated  bool
	Harvestable bool
	Species     string
	Growth      int // percent
	Stage       int // 0 sown .. 3 ripe
	Cut         bool
	Wilted      bool // a crop suffering frost or stalled growth (SIM-016)
	// Zone is the crop of the growing zone the plant stands in ("" when none).
	Zone string
	// Light needs (MAPGEN-3, plant_light.go): NeedsDark withers in light
	// (duskmoss), NeedsLight grows only lit (moss); Withering is losing
	// growth in the light now.
	NeedsDark, NeedsLight, Withering bool
	// Damage (PLAY-016) is cold or heat damage 0..100 (dies at 100);
	// DamageKind "cold", "heat" or "".
	Damage     int
	DamageKind string
	// FOOD-0 contract (0 until FOOD-2): soil fertility, light and lifespan used, percent; blighted.
	Fertility, Light, Lifespan int
	Blighted                   bool
	// Why it is or isn't growing now (PLAY-017) is Game.PlantGrowth: it
	// follows the clock, so it stays off this per-plant view.
}

// PackItemView is one stack in a pawn's pack.
type PackItemView struct {
	ID     uint32
	Name   string
	Qty    int
	Unload bool
	Gear   gearview.Pack // GEAR-1 contract (game/sense)
}

// FoodPolicyView is one food policy: its id, name and allowed kind names.
type FoodPolicyView struct {
	ID      int
	Name    string
	Allowed []string
}

// AutoSlaughterView is one species' herd caps (-1: none).
type AutoSlaughterView struct {
	Species                        string
	MaxMales, MaxFemales, MaxYoung int
}
