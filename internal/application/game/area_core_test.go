package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/fire"
)

// PLAY-015 allowed areas: soft traversal, destination filters, overrides,
// Home, animals, commands and save migration.

func assign(g *Game, id entity.EntityID, ref int) {
	g.Enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(id), Index: ref})
	g.ApplyPending()
}

// paintArea paints area id ref, creating areas up to it first.
func paintArea(g *Game, ref, x0, y0, x1, y1 int) {
	for g.areaIndexOf(ref) < 0 {
		g.Enqueue(command.Command{Kind: command.CreateArea})
		g.ApplyPending()
	}
	g.Enqueue(command.Command{Kind: command.PaintArea, Index: ref, X: x0, Y: y0, X2: x1, Y2: y1, Enabled: true})
	g.ApplyPending()
}

// Two islands of one area joined only across open ground outside it: a
// restricted colonist still crosses to work on the far island.
func TestAreaSoftCrossesBetweenIslands(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 30, Height: 12})
	id, _ := g.SpawnNamedPawn(2, 5, "Ada")
	paintArea(g, 1, 0, 0, 4, 11)
	paintArea(g, 1, 20, 0, 24, 11)
	assign(g, id, 1)
	if !g.reachable(id, 22, 5) {
		t.Fatal("the far island must be reachable")
	}
	path, ok := g.pathTo(id, 22, 5)
	if !ok || len(path) > 22 {
		t.Fatalf("crossing: ok %v, %d steps (want the straight way over)", ok, len(path))
	}
	if _, err := g.SpawnPlant(22, 5); err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 22, Y: 5, X2: 22, Y2: 5})
	reached := false
	for i := 0; i < 1500 && !reached; i++ {
		g.Step(1)
		reached = posOf(g, id).X >= 20
	}
	if !reached {
		t.Fatalf("the colonist never crossed to the far island: %v", posOf(g, id))
	}
}

// softCost prices cells outside the area 300% of a step extra, nothing
// inside, nothing for a drafted pawn; fire adds its own cost on top.
func TestAreaSoftCost(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	s := g.softCostFor(id)
	if !s.on() || s.extra(3, 3) != 0 || s.extra(15, 3) != areaOutsidePct*pathing.StraightCost/100 {
		t.Fatalf("restricted costs: in %d, out %d", s.extra(3, 3), s.extra(15, 3))
	}
	if q, ok := g.pathQuery(id).(softQuery); !ok || q.ExtraCost(15, 3) != 30 || q.BlockedBits() == nil {
		t.Fatal("pathQuery must carry the soft costs and keep the grid's bitset")
	}
	if softExtra(g.costedQuery(id).(crowdCost).Query, 15, 3) != 30 {
		t.Fatal("replans must carry the soft costs")
	}
	g.setDraft(id, true)
	if g.softCostFor(id).on() {
		t.Fatal("a drafted pawn keeps to no area")
	}
	if _, plain := g.pathQuery(id).(softQuery); plain {
		t.Fatal("no costs: the plain grid, so legacy routes do not move")
	}
	g.setDraft(id, false)
	g.fire.fires = []fire.Fire{{X: 15, Y: 3, Size: 50}}
	s = g.softCostFor(id)
	want := (areaOutsidePct + g.fireAvoidCost(15, 3)) * pathing.StraightCost / 100
	if !s.fire || s.extra(15, 3) != want || s.extra(16, 3) != g.fireAvoidCost(16, 3)*pathing.StraightCost/100+30 {
		t.Fatalf("fire cost: %d, want %d", s.extra(15, 3), want)
	}
	// a forced job paths free of the area
	g.jobs.Set(id, job.Record{Kind: job.Haul, Forced: true, State: job.Goto})
	if g.softCostFor(id).area {
		t.Fatal("a forced job pays no area cost")
	}
	if !plainSoft(g.pathQuery(id)).(softQuery).s.fire {
		t.Fatal("the reference search keeps the costs")
	}
	if _, ok := plainSoft(g.grid).(plainQuery); !ok {
		t.Fatal("a plain query stays plain")
	}
}

