package game

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/storyteller"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/pipeline"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/weather"
	"rimworld-go/pkg/randx"
)

// setupFor is a Setup of built-in scenario key on a 100x100 temperate
// site (granite and marble), Seasoned, storyteller steady, its scenario's
// first People candidates; edit adjusts it.
func setupFor(t testing.TB, key string, edit func(*Setup)) *Setup {
	t.Helper()
	d, ok := scenario.Builtin(key)
	if !ok {
		t.Fatalf("no built-in scenario %q", key)
	}
	s := &Setup{
		Scenario: d, Storyteller: "steady", Preset: difficulty.Seasoned, Difficulty: difficulty.Of(difficulty.Seasoned),
		Site: site.Site{Biome: biome.TemperateForest, Climate: biome.DefOf(biome.TemperateForest).Climate,
			Stones: []geology.Stone{geology.Granite, geology.Marble}},
		Factions: faction.Generate(faction.DefaultSlots(), randx.New(7)),
		MapSize:  100, Season: SeasonSpring, Seed: 4242,
	}
	if edit != nil {
		edit(s)
	}
	s.Colonists = NewCandidatePool(s.Seed, s.Scenario.Apply()).Selected()
	return s
}

// buildSetup runs a ColonyBuilder for s to the end.
func buildSetup(t testing.TB, s *Setup) *Game {
	t.Helper()
	b := NewColonyBuilder(Config{Setup: s})
	for i := 0; i <= buildSteps(); i++ {
		if _, _, done := b.Step(1); done {
			return b.Game()
		}
	}
	t.Fatal("the builder never finished")
	return nil
}

// stepUntilLanded steps until every pod has landed.
func stepUntilLanded(t testing.TB, g *Game) {
	t.Helper()
	for i := 0; i < 20 && !allLanded(g); i++ {
		g.Step(60)
	}
	if !allLanded(g) {
		t.Fatal("pods never landed")
	}
}

func allLanded(g *Game) bool {
	for _, p := range g.ng.pods {
		if !p.Landed {
			return false
		}
	}
	return true
}

// bareSetupGame is a small sandbox map with a setup of difficulty preset p
// (hooks, not the builder).
func bareSetupGame(p difficulty.Preset) *Game {
	g := New(Config{Seed: 3, Width: 40, Height: 40})
	g.ng.setup = &Setup{Preset: p, Difficulty: difficulty.Of(p), Storyteller: "steady"}
	return g
}

func itemTotals(g *Game) map[item.Kind]int {
	out := map[item.Kind]int{}
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok {
			out[st.Kind] += st.Qty
		}
	}
	return out
}

// kitTotals is itemTotals less what map generation left lying (ruin loot
// and stone chunks): what the scenario brought.
func kitTotals(t testing.TB, g *Game, s *Setup) map[item.Kind]int {
	t.Helper()
	res, err := pipeline.Generate(mapgen.Config{Seed: s.Seed, W: g.Width(), H: g.Height(), Site: s.Site, Biome: biome.DefOf(s.Site.Biome)})
	if err != nil {
		t.Fatal(err)
	}
	out := itemTotals(g)
	for _, r := range res.Ruins {
		for _, l := range r.Loot {
			out[l.Kind] -= l.Qty
		}
	}
	for _, v := range res.Vaults { // MAPGEN-3: a sealed vault's loot is no kit either
		for _, l := range v.Loot {
			out[l.Kind] -= l.Qty
		}
	}
	for k, n := range out {
		if item.IsChunk(k) || n == 0 {
			delete(out, k)
		}
	}
	return out
}

func countBuildings(g *Game, k construction.Kind) int {
	n := 0
	for _, id := range g.bldgIDs {
		if b, ok := g.bldgs.Get(id); ok && b.Kind == k {
			n++
		}
	}
	return n
}

func TestSetupNilIsLegacy(t *testing.T) {
	t.Parallel()
	cfg := Config{Seed: 11, Width: 60, Height: 60}
	legacy := NewColony(cfg)
	b := NewColonyBuilder(cfg)
	if phase, frac, done := b.Step(1); !done || frac != 1 || phase != "" {
		t.Fatalf("a legacy build is one step: %q %v %v", phase, frac, done)
	}
	g := b.Game()
	if g.hasSetup() {
		t.Fatal("legacy colony has a setup")
	}
	if a, b := legacy.StateHash(), g.StateHash(); a != b {
		t.Fatalf("start hash %x != legacy %x", b, a)
	}
	legacy.Step(4000)
	g.Step(4000)
	if a, b := legacy.StateHash(), g.StateHash(); a != b {
		t.Fatalf("hash after 4000 ticks %x != legacy %x", b, a)
	}
	if g.Factions() == nil && len(g.Factions()) != 0 || len(g.Factions()) != 0 {
		t.Fatal("a legacy colony has no factions")
	}
	if g.diffScale(DiffCrop, 7) != 7 || g.moodOffset() != 0 || g.researchCostPct() != 100 || g.growthPct() != 100 {
		t.Fatal("legacy hooks must be neutral")
	}
}

