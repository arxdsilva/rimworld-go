package game

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
)

// SowWork is the labour to sow one cell of a growing zone.
const SowWork = 20

// CropID is the command value (Command.Build) for a crop name.
func CropID(name string) (uint8, bool) {
	s, ok := plant.Parse(name)
	if !ok || !plant.DefOf(s).Crop {
		return 0, false
	}
	return uint8(s), true
}

// ZoneSpeciesID is the DesignateGrowZone command value (Command.Build) for
// any species a growing zone can sow: crops and trees.
func ZoneSpeciesID(name string) (uint8, bool) {
	s, ok := plant.Parse(name)
	if !ok || !plant.DefOf(s).ZoneSowable() {
		return 0, false
	}
	return uint8(s), true
}

func (g *Game) addGrowZone(x1, y1, x2, y2 int, crop uint8) {
	s := plant.Species(crop)
	if !plant.DefOf(s).ZoneSowable() {
		s = plant.Potato
	}
	g.paintZone(zone.KindGrow, zone.Zone{Crop: uint8(s)}, x1, y1, x2, y2)
}

// sowable reports whether a zone cell can be sown now.
func (g *Game) sowable(x, y int) bool {
	if g.terr.At(x, y).Fertility() <= 0 || !g.grid.Walkable(x, y) {
		return false
	}
	if g.plantAt(x, y) != 0 || !g.growingSeason(x, y) {
		return false
	}
	if g.layerBldgAt(x, y, construction.LayerObject) != 0 || g.layerBldgAt(x, y, construction.LayerFloor) != 0 ||
		g.layerFrameAt(x, y, construction.LayerObject) != 0 || g.layerFrameAt(x, y, construction.LayerFloor) != 0 {
		return false
	}
	return true
}

// zoneAt is the grow zone on tile (x, y).
func (g *Game) zoneAt(x, y int) (zone.Zone, bool) { return g.zoneOf(zone.KindGrow, x, y) }

func (g *Game) assignSow(id entity.EntityID) bool {
	if g.assignZoneChop(id) {
		return true // grown zone trees are felled before their cells are resown
	}
	pos, _ := g.positions.Get(id)
	var tried [][2]int
	for range workTries { // SIM-027: the next nearest cell when one can't be started
		bestD := 1 << 30
		var bx, by int
		found := false
		g.zones.Each(func(z zone.Zone) bool {
			if z.Kind != zone.KindGrow {
				return true
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
		if z, ok := g.zoneAt(bx, by); ok && plant.DefOf(plant.Species(z.Crop)).SowWork > 0 {
			work = plant.DefOf(plant.Species(z.Crop)).SowWork
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
		g.plants.Set(pid, p)
		g.notePlantMark(pid) // jobindex.go
	}
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// wantsHarvest: ripe food plants that are designated or zone crops.
func wantsHarvest(p plant.Plant) bool {
	return p.Harvestable() && (p.Designated || p.Zone)
}

// bestFood picks the most preferred free food, nearest first.
func (g *Game) bestFood(id entity.EntityID) (entity.EntityID, int, int) {
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestPref, bestD := -1, 1<<30
	var bx, by int
	for _, iid := range g.itemIDs {
		st, ok := g.stacks.Get(iid)
		if !ok || !st.Kind.IsFood() || st.Qty <= 0 || st.Carrier != 0 || st.Forbidden {
			continue
		}
		if owner, taken := g.reserve.EntityOwner(iid); taken && owner != id {
			continue
		}
		pref := st.Kind.Preference()
		d := pathing.Dist(pos.X, pos.Y, st.X, st.Y)
		if pref > bestPref || pref == bestPref && (d < bestD || d == bestD && iid < best) {
			best, bestPref, bestD, bx, by = iid, pref, d, st.X, st.Y
		}
	}
	return best, bx, by
}

// LastMeal is what pawn id last ate (for mood/inspector).
func (g *Game) lastMealKind(id entity.EntityID) (item.Kind, bool) {
	k, ok := g.lastMeal[id]
	return k, ok
}

// CropView describes a sowable crop for the UI (crop picker, hover).
type CropView struct {
	Name      string  // species name (zone crop / CropID key)
	Label     string  // produce name
	Days      float64 // days to harvest on plain soil, daylight growth
	GravelDay float64 // days on poor (gravel) soil
	RichDays  float64 // days on rich soil
	Yield     int     // items per plant
	Nutrition int     // per item (a colonist eats about 5000 a day)
	Sens      int     // soil sensitivity, percent
	// FrostKill is the air temperature (tenths of a degree C) that kills
	// the crop; ShelfDays how long the produce keeps at room temperature
	// (0 = forever). Days assume growing weather (10 C or warmer).
	FrostKill int
	ShelfDays float64
	// Food: the produce is edible. Extra and ExtraQty: a by-product per
	// harvest ("straw", 4) (SIM-021).
	Food     bool
	Extra    string
	ExtraQty int
}

// Crops lists the sowable crops, fastest first.
func Crops() []CropView {
	var out []CropView
	for _, s := range plant.Crops() {
		d := plant.DefOf(s)
		out = append(out, CropView{
			Name: d.Name, Label: d.Label, Days: plant.GrowDays(s, terrain.Soil.Fertility()),
			GravelDay: plant.GrowDays(s, terrain.Gravel.Fertility()), RichDays: plant.GrowDays(s, terrain.RichSoil.Fertility()),
			Yield: d.Yield, Nutrition: d.Yields.Nutrition(), Sens: d.Sens,
			FrostKill: d.FrostKill, ShelfDays: float64(d.Yields.ShelfTicks()) / clock.TicksPerDay,
			Food: d.Yields.Nutrition() > 0,
		})
		if d.ExtraQty > 0 {
			out[len(out)-1].Extra, out[len(out)-1].ExtraQty = d.Extra.String(), d.ExtraQty
		}
	}
	return out
}

// ZoneSpecies lists what a growing zone can sow: the crops, then trees.
func ZoneSpecies() []CropView {
	out := Crops()
	for _, s := range []plant.Species{plant.Tree, plant.Pine, plant.JungleTree} { // tree zones (SIM-023)
		d := plant.DefOf(s)
		out = append(out, CropView{
			Name: d.Name, Label: "wood", Days: plant.GrowDays(s, terrain.Soil.Fertility()),
			GravelDay: plant.GrowDays(s, terrain.Gravel.Fertility()),
			RichDays:  plant.GrowDays(s, terrain.RichSoil.Fertility()),
			Yield:     d.Wood, Sens: d.Sens,
		})
	}
	return out
}
