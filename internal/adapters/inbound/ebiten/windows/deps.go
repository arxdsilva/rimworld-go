package windows

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/planet"
)

// Reader is the simulation's read models the windows show beyond the
// frame's world view (consumer-owned; the game implements it). The
// optional readers (WorldSource, the Assign tab's policy and care
// readers) are type assertions on it.
type Reader interface {
	Wildlife() []game.WildlifeRow
	History() []game.HistoryEntryView
	WorldMap() game.WorldMapView
	IncapableReason(id entity.EntityID, t work.Type) string
}

// Env is what the windows read of the program around them, as of the call.
type Env struct {
	Game   Reader         // nil off the colony (no renderer)
	Art    *art.Catalog   // the sprite and icon catalog
	Planet *planet.Planet // the planet the colony was founded on (nil: none kept)
}

// Faction is one row of the colony's Factions table as the window's
// detail shows it: the shell builds the rows (the table is the New Colony
// wizard's) in the table's order.
type Faction struct {
	Name, TypeLabel, Tech string
	Goodwill              int
	Relation              faction.Relation
	Standing              string     // the relation's word ("Hostile")
	Color                 color.RGBA // the relation's colour
	Tip                   string     // what the relation means for goodwill
	History               []int      // goodwill a day, oldest first
	Events                []string   // the last events that moved goodwill, oldest first
}

// Labels names and draws what the root alone knows how to present.
type Labels interface {
	// PawnLabel is a colonist's display name.
	PawnLabel(game.PawnView) string
	// AnimalLabel is an animal's species label ("yorkshire terrier").
	AnimalLabel(game.AnimalView) string
	// AnimalHealth is the Health cell ("62%", "62%!" while it bleeds).
	AnimalHealth(game.AnimalView) string
	// CareShort is a care level's short word ("Herbal").
	CareShort(medcare.Policy) string
	// Caravan is a caravan's hover card (every member's health) and its
	// health note for the list line (sick and hurt members first).
	Caravan(game.CaravanView) (tip, note string)
	// Factions are the Factions table's rows, the Swarm last.
	Factions() []Faction
	// DrawAnimalIcon draws an animal's species icon (or its initial).
	DrawAnimalIcon(dst *ebiten.Image, an game.AnimalView, x, y, size float64)
	// DrawPassion draws the passion glyph of level p (1 minor, 2 major),
	// or stars at text baseline ty while the icon is missing.
	DrawPassion(dst *ebiten.Image, p int, x, y, size float64, st gfx.TextStyle, ty float64)
}

// Host is what the feature needs of the program that no ports role
// carries (at most eight methods, SOLID_GO.md §6).
type Host interface {
	// Env is the program as of now.
	Env() Env
	// WindowRect is the HUD layout's Window rect (zero while no window is
	// open). The layout reads Set.Want, so Want never calls it.
	WindowRect() gfx.Rect
	// CenterOnMap pans the camera so the world point is in the middle of
	// the map viewport.
	CenterOnMap(wx, wy float64)
	// AreaDropdown declares the allowed-area selector in r showing ref;
	// pick gets the chosen ref.
	AreaDropdown(id string, r gfx.Rect, ref int, pick func(ref int))
	// ManageAreas opens the area manager.
	ManageAreas()
	// Float opens the order menu at (x, y) with opts; nil opts closes it.
	Float(x, y float32, opts []game.OrderOption)
	// FormCaravan opens the Form caravan dialog over the World tab;
	// globePick reports the tile last picked on its globe (-1 none).
	FormCaravan(globePick func() int)
	// FactionsTable declares the Factions table over Labels.Factions at
	// the top of body, at most maxH tall (more rows scroll), and returns
	// the rest of body; sel is the picked row.
	FactionsTable(body gfx.Rect, maxH float32, sel *int) (rest gfx.Rect)
}

// Shell composes the roles the feature calls with its Host.
type Shell interface {
	ports.Sizer
	ports.Commands
	ports.World
	ports.PawnSel
	ports.Widgets
	ports.Dispatcher
	Labels
	Host
}

// Set is the colony's main-tab windows (ARCH-005): which one is open, the
// kit window's definition, and the windows' own state. Each window is a
// view type over it (Work, Schedule, Assign, Animals, Research, History,
// Factions, World).
type Set struct {
	sh Shell

	// The state other root files read until the sweep repoints them to
	// methods (ARCH-005 wave 5.3; the lead moved these from App).
	Open  bool              // a main-tab window is open
	Mode  string            // which: "work", "schedule", "research", "animals", or a kit window's id
	Wild  bool              // the Animals tab shows the Wild list
	Brush schedule.Activity // the activity the schedule paints

	// modal is the open kit window's definition. It counts as open only
	// while Open is set and Mode is its id (KitOpen): Escape and a
	// replacing tab click change those, never this field.
	modal *uikit.Modal

	lastPainted string // the schedule cell the drag painted last

	// The Wild list, read again only when the simulation tick moves.
	wildRows []game.WildlifeRow
	wildTick clock.Tick
	wildRead bool

	// focus cycles a Wild row's clicks through the animals of its species.
	focus map[string]int

	// storage is the Storage settings window's state (storage.go).
	storage *storageState

	histTab int // the History window's tab: 0 events, 1 combat
}

// New builds the feature over its shell.
func New(sh Shell) *Set { return &Set{sh: sh, storage: &storageState{}} }

// The windows, each a view over the set.
type (
	// Work is the Work tab: work priorities per colonist.
	Work struct{ *Set }
	// Schedule is the Schedule tab: the 24 hours per colonist.
	Schedule struct{ *Set }
	// Assign is the Assign tab: drug policy and medical care.
	Assign struct{ *Set }
	// Animals is the Animals tab and its Wild list.
	Animals struct{ *Set }
	// Research is the Research tab: the project list.
	Research struct{ *Set }
	// History is the History tab: the colony's notable events.
	History struct{ *Set }
	// Factions is the Factions tab: the table and goodwill history.
	Factions struct{ *Set }
	// World is the World tab: the planet and the caravans.
	World struct{ *Set }
	// Storage is the storage zone settings window (PRD-4).
	Storage struct{ *Set }
)

func (w *Set) Work() Work         { return Work{w} }
func (w *Set) Schedule() Schedule { return Schedule{w} }
func (w *Set) Assign() Assign     { return Assign{w} }
func (w *Set) Animals() Animals   { return Animals{w} }
func (w *Set) Research() Research { return Research{w} }
func (w *Set) History() History   { return History{w} }
func (w *Set) Factions() Factions { return Factions{w} }
func (w *Set) World() World       { return World{w} }
func (w *Set) Storage() Storage   { return Storage{w} }

func (w *Set) kit() *uikit.Kit { return w.sh.Kit() }

// live reports a colony is up (the windows draw nothing on a menu).
func (w *Set) live() bool { return w.sh.Env().Game != nil }

// drawButtons draws hand-placed buttons, hovered under the pointer.
func drawButtons(screen *ebiten.Image, buttons []uikit.Button, mx, my int) {
	for _, b := range buttons {
		uikit.DrawButton(screen, b, b.Contains(mx, my))
	}
}
