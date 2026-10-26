package game

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Bleeding, blood loss and capacity downing (MED-01). Every open cut,
// bite, scratch or surgical wound bleeds until a doctor tends it
// (body/bleed.go); once an hour the medical phase walks the patients index
// (patients.go), never every body: blood loss grows by the hour's
// bleeding (or drains back when nothing bleeds), kills at 1000, and the
// pawn is downed or stood up again by its capacities with hysteresis
// (health.BodyDowns / BodyRecovered). Colonists, prisoners and raiders
// bleed alike: a downed raider no longer dies on a timer, it bleeds out
// unless captured and tended.

// phaseMedical runs the hourly medical work: blood loss and downing, the
// infections (infection.go), then the patients sweep, so right after it
// the patients index is exactly what a scan of the bodies would give.
func (g *Game) phaseMedical() {
	now := uint64(g.clock.Now())
	if now%clock.TicksPerHour != 0 {
		return
	}
	g.stepBloodLoss()
	g.stepInfections(now)
	g.sweepPatients() // patients.go (MED-00): the well and the dead leave the index
}

// stepBloodLoss advances an hour of bleeding on every patient in id
// order: blood loss, death from it, then the capacity downing rule.
func (g *Game) stepBloodLoss() {
	if len(g.med.patients) == 0 {
		return
	}
	for _, id := range slices.Clone(g.med.patients) { // a death may touch the index
		b := g.bodies[id]
		v, ok := g.vital.Get(id)
		if b == nil || !ok || v.Status == health.Dead {
			continue
		}
		if b.BleedPerDay() == 0 && b.BloodLoss == 0 && b.BloodStage == 0 {
			continue // a tended or bruised patient: nothing to step
		}
		b.StepBlood(1)
		if b.BloodLoss >= body.BloodLossDeath {
			pos, _ := g.positions.Get(id)
			g.message(pos.X, pos.Y, "%s bled to death.", g.name(id))
			g.die(id, health.CauseBloodLoss)
			g.clearPatient(id)
			continue
		}
		g.bodyChanged(id, b, &v) // body.go: stats follow consciousness
		g.updateDowned(id, b, &v)
	}
}

// painShockAt is the pain (permille) that downs id: health.PainShock,
// raised or lowered by the pawn's pain tolerance (percent).
func (g *Game) painShockAt(id entity.EntityID) int32 {
	if pt := g.statOf(id, stat.PainTolerance); pt != 0 {
		return max(health.PainShock*(100+pt)/100, 1)
	}
	return health.PainShock
}

// downsNow reports pawn id, with record v and body b, incapacitated: by
// its capacities when it has a body, else by the legacy injury threshold.
func (g *Game) downsNow(id entity.EntityID, v health.Record, b *body.Body) bool {
	if b == nil {
		return v.Injury >= health.DownedInjury
	}
	return health.BodyDowns(b.Capacity(body.Consciousness), b.Capacity(body.Moving), b.Pain(), g.painShockAt(id))
}

// standsUp reports downed pawn id able to stand again: its capacities
// back past the recover values (the hysteresis band), or with no body,
// its injury under the legacy recover threshold.
func (g *Game) standsUp(id entity.EntityID, v health.Record) bool {
	b := g.bodies[id]
	if b == nil {
		return v.Injury <= health.RecoverInjury
	}
	return health.BodyRecovered(b.Capacity(body.Consciousness), b.Capacity(body.Moving), b.Pain(), g.painShockAt(id))
}

// updateDowned applies the downing rule to id after a change to its body:
// a standing pawn that is incapacitated goes down; one lying from its
// wounds stands once it has recovered past the band.
func (g *Game) updateDowned(id entity.EntityID, b *body.Body, v *health.Record) {
	switch {
	case v.Status == health.Alive && g.downsNow(id, *v, b):
		g.down(id, health.CauseInjury)
		*v, _ = g.vital.Get(id)
	case v.Status == health.Downed && v.Cause == health.CauseInjury && g.standsUp(id, *v):
		v.Status, v.Cause = health.Alive, health.CauseNone
		g.vital.Set(id, *v)
	}
}

// bleedingNow reports living humanlike id losing blood from an open
// wound.
func (g *Game) bleedingNow(id entity.EntityID) bool {
	b := g.bodies[id]
	if b == nil || b.BleedPerDay() == 0 {
		return false
	}
	v, ok := g.vital.Get(id)
	return ok && v.Status != health.Dead
}

// bleedDeathHours is how many hours id has before bleeding out at its
// current rate (-1: not bleeding).
func (g *Game) bleedDeathHours(id entity.EntityID) int {
	b := g.bodies[id]
	if b == nil {
		return -1
	}
	return b.BleedDeathHours()
}

// healthState is the Health tab's headline for a pawn with record v and
// body b: dead, bleeding (with its rate and the time left), downed,
// injured (open wounds or blood still to make back), or healthy.
func healthState(v health.Record, b *body.Body) string {
	bleed := b.BleedPerDay()
	switch {
	case v.Status == health.Dead:
		return "Dead"
	case bleed > 0:
		return fmt.Sprintf("Bleeding %d%%/day · death in %dh", bleed/10, b.BleedDeathHours())
	case v.Status == health.Downed:
		return "Downed"
	case hasOpenWound(b) || b.BloodLoss > 0:
		return "Injured"
	}
	return "Healthy"
}
