package overlay

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/application/game"
)

// Roof presentation (ART §16.1, UI-010): Roofed tiles are lit as indoors
// (light.go), with soft eave shadows inside walls; the Roof overlay (R or
// the HUD button, forced on while a roof tool is held) shows built,
// natural and planned roofs.

// RoofBuilt and the rest mirror roof.Cell (internal/domain/world/roof).
const (
	RoofBuilt uint8 = 1 << iota
	RoofNatural
	RoofPlanBuild
	RoofPlanRemove
	RoofSupported
	RoofNoAuto
	RoofThick // with RoofNatural: deep rock overhead (MAPGEN-2)
)

// RoofAutoMax mirrors roof.AutoMaxCells: larger rooms are not auto-Roofed.
const RoofAutoMax = 150

// RoofCellAt is the roof flags at (x, y) of w; 0 out of bounds.
func RoofCellAt(w game.WorldView, x, y int) uint8 {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return 0
	}
	i := y*w.TerrainW + x
	if i >= len(w.RoofCells) {
		return 0
	}
	return w.RoofCells[i]
}

// Roofed reports a built or natural roof.
func Roofed(c uint8) bool { return c&(RoofBuilt|RoofNatural) != 0 }

// RoomIDAt is the room id at (x, y): -1 outdoors, -2 a wall, -3 unknown.
func RoomIDAt(w game.WorldView, x, y int) int {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return -3
	}
	i := y*w.TerrainW + x
	if i >= len(w.RoomGrid) {
		return -3
	}
	return w.RoomGrid[i]
}

// RoofLabelAt is the hover panel's roof line for (x, y): "" on walls, out
// of bounds and without roof data.
func RoofLabelAt(w game.WorldView, x, y int) string {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW || y*w.TerrainW+x >= len(w.RoofCells) {
		return ""
	}
	id := RoomIDAt(w, x, y)
	if id == -2 {
		return ""
	}
	c := RoofCellAt(w, x, y)
	switch {
	case c&RoofPlanRemove != 0:
		return "Roof removal planned"
	case c&RoofPlanBuild != 0:
		return "Roof planned"
	case c&(RoofNatural|RoofThick) == RoofNatural|RoofThick:
		return "Thick rock overhead (permanent)"
	case c&RoofNatural != 0:
		return "Natural roof (rock)"
	case c&RoofBuilt != 0:
		return "Roofed"
	case id >= 0 && id < len(w.Rooms) && w.Rooms[id].Cells > RoofAutoMax:
		return "Unroofed (room too large to auto-roof)"
	}
	return "Unroofed"
}

// RoofDrawState is the roof pass's reusable state (Set.RoofDraw).
type RoofDrawState struct {
	batch        gfx.TriBatch
	Edges        []EaveEdge
	OverlayVerts int // vertices the last overlay drew, for tests
}

// --- eaves ---------------------------------------------------------------------------

// Sides of a tile, for eaves.
const (
	SideN = iota
	SideE
	SideS
	SideW
)

// EaveEdge is a Roofed tile's side against a wall.
type EaveEdge struct{ X, Y, Side int }

// EaveEdges lists, in the tile range, every side of a Roofed (non-wall)
// tile that faces a wall: where the roof's eave shadow falls.
func EaveEdges(w game.WorldView, x0, y0, x1, y1 int, out []EaveEdge) []EaveEdge {
	out = out[:0]
	if len(w.RoofCells) == 0 || len(w.RoomGrid) == 0 {
		return out
	}
	nb := [4][2]int{SideN: {0, -1}, SideE: {1, 0}, SideS: {0, 1}, SideW: {-1, 0}}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !Roofed(RoofCellAt(w, x, y)) || RoomIDAt(w, x, y) == -2 {
				continue
			}
			for s, d := range nb {
				if RoomIDAt(w, x+d[0], y+d[1]) == -2 {
					out = append(out, EaveEdge{x, y, s})
				}
			}
		}
	}
	return out
}

// EaveAlpha is the eave shadow's alpha (0..255): 30 by day, 18 at night.
func EaveAlpha(l light.State) uint8 {
	if l.SunL[2] > 0 {
		return 30
	}
	return 18
}

