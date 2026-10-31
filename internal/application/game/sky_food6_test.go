package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
)

// FOOD-6 sky events (wiki: Eclipse, Toxic fallout) and the growing
// zone's light rule (wiki: Growing zone).

// TestEclipseTakesTheSun: during an eclipse the noon sky is dark, a solar
// panel gives nothing and an open-sky crop stands still; afterwards all
// three come back.
func TestEclipseTakesTheSun(t *testing.T) {
	t.Parallel()
	g := genGame(t, 10, 12, 0, 0, 49, 49)
	panel := placeFuel(t, g, construction.SolarPanel, 20, 20)
	rice, _ := g.spawnPlant(plant.Rice, 40, 40, 0)
	if g.genOutput(panel) != 900 || g.cellLight(40, 42) == 0 {
		t.Fatal("noon: full sun before the eclipse")
	}
	if !g.FireIncident("eclipse") || g.FireIncident("eclipse") {
		t.Fatal("one eclipse at a time")
	}
	now := uint64(g.clock.Now())
	if g.genOutput(panel) != 0 || g.cellLight(40, 42) != 0 || !g.incidents.Eclipsed(now) {
		t.Fatalf("eclipse: solar %d W, sky light %v", g.genOutput(panel), g.cellLight(40, 42))
	}
	if w := g.World(); w.SkyEvent != "eclipse" || w.SkyEventEnds != g.incidents.EclipseUntil {
		t.Fatalf("view: sky %q until %d", w.SkyEvent, w.SkyEventEnds)
	}
	if st, _ := g.PlantGrowth(rice); st.State != growStateDark {
		t.Fatalf("an open-sky crop in the eclipse is %q, want dark", st.State)
	}
	before := growthAt(g, rice)
	g.Step(clock.TicksPerHour)
	if growthAt(g, rice) != before {
		t.Fatal("a crop grew in the eclipse")
	}
	l := g.letters.list
	if len(l) == 0 || l[len(l)-1].Title != "Eclipse" {
		t.Fatal("the eclipse sends a letter")
	}
	g.Step(int(g.incidents.EclipseUntil - uint64(g.clock.Now())))
	jumpTo(g, 13, 12)
	if g.genOutput(panel) != 900 || g.cellLight(40, 42) == 0 {
		t.Fatal("the sun is back after the eclipse")
	}
}

// TestToxicFalloutPoisonsTheOpenSky: a day of fallout gives a colonist
// outdoors 40 % toxic buildup and an animal outdoors 20 %, nothing to one
// under a roof; outdoor plants die within hours and roofed ones live;
// sheltered, the buildup recedes 8 % a day.
func TestToxicFalloutPoisonsTheOpenSky(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 14})
	g.ensureRoofGrid()
	out, _ := g.SpawnNamedPawn(4, 4, "Out")
	in, _ := g.SpawnNamedPawn(12, 4, "In")
	cow, _ := g.spawnAnimal(animal.Cow, 6, 9)
	g.setRoof(4*g.grid.Width()+12, roof.Natural)
	g.setRoof(6*g.grid.Width()+16, roof.Natural)
	field, _ := g.spawnPlant(plant.Potato, 2, 12, 0)
	shed, _ := g.spawnPlant(plant.Potato, 16, 6, 0)
	now := uint64(g.clock.Now())
	if !g.incidents.StartSky(incident.ToxicFallout, now, 3*clock.TicksPerDay) {
		t.Fatal("the fallout starts")
	}
	h0 := now/clock.TicksPerHour + 1
	for h := h0; h < h0+24; h++ {
		stepToxic(g, h*clock.TicksPerHour)
	}
	if got := body.ToxicBuildup(g.bodies[out]); got != body.ToxicPerDay {
		t.Fatalf("a day outdoors: %d, want %d", got, body.ToxicPerDay)
	}
	if got := body.ToxicBuildup(g.bodies[in]); got != 0 {
		t.Fatalf("under a roof: %d, want 0", got)
	}
	a, _ := g.animals.Get(cow)
	if got := body.ToxicBuildup(a.Body); got != body.ToxicPerDay/2 {
		t.Fatalf("an animal's day outdoors: %d, want %d", got, body.ToxicPerDay/2)
	}
	if pawnView(g, out).ToxicBuildup != 40 || animalViewOf(t, g, cow).ToxicBuildup != 20 || g.World().SkyEvent != "toxic fallout" {
		t.Fatal("the views show the buildup and the fallout")
	}
	has := func(id entity.EntityID) bool {
		return slices.ContainsFunc(g.thoughts(id), func(th mood.Thought) bool { return th.Kind == mood.ToxicFallout })
	}
	if !has(out) || has(in) {
		t.Fatal("the -5 fallout thought is for colonists under the open sky")
	}
	g.Step(4 * clock.TicksPerHour)
	if _, alive := g.plants.Get(field); alive {
		t.Fatal("an open-sky crop survived four hours of fallout")
	}
	if _, alive := g.plants.Get(shed); !alive {
		t.Fatal("a roofed crop died in the fallout")
	}
	g.incidents.FalloutUntil = 0 // over: everyone sheltered from now on
	sev := body.ToxicBuildup(g.bodies[out])
	for h := h0 + 100; h < h0+124; h++ {
		stepToxic(g, h*clock.TicksPerHour)
	}
	if got := body.ToxicBuildup(g.bodies[out]); got != sev-body.ToxicRecoverPerDay {
		t.Fatalf("a day after the fallout: %d, want %d", got, sev-body.ToxicRecoverPerDay)
	}
}

