package game

import (
	"rimworld-go/internal/application/game/farm"

	"rimworld-go/internal/domain/building/construction"
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

// growChunks grows (and frosts) the plants of the chunks whose phase is
// this tick, and sprouts wild plants back in them (FOOD-2).
func (g *Game) growChunks() {
	now := uint64(g.clock.Now())
	cg := chunk.New(g.grid.Width(), g.grid.Height())
	n := cg.Count()
	ph := int(now % climateInterval)
	var fp farm.Pass
	var lamps []farm.Lamp
	gp := g.growthPct() // setup.go: a scenario's permanent conditions (Ashfall)
	if ph == 0 {
		g.sunLampTick(now) // plant_light.go: the lamps' hours, short circuits
	}
	for c := (ph*n + climateInterval - 1) / climateInterval; c < n && c*climateInterval/n == ph; c++ {
		if lamps == nil {
			lamps = append(g.sunLamps(now), farm.Lamp{X: -1 << 20}) // the sentinel is never in reach
		}
		wild := 0
		g.chunkPlants(cg, c, func(id entity.EntityID, p plant.Plant) {
			if !p.Zone {
				wild++
			}
			g.growPlant(id, p, now, gp, lamps, &fp)
		})
		if plant.GrowingHour(int(clock.HourOfDay(clock.Tick(now)))) {
			g.sproutWild(cg, c, wild, now) // plant_light.go: in the growing hours only
		}
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

// growPlant runs plant id through the window that ends now (FOOD-2,
// wiki: Plants): a crop past its lifespan rots away; one in an unpowered
// hydroponics basin withers; frost and heat damage it (plant.Expose,
// PLAY-016); blight worsens and spreads; then it grows in the growing
// hours at its soil's, the air's and its growing light's factors. A crop
// that ripens on a growing-zone cell is marked for harvest (PLAY-017).
func (g *Game) growPlant(id entity.EntityID, p plant.Plant, now uint64, gp int, lamps []farm.Lamp, fp *farm.Pass) {
	was, t0 := p, now-min(now, climateInterval)
	if p.Sown == 0 && plant.Lifespan(p.Species) > 0 {
		p.Sown = now // its lifespan counts from the first pass that sees it
	}
	t := int(g.temperatureAt(p.X, p.Y))
	basin, fed := g.basinAt(p.X, p.Y)
	switch {
	case p.Rotted(now):
		g.plantDied(id, p, farm.Rot, fp)
		return
	case basin && !fed:
		if p = p.Starve(t0, now); p.Dead() {
			g.plantDied(id, p, farm.Starved, fp)
			return
		}
	case g.frostKills(id, &p, t, now, fp):
		return
	}
	if !g.lightGrows(id, &p, fp) { // plant_light.go (MAPGEN-3)
		return
	}
	light := plant.FullGrowLight
	if !plant.OwnLightRules(p.Species) {
		light = farm.GrowLight(g.roofAt(p.X, p.Y).Roofed(), g.artificialLight(p.X, p.Y), lamps, p.X, p.Y, g.combat.Sight.Line)
	}
	p = p.Blighten(t0, now)
	farm.Spread(plantField{g}, id, p, now) // plant_light.go
	if !p.Ripe() && fed == basin {
		p = p.AddGrowth(plant.GrowSpan(g.growRate(p, t, gp, light), t0, now))
	}
	if p.Life = uint8(p.LifePct(now)); plant.GrowingHour(int(clock.HourOfDay(clock.Tick(now)))) {
		p.Light = uint8(light / 10) // the readout keeps the day's light through the night
	}
	if p != was {
		g.setPlant(id, p)
		if p.Ripe() && !was.Ripe() {
			g.markRipeHarvest(id) // food.go: zone crops are harvested through the mark
		}
	}
}

// growRate is the growth speed (percent, 100: one growth unit per tick of
// growing time) of plant p in air t at growing light (permille): its
// soil, the temperature, the light (not for moss and duskmoss, which keep
// their own rules) and the scenario's growth percent gp.
func (g *Game) growRate(p plant.Plant, t, gp, light int) int {
	rate := plant.Rate(p.Species, plantFertility(g, p)) * plant.TempFactor(t) / 1000
	if !plant.OwnLightRules(p.Species) {
		rate = rate * plant.LightFactor(light) / 1000
	}
	return rate * gp / 100
}

// plantFertility is the fertility (percent) plant p grows at: a
// hydroponics basin's under it, else its ground's (stone feeds cave
// plants).
func plantFertility(g *Game, p plant.Plant) int {
	if basin, _ := g.basinAt(p.X, p.Y); basin {
		return plant.HydroFertile
	}
	return plant.Fertility(p.Species, g.terr.At(p.X, p.Y))
}

// basinAt reports a hydroponics basin on (x, y) and whether it has power.
func (g *Game) basinAt(x, y int) (basin, powered bool) {
	id := g.layerBldgAt(x, y, construction.LayerObject)
	if b := g.bldgs.Ref(id); b == nil || b.Kind != construction.HydroponicsBasin {
		return false, false
	}
	return true, g.powered(id)
}

// frostKills applies the air t to plant id over the window that ends now
// (p is updated in place): a crop past its frost limit, or a wild biome
// plant out of its climate (SIM-023: a jungle tree in a temperate winter,
// a pine in a desert summer), takes damage over hours (plant.Expose,
// PLAY-016) and is removed at 100%. The original tree and berry bush only
// go dormant. It reports whether the plant died.
func (g *Game) frostKills(id entity.EntityID, p *plant.Plant, t int, now uint64, fp *farm.Pass) bool {
	q, started := p.Expose(t, now-min(now, climateInterval), now)
	if started {
		fp.Hurt = farm.Add(fp.Hurt, q, farm.DamageCause(q))
	}
	if q == *p {
		return false
	}
	*p = q
	if !q.Dead() {
		g.setPlant(id, q)
		return false
	}
	g.plantDied(id, q, farm.DamageCause(q), fp)
	return true
}

// plantDied removes plant id (p), dead of cause in this pass: the list is
// compacted and the notices posted by endFrostPass.
func (g *Game) plantDied(id entity.EntityID, p plant.Plant, cause farm.Cause, fp *farm.Pass) {
	g.dropPlant(id)
	g.plantsChanged()              // jobindex.go
	g.plantLightChanged(p.Species) // light.go: a glowing plant gone
	fp.Died(id, p, cause)
}

// endFrostPass warns of the species frost or heat began to hurt (once a
// day per species and cause), drops the dead plants from plantIDs (once,
// in order) and posts a notice per species and cause. Deaths of one
// species in one climate interval share a notice: a later pass in the
// same interval adds its count to it, so a frost that sweeps the
// staggered chunks still reads as one event, whatever tick a save was
// taken on.
func (g *Game) endFrostPass(fp *farm.Pass) {
	now := uint64(g.clock.Now())
	for _, k := range fp.Hurt {
		noticeOnce(g, k.X, k.Y, farm.HurtText(k.Species, k.Cause == farm.Heat), now-min(now, clock.TicksPerDay-1), 1<<30)
	}
	if fp.Gone == nil {
		return
	}
	g.plantIDs = dropIDs(g.plantIDs, func(id entity.EntityID) bool { return fp.Gone[id] })
	from := now - now%climateInterval
	for _, k := range fp.Kills {
		g.frostNotice(k, from)
	}
}

// frostNotice posts k's notice, or adds k to this interval's notice for
// the same species and cause.
func (g *Game) frostNotice(k farm.Kill, from uint64) {
	for i := len(g.messages) - 1; i >= 0 && g.messages[i].Tick >= from; i-- {
		if n, ok := farm.KillCount(g.messages[i].Text, k.Species, k.Cause); ok {
			g.messages[i].Text = farm.KillText(k.Species, n+k.N, k.Cause)
			return
		}
	}
	g.message(k.X, k.Y, "%s", farm.KillText(k.Species, k.N, k.Cause))
}

// noticeOnce posts text at (x, y) unless a message with the same text
// from tick since on, within reach tiles (Chebyshev), already says it.
func noticeOnce(g *Game, x, y int, text string, since uint64, reach int) {
	for i := len(g.messages) - 1; i >= 0 && g.messages[i].Tick >= since; i-- {
		if m := g.messages[i]; m.Text == text && max(abs(m.X-x), abs(m.Y-y)) <= reach {
			return
		}
	}
	g.message(x, y, "%s", text)
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
// mark they now get on ripening (PLAY-017). markRipeHarvest leaves plants
// outside zones, unripe ones and ones already marked alone.
func (g *Game) migrateHarvestMarks() {
	for _, id := range g.plantIDs {
		if p := g.plants.Ref(id); p != nil && p.Ripe() { // read only
			g.markRipeHarvest(id)
		}
	}
}
