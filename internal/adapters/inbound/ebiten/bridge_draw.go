package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/terrain"
)

// Bridges (MAPGEN-3 §3.4): a bridge is a Building of Kind "bridge" in the
// foundation layer. It draws after the terrain (over the animated water)
// and before floors and objects, as one of 16 connection variants chosen
// by which sides open onto water, marsh or mud. A thin shadow lies on the
// water along the open edges. Land ends and neighbouring bridge cells get
// no edge, so a span reads as one structure.

const bridgeKind = "bridge"

// Mask bits (N, E, S, W) and the neighbour each looks at.
var bridgeSides = [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

// bridgeShadowClr is the water shadow along an open bridge edge.
var bridgeShadowClr = color.NRGBA{R: 8, G: 22, B: 36, A: 70}

// bridgeKey is the sprite of a bridge with open-side mask m (0..15).
func bridgeKey(m int) string { return artKey("structure", "bridge", m&15) }

// bridgeCache holds the set of bridge cells, rebuilt when the read
// model's StructVer moves (like the link grid).
type bridgeCache struct {
	ver   uint64
	n     int
	built bool
	cells map[[2]int]bool
}

// Sync returns the bridge cells of w; a zero StructVer rebuilds always.
func (c *bridgeCache) Sync(w game.WorldView) map[[2]int]bool {
	if c.built && w.StructVer != 0 && w.StructVer == c.ver {
		return c.cells
	}
	c.cells = map[[2]int]bool{}
	for i := range w.Buildings {
		if b := &w.Buildings[i]; b.Kind == bridgeKind {
			c.cells[[2]int{b.X, b.Y}] = true
		}
	}
	c.built, c.ver = true, w.StructVer
	return c.cells
}

// bridgeCells is the set of bridge cells of w (uncached; tests, one-offs).
func bridgeCells(w game.WorldView) map[[2]int]bool {
	var c bridgeCache
	return c.Sync(w)
}

// bridgeWet reports (x, y) is water, marsh or mud: a side that bridge
// edge faces. Outside the map is not wet.
func bridgeWet(w game.WorldView, x, y int) bool {
	code, ok := terrainCode(w, x, y)
	return ok && terrain.Bridgeable(terrain.Type(code))
}

// bridgeMask is the open-side mask of the bridge at (x, y): bit N,E,S,W
// (1,2,4,8) for each neighbour that is not a bridge and is wet.
func bridgeMask(w game.WorldView, set map[[2]int]bool, x, y int) int {
	m := 0
	for i, d := range bridgeSides {
		nx, ny := x+d[0], y+d[1]
		if !set[[2]int{nx, ny}] && bridgeWet(w, nx, ny) {
			m |= 1 << i
		}
	}
	return m
}

// bridgeShadowSides lists the side indexes (0 N, 1 E, 2 S, 3 W) set in m.
func bridgeShadowSides(m int) []int {
	var out []int
	for i := range bridgeSides {
		if m&(1<<i) != 0 {
			out = append(out, i)
		}
	}
	return out
}

// bridgeLabelAt is the hover line of a bridged cell ("Bridge over deep
// water", "over marsh", "over mud"); "" when no bridge stands there.
func bridgeLabelAt(w game.WorldView, x, y int) string {
	found := false
	for i := range w.Buildings {
		if b := &w.Buildings[i]; b.Kind == bridgeKind && b.X == x && b.Y == y {
			found = true
			break
		}
	}
	if !found {
		return ""
	}
	code, ok := terrainCode(w, x, y)
	if !ok {
		return "Bridge"
	}
	switch terrain.Type(code) {
	case terrain.DeepWater:
		return "Bridge over deep water"
	case terrain.ShallowWater:
		return "Bridge over shallow water"
	case terrain.Marsh:
		return "Bridge over marsh"
	case terrain.Mud:
		return "Bridge over mud"
	}
	return "Bridge"
}

// drawFoundation draws the foundation layer in view: bridge shadows on
// the water, then the deck sprites. Tiles are physical pixels (r.cam).
func (r *Renderer) drawFoundation(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.layer("foundation")
	if r.lod.tier != lodLive || r.art == nil { // baked below 48 px (lod.go)
		return
	}
	set := r.bridges.Sync(w)
	if len(set) == 0 {
		return
	}
	for _, cv := range r.lod.views {
		for _, i := range cv.Buildings {
			if int(i) >= len(w.Buildings) {
				continue
			}
			if b := &w.Buildings[i]; b.Kind == bridgeKind {
				r.drawBridgeCell(screen, w, set, b, tile, sw, sh)
			}
		}
	}
}

// drawBridgeCell draws one bridge deck with its water shadow.
func (r *Renderer) drawBridgeCell(dst *ebiten.Image, w game.WorldView, set map[[2]int]bool, b *game.BuildingView, tile float64, sw, sh int) {
	m := bridgeMask(w, set, b.X, b.Y)
	sx, sy := r.worldToScreen(b.X, b.Y)
	t := float32(tile)
	if t <= 0 || sx+t < 0 || sy+t < 0 || sx > float32(sw) || sy > float32(sh) {
		return
	}
	for _, q := range bridgeShadowRects(sx, sy, t, m) {
		vector.FillRect(dst, q[0], q[1], q[2], q[3], bridgeShadowClr, false)
	}
	key := bridgeKey(m)
	if r.art.Has(key) {
		r.drawSprite(dst, key, color.NRGBA{}, float64(b.X), float64(b.Y), tile, sw, sh)
		return
	}
	vector.FillRect(dst, sx, sy, t, t, color.NRGBA{R: 150, G: 108, B: 62, A: 255}, false) // art not landed yet
}

// bridgeLine is a bridge's inspector support line; "" for other buildings.
func bridgeLine(b game.BuildingView) string {
	if b.Kind != bridgeKind {
		return ""
	}
	return "Holds light things: no stone walls or floors"
}

// bridgeShadowRects are the shadow strips (x, y, w, h) on the water along
// the open sides of a bridge cell at screen (sx, sy), t pixels a side:
// thin, outside the cell, one per set bit of mask m.
func bridgeShadowRects(sx, sy, t float32, m int) [][4]float32 {
	th := max(t/10, 1)
	var out [][4]float32
	for _, s := range bridgeShadowSides(m) {
		switch s {
		case 0:
			out = append(out, [4]float32{sx, sy - th, t, th})
		case 1:
			out = append(out, [4]float32{sx + t, sy, th, t})
		case 2:
			out = append(out, [4]float32{sx, sy + t, t, th})
		case 3:
			out = append(out, [4]float32{sx - th, sy, th, t})
		}
	}
	return out
}
