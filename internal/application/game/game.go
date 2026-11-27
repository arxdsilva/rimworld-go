// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Package game orchestrates the authoritative simulation: clock, grid,
// pawns, jobs, movement, items, needs, construction, and harvest.
package game

import (
	"fmt"
	"rimworld-go/internal/application/game/animalstate"

	"rimworld-go/internal/application/commands"
	"rimworld-go/internal/application/game/aqua"
	"rimworld-go/internal/application/game/axview"
	"rimworld-go/internal/application/game/bodytemp"
	"rimworld-go/internal/application/game/books"
	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/mapfields"
	"rimworld-go/internal/application/game/mapview"
	"rimworld-go/internal/application/game/mind"
	"rimworld-go/internal/application/game/pathstage"
	"rimworld-go/internal/application/game/prison"
	"rimworld-go/internal/application/game/raid"
	rlab "rimworld-go/internal/application/game/research"
	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/application/game/sched"
	"rimworld-go/internal/application/game/thermo"
	"rimworld-go/internal/application/game/viewmodel"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/application/game/wealth"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/world/biome"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/reservation"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/area"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/internal/domain/world/planet"
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
	Biome    biome.ID
	Calendar clock.CalendarPolicy
	Climate  *temperature.Climate
	Pool     pathstage.Pool // the path stage's lanes (GAR-035): a built colony and the maps it opens; nil serial
	// Content is the run's frozen content (PLG-024); the zero value is core.
	Content Content
	// Scripts runs the enabled plugins' scripts (PLG-060); nil runs none.
	Scripts scripting.Host
}

// Published read-model names remain stable for callers of game.
type (
	PawnView     = viewmodel.PawnView
	ItemView     = viewmodel.ItemView
	LetterView   = viewmodel.LetterView
	FrameView    = viewmodel.FrameView
	BuildingView = viewmodel.BuildingView
	PlantView    = viewmodel.PlantView
	PackItemView = viewmodel.PackItemView
)

// StoredResourceView is a stockpiled item row (mapview).
type StoredResourceView = mapview.StoredResourceView

const FacilityEndTable = viewmodel.FacilityEndTable
const FacilityDresser = viewmodel.FacilityDresser

// PluginPanelView is one plugin's panel as the Plugins window shows it.
type PluginPanelView struct {
	Plugin, Title string
	Lines         []string
	Actions       []PluginActionView
}

// PluginActionView is a panel's button; pressing it sends a ScriptAction.
type PluginActionView struct{ ID, Label string }

// WorldView is the non-pawn read model for HUD and the map.
type WorldView struct {
	// PluginPanels are the plugins' panels, refreshed hourly (W1-game).
	PluginPanels []PluginPanelView
	// Gameplay is the colony's habit rules (CSE); the Habits tab shows it.
	Gameplay gameplay.Settings
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
	Shots   []projectile.View // DEF-1: projectiles in flight, oldest first (combat/shot.go)
	Smoke   []explosion.Cloud // CBT-5: live smoke clouds, each until its Until (combat/blast.go)
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
	Letters       []LetterView
	Quests        []QuestView  // QST-1: every quest, ids ascending (memoized; quest_host.go)
	NoGoodwill    []int        // QST-083: faction refs whose goodwill the player declines as a reward
	VisitorGroups []visit.View // active visitor parties and purposes, sorted by group ID
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
	// (row-major, TerrainW wide); RoofCollapses are collapses of the last hour.
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
	CombatLog  []CombatEventView // DEF-4 (raids.go): recent fights and threats, oldest first
	Conditions []string
	Extreme    string
	// Electricity (PLAY-021, power_view.go): every power net by id, the
	// solar flare in progress, and PowerVer, which changes whenever the
	// nets or any device's power state do (presentation caches key on it).
	PowerNets  []PowerNetView
	SolarFlare bool
	PowerVer   uint64
	// FOOD-0 contract (zero until FOOD-4/5): days of food (tenths), food policies, auto-slaughter caps.
	FoodDays      int
	FoodPolicies  []FoodPolicyView
	AutoSlaughter []AutoSlaughterView
	// FOOD-6 (incident/sky.go): the sky event under way ("toxic fallout",
	// "eclipse" or "") and its end tick (0: none).
	SkyEvent     string
	SkyEventEnds uint64
	SkySongEnds  uint64 // #13 (incident/windfall.go): the sky song's end tick while it plays (0: none)
	// TEMP-0 contract (zero until TEMP-2, TEMP-1): the climate event under way (its incident key or "") and end tick;
	// TempVer, the temperature overlay's cache key: it changes whenever a cell's air may have (game/thermo).
	ClimateEvent              string
	ClimateEventEnds, TempVer uint64
	ResearchAlert             bool           // RSCH-1: no project picked while one is open, a bench stands and a colonist can research
	TechLevel                 string         // RSCH-1: the colony's tech level (research.TechLevel's name; it prices projects)
	AX                        axview.World   // AX-0 contract (zero until its owners fill it)
	Gear                      gearview.World // GEAR-3 contract (game/outfit)
}

