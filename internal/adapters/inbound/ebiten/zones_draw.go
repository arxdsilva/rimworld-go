package ebiten

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Zone drawing (ZONE-001): every zone is an arbitrary tile set. Tiles get
// the kind's fill sprite; only the outer edges of the set are outlined, as
// continuous runs, so adjacent or merged tiles read as one zone.

// Zone edge bits: set where the neighbour on that side is not in the zone.
const (
	zoneN = 1
	zoneE = 2
	zoneS = 4
	zoneW = 8
)

// zoneEdgeMask reports which sides of (x, y) face outside the zone.
func zoneEdgeMask(cells map[[2]int]bool, x, y int) int {
	m := 0
	if !cells[[2]int{x, y - 1}] {
		m |= zoneN
	}
	if !cells[[2]int{x + 1, y}] {
		m |= zoneE
	}
	if !cells[[2]int{x, y + 1}] {
		m |= zoneS
	}
	if !cells[[2]int{x - 1, y}] {
		m |= zoneW
	}
	return m
}

// edgeRun is a straight stretch of a zone's outline in tile-corner
// coordinates (horizontal: y0 == y1, x0 < x1; vertical: x0 == x1,
// y0 < y1). nx, ny point into the zone. A convex end turns the outline
// inwards; a concave end continues it outwards.
type edgeRun struct {
	x0, y0, x1, y1         int
	nx, ny                 int
	startConvex, endConvex bool
}

// zoneEdgeRuns joins the consecutive outer edges of cells into runs.
// order lists the cells row-major (for deterministic output).
func zoneEdgeRuns(cells map[[2]int]bool, order [][2]int) []edgeRun {
	var out []edgeRun
	has := func(x, y, bit int) bool {
		return cells[[2]int{x, y}] && zoneEdgeMask(cells, x, y)&bit != 0
	}
	for _, c := range order {
		x, y := c[0], c[1]
		m := zoneEdgeMask(cells, x, y)
		// horizontal runs (north and south edges), started at their west end
		for _, side := range [2]int{zoneN, zoneS} {
			if m&side == 0 || has(x-1, y, side) {
				continue
			}
			e := x
			for has(e+1, y, side) {
				e++
			}
			ey, ny := y, 1
			if side == zoneS {
				ey, ny = y+1, -1
			}
			out = append(out, edgeRun{x0: x, y0: ey, x1: e + 1, y1: ey, nx: 0, ny: ny,
				startConvex: !cells[[2]int{x - 1, y}], endConvex: !cells[[2]int{e + 1, y}]})
		}
		// vertical runs (west and east edges), started at their north end
		for _, side := range [2]int{zoneW, zoneE} {
			if m&side == 0 || has(x, y-1, side) {
				continue
			}
			e := y
			for has(x, e+1, side) {
				e++
			}
			ex, nx := x, 1
			if side == zoneE {
				ex, nx = x+1, -1
			}
			out = append(out, edgeRun{x0: ex, y0: y, x1: ex, y1: e + 1, nx: nx, ny: 0,
				startConvex: !cells[[2]int{x, y - 1}], endConvex: !cells[[2]int{x, e + 1}]})
		}
	}
	return out
}

// zoneGeom is a zone's cached outline, rebuilt when its tiles change.
type zoneGeom struct {
	hash  uint64
	cells map[[2]int]bool
	runs  []edgeRun
}

func zoneHash(z game.ZoneView) uint64 {
	h := uint64(14695981039346656037)
	for _, c := range z.Cells {
		h = (h ^ uint64(uint32(c[0]))) * 1099511628211
		h = (h ^ uint64(uint32(c[1]))) * 1099511628211
	}
	return h ^ uint64(len(z.Cells))
}

