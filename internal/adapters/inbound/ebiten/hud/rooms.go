package hud

import (
	"fmt"
	"image/color"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strings"

	"rimworld-go/internal/application/game"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Room stats overlay (UI-016, G): every room tinted by impressiveness,
// and a card for the room under the cursor (role, impressiveness,
// cleanliness, temperature, light, roof) from game.RoomView. Plus the
// Alt-held detailed cell lines of the hover panel.

// RoomTint colours a room by impressiveness: red (awful) through amber
// and yellow to green (impressive).
func RoomTint(score int) color.NRGBA {
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

// DrawOverlay tints the visible room tiles.
func (c RoomStats) DrawOverlay(screen *ebiten.Image) {
	if !c.Rooms || !c.live() {
		return
	}
	w := c.sh.World()
	sw, sh := c.sh.Size()
	tile := c.sh.TilePx()
	x0, y0 := c.sh.TileAt(0, 0)
	x1, y1 := c.sh.TileAt(int(float64(sw)*c.sh.MapScale()), int(float64(sh)*c.sh.MapScale()))
	for y := max(y0-1, 0); y <= y1+1; y++ {
		for x := max(x0-1, 0); x <= x1+1; x++ {
			id := overlay.RoomIDAt(w, x, y)
			if id < 0 || id >= len(w.Rooms) {
				continue
			}
			sx, sy := c.sh.WorldToScreen(x, y)
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), RoomTint(w.Rooms[id].Score), false)
		}
	}
}

// RoomCardLines is the hover card for a room.
func RoomCardLines(rm game.RoomView) []string {
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
		"Temperature " + FmtTemp(rm.Temp),
		fmt.Sprintf("Lit: %s   Roofed: %d%%", lit, int(math.Round(rm.Roofed*100))),
		fmt.Sprintf("%d cells", rm.Cells),
	}
}

// DrawCard shows the hovered room's card beside the cursor.
func (c RoomStats) DrawCard(screen *ebiten.Image) {
	if !c.Rooms || !c.live() {
		return
	}
	mx, my := c.sh.Cursor()
	sw, sh := c.sh.Size()
	if c.sh.ChromeHit(mx, my, sw, sh) {
		return
	}
	w := c.sh.World()
	tx, ty := c.sh.TileAt(c.sh.MapCursor())
	id := overlay.RoomIDAt(w, tx, ty)
	if id < 0 || id >= len(w.Rooms) {
		return
	}
	lines := RoomCardLines(w.Rooms[id])
	wid := int(gfx.MaxTextW(lines, gfx.Body))
	rowH := int(gfx.LineStep(gfx.Body))
	pw, ph := wid+24, len(lines)*rowH+12
	x, y := min(mx+18, sw-pw-4), min(my+18, int(c.sh.Layout().Tabs.Y)-ph-4)
	uikit.DrawPanel(screen, float32(x), float32(y), float32(pw), float32(ph))
	gfx.FillRect(screen, float32(x)+1, float32(y)+1, 4, float32(ph)-2, opaque(RoomTint(w.Rooms[id].Score)), false)
	for i, l := range lines {
		st := gfx.Body
		if i == 0 {
			st = gfx.WithColor(gfx.Label, uikit.Heading)
		}
		gfx.Text(screen, l, float64(x+14), float64(y+6+i*rowH), st)
	}
}

func opaque(c color.NRGBA) color.NRGBA { c.A = 255; return c }

// DetailLines are the Alt-held extra hover lines for a tile: walk cost,
// fertility, room id and score, and every stack with its flags.
func DetailLines(w game.WorldView, x, y int) []string {
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
	switch id := overlay.RoomIDAt(w, x, y); {
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