type FoodPolicyView = viewmodel.FoodPolicyView
type AutoSlaughterView = viewmodel.AutoSlaughterView

// Game holds all authoritative simulation state: the colony-wide part
// here, and the map being stepped or viewed through the embedded
// *mapState (MAP-0). Every map in maps ticks every tick (Step).
type Game struct {
	*mapState
	// maps are the open maps in id order, maps[0] first; view is the one
	// the read models report between ticks (ViewMap).
	maps     []*mapState
	view     *mapState
	clock    *clock.Clock
	entities *entity.Allocator
	rng      *randx.Rand
	sched    schedule.Grid
	// thoughtBuf is thoughtLevel's reused thought list (rooms.go; never
	// saved, never handed out).
	thoughtBuf []mood.Thought

	messages   []MessageView
	letters    letters.State // the letter stack (game/letters, PLAY-018)
	names      *component.Store[string]
	need       *component.Store[needs.Pair]
	vital      *component.Store[health.Record]
	labor      *component.Store[work.Mask]
	lastMeal   map[entity.EntityID]item.Kind
	fuel       map[entity.EntityID]int      // campfire fuel, ticks of burning
	graves     map[entity.EntityID]graveRec // who lies in each filled grave (burial.go, SIM-017)
	exposure   bodytemp.Ledger              // TEMP-3: hypothermia and heatstroke, pawns and animals
	noComfort  map[entity.EntityID]uint64   // no warm/cool spot found until this tick
	troughs    map[entity.EntityID]troughState
	noFood     map[entity.EntityID]uint64 // food out of reach: no eating attempt until this tick
	calm       map[entity.EntityID]uint64 // fled a threat: holding back from work until this tick
	banked     map[entity.EntityID]bool   // campfires banked by their room thermostat (heat.go)
	orderQueue map[entity.EntityID][]queuedOrder
	apparel    map[entity.EntityID]worn
	prio       map[entity.EntityID]work.Priorities
	nextThreat uint64
	threatsOn  bool
	scheds     map[entity.EntityID]schedule.Grid
	joy        map[entity.EntityID]int // recreation need, 0..recreation.Full
	breaks     map[entity.EntityID]breakState
	memories   map[entity.EntityID]mood.Memories
	cmds       commands.Queue // pending commands in (Tick, Seq) order (#8)

	ateAt   map[entity.EntityID]bool
	logs    map[entity.EntityID][]logEntry // per-creature event logs (eventlog.go)
	draft   map[entity.EntityID]bool
	sleptAt map[entity.EntityID]entity.EntityID // pawn -> bed, 0 = ground
	equip   map[entity.EntityID]item.Kind       // wielded weapon; absent = fists
	// lastStrike and lastHit are transient presentation stamps (see
	// action_view.go); the simulation never reads them.
	lastStrike map[entity.EntityID]strikeRec
	lastHit    map[entity.EntityID]hitRec

	lab       *rlab.Lab // game/research (RSCH-1): progress, the current project, the bench job
	moverBuf  []mover
	moverOld  []entity.EntityID // pruneMotion scratch
	raiderBuf []entity.EntityID // phaseThreats scratch
	// enterHook, when set (tests only), replaces enter's verdicts.
	enterHook func(id entity.EntityID, from, to Position) (stepVerdict, Position)
	// systems: the phase systems in order (phases.go, game/sched); its
	// observer is set only from cmd/* and benchmarks, never read by gameplay.
	systems *sched.Schedule
	viewGen uint64
	// sk: bios, skills, stats and learning (skills.go, NG-1 PAWN-G).
	sk      skillState
	scripts scriptRun // scripts.go: the plugins' script host (PLG-060)
	// ng: the new-game setup's live state (setup.go, NG-1 SETUP); zero
	// for a legacy colony.
	ng ng1State
	// Social and host systems (ENG-003): each container is nil until its
	// milestone first needs it, and saved as an additive field.
	soc     *socialState                    // social_state.go
	people  *peopleRegistry                 // people.go
	bodies  map[entity.EntityID]*body.Body  // body.go
	med     medState                        // patients.go (MED-00)
	care    *care.Care                      // care package (SIM-061): carry, policy, doctor
	mind    *mind.Mind                      // mind package (SIM-066): mental breaks
	craft   *craft.Craft                    // craft package (PRD-0): bills, drying, tool cabinets
	surg    *surgeryState                   // surgery.go
	drugs   map[entity.EntityID]*drug.State // dose_host.go, game/dose
	prison  *prison.Prison                  // prison package: prisoners, warden work
	rest    *rest.Sleep                     // rest package (BED-0): beds, sleep, waking thoughts
	worldSt *worldState                     // world_state.go
	history []historyEntry                  // history.go
	streams rngStreams                      // rng_streams.go: never g.rng
	// PLAY-024: traders, request cooldowns and the open trade or comms
	// session (trade.go), saved as the additive Trade field.
	trade tradeState
}

