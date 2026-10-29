package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"rimworld-go/internal/application/game"
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
	h.a.ord.OpenPick(x, y, up, entries, pick)
}

func (h inspectorHost) Hostiles() []game.PawnView {
	if r := h.a.renderer; r != nil {
		return r.hostiles()
	}
	return nil
}

// ---- inspector.Peers

func (h inspectorHost) AreaPicker(title string, cur int, base bool, pick func(ref int)) {
	h.a.arch.Areas.OpenPicker(title, cur, base, pick)
}
func (h inspectorHost) AssignArea(id entity.EntityID, ref int)          { h.a.arch.Areas.Assign(id, ref) }
func (h inspectorHost) AreaChip(p game.PawnView) (inspector.Chip, bool) { return h.a.areaChip(p) }
func (h inspectorHost) LockedBy(key string) string                      { return h.a.win.Research().LockedBy(key) }
func (h inspectorHost) MoveState(b game.BuildingView) (bool, string) {
	return architect.MovableKind(b.Kind, b.Floor), architect.MovingLine(b)
}
func (h inspectorHost) Buildable(kind string) bool { return hasBuildTool(kind) }

func (h inspectorHost) AreaLabel(ref int) (label, tip string) {
	var areas []game.AreaView
	if r := h.a.renderer; r != nil {
		areas = r.game.Areas()
	}
	return architect.AreaRefLabel(areas, ref), architect.AssignAreaTip
}

func (h inspectorHost) Move(b game.BuildingView, cancel bool) {
	if !cancel {
		h.a.arch.Mover.Start(b)
		return
	}
	if f, ok := architect.PendingMoveFrame(h.a.shell.World(), b.ID); ok {
		h.a.shell.Enqueue(architect.MoveCancelRect(f))
	}
}

// ---- inspector.Labels

func (h inspectorHost) PawnLabel(p game.PawnView) string { return hud.PawnLabel(p) }
func (h inspectorHost) Temp(tenths int) string           { return hud.FmtTemp(tenths) }
func (h inspectorHost) WildRow(species string) (game.WildlifeRow, bool) {
	return h.a.win.Animals().WildRow(species)
}
func (h inspectorHost) AnimalBlurb(species string) string { return windows.AnimalBlurb(species) }
func (h inspectorHost) Skill(s game.SkillView) (level, tip string) {
	return wizard.SkillLevelText(s), wizard.SkillTip(s)
}
func (h inspectorHost) Traits(x, y, w float32, v game.CandidateView) float32 {
	return h.a.wiz.TraitChips(x, y, w, v)
}

func (h inspectorHost) Backstory(x, y, w float32, label, title, desc, id string) {
	h.a.wiz.BackstoryLine(x, y, w, label, title, desc, id)
}

func (h inspectorHost) Portrait(dst *ebiten.Image, l look.Look, scarred bool, box gfx.Rect, name string) {
	if !drawPortrait(dst, h.a.art, l, scarred, float64(box.X), float64(box.Y), float64(box.W), float64(box.H)) {
		wizard.DrawSilhouette(dst, box, name)
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
	if lines := a.chrome.Cell().Lines(); len(lines) > 0 { // nothing selected: rest above the cell info
		bottom = min(bottom, hud.CellInfoBox(lines, l.Tabs.Y).Y-hudlayout.HudMargin)
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
		if t.Build == kind {
			return true
		}
	}
	return false
}

// healthOption is inspector.HealthOption in its old field names (the root's
// health and journey tests read them).
type healthOption = inspector.HealthOption

// itemVerb is inspector.ItemVerb in its old field names (haul_ui_test.go,
// item_inspector_test.go).
type itemVerb = inspector.ItemVerb

// ---- aliases and wrappers

func (a *App) applyAnimalClick(id string)   { a.insp.Animal().ApplyAnimalClick(id) }
func (a *App) applyBuildingClick(id string) { a.insp.Building().ApplyBuildingClick(id) }

func (a *App) applyCommandClick(id string) { a.insp.Commands().ApplyCommandClick(id) }
func (a *App) applyFrameClick(id string)   { a.insp.Thing().ApplyFrameClick(id) }
func (a *App) applyItemClick(id string)    { a.insp.Thing().ApplyItemClick(id) }
func (a *App) applyPlantClick(id string)   { a.insp.Thing().ApplyPlantClick(id) }

func (a *App) selectPaneTab(id string) { a.insp.SelectPaneTab(id) }
