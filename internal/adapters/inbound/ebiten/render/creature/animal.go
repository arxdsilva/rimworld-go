package creature

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// AnimalBaseKey is the animal's standing sprite for its current facing:
// "animal/<species>_<facing>" (ART-011) with facing south, east or north
// (west draws the east art mirrored: animalMirrored); a female of a
// dimorphic species uses "animal/<species>_f_<facing>" when that art
// exists (else the male set, so a species without female art still draws).
func (c *Set) AnimalBaseKey(v *render.View, an game.AnimalView) string {
	facing, _ := FacingKey(c.Host.Visual(an.ID).Facing)
	if an.Female {
		if k := "animal/" + an.Species + "_f_" + facing; v.Art.Has(k) {
			return k
		}
	}
	return "animal/" + an.Species + "_" + facing
}

// AnimalFrameKey is the sprite drawn for the animal this frame: its base
// key with the trot, attack, graze or flinch frame (framed falls back to
// the standing sprite when a frame is missing).
func (c *Set) AnimalFrameKey(v *render.View, an game.AnimalView) string {
	base := c.AnimalBaseKey(v, an)
	if c.AnimalResting(v, an) {
		if k := base + "_rest"; v.Art.Has(k) {
			return k
		}
		return motion.Framed(v.Art, base, "_graze1") // no rest pose yet: head down
	}
	return motion.Framed(v.Art, base, c.AnimalFrame(v, an))
}

// AnimalMirrored reports an animal facing west, drawn as its east art
// flipped about its pivot.
func (c *Set) AnimalMirrored(v *render.View, an game.AnimalView) bool {
	_, m := FacingKey(c.Host.Visual(an.ID).Facing)
	return m
}

// AnimalResting reports an animal drawn lying down: a sleeper standing
// still, once its body player has passed the neutral hold (AnimalShow).
func (c *Set) AnimalResting(v *render.View, an game.AnimalView) bool {
	return c.AnimalShow(v, an).Rest
}

// AnimalRestFallback is true when a resting animal has no rest sprite and
// shows its grazing frame dimmed instead.
func (c *Set) AnimalRestFallback(v *render.View, an game.AnimalView) bool {
	return c.AnimalResting(v, an) && !v.Art.Has(c.AnimalBaseKey(v, an)+"_rest")
}

// restDim is the colour scale of the stand-in rest frame (10% darker).
var restDim = sprite.ColorScale{0.9, 0.9, 0.9, 1}

// downedTint greys a downed animal's rest pose slightly (cool and a touch
// darker), so it reads apart from a sleeper.
var downedTint = sprite.ColorScale{0.78, 0.8, 0.86, 1}

// AnimalFigureScale is the colour scale of an animal's figure: the
// downed tint, times the stand-in rest frame's dimming.
func (c *Set) AnimalFigureScale(v *render.View, an game.AnimalView) (sprite.ColorScale, bool) {
	cs, ok := sprite.NoCS, false
	if an.Downed {
		cs, ok = downedTint, true
	}
	if c.AnimalRestFallback(v, an) {
		cs, ok = sprite.MulCS(cs, restDim), true
	}
	return cs, ok
}

// YoungPose scales a juvenile's figure about its feet (Scale 0.55..1; 0
// or 1 is full size).
func YoungPose(ps motion.Pose, scale float64) motion.Pose {
	if scale <= 0 || scale >= 1 {
		return ps
	}
	if ps.Sx == 0 {
		ps.Sx = 1
	}
	if ps.Sy == 0 {
		ps.Sy = 1
	}
	ps.Sx *= scale
	ps.Sy *= scale
	return ps
}

// DrawAnimalFigure draws one animal's body, posed, at (wx, wy): the same
// call in the object pass and in the shadow pass (pass.Cast set). It has no
// side effects beyond drawing.
func (c *Set) DrawAnimalFigure(v *render.View, pass sprite.Pass, dst *ebiten.Image, an game.AnimalView, wx, wy, tile float64) {
	if !v.Art.Has(c.AnimalBaseKey(v, an)) {
		if pass.Cast != nil {
			return // no art, no silhouette
		}
		sx, sy := v.WorldToScreenF(wx, wy)
		vector.FillCircle(dst, sx+float32(tile)/2, sy+float32(tile)/2, float32(tile)*0.3, color.RGBA{150, 120, 80, 255}, false)
		return
	}
	pass.Blood = c.Host.AnimalBlood(an) // the whole figure (Host.AnimalBlood)
	ps := YoungPose(c.AnimalPose(v, an), an.Scale)
	if c.AnimalMirrored(v, an) {
		ps = Mirrored(ps)
	}
	if cs, ok := c.AnimalFigureScale(v, an); ok {
		ps.Cs = cs
	}
	c.DrawSpritePose(v, pass, dst, c.AnimalFrameKey(v, an), color.NRGBA{}, wx, wy, tile, ps)
}

