package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
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
		e.Colony = a.nav.screen() == screenColony
		e.ReducedMotion, e.ViewCached = r.reducedMotion, r.viewCached
		_, pawn := a.shell.Selected()
		e.NothingSelected = !a.grouped() && !pawn && !a.sel.HasAnimal && !a.sel.HasItem && !a.sel.HasPlant &&
			!a.sel.HasBuilding && !a.sel.HasFrame && !a.sel.HasZone
	}
	return e
}

func (h hudHost) Layout() hudlayout.Layout          { return h.a.hud() }
func (h hudHost) ChromeHit(mx, my, sw, sh int) bool { return h.a.colonyChromeHit(mx, my, sw, sh) }
func (h hudHost) SpeedLabel() string                { return h.a.speedLabel() }
func (h hudHost) SetSettings(s config.Settings)     { h.a.setSettings(s) }
func (h hudHost) Pick(c selection.Cand)             { h.a.selectCand(c) }
func (h hudHost) Picked(id entity.EntityID) bool    { return h.a.inGroup(selPawn, id) }
func (h hudHost) Power() (legend, tip string)       { return h.a.powerLegend(), h.a.powerToggleTip() }
func (h hudHost) StoneItem(key, kind string, cs colorScale) (string, colorScale) {
	return h.a.renderer.stoneItem(key, kind, cs)
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
		return a.win.KitID() == assignModalID
	case ids.TabWorld:
		return a.win.KitID() == worldModalID
	case ids.TabHistory:
		return a.win.KitID() == historyModalID
	case ids.TabFactions:
		return a.win.KitID() == colFacModalID
	}
	return false
}

// drawPawnPortrait draws the whole colonist, fitted into the logical box
// (x, y, w, h): the inspector's portrait.
func (a *App) drawPawnPortrait(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) {
	if a.renderer.art != nil {
		a.renderer.drawPawnPortraitArt(screen, p, x, y, w, h)
	}
}

// ---- aliases

const (
	barIconMin        = hud.BarIconMin
	barIconSize       = hud.BarIconSize
	barIdleHold       = hud.BarIdleHold
	barMoreID         = hud.BarMoreID
	globalInnerW      = hud.GlobalInnerW
	globalPad         = hud.GlobalPad
	globalPerRow      = hud.GlobalPerRow
	homeButtonID      = hud.HomeButtonID
	planButtonID      = hud.PlanButtonID
	roofButtonID      = hud.RoofButtonID
	roomsButtonID     = hud.RoomsButtonID
	letterFlashFrames = hud.LetterFlashFrames
	letterFresh       = hud.LetterFresh
	messageFadeFrames = hud.MessageFadeFrames
	messageFrames     = hud.MessageFrames
	resScrollID       = hud.ResScrollID
	toastFrames       = hud.ToastFrames
)

type alertSev = hud.AlertSev

func alertExplain(key string) string                 { return hud.AlertExplain(key) }
func barShown(l hudLayout) int                       { return hud.BarShown(l) }
func barSlotID(id entity.EntityID) string            { return hud.BarSlotID(id) }
func barTip(p game.PawnView) string                  { return hud.BarTip(p) }
func cellInfoBox(lines []string, tabsY float32) rect { return hud.CellInfoBox(lines, tabsY) }
func dateSeasonLine(d clock.Date, season string, maxW float64) string {
	return hud.DateSeasonLine(d, season, maxW)
}
func detailLines(w game.WorldView, x, y int) []string { return hud.DetailLines(w, x, y) }
func fmtTemp(tenths int) string                       { return hud.FmtTemp(tenths) }
func gatheringText(v game.GatheringView) string       { return hud.GatheringText(v) }
func grassLabelAt(w game.WorldView, x, y int) string  { return hud.GrassLabelAt(w, x, y) }
func homeLabelAt(w game.WorldView, x, y int) string   { return hud.HomeLabelAt(w, x, y) }
func letterColor(kind string) color.RGBA              { return hud.LetterColor(kind) }
func letterKind(kind string) int                      { return hud.LetterKind(kind) }
func messageAlpha(age uint64) float64                 { return hud.MessageAlpha(age) }
func messageEdge(m game.MessageView) color.RGBA       { return hud.MessageEdge(m) }
func messageLine(m game.MessageView) string           { return hud.MessageLine(m) }
func messageStyle(m game.MessageView) textStyle       { return hud.MessageStyle(m) }
func pawnLabel(p game.PawnView) string                { return hud.PawnLabel(p) }
func planLabelAt(w game.WorldView, x, y int) string   { return hud.PlanLabelAt(w, x, y) }
func roomCardLines(rm game.RoomView) []string         { return hud.RoomCardLines(rm) }
func roomTint(score int) color.NRGBA                  { return hud.RoomTint(score) }
func soilLines(w game.WorldView, x, y int) []string   { return hud.SoilLines(w, x, y) }
func textRowH() float32                               { return hud.TextRowH() }

