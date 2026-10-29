package inspector

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Sim is the simulation's read models the inspector asks for beyond the
// frame's world view (consumer-owned; the game implements it). The bed's
// prisoner mark is an optional capability asserted on it
// (bedPrisonReader).
type Sim interface {
	// OrdersAt lists what a pawn can be ordered to do at a tile (the
	// colonist verbs, Equip nearby).
	OrdersAt(pawn entity.EntityID, x, y int) []game.OrderOption
	// Health, Social and Prisoner are a colonist's tabs (ok false: nothing
	// to show).
	Health(id entity.EntityID) (game.HealthView, bool)
	Social(id entity.EntityID) (game.SocialView, bool)
	Prisoner(id entity.EntityID) (game.PrisonerView, bool)
	// Bio is a colonist's biography; LearnedThisWeek its experience gained
	// in the last 7 days per skill.
	Bio(id entity.EntityID) (game.BioView, bool)
	LearnedThisWeek(id entity.EntityID) [skill.NumSkills]int
	// PlantGrowth is why a plant is or isn't growing now and when it
	// ripens.
	PlantGrowth(id entity.EntityID) (game.PlantGrowthView, bool)
}

// Env is what the inspector reads of the program around it, as of the
// call.
type Env struct {
	Sim    Sim            // the colony's simulation (nil off the colony)
	Art    *art.Catalog   // the App's sprites and icons (nil while missing)
	MapArt *art.Catalog   // the renderer's (nil off the colony)
	Keys   input.Bindings // the live key bindings (the command keys' labels)

	WinOpen  bool // a main-tab window is open: the pane gives way
	ArchOpen bool // the Architect is open: the grid shows its tools
	UIHidden bool // F11 hid the interface

	Updates     uint64  // the Update counter (the group's command memo)
	Shift, Ctrl bool    // the held modifiers (a bill's step)
	WheelY      float64 // this frame's wheel (the Log tab)
}

// Host is what the inspector needs of the program that no ports role
// carries.
type Host interface {
	// Do takes the cross-feature intents: a building's Copy
	// (PresetBuildTool).
	ports.Dispatcher
	// Hint shows why a click did nothing (a disabled command or tab).
	ports.Hinter
	// Env is the program as of now.
	Env() Env
	// Layout is this frame's colony HUD layout (the pane, its tabs, the
	// expanded panel, the command cells). A producer never calls it: the
	// layout calls the producers to size itself.
	Layout() hudlayout.Layout
	// Pick opens the order picker at a logical point (above it when up)
	// and runs pick with the chosen row.
	Pick(x, y float32, up bool, entries []game.OrderOption, pick func(i int))
	// Click runs a legacy command id ("pawn.draft", "bldg.move") through
	// the shell's click routes, as a click on it would.
	Click(id string)
	// Selection is what is selected (the model outlives the colony).
	Selection() *selection.Model
	// Hostiles are the raiders on the map (the Capture command's target,
	// the names in a colonist's lines).
	Hostiles() []game.PawnView
}

// Peers are the other features' entry points the inspector calls: the
// allowed areas, the Move tool and the research locks (the root wires
// them to the Architect and the Research window).
type Peers interface {
	// AreaPicker opens the allowed-area picker (base lists Unrestricted
	// and Home) with cur marked; pick gets the chosen area ref.
	AreaPicker(title string, cur int, base bool, pick func(ref int))
	// AssignArea restricts a pawn or animal to an allowed area.
	AssignArea(id entity.EntityID, ref int)
	// AreaChip is the colonist pane's allowed-area chip (ok false: the
	// game reports no areas).
	AreaChip(p game.PawnView) (Chip, bool)
	// AreaLabel names an allowed-area ref and says what a restriction
	// means (a tame animal's chip).
	AreaLabel(ref int) (label, tip string)
	// Move starts Move mode on a building, or with cancel takes back the
	// move it waits for.
	Move(b game.BuildingView, cancel bool)
	// MoveState says whether a building can be moved and, while a move is
	// pending, the line that says where to.
	MoveState(b game.BuildingView) (movable bool, line string)
	// Buildable reports the Architect has a tool that builds kind.
	Buildable(kind string) bool
	// LockedBy is the research project key needs ("build:<kind>",
	// "crop:<name>", "recipe:<name>"), "" when it is free.
	LockedBy(key string) string
}

