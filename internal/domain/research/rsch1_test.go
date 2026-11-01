package research

import (
	"slices"
	"testing"
)

// TestRsch1WikiRows (RSCH-1, wiki: Research): spot rows of the table carry
// the wiki cost, tech level, bench tier and prerequisites of the vanilla
// project they map onto.
func TestRsch1WikiRows(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		p        Project
		cost     int
		level    TechLevel
		bench    BenchTier
		req, hid []Project
	}{
		{Electricity, 1600, Industrial, SimpleBench, nil, nil},
		{Microcircuits, 3000, Industrial, SimpleBench, []Project{Electricity}, nil},
		{MultiAnalysis, 4000, Industrial, HiTechBench, []Project{Microcircuits}, []Project{MachineTools}},
		{Fabrication, 4000, Industrial, HiTechAnalyzer, []Project{MultiAnalysis}, nil},
		{SlugTurrets, 3000, Industrial, HiTechAnalyzer, []Project{MultiAnalysis}, []Project{HeavyTurrets, RifledBarrels}},
		{ServoReplacements, 2000, Spacer, HiTechAnalyzer, []Project{Fabrication}, []Project{Prosthetics}},
		{HospitalBeds, 1200, Industrial, HiTechBench, []Project{Microcircuits}, []Project{Upholstery}},
		{Armoursmithing, 600, Medieval, SimpleBench, []Project{Metalworking, Tailoring}, nil},
		{Bowcraft, 400, Neolithic, SimpleBench, nil, nil},
		{Greatbows, 600, Medieval, SimpleBench, []Project{Bowcraft}, nil},
		{PassiveCooling, 400, Neolithic, SimpleBench, nil, nil},
		{Gunsmithing, 500, Industrial, SimpleBench, []Project{MachineTools}, nil},
	} {
		d := DefOf(c.p)
		if d.Cost != c.cost || d.TechLevel != c.level || d.Bench != c.bench || !slices.Equal(d.Requires, c.req) ||
			!slices.Equal(d.Hidden, c.hid) {
			t.Errorf("%s = cost %d %s %s req %v hidden %v", d.Name, d.Cost, d.TechLevel, d.Bench, d.Requires, d.Hidden)
		}
	}
	for _, d := range All() {
		if d.TechLevel == Spacer && d.Bench != HiTechAnalyzer {
			t.Errorf("%s: every spacer project needs the analyzer", d.Name)
		}
		if d.Label == "" || d.Desc == "" || d.Cost <= 0 || len(d.Unlocks) == 0 {
			t.Errorf("%s: incomplete row %+v", d.Name, d)
		}
	}
}

// TestRsch1CostPct (wiki: Research): a tribe pays 1.5x for Medieval and 2x
// for Industrial and later projects; an Industrial faction pays base for
// everything, Spacer included.
func TestRsch1CostPct(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		project, faction TechLevel
		want             int
	}{
		{Neolithic, Neolithic, 100}, {Medieval, Neolithic, 150}, {Industrial, Neolithic, 200}, {Spacer, Neolithic, 200},
		{Neolithic, Industrial, 100}, {Industrial, Industrial, 100}, {Spacer, Industrial, 100}, {Spacer, Spacer, 100},
	} {
		if got := CostPct(c.project, c.faction); got != c.want {
			t.Errorf("%s project, %s faction: %d%%, want %d%%", c.project, c.faction, got, c.want)
		}
	}
}

// TestRsch1HiddenGates: a hidden prerequisite gates availability like a
// drawn one.
func TestRsch1HiddenGates(t *testing.T) {
	t.Parallel()
	var pr Progress
	pr.Add(Electricity, DefOf(Electricity).Cost)
	pr.Add(Microcircuits, DefOf(Microcircuits).Cost)
	if pr.Available(MultiAnalysis) {
		t.Fatal("the multi-analyzer needs machine tools (hidden) too")
	}
	pr.Add(Metalworking, DefOf(Metalworking).Cost)
	pr.Add(MachineTools, DefOf(MachineTools).Cost)
	if !pr.Available(MultiAnalysis) {
		t.Fatal("all prerequisites done")
	}
}

// TestRsch1Acyclic: no project needs itself through any chain of drawn or
// hidden prerequisites, and every prerequisite is a real project.
func TestRsch1Acyclic(t *testing.T) {
	t.Parallel()
	const (
		unseen = iota
		open
		closed
	)
	var state [NumProjects]int
	var visit func(p Project) bool
	visit = func(p Project) bool {
		switch state[p] {
		case open:
			return false
		case closed:
			return true
		}
		state[p] = open
		d := DefOf(p)
		for _, r := range slices.Concat(d.Requires, d.Hidden) {
			if r >= NumProjects || !visit(r) {
				return false
			}
		}
		state[p] = closed
		return true
	}
	for p := Project(0); p < NumProjects; p++ {
		if !visit(p) {
			t.Fatalf("%s sits on a prerequisite cycle", DefOf(p).Name)
		}
	}
}