func TestColonyBuilderStepsToDone(t *testing.T) {
	t.Parallel()
	s := setupFor(t, "broken_hull", nil)
	b := NewColonyBuilder(Config{Setup: s})
	var phases []string
	last := float32(-1)
	for i := 0; ; i++ {
		if i > buildSteps() {
			t.Fatal("never done")
		}
		if b.Game() != nil {
			t.Fatal("Game before done")
		}
		phase, frac, done := b.Step(1)
		if done {
			if frac != 1 || phase != "" {
				t.Fatalf("done: %q %v", phase, frac)
			}
			break
		}
		if frac <= last {
			t.Fatalf("progress went back: %v after %v", frac, last)
		}
		last = frac
		phases = append(phases, phase)
	}
	// one phase per map-generator step, by its label, then the colony
	var want []string
	for _, n := range pipeline.StepNames() {
		want = append(want, pipeline.Label(n))
	}
	want = append(want, "Placing the colony", "Arrival")
	if !slices.Equal(phases, want) {
		t.Fatalf("phases %q, want %q", phases, want)
	}
	if b.Game() == nil {
		t.Fatal("no game when done")
	}
	if _, _, done := b.Step(1); !done {
		t.Fatal("a finished builder stays done")
	}
	// one big step builds the same colony
	b2 := NewColonyBuilder(Config{Setup: setupFor(t, "broken_hull", nil)})
	if _, _, done := b2.Step(100); !done {
		t.Fatal("Step(100) should finish")
	}
	if b.Game().StateHash() != b2.Game().StateHash() {
		t.Fatal("same setup, different colony")
	}
}

func TestSetupBuildsChosenMap(t *testing.T) {
	t.Parallel()
	s := setupFor(t, "broken_hull", func(s *Setup) {
		s.MapSize = 90
		s.Site.Biome = biome.BorealForest
		s.Site.Climate = biome.DefOf(biome.BorealForest).Climate
		s.Site.Stones = []geology.Stone{geology.Slate, geology.Marble}
		s.Season = SeasonWinter
	})
	g := buildSetup(t, s)
	if g.Width() != 90 || g.Height() != 90 {
		t.Fatalf("size %dx%d", g.Width(), g.Height())
	}
	if g.biome != biome.BorealForest || g.World().BiomeKey != biome.DefOf(biome.BorealForest).Key {
		t.Fatalf("biome %v", g.biome)
	}
	if g.climate != s.Site.Climate {
		t.Fatalf("climate %+v, want %+v", g.climate, s.Site.Climate)
	}
	now := uint64(g.clock.Now())
	if g.season(now) != "Winter" || g.Date().Hour != clock.StartHour || g.World().Season != "Winter" {
		t.Fatalf("start: %s at %d:00", g.season(now), g.Date().Hour)
	}
	if g.season(now-clock.TicksPerDay) == "Winter" {
		t.Fatal("the start is not the season's first day")
	}
	if g.relDay(now) != 0 {
		t.Fatalf("colony day %d at the start", g.relDay(now))
	}
	stones := map[geology.Stone]bool{}
	for y := 0; y < g.Height(); y++ {
		for x := 0; x < g.Width(); x++ {
			if st := g.geo.At(x, y); st != geology.None {
				stones[st] = true
			}
		}
	}
	for st := range stones {
		if st != geology.Slate && st != geology.Marble {
			t.Fatalf("stone %v is not the site's", st)
		}
	}
	if len(stones) == 0 {
		t.Fatal("no stone at all")
	}
	if !g.research.Done(research.FoodPreservation) || g.research.Done(research.Herbalism) {
		t.Fatal("research: grants only")
	}
	// southern hemisphere: the season still starts as chosen
	south := setupFor(t, "ember_clan", func(s *Setup) {
		s.Site.Climate.PeakDay = 45
		s.Season = SeasonSummer
	})
	gs := buildSetup(t, south)
	if got := gs.season(uint64(gs.clock.Now())); got != "Summer" {
		t.Fatalf("southern summer start: %s", got)
	}
	if temperature.Season(gs.clock.Now()) == "Summer" {
		t.Fatal("the southern summer should be the north's winter")
	}
}

func TestSeasonStartDay(t *testing.T) {
	t.Parallel()
	for _, peak := range []int{15, 45, 0, 59} {
		c := temperature.Climate{PeakDay: peak}
		for s, name := range map[Season]string{SeasonSpring: "Spring", SeasonSummer: "Summer", SeasonFall: "Fall", SeasonWinter: "Winter"} {
			d := seasonStartDay(peak, s)
			at := func(day int) string {
				return c.SeasonAt(clock.Tick(((day+clock.DaysPerYear)%clock.DaysPerYear)*clock.TicksPerDay + 1))
			}
			if at(d) != name || at(d-1) == name {
				t.Fatalf("peak %d %s: day %d is %s, day before %s", peak, name, d, at(d), at(d-1))
			}
		}
	}
}

