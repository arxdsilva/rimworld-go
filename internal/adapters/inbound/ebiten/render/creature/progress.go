package creature

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// In-world work progress (ART-027): with no tool drawn, a small bar at
// the work spot shows how far the job is. It reads PawnView.ActionProgress
// (0..1, -1 unknown) and is drawn in the overlay pass, over the objects.
// It sits along the work tile's bottom edge, under the worker's feet: a
// colonist mostly stands on or beside the tile it works, and a bar at the
// tile's top would cross the worker's neck.

// Work bar geometry, tiles, and the zoom it needs.
const (
	WorkBarW       = 0.6
	workBarH       = 0.06
	workBarTop     = 0.82 // the bar's top from the work tile's top edge
	WorkBarMinTile = 24.0 // physical px per tile below which no bar is drawn
)

// workBarOutline is the bar's dark edge.
var workBarOutline = color.RGBA{R: 0x0E, G: 0x0F, B: 0x12, A: 0xD0}

// noWorkBar are the work-table actions that show no bar: a meal and a
// game are not work, and a research session runs on (the Research tab
// shows the project's progress).
var noWorkBar = map[string]bool{game.ActionEat: true, game.ActionHorseshoes: true, game.ActionResearch: true}

// workBar is one bar to draw: the work tile and its progress.
type workBar struct {
	x, y int
	k    float64
}

// workBars lists the bars for these pawns: one per work tile (the most
// advanced when several colonists share it), for colonists standing at a
// job whose progress is known and under way.
func workBars(pawns []game.PawnView, dst []workBar) []workBar {
	for i := range pawns {
		p := &pawns[i]
		if p.Hostile || p.Lying || p.CarriedBy != 0 || p.ActionProgress <= 0 || p.ActionProgress >= 1 {
			continue
		}
		act, _, ok := ActionOf(*p)
		if !ok || noWorkBar[act] {
			continue
		}
		dup := false
		for j := range dst {
			if dst[j].x == p.ActX && dst[j].y == p.ActY {
				dst[j].k = max(dst[j].k, p.ActionProgress)
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, workBar{x: p.ActX, y: p.ActY, k: p.ActionProgress})
		}
	}
	return dst
}

// WorkBarRect is a bar's fill area on screen, physical px, and its
// progress.
type WorkBarRect struct {
	X, Y, W, H float32
	K          float64
}

// WorkBarRects places the bars for these pawns on screen; none when zoomed
// out below workBarMinTile or off view.
func (c *Set) WorkBarRects(v *render.View, cull *Cull, pawns []game.PawnView, tile float64, dst []WorkBarRect) []WorkBarRect {
	if tile < WorkBarMinTile {
		return dst
	}
	var buf [16]workBar
	for _, b := range workBars(pawns, buf[:0]) {
		if !cull.InView(b.x, b.y, 1, 1) {
			continue
		}
		sx, sy := v.WorldToScreenF(float64(b.x)+0.5-WorkBarW/2, float64(b.y)+workBarTop)
		dst = append(dst, WorkBarRect{X: sx, Y: sy, W: float32(WorkBarW * tile), H: max(float32(workBarH*tile), 3), K: b.k})
	}
	return dst
}

// DrawWorkProgress draws the work bars at the work spots: the accent
// fill on the slider track, with a dark edge.
func (c *Set) DrawWorkProgress(v *render.View, cull *Cull, screen *ebiten.Image, pawns []game.PawnView, tile float64) {
	var buf [16]WorkBarRect
	for _, b := range c.WorkBarRects(v, cull, pawns, tile, buf[:0]) {
		vector.FillRect(screen, b.X-1, b.Y-1, b.W+2, b.H+2, workBarOutline, false)
		vector.FillRect(screen, b.X, b.Y, b.W, b.H, uikit.SliderTrack, false)
		vector.FillRect(screen, b.X, b.Y, b.W*float32(b.K), b.H, uikit.Accent, false)
	}
}
