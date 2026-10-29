package ebiten

import (
	"cmp"
	"math"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"
	selectionpkg "rimworld-go/internal/adapters/inbound/ebiten/selection"
	"slices"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Map selection (UI-011): a left click picks what is on the tile, in
// precedence pawn → animal → ground item → building → plant → floor;
// clicking the same tile again cycles to the next thing there. Selecting
// one kind clears every other selection.

// pickCover is how much of a tile (tiles, in x and in y) an animal's body
// box must overlap to cover it: a sliver of outline or shadow-side fringe
// over a neighbour does not.
const pickCover = 0.15

// animalBodyRect is the box, in world tiles, of animal an's drawn body:
// its standing frame's opaque pixels (artSprite.opaqueBox) placed as the
// object pass draws it (drawn position, minimum creature size, juvenile
// scale, west mirrored), without the passing pose of a lunge or a breath.
func (r *Renderer) animalBodyRect(an game.AnimalView) (x0, y0, x1, y1 float64, ok bool) {
	c, v := r.pipe().Creatures, r.liveView()
	sp := r.art.Sprite(c.AnimalBaseKey(v, an)) // decoded at most once, for its box
	if sp == nil {
		return 0, 0, 0, 0, false
	}
	bx0, by0, bx1, by1 := sp.OpaqueBox()
	m := &sp.Meta
	p := m.PxPerTile()
	// tiles from the drawn tile's top-left
	x0, y0, x1, y1 = (bx0-m.Origin[0])/p, (by0-m.Origin[1])/p, (bx1-m.Origin[0])/p, (by1-m.Origin[1])/p
	if c.AnimalMirrored(v, an) {
		x0, x1 = 2*motionpkg.FigurePivotX-x1, 2*motionpkg.FigurePivotX-x0
	}
	if s := an.Scale; s > 0 && s < 1 {
		x0, x1 = motionpkg.FigurePivotX+(x0-motionpkg.FigurePivotX)*s, motionpkg.FigurePivotX+(x1-motionpkg.FigurePivotX)*s
		y0, y1 = motionpkg.FigurePivotY+(y0-motionpkg.FigurePivotY)*s, motionpkg.FigurePivotY+(y1-motionpkg.FigurePivotY)*s
	}
	wx, wy := c.AnimalAt(v, an)
	tile := r.cam.Tile()
	k := 1.0
	if ct := worldpkg.LodTile(tile); tile > 0 && ct > tile {
		wx, wy = worldpkg.CreatureAt(wx, wy, tile, ct)
		k = ct / tile
	}
	return wx + x0*k, wy + y0*k, wx + x1*k, wy + y1*k, true
}

// animalPads is how far, in tiles, an animal's drawn body reaches past
// its cell at tile px a tile (sideways and up, and down).
func (r *Renderer) animalPads(tile float64) (pad, padS int) {
	return worldpkg.AnimalPadsOf(r.art, tile)
}

// coversTile reports a body box overlapping tile (tx, ty) by pickCover.
func coversTile(x0, y0, x1, y1 float64, tx, ty int) bool {
	fx, fy := float64(tx), float64(ty)
	return math.Min(x1, fx+1)-math.Max(x0, fx) >= pickCover && math.Min(y1, fy+1)-math.Max(y0, fy) >= pickCover
}

// animalsCovering lists the animals whose drawn body covers tile (tx, ty)
// from another cell, nearest first (PLAY-022 C7). Only animals whose cell
// lies within the largest body's reach of the tile are measured.
func (r *Renderer) animalsCovering(w game.WorldView, tx, ty int) []selectionpkg.Cand {
	pad, padS := r.animalPads(r.cam.Tile())
	type hit struct {
		id entity.EntityID
		d  float64
	}
	var hits []hit
	for i := range w.Animals {
		an := &w.Animals[i]
		if an.X == tx && an.Y == ty || an.X < tx-pad || an.X > tx+pad || an.Y < ty-pad || an.Y > ty+padS {
			continue // on the tile itself (selectablesAt), or too far to reach it
		}
		if overlay.WorldFogged(w, an.X, an.Y) {
			continue
		}
		x0, y0, x1, y1, ok := r.animalBodyRect(*an)
		if !ok || !coversTile(x0, y0, x1, y1, tx, ty) {
			continue
		}
		cx, cy := (x0+x1)/2, (y0+y1)/2
		hits = append(hits, hit{an.ID, math.Hypot(cx-float64(tx)-0.5, cy-float64(ty)-0.5)})
	}
	if len(hits) == 0 {
		return nil
	}
	slices.SortFunc(hits, func(a, b hit) int {
		if c := cmp.Compare(a.d, b.d); c != 0 {
			return c
		}
		return cmp.Compare(a.id, b.id)
	})
	out := make([]selectionpkg.Cand, len(hits))
	for i, h := range hits {
		out[i] = selectionpkg.Cand{Kind: selectionpkg.Animal, ID: h.id}
	}
	return out
}

// pawnUnderCursor is the colonist drawn under physical pixel (mx, my), even
// mid-step onto a neighbouring tile.
func (r *Renderer) pawnUnderCursor(mx, my int) (entity.EntityID, bool) {
	wx, wy := r.screenToWorld(mx, my)
	for _, p := range r.snapshot() {
		px, py := r.interpWorld(p)
		if wx >= px && wx < px+1 && wy >= py && wy < py+1 {
			return p.ID, true
		}
	}
	return 0, false
}
