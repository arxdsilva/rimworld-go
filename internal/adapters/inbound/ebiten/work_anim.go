package ebiten

import (
	"math"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Work presentation (ART §27.2): per-action poses, held tools, loads
// and strike particles. The dispatch in pawn_anim.go calls in here.

// --- work motion --------------------------------------------------------

// actStyle is how a kind of hands-on work moves and what it throws up.
type actStyle struct {
	period int     // frames per swing
	reach  float64 // lean towards the worked tile, tiles
	burst  partKind
	count  int
	motion motion
	tool   string // sprite key of the held tool, "" for bare hands
}

// motion is the arm pattern for a kind of work.
type motion uint8

const (
	motionNone  motion = iota // no arm frames (eating)
	motionSwing               // overhead: wind-up, strike, recover
	motionReach               // hands working low in front
)

// strikeAt is where in a swing the tool lands (phase 0..1).
const strikeAt = 0.48

func styleFor(act string) (actStyle, bool) {
	switch act {
	case "cut":
		return actStyle{period: 46, reach: 0.1, burst: partChip, count: 4, motion: motionSwing, tool: "tool/axe"}, true
	case "construct", "deconstruct":
		return actStyle{period: 34, reach: 0.08, burst: partChip, count: 3, motion: motionSwing, tool: "tool/hammer"}, true
	case "mine":
		return actStyle{period: 50, reach: 0.12, burst: partDust, count: 5, motion: motionSwing, tool: "tool/pickaxe"}, true
	case "work at workbench":
		return actStyle{period: 30, reach: 0.05, burst: partShaving, count: 2, motion: motionSwing, tool: "tool/hammer"}, true
	case "play horseshoes":
		return actStyle{period: 70, reach: 0.04, burst: partDust, count: 3, motion: motionSwing}, true
	case "refuel":
		return actStyle{period: 40, reach: 0.06, burst: partChip, count: 1, motion: motionReach}, true
	case "work at tailoring bench":
		return actStyle{period: 44, reach: 0.05, burst: partShaving, count: 1, motion: motionReach}, true
	case "work at drying rack":
		return actStyle{period: 52, reach: 0.06, burst: partLeaf, count: 1, motion: motionReach}, true
	case "work at butcher table":
		return actStyle{period: 36, reach: 0.06, burst: partBlood, count: 2, motion: motionSwing}, true
	case "harvest":
		return actStyle{period: 44, reach: 0.07, burst: partLeaf, count: 3, motion: motionReach}, true
	case "sow":
		return actStyle{period: 48, reach: 0.05, burst: partSoil, count: 3, motion: motionReach}, true
	case "work at stove":
		return actStyle{period: 40, reach: 0.04, burst: partSteam, count: 1, motion: motionReach}, true
	case "work at campfire":
		return actStyle{period: 40, reach: 0.04, burst: partSteam, count: 1, motion: motionReach}, true
	case "tend":
		return actStyle{period: 60, reach: 0.05, motion: motionReach}, true
	case "eat":
		return actStyle{period: 50, reach: 0.03}, true
	}
	return actStyle{}, false
}

// workPhase is the 0..1 position within the current swing at animT t
// (ticks); each pawn is offset so a crew does not swing in unison.
func workPhase(t float64, id uint64, period int) float64 {
	if period <= 0 {
		return 0
	}
	return math.Mod(t+float64(id*13), float64(period)) / float64(period)
}

// workFrame is the work frame suffix at a swing phase: 1-2 wind-up,
// 3-4 swing, 5 strike, 6 recover.
func workFrame(ph float64) string {
	switch {
	case ph < 0.15:
		return "_work1"
	case ph < 0.30:
		return "_work2"
	case ph < 0.39:
		return "_work3"
	case ph < strikeAt:
		return "_work4"
	case ph < 0.58:
		return "_work5"
	default:
		return "_work6"
	}
}

// workArms is the frame suffix for a motion at a swing phase.
func workArms(m motion, ph float64) string {
	if m == motionNone {
		return ""
	}
	return workFrame(ph)
}

// workSwing is the body lean for a swing at animT t: back a little while
// winding up, hard forward on the strike, easing out.
func workSwing(t float64, id uint64, period int) float64 {
	ph := workPhase(t, id, period)
	switch {
	case ph < 0.40:
		return -0.3 * easeOut(ph/0.40)
	case ph < strikeAt:
		return -0.3 + 1.3*easeOut((ph-0.40)/(strikeAt-0.40))
	default:
		return 1 - easeOut((ph-strikeAt)/(1-strikeAt))
	}
}

func strikeIndex(t float64, id uint64, period int) float64 {
	return math.Floor((t+float64(id*13))/float64(period) - strikeAt)
}

// workStrikeAt reports whether the tool lands while animT runs from prev
// to cur. A paused clock (prev == cur) never strikes; 3x time strikes 3x
// as often.
func workStrikeAt(prev, cur float64, id uint64, period int) bool {
	return period > 0 && strikeIndex(cur, id, period) > strikeIndex(prev, id, period)
}

// toolAngles is the held tool's screen rotation per work frame for an
// east-facing pawn (radians, clockwise; the sprite points east).

// animateWork spawns each working pawn's strike particles.
func (r *Renderer) animateWork(w game.WorldView, pawns []game.PawnView) {
	st := r.animState()
	t := r.animT() // swings run on sim-scaled time
	acting := map[entity.EntityID]bool{}
	for _, p := range pawns {
		if p.Lying || p.CarriedBy != 0 {
			continue
		}
		sty, ok := styleFor(p.Acting)
		if !ok {
			continue
		}
		acting[p.ID] = true
		prev, seen := st.prevWork[p.ID]
		st.prevWork[p.ID] = t
		if !seen || sty.count == 0 || !workStrikeAt(prev, t, uint64(p.ID), sty.period) {
			continue
		}
		x, y := float64(p.ActX)+0.5, float64(p.ActY)+0.45
		switch p.Acting {
		case "work at stove":
			x, y = float64(p.ActX)+0.7, float64(p.ActY)-0.1
		case "work at campfire":
			x, y = float64(p.ActX)+0.5, float64(p.ActY)-0.1
		case "sow", "eat":
			x, y = float64(p.X)+0.5, float64(p.Y)+0.6
		}
		r.burst(sty.burst, x, y, sty.count, uint64(p.ID)*7919+r.present)
	}
	for id := range st.prevWork {
		if !acting[id] {
			delete(st.prevWork, id)
		}
	}
}

// carryPose is the arms-full layer while hauling or rescuing.
func (r *Renderer) carryPose(p game.PawnView, m moveState) actionPose { return actionPose{} }

// workPose is the hands-on work layer while standing at a job.
func (r *Renderer) workPose(p game.PawnView, m moveState) actionPose {
	st, ok := styleFor(p.Acting)
	if !ok || m.Blend >= minBlend {
		return actionPose{}
	}
	t := r.animT()
	a := actionPose{ok: true, suffix: workArms(st.motion, workPhase(t, uint64(p.ID), st.period))}
	k := workSwing(t, uint64(p.ID), st.period)
	ux, uy := unit(float64(p.ActX-p.X), float64(p.ActY-p.Y))
	if ux == 0 && uy == 0 {
		a.squash = 0.07 * k // bending down to the ground
	}
	a.dx, a.dy, a.rot = ux*st.reach*k, uy*st.reach*k, ux*0.14*k
	if st.motion != motionNone {
		a.held = heldItem{set: true, key: st.tool, stance: stanceHands} // the tool (or bare hands) replaces the weapon
	}
	return a
}
