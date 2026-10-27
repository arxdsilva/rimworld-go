package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// The colonist bar (PLAY-018: HUD-TOP owns it): one compact card per
// colonist, top-centre, in the layout's BarSlots. A card is a mood backing
// (a fill that rises with the mood), the colonist's head and shoulders,
// status marks and, at full size, the name. A selected colonist's card has
// an accent outline; the last slot of an overflowing bar reads "+N".

const (
	barMoreID = "bar.more" // the "+N" slot (a kit widget: ../hud.go routes no click for it)

	barPad      = 2  // card inset, at bar scale 1
	barIconSize = 14 // a status mark, at bar scale 1
	barIconMin  = 10 // a status mark never gets smaller than this
	barNameMin  = 1  // names are drawn at this bar scale and above; smaller slots use the tooltip

	// barIdleHold is how long a colonist has to stand idle before the
	// "idle" mark shows, so the gap between two jobs does not flash it.
	barIdleHold = 2 * clock.TicksPerSecond
)

func pawnLabel(p game.PawnView) string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("Colonist %d", p.ID)
}

// barTip is a colonist bar slot's hover line: the name, the age and the
// adulthood title ("Ada, 31, Convoy mechanic"); a legacy colonist has
// neither, so only the name.
func barTip(p game.PawnView) string {
	parts := []string{pawnLabel(p)}
	if p.AgeYears > 0 {
		parts = append(parts, strconv.Itoa(p.AgeYears))
	}
	if p.Title != "" && p.Title != "—" {
		parts = append(parts, p.Title)
	}
	return strings.Join(parts, ", ")
}

// barSlotID is the id of a colonist's bar slot.
func barSlotID(id entity.EntityID) string { return "bar.pawn." + strconv.FormatUint(uint64(id), 10) }

// barClick is the last click on a colonist's bar slot.
type barClick struct {
	id entity.EntityID
	at uint64 // App.updates of the click; 0: none
}

// barDoubleClick records a bar click on id and reports it is the second
// of a double click (same colonist within doubleClickFrames, the map's
// window). A double click is spent: the next click starts a new pair.
func (a *App) barDoubleClick(id entity.EntityID) bool {
	last := a.barClk
	if last.at > 0 && last.id == id && a.updates-last.at <= doubleClickFrames {
		a.barClk = barClick{}
		return true
	}
	a.barClk = barClick{id: id, at: a.updates}
	return false
}

// barState is this region's presentation state (App.hst).
type barState struct {
	// idle is, per colonist, the tick it was first seen idle (kept while
	// it stays idle): the "idle" mark waits barIdleHold ticks.
	idle map[entity.EntityID]clock.Tick
}

// ---- statuses --------------------------------------------------------------

// barStatus is one mark on a colonist's card.
type barStatus struct {
	key   string // the ui_icon is "status_"+key (iconKey overrides)
	icon  string
	label string // the tooltip word
	tone  color.RGBA
}

func (s barStatus) iconKey() string {
	if s.icon != "" {
		return s.icon
	}
	return "status_" + s.key
}

// The marks, most urgent first: a card shows as many as fit, in this order.
var (
	stMental   = barStatus{key: "mental", label: "mental break", tone: uiDanger}
	stDowned   = barStatus{key: "downed", label: "downed", tone: uiDanger}
	stBurning  = barStatus{key: "burning", label: "on fire", tone: uiWarning}
	stFighting = barStatus{key: "fighting", label: "fighting", tone: uiWarning}
	stFleeing  = barStatus{key: "fleeing", label: "fleeing", tone: uiWarning}
	stDrafted  = barStatus{key: "drafted", icon: "draft", label: "drafted", tone: uiAccent}
	stSleeping = barStatus{key: "sleeping", label: "sleeping", tone: uiLetterInfo}
	stIdle     = barStatus{key: "idle", label: "idle", tone: uiTextDim}
)

