package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// architect.Tool owns this feature (ARCH-005); architectHost is the root's side
// of architect.Host, architectMap the renderer's side of architect.Map, and
// newArchitect builds the feature for NewApp. The aliases and one-line
// wrappers below keep the root's other call sites unchanged until the
// sweep (SOLID_GO.md §11); the "transition shims" keep the old lowercase
// field names other root files still read until the lead's renames land.

type architectHost struct{ a *App }

// architectMap is the renderer's side of the map the tools draw on.
type architectMap struct{ a *App }

// architectShell is the feature's view of the App: the ports roles plus its Host.
type architectShell struct {
	shell
	architectHost
	architectMap
}

var _ architect.Shell = architectShell{}

func newArchitect(a *App) *architect.Tool {
	return architect.New(architectShell{a.shell, architectHost{a}, architectMap{a}})
}

// ---- architect.Host

func (h architectHost) Env() architect.Env {
	a := h.a
	e := architect.Env{Art: a.art, Sel: a.sel, Keys: a.bindings(), Window: a.win.Mode, WinOpen: a.win.Open, UIHidden: a.chrome.Hidden}
	if a.renderer != nil {
		e.Sim = a.renderer.game
	}
	return e
}

func (h architectHost) ChromeHit(mx, my, sw, sh int) bool        { return h.a.colonyChromeHit(mx, my, sw, sh) }
func (h architectHost) Layout() hudlayout.Layout                 { return h.a.hud() }
func (h architectHost) CloseFloat()                              { h.a.ord.CloseFloat() }
func (h architectHost) SyncOverlays()                            { h.a.syncRoofForce() }
func (h architectHost) LockedBy(key string) string               { return h.a.win.Research().LockedBy(key) }
func (h architectHost) BuildingTitle(b game.BuildingView) string { return inspector.BuildingTitle(b) }

// ---- architect.Map

func (m architectMap) DrawAreaGhost(dst *ebiten.Image, x0, y0, x1, y1 int, roof bool) {
	m.a.renderer.drawGhostRect(dst, x0, y0, x1, y1)
	if roof {
		m.a.drawRoofToolGhost(dst, x0, y0, x1, y1)
	}
}

func (m architectMap) DrawPlacementGhost(dst *ebiten.Image, kind, stuff, facing string, cells []architect.GhostCell) {
	m.a.renderer.drawPlacementGhost(dst, kind, stuff, facing, ghostCells(cells))
}

func (m architectMap) DrawRangeRing(dst *ebiten.Image, x, y int, radius float64, c color.NRGBA) {
	m.a.renderer.drawRangeRing(dst, x, y, radius, c)
}

func (m architectMap) DrawPowerLink(dst *ebiten.Image, from, to [2]int, c color.NRGBA, tile float64) {
	m.a.renderer.drawPowerLink(dst, from, to, c, tile)
}

func (m architectMap) DrawZoneMark(dst *ebiten.Image, z game.ZoneView, pw, ph int, tile float64) {
	r := m.a.renderer
	r.drawZoneHighlight(dst, m.a.shell.World(), z, pw, ph, tile)
	if z.Hidden {
		r.drawZoneLabel(dst, z, r.overlays().Zones.ZoneGeomOf(z), pw, ph, tile)
	}
}

func (m architectMap) ZoneLabelAt(w game.WorldView, mx, my int, tile float64, shown uint16) (game.ZoneView, bool) {
	return m.a.renderer.zoneLabelAt(w, mx, my, tile, shown)
}

func (m architectMap) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	return m.a.renderer.pipe().PlantsOn(plants, x0, y0, x1, y1, buf)
}

func (m architectMap) Overlays() *overlay.Set {
	if m.a.renderer == nil {
		return nil
	}
	return m.a.renderer.overlays()
}

// architectPanes is what the Schedule window and the colonist pane hand
// the allowed-area selectors they host.
type architectPanes struct{ a *App }

func (p architectPanes) Schedule() architect.Schedule {
	return architect.Schedule{Win: p.a.win.Rect(), Rows: p.a.win.Schedule().Rows(),
		NameW: windows.WorkNameW, CellW: windows.SchedCellW, Top: windows.SchedTop, RowH: windows.SchedRowH}
}
func (p architectPanes) PaneButtons() []uikit.Button         { return p.a.insp.PaneButtons() }
func (p architectPanes) SelectedPawn() (game.PawnView, bool) { return p.a.insp.SelectedPawn() }

// ---- aliases

// the colonist pane's allowed-area chip

// ---- the App's Architect methods