func woodIn(g *Game, x0, y0, x1, y1 int) int {
	n := 0
	for _, sid := range g.itemIDs {
		st, ok := g.stacks.Get(sid)
		if ok && st.Kind == item.Wood && st.Carrier == 0 && st.X >= x0 && st.X <= x1 && st.Y >= y0 && st.Y <= y1 {
			n += st.Qty
		}
	}
	return n
}

// A restricted hauler carries to the stockpile inside its area, even
// though one outside is nearer; with only one outside it hauls nothing.
func TestAreaHaulDestination(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 12, Y: 3, X2: 13, Y2: 4})
	g.ApplyPending()
	if _, err := g.SpawnStack(item.Wood, 10, 8, 3); err != nil {
		t.Fatal(err)
	}
	g.Step(1500)
	if n := woodIn(g, 12, 3, 13, 4); n != 0 {
		t.Fatalf("%d wood hauled to the stockpile outside the area", n)
	}
	if woodIn(g, 8, 3, 8, 3) != 10 {
		t.Fatal("the wood should stay put with no stockpile inside")
	}
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 0, Y: 15, X2: 1, Y2: 16})
	g.ApplyPending()
	for i := 0; i < 6 && woodIn(g, 0, 15, 1, 16) != 10; i++ {
		g.Step(500)
	}
	if n := woodIn(g, 0, 15, 1, 16); n != 10 {
		t.Fatalf("%d wood in the inside stockpile, want 10", n)
	}
	// and the job filter itself: a haul whose destination lies outside
	rec := job.Record{Kind: job.Haul, TX: 3, TY: 3, DestX: 12, DestY: 3}
	if !g.areaRejects(id, rec) {
		t.Fatal("a haul to a cell outside the area must be refused")
	}
	rec.DestX = 5
	rec.SpareX, rec.SpareY, rec.HasSpare = 20, 5, true
	if !g.areaRejects(id, rec) {
		t.Fatal("a spare cell outside the area must be refused")
	}
	// a delivery route with a stop outside
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 15, 8)
	fid := g.layerFrameAt(15, 8, construction.LayerObject)
	rec.HasSpare, rec.Route = false, []entity.EntityID{slotKey(fid, item.Wood)}
	if fid == 0 || !g.areaRejects(id, rec) {
		t.Fatal("a route stop outside the area must be refused")
	}
	rec.Route = nil
	if g.areaRejects(id, rec) {
		t.Fatal("a haul inside the area is fine")
	}
}

// Beds outside the area are never chosen; with none inside, a colonist
// outside its area lies down on the floor inside it.
func TestAreaBedChoiceAndFloorFallback(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	g.positions.Set(id, Position{8, 3})
	out := buildNow(t, g, construction.Bed, item.Wood, construction.South, 12, 3)
	g.rest.Assign(id)
	if j, _ := g.jobs.Get(id); j.Kind != job.Sleep || j.Target == out || !g.areas.Contains(0, j.TX, j.TY) {
		t.Fatalf("sleep job %+v: the bed outside was chosen or the spot is outside", j)
	}
	in := buildNow(t, g, construction.Bed, item.Wood, construction.South, 3, 12)
	g.jobs.Set(id, job.Record{})
	g.rest.Assign(id)
	if j, _ := g.jobs.Get(id); j.Target != in {
		t.Fatalf("sleep job %+v: want the bed inside (%d)", j, in)
	}
	// standing outside with no bed it may use: the floor inside
	g.bldgs.Set(in, construction.Building{})
	g.positions.Set(id, Position{20, 5})
	g.jobs.Set(id, job.Record{})
	g.rest.InPlace(id)
	if j, _ := g.jobs.Get(id); j.Kind != job.Sleep || j.TX != 9 || j.TY != 5 {
		t.Fatalf("floor sleep %+v: want the nearest cell inside (9, 5)", j)
	}
	if x, y := (restBodies{g}).FloorSpot(id, 4, 4); x != 4 || y != 4 {
		t.Fatal("inside its area it sleeps where it stands")
	}
	// a hospital bed is medical rest: exempt wherever it stands
	hb := buildNow(t, g, construction.HospitalBed, item.Wood, construction.South, 20, 10)
	if g.areaRejects(id, job.Record{Kind: job.Sleep, Target: hb, TX: 20, TY: 10}) {
		t.Fatal("rest in a hospital bed is medical care")
	}
	if !g.areaRejects(id, job.Record{Kind: job.Sleep, Target: out, TX: 12, TY: 3}) {
		t.Fatal("a plain bed outside must be refused")
	}
}

