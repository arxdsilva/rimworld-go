package ebiten

// journey_rsch_test.go: the RSCH-1/2 research journeys: picking and
// starting a project, bench tiers, building the hi-tech bench and the
// multi-analyzer, completion and the alert, the tree's search and
// prerequisite links, and the tribal cost multiplier.

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	rlab "rimworld-go/internal/application/game/research"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/randx"
)

// rsClean is a new colony (it knows nothing) with a simple research bench
// at (12, 8); the journey is paused.
func rsClean(t *testing.T) *journey {
	t.Helper()
	g := game.NewColony(game.Config{Seed: 1, Width: 64, Height: 64})
	j := newJourney(t, jGame(g))
	return j
}

// rsProjects is the research list.
func rsProjects(j *journey) []game.ResearchView { return j.world().Research }

// rsByID is project id's view.
func rsByID(j *journey, id int) (game.ResearchView, bool) {
	for _, p := range rsProjects(j) {
		if int(p.ID) == id {
			return p, true
		}
	}
	return game.ResearchView{}, false
}

// rsFinish finishes project id (and everything it needs) with the dev
// hook, one project at a time.
func rsFinish(j *journey, id int) {
	j.t.Helper()
	v, ok := rsByID(j, id)
	if !ok || v.Done {
		return
	}
	for _, h := range v.Hidden {
		for _, p := range rsProjects(j) {
			if p.Label == h {
				rsFinish(j, int(p.ID))
			}
		}
	}
	for _, pre := range v.Prereqs {
		rsFinish(j, pre)
	}
	j.g.PickResearch(v.Name)
	j.g.Enqueue(command.Command{Kind: command.DevFinishResearch, Build: uint8(id)})
	j.g.ApplyPending()
	j.update(1)
}

// rsWith is the first unfinished project whose bench is bench ("" any)
// and for which pred holds.
func rsWith(j *journey, pred func(game.ResearchView) bool) (game.ResearchView, bool) {
	for _, p := range rsProjects(j) {
		if !p.Done && pred(p) {
			return p, true
		}
	}
	return game.ResearchView{}, false
}

// rsPickStart opens the tab, selects project v and presses Start.
func rsPickStart(j *journey, v game.ResearchView) {
	j.t.Helper()
	if j.a.uiModalTop() != "" || !j.a.win.Open || j.a.win.Mode != "research" {
		j.key("F6", 0)
	}
	id := "research.pick." + strconv.Itoa(int(v.ID))
	j.clickID(id)
	if sel, ok := j.a.win.Research().Selected(); !ok || sel != v.ID {
		j.t.Fatalf("clicking %s selected %d (%v)", id, sel, ok)
	}
	if !j.clickID("research.start") {
		j.t.Fatalf("Start research is disabled for %q", v.Label)
	}
	if c, ok := j.lastCmd(command.SetResearch); !ok || c.Build != v.ID {
		j.t.Fatalf("Start sent %+v (%v), want project %d", c, ok, v.ID)
	}
	j.update(2)
}

// rsProgress is project id's progress.
func rsProgress(j *journey, id uint8) int {
	for _, p := range rsProjects(j) {
		if p.ID == id {
			return p.Progress
		}
	}
	return -1
}

// rsBench builds the simple research bench and keeps the researcher
// working.
func rsBench(j *journey, x, y int) {
	j.t.Helper()
	wdBuild(j, "research bench", x, y)
}

// TestJourneyRschStartAndProgress (RSCH-1/2): pick a basic project, press
// Start research, and a researcher at a simple bench makes progress at
// roughly the wiki rate.
func TestJourneyRschStartAndProgress(t *testing.T) {
	t.Parallel()
	j := rsClean(t)
	rsBench(j, 12, 8)
	v, ok := rsWith(j, func(p game.ResearchView) bool { return p.Available && p.BenchMissing == "" })
	if !ok {
		t.Fatalf("no available project a simple bench can work: %+v", rsProjects(j))
	}
	rsPickStart(j, v)
	p0 := rsProgress(j, v.ID)
	t1Hours(j, 3, nil)
	got := rsProgress(j, v.ID) - p0
	// 20.625 points an hour at skill 8 on a hi-tech bench, x0.75 on a simple
	// one, across the colonists' skills (0..20) and the time spent walking
	if per := float64(got) / 3; per < 2 || per > 40 {
		t.Fatalf("progress rose %d points in 3 hours (%.1f an hour), want about 15 at skill 8 on a simple bench", got, per)
	}
}

