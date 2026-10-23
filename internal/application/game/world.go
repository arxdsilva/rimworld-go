package game

import (
	"slices"
	"sort"
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/generation"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// MineYield is the chunks of its stone dropped by mining one rock tile
// (SIM-029: a stonecutter cuts each chunk into blocks).
const MineYield = 1

// MineWork is the labour to mine one rock tile; an ore vein takes half as
// long again (mineWorkAt).
const MineWork = 360

// OreYield is the metal dropped by mining one ore tile (SIM-029; original
// numbers).
var OreYield = [...]int{geology.Steel: 20, geology.Silver: 25, geology.Gold: 8}

// oreMetal is the item mined from an ore vein.
func oreMetal(o geology.Ore) item.Kind {
	switch o {
	case geology.Silver:
		return item.Silver
	case geology.Gold:
		return item.Gold
	}
	return item.Steel
}

// TerrainNames maps WorldView.Terrain codes to names.
func TerrainNames() []string { return terrain.Names() }

// StoneNames maps WorldView.StoneType codes to names ("" is none).
func StoneNames() []string {
	out := []string{""}
	for _, s := range geology.Stones() {
		out = append(out, s.String())
	}
	return out
}

// OreNames maps WorldView.Ore codes to names ("" is none).
func OreNames() []string {
	out := []string{""}
	for _, o := range geology.Ores() {
		out = append(out, o.String())
	}
	return out
}

// generate replaces the map with a seeded generated one for the game's
// biome (temperate forest: the original generator).
func (g *Game) generate(seed uint64) generation.Result {
	res := generation.GenerateBiome(seed, g.grid.Width(), g.grid.Height(), biome.DefOf(g.biome))
	for y := 0; y < g.grid.Height(); y++ {
		for x := 0; x < g.grid.Width(); x++ {
			g.setTerrain(x, y, res.Terrain.At(x, y))
		}
	}
	// SIM-029: stone types and ore veins, from their own RNG stream
	g.geo = geology.Generate(seed, g.terr)
	g.structVer++
	for _, p := range res.Plants {
		_, _ = g.spawnPlant(p.Species, p.X, p.Y, p.Growth)
	}
	return res
}

func (g *Game) setTerrain(x, y int, t terrain.Type) {
	g.terr.Set(x, y, t)
	if t != terrain.Rock {
		g.geo.SetOre(x, y, geology.NoOre) // ore only lives in rough rock
	}
	if len(g.smooths) > 0 {
		delete(g.smooths, [2]int{x, y}) // the marked ground is gone
	}
	g.structVer++
	g.clearGrass(x, y) // grass.go: only soil grows grass
	g.markRoomsDirty()
	g.grid.SetWalkable(x, y, t.Walkable() && g.layerBlockerAt(x, y) == 0)
}

// naturalStone reports ground that has a stone type: rock walls and
// rough or smoothed stone ground.
func naturalStone(t terrain.Type) bool {
	return terrain.IsRockWall(t) || t == terrain.Stone || t == terrain.StoneSmooth
}

// stoneAt is the stone of (x, y): its geology, or granite for natural
// stone on a map without geology (tests, legacy cells); None elsewhere.
func (g *Game) stoneAt(x, y int) geology.Stone {
	s := g.geo.At(x, y)
	if s == geology.None && g.grid.InBounds(x, y) && naturalStone(g.terr.At(x, y)) {
		return geology.Granite
	}
	return s
}

// mineWorkAt is the labour to mine (x, y): ore veins take longer.
func (g *Game) mineWorkAt(x, y int) int {
	if g.geo.OreAt(x, y) != geology.NoOre {
		return MineWork * 3 / 2
	}
	return MineWork
}

// stoneLayers is the shared read-only geology view (WorldView.StoneType,
// Ore), rebuilt into fresh slices only when structVer has moved, so an
// older view never changes under its reader.
func (g *Game) stoneLayers() ([]uint8, []uint8) {
	if g.viewStone != nil && g.viewVer == g.structVer {
		return g.viewStone, g.viewOre
	}
	w, h := g.terr.W, g.terr.H
	st, ore := make([]uint8, w*h), make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			st[i] = uint8(g.stoneAt(x, y)) // geology.Stone codes match StoneNames()
			ore[i] = uint8(g.geo.OreAt(x, y))
		}
	}
	g.viewStone, g.viewOre, g.viewVer = st, ore, g.structVer
	return st, ore
}

// TerrainAt returns the ground type name at (x, y).
func (g *Game) TerrainAt(x, y int) string { return g.terr.At(x, y).String() }