// Food outside the area is not eaten (the meal search cools down, no
// flapping); food inside is, even when food outside lies nearer.
func TestAreaFoodFilter(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	g.positions.Set(id, Position{8, 5})
	out, _ := g.SpawnStack(item.Berries, 5, 12, 5)
	g.assignEat(id)
	if j, _ := g.jobs.Get(id); j.Kind == job.Eat {
		t.Fatalf("ate food outside the area: %+v", j)
	}
	if g.noFood[id] <= uint64(g.clock.Now()) {
		t.Fatal("no food inside: the meal search must cool down")
	}
	in, _ := g.SpawnStack(item.Berries, 5, 2, 5)
	delete(g.noFood, id)
	g.assignEat(id)
	if j, _ := g.jobs.Get(id); j.Kind != job.Eat || j.Target != in || j.Target == out {
		t.Fatalf("want the food inside: %+v", j)
	}
}

// Hungry with food only outside: the colonist does not flap between
// trying to eat and other jobs.
func TestAreaFoodRejectionDoesNotFlap(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	_, _ = g.SpawnStack(item.Berries, 5, 15, 5)
	n, _ := g.need.Get(id)
	n.Hunger = 1000
	g.need.Set(id, n)
	if c := kindChanges(g, id, 2000); c > 16 {
		t.Fatalf("job kind changed %d times in 2000 ticks", c)
	}
	if p := posOf(g, id); p.X > 9 {
		t.Fatalf("went for the food outside: %v", p)
	}
}

// Wander and recreation spots stay inside the area.
func TestAreaRecreationFilters(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	g.positions.Set(id, Position{8, 5})
	buildNow(t, g, construction.Chair, item.Wood, construction.South, 12, 5)
	if g.idleRelax(id, 300) {
		t.Fatal("sat on a chair outside the area")
	}
	buildNow(t, g, construction.Chair, item.Wood, construction.South, 6, 5)
	if !g.idleRelax(id, 300) {
		t.Fatal("the chair inside should do")
	}
	g.jobs.Set(id, job.Record{})
	for i := 0; i < 20; i++ {
		if g.idleWander(id, 300) {
			if j, _ := g.jobs.Get(id); j.TX > 9 {
				t.Fatalf("wandered to %d,%d outside", j.TX, j.TY)
			}
		}
		g.jobs.Set(id, job.Record{})
		g.reserve.ReleaseOwner(id)
	}
}

// A hunt is given up once the prey has stayed outside the area for
// huntOutTicks; forced hunts never are.
func TestAreaHuntAbandon(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	prey, err := g.SpawnAnimal("deer", 20, 5)
	if err != nil {
		t.Fatal(err)
	}
	j := job.Record{Kind: job.Hunt, Target: prey, State: job.Goto}
	if g.huntGivenUp(id, j, Position{20, 5}, 100) || g.huntGivenUp(id, j, Position{20, 5}, 699) {
		t.Fatal("given up too soon")
	}
	if g.huntGivenUp(id, j, Position{5, 5}, 650) || g.huntGivenUp(id, j, Position{20, 5}, 700) {
		t.Fatal("back inside restarts the clock")
	}
	other := job.Record{Kind: job.Hunt, Target: prey + 1000}
	if g.huntGivenUp(id, other, Position{20, 5}, 1400) {
		t.Fatal("another prey restarts the clock")
	}
	if !g.huntGivenUp(id, other, Position{20, 5}, 2000) {
		t.Fatal("the hunt should be given up after huntOutTicks outside")
	}
	if forced := (job.Record{Kind: job.Hunt, Target: prey, Forced: true}); g.huntGivenUp(id, forced, Position{20, 5}, 9000) {
		t.Fatal("a forced hunt never gives up")
	}
	// through doCombat: the job fails and the prey is put off
	a := g.animals.Ref(prey)
	a.Designated = true
	g.equip[id] = item.Rifle
	g.worldS().huntOut[id] = huntOutRec{Pawn: id, Prey: prey, Since: 0}
	g.jobs.Set(id, job.Record{Kind: job.Hunt, Target: prey, TX: 20, TY: 5, State: job.Goto})
	g.combat.Strikes.Do(id, job.Record{Kind: job.Hunt, Target: prey, TX: 20, TY: 5, State: job.Goto}, 5000)
	if r, _ := g.jobs.Get(id); r.State != job.Fail || !g.postponedFor(id, job.Hunt, prey, 0, 0) {
		t.Fatalf("hunt not abandoned: %+v", r)
	}
	if _, ok := g.worldSt.huntOut[id]; ok {
		t.Fatal("the clock must be cleared")
	}
	// saved
	g.worldS().huntOut[id] = huntOutRec{Pawn: id, Prey: prey, Since: 42}
	if m := importHuntOut(exportHuntOut(g.worldSt.huntOut)); m[id].Since != 42 || m[id].Prey != prey {
		t.Fatal("hunt clock lost in the save")
	}
}