func TestPodArrivalPlacesEveryoneOnLand(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "broken_hull", nil))
	if len(g.pawnIDs) != 0 {
		t.Fatal("colonists are in the pods until they land")
	}
	pods := len(g.ng.pods)
	if pods < 4 {
		t.Fatalf("%d pods: one per colonist plus kit pods", pods)
	}
	start := uint64(g.clock.Now())
	cx, cy := g.landingSpot() // the generator's landing spot
	for _, p := range g.ng.pods {
		if p.Tick < start || p.Tick > start+podFall+podStagger {
			t.Fatalf("pod lands at %d, outside the stagger", p.Tick-start)
		}
		if chebyshev(Position{p.X, p.Y}, Position{cx, cy}) > podRadius || !g.dryWalkable(p.X, p.Y) {
			t.Fatalf("pod at (%d,%d) is not on land near the centre", p.X, p.Y)
		}
	}
	if len(g.World().Arrivals) != pods {
		t.Fatal("arrivals are in the view while falling")
	}
	stepUntilLanded(t, g)
	if len(g.pawnIDs) != 3 {
		t.Fatalf("%d colonists landed, want 3", len(g.pawnIDs))
	}
	seen := map[Position]bool{}
	for _, id := range g.pawnIDs {
		p, _ := g.positions.Get(id)
		if !g.dryWalkable(p.X, p.Y) || seen[p] {
			t.Fatalf("colonist at %v: not on free land", p)
		}
		seen[p] = true
		if chebyshev(p, Position{cx, cy}) > podRadius+3+8 { // + a few steps since landing
			t.Fatalf("colonist %v far from the landing spot", p)
		}
		if !g.hasMemory(id, mood.Landed) {
			t.Fatal("no rough-landing thought")
		}
	}
	if n := countBuildings(g, construction.PodShell); n != pods {
		t.Fatalf("%d pod shells for %d pods", n, pods)
	}
	for _, a := range g.World().Arrivals {
		if !a.Landed {
			t.Fatal("a landed pod still falls")
		}
	}
	g.Step(arrivalKeep + 10)
	if len(g.World().Arrivals) != 0 || len(g.ng.pods) != 0 {
		t.Fatal("arrival stamps kept beyond an hour")
	}
}

func TestStandingArrivalRing(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "ember_clan", nil))
	if len(g.pawnIDs) != 5 {
		t.Fatalf("%d clanfolk standing, want 5", len(g.pawnIDs))
	}
	if len(g.ng.pods) != 0 {
		t.Fatal("the standing arrival has no pods")
	}
	lx, ly := g.landingSpot()
	c := Position{lx, ly}
	seen := map[Position]bool{}
	for _, id := range g.pawnIDs {
		p, _ := g.positions.Get(id)
		if d := chebyshev(p, c); d < 2 || d > 6 || seen[p] || !g.dryWalkable(p.X, p.Y) {
			t.Fatalf("clansperson at %v (distance %d) is not in the ring", p, d)
		}
		seen[p] = true
	}
	tot := itemTotals(g)
	if tot[item.Meat] != 40 || tot[item.Spear] != 3 {
		t.Fatalf("kit: meat %d spears %d", tot[item.Meat], tot[item.Spear])
	}
	for _, id := range g.itemIDs {
		st, _ := g.stacks.Get(id)
		if st.Kind == item.Meat && chebyshev(Position{st.X, st.Y}, c) > 4 {
			t.Fatalf("kit pile at (%d,%d) is not in the middle", st.X, st.Y)
		}
	}
}

func TestWalkInFromEdge(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) { s.Scenario.Arrival = scenario.WalkIn }))
	if len(g.pawnIDs) != 3 {
		t.Fatalf("%d walked in", len(g.pawnIDs))
	}
	w, h := g.Width(), g.Height()
	lx, ly := g.landingSpot()
	c := Position{lx, ly}
	start := map[entity.EntityID]int{}
	for _, id := range g.pawnIDs {
		p, _ := g.positions.Get(id)
		if edge := min(p.X, p.Y, w-1-p.X, h-1-p.Y); edge > 4 {
			t.Fatalf("colonist at %v is not at an edge", p)
		}
		start[id] = chebyshev(p, c)
	}
	g.Step(3000)
	for _, id := range g.pawnIDs {
		p, _ := g.positions.Get(id)
		if chebyshev(p, c) >= start[id] {
			t.Fatalf("colonist did not walk in: %v (was %d away)", p, start[id])
		}
	}
}

func TestIntroLetterOnce(t *testing.T) {
	t.Parallel()
	s := setupFor(t, "broken_hull", nil)
	g := buildSetup(t, s)
	letters := func() int {
		n := 0
		for _, m := range g.World().Messages {
			if m.Letter && m.Title == s.Scenario.Name {
				n++
				if !strings.HasPrefix(m.Text, "The alarms") {
					t.Fatalf("intro text %q", m.Text)
				}
			}
		}
		return n
	}
	if letters() != 0 {
		t.Fatal("letter before the landing")
	}
	stepUntilLanded(t, g)
	g.Step(10)
	if letters() != 1 {
		t.Fatalf("%d intro letters after landing", letters())
	}
	g.Step(500)
	if letters() != 1 || !g.ng.introDone {
		t.Fatal("the intro letter comes once")
	}
	// a saved and loaded colony does not post it again
	st := g.exportSetupState()
	g2 := New(Config{Width: g.Width(), Height: g.Height()})
	g2.importSetupState(st)
	g2.Step(100)
	for _, m := range g2.World().Messages {
		if m.Letter {
			t.Fatal("letter posted again after a load")
		}
	}
	// standing arrivals get theirs on the first tick
	e := buildSetup(t, setupFor(t, "ember_clan", nil))
	e.Step(1)
	n := 0
	for _, m := range e.World().Messages {
		if m.Letter {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("ember clan intro letters: %d", n)
	}
}

func TestScenarioKitSpawned(t *testing.T) {
	t.Parallel()
	s := setupFor(t, "broken_hull", nil)
	g := buildSetup(t, s)
	stepUntilLanded(t, g)
	tot := kitTotals(t, g, s) // without the ruins' and vaults' loot
	want := map[item.Kind]int{
		item.Steel: 300 + 400, item.Silver: 600, item.Components: 20 + 6, item.Rifle: 2, item.Knife: 1,
		item.IndustrialMedicine: 12, item.Wood: 200, item.Meal: 10, item.Food: 40 + 8,
	}
	for k, n := range want {
		if tot[k] != n {
			t.Errorf("%s: %d on the map, want %d", k, tot[k], n)
		}
	}
	pets := 0
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && a.Tame {
			pets++
			if !slices.Contains(scenario.PetSet(), a.Species) || a.Name == "" {
				t.Fatalf("pet %v %q", a.Species, a.Name)
			}
		}
	}
	if pets != 1 {
		t.Fatalf("%d pets, want 1", pets)
	}
	gs := setupFor(t, "gilded_expedition", nil)
	ge := buildSetup(t, gs)
	stepUntilLanded(t, ge)
	if tot := kitTotals(t, ge, gs); tot[item.Silver] != 3000 || tot[item.Gold] != 30 {
		t.Fatalf("gilded: silver %d gold %d", tot[item.Silver], tot[item.Gold])
	}
	if len(ge.ng.pods) > 1+maxKitPods {
		t.Fatalf("%d pods", len(ge.ng.pods))
	}
	bs := setupFor(t, "bare_hands", nil)
	bh := buildSetup(t, bs)
	stepUntilLanded(t, bh)
	if len(kitTotals(t, bh, bs)) != 0 || len(bh.ng.pods) != 1 {
		t.Fatalf("bare hands: items %v pods %d", itemTotals(bh), len(bh.ng.pods))
	}
}

