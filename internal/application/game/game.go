// Package game orchestrates the authoritative simulation: clock, grid,
// pawns, jobs, movement, items, needs, construction, and harvest.
package game

import (
	"fmt"

	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/world/biome"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/reservation"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/area"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
	"rimworld-go/pkg/randx"
)

var errSpawn = fmt.Errorf("spawn: tile out of bounds or occupied")

// TicksPerCell is how long a pawn occupies one tile (~4 cells/s at 1x).
const TicksPerCell = clock.TicksPerSecond / 4

// Position is a pawn's location on the grid.
type Position struct {
	X, Y int
}

// Config configures a new Game.
type Config struct {
	// Setup is the new-game wizard's choices (NG-1); nil = legacy start.
	Setup         *Setup
	Seed          uint64
	Width, Height int
	// Biome is the map's setting (SIM-023); zero is temperate forest.
	Biome biome.ID
}

// PawnView is a read-only snapshot of a single pawn.
type PawnView struct {
	ID            entity.EntityID
	Name          string
	X, Y          int
	Activity      schedule.Activity
	Job           string
	Hunger        int
	Rest          int
	Status        string
	Reason        string
	WorkHaul      bool
	WorkConstruct bool
	WorkGrow      bool
	WorkMine      bool
	WorkCook      bool
	WorkHunt      bool
	WorkCraft     bool
	WorkDoctor    bool
	WorkResearch  bool
	WorkClean     bool
	WorkFirefight bool // PLAY-015 (firefight.go)
	// NG-1 identity (PAWN-G fills): sex, build, age, look, incapable work,
	// skill levels and passions (skill.ID order), adulthood title.
	Sex       string
	Build     string
	AgeYears  int
	Look      look.Look
	Incapable work.Mask
	SkillLv   [11]uint8
	Passion   [11]uint8
	Title     string
	Ate       bool
	LastMeal  string // kind name of the last food eaten, "" if none
	// Dirt is the dirt on the pawn's boots, percent of filth.Full (SIM-021).
	Dirt       int
	SleptInBed bool
	Drafted    bool
	Mood       int
	MoodLabel  string
	Thoughts   []ThoughtView
	Health     int // hit points left, percent
	// RaidPlan is a raider\'s raid plan: "assault", "siege" or "sappers".
	RaidPlan string
	// Infection severity (0-100, 0: none); Wounded: an open, untended wound.
	Infection int
	Wounded   bool
	// Exposure: Hypothermia and Heatstroke severity in permille, and the
	// air temperature where the colonist is (tenths of a degree C).
	Hypothermia, Heatstroke int
	Temp                    int
	// Recreation is the recreation need, percent; MentalBreak names a
	// break in progress ("" when none).
	Recreation  int
	MentalBreak string
	// Apparel is what the colonist wears over everyday clothes; ComfortMin
	// and ComfortMax its comfortable range (tenths of a degree C).
	// ApparelForced is, per Apparel entry, whether the player chose it
	// ("Wear X": kept until another garment is ordered for the slot);
	// ChangingInto the garment it is walking to put on ("" when none).
	Apparel                []string
	ApparelForced          []bool
	ChangingInto           string
	ComfortMin, ComfortMax int
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
	// Temperament is the colonist's free-time leaning
	// (wanderer, social, homebody).
	Temperament string
	Weapon      string // wielded weapon name ("fists" when unarmed)
	// Lying: downed or asleep. InBed: lying in its claimed bed.
	// CarriedBy: the rescuer carrying it (0 when not carried).
	Lying     bool
	InBed     bool
	CarriedBy entity.EntityID
	Tended    bool
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
	Log []LogLineView
	// ENG-003: the interaction shown in a bubble (Interaction, an icon key
	// suffix, since InteractTick), the mood the smoothed mood is heading
	// for, and whether the pawn is a prisoner, a visitor or in a social
	// fight. Zero until their milestones fill them.
	Interaction  string
	InteractTick uint64
	MoodTarget   int
	Prisoner     bool
	Visitor      bool
	SocialFight  bool
	// Disfigured: a scar on the face (body.Disfigured); the renderer draws it.
	Disfigured bool
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
	Animal string // species key of a CorpseAnimal ("deer"), SIM-019
	// Catch: a carpet item's share of tracked dirt wiped, percent (SIM-021).
	Catch int
	// Forbidden: colonists leave this stack alone (UI-016).
	Forbidden bool
	// Unclaimed: found on the map, so haulers leave it unless HaulMarked
	// (PLAY-017, haul_claim.go).
	Unclaimed  bool
	HaulMarked bool
}

// LetterView is one letter in the right-side stack (PLAY-018).
type LetterView struct {
	ID    uint64 // stable, for DismissLetter
	Tick  uint64
	Kind  string // "threat", "caution", "info" or "good"
	Title string
	Text  string
	X, Y  int             // where it happened (-1: nowhere)
	Focus entity.EntityID // the creature it is about (0: none)
	Intro bool            // the scenario's opening letter (opens by itself)
}

// StoredResourceView is one row of the resource list (PLAY-018).
type StoredResourceView struct {
	Kind     string // item kind key ("wood")
	Label    string // "Wood"
	Category string // "Food", "Materials", "Medicine", "Apparel", "Weapons", "Manufactured", "Corpses"
	Qty      int
}