// barStatuses are the marks for p, most urgent first. onFire: standing in
// a fire (PawnView has no burning flag); idle: idle for the whole hold.
func barStatuses(p game.PawnView, onFire, idle bool) []barStatus {
	var out []barStatus
	downed := p.Status == "downed"
	if p.MentalBreak != "" {
		out = append(out, stMental)
	}
	if downed {
		out = append(out, stDowned)
	}
	if onFire {
		out = append(out, stBurning)
	}
	if p.Action == game.ActionMelee || p.Action == game.ActionAim {
		out = append(out, stFighting)
	}
	if p.Fleeing && !p.Hostile {
		out = append(out, stFleeing)
	}
	if p.Drafted {
		out = append(out, stDrafted)
	}
	if p.Lying && p.Status == "alive" { // Lying is "downed or asleep"
		out = append(out, stSleeping)
	}
	if idle && !downed {
		out = append(out, stIdle)
	}
	return out
}

// barIdleNow is p standing idle with nothing to do during work time.
func barIdleNow(p game.PawnView) bool {
	return p.Status == "alive" && !p.Lying && p.CarriedBy == 0 && !p.Drafted &&
		p.Job == "idle" && (p.Activity == schedule.Work || p.Activity == schedule.Anything)
}

// updateBarIdle starts and ends each colonist's idle spell (once an Update).
func (a *App) updateBarIdle() {
	st := &a.hst.bar
	if !a.barVisible() {
		st.idle = nil
		return
	}
	tick := a.renderer.game.Tick()
	if st.idle == nil || len(st.idle) > 512 {
		st.idle = map[entity.EntityID]clock.Tick{}
	}
	for _, p := range a.renderer.snapshot() {
		since, was := st.idle[p.ID]
		switch {
		case !barIdleNow(p):
			delete(st.idle, p.ID)
		case !was || since > tick: // a spell from before a new game is no spell
			st.idle[p.ID] = tick
		}
	}
}

// onFire reports a fire burns on the colonist's tile.
func onFire(fires []game.FireView, p game.PawnView) bool {
	for _, f := range fires {
		if f.X == p.X && f.Y == p.Y {
			return true
		}
	}
	return false
}

// barStatusesOf are p's marks now.
func (a *App) barStatusesOf(p game.PawnView) []barStatus {
	return a.barStatusesWith(p, a.renderer.world().Fires)
}

func (a *App) barStatusesWith(p game.PawnView, fires []game.FireView) []barStatus {
	since, idling := a.hst.bar.idle[p.ID]
	held := idling && a.renderer.game.Tick() >= since+barIdleHold
	return barStatuses(p, onFire(fires, p), held)
}

// barSlotTip is the hover line of p's slot: barTip, then the marks.
func barSlotTip(p game.PawnView, sts []barStatus) string {
	tip := barTip(p)
	if len(sts) == 0 {
		return tip
	}
	words := make([]string, len(sts))
	for i, s := range sts {
		words[i] = s.label
	}
	return tip + " (" + strings.Join(words, ", ") + ")"
}

// ---- layout ----------------------------------------------------------------

// barVisible reports the bar is on: a live colony and the Options switch.
func (a *App) barVisible() bool {
	return a.renderer != nil && a.settings.Gameplay.ShowColonistBar
}

// barCount is the number of colonist bar entries the layout reserves.
// It feeds the layout, so it never calls a.hud().
func (a *App) barCount() int {
	if !a.barVisible() {
		return 0
	}
	n := 0
	for _, p := range a.renderer.snapshot() {
		if !p.Prisoner {
			n++
		}
	}
	return n
}

// barShown is how many of the bar's slots hold a colonist (the last of an
// overflowing bar is the "+N" slot).
func barShown(l hudLayout) int {
	if l.BarMore > 0 {
		return max(0, len(l.BarSlots)-1)
	}
	return len(l.BarSlots)
}

// barMoreSlot is the "+N" slot, when the bar overflows.
func barMoreSlot(l hudLayout) (rect, bool) {
	if l.BarMore == 0 || len(l.BarSlots) == 0 {
		return rect{}, false
	}
	return l.BarSlots[len(l.BarSlots)-1], true
}

// barCard is where the parts of a colonist's card go inside its slot.
type barCard struct {
	slot, art, name rect
	icon            float32
}

// barCardOf lays a card out inside slot at bar scale s: the head and
// shoulders on top, the name strip under them at full size only.
func barCardOf(slot rect, s float32) barCard {
	inner := slot.inset(barPad * s)
	c := barCard{slot: slot, art: inner, icon: max(barIconMin, barIconSize*s)}
	if s >= barNameMin {
		c.art, c.name = inner.splitV(-min(lineStep(tsSmall), inner.H))
	}
	return c
}

