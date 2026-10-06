package game

import (
	"sort"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/generation"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// MineYield is the stone blocks dropped by mining one rock tile.
const MineYield = 5

// MineWork is the labour to mine one rock tile.
const MineWork = 360

// TerrainNames maps WorldView.Terrain codes to names.
func TerrainNames() []string { return terrain.Names() }

// generate replaces the map with a seeded generated one.
func (g *Game) generate(seed uint64) generation.Result {
	res := generation.Generate(seed, g.grid.Width(), g.grid.Height())
	for y := 0; y < g.grid.Height(); y++ {
		for x := 0; x < g.grid.Width(); x++ {
			g.setTerrain(x, y, res.Terrain.At(x, y))
		}
	}
	for _, p := range res.Plants {
		_, _ = g.spawnPlant(p.Species, p.X, p.Y, p.Growth)
	}
	return res
}

func (g *Game) setTerrain(x, y int, t terrain.Type) {
	g.terr.Set(x, y, t)
	g.grid.SetWalkable(x, y, t.Walkable() && g.layerBlockerAt(x, y) == 0)
}

// TerrainAt returns the ground type name at (x, y).
func (g *Game) TerrainAt(x, y int) string { return g.terr.At(x, y).String() }

func (g *Game) spawnPlant(s plant.Species, x, y, growth int) (entity.EntityID, error) {
	if !g.grid.InBounds(x, y) || g.plantAt(x, y) != 0 {
		return 0, errSpawn
	}
	id := g.entities.Next()
	g.plantIDs = append(g.plantIDs, id)
	g.plants.Set(id, plant.Plant{Species: s, X: x, Y: y, Growth: growth})
	return id, nil
}

// SpawnPlantSpecies places a plant with growth in ticks (fixtures, generation).
func (g *Game) SpawnPlantSpecies(species string, x, y, growth int) (entity.EntityID, error) {
	s, ok := plant.Parse(species)
	if !ok {
		return 0, errSpawn
	}
	return g.spawnPlant(s, x, y, growth)
}

func (g *Game) removePlant(id entity.EntityID) {
	g.plants.Delete(id)
	kept := g.plantIDs[:0]
	for _, pid := range g.plantIDs {
		if pid != id {
			kept = append(kept, pid)
		}
	}
	g.plantIDs = kept
}

// phasePlants grows every plant on fertile ground.
func (g *Game) phasePlants() {
	now := uint64(g.clock.Now())
	for _, id := range g.plantIDs {
		p, ok := g.plants.Get(id)
		if !ok || p.Ripe() {
			continue
		}
		p = p.Grow(g.terr.At(p.X, p.Y).Fertility(), now)
		g.plants.Set(id, p)
	}
}

func (g *Game) markCut(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := normalizeRect(x1, y1, x2, y2)
	for _, id := range g.plantIDs {
		p, ok := g.plants.Get(id)
		if !ok || p.X < minX || p.X > maxX || p.Y < minY || p.Y > maxY {
			continue
		}
		p.Cut = on
		if on {
			p.Designated = false
		}
		g.plants.Set(id, p)
	}
}

func (g *Game) markMine(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := normalizeRect(x1, y1, x2, y2)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			k := [2]int{x, y}
			if on && g.terr.At(x, y) == terrain.Rock && g.grid.InBounds(x, y) {
				g.mines[k] = true
			} else if !on {
				delete(g.mines, k)
			}
		}
	}
}

// mineList is the mine designations in a stable order.
func (g *Game) mineList() [][2]int {
	out := make([][2]int, 0, len(g.mines))
	for k := range g.mines {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][1] != out[j][1] {
			return out[i][1] < out[j][1]
		}
		return out[i][0] < out[j][0]
	})
	return out
}

func (g *Game) assignCut(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, pid := range g.plantIDs {
		p, ok := g.plants.Get(pid)
		if !ok || !p.Cut {
			continue
		}
		if _, taken := g.reserve.EntityOwner(pid); taken {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, p.X, p.Y)
		if d < bestD || (d == bestD && pid < best) {
			best, bestD = pid, d
		}
	}
	if best == 0 {
		return false
	}
	p, _ := g.plants.Get(best)
	return g.startJob(id, job.Record{Kind: job.Cut, Target: best, TX: p.X, TY: p.Y, WorkLeft: plant.DefOf(p.Species).CutWork})
}

func (g *Game) doCut(id entity.EntityID, j job.Record) {
	p, ok := g.plants.Get(j.Target)
	if !ok {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft--
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	wood := p.WoodYield()
	g.removePlant(j.Target)
	if wood > 0 {
		g.dropStack(item.Wood, wood, p.X, p.Y)
	}
	j.State = job.Complete
	g.jobs.Set(id, j)
}

func (g *Game) assignMine(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	bestD := 1 << 30
	var best [2]int
	found := false
	for _, k := range g.mineList() {
		if _, taken := g.reserve.TileOwner(k[0], k[1]); taken {
			continue
		}
		d := pathing.Dist(pos.X, pos.Y, k[0], k[1])
		if d < bestD {
			best, bestD, found = k, d, true
		}
	}
	if !found {
		return false
	}
	return g.startMine(id, best[0], best[1], false)
}

func (g *Game) startMine(id entity.EntityID, rx, ry int, forced bool) bool {
	tx, ty, ok := g.reachableSpot(id, rx, ry)
	if !ok {
		return false
	}
	if !g.startJob(id, job.Record{Kind: job.Mine, TX: tx, TY: ty, DestX: rx, DestY: ry, WorkLeft: MineWork, Forced: forced}) {
		return false
	}
	g.reserve.TryTile(id, rx, ry)
	return true
}

func (g *Game) doMine(id entity.EntityID, j job.Record) {
	if g.terr.At(j.DestX, j.DestY) != terrain.Rock {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft--
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	g.setTerrain(j.DestX, j.DestY, terrain.Stone)
	delete(g.mines, [2]int{j.DestX, j.DestY})
	g.dropStack(item.Stone, MineYield, j.DestX, j.DestY)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// reachableSpot is a walkable tile next to (or on) (x, y) the pawn can reach.
func (g *Game) reachableSpot(id entity.EntityID, x, y int) (int, int, bool) {
	if g.grid.Walkable(x, y) && g.reachable(id, x, y) {
		return x, y, true
	}
	for _, d := range [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		nx, ny := x+d[0], y+d[1]
		if g.grid.Walkable(nx, ny) && g.reachable(id, nx, ny) {
			return nx, ny, true
		}
	}
	return 0, 0, false
}

// dropStack spawns a stack and merges it with any same-kind stack there.
func (g *Game) dropStack(k item.Kind, qty, x, y int) {
	sid, err := g.SpawnStack(k, qty, x, y)
	if err != nil {
		return
	}
	if st, ok := g.stacks.Get(sid); ok {
		g.mergeOrSet(sid, st)
	}
}