// mapState is one map's state (MAP-0): its grid, terrain, the entities
// standing on it and their map components, and every cache derived
// from them. Entity ids are global, so tables keyed by entity stay on
// Game. It has no methods: Game reaches its fields through the embedded
// pointer, which Step points at each map in turn.
type mapState struct {
	configEdits []command.ConfigMutation // accepted configuration commands in the last step/batch; not saved
	id          uint32                   // 0 for the first map
	tile        int                      // the world tile under the map, -1 without a world
	grid        *grid.Grid
	wl          wildlifeState // SIM-019 wildlife events (wildlife.go)
	// tanks: the fish tanks in use on this map (fuel.go, game/aqua), nil
	// until one is; saved as the optional Tanks field.
	tanks *aqua.Tanks

	pawnIDs   []entity.EntityID
	positions *component.Store[Position]
	jobs      *component.Store[job.Record]

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
	stoneView stoneView // world.go: stoneLayers' cache, at structVer

	zones      *zone.Map           // stockpiles, grow zones and pens (zones.go, ZONE-001)
	weather    weather.State       // outdoor weather (weather.go, SIM-016)
	weatherFix bool                // dev: HoldWeather keeps the forced weather
	biome      biome.ID            // the map's setting (SIM-023)
	climate    temperature.Climate // its outdoor temperature curve
	incidents  incident.State      // the storyteller's nature events (SIM-025)
	wealth     *wealth.Tracker     // game/wealth (EAI-0, initMapHosts); saved
	postponed  postState           // work put off, orders followed (postpone.go, SIM-027)
	clothes    apparelIndex        // free apparel stacks (apparel.go), never saved
	raiderIDs  []entity.EntityID
	raiders    map[entity.EntityID]raid.Raider
	raidGroups []raid.Group
	thermo     *thermo.Model // game/thermo (TEMP-1): the rooms' air and breached cells' residual
	roomGrid   []int
	rooms      []roomStats
	roomDirty  bool
	fence      fenceIndex  // fence and gate cells, derived (fences.go, SIM-028)
	regs       regionState // walkability regions and labels, derived (regions.go, NG-1 E2)
	reserve    *reservation.Table

	animalIDs []entity.EntityID
	animals   *animalstate.Registry
	traces    []projectile.Trace
	wild      bool // wildlife migrates in (generated colonies)
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

	// motion: per-creature step timing (motion_store.go); moverBuf is
	// scratch for the ascending-ID mover list.
	motion *component.Store[motionRec]
	// pathSearch is the game's own A* state (paths.go: searcher), never
	// saved; a pointer so a copied Game shares one search generation.
	pathSearch *pathing.Searcher
	animalQ    [3]animalQCache // fences.go: animalPathQuery, never saved
	occ        occupancy       // movement blocking index (occupancy.go), transient
	// haul: in-flight material deliveries (haul_route.go, HAUL-001).
	haul haulState
	// monitors: each hospital bed's vitals monitor, per structVer (items_ng1.go), never saved.
	monitors monitorCache
	// vents: steam vent cells per structVer (vents.go), never saved.
	vents ventCache
	// light: the simulation's artificial light grid (light.go), never saved.
	light lightGrid
	// idx: derived per-cell lookups (indexes.go), never saved.
	idx indexes
	// cellSets and sapperSet: building/terrain cell sets cached per
	// structVer (defense.go, raid_plans.go), never saved.
	cellSets  map[[4]uint64]*cellCache
	sapperSet cellCache
	// views, viewGen, viewTerr: read-model caches (indexes.go).
	views    viewCache
	viewTerr layerCache
	// plantVer moves on every plant store write (setPlant, dropPlant in
	// world.go): the plant view memo keys on it. Never saved or hashed.
	plantVer uint64
	// viewGrass, viewHome, viewRoof: copy-on-write read-model layers
	// (indexes.go shareLayer).
	viewGrass, viewHome, viewRoof layerCache
	viewPlan                      layerCache // PLAY-012 plan marks
	// minesVer bumps on every mine designation change (mineList cache);
	// seedCache is weatherSeed's (indexes.go). Never saved.
	minesVer  uint64
	mineCache mineCache
	seedCache seedCache
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
	mapFields *mapfields.Cache
	areas     *area.Set // areas.go
	// pw is the colony's electricity (power.go, PLAY-021); nil until the
	// colony has a power device.
	pw *powerState
	// pwc: power nets and device lists per structVer (power.go), never saved.
	pwc powerCache
	// combat package (DEF-0): sight, cover, sentry turrets (their aims
	// saved as the additive Turrets field); combat_host.go.
	combat *combat.Combat
	raid   *raid.Raid // raid package (DEF-4): raider AI, threats, hives, sappers' rock; raid_host.go
}