// The overrides: who and what an area does not bind.
func TestAreaOverridesMatrix(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	out := Position{20, 5}
	if g.inArea(id, out.X, out.Y) {
		t.Fatal("setup: a restricted colonist is barred outside")
	}
	for _, c := range []struct {
		name string
		set  func()
		undo func()
	}{
		{"drafted", func() { g.setDraft(id, true) }, func() { g.setDraft(id, false) }},
		{"mental break", func() { g.breaks[id] = breakState{} }, func() { delete(g.breaks, id) }},
		{"downed", func() {
			v, _ := g.vital.Get(id)
			v.Status = health.Downed
			g.vital.Set(id, v)
		}, func() {
			v, _ := g.vital.Get(id)
			v.Status = health.Alive
			g.vital.Set(id, v)
		}},
	} {
		c.set()
		if !g.inArea(id, out.X, out.Y) || g.restrictedRef(id) != 0 {
			t.Errorf("%s: still bound by the area", c.name)
		}
		c.undo()
	}
	for _, k := range []job.Kind{job.Move, job.Attack, job.Rescue, job.Tend, job.Surgery, job.Warm, job.Hide} {
		if g.areaRejects(id, job.Record{Kind: k, TX: out.X, TY: out.Y}) {
			t.Errorf("%s: an exempt job was refused", k)
		}
	}
	for _, k := range []job.Kind{job.Harvest, job.Sleep, job.Eat, job.Wander, job.Relax, job.Hunt} {
		if !g.areaRejects(id, job.Record{Kind: k, TX: out.X, TY: out.Y}) {
			t.Errorf("%s: a job outside was let through", k)
		}
	}
	if g.areaRejects(id, job.Record{Kind: job.Harvest, TX: out.X, TY: out.Y, Forced: true}) {
		t.Error("a forced order was refused")
	}
	// work from beside: a wall cell just outside, with a neighbour inside,
	// may be worked; one further out may not
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 10, 5)
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 12, 5)
	if g.areaRejects(id, job.Record{Kind: job.Mine, TX: 9, TY: 5, DestX: 10, DestY: 5}) {
		t.Error("a wall beside the area is worked from inside")
	}
	if !g.areaRejects(id, job.Record{Kind: job.Mine, TX: 11, TY: 5, DestX: 12, DestY: 5}) {
		t.Error("a wall two cells out must be refused")
	}
	if !g.areaRejects(id, job.Record{Kind: job.Clean, TX: 9, TY: 5, DestX: 12, DestY: 5}) {
		t.Error("cleaning a cell outside from inside must be refused")
	}
	if !g.areaRejects(id, job.Record{Kind: job.Cut, TX: 9, TY: 5, DestX: 12, DestY: 5}) {
		t.Error("cutting grass outside must be refused")
	}
	if !g.areaRejects(id, job.Record{Kind: job.Bill, TX: 3, TY: 3, DestX: 12, DestY: 5}) {
		t.Error("a bench outside must be refused")
	}
	if g.areaBypass(id, job.Record{Kind: job.Harvest}) {
		t.Error("a plain job does not bypass")
	}
	if !g.areaBypass(id, job.Record{Kind: job.Rescue}) || !g.areaPathFree(id) {
		t.Error("an exempt job paths free")
	}
	g.endAreaBypass()
}

