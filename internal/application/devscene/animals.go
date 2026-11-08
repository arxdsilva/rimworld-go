package devscene

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// animalPen builds the -scenario animals set piece east of the start: a pen
// with a husky, a cow, a horse and a goat, and a wild deer marked for taming.
func animalPen(g *game.Game, cx, cy int) {
	g.ClearPlants(cx+4, cy-6, cx+12, cy+2)
	markerPen(g, cx+4, cy-6, cx+12, cy+2, true)
	for _, t := range []struct {
		species, name string
		x, y          int
	}{{"husky", "Ash", cx + 7, cy - 3}, {"cow", "Clover", cx + 9, cy - 1}, {"horse", "Maple", cx + 6, cy - 1}, {"goat", "Pip", cx + 10, cy - 4}} {
		if id, err := g.SpawnAnimal(t.species, t.x, t.y); err == nil {
			g.TameAnimal(id, t.name)
		}
	}
	if _, err := g.SpawnAnimal("deer", cx-6, cy-5); err == nil {
		g.Enqueue(command.Command{Kind: command.DesignateTame, X: cx - 6, Y: cy - 5, X2: cx - 6, Y2: cy - 5})
	}
}

// zoo pens every animal species, tamed and named after its species, in
// rows of 12 five tiles apart from (4, 3), room for an XL body about 3
// tiles long (PLAY-022).
func zoo(g *game.Game) {
	const cols, step, x0, y0 = 12, 5, 4, 3
	species := animal.All()
	rows := (len(species) + cols - 1) / cols
	x1, y1 := x0+cols*step, y0+rows*step
	g.ClearPlants(x0-1, y0-1, x1, y1)
	markerPen(g, x0-2, y0-2, x1+1, y1+1, false)
	for i, s := range species {
		x, y := x0+(i%cols)*step, y0+(i/cols)*step
		for _, d := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
			if id, err := g.SpawnAnimal(s.String(), x+d[0], y+d[1]); err == nil { // rock: the next tile
				g.TameAnimal(id, s.Label())
				break
			}
		}
	}
}

// penScene builds the -scenario pen set piece east of the start: a
// fenced pen with a feeding trough and four tame
// animals, a marked wild deer, and a few wild animals about.
func penScene(g *game.Game, cx, cy int) {
	g.ClearPlants(cx+3, cy-8, cx+14, cy+3)
	markerPen(g, cx+3, cy-7, cx+12, cy+2, true)
	g.BuildNow("feeding trough", "wood", "south", cx+7, cy-3)
	for _, t := range []struct {
		species, name string
		x, y          int
	}{{"goat", "Pip", cx + 5, cy - 4}, {"cow", "Clover", cx + 9, cy - 1}, {"chicken", "Hen", cx + 6, cy}, {"horse", "Maple", cx + 10, cy - 5}} {
		if id, err := g.SpawnAnimal(t.species, t.x, t.y); err == nil {
			g.TameAnimal(id, t.name)
		}
	}
	for i, sp := range []string{"deer", "deer", "deer", "boar", "timber_wolf", "hare", "hare"} {
		if _, err := g.SpawnAnimal(sp, cx-9+i*2, cy-9+i%3*2); err != nil {
			continue
		}
	}
	g.Step(1)
}

// markerPen fences the rectangle's border (with a gate in the middle of
// its south side when gate) and puts a pen marker in its north-west
// corner: a pen is a marker in a fenced field (PLAY-015).
func markerPen(g *game.Game, x0, y0, x1, y1 int, gate bool) {
	g.ClearPlants(x0, y0, x1, y1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x != x0 && x != x1 && y != y0 && y != y1 {
				continue
			}
			kind := "fence"
			if gate && y == y1 && x == (x0+x1)/2 {
				kind = "fence gate"
			}
			g.BuildNow(kind, "wood", "south", x, y)
		}
	}
	g.BuildNow("pen marker", "wood", "south", x0+1, y0+1)
}

// fenceField builds a wooden fence rectangle with a gate in its south
// side east of the start, and spawns deer outside it.
func fenceField(g *game.Game, cx, cy int) {
	x0, y0, x1, y1 := cx+8, cy-6, cx+16, cy-1
	g.ClearPlants(x0-3, y0-3, x1+3, y1+4)
	for x := x0; x <= x1; x++ {
		g.BuildNow("fence", "wood", "south", x, y0)
		if x == x0+4 {
			g.BuildNow("fence gate", "wood", "south", x, y1)
			continue
		}
		g.BuildNow("fence", "wood", "south", x, y1)
	}
	for y := y0 + 1; y < y1; y++ {
		g.BuildNow("fence", "wood", "south", x0, y)
		g.BuildNow("fence", "wood", "south", x1, y)
	}
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: x0 + 1, Y: y0 + 1, X2: x1 - 1, Y2: y1 - 1})
	for i, p := range [][2]int{{x1 + 2, y0 + 1}, {x1 + 3, y1 - 1}, {x0 + 3, y1 + 2}} {
		_ = i
		_, _ = g.SpawnAnimal("deer", p[0], p[1])
	}
}

// forbidScene drops a few loose stacks near the start and forbids half.
func forbidScene(g *game.Game, cx, cy int) {
	spots := []struct {
		kind item.Kind
		qty  int
	}{{item.Wood, 30}, {item.Meat, 12}, {item.Rifle, 1}, {item.Berries, 15}, {item.Wood, 12}}
	for i, s := range spots {
		id, err := g.SpawnStack(s.kind, s.qty, cx+4+i, cy-5)
		if err == nil && i%2 == 0 {
			g.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(id), Enabled: true})
		}
	}
}
