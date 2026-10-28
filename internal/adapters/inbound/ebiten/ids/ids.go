// Package ids holds the click and widget ids the ebiten client's screens,
// HUD regions and feature panels share, so a feature can name another's
// button without importing it, and the root's click router can be filled
// by registration (SOLID_GO.md §4: one prefix dispatch per boundary).
// An id private to one feature stays a literal in that feature.
package ids

// Prefixes route a click to the feature that owns the id (app.go
// clickRoutes). The text after the prefix is the feature's own.
const (
	PrefixLetter       = "letter."
	PrefixSpeed        = "speed."
	PrefixLearn        = "learn."
	PrefixRes          = "res."
	PrefixCmd          = "cmd."
	PrefixWin          = "win."
	PrefixGroup        = "group."
	PrefixSearch       = "search."
	PrefixDev          = "dev."
	PrefixBarPawn      = "bar.pawn."
	PrefixResearchPick = "research.pick."
	PrefixAnimals      = "animals."
	PrefixSchedBrush   = "sched.brush."
	PrefixSchedCell    = "sched.cell."
	PrefixArchCat      = "arch.cat."
	PrefixArchStuff    = "arch.stuff."
	PrefixArchCrop     = "arch.crop."
	PrefixArchTool     = "arch.tool."
	PrefixFloat        = "float."
	PrefixLoadSlot     = "load.slot."
	PrefixAnimal       = "animal."
	PrefixPlant        = "plant."
	PrefixItem         = "item."
	PrefixFrame        = "frame."
	PrefixZone         = "zone."
	PrefixMsg          = "msg."
	PrefixAlert        = "alert."
	PrefixBldg         = "bldg."
	PrefixBill         = "bill."
	PrefixWorkCell     = "work.cell."
	PrefixOptions      = "options."
	PrefixInspector    = "inspector."
)

// Exact ids more than one feature names.
const (
	TabWork      = "tab.work"
	TabSchedule  = "tab.schedule"
	TabResearch  = "tab.research"
	TabAnimals   = "tab.animals"
	TabWildlife  = "tab.wildlife"
	TabArchitect = "tab.architect"
	TabMenu      = "tab.menu"
	TabWorld     = "tab.world"
	TabHistory   = "tab.history"
	TabAssign    = "tab.assign"
	TabFactions  = "tab.factions"

	MenuNewGame  = "menu.newgame"
	MenuLoad     = "menu.load"
	MenuContinue = "menu.continue"
	MenuOptions  = "menu.options"
	MenuCredits  = "menu.credits"
	MenuQuit     = "menu.quit"

	PauseResume   = "pause.resume"
	PauseSave     = "pause.save"
	PauseSaveQuit = "pause.savequit"
	PauseLoad     = "pause.load"
	PauseOptions  = "pause.options"
	PauseMenu     = "pause.menu"
	PauseQuit     = "pause.quit"

	OptionsBack = "options.back"

	WizardNext  = "wizard.next"
	WizardBack  = "wizard.back"
	WizardStart = "wizard.start"

	PawnDraft       = "pawn.draft"
	PawnClearOrders = "pawn.clearorders"

	InspectorClose = "inspector.close"
)
