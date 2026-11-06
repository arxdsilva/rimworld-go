// Package worldview is the colony's full-screen world view (MAP-2,
// MAIN_UI_SCREENS §33, §35, §36): the planet on the globe with every
// world object marked (our maps, caravans, settlements, pods in flight,
// mineral sites), picking, the world inspector with its gizmos, the
// caravan route preview, the transport pods' range ring and landing menu,
// and the pick-spot cursor on the target map. It replaced the modal World
// tab (windows/world*.go). The simulation keeps ticking under it and the
// HUD stays up; the root skips the map draw while it is open.
//
// It reads the game's read models through its own Reader and emits typed
// commands (ports.Commands) and intents (ports.Dispatcher); it never
// reaches the App.
package worldview

import (
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/planet"
)

// Reader is the simulation's read models the view shows (consumer-owned;
// the game implements it). The optional readers below are type
// assertions on it.
type Reader interface {
	WorldMap() game.WorldMapView
	Maps() []game.MapView
	ViewedMap() game.MapID
	Factions() []game.FactionView
}

// WorldSource is the colony's world params (to chart its globe) and the
// settlements of its peoples (optional on Reader; the game implements it).
type WorldSource interface {
	WorldParams() (planet.Params, bool)
	WorldSites() []game.WorldSiteView
}

// Estimator is how long a caravan takes to reach a world tile and why it
// cannot (optional on Reader; MAP-09, game.TravelEstimate).
type Estimator interface {
	TravelEstimate(caravan uint64, tile int) (ticks int, why string)
}

// Launcher is why a transport pod group would not fly to a tile in a
// landing mode, with the distance (optional on Reader; the simulation's
// rule, travel.LaunchReason). Without it the view mirrors the range rule.
type Launcher interface {
	LaunchReason(pod entity.EntityID, dest int, mode uint8) (string, int)
}

// Attacker is why a caravan cannot attack the settlement on its tile ("":
// it can; optional on Reader; the simulation's rule, MAP-05).
type Attacker interface {
	AttackReason(caravan uint64) string
}

// Env is what the view reads of the program around it, as of the call.
type Env struct {
	Game   Reader         // nil off the colony
	Art    *art.Catalog   // the icon catalog (nil: discs)
	Planet *planet.Planet // the planet the colony was founded on (nil: chart it from the params)
}

// Host is what the view needs of the program that no ports role carries
// (at most eight methods, SOLID_GO.md §6).
type Host interface {
	// Env is the program as of now.
	Env() Env
	// Layout is this frame's HUD layout (the inspector sits where the
	// pane does, above the tabs).
	Layout() hudlayout.Layout
	// ChromeHit reports the logical point is on the HUD, so the globe
	// must not take the pointer there.
	ChromeHit(mx, my int) bool
	// FormCaravan opens the Form caravan dialog; globePick reports the
	// tile picked on the globe (-1 none) as its destination.
	FormCaravan(globePick func() int)
	// TileAt, TilePx and WorldToScreen are the camera on the map (the
	// pick-spot cursor; ports.View's).
	TileAt(mx, my int) (int, int)
	TilePx() float64
	WorldToScreen(tx, ty int) (float32, float32)
	// SettleReason is why caravan id cannot found a colony where it is ("":
	// it can); ok false when the game gives no answer (the view judges it).
	SettleReason(id uint64) (why string, ok bool)
}

// presentation is what the view reads of the player's settings and language.
type presentation interface {
	ports.Settings
	ports.Texts
}

// Shell composes the roles the view calls with its Host.
type Shell interface {
	ports.Sizer
	ports.Commands
	ports.World
	presentation
	ports.Panels
	ports.Widgets
	ports.Dispatcher
	Host
}

// The view's parts, each a view type over it (like the windows' Work and
// Assign; SOLID_GO.md §10): the inspector, pod targeting, the caravan route,
// picking and the toolbar.
type (
	inspector struct{ *View } // the world inspector and its gizmos (inspector.go)
	targeting struct{ *View } // the pods' range and landing menu (pods.go)
	router    struct{ *View } // the route preview (route.go, draw.go)
	picker    struct{ *View } // clicks on tiles and markers (view.go)
	chromeBar struct{ *View } // the toolbar, search, legend, bars (chrome.go)
)