// Home is an area: the live Home mask binds a colonist assigned to it.
func TestAreaHomeRef(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 30, Height: 20})
	id, _ := g.SpawnNamedPawn(3, 3, "Ada")
	g.homeArea.cells = nil
	assign(g, id, command.AreaHome)
	if g.AreaOf(id) != command.AreaHome || g.inArea(id, 3, 3) {
		t.Fatalf("Home ref: AreaOf %d; with no Home every cell is outside", g.AreaOf(id))
	}
	if s := g.softCostFor(id); !s.area || s.extra(3, 3) != 30 {
		t.Fatal("no Home: every cell costs extra")
	}
	g.paintHome(0, 0, 9, 19, true)
	if !g.inArea(id, 3, 3) || g.inArea(id, 15, 3) || !g.inArea(id, 9, 19) {
		t.Fatal("Home mask not followed")
	}
	if s := g.softCostFor(id); s.extra(3, 3) != 0 || s.extra(15, 3) != 30 {
		t.Fatal("Home soft costs")
	}
	g.paintHome(0, 0, 9, 19, false)
	g.paintHome(20, 0, 25, 5, true)
	g.positions.Set(id, Position{3, 3})
	g.areasTick(uint64(g.clock.Now()))
	if j, _ := g.jobs.Get(id); j.Kind != job.Move || j.TX != 20 {
		t.Fatalf("outside Home, it walks back in: %+v", j)
	}
	assign(g, id, command.AreaUnrestricted)
	if g.AreaOf(id) != command.AreaUnrestricted || !g.inArea(id, 3, 3) {
		t.Fatal("unassign")
	}
}

