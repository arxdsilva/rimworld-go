package ebiten

import (
	"math"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Work presentation (ART §27.2): per-action poses, held tools, loads
// and strike particles. The dispatch in pawn_anim.go calls in here.

// --- work motion --------------------------------------------------------

// motion is how an action moves the figure and its tool (ART §27.2): the
// body has no arms, so a whole-figure transform and the tool's turn about
// its grip carry the work.
type motion uint8

const (
	motionNone  motion = iota
	motionSwing        // overhead: tool raised, driven down, recovered; the body squashes on the strike
	motionReach        // low work in front: lean in and dip (tend, plant, pick)
	motionBench        // hands at a bench: a small lean and bob
	motionEat          // the spoon goes to the mouth and back, the head bobs to it
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

// actionDef is one row of the work table: the motion, its timing, what
// is held, how the body leans and what each strike throws up.
type actionDef struct {
	motion motion
	period int     // animT ticks per loop; the strike lands at strikeAt
	reach  float64 // lean towards the worked tile, tiles
	lean   float64 // body rotation towards the tile at full swing, radians
	squash float64 // kneel / crouch at full swing, 0..1
	tool   string  // held sprite key; "" is bare hands
	stance heldStance
	off    string // sprite key held level under the tool (bowl)
	at     partAt
	parts  []partSpec
}

// workTable is the single source of truth for hands-on actions (ART
// §27.2). Combat (aim, melee) and carrying (rescue, haul) are other layers.
var workTable = map[string]actionDef{
	game.ActionChop: {motion: motionSwing, period: 46, reach: 0.10, lean: 0.14, tool: "tool/axe", stance: stanceHands, parts: []partSpec{{partChip, 4}}},
	game.ActionMine: {motion: motionSwing, period: 50, reach: 0.12, lean: 0.16, squash: 0.03, tool: "tool/pickaxe", stance: stanceHands, parts: []partSpec{{partSpark, 3}, {partDust, 4}}},
	// Smoothing (SIM-029): the mine swing, shorter and lighter, with dust.
	game.ActionSmooth:      {motion: motionSwing, period: 40, reach: 0.08, lean: 0.10, squash: 0.03, tool: "tool/pickaxe", stance: stanceHands, parts: []partSpec{{partDust, 3}}},
	game.ActionBreach:      {motion: motionSwing, period: 44, reach: 0.12, lean: 0.16, squash: 0.03, tool: "tool/pickaxe", stance: stanceHands, parts: []partSpec{{partSpark, 3}, {partDust, 4}}},
	game.ActionBuild:       {motion: motionSwing, period: 34, reach: 0.08, lean: 0.10, tool: "tool/hammer", stance: stanceHands, parts: []partSpec{{partSawdust, 3}}},
	game.ActionDeconstruct: {motion: motionSwing, period: 34, reach: 0.08, lean: 0.10, tool: "tool/hammer", stance: stanceHands, parts: []partSpec{{partSawdust, 3}}},
	game.ActionRepair:      {motion: motionSwing, period: 38, reach: 0.08, lean: 0.10, tool: "tool/hammer", stance: stanceHands, parts: []partSpec{{partSawdust, 2}}},
	game.ActionSow:         {motion: motionReach, period: 48, reach: 0.05, lean: 0.06, squash: 0.14, tool: "tool/trowel", stance: stanceUpright, at: atHand, parts: []partSpec{{partSoil, 3}}},
	game.ActionHarvest:     {motion: motionReach, period: 44, reach: 0.07, lean: 0.06, squash: 0.10, tool: "tool/sickle", stance: stanceUpright, parts: []partSpec{{partLeaf, 3}}},
	// Sweeping (SIM-021): the low reach with a broom and a puff of dust.
	game.ActionClean:    {motion: motionReach, period: 40, reach: 0.06, lean: 0.05, squash: 0.06, tool: "tool/broom", stance: stanceUpright, parts: []partSpec{{partDust, 2}}},
	game.ActionCraft:    {motion: motionBench, period: 30, reach: 0.05, lean: 0.03, parts: []partSpec{{partShaving, 2}}},
	game.ActionCook:     {motion: motionBench, period: 40, reach: 0.04, lean: 0.03, at: atStove, parts: []partSpec{{partSteam, 1}}},
	game.ActionTailor:   {motion: motionBench, period: 44, reach: 0.05, lean: 0.03, parts: []partSpec{{partShaving, 1}}},
	game.ActionButcher:  {motion: motionBench, period: 36, reach: 0.06, lean: 0.04, parts: []partSpec{{partBlood, 2}}},
	game.ActionResearch: {motion: motionBench, period: 60, reach: 0.03, lean: 0.02},
	game.ActionTend:     {motion: motionReach, period: 60, reach: 0.05, lean: 0.06, squash: 0.16, tool: "tool/bandage", stance: stanceUpright},
	game.ActionTame:     {motion: motionReach, period: 64, reach: 0.04, lean: 0.04, squash: 0.12},
	// Milking kneels beside the animal; shearing works at its flank (SIM-026).
	game.ActionMilk:  {motion: motionReach, period: 56, reach: 0.04, lean: 0.05, squash: 0.12, stance: stanceUpright},
	game.ActionShear: {motion: motionReach, period: 40, reach: 0.06, lean: 0.05, squash: 0.08, stance: stanceUpright},
	// The spoon on hold, lifted to the mouth; the bowl level under it.
	game.ActionEat:    {motion: motionEat, period: 50, reach: 0.02, tool: "tool/spoon", stance: stanceUpright, off: "tool/bowl"},
	game.ActionRefuel: {motion: motionBench, period: 40, reach: 0.06, lean: 0.04, parts: []partSpec{{partChip, 1}}},
	// Burial (SIM-017): the shovel swung like a pickaxe, kneeling
	// bare-handed to fill the grave, the bench motion at the pyre.
	game.ActionDig:        {motion: motionSwing, period: 56, reach: 0.10, lean: 0.14, squash: 0.04, tool: "tool/shovel", stance: stanceHands, parts: []partSpec{{partSoil, 4}}},
	game.ActionBury:       {motion: motionReach, period: 48, reach: 0.06, lean: 0.05, squash: 0.12, parts: []partSpec{{partSoil, 3}}},
	game.ActionCremate:    {motion: motionBench, period: 44, reach: 0.04, lean: 0.03, parts: []partSpec{{partEmber, 2}, {partSmoke, 1}}},
	game.ActionHorseshoes: {motion: motionSwing, period: 70, reach: 0.04, lean: 0.03, parts: []partSpec{{partDust, 3}}},
}

// legacyActions maps the old Acting strings (a fallback when the sim sets
// no Action) to canonical actions.
var legacyActions = map[string]string{
	"cut": game.ActionChop, "construct": game.ActionBuild, "deconstruct": game.ActionDeconstruct,
	"mine": game.ActionMine, "smooth": game.ActionSmooth, "harvest": game.ActionHarvest, "sow": game.ActionSow,
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

// workPhase is the 0..1 position within the current swing at animT t
// (ticks); each pawn is offset so a crew does not swing in unison.
func workPhase(t float64, id uint64, period int) float64 {
	if period <= 0 {
		return 0
	}
	return math.Mod(t+float64(id*13), float64(period)) / float64(period)
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
		act, d, ok := actionOf(p)
		if !ok {
			continue
		}
		acting[p.ID] = true
		prev, seen := st.prevWork[p.ID]
		st.prevWork[p.ID] = t
		if !seen || !workStrikeAt(prev, t, uint64(p.ID), d.period) {
			continue
		}
		if id, ok := actionSounds[act]; ok {
			r.cue(id, float64(p.ActX)+0.5, float64(p.ActY)+0.5, 1)
		}
		if len(d.parts) == 0 {
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

// Swing arc (motionSwing): the tool's turn about its grip, radians,
// positive raising its head.
const (
	swingRest   = 0.3   // held at rest between strikes
	swingRaised = 1.9   // wound up over the shoulder
	swingStrike = -0.55 // driven down into the work at the strike
	swingSquash = 0.04  // the body's extra squash on the strike
)

// swingAngle is the tool's turn at swing phase ph: raised while winding
// up, driven down to swingStrike by strikeAt, recovering to rest.
func swingAngle(ph float64) float64 {
	const windUp = 0.40
	switch {
	case ph < windUp:
		return swingRest + (swingRaised-swingRest)*easeOut(ph/windUp)
	case ph < strikeAt:
		t := (ph - windUp) / (strikeAt - windUp)
		return swingRaised + (swingStrike-swingRaised)*t*t
	}
	return swingStrike + (swingRest-swingStrike)*easeOut((ph-strikeAt)/(1-strikeAt))
}

// dipK is the reach, bench and eat motions' weight at phase ph: 0 between
// strikes, 1 on the strike.
func dipK(ph float64) float64 {
	return 0.5 + 0.5*math.Cos(2*math.Pi*(ph-strikeAt))
}

// carryPose is the arms-full layer: it applies whenever the pawn holds
// something in its arms, walking or standing. A rescued patient is drawn
// by drawCarried, over the rescuer's back.
func (r *Renderer) carryPose(p game.PawnView, m moveState) actionPose {
	if p.Carrying == "" {
		return actionPose{}
	}
	a := actionPose{ok: true}
	a.held = heldItem{set: true} // bare hands: the patient is drawn over the rescuer
	if p.Carrying != "pawn" {
		key, cs := r.stoneItem(itemSprite(p.Carrying), p.Carrying, noCS) // stone takes its tint
		if p.CarryingAnimal != "" {                                      // a carcass shows its species
			key = "item/corpse_" + p.CarryingAnimal + "_v0"
		}
		a.held = heldItem{set: true, key: key, stance: stanceLoad, cs: cs}
	}
	return a
}

// workPose is the hands-on work layer while standing at a job: the
// action's motion as a transform towards the worked tile and a turn of
// its tool.
func (r *Renderer) workPose(p game.PawnView, m moveState) actionPose {
	_, d, ok := actionOf(p)
	if !ok || m.Blend >= minBlend {
		return actionPose{}
	}
	t := r.animT()
	ph := workPhase(t, uint64(p.ID), d.period)
	ux, uy := unit(float64(p.ActX-p.X), float64(p.ActY-p.Y))
	a := actionPose{ok: true}
	a.held = heldItem{set: true, key: d.tool, stance: d.stance, off: d.off} // the tool, or bare hands
	switch d.motion {
	case motionSwing:
		k := workSwing(t, uint64(p.ID), d.period)
		a.squash = d.squash*(0.7+0.3*clamp01(k)) + swingSquash*clamp01(k)
		a.dx, a.dy, a.rot = ux*d.reach*k, uy*d.reach*k, ux*d.lean*k
		a.held.angle = swingAngle(ph)
	case motionReach:
		k := dipK(ph)
		a.squash = d.squash * (0.7 + 0.3*k) // held low, dipping deeper on the strike
		a.dx, a.dy, a.rot = ux*d.reach*k, uy*d.reach*k+0.02*k, ux*d.lean*k
		a.held.angle = 0.5 * k // the tool's head reaches into the work
	case motionBench:
		k := dipK(ph)
		a.squash = 0.02 * k
		a.dx, a.dy, a.rot = ux*d.reach*k, uy*d.reach*k+0.015*k, ux*d.lean*k
	case motionEat:
		k := dipK(ph)
		a.held.lift = 0.6 * k                       // the hand stops short of covering the face
		a.dx, a.dy = ux*d.reach, uy*d.reach+0.012*k // the head bows to the spoon
	}
	return a
}
