package game

import (
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// PLAY-020 zone management, RimWorld-style. Every handler ignores an
// unknown zone id and a zone of a kind it does not apply to. Settings
// that do not affect merging (sowing, cutting, hidden, name) go through
// zone.Map.Update; the crop through zone.Map.Set, which may merge a zone
// into a touching one of the same crop (the lower id then survives with
// its own settings).

// ZoneNameMax is the longest zone name, in characters.
const ZoneNameMax = 24

// growZoneByID is grow zone id, when there is one.
func (g *Game) growZoneByID(id uint16) (zone.Zone, bool) {
	z, ok := g.zones.Zone(zone.ID(id))
	return z, ok && z.Kind == zone.KindGrow
}

// cmdSetZoneSow allows (on) or stops sowing in zone id. Sowing off covers
// every way a grower sows (sowable): the work giver, forced orders and the
// order menu; a sow already under way in the zone is cancelled. Harvests
// and the ripe mark go on.
func (g *Game) cmdSetZoneSow(id uint16, on bool) {
	z, ok := g.growZoneByID(id)
	if !ok || z.NoSow == !on {
		return
	}
	g.zones.Update(z.ID, func(z *zone.Zone) { z.NoSow = !on })
	if !on {
		g.cancelZoneJobs(z.ID, func(j job.Record) bool { return j.Kind == job.Sow })
	}
}

// cmdSetZoneCut allows (on) or stops growers cutting in zone id: no
// clearing of wrong plants (assignZoneClear) and no chopping of the zone's
// grown trees (assignZoneChop); a grower's cut already under way there is
// cancelled. Plants the player designated for cutting, and the harvest of
// the zone's own ripe crop, are not affected.
func (g *Game) cmdSetZoneCut(id uint16, on bool) {
	z, ok := g.growZoneByID(id)
	if !ok || z.NoCut == !on {
		return
	}
	g.zones.Update(z.ID, func(z *zone.Zone) { z.NoCut = !on })
	if !on {
		g.cancelZoneJobs(z.ID, func(j job.Record) bool {
			if j.Kind != job.Cut || j.Forced || j.Target == 0 {
				return false
			}
			p, ok := g.plants.Get(j.Target)
			return ok && !p.Cut // a plant the player marked for cutting stays marked
		})
	}
}

// cancelZoneJobs interrupts every pawn whose job on a cell of zone id
// matches match.
func (g *Game) cancelZoneJobs(id zone.ID, match func(job.Record) bool) {
	for _, pid := range g.pawnIDs {
		j, ok := g.jobs.Get(pid)
		if !ok || j.Kind == job.None || j.State == job.Idle || j.State == job.Complete || !match(j) {
			continue
		}
		if at, in := g.zones.At(j.TX, j.TY); in && at == id {
			g.interrupt(pid, job.Cancelled)
		}
	}
}

// cmdSetZoneHidden hides (on) or shows zone id's overlay.
func (g *Game) cmdSetZoneHidden(id uint16, on bool) {
	g.zones.Update(zone.ID(id), func(z *zone.Zone) { z.Hidden = on })
}

// cmdRenameZone names zone id: trimmed, at most ZoneNameMax characters
// ("" = the default name).
func (g *Game) cmdRenameZone(id uint16, name string) {
	name = zoneName(name)
	g.zones.Update(zone.ID(id), func(z *zone.Zone) { z.Name = name })
}

// zoneName trims name and cuts it to ZoneNameMax characters.
func zoneName(name string) string {
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > ZoneNameMax {
		name = strings.TrimSpace(string(r[:ZoneNameMax]))
	}
	return name
}

// cmdSetZoneCrop makes grow zone id grow species (a crop or tree growing
// zones can sow, and research has unlocked). The plants of the old crop
// are then harvested when ripe, cut otherwise and the cells resown, by the
// ordinary work of PLAY-017 (both toggles allowing).
func (g *Game) cmdSetZoneCrop(id uint16, species uint8) {
	z, ok := g.growZoneByID(id)
	s := plant.Species(species)
	if !ok || z.Crop == species || !plant.DefOf(s).ZoneSowable() || g.cropLocked(s) {
		return
	}
	z.Crop = species
	g.zones.Set(z.ID, z)
}

// cmdExpandZone adds the valid cells of x0..x1, y0..y1 to zone id: free
// and fit for the zone's kind (as when painting one). Cells that don't
// touch the zone become a new zone with its settings; cells touching
// another zone of the same kind and crop join that one.
func (g *Game) cmdExpandZone(id uint16, x0, y0, x1, y1 int) {
	z, ok := g.zones.Zone(zone.ID(id))
	if !ok {
		return
	}
	minX, minY, maxX, maxY := zone.Normalize(x0, y0, x1, y1)
	minX, minY = max(minX, 0), max(minY, 0)
	maxX, maxY = min(maxX, g.Width()-1), min(maxY, g.Height()-1)
	var cells [][2]int
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if _, taken := g.zones.At(x, y); !taken && g.zoneable(z.Kind, x, y) {
				cells = append(cells, [2]int{x, y})
			}
		}
	}
	if len(cells) == 0 {
		return
	}
	fresh := g.unzonedPlantsIn(minX, minY, maxX, maxY)
	g.zones.AddCells(z.ID, cells)
	for _, pid := range fresh {
		g.markRipeHarvest(pid) // food.go: ripe plants the zone now covers get their harvest mark
	}
	g.autoHomeRect(minX, minY, maxX, maxY) // home.go, as for a painted zone
}

// unzonedPlantsIn lists the plants on x1,y1..x2,y2 that no zone holds yet.
func (g *Game) unzonedPlantsIn(minX, minY, maxX, maxY int) []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.plantIDs {
		p := g.plants.Ref(id) // read only
		if p == nil || p.X < minX || p.X > maxX || p.Y < minY || p.Y > maxY {
			continue
		}
		if _, zoned := g.zones.At(p.X, p.Y); !zoned {
			out = append(out, id)
		}
	}
	return out
}

// cmdRemoveZoneCells removes zone id's cells in x0..x1, y0..y1 (other
// zones' cells there stay). A zone cut in pieces splits, the biggest piece
// keeping the id; an emptied zone is deleted.
func (g *Game) cmdRemoveZoneCells(id uint16, x0, y0, x1, y1 int) {
	g.zones.RemoveCells(zone.ID(id), zone.RectOf(x0, y0, x1, y1))
}

// cmdDeleteZone removes zone id. Its plants stay.
func (g *Game) cmdDeleteZone(id uint16) {
	g.zones.Remove(zone.ID(id))
}
