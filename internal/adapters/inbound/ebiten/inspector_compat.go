package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/sim/entity"
)

// inspector.Pane owns this feature (ARCH-005); inspectorHost is the root's
// side of inspector.Host, Peers and Labels, and newInspector builds the
// feature for NewApp and plugs in the kinds the root describes (the
// Architect's tools for an empty selection, the selected zone). The
// aliases and one-line wrappers below keep the root's other call sites
// unchanged until the sweep (SOLID_GO.md §11).

type inspectorHost struct{ a *App }

// inspectorShell is the feature's view of the App: the ports roles plus its Host.
type inspectorShell struct {
	shell
	inspectorHost
}

var _ inspector.Shell = inspectorShell{}

func newInspector(a *App) *inspector.Pane {
	p := inspector.New(inspectorShell{a.shell, inspectorHost{a}})
	p.Producers[selection.None] = inspector.Producer{
		Commands: func(entity.EntityID) []inspector.Button { return a.archToolCommands() },
	}
	p.Producers[selection.Zone] = inspector.Producer{
		Pane:     func(id entity.EntityID) inspector.Spec { return a.zonePane(uint16(id)) },
		Commands: func(id entity.EntityID) []inspector.Button { return a.zoneCommands(uint16(id)) },
	}
	return p
}

// ---- inspector.Host

func (h inspectorHost) Env() inspector.Env {
	a := h.a
	e := inspector.Env{Art: a.art, Keys: a.bindings(), WinOpen: a.win.Open, ArchOpen: a.arch.Open,
		UIHidden: a.chrome.Hidden, Updates: a.updates, Shift: a.in.Shift(), Ctrl: a.in.Ctrl(), WheelY: a.in.Raw.WheelY}
	if r := a.renderer; r != nil {
		e.Sim, e.MapArt = r.game, r.art
	}
	return e
}

func (h inspectorHost) Selection() *selection.Model { return h.a.sel }
func (h inspectorHost) Layout() hudlayout.Layout    { return h.a.hud() }
func (h inspectorHost) Click(id string)             { h.a.applyClick(id) }

func (h inspectorHost) Pick(x, y float32, up bool, entries []game.OrderOption, pick func(i int)) {
	h.a.openPick(x, y, up, entries, pick)
}

func (h inspectorHost) Hostiles() []game.PawnView {
	if r := h.a.renderer; r != nil {
		return r.hostiles()
	}
	return nil
}

// ---- inspector.Peers

func (h inspectorHost) AreaPicker(title string, cur int, base bool, pick func(ref int)) {
	h.a.openAreaPicker(title, cur, base, pick)
}
func (h inspectorHost) AssignArea(id entity.EntityID, ref int)          { h.a.assignArea(id, ref) }
func (h inspectorHost) AreaChip(p game.PawnView) (inspector.Chip, bool) { return h.a.areaChip(p) }
func (h inspectorHost) LockedBy(key string) string                      { return h.a.lockedBy(key) }
func (h inspectorHost) MoveState(b game.BuildingView) (bool, string) {
	return movableKind(b.Kind, b.Floor), movingLine(b)
}
func (h inspectorHost) Buildable(kind string) bool { return hasBuildTool(kind) }

func (h inspectorHost) AreaLabel(ref int) (label, tip string) {
	var areas []game.AreaView
	if r := h.a.renderer; r != nil {
		areas = r.game.Areas()
	}
	return areaRefLabel(areas, ref), assignAreaTip
}

func (h inspectorHost) Move(b game.BuildingView, cancel bool) {
	if !cancel {
		h.a.startMove(b)
		return
	}
	if f, ok := pendingMoveFrame(h.a.shell.World(), b.ID); ok {
		h.a.shell.Enqueue(moveCancelRect(f))
	}
}

// ---- inspector.Labels

