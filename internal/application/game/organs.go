package game

import (
	"slices"

	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Organs (MED-05). Blood pumping, blood filtration, breathing and
// digestion are capacities a body cannot live without: one at 0 is an
// organ failure (body.OrganFailure). A hit never destroys the last such
// organ (body.LossKills: woundBody spills the damage on, so death from
// wounds still comes at health.MaxHP); a botched operation that does
// kills on the table through body.VitalLost; the hourly medical phase
// catches a failure from anything else (a condition, an older save) and
// kills with health.CauseOrganFailure. Filtration also slows immunity
// (one kidney: 75 %), feeding the infection race.

// organImmunityGain is id's immunity gain factor from its organs
// (permille; 1000 without a body): blood filtration weighs on it.
func (g *Game) organImmunityGain(id entity.EntityID) int32 {
	if b := g.bodies[id]; b != nil {
		return b.ImmunityGain()
	}
	return 1000
}

// stepOrganFailure kills every living patient whose body has an organ
// failure (hourly, in id order over the patients index; no scan). It
// first wears anaesthetics off (MED-06): the first hourly pass over the
// patients.
func (g *Game) stepOrganFailure() {
	if len(g.med.patients) == 0 {
		return
	}
	g.stepAnesthetics(uint64(g.clock.Now())) // surgery.go (MED-06): wear anaesthetics off first

	for _, id := range slices.Clone(g.med.patients) { // a death touches the index
		b := g.bodies[id]
		v, ok := g.vital.Get(id)
		if b == nil || !ok || v.Status == health.Dead || !b.OrganFailure() {
			continue
		}
		pos, _ := g.positions.Get(id)
		g.message(pos.X, pos.Y, "%s died of organ failure.", g.name(id))
		g.dieMedical(id, health.CauseOrganFailure) // caravan_medical.go
		g.clearPatient(id)
	}
}
