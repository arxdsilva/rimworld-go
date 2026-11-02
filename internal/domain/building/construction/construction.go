// Package construction is buildable definitions, frames (blueprints that
// accept wood then work) and completed buildings.
package construction

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// Kind is a buildable structure.
type Kind uint8

const (
	Wall Kind = iota
	Bed
	Door
	Floor
	Table
	Chair
	Lamp
	// SleepingSpot is a bed-sized (1x2, 2x1 on its side) marking on the
	// floor: free, placed instantly, slept on like the ground. The Bed is
	// the built upgrade with the same footprint.
	SleepingSpot
	Workbench
	Stove
	ButcherTable
	// DoubleBed is a 2x2 bed for two sleepers side by side.
	DoubleBed
	// DryingRack is a 2x1 wooden frame food is laid out on to dry.
	DryingRack
	// Campfire burns wood to heat (and light) its room.
	Campfire
	// TailoringBench is a 3x1 table where clothes are sewn.
	TailoringBench
	// HorseshoePin is a stake in a patch of sand for pitching horseshoes.
	HorseshoePin
	// Sandbags are a waist-high line of sacks: walkable, and cover for
	// anyone crouched behind them.
	Sandbags
	// SpikeTrap is a hidden frame of sharpened stakes that springs on
	// hostiles walking over it, then has to be rebuilt.
	SpikeTrap
	// ResearchBench is the simple research bench, a 3x2 desk where
	// colonists research (RSCH-1).
	ResearchBench
	// Grave is a dug 1x2 plot that holds one body under a headstone;
	// Pyre is a 2x1 log stack where bodies are cremated (SIM-017).
	Grave
	Pyre
	// Carpet is a walkable 1x1 entrance carpet installed from a crafted carpet
	// item (its stuff): it wipes tracked dirt off boots (SIM-021).
	Carpet
	// FeedingTrough holds feed for the tame animals of its pen (SIM-024);
	// haulers fill it, animals eat at its spot.
	FeedingTrough
	// Fence keeps animals out (or in); colonists climb over it. A
	// FenceGate lets colonists and tame animals through (SIM-028).
	Fence
	FenceGate
	// StonecutterTable cuts stone chunks into blocks (SIM-029).
	StonecutterTable
	// ToolCabinet is a steel cabinet of tools that speeds nearby benches;
	// HospitalBed is a bed for patients (NG-1 §4.8). Both need components.
	ToolCabinet
	HospitalBed
	// PodShell is the empty shell of an arrival pod: placed by the arrival,
	// never designated, walkable, and salvaged for steel.
	PodShell
	// Social and host systems (ENG-003, append only, saved): a brewing
	// bench and a drug table (3x1 benches), and two free floor markings
	// placed instantly like a sleeping spot: a 2x1 marriage spot where
	// weddings are held and a 1x1 party spot where parties gather.
	BrewingBench
	DrugTable
	MarriageSpot
	PartySpot
	// SteamVent is a natural 2x2 geothermal vent left by map generation
	// (MAPGEN-2): walkable, never built, deconstructed or moved; it warms
	// the air around it.
	SteamVent
	// Bridge (MAPGEN-3) is a wooden foundation over water, marsh or mud: it
	// makes the cell walkable and holds light things up. SleeperCasket is
	// a sealed vault's 1x2 casket: placed by map generation, opened, then
	// salvaged for steel.
	Bridge
	SleeperCasket
	// PenMarker (PLAY-015) is a small signpost: the enclosed region around
	// it (fences, gates, walls, rock) is an animal pen with its settings.
	PenMarker
	// Electricity (PLAY-021, append only, saved). PowerConduit and
	// HiddenConduit carry power between buildings (LayerConduit, under
	// walls and benches; a hidden one is buried and only shows in the power
	// overlay). PowerSwitch conducts while closed. WoodGenerator,
	// SolarPanel, WindTurbine and GeothermalGenerator (over a steam vent)
	// make power; a Battery stores it. TorchLamp is the wood-burning light
	// before electricity; Heater and Cooler (set in a wall gap, facing its
	// cold side) hold a room at a thermostat target. The electric benches
	// work their base kind's recipes (RecipeBench) faster, with power.
	PowerConduit
	HiddenConduit
	PowerSwitch
	WoodGenerator
	SolarPanel
	WindTurbine
	GeothermalGenerator
	Battery
	TorchLamp
	Heater
	Cooler
	ElectricStove
	ElectricTailoringBench
	// ElectricResearchBench is the hi-tech research bench (RSCH-0: the
	// kind and its Name stay, being the save, art, power and unlock key;
	// RSCH-1 shows it as "hi-tech research bench").
	ElectricResearchBench
	// VitalsMonitor (MED-06) stands beside a hospital bed: while powered it
	// raises tend quality, surgery success and immunity gain for the
	// patient in that bed.
	VitalsMonitor
	// Defence and trade (PLAY-024, append only, saved). SentryTurret is a
	// powered 1x1 gun post that shoots hostiles in range; TradeBeacon is
	// an unpowered post that marks the goods an orbital trader can reach;
	// RadioConsole is a powered 3x2 desk a colonist works to call
	// factions and ships.
	SentryTurret
	TradeBeacon
	RadioConsole
	// Beds and bed furniture (BED-1, append only, saved). Bedroll and
	// DoubleBedroll are light sleeping places sewn from cloth or leather;
	// AnimalSleepingSpot and AnimalBed are where tame animals sleep;
	// EndTable and Dresser are facilities that make a nearby bed more
	// comfortable (beds.go).
	Bedroll
	DoubleBedroll
	AnimalSleepingSpot
	AnimalBed
	EndTable
	Dresser
	// Wave D contract (append only, saved). None is offered in the
	// Architect until its feature agent adds the tool row
	// (ebiten/architect/catalog.go) and the art.
	//
	// PRD-3 benches: two free 1x1 spots (crafting, butchering), the
	// forges, machining and fabrication benches, a sculpting bench, a
	// smelter, a fuel refinery and a fermenting barrel (not a bench: it
	// holds a batch while it ferments). Sculptures are carved at the
	// sculpting bench and set down where the player wants them.
	CraftingSpot
	FueledSmithy
	ElectricSmithy
	MachiningTable
	FabricationBench
	ArtBench
	ElectricSmelter
	BiofuelRefinery
	ButcherSpot
	FermentingBarrel
	SmallSculpture
	LargeSculpture
	GrandSculpture
	// BED-3 seats: a plain stool and a padded armchair (Chair is the
	// dining chair).
	Stool
	Armchair
	// DEF-3 defences: a barricade, four buried charges (blast, fire,
	// pulse, smoke), two heavy turrets, a manned mortar and a powered door
	// (SentryTurret is the light turret).
	Barricade
	IEDHighExplosive
	IEDIncendiary
	IEDEMP
	IEDSmoke
	Autocannon
	UraniumSlugTurret
	Mortar
	Autodoor
	// Wave E contract (append only, saved; hidden from the Architect until
	// PRD-4 adds the tool row and the art): a shelf, storage furniture
	// that holds a few stacks per cell and keeps them from weathering.
	Shelf
	// ShipPart (DEF-4) is a crashed ship part: never built by the colony
	// (no Architect row), it lands with the CrashedShipPart threat, its
	// dormant mechanoid guards beside it (game/raid/shippart.go).
	ShipPart
	// STO/CBT contract (append only, saved; hidden from the Architect until
	// STO-2 adds the tool rows and the art): a hopper that holds one stack
	// of raw food for the nutrient paste dispenser beside it, and a
	// sarcophagus, a grave above ground that keeps a body in state.
	Hopper
	NutrientPasteDispenser
	Sarcophagus
	// HiveMound (CBT-4, append only, saved) is an insect hive: never built
	// by the colony (no Architect row), it rises in a dark, warm, roofed
	// cave with an infestation, spawns insects and spreads
	// (game/raid/nest.go).
	HiveMound
	// FOOD-0 contract (append only, saved; hidden from the Architect until
	// FOOD-2 adds the tool rows, the power draw and the art): a
	// hydroponics basin, a powered bed of very fertile ground for crops,
	// and a sun lamp, a powered lamp bright enough to grow crops by.
	HydroponicsBasin
	SunLamp
	// TEMP-0 contract (append only, saved; hidden from the Architect until
	// TEMP-1 adds the tool rows, the fuel, the heat and the art): a
	// passive cooler, a wood-fed basin of evaporating water that cools
	// its room, and a vent, a steel louvre set in a wall gap like the
	// cooler (WallMounted, so it bounds rooms) that lets air through
	// while open (BuildingView.Open, command.ToggleVent).
	PassiveCooler
	Vent
	// RSCH-0 contract (append only, saved; hidden from the Architect
	// until RSCH-1 adds the tool row, the power draw, the link and the
	// art): a multi-analyzer, a powered facility that a hi-tech research
	// bench (ElectricResearchBench) links to for the hardest projects.
	MultiAnalyzer
	// AX-0 contract (append only, saved). Every vanilla Architect item the
	// game lacked (.cursor/ARCHITECT_GAP_ANALYSIS.md), each owned by one AX
	// agent (.cursor/handoffs/AX-0-contract.md). Planned() is true for
	// every one of them until its owner makes it real (ax_<owner>.go), so
	// none can be designated (placement.go placeFrameStuff) and none has
	// an Architect tool, art or behaviour yet.
	//
	// A2 structure, furniture and decor: a 2-wide ornate door, a column
	// (holds a roof, bounds no room), an animal flap, a double sleeping
	// spot, an animal sleeping box, the bigger tables (Table becomes the
	// 1x2), a couch, a plant pot, a small shelf, the wall-mounted torch and
	// lamp, a flood light, an urn (never built: traded or found), the two
	// steles and an egg box.
	OrnateDoor
	Column
	AnimalFlap
	DoubleSleepingSpot
	AnimalSleepingBox
	Table2x2
	Table2x4
	Table3x3
	Couch
	PlantPot
	SmallShelf
	WallTorch
	WallLamp
	FloodLight
	Urn
	LargeStele
	GrandStele
	EggBox
	// A3 floors (LayerFloor): straw matting, concrete, paved tile, steel,
	// silver, gold and sterile tile, stone tile and flagstone (one kind
	// each, the stone is the stuff) and the colour-picked carpet floor
	// (Building.Tint is the colour; Carpet stays the entrance carpet).
	StrawMatting
	Concrete
	PavedTile
	SteelTile
	SilverTile
	GoldTile
	SterileTile
	StoneTile
	Flagstone
	CarpetFloor
	// A4 power and industry: a conduit laid over shallow water, a chemfuel
	// generator, a watermill on a river, a deep drill and the
	// ground-penetrating scanner that finds its ore, a moisture pump and
	// an electric crematorium.
	WaterproofConduit
	ChemfuelGenerator
	Watermill
	DeepDrill
	GroundScanner
	MoisturePump
	ElectricCrematorium
	// A5 fire foam and security: a foam popper, a foam charge, a foam
	// turret, a warhead charge (an antimatter warhead) and a rocket
	// battery.
	FoamPopper
	IEDFoam
	FoamTurret
	IEDWarhead
	RocketBattery
	// A6 recreation: a ring toss, an ur board, chess, billiards and poker
	// tables, three televisions (the big screen is never built: traded
	// or found) and a telescope (traded or found).
	RingToss
	UrBoard
	ChessTable
	BilliardsTable
	PokerTable
	TubeTV
	FlatscreenTV
	BigScreenTV
	Telescope
	// A7 books: a bookcase and a small bookcase.
	Bookcase
	SmallBookcase
	// A8 the escape ship: structural beam, stasis casket, computer core,
	// reactor, engine and sensor cluster.
	ShipBeam
	ShipCasket
	ShipCore
	ShipReactor
	ShipEngine
	ShipSensor
	// A9 travel: a stasis casket, a transport pod and its launcher, a
	// long-range mineral scanner and a caravan hitching spot.
	StasisCasket
	TransportPod
	PodLauncher
	MineralScanner
	HitchingSpot
	kindCount
)

