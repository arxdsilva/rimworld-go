package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	wizardpkg "rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/medcare"
)

// windows.Set owns the colony's main-tab windows (ARCH-005 wave 5.3);
// windowsHost is the root's side of windows.Host and windows.Labels, and
// newWindows builds the feature for NewApp. The aliases and one-line
// wrappers below keep the root's other call sites unchanged until the
// sweep (SOLID_GO.md §11).

type windowsHost struct{ a *App }

var (
	_ windows.Host   = windowsHost{}
	_ windows.Labels = windowsHost{}
	_ windows.Shell  = windowsShell{}
)

// windowsShell is the feature's view of the App: the ports roles plus its Host.
type windowsShell struct {
	shell
	windowsHost
}

func newWindows(a *App) *windows.Set {
	return windows.New(windowsShell{a.shell, windowsHost{a}})
}

// ---- windows.Host

func (h windowsHost) Env() windows.Env {
	a := h.a
	e := windows.Env{Art: a.art}
	if r := a.renderer; r != nil {
		e.Game = r.game
	}
	if a.ng != nil {
		e.Planet = a.ng.Planet
	}
	return e
}

func (h windowsHost) WindowRect() gfx.Rect { return h.a.hud().Window }

func (h windowsHost) CenterOnMap(wx, wy float64) {
	if h.a.renderer != nil {
		h.a.centerOnMap(wx, wy)
	}
}

func (h windowsHost) AreaDropdown(id string, r gfx.Rect, ref int, pick func(ref int)) {
	h.a.arch.Areas.Dropdown(id, r, ref, pick)
}

func (h windowsHost) ManageAreas() { h.a.arch.Areas.OpenManager() }

func (h windowsHost) Float(x, y float32, opts []game.OrderOption) {
	if opts == nil {
		h.a.ord.CloseFloat()
		return
	}
	h.a.ord.OpenPick(x, y, false, opts, func(i int) { h.a.shell.Enqueue(opts[i].Cmd) })
}

func (h windowsHost) FormCaravan(globePick func() int) { h.a.deal.OpenFormCaravan(globePick) }

func (h windowsHost) FactionsTable(body gfx.Rect, maxH float32, sel *int) gfx.Rect {
	rows := h.colonyRows()
	plain := make([]wizardpkg.FacRow, len(rows))
	for i, r := range rows {
		plain[i] = r.FacRow
	}
	listH := min(float32(uikit.TableHeaderH+len(rows)*wizardpkg.FacRowH+4), maxH)
	tb, rest := body.SplitV(listH)
	h.a.wiz.FactionsTable(tb, plain, sel, nil)
	return rest
}

// ---- windows.Labels

func (h windowsHost) PawnLabel(p game.PawnView) string       { return hud.PawnLabel(p) }
func (h windowsHost) AnimalLabel(an game.AnimalView) string  { return inspector.AnimalLabel(an) }
func (h windowsHost) AnimalHealth(an game.AnimalView) string { return inspector.AnimalHealthCell(an) }
func (h windowsHost) CareShort(p medcare.Policy) string      { return inspector.CareShort(p) }
func (h windowsHost) Caravan(c game.CaravanView) (string, string) {
	return dialogs.CaravanTip(c), dialogs.CaravanHealthNote(c)
}

func (h windowsHost) Factions() []windows.Faction {
	rows := h.colonyRows()
	out := make([]windows.Faction, len(rows))
	for i, r := range rows {
		out[i] = windows.Faction{
			Name: r.Name, TypeLabel: r.TypeLabel, Tech: r.Tech, Goodwill: r.Goodwill, Relation: r.Relation,
			Standing: wizardpkg.RelationLabel(r.Relation), Color: wizardpkg.RelationColor(r.Relation), Tip: wizardpkg.GoodwillTip(r.FacRow),
			History: r.History, Events: r.Events,
		}
	}
	return out
}

func (h windowsHost) DrawAnimalIcon(dst *ebiten.Image, an game.AnimalView, x, y, size float64) {
	if r := h.a.renderer; r != nil {
		inspector.DrawAnimalIcon(dst, r.art, an, x, y, size)
	}
}

