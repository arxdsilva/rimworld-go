package game

// Canonical actions on PawnView.Action / AnimalView.Action (ART §27.2).
// The renderer maps each to a pose set, held item and particles; the
// simulation never reads them back.
const (
	ActionAim         = "aim"   // ranged attack: aim, fire, recover
	ActionMelee       = "melee" // melee attack with the wielded weapon
	ActionChop        = "chop"
	ActionMine        = "mine"
	ActionBuild       = "build"
	ActionDeconstruct = "deconstruct"
	ActionRepair      = "repair"
	ActionBreach      = "breach"
	ActionSow         = "sow"
	ActionHarvest     = "harvest"
	ActionCraft       = "craft"
	ActionCook        = "cook"
	ActionTailor      = "tailor"
	ActionButcher     = "butcher"
	ActionResearch    = "research"
	ActionTend        = "tend"
	ActionRescue      = "rescue"
	ActionTame        = "tame"
	ActionEat         = "eat"
	ActionRefuel      = "refuel"
	ActionHaul        = "haul"
	ActionHorseshoes  = "horseshoes"
	ActionDig         = "dig"     // digging a grave
	ActionBury        = "bury"    // filling a grave over a body
	ActionCremate     = "cremate" // burning a body on a pyre

	// Animal actions.
	ActionBite  = "bite"
	ActionKick  = "kick"
	ActionGraze = "graze"
	ActionFlee  = "flee"
)