func TestResearchGrantsAndCostPct(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "gilded_expedition", nil))
	for _, p := range []research.Project{research.FoodPreservation, research.Herbalism, research.Tailoring, research.Traps, research.Gunsmithing} {
		if !g.research.Done(p) {
			t.Fatalf("%s not granted", research.DefOf(p).Label)
		}
		for _, r := range research.DefOf(p).Requires {
			if !g.research.Done(r) {
				t.Fatalf("%s granted without its prerequisite", research.DefOf(p).Label)
			}
		}
	}
	if g.researchCostPct() != 100 {
		t.Fatalf("industrial cost %d%%", g.researchCostPct())
	}
	e := buildSetup(t, setupFor(t, "ember_clan", nil))
	if e.researchCostPct() != 150 {
		t.Fatalf("neolithic cost %d%%", e.researchCostPct())
	}
	for p := research.Project(0); p < research.NumProjects; p++ {
		if e.research.Done(p) {
			t.Fatalf("ember clan knows %s", research.DefOf(p).Label)
		}
	}
	c := buildSetup(t, setupFor(t, "ember_clan", func(s *Setup) {
		s.Scenario.Parts = append(s.Scenario.Parts, scenario.Part{Kind: scenario.PartResearchCost, Percent: 200})
	}))
	if c.researchCostPct() != 300 {
		t.Fatalf("neolithic x200%%: %d%%", c.researchCostPct())
	}
}

func TestDisallowedBuildingRefused(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) {
		s.Scenario.Parts = append(s.Scenario.Parts, scenario.Part{Kind: scenario.PartDisallowBuilding, Building: construction.Bed})
	}))
	x, y, ok := g.walkableNear(g.Width()/2+8, g.Height()/2+8, 6)
	if !ok {
		t.Fatal("no free tile")
	}
	if ok, why := g.CanPlaceAt("bed", "wood", "south", x, y); ok || why != ReasonScenario {
		t.Fatalf("bed: %v %q", ok, why)
	}
	if ok, why := g.CanPlaceAt("wall", "wood", "south", x, y); !ok {
		t.Fatalf("wall refused: %q", why)
	}
	for _, b := range g.Buildables() {
		if (b.Kind == "bed") != (b.Locked == ReasonScenario) {
			t.Fatalf("%s locked %q", b.Kind, b.Locked)
		}
	}
	legacy := NewColony(Config{Seed: 2, Width: 40, Height: 40})
	for _, b := range legacy.Buildables() {
		if b.Locked != "" {
			t.Fatal("legacy colony locks nothing by scenario")
		}
	}
}

func TestScheduledAndDisabledIncidents(t *testing.T) {
	t.Parallel()
	s := setupFor(t, "broken_hull", func(s *Setup) {
		s.Scenario.Parts = append(s.Scenario.Parts,
			scenario.Part{Kind: scenario.PartScheduledIncident, Incident: "farm_animals", FirstDay: 2, EveryDays: 3},
			scenario.Part{Kind: scenario.PartDisabledIncident, Incident: "herd_crossing"},
			scenario.Part{Kind: scenario.PartDisabledIncident, Incident: "manhunter_pack"})
	})
	g := buildSetup(t, s)
	// the scheduled days: 2, 5, 8
	fired := []int{}
	off := g.dayOff()
	for day := 0; day <= 9; day++ {
		before := len(g.animalIDs)
		at := off + uint64(day)*clock.TicksPerDay + scheduledHour*clock.TicksPerHour
		g.clock = clock.NewAt(clock.Tick(at))
		g.scheduledIncidents(at)
		tame := 0
		for _, id := range g.animalIDs[before:] {
			if a, _ := g.animals.Get(id); a.Tame {
				tame++
			}
		}
		if tame > 0 {
			fired = append(fired, day)
		}
	}
	if !slices.Equal(fired, []int{2, 5, 8}) {
		t.Fatalf("farm animals came on days %v, want [2 5 8]", fired)
	}
	// a disabled nature event is never rolled
	for i := 0; i < 60; i++ {
		g.incidents = incident.State{}
		now := off + 40*clock.TicksPerDay + uint64(i)*clock.TicksPerHour
		g.clock = clock.NewAt(clock.Tick(now))
		g.rollNature(now)
		if _, ok := g.incidents.Last[incident.HerdCrossing]; ok {
			t.Fatal("a disabled herd crossing happened")
		}
	}
	// and a disabled threat is never picked
	for roll := 0; roll < threatRolls; roll++ {
		for _, winter := range []bool{false, true} {
			if k, ok := g.allowedThreat(roll, 30, winter); ok && k == threatManhunters {
				t.Fatal("disabled manhunter pack picked")
			}
		}
	}
}

