package hud

import (
	"fmt"
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strconv"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The colonist bar (PLAY-018: HUD-TOP owns it): one compact card per
// colonist, top-centre, in the layout's BarSlots. A card is a mood backing
// (a fill that rises with the mood), the colonist's head and shoulders,
// status marks and, at full size, the name. A selected colonist's card has
// an accent outline; the last slot of an overflowing bar reads "+N".

const (
	BarMoreID = "bar.more" // the "+N" slot (a kit widget: ../hud.go routes no click for it)

	barPad      = 2  // card inset, at bar scale 1
	BarIconSize = 14 // a status mark, at bar scale 1
	BarIconMin  = 10 // a status mark never gets smaller than this
	barNameMin  = 1  // names are drawn at this bar scale and above; smaller slots use the tooltip

	// barIdleHold is how long a colonist has to stand idle before the
	// "idle" mark shows, so the gap between two jobs does not flash it.
	BarIdleHold = 2 * clock.TicksPerSecond
)

// PawnLabel is a colonist's display name ("Colonist 7" when it has none).
func PawnLabel(p game.PawnView) string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("Colonist %d", p.ID)
}

// BarTip is a colonist bar slot's hover line: the name, the age and the
// adulthood title ("Ada, 31, Convoy mechanic"); a legacy colonist has
// neither, so only the name.
func BarTip(p game.PawnView) string {
	parts := []string{PawnLabel(p)}
	if p.AgeYears > 0 {
		parts = append(parts, strconv.Itoa(p.AgeYears))
	}
	if p.Title != "" && p.Title != "—" {
		parts = append(parts, p.Title)
	}
	return strings.Join(parts, ", ")
}

// BarSlotID is the id of a colonist's bar slot.
func BarSlotID(id entity.EntityID) string {
	return ids.PrefixBarPawn + strconv.FormatUint(uint64(id), 10)
}

// barClick is the last click on a colonist's bar slot.
type barClick struct {
	id entity.EntityID
	at uint64 // Env.Updates of the click; 0: none
}

// DoubleClick records a bar click on id and reports it is the second
// of a double click (same colonist within doubleClickFrames, the map's
// window). A double click is spent: the next click starts a new pair.
func (c ColonistBar) DoubleClick(id entity.EntityID) bool {
	last := c.barClk
	if last.at > 0 && last.id == id && c.sh.Env().Updates-last.at <= selection.DoubleClickFrames {
		c.barClk = barClick{}
		return true
	}
	c.barClk = barClick{id: id, at: c.sh.Env().Updates}
	return false
}

// barState is this region's presentation state (Chrome.bar).
type barState struct {
	// idle is, per colonist, the tick it was first seen idle (kept while
	// it stays idle): the "idle" mark waits barIdleHold ticks.
	idle map[entity.EntityID]clock.Tick
}

// ---- statuses --------------------------------------------------------------

// BarStatus is one mark on a colonist's card.
type BarStatus struct {
	Key   string // the ui_icon is "status_"+Key (iconKey overrides)
	icon  string
	Label string // the tooltip word
	tone  color.RGBA
}

func (s BarStatus) iconKey() string {
	if s.icon != "" {
		return s.icon
	}
	return "status_" + s.Key
}

// The marks, most urgent first: a card shows as many as fit, in this order.
var (
	stMental   = BarStatus{Key: "mental", Label: "mental break", tone: uikit.Danger}
	stDowned   = BarStatus{Key: "downed", Label: "downed", tone: uikit.Danger}
	stBurning  = BarStatus{Key: "burning", Label: "on fire", tone: uikit.Warning}
	stFighting = BarStatus{Key: "fighting", Label: "fighting", tone: uikit.Warning}
	stFleeing  = BarStatus{Key: "fleeing", Label: "fleeing", tone: uikit.Warning}
	stDrafted  = BarStatus{Key: "drafted", icon: "draft", Label: "drafted", tone: uikit.Accent}
	stSleeping = BarStatus{Key: "sleeping", Label: "sleeping", tone: uikit.LetterInfo}
	stIdle     = BarStatus{Key: "idle", Label: "idle", tone: uikit.TextDim}
)

