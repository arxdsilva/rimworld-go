package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
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

// Messages: recent event notices stacked top-left under the top bar; each
// shows for messageFrames presentation frames after it first appears.
const (
	messageFrames = 600
	messageShown  = 5
)

func messageKey(m game.MessageView) string { return fmt.Sprintf("%d|%s", m.Tick, m.Text) }

// liveMessages are the notices still on screen, oldest first.
func (a *App) liveMessages() []game.MessageView {
	if a.renderer == nil {
		return nil
	}
	if a.msgSeen == nil {
		a.msgSeen = map[string]uint64{}
	}
	now := a.renderer.present
	msgs := a.renderer.world().Messages
	var live []game.MessageView
	for _, m := range msgs {
		k := messageKey(m)
		first, ok := a.msgSeen[k]
		if !ok {
			a.msgSeen[k], first = now, now
		}
		if now-first < messageFrames {
			live = append(live, m)
		}
	}
	if len(live) > messageShown {
		live = live[len(live)-messageShown:]
	}
	return live
}

// messageRect is the i-th notice's rect: centred under the top bar,
// clear of the side panels and alerts.
func (a *App) messageRect(sw, i int, m game.MessageView) button {
	w := len(messageLine(m))*6 + 16
	return button{x: float32((sw - w) / 2), y: float32(hudTopH + hudPad + i*24), w: float32(w), h: 20}
}

// messageButtons make the notices clickable: a click jumps to the
// creature (or place) the message is about.
func (a *App) messageButtons(sw int) []button {
	var out []button
	for i, m := range a.liveMessages() {
		if m.Focus == 0 && m.X == 0 && m.Y == 0 {
			continue
		}
		r := a.messageRect(sw, i, m)
		r.id = "msg." + strconv.Itoa(i)
		out = append(out, r)
	}
	return out
}

// focusMessage centres the camera on message i's creature (selecting
// it, so the inspector shows it) or else on its tile.
func (a *App) focusMessage(id string) {
	i, err := strconv.Atoi(strings.TrimPrefix(id, "msg."))
	live := a.liveMessages()
	if err != nil || i < 0 || i >= len(live) || a.renderer == nil {
		return
	}
	m := live[i]
	if m.Focus != 0 {
		for _, an := range a.renderer.world().Animals {
			if an.ID == m.Focus {
				a.selectCand(selCand{selAnimal, an.ID})
				a.renderer.centerOn(float64(an.X)+0.5, float64(an.Y)+0.5, a.sw, a.sh)
				return
			}
		}
		for _, p := range a.renderer.snapshot() {
			if p.ID == m.Focus {
				a.renderer.Select(p.ID)
				a.renderer.centerOn(float64(p.X)+0.5, float64(p.Y)+0.5, a.sw, a.sh)
				return
			}
		}
	}
	if m.X != 0 || m.Y != 0 {
		a.renderer.centerOn(float64(m.X)+0.5, float64(m.Y)+0.5, a.sw, a.sh)
	}
}

func (a *App) drawMessages(screen *ebiten.Image) {
	sw, _ := a.size()
	for i, m := range a.liveMessages() {
		r := a.messageRect(sw, i, m)
		vector.FillRect(screen, r.x, r.y, r.w, r.h, color.RGBA{20, 16, 12, 200}, false)
		drawLabel(screen, int(r.x)+8, int(r.y)+3, messageLine(m))
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
