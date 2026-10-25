package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Pens (PLAY-015 §2): marker pens, enclosure, gates, roping, unpenned
// livestock, migration.

// buildMarker builds a pen marker at (x, y) and returns its id.
func buildMarker(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	if !g.BuildNow("pen marker", "wood", "south", x, y) {
		t.Fatalf("pen marker at %d,%d not built", x, y)
	}
	id := g.layerBldgAt(x, y, construction.LayerObject)
	if b, ok := g.bldgs.Get(id); !ok || b.Kind != construction.PenMarker {
		t.Fatalf("no pen marker at %d,%d", x, y)
	}
	return id
}

// penWorld is a 40 x 24 map with a fenced 7 x 7 pen (gate on its south
// side at 24,12) and a marker in it.
func penWorld(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	bareGround(g)
	fenceRing(t, g, 20, 4, 28, 12, [2]int{24, 12})
	return g, buildMarker(t, g, 24, 8)
}

func tameAt(t *testing.T, g *Game, species, name string, x, y int) entity.EntityID {
	t.Helper()
	id, err := g.SpawnAnimal(species, x, y)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(id, name)
	return id
}

func penOf(g *Game, m entity.EntityID) PenView {
	for _, p := range g.World().Pens {
		if p.ID == m {
			return p
		}
	}
	return PenView{}
}

func TestPenEnclosureFloodFill(t *testing.T) {
	g, m := penWorld(t)
	c := g.penRegion(m)
	if !c.enclosed || c.cells != 49 || c.minX != 21 || c.maxX != 27 || c.minY != 5 || c.maxY != 11 {
		t.Fatalf("a fenced 7x7 field is a 49-cell pen: %+v", *c)
	}
	if g.inPenCells(c, 24, 12) || !g.inPenCells(c, 21, 5) || g.inPenCells(c, 20, 8) {
		t.Fatal("the gate and the fence are not in the pen; its corner is")
	}
	v := penOf(g, m)
	if !v.Enclosed || v.Name != "Pen 1" || v.Bits == nil || v.X != 24 || v.Y != 8 {
		t.Fatalf("pen view %+v", v)
	}
	// a marker in the open: the fill reaches the map edge
	open := buildMarker(t, g, 5, 18)
	if c := g.penRegion(open); c.enclosed || c.bits != nil {
		t.Fatalf("an unfenced marker is not enclosed: %+v", *c)
	}
	if penOf(g, open).Name != "Pen 2" {
		t.Fatal("pens are numbered in marker order")
	}
	// open country: a ring too big for a pen
	h := New(Config{Seed: 1, Width: 70, Height: 70})
	bareGround(h)
	fenceRing(t, h, 2, 2, 60, 60)
	if c := h.penRegion(buildMarker(t, h, 30, 30)); c.enclosed {
		t.Fatalf("a %d-cell field is not a pen", 57*57)
	}
}

func TestBrokenFenceOpensPen(t *testing.T) {
	g, m := penWorld(t)
	g.Step(penThinkEvery) // the pass notes the pen enclosed
	v := g.World().PenVer
	post := g.layerBldgAt(20, 8, construction.LayerObject)
	if post == 0 {
		t.Fatal("no fence post")
	}
	g.removeBuilding(post)
	if c := g.penRegion(m); c.enclosed {
		t.Fatal("a gap in the fence opens the pen")
	}
	if g.World().PenVer == v {
		t.Fatal("PenVer moves with the enclosure")
	}
	g.Step(penThinkEvery)
	if !hasMessage(g, "Pen 1 is open") {
		t.Fatal("the open-pen message")
	}
	n := len(g.messages)
	g.Step(penThinkEvery * 3)
	for _, msg := range g.messages[n:] {
		if strings.Contains(msg.Text, "is open") {
			t.Fatal("the open-pen message fires once per change")
		}
	}
	// mended: enclosed again
	if !g.BuildNow("fence", "wood", "south", 20, 8) || !g.penRegion(m).enclosed {
		t.Fatal("a mended fence closes the pen")
	}
}