// FrameView is a construction blueprint.
type FrameView struct {
	ID       entity.EntityID
	Kind     string
	X, Y     int
	Wood     int
	Need     int
	WorkLeft int
	Facing   string
	Floor    bool
	Stuff    string
	W, H     int
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
	// SwapTo is the material a pending in-place swap turns it into, with
	// SwapPct of the work done (PLAY-011); "" when none.
	SwapTo  string
	SwapPct int
	// Ruin: an abandoned piece from map generation (MAPGEN-2).
	Ruin bool
	// Medical: a bed kept for the sick and injured (PLAY-020).
	Medical bool
	W, H    int // footprint in tiles (after facing)
	HasSpot bool
	SpotX   int
	SpotY   int
	Bench   bool
	Bills   []BillView
	// Drying rack contents: DryQty items of DryKind, DryPct percent dry.
	DryKind string
	DryQty  int
	DryPct  int
	// Campfire: FuelPct of capacity left, Burning when lit (not banked).
	Fuel    bool
	FuelPct int
	Burning bool
	// HPPct is the building's health (percent); below 100 it is damaged.
	HPPct int
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
	// the transmitter cell a device's link line goes to (HasLink).
	PowerRole            string
	PowerNet             int
	Powered              bool
	PowerOn, PowerWantOn bool
	PowerUse, PowerOut   int
	StoredWh, CapWh      int
	NoPowerReason        string
	LinkX, LinkY         int
	LinkFromX, LinkFromY int // the device's cell the link line starts from
	HasLink              bool
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
}

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
	// Zone is the crop of the growing zone the plant stands in ("" when
	// none).
	Zone string
	// Light needs (MAPGEN-3, plant_light.go): NeedsDark withers in light
	// (duskmoss), NeedsLight grows only lit (moss); Withering is losing
	// growth in the light now.
	NeedsDark, NeedsLight, Withering bool
	// Damage (PLAY-016) is cold or heat damage 0..100 (dies at 100);
	// DamageKind "cold", "heat" or "".
	Damage     int
	DamageKind string
	// GrowState (PLAY-017) says why the plant is or isn't growing now:
	// "growing", "resting" (night), "dark" (needs sun or a strong lamp),
	// "cold", "hot" or "ripe"; RipeIn is the estimated ticks until ripe at
	// the current conditions (-1 unknown or not growing).
	GrowState string
	RipeIn    int
}

// WorldView is the non-pawn read model for HUD and the map.
type WorldView struct {
	// Biome is the map's setting: display name and key (SIM-023).
	Biome, BiomeKey string
	// TreesFelled is how many trees the visiting timbergnaws have eaten
	// (SIM-025; 0 when none are about).
	TreesFelled int
	Items       []ItemView
	Frames      []FrameView
	Buildings   []BuildingView
	Plants      []PlantView
	Food, Wood  int
	Stone       int // all stone blocks
	// Metals mined from ore veins (SIM-029).
	Steel, Silver, Gold int
	// Terrain is the ground type per tile, row-major, TerrainW wide;
	// codes index TerrainNames().
	Terrain  []uint8
	TerrainW int
	// Mines are tiles designated for mining.
	Mines [][2]int
	// Smooths are tiles designated for smoothing (SIM-029).
	Smooths [][2]int
	// StoneType and Ore are the geology per tile (row-major, TerrainW
	// wide): codes index StoneNames() / OreNames(), 0 = none. Shared
	// read-only slices, rebuilt only when StructVer changes.
	StoneType []uint8
	Ore       []uint8
	// StructVer changes whenever terrain or a building changes, so the
	// renderer can cache link (autotile) data.
	StructVer uint64
	// FoodByKind is ground food per kind name.
	FoodByKind map[string]int
	// RoomGrid is the room id per tile (row-major, TerrainW wide); -1 is
	// outdoors, -2 a wall. Rooms indexes by id.
	RoomGrid []int
	Rooms    []RoomView
	// Animals are the living wild animals; Traces are recent attacks
	// (tracers and swings) for presentation.
	Animals []AnimalView
	Traces  []TraceView
	// Climate: OutdoorTemp and IndoorTemp (enclosed rooms) in tenths of a
	// degree C, and the season name.
	OutdoorTemp, IndoorTemp int
	Season                  string
	// Weather (SIM-016): Weather is the kind ("clear", "rain", ...),
	// WeatherIntensity 0..1, WindX/WindY the wind in tiles per second,
	// SnowDepth and Wetness 0..1 what lies on open ground.
	Weather            string
	WeatherIntensity   float64
	WindX, WindY       float64
	SnowDepth, Wetness float64
	// Messages are recent event notices, oldest first.
	Messages []MessageView
	// Letters (PLAY-018) are the letter stack, newest last: kept and saved
	// until the player dismisses one (command.DismissLetter), at most 40.
	Letters []LetterView
	// Stored (PLAY-018) is what the colony holds in its stockpiles, one row
	// per item kind, for the resource list (the Food/Wood/... totals above
	// still count every loose stack).
	Stored []StoredResourceView
	// Schedule is the colony default schedule (new colonists get a copy).
	Schedule []schedule.Activity
	// Research projects (Research tab) and still-locked unlock keys
	// ("build:<kind>", "crop:<plant>", "recipe:<name>") mapped to the
	// project that opens them.
	Research []ResearchView
	Locked   map[string]string
	// Zones (ZONE-001) are every stockpile, grow zone and pen, tile by
	// tile, with their settings.
	Zones []ZoneView
	// Roofs (SIM-018): RoofCells is the roof.Cell flags per tile
	// (row-major, TerrainW wide); RoofCollapses are collapses of the last
	// hour.
	RoofCells     []uint8
	RoofCollapses []RoofCollapseView
	Bursts        []BurstView // explosive animal deaths of the last hour (SIM-019, wildlife.go)
	// Hour is the time of day 0..24 and DayOfYear 0..DaysPerYear-1, for
	// the sun and moon (presentation only).
	Hour      float64
	DayOfYear int
	// Grass is 1 per tile with grass (row-major, TerrainW wide; empty: no
	// grass data); GrassCut are grass tiles marked for cutting (grass.go).
	Grass    []uint8
	GrassCut [][2]int
	// GrassGrowth is each grass tile's growth, 0..100 (PLAY-010): shoots
	// after grazing or cutting fill out over a few days. Empty means every
	// grass tile is full. GrassVer changes whenever Grass or GrassGrowth
	// does (presentation caches key on it; never saved).
	GrassGrowth []uint8
	GrassVer    uint64
	// Stains are blood and ichor on the ground (SIM-020), sorted y then x.
	Stains []StainView
	// Dirt is tracked-in dirt on the ground (SIM-021), sorted y then x.
	Dirt []DirtView
	// Home is 1 per tile inside the Home area (row-major, TerrainW wide);
	// HomeVer changes whenever it does, so the renderer can cache.
	Home    []uint8
	HomeVer uint32
	// Plan is 1 per tile with a plan mark (PLAY-012: visual placeholders,
	// no gameplay effect; cleared when a building is finished on the tile);
	// PlanVer changes whenever it does.
	Plan    []uint8
	PlanVer uint32
	// Fog (MAPGEN-2) is 1 per unexplored tile (row-major, TerrainW wide;
	// empty: no fog); FogVer changes whenever it does. Landing is the
	// colony's landing spot (the map centre for legacy colonies).
	Fog     []uint8
	FogVer  uint64
	Landing [2]int
	// PLAY-015: per-tile snow depth (0..255; empty: none), the snow
	// clearing area (1 per tile), burning cells, the pens and whether the
	// Home area expands automatically. Each *Ver changes with its layer.
	Snow         []uint8
	SnowVer      uint64
	SnowClear    []uint8
	SnowClearVer uint64
	Fires        []FireView
	FireVer      uint64
	Pens         []PenView
	PenVer       uint64
	HomeAuto     bool

	// Strikes are the lightning strikes of the last hour (lightning.go):
	// the renderer flashes and thunders from them.
	Strikes []StrikeView
	// NG-1 setup (setup_view.go): arrival pods for the fall streak and dust
	// (Tick is the landing tick), the permanent conditions' keys and the
	// heat wave or cold snap in progress ("" when none).
	Arrivals   []ArrivalView
	Conditions []string
	Extreme    string
	// Electricity (PLAY-021, power_view.go): every power net by id, the
	// solar flare in progress, and PowerVer, which changes whenever the
	// nets or any device's power state do (presentation caches key on it).
	PowerNets  []PowerNetView
	SolarFlare bool
	PowerVer   uint64
}

