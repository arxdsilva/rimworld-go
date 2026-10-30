package hud

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
)

// Reader is the simulation's read models the HUD shows beyond the frame's
// world view (consumer-owned; the game implements it). The optional
// readers (the gatherings, the trade status, the allies, the comms
// session) are type assertions on it.
type Reader interface {
	Snapshot() []game.PawnView
	TemperatureAt(x, y int) int
	Factions() []game.FactionView
	Activity() schedule.Activity
}

// Env is what the HUD reads of the program around it, as of the call.
type Env struct {
	Game  Reader       // nil off the colony (no renderer)
	Art   *art.Catalog // the map's catalog (item sprites); nil off the colony
	Icons *art.Catalog // the client's catalog (ui_icon/ sprites)

	Updates uint64 // the Update counter (toasts, double clicks)
	Session int    // colonies started this run (letter ids restart with each)

	Colony          bool // the colony screen is on top with a live map (toasts anchor to the HUD)
	Hint            bool // the hint card is up (toasts stack above it)
	Alt             bool // Alt is held (cell info's detail lines)
	ReducedMotion   bool // the danger alerts hold still
	ViewCached      bool // the read models are this update's cached ones (the alerts memo)
	NothingSelected bool // no pawn, animal, item, plant, building, frame, zone or group is selected
	PanelOpen       bool // the Architect or a main-tab window is open
}

// Host is what the HUD needs of the program that no ports role carries
// (at most eight members, SOLID_GO.md §6).
type Host interface {
	ports.Widgets
	ports.Commands
	// Env is the program as of now.
	Env() Env
	// Layout is this frame's HUD layout (the root's memo over every
	// feature's sizes; the HUD's own sizes feed it, so they never call it).
	Layout() hudlayout.Layout
	// ChromeHit reports the logical point is on any feature's chrome, so
	// it is not the map.
	ChromeHit(mx, my, sw, sh int) bool
	// SpeedLabel is the clock's speed text (it says why the game paused
	// itself).
	SpeedLabel() string
	// CenterOnMap pans the camera so the world point is in the middle of
	// the map viewport.
	CenterOnMap(wx, wy float64)
	// SetSettings stores and applies the player's options the way the
	// Options screen does.
	SetSettings(config.Settings)
}

// Peers is what the HUD reads of, asks of and draws through the other
// features: the selection, the main-tab windows, the Architect's power
// overlay and the renderer's sprites.
type Peers interface {
	ports.PawnSel
	// Pick makes c the only selection.
	Pick(c selection.Cand)
	// Picked reports the colonist is selected, alone or in a group.
	Picked(id entity.EntityID) bool
	// TabOn reports the main tab id's window (or the Architect) is the
	// one open.
	TabOn(id string) bool
	// Power is the power overlay's legend ("" while it is off) and its
	// toggle's tooltip.
	Power() (legend, tip string)
	// StoneItem is a ground stack's sprite key and tint (the item
	// inspector's fallback for a resource without an icon).
	StoneItem(key, kind string, cs sprite.ColorScale) (string, sprite.ColorScale)
	// DrawPawnBust draws the colonist's head and shoulders fitted into the
	// logical box (the colonist bar's portrait).
	DrawPawnBust(dst *ebiten.Image, p game.PawnView, x, y, w, h float64)
	// DrawWeather draws the weather and its wind arrow with the right edge
	// at x.
	DrawWeather(dst *ebiten.Image, x, y int, w game.WorldView)
}

// Shell composes the roles the HUD calls with its Host and Peers.
type Shell interface {
	ports.Sizer
	ports.World
	ports.View
	ports.Overlays
	ports.Speed
	ports.Settings
	Host
	Peers
}

// Chrome is the colony HUD (ARCH-005): the regions' state, the toasts and
// the two switches other root files read. Each region is a view type over
// it (Resources, ColonistBar, Right, Alerts, Global, CellInfo, Tabs,
// Banner, RoomStats).
type Chrome struct {
	sh Shell

	// The state other root files read until the sweep (the lead moved
	// these from App).
	Hidden   bool              // F11: only the map is drawn
	Rooms    bool              // G: room stats overlay
	ZonesOff bool              // Shift+Z: the zones overlay is off (STO-3)
	MsgSeen  map[string]uint64 // message key -> frame first shown

	res        resourcesState
	bar        barState
	barClk     barClick
	right      rightState
	alertCycle map[string]int // alert key -> one more than the target on show
	alerts     alertsMemo
	toasts     []Toast
}