// TestToxicBuildupKills: a colonist left outdoors for two and a half days
// of fallout dies of it.
func TestToxicBuildupKills(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	id, _ := g.SpawnNamedPawn(4, 4, "Out")
	now := uint64(g.clock.Now())
	g.incidents.StartSky(incident.ToxicFallout, now, 10*clock.TicksPerDay)
	h0 := now/clock.TicksPerHour + 1
	for h := h0; h < h0+60; h++ {
		stepToxic(g, h*clock.TicksPerHour)
	}
	if v, _ := g.vital.Get(id); v.Status != health.Dead {
		t.Fatalf("alive after 60 hours outdoors in fallout (buildup %d)", body.ToxicBuildup(g.bodies[id]))
	}
	if _, st, ok := g.bodyOf(id); !ok || !st.Rotten() {
		t.Fatal("a body dead of full toxic buildup rots at once (wiki: Toxic buildup)")
	}
}

// TestToxicAnimalCorpseRots (wiki: Toxic fallout, Toxic buildup): an
// animal killed by the fallout leaves a rotten carcass; one killed
// without buildup a fresh one.
func TestToxicAnimalCorpseRots(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 10})
	cow, _ := g.spawnAnimal(animal.Cow, 4, 4)
	now := uint64(g.clock.Now())
	g.incidents.StartSky(incident.ToxicFallout, now, 10*clock.TicksPerDay)
	h0 := now/clock.TicksPerHour + 1
	for h := h0; h < h0+121; h++ {
		stepToxic(g, h*clock.TicksPerHour)
	}
	if _, alive := g.animals.Get(cow); alive {
		t.Fatal("a cow survived five days outdoors in fallout")
	}
	fresh := g.dropCorpse(animal.Goat, 9, 4)
	rotten := 0
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.CorpseAnimal && st.Animal == uint8(animal.Cow) && st.Rotten() {
			rotten++
		}
	}
	if st, _ := g.stacks.Get(fresh); rotten != 1 || st.Rotten() {
		t.Fatalf("rotten cow carcasses %d (want 1); goat rotten %v", rotten, st.Rotten())
	}
}

// TestRoofedZoneSowsOnlyUnderASunLamp (wiki: Growing zone, Plants, Sun
// lamp): a roofed growing-zone cell is sown only within a powered sun
// lamp's reach, and never with a tree.
func TestRoofedZoneSowsOnlyUnderASunLamp(t *testing.T) {
	t.Parallel()
	g := mildLight(t) // roofed all over
	g.addGrowZone(22, 21, 23, 22, uint8(plant.Rice))
	if g.sowable(22, 21) {
		t.Fatal("a dark roofed cell is sown")
	}
	sun := buildNow(t, g, construction.SunLamp, item.Steel, construction.South, 20, 20)
	sunPower(t, g, 20, 20, 4)
	if !g.powered(sun) {
		t.Fatal("the sun lamp has no power")
	}
	jumpTo(g, 11, 22) // night: the lamp is off, its reach still counts
	if !g.sowable(22, 21) {
		t.Fatal("a cell under a powered sun lamp is not sown")
	}
	g.addGrowZone(22, 30, 22, 30, uint8(plant.Rice))
	if g.sowable(22, 30) {
		t.Fatal("a cell beyond the sun lamp's reach is sown")
	}
	id, ok := ZoneSpeciesID("tree")
	if !ok {
		t.Fatal("no tree species id")
	}
	g.addGrowZone(20, 23, 20, 23, id)
	if g.sowable(20, 23) {
		t.Fatal("a tree is sown under a roof")
	}
}
