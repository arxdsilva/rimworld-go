package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// SowWork is the labour to sow one cell of a growing zone.
const SowWork = 20

// GrowZoneView is a growing zone in the read model.
type GrowZoneView struct {
	MinX, MinY, MaxX, MaxY int
	Crop                   string
}

// CropID is the command value (Command.Build) for a crop name.
func CropID(name string) (uint8, bool) {
	s, ok := plant.Parse(name)
	if !ok || !plant.DefOf(s).Crop {
		return 0, false
	}
	return uint8(s), true
}

func (g *Game) addGrowZone(x1, y1, x2, y2 int, crop uint8) {
	s := plant.Species(crop)
	if !plant.DefOf(s).Crop {
		s = plant.Amberroot
	}
	minX, minY, maxX, maxY := normalizeRect(x1, y1, x2, y2)
	g.growZones = append(g.growZones, zone.GrowZone{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY, Crop: uint8(s)})
}

func (g *Game) cancelGrowZones(minX, minY, maxX, maxY int) {
	kept := g.growZones[:0]
	for _, z := range g.growZones {
		if z.MaxX < minX || z.MinX > maxX || z.MaxY < minY || z.MinY > maxY {
			kept = append(kept, z)
		}
	}
	g.growZones = kept
}

// sowable reports whether a zone cell can be sown now.
func (g *Game) sowable(x, y int) bool {
	if g.terr.At(x, y).Fertility() <= 0 || !g.grid.Walkable(x, y) {
		return false
	}
	if g.plantAt(x, y) != 0 {
		return false
	}
	if g.layerBldgAt(x, y, construction.LayerObject) != 0 || g.layerBldgAt(x, y, construction.LayerFloor) != 0 ||
		g.layerFrameAt(x, y, construction.LayerObject) != 0 || g.layerFrameAt(x, y, construction.LayerFloor) != 0 {
		return false
	}
	return true
}

func (g *Game) zoneAt(x, y int) (zone.GrowZone, bool) {
	for _, z := range g.growZones {
		if z.Contains(x, y) {
			return z, true
		}
	}
	return zone.GrowZone{}, false
}

func (g *Game) assignSow(id entity.EntityID) bool {
	pos, _ := g.positions.Get(id)
	bestD := 1 << 30
	var bx, by int
	found := false
	for _, z := range g.growZones {
		for y := z.MinY; y <= z.MaxY; y++ {
			for x := z.MinX; x <= z.MaxX; x++ {
				if !g.sowable(x, y) {
					continue
				}
				if _, taken := g.reserve.TileOwner(x, y); taken {
					continue
				}
				if d := pathing.Dist(pos.X, pos.Y, x, y); d < bestD {
					bestD, bx, by, found = d, x, y, true
				}
			}
		}
	}
	if !found {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Sow, TX: bx, TY: by, WorkLeft: SowWork})
}

func (g *Game) doSow(id entity.EntityID, j job.Record) {
	z, ok := g.zoneAt(j.TX, j.TY)
	if !ok || !g.sowable(j.TX, j.TY) {
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
	if pid, err := g.spawnPlant(plant.Species(z.Crop), j.TX, j.TY, 0); err == nil {
		p, _ := g.plants.Get(pid)
		p.Zone = true
		g.plants.Set(pid, p)
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
		if !ok || !st.Kind.IsFood() || st.Qty <= 0 || st.Carrier != 0 {
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

func (g *Game) growZoneViews() []GrowZoneView {
	out := make([]GrowZoneView, len(g.growZones))
	for i, z := range g.growZones {
		out[i] = GrowZoneView{z.MinX, z.MinY, z.MaxX, z.MaxY, plant.Species(z.Crop).String()}
	}
	return out
}

// LastMeal is what pawn id last ate (for mood/inspector).
func (g *Game) lastMealKind(id entity.EntityID) (item.Kind, bool) {
	k, ok := g.lastMeal[id]
	return k, ok
}