// Game holds all authoritative simulation state.
type Game struct {
	clock    *clock.Clock
	grid     *grid.Grid
	entities *entity.Allocator
	rng      *randx.Rand
	sched    schedule.Grid
	wl       wildlifeState // SIM-019 wildlife events (wildlife.go)
	// thoughtBuf is thoughtLevel's reused thought list (rooms.go; never
	// saved, never handed out).
	thoughtBuf []mood.Thought

	messages []MessageView
	letters  letterState // the letter stack (letters.go, PLAY-018)

	pawnIDs   []entity.EntityID
	positions *component.Store[Position]
	names     *component.Store[string]
	jobs      *component.Store[job.Record]
	need      *component.Store[needs.Pair]
	vital     *component.Store[health.Record]
	labor     *component.Store[work.Mask]

	itemIDs  []entity.EntityID
	stacks   *component.Store[item.Stack]
	frameIDs []entity.EntityID
	frames   *component.Store[construction.Frame]
	bldgIDs  []entity.EntityID
	bldgs    *component.Store[construction.Building]
	plantIDs []entity.EntityID
	plants   *component.Store[plant.Wild]
	terr     terrain.Map
	mines    map[[2]int]bool
	// Geology (SIM-029): stone type and ore per tile; smoothing marks;
	// structVer bumps on any terrain or building change (view cache key).
	geo       geology.Map
	smooths   map[[2]int]bool
	structVer uint64
	viewStone []uint8
	viewOre   []uint8
	viewVer   uint64

	zones      *zone.Map // stockpiles, grow zones and pens (zones.go, ZONE-001)
	lastMeal   map[entity.EntityID]item.Kind
	bills      map[entity.EntityID][]recipe.Bill
	racks      map[entity.EntityID]dryBatch // what each drying rack holds
	fuel       map[entity.EntityID]int      // campfire fuel, ticks of burning
	weather    weather.State                // outdoor weather (weather.go, SIM-016)
	weatherFix bool                         // dev: HoldWeather keeps the forced weather
	graves     map[entity.EntityID]graveRec // who lies in each filled grave (burial.go, SIM-017)
	exposure   map[entity.EntityID]exposureState
	noComfort  map[entity.EntityID]uint64 // no warm/cool spot found until this tick
	biome      biome.ID                   // the map's setting (SIM-023)
	climate    temperature.Climate        // its outdoor temperature curve
	incidents  incident.State             // the storyteller's nature events (SIM-025)
	troughs    map[entity.EntityID]troughState
	noFood     map[entity.EntityID]uint64 // food out of reach: no eating attempt until this tick
	postponed  postState                  // work put off, orders followed (postpone.go, SIM-027)
	restedFree map[entity.EntityID]bool   // woke fully rested in the current Sleep block: free time
	calm       map[entity.EntityID]uint64 // fled a threat: holding back from work until this tick
	banked     map[entity.EntityID]bool   // campfires banked by their room thermostat (heat.go)
	orderQueue map[entity.EntityID][]queuedOrder
	apparel    map[entity.EntityID]worn
	clothes    apparelIndex // free apparel stacks (apparel.go), never saved
	prio       map[entity.EntityID]work.Priorities
	raiderIDs  []entity.EntityID
	raiders    map[entity.EntityID]raiderState
	raidGroups []raidGroup
	nextThreat uint64
	threatsOn  bool
	scheds     map[entity.EntityID]schedule.Grid
	joy        map[entity.EntityID]int // recreation need, 0..recreation.Full
	breaks     map[entity.EntityID]breakState
	roomTemps  []temperature.Temp // per room id, kept across rebuilds
	memories   map[entity.EntityID]mood.Memories
	roomGrid   []int
	rooms      []roomStats
	roomDirty  bool
	fence      fenceIndex  // fence and gate cells, derived (fences.go, SIM-028)
	regs       regionState // walkability regions and labels, derived (regions.go, NG-1 E2)
	reserve    *reservation.Table
	pending    []command.Command

	ateAt   map[entity.EntityID]bool
	logs    map[entity.EntityID][]logEntry // per-creature event logs (eventlog.go)
	draft   map[entity.EntityID]bool
	sleptAt map[entity.EntityID]entity.EntityID // pawn -> bed, 0 = ground

	animalIDs []entity.EntityID
	animals   *component.Store[animal.State]
	equip     map[entity.EntityID]item.Kind // wielded weapon; absent = fists
	traces    []projectile.Trace
	// lastStrike and lastHit are transient presentation stamps (see
	// action_view.go); the simulation never reads them.
	lastStrike map[entity.EntityID]strikeRec
	lastHit    map[entity.EntityID]hitRec
	wild       bool // wildlife migrates in (generated colonies)

	carrying    map[entity.EntityID]entity.EntityID // rescuer -> patient
	tendedUntil map[entity.EntityID]uint64
	// wounds: open wounds and infections of colonists.
	wounds map[entity.EntityID]wound
	// research progress and the current project (researchSet: one is picked)
	research research.Progress
	// rockDamage: rock tiles sappers are hacking through.
	rockDamage map[[2]int]int
	// roofs: the roof state per tile (roof.go, SIM-018), row-major; empty
	// until the first room rebuild. roofCollapses are recent collapses.
	roofs         []roof.Cell
	roofCollapses []roofCollapse
	// grass is the ground cover layer (grass.go, UI-011).
	grass grassState
	// stains are blood and ichor on the ground (stains.go, SIM-020).
	stains    stainState
	filth     filthState // tracked-in dirt (SIM-021, filth.go)
	homeArea  homeState  // the Home area (SIM-021, home.go)
	planMarks planState  // PLAY-012 plan marks (plan.go)
	roofx     roofIndex  // roof.go: derived plan index and roofed counts
	// residual: air temperature of cells of a freshly breached room.
	residual    map[int]temperature.Temp
	researchCur research.Project
	researchSet bool

	// motion: per-creature step timing (motion_store.go); moverBuf is
	// scratch for the ascending-ID mover list.
	motion    *component.Store[motionRec]
	moverBuf  []mover
	moverOld  []entity.EntityID // pruneMotion scratch
	raiderBuf []entity.EntityID // phaseThreats scratch
	// pathSearch is the game's own A* state (paths.go: searcher), never
	// saved; a pointer so a copied Game shares one search generation.
	pathSearch *pathing.Searcher
	animalQ    [3]animalQCache // fences.go: animalPathQuery, never saved
	occ        occupancy       // movement blocking index (occupancy.go), transient
	// enterHook, when set (tests only), replaces enter's verdicts.
	enterHook func(id entity.EntityID, from, to Position) (stepVerdict, Position)
	// haul: in-flight material deliveries (haul_route.go, HAUL-001).
	haul haulState
	// cabinets: tool cabinet bonus per bench, per structVer (items_ng1.go), never saved.
	cabinets cabinetState
	// vents: steam vent cells per structVer (vents.go), never saved.
	vents ventCache
	// light: the simulation's artificial light grid (light.go), never saved.
	light lightGrid
	// phaseObs: optional per-phase timing observer (phases.go); set only
	// from cmd/* and benchmarks, never read by gameplay.
	phaseObs PhaseObserver
	// idx: derived per-cell lookups (indexes.go), never saved.
	idx indexes
	// cellSets and sapperSet: building/terrain cell sets cached per
	// structVer (defense.go, raid_plans.go), never saved.
	cellSets  map[uint64]*cellCache
	sapperSet cellCache
	// views, viewGen, viewTerr: read-model caches (indexes.go).
	views    viewCache
	viewGen  uint64
	viewTerr layerCache
	// viewGrass, viewHome, viewRoof: copy-on-write read-model layers
	// (indexes.go shareLayer).
	viewGrass, viewHome, viewRoof layerCache
	viewPlan                      layerCache // PLAY-012 plan marks
	// minesVer bumps on every mine designation change (mineList cache);
	// seedCache is weatherSeed's (indexes.go). Never saved.
	minesVer  uint64
	mineCache mineCache
	seedCache seedCache
	// sk: bios, skills, stats and learning (skills.go, NG-1 PAWN-G).
	sk skillState
	// ng: the new-game setup's live state (setup.go, NG-1 SETUP); zero
	// for a legacy colony.
	ng ng1State
	// fog: unexplored tiles (fog.go, MAPGEN-2); zero: no fog.
	fog fogState
	// PLAY-015: fires, per-tile snow and the pens' settings.
	fire fireState
	snow snowState
	pens []penSave
	pen  penState // pens.go: fills, alerts, unpenned clocks
	// vaults: the sealed vaults' lifecycle (vault.go, MAPGEN-3); saved.
	vaults []vaultSave
	// br: bridges' open-sea bitset (bridge.go, MAPGEN-3); zero: no sea known.
	br bridgeState
	// mapFields caches the developer map-field layers (mapfields.go); never saved.
	mapFields *mapFieldCache
	// Social and host systems (ENG-003): each container is nil until its
	// milestone first needs it, and saved as an additive field.
	soc       *socialState                        // social_state.go
	people    *peopleRegistry                     // people.go
	bodies    map[entity.EntityID]*body.Body      // body.go
	surg      *surgeryState                       // surgery.go
	rest      bedRestState                        // bedrest.go (PLAY-020)
	drugs     map[entity.EntityID]*drug.State     // drugs.go
	prisoners map[entity.EntityID]*prisoner.State // prisoners.go
	worldSt   *worldState                         // world_state.go
	areas     *area.Set                           // areas.go
	history   []historyEntry                      // history.go
	streams   rngStreams                          // rng_streams.go: never g.rng
	// pw is the colony's electricity (power.go, PLAY-021); nil until the
	// colony has a power device.
	pw *powerState
	// pwc: power nets and device lists per structVer (power.go), never saved.
	pwc powerCache
}

