package game

import (
	"strconv"
	"strings"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/chunk"
	"rimworld-go/internal/domain/world/plant"
)

// Staggered plant growth and frost (NG-1 §6.6, E4). Every chunk's plants
// grow once per climate interval, at their chunk's own phase of it (chunk
// c at ticks where now % climateInterval == c*climateInterval/chunks), by
// plant.GrowSpan over the interval just ended: one growth rate per
// window (the soil, the air temperature and the scenario's growth
// percent at the chunk's tick) instead of one per tick. Frost and heat
// damage plants in the same pass, at the same temperature, over the same
// window (plant.Expose, PLAY-016), killing them at 100% after hours. So
// the plant work of a tick is a few chunks' worth instead of every plant
// on the map, and the frost scan no longer lands on one tick.
//
// A plant sown during a window grows for the whole window it is first
// seen in (at most climateInterval ticks early, a tenth of an hour).

// frostKill is one species' kills in a pass (a notice per species).
type frostKill struct {
	species plant.Species
	n       int
	x, y    int
	hot     bool
}

// frostPass collects one pass's kills (the ids to drop from plantIDs and
// the notices to post) and the species frost or heat began to hurt (a
// warning each, at the first such plant).
type frostPass struct {
	gone  map[entity.EntityID]bool
	kills []frostKill
	hurt  []frostKill
}

// growChunks grows (and frosts) the plants of the chunks whose phase is
// this tick.
func (g *Game) growChunks() {
	now := uint64(g.clock.Now())
	cg := chunk.New(g.grid.Width(), g.grid.Height())
	n := cg.Count()
	ph := int(now % climateInterval)
	var fp frostPass
	gp := g.growthPct() // setup.go: a scenario's permanent conditions (Ashfall)
	for c := (ph*n + climateInterval - 1) / climateInterval; c < n && c*climateInterval/n == ph; c++ {
		g.chunkPlants(cg, c, func(id entity.EntityID, p plant.Plant) {
			g.growPlant(id, p, now, gp, &fp)
		})
		g.growGrassChunk(c, now) // grass.go: regrowing grass fills out (PLAY-010)
	}
	g.endFrostPass(&fp)
}

// chunkPlants calls fn for every plant in chunk c, cell by cell (from the
// plant cell index; a cell holding more than one plant is scanned).
func (g *Game) chunkPlants(cg chunk.Grid, c int, fn func(entity.EntityID, plant.Plant)) {
	ix := g.plantIndex()
	x0, y0, x1, y1 := cg.Bounds(c)
	w := g.grid.Width()
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*w + x
			if ix.conflict[i] {
				for _, id := range g.plantIDs {
					if p, ok := g.plants.Get(id); ok && p.X == x && p.Y == y {
						fn(id, p)
					}
				}
				continue
			}
			id := ix.slots[0][i]
			if id == 0 {
				continue
			}
			if p, ok := g.plants.Get(id); ok && p.X == x && p.Y == y {
				fn(id, p)
			}
		}
	}
}

// growPlant damages plant id if the air is out of its range (removing it
// at 100%), then grows it over the window that ends now.
func (g *Game) growPlant(id entity.EntityID, p plant.Plant, now uint64, gp int, fp *frostPass) {
	t := int(g.temperatureAt(p.X, p.Y))
	if g.frostKills(id, &p, t, now, fp) || !g.lightGrows(id, &p, fp) || p.Ripe() { // plant_light.go (MAPGEN-3)
		return
	}
	rate := plant.Rate(p.Species, plant.Fertility(p.Species, g.terr.At(p.X, p.Y))) // stone feeds cave plants
	rate = rate * plant.TempFactor(t) / 1000
	if gp != 100 {
		rate = rate * gp / 100
	}
	if n := plant.GrowSpan(rate, now-min(now, climateInterval), now); n > 0 {
		g.plants.Set(id, p.AddGrowth(n))
	}
}

// frostKills applies the air t to plant id over the window that ends now
// (p is updated in place): a crop past its frost limit, or a wild biome
// plant out of its climate (SIM-023: a jungle tree in a temperate winter,
// a pine in a desert summer), takes damage over hours (plant.Expose,
// PLAY-016) and is removed at 100%. The original tree and berry bush only
// go dormant. It reports whether the plant died.
func (g *Game) frostKills(id entity.EntityID, p *plant.Plant, t int, now uint64, fp *frostPass) bool {
	q, started := p.Expose(t, now-min(now, climateInterval), now)
	if started {
		fp.hurt = addKill(fp.hurt, q)
	}
	if q == *p {
		return false
	}
	*p = q
	if !q.Dead() {
		g.plants.Set(id, q)
		return false
	}
	g.plants.Delete(id) // removePlant, with the list compacted by endFrostPass
	g.plantsChanged()   // jobindex.go

	g.plantLightChanged(q.Species) // light.go: a glowing plant gone
	if fp.gone == nil {
		fp.gone = map[entity.EntityID]bool{}
	}
	fp.gone[id] = true
	fp.kills = addKill(fp.kills, q)
	return true
}

// addKill counts plant p in ks: one entry per species and cause, at the
// first plant's spot.
func addKill(ks []frostKill, p plant.Plant) []frostKill {
	for i := range ks {
		if ks[i].species == p.Species && ks[i].hot == p.DamageHeat {
			ks[i].n++
			return ks
		}
	}
	return append(ks, frostKill{species: p.Species, n: 1, x: p.X, y: p.Y, hot: p.DamageHeat})
}