// colonistBarButtons are the bar's slots, one button per shown colonist.
func (a *App) colonistBarButtons(sw, sh int) []button {
	if !a.barVisible() {
		return nil
	}
	l := a.hud()
	pawns := a.barPawns()
	n := min(barShown(l), len(pawns))
	fires := a.renderer.world().Fires
	out := make([]button, 0, n)
	for i, p := range pawns[:n] {
		slot := l.BarSlots[i]
		out = append(out, button{
			id:       barSlotID(p.ID),
			tip:      barSlotTip(p, a.barStatusesWith(p, fires)),
			x:        slot.X,
			y:        slot.Y,
			w:        slot.W,
			h:        slot.H,
			selected: a.barSelected(p.ID),
		})
	}
	return out
}

// barSelected reports the colonist is selected, alone or in a group.
func (a *App) barSelected(id entity.EntityID) bool {
	return a.renderer.HasSelect() && a.renderer.Selected() == id || a.inGroup(selPawn, id)
}

// colonistBarUI declares the "+N" slot (a hit area with a tip) and ticks
// the idle spells (once an Update).
func (a *App) colonistBarUI(sw, sh int) {
	a.updateBarIdle()
	if !a.barVisible() {
		return
	}
	l := a.hud()
	slot, ok := barMoreSlot(l)
	if !ok {
		return
	}
	a.uiSt().add(uiWidget{
		id: barMoreID, label: fmt.Sprintf("+%d", l.BarMore), r: slot,
		tip:   fmt.Sprintf("%d more colonists\nClick to select the next one that has no slot.", l.BarMore),
		click: a.cycleHiddenColonist,
	})
}

// cycleHiddenColonist selects the first colonist without a bar slot, or the
// one after the selected hidden colonist (wrapping).
func (a *App) cycleHiddenColonist() {
	if a.renderer == nil {
		return
	}
	pawns := a.barPawns()
	shown := min(barShown(a.hud()), len(pawns))
	hidden := pawns[shown:]
	if len(hidden) == 0 {
		return
	}
	next := 0
	if a.renderer.HasSelect() {
		for i, p := range hidden {
			if p.ID == a.renderer.Selected() {
				next = (i + 1) % len(hidden)
			}
		}
	}
	a.selectCand(selCand{selPawn, hidden[next].ID})
}

// barPawns are the colonists the colonist bar shows: everyone but
// prisoners and visitors (traders, guests), who stay selectable on the map.
func (a *App) barPawns() []game.PawnView {
	pawns := a.renderer.snapshot()
	for i, p := range pawns {
		if p.Prisoner || p.Visitor {
			out := append(make([]game.PawnView, 0, len(pawns)), pawns[:i]...)
			for _, q := range pawns[i+1:] {
				if !q.Prisoner && !q.Visitor {
					out = append(out, q)
				}
			}
			return out
		}
	}
	return pawns
}

// ---- drawing ---------------------------------------------------------------

// drawColonistBar draws the colonist bar: a card per slot.
func (a *App) drawColonistBar(screen *ebiten.Image, sw, sh int) {
	if !a.barVisible() {
		return
	}
	a.drawColonistPortraits(screen, sh)
}

// moodToken is the palette colour of a mood percentage (the thresholds the
// inspector's mood bar uses).
func moodToken(pct int) color.RGBA {
	switch {
	case pct < 30:
		return uiMoodBad
	case pct < 55:
		return uiMoodMid
	}
	return uiMoodGood
}

// drawColonistPortraits draws every card of the bar, and the "+N" slot.
func (a *App) drawColonistPortraits(screen *ebiten.Image, sh int) {
	if !a.barVisible() {
		return
	}
	l := a.hud()
	pawns := a.barPawns()
	n := min(barShown(l), len(pawns))
	mx, my := a.currentCursor()
	fires := a.renderer.world().Fires
	for i, p := range pawns[:n] {
		slot := l.BarSlots[i]
		a.drawBarCard(screen, barCardOf(slot, l.BarScale), p, a.barSelected(p.ID), slot.hit(mx, my), a.barStatusesWith(p, fires))
	}
	if slot, ok := barMoreSlot(l); ok {
		drawBarMore(screen, slot, l.BarMore, slot.hit(mx, my))
	}
}