func TestLivestockBlockedByGatesUnlessLed(t *testing.T) {
	g, _ := penWorld(t)
	cow := tameAt(t, g, "cow", "Bess", 24, 9)
	dog := tameAt(t, g, "husky", "Ash", 23, 9)
	if !animal.RosterOf(animalOf(g, cow).Species).Penned || animal.RosterOf(animalOf(g, dog).Species).Penned {
		t.Fatal("cows are livestock, huskies area animals")
	}
	a := animalOf(g, cow)
	g.animalGoTo(cow, &a, 24, 18)
	if len(a.Path) != 0 {
		t.Fatalf("a cow does not walk out of the gate on its own: %v", a.Path)
	}
	d := animalOf(g, dog)
	g.animalGoTo(dog, &d, 24, 18)
	if len(d.Path) == 0 {
		t.Fatal("a pet passes the gate")
	}
	// led on a rope, the cow passes
	ada, _ := g.SpawnNamedPawn(24, 10, "Ada")
	if !g.reserve.TryEntity(ada, cow) {
		t.Fatal("reserve")
	}
	g.jobs.Set(ada, job.Record{Kind: job.Rope, Target: cow, State: job.Do, Count: 1})
	if !g.gatesOpen(cow, &a) {
		t.Fatal("a led cow passes gates")
	}
	g.animalGoTo(cow, &a, 24, 18)
	if len(a.Path) == 0 {
		t.Fatal("a led cow walks out through the gate")
	}
	// wild animals never pass
	wild, _ := g.SpawnAnimal("cow", 24, 16)
	w := animalOf(g, wild)
	if g.gatesOpen(wild, &w) {
		t.Fatal("wild animals don't pass gates")
	}
}

// handlerOnly makes id a handler and nothing else.
func handlerOnly(g *Game, id entity.EntityID) {
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		g.setPriority(id, wt, 0)
	}
	g.setPriority(id, work.Handle, work.DefaultPriority)
}

func penRunUntil(g *Game, pawns []entity.EntityID, ticks int, done func() bool) bool {
	for i := 0; i < ticks; i++ {
		for _, p := range pawns {
			g.SetNeeds(p, needs.Full, needs.Full)
		}
		g.Step(1)
		if done() {
			return true
		}
	}
	return false
}

func TestRopeLeadsCowIntoPen(t *testing.T) {
	g, m := penWorld(t)
	cow := tameAt(t, g, "cow", "Bess", 8, 18)
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	handlerOnly(g, ada)
	if g.penTarget(cow, g.animals.Ref(cow), Position{X: 8, Y: 18}) != m {
		t.Fatal("the cow belongs in the pen")
	}
	c := g.penRegion(m)
	roped, led := false, false
	in := penRunUntil(g, []entity.EntityID{ada}, 4000, func() bool {
		j, _ := g.jobs.Get(ada)
		if j.Kind == job.Rope {
			roped = true
			led = led || j.Count == 1
			for _, av := range g.World().Animals {
				if av.ID == cow && (!av.Penned || av.Pen != "Being roped") {
					t.Fatalf("a roped cow's pen line %+v", av)
				}
			}
		}
		p, _ := g.positions.Get(cow)
		return j.Kind != job.Rope && g.inPenCells(c, p.X, p.Y)
	})
	if !in || !roped || !led {
		p, _ := g.positions.Get(cow)
		t.Fatalf("the handler ropes the cow and leads it in: in %v roped %v led %v, cow at %v", in, roped, led, p)
	}
	for _, av := range g.World().Animals {
		if av.ID == cow && av.Pen != "Pen 1" {
			t.Fatalf("a penned cow shows its pen: %q", av.Pen)
		}
	}
	if v := penOf(g, m); v.Animals != 1 {
		t.Fatalf("the pen counts its herd: %+v", v)
	}
	// released, it stays in
	g.Step(3000)
	if p, _ := g.positions.Get(cow); !g.inPenCells(c, p.X, p.Y) {
		t.Fatalf("a released cow stays penned, at %v", p)
	}
}

