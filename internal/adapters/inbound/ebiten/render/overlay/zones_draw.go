package overlay

import (
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// Zone drawing (ZONE-001): every zone is an arbitrary tile set. Tiles get
// the kind's fill sprite; only the outer edges of the set are outlined, as
// continuous runs, so adjacent or merged tiles read as one zone.

// Zone edge bits: set where the neighbour on that side is not in the zone.
const (
	ZoneN = 1
	ZoneE = 2
	ZoneS = 4
	ZoneW = 8
)

// ZoneEdgeMask reports which sides of (x, y) face outside the zone.
func ZoneEdgeMask(cells map[[2]int]bool, x, y int) int {
	m := 0
	if !cells[[2]int{x, y - 1}] {
		m |= ZoneN
	}
	if !cells[[2]int{x + 1, y}] {
		m |= ZoneE
	}
	if !cells[[2]int{x, y + 1}] {
		m |= ZoneS
	}
	if !cells[[2]int{x - 1, y}] {
		m |= ZoneW
	}
	return m
}

// EdgeRun is a straight stretch of a zone's outline in tile-corner
// coordinates (horizontal: y0 == y1, x0 < x1; vertical: x0 == x1,
// y0 < y1). nx, ny point into the zone. A convex end turns the outline
// inwards; a concave end continues it outwards.
type EdgeRun struct {
	X0, Y0, X1, Y1         int
	nx, Ny                 int
	StartConvex, EndConvex bool
}

// ZoneEdgeRuns joins the consecutive outer edges of cells into runs.
// order lists the cells row-major (for deterministic output).
func ZoneEdgeRuns(cells map[[2]int]bool, order [][2]int) []EdgeRun {
	var out []EdgeRun
	has := func(x, y, bit int) bool {
		return cells[[2]int{x, y}] && ZoneEdgeMask(cells, x, y)&bit != 0
	}
	for _, c := range order {
		x, y := c[0], c[1]
		m := ZoneEdgeMask(cells, x, y)
		// horizontal runs (north and south edges), started at their west end
		for _, side := range [2]int{ZoneN, ZoneS} {
			if m&side == 0 || has(x-1, y, side) {
				continue
			}
			e := x
			for has(e+1, y, side) {
				e++
			}
			ey, ny := y, 1
			if side == ZoneS {
				ey, ny = y+1, -1
			}
			out = append(out, EdgeRun{X0: x, Y0: ey, X1: e + 1, Y1: ey, nx: 0, Ny: ny,
				StartConvex: !cells[[2]int{x - 1, y}], EndConvex: !cells[[2]int{e + 1, y}]})
		}
		// vertical runs (west and east edges), started at their north end
		for _, side := range [2]int{ZoneW, ZoneE} {
			if m&side == 0 || has(x, y-1, side) {
				continue
			}
			e := y
			for has(x, e+1, side) {
				e++
			}
			ex, nx := x, 1
			if side == ZoneE {
				ex, nx = x+1, -1
			}
			out = append(out, EdgeRun{X0: ex, Y0: y, X1: ex, Y1: e + 1, nx: nx, Ny: 0,
				StartConvex: !cells[[2]int{x, y - 1}], EndConvex: !cells[[2]int{x, e + 1}]})
		}
	}
	return out
}

// ZoneGeom is a zone's cached outline, rebuilt when its tiles change.
type ZoneGeom struct {
	hash  uint64
	Cells map[[2]int]bool
	Runs  []EdgeRun
	// The bounding box and the label anchor in tile coordinates
	// (ZoneLabelAnchor).
	MinX, MinY, MaxX, MaxY int
	Ax, Ay                 float64
}

func zoneHash(z game.ZoneView) uint64 {
	h := uint64(14695981039346656037)
	for _, c := range z.Cells {
		h = (h ^ uint64(uint32(c[0]))) * 1099511628211
		h = (h ^ uint64(uint32(c[1]))) * 1099511628211
	}
	return h ^ uint64(len(z.Cells))
}

// ZoneGeomOf is zone z's outline, rebuilt only when its cells change.
func (o *Zones) ZoneGeomOf(z game.ZoneView) *ZoneGeom {
	h := zoneHash(z)
	if g, ok := o.geoms[z.ID]; ok && g.hash == h {
		return g
	}
	g := &ZoneGeom{hash: h, Cells: make(map[[2]int]bool, len(z.Cells))}
	for _, c := range z.Cells {
		g.Cells[c] = true
	}
	g.Runs = ZoneEdgeRuns(g.Cells, z.Cells)
	g.MinX, g.MinY, g.MaxX, g.MaxY, g.Ax, g.Ay = ZoneLabelAnchor(z.Cells)
	if o.geoms == nil || len(o.geoms) > 256 {
		o.geoms = map[uint16]*ZoneGeom{}
	}
	o.geoms[z.ID] = g
	return g
}

var (
	StockEdgeClr = color.RGBA{0xdc, 0xc8, 0x50, 255}
	growEdgeClr  = color.RGBA{R: 115, G: 199, B: 92, A: 255}
	stockFillClr = color.RGBA{R: 220, G: 200, B: 80, A: 70}
	growFillClr  = color.RGBA{R: 70, G: 150, B: 70, A: 70}
	penFillClr   = color.RGBA{R: 150, G: 104, B: 62, A: 60}
)

// ZoneStroke is the outline width at the current zoom, clamped 1..3 points
// and drawn in physical pixels (x dev) so it keeps its apparent thickness.
func ZoneStroke(zoom float64) float32 {
	return float32(min(3, max(1, 2*zoom)) * camera.ScreenDev())
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

// ZoneLabelCap is the biggest a zone label may be with tiles tileL
// logical px wide: zoomed in (big tiles) a label stays modest, so it
// doesn't bury the stacks and crops of the zone it names; zoomed out it
// may grow to ZoneLabelMax, so a field's name reads from afar (PLAY-017).
func ZoneLabelCap(tileL float64) float64 {
	if tileL <= 0 {
		return ZoneLabelMax
	}
	return min(ZoneLabelMax, max(ZoneLabelNear, zoneLabelCapArea/tileL))
}

// zoneShelf is a shelf building's own storage zone (PRD-4): no overlay.
const zoneShelf = "shelf"

// DrawZoneLabels names every zone once (DrawZoneLabel). It runs after the
// objects, so stacks and crops never cover a zone's name.
func (o *Zones) DrawZoneLabels(v *render.View, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	for _, z := range w.Zones {
		if o.Hide || z.Hidden || z.Kind == zoneShelf {
			continue // a hidden zone shows no name (while selected, drawZoneSelection)
		}
		o.DrawZoneLabel(v, screen, z, o.ZoneGeomOf(z), sw, sh, tile)
	}
}

// DrawZones fills every zone tile and outlines the outer edges; the
// labels come later (DrawZoneLabels). A growing zone leaves its fill off
// the cells holding the plants it grows (PLAY-020): the furrows and the
// plants show there, and far out, where plants are baked under the zones,
// the fill would cover them.
func (o *Zones) DrawZones(v *render.View, ps sprite.Pass, ch *Chunks, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	none := color.NRGBA{}
	planted := o.zonePlantCells(v, ch, &w)
	for _, z := range w.Zones {
		if o.Hide || z.Hidden || z.Kind == zoneShelf {
			continue // the zones overlay is off; a shelf's zone is never drawn; a hidden zone draws no fill or outline (while selected, drawZoneSelection)
		}
		key, fill := zoneSpriteKey(z.Kind)
		var skip map[[2]int]bool
		if z.Kind == game.ZoneGrow {
			skip = planted
		}
		useArt := v.Art != nil
		filled := useArt && o.zoneFill(v, screen, key, z.Cells, skip, tile, sw, sh) // one batch for every cell
		for _, c := range z.Cells {
			if filled {
				break
			}
			if skip[c] {
				continue
			}
			if useArt {
				o.P.Draw(v, ps, screen, key, none, float64(c[0]), float64(c[1]), tile, sw, sh)
				continue
			}
			sx, sy := v.WorldToScreen(c[0], c[1])
			if sprite.Visible(sx, sy, float32(tile), float32(tile), sw, sh) {
				vector.FillRect(screen, sx, sy, float32(tile), float32(tile), fill, false)
			}
		}
		g := o.ZoneGeomOf(z)
		for _, run := range g.Runs {
			if z.Kind == game.ZoneStockpile || z.Kind == game.ZoneDump {
				o.drawPaletteRun(v, run, ZoneColour(z.ID), sw, sh, tile) // each storage zone in its own colour
				continue
			}
			o.DrawZoneRun(v, &o.Batch, z.Kind, run, sw, sh, tile)
		}
	}
	// every outline of every zone in one draw, over all the fills
	o.Batch.Flush(screen, o.whiteImg(), ebiten.Blend{})
}

// ZonePalette is the colour of each zone : the zone's id picks the entry, so a zone keeps its colour for
// as long as it exists.
var ZonePalette = [...]color.RGBA{
	{R: 230, G: 90, B: 80, A: 255}, {R: 240, G: 170, B: 60, A: 255}, {R: 220, G: 210, B: 70, A: 255}, {R: 110, G: 200, B: 90, A: 255},
	{R: 70, G: 190, B: 190, A: 255}, {R: 90, G: 140, B: 240, A: 255}, {R: 160, G: 110, B: 230, A: 255}, {R: 230, G: 110, B: 190, A: 255},
}

// ZoneColour is zone id's overlay colour (opaque; the fill uses ZoneFillAlpha).
func ZoneColour(id uint16) color.RGBA { return ZonePalette[int(id)%len(ZonePalette)] }

// drawPaletteRun is DrawZoneRun's dashed storage outline in colour c.
func (o *Zones) drawPaletteRun(v *render.View, run EdgeRun, c color.RGBA, sw, sh int, tile float64) {
	if !runVisible(v, run, sw, sh, 8) {
		return
	}
	th := ZoneStroke(v.Cam.Zoom)
	ax, ay := v.WorldToScreenF(float64(run.X0), float64(run.Y0))
	bx, by := v.WorldToScreenF(float64(run.X1), float64(run.Y1))
	off := th / 2
	ax, ay = ax+float32(run.nx)*off, ay+float32(run.Ny)*off
	bx, by = bx+float32(run.nx)*off, by+float32(run.Ny)*off
	dash := float32(max(4*camera.ScreenDev(), tile*0.16))
	DashedRun(&o.Batch, ax, ay, bx, by, th, dash, dash*0.8, c, sw, sh)
}

// zoneFill draws the zone sprite key on every visible cell not in skip as
// one triangle batch: the quads drawSprite would draw one call each (an
// unlit overlay: the same geometry, source and filter), so a big zoomed-
// out colony's zones cost one call, not thousands. False when the
// sprite is missing (the caller falls back).
func (o *Zones) zoneFill(v *render.View, screen *ebiten.Image, key string, cells [][2]int, skip map[[2]int]bool, tile float64, sw, sh int) bool {
	sp := v.Art.Sprite(key)
	if sp == nil {
		return false
	}
	img := v.Art.Image(key, color.NRGBA{})
	if img == nil {
		return false
	}
	b := img.Bounds()
	sx0, sy0, sx1, sy1 := float32(b.Min.X), float32(b.Min.Y), float32(b.Max.X), float32(b.Max.Y)
	verts, idx := o.verts[:0], o.idx[:0]
	for _, c := range cells {
		if skip[c] {
			continue
		}
		px, py := v.WorldToScreenF(float64(c[0]), float64(c[1]))
		x, y, s := art.Origin(sp.Meta, float64(px), float64(py), tile)
		w, h := float64(sp.Meta.Canvas[0])*s, float64(sp.Meta.Canvas[1])*s
		if !sprite.Visible(float32(x), float32(y), float32(w), float32(h), sw, sh) {
			continue
		}
		var geo ebiten.GeoM
		geo.Scale(s, s)
		geo.Translate(x, y)
		n := uint32(len(verts))
		for k := 0; k < 4; k++ {
			u, v := sx0, sy0
			if k&1 != 0 {
				u = sx1
			}
			if k&2 != 0 {
				v = sy1
			}
			dx, dy := geo.Apply(float64(u-sx0), float64(v-sy0))
			verts = append(verts, ebiten.Vertex{DstX: float32(dx), DstY: float32(dy), SrcX: u, SrcY: v,
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1})
		}
		idx = append(idx, n, n+1, n+2, n+1, n+3, n+2)
	}
	o.verts, o.idx = verts, idx
	if len(idx) > 0 {
		screen.DrawTriangles32(verts, idx, img, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear})
	}
	return true
}

// FillQuads is how many quads the last zone fill batched (tests).
func (o *Zones) FillQuads() int { return len(o.idx) / 6 }

// zonePlantCells is this frame's set of cells holding the plant their
// growing zone grows (its crop, or any tree in a tree zone), from the
// chunk lists in view, or every plant of w when w is not the frame's
// world (tests). Nil when there is no growing zone.
func (o *Zones) zonePlantCells(v *render.View, ch *Chunks, w *game.WorldView) map[[2]int]bool {
	grow := false
	for i := range w.Zones {
		grow = grow || w.Zones[i].Kind == game.ZoneGrow
	}
	if !grow {
		return nil
	}
	o.plants = resetCells(o.plants)
	one := func(p *game.PlantView) {
		if p.Zone != "" && (p.Species == p.Zone || p.Zone == "tree" && sprite.IsTreePlant(p.Species)) {
			o.plants[[2]int{p.X, p.Y}] = true
		}
	}
	if frameLists(v, ch, w) {
		for _, cv := range ch.Views {
			for _, i := range cv.Plants {
				if int(i) < len(w.Plants) {
					one(&w.Plants[i])
				}
			}
		}
	} else {
		for i := range w.Plants {
			one(&w.Plants[i])
		}
	}
	return o.plants
}

func runVisible(v *render.View, run EdgeRun, sw, sh int, pad float32) bool {
	ax, ay := v.WorldToScreenF(float64(run.X0), float64(run.Y0))
	bx, by := v.WorldToScreenF(float64(run.X1), float64(run.Y1))
	return bx+pad >= 0 && by+pad >= 0 && ax-pad < float32(sw) && ay-pad < float32(sh)
}

// DrawZoneRun appends one outline run to b (flushed once by DrawZones).
func (o *Zones) DrawZoneRun(v *render.View, b *gfx.TriBatch, kind string, run EdgeRun, sw, sh int, tile float64) {
	if !runVisible(v, run, sw, sh, 8) {
		return
	}
	if kind == game.ZonePen {
		o.DrawFenceRun(v, b, run, tile)
		return
	}
	th := ZoneStroke(v.Cam.Zoom)
	ax, ay := v.WorldToScreenF(float64(run.X0), float64(run.Y0))
	bx, by := v.WorldToScreenF(float64(run.X1), float64(run.Y1))
	off := th / 2
	ax, ay = ax+float32(run.nx)*off, ay+float32(run.Ny)*off
	bx, by = bx+float32(run.nx)*off, by+float32(run.Ny)*off
	if kind == game.ZoneStockpile || kind == game.ZoneDump {
		dash := float32(max(4*camera.ScreenDev(), tile*0.16))
		DashedRun(b, ax, ay, bx, by, th, dash, dash*0.8, StockEdgeClr, sw, sh)
		return
	}
	// solid runs overshoot their ends by half a stroke so corners close
	if ax == bx {
		ay -= off
		by += off
		b.Rect(float64(ax-th/2), float64(ay), float64(th), float64(by-ay), growEdgeClr, 1, 1, 1, 1)
	} else {
		ax -= off
		bx += off
		b.Rect(float64(ax), float64(ay-th/2), float64(bx-ax), float64(th), growEdgeClr, 1, 1, 1, 1)
	}
}

var (
	zoneSelEdgeClr = color.NRGBA{R: 255, G: 238, B: 140, A: 255}
	zoneSelFillClr = color.NRGBA{R: 255, G: 255, B: 255, A: 34}
)

// DrawZoneHighlight marks zone z as selected (PLAY-020): a brighter fill
// on its tiles (a growing zone's own plants stay uncovered, as in
// DrawZones) and a heavier, brighter outline, whatever its kind. It draws
// over a hidden zone too, which is how a hidden zone shows while selected.
func (o *Zones) DrawZoneHighlight(v *render.View, ch *Chunks, screen *ebiten.Image, w game.WorldView, z game.ZoneView, sw, sh int, tile float64) {
	var skip map[[2]int]bool
	if z.Kind == game.ZoneGrow {
		skip = o.zonePlantCells(v, ch, &w)
	}
	for _, c := range z.Cells {
		if skip[c] {
			continue
		}
		if sx, sy := v.WorldToScreen(c[0], c[1]); sprite.Visible(sx, sy, float32(tile), float32(tile), sw, sh) {
			vector.FillRect(screen, sx, sy, float32(tile), float32(tile), zoneSelFillClr, false)
		}
	}
	th := ZoneStroke(v.Cam.Zoom) * 1.5
	off := th / 2
	for _, run := range o.ZoneGeomOf(z).Runs {
		if !runVisible(v, run, sw, sh, 8) {
			continue
		}
		ax, ay := v.WorldToScreenF(float64(run.X0), float64(run.Y0))
		bx, by := v.WorldToScreenF(float64(run.X1), float64(run.Y1))
		ax, ay = ax+float32(run.nx)*off, ay+float32(run.Ny)*off
		bx, by = bx+float32(run.nx)*off, by+float32(run.Ny)*off
		if ax == bx { // solid runs overshoot their ends by half a stroke so corners close
			ay -= off
			by += off
		} else {
			ax -= off
			bx += off
		}
		vector.StrokeLine(screen, ax, ay, bx, by, th, zoneSelEdgeClr, false)
	}
}

// DashedRun appends an axis-aligned dashed line to b as quads. Only the
// dashes on screen are emitted: the walk starts at the first on-screen
// dash, so a run far longer than the view costs its visible part.
func DashedRun(b *gfx.TriBatch, ax, ay, bx, by, th, dash, gap float32, c color.RGBA, sw, sh int) {
	horizontal := ay == by
	length, a0, lo, hi := bx-ax, ax, float32(0), float32(sw)
	if !horizontal {
		length, a0, hi = by-ay, ay, float32(sh)
	}
	period := dash + gap
	var d float32
	if start := lo - a0 - dash; start > 0 {
		d = float32(math.Floor(float64(start/period))) * period
	}
	for ; d < length && a0+d < hi; d += period {
		l := min(dash, length-d)
		if horizontal {
			if !sprite.Visible(ax+d, ay-th/2, l, th, sw, sh) {
				continue
			}
			b.Rect(float64(ax+d), float64(ay-th/2), float64(l), float64(th), c, 1, 1, 1, 1)
		} else {
			if !sprite.Visible(ax-th/2, ay+d, th, l, sw, sh) {
				continue
			}
			b.Rect(float64(ax-th/2), float64(ay+d), float64(th), float64(l), c, 1, 1, 1, 1)
		}
	}
}

// fenceInset is how far (in tiles) a fence stands inside its tile edge.
const fenceInset = 0.08

// DrawFenceRun draws one pen fence run: a rail with a post at every tile
// boundary along it and at its ends (where it turns a corner).
func (o *Zones) DrawFenceRun(v *render.View, b *gfx.TriBatch, run EdgeRun, tile float64) {
	ex0, ex1 := fenceInset, -fenceInset // convex ends pull in, concave ends push out
	if !run.StartConvex {
		ex0 = -fenceInset
	}
	if !run.EndConvex {
		ex1 = fenceInset
	}
	ax, ay := float64(run.X0), float64(run.Y0)
	bx, by := float64(run.X1), float64(run.Y1)
	ox, oy := float64(run.nx)*fenceInset, float64(run.Ny)*fenceInset
	if run.Y0 == run.Y1 {
		ax, bx = ax+ex0, bx+ex1
	} else {
		ay, by = ay+ex0, by+ex1
	}
	ax, ay, bx, by = ax+ox, ay+oy, bx+ox, by+oy
	sax, say := v.WorldToScreenF(ax, ay)
	sbx, sby := v.WorldToScreenF(bx, by)
	rail := color.RGBA{150, 104, 62, 255}
	b.Line(float64(sax), float64(say), float64(sbx), float64(sby), max(2*camera.ScreenDev(), tile*0.05), rail, 220.0/255)
	post := color.RGBA{110, 74, 44, 255}
	ps := float32(max(3*camera.ScreenDev(), tile*0.09))
	dot := func(x, y float32) {
		b.Rect(float64(x-ps/2), float64(y-ps/2), float64(ps), float64(ps), post, 240.0/255, 240.0/255, 240.0/255, 240.0/255)
	}
	dot(sax, say)
	dot(sbx, sby)
	n := run.X1 - run.X0
	if run.X0 == run.X1 {
		n = run.Y1 - run.Y0
	}
	for i := 1; i < n; i++ {
		if run.Y0 == run.Y1 {
			x, y := v.WorldToScreenF(float64(run.X0+i), ay)
			dot(x, y)
		} else {
			x, y := v.WorldToScreenF(ax, float64(run.Y0+i))
			dot(x, y)
		}
	}
}

// ZoneLabelText is a zone's one-per-zone map label: the player's name for
// it, else its crop or kind.
func ZoneLabelText(z game.ZoneView) string {
	if z.Name != "" {
		return z.Name
	}
	switch z.Kind {
	case game.ZoneGrow:
		if z.Crop != "" {
			return strings.ToUpper(z.Crop[:1]) + z.Crop[1:]
		}
		return "Growing"
	case game.ZonePen:
		return "Pen"
	case game.ZoneDump:
		return "Dumping"
	}
	return "Stockpile"
}

// Zone label metrics (PLAY-017), in logical pixels.
const (
	zoneLabelMin  = 11.0 // the smallest label (tsSmall); ellipsized only here
	ZoneLabelMax  = 40.0 // the biggest, zoomed out
	ZoneLabelNear = 16.0 // the biggest when zoomed in (ZoneLabelCap)
	// zoneLabelCapArea sets the zoom cap: cap = area / tile (logical px),
	// so the cap shrinks as tiles grow (ZoneLabelCap).
	zoneLabelCapArea = 640.0
	zoneLabelFrac    = 0.45 // label size as a share of the zone's shorter side
	zoneLabelFit     = 0.9  // widest plate as a share of the zone's width
)

// zoneLabelPlateClr is the soft dark plate behind a zone label.
var zoneLabelPlateClr = uikit.Shade(150)

// ZoneLabelAnchor is a zone's bounding box and where its label is
// centred, in tile coordinates (a tile spans x..x+1): the exact centre of
// a full rectangle, otherwise the centre of the zone cell nearest the
// bounding box centre (the first one in cell order on a tie), so an
// L-shaped zone keeps its label on the zone.
func ZoneLabelAnchor(cells [][2]int) (minX, minY, maxX, maxY int, ax, ay float64) {
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

// ZoneLabelPad is the plate padding around a label of the given size.
func ZoneLabelPad(size float64) (px, py float64) {
	return max(3, math.Round(size*0.3)), max(1, math.Round(size*0.1))
}

// ZoneLabelPlan is a laid-out zone label, in logical pixels.
type ZoneLabelPlan struct {
	Ok     bool          // false: hidden
	Text   string        // the label, ellipsized when it had to be
	Style  gfx.TextStyle // centre-aligned about tx
	Plate  gfx.Rect      // the dark plate behind the text
	tx, ty float64       // the text's top-centre
}

// ZoneLabelLayout lays out a zone's label, centred on (cx, cy), for a
// zone whose bounding box is boxW x boxH on screen. Everything is in
// logical pixels.
//
// Size: 0.45 x the zone's shorter side, from 11 px to capSize
// (ZoneLabelCap: smaller zoomed in) in half steps (the face cache is
// keyed on whole pixels). The plate then has to fit 90% of
// the zone's width, so the size shrinks as far as needed; at 11 px the
// text is ellipsized, and the label is hidden when not even a letter and
// the ellipsis fit, or the plate is taller than the zone.
func ZoneLabelLayout(text string, boxW, boxH, cx, cy, capSize float64) ZoneLabelPlan {
	if text == "" || boxW <= 0 || boxH <= 0 {
		return ZoneLabelPlan{}
	}
	style := func(sz float64) gfx.TextStyle { return gfx.TextStyle{Size: sz, Align: gfx.AlignCenter} }
	maxW := zoneLabelFit * boxW
	fits := func(sz float64) bool {
		px, _ := ZoneLabelPad(sz)
		w, _ := gfx.Measure(text, style(sz))
		return w+2*px <= maxW
	}
	want := math.Round(min(capSize, max(zoneLabelMin, zoneLabelFrac*min(boxW, boxH)))*2) / 2
	size := want
	if !fits(size) {
		// start from the width at 11 px scaled to the room, then settle on
		// the biggest half step that fits (two or three measures)
		w11, _ := gfx.Measure(text, style(zoneLabelMin))
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
	px, py := ZoneLabelPad(size)
	label := text
	if !fits(size) { // only possible at the 11 px floor
		if label = gfx.Ellipsize(text, st, maxW-2*px); label == "…" {
			return ZoneLabelPlan{}
		}
	}
	tw, _ := gfx.Measure(label, st)
	w, h := tw+2*px, gfx.LineH(st)-2+2*py // lineH has 2 px of leading below the glyphs
	if h > boxH {
		return ZoneLabelPlan{}
	}
	return ZoneLabelPlan{Ok: true, Text: label, Style: st,
		Plate: gfx.Rect{X: float32(cx - w/2), Y: float32(cy - h/2), W: float32(w), H: float32(h)},
		tx:    cx, ty: cy - h/2 + py}
}

// ZoneLabelPlanFor lays out zone z's label for tiles tile physical pixels
// wide: the plan (logical pixels) and the zone's bounding box on screen
// (physical origin and size).
func (o *Zones) ZoneLabelPlanFor(v *render.View, z game.ZoneView, g *ZoneGeom, tile float64) (plan ZoneLabelPlan, ox, oy float32, bw, bh float64) {
	ox, oy = v.WorldToScreenF(float64(g.MinX), float64(g.MinY))
	bw, bh = float64(g.MaxX-g.MinX+1)*tile, float64(g.MaxY-g.MinY+1)*tile
	ax, ay := v.WorldToScreenF(g.Ax, g.Ay)
	k := 1 / gfx.MapUIScale() // also from Update (ZoneLabelAt), outside the world pass
	return ZoneLabelLayout(ZoneLabelText(z), bw*k, bh*k, float64(ax)*k, float64(ay)*k, ZoneLabelCap(tile*k)), ox, oy, bw, bh
}

// DrawZoneLabel names a zone once, in the middle of it, on a dark plate.
// The label scales with the zone (ZoneLabelLayout) and is anchored to the
// zone cell nearest its middle; layout is in logical pixels, from the
// physical on-screen box.
func (o *Zones) DrawZoneLabel(v *render.View, screen *ebiten.Image, z game.ZoneView, g *ZoneGeom, sw, sh int, tile float64) {
	if len(z.Cells) == 0 {
		return
	}
	plan, ox, oy, bw, bh := o.ZoneLabelPlanFor(v, z, g, tile)
	if !sprite.Visible(ox, oy, float32(bw), float32(bh), sw, sh) || !plan.Ok {
		return
	}
	s := gfx.UIScale()
	if pr := plan.Plate; !sprite.Visible(pr.X*float32(s), pr.Y*float32(s), pr.W*float32(s), pr.H*float32(s), sw, sh) {
		return
	}
	paintZoneLabelPlate(screen, plan.Plate)
	gfx.WorldText(screen, plan.Text, plan.tx*s, plan.ty*s, plan.Style) // physical anchor in, logical out
}

// ZoneLabelAt is the zone whose name plate is under physical pixel
// (mx, my), the topmost. A hidden zone has no plate, unless it is shown
// (the selected one, whose name drawZoneSelection draws).
func (o *Zones) ZoneLabelAt(v *render.View, w game.WorldView, mx, my int, tile float64, shown uint16) (game.ZoneView, bool) {
	k := 1 / gfx.MapUIScale() // a hit test (Update): map pixels -> logical
	px, py := float32(float64(mx)*k), float32(float64(my)*k)
	for i := len(w.Zones) - 1; i >= 0; i-- {
		z := w.Zones[i]
		if len(z.Cells) == 0 || z.Hidden && z.ID != shown {
			continue
		}
		plan, _, _, _, _ := o.ZoneLabelPlanFor(v, z, o.ZoneGeomOf(z), tile)
		if plan.Ok && plan.Plate.Contains(px, py) {
			return z, true
		}
	}
	return game.ZoneView{}, false
}

// paintZoneLabelPlate fills a zone label's plate (logical pixels).
func paintZoneLabelPlate(screen *ebiten.Image, p gfx.Rect) {
	gfx.FillRect(screen, p.X, p.Y, p.W, p.H, zoneLabelPlateClr, false)
}

// frameLists reports whether w is the world this frame's chunk lists
// index (v.W): the same shared lists.
func frameLists(v *render.View, ch *Chunks, w *game.WorldView) bool {
	d := &v.W
	return len(ch.Views) > 0 &&
		light.First(w.Buildings) == light.First(d.Buildings) && len(w.Buildings) == len(d.Buildings) &&
		light.First(w.Plants) == light.First(d.Plants) && len(w.Plants) == len(d.Plants) &&
		light.First(w.Items) == light.First(d.Items) && len(w.Items) == len(d.Items) &&
		light.First(w.Frames) == light.First(d.Frames) && len(w.Frames) == len(d.Frames)
}

// resetCells is m emptied for reuse (a new map when nil).
func resetCells(m map[[2]int]bool) map[[2]int]bool {
	if m == nil {
		return map[[2]int]bool{}
	}
	clear(m)
	return m
}