func (g *Game) spawnPlant(s plant.Species, x, y, growth int) (entity.EntityID, error) {
	if !g.grid.InBounds(x, y) || g.plantAt(x, y) != 0 {
		return 0, errSpawn
	}
	id := g.entities.Next()
	g.plantIDs = append(g.plantIDs, id)
	p := plant.Plant{Species: s, X: x, Y: y, Growth: growth}
	g.plants.Set(id, p)
	g.notePlant(id, p) // indexes.go
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
	g.plantIDs = dropIDs(g.plantIDs, func(pid entity.EntityID) bool { return pid == id })
}

// phasePlants grows every plant on fertile ground, and grass back on
// grazed or cut tiles that are due (grass.go).
func (g *Game) phasePlants() {
	g.regrowGrass()
	if !plant.GrowingHour(g.Date().Hour) {
		return // plants grow in daylight only
	}
	now := uint64(g.clock.Now())
	for _, id := range g.plantIDs {
		p, ok := g.plants.Get(id)
		if !ok || p.Ripe() {
			continue
		}
		rate := plant.Rate(p.Species, g.terr.At(p.X, p.Y).Fertility())
		rate = rate * plant.TempFactor(int(g.temperatureAt(p.X, p.Y))) / 1000
		p = p.Grow(rate, now)
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
	g.markGrass(minX, minY, maxX, maxY, on) // grass.go
}

func (g *Game) markMine(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := normalizeRect(x1, y1, x2, y2)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			k := [2]int{x, y}
			if on && terrain.IsRockWall(g.terr.At(x, y)) && g.grid.InBounds(x, y) {
				g.mines[k] = true
				g.minesVer++
			} else if !on {
				delete(g.mines, k)
				g.minesVer++
			}
		}
	}
}

// mineList is the mine designations in a stable order (y, then x): a
// shared read-only slice, rebuilt only when minesVer moves.
func (g *Game) mineList() [][2]int {
	c := &g.mineCache
	if c.ok && c.ver == g.minesVer {
		if selfCheck() && !slices.Equal(c.list, g.scanMineList()) {
			panic("mineList: mines changed without a minesVer bump")
		}
		return c.list
	}
	c.list, c.ver, c.ok = g.scanMineList(), g.minesVer, true
	return c.list
}

func (g *Game) scanMineList() [][2]int {
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
	best := g.nearestPlant(id, job.Cut, func(p plant.Plant) bool { return p.Cut })
	if best == 0 {
		return g.assignCutGrass(id) // grass.go: marked grass after plants
	}
	p, _ := g.plants.Get(best)
	return g.startJob(id, job.Record{Kind: job.Cut, Target: best, TX: p.X, TY: p.Y, WorkLeft: plant.DefOf(p.Species).CutWork})
}

// nearestPlant is the closest unreserved plant matching want that pawn id
// can actually walk to (0 if none) for a job of kind k. Plants walled in by
// rock are skipped, and put off (SIM-027) so colonists don't search for a
// way to an unreachable one all day.
func (g *Game) nearestPlant(id entity.EntityID, k job.Kind, want func(plant.Plant) bool) entity.EntityID {
	pos, _ := g.positions.Get(id)
	type cand struct {
		id entity.EntityID
		d  int
	}
	var cs []cand
	for _, pid := range g.plantIDs {
		p, ok := g.plants.Get(pid)
		if !ok || !want(p) {
			continue
		}
		if _, taken := g.reserve.EntityOwner(pid); taken {
			continue
		}
		if g.postponedFor(id, k, pid, 0, 0) {
			continue
		}
		cs = append(cs, cand{pid, pathing.Dist(pos.X, pos.Y, p.X, p.Y)})
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].d < cs[j].d || cs[i].d == cs[j].d && cs[i].id < cs[j].id })
	for i, c := range cs {
		if i >= 16 {
			break
		}
		p, _ := g.plants.Get(c.id)
		if g.reachable(id, p.X, p.Y) {
			return c.id
		}
		g.postpone(postKey{Pawn: id, Kind: k, Target: c.id}, job.Unreachable)
	}
	return 0
}

