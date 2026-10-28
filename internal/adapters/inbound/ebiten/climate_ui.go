package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// fmtTemp renders tenths of a degree C as whole degrees ("-3C").
func fmtTemp(tenths int) string {
	if tenths < 0 {
		return fmt.Sprintf("%dC", (tenths-5)/10)
	}
	return fmt.Sprintf("%dC", (tenths+5)/10)
}

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
func spoilingAlert(w game.WorldView) (alert, bool) {
	a := alert{key: "spoiling"}
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
			a.targets = append(a.targets, [2]int{it.X, it.Y})
		}
	}
	if qty == 0 {
		return a, false
	}
	a.label = fmt.Sprintf("Food spoiling soon (%d)", qty)
	return a, true
}

// Messages (PLAY-018): recent event notices stacked in the layout's
// Messages rect, right-aligned against the right column under the colonist
// bar. Each shows for messageFrames presentation frames after it first
// appears and fades out over the last messageFadeFrames.
const (
	messageFrames     = 600
	messageFadeFrames = 60
	messageShown      = maxMsgs
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
func (a *App) liveMessageList() []liveMessage {
	if a.renderer == nil {
		return nil
	}
	if a.msgSeen == nil {
		a.msgSeen = map[string]uint64{}
	}
	now := a.shell.Present()
	msgs := a.shell.World().Messages
	var live []liveMessage
	for _, m := range msgs {
		k := messageKey(m)
		first, ok := a.msgSeen[k]
		if !ok {
			a.msgSeen[k], first = now, now
		}
		if age := now - first; now >= first && age < messageFrames {
			live = append(live, liveMessage{m, age})
		}
	}
	if len(live) > messageShown {
		live = live[len(live)-messageShown:]
	}
	return live
}

// liveMessages are the notices still on screen, oldest first.
func (a *App) liveMessages() []game.MessageView {
	live := a.liveMessageList()
	out := make([]game.MessageView, len(live))
	for i, lm := range live {
		out[i] = lm.m
	}
	return out
}

// messageAlpha is how opaque a notice of the given age is: fully until
// the last messageFadeFrames, then fading to nothing.
func messageAlpha(age uint64) float64 {
	left := int64(messageFrames) - int64(age)
	switch {
	case left <= 0:
		return 0
	case left >= messageFadeFrames:
		return 1
	}
	return float64(left) / messageFadeFrames
}

// messageRow is one notice and where it sits.
type messageRow struct {
	r     rect
	msg   game.MessageView
	idx   int // position in the live list (the click id msg.<idx>)
	alpha float64
}

// messageRows places the live notices in l's Messages rect, oldest on top,
// each as wide as its line (up to the rect) and right-aligned.
func (a *App) messageRows(l hudLayout) []messageRow {
	if l.Messages.Empty() {
		return nil
	}
	live := a.liveMessageList()
	var rows []messageRow
	for i, lm := range live {
		y := l.Messages.Y + float32(i)*msgStride
		if y+msgRowH > l.Messages.Bottom()+0.01 {
			break
		}
		w := min(textW(messageLine(lm.m), messageStyle(lm.m))+2*messagePadX+messageEdgeW, l.Messages.W)
		rows = append(rows, messageRow{r: rect{X: l.Messages.Right() - w, Y: y, W: w, H: msgRowH}, msg: lm.m, idx: i, alpha: messageAlpha(lm.age)})
	}
	return rows
}

// messageButtons make the notices clickable: a click jumps to the
// creature (or place) the message is about.
func (a *App) messageButtons(sw int) []button { return a.messageButtonsIn(a.hud()) }

func (a *App) messageButtonsIn(l hudLayout) []button {
	var out []button
	for _, row := range a.messageRows(l) {
		if row.msg.Focus == 0 && row.msg.X == 0 && row.msg.Y == 0 {
			continue
		}
		out = append(out, button{ID: ids.PrefixMsg + strconv.Itoa(row.idx), X: row.r.X, Y: row.r.Y, W: row.r.W, H: row.r.H})
	}
	return out
}

// focusEntity selects creature id (an animal or a colonist) and centres
// the camera on it. False when nothing has that id.
func (a *App) focusEntity(id entity.EntityID) bool {
	if a.renderer == nil || id == 0 {
		return false
	}
	for _, an := range a.shell.World().Animals {
		if an.ID == id {
			a.selectCand(selCand{Kind: selAnimal, ID: an.ID})
			a.centerOnMap(float64(an.X)+0.5, float64(an.Y)+0.5)
			return true
		}
	}
	for _, p := range a.shell.Pawns() {
		if p.ID == id {
			a.shell.Select(p.ID)
			a.centerOnMap(float64(p.X)+0.5, float64(p.Y)+0.5)
			return true
		}
	}
	return false
}

// focusMessage centres the camera on message i's creature (selecting
// it, so the inspector shows it) or else on its tile.
func (a *App) focusMessage(id string) {
	i, err := strconv.Atoi(strings.TrimPrefix(id, ids.PrefixMsg))
	live := a.liveMessages()
	if err != nil || i < 0 || i >= len(live) || a.renderer == nil {
		return
	}
	m := live[i]
	if a.focusEntity(m.Focus) {
		return
	}
	if m.X != 0 || m.Y != 0 {
		a.centerOnMap(float64(m.X)+0.5, float64(m.Y)+0.5)
	}
}

// messageStyle is a notice's text style: threats are drawn red.
func messageStyle(m game.MessageView) textStyle {
	st := tsBody
	if m.Danger {
		st.Color = uiDanger
	}
	return st
}

// messageEdge is the colour of a notice's left edge: red for a danger, the
// kind's colour for a kinded notice, and none (zero alpha) otherwise.
func messageEdge(m game.MessageView) color.RGBA {
	switch {
	case m.Danger:
		return uiLetterThreat
	case m.Kind != "":
		return letterColor(m.Kind)
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

func (a *App) drawMessages(screen *ebiten.Image) { a.drawMessagesIn(screen, a.hud()) }

func (a *App) drawMessagesIn(screen *ebiten.Image, l hudLayout) {
	for _, row := range a.messageRows(l) {
		r, k := row.r, row.alpha
		fillRect(screen, r, fadeColor(uiTooltip, 0.85*k))
		if e := messageEdge(row.msg); e.A != 0 {
			fillRect(screen, rect{X: r.X, Y: r.Y, W: messageEdgeW, H: r.H}, fadeColor(e, k))
		}
		st := messageStyle(row.msg)
		base := st.Color
		if base == nil {
			base = uiText
		}
		st.Color = fadeColor(base, k)
		line := fitText(messageLine(row.msg), st, r.W-2*messagePadX-messageEdgeW)
		drawText(screen, line, float64(r.X+messageEdgeW+messagePadX), float64(r.Y)+(float64(r.H)-lineH(st))/2, st)
	}
}

// messageLine is a notice's one line: a letter (the intro, extreme
// weather) opens in its own window, so the strip only names it.
func messageLine(m game.MessageView) string {
	if m.Letter && m.Title != "" {
		return "Letter: " + m.Title
	}
	return m.Text
}
