package game

import (
	"strconv"
	"strings"

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
// kill in the same pass, at the same temperature. So the plant work of a
// tick is a few chunks' worth instead of every plant on the map, and the
// frost scan no longer lands on one tick.
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

// frostPass collects one pass's kills: the ids to drop from plantIDs and
// the notices to post.
type frostPass struct {
	gone  map[entity.EntityID]bool
	kills []frostKill
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

// growPlant kills plant id if the air is out of its range, else grows it
// over the window that ends now.
func (g *Game) growPlant(id entity.EntityID, p plant.Plant, now uint64, gp int, fp *frostPass) {
	t := int(g.temperatureAt(p.X, p.Y))
	if g.frostKills(id, p, t, fp) || p.Ripe() {
		return
	}
	rate := plant.Rate(p.Species, g.terr.At(p.X, p.Y).Fertility())
	rate = rate * plant.TempFactor(t) / 1000
	if gp != 100 {
		rate = rate * gp / 100
	}
	if n := plant.GrowSpan(rate, now-min(now, climateInterval), now); n > 0 {
		g.plants.Set(id, p.AddGrowth(n))
	}
}

// frostKills removes a crop standing in air colder than it can bear, or
// a wild biome plant (pine, jungle tree, cactus, ...) out of its climate
// (SIM-023): a jungle tree sown into a temperate winter, a pine in a
// desert summer. The original tree and berry bush only go dormant.
func (g *Game) frostKills(id entity.EntityID, p plant.Plant, t int, fp *frostPass) bool {
	if !plant.Frostbitten(p.Species, t) && !plant.Killed(p.Species, t) {
		return false
	}
	g.plants.Delete(id) // removePlant, with the list compacted by endFrostPass
	g.plantsChanged()   // jobindex.go
	if fp.gone == nil {
		fp.gone = map[entity.EntityID]bool{}
	}
	fp.gone[id] = true
	for i := range fp.kills {
		if fp.kills[i].species == p.Species {
			fp.kills[i].n++
			return true
		}
	}
	d := plant.DefOf(p.Species)
	fp.kills = append(fp.kills, frostKill{species: p.Species, n: 1, x: p.X, y: p.Y, hot: d.HeatKill != 0 && t > d.HeatKill})
	return true
}

// frostCrops runs the frost check over the whole map now (tests, and
// the reference for the staggered pass).
func (g *Game) frostCrops() {
	cg := chunk.New(g.grid.Width(), g.grid.Height())
	var fp frostPass
	for c := 0; c < cg.Count(); c++ {
		g.chunkPlants(cg, c, func(id entity.EntityID, p plant.Plant) {
			g.frostKills(id, p, int(g.temperatureAt(p.X, p.Y)), &fp)
		})
	}
	g.endFrostPass(&fp)
}

// endFrostPass drops the killed plants from plantIDs (once, in order) and
// posts a notice per species. Kills of one species in one climate
// interval share a notice: a later pass in the same interval adds its
// count to it, so a frost that sweeps the staggered chunks still reads as
// one event, whatever tick a save was taken on.
func (g *Game) endFrostPass(fp *frostPass) {
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
