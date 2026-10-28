package ebiten

import (
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/pawn/skill"
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

func (h windowsHost) WindowRect() rect { return h.a.hud().Window }

func (h windowsHost) CenterOnMap(wx, wy float64) {
	if h.a.renderer != nil {
		h.a.centerOnMap(wx, wy)
	}
}

func (h windowsHost) AreaDropdown(id string, r rect, ref int, pick func(ref int)) {
	h.a.areaDropdown(id, r, ref, pick)
}

func (h windowsHost) ManageAreas() { h.a.openAreaManager() }

func (h windowsHost) Float(x, y float32, opts []game.OrderOption) {
	if opts == nil {
		h.a.closeFloat()
		return
	}
	h.a.openPick(x, y, false, opts, func(i int) { h.a.shell.Enqueue(opts[i].Cmd) })
}

func (h windowsHost) FormCaravan(globePick func() int) { h.a.deal.OpenFormCaravan(globePick) }

func (h windowsHost) FactionsTable(body rect, maxH float32, sel *int) rect {
	rows := h.colonyRows()
	plain := make([]facRow, len(rows))
	for i, r := range rows {
		plain[i] = r.facRow
	}
	listH := min(float32(tableHeaderH+len(rows)*facRowH+4), maxH)
	tb, rest := body.SplitV(listH)
	h.a.factionsTable(tb, plain, sel, nil)
	return rest
}

// ---- windows.Labels

func (h windowsHost) PawnLabel(p game.PawnView) string       { return pawnLabel(p) }
func (h windowsHost) AnimalLabel(an game.AnimalView) string  { return animalLabel(an) }
func (h windowsHost) AnimalHealth(an game.AnimalView) string { return animalHealthCell(an) }
func (h windowsHost) CareShort(p medcare.Policy) string      { return careShort(p) }
func (h windowsHost) Caravan(c game.CaravanView) (string, string) {
	return caravanTip(c), caravanHealthNote(c)
}

func (h windowsHost) Factions() []windows.Faction {
	rows := h.colonyRows()
	out := make([]windows.Faction, len(rows))
	for i, r := range rows {
		out[i] = windows.Faction{
			Name: r.Name, TypeLabel: r.TypeLabel, Tech: r.Tech, Goodwill: r.Goodwill, Relation: r.Relation,
			Standing: relationLabel(r.Relation), Color: relationColor(r.Relation), Tip: goodwillTip(r.facRow),
			History: r.History, Events: r.Events,
		}
	}
	return out
}

func (h windowsHost) DrawAnimalIcon(dst *ebiten.Image, an game.AnimalView, x, y, size float64) {
	if r := h.a.renderer; r != nil {
		drawAnimalIcon(dst, r.art, an, x, y, size)
	}
}

func (h windowsHost) DrawPassion(dst *ebiten.Image, p int, x, y, size float64, st textStyle, ty float64) {
	h.a.drawPassion(dst, p, x, y, size, st, ty)
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
	facRow
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
	return color.NRGBA{R: uiTextDim.R, G: uiTextDim.G, B: uiTextDim.B, A: 0xFF}
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
		r := facRow{
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
		rows = append(rows, colFacRow{facRow: r, History: hist, Events: v.Events})
	}
	for _, s := range factionRows(nil) { // the Swarm
		rows = append(rows, colFacRow{facRow: s})
	}
	return rows
}

// ---- the HUD's tab buttons for the windows (the HUD draws them; these
// widgets answer their clicks)

// tabWidget declares the widget of the HUD tab id (when enabled) that runs
// click; tip carries the button's hover card over.
func (a *App) tabWidget(sw, sh int, id string, tip bool, click func()) {
	for _, b := range a.hudTabButtons(sw, sh) {
		if b.ID != id || b.Disabled {
			continue
		}
		w := uiWidget{ID: b.ID, Label: b.Label, R: rect{X: b.X, Y: b.Y, W: b.W, H: b.H}, Click: click}
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

func (a *App) windowKitOpen() bool { return a.win.KitOpen() }

func (a *App) uiOpenWindow(m modal)   { a.win.OpenKit(a.kitModal(m)) }
func (a *App) uiCloseWindow()         { a.win.CloseKit() }
func (a *App) closeWindowQuietly()    { a.win.CloseQuietly() }
func (a *App) showWindow(mode string) { a.win.Show(mode) }

// ---- window wrappers

func (a *App) drawWindows(screen *ebiten.Image, sw, sh int) { a.win.Draw(screen, sw, sh) }
func (a *App) windowWant() (float32, float32)               { return a.win.Want() }
func (a *App) windowRect() button                           { return a.win.Rect() }
func (a *App) windowAnchor() rect                           { return a.win.Anchor() }
func (a *App) applyWindowClick(id string)                   { a.win.Click(id) }
func (a *App) toggleWildlifeTab()                           { a.win.ToggleWildlife() }
func (a *App) workButtons() []button                        { return a.win.Buttons() }
func (a *App) workTipsUI(sw, sh int)                        { a.win.Work().Tips(sw, sh) }
func (a *App) scheduleRows() []uint64                       { return a.win.Schedule().Rows() }
func (a *App) paintSchedule(id string)                      { a.win.Schedule().Paint(id) }
func (a *App) handleScheduleDrag()                          { a.win.Schedule().Drag() }
func (a *App) toggleAssign()                                { a.win.Assign().Toggle() }
func (a *App) openAssign()                                  { a.win.Assign().Launch() }
func (a *App) animalsTabButtons() []button                  { return a.win.Animals().Buttons() }
func (a *App) animalsTabUI(sw, sh int)                      { a.win.Animals().Declare(sw, sh) }
func (a *App) applyAnimalsTab(id string)                    { a.win.Animals().Click(id) }
func (a *App) drawAnimalsTab(screen *ebiten.Image, sw, sh int) {
	a.win.Animals().Draw(screen, sw, sh)
}
func (a *App) wildRow(species string) (game.WildlifeRow, bool) {
	return a.win.Animals().WildRow(species)
}
func (a *App) lockedBy(key string) string { return a.win.Research().LockedBy(key) }
func (a *App) researchUI(sw, sh int)      { a.win.Research().Declare(sw, sh) }
func (a *App) researchButtons() []button  { return a.win.Research().Buttons() }
func (a *App) pickResearch(id string)     { a.win.Research().Pick(id) }
func (a *App) hasResearchBench() bool     { return a.win.Research().HasBench() }
func (a *App) openHistory()               { a.win.History().Launch() }
func (a *App) openColonyFactions()        { a.win.Factions().Launch() }
func (a *App) openWorld()                 { a.win.World().Launch() }
func (a *App) toggleWorld()               { a.win.World().Toggle() }

// ---- aliases

const (
	assignModalID  = windows.AssignModalID
	assignRowH     = windows.AssignRowH
	colFacModalID  = windows.ColFacModalID
	historyModalID = windows.HistoryModalID
	worldModalID   = windows.WorldModalID
	schedCellW     = windows.SchedCellW
	schedRowH      = windows.SchedRowH
	schedTop       = windows.SchedTop
	workNameW      = windows.WorkNameW
)

type worldSource = windows.WorldSource

var (
	animalBlurbs  = windows.AnimalBlurbs
	assignPresets = windows.AssignPresets
	workColumns   = windows.WorkColumns
	workHelp      = windows.WorkHelp
	workLive      = windows.WorkLive
)

// workColSkill mirrors windows.WorkColSkill in its old field names for
// social_tabs_ui_test.go.
var workColSkill = func() (out [len(windows.WorkColSkill)]struct {
	sk skill.ID
	ok bool
}) {
	for i, c := range windows.WorkColSkill {
		out[i].sk, out[i].ok = c.Skill, c.OK
	}
	return out
}()

var (
	animalAgeText      = windows.AnimalAgeText
	animalBlurb        = windows.AnimalBlurb
	animalDoing        = windows.AnimalDoing
	animalMasterCell   = windows.AnimalMasterCell
	animalPregCell     = windows.AnimalPregCell
	animalProductText  = windows.AnimalProductText
	animalSexLetter    = windows.AnimalSexLetter
	animalTrainingCell = windows.AnimalTrainingCell
	assignCareID       = windows.AssignCareID
	assignDefCareID    = windows.AssignDefCareID
	assignDrugID       = windows.AssignDrugID
	caravanReturnLabel = windows.CaravanReturnLabel
	chartPoints        = windows.ChartPoints
	countZones         = windows.CountZones
	drawGoodwillChart  = windows.DrawGoodwillChart
	followCell         = windows.FollowCell
	historyRows        = windows.HistoryRows
	nextCare           = windows.NextCare
	nextPreset         = windows.NextPreset
	nextPriority       = windows.NextPriority
	presetLabel        = windows.PresetLabel
	workCellRect       = windows.WorkCellRect
	workCellValue      = windows.WorkCellValue
	workPriority       = windows.WorkPriority
	workTypeForCol     = windows.WorkTypeForCol
)

// animalAreaCell is the Area / Pen column's text over the areas.
func animalAreaCell(an game.AnimalView, areas []game.AreaView) string {
	return windows.AnimalAreaCell(an, areaRefLabel(areas, an.AreaRef))
}

// animalRow is one line of the Animals tab (animal_species_test.go).
func animalRow(an game.AnimalView) string { return windows.AnimalRow(an, animalLabel(an)) }
