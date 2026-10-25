package ebiten

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Roof presentation (ART §16.1, UI-010): roofed tiles are lit as indoors
// (light.go), with soft eave shadows inside walls; the Roof overlay (R or
// the HUD button, forced on while a roof tool is held) shows built,
// natural and planned roofs.

// roofBuilt and the rest mirror roof.Cell (internal/domain/world/roof).
const (
	roofBuilt uint8 = 1 << iota
	roofNatural
	roofPlanBuild
	roofPlanRemove
	roofSupported
	roofNoAuto
	roofThick // with roofNatural: deep rock overhead (MAPGEN-2)
)

// roofAutoMax mirrors roof.AutoMaxCells: larger rooms are not auto-roofed.
const roofAutoMax = 150

// roofCellAt is the roof flags at (x, y) of w; 0 out of bounds.
func roofCellAt(w game.WorldView, x, y int) uint8 {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return 0
	}
	i := y*w.TerrainW + x
	if i >= len(w.RoofCells) {
		return 0
	}
	return w.RoofCells[i]
}

// roofed reports a built or natural roof.
func roofed(c uint8) bool { return c&(roofBuilt|roofNatural) != 0 }

// roomIDAt is the room id at (x, y): -1 outdoors, -2 a wall, -3 unknown.
func roomIDAt(w game.WorldView, x, y int) int {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return -3
	}
	i := y*w.TerrainW + x
	if i >= len(w.RoomGrid) {
		return -3
	}
	return w.RoomGrid[i]
}

// roofLabelAt is the hover panel's roof line for (x, y): "" on walls, out
// of bounds and without roof data.
func roofLabelAt(w game.WorldView, x, y int) string {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW || y*w.TerrainW+x >= len(w.RoofCells) {
		return ""
	}
	id := roomIDAt(w, x, y)
	if id == -2 {
		return ""
	}
	c := roofCellAt(w, x, y)
	switch {
	case c&roofPlanRemove != 0:
		return "Roof removal planned"
	case c&roofPlanBuild != 0:
		return "Roof planned"
	case c&(roofNatural|roofThick) == roofNatural|roofThick:
		return "Thick rock overhead (permanent)"
	case c&roofNatural != 0:
		return "Natural roof (rock)"
	case c&roofBuilt != 0:
		return "Roofed"
	case id >= 0 && id < len(w.Rooms) && w.Rooms[id].Cells > roofAutoMax:
		return "Unroofed (room too large to auto-roof)"
	}
	return "Unroofed"
}

// roofDrawState is the roof pass's reusable state (kept on envState).
type roofDrawState struct {
	batch        triBatch
	edges        []eaveEdge
	overlayVerts int // vertices the last overlay drew, for tests
}

// --- eaves ---------------------------------------------------------------------------

// Sides of a tile, for eaves.
const (
	sideN = iota
	sideE
	sideS
	sideW
)

// eaveEdge is a roofed tile's side against a wall.
type eaveEdge struct{ x, y, side int }

// eaveEdges lists, in the tile range, every side of a roofed (non-wall)
// tile that faces a wall: where the roof's eave shadow falls.
func eaveEdges(w game.WorldView, x0, y0, x1, y1 int, out []eaveEdge) []eaveEdge {
	out = out[:0]
	if len(w.RoofCells) == 0 || len(w.RoomGrid) == 0 {
		return out
	}
	nb := [4][2]int{sideN: {0, -1}, sideE: {1, 0}, sideS: {0, 1}, sideW: {-1, 0}}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !roofed(roofCellAt(w, x, y)) || roomIDAt(w, x, y) == -2 {
				continue
			}
			for s, d := range nb {
				if roomIDAt(w, x+d[0], y+d[1]) == -2 {
					out = append(out, eaveEdge{x, y, s})
				}
			}
		}
	}
	return out
}

// eaveAlpha is the eave shadow's alpha (0..255): 30 by day, 18 at night.
func eaveAlpha(l lightState) uint8 {
	if l.SunL[2] > 0 {
		return 30
	}
	return 18
}

