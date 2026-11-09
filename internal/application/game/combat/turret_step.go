package combat

import (
	"cmp"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Stepping clear of a sparking turret (EAI-012; wiki: Mini-turret, a
// turret about to explode is run from). An undrafted colonist inside the
// blast of a turret whose fuse burns walks out to reach+2 tiles off it
// (Guard.Defend, before any threat). In the band between the blast and
// reach+2 it is not sent off again: it carries on with what it was doing
// clear of the blast, drops a job that leads back into it, and an idle one
// starts nothing new there, so work beside the turret cannot walk it back
// in and out (TestTurretDangerStepNoFlap). Raiders
// step out through Tactics.Danger (raid/tactics.go dodge).

// StepOut walks colonist id standing on (x, y) with job j clear of a
// sparking turret, or holds it in the band; it reports whether it did
// either (Defend then does nothing else this tick).
func (gd *Guard) StepOut(id entity.EntityID, j job.Record, x, y int) bool {
	if gd.st == nil || gd.tiles == nil || len(gd.st.turrets.sparks) == 0 {
		return false
	}
	t := gd.st.turrets
	lx, ly, reach, in, ok := t.sparkNear(x, y)
	if !ok {
		return false
	}
	walking := j.Kind == job.Move && j.Active()
	if !in {
		if j.Active() {
			if _, _, _, bad := t.sparkAt(j.TX, j.TY); !bad {
				return walking // on its way out, or at work clear of the blast
			}
			gd.h.Interrupt(id, job.Interrupted) // nothing takes it back in
		}
		return true // idle in the band: it holds
	}
	if walking {
		if _, _, _, bad := t.sparkAt(j.TX, j.TY); !bad {
			return true // already on its way out
		}
	}
	dx, dy := cmp.Compare(x, lx), cmp.Compare(y, ly)
	if dx == 0 && dy == 0 {
		dx = 1
	}
	out := reach + 2
	for _, v := range [...][2]int{{dx, dy}, {dx - dy, dx + dy}, {dx + dy, dy - dx}, {-dy, dx}, {dy, -dx}} {
		vx, vy := cmp.Compare(v[0], 0), cmp.Compare(v[1], 0)
		tx, ty, ok := gd.tiles.WalkableNear(lx+vx*out, ly+vy*out, 1)
		if _, _, _, bad := t.sparkAt(tx, ty); !ok || bad {
			continue
		}
		if j.Active() {
			gd.h.Interrupt(id, job.Interrupted)
		}
		if gd.h.StartJob(id, job.Record{Kind: job.Move, TX: tx, TY: ty}) {
			return true
		}
	}
	return false
}
