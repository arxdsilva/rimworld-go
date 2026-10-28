package orders

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Labels names what the root alone knows how to title.
type Labels interface {
	// BuildingTitle is the name a building shows under ("Granite wall").
	BuildingTitle(game.BuildingView) string
}

// Host is what the feature needs of the program that no ports role
// carries (at most eight methods, SOLID_GO.md §6).
type Host interface {
	// Frame is this Update's input and the Update counter (the search
	// cursor blinks by it).
	Frame() (in *inputframe.Frame, updates uint64)
	// Selection is what is selected; the right button reads the selected
	// colonists and a search result selects through it.
	Selection() *selection.Model
	// Hostiles are the raiders this frame.
	Hostiles() []game.PawnView
	// ChromeHit reports the logical point is on the HUD, so a click there
	// never reaches the map.
	ChromeHit(mx, my int) bool
	// Layout is this frame's HUD layout (the search box sits under the bar).
	Layout() hudlayout.Layout
	// Icon is the UI icon called name (nil while the art is missing).
	Icon(name string) *ebiten.Image
	// OrdersAt is the order menu of colonist id for tile (tx, ty).
	OrdersAt(id entity.EntityID, tx, ty int) []game.OrderOption
	// CenterOnMap pans the camera so the world point is in the middle of
	// the map viewport.
	CenterOnMap(wx, wy float64)
}

// Shell composes the roles the feature calls with its Host.
type Shell interface {
	ports.Hinter
	ports.Sizer
	ports.Commands
	ports.World
	ports.View
	ports.PawnSel
	Labels
	Host
}

// Controller owns the right-click order flow's state (ARCH-005): the
// float menu, the search box and the formation drag.
type Controller struct {
	sh Shell
	// Float is the order or options menu over the map.
	Float Float
	// Search is the Z search box.
	Search Search
	// Form is the drafted colonists' right drag.
	Form Formation
}

// New builds the feature over its shell.
func New(sh Shell) *Controller { return &Controller{sh: sh} }