// TroughCapacity is how many food items a feeding trough holds.
const TroughCapacity = 60

// Layer separates what can share a tile: one floor plus one object.
type Layer uint8

const (
	LayerObject Layer = iota
	LayerFloor
	// LayerFoundation (MAPGEN-3) is under everything: a bridge over water,
	// marsh or mud. Append only: saved.
	LayerFoundation
	// LayerConduit (PLAY-021) holds power conduits, so they can run under
	// walls, doors and benches. Append only: saved.
	LayerConduit
)

// NumLayers is how many layers there are (every Layer is below it).
const NumLayers = int(LayerConduit) + 1

// Def is the static definition of a buildable kind.
type Def struct {
	Kind       Kind
	Name       string
	Wood       int // delivered wood required before work starts
	Work       int // construct ticks after materials are in place
	Layer      Layer
	BlocksMove bool // completed building makes its tile unwalkable
	Rotatable  bool
	StoneOK    bool // may be built from any stone blocks instead of wood
	MetalOK    bool // may be built from steel
	// MetalOnly: steel is the only material (wood is refused); Wood is
	// then the steel needed.
	MetalOnly bool
	// Components are delivered after the material, before work starts.
	Components int
	// Placed kinds are put down by the game (a drop pod's shell), never
	// designated from the architect menu.
	Placed bool
	// Salvage is what deconstructing a Placed kind returns (of its stuff).
	Salvage int
	// Size is the footprint (w, h) when facing south; zero means 1x1.
	Size [2]int
	// Spot is the interaction cell relative to the footprint's top-left
	// when facing south (e.g. where a cook stands); HasSpot gates it.
	Spot    [2]int
	HasSpot bool
	Bench   bool // a workstation that runs bills
	// Dug: made by digging, so it needs work but no materials (a grave).
	Dug bool
	// Carpet: built from a crafted carpet item (Stuffs) rather than wood.
	Carpet bool
	// AnimalBlock: animals can't cross it (fences); Gate lets tame ones
	// through (SIM-028).
	AnimalBlock, Gate bool
	// Natural: a map feature (with Placed), never deconstructed, moved or
	// swapped.
	Natural bool
	// Foundation (MAPGEN-3): built over bridgeable terrain only (water,
	// marsh, mud), in LayerFoundation, holding up light things.
	Foundation bool
	// Fabric (BED-1): sewn from cloth or leather only (Wood is then the
	// fabric needed).
	Fabric bool
	// AX-0 contract. StoneOnly: stone blocks are the only material (Wood is
	// then the blocks needed). Only: the one material it is made of (0:
	// the flags decide), Wood being the units needed (hay matting, silver
	// tile, foam shells for a foam charge). Extra are secondary materials
	// delivered with the main one (gold in an ornate door, plasteel in a
	// ship part); nothing reads them until their owner wires delivery.
	// Unbuildable: never designated as a blueprint (an urn, the big screen,
	// the telescope: traded or found, they arrive crated and are only
	// reinstalled, A1).
	StoneOnly, Unbuildable bool
	Only                   item.Kind
	Extra                  [3]Cost
}

// Cost is N units of item Item (a Def's secondary material, AX-0).
type Cost struct {
	Item item.Kind
	N    int
}

// StoneWorkPct is the work multiplier (percent) for granite, the legacy
// "stone" material; StuffStats has every material.
const StoneWorkPct = 160

// StuffStats is a material's HP and construct work multipliers (percent).
// Original numbers: granite keeps the legacy stone values.
func StuffStats(s item.Kind) (hpPct, workPct int) {
	switch s {
	case item.GraniteBlocks, item.Stone:
		return 200, StoneWorkPct
	case item.SandstoneBlocks:
		return 150, 125
	case item.LimestoneBlocks:
		return 170, 140
	case item.SlateBlocks:
		return 160, 135
	case item.MarbleBlocks:
		return 140, 150
	case item.Steel:
		return 140, 90
	}
	return 100, 100
}

