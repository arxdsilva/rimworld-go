package game

import (
	"slices"
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

// TestRschContractInert (RSCH-0): DevFinishResearch is ignored, so a
// colony that gets it hashes as before and keeps in step with a twin; the
// new view fields read zero.
func TestRschContractInert(t *testing.T) {
	t.Parallel()
	cfg := Config{Seed: 11, Width: 40, Height: 30}
	g, twin := NewColony(cfg), NewColony(cfg)
	for _, c := range []*Game{g, twin} {
		c.PickResearch("traps")
		c.Step(200)
	}
	before := g.StateHash()
	g.Enqueue(command.Command{Kind: command.DevFinishResearch})
	g.ApplyPending()
	if g.StateHash() != before {
		t.Fatal("DevFinishResearch changed the state")
	}
	g.Step(300)
	twin.Step(300)
	if g.StateHash() != twin.StateHash() {
		t.Fatal("the commanded colony drifted from its twin")
	}
	w := g.World()
	if w.ResearchAlert || w.TechLevel != "" {
		t.Errorf("world: alert %v, tech level %q", w.ResearchAlert, w.TechLevel)
	}
	for _, r := range w.Research {
		if r.TechLevel != "" || r.Bench != "" || r.BenchMissing != "" || r.CostNow != 0 || r.ETAHours != 0 || r.Hidden != nil {
			t.Errorf("%s: %+v", r.Name, r)
		}
	}
	for _, b := range w.Buildings {
		if b.ResearchSpeed != 0 || b.Analyzer != 0 {
			t.Errorf("%s: research speed %d, analyzer %d", b.Kind, b.ResearchSpeed, b.Analyzer)
		}
	}
}