// New builds the feature over its shell.
func New(sh Shell) *Chrome {
	return &Chrome{sh: sh, res: resourcesState{collapsed: map[string]bool{}}}
}

// The regions, each a view over the chrome.
type (
	// Resources is the resource list, top-left.
	Resources struct{ *Chrome }
	// ColonistBar is the colonist bar, top-centre.
	ColonistBar struct{ *Chrome }
	// Right is the right column: letters, messages and the hover cards.
	Right struct{ *Chrome }
	// Alerts is the alert stack in the right column.
	Alerts struct{ *Chrome }
	// Global is the lower-right controls: conditions, clock, speed,
	// overlay toggles.
	Global struct{ *Chrome }
	// CellInfo is the hovered cell's lines, lower-left.
	CellInfo struct{ *Chrome }
	// Tabs is the bottom tab strip.
	Tabs struct{ *Chrome }
	// Banner is the gathering banner under the colonist bar.
	Banner struct{ *Chrome }
	// RoomStats is the room overlay and its card (G).
	RoomStats struct{ *Chrome }
)

func (c *Chrome) Res() Resources       { return Resources{c} }
func (c *Chrome) Bar() ColonistBar     { return ColonistBar{c} }
func (c *Chrome) Right() Right         { return Right{c} }
func (c *Chrome) Alerts() Alerts       { return Alerts{c} }
func (c *Chrome) Global() Global       { return Global{c} }
func (c *Chrome) Cell() CellInfo       { return CellInfo{c} }
func (c *Chrome) Tabs() Tabs           { return Tabs{c} }
func (c *Chrome) Banner() Banner       { return Banner{c} }
func (c *Chrome) RoomStats() RoomStats { return RoomStats{c} }

// DeclareRegions declares the four corner regions' widgets in order: the
// resource list, the colonist bar, the right column, the global controls.
func (c *Chrome) DeclareRegions(sw, sh int) {
	c.Res().Declare(sw, sh)
	c.Bar().Declare(sw, sh)
	c.Right().Declare(sw, sh)
	c.Global().Declare(sw, sh)
}

// live reports a colony is up (the regions draw nothing on a menu).
func (c *Chrome) live() bool { return c.sh.Env().Game != nil }

func (c *Chrome) kit() *uikit.Kit { return c.sh.Kit() }

// drawButtons draws hand-placed buttons, hovered under the pointer.
func drawButtons(screen *ebiten.Image, buttons []uikit.Button, mx, my int) {
	for _, b := range buttons {
		uikit.DrawButton(screen, b, b.Contains(mx, my))
	}
}

// icon is the ui_icon sprite name (nil when the art has none).
func (c *Chrome) icon(name string) *ebiten.Image {
	cat := c.sh.Env().Icons
	if cat == nil {
		return nil
	}
	return cat.Image("ui_icon/"+name, color.NRGBA{})
}

// drawIconCS is gfx.Icon multiplied by cs (a stone item's tint).
func drawIconCS(dst, img *ebiten.Image, x, y, size float64, cs sprite.ColorScale) {
	if img == nil {
		return
	}
	s := size / float64(img.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	if cs != sprite.NoCS {
		op.ColorScale.Scale(cs[0], cs[1], cs[2], cs[3])
	}
	gfx.DrawImage(dst, img, op)
}

// titleCase upper-cases the first byte ("granite" -> "Granite").
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// speciesLabel is an animal's species name ("arctic fox"): the view's
// Label, else the key with its underscores spaced out.
func speciesLabel(an game.AnimalView) string {
	switch {
	case an.Label != "":
		return an.Label
	case an.Species != "":
		return strings.ReplaceAll(an.Species, "_", " ")
	}
	return "animal"
}