var defs = [kindCount]Def{
	Wall:         {Kind: Wall, Name: "wall", Wood: 2, Work: 40, Layer: LayerObject, BlocksMove: true, StoneOK: true, MetalOK: true},
	Bed:          {Kind: Bed, Name: "bed", Wood: 5, Work: 90, Layer: LayerObject, Rotatable: true, Size: [2]int{1, 2}, MetalOK: true},
	Door:         {Kind: Door, Name: "door", Wood: 2, Work: 50, Layer: LayerObject, MetalOK: true},
	Floor:        {Kind: Floor, Name: "floor", Wood: 1, Work: 20, Layer: LayerFloor, StoneOK: true},
	Table:        {Kind: Table, Name: "table", Wood: 3, Work: 50, Layer: LayerObject, MetalOK: true},
	Chair:        {Kind: Chair, Name: "chair", Wood: 2, Work: 40, Layer: LayerObject, Rotatable: true, MetalOK: true},
	Lamp:         {Kind: Lamp, Name: "lamp", Wood: 2, Work: 40, Layer: LayerObject},
	SleepingSpot: {Kind: SleepingSpot, Name: "sleeping spot", Layer: LayerObject, Rotatable: true, Size: [2]int{1, 2}},
	// STK-BN, wiki: every production bench below is 3x1 (Workbench, Stove,
	// Butcher table, ...): the worker stands south of the middle cell
	// (Spot {1, 1}), and Footprint turns both with the facing.
	Workbench: {Kind: Workbench, Name: "workbench", Wood: 6, Work: 160, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true},
	Stove: {Kind: Stove, Name: "stove", Wood: 5, Work: 140, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true},
	DoubleBed: {Kind: DoubleBed, Name: "double bed", Wood: 9, Work: 150, Layer: LayerObject, Rotatable: true,
		Size: [2]int{2, 2}, MetalOK: true},
	ButcherTable: {Kind: ButcherTable, Name: "butcher table", Wood: 5, Work: 120, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true},
	HorseshoePin: {Kind: HorseshoePin, Name: "horseshoe pin", Wood: 3, Work: 50, Layer: LayerObject},
	TailoringBench: {Kind: TailoringBench, Name: "tailoring bench", Wood: 7, Work: 150, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true},
	Sandbags:  {Kind: Sandbags, Name: "sandbags", Wood: 2, Work: 60, Layer: LayerObject, StoneOK: true},
	SpikeTrap: {Kind: SpikeTrap, Name: "spike trap", Wood: 4, Work: 80, Layer: LayerObject},
	// RSCH-1, wiki: Simple research bench (3x2, 75 stuff + 25 steel, 2800
	// work). Scaled as the wave D rows (stuff / 9, work / 10); the steel is
	// a secondary material, not modelled yet. The researcher sits at the
	// middle of the south edge.
	ResearchBench: {Kind: ResearchBench, Name: "research bench", Wood: 8, Work: 280, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 2}, Spot: [2]int{1, 2}, HasSpot: true, StoneOK: true, MetalOK: true},
	// A campfire is also a (slow) cooking spot; the cook stands south of it.
	Campfire: {Kind: Campfire, Name: "campfire", Wood: 5, Work: 60, Layer: LayerObject, BlocksMove: true,
		Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	DryingRack: {Kind: DryingRack, Name: "drying rack", Wood: 6, Work: 100, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{2, 1}, Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	// A grave is dug, not built: no materials, just a long work.
	Grave: {Kind: Grave, Name: "grave", Work: 400, Layer: LayerObject, Rotatable: true, Size: [2]int{1, 2}, Dug: true},
	Pyre: {Kind: Pyre, Name: "funeral pyre", Wood: 6, Work: 120, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 1}, Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	// A carpet takes one crafted carpet item (Wood counts items, not wood).
	Carpet: {Kind: Carpet, Name: "carpet", Wood: 1, Work: 30, Layer: LayerObject, Carpet: true},
	FeedingTrough: {Kind: FeedingTrough, Name: "feeding trough", Wood: 4, Work: 60, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Spot: [2]int{0, 1}, HasSpot: true},
	Fence:     {Kind: Fence, Name: "fence", Wood: 1, Work: 20, Layer: LayerObject, StoneOK: true, AnimalBlock: true},
	FenceGate: {Kind: FenceGate, Name: "fence gate", Wood: 3, Work: 40, Layer: LayerObject, Rotatable: true, AnimalBlock: true, Gate: true},
	StonecutterTable: {Kind: StonecutterTable, Name: "stonecutter table", Wood: 6, Work: 160, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true},
	ToolCabinet: {Kind: ToolCabinet, Name: "tool cabinet", Wood: 30, Work: 120, Layer: LayerObject, BlocksMove: true,
		MetalOK: true, MetalOnly: true, Components: 1},
	HospitalBed: {Kind: HospitalBed, Name: "hospital bed", Wood: 45, Work: 200, Layer: LayerObject, Rotatable: true,
		Size: [2]int{1, 2}, MetalOK: true, Components: 2},
	PodShell: {Kind: PodShell, Name: "pod shell", Layer: LayerObject, MetalOK: true, MetalOnly: true, Placed: true, Salvage: 6},
	BrewingBench: {Kind: BrewingBench, Name: "brewing bench", Wood: 8, Work: 150, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true},
	DrugTable: {Kind: DrugTable, Name: "drug table", Wood: 8, Work: 180, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true},
	MarriageSpot: {Kind: MarriageSpot, Name: "marriage spot", Layer: LayerObject, Rotatable: true, Size: [2]int{2, 1}},
	PartySpot:    {Kind: PartySpot, Name: "party spot", Layer: LayerObject},
	SteamVent: {Kind: SteamVent, Name: "steam vent", Layer: LayerFoundation, Placed: true, Natural: true,
		Size: [2]int{2, 2}},
	Bridge: {Kind: Bridge, Name: "bridge", Wood: 6, Work: 110, Layer: LayerFoundation, Foundation: true},
	SleeperCasket: {Kind: SleeperCasket, Name: "sleeper casket", Layer: LayerObject, BlocksMove: true, Rotatable: true,
		Size: [2]int{1, 2}, MetalOK: true, MetalOnly: true, Placed: true, Salvage: 20},
	PenMarker: {Kind: PenMarker, Name: "pen marker", Wood: 10, Work: 40, Layer: LayerObject},
	// Electricity (PLAY-021, our costs). Every electric piece is steel
	// (MetalOnly: Wood is the steel) and most need components; the torch
	// lamp is wood.
	PowerConduit: {Kind: PowerConduit, Name: "power conduit", Wood: 1, Work: 20, Layer: LayerConduit,
		MetalOK: true, MetalOnly: true},
	HiddenConduit: {Kind: HiddenConduit, Name: "hidden conduit", Wood: 3, Work: 60, Layer: LayerConduit,
		MetalOK: true, MetalOnly: true},
	PowerSwitch: {Kind: PowerSwitch, Name: "power switch", Wood: 15, Work: 60, Layer: LayerObject,
		MetalOK: true, MetalOnly: true, Components: 1},
	WoodGenerator: {Kind: WoodGenerator, Name: "wood-fired generator", Wood: 90, Work: 320, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 2}, MetalOK: true, MetalOnly: true, Components: 2},
	SolarPanel: {Kind: SolarPanel, Name: "solar panel", Wood: 110, Work: 420, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{3, 3}, MetalOK: true, MetalOnly: true, Components: 3},
	WindTurbine: {Kind: WindTurbine, Name: "wind turbine", Wood: 95, Work: 380, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{1, 2}, MetalOK: true, MetalOnly: true, Components: 2},
	GeothermalGenerator: {Kind: GeothermalGenerator, Name: "geothermal generator", Wood: 170, Work: 640, Layer: LayerObject,
		BlocksMove: true, Size: [2]int{2, 2}, MetalOK: true, MetalOnly: true, Components: 4},
	Battery: {Kind: Battery, Name: "battery", Wood: 55, Work: 220, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{1, 2}, MetalOK: true, MetalOnly: true, Components: 2},
	TorchLamp: {Kind: TorchLamp, Name: "torch lamp", Wood: 3, Work: 30, Layer: LayerObject},
	// TEMP-1, wiki: Heater (50 steel + 1 component), Cooler (90 steel + 3
	// components), scaled as the wave D rows (material / 9); work ours.
	Heater: {Kind: Heater, Name: "heater", Wood: 6, Work: 110, Layer: LayerObject, BlocksMove: true,
		MetalOK: true, MetalOnly: true, Components: 1},
	// A cooler stands in a wall gap like a door; it faces its cold side.
	Cooler: {Kind: Cooler, Name: "cooler", Wood: 10, Work: 130, Layer: LayerObject, BlocksMove: true, Rotatable: true,
		MetalOK: true, MetalOnly: true, Components: 3},
	ElectricStove: {Kind: ElectricStove, Name: "electric stove", Wood: 45, Work: 190, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true,
		MetalOK: true, MetalOnly: true, Components: 2},
	ElectricTailoringBench: {Kind: ElectricTailoringBench, Name: "electric tailoring bench", Wood: 55, Work: 210, Layer: LayerObject,
		BlocksMove: true, Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true,
		MetalOK: true, MetalOnly: true, Components: 2},
	// RSCH-1, wiki: Hi-tech research bench (5x2, 150 metal + 100 steel +
	// 10 components, 5000 work; 250 W in power.go). Scaled as the wave D
	// rows; the 100 steel is a secondary material, not modelled yet.
	ElectricResearchBench: {Kind: ElectricResearchBench, Name: "electric research bench", Wood: 17, Work: 500, Layer: LayerObject,
		BlocksMove: true, Rotatable: true, Size: [2]int{5, 2}, Spot: [2]int{2, 2}, HasSpot: true,
		MetalOK: true, MetalOnly: true, Components: 10},
	// wiki: Vitals monitor (6000 work against the hospital bed's 4000):
	// 1.5 times our hospital bed's work. The steel is original tuning.
	VitalsMonitor: {Kind: VitalsMonitor, Name: "vitals monitor", Wood: 40, Work: 300, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, MetalOK: true, MetalOnly: true, Components: 1},
	// PLAY-024 (our costs, PLAY024_PLAN.md): steel pieces (Wood is the
	// steel). The console's operator stands at its spot, south of the desk's
	// middle (STK-BN, wiki: Comms console, 3x2).
	SentryTurret: {Kind: SentryTurret, Name: "sentry turret", Wood: 90, Work: 400, Layer: LayerObject, BlocksMove: true,
		MetalOK: true, MetalOnly: true, Components: 3},
	TradeBeacon: {Kind: TradeBeacon, Name: "trade beacon", Wood: 40, Work: 120, Layer: LayerObject,
		MetalOK: true, MetalOnly: true},
	RadioConsole: {Kind: RadioConsole, Name: "radio console", Wood: 80, Work: 300, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 2}, Spot: [2]int{1, 2}, HasSpot: true,
		MetalOK: true, MetalOnly: true, Components: 3},
	// BED-1, wiki: Bedroll, Double bedroll, Animal sleeping spot, Animal
	// bed, End table, Dresser. Costs and work are the wiki's scaled the
	// way the bed's are (cost / 9, work / 10); footprints as in the wiki.
	Bedroll: {Kind: Bedroll, Name: "bedroll", Wood: 4, Work: 60, Layer: LayerObject, Rotatable: true,
		Size: [2]int{1, 2}, Fabric: true},
	DoubleBedroll: {Kind: DoubleBedroll, Name: "double bedroll", Wood: 9, Work: 110, Layer: LayerObject, Rotatable: true,
		Size: [2]int{2, 2}, Fabric: true},
	AnimalSleepingSpot: {Kind: AnimalSleepingSpot, Name: "animal sleeping spot", Layer: LayerObject},
	AnimalBed:          {Kind: AnimalBed, Name: "animal bed", Wood: 4, Work: 40, Layer: LayerObject, Fabric: true},
	EndTable:           {Kind: EndTable, Name: "end table", Wood: 3, Work: 100, Layer: LayerObject, StoneOK: true, MetalOK: true},
	Dresser: {Kind: Dresser, Name: "dresser", Wood: 6, Work: 200, Layer: LayerObject, Rotatable: true,
		Size: [2]int{2, 1}, StoneOK: true, MetalOK: true},
	// Wave D rows (ENG-003). Costs and work are the wiki's scaled the way
	// BED-1 scales them (material / 9 rounded, work / 10); components as
	// the wiki. Footprints are the wiki's (STK-BN: 3x1 production benches,
	// a 5x2 fabrication bench, a 3x2 fuel refinery; the worker stands in
	// front of the middle). Secondary materials (an art bench's
	// steel, a fabrication bench's advanced components, a mortar's
	// barrel) are not modelled yet.
	// PRD-3, wiki: Crafting spot, Butcher spot, Fueled smithy, Electric
	// smithy, Machining table, Fabrication bench, Art bench, Electric
	// smelter, Biofuel refinery, Fermenting barrel.
	CraftingSpot: {Kind: CraftingSpot, Name: "crafting spot", Layer: LayerObject, Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	ButcherSpot:  {Kind: ButcherSpot, Name: "butchering spot", Layer: LayerObject, Spot: [2]int{0, 1}, HasSpot: true, Bench: true},
	FueledSmithy: {Kind: FueledSmithy, Name: "wood-fired forge", Wood: 11, Work: 300, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true, MetalOK: true, MetalOnly: true},
	// wiki: no build work for the electric smithy; the fueled smithy's.
	ElectricSmithy: {Kind: ElectricSmithy, Name: "electric forge", Wood: 11, Work: 300, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true,
		MetalOK: true, MetalOnly: true, Components: 3},
	MachiningTable: {Kind: MachiningTable, Name: "machining table", Wood: 17, Work: 300, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true,
		MetalOK: true, MetalOnly: true, Components: 5},
	FabricationBench: {Kind: FabricationBench, Name: "fabrication bench", Wood: 22, Work: 500, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{5, 2}, Spot: [2]int{2, 2}, HasSpot: true, Bench: true,
		MetalOK: true, MetalOnly: true, Components: 12},
	ArtBench: {Kind: ArtBench, Name: "sculpting bench", Wood: 8, Work: 250, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true, StoneOK: true, MetalOK: true},
	ElectricSmelter: {Kind: ElectricSmelter, Name: "electric smelter", Wood: 19, Work: 350, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 1}, Spot: [2]int{1, 1}, HasSpot: true, Bench: true,
		MetalOK: true, MetalOnly: true, Components: 2},
	BiofuelRefinery: {Kind: BiofuelRefinery, Name: "fuel refinery", Wood: 17, Work: 200, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 2}, Spot: [2]int{1, 2}, HasSpot: true, Bench: true,
		MetalOK: true, MetalOnly: true, Components: 3},
	// 30 wood (+10 steel, not modelled); the wiki gives no work: ours.
	FermentingBarrel: {Kind: FermentingBarrel, Name: "fermenting barrel", Wood: 3, Work: 60, Layer: LayerObject, BlocksMove: true,
		Spot: [2]int{0, 1}, HasSpot: true, Bench: true}, // PRD-4: its fill bill (craft/ferment.go)
	// Sculptures are carved by a sculpting bench bill (recipe.SculptSmall
	// and on: the bill takes Wood units of the chosen material and Work);
	// these rows make the placed piece a valid kind. wiki: Sculpture
	// (50/100/400 stuff, 18000/30000/105000 work, 1x1/1x1/2x2).
	SmallSculpture: {Kind: SmallSculpture, Name: "small sculpture", Wood: 6, Work: 1800, Layer: LayerObject, BlocksMove: true,
		StoneOK: true, MetalOK: true},
	LargeSculpture: {Kind: LargeSculpture, Name: "large sculpture", Wood: 11, Work: 3000, Layer: LayerObject, BlocksMove: true,
		StoneOK: true, MetalOK: true},
	GrandSculpture: {Kind: GrandSculpture, Name: "grand sculpture", Wood: 44, Work: 10500, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 2}, StoneOK: true, MetalOK: true},
	// BED-3, wiki: Stool (25 stuff, 450 work), Armchair (110 fabric,
	// 14000 work).
	Stool: {Kind: Stool, Name: "stool", Wood: 3, Work: 45, Layer: LayerObject, StoneOK: true, MetalOK: true},
	Armchair: {Kind: Armchair, Name: "armchair", Wood: 12, Work: 1400, Layer: LayerObject, Rotatable: true,
		Fabric: true},
	// DEF-3, wiki: Barricade, IED trap, Autocannon turret, Uranium slug
	// turret, Mortar, Autodoor. A charge costs two shells (Wood counts
	// them, in steel until C3 adds the shell item); only the blast charge
	// has a wiki page, the other three copy it.
	Barricade: {Kind: Barricade, Name: "barricade", Wood: 1, Work: 32, Layer: LayerObject, StoneOK: true, MetalOK: true},
	IEDHighExplosive: {Kind: IEDHighExplosive, Name: "blast charge", Wood: 2, Work: 140, Layer: LayerObject,
		MetalOK: true, MetalOnly: true},
	IEDIncendiary: {Kind: IEDIncendiary, Name: "fire charge", Wood: 2, Work: 140, Layer: LayerObject,
		MetalOK: true, MetalOnly: true},
	IEDEMP: {Kind: IEDEMP, Name: "pulse charge", Wood: 2, Work: 140, Layer: LayerObject,
		MetalOK: true, MetalOnly: true},
	IEDSmoke: {Kind: IEDSmoke, Name: "smoke charge", Wood: 2, Work: 140, Layer: LayerObject,
		MetalOK: true, MetalOnly: true},
	Autocannon: {Kind: Autocannon, Name: "autocannon", Wood: 39, Work: 1500, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 2}, MetalOK: true, MetalOnly: true, Components: 6},
	UraniumSlugTurret: {Kind: UraniumSlugTurret, Name: "slug turret", Wood: 33, Work: 1500, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 2}, MetalOK: true, MetalOnly: true, Components: 6},
	// The crew stands at the spot, south of the tube.
	Mortar: {Kind: Mortar, Name: "mortar", Wood: 6, Work: 200, Layer: LayerObject, BlocksMove: true,
		Spot: [2]int{0, 1}, HasSpot: true, MetalOK: true, MetalOnly: true, Components: 6},
	Autodoor: {Kind: Autodoor, Name: "powered door", Wood: 3, Work: 110, Layer: LayerObject, MetalOK: true, Components: 2},
	// PRD-4, wiki: Shelf (2x1, 20 stuff, 500 work, research Complex
	// furniture; it slows passage but does not block it; 3 stacks per
	// cell, PRD-4's storage rule). Scaled as the wave D rows.
	Shelf: {Kind: Shelf, Name: "shelf", Wood: 2, Work: 50, Layer: LayerObject, Rotatable: true, Size: [2]int{2, 1},
		StoneOK: true, MetalOK: true},
	// DEF-4, wiki: Defoliator ship (6x3, 1200 HP, deconstructs to
	// nothing); placed whole by the threat, so no cost and no work.
	ShipPart: {Kind: ShipPart, Name: "ship part", Layer: LayerObject, BlocksMove: true, Size: [2]int{6, 3}},
	// STO/CBT contract, wiki: Hopper (15 steel, 300 work), Nutrient paste
	// dispenser (3x4, 125 steel + 3 components, 2200 work; a colonist
	// takes meals at the cell south of its middle), Sarcophagus (1x2, 60
	// stuff, 2400 work). Scaled as the wave D rows; the dispenser keeps
	// its wiki footprint (hoppers line its sides). A hopper and a
	// sarcophagus are passed through, like the shelf; the dispenser blocks.
	Hopper: {Kind: Hopper, Name: "hopper", Wood: 2, Work: 30, Layer: LayerObject, MetalOK: true, MetalOnly: true},
	NutrientPasteDispenser: {Kind: NutrientPasteDispenser, Name: "nutrient paste dispenser", Wood: 14, Work: 220,
		Layer: LayerObject, BlocksMove: true, Size: [2]int{3, 4}, Spot: [2]int{1, 4}, HasSpot: true,
		MetalOK: true, MetalOnly: true, Components: 3},
	Sarcophagus: {Kind: Sarcophagus, Name: "sarcophagus", Wood: 7, Work: 240, Layer: LayerObject, Rotatable: true,
		Size: [2]int{1, 2}, StoneOK: true, MetalOK: true},
	// CBT-4, wiki: Hive (1x1, 130 HP, deconstructs to nothing); placed by
	// the infestation, so no cost and no work.
	HiveMound: {Kind: HiveMound, Name: "hive mound", Layer: LayerObject, BlocksMove: true},
	// FOOD-0 contract, wiki: Hydroponics basin (1x4, 100 steel + 1
	// component, 2800 work), Sun lamp (40 steel, 330 work). Scaled as the
	// wave D rows; the basin is passed through, like the shelf.
	HydroponicsBasin: {Kind: HydroponicsBasin, Name: "hydroponics basin", Wood: 11, Work: 280, Layer: LayerObject,
		Rotatable: true, Size: [2]int{1, 4}, MetalOK: true, MetalOnly: true, Components: 1},
	SunLamp: {Kind: SunLamp, Name: "sun lamp", Wood: 4, Work: 33, Layer: LayerObject, MetalOK: true, MetalOnly: true},
	// TEMP-1, wiki: Passive cooler (50 wood; game/thermo cools, fuel.go
	// burns its wood), Vent (30 steel, impassable; open or shut like a
	// power toggle). Scaled as the wave D rows; the work is original
	// tuning (the wiki gives none).
	PassiveCooler: {Kind: PassiveCooler, Name: "passive cooler", Wood: 6, Work: 70, Layer: LayerObject, BlocksMove: true},
	Vent: {Kind: Vent, Name: "vent", Wood: 3, Work: 80, Layer: LayerObject, BlocksMove: true, Rotatable: true,
		MetalOK: true, MetalOnly: true},
	// RSCH-0 contract, wiki: Multi-analyzer (2x2, 40 steel + 50 plasteel
	// + 20 gold + 8 components, 10000 work). Scaled as the wave D rows
	// (steel / 9 rounded, work / 10, components as the wiki); the plasteel
	// and gold are secondary materials, not modelled yet.
	MultiAnalyzer: {Kind: MultiAnalyzer, Name: "multi-analyzer", Wood: 4, Work: 1000, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 2}, MetalOK: true, MetalOnly: true, Components: 8},
	// AX-0 contract rows (Planned until their owner flips them). wiki
	// numbers scaled as the wave D rows: materials / 9 rounded (at least
	// 1), work / 10, components and advanced components as the wiki;
	// "orig" marks original tuning where the wiki is silent or [MEM].
	// A2, wiki: Ornate door, Column, Animal flap, Double sleeping spot,
	// Animal sleeping box, Table, Couch, Plant pot, Small shelf, Wall torch
	// lamp, Wall lamp, Flood light, Urn, Large stele, Grand stele, Egg box.
	OrnateDoor: {Kind: OrnateDoor, Name: "ornate door", Wood: 8, Work: 1500, Layer: LayerObject, Rotatable: true,
		Size: [2]int{2, 1}, StoneOK: true, MetalOK: true, Extra: [3]Cost{{item.Gold, 6}}},
	Column: {Kind: Column, Name: "column", Wood: 2, Work: 75, Layer: LayerObject, BlocksMove: true, StoneOK: true, MetalOK: true},
	// wiki (W): fabric or leather; not minifiable.
	AnimalFlap: {Kind: AnimalFlap, Name: "animal flap", Wood: 3, Work: 85, Layer: LayerObject, Fabric: true},
	// wiki text: 2x2, like the double bed.
	DoubleSleepingSpot: {Kind: DoubleSleepingSpot, Name: "double sleeping spot", Layer: LayerObject, Rotatable: true,
		Size: [2]int{2, 2}},
	AnimalSleepingBox: {Kind: AnimalSleepingBox, Name: "animal sleeping box", Wood: 3, Work: 18, Layer: LayerObject,
		StoneOK: true, MetalOK: true},
	Table2x2: {Kind: Table2x2, Name: "table 2x2", Wood: 6, Work: 150, Layer: LayerObject, Size: [2]int{2, 2},
		StoneOK: true, MetalOK: true},
	Table2x4: {Kind: Table2x4, Name: "table 2x4", Wood: 11, Work: 300, Layer: LayerObject, Rotatable: true,
		Size: [2]int{2, 4}, StoneOK: true, MetalOK: true},
	Table3x3: {Kind: Table3x3, Name: "table 3x3", Wood: 11, Work: 330, Layer: LayerObject, Size: [2]int{3, 3},
		StoneOK: true, MetalOK: true},
	Couch: {Kind: Couch, Name: "couch", Wood: 22, Work: 2000, Layer: LayerObject, Rotatable: true, Size: [2]int{2, 1},
		Fabric: true},
	PlantPot:   {Kind: PlantPot, Name: "plant pot", Wood: 2, Work: 25, Layer: LayerObject, StoneOK: true, MetalOK: true},
	SmallShelf: {Kind: SmallShelf, Name: "small shelf", Wood: 1, Work: 25, Layer: LayerObject, StoneOK: true, MetalOK: true},
	// Wall-mounted: placed against a wall, facing away from it (A2).
	WallTorch: {Kind: WallTorch, Name: "wall torch", Wood: 2, Work: 10, Layer: LayerObject, Rotatable: true},
	WallLamp: {Kind: WallLamp, Name: "wall lamp", Wood: 2, Work: 33, Layer: LayerObject, Rotatable: true,
		MetalOK: true, MetalOnly: true},
	FloodLight: {Kind: FloodLight, Name: "flood light", Wood: 6, Work: 60, Layer: LayerObject, MetalOK: true, MetalOnly: true},
	// Never built (traded or found; wiki: 20 stuff): it arrives crated and
	// is only reinstalled. Work orig (its deconstruct time).
	Urn: {Kind: Urn, Name: "urn", Wood: 2, Work: 50, Layer: LayerObject, StoneOK: true, MetalOK: true, Unbuildable: true},
	LargeStele: {Kind: LargeStele, Name: "large stele", Wood: 14, Work: 800, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 2}, StoneOnly: true},
	GrandStele: {Kind: GrandStele, Name: "grand stele", Wood: 28, Work: 1600, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{3, 3}, StoneOnly: true},
	EggBox: {Kind: EggBox, Name: "egg box", Wood: 2, Work: 200, Layer: LayerObject, StoneOK: true, MetalOK: true},
	// A3, wiki: Floors, Straw matting, Sterile tile, Stone tile, Flagstone,
	// Carpet (per tile; the lead's ruling: flagstone takes 4 blocks).
	StrawMatting: {Kind: StrawMatting, Name: "straw matting", Wood: 1, Work: 12, Layer: LayerFloor, Only: item.Hay},
	Concrete:     {Kind: Concrete, Name: "concrete", Wood: 1, Work: 10, Layer: LayerFloor, MetalOK: true, MetalOnly: true},
	PavedTile:    {Kind: PavedTile, Name: "paved tile", Wood: 1, Work: 30, Layer: LayerFloor, MetalOK: true, MetalOnly: true},
	SteelTile:    {Kind: SteelTile, Name: "steel tile", Wood: 1, Work: 80, Layer: LayerFloor, MetalOK: true, MetalOnly: true},
	SilverTile:   {Kind: SilverTile, Name: "silver tile", Wood: 8, Work: 80, Layer: LayerFloor, Only: item.Silver},
	GoldTile:     {Kind: GoldTile, Name: "gold tile", Wood: 8, Work: 80, Layer: LayerFloor, Only: item.Gold},
	SterileTile: {Kind: SterileTile, Name: "sterile tile", Wood: 1, Work: 160, Layer: LayerFloor, MetalOK: true, MetalOnly: true,
		Extra: [3]Cost{{item.Silver, 1}}},
	StoneTile:   {Kind: StoneTile, Name: "stone tile", Wood: 1, Work: 110, Layer: LayerFloor, StoneOnly: true},
	Flagstone:   {Kind: Flagstone, Name: "flagstone", Wood: 1, Work: 50, Layer: LayerFloor, StoneOnly: true},
	CarpetFloor: {Kind: CarpetFloor, Name: "carpet floor", Wood: 1, Work: 80, Layer: LayerFloor, Only: item.Cloth},
	// A4, wiki: Waterproof conduit, Chemfuel powered generator, Watermill
	// generator (5x6 with its wheel; 280 wood + 80 steel), Deep drill,
	// Ground-penetrating scanner, Moisture pump, Electric crematorium. The
	// chemfuel generator's work is orig (the wood-fired generator's).
	WaterproofConduit: {Kind: WaterproofConduit, Name: "waterproof conduit", Wood: 1, Work: 4, Layer: LayerConduit,
		MetalOK: true, MetalOnly: true},
	ChemfuelGenerator: {Kind: ChemfuelGenerator, Name: "chemfuel generator", Wood: 11, Work: 320, Layer: LayerObject,
		BlocksMove: true, Size: [2]int{2, 2}, MetalOK: true, MetalOnly: true, Components: 3},
	Watermill: {Kind: Watermill, Name: "watermill generator", Wood: 31, Work: 400, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{5, 6}, Components: 3, Extra: [3]Cost{{item.Steel, 9}}},
	DeepDrill: {Kind: DeepDrill, Name: "deep drill", Wood: 11, Work: 1000, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{1, 2}, Spot: [2]int{0, 2}, HasSpot: true, MetalOK: true, MetalOnly: true, Components: 2},
	GroundScanner: {Kind: GroundScanner, Name: "ground-penetrating scanner", Wood: 17, Work: 1200, Layer: LayerObject,
		BlocksMove: true, Rotatable: true, Size: [2]int{3, 3}, Spot: [2]int{1, 3}, HasSpot: true, MetalOK: true, MetalOnly: true,
		Components: 4, Extra: [3]Cost{{item.AdvancedComponents, 1}}},
	MoisturePump: {Kind: MoisturePump, Name: "moisture pump", Wood: 8, Work: 150, Layer: LayerObject, BlocksMove: true,
		MetalOK: true, MetalOnly: true, Components: 4},
	ElectricCrematorium: {Kind: ElectricCrematorium, Name: "electric crematorium", Wood: 17, Work: 450, Layer: LayerObject,
		BlocksMove: true, Rotatable: true, Size: [2]int{3, 2}, Spot: [2]int{1, 2}, HasSpot: true, Bench: true, StoneOnly: true,
		Components: 2, Extra: [3]Cost{{item.Steel, 2}}},
	// A5, wiki: Firefoam popper, IED firefoam trap (2 firefoam shells),
	// Foam turret (100 metal; 140 chemfuel), IED antigrain warhead trap (1
	// warhead), Rocketswarm launcher (20 plasteel; 180 chemfuel). A charge's
	// Wood counts its shells or warhead (Only).
	FoamPopper: {Kind: FoamPopper, Name: "foam popper", Wood: 8, Work: 150, Layer: LayerObject, MetalOK: true, MetalOnly: true,
		Components: 1},
	IEDFoam: {Kind: IEDFoam, Name: "foam charge", Wood: 2, Work: 140, Layer: LayerObject, Only: item.FoamShells},
	FoamTurret: {Kind: FoamTurret, Name: "foam turret", Wood: 11, Work: 180, Layer: LayerObject, BlocksMove: true,
		MetalOK: true, MetalOnly: true, Components: 3, Extra: [3]Cost{{item.Chemfuel, 16}}},
	IEDWarhead: {Kind: IEDWarhead, Name: "warhead charge", Wood: 1, Work: 140, Layer: LayerObject, Only: item.AntimatterWarhead},
	RocketBattery: {Kind: RocketBattery, Name: "rocket battery", Wood: 22, Work: 1000, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 2}, MetalOK: true, MetalOnly: true, Components: 2,
		Extra: [3]Cost{{item.Plasteel, 2}, {item.Chemfuel, 20}}},
	// A6, wiki: Hoopstone ring, Game of Ur, Chess table, Billiards table
	// (80 cloth + 30 wood), Poker table (50 cloth + 25 wood), Tube
	// television, Flatscreen television, Megascreen television and
	// Telescope (never built: traded or found; their material and work
	// are orig, for salvage and deconstruction).
	RingToss: {Kind: RingToss, Name: "ring toss", Wood: 2, Work: 10, Layer: LayerObject, StoneOK: true, MetalOK: true},
	UrBoard:  {Kind: UrBoard, Name: "ur board", Wood: 4, Work: 600, Layer: LayerObject, StoneOK: true, MetalOK: true},
	ChessTable: {Kind: ChessTable, Name: "chess table", Wood: 8, Work: 800, Layer: LayerObject, StoneOK: true,
		MetalOK: true},
	BilliardsTable: {Kind: BilliardsTable, Name: "billiards table", Wood: 9, Work: 1200, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{2, 3}, StoneOK: true, MetalOK: true, Extra: [3]Cost{{item.Cloth, 9}, {item.Wood, 3}}},
	PokerTable: {Kind: PokerTable, Name: "poker table", Wood: 6, Work: 1000, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 2}, StoneOK: true, MetalOK: true, Extra: [3]Cost{{item.Cloth, 6}, {item.Wood, 3}}},
	TubeTV: {Kind: TubeTV, Name: "tube television", Wood: 9, Work: 1000, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, MetalOK: true, MetalOnly: true, Components: 4},
	FlatscreenTV: {Kind: FlatscreenTV, Name: "flatscreen television", Wood: 16, Work: 4000, Layer: LayerObject,
		BlocksMove: true, Rotatable: true, Size: [2]int{2, 1}, MetalOK: true, MetalOnly: true, Components: 16},
	BigScreenTV: {Kind: BigScreenTV, Name: "big screen television", Wood: 16, Work: 400, Layer: LayerObject,
		BlocksMove: true, Rotatable: true, Size: [2]int{3, 1}, MetalOK: true, MetalOnly: true, Unbuildable: true},
	Telescope: {Kind: Telescope, Name: "telescope", Wood: 4, Work: 300, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Spot: [2]int{0, 1}, HasSpot: true, MetalOK: true, MetalOnly: true, Unbuildable: true},
	// A7, wiki: Bookcase, Small bookcase.
	Bookcase: {Kind: Bookcase, Name: "bookcase", Wood: 2, Work: 50, Layer: LayerObject, Rotatable: true, Size: [2]int{2, 1},
		StoneOK: true, MetalOK: true},
	SmallBookcase: {Kind: SmallBookcase, Name: "small bookcase", Wood: 1, Work: 25, Layer: LayerObject, StoneOK: true,
		MetalOK: true},
	// A8, wiki: Ship structural beam, Ship cryptosleep casket, Ship
	// computer core (+1 persona core), Ship reactor, Ship engine, Sensor
	// cluster (steel the main material).
	ShipBeam: {Kind: ShipBeam, Name: "ship structural beam", Wood: 22, Work: 800, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{2, 6}, MetalOK: true, MetalOnly: true, Components: 3,
		Extra: [3]Cost{{item.Plasteel, 4}, {item.AdvancedComponents, 1}}},
	ShipCasket: {Kind: ShipCasket, Name: "ship stasis casket", Wood: 13, Work: 800, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{1, 2}, MetalOK: true, MetalOnly: true, Components: 3,
		Extra: [3]Cost{{item.Uranium, 2}, {item.AdvancedComponents, 3}}},
	ShipCore: {Kind: ShipCore, Name: "ship computer core", Wood: 17, Work: 1600, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 2}, MetalOK: true, MetalOnly: true,
		Extra: [3]Cost{{item.AdvancedComponents, 4}, {item.Gold, 8}, {item.MindCore, 1}}},
	ShipReactor: {Kind: ShipReactor, Name: "ship reactor", Wood: 39, Work: 6500, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{6, 7}, MetalOK: true, MetalOnly: true,
		Extra: [3]Cost{{item.Plasteel, 31}, {item.Uranium, 8}, {item.AdvancedComponents, 8}}},
	ShipEngine: {Kind: ShipEngine, Name: "ship engine", Wood: 29, Work: 5000, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{3, 4}, MetalOK: true, MetalOnly: true,
		Extra: [3]Cost{{item.Plasteel, 16}, {item.Uranium, 8}, {item.AdvancedComponents, 6}}},
	ShipSensor: {Kind: ShipSensor, Name: "ship sensor cluster", Wood: 16, Work: 3000, Layer: LayerObject, BlocksMove: true,
		Size: [2]int{2, 2}, MetalOK: true, MetalOnly: true, Components: 6,
		Extra: [3]Cost{{item.Gold, 1}, {item.AdvancedComponents, 6}}},
	// A9, wiki: Cryptosleep casket, Transport pod, Pod launcher,
	// Long-range mineral scanner, Caravan hitching spot (free).
	StasisCasket: {Kind: StasisCasket, Name: "stasis casket", Wood: 20, Work: 320, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{1, 2}, MetalOK: true, MetalOnly: true, Components: 4,
		Extra: [3]Cost{{item.Uranium, 1}, {item.AdvancedComponents, 1}}},
	TransportPod: {Kind: TransportPod, Name: "transport pod", Wood: 7, Work: 160, Layer: LayerObject, MetalOK: true,
		MetalOnly: true, Components: 1},
	PodLauncher: {Kind: PodLauncher, Name: "pod launcher", Wood: 6, Work: 300, Layer: LayerObject, BlocksMove: true,
		Rotatable: true, Size: [2]int{1, 2}, MetalOK: true, MetalOnly: true, Components: 1},
	MineralScanner: {Kind: MineralScanner, Name: "long-range mineral scanner", Wood: 22, Work: 1000, Layer: LayerObject,
		BlocksMove: true, Rotatable: true, Size: [2]int{3, 3}, Spot: [2]int{1, 3}, HasSpot: true, MetalOK: true,
		MetalOnly: true, Components: 6, Extra: [3]Cost{{item.AdvancedComponents, 2}}},
	HitchingSpot: {Kind: HitchingSpot, Name: "caravan hitching spot", Layer: LayerObject},
}