// TestJourneyRschHiTechNeedsHiTechBench (RSCH-1/2): a hi-tech project shows
// the missing bench and a simple bench makes no progress on it.
func TestJourneyRschHiTechNeedsHiTechBench(t *testing.T) {
	t.Parallel()
	j := rsClean(t)
	rsBench(j, 12, 8)
	v, ok := rsWith(j, func(p game.ResearchView) bool { return strings.Contains(p.Bench, "hi-tech") })
	if !ok {
		t.Fatal("no project needs the hi-tech bench")
	}
	for _, pre := range v.Prereqs {
		rsFinish(j, pre)
	}
	for _, h := range v.Hidden {
		for _, p := range rsProjects(j) {
			if p.Label == h {
				rsFinish(j, int(p.ID))
			}
		}
	}
	v, _ = rsByID(j, int(v.ID))
	if v.Done {
		t.Skip("finishing the prerequisites finished the project too")
	}
	if !v.Available {
		t.Fatalf("%q is still unavailable after its prerequisites: %+v", v.Label, v)
	}
	if v.BenchMissing != rlab.MissingHiTech && v.BenchMissing != rlab.MissingAnalyzer {
		t.Fatalf("BenchMissing is %q, want a hi-tech bench line", v.BenchMissing)
	}
	j.g.PickResearch(v.Name) // its box can lie outside the tree's view: start it with the dev hook
	j.update(2)
	p0 := rsProgress(j, v.ID)
	t1Hours(j, 3, nil)
	if d := rsProgress(j, v.ID) - p0; d != 0 {
		t.Fatalf("a simple bench made %d points on a hi-tech project", d)
	}
}