func TestRopeRefusedOutsideHandlerReach(t *testing.T) {
	g, _ := penWorld(t)
	cow := tameAt(t, g, "cow", "Bess", 8, 18)
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	handlerOnly(g, ada)
	g.postpone(postKey{Pawn: ada, Kind: job.Rope, Target: cow}, job.Unreachable)
	if g.assignRope(ada) {
		t.Fatal("a postponed rope job is not taken")
	}
	// no pen accepting cows: nothing to rope
	h, m := penWorld(t)
	s := defaultPenSettings()
	s.Species = []PenSpecies{{Key: "cow", Allow: false, Male: true, Female: true}}
	h.Enqueue(command.Command{Kind: command.SetPenSettings, Target: uint64(m), Name: EncodePenSettings(s)})
	h.Step(1)
	tameAt(t, h, "cow", "Bess", 8, 18)
	bo, _ := h.SpawnNamedPawn(4, 4, "Bo")
	if h.assignRope(bo) {
		t.Fatal("no rope job when no pen takes the animal")
	}
}

func TestPenSettingsFilterAndReRope(t *testing.T) {
	g, m := penWorld(t)
	fenceRing(t, g, 4, 4, 12, 12, [2]int{8, 12})
	m2 := buildMarker(t, g, 8, 8)
	cow := tameAt(t, g, "cow", "Bess", 24, 8)
	setAnimal(g, cow, func(a *animal.State) { a.HasSex, a.Female = true, true })
	a := g.animals.Ref(cow)
	if g.penTarget(cow, a, Position{X: 24, Y: 8}) != m {
		t.Fatal("standing in an accepting pen, that is its pen")
	}
	// males only in pen 1: the cow belongs in pen 2
	s := defaultPenSettings()
	for i := range s.Species {
		if s.Species[i].Key == "cow" {
			s.Species[i].Female = false
		}
	}
	g.Enqueue(command.Command{Kind: command.SetPenSettings, Target: uint64(m), Name: EncodePenSettings(s)})
	g.Enqueue(command.Command{Kind: command.SetPenSettings, Target: uint64(m), Name: "not json"})
	g.Step(1)
	if got := penOf(g, m).Settings; len(got.Species) != len(s.Species) {
		t.Fatalf("settings applied (bad payload ignored): %+v", got)
	}
	if g.penTarget(cow, a, Position{X: 24, Y: 8}) != m2 {
		t.Fatal("a pen that no longer takes her: the next accepting pen")
	}
	ada, _ := g.SpawnNamedPawn(30, 16, "Ada")
	handlerOnly(g, ada)
	c2 := g.penRegion(m2)
	if !penRunUntil(g, []entity.EntityID{ada}, 5000, func() bool {
		p, _ := g.positions.Get(cow)
		j, _ := g.jobs.Get(ada)
		return j.Kind != job.Rope && g.inPenCells(c2, p.X, p.Y)
	}) {
		p, _ := g.positions.Get(cow)
		t.Fatalf("re-roped into pen 2: cow at %v", p)
	}
	if !penAccepts(defaultPenSettings(), animal.Cow, true) || penAccepts(s, animal.Cow, true) || !penAccepts(s, animal.Cow, false) {
		t.Fatal("penAccepts by species and sex")
	}
	if !penAccepts(PenSettings{}, animal.Cow, true) {
		t.Fatal("a species the list doesn't name is taken")
	}
}