// RecipeBench is the kind whose recipes and work k runs (PLAY-021): an
// electric bench works its base kind's bills (or research); any other
// kind is its own.
func RecipeBench(k Kind) Kind {
	switch k {
	case ElectricStove:
		return Stove
	case ElectricTailoringBench:
		return TailoringBench
	case ElectricResearchBench:
		return ResearchBench
	}
	return k
}

// Requires is the support kind k built from stuff needs under it
// (MAPGEN-3): stone and steel walls and floors need firm ground; a bridge
// holds everything else (wooden walls, doors, fences, furniture). A
// bridge itself needs nothing (it is the support).
func Requires(k Kind, stuff item.Kind) terrain.Support {
	d := DefOf(k)
	switch {
	case d.Foundation:
		return terrain.SupportNone
	case d.Layer == LayerFloor || d.Carpet:
		return terrain.SupportHeavy
	case k == Wall && stuff != item.Wood:
		return terrain.SupportHeavy
	}
	return terrain.SupportLight
}

// Provides is the support a finished foundation kind gives (a bridge:
// light); none for every other kind.
func Provides(k Kind) terrain.Support {
	if DefOf(k).Foundation {
		return terrain.SupportLight
	}
	return terrain.SupportNone
}

// MaxHP is how much damage a building takes before it is destroyed.
func MaxHP(k Kind, s item.Kind) int {
	hp := 60
	switch k {
	case Wall:
		hp = 160
	case Door, Autodoor: // wiki: Door, Autodoor (the same base)
		hp = 110
	case Sandbags, Barricade: // wiki: Sandbags, Barricade (both 300 base)
		hp = 140
	case SpikeTrap:
		hp = 40
	case SentryTurret:
		hp = 100 // 140 in steel, its only material (PLAY-024)
	// DEF-3, steel-built: wiki: Autocannon turret, Uranium slug turret
	// (380), Mortar (180), divided by steel's 140 % so steel gives them.
	case Autocannon, UraniumSlugTurret:
		hp = 272
	case Mortar:
		hp = 129
	case Shelf: // wiki: Shelf (100 base)
		hp = 100
	case ShipPart: // wiki: Defoliator ship
		hp = 1200
	// STO/CBT contract, steel-built ones divided by steel's 140 % as DEF-3:
	// wiki: Hopper (100), Nutrient paste dispenser (350), Sarcophagus (250).
	case Hopper:
		hp = 71
	case NutrientPasteDispenser:
		hp = 250
	case Sarcophagus:
		hp = 250
	case HiveMound: // wiki: Hive
		hp = 130
	default:
		if ax, ok := axMaxHP(k); ok { // ax_hp.go (AX-0 contract)
			hp = ax
		}
	}
	pct, _ := StuffStats(s)
	return hp * pct / 100
}