// EaveWidth is the eave gradient's width in screen px for a side at tile
// size tile: 10 px under a top wall, 6 px elsewhere, at zoom 1.
func EaveWidth(side int, tile float64) float64 {
	px := 6.0
	if side == SideN {
		px = 10
	}
	return px * tile / camera.TileSize
}

// DrawEaves draws the soft shadow inside walls around Roofed interiors.
func (o *Set) DrawEaves(v *render.View, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if len(w.RoofCells) == 0 {
		return
	}
	st := &o.RoofDraw
	x0, y0, x1, y1 := visibleTiles(v, sw, sh, tile)
	st.Edges = EaveEdges(w, x0, y0, x1, y1, st.Edges)
	if len(st.Edges) == 0 {
		return
	}
	a := float64(EaveAlpha(o.Lit.Light)) / 255
	c := color.RGBA{shadowColorU8(0), shadowColorU8(1), shadowColorU8(2), 255}
	b := &st.batch
	b.Reset()
	for _, e := range st.Edges {
		sx, sy := v.WorldToScreen(e.X, e.Y)
		x, y := float64(sx), float64(sy)
		px := EaveWidth(e.Side, tile)
		switch e.Side {
		case SideN:
			b.Rect(x, y, tile, px, c, a, a, 0, 0)
		case SideS:
			b.Rect(x, y+tile-px, tile, px, c, 0, 0, a, a)
		case SideE:
			b.Rect(x+tile-px, y, px, tile, c, 0, a, 0, a)
		case SideW:
			b.Rect(x, y, px, tile, c, a, 0, a, 0)
		}
	}
	b.Flush(screen, o.whiteImg(), ebiten.BlendSourceOver)
}

func shadowColorU8(i int) uint8 { return uint8(light.ShadowColor[i]) }

// --- overlay -------------------------------------------------------------------------

// RoofStyle is how the Roof overlay marks a tile; a zero colour is off.
type RoofStyle struct {
	Fill    color.RGBA // the tile fill
	Border  color.RGBA // solid edge where a built roof meets a non-built cell
	Stipple color.RGBA // diagonal dots (natural)
	Dash    color.RGBA // dashed outline (planned)
	Hatch   color.RGBA // diagonal hatch (removal planned)
}

var (
	roofBuiltFill     = color.RGBA{90, 140, 210, 70}
	roofBuiltBorder   = color.RGBA{120, 170, 235, 150}
	roofNaturalFill   = color.RGBA{110, 110, 125, 85}
	RoofNaturalDots   = color.RGBA{70, 70, 84, 150}
	roofPlanDash      = color.RGBA{235, 175, 60, 200}
	roofPlanFill      = color.RGBA{235, 175, 60, 30}
	roofRemoveHatch   = color.RGBA{220, 70, 60, 120}
	roofUnsupportFill = color.RGBA{220, 60, 50, 90}
	roofUnsupportMark = color.RGBA{235, 80, 70, 220}
)

// RoofStyleFor is the overlay style of roof cell c.
func RoofStyleFor(c uint8) RoofStyle {
	var s RoofStyle
	switch {
	case c&RoofBuilt != 0:
		s.Fill, s.Border = roofBuiltFill, roofBuiltBorder
	case c&RoofNatural != 0:
		s.Fill, s.Stipple = roofNaturalFill, RoofNaturalDots
	case c&RoofPlanBuild != 0:
		s.Fill = roofPlanFill
	}
	if c&RoofPlanBuild != 0 {
		s.Dash = roofPlanDash
	}
	if c&RoofPlanRemove != 0 {
		s.Hatch = roofRemoveHatch
	}
	return s
}

// alphaOf is c's alpha as 0..1.
func alphaOf(c color.RGBA) float64 { return float64(c.A) / 255 }