// New builds a Game from cfg. Width/Height must be positive.
func New(cfg Config) *Game {
	return &Game{
		clock:       clock.NewAt(clock.StartingMorning()),
		grid:        grid.New(cfg.Width, cfg.Height),
		entities:    entity.NewAllocator(),
		rng:         randx.New(cfg.Seed),
		sched:       schedule.DefaultColony(),
		positions:   component.NewStore[Position](),
		names:       component.NewStore[string](),
		jobs:        component.NewStore[job.Record](),
		need:        component.NewStore[needs.Pair](),
		vital:       component.NewStore[health.Record](),
		labor:       component.NewStore[work.Mask](),
		stacks:      component.NewStore[item.Stack](),
		frames:      component.NewStore[construction.Frame](),
		bldgs:       component.NewStore[construction.Building](),
		plants:      component.NewStore[plant.Wild](),
		terr:        terrain.NewMap(cfg.Width, cfg.Height),
		zones:       zone.NewMap(cfg.Width, cfg.Height),
		mines:       make(map[[2]int]bool),
		smooths:     make(map[[2]int]bool),
		geo:         geology.NewMap(cfg.Width, cfg.Height),
		lastMeal:    make(map[entity.EntityID]item.Kind),
		bills:       make(map[entity.EntityID][]recipe.Bill),
		racks:       make(map[entity.EntityID]dryBatch),
		fuel:        make(map[entity.EntityID]int),
		exposure:    make(map[entity.EntityID]exposureState),
		noComfort:   make(map[entity.EntityID]uint64),
		biome:       cfg.Biome,
		climate:     biome.DefOf(cfg.Biome).Climate,
		troughs:     make(map[entity.EntityID]troughState),
		noFood:      make(map[entity.EntityID]uint64),
		postponed:   newPostState(),
		restedFree:  make(map[entity.EntityID]bool),
		calm:        make(map[entity.EntityID]uint64),
		banked:      make(map[entity.EntityID]bool),
		orderQueue:  make(map[entity.EntityID][]queuedOrder),
		apparel:     make(map[entity.EntityID]worn),
		prio:        make(map[entity.EntityID]work.Priorities),
		raiders:     make(map[entity.EntityID]raiderState),
		scheds:      make(map[entity.EntityID]schedule.Grid),
		joy:         make(map[entity.EntityID]int),
		breaks:      make(map[entity.EntityID]breakState),
		memories:    make(map[entity.EntityID]mood.Memories),
		roomDirty:   true,
		reserve:     reservation.NewTable(),
		ateAt:       make(map[entity.EntityID]bool),
		logs:        make(map[entity.EntityID][]logEntry),
		draft:       make(map[entity.EntityID]bool),
		sleptAt:     make(map[entity.EntityID]entity.EntityID),
		animals:     component.NewStore[animal.State](),
		equip:       make(map[entity.EntityID]item.Kind),
		carrying:    make(map[entity.EntityID]entity.EntityID),
		tendedUntil: make(map[entity.EntityID]uint64),
		wounds:      make(map[entity.EntityID]wound),
		rockDamage:  make(map[[2]int]int),
		research:    research.Complete(), // sandbox: everything known
		motion:      component.NewStore[motionRec](),
		lastStrike:  make(map[entity.EntityID]strikeRec),
		lastHit:     make(map[entity.EntityID]hitRec),
		streams:     rngStreams{seed: cfg.Seed},
	}
}