// eaveWidth is the eave gradient's width in screen px for a side at tile
// size tile: 10 px under a top wall, 6 px elsewhere, at zoom 1.
func eaveWidth(side int, tile float64) float64 {
	px := 6.0
	if side == sideN {
		px = 10
	}
	return px * tile / TileSize
}

// drawEaves draws the soft shadow inside walls around roofed interiors.
func (r *Renderer) drawEaves(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if len(w.RoofCells) == 0 {
		return
	}
	r.setView(sw, sh, tile)
	st := &r.env.roof
	x0, y0, x1, y1 := r.visibleTiles()
	st.edges = eaveEdges(w, x0, y0, x1, y1, st.edges)
	if len(st.edges) == 0 {
		return
	}
	a := float64(eaveAlpha(r.lit.light)) / 255
	c := color.RGBA{shadowColorU8(0), shadowColorU8(1), shadowColorU8(2), 255}
	b := &st.batch
	b.reset()
	for _, e := range st.edges {
		sx, sy := r.worldToScreen(e.x, e.y)
		x, y := float64(sx), float64(sy)
		px := eaveWidth(e.side, tile)
		switch e.side {
		case sideN:
			b.rect(x, y, tile, px, c, a, a, 0, 0)
		case sideS:
			b.rect(x, y+tile-px, tile, px, c, 0, 0, a, a)
		case sideE:
			b.rect(x+tile-px, y, px, tile, c, 0, a, 0, a)
		case sideW:
			b.rect(x, y, px, tile, c, a, 0, a, 0)
		}
	}
	b.flush(screen, r.env.whiteImg(), ebiten.BlendSourceOver)
}

func shadowColorU8(i int) uint8 { return uint8(shadowColor[i]) }

// --- overlay -------------------------------------------------------------------------

// roofStyle is how the Roof overlay marks a tile; a zero colour is off.
type roofStyle struct {
	fill    color.RGBA // the tile fill
	border  color.RGBA // solid edge where a built roof meets a non-built cell
	stipple color.RGBA // diagonal dots (natural)
	dash    color.RGBA // dashed outline (planned)
	hatch   color.RGBA // diagonal hatch (removal planned)
}

var (
	roofBuiltFill     = color.RGBA{90, 140, 210, 70}
	roofBuiltBorder   = color.RGBA{120, 170, 235, 150}
	roofNaturalFill   = color.RGBA{110, 110, 125, 85}
	roofNaturalDots   = color.RGBA{70, 70, 84, 150}
	roofPlanDash      = color.RGBA{235, 175, 60, 200}
	roofPlanFill      = color.RGBA{235, 175, 60, 30}
	roofRemoveHatch   = color.RGBA{220, 70, 60, 120}
	roofUnsupportFill = color.RGBA{220, 60, 50, 90}
	roofUnsupportMark = color.RGBA{235, 80, 70, 220}
)

// roofStyleFor is the overlay style of roof cell c.
func roofStyleFor(c uint8) roofStyle {
	var s roofStyle
	switch {
	case c&roofBuilt != 0:
		s.fill, s.border = roofBuiltFill, roofBuiltBorder
	case c&roofNatural != 0:
		s.fill, s.stipple = roofNaturalFill, roofNaturalDots
	case c&roofPlanBuild != 0:
		s.fill = roofPlanFill
	}
	if c&roofPlanBuild != 0 {
		s.dash = roofPlanDash
	}
	if c&roofPlanRemove != 0 {
		s.hatch = roofRemoveHatch
	}
	return s
}

// roofOverlayOn reports the Roof overlay is showing.
func (r *Renderer) roofOverlayOn() bool { return r.roofOverlay || r.roofOverlayForced }

// alphaOf is c's alpha as 0..1.
func alphaOf(c color.RGBA) float64 { return float64(c.A) / 255 }

