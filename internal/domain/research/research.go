package research

// Project is one research project.
type Project uint8

const (
	// FoodPreservation: drying racks.
	FoodPreservation Project = iota
	// Herbalism: healroot can be sown.
	Herbalism
	// Tailoring: the tailoring bench (parkas, tuques, dusters).
	Tailoring
	// Traps: spike traps.
	Traps
	// Gunsmithing: crude rifles at the workbench.
	Gunsmithing
	// Fabrication: components at the fabrication bench (NG-1 §4.8).
	Fabrication
	// Pharmacology: herbal medicine refined into industrial medicine;
	// the medical drugs (MED-07).
	Pharmacology
	// ENG-003: Brewing (brewgrain and the brewing bench), HerbalRemedies
	// (ashleaf, the drug table, spark tablets) and Prosthetics (steel limbs).
	Brewing
	HerbalRemedies
	Prosthetics
	// Bridges (MAPGEN-3, append only, saved): wooden bridge foundations.
	Bridges
	// Electricity (PLAY-021, append only, saved) opens power grids;
	// Batteries, SolarPower, WindPower and GeothermalPower each need it.
	Electricity
	Batteries
	SolarPower
	WindPower
	GeothermalPower
	// ServoReplacements (MED-05, append only, saved): the servo tier of
	// implants. It needs Prosthetics and Fabrication (the components they
	// are built from; there is no microelectronics project).
	ServoReplacements
	// Defence and trade (PLAY-024, append only, saved): Sentries opens the
	// sentry turret (needs Gunsmithing and Electricity); Radio opens the
	// radio console and the trade beacon (needs Electricity).
	Sentries
	Radio
	// Wave D (ENG-003, append only, saved; original names after the
	// wiki research tree, our costs): Metalworking (wiki: Smithing) opens
	// the forges and their blades; Bladesmithing (Long blades) the
	// longsword; Bowcraft (Recurve bow, Greatbow) the better bows;
	// MachineTools (Machining) the machining table; SelfLoading
	// (Blowback operation), GasOperated (Gas operation), RifledBarrels
	// (Precision rifling) and RotaryGuns (Multibarrel weapons) the guns;
	// Microcircuits (Microelectronics) the EMP weapons and charge;
	// FuelRefining (Biofuel refining) the refinery; Upholstery (Complex
	// furniture) the armchair; Charges (IEDs) the traps; Artillery
	// (Mortars) the mortar; HeavyTurrets (Autocannon turret) and
	// SlugTurrets (Uranium slug turret) the big guns; PoweredDoors
	// (Autodoor) the powered door.
	Metalworking
	Bladesmithing
	Bowcraft
	MachineTools
	SelfLoading
	GasOperated
	RifledBarrels
	RotaryGuns
	Microcircuits
	FuelRefining
	Upholstery
	Charges
	Artillery
	HeavyTurrets
	SlugTurrets
	PoweredDoors
	// PRD-4 (append only, saved; original names after the wiki research
	// tree, our costs): TrailRations (wiki: Pemmican), SealedRations
	// (Packaged survival meal), LeafRefining (Psychite refining) the ash
	// flake and silverdust, TonicSynthesis (Go-juice production) battle
	// tonic and reagent, PrecisionFabrication (Advanced fabrication)
	// advanced components, FlakArmor (Flak armor), Armoursmithing (Plate
	// armor) and PoweredArmor (Marine armor).
	TrailRations
	SealedRations
	LeafRefining
	TonicSynthesis
	PrecisionFabrication
	FlakArmor
	Armoursmithing
	PoweredArmor
	// NutrientPaste (STO-2, append only, saved; wiki: Nutrient paste,
	// needs Electricity, our cost) opens the hopper and the nutrient paste
	// dispenser.
	NutrientPaste
	// Hydroponics (FOOD-0, append only, saved; wiki: Hydroponics, needs
	// Electricity, our cost) opens the hydroponics basin.
	Hydroponics
	// RSCH-1 (append only, saved; wiki: Research): MultiAnalysis
	// (Multi-analyzer) opens the analyzer facility; PassiveCooling,
	// Stonecutting, AirConditioning, HospitalBeds, VitalsMonitors,
	// CarpetMaking, Greatbows and ProphylPressing (Penoxycyline
	// production) gate content that was free or under another project.
	MultiAnalysis
	PassiveCooling
	Stonecutting
	AirConditioning
	HospitalBeds
	VitalsMonitors
	CarpetMaking
	Greatbows
	ProphylPressing
	NumProjects
)