func (h windowsHost) DrawPassion(dst *ebiten.Image, p int, x, y, size float64, st gfx.TextStyle, ty float64) {
	h.a.insp.Tabs().DrawPassion(dst, p, x, y, size, st, ty)
}

// colonyRows are the live Factions table rows (none off the colony).
func (h windowsHost) colonyRows() []colFacRow {
	if h.a.renderer == nil {
		return nil
	}
	return colonyFactionRows(h.a.renderer.game.Factions())
}

// ---- the Factions table's rows: the wizard's table over the live read
// model. This glue joins two features, so it stays at the root.

// colFacRow is one table row and the faction's detail.
type colFacRow struct {
	wizardpkg.FacRow
	History []int
	Events  []string
}

// relationOfView maps the read model's relation word to the domain value.
func relationOfView(s string) faction.Relation {
	switch s {
	case "hostile":
		return faction.Hostile
	case "friendly":
		return faction.Friendly
	}
	return faction.Neutral
}

// accentOfView parses a "#rrggbb" accent; a bad value gives the neutral
// text colour.
func accentOfView(hex string) color.NRGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 6 {
		if v, err := strconv.ParseUint(hex, 16, 32); err == nil {
			return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}
		}
	}
	return color.NRGBA{R: uikit.TextDim.R, G: uikit.TextDim.G, B: uikit.TextDim.B, A: 0xFF}
}

// colonyFactionRows turns the read model into table rows: the visible
// factions in world order, then the always-present Swarm, as in the setup
// dialog. Hidden factions (the colony itself) are left out.
func colonyFactionRows(views []game.FactionView) []colFacRow {
	rows := make([]colFacRow, 0, len(views)+1)
	for _, v := range views {
		if v.Hidden {
			continue
		}
		r := wizardpkg.FacRow{
			ID: v.ID, Name: v.Name, TypeLabel: v.Type, Goodwill: v.Goodwill,
			Relation: relationOfView(v.Relation), Settlements: v.Settlements, Tech: v.Tech,
			Emblem: v.Emblem, Accent: accentOfView(v.Accent), Capital: -1,
		}
		if v.Leader != "" {
			r.Leader = strings.TrimSpace(v.LeaderTitle + " " + v.Leader)
		}
		hist := make([]int, len(v.History))
		for i, h := range v.History {
			hist[i] = int(h)
		}
		rows = append(rows, colFacRow{FacRow: r, History: hist, Events: v.Events})
	}
	for _, s := range wizardpkg.FactionRows(nil) { // the Swarm
		rows = append(rows, colFacRow{FacRow: s})
	}
	return rows
}

// ---- the HUD's tab buttons for the windows (the HUD draws them; these
// widgets answer their clicks)

// tabWidget declares the widget of the HUD tab id (when enabled) that runs
// click; tip carries the button's hover card over.
func (a *App) tabWidget(sw, sh int, id string, tip bool, click func()) {
	for _, b := range a.chrome.Tabs().Buttons(sw, sh) {
		if b.ID != id || b.Disabled {
			continue
		}
		w := uikit.Widget{ID: b.ID, Label: b.Label, R: gfx.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}, Click: click}
		if tip {
			w.Tip = b.Tip
		}
		a.uiSt().Add(w)
	}
}

func (a *App) assignTabUI(sw, sh int) {
	a.tabWidget(sw, sh, ids.TabAssign, false, a.win.Assign().Toggle)
}
func (a *App) historyTabUI(sw, sh int) {
	a.tabWidget(sw, sh, ids.TabHistory, false, a.win.History().Toggle)
}
func (a *App) factionsTabUI(sw, sh int) {
	a.tabWidget(sw, sh, ids.TabFactions, false, a.win.Factions().Toggle)
}
func (a *App) worldTabUI(sw, sh int) { a.tabWidget(sw, sh, ids.TabWorld, false, a.win.World().Toggle) }

// mainWindowUI declares the Animals tab button's toggle (the app's toggle
// does not know Wildlife) and the open window's kit parts.
func (a *App) mainWindowUI(sw, sh int) {
	if a.renderer == nil {
		return
	}
	a.tabWidget(sw, sh, ids.TabAnimals, true, a.win.ToggleAnimals)
	a.win.Declare()
}

// ---- the kit window's shim

// ---- window wrappers

// ---- aliases
