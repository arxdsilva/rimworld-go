package ebiten

import (
	"math"

	"rimworld-go/internal/application/game"
)

// Combat presentation (ART §27.2): attack and hit reactions, combat
// particles, aiming and firing poses, falls. Owned by the combat
// animation work; the dispatch in pawn_anim.go calls in here.

// --- reactions from attack traces ---------------------------------------

// minReactionFrames is how long a recoil, lunge or flash stays visible in
// present frames, however fast the simulation runs (ART §27 clocks).
const minReactionFrames = 8

// reactTrace is a trace as the reaction sees it: sim is its age in sim
// ticks (+Inf once the sim dropped it) and frames its age in present frames.
type reactTrace struct {
	game.TraceView
	sim, frames float64
}

type reactRec struct {
	trace game.TraceView
	seen  uint64 // present frame first seen
	live  bool
}

// reactionAge is the age a reaction of duration dur ticks is shown at: the
// sim age, but never advancing faster than dur over minReactionFrames.
func reactionAge(sim, frames, dur float64) float64 {
	return math.Min(sim, frames*dur/minReactionFrames)
}

// attackReaction offsets an actor at (x, y): rifle recoil, melee lunge,
// and knockback plus a flash when struck, timed by sim age alone.
func attackReaction(traces []game.TraceView, x, y int) (dx, dy float64, flash float32) {
	rs := make([]reactTrace, len(traces))
	for i, t := range traces {
		rs[i] = reactTrace{TraceView: t, sim: float64(t.Age), frames: math.Inf(1)}
	}
	return attackReactionF(rs, x, y)
}

// attackReactionF is attackReaction with the minimum on-screen duration.
func attackReactionF(traces []reactTrace, x, y int) (dx, dy float64, flash float32) {
	for _, t := range traces {
		if t.FromX == x && t.FromY == y && (t.ToX != x || t.ToY != y) {
			ux, uy := unit(float64(t.ToX-x), float64(t.ToY-y))
			switch {
			case t.Melee && reactionAge(t.sim, t.frames, 10) < 10:
				k := math.Sin(math.Pi * reactionAge(t.sim, t.frames, 10) / 10)
				dx, dy = dx+ux*0.22*k, dy+uy*0.22*k
			case !t.Melee && reactionAge(t.sim, t.frames, 6) < 6:
				k := 1 - reactionAge(t.sim, t.frames, 6)/6
				dx, dy = dx-ux*0.08*k, dy-uy*0.08*k
			}
		}
		if t.ToX == x && t.ToY == y && t.Hit && (t.FromX != x || t.FromY != y) {
			if age := reactionAge(t.sim, t.frames, 12); age < 12 {
				k := 1 - age/12
				ux, uy := unit(float64(x-t.FromX), float64(y-t.FromY))
				dx, dy = dx+ux*0.1*k, dy+uy*0.1*k
				if f := float32(k); f > flash {
					flash = f
				}
			}
		}
	}
	return dx, dy, flash
}

func traceKey(t game.TraceView, tick uint64) [3]uint64 {
	return [3]uint64{uint64(t.FromX)<<20 | uint64(t.FromY), uint64(t.ToX)<<20 | uint64(t.ToY), tick - uint64(t.Age)}
}

// trackReactions remembers every trace for minReactionFrames present
// frames after the sim drops it.
func (st *animState) trackReactions(traces []game.TraceView, tick, present uint64) {
	for _, rec := range st.recs {
		rec.live = false
	}
	for _, t := range traces {
		k := traceKey(t, tick)
		rec, ok := st.recs[k]
		if !ok {
			rec = &reactRec{seen: present}
			st.recs[k] = rec
		}
		rec.trace, rec.live = t, true
	}
	for k, rec := range st.recs {
		if !rec.live && present-rec.seen > 3*minReactionFrames {
			delete(st.recs, k)
		}
	}
}

// reactions appends the tracked traces, aged for display.
func (st *animState) reactions(present uint64, dst []reactTrace) []reactTrace {
	for _, rec := range st.recs {
		rt := reactTrace{TraceView: rec.trace, sim: math.Inf(1), frames: float64(present - rec.seen)}
		if rec.live {
			rt.sim = float64(rec.trace.Age)
		}
		dst = append(dst, rt)
	}
	return dst
}

// reaction is the pose offset and flash of the creature at (x, y).
func (r *Renderer) reaction(x, y int) (dx, dy float64, flash float32) {
	return attackReactionF(r.animState().reactions(r.present, nil), x, y)
}

// animateCombat spawns hit splatter and muzzle smoke, once per trace.
func (r *Renderer) animateCombat(w game.WorldView, pawns []game.PawnView) {
	st := r.animState()
	now := uint64(r.game.Tick())
	st.trackReactions(w.Traces, now, r.present)
	for _, t := range w.Traces {
		key := [3]uint64{uint64(t.FromX)<<20 | uint64(t.FromY), uint64(t.ToX)<<20 | uint64(t.ToY), now - uint64(t.Age)}
		if r.seenTrace[key] {
			continue
		}
		r.seenTrace[key] = true
		r.floatTrace(t, key[0]^key[1]^key[2])
		switch {
		case t.Hit && t.Building:
			r.burst(partChip, float64(t.ToX)+0.5, float64(t.ToY)+0.4, 5, key[0]^key[1]^key[2])
		case t.Hit:
			r.burst(partBlood, float64(t.ToX)+0.5, float64(t.ToY)+0.35, 5, key[0]^key[1]^key[2])
		}
		if !t.Melee {
			ux, uy := unit(float64(t.ToX-t.FromX), float64(t.ToY-t.FromY))
			r.burst(partSmoke, float64(t.FromX)+0.5+ux*0.5, float64(t.FromY)+0.3+uy*0.5, 2, key[2])
		}
	}
	if len(r.seenTrace) > 256 {
		r.seenTrace = map[[3]uint64]bool{}
	}
}

// hitReaction is the flinch layer while a hit is fresh (HitTick).
func (r *Renderer) hitReaction(p game.PawnView, m moveState) actionPose { return actionPose{} }

// combatPose is the aim/fire/melee layer while attacking.
func (r *Renderer) combatPose(p game.PawnView, m moveState) actionPose { return actionPose{} }

// combatOffset is the recoil, lunge and knockback nudge applied on top of
// any layer.
func (r *Renderer) combatOffset(p game.PawnView) (dx, dy float64, flash float32) {
	return r.reaction(p.X, p.Y)
}
