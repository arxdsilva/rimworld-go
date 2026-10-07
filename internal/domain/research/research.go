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

// All researched, for sandboxes and tests.
func Complete() Progress {
	var pr Progress
	for _, d := range defs {
		pr[d.Project] = d.Cost
	}
	return pr
}
