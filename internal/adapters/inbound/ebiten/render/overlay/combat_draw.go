package overlay

import (
	"fmt"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// Combat feedback (DEF-1 UI): the weapon range ring of a drafted colonist,
// the line of sight and hit-chance card over a hovered hostile, the cover
// overlay and the projectiles in flight. Presentation only. The inputs are
// small consumer-owned shapes (WeaponReach, ShotInfo, CoverCell,
// ProjectileMark) the root fills from the read models, so the package does
// not depend on how the sim names them. Every draw returns at once when
// there is nothing to show: a frame with no overlay costs nothing.

// Combat is the combat overlays: the cover toggle and the draws below.
type Combat struct {
	*base
	Cover Toggle // the cover overlay (key binding flips On)
	// Marks, Cells and Factors are per-frame scratch the root refills each
	// frame (buf[:0]), so feeding the draws allocates nothing in steady
	// state (the colony draw's no-allocation rule).
	Marks   []ProjectileMark
	Cells   []CoverCell
	Factors []ShotFactor
}

// WeaponReach is a selected colonist's weapon reach in tiles: Min is the
// closest it can fire (0: none), Max the farthest.
type WeaponReach struct{ Min, Max float64 }

// ShotFactor is one line of a hit chance: a label and its effect in
// percentage points (negative lowers the chance).
type ShotFactor struct {
	Label string
	Pct   int
}

// ShotInfo answers "can this colonist hit that target": the chance with its
// factors, or why the shot is blocked.
type ShotInfo struct {
	FromX, FromY, ToX, ToY int
	Blocked                bool
	Reason                 string // why a blocked shot is blocked
	Chance                 int    // percent, 0..100
	Factors                []ShotFactor
}

// CoverCell is the cover a tile gives against fire, percent 0..100.
type CoverCell struct{ X, Y, Value int }

// ProjectileMark is one projectile in flight: its position in world tiles
// (cell centres at +0.5), unit heading and kind ("bullet", "arrow", "shell"...).
type ProjectileMark struct {
	X, Y, DX, DY float64
	Kind         string
}

// The combat colours reuse the palette of the paths and zones overlays.
var (
	rangeMaxColour  = color.NRGBA{232, 90, 70, 200}
	rangeMinColour  = color.NRGBA{232, 190, 70, 170}
	losClearColour  = color.NRGBA{255, 255, 255, 190}
	losBlockColour  = color.NRGBA{232, 90, 70, 220}
	coverBaseColour = [3]uint8{70, 150, 230}
	tracerColour    = color.NRGBA{255, 226, 140, 255}
)

// minCoverShown is the cover value (percent) below which a tile is not marked.
const minCoverShown = 5

// DrawWeaponRange strokes the weapon reach of the colonist at (x, y): the
// far ring, and the near one when the weapon has a minimum range.
func (c *Combat) DrawWeaponRange(v *render.View, dst *ebiten.Image, x, y int, r WeaponReach) {
	if r.Max <= 0 {
		return
	}
	cx, cy := v.WorldToScreenF(float64(x)+0.5, float64(y)+0.5)
	t := v.Cam.Tile()
	th := float32(max(1.5, t*0.04))
	vector.StrokeCircle(dst, cx, cy, float32(r.Max*t), th, rangeMaxColour, true)
	if r.Min > 0 && r.Min < r.Max {
		vector.StrokeCircle(dst, cx, cy, float32(r.Min*t), th, rangeMinColour, true)
	}
}

// ShotLines are the lines of the hit-chance card: the headline, then one
// per factor ("+10% aim"), or the reason for a blocked shot.
func ShotLines(s ShotInfo) []string {
	if s.Blocked {
		reason := s.Reason
		if reason == "" {
			reason = "no line of fire"
		}
		return []string{"Blocked: " + reason}
	}
	out := make([]string, 0, 1+len(s.Factors))
	out = append(out, fmt.Sprintf("Hit chance %d%%", s.Chance))
	for _, f := range s.Factors {
		out = append(out, fmt.Sprintf("%+d%% %s", f.Pct, f.Label))
	}
	return out
}

// DrawShotFeedback draws the line of sight from the shooter to the hovered
// target (white clear, red blocked) and the hit-chance card beside the
// cursor at screen (mx, my). s nil draws nothing.
func (c *Combat) DrawShotFeedback(v *render.View, dst *ebiten.Image, s *ShotInfo, mx, my int) {
	if s == nil {
		return
	}
	ax, ay := v.WorldToScreenF(float64(s.FromX)+0.5, float64(s.FromY)+0.5)
	bx, by := v.WorldToScreenF(float64(s.ToX)+0.5, float64(s.ToY)+0.5)
	col := losClearColour
	if s.Blocked {
		col = losBlockColour
	}
	vector.StrokeLine(dst, ax, ay, bx, by, float32(max(1.5, v.Cam.Tile()*0.05)), col, true)
	lines := ShotLines(*s)
	st := gfx.Small
	k := gfx.UIScale() // map pixels per logical pixel: the card is drawn in map pixels
	w := 0.0
	for _, l := range lines {
		lw, _ := gfx.Measure(l, st)
		w = max(w, lw*k)
	}
	pad, lh := float32(6*k), float32(14*k)
	x, y := float32(mx)+float32(16*k), float32(my)+float32(16*k)
	cw, ch := float32(w)+2*pad, float32(len(lines))*lh+2*pad
	if x+cw > float32(v.SW) {
		x = float32(mx) - float32(16*k) - cw
	}
	if y+ch > float32(v.SH) {
		y = float32(v.SH) - ch
	}
	vector.FillRect(dst, x, y, cw, ch, uikit.Panel, false)
	vector.StrokeRect(dst, x, y, cw, ch, max(1, float32(k)), uikit.PanelBorder, false)
	for i, l := range lines {
		ls := st
		if i == 0 && s.Blocked {
			ls.Color = losBlockColour
		}
		gfx.WorldText(dst, l, float64(x+pad), float64(y+pad+float32(i)*lh), ls)
	}
}

// CoverAlpha is the fill alpha (0..255) of a cover value: nothing below
// minCoverShown, full cover (100) at 150.
func CoverAlpha(value int) uint8 {
	if value < minCoverShown {
		return 0
	}
	return uint8(min(value, 100) * 150 / 100)
}

// DrawCover tints each cell by the cover it gives, with its percent when
// tiles are large enough to read.
func (c *Combat) DrawCover(v *render.View, dst *ebiten.Image, cells []CoverCell) {
	if !c.Cover.Shown() || len(cells) == 0 {
		return
	}
	t := float32(v.Cam.Tile())
	for _, cell := range cells {
		a := CoverAlpha(cell.Value)
		if a == 0 {
			continue
		}
		sx, sy := v.WorldToScreenF(float64(cell.X), float64(cell.Y))
		if sx+t < 0 || sy+t < 0 || sx > float32(v.SW) || sy > float32(v.SH) {
			continue
		}
		c.solidRect(dst, sx, sy, t, t, color.NRGBA{coverBaseColour[0], coverBaseColour[1], coverBaseColour[2], a})
		if t >= 28 {
			gfx.WorldText(dst, fmt.Sprintf("%d", cell.Value), float64(sx)+3, float64(sy)+3, gfx.Small)
		}
	}
}

// ProjectileSegment is the screen-space tracer of p: from tail to head,
// length in tiles by kind (a shell is a dot, a bullet a short streak).
func ProjectileSegment(v *render.View, p ProjectileMark) (ax, ay, bx, by float32) {
	l := 0.5
	switch p.Kind {
	case "shell", "grenade":
		l = 0.08
	case "arrow":
		l = 0.7
	}
	d := math.Hypot(p.DX, p.DY)
	if d == 0 {
		d = 1
	}
	ax, ay = v.WorldToScreenF(p.X-p.DX/d*l, p.Y-p.DY/d*l)
	bx, by = v.WorldToScreenF(p.X, p.Y)
	return
}

// DrawProjectiles strokes each projectile as a short streak along its heading.
func (c *Combat) DrawProjectiles(v *render.View, dst *ebiten.Image, ps []ProjectileMark) {
	if len(ps) == 0 {
		return
	}
	th := float32(max(1.5, v.Cam.Tile()*0.06))
	for _, p := range ps {
		ax, ay, bx, by := ProjectileSegment(v, p)
		if bx < 0 || by < 0 || bx > float32(v.SW) || by > float32(v.SH) {
			continue
		}
		vector.StrokeLine(dst, ax, ay, bx, by, th, tracerColour, true)
	}
}
