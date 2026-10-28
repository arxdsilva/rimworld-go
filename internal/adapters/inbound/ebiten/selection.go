package ebiten

import (
	"cmp"
	"math"
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
	sp := r.art.Sprite(r.animalBaseKey(an)) // decoded at most once, for its box
	if sp == nil {
		return 0, 0, 0, 0, false
	}
	bx0, by0, bx1, by1 := sp.OpaqueBox()
	m := &sp.Meta
	p := m.PxPerTile()
	// tiles from the drawn tile's top-left
	x0, y0, x1, y1 = (bx0-m.Origin[0])/p, (by0-m.Origin[1])/p, (bx1-m.Origin[0])/p, (by1-m.Origin[1])/p
	if r.animalMirrored(an) {
		x0, x1 = 2*figurePivotX-x1, 2*figurePivotX-x0
	}
	if s := an.Scale; s > 0 && s < 1 {
		x0, x1 = figurePivotX+(x0-figurePivotX)*s, figurePivotX+(x1-figurePivotX)*s
		y0, y1 = figurePivotY+(y0-figurePivotY)*s, figurePivotY+(y1-figurePivotY)*s
	}
	wx, wy := r.interpAnimal(an)
	tile := r.tilePx()
	k := 1.0
	if ct := lodTile(tile); tile > 0 && ct > tile {
		wx, wy = creatureAt(wx, wy, tile, ct)
		k = ct / tile
	}
	return wx + x0*k, wy + y0*k, wx + x1*k, wy + y1*k, true
}

// coversTile reports a body box overlapping tile (tx, ty) by pickCover.
func coversTile(x0, y0, x1, y1 float64, tx, ty int) bool {
	fx, fy := float64(tx), float64(ty)
	return math.Min(x1, fx+1)-math.Max(x0, fx) >= pickCover && math.Min(y1, fy+1)-math.Max(y0, fy) >= pickCover
}

// animalsCovering lists the animals whose drawn body covers tile (tx, ty)
// from another cell, nearest first (PLAY-022 C7). Only animals whose cell
// lies within the largest body's reach of the tile are measured.
func (r *Renderer) animalsCovering(w game.WorldView, tx, ty int) []selCand {
	pad, padS := r.animalPads(r.tilePx())
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
		if worldFogged(w, an.X, an.Y) {
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
	out := make([]selCand, len(hits))
	for i, h := range hits {
		out[i] = selCand{Kind: selAnimal, ID: h.id}
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