func TestPenFoodFigures(t *testing.T) {
	g, m := penWorld(t)
	sowGrass(g, 21, 5, 27, 11) // 49 tiles
	v := penOf(g, m)
	perTile := animal.GrazeFood * clock.TicksPerDay / plant.GrassFillTicks
	if v.GrazePerDay != 49*perTile || v.DemandPerDay != 0 || v.TroughStock != 0 {
		t.Fatalf("grazing for 49 full grass tiles: %+v (want %d)", v, 49*perTile)
	}
	tameAt(t, g, "cow", "Bess", 24, 9)
	placeBuilt(t, g, construction.FeedingTrough, construction.South, 22, 6)
	var bid entity.EntityID
	for _, id := range g.bldgIDs {
		if b, _ := g.bldgs.Get(id); b.Kind == construction.FeedingTrough {
			bid = id
		}
	}
	sid, _ := g.SpawnStack(item.Berries, 5, 22, 7)
	g.absorbInto(bid, sid)
	v = penOf(g, m)
	want := animal.FoodPerDay * 100 / animal.Meal(animal.Cow, 100)
	if v.DemandPerDay != want || v.TroughStock <= 0 || v.Animals != 1 {
		t.Fatalf("a cow's demand %d and the trough's stock: %+v", want, v)
	}
	if v.GrazePerDay >= 49*perTile {
		t.Fatal("the trough's cell holds no grass")
	}
}

func TestPenAutoCut(t *testing.T) {
	g, m := penWorld(t)
	in, _ := g.spawnPlant(plant.Tree, 22, 6, plant.DefOf(plant.Tree).GrowTicks)
	out, _ := g.spawnPlant(plant.Tree, 10, 18, plant.DefOf(plant.Tree).GrowTicks)
	berry, _ := g.spawnPlant(plant.BerryBush, 26, 10, plant.DefOf(plant.BerryBush).GrowTicks)
	if !g.penRegion(m).enclosed {
		t.Fatal("pen")
	}
	g.penTick(penAutoCutEvery)
	if p, _ := g.plants.Get(in); !p.Cut {
		t.Fatal("a tree in the pen is designated for cutting")
	}
	if p, _ := g.plants.Get(out); p.Cut {
		t.Fatal("a tree outside is left alone")
	}
	if p, _ := g.plants.Get(berry); p.Cut {
		t.Fatal("a berry bush is not on the default list")
	}
	if !strings.Contains(strings.Join(defaultAutoCut(), ","), "bramble") {
		t.Fatal("brambles are cut by default")
	}
}

func TestUnpennedLivestockRoamAndLeave(t *testing.T) {
	g := New(Config{Seed: 3, Width: 120, Height: 30})
	bareGround(g)
	cow := tameAt(t, g, "cow", "Bess", 100, 15)
	pet := tameAt(t, g, "husky", "Ash", 100, 16)
	g.paintHome(5, 5, 8, 8, true) // Home far west
	g.penTick(penThinkEvery)
	if s := g.pen.since[cow]; s != penThinkEvery {
		t.Fatalf("an unpenned cow's clock starts: %d", s)
	}
	if _, ok := g.pen.since[pet]; ok {
		t.Fatal("pets have no pen clock")
	}
	for _, av := range g.World().Animals {
		if av.ID == cow && (!av.Penned || av.Pen != "Roaming") {
			t.Fatalf("roaming livestock: %+v", av)
		}
		if av.ID == pet && av.Penned {
			t.Fatal("a pet is no penned animal")
		}
	}
	// days 1 and 2: no leaving, whatever the dice
	for d := uint64(1); d < penLeaveDays; d++ {
		g.penTick(d * clock.TicksPerDay)
		if animalOf(g, cow).Leaving {
			t.Fatalf("day %d: too early to leave", d)
		}
	}
	left := 0
	for d := uint64(penLeaveDays + 1); d < 80 && left == 0; d++ {
		g.penTick(d * clock.TicksPerDay)
		if animalOf(g, cow).Leaving {
			left = int(d)
		}
	}
	if left == 0 || !hasMessage(g, "Bess wandered off") {
		t.Fatal("an unpenned cow far from Home wanders off in the end")
	}
	a := animalOf(g, cow)
	if a.ExitY != g.grid.Height()-1 { // 14 cells south, 19 east
		t.Fatalf("it heads for the nearest edge: %d,%d", a.ExitX, a.ExitY)
	}
	// near Home: never
	h := New(Config{Seed: 3, Width: 120, Height: 30})
	near := tameAt(t, h, "cow", "Moo", 10, 10)
	h.paintHome(5, 5, 8, 8, true)
	h.penTick(penThinkEvery)
	for d := uint64(1); d < 60; d++ {
		h.penTick(d * clock.TicksPerDay)
	}
	if animalOf(h, near).Leaving {
		t.Fatal("livestock near Home stay")
	}
}

