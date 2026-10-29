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
	// Fabrication: components at the workbench, tool cabinets and hospital
	// beds (NG-1 §4.8).
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
	NumProjects
)

// Def is a project's static definition. Cost is research work (one point
// per working tick at a research bench). Unlocks names what it opens up,
// as "build:<construction kind>", "crop:<plant>" or "recipe:<recipe name>".
type Def struct {
	Project  Project
	Name     string
	Label    string
	Desc     string
	Cost     int
	Requires []Project
	Unlocks  []string
}

var defs = [NumProjects]Def{
	FoodPreservation: {Project: FoodPreservation, Name: "food_preservation", Label: "Food preservation", Cost: 2000,
		Desc: "Racks to dry food so it keeps.", Unlocks: []string{"build:drying rack"}},
	Herbalism: {Project: Herbalism, Name: "herbalism", Label: "Herbalism", Cost: 2500,
		Desc: "Grow healroot for herbal medicine.", Unlocks: []string{"crop:healroot"}},
	Tailoring: {Project: Tailoring, Name: "tailoring", Label: "Tailoring", Cost: 3000,
		Desc: "A bench to sew leather into warm clothes.", Unlocks: []string{"build:tailoring bench"}},
	Traps: {Project: Traps, Name: "traps", Label: "Traps", Cost: 2500,
		Desc: "Spike traps that spring on raiders and predators.", Unlocks: []string{"build:spike trap"}},
	Gunsmithing: {Project: Gunsmithing, Name: "gunsmithing", Label: "Gunsmithing", Cost: 6000, Requires: []Project{Traps},
		Desc: "Crude rifles made at the workbench.", Unlocks: []string{"recipe:make_rifle", "recipe:make_revolver",
			"recipe:make_pump_shotgun", "recipe:make_incendiary_launcher"}},
	Fabrication: {Project: Fabrication, Name: "fabrication", Label: "Fabrication", Cost: 5000, Requires: []Project{Gunsmithing},
		Desc: "Machine steel into components for advanced furniture.",
		Unlocks: []string{"recipe:fabricate_component", "build:tool cabinet", "build:hospital bed", "build:vitals monitor",
			"build:fabrication bench"}},
	Pharmacology: {Project: Pharmacology, Name: "pharmacology", Label: "Pharmacology", Cost: 4000, Requires: []Project{Herbalism},
		Desc:    "Refine herbal medicine into stronger industrial medicine, numbroot painkiller and prophyl tablets.",
		Unlocks: []string{"recipe:refine_medicine", "recipe:make_numbroot", "recipe:press_prophyl"}}, // MED-07: the medical drugs
	Brewing: {Project: Brewing, Name: "brewing", Label: "Brewing", Cost: 2500,
		Desc: "Grow brewgrain and brew it into beer.", Unlocks: []string{"crop:brewgrain", "build:brewing bench", "build:fermenting barrel"}},
	HerbalRemedies: {Project: HerbalRemedies, Name: "herbal_remedies", Label: "Herbal remedies", Cost: 3500,
		Requires: []Project{Herbalism},
		Desc:     "Grow ashleaf and steep or press it into stimulants.",
		Unlocks:  []string{"crop:ashleaf", "build:drug table", "recipe:press_tablet"}},
	Prosthetics: {Project: Prosthetics, Name: "prosthetics", Label: "Prosthetics", Cost: 4000, Requires: []Project{Herbalism},
		Desc: "Steel limbs to replace lost legs and arms.", Unlocks: []string{"recipe:make_prosthetic_leg", "recipe:make_prosthetic_arm", "recipe:make_glass_eye",
			"recipe:make_valve_heart"}},
	Bridges: {Project: Bridges, Name: "bridges", Label: "Bridges", Cost: 1500,
		Desc: "Timber platforms over water, marsh and mud. They hold light things, not stone walls.", Unlocks: []string{"build:bridge"}},
	// PLAY-021 (our costs). Lamps need power and come with Electricity;
	// the torch lamp is free, so a colony is never left without a light
	// it can build.
	Electricity: {Project: Electricity, Name: "electricity", Label: "Electricity", Cost: 3000,
		Desc: "Conduits, switches and a wood-fired generator to run lamps, heaters, coolers and electric benches.",
		Unlocks: []string{"build:power conduit", "build:hidden conduit", "build:power switch", "build:wood-fired generator",
			"build:lamp", "build:heater", "build:cooler", "build:electric stove", "build:electric tailoring bench",
			"build:electric research bench", "build:electric forge", "build:electric smelter"}},
	Batteries: {Project: Batteries, Name: "batteries", Label: "Batteries", Cost: 2500, Requires: []Project{Electricity},
		Desc: "Store spare power for the night and for lean hours.", Unlocks: []string{"build:battery"}},
	SolarPower: {Project: SolarPower, Name: "solar_power", Label: "Solar power", Cost: 4000, Requires: []Project{Electricity},
		Desc: "Panels that turn daylight into power. Nothing at night or under a roof.", Unlocks: []string{"build:solar panel"}},
	WindPower: {Project: WindPower, Name: "wind_power", Label: "Wind power", Cost: 3500, Requires: []Project{Electricity},
		Desc: "Turbines that turn with the wind. Walls, rock and trees beside them steal it.", Unlocks: []string{"build:wind turbine"}},
	GeothermalPower: {Project: GeothermalPower, Name: "geothermal_power", Label: "Geothermal power", Cost: 5000,
		Requires: []Project{Electricity},
		Desc:     "A generator built over a steam vent: steady power day and night.", Unlocks: []string{"build:geothermal generator"}},
	// MED-05 (our cost).
	ServoReplacements: {Project: ServoReplacements, Name: "servo_replacements", Label: "Servo replacements", Cost: 6000,
		Requires: []Project{Prosthetics, Fabrication},
		Desc:     "Powered limbs, eyes, ears, spines, hearts and stomachs that work better than the ones they replace.",
		Unlocks: []string{"recipe:make_servo_arm", "recipe:make_servo_leg", "recipe:make_optic_implant", "recipe:make_cochlear_implant",
			"recipe:make_spinal_brace", "recipe:make_pump_heart", "recipe:make_synth_stomach"}},
	// PLAY-024 (our costs).
	Sentries: {Project: Sentries, Name: "sentries", Label: "Sentry guns", Cost: 4500,
		Requires: []Project{Gunsmithing, Electricity},
		Desc:     "Powered gun posts that shoot raiders and hostile animals on sight.", Unlocks: []string{"build:sentry turret"}},
	Radio: {Project: Radio, Name: "radio", Label: "Radio relay", Cost: 3000, Requires: []Project{Electricity},
		Desc:    "A console to talk with other factions and passing ships, and a beacon to mark goods for orbital trade.",
		Unlocks: []string{"build:radio console", "build:trade beacon"}},
	// Wave D (our costs). A key listed under two projects needs both
	// (Locks names the first one not done): the electric forge needs
	// Electricity and Metalworking, the pulse charge Charges and
	// Microcircuits. The crafting and butcher spots, the sculpting bench,
	// the stool, the barricade and the sculptures need no research (wiki).
	Metalworking: {Project: Metalworking, Name: "metalworking", Label: "Metalworking", Cost: 3000,
		Desc:    "A wood-fired forge to beat steel into maces, short swords and throwing spears.",
		Unlocks: []string{"build:wood-fired forge", "build:electric forge", "recipe:make_mace", "recipe:make_gladius", "recipe:make_pila"}},
	Bladesmithing: {Project: Bladesmithing, Name: "bladesmithing", Label: "Bladesmithing", Cost: 3500, Requires: []Project{Metalworking},
		Desc: "The patience to forge a long, true blade.", Unlocks: []string{"recipe:make_longsword"}},
	Bowcraft: {Project: Bowcraft, Name: "bowcraft", Label: "Bowcraft", Cost: 2500,
		Desc: "Bent and layered bows that throw arrows harder and farther.", Unlocks: []string{"recipe:make_recurve_bow", "recipe:make_greatbow"}},
	MachineTools: {Project: MachineTools, Name: "machine_tools", Label: "Machine tools", Cost: 4000,
		Requires: []Project{Metalworking, Electricity},
		Desc:     "A powered table of lathes and drills for gun parts and steel limbs.", Unlocks: []string{"build:machining table"}},
	SelfLoading: {Project: SelfLoading, Name: "self_loading", Label: "Self-loading actions", Cost: 4000,
		Requires: []Project{MachineTools, Gunsmithing},
		Desc:     "Pistols that chamber the next round from the recoil of the last.",
		Unlocks:  []string{"recipe:make_autopistol", "recipe:make_machine_pistol"}},
	GasOperated: {Project: GasOperated, Name: "gas_operated", Label: "Gas-operated actions", Cost: 5000, Requires: []Project{SelfLoading},
		Desc:    "Bleed the powder gas back to cycle heavier guns.",
		Unlocks: []string{"recipe:make_chain_shotgun", "recipe:make_heavy_smg", "recipe:make_lmg"}},
	RifledBarrels: {Project: RifledBarrels, Name: "rifled_barrels", Label: "Rifled barrels", Cost: 6000, Requires: []Project{GasOperated},
		Desc: "Spiral grooves cut true enough for long, accurate shots.", Unlocks: []string{"recipe:make_assault_rifle", "recipe:make_sniper_rifle"}},
	RotaryGuns: {Project: RotaryGuns, Name: "rotary_guns", Label: "Rotary guns", Cost: 7000, Requires: []Project{GasOperated},
		Desc: "A spinning cluster of barrels that never stops to cool.", Unlocks: []string{"recipe:make_minigun"}},
	Microcircuits: {Project: Microcircuits, Name: "microcircuits", Label: "Microcircuits", Cost: 6000, Requires: []Project{Fabrication},
		Desc: "Tiny circuits for pulse weapons that knock out machines.", Unlocks: []string{"recipe:make_emp_launcher",
			"recipe:make_emp_grenades", "build:pulse charge"}},
	FuelRefining: {Project: FuelRefining, Name: "fuel_refining", Label: "Fuel refining", Cost: 3500, Requires: []Project{Electricity},
		Desc: "A powered refinery that cooks wood and plant matter down to liquid fuel.", Unlocks: []string{"build:fuel refinery"}},
	Upholstery: {Project: Upholstery, Name: "upholstery", Label: "Upholstery", Cost: 2000,
		Desc: "Padded, fabric-covered chairs fit for a long evening.", Unlocks: []string{"build:armchair"}},
	Charges: {Project: Charges, Name: "charges", Label: "Buried charges", Cost: 3000, Requires: []Project{Traps},
		Desc:    "Shells wired to a trip line, buried where raiders will walk.",
		Unlocks: []string{"build:blast charge", "build:fire charge", "build:pulse charge", "build:smoke charge"}},
	Artillery: {Project: Artillery, Name: "artillery", Label: "Field artillery", Cost: 5000, Requires: []Project{Gunsmithing},
		Desc: "A crewed tube that lobs shells far past the walls.", Unlocks: []string{"build:mortar"}},
	HeavyTurrets: {Project: HeavyTurrets, Name: "heavy_turrets", Label: "Heavy turrets", Cost: 6000, Requires: []Project{Sentries},
		Desc: "A powered cannon turret that fires bursts of explosive rounds.", Unlocks: []string{"build:autocannon"}},
	SlugTurrets: {Project: SlugTurrets, Name: "slug_turrets", Label: "Slug turrets", Cost: 7000, Requires: []Project{HeavyTurrets},
		Desc: "A long-barrelled turret that punches dense slugs through armor.", Unlocks: []string{"build:slug turret"}},
	PoweredDoors: {Project: PoweredDoors, Name: "powered_doors", Label: "Powered doors", Cost: 2500, Requires: []Project{Electricity},
		Desc: "Doors that slide open on their own, fast, while they have power.", Unlocks: []string{"build:powered door"}},
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

// Available reports p can be researched now: not done, prerequisites done.
func (pr Progress) Available(p Project) bool {
	if p >= NumProjects || pr.Done(p) {
		return false
	}
	for _, r := range DefOf(p).Requires {
		if !pr.Done(r) {
			return false
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
