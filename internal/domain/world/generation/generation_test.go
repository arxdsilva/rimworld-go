package generation

import (
	"testing"

	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

func TestGenerateIsDeterministic(t *testing.T) {
	t.Parallel()
	a, b := Generate(7, 48, 40), Generate(7, 48, 40)
	for i := range a.Terrain.Cells {
		if a.Terrain.Cells[i] != b.Terrain.Cells[i] {
			t.Fatal("terrain differs for the same seed")
		}
	}
	if len(a.Plants) != len(b.Plants) {
		t.Fatal("plants differ for the same seed")
	}
	c := Generate(8, 48, 40)
	same := true
	for i := range a.Terrain.Cells {
		if a.Terrain.Cells[i] != c.Terrain.Cells[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds should give different maps")
	}
}

func TestGenerateHasVarietyAndClearStart(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 5; seed++ {
		g := Generate(seed, 48, 40)
		count := map[terrain.Type]int{}
		for _, c := range g.Terrain.Cells {
			count[c]++
		}
		if count[terrain.Rock] == 0 || count[terrain.Soil] == 0 {
			t.Fatalf("seed %d: no rock or soil: %v", seed, count)
		}
		for y := g.ClearY0; y <= g.ClearY1; y++ {
			for x := g.ClearX0; x <= g.ClearX1; x++ {
				if !g.Terrain.At(x, y).Walkable() {
					t.Fatalf("seed %d: start area blocked at %d,%d", seed, x, y)
				}
			}
		}
		trees := 0
		for _, p := range g.Plants {
			if p.X >= g.ClearX0 && p.X <= g.ClearX1 && p.Y >= g.ClearY0 && p.Y <= g.ClearY1 {
				t.Fatalf("seed %d: plant in the start area", seed)
			}
			if g.Terrain.At(p.X, p.Y).Fertility() == 0 {
				t.Fatalf("seed %d: plant on barren ground", seed)
			}
			if p.Species == plant.Tree {
				trees++
			}
		}
		if trees < 10 {
			t.Fatalf("seed %d: only %d trees", seed, trees)
		}
	}
}