// drawBarCard draws one colonist's card.
func (a *App) drawBarCard(screen *ebiten.Image, c barCard, p game.PawnView, selected, hover bool, sts []barStatus) {
	slot := c.slot
	uiFillRect(screen, slot.X, slot.Y, slot.W, slot.H, uiPanel, false)
	// the mood backing: a fill from the floor of the card up, as tall as the mood
	if mood := max(0, min(p.Mood, 100)); mood > 0 {
		fh := slot.H * float32(mood) / 100
		uiFillRect(screen, slot.X, slot.bottom()-fh, slot.W, fh, fade(moodToken(mood), 0.55), false)
	}
	switch {
	case selected:
		uiFillRect(screen, slot.X, slot.Y, slot.W, slot.H, fade(uiBtnSelected, 0.4), false)
	case hover:
		uiFillRect(screen, slot.X, slot.Y, slot.W, slot.H, fade(uiBtnHover, 0.5), false)
	}
	a.drawPawnBust(screen, p, float64(c.art.X), float64(c.art.Y), float64(c.art.W), float64(c.art.H))
	a.drawBarStatuses(screen, c, sts)
	if !c.name.empty() {
		uiFillRect(screen, slot.X, c.name.Y, slot.W, slot.bottom()-c.name.Y, fade(uiSpace, 0.5), false) // a quiet plate: the name reads over any mood
		st := withColor(tsSmall, uiText)
		st.Align = alignCenter
		drawText(screen, ellipsize(pawnLabel(p), tsSmall, float64(c.name.W)), float64(c.name.X+c.name.W/2), float64(c.name.Y), st)
	}
	if selected { // unmistakable: a 2 px accent frame inside the card
		uiStrokeRect(screen, slot.X+1, slot.Y+1, slot.W-2, slot.H-2, 2, uiAccent, false)
		return
	}
	uiStrokeRect(screen, slot.X, slot.Y, slot.W, slot.H, 1, uiPanelBorder, false)
}

// drawBarStatuses stacks the card's marks down its right edge, as many as
// fit (the most urgent first).
func (a *App) drawBarStatuses(screen *ebiten.Image, c barCard, sts []barStatus) {
	size := c.icon
	fit := max(1, int((c.art.H+1)/(size+1)))
	x := c.art.right() - size
	y := c.art.Y
	for i, s := range sts {
		if i == fit {
			break
		}
		if img := a.icon(s.iconKey()); img != nil {
			uiFillCircle(screen, x+size/2, y+size/2, size/2, fade(uiSpace, 0.6), false)
			drawIcon(screen, img, float64(x), float64(y), float64(size))
		} else { // the art is missing (or not drawn yet): a pip keeps the status readable
			uiFillCircle(screen, x+size/2, y+size/2, size/2-1, s.tone, false)
		}
		y += size + 1
	}
}

// drawBarMore draws the "+N" slot of an overflowing bar.
func drawBarMore(screen *ebiten.Image, slot rect, n int, hover bool) {
	uiFillRect(screen, slot.X, slot.Y, slot.W, slot.H, uiPanel, false)
	if hover {
		uiFillRect(screen, slot.X, slot.Y, slot.W, slot.H, fade(uiBtnHover, 0.5), false)
	}
	uiStrokeRect(screen, slot.X, slot.Y, slot.W, slot.H, 1, uiPanelBorder, false)
	drawCentered(screen, "+"+strconv.Itoa(n), float64(slot.X+slot.W/2), float64(slot.Y+slot.H/2), withColor(tsLabel, uiHeading))
}

// drawPawnPortrait draws the whole colonist, fitted into the logical box
// (x, y, w, h): the inspector's portrait.
func (a *App) drawPawnPortrait(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) {
	if a.renderer.art != nil {
		a.renderer.drawPawnPortraitArt(screen, p, x, y, w, h)
	}
}

// drawPawnBust draws the colonist's head and shoulders fitted into the
// logical box (x, y, w, h): the colonist bar's portrait.
func (a *App) drawPawnBust(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) {
	if a.renderer.art != nil {
		a.renderer.drawPawnBustArt(screen, p, x, y, w, h)
	}
}
