package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	hudpkg "rimworld-go/internal/adapters/inbound/ebiten/hud"
	portspkg "rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"
)

// hud.Chrome owns the colony HUD's regions (ARCH-005 wave 5.5); hudHost is
// the root's side of hud.Host and hud.Peers, and newHud builds the
// feature for NewApp. The composition (colonyUI, colonyButtons,
// drawColonyHUD, colonyChromeHit, the hud() memo) is colony_compose.go;
// the renderer's side (overlay toggles, camera jumps, why the game
// paused) is hud_global_render.go. The aliases and one-line wrappers
// below keep the root's other call sites unchanged until the sweep
// (SOLID_GO.md §11); the converting shims (lowercase-field copies of
// hud's types) serve the root tests that still read the old fields.

type hudHost struct{ a *App }

var (
	_ hud.Host  = hudShell{}
	_ hud.Peers = hudShell{}
	_ hud.Shell = hudShell{}
)

// hudShell is the feature's view of the App: the ports roles plus its Host.
type hudShell struct {
	shell
	hudHost
}

func newHud(a *App) *hud.Chrome {
	c := hud.New(hudShell{a.shell, hudHost{a}})

	return c
}

// ---- hud.Host

func (h hudHost) Env() hud.Env {
	a := h.a
	e := hud.Env{
		Icons: a.art, Updates: a.updates, Session: a.simsStarted, Hint: a.hint != "", Alt: a.in.Alt(),
		PanelOpen: a.arch.Open || a.win.Open,
	}
	if r := a.renderer; r != nil {
		e.Game, e.Art = r.game, r.art
		e.Colony = a.nav.screen() == portspkg.ScreenColony
		e.ReducedMotion, e.ViewCached = r.reducedMotion, r.viewCached
		_, pawn := a.shell.Selected()
		e.NothingSelected = !a.sel.Grouped() && !pawn && !a.sel.HasAnimal && !a.sel.HasItem && !a.sel.HasPlant &&
			!a.sel.HasBuilding && !a.sel.HasFrame && !a.sel.HasZone
	}
	return e
}

func (h hudHost) Layout() hudlayout.Layout          { return h.a.hud() }
func (h hudHost) ChromeHit(mx, my, sw, sh int) bool { return h.a.colonyChromeHit(mx, my, sw, sh) }
func (h hudHost) SpeedLabel() string                { return h.a.speedLabel() }
func (h hudHost) SetSettings(s config.Settings)     { h.a.setSettings(s) }
func (h hudHost) Pick(c selection.Cand)             { h.a.sel.Select(c) }
func (h hudHost) Picked(id entity.EntityID) bool    { return h.a.sel.InGroup(selection.Pawn, id) }
func (h hudHost) Power() (legend, tip string) {
	return h.a.arch.Marks.PowerLegend(), h.a.arch.Marks.PowerToggleTip()
}
func (h hudHost) StoneItem(key, kind string, cs sprite.ColorScale) (string, sprite.ColorScale) {
	return h.a.renderer.pipe().StoneItem(key, kind, cs)
}

func (h hudHost) CenterOnMap(wx, wy float64) {
	if h.a.renderer != nil {
		h.a.centerOnMap(wx, wy)
	}
}

func (h hudHost) DrawPawnBust(dst *ebiten.Image, p game.PawnView, x, y, w, ht float64) {
	if r := h.a.renderer; r != nil && r.art != nil {
		r.drawPawnBustArt(dst, p, x, y, w, ht)
	}
}

func (h hudHost) DrawWeather(dst *ebiten.Image, x, y int, w game.WorldView) {
	drawWeatherLabelRight(dst, x, y, w)
}

