package hud

import (
	"fmt"
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"strconv"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// FmtTemp renders tenths of a degree C as whole degrees of unit u ("-3C").
func FmtTemp(u units.Temp, tenths int) string { return u.Compact(tenths) }

// tileTemp is the air temperature on a tile: enclosed rooms hold the
// indoor temperature, everything else is outdoors.
func tileTemp(w game.WorldView, x, y int) (int, bool) {
	if w.TerrainW > 0 && x >= 0 && y >= 0 && x < w.TerrainW {
		if i := y*w.TerrainW + x; i < len(w.RoomGrid) {
			if r := w.RoomGrid[i]; r >= 0 && r < len(w.Rooms) {
				return w.Rooms[r].Temp, true
			}
		}
	}
	return w.OutdoorTemp, false
}

// freshLabel describes how long a stack keeps where it lies.
func freshLabel(it game.ItemView) string {
	switch {
	case it.Dried:
		return "keeps forever"
	case it.Rotten:
		return "rotten"
	case it.FreshDays < 0:
		return ""
	case it.RotRate == 0:
		return fmt.Sprintf("frozen, %.1f days fresh", it.FreshDays)
	}
	days := it.FreshDays * 1000 / float64(it.RotRate)
	s := fmt.Sprintf("spoils in %.1f days", days)
	if it.RotRate < 1000 {
		s += " (chilled)"
	}
	return s
}

// spoilSoonDays raises the "Food spoiling" alert below this many days.
const spoilSoonDays = 1.0

// spoilingAlert flags ground food that will rot within spoilSoonDays.
func spoilingAlert(w game.WorldView) (Alert, bool) {
	a := Alert{Key: "spoiling"}
	qty := 0
	for _, it := range w.Items {
		if it.Carried || it.Rotten || it.FreshDays < 0 || it.RotRate == 0 {
			continue
		}
		if strings.HasSuffix(it.Kind, "corpse") {
			continue
		}
		if it.FreshDays*1000/float64(it.RotRate) < spoilSoonDays {
			qty += it.Qty
			a.Targets = append(a.Targets, [2]int{it.X, it.Y})
		}
	}
	if qty == 0 {
		return a, false
	}
	a.Label = fmt.Sprintf("Food spoiling soon (%d)", qty)
	return a, true
}

// Messages (PLAY-018): recent event notices stacked in the layout's
// Messages rect, right-aligned against the right column under the colonist
// bar. Each shows for messageFrames presentation frames after it first
// appears and fades out over the last MessageFadeFrames.
const (
	MessageFrames     = 600
	MessageFadeFrames = 60
	messageShown      = hudlayout.MaxMsgs
	messageEdgeW      = 3 // the kind-coloured edge on a row's left
	messagePadX       = 8
)

func messageKey(m game.MessageView) string { return fmt.Sprintf("%d|%s", m.Tick, m.Text) }

// liveMessage is a notice still on screen and how many presentation
// frames it has been there.
type liveMessage struct {
	m   game.MessageView
	age uint64
}

// liveMessageList are the notices still on screen with their ages, oldest
// first. The first call that sees a notice starts its clock.
func (c Right) liveMessageList() []liveMessage {
	if !c.live() {
		return nil
	}
	if c.MsgSeen == nil {
		c.MsgSeen = map[string]uint64{}
	}
	now := c.sh.Present()
	msgs := c.sh.World().Messages
	st := c.session()
	for k := range st.dismissed { // a cleared notice that left the list is forgotten
		if !listed(msgs, k) {
			delete(st.dismissed, k)
		}
	}
	var live []liveMessage
	for _, m := range msgs {
		k := messageKey(m)
		if st.dismissed[k] {
			continue
		}
		first, ok := c.MsgSeen[k]
		if !ok {
			c.MsgSeen[k], first = now, now
		}
		if age := now - first; now >= first && age < MessageFrames {
			live = append(live, liveMessage{m, age})
		}
	}
	if len(live) > messageShown {
		live = live[len(live)-messageShown:]
	}
	return live
}

// listed reports whether a notice with key k is in msgs.
func listed(msgs []game.MessageView, k string) bool {
	for _, m := range msgs {
		if messageKey(m) == k {
			return true
		}
	}
	return false
}

// dismissMessage hides m for as long as the game still lists it (a right
// click on its row). It is presentation state, outside the save.
func (st *rightState) dismissMessage(m game.MessageView) {
	if st.dismissed == nil {
		st.dismissed = map[string]bool{}
	}
	st.dismissed[messageKey(m)] = true
}

// liveMessages are the notices still on screen, oldest first.
func (c Right) liveMessages() []game.MessageView {
	live := c.liveMessageList()
	out := make([]game.MessageView, len(live))
	for i, lm := range live {
		out[i] = lm.m
	}
	return out
}

// MessageAlpha is how opaque a notice of the given age is: fully until
// the last MessageFadeFrames, then fading to nothing.
func MessageAlpha(age uint64) float64 {
	left := int64(MessageFrames) - int64(age)
	switch {
	case left <= 0:
		return 0
	case left >= MessageFadeFrames:
		return 1
	}
	return float64(left) / MessageFadeFrames
}

// MessageRow is one notice and where it sits.
type MessageRow struct {
	R     gfx.Rect
	Msg   game.MessageView
	Idx   int // position in the live list (the click id msg.<idx>)
	Alpha float64
}

// MessageRows places the live notices in l's Messages rect, oldest on top,
// each as wide as its line (up to the rect) and right-aligned.
func (c Right) MessageRows(l hudlayout.Layout) []MessageRow {
	if l.Messages.Empty() {
		return nil
	}
	live := c.liveMessageList()
	var rows []MessageRow
	for i, lm := range live {
		y := l.Messages.Y + float32(i)*hudlayout.MsgStride
		if y+hudlayout.MsgRowH > l.Messages.Bottom()+0.01 {
			break
		}
		w := min(gfx.TextW(MessageLineIn(c.sh, lm.m), MessageStyle(lm.m))+2*messagePadX+messageEdgeW, l.Messages.W)
		rows = append(rows, MessageRow{R: gfx.Rect{X: l.Messages.Right() - w, Y: y, W: w, H: hudlayout.MsgRowH}, Msg: lm.m, Idx: i, Alpha: MessageAlpha(lm.age)})
	}
	return rows
}

// MessageButtons make the notices clickable: a click jumps to the
// creature (or place) the message is about.
func (c Right) MessageButtons(sw int) []uikit.Button { return c.messageButtonsIn(c.sh.Layout()) }

func (c Right) messageButtonsIn(l hudlayout.Layout) []uikit.Button {
	var out []uikit.Button
	for _, row := range c.MessageRows(l) {
		if row.Msg.Focus == 0 && row.Msg.X == 0 && row.Msg.Y == 0 {
			continue
		}
		out = append(out, uikit.Button{ID: ids.PrefixMsg + strconv.Itoa(row.Idx), X: row.R.X, Y: row.R.Y, W: row.R.W, H: row.R.H})
	}
	return out
}

// focusEntity selects creature id (an animal or a colonist) and centres
// the camera on it. A creature on another map than the one on screen goes
// through Jump, which switches maps first (x, y is where the news was). False
// when nothing has that id.
func (c Right) focusEntity(id entity.EntityID, mp game.MapID, x, y int) bool {
	if !c.live() || id == 0 {
		return false
	}
	if g := c.sh.Env().Game; g != nil && mp != g.ViewedMap() && mp != ports.CurrentMap {
		c.sh.Jump(ports.Spot{Map: mp, X: x, Y: y, Focus: id})
		return true
	}
	for _, an := range c.sh.World().Animals {
		if an.ID == id {
			c.sh.Pick(selection.Cand{Kind: selection.Animal, ID: an.ID})
			c.sh.Jump(ports.Spot{Map: ports.CurrentMap, X: an.X, Y: an.Y})
			return true
		}
	}
	for _, p := range c.sh.Pawns() {
		if p.ID == id {
			c.sh.Select(p.ID)
			c.sh.Jump(ports.Spot{Map: ports.CurrentMap, X: p.X, Y: p.Y})
			return true
		}
	}
	return false
}

// FocusMessage centres the camera on message i's creature (selecting
// it, so the inspector shows it) or else on its tile.
func (c Right) FocusMessage(id string) {
	i, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixMsg))
	live := c.liveMessages()
	if err != nil || i < 0 || i >= len(live) || !c.live() {
		return
	}
	m := live[i]
	if c.focusEntity(m.Focus, m.Map, m.X, m.Y) {
		return
	}
	if m.X != 0 || m.Y != 0 {
		c.sh.Jump(ports.Spot{Map: m.Map, X: m.X, Y: m.Y})
	}
}

