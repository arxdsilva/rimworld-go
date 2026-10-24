package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Fences (SIM-028).

// fenceRing builds a fence around the rectangle's edge, with a gate on
// each listed cell.
func fenceRing(t *testing.T, g *Game, x0, y0, x1, y1 int, gates ...[2]int) {
	t.Helper()
	isGate := func(x, y int) bool {
		for _, c := range gates {
			if c[0] == x && c[1] == y {
				return true
			}
		}
		return false
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x != x0 && x != x1 && y != y0 && y != y1 {
				continue
			}
			kind := "fence"
			if isGate(x, y) {
				kind = "fence gate"
			}
			if !g.BuildNow(kind, "wood", "south", x, y) {
				t.Fatalf("%s at %d,%d not built", kind, x, y)
			}
		}
	}
}

func inside(p Position, x0, y0, x1, y1 int) bool {
	return p.X > x0 && p.X < x1 && p.Y > y0 && p.Y < y1
}

func TestWildDeerCannotReachFencedCrop(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	var crops []entity.EntityID
	for y := 8; y <= 11; y++ {
		for x := 14; x <= 17; x++ {
			c, _ := g.spawnPlant(plant.Potato, x, y, plant.DefOf(plant.Potato).GrowTicks)
			p, _ := g.plants.Get(c)
			p.Zone = true
			g.plants.Set(c, p)
			g.notePlantMark(c) // jobindex.go: as doSow does
			crops = append(crops, c)
		}
	}
	fenceRing(t, g, 13, 7, 18, 12)
	deer, _ := g.SpawnAnimal("deer", 10, 10)
	setAnimal(g, deer, func(a *animal.State) { a.Food = 3000 })
	start := animalOf(g, deer).Food
	retries := 0
	lastRetry := animalOf(g, deer).RetryAt
	for i := 0; i < 3000; i++ {
		if i == 1500 {
			// the fenced crop was the only food so far: grass it can
			// reach comes up outside the field, and it goes there
			sowGrass(g, 2, 2, 8, 17)
			sowGrass(g, 20, 2, 28, 17)
		}
		g.Step(1)
		a := animalOf(g, deer)
		if a.Feed.Kind != animal.FeedNone && inside(Position{a.Feed.X, a.Feed.Y}, 13, 7, 18, 12) {
			t.Fatalf("tick %d: the deer picked food behind the fence: %+v", i, a.Feed)
		}
		if p, _ := g.positions.Get(deer); inside(p, 13, 7, 18, 12) || g.fenceAt(p.X, p.Y) != fenceNone {
			t.Fatalf("tick %d: the deer got into the field at %v", i, p)
		}
		if a.RetryAt != lastRetry {
			retries++
			lastRetry = a.RetryAt
		}
	}
	for _, c := range crops {
		if p, ok := g.plants.Get(c); !ok || p.Percent() != 100 {
			t.Fatalf("a fenced crop was grazed: %+v %v", p, ok)
		}
	}
	if a := animalOf(g, deer); a.Food <= start {
		t.Fatalf("the deer fed elsewhere: food %d -> %d", start, a.Food)
	}
	if retries > 3 {
		t.Fatalf("%d failed path searches in 3000 ticks", retries)
	}
}