// archToolCommands are the open category's tools as command buttons.
func (a *App) archToolCommands() []inspector.Button {
	cards := a.arch.Cards()
	if cards == nil {
		return nil
	}
	out := make([]inspector.Button, len(cards))
	for i, c := range cards {
		out[i] = inspector.Button{ID: c.ID, Label: c.Label, Icon: c.Icon, Desc: c.Desc,
			Disabled: c.Disabled, Reason: c.Reason, Toggle: c.Toggle, On: c.On}
	}
	return out
}

// previewCells is what the current tool would affect right now.
func (a *App) previewCells() (archTool, []ghostCell) {
	d, cells := a.arch.PreviewCells()
	return fromDef(d), ghostCells(cells)
}

// syncPowerForce keeps the power overlay forced on while a power tool is
// the selected tool of an open Architect (beside syncRoofForce).
func (a *App) syncPowerForce() {
	if a.renderer != nil {
		a.renderer.overlays().Power.Forced = a.arch.Marks.ForcesPower()
	}
}

// syncSnowClearForce shows the snow clearing overlay while its tool is held.
func (a *App) syncSnowClearForce() {
	if a.renderer != nil {
		a.renderer.overlays().SnowClear.Forced = a.arch.Marks.ForcesSnowClear()
	}
}

// ---- zones

func (a *App) applyZoneClick(id string) { a.arch.Zones.Click(id) }

// zonePane is the selected zone's inspect pane.
func (a *App) zonePane(id uint16) inspector.Spec {
	p := a.arch.Zones.Pane(id)
	return inspector.Spec{Title: p.Title, Subtitle: p.Subtitle, Lines: p.Lines, Close: p.Close}
}

// ---- areas

func (a *App) areaSelectorsUI(sw, sh int) { a.arch.Areas.Selectors(architectPanes{a}) }

// areaChip is the colonist pane's allowed-area chip.
func (a *App) areaChip(p game.PawnView) (inspector.Chip, bool) {
	c, ok := a.arch.Areas.Chip(p)
	return inspector.Chip{ID: c.ID, Label: c.Label, Tip: c.Tip}, ok
}

// ---- move mode

// ---- transition shims (ARCH-005 wave 5.3): until the lead renames the
// fields root-wide (lead_edits_architect.pl), other root files read these
// types' old lowercase fields. Each shim and converter goes with its last
// lowercase reader.

// archTool is architect.ToolDef with its old field names.
type archTool struct {
	id, label, category, icon, desc string
	mode                            architect.PlaceMode
	build                           string
	cmd                             command.Kind
	crop                            string
	enable                          bool
	variant                         uint8
}

// ghostCell is architect.GhostCell with its old field names.
type ghostCell struct {
	x, y     int
	ok       bool
	reason   string
	replaces string
	facing   string
	same     bool
}

// rowRun is architect.RowRun with its old field names.
type rowRun struct{ y, x0, x1 int }

var (
	archCategories = categoriesShim()
	archTools      = fromDefs(architect.Tools)
)

func categoriesShim() []struct{ id, label string } {
	out := make([]struct{ id, label string }, len(architect.Categories))
	for i, c := range architect.Categories {
		out[i].id, out[i].label = c.ID, c.Label
	}
	return out
}

func toDef(t archTool) architect.ToolDef {
	return architect.ToolDef{ID: t.id, Label: t.label, Category: t.category, Icon: t.icon, Desc: t.desc,
		Mode: t.mode, Build: t.build, Cmd: t.cmd, Crop: t.crop, Enable: t.enable, Variant: t.variant}
}

func fromDef(d architect.ToolDef) archTool {
	return archTool{id: d.ID, label: d.Label, category: d.Category, icon: d.Icon, desc: d.Desc,
		mode: d.Mode, build: d.Build, cmd: d.Cmd, crop: d.Crop, enable: d.Enable, variant: d.Variant}
}

func fromDefs(ds []architect.ToolDef) []archTool {
	out := make([]archTool, len(ds))
	for i, d := range ds {
		out[i] = fromDef(d)
	}
	return out
}

func ghostCells(cs []architect.GhostCell) []ghostCell {
	if cs == nil {
		return nil
	}
	out := make([]ghostCell, len(cs))
	for i, c := range cs {
		out[i] = ghostCell{x: c.X, y: c.Y, ok: c.OK, reason: c.Reason, replaces: c.Replaces, facing: c.Facing, same: c.Same}
	}
	return out
}

func toolByID(id string) (archTool, bool) {
	d, ok := architect.ToolByID(id)
	return fromDef(d), ok
}
func allArchTools() []archTool { return fromDefs(architect.AllTools()) }