func TestUnpennedClockResetNoFlap(t *testing.T) {
	g := New(Config{Seed: 3, Width: 40, Height: 24})
	bareGround(g)
	cow := tameAt(t, g, "cow", "Bess", 8, 18)
	g.penTick(penThinkEvery)
	if _, ok := g.pen.since[cow]; !ok {
		t.Fatal("clock running")
	}
	// a pen appears: the cow has somewhere to go, the clock resets
	fenceRing(t, g, 20, 4, 28, 12, [2]int{24, 12})
	m := buildMarker(t, g, 24, 8)
	g.penTick(4 * clock.TicksPerDay)
	if _, ok := g.pen.since[cow]; ok {
		t.Fatal("with a pen to go to the clock resets")
	}
	// the pen breaks open again: the clock starts over, so no leave roll
	// can come before three more days
	g.removeBuilding(g.layerBldgAt(20, 8, construction.LayerObject))
	if g.penRegion(m).enclosed {
		t.Fatal("open")
	}
	g.penTick(5 * clock.TicksPerDay)
	if s := g.pen.since[cow]; s != 5*clock.TicksPerDay {
		t.Fatalf("the clock restarts: %d", s)
	}
	for d := uint64(6); d < 5+penLeaveDays; d++ {
		g.penTick(d * clock.TicksPerDay)
		if animalOf(g, cow).Leaving {
			t.Fatal("no leaving within three days of losing the pen")
		}
	}
	// roping resets it too
	a := g.animals.Ref(cow)
	a.Leaving = true
	g.roped(cow, a)
	if _, ok := g.pen.since[cow]; ok || a.Leaving {
		t.Fatal("a rope cancels the leave and resets the clock")
	}
}

func TestPaintedPenMigrates(t *testing.T) {
	g := New(Config{Seed: 1, Width: 40, Height: 24})
	bareGround(g)
	fenceRing(t, g, 20, 4, 28, 12, [2]int{24, 12})
	zoneCmd(g, command.DesignatePen, 21, 5, 27, 11, 0)
	zoneCmd(g, command.DesignatePen, 4, 14, 8, 18, 0) // unfenced
	h := saveLoad(t, g)
	found := 0
	h.zones.Each(func(z zone.Zone) bool {
		if z.Kind == zone.KindPen {
			found++
		}
		return true
	})
	if found != 0 {
		t.Fatal("pen zones are cleared")
	}
	pens := h.World().Pens
	if len(pens) != 2 {
		t.Fatalf("each painted pen became a marker: %+v", pens)
	}
	var fenced, open PenView
	for _, p := range pens {
		if p.X >= 21 && p.X <= 27 {
			fenced = p
		} else {
			open = p
		}
	}
	if !fenced.Enclosed || fenced.X != 24 || fenced.Y != 8 {
		t.Fatalf("the fenced zone's marker sits at its middle, enclosed: %+v", fenced)
	}
	if open.Enclosed || open.X != 6 || open.Y != 16 || !hasMessage(h, "is open") {
		t.Fatalf("an unfenced zone is an open pen with the alert: %+v", open)
	}
}

