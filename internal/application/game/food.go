package game

import (
	"slices"

	"rimworld-go/internal/application/game/crops"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// SowWork is the labour to sow one cell of a growing zone.
const SowWork = 20

// CropID is the command value (Command.Build) for a crop name.
func CropID(name string) (uint8, bool) { return crops.ID(name) }

// ZoneSpeciesID is the DesignateGrowZone command value (Command.Build) for
// any species a growing zone can sow: crops and trees.
func ZoneSpeciesID(name string) (uint8, bool) { return crops.ZoneID(name) }

func (g *Game) addGrowZone(x1, y1, x2, y2 int, crop uint8) {
	s := plant.Species(crop)
	if !plant.DefOf(s).ZoneSowable() {
		s = plant.Potato
	}
	// plants on cells no zone holds yet: once painted, the ripe ones get
	// their harvest mark (a plant already in a zone keeps the mark, or the
	// cancel, it has)
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	fresh := g.unzonedPlantsIn(minX, minY, maxX, maxY) // zone_cmds.go
	g.paintZone(zone.KindGrow, zone.Zone{Crop: uint8(s)}, x1, y1, x2, y2)
	for _, id := range fresh {
		g.markRipeHarvest(id)
	}
}

// sowable reports whether a zone cell can be sown now: empty, fit for
// sowing and not in a zone whose sowing the player turned off (PLAY-020).
// The work giver, forced orders, the order menu and an under-way sow all
// ask it.
func (g *Game) sowable(x, y int) bool {
	if z, in := g.zoneAt(x, y); in && z.NoSow {
		return false
	}
	return g.sowableIgnoringPlant(x, y) && g.plantAt(x, y) == 0
}

// sowableIgnoringPlant is sowable for a cell that may still hold a plant:
// the season, soil, walkability and buildings allow sowing once the plant
// is gone (assignZoneClear cuts only on such cells).
func (g *Game) sowableIgnoringPlant(x, y int) bool {
	if basin, _ := g.basinAt(x, y); basin { // FOOD-2: a hydroponic crop on any ground (wiki: Hydroponics basin)
		z, in := g.zoneAt(x, y)
		return in && plant.Hydroponic(plant.Species(z.Crop)) && g.growingSeason(x, y) && g.layerFrameAt(x, y, construction.LayerObject) == 0
	}
	if g.terr.At(x, y).Fertility() <= 0 || !g.grid.Walkable(x, y) || !g.growingSeason(x, y) {
		return false
	}
	return !g.occupiedAt(x, y) && g.layerBldgAt(x, y, construction.LayerFloor) == 0 &&
		g.layerFrameAt(x, y, construction.LayerObject) == 0 && g.layerFrameAt(x, y, construction.LayerFloor) == 0
}

// zoneAt is the grow zone on tile (x, y).
func (g *Game) zoneAt(x, y int) (zone.Zone, bool) { return g.zoneOf(zone.KindGrow, x, y) }

// wrongForZone: plant p stands where zone z wants its own crop: it was not
// sown by a growing zone (wild plants, ground plants, bushes, trees), or it
// is a crop of another species.
func wrongForZone(p plant.Plant, z zone.Zone) bool {
	return !p.Zone || p.Species != plant.Species(z.Crop)
}

// assignZoneClear sends grower id to cut the nearest plant that stands in
// the way of a growing zone (PLAY-017): a wrong plant (wrongForZone) on a
// cell that could otherwise be sown now. A ripe one is left to the harvest
// (its mark, see markRipeHarvest); one another pawn has reserved, or this
// pawn has put off, is passed over. It is the ordinary cut job, so doCut is
// unchanged and the next assignSow sows the emptied cell. A zone whose
// cutting the player turned off (NoCut) keeps its wrong plants, and their
// cells stay unsown.
func (g *Game) assignZoneClear(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	var tried []entity.EntityID
	for range workTries { // SIM-027: the next nearest plant when one can't be started
		var best entity.EntityID
		bestD := 1 << 30
		g.zones.Each(func(z zone.Zone) bool {
			if z.Kind != zone.KindGrow || z.NoCut {
				return true
			}
			g.zones.EachCell(z.ID, func(x, y int) bool {
				d := pathing.Dist(pos.X, pos.Y, x, y)
				if d >= bestD {
					return true
				}
				pid := g.plantAt(x, y)
				if pid == 0 {
					return true
				}
				p := g.plants.Ref(pid) // read only: nothing below changes the store
				if p == nil || !wrongForZone(*p, z) || p.Harvestable() || !g.sowableIgnoringPlant(x, y) {
					return true
				}
				if _, taken := g.reserve.EntityOwner(pid); taken || slices.Contains(tried, pid) ||
					g.postponedFor(id, job.Cut, pid, 0, 0) {
					return true
				}
				best, bestD = pid, d
				return true
			})
			return true
		})
		if best == 0 {
			return false
		}
		p, _ := g.plants.Get(best)
		if !g.reachable(id, p.X, p.Y) {
			g.postpone(postKey{Pawn: id, Kind: job.Cut, Target: best}, job.Unreachable)
			tried = append(tried, best)
			continue
		}
		if g.startJob(id, job.Record{Kind: job.Cut, Target: best, TX: p.X, TY: p.Y, WorkLeft: plant.DefOf(p.Species).CutWork}) {
			return true
		}
		tried = append(tried, best)
	}
	return false
}

// sowWorkOf is the labour to sow one cell of zone z: its species' own.
func sowWorkOf(z zone.Zone) int {
	if w := plant.DefOf(plant.Species(z.Crop)).SowWork; w > 0 {
		return w
	}
	return SowWork
}

func (g *Game) assignSow(id entity.EntityID) bool {
	if g.assignZoneChop(id) {
		return true // grown zone trees are felled before their cells are resown
	}
	if g.assignZoneClear(id) {
		return true // wrong plants are cut before their cells are sown
	}
	pos, _ := g.positions.Get(id)
	var tried [][2]int
	for range workTries { // SIM-027: the next nearest cell when one can't be started
		bestD := 1 << 30
		var bx, by int
		found := false
		g.zones.Each(func(z zone.Zone) bool {
			if z.Kind != zone.KindGrow || z.NoSow || int(g.skillOf(id, skPlants).Level) < plant.DefOf(plant.Species(z.Crop)).MinSkill {
				return true // PLAY-020: sowing turned off; FOOD-2: the crop needs more Plants skill (wiki)
			}
			g.zones.EachCell(z.ID, func(x, y int) bool {
				d := pathing.Dist(pos.X, pos.Y, x, y)
				if d >= bestD || !g.sowable(x, y) {
					return true
				}
				if slices.Contains(tried, [2]int{x, y}) || g.postponedFor(id, job.Sow, 0, x, y) {
					return true
				}
				if _, taken := g.reserve.TileOwner(x, y); !taken {
					bestD, bx, by, found = d, x, y, true
				}
				return true
			})
			return true
		})
		if !found {
			return false
		}
		work := SowWork
		if z, ok := g.zoneAt(bx, by); ok {
			work = sowWorkOf(z)
		}
		if g.startJob(id, job.Record{Kind: job.Sow, TX: bx, TY: by, WorkLeft: work}) {
			return true
		}
		tried = append(tried, [2]int{bx, by})
	}
	return false
}

func (g *Game) doSow(id entity.EntityID, j job.Record) {
	z, ok := g.zoneAt(j.TX, j.TY)
	if !ok || !g.sowable(j.TX, j.TY) {
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skPlants, true) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	g.clearGrass(j.TX, j.TY) // grass.go: sowing clears it
	if pid, err := g.spawnPlant(plant.Species(z.Crop), j.TX, j.TY, 0); err == nil {
		p, _ := g.plants.Get(pid)
		p.Zone = true
		g.setPlant(pid, p)
		g.notePlantMark(pid) // jobindex.go
	}
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// markRipeHarvest gives plant id its harvest mark when it is ripe on a
// growing-zone cell (PLAY-017): zone crops are harvested through the
// mark, so the overlay shows and "Cancel harvest" holds until the next
// ripening.
func (g *Game) markRipeHarvest(id entity.EntityID) {
	p, ok := g.plants.Get(id)
	if !ok || p.Designated || !p.Harvestable() {
		return
	}
	if _, in := g.zoneAt(p.X, p.Y); !in {
		return
	}
	p.Designated = true
	g.setPlant(id, p)
	g.notePlantMark(id) // jobindex.go
}

// wantsHarvest: ripe food plants that carry a harvest mark. Zone crops get
// theirs when they ripen or when a zone is drawn over them
// (markRipeHarvest), so "Cancel harvest" cancels until the next ripening.
func wantsHarvest(p plant.Plant) bool {
	return p.HarvestableNow() && p.Designated // FOOD-2: from 65 % growth (wiki: Plants)
}

// bestFood picks the most preferred free food, nearest first; a
// malnourished pawn eats anything, carcasses too (wiki: Malnutrition).
func (g *Game) bestFood(id entity.EntityID) (entity.EntityID, int, int) {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestPref, bestD := -1, 1<<30
	var bx, by int
	corpses := body.Malnutrition(g.bodies[id]) > 0
	for _, iid := range g.itemIDs {
		st := g.stacks.Ref(iid) // read only: nothing below changes the store
		if st == nil || !st.Kind.IsFood() && !(corpses && animal.EdibleCorpse(*st)) || st.Qty <= 0 || st.Carrier != 0 || st.Forbidden {
			continue
		}
		if g.reserve.FreeFor(id, iid, st.Qty) <= 0 || !g.inArea(id, st.X, st.Y) {
			continue // RES-1: every portion claimed by other eaters; areas.go: outside the allowed area
		}
		pref := st.Kind.Preference()
		d := pathing.Dist(pos.X, pos.Y, st.X, st.Y)
		if pref > bestPref || pref == bestPref && (d < bestD || d == bestD && iid < best) {
			best, bestPref, bestD, bx, by = iid, pref, d, st.X, st.Y
		}
	}
	return best, bx, by
}

// claimJobTarget claims job rec's target for pawn id (RES-1): the part of
// a stack it takes (the portions it eats, a planned haul load, a bill's
// first ingredient) as a count claim, capped by what is unclaimed;
// anything else whole. A tend also claims its dose of medicine.
func claimJobTarget(g *Game, id entity.EntityID, rec job.Record) bool {
	if med := g.stacks.Ref(rec.Dest); med != nil && (rec.Kind == job.Tend || rec.Kind == job.EmergencyTend) && !g.reserve.TryCount(id, rec.Dest, 1, med.Qty) {
		return false // the one dose a tend fetches
	}
	st, n := g.stacks.Ref(rec.Target), 0
	switch {
	case st == nil:
	case rec.Kind == job.Eat:
		nd, _ := g.need.Get(id)
		if n = portionsFor(nd.Hunger, *st); rec.Count == bingeEat {
			n += 2 // a binge eats two past full
		}
	case rec.Kind == job.HaulIngredients && len(rec.Route) > 1:
		n = int(rec.Route[1])
	case rec.Kind == job.Bill || rec.Kind == job.Haul && (rec.Dest == 0 || len(rec.Route) > 0):
		n = rec.Count
	}
	if n <= 0 {
		return g.reserve.TryEntity(id, rec.Target)
	}
	return g.reserve.TryCount(id, rec.Target, min(n, g.reserve.FreeFor(id, rec.Target, st.Qty)), st.Qty)
}

// LastMeal is what pawn id last ate (for mood/inspector).
func (g *Game) lastMealKind(id entity.EntityID) (item.Kind, bool) {
	k, ok := g.lastMeal[id]
	return k, ok
}

// CropView describes a sowable crop for the UI (game/crops).
type CropView = crops.View

// Crops lists the sowable crops, fastest first.
func Crops() []CropView { return crops.All() }

// ZoneSpecies lists what a growing zone can sow: the crops, then trees.
func ZoneSpecies() []CropView { return crops.ZoneSpecies() }