// Labels are the names and drawings the root's other packages own: the
// colonist bar's names, the temperature format, the Animals window's wild
// rows and blurbs and the New Colony wizard's biography widgets.
type Labels interface {
	// PawnLabel is a colonist's display name.
	PawnLabel(p game.PawnView) string
	// Temp formats a temperature in tenths of a degree C ("21C").
	Temp(tenths int) string
	// WildRow is the Animals window's row for a wild species (its tame
	// chance).
	WildRow(species string) (game.WildlifeRow, bool)
	// AnimalBlurb is a species' one-line description.
	AnimalBlurb(species string) string
	// Skill is a skill's level text ("12 (burning)") and its hover card.
	Skill(s game.SkillView) (level, tip string)
	// Portrait draws a colonist's portrait in box, or the silhouette of
	// name when the art has no body for the look.
	Portrait(dst *ebiten.Image, l look.Look, scarred bool, box gfx.Rect, name string)
	// Backstory declares a backstory line (label, title, its hover card).
	Backstory(x, y, w float32, label, title, desc, id string)
	// Traits declares the trait chips under y and returns the y below them.
	Traits(x, y, w float32, v game.CandidateView) float32
}

// Shell composes the roles the inspector calls with its Host, the other
// features and the labels.
type Shell interface {
	ports.Sizer
	ports.Commands
	ports.World
	ports.PawnSel
	ports.Widgets
	Host
	Peers
	Labels
}

// Producer is what the inspector shows for one selection kind: its pane
// and its command buttons (either may be nil). New fills the kinds the
// inspector describes itself; the root plugs in the rest (the Architect's
// tools for selection.None, the selected zone).
type Producer struct {
	Pane     func(id entity.EntityID) Spec
	Commands func(id entity.EntityID) []Button
}

// Pane is the selection's inspector (ARCH-005): the pane and its tabs,
// the command grid, and the per-kind producers. The kinds' own code hangs
// off the sub-types (Commands, Pawn, Tabs, Animal, Building, Thing), each
// a view of the same Pane.
type Pane struct {
	shell Shell
	sel   *selection.Model

	// Tab is the open inspector tab ("" the default); other root files read
	// it until the sweep (the lead moved it from App).
	Tab string

	// Producers are each selection kind's pane and commands.
	Producers map[selection.Kind]Producer

	// TestPane and TestCommands replace every kind's producer (tests only;
	// nil in production). selection.None stands for the Architect's tools.
	TestPane     func(k selection.Kind, id entity.EntityID) Spec
	TestCommands func(k selection.Kind, id entity.EntityID) []Button

	openTab   string // id of the open tab ("inspector.needs"); "" collapsed
	lastTitle string // the body scrolls back to the top when the title changes

	memo       groupMemo       // the group's merged commands for one Update
	equipCache equipCacheState // the Equip command's nearby list

	// LogScroll is the pawn Log tab's first row and LogFor the colonist
	// it belongs to (the root's log tests read them).
	LogScroll int
	LogFor    entity.EntityID

	animalLogFor entity.EntityID // the animal whose Log scroll is kept
}

// New builds the inspector over its shell, with the producers of the
// kinds it describes itself.
func New(sh Shell) *Pane {
	p := &Pane{shell: sh, sel: sh.Selection()}
	p.Producers = map[selection.Kind]Producer{
		selection.Pawn:     {Pane: p.Pawn().PawnPane, Commands: p.Pawn().PawnCommands},
		selection.Animal:   {Pane: p.Animal().AnimalPane, Commands: p.Animal().AnimalCommands},
		selection.Item:     {Pane: p.Thing().ItemPane, Commands: p.Thing().ItemCommands},
		selection.Plant:    {Pane: p.Thing().PlantPane, Commands: p.Thing().PlantCommands},
		selection.Building: {Pane: p.Building().BuildingPane, Commands: p.Building().BuildingCommands},
		selection.Frame:    {Pane: p.Thing().FramePane, Commands: p.Thing().FrameCommands},
	}
	return p
}

// The sub-types: each kind's code as a view of the Pane.
type (
	// Commands is the command grid, its keys and options, and the group's
	// merged commands.
	Commands struct{ *Pane }
	// Pawn is a colonist's pane, commands, Needs, Gear and Log tabs.
	Pawn struct{ *Pane }
	// Tabs are a colonist's Health, Social, Bio and Prisoner tabs.
	Tabs struct{ *Pane }
	// Animal is an animal's pane, commands, Log and Training tabs.
	Animal struct{ *Pane }
	// Building is a building's pane and commands: bills, beds, power,
	// pens, troughs and caskets.
	Building struct{ *Pane }
	// Thing is a loose stack's, a plant's and a blueprint's pane and
	// commands.
	Thing struct{ *Pane }
)

func (a *Pane) Commands() Commands { return Commands{a} }
func (a *Pane) Pawn() Pawn         { return Pawn{a} }
func (a *Pane) Tabs() Tabs         { return Tabs{a} }
func (a *Pane) Animal() Animal     { return Animal{a} }
func (a *Pane) Building() Building { return Building{a} }
func (a *Pane) Thing() Thing       { return Thing{a} }

// env is the program as of now.
func (a *Pane) env() Env { return a.shell.Env() }

// sim is the colony's simulation (nil off the colony: every producer
// shows nothing then).
func (a *Pane) sim() Sim { return a.shell.Env().Sim }

// kit is the widget kit.
func (a *Pane) kit() *uikit.Kit { return a.shell.Kit() }