// TestJourneyRschBuildHiTechBenchAndAnalyzer (RSCH-1/2): the hi-tech bench
// and the multi-analyzer built through the Architect link, and the
// bench's pane shows it and a higher speed.
func TestJourneyRschBuildHiTechBenchAndAnalyzer(t *testing.T) {
	t.Parallel()
	j := rsClean(t)
	// open the unlocks: finish the projects that name the two buildings
	for _, want := range []string{"hi-tech research bench", "multi-analyzer"} {
		done := false
		for _, p := range rsProjects(j) {
			for _, u := range p.Unlocks {
				if !done && strings.Contains(strings.ToLower(u), want) {
					rsFinish(j, int(p.ID))
					done = true
				}
			}
		}
		if !done {
			t.Fatalf("no project unlocks the %s: %+v", want, rsProjects(j))
		}
	}
	pawn := j.g.Snapshot()[0]
	ox, oy := pawn.X-2, pawn.Y-4
	j.g.SetGround(ox-1, oy-1, ox+14, oy+7, "soil", "", "")
	j.g.ClearPlants(ox-1, oy-1, ox+14, oy+7)
	for _, gx := range []int{ox + 3, ox + 5} {
		wdBuild(j, "wood-fired generator", gx, oy)
		j.g.FuelCampfire(gx, oy)
	}
	for x := ox; x <= ox+12; x++ {
		j.g.BuildNow("power conduit", "steel", "south", x, oy+2)
	}
	pos := entity.EntityID(0)
	_ = pos
	for i, k := range []item.Kind{item.Steel, item.Components} {
		if _, err := j.g.SpawnStack(k, 80, ox+i, oy+5); err != nil {
			t.Fatal(err)
		}
	}
	j.tick(240)
	place := func(cat, tool string, x, y int) {
		j.openArch(cat, tool)
		j.lookAt(x, y)
		j.update(1)
		j.settle()
		j.clickTile(x, y, 0, 0)
		if c, ok := j.lastCmd(command.DesignateBuild); !ok || c.X != x || c.Y != y {
			t.Fatalf("%s at (%d, %d) sent %+v (%v)", tool, x, y, c, ok)
		}
		j.leaveArch()
	}
	run := func(done func() bool, what string) {
		careFeed(j)
		j.ff(72000, ffStride, func() bool { careFeed(j); return done() }) // 12000 Updates at Very fast
		if !done() {
			t.Fatalf("the %s was not built: %+v", what, j.world().Frames)
		}
		j.tick(300)
	}
	find := func(sub string) (game.BuildingView, bool) {
		for _, v := range j.world().Buildings {
			if strings.Contains(v.Kind, sub) {
				return v, true
			}
		}
		return game.BuildingView{}, false
	}
	place("production", "electric_research", ox+1, oy+3)
	run(func() bool { _, ok := find("research"); return ok }, "hi-tech bench")
	alone, _ := find("research")
	place("misc", "multi_analyzer", ox+6, oy+3) // STK-AG: vanilla Misc
	run(func() bool { _, ok := find("analyzer"); return ok }, "multi-analyzer")
	bench, _ := find("research")
	lines := inspector.ResearchLines(bench, j.world())
	joined := strings.Join(lines, "|")
	if !strings.Contains(joined, "Linked: multi-analyzer") || bench.ResearchSpeed <= alone.ResearchSpeed {
		var an game.BuildingView
		for _, v := range j.world().Buildings {
			if strings.Contains(v.Kind, "analyzer") {
				an = v
			}
		}
		t.Fatalf("hi-tech bench pane %q, speed %d permille; want it linked and above the %d permille it had alone; bench (%d,%d) powered %v net %d; analyzer (%d,%d) powered %v net %d reason %q", joined, bench.ResearchSpeed, alone.ResearchSpeed, bench.X, bench.Y, bench.Powered, bench.PowerNet, an.X, an.Y, an.Powered, an.PowerNet, an.NoPowerReason)
	}
}

// TestJourneyRschCompletionLetterAndAlert (RSCH-1/2): finishing a project
// posts its letter; with none selected the alert shows and opens the tab.
func TestJourneyRschCompletionLetterAndAlert(t *testing.T) {
	t.Parallel()
	j := rsClean(t)
	rsBench(j, 12, 8)
	v, ok := rsWith(j, func(p game.ResearchView) bool { return p.Available && p.BenchMissing == "" })
	if !ok {
		t.Fatal("no available project")
	}
	rsPickStart(j, v)
	j.g.AddResearch(v.Name, v.Cost-2) // two points short: the researcher finishes it
	j.update(1)
	have := func() bool {
		for _, l := range j.world().Letters {
			if l.Title == "Research complete: "+v.Label {
				return true
			}
		}
		return false
	}
	for i := 0; i < 6 && !have(); i++ {
		t1Hours(j, 1, nil)
	}
	want := "Research complete: " + v.Label
	if !have() {
		t.Fatalf("no letter titled %q; letters %+v", want, j.world().Letters)
	}
	j.key("F6", 0) // close the tab
	j.update(5)
	if !j.world().ResearchAlert {
		t.Fatalf("WorldView.ResearchAlert is false after the project finished (benches %v)", len(j.world().Buildings))
	}
	if !j.clickID("alert.research") {
		t.Fatal("the research alert is disabled")
	}
	if !j.a.win.Open || j.a.win.Mode != "research" {
		t.Fatalf("clicking the alert left mode %q (open %v)", j.a.win.Mode, j.a.win.Open)
	}
}

