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
		Desc: "Crude rifles made at the workbench.", Unlocks: []string{"recipe:make_rifle"}},
	Fabrication: {Project: Fabrication, Name: "fabrication", Label: "Fabrication", Cost: 5000, Requires: []Project{Gunsmithing},
		Desc:    "Machine steel into components for advanced furniture.",
		Unlocks: []string{"recipe:fabricate_component", "build:tool cabinet", "build:hospital bed"}},
	Pharmacology: {Project: Pharmacology, Name: "pharmacology", Label: "Pharmacology", Cost: 4000, Requires: []Project{Herbalism},
		Desc:    "Refine herbal medicine into stronger industrial medicine, numbroot painkiller and prophyl tablets.",
		Unlocks: []string{"recipe:refine_medicine", "recipe:make_numbroot", "recipe:press_prophyl"}}, // MED-07: the medical drugs
	Brewing: {Project: Brewing, Name: "brewing", Label: "Brewing", Cost: 2500,
		Desc: "Grow brewgrain and brew it into beer.", Unlocks: []string{"crop:brewgrain", "build:brewing bench"}},
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
			"build:electric research bench"}},
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
