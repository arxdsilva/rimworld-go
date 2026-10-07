package ebiten

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// stackCountMinTile: below this tile size (px) counts would just clutter.
const stackCountMinTile = 36

// drawStackCounts prints each ground stack's quantity in the bottom-right
// corner of its tile, on a small dark tag so it reads over any sprite.
func (r *Renderer) drawStackCounts(screen *ebiten.Image, w game.WorldView, tile float64) {
	if tile < stackCountMinTile {
		return
	}
	for _, it := range w.Items {
		if it.Carried || it.Qty <= 1 {
			continue
		}
		s := strconv.Itoa(it.Qty)
		sx, sy := r.worldToScreen(it.X, it.Y)
		tw := float32(len(s)*6 + 4)
		x := sx + float32(tile) - tw - 2
		y := sy + float32(tile) - 16
		vector.FillRect(screen, x, y, tw, 14, color.RGBA{16, 12, 8, 190}, false)
		ebitenutil.DebugPrintAt(screen, s, int(x)+2, int(y)-1)
	}
}

// drawChatBubbles floats a small speech bubble over colonists chatting.
func (r *Renderer) drawChatBubbles(screen *ebiten.Image, tile float64) {
	for _, p := range r.drawPawns {
		if !strings.HasPrefix(p.Job, "chat do") {
			continue
		}
		// bubbles come and go so two chatting colonists take turns
		if (r.present/50+uint64(p.ID))%2 == 1 {
			continue
		}
		wx, wy := r.interpWorld(p)
		sx, sy := r.worldToScreenF(wx+0.75, wy-0.55)
		bw, bh := float32(tile*0.42), float32(tile*0.24)
		vector.FillRect(screen, sx-bw/2, sy-bh/2, bw, bh, color.RGBA{250, 248, 240, 235}, true)
		vector.StrokeRect(screen, sx-bw/2, sy-bh/2, bw, bh, 1, color.RGBA{60, 50, 40, 220}, true)
		vector.FillCircle(screen, sx-bw/2+bw*0.2, sy+bh/2+2, float32(tile*0.035), color.RGBA{250, 248, 240, 235}, true)
		for i := -1; i <= 1; i++ {
			vector.FillCircle(screen, sx+float32(i)*bw*0.25, sy, float32(tile*0.025), color.RGBA{70, 60, 50, 255}, true)
		}
	}
}

// drawBreakClouds floats a small dark storm cloud over colonists having a
// mental break.
func (r *Renderer) drawBreakClouds(screen *ebiten.Image, tile float64) {
	for _, p := range r.drawPawns {
		if p.MentalBreak == "" {
			continue
		}
		wx, wy := r.interpWorld(p)
		bob := math.Sin(float64(r.present)/14+float64(p.ID)) * 0.04
		sx, sy := r.worldToScreenF(wx+0.5, wy-1.0+bob)
		c := color.RGBA{70, 70, 84, 230}
		for _, o := range [][3]float64{{-0.12, 0.02, 0.10}, {0.0, -0.03, 0.13}, {0.13, 0.02, 0.10}} {
			vector.FillCircle(screen, sx+float32(o[0]*tile), sy+float32(o[1]*tile), float32(o[2]*tile), c, true)
		}
		if (r.present/8+uint64(p.ID))%6 == 0 { // a flicker of lightning
			vector.StrokeLine(screen, sx, sy+float32(tile*0.08), sx-float32(tile*0.04), sy+float32(tile*0.2), 2, color.RGBA{250, 230, 120, 255}, true)
		}
	}
}

// drawHostileMarker floats a red chevron over a raider (orange while it
// flees).
func (r *Renderer) drawHostileMarker(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64) {
	c := color.RGBA{220, 50, 40, 240}
	if p.Fleeing {
		c = color.RGBA{230, 150, 50, 220}
	}
	sx, sy := r.worldToScreenF(wx+0.5, wy-0.95)
	w, h := float32(tile*0.13), float32(tile*0.1)
	th := float32(tile * 0.06)
	vector.StrokeLine(screen, sx-w, sy-h, sx, sy+h, th, c, true)
	vector.StrokeLine(screen, sx, sy+h, sx+w, sy-h, th, c, true)
}