// frostCrops runs the frost pass over the whole map for the window that
// ends now (tests, and the reference for the staggered pass).
func (g *Game) frostCrops() {
	cg := chunk.New(g.grid.Width(), g.grid.Height())
	now := uint64(g.clock.Now())
	var fp frostPass
	for c := 0; c < cg.Count(); c++ {
		g.chunkPlants(cg, c, func(id entity.EntityID, p plant.Plant) {
			g.frostKills(id, &p, int(g.temperatureAt(p.X, p.Y)), now, &fp)
		})
	}
	g.endFrostPass(&fp)
}

// endFrostPass warns of the species frost or heat began to hurt
// (hurtNotice), drops the killed plants from plantIDs (once, in order)
// and posts a notice per species. Kills of one species in one climate
// interval share a notice: a later pass in the same interval adds its
// count to it, so a frost that sweeps the staggered chunks still reads as
// one event, whatever tick a save was taken on.
func (g *Game) endFrostPass(fp *frostPass) {
	for _, k := range fp.hurt {
		g.hurtNotice(k)
	}
	if fp.gone == nil {
		return
	}
	g.plantIDs = dropIDs(g.plantIDs, func(id entity.EntityID) bool { return fp.gone[id] })
	now := uint64(g.clock.Now())
	from := now - now%climateInterval
	for _, k := range fp.kills {
		g.frostNotice(k, from)
	}
}

// frostNotice posts k's notice, or adds k to this interval's notice for
// the same species and cause.
func (g *Game) frostNotice(k frostKill, from uint64) {
	for i := len(g.messages) - 1; i >= 0 && g.messages[i].Tick >= from; i-- {
		if n, ok := frostCount(g.messages[i].Text, k.species, k.hot); ok {
			g.messages[i].Text = frostText(k.species, n+k.n, k.hot)
			return
		}
	}
	g.message(k.x, k.y, "%s", frostText(k.species, k.n, k.hot))
}

// hurtNotice warns that frost or heat began to hurt k's species: once a
// day per species and cause (a warning of the last day, still among the
// recent messages, covers it).
func (g *Game) hurtNotice(k frostKill) {
	text := hurtText(k.species, k.hot)
	now := uint64(g.clock.Now())
	for i := len(g.messages) - 1; i >= 0 && g.messages[i].Tick+clock.TicksPerDay > now; i-- {
		if g.messages[i].Text == text {
			return
		}
	}
	g.message(k.x, k.y, "%s", text)
}

// hurtText is the warning that frost (or heat, hot) is hurting species s:
// "Frost is damaging the corn", "Heat is scorching the pine plants".
func hurtText(s plant.Species, hot bool) string {
	name := strings.ReplaceAll(plantLabel(s), "_", " ") + " plants"
	if plant.DefOf(s).Crop {
		name = cropField(s)
	}
	if hot {
		return "Heat is scorching the " + name
	}
	return "Frost is damaging the " + name
}

// cropField is a crop's name as a field of it ("the potatoes").
func cropField(s plant.Species) string {
	switch s {
	case plant.Potato:
		return "potatoes"
	case plant.Radish:
		return "radishes"
	case plant.Pumpkin:
		return "pumpkins"
	}
	return s.String()
}

// frostText is the notice for n plants of species s killed by frost (a
// crop) or by cold or heat (a wild plant).
func frostText(s plant.Species, n int, hot bool) string {
	d := plant.DefOf(s)
	switch {
	case d.Crop && n == 1:
		return "Frost killed a " + s.String() + " plant"
	case d.Crop:
		return "Frost killed the " + d.Label + " (" + strconv.Itoa(n) + " plants)"
	}
	cause := "cold"
	if hot {
		cause = "heat"
	}
	name := strings.ReplaceAll(plantLabel(s), "_", " ")
	if n == 1 {
		return "The " + cause + " killed a " + name
	}
	return "The " + cause + " killed " + strconv.Itoa(n) + " " + name + " plants"
}

// frostCount is the plant count of a notice frostText wrote for s and
// hot (false if text is not one).
func frostCount(text string, s plant.Species, hot bool) (int, bool) {
	if text == frostText(s, 1, hot) {
		return 1, true
	}
	const mark = 987654321 // a count no notice holds: splits the plural text
	pre, post, _ := strings.Cut(frostText(s, mark, hot), strconv.Itoa(mark))
	if !strings.HasPrefix(text, pre) || !strings.HasSuffix(text, post) || len(text) <= len(pre)+len(post) {
		return 0, false
	}
	n, err := strconv.Atoi(text[len(pre) : len(text)-len(post)])
	if err != nil || n < 2 {
		return 0, false
	}
	return n, true
}

// plantWalkCost is the step time (percent) of the plant on (x, y), 0 for
// none or a plant that doesn't slow walkers (PLAY-010: brambles). It
// reads the plant cell index directly (no scan cross-check): it runs per
// A* node and per step.
func (g *Game) plantWalkCost(x, y int) int {
	id := g.plantAtIndexed(x, y)
	if id == 0 {
		return 0
	}
	p, _ := g.plants.Get(id)
	return plant.DefOf(p.Species).WalkCost
}

// migrateHarvestMarks gives an older save's ripe zone crops the harvest
// mark they now get on ripening (PLAY-017).
func (g *Game) migrateHarvestMarks() {}