// DrawRoofOverlay draws the Roof overlay when it is on, in one batch.
func (o *Set) DrawRoofOverlay(v *render.View, screen *ebiten.Image, w game.WorldView, areas func() []game.AreaView, sw, sh int, tile float64) {
	o.DrawHomeOverlay(v, screen, w, sw, sh, tile)        // home_draw.go: under the roof tint
	o.DrawAreaOverlay(v, screen, w, areas, sw, sh, tile) // area_overlay.go: the armed or hovered allowed area
	st := &o.RoofDraw
	st.OverlayVerts = 0
	if !o.Roof.Shown() || len(w.RoofCells) == 0 {
		return
	}
	x0, y0, x1, y1 := visibleTiles(v, sw, sh, tile)
	b := &st.batch
	b.Reset()
	k := tile / camera.TileSize
	edge := math.Max(1.5, 2*k)
	thin := math.Max(1, 1.5*k)
	nb := [4][2]int{SideN: {0, -1}, SideE: {1, 0}, SideS: {0, 1}, SideW: {-1, 0}}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c := RoofCellAt(w, x, y)
			if c == 0 {
				continue
			}
			s := RoofStyleFor(c)
			sx, sy := v.WorldToScreen(x, y)
			X, Y := float64(sx), float64(sy)
			if s.Fill.A > 0 {
				a := alphaOf(s.Fill)
				b.Rect(X, Y, tile, tile, s.Fill, a, a, a, a)
			}
			if s.Stipple.A > 0 {
				a, d := alphaOf(s.Stipple), math.Max(1.5, 2.5*k)
				for i := 0; i < 4; i++ {
					for j := 0; j < 4; j++ {
						if (i+j)%2 == 0 {
							b.Rect(X+(float64(i)+0.5)*tile/4-d/2, Y+(float64(j)+0.5)*tile/4-d/2, d, d, s.Stipple, a, a, a, a)
						}
					}
				}
			}
			if s.Hatch.A > 0 {
				a := alphaOf(s.Hatch)
				for t := 0.25; t < 2; t += 0.25 {
					if t <= 1 {
						b.Line(X+t*tile, Y, X, Y+t*tile, thin, s.Hatch, a)
					} else {
						b.Line(X+tile, Y+(t-1)*tile, X+(t-1)*tile, Y+tile, thin, s.Hatch, a)
					}
				}
			}
			for side, d := range nb {
				n := RoofCellAt(w, x+d[0], y+d[1])
				if s.Border.A > 0 && n&RoofBuilt == 0 {
					edgeStrip(b, X, Y, tile, side, edge, s.Border, 0, 1)
				}
				if s.Dash.A > 0 && n&RoofPlanBuild == 0 {
					for t := 0.0; t < 1; t += 0.25 {
						edgeStrip(b, X, Y, tile, side, edge, s.Dash, t+0.03, t+0.16)
					}
				}
			}
		}
	}
	st.OverlayVerts = len(b.Verts)
	b.Flush(screen, o.whiteImg(), ebiten.BlendSourceOver)
}

// edgeStrip is a strip width px wide just inside a tile's side, from t0
// to t1 along it (0..1).
func edgeStrip(b *gfx.TriBatch, X, Y, tile float64, side int, width float64, c color.RGBA, t0, t1 float64) {
	a := alphaOf(c)
	l, n := (t1-t0)*tile, t0*tile
	switch side {
	case SideN:
		b.Rect(X+n, Y, l, width, c, a, a, a, a)
	case SideS:
		b.Rect(X+n, Y+tile-width, l, width, c, a, a, a, a)
	case SideW:
		b.Rect(X, Y+n, width, l, c, a, a, a, a)
	case SideE:
		b.Rect(X+tile-width, Y+n, width, l, c, a, a, a, a)
	}
}

// DrawRoofUnsupported marks tile (x, y) where a planned roof would have no
// support: a red fill and an "×" (the roof tool's ghost, archtools.go).
func (o *Set) DrawRoofUnsupported(v *render.View, screen *ebiten.Image, x, y int, tile float64) {
	b := &o.RoofDraw.batch
	b.Reset()
	sx, sy := v.WorldToScreen(x, y)
	X, Y := float64(sx), float64(sy)
	a := alphaOf(roofUnsupportFill)
	b.Rect(X, Y, tile, tile, roofUnsupportFill, a, a, a, a)
	m, wd := 0.28*tile, math.Max(1.5, 3*tile/camera.TileSize)
	b.Line(X+m, Y+m, X+tile-m, Y+tile-m, wd, roofUnsupportMark, alphaOf(roofUnsupportMark))
	b.Line(X+tile-m, Y+m, X+m, Y+tile-m, wd, roofUnsupportMark, alphaOf(roofUnsupportMark))
	b.Flush(screen, o.whiteImg(), ebiten.BlendSourceOver)
}