func (h inspectorHost) PawnLabel(p game.PawnView) string                { return pawnLabel(p) }
func (h inspectorHost) Temp(tenths int) string                          { return fmtTemp(tenths) }
func (h inspectorHost) WildRow(species string) (game.WildlifeRow, bool) { return h.a.wildRow(species) }
func (h inspectorHost) AnimalBlurb(species string) string               { return animalBlurb(species) }
func (h inspectorHost) Skill(s game.SkillView) (level, tip string) {
	return skillLevelText(s), skillTip(s)
}
func (h inspectorHost) Traits(x, y, w float32, v game.CandidateView) float32 {
	return h.a.traitChips(x, y, w, v)
}

func (h inspectorHost) Backstory(x, y, w float32, label, title, desc, id string) {
	h.a.backstoryLine(x, y, w, label, title, desc, id)
}

func (h inspectorHost) Portrait(dst *ebiten.Image, l look.Look, scarred bool, box rect, name string) {
	if !drawPortrait(dst, h.a.art, l, scarred, float64(box.X), float64(box.Y), float64(box.W), float64(box.H)) {
		drawSilhouette(dst, box, name)
	}
}

// ---- HUD-side helpers that stay root: they read the HUD's cell info and
// the History window, not the inspector.
// hintAnchor is where disabled-reason hints and toasts sit: their left
// edge and the bottom they rest on (the layout's: just above the command
// buttons or the pane, else just above the tab strip).
func (a *App) hintAnchor() (x, bottom float32) {
	l := a.hud()
	x, bottom = l.HintX, l.HintBottom
	if lines := a.cellInfoLines(); len(lines) > 0 { // nothing selected: rest above the cell info
		bottom = min(bottom, cellInfoBox(lines, l.Tabs.Y).Y-hudMargin)
	}
	return x, bottom
}

// socialTabsUI declares the History tab's widgets (history_ui.go); the
// Health and Social tabs' own bodies come from the pane (kitTabBody).
func (a *App) socialTabsUI() {
	a.historyTabUI(a.sw, a.sh) // history_ui.go: the HUD's History tab
}

// ---- transition shims (ARCH-005 wave 5.4): until the lead's edits land
// (lead_edits_inspector.pl), hud.go keeps the hst.pane and hst.cmds fields
// and journey_orders_test.go sets hst.cmds.testProducer. Each goes with
// its last reader.

// hasBuildTool reports the Architect has a tool that builds kind.
func hasBuildTool(kind string) bool {
	for _, t := range allArchTools() {
		if t.build == kind {
			return true
		}
	}
	return false
}

// bioSkillTip is a Bio skill row's hover card.
func bioSkillTip(s game.SkillView, week int) string {
	return inspector.BioSkillTip(skillTip(s), s, week)
}

// healthOption is inspector.HealthOption in its old field names (the root's
// health and journey tests read them).
type healthOption struct {
	label  string
	op     body.OpKind
	part   body.Part
	item   item.Kind
	reason string
}

func toHealthOptions(in []inspector.HealthOption) []healthOption {
	out := make([]healthOption, len(in))
	for i, o := range in {
		out[i] = healthOption{o.Label, o.Op, o.Part, o.Item, o.Reason}
	}
	return out
}

func pickerOptions(h game.HealthView) []healthOption {
	return toHealthOptions(inspector.PickerOptions(h))
}
func healthOptions(h game.HealthView) []healthOption {
	return toHealthOptions(inspector.HealthOptions(h))
}
func healthOptionTip(o healthOption) string {
	return inspector.HealthOptionTip(inspector.HealthOption{Label: o.label, Op: o.op, Part: o.part, Item: o.item, Reason: o.reason})
}

// itemVerb is inspector.ItemVerb in its old field names (haul_ui_test.go,
// item_inspector_test.go).
type itemVerb struct {
	id, label, icon, desc string
	match                 func(game.OrderOption) bool
}

func itemVerbs(it game.ItemView, stored bool) []itemVerb {
	in := inspector.ItemVerbs(it, stored)
	out := make([]itemVerb, len(in))
	for i, v := range in {
		out[i] = itemVerb{v.ID, v.Label, v.Icon, v.Desc, v.Match}
	}
	return out
}

// ---- aliases and wrappers

