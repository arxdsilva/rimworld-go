package ebiten

import (
	"fmt"
	"image/color"
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

func (a *App) drawMessages(screen *ebiten.Image) {
	if a.renderer == nil {
		return
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
	// centred under the top bar, clear of the side panels and alerts
	sw, _ := a.size()
	y := hudTopH + hudPad
	for _, m := range live {
		w := len(m.Text)*6 + 16
		x := (sw - w) / 2
		vector.FillRect(screen, float32(x), float32(y), float32(w), 20, color.RGBA{20, 16, 12, 200}, false)
		drawLabel(screen, x+8, y+3, m.Text)
		y += 24
	}
}