// WallMounted reports a building set in a wall gap that bounds rooms like
// a wall: the cooler (PLAY-021) and the vent (TEMP-0).
func (k Kind) WallMounted() bool { return k == Cooler || k == Vent }

// WallFacing is the facing of a cooler (PLAY-021) or vent (TEMP-0) with h
// solid neighbours east and west and v north and south: across the wall it
// stands in (a cooler's cold side). Walls east and west of it (h > v) give
// north or south, walls north and south of it (v > h) east or west. A facing across the wall is kept (prefer is the current
// or the player's choice); another turns on, in the rotation order south,
// east, north, west, to the next one across, so rotating through all four
// still reaches both. With no clear wall axis prefer stays.
func WallFacing(h, v int, prefer Facing) Facing {
	if h == v {
		return prefer
	}
	across := func(f Facing) bool {
		return (f == North || f == South) == (h > v)
	}
	for i := 0; i < 4 && !across(prefer); i++ {
		prefer = (prefer + 1) % 4
	}
	return prefer
}

// IsDoor reports a door: the plain door and the powered autodoor (DEF-3).
func (k Kind) IsDoor() bool { return k == Door || k == Autodoor }

// Sleeps reports whether k is something to sleep in.
func (k Kind) Sleeps() bool {
	return k == Bed || k == SleepingSpot || k == DoubleBed || k == HospitalBed || k == Bedroll || k == DoubleBedroll
}

