package raid

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/grid"
)

// Hive swarms (SIM-019). A storyteller threat from day 12: three to six
// hive insects break out of the rock face nearest the colony (or come in
// at a map edge when there is no rock). They are hostile from the moment
// they appear and stay so: they hunt colonists like manhunters, never
// flee, are never tamed, and their rage never runs out.

const (
	hiveMin = 3
	hiveMax = 6
	// HiveForever is a manhunter deadline that never passes.
	HiveForever = ^uint64(0)
)

// Hives sends hive swarms.
type Hives struct {
	h    Host
	w    Wild
	exit exitCache
}

// exitCache is Exit's last answer and the structure it was for.
type exitCache struct {
	ok                 bool
	grid               *grid.Grid
	gridVer, structVer uint64
	hx, hy, x, y       int
	found              bool
}

// Swarm is the threat itself (also the game's HiveSwarm dev hook).
func (hv *Hives) Swarm() {
	n := hiveMin + hv.h.Roll(hiveMax-hiveMin+1)
	x, y, ok := hv.Exit()
	if !ok {
		if x, y, ok = hv.w.EdgeTile(); !ok {
			return
		}
	}
	brutes, spawned := 0, 0
	for i := 0; i < n; i++ {
		s := animal.HiveBeetle
		switch hv.h.Roll(3) {
		case 1:
			s = animal.HiveCrawler
		case 2:
			if brutes < n/3 {
				s = animal.HiveBrute
			} else {
				s = animal.HiveCrawler
			}
		}
		if id, ok := hv.SpawnNear(s, x, y); ok {
			if s == animal.HiveBrute {
				brutes++
			}
			a := hv.w.Animal(id)
			a.Manhunter, a.Mode = HiveForever, animal.Attack
			hv.w.SetAnimal(id, a)
			spawned++
		}
	}
	if spawned > 0 {
		hv.h.Threat("Hive swarm", x, y, "A hive swarm bursts from the rock!")
	}
}

// SpawnNear places s on a walkable tile within two of (x, y).
func (hv *Hives) SpawnNear(s animal.Species, x, y int) (entity.EntityID, bool) {
	for try := 0; try < 12; try++ {
		hx, hy := x+hv.h.Roll(5)-2, y+hv.h.Roll(5)-2
		if !hv.h.InBounds(hx, hy) || hv.h.Rock(hx, hy) {
			continue
		}
		if id, err := hv.w.SpawnAnimal(s, hx, hy); err == nil {
			return id, true
		}
	}
	return 0, false
}

// Exit is the open tile beside the rock face nearest the colony: the
// walkable tile bordering natural rock closest to home (ties: row-major
// first). The answer is cached per structure: grid, terrain and buildings
// unchanged and home in the same place give the same tile.
func (hv *Hives) Exit() (int, int, bool) {
	hx, hy := hv.h.Home()
	g, gridVer, structVer := hv.h.Shape()
	c := &hv.exit
	if c.ok && c.grid == g && c.gridVer == gridVer && c.structVer == structVer && c.hx == hx && c.hy == hy {
		return c.x, c.y, c.found
	}
	w, h := hv.h.Size()
	best, bx, by := -1, 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !hv.h.DryWalkable(x, y) || !hv.bordersRock(x, y) {
				continue
			}
			if d := max(abs(x-hx), abs(y-hy)); best < 0 || d < best {
				best, bx, by = d, x, y
			}
		}
	}
	*c = exitCache{ok: true, grid: g, gridVer: gridVer, structVer: structVer,
		hx: hx, hy: hy, x: bx, y: by, found: best >= 0}
	return bx, by, best >= 0
}

// bordersRock: an in-map rock tile touches (x, y) on a side.
func (hv *Hives) bordersRock(x, y int) bool {
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if hv.h.InBounds(nx, ny) && hv.h.Rock(nx, ny) {
			return true
		}
	}
	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
