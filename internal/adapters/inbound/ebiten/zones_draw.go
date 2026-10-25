package ebiten

import (
	"image/color"
	"math"
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
	// The bounding box and the label anchor in tile coordinates
	// (zoneLabelAnchor).
	minX, minY, maxX, maxY int
	ax, ay                 float64
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
	g.minX, g.minY, g.maxX, g.maxY, g.ax, g.ay = zoneLabelAnchor(z.Cells)
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

// zoneStroke is the outline width at the current zoom, clamped 1..3 points
// and drawn in physical pixels (x dev) so it keeps its apparent thickness.
func zoneStroke(zoom float64) float32 {
	return float32(min(3, max(1, 2*zoom)) * screenDev())
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

// zoneLabelCap is the biggest a zone label may be with tiles tileL
// logical px wide: zoomed in (big tiles) a label stays modest, so it
// doesn't bury the stacks and crops of the zone it names; zoomed out it
// may grow to zoneLabelMax, so a field's name reads from afar (PLAY-017).
func zoneLabelCap(tileL float64) float64 {
	if tileL <= 0 {
		return zoneLabelMax
	}
	return min(zoneLabelMax, max(zoneLabelNear, zoneLabelCapArea/tileL))
}

// drawZoneLabels names every zone once (drawZoneLabel). It runs after the
// objects, so stacks and crops never cover a zone's name.
func (r *Renderer) drawZoneLabels(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	for _, z := range w.Zones {
		r.drawZoneLabel(screen, z, r.zoneGeomOf(z), sw, sh, tile)
	}
}

// drawZones fills every zone tile and outlines the outer edges; the
// labels come later (drawZoneLabels).
func (r *Renderer) drawZones(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	none := color.NRGBA{}
	for _, z := range w.Zones {
		key, fill := zoneSpriteKey(z.Kind)
		useArt := r.art != nil
		filled := useArt && r.zoneFill(screen, key, z.Cells, tile, sw, sh) // one batch for every cell
		for _, c := range z.Cells {
			if filled {
				break
			}
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
	}
}

// zoneFill draws the zone sprite key on every visible cell as one
// triangle batch: the quads drawSprite would draw one call each (an
// unlit overlay: the same geometry, source and filter), so a big zoomed-
// out colony's zones cost one call, not thousands. False when the
// sprite is missing (the caller falls back).
func (r *Renderer) zoneFill(screen *ebiten.Image, key string, cells [][2]int, tile float64, sw, sh int) bool {
	sp := r.art.sprite(key)
	if sp == nil {
		return false
	}
	img := r.art.image(key, color.NRGBA{})
	if img == nil {
		return false
	}
	b := img.Bounds()
	sx0, sy0, sx1, sy1 := float32(b.Min.X), float32(b.Min.Y), float32(b.Max.X), float32(b.Max.Y)
	l := &r.lod
	l.verts, l.idx = l.verts[:0], l.idx[:0]
	for _, c := range cells {
		px, py := r.worldToScreenF(float64(c[0]), float64(c[1]))
		x, y, s := artOrigin(sp.meta, float64(px), float64(py), tile)
		w, h := float64(sp.meta.Canvas[0])*s, float64(sp.meta.Canvas[1])*s
		if !visible(float32(x), float32(y), float32(w), float32(h), sw, sh) {
			continue
		}
		var geo ebiten.GeoM
		geo.Scale(s, s)
		geo.Translate(x, y)
		n := uint32(len(l.verts))
		for k := 0; k < 4; k++ {
			u, v := sx0, sy0
			if k&1 != 0 {
				u = sx1
			}
			if k&2 != 0 {
				v = sy1
			}
			dx, dy := geo.Apply(float64(u-sx0), float64(v-sy0))
			l.verts = append(l.verts, ebiten.Vertex{DstX: float32(dx), DstY: float32(dy), SrcX: u, SrcY: v,
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1})
		}
		l.idx = append(l.idx, n, n+1, n+2, n+1, n+3, n+2)
	}
	if len(l.idx) > 0 {
		screen.DrawTriangles32(l.verts, l.idx, img, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
	}
	return true
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
		dash := float32(max(4*screenDev(), tile*0.16))
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
	vector.StrokeLine(screen, sax, say, sbx, sby, float32(max(2*screenDev(), tile*0.05)), rail, true)
	post := color.RGBA{110, 74, 44, 240}
	ps := float32(max(3*screenDev(), tile*0.09))
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

// Zone label metrics (PLAY-017), in logical pixels.
const (
	zoneLabelMin  = 11.0 // the smallest label (tsSmall); ellipsized only here
	zoneLabelMax  = 40.0 // the biggest, zoomed out
	zoneLabelNear = 16.0 // the biggest when zoomed in (zoneLabelCap)
	// zoneLabelCapArea sets the zoom cap: cap = area / tile (logical px),
	// so the cap shrinks as tiles grow (zoneLabelCap).
	zoneLabelCapArea = 640.0
	zoneLabelFrac    = 0.45 // label size as a share of the zone's shorter side
	zoneLabelFit     = 0.9  // widest plate as a share of the zone's width
)

// zoneLabelPlateClr is the soft dark plate behind a zone label.
var zoneLabelPlateClr = color.RGBA{16, 12, 8, 150}

// zoneLabelAnchor is a zone's bounding box and where its label is
// centred, in tile coordinates (a tile spans x..x+1): the exact centre of
// a full rectangle, otherwise the centre of the zone cell nearest the
// bounding box centre (the first one in cell order on a tie), so an
// L-shaped zone keeps its label on the zone.
func zoneLabelAnchor(cells [][2]int) (minX, minY, maxX, maxY int, ax, ay float64) {
	if len(cells) == 0 {
		return
	}
	minX, minY, maxX, maxY = cells[0][0], cells[0][1], cells[0][0], cells[0][1]
	for _, c := range cells {
		minX, maxX = min(minX, c[0]), max(maxX, c[0])
		minY, maxY = min(minY, c[1]), max(maxY, c[1])
	}
	cx, cy := float64(minX+maxX+1)/2, float64(minY+maxY+1)/2
	if len(cells) == (maxX-minX+1)*(maxY-minY+1) {
		return minX, minY, maxX, maxY, cx, cy
	}
	best := math.Inf(1)
	for _, c := range cells {
		x, y := float64(c[0])+0.5, float64(c[1])+0.5
		if d := (x-cx)*(x-cx) + (y-cy)*(y-cy); d < best {
			best, ax, ay = d, x, y
		}
	}
	return minX, minY, maxX, maxY, ax, ay
}

// zoneLabelPad is the plate padding around a label of the given size.
func zoneLabelPad(size float64) (px, py float64) {
	return max(3, math.Round(size*0.3)), max(1, math.Round(size*0.1))
}

// zoneLabelPlan is a laid-out zone label, in logical pixels.
type zoneLabelPlan struct {
	ok     bool      // false: hidden
	text   string    // the label, ellipsized when it had to be
	style  textStyle // centre-aligned about tx
	plate  rect      // the dark plate behind the text
	tx, ty float64   // the text's top-centre
}

// zoneLabelLayout lays out a zone's label, centred on (cx, cy), for a
// zone whose bounding box is boxW x boxH on screen. Everything is in
// logical pixels.
//
// Size: 0.45 x the zone's shorter side, from 11 px to capSize
// (zoneLabelCap: smaller zoomed in) in half steps (the face cache is
// keyed on whole pixels). The plate then has to fit 90% of
// the zone's width, so the size shrinks as far as needed; at 11 px the
// text is ellipsized, and the label is hidden when not even a letter and
// the ellipsis fit, or the plate is taller than the zone.
func zoneLabelLayout(text string, boxW, boxH, cx, cy, capSize float64) zoneLabelPlan {
	if text == "" || boxW <= 0 || boxH <= 0 {
		return zoneLabelPlan{}
	}
	style := func(sz float64) textStyle { return textStyle{Size: sz, Align: alignCenter} }
	maxW := zoneLabelFit * boxW
	fits := func(sz float64) bool {
		px, _ := zoneLabelPad(sz)
		w, _ := measure(text, style(sz))
		return w+2*px <= maxW
	}
	want := math.Round(min(capSize, max(zoneLabelMin, zoneLabelFrac*min(boxW, boxH)))*2) / 2
	size := want
	if !fits(size) {
		// start from the width at 11 px scaled to the room, then settle on
		// the biggest half step that fits (two or three measures)
		w11, _ := measure(text, style(zoneLabelMin))
		est := maxW / (w11/zoneLabelMin + 0.6) // plate width per px of size
		size = max(zoneLabelMin, min(want, math.Floor(est*2)/2))
		for size < want && fits(size+0.5) {
			size += 0.5
		}
		for size > zoneLabelMin && !fits(size) {
			size -= 0.5
		}
	}
	st := style(size)
	px, py := zoneLabelPad(size)
	label := text
	if !fits(size) { // only possible at the 11 px floor
		if label = ellipsize(text, st, maxW-2*px); label == "…" {
			return zoneLabelPlan{}
		}
	}
	tw, _ := measure(label, st)
	w, h := tw+2*px, lineH(st)-2+2*py // lineH has 2 px of leading below the glyphs
	if h > boxH {
		return zoneLabelPlan{}
	}
	return zoneLabelPlan{ok: true, text: label, style: st,
		plate: rect{X: float32(cx - w/2), Y: float32(cy - h/2), W: float32(w), H: float32(h)},
		tx:    cx, ty: cy - h/2 + py}
}

// drawZoneLabel names a zone once, in the middle of it, on a dark plate.
// The label scales with the zone (zoneLabelLayout) and is anchored to the
// zone cell nearest its middle; layout is in logical pixels, from the
// physical on-screen box.
func (r *Renderer) drawZoneLabel(screen *ebiten.Image, z game.ZoneView, g *zoneGeom, sw, sh int, tile float64) {
	if len(z.Cells) == 0 {
		return
	}
	ox, oy := r.worldToScreenF(float64(g.minX), float64(g.minY))
	bw, bh := float64(g.maxX-g.minX+1)*tile, float64(g.maxY-g.minY+1)*tile
	if !visible(ox, oy, float32(bw), float32(bh), sw, sh) {
		return
	}
	ax, ay := r.worldToScreenF(g.ax, g.ay)
	k := 1 / uiScale()
	plan := zoneLabelLayout(zoneLabelText(z), bw*k, bh*k, float64(ax)*k, float64(ay)*k, zoneLabelCap(tile*k))
	if !plan.ok {
		return
	}
	s := uiScale()
	if pr := plan.plate; !visible(pr.X*float32(s), pr.Y*float32(s), pr.W*float32(s), pr.H*float32(s), sw, sh) {
		return
	}
	paintZoneLabelPlate(screen, plan.plate)
	worldText(screen, plan.text, plan.tx*s, plan.ty*s, plan.style) // physical anchor in, logical out
}

// paintZoneLabelPlate fills a zone label's plate (logical pixels).
func paintZoneLabelPlate(screen *ebiten.Image, p rect) {
	uiFillRect(screen, p.X, p.Y, p.W, p.H, zoneLabelPlateClr, false)
}
