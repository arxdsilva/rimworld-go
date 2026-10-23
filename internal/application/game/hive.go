package game

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// Hive swarms (SIM-019). A storyteller threat from day 12: three to six
// hive insects break out of the rock face nearest the colony (or come in
// at a map edge when there is no rock). They are hostile from the moment
// they appear and stay so: they hunt colonists like manhunters, never
// flee, are never tamed, and their rage never runs out.

const (
	hiveMin = 3
	hiveMax = 6
	// hiveForever is a manhunter deadline that never passes.
	hiveForever = ^uint64(0)
)

// hiveSwarm is the threat itself (also a dev hook through HiveSwarm).
func (g *Game) hiveSwarm() {
	n := hiveMin + g.rng.IntN(hiveMax-hiveMin+1)
	x, y, ok := g.hiveExit()
	if !ok {
		if x, y, ok = g.edgeTile(); !ok {
			return
		}
	}
	brutes, spawned := 0, 0
	for i := 0; i < n; i++ {
		s := animal.HiveBeetle
		switch g.rng.IntN(3) {
		case 1:
			s = animal.HiveCrawler
		case 2:
			if brutes < n/3 {
				s = animal.HiveBrute
			} else {
				s = animal.HiveCrawler
			}
		}
		if id, ok := g.spawnNear(s, x, y); ok {
			if s == animal.HiveBrute {
				brutes++
			}
			a, _ := g.animals.Get(id)
			a.Manhunter, a.Mode = hiveForever, animal.Attack
			g.animals.Set(id, a)
			spawned++
		}
	}
	if spawned > 0 {
		g.threat(x, y, "A hive swarm bursts from the rock!")
	}
}

// HiveSwarm is the dev hook for a hive swarm.
func (g *Game) HiveSwarm() {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.hiveSwarm()
}

// spawnNear places s on a walkable tile within two of (x, y).
func (g *Game) spawnNear(s animal.Species, x, y int) (entity.EntityID, bool) {
	for try := 0; try < 12; try++ {
		hx, hy := x+g.rng.IntN(5)-2, y+g.rng.IntN(5)-2
		if !g.grid.InBounds(hx, hy) || terrain.IsRockWall(g.terr.At(hx, hy)) {
			continue
		}
		if id, err := g.spawnAnimal(s, hx, hy); err == nil {
			return id, true
		}
	}
	return 0, false
}

// hiveExit is the open tile beside the rock face nearest the colony: the
// walkable tile bordering natural rock closest to home (ties: row-major
// first). The answer is cached per structure: grid, terrain and buildings
// unchanged and home in the same place give the same tile.
func (g *Game) hiveExit() (int, int, bool) {
	hx, hy := g.home()
	c := &g.regs.hive
	if c.ok && c.grid == g.grid && c.gridVer == g.grid.Version() && c.structVer == g.structVer && c.hx == hx && c.hy == hy {
		return c.x, c.y, c.found
	}
	w, h := g.grid.Width(), g.grid.Height()
	best, bx, by := -1, 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !g.dryWalkable(x, y) || !g.bordersRock(x, y) {
				continue
			}
			if d := chebyshev(Position{X: x, Y: y}, Position{X: hx, Y: hy}); best < 0 || d < best {
				best, bx, by = d, x, y
			}
		}
	}
	*c = hiveExitCache{ok: true, grid: g.grid, gridVer: g.grid.Version(), structVer: g.structVer,
		hx: hx, hy: hy, x: bx, y: by, found: best >= 0}
	return bx, by, best >= 0
}

// bordersRock: an in-map rock tile touches (x, y) on a side.
func (g *Game) bordersRock(x, y int) bool {
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if g.grid.InBounds(nx, ny) && terrain.IsRockWall(g.terr.At(nx, ny)) {
			return true
		}
	}
	return false
}