func TestPermanentConditions(t *testing.T) {
	t.Parallel()
	base := biome.DefOf(biome.TemperateForest).Climate
	with := func(c scenario.Condition) *Game {
		return buildSetup(t, setupFor(t, "ember_clan", func(s *Setup) {
			s.MapSize = 60
			s.Scenario.Parts = append(s.Scenario.Parts, scenario.Part{Kind: scenario.PartCondition, Cond: c})
		}))
	}
	if g := with(scenario.EndlessChill); g.climate.YearMean != base.YearMean-80 || g.climate.Amplitude != base.Amplitude {
		t.Fatalf("endless chill: %+v", g.climate)
	}
	if g := with(scenario.LongSummer); g.climate.YearMean != base.YearMean+60 || g.climate.Amplitude != base.Amplitude*7/10 {
		t.Fatalf("long summer: %+v", g.climate)
	}
	ash := with(scenario.AshfallSky)
	if ash.growthPct() != 85 || !slices.Contains(ash.World().Conditions, "ashfall_sky") {
		t.Fatalf("ashfall: growth %d, view %v", ash.growthPct(), ash.World().Conditions)
	}
	still := with(scenario.StillAir)
	for b := uint64(0); b < 400; b++ {
		now := b * weatherBlockTicks
		still.clock = clock.NewAt(clock.Tick(now))
		still.phaseWeather()
		if k := still.weather.Target; k != weather.Clear && k != weather.Cloudy {
			t.Fatalf("still air: %s on block %d", k, b)
		}
	}
	plain := with(scenario.NoCondition)
	rained := false
	for b := uint64(0); b < 400 && !rained; b++ {
		now := b * weatherBlockTicks
		plain.clock = clock.NewAt(clock.Tick(now))
		plain.phaseWeather()
		rained = plain.weather.Target.Falling()
	}
	if !rained {
		t.Fatal("without still air it should rain or snow some time in 100 days")
	}
}

func TestStorytellerPacingInGame(t *testing.T) {
	t.Parallel()
	// the legacy storyteller with a setup makes the pre-NG-1 draws
	g := buildSetup(t, setupFor(t, "ember_clan", func(s *Setup) {
		s.Storyteller = "legacy"
		s.Difficulty.PeacefulDays = 0
	}))
	blob, _ := g.rng.MarshalBinary()
	ref := randx.New(0)
	_ = ref.UnmarshalBinary(blob)
	g.scheduleStory(uint64(g.clock.Now()))
	if want := g.dayOff() + uint64(firstRaidDay*clock.TicksPerDay+ref.IntN(clock.TicksPerDay)); g.nextThreat != want {
		t.Fatalf("legacy first threat %d, want %d", g.nextThreat, want)
	}
	// Orla's rhythm: threats only in the pressure phase
	steady := buildSetup(t, setupFor(t, "ember_clan", nil))
	p, _ := storyteller.ByKey("steady")
	prev := -1
	for i := 0; i < 8; i++ {
		at := steady.nextThreat
		if at < steady.dayOff() {
			t.Fatal("threat before the landing")
		}
		day := steady.relDay(at)
		if pressure, _ := storyteller.Phase(p, day); !pressure {
			t.Fatalf("threat on rest day %d", day)
		}
		if day <= prev {
			t.Fatalf("threat days not increasing: %d after %d", day, prev)
		}
		if i == 0 && day < p.FirstThreatDay {
			t.Fatalf("first threat on day %d", day)
		}
		prev = day
		steady.clock = clock.NewAt(clock.Tick(at))
		steady.fireThreat(at)
	}
	// Bram's calm gaps
	hearth := buildSetup(t, setupFor(t, "ember_clan", func(s *Setup) { s.Storyteller = "hearth" }))
	prev = -1
	for i := 0; i < 5; i++ {
		at := hearth.nextThreat
		day := hearth.relDay(at)
		if prev >= 0 && day-prev < 9 {
			t.Fatalf("hearth gap %d days", day-prev)
		}
		prev = day
		hearth.clock = clock.NewAt(clock.Tick(at))
		hearth.fireThreat(at)
	}
}

func TestPeacefulDaysRespected(t *testing.T) {
	t.Parallel()
	for _, preset := range []difficulty.Preset{difficulty.Gentle, difficulty.Settled, difficulty.Merciless} {
		g := buildSetup(t, setupFor(t, "ember_clan", func(s *Setup) {
			s.Storyteller = "gambler" // first threat day 4
			s.Preset, s.Difficulty = preset, difficulty.Of(preset)
		}))
		want := max(4, difficulty.Of(preset).PeacefulDays)
		if day := g.relDay(g.nextThreat); day < want {
			t.Fatalf("%s: first threat on day %d, want >= %d", preset, day, want)
		}
	}
}

