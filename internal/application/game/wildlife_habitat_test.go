package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
)

const habW, habH = 120, 90

// habitatFixture is a Setup game: landing at (10, 45), a shallow river
// (x 55..58) with a deep channel (x 56) and a ford at y 44..46, a roofed
// rock cave open to land (x 80..89, y 10..19), and a sealed pocket
// (x 100..107, y 60..67) walled by rock. Grass covers the north half of
// the east bank (x 60..119, y < 45).
func habitatFixture(seed uint64) *Game {
	g := New(Config{Seed: seed, Width: habW, Height: habH})
	g.ng.setup = &Setup{Preset: difficulty.Wanderer, Difficulty: difficulty.Of(difficulty.Wanderer), Storyteller: "steady"}
	g.ng.landing, g.ng.hasLanding = [2]int{10, 45}, true
	g.roofs = make([]roof.Cell, habW*habH)
	for y := 0; y < habH; y++ {
		for x := 55; x <= 58; x++ {
			g.terr.Set(x, y, terrain.ShallowWater)
		}
		if y < 44 || y > 46 { // a shallow ford at y 44..46
			g.terr.Set(56, y, terrain.DeepWater)
			g.grid.SetWalkable(56, y, false)
		}
	}
	for y := 10; y < 20; y++ {
		for x := 80; x < 90; x++ {
			g.roofs[y*habW+x] = roof.Natural
		}
	}
	for y := 58; y < 70; y++ {
		for x := 98; x < 110; x++ {
			edge := y == 58 || y == 69 || x == 98 || x == 109
			g.roofs[y*habW+x] = roof.Natural
			if edge {
				g.terr.Set(x, y, terrain.Rock)
				g.grid.SetWalkable(x, y, false)
			}
		}
	}
	s := g.grassSt()
	s.seeded, s.cells = true, make([]uint8, habW*habH)
	for y := 0; y < 45; y++ {
		for x := 60; x < habW; x++ {
			s.cells[y*habW+x] = 1
		}
	}
	return g
}

func herdCells(g *Game) [][2]int {
	var out [][2]int
	for _, id := range g.animalIDs {
		p, _ := g.positions.Get(id)
		out = append(out, [2]int{p.X, p.Y})
	}
	return out
}

func TestHabitatSpawnsAvoidBadCells(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 12; seed++ {
		g := habitatFixture(seed)
		g.spawnWildlife()
		cells := herdCells(g)
		if len(cells) == 0 {
			t.Fatalf("seed %d: no wildlife", seed)
		}
		for _, c := range cells {
			x, y := c[0], c[1]
			switch {
			case terrain.IsWater(g.terr.At(x, y)):
				t.Fatalf("seed %d: animal in water at %v", seed, c)
			case g.roofAt(x, y).Roofed():
				t.Fatalf("seed %d: animal under a roof at %v", seed, c)
			case x >= 98 && x < 110 && y >= 58 && y < 70:
				t.Fatalf("seed %d: animal in the sealed pocket at %v", seed, c)
			}
		}
	}
}

func TestHabitatPointsKeepFromLanding(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 12; seed++ {
		g := habitatFixture(seed)
		h := g.newHabitat()
		for i := 0; i < 40; i++ {
			x, y, ok := h.pick(animal.Deer)
			if !ok {
				continue
			}
			if chebyshev(Position{X: x, Y: y}, Position{X: 10, Y: 45}) < habitatMinDist {
				t.Fatalf("seed %d: point %d,%d within %d of the landing", seed, x, y, habitatMinDist)
			}
			if !h.open(x, y) {
				t.Fatalf("seed %d: point %d,%d not open", seed, x, y)
			}
		}
	}
}

func TestHabitatUnreachableLandAndFog(t *testing.T) {
	t.Parallel()
	g := habitatFixture(5)
	// wall the landing side off from the east bank entirely
	for y := 0; y < habH; y++ {
		g.grid.SetWalkable(54, y, false)
	}
	h := g.newHabitat()
	for y := 0; y < habH; y++ {
		for x := 59; x < habW; x++ {
			if h.open(x, y) {
				t.Fatalf("east bank %d,%d reachable behind a wall", x, y)
			}
		}
	}
	// fogged cells are not open
	g = habitatFixture(5)
	g.fog.hidden = make([]uint64, (habW*habH+63)/64)
	i := 50*habW + 30
	g.fog.hidden[i>>6] |= 1 << (uint(i) & 63)
	if h := g.newHabitat(); h.open(30, 50) || !h.open(31, 50) {
		t.Fatal("fog should hide exactly the fogged cell")
	}
}