// ---- converting shims (lowercase-field copies for the root tests)

// alert is hud.Alert as the root tests read it.
type alert struct {
	key     string
	label   string
	danger  bool
	sev     alertSev
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

func buildAlerts(pawns []game.PawnView, w game.WorldView) []alert {
	return toAlerts(hud.BuildAlerts(pawns, w))
}
func tradeAlertsFrom(src any) []alert { return toAlerts(hud.TradeAlertsFrom(src)) }
func fireAlert(w game.WorldView) (alert, bool) {
	a, ok := hud.FireAlert(w)
	return toAlert(a), ok
}
func waitingFramesAlert(w game.WorldView) (alert, bool) {
	a, ok := hud.WaitingFramesAlert(w)
	return toAlert(a), ok
}
func (a *App) currentAlerts() []alert { return toAlerts(a.chrome.Alerts().Current()) }

// alertRow is hud.AlertRow as the root tests read it.
type alertRow struct {
	r      rect
	al     alert
	more   int
	hidden []alert
}

func alertRowLabel(r alertRow) string {
	return hud.AlertRowLabel(hud.AlertRow{R: r.r, Al: fromAlert(r.al), More: r.more, Hidden: fromAlerts(r.hidden)})
}

func (a *App) alertRows(l hudLayout) []alertRow {
	rows := a.chrome.Alerts().Rows(l)
	if rows == nil {
		return nil
	}
	out := make([]alertRow, len(rows))
	for i, r := range rows {
		out[i] = alertRow{r: r.R, al: toAlert(r.Al), more: r.More, hidden: toAlerts(r.Hidden)}
	}
	return out
}

// rightCard is hud.RightCard as the root tests read it.
type rightCard struct {
	title, body string
	clr         color.RGBA
}

func layoutRightCard(c rightCard, anchor rect, sw, sh int) (box rect, title, body []string) {
	return hud.LayoutRightCard(hud.RightCard{Title: c.title, Body: c.body, Clr: c.clr}, anchor, sw, sh)
}

func (a *App) rightHover(l hudLayout) (rightCard, rect, bool) {
	c, r, ok := a.chrome.Right().Hover(l)
	return rightCard{title: c.Title, body: c.Body, clr: c.Clr}, r, ok
}

// letterRow is hud.LetterRow as the root tests read it.
type letterRow struct {
	r      rect
	letter game.LetterView
	older  int
}

func letterRowLabel(r letterRow) string {
	return hud.LetterRowLabel(hud.LetterRow{R: r.r, Letter: r.letter, Older: r.older})
}

func (a *App) letterRows(l hudLayout) []letterRow {
	rows := a.chrome.Right().LetterRows(l)
	if rows == nil {
		return nil
	}
	out := make([]letterRow, len(rows))
	for i, r := range rows {
		out[i] = letterRow{r: r.R, letter: r.Letter, older: r.Older}
	}
	return out
}

// letterKinds is hud.LetterKinds as the root tests read it.
var letterKinds = func() []struct {
	kind, icon string
	clr        color.RGBA
} {
	out := make([]struct {
		kind, icon string
		clr        color.RGBA
	}, len(hud.LetterKinds))
	for i, k := range hud.LetterKinds {
		out[i].kind, out[i].icon, out[i].clr = k.Kind, k.Icon, k.Clr
	}
	return out
}()

// messageRow is hud.MessageRow as the root tests read it.
type messageRow struct {
	r     rect
	msg   game.MessageView
	idx   int
	alpha float64
}

func (a *App) messageRows(l hudLayout) []messageRow {
	rows := a.chrome.Right().MessageRows(l)
	if rows == nil {
		return nil
	}
	out := make([]messageRow, len(rows))
	for i, r := range rows {
		out[i] = messageRow{r: r.R, msg: r.Msg, idx: r.Idx, alpha: r.Alpha}
	}
	return out
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

func statusChips(w game.WorldView) []statusChip { return toChips(hud.StatusChips(w)) }
func (a *App) chipRects(chips []statusChip, sw int) []rect {
	return a.chrome.Global().ChipRects(fromChips(chips), sw)
}

// globalPlan is hud.GlobalPlan as the root tests read it.
type globalPlan struct {
	reason  string
	weather bool
	chips   []statusChip
	legend  []string
}

func (a *App) globalPlan() globalPlan {
	p := a.chrome.Global().Plan()
	return globalPlan{reason: p.Reason, weather: p.Weather, chips: toChips(p.Chips), legend: p.Legend}
}

// globalToggleList is hud.GlobalToggles as the root tests read it.
var globalToggleList = func() []struct{ id, icon, short string } {
	out := make([]struct{ id, icon, short string }, len(hud.GlobalToggles))
	for i, t := range hud.GlobalToggles {
		out[i].id, out[i].icon, out[i].short = t.ID, t.Icon, t.Short
	}
	return out
}()

// toast is hud.Toast as the root tests read it.
type toast struct {
	text string
	at   uint64
}

func (a *App) liveToasts() []toast {
	ts := a.chrome.LiveToasts()
	if ts == nil {
		return nil
	}
	out := make([]toast, len(ts))
	for i, t := range ts {
		out[i] = toast{text: t.Text, at: t.At}
	}
	return out
}

// resRow is hud.ResRow as the root tests read it.
type resRow struct {
	header           bool
	cat, kind, label string
	qty              int
}

func (a *App) resourceRows() []resRow {
	rows := a.chrome.Res().Rows()
	if rows == nil {
		return nil
	}
	out := make([]resRow, len(rows))
	for i, r := range rows {
		out[i] = resRow{header: r.Header, cat: r.Cat, kind: r.Kind, label: r.Label, qty: r.Qty}
	}
	return out
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

func (a *App) barStatusesOf(p game.PawnView) []barStatus {
	return toStatuses(a.chrome.Bar().StatusesOf(p))
}

// ---- one-line wrappers

func (a *App) alertButtons(sw int) []button                { return a.chrome.Alerts().Buttons(sw) }
func (a *App) alertButtonsIn(l hudLayout) []button         { return a.chrome.Alerts().ButtonsIn(l) }
func (a *App) alertCount() int                             { return a.chrome.Alerts().Count() }
func (a *App) applyLetterClick(id string)                  { a.chrome.Right().LetterClick(id) }
func (a *App) applyResourceClick(id string)                { a.chrome.Res().Click(id) }
func (a *App) applySpeedClick(id string)                   { a.chrome.Global().SpeedClick(id) }
func (a *App) barCount() int                               { return a.chrome.Bar().Count() }
func (a *App) barDoubleClick(id entity.EntityID) bool      { return a.chrome.Bar().DoubleClick(id) }
func (a *App) barPawns() []game.PawnView                   { return a.chrome.Bar().Pawns() }
func (a *App) barSelected(id entity.EntityID) bool         { return a.chrome.Bar().Picked(id) }
func (a *App) cellInfoLines() []string                     { return a.chrome.Cell().Lines() }
func (a *App) cellInfoRect() rect                          { return a.chrome.Cell().Rect() }
func (a *App) colonistBarButtons(sw, sh int) []button      { return a.chrome.Bar().Buttons(sw, sh) }
func (a *App) dateLine() string                            { return a.chrome.Global().DateLine() }
func (a *App) drawAlerts(screen *ebiten.Image, sw int)     { a.chrome.Alerts().Draw(screen, sw) }
func (a *App) drawCellInfo(screen *ebiten.Image, sh int)   { a.chrome.Cell().Draw(screen, sh) }
func (a *App) drawColonistBar(s *ebiten.Image, sw, sh int) { a.chrome.Bar().Draw(s, sw, sh) }
func (a *App) drawColonistPortraits(s *ebiten.Image, sh int) {
	a.chrome.Bar().DrawPortraits(s, sh)
}
func (a *App) drawGatheringBanner(s *ebiten.Image, sw int)    { a.chrome.Banner().Draw(s, sw) }
func (a *App) drawGlobalControls(s *ebiten.Image, sw, sh int) { a.chrome.Global().Draw(s, sw, sh) }
func (a *App) drawMessages(s *ebiten.Image)                   { a.chrome.Right().DrawMessages(s) }
func (a *App) drawMessagesIn(s *ebiten.Image, l hudLayout)    { a.chrome.Right().DrawMessagesIn(s, l) }
func (a *App) drawResources(s *ebiten.Image, sw, sh int)      { a.chrome.Res().Draw(s, sw, sh) }
func (a *App) drawRightColumn(s *ebiten.Image, sw, sh int)    { a.chrome.Right().Draw(s, sw, sh) }
func (a *App) drawStatusChips(s *ebiten.Image, sw int)        { a.chrome.Global().DrawChips(s, sw) }
func (a *App) drawTabs(s *ebiten.Image, sw, sh int)           { a.chrome.Tabs().Draw(s, sw, sh) }
func (a *App) drawToasts(s *ebiten.Image, sw, sh int)         { a.chrome.DrawToasts(s, sw, sh) }
func (a *App) focusAlert(id string)                           { a.chrome.Alerts().Focus(id) }
func (a *App) focusMessage(id string)                         { a.chrome.Right().FocusMessage(id) }
func (a *App) gatheringBanner(sw int) (string, rect, bool)    { return a.chrome.Banner().Line(sw) }
func (a *App) gatheringUI(sw int)                             { a.chrome.Banner().Declare(sw) }
func (a *App) globalButtons(sw int) []button                  { return a.chrome.Global().Buttons(sw) }
func (a *App) globalRows() hud.GlobalRowRects                 { return a.chrome.Global().Rows() }
func (a *App) globalToggleRect(id string) (rect, bool)        { return a.chrome.Global().ToggleRect(id) }
func (a *App) globalUI(sw, sh int)                            { a.chrome.Global().Declare(sw, sh) }
func (a *App) hourLine() string                               { return a.chrome.Global().HourLine() }
func (a *App) hudTabButtons(sw, sh int) []button              { return a.chrome.Tabs().Buttons(sw, sh) }
func (a *App) hudTimeText() string                            { return a.chrome.Global().TimeText() }
func (a *App) letterButtons() []button                        { return a.chrome.Right().LetterButtons() }
func (a *App) letterCount() int                               { return a.chrome.Right().LetterCount() }
func (a *App) letterFlash(id uint64) float64                  { return a.chrome.Right().LetterFlash(id) }
func (a *App) letterKey(m game.MessageView) string            { return a.chrome.Right().LetterKey(m) }
func (a *App) letterRead(id uint64) bool                      { return a.chrome.Right().LetterRead(id) }
func (a *App) lettersUI()                                     { a.chrome.Right().DeclareLetters() }
func (a *App) messageButtons(sw int) []button                 { return a.chrome.Right().MessageButtons(sw) }
func (a *App) messageCount() int                              { return a.chrome.Right().MessageCount() }
func (a *App) openLetter(m game.MessageView)                  { a.chrome.Right().OpenLetter(m) }
func (a *App) resourceButtons() []button                      { return a.chrome.Res().Buttons() }
func (a *App) resourceIcon(kind string) (*ebiten.Image, colorScale) {
	return a.chrome.Res().Icon(kind)
}
func (a *App) resourcesHeight() float32              { return a.chrome.Res().Height() }
func (a *App) rightClickResources(mx, my int) bool   { return a.chrome.Res().RightClick(mx, my) }
func (a *App) rightClickRightColumn(mx, my int) bool { return a.chrome.Right().RightClick(mx, my) }
func (a *App) rightColumnButtons(sw int) []button    { return a.chrome.Right().Buttons(sw) }
func (a *App) rightColumnUI(sw, sh int)              { a.chrome.Right().Declare(sw, sh) }
func (a *App) roofLegend() string                    { return a.chrome.Global().RoofLegend() }
func (a *App) statusChipsUI(sw int)                  { a.chrome.Global().DeclareChips(sw) }
func (a *App) temperatureLine() string               { return a.chrome.Global().TemperatureLine() }
func (a *App) toast(msg string)                      { a.chrome.Toast(msg) }
func (a *App) toastRects(sw, sh int) []rect          { return a.chrome.ToastRects(sw, sh) }

// globalHeight is the Global controls' height the layout reserves; the
// minimap, which sits above Global, learns it here too.
func (a *App) globalHeight() float32 {
	h := a.chrome.Global().Height()
	if a.renderer != nil {
		a.renderer.mini.GlobalH = h
	}
	return h
}