func TestColonistClimbsFence(t *testing.T) {
	t.Parallel()
	walk := func(fence string) int {
		g := New(Config{Seed: 1, Width: 20, Height: 9})
		for x := 0; x < 20; x++ {
			if !g.BuildNow("wall", "wood", "south", x, 3) || !g.BuildNow("wall", "wood", "south", x, 5) {
				t.Fatal("walls not built")
			}
		}
		if fence != "" && !g.BuildNow(fence, "wood", "south", 9, 4) {
			t.Fatalf("%s not built", fence)
		}
		id := draftedPawn(t, g, 2, 4)
		order(g, id, 16, 4)
		return runUntil(t, g, 2000, func() bool {
			p, _ := g.positions.Get(id)
			return p.X == 16 && p.Y == 4
		})
	}
	open, gate, fence := walk(""), walk("fence gate"), walk("fence")
	if open < 0 || gate < 0 || fence < 0 {
		t.Fatalf("never arrived: open %d gate %d fence %d", open, gate, fence)
	}
	if gate != open {
		t.Fatalf("a gate costs nothing: %d vs %d ticks", gate, open)
	}
	if fence <= open {
		t.Fatalf("climbing the fence is slower: %d vs %d ticks", fence, open)
	}
	g := New(Config{Seed: 1, Width: 8, Height: 8})
	g.BuildNow("fence", "wood", "south", 3, 3)
	g.BuildNow("fence gate", "wood", "south", 4, 3)
	if c := g.weatherSlow(3, 3, 10); c != 10*fenceClimb {
		t.Fatalf("fence step cost %d, want %d", c, 10*fenceClimb)
	}
	if c := g.weatherSlow(4, 3, 10); c != 10 {
		t.Fatalf("gate step cost %d, want 10", c)
	}
	if !g.grid.Walkable(3, 3) || !g.pathQuery(0).Walkable(3, 3) {
		t.Fatal("colonists walk over fences")
	}
}

func TestTameAnimalUsesGateWildDoesNot(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 16})
	bareGround(g)
	gate := [2]int{8, 4}
	fenceRing(t, g, 4, 4, 12, 10, gate)
	pet, _ := g.SpawnAnimal("deer", 8, 7)
	g.TameAnimal(pet, "Clover")
	wild, _ := g.SpawnAnimal("deer", 18, 7)
	through := func(p []pathing.Point, x, y int) bool {
		for _, s := range p {
			if s.X == x && s.Y == y {
				return true
			}
		}
		return false
	}
	a := animalOf(g, pet)
	g.animalGoTo(pet, &a, 8, 1)
	if len(a.Path) == 0 || !through(a.Path, gate[0], gate[1]) {
		t.Fatalf("the tame deer walks out through the gate: %v", a.Path)
	}
	w := animalOf(g, wild)
	g.animalGoTo(wild, &w, 8, 7)
	if len(w.Path) != 0 {
		t.Fatalf("a wild deer has no way in: %v", w.Path)
	}
	// on its feet: the tame deer gets out
	setAnimal(g, pet, func(s *animal.State) { s.Path, s.PathI, s.GoalX, s.GoalY = a.Path, 0, 8, 1 })
	for i := 0; i < 600; i++ {
		setAnimal(g, pet, func(s *animal.State) {
			s.NextThink, s.NextDecide = uint64(g.clock.Now())+1000, uint64(g.clock.Now())+1000
		})
		g.Step(1)
		if p, _ := g.positions.Get(pet); p.Y < 4 {
			break
		}
	}
	if p, _ := g.positions.Get(pet); p.Y >= 4 {
		t.Fatalf("the tame deer never left the pen: %v", p)
	}
	// no gate: the tame deer is fenced in too
	h := New(Config{Seed: 1, Width: 24, Height: 16})
	fenceRing(t, h, 4, 4, 12, 10)
	p2, _ := h.SpawnAnimal("deer", 8, 7)
	h.TameAnimal(p2, "Daisy")
	b := animalOf(h, p2)
	h.animalGoTo(p2, &b, 8, 1)
	if len(b.Path) != 0 {
		t.Fatalf("tame animals do not cross fences: %v", b.Path)
	}
}