// hostileOnly makes faction i the only hostile one.
func hostileOnly(fs []faction.Faction, i int) {
	for j := range fs {
		fs[j].Goodwill, fs[j].Relation = 0, faction.Neutral
		if j == i {
			fs[j].Goodwill, fs[j].Relation = -80, faction.Hostile
		}
	}
}

func TestRaidsAttributedToFaction(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "broken_hull", nil))
	stepUntilLanded(t, g)
	idx := slices.IndexFunc(g.ng.factions, func(f faction.Faction) bool { return f.Type == faction.Clans })
	hostileOnly(g.ng.factions, idx)
	f := &g.ng.factions[idx]
	before := f.Goodwill
	g.storyRaid(20, 400)
	grp := g.raidGroups[len(g.raidGroups)-1]
	if grp.Faction != idx+1 || grp.Size == 0 {
		t.Fatalf("raid group %+v, want faction %d", grp, idx+1)
	}
	msg := g.messages[len(g.messages)-1].Text
	if !strings.Contains(msg, "of "+f.Name) || !strings.Contains(msg, "coming from the") {
		t.Fatalf("message %q does not name %s", msg, f.Name)
	}
	for _, id := range g.raiderIDs {
		if g.equip[id] == item.Rifle {
			t.Fatal("a clan raider carries a rifle")
		}
		if g.name(id) == "" {
			t.Fatal("raider without a name")
		}
	}
	// the raid is beaten: the clan's goodwill pays for it, once
	for _, id := range slices.Clone(g.raiderIDs) {
		g.removeRaider(id)
	}
	g.checkRaidsOver(uint64(g.clock.Now()))
	g.checkRaidsOver(uint64(g.clock.Now()))
	if f.Goodwill != before+raidRepelled || len(f.Events) == 0 || !strings.Contains(f.Events[len(f.Events)-1], "raid repelled") {
		t.Fatalf("goodwill %d (was %d), events %v", f.Goodwill, before, f.Events)
	}
	// the view shows it
	var fv FactionView
	for _, v := range g.Factions() {
		if v.ID == f.ID {
			fv = v
		}
	}
	if fv.Relation != "hostile" || fv.Emblem != "ui_icon/faction_clans" || fv.Accent == "" || len(fv.Events) == 0 {
		t.Fatalf("faction view %+v", fv)
	}
}

func TestNoHostileFactionConvertsRaid(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "ember_clan", nil))
	hostileOnly(g.ng.factions, -1)
	for roll := 0; roll < threatRolls; roll++ {
		for _, winter := range []bool{false, true} {
			k, ok := g.allowedThreat(roll, 30, winter)
			if !ok || k == threatRaid {
				t.Fatalf("roll %d winter %v: %v %v", roll, winter, k, ok)
			}
		}
	}
	// Wanderer: no raids, hives or packs at all
	g.ng.setup.Difficulty = difficulty.Of(difficulty.Wanderer)
	hostileOnly(g.ng.factions, 0)
	for roll := 0; roll < threatRolls; roll++ {
		if k, ok := g.allowedThreat(roll, 30, true); ok && k != threatManhunters && k != threatMad {
			t.Fatalf("wanderer roll %d: %v", roll, k)
		}
	}
}

func TestThreatPointsUsesDifficulty(t *testing.T) {
	t.Parallel()
	pts := func(p difficulty.Preset) (*Game, int) {
		g := buildSetup(t, setupFor(t, "ember_clan", func(s *Setup) { s.Preset, s.Difficulty = p, difficulty.Of(p) }))
		return g, g.threatPoints()
	}
	g, seasoned := pts(difficulty.Seasoned)
	want := difficulty.ThreatPoints(difficulty.Of(difficulty.Seasoned), g.Wealth(), g.standingColonists(), 0, 0)
	if seasoned != want || g.raidPoints(3) != want {
		t.Fatalf("seasoned %d, want %d", seasoned, want)
	}
	_, merciless := pts(difficulty.Merciless)
	if merciless < seasoned*19/10 || merciless > seasoned*21/10 {
		t.Fatalf("merciless %d vs seasoned %d: want twice", merciless, seasoned)
	}
	// adaptation raises them
	g.ng.adapt.Value = 20
	if g.threatPoints() <= seasoned {
		t.Fatal("adaptation does not raise threat points")
	}
	// raid sizes grow with the map
	big := bareSetupGame(difficulty.Seasoned)
	big.grid = New(Config{Width: 500, Height: 500}).grid
	if big.maxRaiders() != 14 || bareSetupGame(difficulty.Seasoned).maxRaiders() != 8 {
		t.Fatalf("max raiders %d", big.maxRaiders())
	}
	if NewColony(Config{Seed: 1, Width: 60, Height: 60}).maxRaiders() != maxScaledRaiders {
		t.Fatal("legacy raids stay at 8")
	}
}

