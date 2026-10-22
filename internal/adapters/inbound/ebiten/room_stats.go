package ebiten

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/input"
)

// Room stats overlay (UI-016, G): every room tinted by impressiveness,
// and a card for the room under the cursor (role, impressiveness,
// cleanliness, temperature, light, roof) from game.RoomView. Plus the
// Alt-held detailed cell lines of the hover panel.

const (
	roomsButtonID = "hud.rooms"
	roomsButtonW  = 48
)

// roomsButtons is the HUD Rooms toggle, left of the Home button.
func (a *App) roomsButtons(sw int) []button {
	hb := a.homeButtons(sw)
	if len(hb) == 0 {
		return nil
	}
	return []button{{id: roomsButtonID, label: "Rooms", x: hb[0].x - roomsButtonW - hudPad/2, y: 4, w: roomsButtonW, h: hudTopH - 8,
		selected: a.rooms, tip: "Room stats overlay" + a.keyHint(input.RoomStats)}}
}

func (a *App) drawRoomsButton(screen *ebiten.Image, sw int) {
	mx, my := a.currentCursor()
	for _, b := range a.roomsButtons(sw) {
		drawButton(screen, b, b.contains(mx, my))
		if img := a.icon("overlay_rooms"); img != nil {
			vector.FillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtn, false)
			if b.selected {
				vector.FillRect(screen, b.x+2, b.y+2, b.w-4, b.h-4, uiBtnSelected, false)
			}
			drawIcon(screen, img, float64(b.x)+(float64(b.w)-float64(b.h)+4)/2, float64(b.y)+2, float64(b.h)-4)
		}
	}
}

// roomTint colours a room by impressiveness: red (awful) through amber
// and yellow to green (impressive).
func roomTint(score int) color.NRGBA {
	stops := []struct {
		at  float64
		clr [3]float64
	}{{-6, [3]float64{200, 50, 40}}, {3, [3]float64{220, 140, 50}}, {9, [3]float64{220, 210, 70}}, {18, [3]float64{70, 190, 80}}}
	s := float64(score)
	c := stops[len(stops)-1].clr
	for i := 0; i+1 < len(stops); i++ {
		lo, hi := stops[i], stops[i+1]
		if s <= lo.at {
			c = lo.clr
			break
		}
		if s < hi.at {
			t := (s - lo.at) / (hi.at - lo.at)
			for k := range c {
				c[k] = lo.clr[k] + (hi.clr[k]-lo.clr[k])*t
			}
			break
		}
	}
	return color.NRGBA{R: uint8(c[0]), G: uint8(c[1]), B: uint8(c[2]), A: 90}
}

// drawRoomOverlay tints the visible room tiles.
func (a *App) drawRoomOverlay(screen *ebiten.Image) {
	if !a.rooms || a.renderer == nil {
		return
	}
	r := a.renderer
	w := r.world()
	sw, sh := a.size()
	tile := r.tilePx()
	x0, y0 := r.TileAt(0, 0)
	x1, y1 := r.TileAt(sw, sh)
	for y := max(y0-1, 0); y <= y1+1; y++ {
		for x := max(x0-1, 0); x <= x1+1; x++ {
			id := roomIDAt(w, x, y)
			if id < 0 || id >= len(w.Rooms) {
				continue
			}
			sx, sy := r.worldToScreen(x, y)
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), roomTint(w.Rooms[id].Score), false)
		}
	}
}

// roomCardLines is the hover card for a room.
func roomCardLines(rm game.RoomView) []string {
	lit := "no"
	if rm.Lit {
		lit = "yes"
	}
	clean := rm.Cleanliness
	if clean == "" {
		clean = "clean"
	}
	return []string{
		titleCase(rm.Role),
		fmt.Sprintf("Impressiveness %d (%s)", rm.Score, rm.Label),
		fmt.Sprintf("Cleanliness: %s", clean),
		"Temperature " + fmtTemp(rm.Temp),
		fmt.Sprintf("Lit: %s   Roofed: %d%%", lit, int(math.Round(rm.Roofed*100))),
		fmt.Sprintf("%d cells", rm.Cells),
	}
}

// drawRoomCard shows the hovered room's card beside the cursor.
func (a *App) drawRoomCard(screen *ebiten.Image) {
	if !a.rooms || a.renderer == nil {
		return
	}
	mx, my := a.currentCursor()
	sw, sh := a.size()
	if a.colonyChromeHit(mx, my, sw, sh) {
		return
	}
	w := a.renderer.world()
	tx, ty := a.renderer.TileAt(mx, my)
	id := roomIDAt(w, tx, ty)
	if id < 0 || id >= len(w.Rooms) {
		return
	}
	lines := roomCardLines(w.Rooms[id])
	wid := 0
	for _, l := range lines {
		wid = max(wid, len(l)*6)
	}
	pw, ph := wid+20, len(lines)*16+12
	x, y := min(mx+18, sw-pw-4), min(my+18, int(hudBarY(sh))-ph-4)
	drawPanel(screen, float32(x), float32(y), float32(pw), float32(ph))
	vector.FillRect(screen, float32(x)+1, float32(y)+1, 4, float32(ph)-2, opaque(roomTint(w.Rooms[id].Score)), false)
	for i, l := range lines {
		drawLabel(screen, x+10, y+6+i*16, l)
	}
}

func opaque(c color.NRGBA) color.NRGBA { c.A = 255; return c }

// detailLines are the Alt-held extra hover lines for a tile: walk cost,
// fertility, room id and score, and every stack with its flags.
func detailLines(w game.WorldView, x, y int) []string {
	var out []string
	if w.TerrainW > 0 && x >= 0 && y >= 0 && x < w.TerrainW {
		if i := y*w.TerrainW + x; i < len(w.Terrain) {
			if soils := game.Soils(); int(w.Terrain[i]) < len(soils) {
				s := soils[w.Terrain[i]]
				walk := "impassable"
				if s.Walkable {
					walk = fmt.Sprintf("%d%%", max(s.WalkCost, 100))
				}
				out = append(out, fmt.Sprintf("Walk cost %s   Fertility %d%% (%s)", walk, s.Fertility, s.Name))
			}
		}
	}
	switch id := roomIDAt(w, x, y); {
	case id >= 0 && id < len(w.Rooms):
		rm := w.Rooms[id]
		out = append(out, fmt.Sprintf("Room #%d: %s, score %d, %d cells", rm.ID, rm.Role, rm.Score, rm.Cells))
	case id == -1:
		out = append(out, "Room: outdoors")
	case id == -2:
		out = append(out, "Room: wall")
	}
	out = append(out, fmt.Sprintf("Cell (%d, %d)", x, y))
	for _, it := range w.Items {
		if it.X != x || it.Y != y {
			continue
		}
		var flags []string
		if it.Forbidden {
			flags = append(flags, "forbidden")
		}
		if it.Carried {
			flags = append(flags, "carried")
		}
		if it.Rotten {
			flags = append(flags, "rotten")
		}
		s := fmt.Sprintf("  #%d %s x %d", it.ID, it.Kind, it.Qty)
		if len(flags) > 0 {
			s += " [" + strings.Join(flags, ", ") + "]"
		}
		out = append(out, s)
	}
	return out
}