func TestPenSaveRoundTrip(t *testing.T) {
	g, m := penWorld(t)
	cow := tameAt(t, g, "cow", "Bess", 8, 18)
	s := defaultPenSettings()
	s.AutoCut = []string{"cactus"}
	g.Enqueue(command.Command{Kind: command.SetPenSettings, Target: uint64(m), Name: EncodePenSettings(s)})
	g.Step(1)
	g.pen.since = map[entity.EntityID]uint64{cow: 77}
	h := saveLoad(t, g)
	if got := h.penSettings(m); len(got.AutoCut) != 1 || got.AutoCut[0] != "cactus" {
		t.Fatalf("settings survive a save: %+v", got)
	}
	if h.pen.since[cow] != 77 {
		t.Fatal("the unpenned clock survives a save")
	}
	if !h.penRegion(m).enclosed {
		t.Fatal("the enclosure is rebuilt")
	}
	// a removed marker loses its settings
	h.removeBuilding(m)
	if len(h.penMarkers()) != 0 || len(h.pens) != 0 {
		t.Fatal("settings go with the marker")
	}
}

func TestDoRopeEdgeCases(t *testing.T) {
	g, m := penWorld(t)
	cow := tameAt(t, g, "cow", "Bess", 8, 18)
	ada, _ := g.SpawnNamedPawn(9, 18, "Ada")
	set := func(j job.Record) job.Record {
		g.reserve.ReleaseOwner(ada)
		g.reserve.TryEntity(ada, cow)
		g.jobs.Set(ada, j)
		g.doRope(ada, j)
		r, _ := g.jobs.Get(ada)
		return r
	}
	// the animal walked off before the rope went on
	g.positions.Set(cow, Position{X: 14, Y: 18})
	if r := set(job.Record{Kind: job.Rope, Target: cow, State: job.Do, WorkLeft: 1}); r.State != job.Fail || r.Fail != job.TargetGone {
		t.Fatalf("an animal no longer beside the handler: %+v", r)
	}
	g.positions.Set(cow, Position{X: 8, Y: 18})
	// roped: off to the pen
	var r job.Record
	for i := 0; i < 5 && r.Count == 0; i++ {
		r = set(job.Record{Kind: job.Rope, Target: cow, State: job.Do, WorkLeft: 1})
	}
	if r.Count != 1 || r.Dest != m || r.State != job.Goto || r.TX != 24 || r.TY != 8 {
		t.Fatalf("roped, the handler heads for the marker: %+v", r)
	}
	// at the pen, the cow lagging: the wait runs out
	r = set(job.Record{Kind: job.Rope, Target: cow, State: job.Do, Count: 1, Dest: m, WorkLeft: 1})
	if r.State != job.Fail || r.Fail != job.Blocked || !g.postponedFor(ada, job.Rope, cow, 0, 0) {
		t.Fatalf("a cow that never comes: the job is put off: %+v", r)
	}
	// the pen broke open on the way
	g.removeBuilding(g.layerBldgAt(20, 8, construction.LayerObject))
	r = set(job.Record{Kind: job.Rope, Target: cow, State: job.Do, Count: 1, Dest: m, WorkLeft: 50})
	if r.State != job.Fail || r.Fail != job.TargetGone {
		t.Fatalf("an open pen ends the lead: %+v", r)
	}
	// no pen left to go to once roped: let it be
	r = set(job.Record{Kind: job.Rope, Target: cow, State: job.Do, WorkLeft: 1})
	for i := 0; i < 5 && r.State == job.Do; i++ {
		r = set(r)
	}
	if r.State != job.Complete {
		t.Fatalf("roped with nowhere to go: done: %+v", r)
	}
	// the animal is gone
	g.removeAnimal(cow)
	if r := set(job.Record{Kind: job.Rope, Target: cow, State: job.Do}); r.State != job.Complete {
		t.Fatalf("a vanished animal ends the job: %+v", r)
	}
}
