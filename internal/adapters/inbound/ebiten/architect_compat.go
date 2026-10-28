package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
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
	e := architect.Env{Art: a.art, Sel: a.sel, Keys: a.bindings(), Window: a.win.Mode, WinOpen: a.win.Open, UIHidden: a.hideUI}
	if a.renderer != nil {
		e.Sim = a.renderer.game
	}
	return e
}

func (h architectHost) ChromeHit(mx, my, sw, sh int) bool        { return h.a.colonyChromeHit(mx, my, sw, sh) }
func (h architectHost) Layout() hudlayout.Layout                 { return h.a.hud() }
func (h architectHost) CloseFloat()                              { h.a.closeFloat() }
func (h architectHost) SyncOverlays()                            { h.a.syncRoofForce() }
func (h architectHost) LockedBy(key string) string               { return h.a.lockedBy(key) }
func (h architectHost) BuildingTitle(b game.BuildingView) string { return buildingTitle(b) }

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
		r.drawZoneLabel(dst, z, r.zoneGeomOf(z), pw, ph, tile)
	}
}

func (m architectMap) ZoneLabelAt(w game.WorldView, mx, my int, tile float64, shown uint16) (game.ZoneView, bool) {
	return m.a.renderer.zoneLabelAt(w, mx, my, tile, shown)
}

func (m architectMap) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	return m.a.renderer.plantsOn(plants, x0, y0, x1, y1, buf)
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
	return architect.Schedule{Win: p.a.windowRect(), Rows: p.a.scheduleRows(),
		NameW: workNameW, CellW: schedCellW, Top: schedTop, RowH: schedRowH}
}
func (p architectPanes) PaneButtons() []button               { return p.a.paneButtons() }
func (p architectPanes) SelectedPawn() (game.PawnView, bool) { return p.a.selectedPawn() }

// ---- aliases

type (
	drawStyle    = architect.DrawStyle
	placeMode    = architect.PlaceMode
	zoneToolKind = architect.ZoneToolKind
)

const (
	paneAreaChipID     = architect.AreaChipID // the colonist pane's allowed-area chip
	archCatPower       = architect.CatPower
	archCatTemperature = architect.CatTemperature
	archInnerW         = architect.InnerW
	areaPickID         = architect.AreaPickID
	areasConfirm       = architect.AreasConfirm
	areasMax           = architect.AreasMax
	areasModalID       = architect.AreasModalID
	assignAreaTip      = architect.AssignAreaTip
	homeAutoButtonID   = architect.HomeAutoButtonID
	placeSingle        = architect.PlaceSingle
	placeLine          = architect.PlaceLine
	placeFill          = architect.PlaceFill
	placeArea          = architect.PlaceArea
	powerButtonID      = architect.PowerButtonID
	powerGhostLabel    = architect.PowerGhostLabel
	schedAreaGap       = architect.SchedAreaGap
	schedAreaW         = architect.SchedAreaW
	styleDefault       = architect.StyleDefault
	styleLine          = architect.StyleLine
	styleRect          = architect.StyleRect
	styleRectEdge      = architect.StyleRectEdge
	styleOval          = architect.StyleOval
	styleOvalEdge      = architect.StyleOvalEdge
	zoneNoReason       = architect.ZoneNoReason
	zoneToolNone       = architect.ZoneToolNone
	zoneToolExpand     = architect.ZoneToolExpand
	zoneToolShrink     = architect.ZoneToolShrink
)

var (
	facingNames = architect.FacingNames
	styleNames  = architect.StyleNames
)

// The feature's pure helpers.
var (
	anyZoneCell      = architect.AnyZoneCell
	archPowerLine    = architect.PowerLine
	archStockLine    = architect.StockLine
	areaRefLabel     = architect.AreaRefLabel
	areaTint         = architect.AreaTint
	buildSprite      = architect.BuildSprite
	buildSpriteStuff = architect.BuildSpriteStuff
	buildable        = architect.Buildable
	cropLabel        = architect.CropLabel
	cropLines        = architect.CropLines
	drawIconCS       = architect.DrawIconCS
	facingIndex      = architect.FacingIndex
	facingOr         = architect.FacingOr
	kindID           = architect.KindID
	lockWord         = architect.LockWord
	movableKind      = architect.MovableKind
	moveCancelRect   = architect.MoveCancelRect
	movingLine       = architect.MovingLine
	penEdges         = architect.PenEdges
	pendingMoveFrame = architect.PendingMoveFrame
	placementCells   = architect.PlacementCells
	rotateFacing     = architect.RotateFacing
	shapeCells       = architect.ShapeCells
	snowClearToolID  = architect.SnowClearToolID
	stockOf          = architect.StockOf
	stuffLabel       = architect.StuffLabel
	zoneByID         = architect.ZoneByID
	zoneCellAt       = architect.ZoneCellAt
	zoneCropName     = architect.ZoneCropName
	zoneDefaultName  = architect.ZoneDefaultName
	zoneDragHint     = architect.ZoneDragHint
	zoneDragRefusal  = architect.ZoneDragRefusal
	zoneKindName     = architect.ZoneKindName
	zonePriorityName = architect.ZonePriorityName
	zoneSpecies      = architect.ZoneSpecies
	zoneTipText      = architect.ZoneTipText
	zoneToolHint     = architect.ZoneToolHint
)

