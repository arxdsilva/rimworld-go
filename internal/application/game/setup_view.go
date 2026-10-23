package game

import (
	"rimworld-go/internal/domain/building/construction"
)

// NG-1 read-model additions of a setup colony (SETUP).

// fillSetupView adds the arrival pods, permanent conditions, extreme
// weather and the map's own season to the world view.
func (g *Game) fillSetupView(w *WorldView) {
	if !g.hasSetup() {
		return
	}
	now := uint64(g.clock.Now())
	w.Arrivals = g.arrivalViews(now)
	for _, c := range g.ng.fx.Conditions {
		w.Conditions = append(w.Conditions, c.Key())
	}
	w.Extreme = g.extremeKey()
	w.Season = g.season(now)
}

// Buildables is the catalog for this colony: Buildables() with the
// scenario's disallowed buildings Locked.
func (g *Game) Buildables() []BuildableView {
	out := Buildables()
	if !g.hasSetup() {
		return out
	}
	for i := range out {
		if g.scenarioForbids(construction.Kind(out[i].ID)) {
			out[i].Locked = ReasonScenario
		}
	}
	return out
}