// New builds a Game from cfg. Width/Height must be positive.
func New(cfg Config) *Game {
	m := newMapState(cfg, 0)
	g := &Game{
		mapState:   m,
		maps:       []*mapState{m},
		view:       m,
		clock:      clock.NewAt(clock.StartingMorning(), cfg.Calendar),
		entities:   entity.NewAllocator(),
		rng:        randx.New(cfg.Seed),
		sched:      schedule.DefaultColony(),
		names:      component.NewStore[string](),
		need:       component.NewStore[needs.Pair](),
		vital:      component.NewStore[health.Record](),
		labor:      component.NewStore[work.Mask](),
		lastMeal:   make(map[entity.EntityID]item.Kind),
		fuel:       make(map[entity.EntityID]int),
		exposure:   make(bodytemp.Ledger),
		noComfort:  make(map[entity.EntityID]uint64),
		troughs:    make(map[entity.EntityID]troughState),
		noFood:     make(map[entity.EntityID]uint64),
		calm:       make(map[entity.EntityID]uint64),
		banked:     make(map[entity.EntityID]bool),
		orderQueue: make(map[entity.EntityID][]queuedOrder),
		apparel:    make(map[entity.EntityID]worn),
		prio:       make(map[entity.EntityID]work.Priorities),
		scheds:     make(map[entity.EntityID]schedule.Grid),
		joy:        make(map[entity.EntityID]int),
		breaks:     make(map[entity.EntityID]breakState),
		memories:   make(map[entity.EntityID]mood.Memories),
		ateAt:      make(map[entity.EntityID]bool),
		logs:       make(map[entity.EntityID][]logEntry),
		draft:      make(map[entity.EntityID]bool),
		sleptAt:    make(map[entity.EntityID]entity.EntityID),
		equip:      make(map[entity.EntityID]item.Kind),
		lastStrike: make(map[entity.EntityID]strikeRec),
		lastHit:    make(map[entity.EntityID]hitRec),
		streams:    rngStreams{seed: cfg.Seed},
	}
	g.care = care.New(newCareHost(g), care.Roles{Prison: carePrison{g}, Guests: careGuests{g}, Social: careSocial{g}, Gear: careGear{g}, Meals: careMeals{g}, Gifts: careGifts{g}, Captives: careCaptives{g}}) // care_host*.go, quest_host.go
	g.mind, g.systems = mind.New(newMindHost(g)), phaseSystems(g)
	initMapHosts(g)
	g.craft = craft.New(newCraftHost(g))    // craft_host.go
	g.prison = prison.New(newPrisonHost(g)) // prison_host.go
	g.rest = rest.New(newRestHost(g))       // rest_host.go
	g.lab = rlab.New(researchHost{g})       // research.go
	g.sk.content = cfg.Content              // the run's books (skills.go, PLG-024)
	g.scripts.host = cfg.Scripts            // scripts.go (PLG-060); nil runs none
	g.craft.Bills.UseBook(cfg.Content.Recipes)
	g.lab.Prog, g.lab.Recipes = cfg.Content.Research.Complete(), cfg.Content.Recipes // sandbox: everything known; the run's recipes (PLG-025)
	return g
}

// CalendarOf returns the authoritative colony calendar.
func CalendarOf(g *Game) clock.CalendarPolicy { return g.clock.Calendar() }

