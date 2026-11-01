package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/world/terrain"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
)

// TestRschContractLevelsAndTiers (RSCH-0): the research tech scale names
// its five levels in order and maps each faction level onto it; the bench
// tiers name what they ask for; every project's level, tier and hidden
// prerequisites are in range (hidden ones never repeat a drawn one).
func TestRschContractLevelsAndTiers(t *testing.T) {
	t.Parallel()
	want := []string{"Neolithic", "Medieval", "Industrial", "Spacer", "Ultra"}
	for l := research.TechLevel(0); l < research.NumTechLevels; l++ {
		if l.String() != want[l] {
			t.Errorf("level %d = %q, want %q", l, l, want[l])
		}
	}
	if research.NumTechLevels.String() != "" || !(research.Neolithic < research.Medieval && research.Medieval < research.Industrial &&
		research.Industrial < research.Spacer && research.Spacer < research.Ultra) {
		t.Error("the tech scale is not ordered")
	}
	for f, l := range map[faction.Tech]research.TechLevel{faction.Neolithic: research.Neolithic,
		faction.Industrial: research.Industrial, faction.Spacer: research.Spacer} {
		if research.LevelOf(f) != l {
			t.Errorf("faction tech %d maps to %s, want %s", f, research.LevelOf(f), l)
		}
	}
	if research.SimpleBench != 0 || research.HiTechBench.String() != "hi-tech research bench" ||
		research.HiTechAnalyzer.String() != "hi-tech research bench + multi-analyzer" || research.NumBenchTiers.String() != "" {
		t.Error("bench tiers: the zero tier is any bench; the others name the hi-tech bench and its analyzer")
	}
	for _, d := range research.All() {
		if d.TechLevel >= research.NumTechLevels || d.Bench >= research.NumBenchTiers {
			t.Errorf("%s: level %d, bench %d", d.Name, d.TechLevel, d.Bench)
		}
		for _, h := range d.Hidden {
			if h >= research.NumProjects || h == d.Project || slices.Contains(d.Requires, h) {
				t.Errorf("%s: hidden prerequisite %d", d.Name, h)
			}
		}
	}
}

// TestRschContractMultiAnalyzer (RSCH-0): the multi-analyzer is a valid
// 2x2 steel kind with its wiki components, which the Architect does not
// offer yet; the electric research bench keeps its kind and key (the
// hi-tech research bench) and still works research bench projects.
func TestRschContractMultiAnalyzer(t *testing.T) {
	t.Parallel()
	k := construction.MultiAnalyzer
	d := construction.DefOf(k)
	if got, ok := construction.Parse("multi-analyzer"); !ok || got != k || d.Size != [2]int{2, 2} || !d.MetalOnly ||
		d.Components != 8 || d.Wood <= 0 || d.Work <= 0 || !d.BlocksMove || d.Bench || construction.MaxHP(k, item.Steel) <= 0 {
		t.Errorf("multi-analyzer: %+v", d)
	}
	if construction.ElectricResearchBench.String() != "electric research bench" ||
		construction.RecipeBench(construction.ElectricResearchBench) != construction.ResearchBench {
		t.Error("the hi-tech research bench is the electric research bench kind")
	}
}

// TestRsch1DevFinishAndViews (RSCH-1): DevFinishResearch finishes the
// current project at once and drops it; the views carry the tech level,
// bench tier, cost now and the alert; a research bench's view carries its
// factor and its analyzer.
func TestRsch1DevFinishAndViews(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 11, Width: 40, Height: 30})
	g.ResetResearch()
	bench := buildNow(t, g, construction.ElectricResearchBench, item.Steel, construction.South, 4, 4)
	an := buildNow(t, g, construction.MultiAnalyzer, item.Steel, construction.South, 10, 4)
	if _, err := g.SpawnPawn(20, 20); err != nil {
		t.Fatal(err)
	}
	w := g.World()
	if !w.ResearchAlert || w.TechLevel != "Industrial" {
		t.Errorf("world: alert %v, tech level %q", w.ResearchAlert, w.TechLevel)
	}
	if r := w.Research[research.Fabrication]; r.TechLevel != "Industrial" || r.Bench != research.HiTechAnalyzer.String() ||
		r.CostNow != 4000 || r.Available {
		t.Errorf("fabrication view %+v", r)
	}
	if r := w.Research[research.ServoReplacements]; len(r.Hidden) != 1 || r.Hidden[0] != research.DefOf(research.Prosthetics).Label {
		t.Errorf("hidden prerequisites %v", r.Hidden)
	}
	for _, b := range w.Buildings {
		if b.ID == bench && (b.Analyzer != uint64(an) || b.ResearchSpeed <= 0) {
			t.Errorf("hi-tech bench view: analyzer %d (want %d), factor %d", b.Analyzer, an, b.ResearchSpeed)
		}
	}
	g.PickResearch("traps")
	if g.World().ResearchAlert {
		t.Error("a project is picked: no alert")
	}
	g.Enqueue(command.Command{Kind: command.DevFinishResearch})
	g.ApplyPending()
	if !g.lab.Prog.Done(research.Traps) || g.lab.Set || !g.World().ResearchAlert {
		t.Fatal("dev finish: the project is done and dropped, the alert is back")
	}
	letter := false
	for _, m := range g.World().Messages {
		letter = letter || m.Letter && m.Title == "Research complete: Traps"
	}
	if !letter {
		t.Fatal("dev finish posts the research-complete letter, as bench work does")
	}
}