// NewColony builds a generated map (terrain, trees, bushes) with the
// first-playable start kit in its open centre.
func NewColony(cfg Config) *Game {
	g := New(cfg)
	g.generate(cfg.Seed)
	g.seedGrass() // before anything is built or planted, so it never depends on when World() is read
	if err := g.LoadStartKit(); err != nil {
		panic(err)
	}
	g.startHome() // home.go: a 13x13 Home around the landing spot (SIM-021)
	g.spawnWildlife()
	g.scheduleThreats()
	g.research = research.Progress{} // a new colony knows nothing yet
	g.warmIndexes(false)             // jobindex.go: not on the first tick
	return g
}

// SpawnPawn creates a pawn at (x, y) with a generated display name.
func (g *Game) SpawnPawn(x, y int) (entity.EntityID, error) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	return g.SpawnNamedPawn(x, y, "")
}

// SpawnNamedPawn creates a pawn with an explicit display name.
func (g *Game) SpawnNamedPawn(x, y int, name string) (entity.EntityID, error) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	if !g.grid.Walkable(x, y) {
		return 0, fmt.Errorf("spawn pawn at (%d, %d): tile not walkable", x, y)
	}
	id := g.entities.Next()
	if name == "" {
		name = fmt.Sprintf("Colonist %d", id)
	}
	g.pawnIDs = append(g.pawnIDs, id)
	g.positions.Set(id, Position{X: x, Y: y})
	g.names.Set(id, name)
	g.jobs.Set(id, job.Record{})
	g.need.Set(id, needs.NewFull())
	g.vital.Set(id, health.Record{Status: health.Alive})
	g.prio[id] = work.DefaultPriorities()
	g.labor.Set(id, g.prio[id].Mask())
	g.scheds[id] = g.sched
	g.joy[id] = recreation.Start
	g.giveBio(id, legacyBio(id, name)) // skills.go: a neutral pawn (NG-1 §0.4)
	return id, nil
}

// SpawnStack places a ground item stack.
func (g *Game) SpawnStack(kind item.Kind, qty, x, y int) (entity.EntityID, error) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	if !g.grid.InBounds(x, y) || qty <= 0 {
		return 0, fmt.Errorf("spawn stack")
	}
	id := g.entities.Next()
	g.itemIDs = append(g.itemIDs, id)
	g.stacks.Set(id, item.Stack{Kind: kind, Qty: qty, X: x, Y: y})
	if kind.IsApparel() {
		g.noteApparel(id) // apparel.go: the free-clothes index
	}
	return id, nil
}

// SpawnPlant places a ripe wild berry bush.
func (g *Game) SpawnPlant(x, y int) (entity.EntityID, error) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	return g.spawnPlant(plant.BerryBush, x, y, plant.DefOf(plant.BerryBush).GrowTicks)
}

