package ebiten

import (
	"reflect"

	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Animals, held weapons and attack traces are drawn by render/creature
// (ARCH-004). The animals' visual records stay here beside the pawns'
// (syncPawns, renderer.go) until pawnVisual moves with them.

// syncAnimals advances per-animal presentation state (motion, facing).
// Footstep dust comes from animate, at the feet on contact frames.
func (r *Renderer) syncAnimals(w game.WorldView) {
	tick := r.game.Tick()
	running := r.mo.Stepped > 0
	l := r.pipe().LOD
	for i := range w.Animals {
		an := &w.Animals[i]
		if !l.InView(an.X, an.Y, world.MotionViewPad, world.MotionViewPad) {
			// far off screen (a big map's wildlife): no motion track; it
			// starts afresh, on its cell, when it nears the view again
			delete(r.last, an.ID)
			continue
		}
		prev, seen := r.last[an.ID]
		v := nextVisualStep(prev, seen, an.X, an.Y, tick, r.present, an.StepTicks)
		k := r.mo.Observe(an.ID, motionpkg.AnimalMoView(*an), v.facing)
		want, moving := k.WantFacing(r.mo.Ref)
		if !moving {
			want = faceAttack(pawnVisual{facing: want}, w.Traces, an.X, an.Y).facing
		}
		v.facing = k.Turn(want, an.ID, running)
		r.last[an.ID] = v
	}
}

// nextVisualStep is nextVisual for movers with their own pace.
func nextVisualStep(prev pawnVisual, seen bool, x, y int, tick clock.Tick, present uint64, step int) pawnVisual {
	v := nextVisual(prev, seen, x, y, tick, present)
	if seen && prev.x == x && prev.y == y && tick-prev.moveTick < clock.Tick(max(step, 1)) {
		v.fromX, v.fromY = prev.fromX, prev.fromY
	}
	return v
}

// settleVisuals finishes every in-flight step. Captures freeze the
// simulation, so without this a mover is drawn mid-step.
func (r *Renderer) settleVisuals() {
	for id, v := range r.last {
		v.fromX, v.fromY = v.x, v.y
		r.last[id] = v
	}
	for _, k := range r.mo.Tracks {
		k.Settle()
	}
}

// shieldStates appends each pawn's shield belt charge to out for the cue
// bus (Bus.ListenShields: a pop when a charge falls to 0). PawnView.ShieldPct
// is CBT-3's; until the field exists nothing is appended. It reads the
// field by index, so a frame allocates nothing once out has grown.
func shieldStates(out []cue.ShieldState, pawns []game.PawnView) []cue.ShieldState {
	t := reflect.TypeOf(game.PawnView{})
	idx := -1
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == "ShieldPct" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return out
	}
	for i := range pawns {
		f := reflect.ValueOf(&pawns[i]).Elem().Field(idx)
		if f.CanInt() {
			out = append(out, cue.ShieldState{ID: pawns[i].ID, Pct: int(f.Int()), X: pawns[i].X, Y: pawns[i].Y})
		}
	}
	return out
}