// Create, invert and delete by id; deleting resets every pawn using the
// area, with one message.
func TestAreaCreateInvertDelete(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	b, _ := g.SpawnNamedPawn(4, 4, "Bo")
	assign(g, b, 1)
	g.Enqueue(command.Command{Kind: command.CreateArea, Name: "  Barn  "})
	g.Enqueue(command.Command{Kind: command.CreateArea})
	g.ApplyPending()
	v := g.Areas()
	if len(v) != 3 || v[1].Name != "Barn" || v[1].ID != 2 || v[2].Name != "Area 3" || v[2].Cells != 0 {
		t.Fatalf("created %+v", v)
	}
	before := v[0]
	g.Enqueue(command.Command{Kind: command.InvertArea, Index: 1})
	g.ApplyPending()
	after := g.Areas()[0]
	if after.Cells != 600-200 || after.Ver == before.Ver || before.Cells != 200 {
		t.Fatalf("invert: %d cells, ver %d -> %d", after.Cells, before.Ver, after.Ver)
	}
	if &before.Bits[0] == &after.Bits[0] || before.Bits[0]&1 == 0 {
		t.Fatal("copy-on-write: the old view's bits must stay as they were")
	}
	g.Enqueue(command.Command{Kind: command.DeleteArea, Index: 1})
	g.Enqueue(command.Command{Kind: command.DeleteArea, Index: 77}) // none: no-op
	g.ApplyPending()
	if g.AreaOf(id) != command.AreaUnrestricted || g.AreaOf(b) != command.AreaUnrestricted {
		t.Fatal("pawns on a deleted area must be unrestricted")
	}
	if v := g.Areas(); len(v) != 2 || v[0].ID != 2 || v[0].Index != 0 {
		t.Fatalf("after delete %+v", v)
	}
	n := 0
	for _, m := range g.messages {
		if strings.Contains(m.Text, "pawns are now unrestricted") {
			n++
			if m.Text != "2 pawns are now unrestricted" {
				t.Fatalf("message %q", m.Text)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d messages, want one", n)
	}
	// assigning a missing area is ignored; a raider cannot be assigned
	assign(g, id, 1)
	if g.AreaOf(id) != command.AreaUnrestricted {
		t.Fatal("assigned to a deleted area")
	}
	g.Enqueue(command.Command{Kind: command.RenameArea, Index: 2, Name: "Yard"})
	g.ApplyPending()
	if g.Areas()[0].Name != "Yard" {
		t.Fatal("rename")
	}
	assign(g, b, 2)
	g.Enqueue(command.Command{Kind: command.DeleteArea, Index: 2})
	g.ApplyPending()
	if last := g.messages[len(g.messages)-1].Text; last != "1 pawn is now unrestricted" {
		t.Fatalf("message %q", last)
	}
}

// Tame non-penned animals keep to their areas; penned livestock and wild
// animals cannot be assigned; an animal following its drafted master is
// free.
func TestAreaAnimals(t *testing.T) {
	t.Parallel()
	g, col := areaGame(t)
	dog, _ := g.SpawnAnimal("husky", 20, 5)
	cow, _ := g.SpawnAnimal("cow", 22, 5)
	wild, _ := g.SpawnAnimal("husky", 24, 5)
	for _, aid := range []entity.EntityID{dog, cow} {
		a := g.animals.Ref(aid)
		a.Tame = true
	}
	for _, aid := range []entity.EntityID{dog, cow, wild} {
		assign(g, aid, 1)
	}
	if g.AreaOf(dog) != 1 || g.AreaOf(cow) != command.AreaUnrestricted || g.AreaOf(wild) != command.AreaUnrestricted {
		t.Fatalf("assignments: dog %d cow %d wild %d", g.AreaOf(dog), g.AreaOf(cow), g.AreaOf(wild))
	}
	if g.inArea(dog, 20, 5) || !g.inArea(dog, 5, 5) || !g.inArea(cow, 20, 5) {
		t.Fatal("inArea for animals")
	}
	if s := g.softCostFor(dog); !s.area {
		t.Fatal("a restricted animal's paths pay the area cost")
	}
	if _, ok := g.softAnimalQuery(dog, g.grid).(softQuery); !ok {
		t.Fatal("animal query carries the cost")
	}
	if _, ok := g.softAnimalQuery(col, g.grid).(softQuery); ok {
		t.Fatal("a colonist's soft costs come from pathQuery, not twice")
	}
	// idle outside: walked back in
	a := g.animals.Ref(dog)
	a.Path, a.PathI, a.RetryAt = nil, 0, 0
	g.areasTick(uint64(g.clock.Now()))
	if a = g.animals.Ref(dog); a.PathI >= len(a.Path) || a.GoalX > 9 {
		t.Fatalf("dog not walked back: goal %d,%d path %d", a.GoalX, a.GoalY, len(a.Path))
	}
	views := g.animalViews()
	for _, v := range views {
		if v.ID == dog && v.AreaRef != 1 || v.ID == cow && v.AreaRef != command.AreaUnrestricted {
			t.Fatalf("view %d AreaRef %d", v.ID, v.AreaRef)
		}
	}
	// following a drafted master
	a.Master = uint64(col)
	g.setDraft(col, true)
	if !g.inArea(dog, 20, 5) {
		t.Fatal("an animal following its drafted master goes anywhere")
	}
	g.setDraft(col, false)
	// a new assignment drops a walk ending outside
	a.Path, a.PathI, a.GoalX, a.GoalY = []pathing.Point{{X: 19, Y: 5}}, 0, 25, 5
	assign(g, dog, 1)
	if a = g.animals.Ref(dog); a.PathI < len(a.Path) {
		t.Fatal("a walk to outside the new area must stop")
	}
}

// Old saves stored area positions: index i becomes id i+1; new saves
// round-trip refs (Home included).
func TestAreaOldSaveMigration(t *testing.T) {
	t.Parallel()
	g, id := areaGame(t)
	b, _ := g.SpawnNamedPawn(4, 4, "Bo")
	g.importWorld(&worldSave{Version: 1, AreaOf: []areaAssign{{Pawn: id, Area: 0}}})
	if g.areaRef(id) != 1 {
		t.Fatalf("old index 0 -> ref %d, want id 1", g.areaRef(id))
	}
	assign(g, b, command.AreaHome)
	s := g.exportWorld()
	if s.Version != worldSaveVersion {
		t.Fatalf("version %d", s.Version)
	}
	g.importWorld(s)
	if g.areaRef(id) != 1 || g.AreaOf(b) != command.AreaHome {
		t.Fatal("refs lost in the round trip")
	}
	g.Step(5)
	h := saveLoad(t, g)
	if h.AreaOf(b) != command.AreaHome || h.areaRef(id) != 1 || h.StateHash() != g.StateHash() {
		t.Fatal("full save round trip")
	}
	if importAreaRef(1, -1) != -1 || importAreaRef(2, 3) != 3 {
		t.Fatal("importAreaRef")
	}
}