// TestRsch1BenchGatingInGame (RSCH-1, wiki: Research): with only a simple
// bench nobody starts a hi-tech project; a powered hi-tech bench takes it.
func TestRsch1BenchGatingInGame(t *testing.T) {
	t.Parallel()
	g, ada := researchGame(t, construction.ResearchBench, false)
	g.lab.Prog = research.Complete()
	g.lab.Prog[research.RifledBarrels] = 0
	g.PickResearch("rifled_barrels")
	for i := 0; i < 20; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(100)
		if j, _ := g.jobs.Get(ada); j.Kind == job.Research {
			t.Fatal("a hi-tech project at a simple bench")
		}
	}
	if m := g.World().Research[research.RifledBarrels].BenchMissing; m == "" {
		t.Fatal("the view says what is missing")
	}
	h, hada := researchGame(t, construction.ElectricResearchBench, true)
	h.lab.Prog = research.Complete()
	h.lab.Prog[research.RifledBarrels] = 0
	h.PickResearch("rifled_barrels")
	researchPerTick(t, h, hada, 50)
	if h.lab.Prog[research.RifledBarrels] == 0 && h.lab.Carry == 0 {
		t.Fatal("the hi-tech bench works the hi-tech project")
	}
}

// TestRsch1SaveMigration (RSCH-1): a v8 save keeps each project's
// fraction of its cost under the wiki costs; a research bench whose
// larger footprint still fits keeps its place and blocks its new cells,
// one that no longer fits is taken down and refunded with a message; a
// blueprint is cancelled.
func TestRsch1SaveMigration(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 30, Height: 20})
	g.ResetResearch()
	keep := buildNow(t, g, construction.ResearchBench, item.Wood, construction.South, 4, 4)
	lost := buildNow(t, g, construction.ElectricResearchBench, item.Steel, construction.South, 12, 4)
	g.terr.Set(16, 5, terrain.Rock)         // inside the 5x2 bench, outside its old 2x1 cells
	g.lab.Prog[research.Electricity] = 1500 // half the old 3000
	g.lab.Prog[research.Traps] = 2500       // done at the old cost
	g.placeFrameStuff(construction.ResearchBench, item.Wood, construction.South, 20, 10)
	if len(g.frameIDs) != 1 {
		t.Fatal("blueprint")
	}
	l := loadAsVersion(t, g, 8)
	if len(l.frameIDs) != 0 {
		t.Fatal("a research bench blueprint is cancelled: placed again at the new size")
	}
	if l.lab.Prog[research.Electricity] != 800 || !l.lab.Prog.Done(research.Traps) {
		t.Fatalf("progress %d, traps done %v", l.lab.Prog[research.Electricity], l.lab.Prog.Done(research.Traps))
	}
	if _, ok := l.bldgs.Get(keep); !ok || l.grid.Walkable(6, 5) {
		t.Fatal("the simple bench fits: it stays and blocks its new cells")
	}
	if _, ok := l.bldgs.Get(lost); ok {
		t.Fatal("the hi-tech bench no longer fits: it is taken down")
	}
	steel, comps := 0, 0
	for _, id := range l.itemIDs {
		st, _ := l.stacks.Get(id)
		switch st.Kind {
		case item.Steel:
			steel += st.Qty
		case item.Components:
			comps += st.Qty
		}
	}
	found := false
	for _, m := range l.World().Messages {
		found = found || strings.Contains(m.Text, "no longer fits")
	}
	if steel != 65 || comps != 3 || !found {
		t.Fatalf("refund %d steel, %d components, message %v", steel, comps, found)
	}
	l.Step(300) // and it keeps running
	if again := loadAsVersion(t, l, SaveVersion); again.lab.Prog != l.lab.Prog {
		t.Fatal("a v9 save is not migrated again")
	}
}