// LoadStartKit places three named colonists, food, wood, and ripe berry
// bushes around the map centre.
func (g *Game) LoadStartKit() error {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	cx, cy := g.grid.Width()/2, g.grid.Height()/2
	names := []string{"Ada", "Bo", "Cora"}
	spots := []Position{{cx - 1, cy}, {cx, cy}, {cx + 1, cy}}
	for i, n := range names {
		if _, err := g.SpawnNamedPawn(spots[i].X, spots[i].Y, n); err != nil {
			return err
		}
	}
	if _, err := g.SpawnStack(item.Food, 20, cx+2, cy+1); err != nil {
		return err
	}
	if _, err := g.SpawnStack(item.Food, 10, cx+3, cy+1); err != nil {
		return err
	}
	if _, err := g.SpawnStack(item.Wood, 30, cx+2, cy+3); err != nil {
		return err
	}
	if _, err := g.SpawnStack(item.Rifle, 1, cx-2, cy+2); err != nil {
		return err
	}
	for i := 0; i < 4; i++ {
		if _, err := g.SpawnPlant(cx+2+i, cy-4); err != nil {
			return err
		}
	}
	g.seedFamilies() // people.go: the founders' relations
	return nil
}

// Tick returns the current simulation tick.
func (g *Game) Tick() clock.Tick { return g.clock.Now() }

// Date returns the calendar date for the current simulation tick.
func (g *Game) Date() clock.Date { return g.clock.Date() }

// Activity is the shared colony schedule block for the current hour.
func (g *Game) Activity() schedule.Activity {
	return g.sched.At(g.Date().Hour)
}

// Width returns the map width in tiles.
func (g *Game) Width() int { return g.grid.Width() }

// Height returns the map height in tiles.
func (g *Game) Height() int { return g.grid.Height() }

// Apply queues a typed gameplay command for the next tick's command phase.
func (g *Game) Apply(c command.Command) {
	if c.Kind == command.None {
		return
	}
	g.pending = append(g.pending, c)
}

// Enqueue is the command-gateway alias used by the inbound adapter.
func (g *Game) Enqueue(c command.Command) { g.Apply(c) }

// SetNeeds overwrites hunger/rest (fixtures).
func (g *Game) SetNeeds(id entity.EntityID, hunger, rest int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.need.Set(id, needs.Pair{Hunger: hunger, Rest: rest})
}

// SetWork sets a work-type enable flag immediately (also available via command).
func (g *Game) SetWork(id entity.EntityID, t work.Type, on bool) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	m, ok := g.labor.Get(id)
	if !ok {
		return
	}
	g.labor.Set(id, m.Set(t, on))
}

// Step advances the simulation by n ticks.
// Phases: commands → needs/health → AI/jobs → movement → clock.
func (g *Game) Step(n int) {
	g.viewGen++ // view cache (indexes.go): a new state
	if g.phaseObs != nil {
		g.stepObserved(n)
		return
	}
	for i := 0; i < n; i++ {
		g.phaseCommands()
		g.phasePlants()
		g.phasePower() // power.go: nets balance before the climate (PLAY-021)
		g.phaseClimate()
		g.phaseInfection()
		g.phaseNeeds()
		g.phaseAI()
		g.phaseJobs()
		g.phaseSocial() // social.go: interactions (ENG-003)
		g.phaseAnimals()
		g.phaseThreats()
		g.phaseIncidents() // incidents.go: nature events, departures (SIM-025)
		g.phaseWorld()     // world_state.go: caravans (ENG-003)
		g.phaseWeather()
		g.phaseMovement()
		g.clock.Advance(1)
	}
}

// Snapshot returns a read-only view of every pawn, in insertion order.
// With the view cache on (SetViewCache) repeated calls between ticks share
// one result: treat it as read only.
func (g *Game) Snapshot() []PawnView {
	if g.views.on {
		if g.views.pawnsOK && g.views.pawnsKey == g.viewKey() {
			return g.views.pawns
		}
		g.views.pawns, g.views.pawnsKey, g.views.pawnsOK = g.snapshot(), g.viewKey(), true
		return g.views.pawns
	}
	return g.snapshot()
}

