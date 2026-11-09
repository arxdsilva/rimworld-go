package game

import (
	"math/rand/v2"
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// groundExtraDirect is the path cost of a cell computed from the terrain,
// the plant and the tread, with no cache: the reference for groundExtra.
func groundExtraDirect(g *Game, x, y int) int {
	if !g.grid.InBounds(x, y) {
		return 0
	}
	extra := 0
	if c := g.terr.At(x, y).WalkCost(); c > 100 {
		extra += (c - 100) / 10
	}
	if c := g.plantWalkCost(x, y); c > 100 {
		extra += (c - 100) / 10
	}
	if extra == 0 || g.treadAt(y*g.grid.Width()+x)&treadFloor != 0 {
		return 0
	}
	return extra
}

// TestGroundExtraCacheMatchesDirect: through random plant growth and
// cutting, building, terrain and water changes (and reads between them,
// so the cache is warm), every cell's cached cost equals the direct one.
func TestGroundExtraCacheMatchesDirect(t *testing.T) {
	t.Parallel()
	const w, h = 24, 18
	g := New(Config{Seed: 5, Width: w, Height: h})
	rng := rand.New(rand.NewPCG(11, 13))
	var plants []entity.EntityID
	species := []plant.Species{plant.Bramble, plant.ElephantEar, plant.Moss, plant.BerryBush}
	ground := []terrain.Type{terrain.Soil, terrain.Marsh, terrain.Mud, terrain.ShallowWater, terrain.Stone}
	seen := 0
	all := func(step int, what string) {
		t.Helper()
		for y := -1; y <= h; y++ {
			for x := -1; x <= w; x++ {
				got, want := g.groundExtra(x, y), groundExtraDirect(g, x, y)
				if got != want {
					t.Fatalf("step %d (%s): groundExtra(%d,%d) = %d, direct %d", step, what, x, y, got, want)
				}
				if got != 0 {
					seen++
				}
			}
		}
	}
	for step := range 250 {
		what := ""
		x, y := rng.IntN(w), rng.IntN(h)
		switch rng.IntN(6) {
		case 0, 1:
			what = "plant"
			if id, err := g.spawnPlant(species[rng.IntN(len(species))], x, y, 100); err == nil {
				plants = append(plants, id)
			}
		case 2:
			what = "cut"
			if len(plants) > 0 {
				i := rng.IntN(len(plants))
				g.removePlant(plants[i])
				plants = append(plants[:i], plants[i+1:]...)
			}
		case 3:
			what = "terrain"
			g.setTerrain(x, y, ground[rng.IntN(len(ground))])
		case 4:
			what = "build"
			if ok, _ := g.canPlace(construction.Floor, x, y); ok {
				g.placeFrameStuff(construction.Floor, construction.DefaultStuff(construction.Floor), construction.South, x, y)
				if fid := g.layerFrameAt(x, y, construction.LayerFloor); fid != 0 {
					fr, _ := g.frames.Get(fid)
					g.completeFrame(fid, fr)
				}
			}
		default:
			what = "tick"
			g.Step(1 + rng.IntN(40)) // plants grow, rooms rebuild
		}
		if step%2 == 0 {
			all(step, what)
		}
	}
	if seen < 200 {
		t.Fatalf("only %d costly cells seen: the walk proves little", seen)
	}
}
