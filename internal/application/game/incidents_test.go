package game

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
)

// eventMap is an open wild map of biome b at day 20 (every event's first
// day has passed) with a colonist at its centre.
func eventMap(t *testing.T, seed uint64, w, h int, b biome.ID) *Game {
	t.Helper()
	g := wildMapOf(seed, w, h, b)
	g.clock = clock.NewAt(clock.Tick(20*clock.TicksPerDay + 9*clock.TicksPerHour))
	if _, err := g.SpawnNamedPawn(w/2, h/2, "Ada"); err != nil {
		t.Fatal(err)
	}
	return g
}

// newAnimals are the animals spawned since before (a count of animalIDs).
func newAnimals(g *Game, before int) []entity.EntityID {
	return append([]entity.EntityID(nil), g.animalIDs[before:]...)
}

func lastMessage(g *Game) MessageView {
	if len(g.messages) == 0 {
		return MessageView{}
	}
	return g.messages[len(g.messages)-1]
}

func TestHerdCrossesEdgeToEdge(t *testing.T) {
	g := eventMap(t, 3, 60, 40, biome.Tundra)
	before := len(g.animalIDs)
	if !g.FireIncident("herd_crossing") {
		t.Fatal("no crossing")
	}
	herd := newAnimals(g, before)
	if len(herd) < crossMin || len(herd) > crossMax {
		t.Fatalf("%d crossers", len(herd))
	}
	first, _ := g.animals.Get(herd[0])
	start, _ := g.positions.Get(herd[0])
	w, h := g.grid.Width(), g.grid.Height()
	for _, id := range herd {
		a, _ := g.animals.Get(id)
		if a.Species != first.Species || !a.Leaving || a.Tame || !crosser(a.Species) || animal.Weight(a.Species, biome.Tundra) == 0 {
			t.Fatalf("crosser %+v", a)
		}
		// the exit is on the edge opposite the entry, within crossDrift
		switch {
		case start.X <= 2:
			if a.ExitX != w-1 || abs(a.ExitY-start.Y) > crossDrift+2 {
				t.Fatalf("entered west at %v, exit %d,%d", start, a.ExitX, a.ExitY)
			}
		case start.X >= w-3:
			if a.ExitX != 0 || abs(a.ExitY-start.Y) > crossDrift+2 {
				t.Fatalf("entered east at %v, exit %d,%d", start, a.ExitX, a.ExitY)
			}
		case start.Y <= 2:
			if a.ExitY != h-1 || abs(a.ExitX-start.X) > crossDrift+2 {
				t.Fatalf("entered north at %v, exit %d,%d", start, a.ExitX, a.ExitY)
			}
		default:
			if a.ExitY != 0 || abs(a.ExitX-start.X) > crossDrift+2 {
				t.Fatalf("entered south at %v, exit %d,%d", start, a.ExitX, a.ExitY)
			}
		}
	}
	m := lastMessage(g)
	if m.Focus != herd[0] || !strings.Contains(m.Text, "is crossing the land") || m.Danger {
		t.Fatalf("message %+v", m)
	}
	if n := g.recountWild(uint64(g.clock.Now())); n != 0 {
		t.Fatalf("crossers are not counted against capacity: %d", n)
	}
	for i := 0; i < 2*clock.TicksPerDay; i++ {
		g.Step(1)
		gone := true
		for _, id := range herd {
			if _, ok := g.animals.Get(id); ok {
				gone = false
			}
		}
		if gone {
			t.Logf("crossed in %d ticks", i)
			return
		}
	}
	t.Fatal("the herd never reached the far edge")
}