// --- walk phase and idle poses (PLAY-022 B) ----------------------------------

// Walk cycle tuning. ART (PLAY-022): walk03 and walk07 are the neutral
// frames, closest to standing (all feet under the body, body at standing
// height); walk01 and walk05 are full contact (body lowest).
const (
	// walkStartPhase is where a walk begins: walk03, a neutral frame.
	walkStartPhase = 2.0 / motion.TrotFrames
	// neutralMid is the middle of a neutral frame's phase window.
	neutralMid = walkStartPhase + 0.5/motion.TrotFrames
	// GaitFallback is the ground travel per cycle of a set without "gait".
	GaitFallback = motion.StrideLen
	// MaxPhasePerDraw caps how far the walk advances between two draws
	// (cycles): past it the frames would alias (6x at 60 Hz).
	MaxPhasePerDraw = 0.3
	// settlePerTick is how fast a stopped walk finishes its step onto a
	// neutral frame (cycles per animT tick: a cycle in half a second).
	settlePerTick = 1.0 / 30
	// steerMinRate is the slowest the legs may run, as a share of the
	// body's pace, while a stopping walk steers onto a neutral frame.
	steerMinRate = 0.4
)

// Idle pose timing, in animT ticks (sim time: it freezes when paused and
// runs faster at higher speeds). Every pose holds bodyMinHold before it
// may change (no flap); grazing and lying down start after standing
// BodyNeutralHold; the head passes the graze1 frame bodyReachTicks on its
// way down and up. A graze bout keeps the head down grazeDownMin plus up
// to grazeDownSpan, then looks up (standing) grazeLookMin plus up to
// GrazeLookSpan, both per animal and bout from a stable hash.
const (
	BodyMinHold     = 12
	BodyNeutralHold = 18
	BodyReachTicks  = 12
	GrazeDownMin    = 3 * 60
	GrazeDownSpan   = 5 * 60
	GrazeLookMin    = 1 * 60
	GrazeLookSpan   = 1 * 60
)

// bodyCls is the pose class an animal's read model asks for.
type bodyCls uint8

const (
	clsStand bodyCls = iota
	clsWalk
	clsGraze
	clsRest
)

// bodyStage is what an animal's body shows.
type bodyStage uint8

const (
	stStand bodyStage = iota // the standing frame (a predator's idle too)
	stWalk                   // the walk cycle, a stop finishing its step included
	stReach                  // graze1, the head on its way down
	stDown                   // graze2, grazing
	stRise                   // graze1, the head on its way up
	stLook                   // standing: a look up inside a graze bout
	stRest                   // lying
)

// Body is one animal's walk phase and idle pose player, kept on its
// motion track (motion.Track.Body), advanced once per animT (a draw).
type Body struct {
	Ok     bool
	t      float64 // animT of the last advance
	stride float64 // stride at t
	ph     float64 // walk phase, cycles
	// steer: the walk's end is known (endS, in stride) and the phase
	// lands on target, a neutral frame, there.
	steer        bool
	target, endS float64
	stage, from  bodyStage
	since        float64 // animT the stage began
	bout         uint64  // graze bout counter (timing hash)
}

// BodyShow is what the body player shows this draw.
type BodyShow struct {
	Suffix string // "", "_walkNN", "_graze1" or "_graze2"
	Rest   bool   // lying (the rest frame)
	Walk   bool
	Ph     float64 // walk phase, while walk
}

// bodyWant is the pose class an animal's view asks for.
func bodyWant(an game.AnimalView, m motion.MoveState) bodyCls {
	switch {
	case m.Blend >= motion.MinBlend:
		return clsWalk
	case an.Sleeping:
		return clsRest
	case an.Action == game.ActionGraze:
		return clsGraze
	}
	return clsStand
}