// BarStatuses are the marks for p, most urgent first. onFire: standing in
// a fire (PawnView has no burning flag); idle: idle for the whole hold.
func BarStatuses(p game.PawnView, onFire, idle bool) []BarStatus {
	var out []BarStatus
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

// updateIdle starts and ends each colonist's idle spell (once an Update).
func (c ColonistBar) updateIdle() {
	st := &c.bar
	if !c.Visible() {
		st.idle = nil
		return
	}
	tick := c.sh.Tick()
	if st.idle == nil || len(st.idle) > 512 {
		st.idle = map[entity.EntityID]clock.Tick{}
	}
	for _, p := range c.sh.Pawns() {
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

// StatusesOf are p's marks now.
func (c ColonistBar) StatusesOf(p game.PawnView) []BarStatus {
	return c.statusesWith(p, c.sh.World().Fires)
}

func (c ColonistBar) statusesWith(p game.PawnView, fires []game.FireView) []BarStatus {
	since, idling := c.bar.idle[p.ID]
	held := idling && c.sh.Tick() >= since+BarIdleHold
	return BarStatuses(p, onFire(fires, p), held)
}

// barSlotTip is the hover line of p's slot: barTip, then the marks.
func barSlotTip(p game.PawnView, sts []BarStatus) string {
	tip := BarTip(p)
	if len(sts) == 0 {
		return tip
	}
	words := make([]string, len(sts))
	for i, s := range sts {
		words[i] = s.Label
	}
	return tip + " (" + strings.Join(words, ", ") + ")"
}

// ---- layout ----------------------------------------------------------------

// Visible reports the bar is on: a live colony and the Options switch.
func (c ColonistBar) Visible() bool {
	return c.live() && c.sh.Settings().Gameplay.ShowColonistBar
}

// Count is the number of colonist bar entries the layout reserves.
// It feeds the layout, so it never asks for it.
func (c ColonistBar) Count() int {
	if !c.Visible() {
		return 0
	}
	n := 0
	for _, p := range c.sh.Pawns() {
		if !p.Prisoner && !p.Visitor { // as barPawns: visitors and allies stay off the bar
			n++
		}
	}
	return n
}

// BarShown is how many of the bar's slots hold a colonist (the last of an
// overflowing bar is the "+N" slot).
func BarShown(l hudlayout.Layout) int {
	if l.BarMore > 0 {
		return max(0, len(l.BarSlots)-1)
	}
	return len(l.BarSlots)
}

// barMoreSlot is the "+N" slot, when the bar overflows.
func barMoreSlot(l hudlayout.Layout) (gfx.Rect, bool) {
	if l.BarMore == 0 || len(l.BarSlots) == 0 {
		return gfx.Rect{}, false
	}
	return l.BarSlots[len(l.BarSlots)-1], true
}

// barCard is where the parts of a colonist's card go inside its slot.
type barCard struct {
	slot, art, name gfx.Rect
	icon            float32
}

// barCardOf lays a card out inside slot at bar scale s: the head and
// shoulders on top, the name strip under them at full size only.
func barCardOf(slot gfx.Rect, s float32) barCard {
	inner := slot.Inset(barPad * s)
	c := barCard{slot: slot, art: inner, icon: max(BarIconMin, BarIconSize*s)}
	if s >= barNameMin {
		c.art, c.name = inner.SplitV(-min(gfx.LineStep(gfx.Small), inner.H))
	}
	return c
}

// Buttons are the bar's slots, one button per shown colonist.
func (c ColonistBar) Buttons(sw, sh int) []uikit.Button {
	if !c.Visible() {
		return nil
	}
	l := c.sh.Layout()
	pawns := c.Pawns()
	n := min(BarShown(l), len(pawns))
	fires := c.sh.World().Fires
	out := make([]uikit.Button, 0, n)
	for i, p := range pawns[:n] {
		slot := l.BarSlots[i]
		out = append(out, uikit.Button{
			ID:       BarSlotID(p.ID),
			Tip:      barSlotTip(p, c.statusesWith(p, fires)),
			X:        slot.X,
			Y:        slot.Y,
			W:        slot.W,
			H:        slot.H,
			Selected: c.Picked(p.ID),
		})
	}
	return out
}

// Picked reports the colonist is selected, alone or in a group.
func (c ColonistBar) Picked(id entity.EntityID) bool {
	sel, ok := c.sh.Selected()
	return ok && sel == id || c.sh.Picked(id)
}

// Declare declares the "+N" slot (a hit area with a tip) and ticks
// the idle spells (once an Update).
func (c ColonistBar) Declare(sw, sh int) {
	c.updateIdle()
	if !c.Visible() {
		return
	}
	l := c.sh.Layout()
	slot, ok := barMoreSlot(l)
	if !ok {
		return
	}
	c.kit().Add(uikit.Widget{
		ID: BarMoreID, Label: fmt.Sprintf("+%d", l.BarMore), R: slot,
		Tip:   fmt.Sprintf("%d more colonists\nClick to select the next one that has no slot.", l.BarMore),
		Click: c.cycleHidden,
	})
}

// cycleHidden selects the first colonist without a bar slot, or the
// one after the selected hidden colonist (wrapping).
func (c ColonistBar) cycleHidden() {
	if !c.live() {
		return
	}
	pawns := c.Pawns()
	shown := min(BarShown(c.sh.Layout()), len(pawns))
	hidden := pawns[shown:]
	if len(hidden) == 0 {
		return
	}
	next := 0
	if sel, ok := c.sh.Selected(); ok {
		for i, p := range hidden {
			if p.ID == sel {
				next = (i + 1) % len(hidden)
			}
		}
	}
	c.sh.Pick(selection.Cand{Kind: selection.Pawn, ID: hidden[next].ID})
}

// Pawns are the colonists the colonist bar shows: everyone but
// prisoners and visitors (traders, guests), who stay selectable on the map.
func (c ColonistBar) Pawns() []game.PawnView {
	pawns := c.sh.Pawns()
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

// Draw draws the colonist bar: a card per slot.
func (c ColonistBar) Draw(screen *ebiten.Image, sw, sh int) {
	if !c.Visible() {
		return
	}
	c.DrawPortraits(screen, sh)
}

// MoodToken is the palette colour of a mood percentage (the thresholds the
// inspector's mood bar uses).
func MoodToken(pct int) color.RGBA {
	switch {
	case pct < 30:
		return uikit.MoodBad
	case pct < 55:
		return uikit.MoodMid
	}
	return uikit.MoodGood
}

// DrawPortraits draws every card of the bar, and the "+N" slot.
func (c ColonistBar) DrawPortraits(screen *ebiten.Image, sh int) {
	if !c.Visible() {
		return
	}
	l := c.sh.Layout()
	pawns := c.Pawns()
	n := min(BarShown(l), len(pawns))
	mx, my := c.sh.Cursor()
	fires := c.sh.World().Fires
	for i, p := range pawns[:n] {
		slot := l.BarSlots[i]
		c.drawCard(screen, barCardOf(slot, l.BarScale), p, c.Picked(p.ID), slot.Hit(mx, my), c.statusesWith(p, fires))
	}
	if slot, ok := barMoreSlot(l); ok {
		drawBarMore(screen, slot, l.BarMore, slot.Hit(mx, my))
	}
}

// drawCard draws one colonist's card.
func (c ColonistBar) drawCard(screen *ebiten.Image, card barCard, p game.PawnView, selected, hover bool, sts []BarStatus) {
	slot := card.slot
	gfx.FillRect(screen, slot.X, slot.Y, slot.W, slot.H, uikit.Panel, false)
	// the mood backing: a fill from the floor of the card up, as tall as the mood
	if mood := max(0, min(p.Mood, 100)); mood > 0 {
		fh := slot.H * float32(mood) / 100
		gfx.FillRect(screen, slot.X, slot.Bottom()-fh, slot.W, fh, creature.Fade(MoodToken(mood), 0.55), false)
	}
	switch {
	case selected:
		gfx.FillRect(screen, slot.X, slot.Y, slot.W, slot.H, creature.Fade(uikit.BtnSelected, 0.4), false)
	case hover:
		gfx.FillRect(screen, slot.X, slot.Y, slot.W, slot.H, creature.Fade(uikit.BtnHover, 0.5), false)
	}
	c.sh.DrawPawnBust(screen, p, float64(card.art.X), float64(card.art.Y), float64(card.art.W), float64(card.art.H))
	c.drawStatuses(screen, card, sts)
	if !card.name.Empty() {
		gfx.FillRect(screen, slot.X, card.name.Y, slot.W, slot.Bottom()-card.name.Y, creature.Fade(uikit.Space, 0.5), false) // a quiet plate: the name reads over any mood
		st := gfx.WithColor(gfx.Small, uikit.Text)
		st.Align = gfx.AlignCenter
		gfx.Text(screen, gfx.Ellipsize(PawnLabel(p), gfx.Small, float64(card.name.W)), float64(card.name.X+card.name.W/2), float64(card.name.Y), st)
	}
	if selected { // unmistakable: a 2 px accent frame inside the card
		gfx.StrokeRect(screen, slot.X+1, slot.Y+1, slot.W-2, slot.H-2, 2, uikit.Accent, false)
		return
	}
	gfx.StrokeRect(screen, slot.X, slot.Y, slot.W, slot.H, 1, uikit.PanelBorder, false)
}

// drawStatuses stacks the card's marks down its right edge, as many as
// fit (the most urgent first).
func (c ColonistBar) drawStatuses(screen *ebiten.Image, card barCard, sts []BarStatus) {
	size := card.icon
	fit := max(1, int((card.art.H+1)/(size+1)))
	x := card.art.Right() - size
	y := card.art.Y
	for i, s := range sts {
		if i == fit {
			break
		}
		if img := c.icon(s.iconKey()); img != nil {
			gfx.FillCircle(screen, x+size/2, y+size/2, size/2, creature.Fade(uikit.Space, 0.6), false)
			gfx.Icon(screen, img, float64(x), float64(y), float64(size))
		} else { // the art is missing (or not drawn yet): a pip keeps the status readable
			gfx.FillCircle(screen, x+size/2, y+size/2, size/2-1, s.tone, false)
		}
		y += size + 1
	}
}

// drawBarMore draws the "+N" slot of an overflowing bar.
func drawBarMore(screen *ebiten.Image, slot gfx.Rect, n int, hover bool) {
	gfx.FillRect(screen, slot.X, slot.Y, slot.W, slot.H, uikit.Panel, false)
	if hover {
		gfx.FillRect(screen, slot.X, slot.Y, slot.W, slot.H, creature.Fade(uikit.BtnHover, 0.5), false)
	}
	gfx.StrokeRect(screen, slot.X, slot.Y, slot.W, slot.H, 1, uikit.PanelBorder, false)
	uikit.DrawCentered(screen, "+"+strconv.Itoa(n), float64(slot.X+slot.W/2), float64(slot.Y+slot.H/2), gfx.WithColor(gfx.Label, uikit.Heading))
}
