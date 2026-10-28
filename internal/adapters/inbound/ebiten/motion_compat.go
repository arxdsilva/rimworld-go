package ebiten

import (
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Creature motion, the animation clock, poses and gaits live in
// render/motion (ARCH-004), imported as motionpkg because the root still
// has a type named motion (work_anim.go). These aliases and one-line
// wrappers keep the root's call sites unchanged while its features move
// out one package at a time; each mover repoints its own files to motion
// and the last one deletes this file (SOLID_GO.md §11).

type (
	moveState   = motionpkg.MoveState
	motionState = motionpkg.State[animalBody] // each track carries its animal's body player
	moTrack     = motionpkg.Track[animalBody]
	pose        = motionpkg.Pose
)

const (
	alphaMax     = motionpkg.AlphaMax
	drawnReach   = motionpkg.DrawnReach
	figurePivotX = motionpkg.FigurePivotX
	figurePivotY = motionpkg.FigurePivotY
	walkFrames   = motionpkg.WalkFrames
	trotFrames   = motionpkg.TrotFrames
	strideLen    = motionpkg.StrideLen
	contactEvery = motionpkg.ContactEvery
	minBlend     = motionpkg.MinBlend
	easeFrames   = motionpkg.EaseFrames
)

// motionViewPad: animals within this many tiles of the last frame's view
// keep a motion track (syncAnimals); every pass that samples their motion
// culls tighter (objects, shadows, plant contacts).
const motionViewPad = contactViewPad + 2

func pawnMoView(p game.PawnView) motionpkg.MoView      { return motionpkg.PawnMoView(p) }
func animalMoView(an game.AnimalView) motionpkg.MoView { return motionpkg.AnimalMoView(an) }

func easeOut(t float64) float64                     { return motionpkg.EaseOut(t) }
func hash01(keys ...uint64) float64                 { return motionpkg.Hash01(keys...) }
func trotSuffix(ph float64) string                  { return motionpkg.TrotSuffix(ph) }
func trotFrame(ph float64) int                      { return motionpkg.TrotFrame(ph) }
func gaitScale(m moveState) float64                 { return motionpkg.GaitScale(m) }
func bobRaise(stride, speed, blend float64) float64 { return motionpkg.BobRaise(stride, speed, blend) }
func gaitSway(m moveState) float64                  { return motionpkg.GaitSway(m) }
func gaitLean(f facing, m moveState) float64        { return motionpkg.GaitLean(f, m) }
func walkPose(f facing, m moveState) pose           { return motionpkg.WalkPose(standPose(), f, m) }
func contactsCrossed(prev, cur float64) int         { return motionpkg.ContactsCrossed(prev, cur) }
func smoothEase(now, since uint64, frames float64) float64 {
	return motionpkg.SmoothEase(now, since, frames)
}
func lerpPose(a, b pose, k float64) pose   { return motionpkg.LerpPose(a, b, k) }
func unit(x, y float64) (float64, float64) { return motionpkg.Unit(x, y) }
func smoothstep01(x float64) float64       { return motionpkg.Smoothstep01(x) }

func footPoint(x, y float64, f facing, parity int) [2]float64 {
	return motionpkg.FootPoint(x, y, f, parity)
}

func blockedFacing(p game.PawnView, pawns []game.PawnView, animals []game.AnimalView, prev facing) facing {
	return motionpkg.BlockedFacing(p, pawns, animals, prev)
}

// setAlpha records the Draw's position inside the update interval.
func (r *Renderer) setAlpha(a float64) { r.mo.SetAlpha(a) }

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
func (r *Renderer) pawnMotion(id entity.EntityID) moveState {
	m, _ := r.motionOf(id)
	return m
}

// animalMotion is the on-screen motion of an animal.
func (r *Renderer) animalMotion(id entity.EntityID) moveState {
	m, _ := r.motionOf(id)
	return m
}

// motionOf is a creature's motion at animT; false without a track.
func (r *Renderer) motionOf(id entity.EntityID) (moveState, bool) {
	return r.mo.MotionOf(id, r.animT())
}

// framed returns key+suffix when that frame exists, else key.
func (r *Renderer) framed(key, suffix string) string { return motionpkg.Framed(r.art, key, suffix) }

// itemPop is the pop-in scale of a ground item (syncItems).
func (r *Renderer) itemPop(id entity.EntityID) float64 {
	return motionpkg.PopScale(r.itemBorn[id], r.present)
}

// sway is the wind shear for a plant at (x, y).
func (r *Renderer) sway(x, y int, amount float64) float64 {
	return motionpkg.Sway(r.present, x, y, amount)
}
