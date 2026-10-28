package architect

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Sim is the simulation's read-only placement, zone and area rules the
// tools judge a drag with (consumer-owned; the game implements it).
type Sim interface {
	// CanPlaceAt is the placement rule (material, facing, every footprint
	// cell) and why it refuses.
	CanPlaceAt(kind, stuff, facing string, x, y int) (bool, string)
	// ReplacesAt names the wall or fence a door or gate would replace, or
	// the material swap ("wood wall → steel"); "" when none.
	ReplacesAt(kind, stuff string, x, y int) string
	// AutoFacingAt is the facing the sim would store for a piece there.
	AutoFacingAt(kind, facing string, x, y int) string
	// CanZoneAt reports a zone of kind could claim the tile.
	CanZoneAt(kind string, x, y int) bool
	// Areas are the colony's allowed areas.
	Areas() []game.AreaView
	// TemperatureAt is a tile's air temperature in tenths of a degree C.
	TemperatureAt(x, y int) int
	// Buildables is the building catalog with the scenario's locks.
	Buildables() []game.BuildableView
}

// ZoneRefuser is the optional rule behind a refused zone cell (PLAY-023):
// why a zone of kind can't claim a tile, "" when it can.
type ZoneRefuser interface {
	ZoneRefusal(kind string, x, y int) string
}

// AreaReader is the optional per-pawn allowed-area ref: -1 unrestricted,
// -2 Home, else an area id.
type AreaReader interface {
	AreaOf(id entity.EntityID) int
}

// BuildingMover is the optional rule Move mode asks (UI-011).
type BuildingMover interface {
	CanMoveTo(bid entity.EntityID, facing string, x, y int) (bool, string)
}

// Env is what the tool reads of the program around it, as of the call.
type Env struct {
	Sim      Sim              // the colony's simulation (nil off the colony)
	Art      *art.Catalog     // nil while the art is missing
	Sel      *selection.Model // what is selected (zones, buildings)
	Keys     input.Bindings   // the live key bindings
	Window   string           // the open main-tab window ("work", "schedule", …)
	WinOpen  bool             // a main-tab window is open
	UIHidden bool             // F11 hid the interface
}

// Host is what the tool needs of the program that no ports role carries.
type Host interface {
	// Do takes the CloseWindows intent when the Architect opens.
	ports.Dispatcher
	// Env is the program as of now.
	Env() Env
	// ChromeHit reports the logical point is on the HUD: a click there
	// never reaches the map.
	ChromeHit(mx, my, sw, sh int) bool
	// Layout is this frame's colony HUD layout (the category panel, the
	// description box, the command grid).
	Layout() hudlayout.Layout
	// CloseFloat closes the float menu (a tool takes the map mouse).
	CloseFloat()
	// SyncOverlays brings the overlays a held tool forces (Tool.Forces*)
	// up to date after the tool in hand changed.
	SyncOverlays()
	// LockedBy is the research project key needs ("build:<kind>",
	// "crop:<name>"), "" when it is free.
	LockedBy(key string) string
	// BuildingTitle is a building's name as its inspector shows it.
	BuildingTitle(b game.BuildingView) string
}

// Map is the renderer's side of the map the tools draw on and read
// (physical pixels, the renderer's camera).
type Map interface {
	// DrawAreaGhost outlines an area tool's rectangle; roof adds the red
	// marks on cells a roof cannot be supported over.
	DrawAreaGhost(dst *ebiten.Image, x0, y0, x1, y1 int, roof bool)
	// DrawPlacementGhost previews a build tool's cells with their sprites.
	DrawPlacementGhost(dst *ebiten.Image, kind, stuff, facing string, cells []GhostCell)
	// DrawRangeRing rings a placement's radius around tile (x, y).
	DrawRangeRing(dst *ebiten.Image, x, y int, radius float64, c color.NRGBA)
	// DrawPowerLink draws the link a ghost would make, in its net's colour.
	DrawPowerLink(dst *ebiten.Image, from, to [2]int, c color.NRGBA, tile float64)
	// DrawZoneMark highlights the selected zone (pw x ph map pixels) and,
	// when it is hidden, its name plate.
	DrawZoneMark(dst *ebiten.Image, z game.ZoneView, pw, ph int, tile float64)
	// ZoneLabelAt is the zone whose name plate is under physical pixel
	// (mx, my); shown is the one hidden zone whose plate still shows.
	ZoneLabelAt(w game.WorldView, mx, my int, tile float64, shown uint16) (game.ZoneView, bool)
	// PlantsOn appends the indexes into plants of the plants on tiles
	// x0..x1 × y0..y1 (the chunk index when plants is the live list).
	PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32
	// Overlays are the map overlays (allowed area, power nets, zone
	// geometry, the forced flags).
	Overlays() *overlay.Set
}

// Shell composes the roles the tool calls with its Host and the map.
type Shell interface {
	ports.Hinter
	ports.Sizer
	ports.Commands
	ports.World
	ports.View
	ports.Widgets
	Host
	Map
}

// Panes is what the legacy panels that host an allowed-area selector hand
// it each Update (the Schedule window's column, the colonist pane's chip):
// their geometry stays theirs.
type Panes interface {
	// Schedule is the open Schedule window's geometry and rows.
	Schedule() Schedule
	// PaneButtons are the inspector pane's buttons this frame.
	PaneButtons() []uikit.Button
	// SelectedPawn is the colonist the pane shows.
	SelectedPawn() (game.PawnView, bool)
}

// Schedule is the Schedule window as its area column reads it.
type Schedule struct {
	Win   uikit.Button // the window's rectangle
	Rows  []uint64     // the row ids (row 0, Everyone, has no column)
	NameW float32      // the name column
	CellW float32      // one hour cell
	Top   float32      // the first row's top below the header
	RowH  float32      // a row's height
}

// Chip is a small status chip of the colonist pane (the allowed area).
type Chip struct{ ID, Label, Tip string }

// ZonePane is what the inspector pane shows for the selected zone.
type ZonePane struct {
	Title, Subtitle string
	Lines           []string
	Close           string // the close button's id
}

// Card is one Architect tool as a command-grid button.
type Card struct {
	ID, Label, Icon, Desc string
	Disabled              bool
	Reason                string
	Toggle, On            bool
}