type (
	tabBody        = inspector.TabBody
	paneGeom       = inspector.Geom
	cmdButton      = inspector.Button
	cmdOption      = inspector.Option
	colonistAction = inspector.ColonistAction
	paneBar        = inspector.Bar
	paneChip       = inspector.Chip
	paneRow        = inspector.Row
	paneSpec       = inspector.Spec
	paneTab        = inspector.Tab
)

const (
	animalLogID       = inspector.AnimalLogID
	animalPartRowsMax = inspector.AnimalPartRowsMax
	bedMedicalID      = inspector.BedMedicalID
	bedOwnerModalID   = inspector.BedOwnerModalID
	billMaxCount      = inspector.BillMaxCount
	billModeDo        = inspector.BillModeDo
	billModeForever   = inspector.BillModeForever
	billModeUntil     = inspector.BillModeUntil
	billPickBtn       = inspector.BillPickBtn
	billPickID        = inspector.BillPickID
	billPickList      = inspector.BillPickList
	billRowH          = inspector.BillRowH
	casketBtnID       = inspector.CasketBtnID
	cmdLabelPad       = inspector.CmdLabelPad
	cmdMaxKeys        = inspector.CmdMaxKeys
	healthAddID       = inspector.HealthAddID
	healthBloodID     = inspector.HealthBloodID
	healthPickID      = inspector.HealthPickID
	healthPickList    = inspector.HealthPickList
	healthSelfID      = inspector.HealthSelfID
	healthStateID     = inspector.HealthStateID
	maxTroughAnimals  = inspector.MaxTroughAnimals
	paneBodyID        = inspector.PaneBodyID
	paneInset         = inspector.PaneInset
	paneRowH          = inspector.PaneRowH
	penTabW           = inspector.PenTabW
	powerConsumer     = inspector.PowerConsumer
	powerProducer     = inspector.PowerProducer
	prisonerArrest    = inspector.PrisonerArrest
	prisonerCapture   = inspector.PrisonerCapture
	prisonerRelease   = inspector.PrisonerRelease
	prisonerTabID     = inspector.PrisonerTabID
	socialEmptyText   = inspector.SocialEmptyText
	troughKind        = inspector.TroughKind
)