func (g *Game) snapshot() []PawnView {
	act := g.Activity()
	live := g.postponedTops(g.views.postBuf) // postpone.go: one pass for every colonist
	g.views.postBuf = live
	views := make([]PawnView, 0, len(g.pawnIDs))
	for _, id := range g.pawnIDs {
		pos, _ := g.positions.Get(id)
		name, _ := g.names.Get(id)
		j, _ := g.jobs.Get(id)
		n, _ := g.need.Get(id)
		v, _ := g.vital.Get(id)
		m, _ := g.labor.Get(id)
		reason := v.Cause.String()
		if j.Kind == job.Sleep && v.Status == health.Alive && g.sleptAt[id] == 0 && j.State != job.Idle {
			reason = health.CauseNoBed.String()
		}
		views = append(views, PawnView{
			ID:            id,
			Name:          name,
			X:             pos.X,
			Y:             pos.Y,
			Activity:      act,
			Job:           j.Line(),
			Hunger:        needs.Percent(n.Hunger),
			Rest:          needs.Percent(n.Rest),
			Status:        v.Status.String(),
			Reason:        reason,
			WorkHaul:      m.Enabled(work.Haul),
			WorkConstruct: m.Enabled(work.Construct),
			WorkGrow:      m.Enabled(work.Grow),
			WorkMine:      m.Enabled(work.Mine),
			WorkCook:      m.Enabled(work.Cook),
			WorkHunt:      m.Enabled(work.Hunt),
			WorkCraft:     m.Enabled(work.Craft),
			WorkDoctor:    m.Enabled(work.Doctor),
			WorkResearch:  m.Enabled(work.Research),
			WorkClean:     m.Enabled(work.Clean),
			WorkFirefight: m.Enabled(work.Firefight),
			Dirt:          g.carriedPct(id),
			Ate:           g.ateAt[id],
			LastMeal:      lastMealName(g, id),
			SleptInBed:    g.sleptAt[id] != 0,
			Drafted:       g.draft[id],
		})
		ts := g.thoughts(id)
		pv := &views[len(views)-1]
		pv.Mood, pv.MoodLabel, pv.Thoughts = mood.Level(ts), mood.Label(mood.Level(ts)), thoughtViews(ts)
		g.fillSocialPawn(pv, pv.Mood) // social_view.go: smoothed mood, target, bubble (ENG-003)
		pv.Health = v.HP() * 100 / health.MaxHP
		pv.Infection, pv.Wounded = g.wounds[id].Infection, g.wounds[id].At != 0 && v.Injury > 0
		pv.Weapon = weaponDefOf(g, id).Name
		pv.Lying = v.Status != health.Alive || (j.Kind == job.Sleep && (j.State == job.Do || j.State == job.Complete))
		pv.InBed = g.inBed(id)
		pv.CarriedBy = g.carriedBy(id)
		pv.Tended = g.tended(id)
		pv.Hypothermia, pv.Heatstroke = g.exposure[id].hypo, g.exposure[id].heat
		pv.Temperament = g.temperamentOf(id).String()
		pr := g.prioOf(id)
		for _, v := range pr {
			pv.Priorities = append(pv.Priorities, int(v))
		}
		sg := g.scheduleOf(id)
		pv.Schedule = append([]schedule.Activity(nil), sg[:]...)
		pv.Activity = g.activityOf(id)
		pv.Apparel = g.Wearing(id)
		pv.ApparelForced, pv.ChangingInto = g.wornForced(id), g.changingInto(id) // apparel.go
		pv.Recreation = g.joy[id] * 100 / recreation.Full
		if b, ok := g.breaks[id]; ok {
			pv.MentalBreak = b.kind.String()
		}
		g.fillFleeView(pv, id)       // an unarmed colonist running from a threat (flee.go)
		g.fillBedRestView(pv, id, j) // bedrest.go: "Rest when hurt", resting in bed
		b := g.bandsOf(id)
		pv.ComfortMin, pv.ComfortMax = b.Min, b.Max
		if j, _ := g.jobs.Get(id); j.Forced && j.Active() {
			pv.HasOrder, pv.OrderX, pv.OrderY = true, j.TX, j.TY
			if j.Kind == job.Bill || j.Kind == job.Refuel || j.Carry != 0 {
				pv.OrderX, pv.OrderY = j.DestX, j.DestY
			}
		}
		for _, q := range g.orderQueue[id] {
			pv.QueuedOrders = append(pv.QueuedOrders, QueuedOrderView{Label: q.label, X: q.cmd.X, Y: q.cmd.Y})
		}
		pv.Postponed = g.postponedLinesIn(live, id) // postpone.go (SIM-027)
		if pos, ok := g.positions.Get(id); ok {
			pv.Temp = int(g.temperatureAt(pos.X, pos.Y))
		}
		pv.Acting, pv.ActX, pv.ActY = g.acting(id, j)
		g.fillPawnAction(pv, id, j)
	}
	views = append(views, g.raiderViews()...)
	for i := range views {
		g.fillPawnMotion(&views[i])
		g.fillPawnDeath(&views[i])             // burial.go
		g.fillPawnWeather(&views[i])           // weather.go
		views[i].Log = g.logLines(views[i].ID) // eventlog.go
		g.fillPawnBio(&views[i])               // skills.go: NG-1 identity
		views[i].Disfigured = g.bodies[views[i].ID] != nil && g.bodies[views[i].ID].Disfigured()
	}
	return g.fogPawns(views) // fog_view.go: strangers in the dark don't show
}

// World returns the non-pawn read model. Per-tile layers (Terrain,
// RoomGrid, StoneType, Ore) are shared read-only slices replaced, never
// rewritten, when they change. With the view cache on (SetViewCache)
// repeated calls between ticks share one result: treat it as read only.
func (g *Game) World() WorldView {
	if g.views.on {
		if g.views.worldOK && g.views.worldKey == g.viewKey() {
			return g.views.world
		}
		g.views.world, g.views.worldKey, g.views.worldOK = g.world(), g.viewKey(), true
		return g.views.world
	}
	return g.world()
}