func abs(v int) int { return architect.Abs(v) }

// ---- the App's Architect methods

func (a *App) openArchitect()               { a.arch.OpenPanel() }
func (a *App) closeArchitect()              { a.arch.ClosePanel() }
func (a *App) selectArchCategory(id string) { a.arch.SelectCategory(id) }
func (a *App) selectArchTool(id string)     { a.arch.SelectTool(id) }
func (a *App) presetArchTool(kind, stuff, facing string) bool {
	return a.arch.Preset(kind, stuff, facing)
}
func (a *App) submitArchitect()                    { a.arch.Submit() }
func (a *App) handleArchitectInput()               { a.arch.HandleInput() }
func (a *App) rotateTool(dir int)                  { a.arch.RotateTool(dir) }
func (a *App) dragSpan() (x0, y0, x1, y1 int)      { return a.arch.DragSpan() }
func (a *App) architectButtons() []button          { return a.arch.Buttons() }
func (a *App) architectRect() button               { return a.arch.Rect() }
func (a *App) archDescHeight() float32             { return a.arch.DescHeight() }
func (a *App) archPlan() *architect.DescPlan       { return a.arch.Plan() }
func (a *App) archToolCell(id string) (rect, bool) { return a.arch.ToolCell(id) }
func (a *App) drawArchitect(screen *ebiten.Image)  { a.arch.Draw(screen) }
func (a *App) drawArchitectGhost(screen *ebiten.Image) {
	a.arch.DrawGhost(screen)
}
func (a *App) toggleHomeAuto()                   { a.arch.ToggleHomeAuto() }
func (a *App) icon(name string) *ebiten.Image    { return a.arch.Icon(name) }
func (a *App) curStyle(t archTool) drawStyle     { return a.arch.CurStyle(toDef(t)) }
func (a *App) toolStuff(t archTool) string       { return a.arch.ToolStuff(toDef(t)) }
func (a *App) toolLock(t archTool) string        { return a.arch.ToolLock(toDef(t)) }
func (a *App) powerLegend() string               { return a.arch.Marks.PowerLegend() }
func (a *App) powerToggleTip() string            { return a.arch.Marks.PowerToggleTip() }
func (a *App) penOutlined() (game.PenView, bool) { return a.arch.Marks.PenOutlined() }
func (a *App) penOutlines() []game.PenView       { return a.arch.Marks.PenOutlines() }
func (a *App) drawPenOutline(screen *ebiten.Image) {
	a.arch.Marks.DrawPenOutline(screen)
}

