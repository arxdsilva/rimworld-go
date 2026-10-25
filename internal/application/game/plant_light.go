package game

import (
	"strings"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Plant light rules in the growth pass (MAPGEN-3). A plant with MinLight
// grows only on a cell lit at least that much (ordinary moss: daylight
// or lamplight, never a dark cave). A plant with DiesInLight (duskmoss)
// withers in that much light, losing a share of its full growth each
// window it stands in it, and is removed at nothing; once withering it
// recovers only below plant.WitherStopLight (hysteresis). Species with
// neither rule never read the light: their growth is as before.

// witherNoticeRange and witherNoticeTicks: one wither notice per patch —
// a notice within this range (Chebyshev) and time covers a new death.
const (
	witherNoticeRange = 12
	witherNoticeTicks = clock.TicksPerDay
)

// lightGrows applies plant id's light rules for the window that ends now
// (p is updated in place) and reports whether it may grow this window.
// A plant withered to nothing is deleted and added to fp.gone, like a
// frost kill.
func (g *Game) lightGrows(id entity.EntityID, p *plant.Plant, fp *frostPass) bool {
	d := plant.DefOf(p.Species)
	if d.MinLight <= 0 && d.DiesInLight <= 0 {
		return true
	}
	light := g.cellLightPermil(p.X, p.Y)
	if plant.Harmed(p.Species, p.Withering, light) {
		p.Withering = true
		p.Growth -= plant.WitherLoss(p.Species, climateInterval)
		if p.Growth > 0 {
			g.plants.Set(id, *p)
			return false
		}
		g.witherAway(id, *p, fp)
		return false
	}
	if p.Withering {
		p.Withering = false
		g.plants.Set(id, *p)
	}
	return plant.LitEnough(p.Species, light)
}

// witherAway removes a plant withered to nothing (the list is compacted
// by endFrostPass) and posts one notice per patch.
func (g *Game) witherAway(id entity.EntityID, p plant.Plant, fp *frostPass) {
	g.plants.Delete(id)
	g.plantsChanged()
	g.plantLightChanged(p.Species)
	if fp.gone == nil {
		fp.gone = map[entity.EntityID]bool{}
	}
	fp.gone[id] = true
	text := witherText(p.Species)
	now := uint64(g.clock.Now())
	for i := len(g.messages) - 1; i >= 0 && g.messages[i].Tick+witherNoticeTicks > now; i-- {
		m := g.messages[i]
		if m.Text == text && max(abs(m.X-p.X), abs(m.Y-p.Y)) <= witherNoticeRange {
			return
		}
	}
	g.message(p.X, p.Y, "%s", text)
}

// witherText is the notice for a plant of species s withered by light.
func witherText(s plant.Species) string {
	name := strings.ReplaceAll(plantLabel(s), "_", " ")
	if name == "" {
		return "A plant withered in the light"
	}
	return strings.ToUpper(name[:1]) + name[1:] + " withered in the light"
}

// fillPlantLight sets the view's light needs for the inspector ("Needs
// darkness" / "Needs light", "Withering in the light").
func fillPlantLight(pv *PlantView, p plant.Plant) {
	d := plant.DefOf(p.Species)
	pv.NeedsDark, pv.NeedsLight, pv.Withering = d.DiesInLight > 0, d.MinLight > 0, p.Withering
}