// newMapState allocates map id's stores at cfg's size (MAP-0). Its
// counters start at id<<48, so map versions never meet another map's in a
// cache keyed on them; the first map's start at zero as before.
func newMapState(cfg Config, id uint32) *mapState {
	v := uint64(id) << 48
	return &mapState{
		id:        id,
		tile:      -1,
		structVer: v,
		plantVer:  v,
		minesVer:  v,
		grid:      grid.New(cfg.Width, cfg.Height),
		positions: component.NewStore[Position](),
		jobs:      component.NewStore[job.Record](),
		stacks:    component.NewStore[item.Stack](),
		frames:    component.NewStore[construction.Frame](),
		bldgs:     component.NewStore[construction.Building](),
		plants:    component.NewStore[plant.Wild](),
		terr:      terrain.NewMap(cfg.Width, cfg.Height),
		zones:     zone.NewMap(cfg.Width, cfg.Height),
		mines:     make(map[[2]int]bool),
		smooths:   make(map[[2]int]bool),
		geo:       geology.NewMap(cfg.Width, cfg.Height),
		biome:     cfg.Biome,
		climate:   biome.ClimateOf(cfg.Biome, cfg.Climate, cfg.Calendar),
		postponed: newPostState(),
		raiders:   make(map[entity.EntityID]raid.Raider),
		roomDirty: true,
		reserve:   reservation.NewTable(),
		animals:   animalstate.New(),
		motion:    component.NewStore[motionRec](),
	}
}

// initMapHosts builds the current map's own clusters over g (MAP-0): their
// hosts reach the map through g, so they serve whichever map is current.
func initMapHosts(g *Game) {
	g.combat = newCombat(g)
	g.raid = newRaid(g)                     // raid_host.go
	g.thermo = thermo.New(newThermoHost(g)) // thermo_host.go
	g.wealth = wealth.New(g)                // game/wealth (EAI-0)
}

// MapID preserves the native identifier in saved message and map state.
type MapID uint32

// MapView is one open map for the map switcher and the world view (MAP-0).
type MapView struct {
	ID                   MapID
	Label                string // "Colony", a site or settlement name
	Kind                 string // "colony", "site", "settlement" or "encounter"
	Tile                 int    // world tile, -1 without a world
	Colonists            int    // our colonists standing on it
	Threat               bool   // hostiles on the map
	Countdown, Reinforce int    // ticks left to pack up before it closes, and to a quest site's reinforcements (QST-017); 0: none
}

// primary reports whether the current pass runs the colony-wide work: the
// first map's (MAP-0). With one map it always does.
func primary(g *Game) bool { return len(g.maps) <= 1 || g.mapState == g.maps[0] }

// owns reports whether the current map's pass handles id's work in a
// colony-wide table (needs, patients, caravan members): the map it stands
// on, or the first map for a creature on no map. With one map it always does.
func owns(g *Game, id entity.EntityID) bool {
	if len(g.maps) <= 1 || g.positions.Has(id) || g.animals.Has(id) {
		return true
	}
	if !primary(g) {
		return false
	}
	for _, m := range g.maps[1:] {
		if m.positions.Has(id) || m.animals.Has(id) {
			return false
		}
	}
	return true
}

// mapByID returns the open map id, or nil.
func mapByID(g *Game, id uint32) *mapState {
	for _, m := range g.maps {
		if m.id == id {
			return m
		}
	}
	return nil
}

// ViewMap makes map id the one the read models report (MAP-0). It is a
// presentation choice, not a command: it never changes how a tick runs.
// An unknown id is ignored.
func (g *Game) ViewMap(id MapID) {
	m := mapByID(g, uint32(id))
	if m == nil || m == g.view {
		return
	}
	g.view, g.mapState = m, m
	g.viewGen++ // view cache (indexes.go): another map's state
}

// ViewedMap returns the map the read models report.
func (g *Game) ViewedMap() MapID { return MapID(g.view.id) }

// Maps lists the open maps in id order.
func (g *Game) Maps() []MapView {
	out := make([]MapView, 0, len(g.maps))
	for _, m := range g.maps {
		out = append(out, mapView(g, m))
	}
	return out
}

// mapView reads map m's switcher row; the census swaps to m and back.
func mapView(g *Game, m *mapState) MapView {
	v := MapView{ID: MapID(m.id), Label: "Colony", Kind: "colony", Tile: m.tile}
	if m.id == 0 {
		v.Tile = g.colonyTile()
	}
	v.Label, v.Kind, v.Countdown, v.Reinforce = mapsLabel(g, m.id, v.Label, v.Kind) // world_state.go: game/maps' atlas
	cur := g.mapState
	g.mapState = m
	for _, id := range g.pawnIDs {
		if _, r := g.raiders[id]; !r {
			v.Colonists++
		} else if g.isRaider(id) {
			v.Threat = true
		}
	}
	g.mapState = cur
	return v
}

