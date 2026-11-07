package hud

import (
	"fmt"
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/mapnav"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/config"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The colonist bar (PLAY-018: HUD-TOP owns it): one compact card per
// colonist, top-centre, in the layout's BarSlots. A card is a mood backing
// (a fill that rises with the mood), the colonist's head and shoulders,
// status marks and, at full size, the name. A selected colonist's card has
// an accent outline; the last slot of an overflowing bar reads "+N".

const (
	BarMoreID = "bar.more" // the "+N" slot (a kit widget: ../hud.go routes no click for it)

	barPad        = 2  // card inset, at bar scale 1
	BarIconSize   = 14 // a status mark, at bar scale 1
	BarIconMin    = 10 // a status mark never gets smaller than this
	barBracketOut = 2  // the selection's corner brackets lie this far outside the portrait box (UIS-027)
	barBracketLen = 7  // each bracket arm's length, at any slot size
	// barMsgVeil is the opacity of the veil over a slot a message covers (UIS-029).
	barMsgVeil = 0.6

	// barIdleHold is how long a colonist has to stand idle before the
	// "idle" mark shows, so the gap between two jobs does not flash it.
	BarIdleHold = 2 * clock.TicksPerSecond
)

// PawnLabel is a colonist's display name ("Colonist 7" when it has none).
func PawnLabel(p game.PawnView) string { return PawnLabelIn(en, p) }

// PawnLabelIn is PawnLabel in tr's language.
func PawnLabelIn(tr lang.Tr, p game.PawnView) string {
	if p.Name != "" {
		return p.Name
	}
	return lang.Tf(tr, "Colonist %d", p.ID)
}

// BarTip is a colonist bar slot's hover line: the name, the age and the
// adulthood title ("Ada, 31, Convoy mechanic"); a legacy colonist has
// neither, so only the name.
func BarTip(p game.PawnView) string { return BarTipIn(en, p) }

// BarTipIn is BarTip in tr's language.
func BarTipIn(tr lang.Tr, p game.PawnView) string {
	parts := []string{PawnLabelIn(tr, p)}
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
	// mp is the bar's entries with several maps open (MAP-1), built once an
	// Update; groups are the non-empty maps, parallel to the layout's frames.
	mp barMaps
	// ord is the sorted copy of the colonists (bar order), reused; drag is
	// the right-button gesture in flight (bar_drag.go).
	ord  []game.PawnView
	drag barDrag
}

// barMaps is the multi-map bar's per-Update memo.
type barMaps struct {
	at     uint64 // Env.Updates it was built at; 0: none
	pawns  []game.PawnView
	groups []mapnav.Group
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
	stMental   = BarStatus{Key: "mental", Label: en.T("mental break"), tone: uikit.Danger}
	stDowned   = BarStatus{Key: "downed", Label: en.T("downed"), tone: uikit.Danger}
	stBurning  = BarStatus{Key: "burning", Label: en.T("on fire"), tone: uikit.Warning}
	stFighting = BarStatus{Key: "fighting", Label: en.T("fighting"), tone: uikit.Warning}
	stFleeing  = BarStatus{Key: "fleeing", Label: en.T("fleeing"), tone: uikit.Warning}
	stDrafted  = BarStatus{Key: "drafted", icon: "draft", Label: en.T("drafted"), tone: uikit.Accent}
	stSleeping = BarStatus{Key: "sleeping", Label: en.T("sleeping"), tone: uikit.LetterInfo}
	stIdle     = BarStatus{Key: "idle", Label: en.T("idle"), tone: uikit.TextDim}
	// StatusCaravan marks a colonist travelling with a caravan. Caravan
	// members are not in Colonists() yet, so nothing raises it; it shows the
	// generic caravan icon until its own art lands.
	StatusCaravan = BarStatus{Key: "caravan", icon: "caravan", Label: en.T("in a caravan"), tone: uikit.LetterInfo}
)

// BarStatuses are the marks for p, most urgent first. onFire: standing in
// a fire (PawnView has no burning flag); idle: idle for the whole hold.
func BarStatuses(p game.PawnView, onFire, idle bool) []BarStatus {
	return BarStatusesIn(en, p, onFire, idle)
}

// BarStatusesIn is BarStatuses with the labels in tr's language.
func BarStatusesIn(tr lang.Tr, p game.PawnView, onFire, idle bool) []BarStatus {
	var out []BarStatus
	add := func(s BarStatus) {
		s.Label = tr.T(s.Label)
		out = append(out, s)
	}
	downed := p.Status == "downed"
	if p.MentalBreak != "" {
		m := stMental
		m.Label = lang.Tf(tr, "mental break: %s", p.MentalBreak)
		out = append(out, m)
	}
	if downed {
		add(stDowned)
	}
	if onFire {
		add(stBurning)
	}
	if p.Action == game.ActionMelee || p.Action == game.ActionAim {
		add(stFighting)
	}
	if p.Fleeing && !p.Hostile {
		add(stFleeing)
	}
	if p.Drafted {
		add(stDrafted)
	}
	if p.Lying && p.Status == "alive" { // Lying is "downed or asleep"
		add(stSleeping)
	}
	if idle && !downed {
		add(stIdle)
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
	for _, p := range c.Pawns() {
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
	if g := c.sh.Env().Game; g != nil && p.Map != g.ViewedMap() {
		fires = nil // the fires are the viewed map's
	}
	return BarStatusesIn(c.sh, p, onFire(fires, p), held)
}

// barSlotTip is the hover line of p's slot: barTip, then the marks.
func barSlotTip(tr lang.Tr, p game.PawnView, sts []BarStatus) string {
	tip := BarTipIn(tr, p)
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
	return len(c.Pawns())
}

// GroupSizes are the slots per map group for the layout (MAP-1), zero after
// the last; all zero with one map open, so a single-map bar has no frames.
func (c ColonistBar) GroupSizes() (out [hudlayout.MaxBarGroups]int) {
	if !c.Visible() {
		return out
	}
	for i, g := range c.groups() {
		out[i] = len(g.Pawns)
	}
	return out
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

// barCard is where the parts of a colonist's card go: the portrait inside
// its slot, the name plate under it (UIS-020), outside the portrait box.
type barCard struct {
	slot, art, name gfx.Rect
	icon            float32
}

// barCardOf lays a card out at bar scale s: the head and shoulders fill the
// slot, the name plate (BarNameH tall, as wide as the slot) hangs under it
// at the same size at every scale (UIS-026).
func barCardOf(slot gfx.Rect, s float32) barCard {
	return barCard{slot: slot, art: slot.Inset(barPad * s), icon: max(BarIconMin, BarIconSize*s),
		name: gfx.Rect{X: slot.X, Y: slot.Bottom(), W: slot.W, H: hudlayout.BarNameH}}
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
			Tip:      barSlotTip(c.sh, p, c.statusesWith(p, fires)),
			X:        slot.X,
			Y:        slot.Y,
			W:        slot.W,
			H:        slot.H,
			Selected: c.Picked(p.ID),
		})
	}
	// the map frames come after the slots: hit-testing takes the first match
	gs := c.groups()
	for i, g := range l.BarGroups {
		if i >= len(gs) {
			break
		}
		out = append(out, uikit.Button{
			ID: BarMapID(gs[i].Map), Tip: groupTip(c.sh, gs[i]),
			X: g.Frame.X, Y: g.Frame.Y, W: g.Frame.W, H: g.Frame.H, Selected: gs[i].Current,
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
		Tip:   c.sh.Tf("%d more colonists\nClick to select the next one that has no slot.", l.BarMore),
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
// With several maps open, every map's colonists in group order.
func (c ColonistBar) Pawns() []game.PawnView {
	if pawns, _ := c.multi(); pawns != nil {
		return pawns
	}
	pawns := c.sh.Pawns()
	for i, p := range pawns {
		if p.Prisoner || p.Visitor {
			out := append(make([]game.PawnView, 0, len(pawns)), pawns[:i]...)
			for _, q := range pawns[i+1:] {
				if !q.Prisoner && !q.Visitor {
					out = append(out, q)
				}
			}
			slices.SortStableFunc(out, byBarOrder)
			return out
		}
	}
	return c.barSorted(pawns)
}

// groups are the map groups with colonists on the bar; nil with one map
// open (the plain bar).
func (c ColonistBar) groups() []mapnav.Group {
	_, gs := c.multi()
	return gs
}

// multi builds, once an Update, the bar of several open maps: every map's
// colonists (no prisoners, visitors), grouped in map order, the empty maps
// left out and the maps beyond hudlayout.MaxBarGroups too.
func (c ColonistBar) multi() ([]game.PawnView, []mapnav.Group) {
	env := c.sh.Env()
	if env.Game == nil {
		return nil, nil
	}
	if m := &c.bar.mp; env.Updates != 0 && m.at == env.Updates {
		return m.pawns, m.groups
	}
	maps := env.Game.Maps()
	var pawns []game.PawnView
	var groups []mapnav.Group
	if len(maps) > 1 {
		for _, g := range mapnav.Groups(maps, env.Game.Colonists(), env.Game.ViewedMap()) {
			ps := g.Pawns[:0:0]
			for _, p := range g.Pawns {
				if !p.Prisoner && !p.Visitor {
					ps = append(ps, p)
				}
			}
			if len(ps) == 0 || len(groups) == hudlayout.MaxBarGroups {
				continue
			}
			slices.SortStableFunc(ps, byBarOrder)
			g.Pawns = ps
			groups = append(groups, g)
			pawns = append(pawns, ps...)
		}
	}
	c.bar.mp = barMaps{at: env.Updates, pawns: pawns, groups: groups}
	return pawns, groups
}

// BarCrossMapHint is why Shift-click refuses a colonist on another map.
const BarCrossMapHint = "Switch to that map first: a selection holds colonists of one map."

// ClickAway handles a click on a colonist who stands on another map than the
// one on screen (MAP-1): it jumps there, selecting and centring the colonist,
// or with shift refuses with a toast. False for a colonist on this map, which
// the caller handles.
func (c ColonistBar) ClickAway(id entity.EntityID, shift bool) bool {
	g := c.sh.Env().Game
	if g == nil {
		return false
	}
	for _, p := range c.Pawns() {
		if p.ID != id || p.Map == g.ViewedMap() {
			continue
		}
		if shift {
			c.Toast(BarCrossMapHint)
		} else {
			c.sh.Jump(ports.Spot{Map: p.Map, X: p.X, Y: p.Y, Focus: id})
		}
		return true
	}
	return false
}

// BarMapPrefix starts the id of a map group's frame button; the root routes
// it to a ports.SwitchMap intent.
const BarMapPrefix = "bar.map."

// BarMapID is the id of a map group's frame button.
func BarMapID(m game.MapID) string { return BarMapPrefix + strconv.FormatUint(uint64(m), 10) }

// BarMapOf is the map a frame button id names.
func BarMapOf(id string) (game.MapID, bool) {
	n, err := strconv.ParseUint(strings.TrimPrefix(id, BarMapPrefix), 10, 32)
	return game.MapID(n), err == nil && strings.HasPrefix(id, BarMapPrefix)
}

// groupTip is a map frame's hover card.
func groupTip(tr lang.Tr, g mapnav.Group) string {
	tip := g.Label + tr.T("\nClick to switch to this map.")
	if g.Threat {
		tip += tr.T("\nHostiles are on it.")
	}
	return tip
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
	c.drawGroups(screen, l, mx, my)
	msgs := Right(c).MessageRows(l)
	for i, p := range pawns[:n] {
		slot := l.BarSlots[i]
		c.drawCard(screen, barCardOf(slot, l.BarScale), p, c.Picked(p.ID), slot.Hit(mx, my), c.statusesWith(p, fires))
		for _, m := range msgs {
			if !m.R.Intersect(slot).Empty() {
				gfx.FillRect(screen, slot.X, slot.Y, slot.W, slot.H, creature.Fade(uikit.Space, barMsgVeil), false) // UIS-029: the slot fades under a message
				break
			}
		}
	}
	c.drawDrag(screen, pawns[:n])
	if slot, ok := barMoreSlot(l); ok {
		drawBarMore(screen, slot, l.BarMore, slot.Hit(mx, my))
	}
}

// drawCard draws one colonist's card.
func (c ColonistBar) drawCard(screen *ebiten.Image, card barCard, p game.PawnView, selected, hover bool, sts []BarStatus) {
	slot := card.slot
	gfx.FillRect(screen, slot.X, slot.Y, slot.W, slot.H, uikit.Panel, false)
	// the mood backing: a fill from the floor of the card up, as tall as the mood
	if mood := max(0, min(p.Mood, 100)); mood > 0 && c.sh.Settings().Interface.BarMood {
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
	c.drawWeapon(screen, card, p)
	c.drawName(screen, card, p)
	if selected { // UIS-027: four corner brackets just outside the portrait box
		drawBrackets(screen, slot)
		return
	}
	gfx.StrokeRect(screen, slot.X, slot.Y, slot.W, slot.H, 1, uikit.PanelBorder, false)
}

// drawName draws the name plate under the portrait: Small at every slot
// size, ellipsized to the slot's width, underlined while drafted (UIS-027).
func (c ColonistBar) drawName(screen *ebiten.Image, card barCard, p game.PawnView) {
	n := card.name
	st := gfx.WithColor(gfx.Small, uikit.Text)
	st.Align = gfx.AlignCenter
	label := gfx.Ellipsize(PawnLabelIn(c.sh, p), gfx.Small, float64(n.W))
	_, th := gfx.Measure(label, gfx.Small)
	ty := float64(n.Bottom()) - th // the line box ends at the plate's foot, inside the bar's box
	gfx.Text(screen, label, float64(n.X+n.W/2), ty, st)
	if p.Drafted {
		w, _ := gfx.Measure(label, gfx.Small)
		y := n.Bottom() - 1
		gfx.Line(screen, n.X+n.W/2-float32(w)/2, y, n.X+n.W/2+float32(w)/2, y, 1, uikit.Text, false)
	}
}

// drawBrackets marks the selected card with four white corner brackets
// barBracketOut outside the portrait box.
func drawBrackets(screen *ebiten.Image, slot gfx.Rect) {
	const o, l, w = barBracketOut, barBracketLen, 2
	x0, y0, x1, y1 := slot.X-o, slot.Y-o, slot.Right()+o, slot.Bottom()+o
	for _, c := range [...]struct{ x, y, dx, dy float32 }{{x0, y0, 1, 1}, {x1, y0, -1, 1}, {x0, y1, 1, -1}, {x1, y1, -1, -1}} {
		gfx.Line(screen, c.x, c.y+c.dy*w/2, c.x+c.dx*l, c.y+c.dy*w/2, w, uikit.Heading, false)
		gfx.Line(screen, c.x+c.dx*w/2, c.y, c.x+c.dx*w/2, c.y+c.dy*l, w, uikit.Heading, false)
	}
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

// drawGroups draws each map's frame and label chip behind its cards: the
// map on screen has an accent frame, a map with hostiles a red dot.
func (c ColonistBar) drawGroups(screen *ebiten.Image, l hudlayout.Layout, mx, my int) {
	gs := c.groups()
	for i, g := range l.BarGroups {
		if i >= len(gs) {
			return
		}
		mg := gs[i]
		gfx.FillRect(screen, g.Frame.X, g.Frame.Y, g.Frame.W, g.Frame.H, creature.Fade(uikit.Space, 0.35), false)
		chip := creature.Fade(uikit.Panel, 0.9)
		if g.Frame.Hit(mx, my) {
			chip = creature.Fade(uikit.BtnHover, 0.7)
		}
		gfx.FillRect(screen, g.Chip.X, g.Chip.Y, g.Chip.W, g.Chip.H, chip, false)
		textW := g.Chip.W - 8
		if mg.Threat {
			textW -= g.Chip.H
		}
		st := gfx.WithColor(gfx.Small, uikit.Text)
		gfx.Text(screen, gfx.Ellipsize(mg.Label, gfx.Small, float64(textW)), float64(g.Chip.X+4), gfx.CenterTop(float64(g.Chip.Y), float64(g.Chip.H), gfx.Small), st)
		if mg.Threat {
			r := g.Chip.H / 4
			gfx.FillCircle(screen, g.Chip.Right()-r-4, g.Chip.Y+g.Chip.H/2, r, uikit.Danger, false)
		}
		if mg.Current {
			gfx.StrokeRect(screen, g.Frame.X, g.Frame.Y, g.Frame.W, g.Frame.H, 2, uikit.Accent, false)
		} else {
			gfx.StrokeRect(screen, g.Frame.X, g.Frame.Y, g.Frame.W, g.Frame.H, 1, uikit.PanelBorder, false)
		}
	}
}

// showWeapon reports the weapon icon is due under p's portrait (OPT-011):
// never, only while drafted, or always; bare fists have no icon.
func showWeapon(mode string, p game.PawnView) bool {
	if _, ok := creature.HeldWeaponKey(p.Weapon); !ok {
		return false
	}
	switch mode {
	case config.BarWeaponsAlways:
		return true
	case config.BarWeaponsDrafted:
		return p.Drafted
	}
	return false
}

// weaponRect is the weapon icon's box: the bottom-left corner of the portrait,
// a status mark's size but never more than half the portrait, so it fits
// inside the slot at every bar scale.
func weaponRect(card barCard) gfx.Rect {
	sz := min(card.icon, card.art.W/2, card.art.H/2)
	return gfx.Rect{X: card.art.X, Y: card.art.Bottom() - sz, W: sz, H: sz}
}

// drawWeapon draws the wielded weapon's sprite under the portrait.
func (c ColonistBar) drawWeapon(screen *ebiten.Image, card barCard, p game.PawnView) {
	if !showWeapon(c.sh.Settings().Interface.BarWeapons, p) {
		return
	}
	cat := c.sh.Env().Art
	if cat == nil {
		return
	}
	key, _ := creature.HeldWeaponKey(p.Weapon)
	img := cat.Image(key, color.NRGBA{})
	if img == nil {
		return
	}
	r := weaponRect(card)
	gfx.FillRect(screen, r.X, r.Y, r.W, r.H, creature.Fade(uikit.Space, 0.5), false)
	gfx.Icon(screen, img, float64(r.X), float64(r.Y), float64(r.W))
}