func (r *Renderer) zoneGeomOf(z game.ZoneView) *zoneGeom {
	h := zoneHash(z)
	if g, ok := r.zoneGeoms[z.ID]; ok && g.hash == h {
		return g
	}
	g := &zoneGeom{hash: h, cells: make(map[[2]int]bool, len(z.Cells))}
	for _, c := range z.Cells {
		g.cells[c] = true
	}
	g.runs = zoneEdgeRuns(g.cells, z.Cells)
	if r.zoneGeoms == nil || len(r.zoneGeoms) > 256 {
		r.zoneGeoms = map[uint16]*zoneGeom{}
	}
	r.zoneGeoms[z.ID] = g
	return g
}

var (
	stockEdgeClr = color.NRGBA{0xdc, 0xc8, 0x50, 255}
	growEdgeClr  = color.NRGBA{R: 115, G: 199, B: 92, A: 255}
	stockFillClr = color.RGBA{R: 220, G: 200, B: 80, A: 70}
	growFillClr  = color.RGBA{R: 70, G: 150, B: 70, A: 70}
	penFillClr   = color.RGBA{R: 150, G: 104, B: 62, A: 60}
)

// zoneStroke is the outline width at the current zoom, clamped 1..3 px.
func zoneStroke(zoom float64) float32 {
	return float32(min(3, max(1, 2*zoom)))
}

func zoneSpriteKey(kind string) (key string, fill color.RGBA) {
	switch kind {
	case game.ZoneGrow:
		return "overlay/grow_zone", growFillClr
	case game.ZonePen:
		return "overlay/pen_zone", penFillClr
	}
	return "overlay/stockpile", stockFillClr
}

// drawZones fills every zone tile, outlines the outer edges and labels
// each zone once at its top-left tile.
func (r *Renderer) drawZones(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	none := color.NRGBA{}
	for _, z := range w.Zones {
		key, fill := zoneSpriteKey(z.Kind)
		useArt := r.art != nil
		for _, c := range z.Cells {
			if useArt {
				r.drawSprite(screen, key, none, float64(c[0]), float64(c[1]), tile, sw, sh)
				continue
			}
			sx, sy := r.worldToScreen(c[0], c[1])
			if visible(sx, sy, float32(tile), float32(tile), sw, sh) {
				vector.FillRect(screen, sx, sy, float32(tile), float32(tile), fill, false)
			}
		}
		g := r.zoneGeomOf(z)
		for _, run := range g.runs {
			r.drawZoneRun(screen, z.Kind, run, sw, sh, tile)
		}
		r.drawZoneLabel(screen, z, sw, sh, tile)
	}
}

func (r *Renderer) runVisible(run edgeRun, sw, sh int, pad float32) bool {
	ax, ay := r.worldToScreenF(float64(run.x0), float64(run.y0))
	bx, by := r.worldToScreenF(float64(run.x1), float64(run.y1))
	return bx+pad >= 0 && by+pad >= 0 && ax-pad < float32(sw) && ay-pad < float32(sh)
}

func (r *Renderer) drawZoneRun(screen *ebiten.Image, kind string, run edgeRun, sw, sh int, tile float64) {
	if !r.runVisible(run, sw, sh, 8) {
		return
	}
	if kind == game.ZonePen {
		r.drawFenceRun(screen, run, tile)
		return
	}
	th := zoneStroke(r.cam.zoom)
	ax, ay := r.worldToScreenF(float64(run.x0), float64(run.y0))
	bx, by := r.worldToScreenF(float64(run.x1), float64(run.y1))
	off := th / 2
	ax, ay = ax+float32(run.nx)*off, ay+float32(run.ny)*off
	bx, by = bx+float32(run.nx)*off, by+float32(run.ny)*off
	if kind == game.ZoneStockpile {
		dash := float32(max(4, tile*0.16))
		dashedRun(screen, ax, ay, bx, by, th, dash, dash*0.8, stockEdgeClr)
		return
	}
	// solid runs overshoot their ends by half a stroke so corners close
	if ax == bx {
		ay -= off
		by += off
	} else {
		ax -= off
		bx += off
	}
	vector.StrokeLine(screen, ax, ay, bx, by, th, growEdgeClr, false)
}

