// Package actionview is the canonical action words on game.PawnView.Action
// and game.AnimalView.Action (ART §27.2), the action a job reads as while
// its pawn is working, and the fixed work of the jobs whose total does not
// depend on their target. The game names the words game.Action* and fills
// the views from them; the renderer maps each word to a pose set, held
// item and particles, and the simulation never reads them back. Pure: no
// state, no host.
package actionview

import (
	"rimworld-go/internal/application/game/craft"
	rlab "rimworld-go/internal/application/game/research"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	aquarules "rimworld-go/internal/domain/building/aqua"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/world/roof"
)

// The canonical actions.
const (
	Aim         = "aim"   // ranged attack: aim, fire, recover
	Melee       = "melee" // melee attack with the wielded weapon
	Chop        = "chop"
	Mine        = "mine"
	Smooth      = "smooth" // smoothing rock or a stone floor (SIM-029)
	Build       = "build"
	Deconstruct = "deconstruct"
	Repair      = "repair"
	Breach      = "breach"
	Sow         = "sow"
	Harvest     = "harvest"
	Craft       = craft.ActionCraft
	Cook        = craft.ActionCook
	Tailor      = craft.ActionTailor
	Butcher     = craft.ActionButcher
	Research    = "research"
	Tend        = "tend"
	Rescue      = "rescue"
	Tame        = "tame"
	Milk        = "milk"  // milking, or collecting ember oil (SIM-026)
	Shear       = "shear" // shearing wool (SIM-026)
	Eat         = "eat"
	Refuel      = "refuel"
	Haul        = "haul"
	Horseshoes  = "horseshoes"
	Dig         = "dig"   // digging a grave
	Clean       = "clean" // sweeping a Home tile (SIM-021)
	Bury        = "bury"  // filling a grave over a body
	Cremate     = craft.ActionCremate

	// Animal actions.
	Bite     = "bite"
	Kick     = "kick"
	Claw     = "claw"
	Gore     = "gore"
	Headbutt = "headbutt"
	Peck     = "peck"
	Graze    = "graze"
	Flee     = "flee"
)

// Working is the action a job of kind k reads as while its pawn works it
// (Build for construction: the game reads a grave's frame as Dig); "" for
// the kinds that read as no action or that the game decides by state.
func Working(k job.Kind) string {
	switch k {
	case job.Construct, job.Roof, job.Unroof, job.Install:
		return Build
	case job.Deconstruct, job.Uninstall, job.OpenContainer: // AX-1: crating and opening read as taking apart
		return Deconstruct
	case job.Cut:
		return Chop
	case job.Mine:
		return Mine
	case job.Smooth:
		return Smooth
	case job.Sow:
		return Sow
	case job.Harvest:
		return Harvest
	case job.Eat:
		return Eat
	case job.Horseshoes:
		return Horseshoes
	case job.Research, job.ScanGround, job.ScanMinerals:
		return Research
	case job.Tame:
		return Tame
	case job.Milk, job.StockTank, job.NetTank: // FISH-*: hands at an animal's side
		return Milk
	case job.Shear:
		return Shear
	case job.Repair:
		return Repair
	case job.Breach:
		return Breach
	case job.Clean, job.CleanRoom, job.ClearSnow: // game/snow: shovelling reads as sweeping
		return Clean
	}
	return ""
}

// FixedWork is the full work of a job of kind k when its content def fixes
// it whatever the target; ok is false for every other kind.
func FixedWork(k job.Kind) (work int, ok bool) {
	switch k {
	case job.Research, job.OperateDrill, job.ScanGround, job.ScanMinerals:
		return rlab.Session, true
	case job.Tame:
		return animal.TameWork, true
	case job.Milk, job.Shear:
		return animal.CollectWork, true
	case job.StockTank:
		return aquarules.StockWork, true
	case job.NetTank:
		return aquarules.NetWork, true
	case job.Eat:
		return needs.EatWorkTicks, true
	case job.Roof:
		return roof.BuildWork, true
	case job.Unroof:
		return roof.RemoveWork, true
	}
	return 0, false
}
