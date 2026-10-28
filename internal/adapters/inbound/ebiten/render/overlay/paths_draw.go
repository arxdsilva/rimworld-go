package overlay

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// Colonist paths (UI-013): every walking colonist's remaining route is a
// faint dotted polyline to its goal; drafted ones are bolder, the
// selected one white and last. Presentation only, from PawnView.Path.

// pathMinZoom is the zoom below which only the goal markers are drawn.
const pathMinZoom = 0.35

type pathSeg struct{ ax, ay, bx, by float32 }

// PawnPathPlan is what to draw for one colonist, in screen pixels.
type PawnPathPlan struct {
	id           uint64
	Drafted      bool
	Selected     bool
	Segs         []pathSeg // dashes, already culled to the screen
	HasGoal      bool
	gx, gy       float32
	HasDrop      bool
	dx, dy       float32
	fromX, fromY float32
	hasFrom      bool
}

func segOnScreen(s pathSeg, sw, sh int) bool {
	w, h := float32(sw), float32(sh)
	if s.ax < 0 && s.bx < 0 || s.ay < 0 && s.by < 0 {
		return false
	}
	return !(s.ax > w && s.bx > w || s.ay > h && s.by > h)
}

// PlanPaths turns pawns into draw plans. toScreen maps world tile
// coordinates; pos is each pawn's drawn tile position. Pure: no pixels.
func PlanPaths(pawns []game.PawnView, selected uint64, hasSelected bool, zoom float64, sw, sh int,
	toScreen func(x, y float64) (float32, float32), pos func(game.PawnView) (float64, float64)) []PawnPathPlan {
	var out []PawnPathPlan
	for _, p := range pawns {
		if p.Hostile || p.Embodied || len(p.Path) == 0 {
			continue
		}
		pl := PawnPathPlan{id: uint64(p.ID), Drafted: p.Drafted, Selected: hasSelected && uint64(p.ID) == selected}
		cx, cy := pos(p)
		prevX, prevY := toScreen(cx+0.5, cy+0.5)
		lastX, lastY := prevX, prevY
		for _, c := range p.Path {
			nx, ny := toScreen(float64(c[0])+0.5, float64(c[1])+0.5)
			s := pathSeg{lastX, lastY, nx, ny}
			lastX, lastY = nx, ny
			if zoom < pathMinZoom || !segOnScreen(s, sw, sh) {
				continue
			}
			if !p.Drafted && !pl.Selected { // dotted: the first 60% of each cell step
				s.bx = s.ax + (s.bx-s.ax)*0.6
				s.by = s.ay + (s.by-s.ay)*0.6
			}
			pl.Segs = append(pl.Segs, s)
		}
		gx, gy := lastX, lastY
		if p.HasGoal {
			gx, gy = toScreen(float64(p.GoalX)+0.5, float64(p.GoalY)+0.5)
			pl.HasGoal, pl.gx, pl.gy = true, gx, gy
		}
		if p.HasDrop {
			pl.HasDrop = true
			pl.dx, pl.dy = toScreen(float64(p.DropX)+0.5, float64(p.DropY)+0.5)
		}
		if len(pl.Segs) == 0 && !pl.HasGoal && !pl.HasDrop {
			continue
		}
		if pl.HasGoal && !(gx >= -16 && gy >= -16 && gx <= float32(sw)+16 && gy <= float32(sh)+16) &&
			len(pl.Segs) == 0 && !pl.HasDrop {
			continue
		}
		out = append(out, pl)
	}
	return out
}

// DrawPaths draws every colonist's route: normal, then drafted, then the
// selected colonist last, one batched stroke per style. pos is where a
// pawn is drawn (its motion, render/motion).
func (o *Set) DrawPaths(v *render.View, screen *ebiten.Image, sw, sh int, tile float64, pos func(game.PawnView) (float64, float64)) {
	var sel uint64
	if v.HasSelect {
		sel = uint64(v.Selected)
	}
	plans := PlanPaths(v.Pawns, sel, v.HasSelect, v.Cam.Zoom, sw, sh, v.WorldToScreenF, pos)
	if len(plans) == 0 {
		return
	}
	th := PathStroke(v.Cam.Zoom)
	style := func(match func(PawnPathPlan) bool, w float32, c color.NRGBA) {
		var path vector.Path
		any := false
		for _, pl := range plans {
			if !match(pl) {
				continue
			}
			for _, s := range pl.Segs {
				path.MoveTo(s.ax, s.ay)
				path.LineTo(s.bx, s.by)
				any = true
			}
		}
		if any {
			vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: w}, &vector.DrawPathOptions{AntiAlias: true, ColorScale: colorScaleOf(c)})
		}
	}
	style(func(p PawnPathPlan) bool { return !p.Drafted && !p.Selected }, th, color.NRGBA{255, 255, 255, 140})
	style(func(p PawnPathPlan) bool { return p.Drafted && !p.Selected }, th*5/3, color.NRGBA{232, 90, 70, 217})
	style(func(p PawnPathPlan) bool { return p.Selected }, th*4/3, color.NRGBA{255, 255, 255, 235})
	rad := float32(max(3*camera.ScreenDev(), tile*0.16))
	for _, pl := range plans {
		c := color.NRGBA{255, 255, 255, 150}
		switch {
		case pl.Selected:
			c = color.NRGBA{255, 255, 255, 240}
		case pl.Drafted:
			c = color.NRGBA{232, 90, 70, 230}
		}
		if pl.HasGoal {
			if pl.Drafted {
				vector.FillCircle(screen, pl.gx, pl.gy, rad*0.8, c, true)
			}
			vector.StrokeCircle(screen, pl.gx, pl.gy, rad, th, c, true)
		}
		if pl.HasDrop {
			vector.StrokeRect(screen, pl.dx-rad, pl.dy-rad, rad*2, rad*2, th, c, false)
		}
	}
}

func colorScaleOf(c color.NRGBA) ebiten.ColorScale {
	var cs ebiten.ColorScale
	cs.ScaleWithColor(c)
	return cs
}

// PathStroke is the path line width: 1..3 points at the zoom, in physical
// pixels (x dev) so it keeps its apparent thickness.
func PathStroke(zoom float64) float32 {
	return float32(min(3, max(1, zoom*1.5)) * camera.ScreenDev())
}