// AnimalSleeps reports a sleeping place for tame animals (BED-1).
func (k Kind) AnimalSleeps() bool { return k == AnimalSleepingSpot || k == AnimalBed }

// Sleepers is how many pawns a sleeping place holds side by side.
func (k Kind) Sleepers() int {
	switch {
	case k == DoubleBed || k == DoubleBedroll:
		return 2
	case k.Sleeps():
		return 1
	}
	return 0
}

// SleepCell is the head-end cell of sleeping slot `slot` (pillow side by
// side across the bed's width; the sleeper's feet point to the facing).
func SleepCell(k Kind, f Facing, x, y, slot int) (int, int) {
	cells, _, _ := Footprint(k, f, x, y)
	if slot < 0 || slot >= len(cells) || slot >= k.Sleepers() {
		slot = 0
	}
	return cells[slot][0], cells[slot][1]
}

// Instant kinds need no materials or work: designating one places it,
// and deconstructing removes it on the spot.
func (k Kind) Instant() bool {
	d := DefOf(k)
	return d.Wood == 0 && d.Work == 0
}

// rotate maps a south-frame point to facing f (screen coords, y down),
// matching the art: east = (x, y) -> (y, -x).
func rotate(f Facing, x, y int) (int, int) {
	switch f {
	case East:
		return y, -x
	case North:
		return -x, -y
	case West:
		return -y, x
	}
	return x, y
}