// AnimalGait is the animal's ground travel per walk cycle in tiles: its
// set's metadata "gait" (gaitFallback without one), times a juvenile's
// Scale (youngPose draws it smaller about its feet).
func (c *Set) AnimalGait(v *render.View, an game.AnimalView) float64 {
	g := GaitFallback
	if m := v.Art.MetaOf(c.AnimalBaseKey(v, an)); m != nil && m.Gait > 0 {
		g = m.Gait
	}
	if an.Scale > 0 && an.Scale < 1 {
		g *= an.Scale
	}
	return g
}

// AnimalShow advances the animal's body player to animT (once per animT;
// every pass of a draw sees the same frame) and returns what it shows. An
// animal without a motion track (never synced) shows its view's class
// directly, on the same clocks.
func (c *Set) AnimalShow(v *render.View, an game.AnimalView) BodyShow {
	m, ok := c.motionOf(v, an.ID)
	t := v.AnimT
	k := c.mo.Tracks[an.ID]
	if !ok || k == nil {
		return StaticShow(an, m, t, c.AnimalGait(v, an))
	}
	b := &k.Body
	want := bodyWant(an, m)
	switch {
	case !b.Ok || t < b.t-1:
		b.start(an.ID, want, m, t)
	case t > b.t:
		b.advance(an.ID, want, m, c.AnimalGait(v, an), t)
	}
	return b.show()
}

// start puts a newly seen body straight into the pose its view asks for,
// as if it had held it a while (a capture's single frame shows it), a
// grazer somewhere inside its bout.
func (b *Body) start(id entity.EntityID, want bodyCls, m motion.MoveState, t float64) {
	*b = Body{Ok: true, t: t, stride: m.Stride, since: t - BodyNeutralHold}
	switch want {
	case clsWalk:
		b.stage, b.ph = stWalk, walkStartPhase
	case clsRest:
		b.stage = stRest
	case clsGraze:
		b.stage = stDown
		b.since = t - 0.8*motion.Hash01(uint64(id), 0x67a2e)*float64(grazeDown(id, 0))
	}
}

func (b *Body) enter(s bodyStage, t float64) {
	b.stage, b.from, b.since = s, b.stage, t
}

// advance moves the body on to animT t (> b.t).
func (b *Body) advance(id entity.EntityID, want bodyCls, m motion.MoveState, g, t float64) {
	dt := t - b.t
	dS := math.Max(0, m.Stride-b.stride)
	b.t, b.stride = t, m.Stride
	held := t - b.since
	if b.stage == stWalk {
		if want == clsWalk {
			b.walkOn(m, g, dS)
		} else if b.settle(dt) {
			b.enter(stStand, t)
		}
		return
	}
	if want == clsWalk { // the body moves: get the head up or get up first
		switch b.stage {
		case stReach, stDown:
			b.enter(stRise, t)
		case stRest:
			b.enter(stStand, t)
		case stRise:
			if held >= BodyReachTicks {
				b.startWalk(t)
			}
		default: // standing; just up from lying: a moment upright first
			if b.from != stRest || held >= BodyReachTicks {
				b.startWalk(t)
			}
		}
		return
	}
	if held < BodyMinHold {
		return // every pose holds a moment (no flap)
	}
	switch want {
	case clsGraze:
		switch b.stage {
		case stStand:
			if held >= BodyNeutralHold {
				b.enter(stReach, t)
			}
		case stReach:
			b.enter(stDown, t)
		case stDown:
			if held >= float64(grazeDown(id, b.bout)) {
				b.enter(stRise, t)
			}
		case stRise:
			b.enter(stLook, t)
		case stLook:
			if held >= float64(grazeLook(id, b.bout)) {
				b.bout++
				b.enter(stReach, t)
			}
		case stRest:
			b.enter(stStand, t)
		}
	case clsRest:
		switch b.stage {
		case stStand, stLook:
			if held >= BodyNeutralHold {
				b.enter(stRest, t)
			}
		case stReach, stDown:
			b.enter(stRise, t)
		case stRise:
			b.enter(stStand, t)
		}
	default:
		switch b.stage {
		case stReach, stDown:
			b.enter(stRise, t)
		case stRise, stRest:
			b.enter(stStand, t)
		case stLook:
			b.stage = stStand // the same frame: no new hold
		}
	}
}