var (
	animalBondRows    = inspector.AnimalBondRows
	animalDietLine    = inspector.AnimalDietLine
	animalHasNeeds    = inspector.AnimalHasNeeds
	animalHealthCell  = inspector.AnimalHealthCell
	animalHealthRows  = inspector.AnimalHealthRows
	animalHuntLine    = inspector.AnimalHuntLine
	animalInitial     = inspector.AnimalInitial
	animalLabel       = inspector.AnimalLabel
	animalLifeLine    = inspector.AnimalLifeLine
	animalPartRows    = inspector.AnimalPartRows
	animalPregLine    = inspector.AnimalPregLine
	animalTitle       = inspector.AnimalTitle
	bedOwner          = inspector.BedOwner
	bedOwnerID        = inspector.BedOwnerID
	bedOwnerPickID    = inspector.BedOwnerPickID
	billLine          = inspector.BillLine
	billsBody         = inspector.BillsBody
	bioIncapable      = inspector.BioIncapable
	bioInfoLine       = inspector.BioInfoLine
	bioName           = inspector.BioName
	bioRelationLines  = inspector.BioRelationLines
	buildInfo         = inspector.BuildInfo
	buildingAt        = inspector.BuildingAt
	buildingTitle     = inspector.BuildingTitle
	capitalize        = inspector.Capitalize
	careShort         = inspector.CareShort
	carpetLine        = inspector.CarpetLine
	casketCommand     = inspector.CasketCommand
	clampLogScroll    = inspector.ClampLogScroll
	cmdLabelLines     = inspector.CmdLabelLines
	colonistOptions   = inspector.ColonistOptions
	dirtyBootsLine    = inspector.DirtyBootsLine
	drawAnimalIcon    = inspector.DrawAnimalIcon
	drawFracBar       = inspector.DrawFracBar
	fmtTarget         = inspector.FmtTarget
	frameName         = inspector.FrameName
	gearLayoutFor     = inspector.GearLayoutFor
	gearLine          = inspector.GearLine
	gearRows          = inspector.GearRows
	graveLines        = inspector.GraveLines
	hasBio            = inspector.HasBio
	healthCapID       = inspector.HealthCapID
	healthCareID      = inspector.HealthCareID
	healthChanceColor = inspector.HealthChanceColor
	healthChanceID    = inspector.HealthChanceID
	healthColor       = inspector.HealthColor
	healthCondDue     = inspector.HealthCondDue
	healthCondID      = inspector.HealthCondID
	healthDownID      = inspector.HealthDownID
	healthRemoveID    = inspector.HealthRemoveID
	healthStateColor  = inspector.HealthStateColor
	healthUpID        = inspector.HealthUpID
	inStockpile       = inspector.InStockpile
	isTrough          = inspector.IsTrough
	itemCategory      = inspector.ItemCategory
	itemLines         = inspector.ItemLines
	itemTitle         = inspector.ItemTitle
	logBodyRect       = inspector.LogBodyRect
	logLineText       = inspector.LogLineText
	logNewestFirst    = inspector.LogNewestFirst
	mergeCommands     = inspector.MergeCommands
	moodColor         = inspector.MoodColor
	opStatusColor     = inspector.OpStatusColor
	opinionBand       = inspector.OpinionBand
	opinionColor      = inspector.OpinionColor
	orderIs           = inspector.OrderIs
	paneLayout        = inspector.PaneLayout
	paneLineWarns     = inspector.PaneLineWarns
	pawnStatusLines   = inspector.PawnStatusLines
	pawnSubtitle      = inspector.PawnSubtitle
	penAnimals        = inspector.PenAnimals
	penFoodLines      = inspector.PenFoodLines
	penStatusLine     = inspector.PenStatusLine
	penTabBody        = inspector.PenTabBody
	plantBodyLines    = inspector.PlantBodyLines
	plantCategory     = inspector.PlantCategory
	plantGrowCaption  = inspector.PlantGrowCaption
	plantLines        = inspector.PlantLines
	plantMarkCmd      = inspector.PlantMarkCmd
	plantPaneLines    = inspector.PlantPaneLines
	plantRipeLine     = inspector.PlantRipeLine
	plantStageWord    = inspector.PlantStageWord
	plantTitle        = inspector.PlantTitle
	postponedLine     = inspector.PostponedLine
	powerLines        = inspector.PowerLines
	prisonerModeID    = inspector.PrisonerModeID
	recipeBenchName   = inspector.RecipeBenchName
	relationTip       = inspector.RelationTip
	statusTag         = inspector.StatusTag
	swapLine          = inspector.SwapLine
	titleCase         = inspector.TitleCase
	waitingForLine    = inspector.WaitingForLine
	weekText          = inspector.WeekText
	wrapLogRows       = inspector.WrapLogRows
	logStyle          = inspector.LogStyle
	prisonerModes     = inspector.PrisonerModes
)