func TestDifficultyHooks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		preset                              difficulty.Preset
		crop, mine, butcher, research, mood int
		infection50, revenge50, infection90 int
		huntColonists                       bool
	}{
		{difficulty.Wanderer, 13, 13, 12, 13, 12, 200, 25, 360, false},
		{difficulty.Seasoned, 10, 10, 10, 10, 0, 500, 50, 900, true},
		{difficulty.Merciless, 8, 8, 8, 9, -8, 650, 65, 1000, true},
	}
	for _, c := range cases {
		g := bareSetupGame(c.preset)
		got := []int{g.diffScale(DiffCrop, 10), g.diffScale(DiffMine, 10), g.diffScale(DiffButcher, 10), g.diffScale(DiffResearch, 10), g.moodOffset()}
		want := []int{c.crop, c.mine, c.butcher, c.research, c.mood}
		if !slices.Equal(got, want) {
			t.Fatalf("%s yields/mood %v, want %v", c.preset, got, want)
		}
		if int(g.infectionChance(500)) != c.infection50 || g.revengePct(50) != c.revenge50 || int(g.infectionChance(900)) != c.infection90 { // permille (MED-02)
			t.Fatalf("%s chances %d %d %d", c.preset, g.infectionChance(500), g.revengePct(50), g.infectionChance(900))
		}
		if g.predatorsHuntColonists() != c.huntColonists {
			t.Fatalf("%s predators hunt colonists %v", c.preset, g.predatorsHuntColonists())
		}
		// a starving wolf next to a colonist, nothing else to eat
		pid, _ := g.SpawnPawn(20, 20)
		wid, err := g.spawnAnimal(animal.TimberWolf, 22, 20)
		if err != nil {
			t.Fatal(err)
		}
		g.invalidateCreatures()
		prey := g.findPrey(wid, Position{22, 20}, animal.DefOf(animal.TimberWolf))
		if (prey == pid) != c.huntColonists {
			t.Fatalf("%s: wolf picked %d (colonist %d)", c.preset, prey, pid)
		}
	}
	if bareSetupGame(difficulty.Wanderer).diffScale(DiffMine, 1) != 1 {
		t.Fatal("a mine yield of 1 never rounds to 0")
	}
	legacy := New(Config{Width: 10, Height: 10})
	if legacy.infectionChance(370) != 370 || legacy.revengePct(37) != 37 || !legacy.predatorsHuntColonists() {
		t.Fatal("legacy hooks must be neutral")
	}
}

func TestExtremeWeatherEvents(t *testing.T) {
	t.Parallel()
	g := bareSetupGame(difficulty.Seasoned)
	base := g.climate
	type span struct {
		kind     uint8
		from, to int
	}
	var spans []span
	cur := extremeNone
	for day := 1; day < 3000; day++ {
		now := uint64(day) * clock.TicksPerDay
		g.clock = clock.NewAt(clock.Tick(now))
		g.extremeDay(now)
		k := g.ng.extreme.Kind
		switch {
		case k != cur && k != extremeNone:
			spans = append(spans, span{kind: k, from: day})
			season := g.season(now)
			if k == extremeHeat && season != "Summer" || k == extremeCold && season != "Winter" {
				t.Fatalf("%d in %s", k, season)
			}
			if g.climate.YearMean == base.YearMean || g.World().Extreme == "" {
				t.Fatal("an extreme event does not change the climate or the view")
			}
		case k != cur:
			spans[len(spans)-1].to = day
			if g.climate != base {
				t.Fatalf("climate not restored: %+v", g.climate)
			}
		}
		cur = k
	}
	heat, cold := 0, 0
	for i, s := range spans {
		if s.to == 0 {
			continue
		}
		if d := s.to - s.from; d < extremeMinDays || d > extremeMinDays+extremeSpread-1 {
			t.Fatalf("event lasted %d days", d)
		}
		if i > 0 && s.from-spans[i-1].to < extremeCool/clock.TicksPerDay {
			t.Fatalf("events flapped: %d days apart", s.from-spans[i-1].to)
		}
		if s.kind == extremeHeat {
			heat++
		} else {
			cold++
		}
	}
	if heat == 0 || cold == 0 {
		t.Fatalf("heat waves %d, cold snaps %d in 50 years", heat, cold)
	}
	off := bareSetupGame(difficulty.Wanderer) // extreme weather off
	for day := 1; day < 3000; day++ {
		now := uint64(day) * clock.TicksPerDay
		off.clock = clock.NewAt(clock.Tick(now))
		off.extremeDay(now)
		if off.ng.extreme.Kind != extremeNone {
			t.Fatal("extreme weather while off")
		}
	}
}

func TestSetStorytellerCommand(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "ember_clan", nil))
	next := g.nextThreat
	g.Apply(command.Command{Kind: command.SetStoryteller, Name: "gambler"})
	g.Apply(command.Command{Kind: command.SetDifficulty, Name: "grim"})
	g.Step(1)
	if g.ng.setup.Storyteller != "gambler" || g.Story().Storyteller != "Kestrel Quay" {
		t.Fatalf("storyteller %q", g.ng.setup.Storyteller)
	}
	if g.ng.setup.Preset != difficulty.Grim || g.Story().Preset != "Grim" || g.diff() != difficulty.Of(difficulty.Grim) {
		t.Fatalf("difficulty %v", g.ng.setup.Preset)
	}
	if g.nextThreat != next {
		t.Fatal("the pending threat stays")
	}
	custom := difficulty.Of(difficulty.Seasoned)
	custom.ThreatScale = 222
	g.Apply(command.Command{Kind: command.SetStoryteller, Name: "nobody"})
	g.Apply(command.Command{Kind: command.SetDifficulty, Name: "custom:" + difficulty.Encode(custom)})
	g.Apply(command.Command{Kind: command.SetDifficulty, Name: "custom:ts=9999"})
	g.Step(1)
	if g.ng.setup.Storyteller != "gambler" || g.ng.setup.Preset != difficulty.Custom || g.ng.setup.Difficulty != custom {
		t.Fatalf("after custom: %q %v %+v", g.ng.setup.Storyteller, g.ng.setup.Preset, g.ng.setup.Difficulty)
	}
	legacy := New(Config{Width: 10, Height: 10})
	legacy.Apply(command.Command{Kind: command.SetStoryteller, Name: "gambler"})
	legacy.Step(1)
	if legacy.hasSetup() {
		t.Fatal("a legacy colony keeps its pacing")
	}
}