// Colonists returns every colonist on every open map, each tagged with its
// map, maps in id order (MAP-0). Caravan members are listed by WorldMap.
func (g *Game) Colonists() []PawnView {
	cur := g.mapState
	var out []PawnView
	for _, m := range g.maps {
		g.mapState = m
		for _, v := range g.Snapshot() { // each map keeps its own view cache
			if _, r := g.raiders[v.ID]; !r {
				out = append(out, v)
			}
		}
	}
	g.mapState = cur
	return out
}

// NewColony builds a generated map (terrain, trees, bushes) with the
// first-playable start kit in its open centre.
func NewColony(cfg Config) *Game {
	g := New(cfg)
	g.generate(cfg.Seed, planet.LegacySite(cfg.Seed, g.biome))
	g.seedGrass() // before anything is built or planted, so it never depends on when World() is read
	if err := g.LoadStartKit(); err != nil {
		panic(err)
	}
	g.startHome() // home.go: a 13x13 Home around the landing spot (SIM-021)
	g.spawnWildlife()
	g.scheduleThreats()
	g.lab.Prog = research.Progress{} // a new colony knows nothing yet
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
	addPawnID(g, id) // scripts.go: pawnIDs and the cadence index (PLG-060)
	g.positions.Set(id, Position{X: x, Y: y})
	g.names.Set(id, name)
	g.jobs.Set(id, job.Record{})
	g.need.Set(id, needs.NewFull())
	g.vital.Set(id, health.Record{Status: health.Alive})
	g.prio[id] = work.DefaultPriorities()
	g.labor.Set(id, g.prio[id].Mask())
	g.scheds[id] = g.sched
	g.joy[id] = recreation.Start
	g.giveBio(id, legacyBio(id, name))        // skills.go: a neutral pawn (NG-1 §0.4)
	g.apparel[id] = craft.Outfit{}.Everyday() // TEMP-4: arrives in a T-shirt and pants
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
	g.putStack(id, item.Stack{Kind: kind, Qty: qty, X: x, Y: y})
	if kind.IsBook() {
		books.Mint(g.stacks.Ref(id), g.streams.seed, uint64(id), g.lab.Done) // game/books (A7): quality and title, hashed
	}
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
	if _, err := g.SpawnStack(item.SurvivalMeal, 20, cx+2, cy+1); err != nil {
		return err
	}
	if _, err := g.SpawnStack(item.SurvivalMeal, 10, cx+3, cy+1); err != nil {
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

// Apply queues a command for its target tick's command phase, in Seq order.
func (g *Game) Apply(c command.Command) { g.cmds.Push(c, uint64(g.clock.Now())) }

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

// Step advances the simulation by n ticks. Every open map runs its 15
// phases in id order (runPhases), the first map's pass also running the
// colony-wide work (primary); then map requests are applied, the viewed
// map is current again and the clock moves (MAP-0).
func (g *Game) Step(n int) {
	g.viewGen++ // view cache (indexes.go): a new state
	for _, m := range g.maps {
		m.configEdits = m.configEdits[:0]
	}
	for i := 0; i < n; i++ {
		for _, m := range g.maps {
			g.mapState = m
			g.runPhases()
		}
		g.mapState = g.maps[0]
		mapsFlush(g) // world_state.go: game/maps opens, enters and closes maps (MAP-01..04)
		g.mapState = g.view
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
	beds := g.rest.Beds.Bedded(g.pawnIDs)                 // rest/beds.go (BED-3)
	places := g.sk.bar.Places(barColonists(g), g.pawnIDs) // skills.go: game/roster (CTL-011)
	for i, id := range g.pawnIDs {
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
			BarOrder:      places[i],
			ID:            id,
			Map:           uint32(g.mapState.id),
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
			HasBed:        beds[id],
			Drafted:       g.draft[id],
			FireAtWill:    g.combat.Drafts.FireAtWill(id), HoldPosition: g.combat.Drafts.Holding(id), Grenades: g.combat.Drafts.Grenades(id), // DEF-4
		})
		ts := g.thoughts(id)
		pv := &views[len(views)-1]
		pv.Mood, pv.MoodLabel, pv.Thoughts = mood.Level(ts), mood.Label(mood.Level(ts)), thoughtViews(ts)
		g.fillSocialPawn(pv, pv.Mood) // social_view.go: smoothed mood, target, bubble (ENG-003)
		pv.Health, pv.Malnutrition, pv.FoodPolicy = v.HP()*100/health.MaxHP, int(body.Malnutrition(g.bodies[id]))/10, g.sk.diet.Of(id, false).Name
		pv.ToxicBuildup = int(body.ToxicBuildup(g.bodies[id])) / 10 // FOOD-6
		pv.Infection, pv.Wounded = g.woundView(id)                  // infection.go (MED-02)
		pv.InfectionLosing, pv.UntendedHours = g.infectionLosing(id), g.untendedHours(id)
		pv.Bleeding, pv.BleedDeathHours = g.bleedingNow(id), g.bleedDeathHours(id) // bleed.go (MED-01)
		cv := g.care.View(id, j)                                                   // care package: carry, tend, care level, bed rest
		pv.CareLevel = uint8(cv.CareLevel)                                         // MED-03
		pv.Disease = g.diseaseLabel(id)                                            // disease.go (MED-04)
		pv.OverdosePct, pv.OverdoseStage = doseOf(g).Overdose(id)                  // game/dose (MED-07)
		pv.Weapon, pv.WeaponStats = weaponDefOf(g, id).Name, weapon.Stats(weaponOf(g, id))
		gv := g.combat.Gear.View(id) // combat/gear.go (CBT-3)
		pv.WeaponQuality, pv.WeaponStuff, pv.Spare, pv.ShieldPct, pv.Smokepops, pv.Kills, pv.DamageDealt, pv.DamageTaken = gv.Quality, gv.Stuff, gv.Spare, gv.ShieldPct, gv.Smokepops, gv.Kills, gv.Damage, gv.Taken
		pv.ThreatResponse = g.combat.Drafts.Response(id).String()
		pv.Pack, pv.PackGrams, pv.PackLimitGrams = packView(g, id) // sense_gear_host.go (CSE-036)
		pv.Gear = outfitView(g, id, gearFacts(g, id))              // body.go: game/gearfacts (GEAR-1); apparel.go: game/outfit (GEAR-3)
		pv.Gear.NoDrop = senseOf(g).Gear.DropWhy(id, -1, 0)        // game/sense/drop.go (GEAR-1): -1 is sense.ModeAny
		pv.Lying = v.Status != health.Alive || (j.Kind == job.Sleep && (j.State == job.Do || j.State == job.Complete))
		pv.InBed = g.inBed(id)
		pv.CarriedBy, pv.CarryingPawn, pv.Tended, pv.OnMedicalBed = cv.CarriedBy, cv.Carrying, cv.Tended, cv.OnMedicalBed
		pv.Hypothermia, pv.Heatstroke, pv.Frostbite, pv.ExposureNote = exposureNoted(g, id, v.Status == health.Downed) // heat.go (TEMP-3, AIL-004)
		pv.Temperament = g.temperamentOf(id).String()
		pr := g.prioOf(id)
		for _, v := range pr {
			pv.Priorities = append(pv.Priorities, int(v))
		}
		sg := g.scheduleOf(id)
		pv.Schedule = append([]schedule.Activity(nil), sg[:]...)
		pv.Activity = g.activityOf(id)
		pv.Apparel = g.Wearing(id)
		pv.ApparelArmor = g.apparel[id].Armor()                                  // craft/garment.go: in Wearing's order
		pv.ApparelForced, pv.ChangingInto = g.wornForced(id), g.changingInto(id) // apparel.go
		pv.Worn = g.apparel[id].Views()                                          // craft/garment.go (PRD-2)
		if in, ok := g.mind.Inspire.Of(id); ok {                                 // mind/inspiration.go (PRD-2)
			pv.Inspiration, pv.InspirationTicks = in.Kind.String(), in.Until-uint64(g.clock.Now())
		}
		pv.NoRecreation = !recreationEligible(g, id)
		if !pv.NoRecreation {
			pv.Recreation, pv.AX.Joy, pv.AX.Tolerance = g.joy[id]*100/recreation.Full, recView(g).Joy(id), recView(g).Tolerances(id)
		} // AX-6
		pv.Comfort, pv.Beauty, pv.Outdoors = g.rest.Needs.Percents(id) // rest/needs.go (BED-3)
		if b, ok := g.breaks[id]; ok {
			pv.MentalBreak = b.kind.String()
		}
		pv.MentalTarget = g.mind.Target(id)
		g.fillFleeView(pv, id)                             // an unarmed colonist running from a threat (flee.go)
		if pv.RestWhenHurt = cv.RestWhenHurt; cv.BedRest { // PLAY-020: resting in bed
			pv.BedRest, pv.Job = true, care.RestingLabel
		}
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
// repeated calls between ticks share one result, and across ticks the
// entity lists (Items, Frames, Buildings, Plants), Zones and the small
// lists keep their slice while they read the same, so presentation caches
// keyed on a slice's identity hold (PERF-002): treat it all as read only.
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
		PluginPanels: pluginPanelViews(g.scripts.panels), // scripts.go: hourly
		Biome:        bd.Name, BiomeKey: bd.Key,
		OutdoorTemp: int(g.climate.Outdoor(g.clock.Now())),
		IndoorTemp:  int(g.climate.Indoor(g.clock.Now())),
		Season:      g.season(uint64(g.clock.Now())),
		Messages:    g.messageViews(),
		Letters:     g.sharedLetters(), // letters.go (PLAY-018)
		Quests:      questViews(g),
		NoGoodwill:  questOf(g).NoGoodwill(),
		Zones:       g.zoneViews(),
		Schedule:    g.scheduleView(),
	}
	if g.worldSt != nil {
		now := uint64(g.clock.Now())
		for _, gr := range g.worldSt.visitGroups {
			w.VisitorGroups = append(w.VisitorGroups, gr.View(now))
		}
	}
	w.Research, w.Locked = g.lab.Lists(g.views.on) // game/research
	w.Gameplay = senseOf(g).Rules                  // game/sense (CSE)
	w.AX.DeepOreVer, w.AX.DeepOre = g.br.ind.DeepOre(g.grid.Width())
	shipOf(g).FillWorld(shipHost{g}, &w.AX) // story.go: game/ship (AX-8)
	w.AX.EveryoneGone = everyoneGone(g)
	travelOf(g).World(&w.AX) // world_stash.go: game/travel (AX-9): pods in flight, mineral sites
	w.ResearchAlert, w.TechLevel = g.lab.Alert(), researchHost{g}.Tech().String()
	if n, felled := g.Timbergnaws(); n > 0 {
		w.TreesFelled = felled
	}
	// The entity lists (item_view.go, frame_view.go, building_view.go,
	// plant_view.go): with the view cache on, each keeps its last slice
	// while it reads the same (PERF-002).
	g.fillItemViews(&w)
	fillDietView(g, &w)     // food.go (FOOD-4): food policies, days of food
	w.Gear = outfitWorld(g) // apparel.go: game/outfit (GEAR-3)
	w.SkyEvent, w.SkyEventEnds = g.incidents.SkyView(uint64(g.clock.Now()))
	w.SkySongEnds = g.incidents.SongView(uint64(g.clock.Now()))
	w.ClimateEvent, w.ClimateEventEnds = g.incidents.ClimateView(uint64(g.clock.Now())) // TEMP-2 (incident/climate.go)
	w.Extreme = w.ClimateEvent
	w.Frames = g.frameViews()
	w.Buildings = g.buildingViews()
	w.Plants = g.plantViews()
	w.Landing[0], w.Landing[1] = g.landingSpot()
	w.TerrainW = g.terr.W
	w.Terrain = g.terrainView() // indexes.go: shared, rebuilt per structVer
	w.Mines = g.mineList()
	w.Smooths = g.smoothList()
	w.StoneType, w.Ore = g.stoneLayers()
	w.StructVer = g.structVer
	g.ensureRooms()
	w.RoomGrid = g.roomGrid                            // shared read-only: ensureRooms replaces it, never writes in place
	w.Rooms, w.TempVer = g.roomViews(), g.thermo.Ver() // game/thermo (TEMP-1)
	w.Animals = g.animalViews()
	w.Traces, w.Shots, w.Smoke = g.traceViews(), g.combat.Shots.Views(), g.combat.Blasts.Views()
	g.fillWorldWeather(&w) // weather.go
	g.fillRoofView(&w)     // roof.go
	g.fillWildlifeView(&w) // wildlife.go
	g.fillGrassView(&w)    // grass.go
	g.fillStainView(&w)    // stains.go
	g.fillDirtView(&w)     // filth.go
	g.fillHomeView(&w)     // home.go
	g.fillSnowView(&w)     // game/snow (PLAY-015)
	g.fillPenView(&w)      // pens.go
	g.fillPlanView(&w)     // plan.go
	g.fillSetupView(&w)    // setup_view.go (NG-1)
	fillRaidView(g, &w)    // raids.go (DEF-4): combat log, hostile drop pods
	g.fillFogView(&w)      // fog_view.go (MAPGEN-2)
	g.fillFireView(&w)     // fire.go (PLAY-015)
	g.fillPowerView(&w)    // power_view.go (PLAY-021)
	now := uint64(g.clock.Now())
	w.Hour = float64(now%clock.TicksPerDay) / clock.TicksPerHour
	w.DayOfYear = int(now/clock.TicksPerDay) % g.clock.Calendar().YearDays()
	return w
}

func lastMealName(g *Game, id entity.EntityID) string {
	if k, ok := g.lastMeal[id]; ok {
		return k.String()
	}
	return ""
}

func footprintView(k construction.Kind, f construction.Facing, x, y int) (w, h, sx, sy int, has bool) {
	return construction.FootprintView(k, f, x, y)
}
