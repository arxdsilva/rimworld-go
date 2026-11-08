package game

import "rimworld-go/internal/application/game/actionview"

// Canonical actions on PawnView.Action / AnimalView.Action (ART §27.2,
// game/actionview). The renderer maps each to a pose set, held item and
// particles; the simulation never reads them back.
const (
	ActionAim         = actionview.Aim
	ActionMelee       = actionview.Melee
	ActionChop        = actionview.Chop
	ActionMine        = actionview.Mine
	ActionSmooth      = actionview.Smooth
	ActionBuild       = actionview.Build
	ActionDeconstruct = actionview.Deconstruct
	ActionRepair      = actionview.Repair
	ActionBreach      = actionview.Breach
	ActionSow         = actionview.Sow
	ActionHarvest     = actionview.Harvest
	ActionCraft       = actionview.Craft
	ActionCook        = actionview.Cook
	ActionTailor      = actionview.Tailor
	ActionButcher     = actionview.Butcher
	ActionResearch    = actionview.Research
	ActionTend        = actionview.Tend
	ActionRescue      = actionview.Rescue
	ActionTame        = actionview.Tame
	ActionMilk        = actionview.Milk
	ActionShear       = actionview.Shear
	ActionEat         = actionview.Eat
	ActionRefuel      = actionview.Refuel
	ActionHaul        = actionview.Haul
	ActionHorseshoes  = actionview.Horseshoes
	ActionDig         = actionview.Dig
	ActionClean       = actionview.Clean
	ActionBury        = actionview.Bury
	ActionCremate     = actionview.Cremate

	// Animal actions.
	ActionBite     = actionview.Bite
	ActionKick     = actionview.Kick
	ActionClaw     = actionview.Claw
	ActionGore     = actionview.Gore
	ActionHeadbutt = actionview.Headbutt
	ActionPeck     = actionview.Peck
	ActionGraze    = actionview.Graze
	ActionFlee     = actionview.Flee
)