func TestPredatorBlockedByFence(t *testing.T) {
	g := New(Config{Seed: 1, Width: 30, Height: 20})
	bareGround(g)
	fenceRing(t, g, 12, 6, 20, 14, [2]int{16, 6})
	// the hare belongs to the pen: an unpenned tame animal may wander out
	// of the gate on its own (PLAY-010 made the seeded walk leave)
	zoneCmd(g, command.DesignatePen, 13, 7, 19, 13, 0)
	hare, _ := g.SpawnAnimal("hare", 16, 10)
	g.TameAnimal(hare, "Bun")
	wolf, _ := g.SpawnAnimal("timber_wolf", 5, 10)
	setAnimal(g, wolf, func(a *animal.State) {
		a.Mode, a.Target, a.Food, a.HuntSince = animal.Attack, hare, 2000, uint64(g.clock.Now())
	})
	// a stale path straight through the fence is refused at the post
	setAnimal(g, wolf, func(a *animal.State) {
		for x := 6; x <= 16; x++ {
			a.Path = append(a.Path, pathing.Point{X: x, Y: 10})
		}
		a.PathI, a.GoalX, a.GoalY = 0, 16, 10
	})
	for i := 0; i < 2000; i++ {
		g.Step(1)
		p, _ := g.positions.Get(wolf)
		if g.fenceAt(p.X, p.Y) != fenceNone || inside(p, 12, 6, 20, 14) {
			t.Fatalf("tick %d: the wolf crossed the fence at %v", i, p)
		}
		setAnimal(g, hare, func(a *animal.State) { a.Food = animal.Full })
	}
	if h := animalOf(g, hare); h.Dead() || h.HP() < animal.DefOf(h.Species).MaxHP {
		t.Fatalf("the hare behind the fence was hurt: %+v", h)
	}
}

func TestFenceViewInWorld(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	// a gate at the end of an east-west fence run follows the run (PLAY-011);
	// a free-standing gate keeps the player's facing
	if !g.BuildNow("fence", "wood", "south", 2, 2) || !g.BuildNow("fence gate", "wood", "east", 3, 2) ||
		!g.BuildNow("fence gate", "wood", "east", 7, 5) {
		t.Fatal("not built")
	}
	got := map[string]BuildingView{}
	var free BuildingView
	for _, b := range g.World().Buildings {
		if b.X == 7 {
			free = b
			continue
		}
		got[b.Kind] = b
	}
	if f, ok := got["fence"]; !ok || f.X != 2 || f.Facing != "south" || f.Floor {
		t.Fatalf("fence view %+v %v", f, ok)
	}
	if gv, ok := got["fence gate"]; !ok || gv.X != 3 || gv.Facing != "south" {
		t.Fatalf("gate view %+v %v", gv, ok)
	}
	if free.Kind != "fence gate" || free.Facing != "east" {
		t.Fatalf("free-standing gate view %+v", free)
	}
	var stone bool
	for _, b := range Buildables() {
		if b.Kind == "fence" {
			stone = b.StoneOK && !b.Blocks
		}
	}
	if !stone {
		t.Fatal("a fence is walkable and may be stone")
	}
}

func TestFenceSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 16})
	fenceRing(t, g, 4, 4, 12, 10, [2]int{8, 4})
	var buf bytes.Buffer
	if err := g.Save(&buf, "fences"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if h.fenceAt(4, 4) != fencePost || h.fenceAt(8, 4) != fenceGate || h.fenceAt(8, 7) != fenceNone {
		t.Fatalf("fence cells after load: %d %d %d", h.fenceAt(4, 4), h.fenceAt(8, 4), h.fenceAt(8, 7))
	}
	wild, _ := h.SpawnAnimal("deer", 18, 7)
	w := animalOf(h, wild)
	h.animalGoTo(wild, &w, 8, 7)
	if len(w.Path) != 0 {
		t.Fatalf("a wild deer has no way in after a load: %v", w.Path)
	}
	// taking a fence post down opens the ring
	for _, id := range append([]entity.EntityID(nil), h.bldgIDs...) {
		if b, _ := h.bldgs.Get(id); b.X == 12 && b.Y == 7 {
			h.removeBuilding(id)
		}
	}
	w = animalOf(h, wild)
	h.animalGoTo(wild, &w, 8, 7)
	if len(w.Path) == 0 {
		t.Fatal("a gap in the fence lets it in")
	}
}