// Def is a project's static definition. Cost is the wiki base cost in
// research points (a faction below the project's tech level pays CostPct
// of it: game/research divides the points it earns instead). Unlocks names
// what it opens up, as "build:<construction kind>", "crop:<plant>" or
// "recipe:<recipe name>".
type Def struct {
	Project  Project
	Name     string
	Label    string
	Desc     string
	Cost     int
	Requires []Project
	Unlocks  []string
	// The project's TechLevel (its cost grows for a faction below it), the
	// Bench tier that can work it, and Hidden prerequisites (wiki: listed
	// in the description, never drawn in the tree; they gate like
	// Requires).
	TechLevel TechLevel
	Bench     BenchTier
	Hidden    []Project
}

// The project table (RSCH-1, ENG-003; wiki: Research): costs, tech
// levels, bench tiers and prerequisites are the wiki row each project
// maps onto (the vanilla name follows "wiki:"). Our own projects with no
// wiki row take the nearest row's numbers ("own"). Hidden prerequisites
// gate like Requires but are never drawn in the tree.

// The project table (RSCH-1, ENG-003; wiki: Research): costs, tech
// levels, bench tiers and prerequisites are the wiki row each project
// maps onto (the vanilla name follows "wiki:"). Our own projects with no
// wiki row take the nearest row's numbers ("own"). Hidden prerequisites
// gate like Requires but are never drawn in the tree.
var defs = [NumProjects]Def{
	// own: drying racks; wiki: Pemmican (Neolithic, 500), the nearest food row.
	FoodPreservation: {Project: FoodPreservation, Name: "food_preservation", Label: "Food preservation", Cost: 500, TechLevel: Neolithic,
		Desc: "Racks to dry food so it keeps.", Unlocks: []string{"build:drying rack"}},
	// own: healroot (the wiki gates no medicinal plant); wiki: Psychoid brewing (Neolithic, 500), the herbal row.
	Herbalism: {Project: Herbalism, Name: "herbalism", Label: "Herbalism", Cost: 500, TechLevel: Neolithic,
		Desc: "Grow healroot for herbal medicine.", Unlocks: []string{"crop:healroot"}},
	// wiki: Complex clothing (Medieval, 600).
	Tailoring: {Project: Tailoring, Name: "tailoring", Label: "Tailoring", Cost: 600, TechLevel: Medieval,
		Desc: "A bench to sew leather into warm clothes.", Unlocks: []string{"build:tailoring bench"}},
	// own: spike traps; wiki: Smithing (Medieval, 700).
	Traps: {Project: Traps, Name: "traps", Label: "Traps", Cost: 700, TechLevel: Medieval,
		Desc: "Spike traps that spring on raiders and predators.", Unlocks: []string{"build:spike trap"}},
	// wiki: Gunsmithing (Industrial, 500, Machining).
	Gunsmithing: {Project: Gunsmithing, Name: "gunsmithing", Label: "Gunsmithing", Cost: 500, TechLevel: Industrial,
		Requires: []Project{MachineTools},
		Desc:     "Crude rifles made at the workbench.", Unlocks: []string{"recipe:make_rifle", "recipe:make_revolver",
			"recipe:make_pump_shotgun", "recipe:make_incendiary_launcher"}},
	// wiki: Fabrication (Industrial, 4000, Multi-analyzer; hi-tech bench + analyzer).
	Fabrication: {Project: Fabrication, Name: "fabrication", Label: "Fabrication", Cost: 4000, TechLevel: Industrial,
		Bench: HiTechAnalyzer, Requires: []Project{MultiAnalysis},
		Desc:    "Machine steel into components at a fabrication bench.",
		Unlocks: []string{"recipe:fabricate_component", "build:fabrication bench"}},
	// wiki: Medicine production (Industrial, 1500, Drug production + Microelectronics; hi-tech bench).
	Pharmacology: {Project: Pharmacology, Name: "pharmacology", Label: "Pharmacology", Cost: 1500, TechLevel: Industrial,
		Bench: HiTechBench, Requires: []Project{HerbalRemedies, Microcircuits},
		Desc:    "Refine herbal medicine into stronger industrial medicine and numbroot painkiller.",
		Unlocks: []string{"recipe:refine_medicine", "recipe:make_numbroot"}}, // MED-07: the medical drugs
	// wiki: Beer brewing (Neolithic, 400).
	Brewing: {Project: Brewing, Name: "brewing", Label: "Brewing", Cost: 400, TechLevel: Neolithic,
		Desc: "Grow brewgrain and brew it into beer.", Unlocks: []string{"crop:brewgrain", "build:brewing bench", "build:fermenting barrel",
			"crop:hops"}}, // FOOD-0, wiki: Hop plant (Brewing)
	// wiki: Drug production (Industrial, 500): the drug table is the drug lab.
	HerbalRemedies: {Project: HerbalRemedies, Name: "herbal_remedies", Label: "Herbal remedies", Cost: 500, TechLevel: Industrial,
		Desc:    "Grow ashleaf and steep or press it into stimulants at a drug table.",
		Unlocks: []string{"crop:ashleaf", "build:drug table", "recipe:press_tablet"}},
	// wiki: Prosthetics (Industrial, 600, Machining).
	Prosthetics: {Project: Prosthetics, Name: "prosthetics", Label: "Prosthetics", Cost: 600, TechLevel: Industrial,
		Requires: []Project{MachineTools},
		Desc:     "Steel limbs to replace lost legs and arms.", Unlocks: []string{"recipe:make_prosthetic_leg", "recipe:make_prosthetic_arm",
			"recipe:make_glass_eye", "recipe:make_valve_heart"}},
	// own: bridges; wiki: Complex furniture (Medieval, 300).
	Bridges: {Project: Bridges, Name: "bridges", Label: "Bridges", Cost: 300, TechLevel: Medieval,
		Desc: "Timber platforms over water, marsh and mud. They hold light things, not stone walls.", Unlocks: []string{"build:bridge"}},
	// wiki: Electricity (Industrial, 1600). Lamps need power and come with
	// Electricity; the torch lamp is free, so a colony is never left
	// without a light it can build.
	Electricity: {Project: Electricity, Name: "electricity", Label: "Electricity", Cost: 1600, TechLevel: Industrial,
		Desc: "Conduits, switches and a wood-fired generator to run lamps, heaters and electric benches.",
		Unlocks: []string{"build:power conduit", "build:hidden conduit", "build:power switch", "build:wood-fired generator",
			"build:lamp", "build:heater", "build:electric stove", "build:electric tailoring bench",
			"build:electric forge", "build:electric smelter",
			"build:sun lamp"}}, // FOOD-0, wiki: Sun lamp (Electricity)
	// wiki: Battery (Industrial, 400, Electricity).
	Batteries: {Project: Batteries, Name: "batteries", Label: "Batteries", Cost: 400, TechLevel: Industrial, Requires: []Project{Electricity},
		Desc: "Store spare power for the night and for lean hours.", Unlocks: []string{"build:battery"}},
	// wiki: Solar panel (Industrial, 600, Electricity).
	SolarPower: {Project: SolarPower, Name: "solar_power", Label: "Solar power", Cost: 600, TechLevel: Industrial,
		Requires: []Project{Electricity},
		Desc:     "Panels that turn daylight into power. Nothing at night or under a roof.", Unlocks: []string{"build:solar panel"}},
	// own: wind turbines; wiki: Solar panel (Industrial, 600, Electricity).
	WindPower: {Project: WindPower, Name: "wind_power", Label: "Wind power", Cost: 600, TechLevel: Industrial, Requires: []Project{Electricity},
		Desc: "Turbines that turn with the wind. Walls, rock and trees beside them steal it.", Unlocks: []string{"build:wind turbine"}},
	// wiki: Geothermal power (Industrial, 3200, Electricity).
	GeothermalPower: {Project: GeothermalPower, Name: "geothermal_power", Label: "Geothermal power", Cost: 3200, TechLevel: Industrial,
		Requires: []Project{Electricity},
		Desc:     "A generator built over a steam vent: steady power day and night.", Unlocks: []string{"build:geothermal generator"}},
	// wiki: Bionic replacements (Spacer, 2000, Fabrication, hidden Prosthetics; hi-tech bench + analyzer).
	ServoReplacements: {Project: ServoReplacements, Name: "servo_replacements", Label: "Servo replacements", Cost: 2000, TechLevel: Spacer,
		Bench: HiTechAnalyzer, Requires: []Project{Fabrication}, Hidden: []Project{Prosthetics},
		Desc: "Powered limbs, eyes, ears, spines, hearts and stomachs that work better than the ones they replace.",
		Unlocks: []string{"recipe:make_servo_arm", "recipe:make_servo_leg", "recipe:make_optic_implant", "recipe:make_cochlear_implant",
			"recipe:make_spinal_brace", "recipe:make_pump_heart", "recipe:make_synth_stomach"}},
	// wiki: Gun turrets (Industrial, 500, Blowback operation).
	Sentries: {Project: Sentries, Name: "sentries", Label: "Sentry guns", Cost: 500, TechLevel: Industrial,
		Requires: []Project{SelfLoading},
		Desc:     "Powered gun posts that shoot raiders and hostile animals on sight.", Unlocks: []string{"build:sentry turret"}},
	// own: the radio console and trade beacon; wiki: Advanced lights (Industrial, 300, Electricity).
	Radio: {Project: Radio, Name: "radio", Label: "Radio relay", Cost: 300, TechLevel: Industrial, Requires: []Project{Electricity},
		Desc:    "A console to talk with other factions and passing ships, and a beacon to mark goods for orbital trade.",
		Unlocks: []string{"build:radio console", "build:trade beacon"}},
	// wiki: Smithing (Medieval, 700). A key listed under two projects
	// needs both (Locks names the first one not done): the electric forge
	// needs Electricity and Metalworking, the pulse charge Charges and
	// Microcircuits. The crafting and butcher spots, the sculpting bench,
	// the stool, the barricade and the sculptures need no research (wiki).
	Metalworking: {Project: Metalworking, Name: "metalworking", Label: "Metalworking", Cost: 700, TechLevel: Medieval,
		Desc: "A wood-fired forge to beat steel into maces, short swords and throwing spears.",
		Unlocks: []string{"build:wood-fired forge", "build:electric forge", "recipe:make_mace", "recipe:make_gladius", "recipe:make_pila",
			"recipe:make_simple_helmet"}}, // PRD-4: wiki Simple helmet (Smithing)
	// wiki: Long blades (Medieval, 400, Smithing).
	Bladesmithing: {Project: Bladesmithing, Name: "bladesmithing", Label: "Bladesmithing", Cost: 400, TechLevel: Medieval,
		Requires: []Project{Metalworking},
		Desc:     "The patience to forge a long, true blade.", Unlocks: []string{"recipe:make_longsword"}},
	// wiki: Recurve bow (Neolithic, 400).
	Bowcraft: {Project: Bowcraft, Name: "bowcraft", Label: "Bowcraft", Cost: 400, TechLevel: Neolithic,
		Desc: "Bent and layered bows that throw arrows harder and farther.", Unlocks: []string{"recipe:make_recurve_bow"}},
	// wiki: Machining (Industrial, 1000, Electricity + Smithing).
	MachineTools: {Project: MachineTools, Name: "machine_tools", Label: "Machine tools", Cost: 1000, TechLevel: Industrial,
		Requires: []Project{Electricity, Metalworking},
		Desc:     "A powered table of lathes and drills for gun parts and steel limbs.", Unlocks: []string{"build:machining table"}},
	// wiki: Blowback operation (Industrial, 500, Gunsmithing).
	SelfLoading: {Project: SelfLoading, Name: "self_loading", Label: "Self-loading actions", Cost: 500, TechLevel: Industrial,
		Requires: []Project{Gunsmithing},
		Desc:     "Pistols that chamber the next round from the recoil of the last.",
		Unlocks:  []string{"recipe:make_autopistol", "recipe:make_machine_pistol"}},
	// wiki: Gas operation (Industrial, 1000, Blowback operation).
	GasOperated: {Project: GasOperated, Name: "gas_operated", Label: "Gas-operated actions", Cost: 1000, TechLevel: Industrial,
		Requires: []Project{SelfLoading},
		Desc:     "Bleed the powder gas back to cycle heavier guns.",
		Unlocks:  []string{"recipe:make_chain_shotgun", "recipe:make_heavy_smg", "recipe:make_lmg"}},
	// wiki: Precision rifling (Industrial, 1400, Microelectronics + Gas operation; hi-tech bench).
	RifledBarrels: {Project: RifledBarrels, Name: "rifled_barrels", Label: "Rifled barrels", Cost: 1400, TechLevel: Industrial,
		Bench: HiTechBench, Requires: []Project{Microcircuits, GasOperated},
		Desc: "Spiral grooves cut true enough for long, accurate shots.", Unlocks: []string{"recipe:make_assault_rifle", "recipe:make_sniper_rifle",
			"recipe:make_advanced_helmet"}}, // PRD-4: wiki Flak helmet (Precision rifling)
	// wiki: Multibarrel weapons (Industrial, 2600, Microelectronics + Gas operation; hi-tech bench).
	RotaryGuns: {Project: RotaryGuns, Name: "rotary_guns", Label: "Rotary guns", Cost: 2600, TechLevel: Industrial,
		Bench: HiTechBench, Requires: []Project{Microcircuits, GasOperated},
		Desc: "A spinning cluster of barrels that never stops to cool.", Unlocks: []string{"recipe:make_minigun"}},
	// wiki: Microelectronics (Industrial, 3000, Electricity): the hi-tech
	// research bench and the EMP weapons.
	Microcircuits: {Project: Microcircuits, Name: "microcircuits", Label: "Microcircuits", Cost: 3000, TechLevel: Industrial,
		Requires: []Project{Electricity},
		Desc:     "Tiny circuits for a hi-tech research bench and for pulse weapons that knock out machines.",
		Unlocks: []string{"build:electric research bench", "recipe:make_emp_launcher", "recipe:make_emp_grenades", "build:pulse charge",
			"recipe:make_emp_shells"}},
	// wiki: Biofuel refining (Industrial, 700, Electricity).
	FuelRefining: {Project: FuelRefining, Name: "fuel_refining", Label: "Fuel refining", Cost: 700, TechLevel: Industrial,
		Requires: []Project{Electricity},
		Desc:     "A powered refinery that cooks wood and plant matter down to liquid fuel.", Unlocks: []string{"build:fuel refinery"}},
	// wiki: Complex furniture (Medieval, 300): the armchair, shelf,
	// sarcophagus, dresser, end table, tool cabinet and vent.
	Upholstery: {Project: Upholstery, Name: "upholstery", Label: "Upholstery", Cost: 300, TechLevel: Medieval,
		Desc: "Padded chairs, dressers, end tables, shelves that keep goods off the ground, tool cabinets and vents.",
		Unlocks: []string{"build:armchair", "build:shelf", "build:sarcophagus", "build:dresser", "build:end table",
			"build:tool cabinet", "build:vent"}},
	// wiki: IEDs (Industrial, 500, Electricity).
	Charges: {Project: Charges, Name: "charges", Label: "Buried charges", Cost: 500, TechLevel: Industrial, Requires: []Project{Electricity},
		Desc:    "Shells wired to a trip line, buried where raiders will walk.",
		Unlocks: []string{"build:blast charge", "build:fire charge", "build:pulse charge", "build:smoke charge"}},
	// wiki: Mortars (Industrial, 2000, Gunsmithing).
	Artillery: {Project: Artillery, Name: "artillery", Label: "Field artillery", Cost: 2000, TechLevel: Industrial,
		Requires: []Project{Gunsmithing},
		Desc:     "A crewed tube that lobs shells far past the walls.", Unlocks: []string{"build:mortar",
			"recipe:make_he_shells", "recipe:make_incendiary_shells", "recipe:make_smoke_shells"}},
	// wiki: Autocannon turret (Industrial, 1600, Microelectronics + Gun turrets + Gas operation; hi-tech bench).
	HeavyTurrets: {Project: HeavyTurrets, Name: "heavy_turrets", Label: "Heavy turrets", Cost: 1600, TechLevel: Industrial,
		Bench: HiTechBench, Requires: []Project{Microcircuits, Sentries, GasOperated},
		Desc: "A powered cannon turret that fires bursts of explosive rounds.", Unlocks: []string{"build:autocannon"}},
	// wiki: Uranium slug turret (Industrial, 3000, Multi-analyzer, hidden
	// Autocannon turret + Precision rifling; hi-tech bench + analyzer).
	SlugTurrets: {Project: SlugTurrets, Name: "slug_turrets", Label: "Slug turrets", Cost: 3000, TechLevel: Industrial,
		Bench: HiTechAnalyzer, Requires: []Project{MultiAnalysis}, Hidden: []Project{HeavyTurrets, RifledBarrels},
		Desc: "A long-barrelled turret that punches dense slugs through armor.", Unlocks: []string{"build:slug turret"}},
	// wiki: Autodoor (Industrial, 600, Electricity).
	PoweredDoors: {Project: PoweredDoors, Name: "powered_doors", Label: "Powered doors", Cost: 600, TechLevel: Industrial,
		Requires: []Project{Electricity},
		Desc:     "Doors that slide open on their own, fast, while they have power.", Unlocks: []string{"build:powered door"}},
	// wiki: Pemmican (Neolithic, 500).
	TrailRations: {Project: TrailRations, Name: "trail_rations", Label: "Trail rations", Cost: 500, TechLevel: Neolithic,
		Desc: "Pound dried meat and plants into dense cakes that keep for months.", Unlocks: []string{"recipe:cook_pemmican"}},
	// wiki: Packaged survival meal (Industrial, 500, Nutrient paste).
	SealedRations: {Project: SealedRations, Name: "sealed_rations", Label: "Sealed rations", Cost: 500, TechLevel: Industrial,
		Requires: []Project{NutrientPaste},
		Desc:     "Cooked meals sealed in packs that never spoil.", Unlocks: []string{"recipe:cook_survival_meal"}},
	// wiki: Psychite refining (Industrial, 400, Drug production).
	LeafRefining: {Project: LeafRefining, Name: "leaf_refining", Label: "Leaf refining", Cost: 400, TechLevel: Industrial,
		Requires: []Project{HerbalRemedies},
		Desc:     "Refine ashleaf into flake and the purer, stronger silverdust.", Unlocks: []string{"recipe:make_ash_flake", "recipe:make_silverdust"}},
	// wiki: Go-juice (Industrial, 1000, Drug production).
	TonicSynthesis: {Project: TonicSynthesis, Name: "tonic_synthesis", Label: "Tonic synthesis", Cost: 1000, TechLevel: Industrial,
		Requires: []Project{HerbalRemedies},
		Desc:     "Distil a chemical reagent and blend it into a tonic that lets a fighter shrug off pain and fatigue.",
		Unlocks:  []string{"recipe:make_battle_tonic", "recipe:make_reagent"}},
	// wiki: Advanced fabrication (Industrial, 4000, Multi-analyzer; hi-tech bench + analyzer).
	PrecisionFabrication: {Project: PrecisionFabrication, Name: "precision_fabrication", Label: "Precision fabrication", Cost: 4000,
		TechLevel: Industrial, Bench: HiTechAnalyzer, Requires: []Project{MultiAnalysis},
		Desc: "Layer plasteel, gold and components into advanced components.", Unlocks: []string{"recipe:fabricate_advanced"}},
	// wiki: Flak armor (Industrial, 1200, Machining, hidden Plate armor).
	FlakArmor: {Project: FlakArmor, Name: "flak_armor", Label: "Flak armor", Cost: 1200, TechLevel: Industrial,
		Requires: []Project{MachineTools}, Hidden: []Project{Armoursmithing},
		Desc:    "Vests, jackets and trousers of steel plates sewn into cloth.",
		Unlocks: []string{"recipe:make_flak_vest", "recipe:make_flak_pants", "recipe:make_flak_jacket"}},
	// wiki: Plate armor (Medieval, 600, Smithing + Complex clothing).
	Armoursmithing: {Project: Armoursmithing, Name: "armoursmithing", Label: "Armoursmithing", Cost: 600, TechLevel: Medieval,
		Requires: []Project{Metalworking, Tailoring},
		Desc:     "Beat a full suit of plates for the whole body.", Unlocks: []string{"recipe:make_plate_armor"}},
	// wiki: Marine armor (Spacer, 6000; hi-tech bench + analyzer). The
	// recon armor step is not in the game, so its prerequisites stand in:
	// Fabrication, hidden Complex clothing.
	PoweredArmor: {Project: PoweredArmor, Name: "powered_armor", Label: "Powered armor", Cost: 6000, TechLevel: Spacer,
		Bench: HiTechAnalyzer, Requires: []Project{Fabrication}, Hidden: []Project{Tailoring},
		Desc:    "Sealed plasteel suits and helmets with their own servos.",
		Unlocks: []string{"recipe:make_marine_armor", "recipe:make_marine_helmet"}},
	// wiki: Nutrient paste (Industrial, 400, Electricity).
	NutrientPaste: {Project: NutrientPaste, Name: "nutrient_paste", Label: "Nutrient paste", Cost: 400, TechLevel: Industrial,
		Requires: []Project{Electricity},
		Desc:     "A powered machine that mashes raw food from the hoppers beside it into bland but filling paste meals.",
		Unlocks:  []string{"build:hopper", "build:nutrient paste dispenser"}},
	// wiki: Hydroponics (Industrial, 700, Electricity).
	Hydroponics: {Project: Hydroponics, Name: "hydroponics", Label: "Hydroponics", Cost: 700, TechLevel: Industrial,
		Requires: []Project{Electricity},
		Desc:     "Powered basins of nutrient-rich water that grow crops far faster than soil, indoors under a sun lamp.",
		Unlocks:  []string{"build:hydroponics basin"}},
	// wiki: Multi-analyzer (Industrial, 4000, Microelectronics, hidden Machining; hi-tech bench).
	MultiAnalysis: {Project: MultiAnalysis, Name: "multi_analyzer", Label: "Multi-analyzer", Cost: 4000, TechLevel: Industrial,
		Bench: HiTechBench, Requires: []Project{Microcircuits}, Hidden: []Project{MachineTools},
		Desc:    "A bank of sensors that a nearby hi-tech research bench links to, for faster research and the hardest projects.",
		Unlocks: []string{"build:multi-analyzer"}},
	// wiki: Passive cooler (Neolithic, 400).
	PassiveCooling: {Project: PassiveCooling, Name: "passive_cooling", Label: "Passive cooling", Cost: 400, TechLevel: Neolithic,
		Desc:    "A wooden frame of wet straw that cools a room as the water evaporates, with no power at all.",
		Unlocks: []string{"build:passive cooler"}},
	// wiki: Stonecutting (Medieval, 300).
	Stonecutting: {Project: Stonecutting, Name: "stonecutting", Label: "Stonecutting", Cost: 300, TechLevel: Medieval,
		Desc: "A table to saw rough chunks of rock into blocks fit for walls and floors.", Unlocks: []string{"build:stonecutter table"}},
	// wiki: Air conditioning (Industrial, 500, Electricity).
	AirConditioning: {Project: AirConditioning, Name: "air_conditioning", Label: "Air conditioning", Cost: 500, TechLevel: Industrial,
		Requires: []Project{Electricity},
		Desc:     "A powered cooler set in a wall that pumps heat out of a room.", Unlocks: []string{"build:cooler"}},
	// wiki: Hospital bed (Industrial, 1200, Microelectronics, hidden
	// Complex furniture; hi-tech bench). Its hidden Sterile materials is
	// not in the game.
	HospitalBeds: {Project: HospitalBeds, Name: "hospital_beds", Label: "Hospital beds", Cost: 1200, TechLevel: Industrial,
		Bench: HiTechBench, Requires: []Project{Microcircuits}, Hidden: []Project{Upholstery},
		Desc: "Adjustable beds with rails and clean linen, where the sick and wounded heal faster.", Unlocks: []string{"build:hospital bed"}},
	// wiki: Vitals monitor (Industrial, 2500, Multi-analyzer + Hospital bed; hi-tech bench + analyzer).
	VitalsMonitors: {Project: VitalsMonitors, Name: "vitals_monitors", Label: "Vitals monitors", Cost: 2500, TechLevel: Industrial,
		Bench: HiTechAnalyzer, Requires: []Project{MultiAnalysis, HospitalBeds},
		Desc:    "A bedside machine that watches a patient's heart and breath, so doctors catch trouble early.",
		Unlocks: []string{"build:vitals monitor"}},
	// wiki: Carpet making (Medieval, 800).
	CarpetMaking: {Project: CarpetMaking, Name: "carpet_making", Label: "Carpet making", Cost: 800, TechLevel: Medieval,
		Desc: "Weave and sew soft carpets that make a floor pleasant to walk on.",
		Unlocks: []string{"build:carpet", "recipe:make_straw_carpet", "recipe:make_wood_carpet", "recipe:sew_cloth_carpet",
			"recipe:sew_leather_carpet", "recipe:weave_wool_carpet"}},
	// wiki: Greatbow (Medieval, 600, Recurve bow).
	Greatbows: {Project: Greatbows, Name: "greatbows", Label: "Greatbows", Cost: 600, TechLevel: Medieval, Requires: []Project{Bowcraft},
		Desc: "A bow as tall as its archer, slow to draw and deadly at range.", Unlocks: []string{"recipe:make_greatbow"}},
	// wiki: Penoxycyline production (Industrial, 500, Drug production).
	ProphylPressing: {Project: ProphylPressing, Name: "prophyl_pressing", Label: "Prophyl pressing", Cost: 500, TechLevel: Industrial,
		Requires: []Project{HerbalRemedies},
		Desc:     "Press prophyl tablets that keep the body from catching the common sicknesses.", Unlocks: []string{"recipe:press_prophyl"}},
}