// archToolCommands are the open category's tools as command buttons.
func (a *App) archToolCommands() []cmdButton {
	cards := a.arch.Cards()
	if cards == nil {
		return nil
	}
	out := make([]cmdButton, len(cards))
	for i, c := range cards {
		out[i] = cmdButton{ID: c.ID, Label: c.Label, Icon: c.Icon, Desc: c.Desc,
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

func (a *App) selectedZone() (game.ZoneView, bool)      { return a.arch.Zones.Selected() }
func (a *App) zoneView(id uint16) (game.ZoneView, bool) { return a.arch.Zones.View(id) }
func (a *App) selectZone(id uint16)                     { a.arch.Zones.Select(id) }
func (a *App) clearZone()                               { a.arch.Zones.Clear() }
func (a *App) selectZoneLabel(w game.WorldView, mx, my, tx, ty int) bool {
	return a.arch.Zones.SelectLabel(w, mx, my, tx, ty)
}
func (a *App) applyZoneClick(id string)               { a.arch.Zones.Click(id) }
func (a *App) drawZoneSelection(screen *ebiten.Image) { a.arch.Zones.DrawSelection(screen) }
func (a *App) zoneLines(z game.ZoneView) []string     { return a.arch.Zones.Lines(z) }
func (a *App) startZoneTool(k zoneToolKind)           { a.arch.Zones.Start(k) }
func (a *App) zoneToolOn(k zoneToolKind) bool         { return a.arch.Zones.On(k) }
func (a *App) handleZoneTool() bool                   { return a.arch.Zones.Handle() }
func (a *App) escapeZoneTool() bool                   { return a.arch.Zones.Escape() }
func (a *App) zoneRefusalAt(kind string, x, y int) string {
	return a.arch.Zones.RefusalAt(kind, x, y)
}
func (a *App) zoneToolCells(z game.ZoneView) []ghostCell {
	return ghostCells(a.arch.Zones.Cells(z))
}

// zonePane is the selected zone's inspect pane.
func (a *App) zonePane(id uint16) paneSpec {
	p := a.arch.Zones.Pane(id)
	return paneSpec{Title: p.Title, Subtitle: p.Subtitle, Lines: p.Lines, Close: p.Close}
}

// ---- areas

func (a *App) openAreaManager()                         { a.arch.Areas.OpenManager() }
func (a *App) areaPaintIndex() int                      { return a.arch.Areas.PaintIndex() }
func (a *App) areaUsers(id int) int                     { return a.arch.Areas.Users(id) }
func (a *App) assignArea(pawn entity.EntityID, ref int) { a.arch.Areas.Assign(pawn, ref) }
func (a *App) areaSelectorsUI(sw, sh int)               { a.arch.Areas.Selectors(architectPanes{a}) }
func (a *App) openAreaPicker(title string, cur int, base bool, pick func(ref int)) {
	a.arch.Areas.OpenPicker(title, cur, base, pick)
}
func (a *App) areaDropdown(id string, r rect, ref int, pick func(ref int)) {
	a.arch.Areas.Dropdown(id, r, ref, pick)
}

// areaChip is the colonist pane's allowed-area chip.
func (a *App) areaChip(p game.PawnView) (paneChip, bool) {
	c, ok := a.arch.Areas.Chip(p)
	return paneChip{ID: c.ID, Label: c.Label, Tip: c.Tip}, ok
}

// ---- move mode

func (a *App) moveSt() *architect.MoveState { return a.arch.Mover.State() }
func (a *App) moveStep(tx, ty int, onMap bool, rotate int, left, cancel bool) bool {
	return a.arch.Mover.Step(tx, ty, onMap, rotate, left, cancel)
}
func (a *App) startMove(b game.BuildingView)        { a.arch.Mover.Start(b) }
func (a *App) handleMoveMode() bool                 { return a.arch.Mover.Handle() }
func (a *App) drawMoveMode(screen *ebiten.Image)    { a.arch.Mover.Draw(screen) }
func (a *App) drawMoveMarkers(screen *ebiten.Image) { a.arch.Mover.DrawMarkers(screen) }

// ---- transition shims (ARCH-005 wave 5.3): until the lead renames the
// fields root-wide (lead_edits_architect.pl), other root files read these
// types' old lowercase fields. Each shim and converter goes with its last
// lowercase reader.

// archTool is architect.ToolDef with its old field names.
type archTool struct {
	id, label, category, icon, desc string
	mode                            placeMode
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
func allArchTools() []archTool           { return fromDefs(architect.AllTools()) }
func toolsIn(category string) []archTool { return fromDefs(architect.ToolsIn(category)) }
func zoneKindOf(t archTool) string       { return architect.ZoneKindOf(toDef(t)) }
func drawIconFallback(screen *ebiten.Image, t archTool, x, y float32) {
	architect.DrawIconFallback(screen, toDef(t), x, y)
}
func archTooltipLines(t archTool, stuff string, wood, stock int) []string {
	return architect.TooltipLines(toDef(t), stuff, wood, stock)
}
func zoneGhostCells(x0, y0, x1, y1 int, clip [4]int, can func(x, y int) bool) []ghostCell {
	return ghostCells(architect.ZoneGhostCells(x0, y0, x1, y1, clip, can))
}
func rowRuns(cells [][2]int) []rowRun {
	rs := architect.RowRuns(cells)
	out := make([]rowRun, len(rs))
	for i, r := range rs {
		out[i] = rowRun{y: r.Y, x0: r.X0, x1: r.X1}
	}
	return out
}
