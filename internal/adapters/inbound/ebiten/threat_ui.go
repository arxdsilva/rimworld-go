package ebiten

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Threat pause and floating damage numbers (presentation only).

// watchThreats pauses the game when a new danger message arrives (if the
// player wants that). Messages already there when the session starts or
// a save loads never pause.
func (r *Renderer) watchThreats(w game.WorldView) {
	newest := r.lastDanger
	for _, m := range w.Messages {
		if m.Danger && m.Tick > newest {
			newest = m.Tick
		}
	}
	if !r.dangerInit {
		r.dangerInit, r.lastDanger = true, newest
		return
	}
	if newest == r.lastDanger {
		return
	}
	r.lastDanger = newest
	if r.pauseOnThreat && r.speed != speedPaused {
		r.speed = speedPaused
		r.threatPaused = true
	}
}

// speedRate is the HUD speed label, saying why the game paused itself.
func (r *Renderer) speedRate() string {
	if r.threatPaused && r.speed == speedPaused {
		return "paused (threat)"
	}
	r.threatPaused = false
	return r.speed.rate()
}

// floater is a number drifting up from a hit.
type floater struct {
	x, y float64 // tiles
	text string
	clr  color.RGBA
	born uint64
}

const floaterLife = 50 // frames

var (
	floatHit      = color.RGBA{255, 236, 210, 255}
	floatBuilding = color.RGBA{230, 190, 120, 255}
	floatMiss     = color.RGBA{190, 190, 190, 255}
)

// floatTrace starts a floating number for a newly seen attack.
func (r *Renderer) floatTrace(t game.TraceView, seed uint64) {
	jx := float64(seed%7)/20 - 0.15
	f := floater{x: float64(t.ToX) + 0.5 + jx, y: float64(t.ToY) - 0.1, born: r.present}
	switch {
	case t.Hit && t.Damage > 0:
		f.text, f.clr = "-"+strconv.Itoa(t.Damage), floatHit
		if t.Building {
			f.clr = floatBuilding
		}
	case !t.Hit && !t.Melee:
		f.text, f.clr = "miss", floatMiss
	default:
		return
	}
	r.floaters = append(r.floaters, f)
	if len(r.floaters) > 64 {
		r.floaters = r.floaters[len(r.floaters)-64:]
	}
}

// drawFloaters draws and ages the floating numbers.
func (r *Renderer) drawFloaters(screen *ebiten.Image, tile float64) {
	kept := r.floaters[:0]
	for _, f := range r.floaters {
		age := r.present - f.born
		if age >= floaterLife {
			continue
		}
		kept = append(kept, f)
		rise := float64(age) / floaterLife * 0.7
		sx, sy := r.worldToScreenF(f.x, f.y-rise)
		k := min(3, max(1.5, tile/36)) // text scale follows the zoom
		w := float32(float64(len(f.text)*6+4) * k)
		h := float32(14 * k)
		fade := float32(1)
		if age > floaterLife/2 {
			fade = float32(floaterLife-age) / float32(floaterLife/2)
		}
		vector.FillRect(screen, sx-w/2, sy-h/2, w, h, color.RGBA{16, 12, 8, uint8(170 * fade)}, false)
		img := floaterText(f.text)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(float64(sx-w/2)+2*k, float64(sy-h/2)-k)
		op.ColorScale.ScaleWithColor(f.clr)
		op.ColorScale.ScaleAlpha(fade)
		screen.DrawImage(img, op)
	}
	r.floaters = kept
}

var floaterCache = map[string]*ebiten.Image{}

// floaterText renders (and caches) a short white debug-font label.
func floaterText(s string) *ebiten.Image {
	if img, ok := floaterCache[s]; ok {
		return img
	}
	img := ebiten.NewImage(len(s)*6+2, 16)
	ebitenutil.DebugPrintAt(img, s, 0, 0)
	if len(floaterCache) < 256 {
		floaterCache[s] = img
	}
	return img
}