// TestJourneyRschSearchAndPrereqLink (RSCH-2): the search box finds a
// project, and a prerequisite link in the info panel selects it.
func TestJourneyRschSearchAndPrereqLink(t *testing.T) {
	t.Parallel()
	j := rsClean(t)
	j.key("F6", 0)
	// a project with a drawn prerequisite whose box is in view
	shown := map[string]bool{}
	for _, b := range j.a.win.Research().Buttons() {
		shown[b.ID] = true
	}
	var child game.ResearchView
	for _, p := range rsProjects(j) {
		if len(p.Prereqs) > 0 && shown["research.pick."+strconv.Itoa(int(p.ID))] {
			child = p
			break
		}
	}
	if child.Label == "" {
		t.Fatal("no project with a drawn prerequisite is in view")
	}
	// the search jumps to the first match on Enter
	far, _ := rsWith(j, func(p game.ResearchView) bool { return !shown["research.pick."+strconv.Itoa(int(p.ID))] })
	j.clickID("research.search")
	j.typeText(strings.ToLower(far.Label))
	j.key("Enter", 0)
	j.update(2)
	if sel, ok := j.a.win.Research().Selected(); !ok || sel != far.ID {
		t.Errorf("searching %q and pressing Enter selected %d (%v), want the first match %d", far.Label, sel, ok, far.ID)
	}
	for range 40 {
		j.key("Backspace", 0)
	}
	j.key("Enter", 0)
	j.update(2)
	// the jump scrolled the tree: pick a child drawn in the view it left
	shown = map[string]bool{}
	for _, b := range j.a.win.Research().Buttons() {
		shown[b.ID] = true
	}
	child = game.ResearchView{}
	for _, p := range rsProjects(j) {
		if len(p.Prereqs) > 0 && shown["research.pick."+strconv.Itoa(int(p.ID))] {
			child = p
			break
		}
	}
	if child.Label == "" {
		t.Fatal("no project with a drawn prerequisite is in view after the search")
	}
	if !j.clickID("research.pick." + strconv.Itoa(int(child.ID))) {
		t.Fatalf("project %d is not clickable", child.ID)
	}
	pre := child.Prereqs[0]
	if !j.clickID("research.prereq." + strconv.Itoa(pre)) {
		t.Fatalf("the prerequisite link %d is disabled", pre)
	}
	if sel, ok := j.a.win.Research().Selected(); !ok || int(sel) != pre {
		t.Fatalf("the link selected %d (%v), want %d", sel, ok, pre)
	}
}

// TestJourneyRschTribalCostMultiplier (RSCH-1/2): a tribal colony pays
// double for Industrial projects.
func TestJourneyRschTribalCostMultiplier(t *testing.T) {
	t.Parallel()
	d, ok := scenario.Builtin("ember_clan")
	if !ok {
		t.Fatal("no ember_clan scenario")
	}
	s := &game.Setup{
		Scenario: d, Storyteller: "steady", Preset: difficulty.Seasoned, Difficulty: difficulty.Of(difficulty.Seasoned),
		Site: site.Site{Biome: biome.TemperateForest, Climate: biome.DefOf(biome.TemperateForest).Climate,
			Stones: []geology.Stone{geology.Granite, geology.Marble}},
		Factions: faction.Generate(faction.DefaultSlots(), randx.New(7)),
		MapSize:  64, Season: game.SeasonSpring, Seed: 4242,
	}
	s.Colonists = game.NewCandidatePool(s.Seed, s.Scenario.Apply()).Selected()
	b := game.NewColonyBuilder(game.Config{Setup: s})
	var g *game.Game
	for i := 0; i < 200 && g == nil; i++ {
		if _, _, done := b.Step(1); done {
			g = b.Game()
		}
	}
	if g == nil {
		t.Fatal("the colony builder never finished")
	}
	j := newJourney(t, jGame(g))
	j.key("F6", 0)
	var ind game.ResearchView
	for _, p := range rsProjects(j) {
		if p.TechLevel == "Industrial" && !p.Done {
			ind = p
			break
		}
	}
	if ind.Label == "" {
		t.Fatal("no unfinished Industrial project")
	}
	if ind.CostNow != 2*ind.Cost {
		t.Fatalf("%q costs %d now against a base %d, want double for a tribe", ind.Label, ind.CostNow, ind.Cost)
	}
	var _ entity.EntityID
}