// Footprint returns the covered cells and the interaction spot (absolute,
// ok=false if none) for kind k with its top-left at (x, y) facing f.
func Footprint(k Kind, f Facing, x, y int) (cells [][2]int, spot [2]int, ok bool) {
	d := DefOf(k)
	w, h := d.Size[0], d.Size[1]
	if w <= 0 || h <= 0 {
		w, h = 1, 1
	}
	if !d.Rotatable {
		f = South
	}
	type pt struct{ x, y int }
	var raw []pt
	for cy := 0; cy < h; cy++ {
		for cx := 0; cx < w; cx++ {
			rx, ry := rotate(f, cx, cy)
			raw = append(raw, pt{rx, ry})
		}
	}
	minX, minY := raw[0].x, raw[0].y
	for _, p := range raw {
		if p.x < minX {
			minX = p.x
		}
		if p.y < minY {
			minY = p.y
		}
	}
	for _, p := range raw {
		cells = append(cells, [2]int{x + p.x - minX, y + p.y - minY})
	}
	if d.HasSpot {
		sx, sy := rotate(f, d.Spot[0], d.Spot[1])
		return cells, [2]int{x + sx - minX, y + sy - minY}, true
	}
	return cells, [2]int{}, false
}

// Covers reports whether (px, py) is inside k's footprint at (x, y, f).
func Covers(k Kind, f Facing, x, y, px, py int) bool {
	cells, _, _ := Footprint(k, f, x, y)
	for _, c := range cells {
		if c[0] == px && c[1] == py {
			return true
		}
	}
	return false
}

// DefOf returns the definition for k.
// Movable reports a building that can be moved (reinstalled) elsewhere:
// everything but walls, doors, fences, fence gates, floors and graves
// (PLAY-011: fixed pieces change material in place instead). Conduits are
// fixed too (PLAY-021: cheap, laid in lines, rebuilt rather than moved),
// as are a cooler and a vent (they fill a wall gap, like a door) and a
// geothermal generator (it only stands over a steam vent).
func Movable(k Kind) bool {
	switch k {
	case Wall, Door, Autodoor, Fence, FenceGate, Grave, Bridge, SleeperCasket,
		PowerConduit, HiddenConduit, Cooler, Vent, GeothermalGenerator,
		// AX-0 contract: door-like, wall-mounted or fixed pieces (wiki: not
		// minifiable), and the escape ship's parts.
		OrnateDoor, Column, AnimalFlap, WallTorch, WallLamp, WaterproofConduit, Watermill, HitchingSpot,
		ShipBeam, ShipCasket, ShipCore, ShipReactor, ShipEngine, ShipSensor:
		return false
	}
	return DefOf(k).Layer != LayerFloor
}

func DefOf(k Kind) Def {
	if k >= kindCount {
		return Def{}
	}
	return defs[k]
}

// All lists every buildable definition in Kind order.
func All() []Def {
	out := make([]Def, 0, kindCount)
	for _, d := range defs {
		out = append(out, d)
	}
	return out
}