// dashedRun draws an axis-aligned dashed line with filled rectangles.
func dashedRun(screen *ebiten.Image, ax, ay, bx, by, th, dash, gap float32, c color.Color) {
	horizontal := ay == by
	length := bx - ax
	if !horizontal {
		length = by - ay
	}
	for d := float32(0); d < length; d += dash + gap {
		l := min(dash, length-d)
		if horizontal {
			vector.FillRect(screen, ax+d, ay-th/2, l, th, c, false)
		} else {
			vector.FillRect(screen, ax-th/2, ay+d, th, l, c, false)
		}
	}
}

// fenceInset is how far (in tiles) a fence stands inside its tile edge.
const fenceInset = 0.08

// drawFenceRun draws one pen fence run: a rail with a post at every tile
// boundary along it and at its ends (where it turns a corner).
func (r *Renderer) drawFenceRun(screen *ebiten.Image, run edgeRun, tile float64) {
	ex0, ex1 := fenceInset, -fenceInset // convex ends pull in, concave ends push out
	if !run.startConvex {
		ex0 = -fenceInset
	}
	if !run.endConvex {
		ex1 = fenceInset
	}
	ax, ay := float64(run.x0), float64(run.y0)
	bx, by := float64(run.x1), float64(run.y1)
	ox, oy := float64(run.nx)*fenceInset, float64(run.ny)*fenceInset
	if run.y0 == run.y1 {
		ax, bx = ax+ex0, bx+ex1
	} else {
		ay, by = ay+ex0, by+ex1
	}
	ax, ay, bx, by = ax+ox, ay+oy, bx+ox, by+oy
	sax, say := r.worldToScreenF(ax, ay)
	sbx, sby := r.worldToScreenF(bx, by)
	rail := color.RGBA{150, 104, 62, 220}
	vector.StrokeLine(screen, sax, say, sbx, sby, float32(max(2, tile*0.05)), rail, true)
	post := color.RGBA{110, 74, 44, 240}
	ps := float32(max(3, tile*0.09))
	dot := func(x, y float32) { vector.FillRect(screen, x-ps/2, y-ps/2, ps, ps, post, false) }
	dot(sax, say)
	dot(sbx, sby)
	n := run.x1 - run.x0
	if run.x0 == run.x1 {
		n = run.y1 - run.y0
	}
	for i := 1; i < n; i++ {
		if run.y0 == run.y1 {
			x, y := r.worldToScreenF(float64(run.x0+i), ay)
			dot(x, y)
		} else {
			x, y := r.worldToScreenF(ax, float64(run.y0+i))
			dot(x, y)
		}
	}
}

// zoneLabelText is a zone's one-per-zone map label.
func zoneLabelText(z game.ZoneView) string {
	switch z.Kind {
	case game.ZoneGrow:
		if z.Crop != "" {
			return strings.ToUpper(z.Crop[:1]) + z.Crop[1:]
		}
		return "Growing"
	case game.ZonePen:
		return "Pen"
	}
	return "Stockpile"
}

func (r *Renderer) drawZoneLabel(screen *ebiten.Image, z game.ZoneView, sw, sh int, tile float64) {
	if len(z.Cells) == 0 || tile < 40 {
		return
	}
	sx, sy := r.worldToScreen(z.Cells[0][0], z.Cells[0][1])
	if !visible(sx, sy, float32(tile), float32(tile), sw, sh) {
		return
	}
	lbl := ellipsize(zoneLabelText(z), tsSmall, tile*float64(max(1, z.MaxX-z.MinX+1))-8)
	drawText(screen, lbl, float64(sx)+5, float64(sy)+4, withColor(tsSmall, color.RGBA{0, 0, 0, 170})) // shadow
	drawText(screen, lbl, float64(sx)+4, float64(sy)+3, tsSmall)
}
