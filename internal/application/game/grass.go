package game

// Grass (UI-011): ground cover on soil, owned by the simulation so that
// building, sowing and Cut plants remove it. Seeded from terrain by
// plant.GrassAt (the rule the renderer used), never regrows (v1).

// clearGrass removes grass at (x, y), and any cut mark there.
func (g *Game) clearGrass(x, y int) {} // UI-011 W5

// markGrass marks (on) or unmarks grass tiles in the rectangle for cutting.
func (g *Game) markGrass(minX, minY, maxX, maxY int, on bool) {} // UI-011 W5

// unmarkGrass drops cut marks on grass in the (normalized) rectangle.
func (g *Game) unmarkGrass(minX, minY, maxX, maxY int) { g.markGrass(minX, minY, maxX, maxY, false) }

// fillGrassView copies grass and its cut marks into the view.
func (g *Game) fillGrassView(w *WorldView) {} // UI-011 W5