func TestHabitatGrazersLeanToGrass(t *testing.T) {
	t.Parallel()
	var picked, uniform, np, nu int
	for seed := uint64(1); seed <= 30; seed++ {
		g := habitatFixture(seed)
		h := g.newHabitat()
		for i := 0; i < 20; i++ {
			if x, y, ok := h.pick(animal.Deer); ok {
				picked += h.forage(x, y)
				np++
			}
			for try := 0; try < 50; try++ {
				x, y := g.rng.IntN(habW), g.rng.IntN(habH)
				if chebyshev(Position{X: x, Y: y}, Position{X: 10, Y: 45}) >= habitatMinDist && h.open(x, y) {
					uniform += h.forage(x, y)
					nu++
					break
				}
			}
		}
	}
	if np == 0 || nu == 0 {
		t.Fatal("no samples")
	}
	if picked*nu <= uniform*np {
		t.Fatalf("grazer points average %d/%d forage, uniform %d/%d", picked, np, uniform, nu)
	}
}

func TestHabitatPredatorsSpreadOut(t *testing.T) {
	t.Parallel()
	g := habitatFixture(7)
	h := g.newHabitat()
	var pts [][2]int
	for i := 0; i < 4; i++ {
		x, y, ok := h.pick(animal.TimberWolf)
		if !ok {
			t.Fatal("no predator point")
		}
		pts = append(pts, [2]int{x, y})
	}
	if len(h.preds) != 4 {
		t.Fatalf("predator spawns not recorded: %d", len(h.preds))
	}
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			if pts[i] == pts[j] {
				t.Fatalf("two predator herds at %v", pts[i])
			}
		}
	}
}

func TestHabitatNoDomesticWild(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 40; seed++ {
		g := habitatFixture(seed)
		for i := 0; i < 60; i++ {
			if s, ok := g.pickSpecies(anySpecies); ok && !isWildSpecies(s) {
				t.Fatalf("seed %d: %v drawn wild", seed, s)
			}
		}
	}
	g := habitatFixture(3)
	g.spawnWildlife()
	for _, id := range g.animalIDs {
		a, _ := g.animals.Get(id)
		if !isWildSpecies(a.Species) {
			t.Fatalf("%v spawned wild", a.Species)
		}
	}
}

func TestHabitatDeterministic(t *testing.T) {
	t.Parallel()
	a, b := habitatFixture(9), habitatFixture(9)
	a.spawnWildlife()
	b.spawnWildlife()
	ca, cb := herdCells(a), herdCells(b)
	if len(ca) != len(cb) {
		t.Fatalf("%d vs %d animals", len(ca), len(cb))
	}
	for i := range ca {
		if ca[i] != cb[i] {
			t.Fatalf("animal %d at %v vs %v", i, ca[i], cb[i])
		}
	}
}

func TestHabitatArrivalUsesHabitat(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 8; seed++ {
		g := habitatFixture(seed)
		n, _ := g.wildArrival(6, 0, 0)
		if n == 0 {
			continue
		}
		for _, c := range herdCells(g) {
			if terrain.IsWater(g.terr.At(c[0], c[1])) || g.roofAt(c[0], c[1]).Roofed() {
				t.Fatalf("seed %d: arrival at bad cell %v", seed, c)
			}
		}
	}
}

func TestHabitatArrivalsEnterAtEdgeReachable(t *testing.T) {
	t.Parallel()
	came := 0
	for seed := uint64(1); seed <= 20; seed++ {
		g := habitatFixture(seed)
		h := g.newHabitat()
		n, _ := g.wildArrival(6, 0, 0)
		came += n
		for _, id := range g.animalIDs {
			p, _ := g.positions.Get(id)
			// herd members sit within 2 of the entry point on the edge
			if min(p.X, p.Y, habW-1-p.X, habH-1-p.Y) > 2 {
				t.Fatalf("seed %d: arrival mid-map at %v", seed, p)
			}
			if !h.reach[p.Y*habW+p.X] {
				t.Fatalf("seed %d: arrival at %v not reachable", seed, p)
			}
		}
		if x, y, ok := h.pickEdge(animal.Deer); ok && min(x, y, habW-1-x, habH-1-y) != 0 {
			t.Fatalf("seed %d: pickEdge %d,%d not on the edge", seed, x, y)
		}
	}
	if came == 0 {
		t.Fatal("no arrivals")
	}
}

func TestHabitatLegacySpawnUnchanged(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 4, Width: 80, Height: 60})
	g.spawnWildlife()
	if g.hasSetup() || len(g.animalIDs) == 0 {
		t.Fatal("legacy fixture")
	}
	// legacy placement must keep drawing from randomTileAway: replay it
	r := New(Config{Seed: 4, Width: 80, Height: 60})
	r.spawnWildlife()
	if a, b := herdCells(g), herdCells(r); len(a) != len(b) || a[0] != b[0] {
		t.Fatal("legacy spawn not deterministic")
	}
}
