package motion

import (
	"math"

	"rimworld-go/internal/domain/sim/clock"
)

// Frame pacing (GAR-091). Ebitengine runs Updates on its own clock. Below
// 60 fps some frames get two Updates back to back; at 60 its clock drifting
// against the display gives one frame two Updates and the next none. Alpha
// is measured from the last Update's wall time, which cannot tell two
// back-to-back Updates apart: at 59 fps a walker lurches two ticks about
// once a second, at 45 fps every third frame, and a 2-then-0 pair advances
// 2, 1 and 0 ticks where it should advance 1, 1 and 1. Pace keeps a drawn clock that advances by the measured tick rate
// each Draw and pulls toward the interpolation point
// (Tick - (1-Alpha)*Stepped). It never goes backwards, never passes the
// latest tick, and never trails it by more than two Updates.
const (
	paceTau   = 0.25 // s: how fast the drawn clock pulls toward the interpolation point
	rateTau   = 1.5  // s: the averaging window of the measured tick rate
	paceMaxDt = 0.25 // s: a longer gap between Draws (a hitch, a drag) snaps
	paceSnap  = 1e-3 // ticks: closer than this, the clock takes the point
	paceStall = 5    // Draws without a tick: a pause; the rate re-seeds on resume
)

// pace is the drawn clock between Draws.
type pace struct {
	on      bool
	tick    float64 // the sim tick of the latest paced Draw
	t       float64 // the drawn animT
	rate    float64 // sim ticks per second, averaged
	nominal int     // ticks per Update the rate was seeded at
	idle    int     // Draws since the tick last moved
}

// Pace advances the drawn clock for a Draw dt seconds after the previous
// one, at sim tick tick, with nominal ticks per Update at tps Updates a
// second. Call it after SetAlpha; AnimT(tick) then returns the drawn clock
// until the tick changes. Captures never call it, so their frames stay
// deterministic.
func (mo *State[B]) Pace(tick clock.Tick, dt float64, nominal int, tps float64) {
	tk := float64(tick)
	target := mo.target(tk)
	p := &mo.pace
	if !p.on || tk < p.tick || dt <= 0 || dt > paceMaxDt {
		t := target
		if p.on && tk >= p.tick {
			t = math.Max(t, p.t) // a snap forward only
		}
		*p = pace{on: true, tick: tk, t: math.Min(t, tk), rate: float64(nominal) * tps, nominal: nominal}
		return
	}
	switch {
	case nominal != p.nominal, tk > p.tick && p.idle >= paceStall:
		// a speed change, or ticks again after a pause (the pause menu
		// stops them without changing the tier): the tier's rate at once,
		// no snap
		p.rate, p.nominal = float64(nominal)*tps, nominal
	default:
		p.rate += ((tk-p.tick)/dt - p.rate) * (1 - math.Exp(-dt/rateTau))
	}
	next := p.t + p.rate*dt
	next += (target - next) * (1 - math.Exp(-dt/paceTau))
	if math.Abs(target-next) < paceSnap {
		next = target // a pause holds exactly instead of creeping
	}
	window := float64(max(mo.Stepped, nominal, 1))
	next = math.Min(math.Max(math.Max(next, p.t), tk-2*window), tk)
	if tk == p.tick {
		p.idle++
	} else {
		p.idle = 0
	}
	p.t, p.tick = next, tk
}

// target is the interpolation point at sim tick tk: one Update behind it,
// alpha of the way through.
func (mo *State[B]) target(tk float64) float64 {
	return tk - (1-mo.Alpha)*float64(mo.Stepped)
}