// DefOf returns the definition of p.
func DefOf(p Project) Def {
	if p >= NumProjects {
		return Def{}
	}
	return defs[p]
}

// All lists every project in display order.
func All() []Def { return append([]Def(nil), defs[:]...) }

// Parse finds a project by name.
func Parse(name string) (Project, bool) {
	for _, d := range defs {
		if d.Name == name {
			return d.Project, true
		}
	}
	return 0, false
}

// Progress is the colony's research state: points put into each project.
type Progress [NumProjects]int

// Done reports p finished.
func (pr Progress) Done(p Project) bool { return p < NumProjects && pr[p] >= DefOf(p).Cost }

// Available reports p can be researched now: not done, prerequisites
// (drawn and hidden) done.
func (pr Progress) Available(p Project) bool {
	if p >= NumProjects || pr.Done(p) {
		return false
	}
	d := DefOf(p)
	for _, rs := range [2][]Project{d.Requires, d.Hidden} {
		for _, r := range rs {
			if !pr.Done(r) {
				return false
			}
		}
	}
	return true
}

// Locks returns the project that must be finished before unlock key is
// usable (ok=false if nothing gates it or it is already researched).
func (pr Progress) Locks(key string) (Project, bool) {
	for _, d := range defs {
		for _, u := range d.Unlocks {
			if u == key && !pr.Done(d.Project) {
				return d.Project, true
			}
		}
	}
	return 0, false
}