// MessageStyle is a notice's text style: threats are drawn red.
func MessageStyle(m game.MessageView) gfx.TextStyle {
	st := gfx.Body
	if m.Danger {
		st.Color = uikit.Danger
	}
	return st
}

// MessageEdge is the colour of a notice's left edge: red for a danger, the
// kind's colour for a kinded notice, and none (zero alpha) otherwise.
func MessageEdge(m game.MessageView) color.RGBA {
	switch {
	case m.Danger:
		return uikit.LetterThreat
	case m.Kind != "":
		return LetterColor(m.Kind)
	}
	return color.RGBA{}
}

// fadeColor is c with its opacity scaled by k.
func fadeColor(c color.Color, k float64) color.Color {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return c
	}
	return color.NRGBA{uint8(r * 255 / a), uint8(g * 255 / a), uint8(b * 255 / a), uint8(float64(a>>8) * k)}
}

func (c Right) DrawMessages(screen *ebiten.Image) { c.DrawMessagesIn(screen, c.sh.Layout()) }

func (c Right) DrawMessagesIn(screen *ebiten.Image, l hudlayout.Layout) {
	for _, row := range c.MessageRows(l) {
		r, k := row.R, row.Alpha
		uikit.FillRect(screen, r, fadeColor(uikit.Tooltip, 0.85*k))
		if e := MessageEdge(row.Msg); e.A != 0 {
			uikit.FillRect(screen, gfx.Rect{X: r.X, Y: r.Y, W: messageEdgeW, H: r.H}, fadeColor(e, k))
		}
		st := MessageStyle(row.Msg)
		base := st.Color
		if base == nil {
			base = uikit.Text
		}
		st.Color = fadeColor(base, k)
		line := uikit.FitText(MessageLineIn(c.sh, row.Msg), st, r.W-2*messagePadX-messageEdgeW)
		gfx.Text(screen, line, float64(r.X+messageEdgeW+messagePadX), float64(r.Y)+(float64(r.H)-gfx.LineH(st))/2, st)
	}
}

// MessageLine is a notice's one line: a letter (the intro, extreme
// weather) opens in its own window, so the strip only names it.
func MessageLine(m game.MessageView) string { return MessageLineIn(en, m) }

// MessageLineIn is MessageLine in tr's language.
func MessageLineIn(tr lang.Tr, m game.MessageView) string {
	if m.Letter && m.Title != "" {
		return lang.Tf(tr, "Letter: %s", m.Title)
	}
	return m.Text
}
