package ebiten

import (
	"math"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// moveState is how a creature moves on screen right now (ART §27). It is
// presentation-only: derived from read models and the presentation clocks,
// never written back to the simulation.
type moveState struct {
	X, Y   float64 // world tiles, sub-tile, sampled at animT
	VX, VY float64 // tiles per tick, smoothed
	Speed  float64 // |V|; 0 when idle
	Stride float64 // distance walked along the drawn path, in tiles
	Blend  float64 // 0..1 walk weight (eases idle <-> walk)
	Facing facing  // hysteresis-smoothed
	Moving bool    // inside a step (false while blocked or idle)
}

// animT is sim-scaled presentation time in ticks: it freezes when paused and
// runs faster at higher speeds. Movement, walk cycles, work swings and
// combat reactions use it; ambient effects use present.
func (r *Renderer) animT() float64 {
	return float64(r.game.Tick())
}

// pawnMotion is the on-screen motion of a colonist or raider.
func (r *Renderer) pawnMotion(id entity.EntityID) moveState {
	return r.stepMotion(id, game.TicksPerCell)
}

// animalMotion is the on-screen motion of an animal.
func (r *Renderer) animalMotion(id entity.EntityID) moveState {
	return r.stepMotion(id, game.TicksPerCell)
}

// stepMotion is the contract stand-in: the old linear tile lerp.
func (r *Renderer) stepMotion(id entity.EntityID, period int) moveState {
	vis, ok := r.last[id]
	if !ok {
		return moveState{}
	}
	t := moveProgress(int(r.game.Tick()-vis.moveTick), period)
	m := moveState{
		X:      lerp(vis.fromX, vis.x, t),
		Y:      lerp(vis.fromY, vis.y, t),
		Facing: vis.facing,
		Stride: float64(vis.steps),
	}
	if t < 1 && (vis.fromX != vis.x || vis.fromY != vis.y) {
		dx, dy := float64(vis.x-vis.fromX), float64(vis.y-vis.fromY)
		m.VX, m.VY = dx/float64(period), dy/float64(period)
		m.Speed = math.Hypot(m.VX, m.VY)
		m.Stride = float64(vis.steps) - 1 + t
		m.Blend, m.Moving = 1, true
	}
	return m
}