func TestStrayJoinsTame(t *testing.T) {
	for _, b := range []biome.ID{biome.TemperateForest, biome.IceSheet, biome.Desert} {
		g := eventMap(t, 4, 40, 40, b)
		before := len(g.animalIDs)
		if !g.FireIncident("stray_joins") {
			t.Fatalf("%s: no stray", biome.DefOf(b).Key)
		}
		got := newAnimals(g, before)
		if len(got) != 1 {
			t.Fatalf("%d strays", len(got))
		}
		a, _ := g.animals.Get(got[0])
		if !a.Tame || a.Name == "" || !evHasSpecies(straySpecies, a.Species) || !g.climateFit(a.Species) {
			t.Fatalf("%s: stray %+v", biome.DefOf(b).Key, a)
		}
		if b == biome.IceSheet && a.Species != animal.Husky && a.Species != animal.Cat {
			t.Fatalf("an ice sheet stray is a %s", a.Species)
		}
		if b == biome.Desert && (a.Species == animal.Husky || a.Species == animal.Labrador || a.Species == animal.Horse) {
			t.Fatalf("a desert stray is a %s", a.Species)
		}
		if p, _ := g.positions.Get(got[0]); !onEdge(g, p, 2) {
			t.Fatalf("the stray walked in at %v", p)
		}
		if !evHasLog(g, got[0], logJoined) {
			t.Fatal("its log says it joined")
		}
		m := lastMessage(g)
		if m.Focus != got[0] || !strings.Contains(m.Text, a.Name) || !strings.Contains(m.Text, "has joined the colony") {
			t.Fatalf("message %+v", m)
		}
		// it heads for home like any tame animal
		g.Step(3 * clock.TicksPerHour)
		p, _ := g.positions.Get(got[0])
		if chebyshev(p, Position{X: 20, Y: 20}) > 14 {
			t.Fatalf("%s: the stray is still at %v", biome.DefOf(b).Key, p)
		}
	}
}

