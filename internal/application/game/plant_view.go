package game

import (
	"rimworld-go/internal/domain/world/plant"
)

// plantMemo is the view cache's last plant list (NG-1 §6.7): the views it
// handed out and the plant state each was built from. Plants grow a few
// chunks per tick, so most of a big map's plants are unchanged between
// two builds and copy their entry instead of deriving it again.
type plantMemo struct {
	views []PlantView   // the last World().Plants (shared, never written)
	src   []plant.Plant // the state views[i] was built from
	spare []plant.Plant // the other src buffer (memo-only, reused)
}

// plantViews appends every plant's view to out (World().Plants, in
// plantIDs order). With the view cache on, a plant whose state equals the
// one its last view was built from reuses that view; Wilted, Zone,
// GrowState and RipeIn, which follow the weather, the zones, the hour and
// the lamps, are always redone.
func (g *Game) plantViews(out []PlantView) []PlantView {
	m := &g.views.plants
	memo := g.views.on && len(m.src) == len(m.views)
	var src []plant.Plant
	if g.views.on {
		src = m.spare[:0]
	}
	now := g.growNowOf()
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
			fillPlantLight(&pv, p) // plant_light.go (MAPGEN-3)
			fillPlantDamage(&pv, p)
		}
		g.fillPlantWeather(&pv, p)     // weather.go
		g.fillPlantGrowth(&pv, p, now) // plant_light.go: follows the hour and the lamps (PLAY-017)
		if z, ok := g.zoneAt(p.X, p.Y); ok {
			pv.Zone = plant.DefOf(plant.Species(z.Crop)).Name
		}
		out = append(out, pv)
		if g.views.on {
			src = append(src, p)
		}
	}
	if g.views.on {
		m.spare, m.src, m.views = m.src, src, out
	}
	return out
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