func (g *Game) doCut(id entity.EntityID, j job.Record) {
	if j.Target == 0 {
		g.doCutGrass(id, j) // grass.go: a grass tile, not a plant
		return
	}
	p, ok := g.plants.Get(j.Target)
	if !ok {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	if !g.workTick(id) {
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

// assignMine mines the nearest designated rock, trying the next nearest
// when one can't be started (walled in, postponed: SIM-027), at most
// workTries rocks.
func (g *Game) assignMine(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	list := g.mineList()
	var tried [][2]int
	for range workTries {
		bestD := 1 << 30
		var best [2]int
		found := false
		for _, k := range list {
			if _, taken := g.reserve.TileOwner(k[0], k[1]); taken {
				continue
			}
			d := pathing.Dist(pos.X, pos.Y, k[0], k[1])
			if d >= bestD || slices.Contains(tried, k) || g.postponedFor(id, job.Mine, 0, k[0], k[1]) {
				continue
			}
			best, bestD, found = k, d, true
		}
		if !found {
			return false
		}
		if g.startMine(id, best[0], best[1], false) {
			return true
		}
		tried = append(tried, best)
	}
	return false
}

func (g *Game) startMine(id entity.EntityID, rx, ry int, forced bool) bool {
	tx, ty, ok := g.reachableSpot(id, rx, ry)
	if !ok {
		g.postponeTile(id, job.Mine, rx, ry) // walled in: SIM-027
		return false
	}
	if !g.startJob(id, job.Record{Kind: job.Mine, TX: tx, TY: ty, DestX: rx, DestY: ry, WorkLeft: g.mineWorkAt(rx, ry), Forced: forced}) {
		return false
	}
	g.reserve.TryTile(id, rx, ry)
	return true
}

func (g *Game) doMine(id entity.EntityID, j job.Record) {
	if !terrain.IsRockWall(g.terr.At(j.DestX, j.DestY)) {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	if !g.workTick(id) {
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft--
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	ore := g.geo.OreAt(j.DestX, j.DestY)
	g.setTerrain(j.DestX, j.DestY, terrain.Stone) // clears the ore; the stone stays
	g.roofOnMined(j.DestX, j.DestY)
	delete(g.mines, [2]int{j.DestX, j.DestY})
	g.minesVer++
	if ore != geology.NoOre {
		g.dropStack(oreMetal(ore), OreYield[ore], j.DestX, j.DestY)
	} else {
		g.dropStack(item.ChunkOf(g.stoneAt(j.DestX, j.DestY)), MineYield, j.DestX, j.DestY)
	}
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

// SoilView describes a ground type for the hover readout.
type SoilView struct {
	Name      string // terrain code name, e.g. "rich_soil"
	Label     string // display name, e.g. "Rich soil"
	Fertility int    // growth rate in percent; 0 = nothing grows
	Farmable  bool   // growing zones may be placed here
	Walkable  bool
	// WalkCost is the time to cross it, percent of plain ground (marsh 200:
	// "slow ground" in the hover; floors cancel it).
	WalkCost int
	// Smoothable: a builder may smooth it (rough rock without ore, rough
	// stone ground without a building; SIM-029).
	Smoothable bool
}

var soilLabels = map[string]string{
	"soil": "Soil", "rich_soil": "Rich soil", "gravel": "Gravel",
	"stone": "Rough stone", "rock": "Rock wall",
	"sand": "Sand", "marsh": "Marsh", "ice": "Ice",
	"rock_smooth": "Smoothed rock wall", "stone_smooth": "Smooth stone floor",
	"shallow_water": "Shallow water", "deep_water": "Deep water", "mud": "Mud", "track": "Dirt track",
}

// Soils lists every ground type, indexed by the WorldView.Terrain code.
func Soils() []SoilView {
	names := terrain.Names()
	out := make([]SoilView, len(names))
	for i, n := range names {
		t := terrain.Type(i)
		out[i] = SoilView{Name: n, Label: soilLabels[n], Fertility: t.Fertility(),
			Farmable: t.Fertility() > 0, Walkable: t.Walkable(), WalkCost: t.WalkCost(),
			Smoothable: t == terrain.Rock || t == terrain.Stone}
	}
	return out
}

// SoilLabel is the hover name of a tile's ground (SIM-029): ground, stone
// and ore are names as in TerrainNames(), StoneNames() and OreNames(), ""
// meaning none. Natural stone names its stone ("Granite rock wall",
// "Smooth granite floor", "Compacted steel (in granite)"); without a
// stone it is the plain Soils() label; an unknown ground is "".
func SoilLabel(ground, stone, ore string) string {
	s, ok := geology.ParseStone(stone)
	if !ok {
		return soilLabels[ground]
	}
	name := strings.ToLower(s.Label())
	switch ground {
	case "rock":
		for _, o := range geology.Ores() {
			if o.String() == ore {
				return o.Label() + " (in " + name + ")"
			}
		}
		return s.Label() + " rock wall"
	case "rock_smooth":
		return "Smoothed " + name + " wall"
	case "stone":
		return "Rough " + name + " ground"
	case "stone_smooth":
		return "Smooth " + name + " floor"
	}
	return soilLabels[ground]
}