// drawRoofOverlay draws the Roof overlay when it is on, in one batch.
func (r *Renderer) drawRoofOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.drawHomeOverlay(screen, w, sw, sh, tile) // home_draw.go: under the roof tint
	st := &r.env.roof
	st.overlayVerts = 0
	if !r.roofOverlayOn() || len(w.RoofCells) == 0 {
		return
	}
	r.setView(sw, sh, tile)
	x0, y0, x1, y1 := r.visibleTiles()
	b := &st.batch
	b.reset()
	k := tile / TileSize
	edge := math.Max(1.5, 2*k)
	thin := math.Max(1, 1.5*k)
	nb := [4][2]int{sideN: {0, -1}, sideE: {1, 0}, sideS: {0, 1}, sideW: {-1, 0}}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c := roofCellAt(w, x, y)
			if c == 0 {
				continue
			}
			s := roofStyleFor(c)
			sx, sy := r.worldToScreen(x, y)
			X, Y := float64(sx), float64(sy)
			if s.fill.A > 0 {
				a := alphaOf(s.fill)
				b.rect(X, Y, tile, tile, s.fill, a, a, a, a)
			}
			if s.stipple.A > 0 {
				a, d := alphaOf(s.stipple), math.Max(1.5, 2.5*k)
				for i := 0; i < 4; i++ {
					for j := 0; j < 4; j++ {
						if (i+j)%2 == 0 {
							b.rect(X+(float64(i)+0.5)*tile/4-d/2, Y+(float64(j)+0.5)*tile/4-d/2, d, d, s.stipple, a, a, a, a)
						}
					}
				}
			}
			if s.hatch.A > 0 {
				a := alphaOf(s.hatch)
				for t := 0.25; t < 2; t += 0.25 {
					if t <= 1 {
						b.line(X+t*tile, Y, X, Y+t*tile, thin, s.hatch, a)
					} else {
						b.line(X+tile, Y+(t-1)*tile, X+(t-1)*tile, Y+tile, thin, s.hatch, a)
					}
				}
			}
			for side, d := range nb {
				n := roofCellAt(w, x+d[0], y+d[1])
				if s.border.A > 0 && n&roofBuilt == 0 {
					r.edgeStrip(b, X, Y, tile, side, edge, s.border, 0, 1)
				}
				if s.dash.A > 0 && n&roofPlanBuild == 0 {
					for t := 0.0; t < 1; t += 0.25 {
						r.edgeStrip(b, X, Y, tile, side, edge, s.dash, t+0.03, t+0.16)
					}
				}
			}
		}
	}
	st.overlayVerts = len(b.verts)
	b.flush(screen, r.env.whiteImg(), ebiten.BlendSourceOver)
}

// edgeStrip is a strip width px wide just inside a tile's side, from t0
// to t1 along it (0..1).
func (r *Renderer) edgeStrip(b *triBatch, X, Y, tile float64, side int, width float64, c color.RGBA, t0, t1 float64) {
	a := alphaOf(c)
	l, n := (t1-t0)*tile, t0*tile
	switch side {
	case sideN:
		b.rect(X+n, Y, l, width, c, a, a, a, a)
	case sideS:
		b.rect(X+n, Y+tile-width, l, width, c, a, a, a, a)
	case sideW:
		b.rect(X, Y+n, width, l, c, a, a, a, a)
	case sideE:
		b.rect(X+tile-width, Y+n, width, l, c, a, a, a, a)
	}
}

// drawRoofUnsupported marks tile (x, y) where a planned roof would have no
// support: a red fill and an "×" (the roof tool's ghost, archtools.go).
func (r *Renderer) drawRoofUnsupported(screen *ebiten.Image, x, y int, tile float64) {
	b := &r.env.roof.batch
	b.reset()
	sx, sy := r.worldToScreen(x, y)
	X, Y := float64(sx), float64(sy)
	a := alphaOf(roofUnsupportFill)
	b.rect(X, Y, tile, tile, roofUnsupportFill, a, a, a, a)
	m, wd := 0.28*tile, math.Max(1.5, 3*tile/TileSize)
	b.line(X+m, Y+m, X+tile-m, Y+tile-m, wd, roofUnsupportMark, alphaOf(roofUnsupportMark))
	b.line(X+tile-m, Y+m, X+m, Y+tile-m, wd, roofUnsupportMark, alphaOf(roofUnsupportMark))
	b.flush(screen, r.env.whiteImg(), ebiten.BlendSourceOver)
}