// Add puts n points into p, reporting whether that finished it.
func (pr *Progress) Add(p Project, n int) bool {
	if p >= NumProjects || pr.Done(p) {
		return false
	}
	pr[p] = min(DefOf(p).Cost, pr[p]+n)
	return pr.Done(p)
}

// V1Projects is how many projects a save from before NG-1 holds.
const V1Projects = 5

// V1 is the research array of a version-1 save: gob only decodes an array
// of the exact length it was written with.
type V1 [V1Projects]int

// V1 is the first V1Projects entries, for a save field old saves decode.
func (pr Progress) V1() V1 {
	var v V1
	copy(v[:], pr[:])
	return v
}

// FromV1 is the progress of a version-1 save (later projects at zero).
func FromV1(v V1) Progress { return FromSlice(v[:]) }

// Slice is the progress as a plain slice, indexed by Project. Saves should
// store this rather than the fixed-size array: gob refuses an array whose
// length changed, and appending a project changes NumProjects.
func (pr Progress) Slice() []int { return append([]int(nil), pr[:]...) }

// FromSlice rebuilds progress from Slice's output; projects missing from an
// older, shorter slice start at zero and extra entries are ignored.
func FromSlice(pts []int) Progress {
	var pr Progress
	copy(pr[:], pts)
	return pr
}

// All researched, for sandboxes and tests.
func Complete() Progress {
	var pr Progress
	for _, d := range defs {
		pr[d.Project] = d.Cost
	}
	return pr
}
