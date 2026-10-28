package creature

import (
	"math"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Work presentation (ART §27.2): per-action poses, loads and strike
// particles. No tool is drawn (ART-027): the pawn faces the job, leans
// and dips into it, the strike throws the job's particles at the work
// spot, and work_progress.go draws the job's progress over it. The
// dispatch in pawn.go calls in here.

// --- work motion --------------------------------------------------------

// WorkMotion is how an action moves the figure (ART §27.2): the body has no
// arms and holds no tool, so a whole-figure transform carries the work.
type WorkMotion uint8

const (
	MotionNone  WorkMotion = iota
	MotionSwing            // overhead work: wind back, drive in on the strike, recover; the body squashes on the strike
	MotionReach            // low work in front: lean in and dip (tend, plant, pick)
	MotionBench            // at a bench: a small lean and bob
	MotionEat              // the head bobs to the meal
)

// PartAt is where an action's particles spawn.
type PartAt uint8

const (
	AtTarget PartAt = iota // the work spot: the worked tile's face towards the pawn
	AtStove                // the stove's top
	AtFire                 // above a campfire
)

type PartSpec struct {
	Kind  PartKind
	count int
}

// ActionDef is one row of the work table: the motion, its timing, how
// the body leans and what each strike throws up.
type ActionDef struct {
	Motion WorkMotion
	Period int     // animT ticks per loop; the strike lands at strikeAt
	reach  float64 // lean towards the worked tile, tiles
	lean   float64 // body rotation towards the tile at full swing, radians
	Squash float64 // kneel / crouch at full swing, 0..1
	At     PartAt
	Parts  []PartSpec
}

// WorkTable is the single source of truth for hands-on actions (ART
// §27.2). Combat (aim, melee) and carrying (rescue, haul) are other layers.
var WorkTable = map[string]ActionDef{
	game.ActionChop: {Motion: MotionSwing, Period: 46, reach: 0.10, lean: 0.14, Parts: []PartSpec{{PartChip, 4}}},
	game.ActionMine: {Motion: MotionSwing, Period: 50, reach: 0.12, lean: 0.16, Squash: 0.03, Parts: []PartSpec{{PartSpark, 3}, {PartDust, 4}}},
	// Smoothing (SIM-029): the mine swing, shorter and lighter, with dust.
	game.ActionSmooth:      {Motion: MotionSwing, Period: 40, reach: 0.08, lean: 0.10, Squash: 0.03, Parts: []PartSpec{{PartDust, 3}}},
	game.ActionBreach:      {Motion: MotionSwing, Period: 44, reach: 0.12, lean: 0.16, Squash: 0.03, Parts: []PartSpec{{PartSpark, 3}, {PartDust, 4}}},
	game.ActionBuild:       {Motion: MotionSwing, Period: 34, reach: 0.08, lean: 0.10, Parts: []PartSpec{{PartSawdust, 3}}},
	game.ActionDeconstruct: {Motion: MotionSwing, Period: 34, reach: 0.08, lean: 0.10, Parts: []PartSpec{{PartSawdust, 3}}},
	game.ActionRepair:      {Motion: MotionSwing, Period: 38, reach: 0.08, lean: 0.10, Parts: []PartSpec{{PartSawdust, 2}}},
	game.ActionSow:         {Motion: MotionReach, Period: 48, reach: 0.05, lean: 0.06, Squash: 0.14, Parts: []PartSpec{{PartSoil, 3}}},
	game.ActionHarvest:     {Motion: MotionReach, Period: 44, reach: 0.07, lean: 0.06, Squash: 0.10, Parts: []PartSpec{{PartLeaf, 3}}},
	// Sweeping (SIM-021): the low reach and a puff of dust.
	game.ActionClean:    {Motion: MotionReach, Period: 40, reach: 0.06, lean: 0.05, Squash: 0.06, Parts: []PartSpec{{PartDust, 2}}},
	game.ActionCraft:    {Motion: MotionBench, Period: 30, reach: 0.05, lean: 0.03, Parts: []PartSpec{{PartShaving, 2}}},
	game.ActionCook:     {Motion: MotionBench, Period: 40, reach: 0.04, lean: 0.03, At: AtStove, Parts: []PartSpec{{PartSteam, 1}}},
	game.ActionTailor:   {Motion: MotionBench, Period: 44, reach: 0.05, lean: 0.03, Parts: []PartSpec{{PartShaving, 1}}},
	game.ActionButcher:  {Motion: MotionBench, Period: 36, reach: 0.06, lean: 0.04, Parts: []PartSpec{{PartBlood, 2}}},
	game.ActionResearch: {Motion: MotionBench, Period: 60, reach: 0.03, lean: 0.02},
	game.ActionTend:     {Motion: MotionReach, Period: 60, reach: 0.05, lean: 0.06, Squash: 0.16},
	game.ActionTame:     {Motion: MotionReach, Period: 64, reach: 0.04, lean: 0.04, Squash: 0.12},
	// Milking kneels beside the animal; shearing works at its flank (SIM-026).
	game.ActionMilk:  {Motion: MotionReach, Period: 56, reach: 0.04, lean: 0.05, Squash: 0.12},
	game.ActionShear: {Motion: MotionReach, Period: 40, reach: 0.06, lean: 0.05, Squash: 0.08},
	// Eating: nothing in hand, the head bobs to the meal.
	game.ActionEat:    {Motion: MotionEat, Period: 50, reach: 0.02},
	game.ActionRefuel: {Motion: MotionBench, Period: 40, reach: 0.06, lean: 0.04, Parts: []PartSpec{{PartChip, 1}}},
	// Burial (SIM-017): digging swings like mining, kneeling to fill
	// the grave, the bench motion at the pyre.
	game.ActionDig:        {Motion: MotionSwing, Period: 56, reach: 0.10, lean: 0.14, Squash: 0.04, Parts: []PartSpec{{PartSoil, 4}}},
	game.ActionBury:       {Motion: MotionReach, Period: 48, reach: 0.06, lean: 0.05, Squash: 0.12, Parts: []PartSpec{{PartSoil, 3}}},
	game.ActionCremate:    {Motion: MotionBench, Period: 44, reach: 0.04, lean: 0.03, Parts: []PartSpec{{PartEmber, 2}, {PartSmoke, 1}}},
	game.ActionHorseshoes: {Motion: MotionSwing, Period: 70, reach: 0.04, lean: 0.03, Parts: []PartSpec{{PartDust, 3}}},
}

// LegacyActions maps the old Acting strings (a fallback when the sim sets
// no Action) to canonical actions.
var LegacyActions = map[string]string{
	"cut": game.ActionChop, "construct": game.ActionBuild, "deconstruct": game.ActionDeconstruct,
	"mine": game.ActionMine, "smooth": game.ActionSmooth, "harvest": game.ActionHarvest, "sow": game.ActionSow,
	"refuel": game.ActionRefuel, "play horseshoes": game.ActionHorseshoes,
	"tend": game.ActionTend, "eat": game.ActionEat,
	"work at workbench": game.ActionCraft, "work at stove": game.ActionCook,
	"work at campfire": game.ActionCook, "work at tailoring bench": game.ActionTailor,
	"work at butcher table": game.ActionButcher, "work at drying rack": game.ActionCraft,
	"work at research bench": game.ActionResearch,
}

// ActionOf resolves a pawn's hands-on work action and its table row. A
// non-work Action (aim, melee, rescue, haul) has no row.
func ActionOf(p game.PawnView) (string, ActionDef, bool) {
	act := p.Action
	if act == "" {
		act = LegacyActions[p.Acting]
	}
	d, ok := WorkTable[act]
	return act, d, ok
}

// StrikeAt is where in a swing the strike lands (phase 0..1).
const StrikeAt = 0.48

// workPhase is the 0..1 position within the current swing at animT t
// (ticks); each pawn is offset so a crew does not swing in unison.
func workPhase(t float64, id uint64, period int) float64 {
	if period <= 0 {
		return 0
	}
	return math.Mod(t+float64(id*13), float64(period)) / float64(period)
}

// WorkSwing is the body lean for a swing at animT t: back a little while
// winding up, hard forward on the strike, easing out.
func WorkSwing(t float64, id uint64, period int) float64 {
	ph := workPhase(t, id, period)
	switch {
	case ph < 0.40:
		return -0.3 * motion.EaseOut(ph/0.40)
	case ph < StrikeAt:
		return -0.3 + 1.3*motion.EaseOut((ph-0.40)/(StrikeAt-0.40))
	default:
		return 1 - motion.EaseOut((ph-StrikeAt)/(1-StrikeAt))
	}
}

func strikeIndex(t float64, id uint64, period int) float64 {
	return math.Floor((t+float64(id*13))/float64(period) - StrikeAt)
}

// WorkStrikeAt reports whether a strike lands while animT runs from prev
// to cur. A paused clock (prev == cur) never strikes; 3x time strikes 3x
// as often.
func WorkStrikeAt(prev, cur float64, id uint64, period int) bool {
	return period > 0 && strikeIndex(cur, id, period) > strikeIndex(prev, id, period)
}

// AnimateWork spawns each working pawn's strike particles, once per swing.
func (c *Set) AnimateWork(v *render.View, w game.WorldView, pawns []game.PawnView) {
	st := c
	t := v.AnimT // swings run on sim-scaled time
	for _, b := range w.Buildings {
		if b.Kind == "funeral pyre" {
			c.EmitPyre(v, b)
		}
	}
	acting := map[entity.EntityID]bool{}
	for _, p := range pawns {
		if p.Lying || p.CarriedBy != 0 {
			continue
		}
		act, d, ok := ActionOf(p)
		if !ok {
			continue
		}
		acting[p.ID] = true
		prev, seen := st.PrevWork[p.ID]
		st.PrevWork[p.ID] = t
		if !seen || !WorkStrikeAt(prev, t, uint64(p.ID), d.Period) {
			continue
		}
		c.Host.WorkCue(act, float64(p.ActX)+0.5, float64(p.ActY)+0.5)
		if len(d.Parts) == 0 {
			continue
		}
		x, y := workSpot(p)
		switch d.At {
		case AtStove:
			x, y = float64(p.ActX)+0.7, float64(p.ActY)-0.1
			if p.Acting == "work at campfire" {
				x = float64(p.ActX) + 0.5
			}
		case AtFire:
			x, y = float64(p.ActX)+0.5, float64(p.ActY)-0.1
		}
		ux, uy := motion.Unit(float64(p.ActX-p.X), float64(p.ActY-p.Y))
		for i, ps := range d.Parts {
			seed := uint64(p.ID)*7919 + v.Present + uint64(i)*104729
			if ps.Kind == PartSpark { // sparks glance back off the rock
				c.BurstDir(v, ps.Kind, x, y, ps.count, seed, -ux, -uy, 0.02)
				continue
			}
			c.Burst(v, ps.Kind, x, y, ps.count, seed)
		}
	}
	for id := range st.PrevWork {
		if !acting[id] {
			delete(st.PrevWork, id)
		}
	}
}

// WorkFace is how far the work spot sits from the worked tile's centre
// towards the pawn, tiles: the face of the trunk, rock or wall it works.
const WorkFace = 0.25

// swingSquash is a swing's extra squash on the strike.
const swingSquash = 0.04

// workSpot is where a job's particles spawn, in world tiles: the worked
// tile's face towards the pawn (its centre when the pawn stands on it).
func workSpot(p game.PawnView) (float64, float64) {
	ux, uy := motion.Unit(float64(p.ActX-p.X), float64(p.ActY-p.Y))
	return float64(p.ActX) + 0.5 - ux*WorkFace, float64(p.ActY) + 0.45 - uy*WorkFace
}

// dipK is the reach, bench and eat motions' weight at phase ph: 0 between
// strikes, 1 on the strike.
func dipK(ph float64) float64 {
	return 0.5 + 0.5*math.Cos(2*math.Pi*(ph-StrikeAt))
}

// CarryPose is the hauling layer: it applies whenever the pawn carries
// something, walking or standing. The item sits on the body (stanceLoad,
// in front; behind facing north). A rescued patient is drawn by
// drawCarried, over the rescuer's back.
func (c *Set) CarryPose(v *render.View, p game.PawnView, m motion.MoveState) ActionPose {
	if p.Carrying == "" {
		return ActionPose{}
	}
	a := ActionPose{Ok: true}
	a.Held = HeldItem{Set: true} // bare hands: the patient is drawn over the rescuer
	if p.Carrying != "pawn" {
		key, cs := c.Host.StoneItem(c.Host.ItemSprite(p.Carrying), p.Carrying, sprite.NoCS) // stone takes its tint
		if p.CarryingAnimal != "" {                                                         // a carcass shows its species
			key = "item/corpse_" + p.CarryingAnimal + "_v0"
		}
		a.Held = HeldItem{Set: true, Key: key, Stance: StanceLoad, Cs: cs}
	}
	return a
}

// WorkPose is the hands-on work layer while standing at a job: the
// action's motion as a transform towards the worked tile. It holds
// nothing of its own: off duty nothing is drawn, and a raider breaching
// keeps its weapon carried.
func (c *Set) WorkPose(v *render.View, p game.PawnView, m motion.MoveState) ActionPose {
	_, d, ok := ActionOf(p)
	if !ok || m.Blend >= motion.MinBlend {
		return ActionPose{}
	}
	t := v.AnimT
	ph := workPhase(t, uint64(p.ID), d.Period)
	ux, uy := motion.Unit(float64(p.ActX-p.X), float64(p.ActY-p.Y))
	a := ActionPose{Ok: true}
	switch d.Motion {
	case MotionSwing:
		k := WorkSwing(t, uint64(p.ID), d.Period)
		a.Squash = d.Squash*(0.7+0.3*gfx.Clamp01(k)) + swingSquash*gfx.Clamp01(k)
		a.Dx, a.Dy, a.Rot = ux*d.reach*k, uy*d.reach*k, ux*d.lean*k
	case MotionReach:
		k := dipK(ph)
		a.Squash = d.Squash * (0.7 + 0.3*k) // held low, dipping deeper on the strike
		a.Dx, a.Dy, a.Rot = ux*d.reach*k, uy*d.reach*k+0.02*k, ux*d.lean*k
	case MotionBench:
		k := dipK(ph)
		a.Squash = 0.02 * k
		a.Dx, a.Dy, a.Rot = ux*d.reach*k, uy*d.reach*k+0.015*k, ux*d.lean*k
	case MotionEat:
		k := dipK(ph)
		a.Dx, a.Dy = ux*d.reach, uy*d.reach+0.012*k // the head bows to the meal
	}
	return a
}
