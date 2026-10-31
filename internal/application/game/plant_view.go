package game

import (
	"slices"

	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/world/plant"
)

// plantMemo is the view cache's last plant list (NG-1 §6.7, PERF-002).
// A PlantView is a function of the plant's state, its grow zone and (for
// crops) the air on its tile, so:
//
//   - while no plant was written (plantVer), no zone was edited (the zone
//     map's Ver) and no crop's Wilted flipped, the last list is handed out
//     again as it is;
//   - otherwise the list is rebuilt into scratch (a plant whose state
//     equals the one its last view was built from copies that view) and
//     compared with the last one: an equal list still hands out the old
//     slice, so the renderer's identity memos hold.
//
// views is handed out and never written again; scratch, src and spare are
// memo-private.
type plantMemo struct {
	views, scratch []PlantView
	src, spare     []plant.Plant // the plant state views[i] (src) / the next build (spare) is made from
	crops          []int32       // indexes into views of the crops (the Wilted re-check)
	ver, zoneVer   uint64        // plantVer and zones.Ver() of the build
	store          *component.Store[plant.Wild]
	zones          *zone.Map
	ok             bool
}

// plantViews is World().Plants: every plant's view in plantIDs order.
// With the view cache on it is shared and read only (plantMemo).
func (g *Game) plantViews() []PlantView {
	if !g.views.on {
		return g.appendPlantViews(make([]PlantView, 0, len(g.plantIDs)), nil)
	}
	m := &g.views.plants
	if m.ok && m.ver == g.plantVer && m.store == g.plants && m.zones == g.zones && m.zoneVer == g.zones.Ver() && !g.cropWiltFlipped(m) {
		if selfCheck() {
			g.checkPlantMemo(m)
		}
		return m.views
	}
	m.scratch = g.appendPlantViews(m.scratch[:0], m)
	m.ver, m.store, m.zones, m.zoneVer = g.plantVer, g.plants, g.zones, g.zones.Ver()
	if !m.ok || !slices.Equal(m.scratch, m.views) {
		m.views = make([]PlantView, len(m.scratch))
		copy(m.views, m.scratch)
	}
	m.ok = true
	return m.views
}

// appendPlantViews appends every plant's view to out. With a memo (m not
// nil) a plant whose state equals the one its last view was built from
// reuses that view, and the memo's src and crops are rebuilt for the new
// list; Wilted and Zone, which follow the air and the zones, are redone.
func (g *Game) appendPlantViews(out []PlantView, m *plantMemo) []PlantView {
	memo := m != nil && m.ok && len(m.src) == len(m.views)
	var src []plant.Plant
	if m != nil {
		src, m.crops = m.spare[:0], m.crops[:0]
	}
	j := 0 // the memo entry expected next (plantIDs keeps its order)
	for _, id := range g.plantIDs {
		p, ok := g.plants.Get(id)
		if !ok {
			continue
		}
		var pv PlantView
		if memo && j < len(m.views) && m.views[j].ID != id && j+1 < len(m.views) && m.views[j+1].ID == id {
			j++ // the plant before it is gone
		}
		if memo && j < len(m.views) && m.views[j].ID == id && m.src[j] == p {
			pv = m.views[j]
			pv.Wilted, pv.Zone = false, ""
			j++
		} else {
			if memo && j < len(m.views) && m.views[j].ID == id {
				j++
			}
			pv = PlantView{
				ID: id, X: p.X, Y: p.Y, Designated: p.Designated, Harvestable: p.Harvestable(),
				Species: p.Species.String(), Growth: p.Percent(), Stage: p.Stage(), Cut: p.Cut,
			}
			fillPlantLight(&pv, p, plantFertility(g, p)) // plant_light.go (MAPGEN-3, FOOD-2)
			fillPlantDamage(&pv, p)
		}
		g.fillPlantWeather(&pv, p) // weather.go
		pv.Zone = g.plantZoneName(p)
		if m != nil {
			if plant.DefOf(p.Species).Crop {
				m.crops = append(m.crops, int32(len(out)))
			}
			src = append(src, p)
		}
		out = append(out, pv)
	}
	if m != nil {
		m.spare, m.src = m.src, src
	}
	return out
}

// plantZoneName is PlantView.Zone: the crop of the grow zone plant p
// stands in, "" when none.
func (g *Game) plantZoneName(p plant.Plant) string {
	if z, ok := g.zoneAt(p.X, p.Y); ok {
		return plant.DefOf(plant.Species(z.Crop)).Name
	}
	return ""
}

// cropWiltFlipped reports a crop of the memo's list whose Wilted would
// read differently now (the air on its tile moved across its frost or
// growth limit, or a roof came or went). Only crops can wilt.
func (g *Game) cropWiltFlipped(m *plantMemo) bool {
	for _, i := range m.crops {
		var pv PlantView
		g.fillPlantWeather(&pv, m.src[i])
		if pv.Wilted != m.views[i].Wilted {
			return true
		}
	}
	return false
}

// checkPlantMemo (tests: selfCheck) proves the fast path right: every
// plant and zone still matches what the memo was built from.
func (g *Game) checkPlantMemo(m *plantMemo) {
	j := 0
	for _, id := range g.plantIDs {
		p, ok := g.plants.Get(id)
		if !ok {
			continue
		}
		if j >= len(m.src) || m.views[j].ID != id || m.src[j] != p {
			panic("plantViews: plant changed without a plantVer bump")
		}
		if g.plantZoneName(p) != m.views[j].Zone {
			panic("plantViews: zone changed without a zone Ver bump")
		}
		j++
	}
	if j != len(m.src) {
		panic("plantViews: plant changed without a plantVer bump")
	}
}

// fillPlantDamage sets PlantView.Damage and DamageKind (PLAY-016): cold or
// heat damage, "" while unhurt.
func fillPlantDamage(pv *PlantView, p plant.Plant) {
	pv.Damage = int(p.Damage)
	switch {
	case p.Damage == 0:
	case p.DamageHeat:
		pv.DamageKind = "heat"
	default:
		pv.DamageKind = "cold"
	}
}