// TabOn reports tab id's window (or the Architect) is the one open.
func (h hudHost) TabOn(id string) bool {
	a := h.a
	switch id {
	case ids.TabArchitect:
		return a.arch.Open
	case ids.TabWork:
		return a.win.Open && (a.win.Mode == "work" || a.win.Mode == "")
	case ids.TabSchedule:
		return a.win.Open && a.win.Mode == "schedule"
	case ids.TabResearch:
		return a.win.Open && a.win.Mode == "research"
	case ids.TabAnimals:
		return a.win.Open && a.win.Mode == "animals" && !a.win.Wild
	case ids.TabWildlife:
		return a.win.Open && a.win.Mode == "animals" && a.win.Wild
	case ids.TabAssign:
		return a.win.KitID() == windows.AssignModalID
	case ids.TabWorld:
		return a.win.KitID() == windows.WorldModalID
	case ids.TabHistory:
		return a.win.KitID() == windows.HistoryModalID
	case ids.TabFactions:
		return a.win.KitID() == windows.ColFacModalID
	}
	return false
}

// ---- aliases

// ---- converting shims (lowercase-field copies for the root tests)

// alert is hud.Alert as the root tests read it.
type alert struct {
	key     string
	label   string
	danger  bool
	sev     hudpkg.AlertSev
	explain string
	targets [][2]int
}

func toAlert(a hud.Alert) alert {
	return alert{key: a.Key, label: a.Label, danger: a.Danger, sev: a.Sev, explain: a.Explain, targets: a.Targets}
}

func fromAlert(a alert) hud.Alert {
	return hud.Alert{Key: a.key, Label: a.label, Danger: a.danger, Sev: a.sev, Explain: a.explain, Targets: a.targets}
}

func toAlerts(as []hud.Alert) []alert {
	if as == nil {
		return nil
	}
	out := make([]alert, len(as))
	for i, a := range as {
		out[i] = toAlert(a)
	}
	return out
}

func fromAlerts(as []alert) []hud.Alert {
	if as == nil {
		return nil
	}
	out := make([]hud.Alert, len(as))
	for i, a := range as {
		out[i] = fromAlert(a)
	}
	return out
}

// alertRow is hud.AlertRow as the root tests read it.
type alertRow struct {
	r      gfx.Rect
	al     alert
	more   int
	hidden []alert
}

// rightCard is hud.RightCard as the root tests read it.
type rightCard struct {
	title, body string
	clr         color.RGBA
}

// letterRow is hud.LetterRow as the root tests read it.
type letterRow struct {
	r      gfx.Rect
	letter game.LetterView
	older  int
}

// messageRow is hud.MessageRow as the root tests read it.
type messageRow struct {
	r     gfx.Rect
	msg   game.MessageView
	idx   int
	alpha float64
}

// statusChip is hud.StatusChip as the root tests read it.
type statusChip struct {
	label, tip string
	edge       color.RGBA
}

func toChips(cs []hud.StatusChip) []statusChip {
	if cs == nil {
		return nil
	}
	out := make([]statusChip, len(cs))
	for i, c := range cs {
		out[i] = statusChip{label: c.Label, tip: c.Tip, edge: c.Edge}
	}
	return out
}

func fromChips(cs []statusChip) []hud.StatusChip {
	if cs == nil {
		return nil
	}
	out := make([]hud.StatusChip, len(cs))
	for i, c := range cs {
		out[i] = hud.StatusChip{Label: c.label, Tip: c.tip, Edge: c.edge}
	}
	return out
}

// globalPlan is hud.GlobalPlan as the root tests read it.
type globalPlan struct {
	reason  string
	weather bool
	chips   []statusChip
	legend  []string
}

// toast is hud.Toast as the root tests read it.
type toast struct {
	text string
	at   uint64
}

// resRow is hud.ResRow as the root tests read it.
type resRow struct {
	header           bool
	cat, kind, label string
	qty              int
}

// barStatus is hud.BarStatus as the root tests read it.
type barStatus struct{ key, label string }

func toStatuses(ss []hud.BarStatus) []barStatus {
	if ss == nil {
		return nil
	}
	out := make([]barStatus, len(ss))
	for i, s := range ss {
		out[i] = barStatus{key: s.Key, label: s.Label}
	}
	return out
}

// ---- one-line wrappers

// globalHeight is the Global controls' height the layout reserves; the
// minimap, which sits above Global, learns it here too.
func (a *App) globalHeight() float32 {
	h := a.chrome.Global().Height()
	if a.renderer != nil {
		a.renderer.mini.GlobalH = h
	}
	return h
}