func (a *App) animalCommands(id entity.EntityID) []inspector.Button {
	return a.insp.Animal().AnimalCommands(id)
}
func (a *App) animalPane(id entity.EntityID) inspector.Spec { return a.insp.Animal().AnimalPane(id) }
func (a *App) animalPaneLines(an game.AnimalView) []string {
	return a.insp.Animal().AnimalPaneLines(an)
}
func (a *App) animalStateLine(an game.AnimalView) string { return a.insp.Animal().AnimalStateLine(an) }
func (a *App) applyAnimalClick(id string)                { a.insp.Animal().ApplyAnimalClick(id) }
func (a *App) applyBuildingClick(id string)              { a.insp.Building().ApplyBuildingClick(id) }
func (a *App) applyCasketClick(id string, b game.BuildingView) bool {
	return a.insp.Building().ApplyCasketClick(id, b)
}
func (a *App) applyCasketClickFor(id string, b game.BuildingView, sealed bool) bool {
	return a.insp.Building().ApplyCasketClickFor(id, b, sealed)
}
func (a *App) applyCommandClick(id string)               { a.insp.Commands().ApplyCommandClick(id) }
func (a *App) applyFrameClick(id string)                 { a.insp.Thing().ApplyFrameClick(id) }
func (a *App) applyItemClick(id string)                  { a.insp.Thing().ApplyItemClick(id) }
func (a *App) applyPlantClick(id string)                 { a.insp.Thing().ApplyPlantClick(id) }
func (a *App) billsTabUI(id entity.EntityID, r gfx.Rect) { a.insp.Building().BillsTabUI(id, r) }
func (a *App) bioTabOn() bool                            { return a.insp.Tabs().BioTabOn() }
func (a *App) buildingCommands(id entity.EntityID) []inspector.Button {
	return a.insp.Building().BuildingCommands(id)
}
func (a *App) buildingPane(id entity.EntityID) inspector.Spec {
	return a.insp.Building().BuildingPane(id)
}
func (a *App) buildingView(id entity.EntityID) (game.BuildingView, bool) {
	return a.insp.BuildingView(id)
}
func (a *App) casketCommands(b game.BuildingView) []inspector.Button {
	return a.insp.Building().CasketCommands(b)
}
func (a *App) clearBuilding()     { a.insp.Building().ClearBuilding() }
func (a *App) clearFrame()        { a.insp.Thing().ClearFrame() }
func (a *App) closePaneTab() bool { return a.insp.ClosePaneTab() }
func (a *App) colonistActions(tx, ty int, match func(game.OrderOption) bool) []inspector.ColonistAction {
	return a.insp.Pawn().ColonistActions(tx, ty, match)
}
func (a *App) commandCount() int                  { return a.insp.Commands().CommandCount() }
func (a *App) commandGridButtons() []uikit.Button { return a.insp.Commands().CommandGridButtons() }
func (a *App) commandGridUI(sw, sh int)           { a.insp.Commands().CommandGridUI(sw, sh) }
func (a *App) commandKeyLabel(i int, c inspector.Button) string {
	return a.insp.Commands().CommandKeyLabel(i, c)
}
func (a *App) creatureName(id entity.EntityID) string { return a.insp.Pawn().CreatureName(id) }
func (a *App) drawCommandGrid(screen *ebiten.Image)   { a.insp.Commands().DrawCommandGrid(screen) }
func (a *App) drawPane(screen *ebiten.Image)          { a.insp.DrawPane(screen) }
func (a *App) drawPassion(dst *ebiten.Image, p int, x, y, size float64, st gfx.TextStyle, ty float64) {
	a.insp.Tabs().DrawPassion(dst, p, x, y, size, st, ty)
}
func (a *App) drawPawnLogRows(screen *ebiten.Image, p game.PawnView, r gfx.Rect) {
	a.insp.Pawn().DrawPawnLogRows(screen, p, r)
}
func (a *App) expandedRect() gfx.Rect           { return a.insp.ExpandedRect() }
func (a *App) expandedWant() (float32, float32) { return a.insp.ExpandedWant() }
func (a *App) frameCommands(id entity.EntityID) []inspector.Button {
	return a.insp.Thing().FrameCommands(id)
}
func (a *App) framePane(id entity.EntityID) inspector.Spec { return a.insp.Thing().FramePane(id) }
func (a *App) groupButtons() []uikit.Button                { return a.insp.Commands().GroupButtons() }
func (a *App) groupCommands() []inspector.Button           { return a.insp.Commands().GroupCommands() }
func (a *App) groupPane() inspector.Spec                   { return a.insp.Commands().GroupPane() }
func (a *App) handleLogWheel() bool                        { return a.insp.Pawn().HandleLogWheel() }
func (a *App) healthLines(p game.PawnView) []string        { return a.insp.Tabs().HealthLines(p) }
func (a *App) itemCommands(id entity.EntityID) []inspector.Button {
	return a.insp.Thing().ItemCommands(id)
}
func (a *App) itemPane(id entity.EntityID) inspector.Spec { return a.insp.Thing().ItemPane(id) }
func (a *App) kitTabOn() bool                             { return a.insp.Tabs().KitTabOn() }
func (a *App) moveCommands(b game.BuildingView) []inspector.Button {
	return a.insp.Building().MoveCommands(b)
}
func (a *App) openPaneTab(name string) bool { return a.insp.OpenPaneTab(name) }
func (a *App) paneButtons() []uikit.Button  { return a.insp.PaneButtons() }
func (a *App) paneOpenTab() string          { return a.insp.PaneOpenTab() }
func (a *App) paneRect() gfx.Rect           { return a.insp.PaneRect() }
func (a *App) paneShown() bool              { return a.insp.PaneShown() }
func (a *App) paneUI(sw, sh int)            { a.insp.PaneUI(sw, sh) }
func (a *App) pawnCommands(id entity.EntityID) []inspector.Button {
	return a.insp.Pawn().PawnCommands(id)
}
func (a *App) pawnLines(p game.PawnView) []string             { return a.insp.Pawn().PawnLines(p) }
func (a *App) pawnName(id entity.EntityID) string             { return a.insp.Building().PawnName(id) }
func (a *App) pawnPane(id entity.EntityID) inspector.Spec     { return a.insp.Pawn().PawnPane(id) }
func (a *App) penOf(b game.BuildingView) (game.PenView, bool) { return a.insp.Building().PenOf(b) }
func (a *App) plantCommands(id entity.EntityID) []inspector.Button {
	return a.insp.Thing().PlantCommands(id)
}
func (a *App) plantPane(id entity.EntityID) inspector.Spec { return a.insp.Thing().PlantPane(id) }
func (a *App) powerSelection()                             { a.insp.Building().PowerSelection() }
func (a *App) pressCommandKey(i int)                       { a.insp.Commands().PressCommandKey(i) }
func (a *App) prisonerTabOn() bool                         { return a.insp.Tabs().PrisonerTabOn() }
func (a *App) prisonerWarnings(p game.PawnView, v game.PrisonerView) []string {
	return a.insp.Tabs().PrisonerWarnings(p, v)
}
func (a *App) pruneGroup()                              { a.insp.Commands().PruneGroup() }
func (a *App) recipesFor(kind string) []game.RecipeView { return a.insp.Building().RecipesFor(kind) }
func (a *App) rightClickCommand(mx, my int) bool        { return a.insp.Commands().RightClickCommand(mx, my) }
func (a *App) runCommand(c inspector.Button)            { a.insp.Commands().RunCommand(c) }
func (a *App) scrollLogAt(mx, my int, wy float64) bool  { return a.insp.Pawn().ScrollLogAt(mx, my, wy) }
func (a *App) scrollLogIn(r gfx.Rect, mx, my int, wy float64) bool {
	return a.insp.Pawn().ScrollLogIn(r, mx, my, wy)
}
func (a *App) selectAnimalAt(tx, ty int) bool              { return a.insp.Animal().SelectAnimalAt(tx, ty) }
func (a *App) selectBuildingAt(tx, ty int) bool            { return a.insp.Building().SelectBuildingAt(tx, ty) }
func (a *App) selectPaneTab(id string)                     { a.insp.SelectPaneTab(id) }
func (a *App) selectedAnimal() (game.AnimalView, bool)     { return a.insp.SelectedAnimal() }
func (a *App) selectedBuilding() (game.BuildingView, bool) { return a.insp.SelectedBuilding() }
func (a *App) selectedFrame() (game.FrameView, bool)       { return a.insp.SelectedFrame() }
func (a *App) selectedPawn() (game.PawnView, bool)         { return a.insp.SelectedPawn() }
func (a *App) selectionCommands() []inspector.Button       { return a.insp.Commands().SelectionCommands() }
func (a *App) selectionPane() inspector.Spec               { return a.insp.SelectionPane() }
func (a *App) socialLines(p game.PawnView) []string        { return a.insp.Tabs().SocialLines(p) }
func (a *App) syncLogFor(id entity.EntityID)               { a.insp.Pawn().SyncLogFor(id) }