func (g *Game) world() WorldView {
	bd := biome.DefOf(g.biome)
	w := WorldView{
		Biome: bd.Name, BiomeKey: bd.Key,
		Items:       make([]ItemView, 0, len(g.itemIDs)),
		Frames:      make([]FrameView, 0, len(g.frameIDs)),
		Buildings:   make([]BuildingView, 0, len(g.bldgIDs)),
		Plants:      make([]PlantView, 0, len(g.plantIDs)),
		OutdoorTemp: int(g.climate.Outdoor(g.clock.Now())),
		IndoorTemp:  int(g.climate.Indoor(g.clock.Now())),
		Season:      temperature.Season(g.clock.Now()),
		Messages:    append([]MessageView(nil), g.messages...),
		Letters:     g.letterViews(), // letters.go (PLAY-018)
		Research:    g.researchViews(),
		Zones:       g.zoneViews(),
		Locked:      g.lockedKeys(),
		Schedule:    append([]schedule.Activity(nil), g.sched[:]...),
	}
	if n, felled := g.Timbergnaws(); n > 0 {
		w.TreesFelled = felled
	}

	var stored [256]int // stockpiled quantity per item kind (stored.go)
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if !ok || g.fogged(st.X, st.Y) { // fog.go: hidden items neither show nor count
			continue
		}
		g.countStored(&stored, st)
		w.Items = append(w.Items, ItemView{
			ID: id, Kind: st.Kind.String(), Qty: st.Qty, X: st.X, Y: st.Y, Carried: st.Carrier != 0,
			FreshDays: freshDays(st), Rotten: st.Rotten(), Dried: st.Dried,
			RotRate: item.RotRate(int(g.temperatureAt(st.X, st.Y))),
			Catch:   filth.Catch(st.Kind) / 10, Forbidden: st.Forbidden,
			Unclaimed: st.Unclaimed, HaulMarked: st.HaulMarked,
		})
		g.fillCorpseView(&w.Items[len(w.Items)-1], st) // burial.go
		if st.Carrier == 0 {
			switch {
			case st.Kind.IsFood():
				w.Food += st.Qty
				if w.FoodByKind == nil {
					w.FoodByKind = map[string]int{}
				}
				w.FoodByKind[st.Kind.String()] += st.Qty
			case st.Kind == item.Wood:
				w.Wood += st.Qty
			case item.IsStoneBlocks(st.Kind) || st.Kind == item.Stone:
				w.Stone += st.Qty
			case st.Kind == item.Steel:
				w.Steel += st.Qty
			case st.Kind == item.Silver:
				w.Silver += st.Qty
			case st.Kind == item.Gold:
				w.Gold += st.Qty
			}
		}
	}
	w.Stored = storedRows(&stored)
	for _, id := range g.frameIDs {
		fr, ok := g.frames.Get(id)
		if !ok {
			continue
		}
		w.Frames = append(w.Frames, FrameView{
			ID: id, Kind: fr.Kind.String(), X: fr.X, Y: fr.Y, Wood: fr.Wood, Need: fr.Need, WorkLeft: fr.WorkLeft,
			Facing: fr.Facing.String(), Floor: construction.DefOf(fr.Kind).Layer == construction.LayerFloor,
			Stuff: fr.Stuff.String(),
		})
		fv := &w.Frames[len(w.Frames)-1]
		fv.W, fv.H, _, _, _ = footprintView(fr.Kind, fr.Facing, fr.X, fr.Y)
		if fr.Reinstall != 0 {
			fv.Moving, fv.MovingFrom = true, entity.EntityID(fr.Reinstall)
		}
	}
	g.fillFrameViews(w.Frames) // frame_view.go: inspector fields
	for _, id := range g.bldgIDs {
		b, ok := g.bldgs.Get(id)
		if !ok {
			continue
		}
		w.Buildings = append(w.Buildings, BuildingView{
			ID: id, Kind: b.Kind.String(), X: b.X, Y: b.Y, Claimant: entity.EntityID(b.Claimant), Claimant2: entity.EntityID(b.Claimant2),
			Facing: b.Facing.String(), Floor: construction.DefOf(b.Kind).Layer == construction.LayerFloor,
			Deconstruct: b.Deconstruct, Stuff: b.Stuff.String(),
			Bench: construction.DefOf(b.Kind).Bench, Bills: g.billViews(id),
		})
		bv := &w.Buildings[len(w.Buildings)-1]
		bv.HPPct = b.HP() * 100 / max(1, construction.MaxHP(b.Kind, b.Stuff))
		if b.Kind == construction.Carpet {
			bv.Catch = filth.Catch(b.Stuff) / 10
		}
		if pct, ok := g.fuelPct(id, b); ok { // campfire, torch lamp, wood-fired generator (fuel.go)
			bv.Fuel, bv.FuelPct, bv.Burning = true, pct, g.burning(id, b)
		}
		if b.Kind == construction.FeedingTrough {
			bv.TroughFill, bv.TroughCap = g.troughFill(id), construction.TroughCapacity
			bv.TroughFilter = troughFilterNames[min(int(g.troughs[id].Filter), len(troughFilterNames)-1)]
		}
		if rb := g.racks[id]; rb.Qty > 0 {
			bv.DryKind, bv.DryQty, bv.DryPct = rb.Kind.String(), rb.Qty, rb.percent()
		}
		bv.W, bv.H, bv.SpotX, bv.SpotY, bv.HasSpot = footprintView(b.Kind, b.Facing, b.X, b.Y)
		g.fillGraveView(bv, id, b)   // burial.go
		bv.MovingTo = g.movingTo(id) // move.go
		g.fillSwapView(bv, id)       // placement.go: pending material change (PLAY-011)
		bv.Ruin = b.Ruin             // MAPGEN-2: an abandoned piece
		bv.Medical = b.Medical       // PLAY-020: kept for the sick and injured
		g.fillCasketView(bv, id, b)  // casket.go (MAPGEN-3)
		// PLAY-021: the light the renderer draws (light.go), then power
		// (power_view.go)
		bv.Glow, bv.GlowRadius = g.glowOf(id, b)
		g.fillPowerBuilding(bv, id, b)
	}
	w.Plants = g.plantViews(w.Plants) // plant_view.go
	w.Landing[0], w.Landing[1] = g.landingSpot()
	w.TerrainW = g.terr.W
	w.Terrain = g.terrainView() // indexes.go: shared, rebuilt per structVer
	w.Mines = g.mineList()
	w.Smooths = g.smoothList()
	w.StoneType, w.Ore = g.stoneLayers()
	w.StructVer = g.structVer
	g.ensureRooms()
	w.RoomGrid = g.roomGrid // shared read-only: ensureRooms replaces it, never writes in place
	w.Rooms = g.roomViews()
	w.Animals = g.animalViews()
	w.Traces = g.traceViews()
	g.fillWorldWeather(&w) // weather.go
	g.fillRoofView(&w)     // roof.go
	g.fillWildlifeView(&w) // wildlife.go
	g.fillGrassView(&w)    // grass.go
	g.fillStainView(&w)    // stains.go
	g.fillDirtView(&w)     // filth.go
	g.fillHomeView(&w)     // home.go
	g.fillSnowView(&w)     // snow_clear.go (PLAY-015)
	g.fillPenView(&w)      // pens.go
	g.fillPlanView(&w)     // plan.go
	g.fillSetupView(&w)    // setup_view.go (NG-1)
	g.fillFogView(&w)      // fog_view.go (MAPGEN-2)
	g.fillFireView(&w)     // fire.go (PLAY-015)
	g.fillPowerView(&w)    // power_view.go (PLAY-021)
	now := uint64(g.clock.Now())
	w.Hour = float64(now%clock.TicksPerDay) / clock.TicksPerHour
	w.DayOfYear = int(now/clock.TicksPerDay) % clock.DaysPerYear
	return w
}

func lastMealName(g *Game, id entity.EntityID) string {
	if k, ok := g.lastMealKind(id); ok {
		return k.String()
	}
	return ""
}

func footprintView(k construction.Kind, f construction.Facing, x, y int) (w, h, sx, sy int, has bool) {
	cells, spot, ok := construction.Footprint(k, f, x, y)
	maxX, maxY := x, y
	for _, c := range cells {
		if c[0] > maxX {
			maxX = c[0]
		}
		if c[1] > maxY {
			maxY = c[1]
		}
	}
	return maxX - x + 1, maxY - y + 1, spot[0], spot[1], ok
}
