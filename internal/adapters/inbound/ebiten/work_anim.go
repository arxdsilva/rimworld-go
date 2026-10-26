package ebiten

import (
	"math"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Work presentation (ART §27.2): per-action poses, loads and strike
// particles. No tool is drawn (ART-027): the pawn faces the job, leans
// and dips into it, the strike throws the job's particles at the work
// spot, and work_progress.go draws the job's progress over it. The
// dispatch in pawn_anim.go calls in here.

// --- work motion --------------------------------------------------------

// motion is how an action moves the figure (ART §27.2): the body has no
// arms and holds no tool, so a whole-figure transform carries the work.
type motion uint8

const (
	motionNone  motion = iota
	motionSwing        // overhead work: wind back, drive in on the strike, recover; the body squashes on the strike
	motionReach        // low work in front: lean in and dip (tend, plant, pick)
	motionBench        // at a bench: a small lean and bob
	motionEat          // the head bobs to the meal
)

// partAt is where an action's particles spawn.
type partAt uint8

const (
	atTarget partAt = iota // the work spot: the worked tile's face towards the pawn
	atStove                // the stove's top
	atFire                 // above a campfire
)

type partSpec struct {
	kind  partKind
	count int
}

// actionDef is one row of the work table: the motion, its timing, how
// the body leans and what each strike throws up.
type actionDef struct {
	motion motion
	period int     // animT ticks per loop; the strike lands at strikeAt
	reach  float64 // lean towards the worked tile, tiles
	lean   float64 // body rotation towards the tile at full swing, radians
	squash float64 // kneel / crouch at full swing, 0..1
	at     partAt
	parts  []partSpec
}

// workTable is the single source of truth for hands-on actions (ART
// §27.2). Combat (aim, melee) and carrying (rescue, haul) are other layers.
var workTable = map[string]actionDef{
	game.ActionChop: {motion: motionSwing, period: 46, reach: 0.10, lean: 0.14, parts: []partSpec{{partChip, 4}}},
	game.ActionMine: {motion: motionSwing, period: 50, reach: 0.12, lean: 0.16, squash: 0.03, parts: []partSpec{{partSpark, 3}, {partDust, 4}}},
	// Smoothing (SIM-029): the mine swing, shorter and lighter, with dust.
	game.ActionSmooth:      {motion: motionSwing, period: 40, reach: 0.08, lean: 0.10, squash: 0.03, parts: []partSpec{{partDust, 3}}},
	game.ActionBreach:      {motion: motionSwing, period: 44, reach: 0.12, lean: 0.16, squash: 0.03, parts: []partSpec{{partSpark, 3}, {partDust, 4}}},
	game.ActionBuild:       {motion: motionSwing, period: 34, reach: 0.08, lean: 0.10, parts: []partSpec{{partSawdust, 3}}},
	game.ActionDeconstruct: {motion: motionSwing, period: 34, reach: 0.08, lean: 0.10, parts: []partSpec{{partSawdust, 3}}},
	game.ActionRepair:      {motion: motionSwing, period: 38, reach: 0.08, lean: 0.10, parts: []partSpec{{partSawdust, 2}}},
	game.ActionSow:         {motion: motionReach, period: 48, reach: 0.05, lean: 0.06, squash: 0.14, parts: []partSpec{{partSoil, 3}}},
	game.ActionHarvest:     {motion: motionReach, period: 44, reach: 0.07, lean: 0.06, squash: 0.10, parts: []partSpec{{partLeaf, 3}}},
	// Sweeping (SIM-021): the low reach and a puff of dust.
	game.ActionClean:    {motion: motionReach, period: 40, reach: 0.06, lean: 0.05, squash: 0.06, parts: []partSpec{{partDust, 2}}},
	game.ActionCraft:    {motion: motionBench, period: 30, reach: 0.05, lean: 0.03, parts: []partSpec{{partShaving, 2}}},
	game.ActionCook:     {motion: motionBench, period: 40, reach: 0.04, lean: 0.03, at: atStove, parts: []partSpec{{partSteam, 1}}},
	game.ActionTailor:   {motion: motionBench, period: 44, reach: 0.05, lean: 0.03, parts: []partSpec{{partShaving, 1}}},
	game.ActionButcher:  {motion: motionBench, period: 36, reach: 0.06, lean: 0.04, parts: []partSpec{{partBlood, 2}}},
	game.ActionResearch: {motion: motionBench, period: 60, reach: 0.03, lean: 0.02},
	game.ActionTend:     {motion: motionReach, period: 60, reach: 0.05, lean: 0.06, squash: 0.16},
	game.ActionTame:     {motion: motionReach, period: 64, reach: 0.04, lean: 0.04, squash: 0.12},
	// Milking kneels beside the animal; shearing works at its flank (SIM-026).
	game.ActionMilk:  {motion: motionReach, period: 56, reach: 0.04, lean: 0.05, squash: 0.12},
	game.ActionShear: {motion: motionReach, period: 40, reach: 0.06, lean: 0.05, squash: 0.08},
	// Eating: nothing in hand, the head bobs to the meal.
	game.ActionEat:    {motion: motionEat, period: 50, reach: 0.02},
	game.ActionRefuel: {motion: motionBench, period: 40, reach: 0.06, lean: 0.04, parts: []partSpec{{partChip, 1}}},
	// Burial (SIM-017): digging swings like mining, kneeling to fill
	// the grave, the bench motion at the pyre.
	game.ActionDig:        {motion: motionSwing, period: 56, reach: 0.10, lean: 0.14, squash: 0.04, parts: []partSpec{{partSoil, 4}}},
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

// strikeAt is where in a swing the strike lands (phase 0..1).
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

// workStrikeAt reports whether a strike lands while animT runs from prev
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
		x, y := workSpot(p)
		switch d.at {
		case atStove:
			x, y = float64(p.ActX)+0.7, float64(p.ActY)-0.1
			if p.Acting == "work at campfire" {
				x = float64(p.ActX) + 0.5
			}
		case atFire:
			x, y = float64(p.ActX)+0.5, float64(p.ActY)-0.1
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

// workFace is how far the work spot sits from the worked tile's centre
// towards the pawn, tiles: the face of the trunk, rock or wall it works.
const workFace = 0.25

// swingSquash is a swing's extra squash on the strike.
const swingSquash = 0.04

// workSpot is where a job's particles spawn, in world tiles: the worked
// tile's face towards the pawn (its centre when the pawn stands on it).
func workSpot(p game.PawnView) (float64, float64) {
	ux, uy := unit(float64(p.ActX-p.X), float64(p.ActY-p.Y))
	return float64(p.ActX) + 0.5 - ux*workFace, float64(p.ActY) + 0.45 - uy*workFace
}

// dipK is the reach, bench and eat motions' weight at phase ph: 0 between
// strikes, 1 on the strike.
func dipK(ph float64) float64 {
	return 0.5 + 0.5*math.Cos(2*math.Pi*(ph-strikeAt))
}

// carryPose is the hauling layer: it applies whenever the pawn carries
// something, walking or standing. The item sits on the body (stanceLoad,
// in front; behind facing north). A rescued patient is drawn by
// drawCarried, over the rescuer's back.
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
// action's motion as a transform towards the worked tile. It holds
// nothing of its own: off duty nothing is drawn, and a raider breaching
// keeps its weapon carried.
func (r *Renderer) workPose(p game.PawnView, m moveState) actionPose {
	_, d, ok := actionOf(p)
	if !ok || m.Blend >= minBlend {
		return actionPose{}
	}
	t := r.animT()
	ph := workPhase(t, uint64(p.ID), d.period)
	ux, uy := unit(float64(p.ActX-p.X), float64(p.ActY-p.Y))
	a := actionPose{ok: true}
	switch d.motion {
	case motionSwing:
		k := workSwing(t, uint64(p.ID), d.period)
		a.squash = d.squash*(0.7+0.3*clamp01(k)) + swingSquash*clamp01(k)
		a.dx, a.dy, a.rot = ux*d.reach*k, uy*d.reach*k, ux*d.lean*k
	case motionReach:
		k := dipK(ph)
		a.squash = d.squash * (0.7 + 0.3*k) // held low, dipping deeper on the strike
		a.dx, a.dy, a.rot = ux*d.reach*k, uy*d.reach*k+0.02*k, ux*d.lean*k
	case motionBench:
		k := dipK(ph)
		a.squash = 0.02 * k
		a.dx, a.dy, a.rot = ux*d.reach*k, uy*d.reach*k+0.015*k, ux*d.lean*k
	case motionEat:
		k := dipK(ph)
		a.dx, a.dy = ux*d.reach, uy*d.reach+0.012*k // the head bows to the meal
	}
	return a
}
