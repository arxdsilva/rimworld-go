package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	motionpkgpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/domain/sim/entity"
)

// Creature motion, the animation clock, poses and gaits live in
// render/motion (ARCH-004), imported as motionpkg because the root still
// has a type named motion (work_anim.go). These aliases and one-line
// wrappers keep the root's call sites unchanged while its features move
// out one package at a time; each mover repoints its own files to motion
// and the last one deletes this file (SOLID_GO.md §11).

type (
	motionState = motionpkg.State[creature.Body] // each track carries its animal's body player
	moTrack     = motionpkg.Track[creature.Body]
)

// motionViewPad: animals within this many tiles of the last frame's view
// keep a motion track (syncAnimals); every pass that samples their motion
// culls tighter (objects, shadows, plant contacts).
const motionViewPad = env.ContactViewPad + 2

func hash01(keys ...uint64) float64 { return motionpkg.Hash01(keys...) }

// animT is the sim-scaled presentation clock in ticks (motionpkg.State.AnimT);
// 0 without a simulation.
func (r *Renderer) animT() float64 {
	if r.game == nil {
		return 0
	}
	return r.mo.AnimT(r.game.Tick())
}

// pawnMotion is the on-screen motion of a colonist or raider. It and
// animalMotion go through motionOf (not State.PawnMotion) so they still
// inline into their per-creature callers.
func (r *Renderer) pawnMotion(id entity.EntityID) motionpkgpkg.MoveState {
	m, _ := r.motionOf(id)
	return m
}

// animalMotion is the on-screen motion of an animal.
func (r *Renderer) animalMotion(id entity.EntityID) motionpkgpkg.MoveState {
	m, _ := r.motionOf(id)
	return m
}

// motionOf is a creature's motion at animT; false without a track.
func (r *Renderer) motionOf(id entity.EntityID) (motionpkgpkg.MoveState, bool) {
	return r.mo.MotionOf(id, r.animT())
}