func TestMapSizes(t *testing.T) {
	t.Parallel()
	sizes := MapSizes()
	var sides []int
	for _, s := range sizes {
		sides = append(sides, s.Side)
		if s.Enabled != (s.Side <= MaxEnabledSide || s.Side == preReleaseSide && MaxEnabledSide < 200) {
			t.Fatalf("%d enabled %v", s.Side, s.Enabled)
		}
		if s.Large != (s.Side >= 350) {
			t.Fatalf("%d large %v", s.Side, s.Large)
		}
		if !s.Enabled && s.Reason == "" {
			t.Fatalf("%d disabled without a reason", s.Side)
		}
	}
	want := []int{200, 225, 250, 275, 300, 350, 400, 450, 500}
	if MaxEnabledSide < 200 {
		want = append([]int{120}, want...)
	}
	if !slices.Equal(sides, want) {
		t.Fatalf("sides %v, want %v", sides, want)
	}
}

func TestScenarioAndFactionViews(t *testing.T) {
	t.Parallel()
	views := BuiltinScenarios()
	if len(views) != 4 || views[0].Key != "broken_hull" || views[3].Hardness != 4 {
		t.Fatalf("builtins %d", len(views))
	}
	for _, v := range views {
		if len(v.Lines) == 0 || !v.Builtin || scenario.HasErrors(v.Problems) || !strings.Contains(v.Lines[0].Text, "\t") {
			t.Fatalf("%s: lines %d problems %v", v.Key, len(v.Lines), v.Problems)
		}
	}
	bad := ScenarioViewOf(scenario.Def{})
	if !scenario.HasErrors(bad.Problems) {
		t.Fatal("an empty scenario has errors")
	}
	fs := faction.Generate([]faction.Slot{{Type: faction.Settlers, Count: 1}, {Type: faction.Swarm, Count: 1}}, randx.New(1))
	fv := FactionViewsOf(fs)
	if len(fv) != 2 || fv[0].Hidden || !fv[1].Hidden || fv[0].Emblem != "ui_icon/faction_settlers" || fv[0].Accent == "" {
		t.Fatalf("faction views %+v", fv)
	}
}

func TestSetupStateRoundTrip(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "broken_hull", func(s *Setup) {
		s.Scenario.Parts = append(s.Scenario.Parts, scenario.Part{Kind: scenario.PartCondition, Cond: scenario.EndlessChill})
	}))
	g.Step(30)
	g.ng.adapt = difficulty.Adapter{Days: 12, Value: 3}
	g.ng.extreme = extremeEvent{Kind: extremeCold, Delta: coldSnapDelta, Until: 99}
	st := g.exportSetupState()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(st); err != nil {
		t.Fatal(err)
	}
	var back *SetupState
	if err := gob.NewDecoder(&buf).Decode(&back); err != nil {
		t.Fatal(err)
	}
	g2 := New(Config{Width: g.Width(), Height: g.Height()})
	g2.importSetupState(back)
	if g2.climate != g.climate || !reflect.DeepEqual(g2.ng.factions, g.ng.factions) || g2.ng.startDay != g.ng.startDay ||
		g2.ng.story != g.ng.story || g2.ng.plan != g.ng.plan || g2.ng.adapt != g.ng.adapt || g2.ng.extreme != g.ng.extreme ||
		len(g2.ng.pods) != len(g.ng.pods) || g2.ng.setup.Scenario.Name != g.ng.setup.Scenario.Name ||
		!reflect.DeepEqual(g2.ng.fx, g.ng.fx) || len(g2.ng.setup.Colonists) != 3 {
		t.Fatal("setup state did not survive a gob round trip")
	}
	if New(Config{Width: 5, Height: 5}).exportSetupState() != nil {
		t.Fatal("a legacy colony saves no setup state")
	}
}

func TestLossesEaseAndAdapt(t *testing.T) {
	t.Parallel()
	g := buildSetup(t, setupFor(t, "ember_clan", func(s *Setup) { s.Storyteller = "hearth" }))
	next := g.nextThreat
	id := g.pawnIDs[0]
	g.die(id, health.CauseInjury)
	now := uint64(g.clock.Now())
	g.checkLosses(now)
	g.checkLosses(now)
	if g.nextThreat != next+6*clock.TicksPerDay || !g.ng.lost || len(g.ng.dead) != 1 {
		t.Fatalf("loss ease: next %d (was %d), lost %v", g.nextThreat, next, g.ng.lost)
	}
	g.ng.adapt = difficulty.Adapter{Days: 40}
	g.dayRollover(now)
	if g.ng.adapt.Days != 15 || g.ng.lost {
		t.Fatalf("adaptation after a loss: %+v", g.ng.adapt)
	}
}
