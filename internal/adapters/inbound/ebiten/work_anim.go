package ebiten

import (
	"fmt"
	"math"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Work presentation (ART §27.2): per-action poses, held tools, loads
// and strike particles. The dispatch in pawn_anim.go calls in here.

// --- work motion --------------------------------------------------------

// motion is the generic arm pattern used when an action's own pose set
// is missing from the art.
type motion uint8

const (
	motionNone  motion = iota // no arm frames
	motionSwing               // overhead: wind-up, strike, recover
	motionReach               // hands working low in front
)

// partAt is where an action's particles spawn.
type partAt uint8

const (
	atTarget partAt = iota // the worked tile
	atHand                 // at the pawn's own hands (planting)
	atStove                // the stove's top
	atFire                 // above a campfire
)

type partSpec struct {
	kind  partKind
	count int
}

// actionDef is one row of the work table: the pose set, its timing, what
// is held, how the body leans and what each strike throws up.
type actionDef struct {
	set      string  // body frame prefix ("_chop" -> _chop1.._chop6)
	frames   int     // frames in the set
	strike   int     // 1-based frame where the tool lands
	period   int     // animT ticks per loop
	reach    float64 // lean towards the worked tile, tiles
	lean     float64 // body rotation towards the tile at full swing, radians
	squash   float64 // kneel / crouch at full swing, 0..1
	tool     string  // held sprite key; "" is bare hands
	stance   heldStance
	off      string // sprite key drawn at hand_l (bowl)
	kneel    bool   // fall back to the _kneel frame before the generic ones
	fallback motion // generic _work frames when the set is missing
	at       partAt
	parts    []partSpec
}

// workTable is the single source of truth for hands-on actions (ART
// §27.2). Combat (aim, melee) and carrying (rescue, haul) are other layers.
var workTable = map[string]actionDef{
	game.ActionChop:        {set: "_chop", frames: 6, strike: 4, period: 46, reach: 0.10, lean: 0.14, tool: "tool/axe", stance: stanceHands, fallback: motionSwing, parts: []partSpec{{partChip, 4}}},
	game.ActionMine:        {set: "_mine", frames: 6, strike: 4, period: 50, reach: 0.12, lean: 0.16, squash: 0.03, tool: "tool/pickaxe", stance: stanceHands, fallback: motionSwing, parts: []partSpec{{partSpark, 3}, {partDust, 4}}},
	game.ActionBreach:      {set: "_mine", frames: 6, strike: 4, period: 44, reach: 0.12, lean: 0.16, squash: 0.03, tool: "tool/pickaxe", stance: stanceHands, fallback: motionSwing, parts: []partSpec{{partSpark, 3}, {partDust, 4}}},
	game.ActionBuild:       {set: "_hammer", frames: 4, strike: 3, period: 34, reach: 0.08, lean: 0.10, tool: "tool/hammer", stance: stanceHands, fallback: motionSwing, parts: []partSpec{{partSawdust, 3}}},
	game.ActionDeconstruct: {set: "_hammer", frames: 4, strike: 3, period: 34, reach: 0.08, lean: 0.10, tool: "tool/hammer", stance: stanceHands, fallback: motionSwing, parts: []partSpec{{partSawdust, 3}}},
	game.ActionRepair:      {set: "_hammer", frames: 4, strike: 3, period: 38, reach: 0.08, lean: 0.10, tool: "tool/hammer", stance: stanceHands, fallback: motionSwing, parts: []partSpec{{partSawdust, 2}}},
	game.ActionSow:         {set: "_plant", frames: 4, strike: 3, period: 48, reach: 0.05, lean: 0.06, squash: 0.14, tool: "tool/trowel", stance: stanceUpright, kneel: true, fallback: motionReach, at: atHand, parts: []partSpec{{partSoil, 3}}},
	game.ActionHarvest:     {set: "_pick", frames: 4, strike: 3, period: 44, reach: 0.07, lean: 0.06, squash: 0.10, tool: "tool/sickle", stance: stanceUpright, fallback: motionReach, parts: []partSpec{{partLeaf, 3}}},
	game.ActionCraft:       {set: "_bench", frames: 4, strike: 2, period: 30, reach: 0.05, lean: 0.03, fallback: motionSwing, parts: []partSpec{{partShaving, 2}}},
	game.ActionCook:        {set: "_bench", frames: 4, strike: 2, period: 40, reach: 0.04, lean: 0.03, fallback: motionReach, at: atStove, parts: []partSpec{{partSteam, 1}}},
	game.ActionTailor:      {set: "_bench", frames: 4, strike: 2, period: 44, reach: 0.05, lean: 0.03, fallback: motionReach, parts: []partSpec{{partShaving, 1}}},
	game.ActionButcher:     {set: "_bench", frames: 4, strike: 3, period: 36, reach: 0.06, lean: 0.04, fallback: motionSwing, parts: []partSpec{{partBlood, 2}}},
	game.ActionResearch:    {set: "_bench", frames: 4, strike: 2, period: 60, reach: 0.03, lean: 0.02, fallback: motionReach},
	game.ActionTend:        {set: "_tend", frames: 4, strike: 2, period: 60, reach: 0.05, lean: 0.06, squash: 0.16, tool: "tool/bandage", stance: stanceUpright, kneel: true, fallback: motionReach},
	game.ActionTame:        {set: "_offer", frames: 2, strike: 2, period: 64, reach: 0.04, lean: 0.04, squash: 0.12, fallback: motionReach},
	// Spoon in the right hand, bowl level in the left (hand_l).
	game.ActionEat:    {set: "_eat", frames: 4, strike: 3, period: 50, reach: 0.02, tool: "tool/spoon", stance: stanceUpright, off: "tool/bowl"},
	game.ActionRefuel: {set: "_bench", frames: 4, strike: 2, period: 40, reach: 0.06, lean: 0.04, fallback: motionReach, parts: []partSpec{{partChip, 1}}},
	// Burial (SIM-017): shovel on the mine frames, kneeling bare-handed
	// to fill the grave, the bench frames at the pyre.
	game.ActionDig:        {set: "_mine", frames: 6, strike: 4, period: 56, reach: 0.10, lean: 0.14, squash: 0.04, tool: "tool/shovel", stance: stanceHands, fallback: motionSwing, parts: []partSpec{{partSoil, 4}}},
	game.ActionBury:       {set: "_pick", frames: 4, strike: 3, period: 48, reach: 0.06, lean: 0.05, squash: 0.12, kneel: true, fallback: motionReach, parts: []partSpec{{partSoil, 3}}},
	game.ActionCremate:    {set: "_bench", frames: 4, strike: 2, period: 44, reach: 0.04, lean: 0.03, fallback: motionReach, parts: []partSpec{{partEmber, 2}, {partSmoke, 1}}},
	game.ActionHorseshoes: {period: 70, reach: 0.04, lean: 0.03, fallback: motionSwing, parts: []partSpec{{partDust, 3}}},
}

// legacyActions maps the old Acting strings (a fallback when the sim sets
// no Action) to canonical actions.
var legacyActions = map[string]string{
	"cut": game.ActionChop, "construct": game.ActionBuild, "deconstruct": game.ActionDeconstruct,
	"mine": game.ActionMine, "harvest": game.ActionHarvest, "sow": game.ActionSow,
	"refuel": game.ActionRefuel, "play horseshoes": game.ActionHorseshoes,
	"tend": game.ActionTend, "eat": game.ActionEat,
	"work at workbench": game.ActionCraft, "work at stove": game.ActionCook,
	"work at campfire": game.ActionCook, "work at tailoring bench": game.ActionTailor,
	"work at butcher table": game.ActionButcher, "work at drying rack": game.ActionCraft,
	"work at research bench": game.ActionResearch,
}

// actionOf resolves a pawn's hands-on work action and its table row. A
// non-work Action (aim, melee, rescue, haul) has no row.
func actionOf(p game.PawnView) (string, actionDef, bool) {
	act := p.Action
	if act == "" {
		act = legacyActions[p.Acting]
	}
	d, ok := workTable[act]
	return act, d, ok
}

// strikeAt is where in a swing the tool lands (phase 0..1).
const strikeAt = 0.48

// strikeHold is how long the strike frame stays up, in phase.
const strikeHold = 0.10

// setFrame is the frame suffix of an n-frame set at swing phase ph: the
// strike frame shows from strikeAt, earlier frames spread before it and
// later ones after it.
func setFrame(prefix string, n, strike int, ph float64) string {
	idx := strike
	switch {
	case ph < strikeAt && strike > 1:
		idx = 1 + int(ph/strikeAt*float64(strike-1))
	case ph >= strikeAt+strikeHold && n > strike:
		idx = strike + 1 + int((ph-strikeAt-strikeHold)/(1-strikeAt-strikeHold)*float64(n-strike))
	}
	idx = max(1, min(idx, n))
	return fmt.Sprintf("%s%d", prefix, idx)
}

// hasFrame reports whether the body art has a frame suffix. Without a
// catalog (unit tests) every frame is assumed present.
func (r *Renderer) hasFrame(suffix string) bool {
	return r.art == nil || r.art.has("pawn/body_east"+suffix)
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

// animateWork spawns each working pawn's strike particles, once per swing.
func (r *Renderer) animateWork(w game.WorldView, pawns []game.PawnView) {
	st := r.animState()
	t := r.animT() // swings run on sim-scaled time
	for _, b := range w.Buildings {
		if b.Kind == "funeral pyre" {
			r.emitPyre(b)
		}
	}
	acting := map[entity.EntityID]bool{}
	for _, p := range pawns {
		if p.Lying || p.CarriedBy != 0 {
			continue
		}
		_, d, ok := actionOf(p)
		if !ok {
			continue
		}
		acting[p.ID] = true
		prev, seen := st.prevWork[p.ID]
		st.prevWork[p.ID] = t
		if !seen || len(d.parts) == 0 || !workStrikeAt(prev, t, uint64(p.ID), d.period) {
			continue
		}
		x, y := float64(p.ActX)+0.5, float64(p.ActY)+0.45
		switch d.at {
		case atStove:
			x, y = float64(p.ActX)+0.7, float64(p.ActY)-0.1
			if p.Acting == "work at campfire" {
				x = float64(p.ActX) + 0.5
			}
		case atFire:
			x, y = float64(p.ActX)+0.5, float64(p.ActY)-0.1
		case atHand:
			x, y = float64(p.X)+0.5, float64(p.Y)+0.6
		}
		ux, uy := unit(float64(p.ActX-p.X), float64(p.ActY-p.Y))
		for i, ps := range d.parts {
			seed := uint64(p.ID)*7919 + r.present + uint64(i)*104729
			if ps.kind == partSpark { // sparks glance back off the rock
				r.burstDir(ps.kind, x, y, ps.count, seed, -ux, -uy, 0.02)
				continue
			}
			r.burst(ps.kind, x, y, ps.count, seed)
		}
	}
	for id := range st.prevWork {
		if !acting[id] {
			delete(st.prevWork, id)
		}
	}
}

// carryPose is the arms-full layer: it applies whenever the pawn holds
// something in its arms, walking or standing. A rescued patient is drawn
// by drawCarried; the arms pose is the same.
func (r *Renderer) carryPose(p game.PawnView, m moveState) actionPose {
	if p.Carrying == "" {
		return actionPose{}
	}
	a := actionPose{ok: true}
	if m.Blend >= minBlend {
		if s := fmt.Sprintf("_carry_walk%02d", walkIndex(m.Stride)+1); r.hasFrame(s) {
			a.suffix = s
		} // else the plain walk frame stays
	} else if r.hasFrame("_carry") {
		a.suffix = "_carry"
	}
	a.held = heldItem{set: true} // bare hands: the patient is drawn over the rescuer
	if p.Carrying != "pawn" {
		a.held = heldItem{set: true, key: itemSprite(p.Carrying), stance: stanceLoad}
	}
	return a
}

// workPose is the hands-on work layer while standing at a job. It uses the
// action's own pose set, then _kneel for kneeling work, then the generic
// _work frames, then the static pose.
func (r *Renderer) workPose(p game.PawnView, m moveState) actionPose {
	_, d, ok := actionOf(p)
	if !ok || m.Blend >= minBlend {
		return actionPose{}
	}
	t := r.animT()
	ph := workPhase(t, uint64(p.ID), d.period)
	a := actionPose{ok: true}
	switch {
	case d.set != "" && r.hasFrame(d.set+"1"):
		a.suffix = setFrame(d.set, d.frames, d.strike, ph)
	case d.kneel && r.hasFrame("_kneel"):
		a.suffix = "_kneel"
	case d.fallback == motionSwing || d.fallback == motionReach:
		a.suffix = workFrame(ph)
	}
	k := workSwing(t, uint64(p.ID), d.period)
	ux, uy := unit(float64(p.ActX-p.X), float64(p.ActY-p.Y))
	a.squash = d.squash * (0.7 + 0.3*clamp01(k)) // held low, easing deeper on the strike
	a.dx, a.dy, a.rot = ux*d.reach*k, uy*d.reach*k, ux*d.lean*k
	a.held = heldItem{set: true, key: d.tool, stance: d.stance, off: d.off} // the tool, or bare hands
	return a
}