func evHasLog(g *Game, id entity.EntityID, kind uint8) bool {
	for _, e := range g.logs[id] {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

func evHasSpecies(list []animal.Species, s animal.Species) bool {
	for _, o := range list {
		if o == s {
			return true
		}
	}
	return false
}

func TestFarmAnimalsBothSexes(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		b := []biome.ID{biome.TemperateForest, biome.Desert, biome.Tundra, biome.TropicalRainforest}[seed%4]
		g := eventMap(t, seed, 40, 40, b)
		before := len(g.animalIDs)
		if !g.FireIncident("farm_animals") {
			t.Fatalf("seed %d: no farm animals", seed)
		}
		got := newAnimals(g, before)
		if len(got) < farmMin || len(got) > farmMax {
			t.Fatalf("%d farm animals", len(got))
		}
		females, males := 0, 0
		for _, id := range got {
			a, _ := g.animals.Get(id)
			first, _ := g.animals.Get(got[0])
			if !a.Tame || a.Name == "" || a.Species != first.Species || !evHasSpecies(farmSpecies, a.Species) || !g.climateFit(a.Species) {
				t.Fatalf("seed %d (%s): farm animal %+v", seed, biome.DefOf(b).Key, a)
			}
			if a.Female {
				females++
			} else {
				males++
			}
		}
		if females == 0 || males == 0 {
			t.Fatalf("seed %d: %d females, %d males", seed, females, males)
		}
		if m := lastMessage(g); m.Focus != got[0] || !strings.Contains(m.Text, "wandered in and stayed") {
			t.Fatalf("message %+v", m)
		}
	}
	// the catalogue keeps farm animals off the ice
	if incident.DefOf(incident.FarmAnimals).Allowed(incident.State{}, 20*clock.TicksPerDay, biome.SeaIce) {
		t.Fatal("farm animals on sea ice")
	}
}

func TestClimateFit(t *testing.T) {
	g := wildMapOf(1, 10, 10, biome.Desert)
	if g.climateFit(animal.Cow) || g.climateFit(animal.Husky) || !g.climateFit(animal.Goat) || !g.climateFit(animal.Donkey) {
		t.Fatal("desert fit: no cows or huskies, goats and donkeys")
	}
	g = wildMapOf(1, 10, 10, biome.BorealForest)
	if !g.climateFit(animal.Yak) || !g.climateFit(animal.Husky) || g.climateFit(animal.YorkshireTerrier) || !g.climateFit(animal.Horse) {
		t.Fatal("boreal fit: yaks and huskies, horses (wild there), no terriers")
	}
}

func TestSelfTameNearHome(t *testing.T) {
	g := eventMap(t, 5, 80, 80, biome.TemperateForest)
	near, _ := g.SpawnAnimal("deer", 45, 40)
	wolf, _ := g.SpawnAnimal("timber_wolf", 38, 42)
	far, _ := g.SpawnAnimal("deer", 78, 78)
	stag, _ := g.SpawnAnimal("glimmerstag", 42, 38)
	if !g.FireIncident("taken_a_liking") {
		t.Fatal("no animal took a liking")
	}
	a, _ := g.animals.Get(near)
	if !a.Tame || a.Name == "" || a.Herd != 0 || !evHasLog(g, near, logLiking) {
		t.Fatalf("the near deer %+v", a)
	}
	for _, id := range []entity.EntityID{wolf, far, stag} {
		if o, _ := g.animals.Get(id); o.Tame {
			t.Fatalf("%s %d tamed itself", o.Species, id)
		}
	}
	m := lastMessage(g)
	if m.Focus != near || !strings.Contains(m.Text, "has taken a liking to the colony: meet "+a.Name) {
		t.Fatalf("message %+v", m)
	}
	if g.FireIncident("taken_a_liking") {
		t.Fatal("no candidate left: the far deer is out of reach, the wolf a predator")
	}
}

func TestTimbergnawsEatTreesAndLeave(t *testing.T) {
	g := eventMap(t, 6, 50, 30, biome.BorealForest)
	for i := 0; i < 14; i++ {
		if _, err := g.spawnPlant(plant.Pine, 8+i*2, 6+i%3*8, plant.DefOf(plant.Pine).GrowTicks); err != nil {
			t.Fatal(err)
		}
	}
	if !g.incidentFits(incident.TimbergnawGnawing, uint64(g.clock.Now())) {
		t.Fatal("14 trees host a gnawing")
	}
	before := len(g.animalIDs)
	if !g.FireIncident("timbergnaw_gnawing") {
		t.Fatal("no timbergnaws")
	}
	gnaws := newAnimals(g, before)
	if len(gnaws) < gnawMin || len(gnaws) > gnawMax {
		t.Fatalf("%d timbergnaws", len(gnaws))
	}
	if n, felled := g.Timbergnaws(); n != len(gnaws) || felled != 0 {
		t.Fatalf("Timbergnaws() = %d, %d", n, felled)
	}
	if g.incidentFits(incident.TimbergnawGnawing, uint64(g.clock.Now())) {
		t.Fatal("one gnawing at a time")
	}
	trees := g.livingTrees(100)
	for i := 0; i < clock.TicksPerDay; i++ {
		g.Step(1)
		if _, felled := g.Timbergnaws(); felled >= 2 {
			break
		}
	}
	_, felled := g.Timbergnaws()
	if felled < 2 || g.livingTrees(100) != trees-felled {
		t.Fatalf("felled %d, trees %d -> %d", felled, trees, g.livingTrees(100))
	}
	// four days on they leave (or sooner, with no tree left in reach)
	for i := 0; i < 5*clock.TicksPerDay && len(g.wl.Gnaw.IDs) > 0; i++ {
		g.Step(1)
		if i%clock.TicksPerHour == 0 {
			if n, _ := g.Timbergnaws(); n == 0 && !g.wl.Gnaw.Gone {
				t.Fatal("gnaws gone missing before they were sent off")
			}
		}
	}
	for _, id := range gnaws {
		if _, ok := g.animals.Get(id); ok {
			t.Fatalf("timbergnaw %d never left", id)
		}
	}
	if n, _ := g.Timbergnaws(); n != 0 {
		t.Fatal("no timbergnaws after they left")
	}
	found := false
	for _, m := range g.messages {
		found = found || strings.HasPrefix(m.Text, "The timbergnaws are leaving")
	}
	if !found {
		t.Fatal("no leaving message")
	}
}

func TestMadAnimalGoesManhunter(t *testing.T) {
	g := eventMap(t, 7, 100, 100, biome.TemperateForest)
	near, _ := g.SpawnAnimal("deer", 60, 70)
	far, _ := g.SpawnAnimal("deer", 99, 99) // beyond madRadius of home
	pet, _ := g.SpawnAnimal("labrador", 51, 51)
	setAnimal(g, pet, func(a *animal.State) { a.Tame, a.Name = true, "Rex" })
	if pickThreat(madRoll, 6, false) != threatMad || pickThreat(madRoll, 5, true) != threatRaid {
		t.Fatal("the mad animal joins the threat die from day 6")
	}
	now := uint64(g.clock.Now())
	if !g.madAnimal(now, false) {
		t.Fatal("no animal went mad")
	}
	if a, _ := g.animals.Get(near); !a.Enraged(now) {
		t.Fatalf("the deer near home %+v", a)
	}
	for _, id := range []entity.EntityID{far, pet} {
		if a, _ := g.animals.Get(id); a.Enraged(now) {
			t.Fatalf("%s went mad", a.Species)
		}
	}
	if m := lastMessage(g); !m.Danger || !strings.Contains(m.Text, "gone manhunter") {
		t.Fatalf("message %+v", m)
	}
	// cooling down: a second roll within 10 days is a raid instead
	other, _ := g.SpawnAnimal("deer", 45, 45)
	if g.madAnimal(now+clock.TicksPerDay, false) {
		t.Fatal("mad animals cool down")
	}
	if a, _ := g.animals.Get(other); a.Enraged(now) {
		t.Fatal("cooling down")
	}
	if !g.madAnimal(now+11*clock.TicksPerDay, false) {
		t.Fatal("after the cooldown another may go mad")
	}
	early := eventMap(t, 7, 60, 60, biome.TemperateForest)
	early.clock = clock.NewAt(clock.Tick(3 * clock.TicksPerDay))
	_, _ = early.SpawnAnimal("deer", 31, 31)
	if early.madAnimal(uint64(early.clock.Now()), false) {
		t.Fatal("no mad animal before day 6")
	}
}

// TestNoEventDuringThreat: a due nature roll waits while a hostile is on
// the map, then fires once it is gone.
func TestNoEventDuringThreat(t *testing.T) {
	g := eventMap(t, 8, 60, 60, biome.TemperateForest)
	for i := 0; i < 6; i++ {
		_, _ = g.SpawnAnimal("deer", 20+i, 25)
	}
	mad, _ := g.SpawnAnimal("bison", 10, 10)
	now := uint64(g.clock.Now())
	setAnimal(g, mad, func(a *animal.State) { a.Manhunter, a.Mode = now+animal.ManhunterTicks, animal.Attack })
	g.incidents.NextNature = now
	before := len(g.animalIDs)
	g.Step(1)
	if len(g.incidents.Last) != 0 || len(g.animalIDs) != before {
		t.Fatalf("an event fired during a threat: %v", g.incidents.Last)
	}
	if g.incidents.NextNature != now+natureRetry {
		t.Fatalf("the roll waits an hour: next %d, now %d", g.incidents.NextNature, now)
	}
	g.removeAnimal(mad)
	for i := 0; i <= natureRetry && len(g.incidents.Last) == 0; i++ {
		g.Step(1)
	}
	if len(g.incidents.Last) != 1 {
		t.Fatalf("no event once the threat was gone: %v", g.incidents.Last)
	}
	if next := g.incidents.NextNature; next < now+incident.NatureGapMin*clock.TicksPerDay-clock.TicksPerDay {
		t.Fatalf("next roll %d too soon", next)
	}
}

func TestIncidentCooldownsAndSave(t *testing.T) {
	g := NewColony(Config{Seed: 9, Width: 48, Height: 40})
	if g.incidents.NextNature == 0 {
		g.Step(1)
	}
	if next := g.incidents.NextNature; next < incident.NatureGapMin*clock.TicksPerDay || next > (incident.NatureGapMax+1)*clock.TicksPerDay {
		t.Fatalf("first nature roll at tick %d", next)
	}
	g.clock = clock.NewAt(clock.Tick(20*clock.TicksPerDay + 7*clock.TicksPerHour))
	now := uint64(g.clock.Now())
	g.incidents.NextNature = now
	g.nextThreat = now + 30*clock.TicksPerDay // the jump would bring a raid first
	g.Step(1)
	if len(g.incidents.Last) != 1 {
		t.Fatalf("one event at the roll: %v", g.incidents.Last)
	}
	var fired incident.ID
	for id := range g.incidents.Last {
		fired = id
	}
	if incident.DefOf(fired).Allowed(g.incidents, now+clock.TicksPerDay, g.biome) {
		t.Fatalf("%s cools down", incident.DefOf(fired).Key)
	}
	// round trip: the schedule, the cooldowns and the wildlife counts load,
	// and the game goes on the same
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l.incidents, g.incidents) || l.wl.Count != g.wl.Count || l.wl.CountHour != g.wl.CountHour {
		t.Fatalf("loaded %+v / %d, want %+v / %d", l.incidents, l.wl.Count, g.incidents, g.wl.Count)
	}
	g.Step(3 * clock.TicksPerHour)
	l.Step(3 * clock.TicksPerHour)
	if !reflect.DeepEqual(l.incidents, g.incidents) || !reflect.DeepEqual(speciesOf(l), speciesOf(g)) || l.wildCount() != g.wildCount() {
		t.Fatal("the loaded game diverged")
	}
}