// Parse returns the Kind for a definition name.
func Parse(name string) (Kind, bool) {
	for _, d := range defs {
		if d.Name == name {
			return d.Kind, true
		}
	}
	return 0, false
}

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool { return k < kindCount }

func (k Kind) String() string {
	if k >= kindCount {
		return "unknown"
	}
	return defs[k].Name
}

// WoodCost is delivered wood required before construction work.
func WoodCost(k Kind) int { return DefOf(k).Wood }

// ComponentCost is the components delivered after the material.
func ComponentCost(k Kind) int { return DefOf(k).Components }

// WorkTicks is Do-phase labor after materials are delivered.
func WorkTicks(k Kind) int { return DefOf(k).Work }

// DeconstructTicks is the labor to take a building down.
func DeconstructTicks(k Kind) int { return (DefOf(k).Work + 1) / 2 }

// Refund is the material returned by deconstruction (half, rounded down;
// a carpet comes back whole, a placed kind gives its Salvage).
func Refund(k Kind) int {
	if s := DefOf(k).Salvage; s > 0 {
		return s
	}
	if DefOf(k).Carpet {
		return DefOf(k).Wood
	}
	return DefOf(k).Wood / 2
}

// StuffOK reports whether k may be built from material s.
func StuffOK(k Kind, s item.Kind) bool {
	if d := DefOf(k); d.Only != 0 || d.StoneOnly { // AX-0 contract
		return s == d.Only || d.StoneOnly && item.IsStoneBlocks(s)
	}
	if DefOf(k).Carpet || s.IsCarpet() {
		return DefOf(k).Carpet && s.IsCarpet()
	}
	if DefOf(k).Fabric || IsFabric(s) {
		return DefOf(k).Fabric && IsFabric(s)
	}
	switch {
	case s == item.Wood:
		return !DefOf(k).MetalOnly
	case item.IsStoneBlocks(s):
		return DefOf(k).StoneOK
	case s == item.Steel:
		return DefOf(k).MetalOK
	}
	return false
}

// DefaultStuff is the material k is built from when none is chosen.
func DefaultStuff(k Kind) item.Kind {
	if d := DefOf(k); d.Only != 0 { // AX-0 contract
		return d.Only
	} else if d.StoneOnly {
		return item.StoneBlocks()[0]
	}
	if DefOf(k).Carpet {
		return item.CarpetStraw
	}
	if DefOf(k).MetalOnly {
		return item.Steel
	}
	if DefOf(k).Fabric {
		return item.Cloth
	}
	return item.Wood
}

// IsFabric reports a material a Fabric kind is sewn from (BED-1).
func IsFabric(s item.Kind) bool { return s == item.Cloth || s == item.Leather }

// Stuffs lists the materials k may be built from, default first.
func Stuffs(k Kind) []item.Kind {
	d := DefOf(k)
	switch { // AX-0 contract
	case d.Only != 0:
		return []item.Kind{d.Only}
	case d.StoneOnly:
		return item.StoneBlocks()
	}
	if d.Carpet {
		return item.Carpets()
	}
	if d.MetalOnly {
		return []item.Kind{item.Steel}
	}
	if d.Fabric {
		return []item.Kind{item.Cloth, item.Leather}
	}
	out := []item.Kind{item.Wood}
	if d.StoneOK {
		out = append(out, item.StoneBlocks()...)
	}
	if d.MetalOK {
		out = append(out, item.Steel)
	}
	return out
}

// WorkFor is the construct work for k built from s.
func WorkFor(k Kind, s item.Kind) int {
	_, pct := StuffStats(s)
	return DefOf(k).Work * pct / 100
}

// Facing is the orientation of a rotatable building.
type Facing uint8

const (
	South Facing = iota
	East
	North
	West
)

func (f Facing) String() string {
	switch f {
	case East:
		return "east"
	case North:
		return "north"
	case West:
		return "west"
	default:
		return "south"
	}
}

// ParseFacing maps a raw command value to a Facing (unknown -> South).
func ParseFacing(v uint8) Facing {
	if v > uint8(West) {
		return South
	}
	return Facing(v)
}

// Frame is a blueprint that accepts wood then construct work.
type Frame struct {
	Kind   Kind
	Stuff  item.Kind
	Facing Facing
	X, Y   int
	Wood   int
	Need   int
	// Comp of CompNeed components delivered (zero for kinds without).
	Comp, CompNeed int
	WorkLeft       int
	// Rebuild marks the blueprint of a smashed building: builders and
	// haulers see to it before anything else.
	Rebuild bool
	// Reinstall is the building (entity id) this frame moves here; 0 for
	// a new blueprint. A reinstall needs no materials (UI-011).
	Reinstall uint64
	// Replaces is the wall, fence, door or gate this door or gate frame
	// takes the place of; the frame waits until that piece is gone.
	// ReplaceMarked: the frame itself set the piece's deconstruct mark
	// (so cancelling the frame clears it again).
	Replaces      entity.EntityID
	ReplaceMarked bool
	// Swaps is the building this frame turns into its own material in
	// place (same kind, same footprint, PLAY-011): built without taking
	// the old piece down.
	Swaps entity.EntityID
	// AX-0 contract (0 until their owners use them): Tint is a carpet
	// floor's colour (A3, the building's Tint once built); ExtraGot the
	// Def.Extra secondary materials delivered so far, slot by slot.
	Tint     uint8
	ExtraGot [3]int
}

// Ready reports materials and components are in place.
func (f Frame) Ready() bool { return f.Wood >= f.Need && f.Comp >= f.CompNeed }

// Building is a completed structure.
type Building struct {
	Kind        Kind
	Stuff       item.Kind
	Facing      Facing
	X, Y        int
	Claimant    uint64 // EntityID of the pawn who claimed a bed; 0 if free
	Claimant2   uint64 // second sleeper of a double bed
	Deconstruct bool   // designated for removal
	WorkLeft    int    // remaining deconstruct work once started
	Damage      int    // damage taken (raiders bashing, bullets); repaired by builders
	// ForPrisoners marks a bed for prisoners only (ENG-003).
	ForPrisoners bool
	// Medical keeps this bed for the sick and injured (PLAY-020): it has no
	// owner and healthy colonists don't sleep in it.
	Medical bool
	// Ruin: an abandoned piece left by map generation (MAPGEN-2): not
	// colony wealth, not a Home anchor, salvaged for half.
	Ruin bool
	// Ancient (MAPGEN-3): an intact piece of a sealed vault (walls, floor,
	// caskets): not colony wealth, not a Home anchor, not repaired.
	Ancient bool
	// Sealed sleeper caskets (MAPGEN-3): Occupant is a vault.Occupant
	// (0: open or empty shell); CasketGroup the vault id whose caskets open
	// together.
	Occupant    uint8
	CasketGroup int
	// Forbidden (BED-1): nobody claims or uses it until allowed
	// (command.SetForbidden on the building).
	Forbidden bool
	// Quality (PRD-2) is the building's quality, quality.Level+1 (0 = none
	// or built before quality existed: read as Normal by the stats).
	Quality uint8
	// AX-0 contract (0 until their owners use them): Tint is a carpet
	// floor's colour index (A3; 0 the default colour); Uninstall marks it
	// to be crated by a colonist (A1, like Deconstruct; WorkLeft is the
	// work left once started).
	Tint      uint8
	Uninstall bool
}

// HP is the building's remaining hit points.
func (b Building) HP() int { return max(0, MaxHP(b.Kind, b.Stuff)-b.Damage) }

// SlotOf is the sleeping slot pawn id holds here, or -1.
func (b Building) SlotOf(id uint64) int {
	switch {
	case id == 0:
		return -1
	case b.Claimant == id:
		return 0
	case b.Claimant2 == id && b.Kind.Sleepers() > 1:
		return 1
	}
	return -1
}

// CanClaim reports whether pawn id may sleep here (it already holds a
// slot, or one is free).
func (b Building) CanClaim(id uint64) bool {
	if b.SlotOf(id) >= 0 {
		return true
	}
	return b.Claimant == 0 || (b.Kind.Sleepers() > 1 && b.Claimant2 == 0)
}

// Claim gives pawn id a slot (keeping one it already holds).
func (b Building) Claim(id uint64) Building {
	switch {
	case b.SlotOf(id) >= 0:
	case b.Claimant == 0:
		b.Claimant = id
	case b.Kind.Sleepers() > 1 && b.Claimant2 == 0:
		b.Claimant2 = id
	}
	return b
}

// Release frees pawn id's slot.
func (b Building) Release(id uint64) Building {
	if b.Claimant == id {
		b.Claimant = 0
	}
	if b.Claimant2 == id {
		b.Claimant2 = 0
	}
	return b
}

// Owner is the first claimant that is not id (for "owned by" reasons).
func (b Building) Owner(id uint64) uint64 {
	if b.Claimant != 0 && b.Claimant != id {
		return b.Claimant
	}
	if b.Claimant2 != id {
		return b.Claimant2
	}
	return 0
}