func (b *Body) startWalk(t float64) {
	b.enter(stWalk, t)
	b.ph, b.steer = walkStartPhase, false
}

// walkOn advances a walk by dS tiles of stride: one cycle per g tiles, so
// the planted feet stay put; once the walk's end is known, at the rate that
// lands it on a neutral frame there; never more than maxPhasePerDraw.
func (b *Body) walkOn(m motion.MoveState, g, dS float64) {
	d := dS / g
	switch left := m.Left; {
	case math.IsInf(left, 1):
		b.steer = false
	case left > 0:
		rem := left + dS // the stride left before this advance
		if end := m.Stride + left; !b.steer || math.Abs(end-b.endS) > 1e-6 {
			b.steer, b.endS, b.target = true, end, neutralTarget(b.ph, rem/g)
		}
		d = (b.target - b.ph) * dS / rem
	}
	b.ph += math.Max(0, math.Min(d, MaxPhasePerDraw))
}

// settle finishes a stopped walk's step: true once the phase is on a
// neutral frame, else it moves on towards the next one over dt ticks.
func (b *Body) settle(dt float64) bool {
	b.steer = false
	if NeutralFrame(b.ph) {
		return true
	}
	next := math.Ceil((b.ph-walkStartPhase)*2)/2 + walkStartPhase
	if adv := math.Min(settlePerTick*dt, MaxPhasePerDraw); b.ph+adv >= next {
		b.ph = next + neutralMid - walkStartPhase
	} else {
		b.ph += adv
	}
	return false
}

func (b *Body) show() BodyShow {
	switch b.stage {
	case stWalk:
		return BodyShow{Suffix: motion.TrotSuffix(b.ph), Walk: true, Ph: b.ph}
	case stReach, stRise:
		return BodyShow{Suffix: "_graze1"}
	case stDown:
		return BodyShow{Suffix: "_graze2"}
	case stRest:
		return BodyShow{Rest: true}
	}
	return BodyShow{}
}

// NeutralFrame reports a phase on walk03 or walk07.
func NeutralFrame(ph float64) bool {
	f := motion.TrotFrame(ph)
	return f == int(walkStartPhase*motion.TrotFrames) || f == int(walkStartPhase*motion.TrotFrames)+motion.TrotFrames/2
}

// neutralTarget is the neutral-frame phase a walk at phase ph lands on
// with rem cycles of natural travel left: the middle of the neutral frame
// nearest its natural end, ahead of ph, and the next one when the legs
// would have to slow below steerMinRate of the body's pace to reach it.
func neutralTarget(ph, rem float64) float64 {
	fe := ph + rem
	n := math.Round((fe-neutralMid)*2)/2 + neutralMid
	for n < ph || (rem > 0 && n-ph < steerMinRate*rem) {
		n += 0.5
	}
	return n
}

// grazeDown and grazeLook are bout k's head-down and look-up times, in
// ticks, for animal id.
func grazeDown(id entity.EntityID, k uint64) int {
	return GrazeDownMin + int(motion.Hash01(uint64(id), k, 0x96a2)*GrazeDownSpan)
}

func grazeLook(id entity.EntityID, k uint64) int {
	return GrazeLookMin + int(motion.Hash01(uint64(id), k, 0x10c4)*GrazeLookSpan)
}

// StaticShow is what an animal without a motion track shows at animT t:
// the walk frame by stride, lying, or its first graze bout repeating (no
// transitions to hold without state).
func StaticShow(an game.AnimalView, m motion.MoveState, t, g float64) BodyShow {
	switch bodyWant(an, m) {
	case clsWalk:
		ph := walkStartPhase + m.Stride/g
		return BodyShow{Suffix: motion.TrotSuffix(ph), Walk: true, Ph: ph}
	case clsRest:
		return BodyShow{Rest: true}
	case clsGraze:
		down, look := float64(grazeDown(an.ID, 0)), float64(grazeLook(an.ID, 0))
		period := down + look + 2*BodyReachTicks
		u := math.Mod(t+motion.Hash01(uint64(an.ID), 0x67a2e)*period, period)
		switch {
		case u < BodyReachTicks || (u >= BodyReachTicks+down && u < 2*BodyReachTicks+down):
			return BodyShow{Suffix: "_graze1"}
		case u < BodyReachTicks+down:
			return BodyShow{Suffix: "_graze2"}
		}
	}
	return BodyShow{}
}
